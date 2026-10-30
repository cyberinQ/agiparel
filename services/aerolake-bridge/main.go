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

	killSwitch := os.Getenv("AEROLAKE_PRUNING_KILL_SWITCH") == "true" || os.Getenv("AEROLAKE_PRUNING_KILL_SWITCH") == "1"

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

func runCycle(db *sql.DB, producer *FlightProducer, cfg Config) {
	if cfg.KillSwitch {
		log.Println("[aerolake-bridge] Pruning cycle skipped: kill-switch is ACTIVE")
		return
	}

	// 1. Probe socket listener health
	if !producer.CheckSocketLive(2 * time.Second) {
		log.Printf("[aerolake-bridge] Skipping cycle: Aerolake Flight gRPC port (%s) not responsive", cfg.AerolakeEndpoint)
		return
	}

	// 2. Compute safety horizon
	cutoff := time.Now().UTC().Add(-time.Duration(cfg.RetentionHours) * time.Hour)

	// 3. Query PostgreSQL procedure for watermark verification
	var rowsDeleted int
	err := db.QueryRow(
		"SELECT prune_v45_telemetry_batch($1, $2)",
		cutoff, cfg.BatchSize,
	).Scan(&rowsDeleted)

	if err != nil {
		log.Printf("[aerolake-bridge] Pruning execution halted: %v", err)
		return
	}

	if rowsDeleted > 0 {
		log.Printf("[aerolake-bridge] Pruned %d telemetry records older than %s", rowsDeleted, cutoff.Format(time.RFC3339))
	} else {
		log.Println("[aerolake-bridge] Prune check complete: 0 rows past retention horizon")
	}
}
