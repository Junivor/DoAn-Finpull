package main

import (
	"flag"
	"log"
	"os"

	"FinPull/internal/di"
	"FinPull/pkg/config"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config/config.yaml", "config file path")
	flag.Parse()

	// Load config
	cfg, err := config.LoadWithEnv(*configPath)
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	log.Printf("env=%s backend=%s", cfg.Environment, cfg.Backend.Type)

	// Wire DI: Initialize all dependencies
	app, err := di.InitializeApp(cfg)
	if err != nil {
		log.Fatalf("app initialization failed: %v", err)
	}

	log.Printf("clickhouse: connected and schema ready - db: %s\n", cfg.ClickHouse.Database)
	log.Printf("kafka: connected brokers=%v topic=%s", cfg.Kafka.Brokers, cfg.Kafka.Topic)

	// Run application (blocks until signal)
	if err := app.Run(); err != nil {
		log.Printf("app error: %v", err)
		os.Exit(1)
	}
}
