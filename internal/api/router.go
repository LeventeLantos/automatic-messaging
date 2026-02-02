package api

import "net/http"

func Router(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/health", h.Health)

	mux.HandleFunc("GET /v1/scheduler/status", h.SchedulerStatus)
	mux.HandleFunc("POST /v1/scheduler/start", h.SchedulerStart)
	mux.HandleFunc("POST /v1/scheduler/stop", h.SchedulerStop)

	mux.HandleFunc("GET /v1/messages/sent", h.ListSentMessages)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("automatic-messaging"))
	})

	return mux
}
