// Author: Sunil Jain, 2026-11-12 — Fail-closed Aerolake diagnostics; block unsafe telemetry pruning.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

type Config struct {
	DatabaseURL      string
	AerolakeEndpoint string
	BatchSize        int
	RetentionHours   int
	PollIntervalSec  int
	KillSwitch       bool
}

func loadConfig() Config {
	dbURL := os.Getenv("AGIPAREL_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@127.0.0.1:5432/agiparel?sslmode=disable"
	}

	endpoint := os.Getenv("AEROLAKE_FLIGHT_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("AEROLAKE_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = "127.0.0.1:51051"
	}

	batchSize, _ := strconv.Atoi(os.Getenv("AEROLAKE_PRUNING_BATCH_SIZE"))
	if batchSize <= 0 {
		batchSize = 5000
	}

	retentionHours, _ := strconv.Atoi(os.Getenv("AEROLAKE_RETENTION_HOURS"))
	if retentionHours <= 0 {
		retentionHours = 72
	}

	pollInterval, _ := strconv.Atoi(os.Getenv("AEROLAKE_POLL_INTERVAL_SEC"))
	if pollInterval <= 0 {
		pollInterval = 30
	}

	// Default must be safe even when an installer omits a variable.
	killSwitch := os.Getenv("AEROLAKE_PRUNING_KILL_SWITCH") != "false"

	return Config{
		DatabaseURL:      dbURL,
		AerolakeEndpoint: endpoint,
		BatchSize:        batchSize,
		RetentionHours:   retentionHours,
		PollIntervalSec:  pollInterval,
		KillSwitch:       killSwitch,
	}
}

func main() {
	cfg := loadConfig()
	log.Printf("[aerolake-bridge] Initializing sidecar bridge service")
	log.Printf("[aerolake-bridge] Flight endpoint: %s, Batch size: %d, Retention: %dh, Kill switch: %t",
		cfg.AerolakeEndpoint, cfg.BatchSize, cfg.RetentionHours, cfg.KillSwitch)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[aerolake-bridge] Fatal: Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	producer := NewFlightProducer(cfg.AerolakeEndpoint)
	defer producer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := time.NewTicker(time.Duration(cfg.PollIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[aerolake-bridge] Terminating bridge service loop")
			return
		case <-ticker.C:
			runCycle(db, producer, cfg)
		}
	}
}

// runCycle is deliberately non-destructive in v50. A TCP listener is not a
// durable Parquet watermark. Even explicit disabling of the kill switch cannot
// authorize deletion until an authenticated, table-bound checkpoint verifier,
// persisted replay evidence and rollback tests are implemented.
func runCycle(db *sql.DB, producer *FlightProducer, cfg Config) {
	if cfg.KillSwitch {
		log.Println("[aerolake-bridge] Retention is OFF (default safe setting).")
		return
	}
	log.Println("[aerolake-bridge] RETENTION BLOCKED: no trusted durable archive checkpoint is connected; no PostgreSQL records deleted")
}
