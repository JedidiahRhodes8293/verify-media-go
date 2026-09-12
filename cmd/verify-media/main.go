package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/verify-media-go/internal/infrai"
	"example.com/verify-media-go/internal/signup"
)

type emailAdapter struct{ client *infrai.Client }

func (a emailAdapter) SendEmail(ctx context.Context, mail signup.Email, key string) (signup.SendResult, error) {
	result, err := a.client.SendEmail(ctx, infrai.Email{To: mail.To, Subject: mail.Subject, HTML: mail.HTML}, key)
	return signup.SendResult{MessageID: result.MessageID}, err
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	secret := os.Getenv("VERIFY_SECRET")
	if apiKey == "" || secret == "" {
		log.Fatal("INFRAI_API_KEY and VERIFY_SECRET are required")
	}
	publicURL := getenv("PUBLIC_URL", "http://localhost:8080")
	workflow := signup.New(emailAdapter{infrai.NewEmailClient(apiKey)}, publicURL, []byte(secret))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var input signup.Request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signup request"})
			return
		}
		state, err := workflow.Start(r.Context(), input)
		if err != nil {
			var apiErr *infrai.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Error()})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, state)
	})
	mux.HandleFunc("GET /verify", func(w http.ResponseWriter, r *http.Request) {
		state, err := workflow.Verify(r.URL.Query().Get("creator_id"), r.URL.Query().Get("token"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, state)
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("verify-media listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func getenv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
