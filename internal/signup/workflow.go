package signup

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/mail"
	"net/url"
	"strings"
	"sync"
)

type EmailSender interface {
	SendEmail(context.Context, Email, string) (SendResult, error)
}

type Email struct {
	To      string
	Subject string
	HTML    string
}

type SendResult struct {
	MessageID string
}

type Request struct {
	CreatorID string `json:"creator_id"`
	Email     string `json:"email"`
	AssetID   string `json:"asset_id"`
	Title     string `json:"title"`
}

type State struct {
	CreatorID     string `json:"creator_id"`
	AssetID       string `json:"asset_id"`
	AssetState    string `json:"asset_state"`
	JobState      string `json:"job_state"`
	DeliveryState string `json:"delivery_state"`
	MessageID     string `json:"message_id,omitempty"`
}

type Workflow struct {
	Sender    EmailSender
	PublicURL string
	Secret    []byte

	mu      sync.Mutex
	states  map[string]State
	tokens  map[string]string
	pending map[string]chan struct{}
}

func New(sender EmailSender, publicURL string, secret []byte) *Workflow {
	return &Workflow{Sender: sender, PublicURL: strings.TrimRight(publicURL, "/"), Secret: secret, states: map[string]State{}, tokens: map[string]string{}, pending: map[string]chan struct{}{}}
}

func (w *Workflow) Start(ctx context.Context, req Request) (State, error) {
	if err := validate(req); err != nil {
		return State{}, err
	}

	w.mu.Lock()
	if ready, ok := w.states[req.CreatorID]; ok {
		w.mu.Unlock()
		return ready, nil
	}
	if done, ok := w.pending[req.CreatorID]; ok {
		w.mu.Unlock()
		select {
		case <-ctx.Done():
			return State{}, ctx.Err()
		case <-done:
			w.mu.Lock()
			state, exists := w.states[req.CreatorID]
			w.mu.Unlock()
			if !exists {
				return State{}, errors.New("signup delivery did not complete")
			}
			return state, nil
		}
	}
	done := make(chan struct{})
	w.pending[req.CreatorID] = done
	w.mu.Unlock()

	token := w.sign(req.CreatorID, req.Email)
	verifyURL := w.PublicURL + "/verify?creator_id=" + url.QueryEscape(req.CreatorID) + "&token=" + token
	result, err := w.Sender.SendEmail(ctx, Email{
		To:      req.Email,
		Subject: "Verify your Streamforge creator email",
		HTML:    "<p>Confirm email for <strong>" + html.EscapeString(req.Title) + "</strong>.</p><p><a href=\"" + html.EscapeString(verifyURL) + "\">Verify email</a></p>",
	}, "creator-signup-"+req.CreatorID)

	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.pending, req.CreatorID)
	close(done)
	if err != nil {
		return State{}, err
	}
	state := State{CreatorID: req.CreatorID, AssetID: req.AssetID, AssetState: "held_for_verification", JobState: "waiting_for_creator", DeliveryState: "email_verification_sent", MessageID: result.MessageID}
	w.states[req.CreatorID] = state
	w.tokens[req.CreatorID] = token
	return state, nil
}

func (w *Workflow) Verify(creatorID, token string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	state, ok := w.states[creatorID]
	if !ok || !hmac.Equal([]byte(w.tokens[creatorID]), []byte(token)) {
		return State{}, errors.New("invalid verification link")
	}
	state.AssetState = "ready_for_ingest"
	state.JobState = "queued"
	state.DeliveryState = "processing"
	w.states[creatorID] = state
	return state, nil
}

func (w *Workflow) sign(creatorID, email string) string {
	mac := hmac.New(sha256.New, w.Secret)
	mac.Write([]byte(creatorID + "\x00" + strings.ToLower(email)))
	return hex.EncodeToString(mac.Sum(nil))
}

func validate(req Request) error {
	if strings.TrimSpace(req.CreatorID) == "" || strings.TrimSpace(req.AssetID) == "" || strings.TrimSpace(req.Title) == "" {
		return errors.New("creator_id, asset_id, and title are required")
	}
	parsed, err := mail.ParseAddress(req.Email)
	if err != nil || parsed.Address != req.Email {
		return fmt.Errorf("valid email is required")
	}
	return nil
}
