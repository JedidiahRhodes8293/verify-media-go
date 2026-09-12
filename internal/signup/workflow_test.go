package signup

import (
	"context"
	"strings"
	"testing"
)

type recordingSender struct {
	calls int
	mail  Email
	key   string
}

func (s *recordingSender) SendEmail(_ context.Context, mail Email, key string) (SendResult, error) {
	s.calls++
	s.mail = mail
	s.key = key
	return SendResult{MessageID: "msg_test_42"}, nil
}

func TestCreatorVerificationTransitions(t *testing.T) {
	tests := []struct {
		name         string
		verify       bool
		wantAsset    string
		wantJob      string
		wantDelivery string
	}{
		{name: "signup holds ingestion", wantAsset: "held_for_verification", wantJob: "waiting_for_creator", wantDelivery: "email_verification_sent"},
		{name: "verified creator queues processing", verify: true, wantAsset: "ready_for_ingest", wantJob: "queued", wantDelivery: "processing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &recordingSender{}
			workflow := New(sender, "https://stream.example", []byte("test-secret"))
			input := Request{CreatorID: "creator_42", Email: "ada@example.com", AssetID: "asset_9", Title: "Night Shift"}
			state, err := workflow.Start(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if tt.verify {
				token := strings.Split(strings.Split(sender.mail.HTML, "token=")[1], "\"")[0]
				state, err = workflow.Verify(input.CreatorID, token)
				if err != nil {
					t.Fatal(err)
				}
			}
			if state.AssetState != tt.wantAsset || state.JobState != tt.wantJob || state.DeliveryState != tt.wantDelivery {
				t.Fatalf("state = %#v", state)
			}
			if sender.calls != 1 || sender.key != "creator-signup-creator_42" || state.MessageID != "msg_test_42" {
				t.Fatalf("delivery calls=%d key=%q state=%#v", sender.calls, sender.key, state)
			}
		})
	}
}

func TestRepeatedSignupSendsOnce(t *testing.T) {
	sender := &recordingSender{}
	workflow := New(sender, "https://stream.example", []byte("test-secret"))
	input := Request{CreatorID: "creator_7", Email: "lin@example.com", AssetID: "asset_3", Title: "Pilot"}
	for i := 0; i < 2; i++ {
		if _, err := workflow.Start(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	if sender.calls != 1 {
		t.Fatalf("send calls = %d, want 1", sender.calls)
	}
}
