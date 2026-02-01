package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LeventeLantos/automatic-messaging/internal/api"
	"github.com/LeventeLantos/automatic-messaging/internal/config"
	"github.com/LeventeLantos/automatic-messaging/internal/repo"
	"github.com/LeventeLantos/automatic-messaging/internal/scheduler"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.LoadAll()
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	db, err := sql.Open("pgx", cfg.Database.PostgresURL)
	if err != nil {
		slog.Error("failed to open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		slog.Error("failed to ping db", "err", err)
		os.Exit(1)
	}
	slog.Info("db connected")

	_ = repo.NewPostgresMessageRepo(db) // TODO

	// TODO
	sched, err := scheduler.New(cfg.Scheduler.Interval, func(ctx context.Context) {
		slog.Info("placeholder tick: scheduler is running (no-op)")
	})
	if err != nil {
		slog.Error("failed to create scheduler", "err", err)
		panic(err)
	}

	h := api.NewHandler(sched)
	router := api.Router(h)

	srv := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           loggingMiddleware(router),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("http server starting", "addr", cfg.Server.Address)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown requested")

	sched.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	} else {
		slog.Info("shutdown complete")
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &wrapWriter{ResponseWriter: w, status: 200}

		next.ServeHTTP(ww, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type wrapWriter struct {
	http.ResponseWriter
	status int
}

func (w *wrapWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
