package config

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var envMu sync.Mutex

func TestLoadAll_HappyPath_NoRedis(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	t.Setenv("POSTGRES_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("WEBHOOK_URL", "https://example.com/webhook")

	cfg, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error: %v", err)
	}

	if cfg.Database.PostgresURL != "postgres://u:p@localhost:5432/db?sslmode=disable" {
		t.Fatalf("unexpected PostgresURL: %q", cfg.Database.PostgresURL)
	}
	if cfg.Webhook.URL != "https://example.com/webhook" {
		t.Fatalf("unexpected Webhook.URL: %q", cfg.Webhook.URL)
	}
	if cfg.Server.Address != ":8080" {
		t.Fatalf("unexpected Server.Address default: %q", cfg.Server.Address)
	}
	if cfg.Webhook.ContentMax != 160 {
		t.Fatalf("unexpected ContentMax default: %d", cfg.Webhook.ContentMax)
	}
	if cfg.Scheduler.Interval != 120*time.Second {
		t.Fatalf("unexpected Scheduler.Interval default: %v", cfg.Scheduler.Interval)
	}
	if cfg.Scheduler.BatchSize != 2 {
		t.Fatalf("unexpected Scheduler.BatchSize default: %d", cfg.Scheduler.BatchSize)
	}

	if cfg.Redis.Enabled {
		t.Fatalf("expected Redis disabled when REDIS_ADDR not set")
	}
}

func TestLoadAll_HappyPath_WithRedis(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	t.Setenv("POSTGRES_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("WEBHOOK_URL", "https://example.com/webhook")

	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_TTL_SECONDS", "42")

	cfg, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error: %v", err)
	}

	if !cfg.Redis.Enabled {
		t.Fatalf("expected Redis enabled")
	}
	if cfg.Redis.Address != "localhost:6379" {
		t.Fatalf("unexpected Redis.Address: %q", cfg.Redis.Address)
	}
	if cfg.Redis.Password != "secret" {
		t.Fatalf("unexpected Redis.Password: %q", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 3 {
		t.Fatalf("unexpected Redis.DB: %d", cfg.Redis.DB)
	}
	if cfg.Redis.TTL != 42*time.Second {
		t.Fatalf("unexpected Redis.TTL: %v", cfg.Redis.TTL)
	}
}

func TestLoadAll_RequiredEnvMissing(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	t.Run("missing POSTGRES_URL", func(t *testing.T) {
		t.Setenv("WEBHOOK_URL", "https://example.com/webhook")

		_, err := LoadAll()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "POSTGRES_URL") {
			t.Fatalf("expected error mentioning POSTGRES_URL, got: %v", err)
		}
	})

	t.Run("missing WEBHOOK_URL", func(t *testing.T) {
		clearTestEnv(t)

		t.Setenv("POSTGRES_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")

		_, err := LoadAll()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "WEBHOOK_URL") {
			t.Fatalf("expected error mentioning WEBHOOK_URL, got: %v", err)
		}
	})
}

func TestLoadAll_InvalidInts(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	t.Setenv("POSTGRES_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("WEBHOOK_URL", "https://example.com/webhook")

	cases := []struct {
		name string
		key  string
		val  string
	}{
		{"invalid CONTENT_MAX", "CONTENT_MAX", "abc"},
		{"invalid SCHED_INTERVAL_SECONDS", "SCHED_INTERVAL_SECONDS", "nope"},
		{"invalid SCHED_BATCH_SIZE", "SCHED_BATCH_SIZE", "x"},
		{"invalid REDIS_DB", "REDIS_DB", "bad"},
		{"invalid REDIS_TTL_SECONDS", "REDIS_TTL_SECONDS", "bad"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearTestEnv(t)

			t.Setenv("POSTGRES_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
			t.Setenv("WEBHOOK_URL", "https://example.com/webhook")

			// Enable redis only for redis-related invalid ints.
			if strings.HasPrefix(tc.key, "REDIS_") {
				t.Setenv("REDIS_ADDR", "localhost:6379")
			}

			t.Setenv(tc.key, tc.val)

			_, err := LoadAll()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected error mentioning %s, got: %v", tc.key, err)
			}
		})
	}
}

func TestLoadAll_ValidationFailures(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	t.Setenv("POSTGRES_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("WEBHOOK_URL", "https://example.com/webhook")

	cases := []struct {
		name string
		set  func()
		want string
	}{
		{
			name: "batch size <= 0",
			set: func() {
				t.Setenv("SCHED_BATCH_SIZE", "0")
			},
			want: "SCHED_BATCH_SIZE",
		},
		{
			name: "interval <= 0",
			set: func() {
				t.Setenv("SCHED_INTERVAL_SECONDS", "0")
			},
			want: "SCHED_INTERVAL_SECONDS",
		},
		{
			name: "content max <= 0",
			set: func() {
				t.Setenv("CONTENT_MAX", "0")
			},
			want: "CONTENT_MAX",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			clearTestEnv(t)

			t.Setenv("POSTGRES_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
			t.Setenv("WEBHOOK_URL", "https://example.com/webhook")
			tc.set()

			_, err := LoadAll()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error mentioning %s, got: %v", tc.want, err)
			}
		})
	}
}

func TestRequireEnv(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	_, err := requireEnv("MISSING_KEY")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	t.Setenv("FOO", "bar")
	v, err := requireEnv("FOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "bar" {
		t.Fatalf("expected %q, got %q", "bar", v)
	}
}

func TestGetEnv(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	if got := getEnv("NOPE", "default"); got != "default" {
		t.Fatalf("expected default, got %q", got)
	}

	t.Setenv("A", "x")
	if got := getEnv("A", "default"); got != "x" {
		t.Fatalf("expected x, got %q", got)
	}
}

func TestGetEnvInt(t *testing.T) {
	envMu.Lock()
	defer envMu.Unlock()

	clearTestEnv(t)

	got, err := getEnvInt("MISSING", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 7 {
		t.Fatalf("expected default 7, got %d", got)
	}

	t.Setenv("N", "123")
	got, err = getEnvInt("N", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 123 {
		t.Fatalf("expected 123, got %d", got)
	}

	t.Setenv("BAD", "abc")
	_, err = getEnvInt("BAD", 7)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "BAD") {
		t.Fatalf("expected error mentioning BAD, got: %v", err)
	}
}

func TestJoinErrors(t *testing.T) {
	if err := joinErrors(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	e1 := errors.New("one")
	e2 := errors.New("two")
	err := joinErrors([]error{e1, e2})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, e1) {
		t.Fatalf("expected errors.Is(err, e1) to be true")
	}
	if !errors.Is(err, e2) {
		t.Fatalf("expected errors.Is(err, e2) to be true")
	}
}

func clearTestEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"POSTGRES_URL",
		"WEBHOOK_URL",
		"CONTENT_MAX",
		"SCHED_INTERVAL_SECONDS",
		"SCHED_BATCH_SIZE",
		"SERVER_ADDRESS",
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_TTL_SECONDS",
		"FOO",
		"A",
		"N",
		"BAD",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}
