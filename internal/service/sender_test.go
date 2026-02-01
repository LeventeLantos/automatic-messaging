package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/LeventeLantos/automatic-messaging/internal/client"
	"github.com/LeventeLantos/automatic-messaging/internal/model"
	"github.com/LeventeLantos/automatic-messaging/internal/service"
)

func TestSender_MarksSentOn202(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message":   "Accepted",
			"messageId": "67f2f8a8-ea58-4ed0-a6f9-ff217df4d849",
		})
	}))
	t.Cleanup(srv.Close)

	c := client.NewWebhookClient(srv.URL)
	sender := service.NewSender(c, 160)

	var (
		mu        sync.Mutex
		sentIDs   []int64
		remoteIDs []string
	)

	sender.WithHooks(
		func(ctx context.Context, internalID int64, remoteMessageID string) error {
			mu.Lock()
			defer mu.Unlock()
			sentIDs = append(sentIDs, internalID)
			remoteIDs = append(remoteIDs, remoteMessageID)
			return nil
		},
		func(ctx context.Context, internalID int64, reason string) error {
			t.Fatalf("did not expect failure hook, got id=%d reason=%s", internalID, reason)
			return nil
		},
	)

	sent, failed := sender.ProcessBatch(context.Background(), []model.Message{
		{ID: 1, RecipientPhone: "+361234567", Content: "hello"},
	})

	if failed != 0 {
		t.Fatalf("expected failed=0, got %d", failed)
	}
	if sent != 1 {
		t.Fatalf("expected sent=1, got %d", sent)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(sentIDs) != 1 || sentIDs[0] != 1 {
		t.Fatalf("expected sent hook for id=1, got %+v", sentIDs)
	}
	if len(remoteIDs) != 1 || remoteIDs[0] == "" {
		t.Fatalf("expected remote messageId, got %+v", remoteIDs)
	}
}

func TestSender_FailsWhenContentTooLong(t *testing.T) {
	t.Parallel()

	noopClient := &fakeClient{}
	sender := service.NewSender(noopClient, 3)

	var (
		mu      sync.Mutex
		failed  []int64
		reasons []string
	)

	sender.WithHooks(
		func(ctx context.Context, internalID int64, remoteMessageID string) error {
			t.Fatalf("did not expect sent hook")
			return nil
		},
		func(ctx context.Context, internalID int64, reason string) error {
			mu.Lock()
			defer mu.Unlock()
			failed = append(failed, internalID)
			reasons = append(reasons, reason)
			return nil
		},
	)

	sent, failCount := sender.ProcessBatch(context.Background(), []model.Message{
		{ID: 10, RecipientPhone: "+361234567", Content: "abcd"},
	})

	if sent != 0 {
		t.Fatalf("expected sent=0, got %d", sent)
	}
	if failCount != 1 {
		t.Fatalf("expected failed=1, got %d", failCount)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(failed) != 1 || failed[0] != 10 {
		t.Fatalf("expected failed id=10, got %+v", failed)
	}
	if len(reasons) != 1 || reasons[0] == "" {
		t.Fatalf("expected a reason, got %+v", reasons)
	}
}

type fakeClient struct{}

func (f *fakeClient) Send(ctx context.Context, phoneNumber, message string) (string, error) {
	return "ignored", nil
}
