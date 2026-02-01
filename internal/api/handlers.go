package api

import (
	"encoding/json"
	"net/http"

	"github.com/LeventeLantos/automatic-messaging/internal/scheduler"
)

type Handler struct {
	scheduler *scheduler.Scheduler
}

func NewHandler(s *scheduler.Scheduler) *Handler {
	return &Handler{scheduler: s}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) SchedulerStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"running": h.scheduler.IsRunning()})
}

func (h *Handler) SchedulerStart(w http.ResponseWriter, r *http.Request) {
	h.scheduler.Start()
	writeJSON(w, http.StatusOK, map[string]any{"running": h.scheduler.IsRunning()})
}

func (h *Handler) SchedulerStop(w http.ResponseWriter, r *http.Request) {
	h.scheduler.Stop()
	writeJSON(w, http.StatusOK, map[string]any{"running": h.scheduler.IsRunning()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
