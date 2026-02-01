package main

import (
	"log"

	"github.com/LeventeLantos/automatic-messaging/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.LoadAll()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("messaging app starting (addr=%s, interval=%s, batch=%d, redis=%v)",
		cfg.Server.Address,
		cfg.Scheduler.Interval,
		cfg.Scheduler.BatchSize,
		cfg.Redis.Enabled,
	)
}
