package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookClient_Send_Success(t *testing.T) {
	t.Parallel()

	type gotReq struct {
		Method      string
		ContentType string
		Body        []byte
	}

	var captured gotReq

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.ContentType = r.Header.Get("Content-Type")

		b, _ := ioReadAll(r)
		captured.Body = b

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message":"Accepted","messageId":"abc-123"}`))
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msgID, err := c.Send(ctx, "+361234567", "hello")
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if msgID != "abc-123" {
		t.Fatalf("expected messageId %q, got %q", "abc-123", msgID)
	}

	if captured.Method != http.MethodPost {
		t.Fatalf("expected method POST, got %q", captured.Method)
	}
	if captured.ContentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", captured.ContentType)
	}

	var req sendRequest
	if err := json.Unmarshal(captured.Body, &req); err != nil {
		t.Fatalf("failed to decode request json: %v body=%q", err, string(captured.Body))
	}
	if req.PhoneNumber != "+361234567" {
		t.Fatalf("expected phoneNumber %q, got %q", "+361234567", req.PhoneNumber)
	}
	if req.Message != "hello" {
		t.Fatalf("expected message %q, got %q", "hello", req.Message)
	}
}

func TestWebhookClient_Send_Non202_ReturnsErrorWithBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not accepted"))
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL)

	_, err := c.Send(context.Background(), "+361", "hi")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "unexpected status code: 200") {
		t.Fatalf("expected error to mention status code, got: %v", err)
	}
	if !strings.Contains(msg, `body="not accepted"`) {
		t.Fatalf("expected error to include body, got: %v", err)
	}
}

func TestWebhookClient_Send_InvalidJSON_ReturnsErrorWithBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("THIS IS NOT JSON"))
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL)

	_, err := c.Send(context.Background(), "+361", "hi")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "failed to decode json") {
		t.Fatalf("expected decode error, got: %v", err)
	}
	if !strings.Contains(msg, `body="THIS IS NOT JSON"`) {
		t.Fatalf("expected error to include body, got: %v", err)
	}
}

func TestWebhookClient_Send_MissingMessageId_ReturnsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message":"Accepted"}`))
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL)

	_, err := c.Send(context.Background(), "+361", "hi")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing messageId") {
		t.Fatalf("expected missing messageId error, got: %v", err)
	}
}

func TestWebhookClient_Send_ContextCanceled(t *testing.T) {
	t.Parallel()

	// Server that intentionally blocks longer than our context deadline.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message":"Accepted","messageId":"abc"}`))
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := c.Send(ctx, "+361", "hi")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// On cancellation, net/http returns context deadline exceeded.
	if !strings.Contains(strings.ToLower(err.Error()), "context") &&
		!strings.Contains(strings.ToLower(err.Error()), "deadline") {
		t.Fatalf("expected context/deadline error, got: %v", err)
	}
}

func ioReadAll(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
