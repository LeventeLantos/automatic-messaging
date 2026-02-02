package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LeventeLantos/automatic-messaging/internal/model"
	"github.com/LeventeLantos/automatic-messaging/internal/repo"
	"github.com/LeventeLantos/automatic-messaging/internal/scheduler"
)

type fakeRepo struct {
	// capture args
	gotLimit  int
	gotOffset int

	// behavior
	items []model.Message
	err   error
}

var _ repo.MessageRepository = (*fakeRepo)(nil)

func (f *fakeRepo) ClaimPending(ctx context.Context, limit int) ([]model.Message, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeRepo) MarkSent(ctx context.Context, id int64, remoteMessageID string) error {
	return errors.New("not implemented")
}

func (f *fakeRepo) MarkFailed(ctx context.Context, id int64, reason string) error {
	return errors.New("not implemented")
}

func (f *fakeRepo) ListSent(ctx context.Context, limit, offset int) ([]model.Message, error) {
	f.gotLimit = limit
	f.gotOffset = offset
	return f.items, f.err
}

func newTestServer(t *testing.T, r repo.MessageRepository) (*scheduler.Scheduler, http.Handler) {
	t.Helper()

	// Long interval so only the immediate tick happens (noop anyway).
	s, err := scheduler.New(time.Hour, func(context.Context) {})
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	h := NewHandler(s, r)
	return s, Router(h)
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("failed to decode json: %v body=%q", err, rr.Body.String())
	}
	return m
}

func TestHealth(t *testing.T) {
	s, mux := newTestServer(t, &fakeRepo{})
	defer s.Stop()

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	body := decodeJSON(t, rr)
	if v, ok := body["ok"].(bool); !ok || !v {
		t.Fatalf("expected {ok:true}, got %v", body)
	}
}

func TestSchedulerEndpoints(t *testing.T) {
	s, mux := newTestServer(t, &fakeRepo{})
	defer s.Stop()

	// Initially should be false.
	{
		req := httptest.NewRequest(http.MethodGet, "/v1/scheduler/status", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
		}
		body := decodeJSON(t, rr)
		if running, ok := body["running"].(bool); !ok || running {
			t.Fatalf("expected running=false, got %v", body)
		}
	}

	// Start
	{
		req := httptest.NewRequest(http.MethodPost, "/v1/scheduler/start", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
		}
		body := decodeJSON(t, rr)
		if running, ok := body["running"].(bool); !ok || !running {
			t.Fatalf("expected running=true after start, got %v", body)
		}
	}

	// Stop
	{
		req := httptest.NewRequest(http.MethodPost, "/v1/scheduler/stop", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
		}
		body := decodeJSON(t, rr)
		if running, ok := body["running"].(bool); !ok || running {
			t.Fatalf("expected running=false after stop, got %v", body)
		}
	}
}

func TestListSentMessages_DefaultsAndArgs(t *testing.T) {
	fr := &fakeRepo{
		items: []model.Message{
			{ID: 1, RecipientPhone: "+361", Content: "a", Status: model.Sent},
		},
	}

	s, mux := newTestServer(t, fr)
	defer s.Stop()

	// No query params => defaults (limit=50, offset=0)
	req := httptest.NewRequest(http.MethodGet, "/v1/messages/sent", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if fr.gotLimit != 50 || fr.gotOffset != 0 {
		t.Fatalf("expected repo called with limit=50 offset=0, got limit=%d offset=%d", fr.gotLimit, fr.gotOffset)
	}

	body := decodeJSON(t, rr)
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %T %v", body["items"], body)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestListSentMessages_ParsesLimitOffset(t *testing.T) {
	fr := &fakeRepo{}
	s, mux := newTestServer(t, fr)
	defer s.Stop()

	req := httptest.NewRequest(http.MethodGet, "/v1/messages/sent?limit=10&offset=5", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if fr.gotLimit != 10 || fr.gotOffset != 5 {
		t.Fatalf("expected repo called with limit=10 offset=5, got limit=%d offset=%d", fr.gotLimit, fr.gotOffset)
	}
}

func TestListSentMessages_InvalidLimitOffsetFallsBackToDefaults(t *testing.T) {
	fr := &fakeRepo{}
	s, mux := newTestServer(t, fr)
	defer s.Stop()

	req := httptest.NewRequest(http.MethodGet, "/v1/messages/sent?limit=abc&offset=zzz", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if fr.gotLimit != 50 || fr.gotOffset != 0 {
		t.Fatalf("expected defaults limit=50 offset=0, got limit=%d offset=%d", fr.gotLimit, fr.gotOffset)
	}
}

func TestListSentMessages_RepoErrorReturns500(t *testing.T) {
	fr := &fakeRepo{err: errors.New("db down")}
	s, mux := newTestServer(t, fr)
	defer s.Stop()

	req := httptest.NewRequest(http.MethodGet, "/v1/messages/sent", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "db down") {
		t.Fatalf("expected error body to contain repo error, got %q", rr.Body.String())
	}
}

func TestRouterRoot(t *testing.T) {
	s, mux := newTestServer(t, &fakeRepo{})
	defer s.Stop()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "automatic-messaging" {
		t.Fatalf("expected body %q, got %q", "automatic-messaging", got)
	}
}
