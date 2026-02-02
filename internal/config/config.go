package config

import (
	"errors"
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
	pgURL, err := requireEnv("POSTGRES_URL")
	if err != nil {
		return nil, err
	}
	webhookURL, err := requireEnv("WEBHOOK_URL")
	if err != nil {
		return nil, err
	}

	contentMax, err := getEnvInt("CONTENT_MAX", 160)
	if err != nil {
		return nil, err
	}

	intervalSeconds, err := getEnvInt("SCHED_INTERVAL_SECONDS", 120)
	if err != nil {
		return nil, err
	}

	batchSize, err := getEnvInt("SCHED_BATCH_SIZE", 2)
	if err != nil {
		return nil, err
	}

	redisCfg, err := loadRedisConfig()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Address: getEnv("SERVER_ADDRESS", ":8080"),
		},
		Database: DatabaseConfig{
			PostgresURL: pgURL,
		},
		Webhook: WebhookConfig{
			URL:        webhookURL,
			ContentMax: contentMax,
		},
		Scheduler: SchedulerConfig{
			Interval:  time.Duration(intervalSeconds) * time.Second,
			BatchSize: batchSize,
		},
		Redis: redisCfg,
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadRedisConfig() (RedisConfig, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return RedisConfig{Enabled: false}, nil
	}

	db, err := getEnvInt("REDIS_DB", 0)
	if err != nil {
		return RedisConfig{}, err
	}

	ttlSeconds, err := getEnvInt("REDIS_TTL_SECONDS", 86400)
	if err != nil {
		return RedisConfig{}, err
	}

	return RedisConfig{
		Enabled:  true,
		Address:  addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
		TTL:      time.Duration(ttlSeconds) * time.Second,
	}, nil
}

func validate(cfg *Config) error {
	var errs []error

	if cfg.Scheduler.BatchSize <= 0 {
		errs = append(errs, errors.New("SCHED_BATCH_SIZE must be > 0"))
	}
	if cfg.Scheduler.Interval <= 0 {
		errs = append(errs, errors.New("SCHED_INTERVAL_SECONDS must be > 0"))
	}
	if cfg.Webhook.ContentMax <= 0 {
		errs = append(errs, errors.New("CONTENT_MAX must be > 0"))
	}

	return joinErrors(errs)
}

func requireEnv(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("missing required env var: %s", key)
	}
	return val, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid int for env %s: %q", key, v)
	}
	return i, nil
}

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
