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
	"github.com/LeventeLantos/automatic-messaging/internal/cache"
	"github.com/LeventeLantos/automatic-messaging/internal/client"
	"github.com/LeventeLantos/automatic-messaging/internal/config"
	"github.com/LeventeLantos/automatic-messaging/internal/repo"
	"github.com/LeventeLantos/automatic-messaging/internal/scheduler"
	"github.com/LeventeLantos/automatic-messaging/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	_ = godotenv.Load()

	cfg := mustLoadConfig()
	setupLogger()

	db := mustConnectDB(cfg)
	defer db.Close()

	msgRepo := repo.NewPostgresMessageRepo(db)
	msgCache := setupRedis(cfg)

	sender := buildSender(cfg, msgRepo, msgCache)
	sched := buildScheduler(cfg, msgRepo, sender)
	sched.Start()

	srv := buildHTTPServer(cfg, sched, msgRepo)
	runWithGracefulShutdown(srv, sched)
}

func mustLoadConfig() *config.Config {
	cfg, err := config.LoadAll()
	if err != nil {
		panic(err)
	}
	return cfg
}

func setupLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func mustConnectDB(cfg *config.Config) *sql.DB {
	db, err := sql.Open("pgx", cfg.Database.PostgresURL)
	if err != nil {
		slog.Error("failed to open db", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		slog.Error("failed to ping db", "err", err)
		os.Exit(1)
	}

	slog.Info("db connected")
	return db
}

func setupRedis(cfg *config.Config) cache.MessageCache {
	if !cfg.Redis.Enabled {
		return nil
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("redis ping failed (disabling cache)", "err", err)
		return nil
	}

	slog.Info("redis cache enabled", "addr", cfg.Redis.Address, "db", cfg.Redis.DB)
	return cache.NewRedisCache(rdb, cfg.Redis.TTL)
}

func buildSender(
	cfg *config.Config,
	msgRepo repo.MessageRepository,
	msgCache cache.MessageCache,
) *service.Sender {
	webhookClient := client.NewWebhookClient(cfg.Webhook.URL)

	return service.NewSender(webhookClient, cfg.Webhook.ContentMax).
		WithHooks(
			func(ctx context.Context, internalID int64, remoteMessageID string) error {
				if err := msgRepo.MarkSent(ctx, internalID, remoteMessageID); err != nil {
					slog.Error("failed to mark sent", "id", internalID, "err", err)
					return err
				}

				slog.Info("message sent", "id", internalID, "remote_message_id", remoteMessageID)

				if msgCache != nil {
					if err := msgCache.StoreSent(ctx, internalID, remoteMessageID, time.Now().UTC()); err != nil {
						slog.Warn("failed to store redis cache", "id", internalID, "err", err)
					}
				}

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
}

func buildScheduler(
	cfg *config.Config,
	msgRepo repo.MessageRepository,
	sender *service.Sender,
) *scheduler.Scheduler {
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
	return sched
}

func buildHTTPServer(
	cfg *config.Config,
	sched *scheduler.Scheduler,
	msgRepo repo.MessageRepository,
) *http.Server {
	h := api.NewHandler(sched, msgRepo)
	router := api.Router(h)

	return &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           loggingMiddleware(router),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func runWithGracefulShutdown(srv *http.Server, sched *scheduler.Scheduler) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("http server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
