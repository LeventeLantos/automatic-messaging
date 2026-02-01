package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Scheduler SchedulerConfig
	Webhook   WebhookConfig
}

type ServerConfig struct {
	Address string
}

type DatabaseConfig struct {
	PostgresURL string
}

type RedisConfig struct {
	Enabled  bool
	Address  string
	Password string
	DB       int
	TTL      time.Duration
}

type SchedulerConfig struct {
	Interval  time.Duration
	BatchSize int
}

type WebhookConfig struct {
	URL        string
	ContentMax int
}

func LoadAll() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Address: getEnv("SERVER_ADDRESS", ":8080"),
		},
		Database: DatabaseConfig{
			PostgresURL: mustEnv("POSTGRES_URL"),
		},
		Webhook: WebhookConfig{
			URL:        mustEnv("WEBHOOK_URL"),
			ContentMax: getEnvInt("CONTENT_MAX", 160),
		},
		Scheduler: SchedulerConfig{
			Interval:  time.Duration(getEnvInt("SCHED_INTERVAL_SECONDS", 120)) * time.Second,
			BatchSize: getEnvInt("SCHED_BATCH_SIZE", 2),
		},
		Redis: loadRedisConfig(),
	}

	validate(cfg)
	return cfg, nil
}

func loadRedisConfig() RedisConfig {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return RedisConfig{Enabled: false}
	}

	return RedisConfig{
		Enabled:  true,
		Address:  addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       getEnvInt("REDIS_DB", 0),
		TTL:      time.Duration(getEnvInt("REDIS_TTL_SECONDS", 86400)) * time.Second,
	}
}

func validate(cfg *Config) {
	if cfg.Scheduler.BatchSize <= 0 {
		panic("SCHED_BATCH_SIZE must be > 0")
	}
	if cfg.Scheduler.Interval <= 0 {
		panic("SCHED_INTERVAL_SECONDS must be > 0")
	}
	if cfg.Webhook.ContentMax <= 0 {
		panic("CONTENT_MAX must be > 0")
	}
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("missing required env var: %s", key))
	}
	return val
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("invalid int for env %s: %s", key, v))
	}
	return i
}
