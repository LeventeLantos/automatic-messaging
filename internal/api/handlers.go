package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/LeventeLantos/automatic-messaging/internal/repo"
	"github.com/LeventeLantos/automatic-messaging/internal/scheduler"
)

type Handler struct {
	sched *scheduler.Scheduler
	repo  repo.MessageRepository
}

func NewHandler(s *scheduler.Scheduler, r repo.MessageRepository) *Handler {
	return &Handler{sched: s, repo: r}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) SchedulerStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"running": h.sched.IsRunning()})
}

func (h *Handler) SchedulerStart(w http.ResponseWriter, r *http.Request) {
	h.sched.Start()
	writeJSON(w, http.StatusOK, map[string]any{"running": h.sched.IsRunning()})
}

func (h *Handler) SchedulerStop(w http.ResponseWriter, r *http.Request) {
	h.sched.Stop()
	writeJSON(w, http.StatusOK, map[string]any{"running": h.sched.IsRunning()})
}

func (h *Handler) ListSentMessages(w http.ResponseWriter, r *http.Request) {
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	items, err := h.repo.ListSent(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func parseInt(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
