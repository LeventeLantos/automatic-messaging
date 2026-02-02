package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LeventeLantos/automatic-messaging/internal/api"
	"github.com/LeventeLantos/automatic-messaging/internal/client"
	"github.com/LeventeLantos/automatic-messaging/internal/config"
	"github.com/LeventeLantos/automatic-messaging/internal/repo"
	"github.com/LeventeLantos/automatic-messaging/internal/scheduler"
	"github.com/LeventeLantos/automatic-messaging/internal/service"

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

	msgRepo := repo.NewPostgresMessageRepo(db)

	webhookClient := client.NewWebhookClient(cfg.Webhook.URL)

	sender := service.NewSender(webhookClient, cfg.Webhook.ContentMax).
		WithHooks(
			func(ctx context.Context, internalID int64, remoteMessageID string) error {
				if err := msgRepo.MarkSent(ctx, internalID, remoteMessageID); err != nil {
					slog.Error("failed to mark sent", "id", internalID, "err", err)
					return err
				}
				slog.Info("message sent", "id", internalID, "remote_message_id", remoteMessageID)
				return nil
			},
			func(ctx context.Context, internalID int64, reason string) error {
				if err := msgRepo.MarkFailed(ctx, internalID, reason); err != nil {
					slog.Error("failed to mark failed", "id", internalID, "err", err)
					return err
				}
				slog.Warn("message failed", "id", internalID, "reason", reason)
				return nil
			},
		)

	sched, err := scheduler.New(cfg.Scheduler.Interval, func(ctx context.Context) {
		msgs, err := msgRepo.ClaimPending(ctx, cfg.Scheduler.BatchSize)
		if err != nil {
			slog.Error("claim pending failed", "err", err)
			return
		}
		if len(msgs) == 0 {
			slog.Info("no pending messages")
			return
		}

		slog.Info("claimed messages", "count", len(msgs))
		sent, failed := sender.ProcessBatch(ctx, msgs)
		slog.Info("batch processed", "sent", sent, "failed", failed)
	})
	if err != nil {
		slog.Error("failed to create scheduler", "err", err)
		panic(err)
	}

	sched.Start()

	h := api.NewHandler(sched, msgRepo)
	router := api.Router(h)

	srv := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           loggingMiddleware(router),
		ReadHeaderTimeout: 5 * time.Second,
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("http server starting", "addr", cfg.Server.Address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
			stop()
		}
	}()

	<-rootCtx.Done()
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
