package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DatabaseURL          string
	AerolakeEndpoint     string
	PruneIntervalSec     int
	TelemetryRetention   string
	VerifiedPruneEnabled bool
}

func loadConfig() Config {
	pruneInterval := 3600
	if val := os.Getenv("PRUNE_INTERVAL_SEC"); val != "" {
		if ival, err := strconv.Atoi(val); err == nil {
			pruneInterval = ival
		}
	}

	retention := "14 days"
	if val := os.Getenv("TELEMETRY_RETENTION_INTERVAL"); val != "" {
		retention = val
	}

	verifiedPrune := false
	if val := os.Getenv("AEROLAKE_VERIFIED_PRUNE_ENABLED"); val == "true" || val == "1" {
		verifiedPrune = true
	}

	return Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		AerolakeEndpoint:     os.Getenv("AEROLAKE_ENDPOINT"),
		PruneIntervalSec:     pruneInterval,
		TelemetryRetention:   retention,
		VerifiedPruneEnabled: verifiedPrune,
	}
}

func main() {
	cfg := loadConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable must be set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	log.Printf("[aerolake-bridge] Service initialized. Aerolake Endpoint: %s, Prune Enabled: %t",
		cfg.AerolakeEndpoint, cfg.VerifiedPruneEnabled)

	ticker := time.NewTicker(time.Duration(cfg.PruneIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[aerolake-bridge] Shutdown signal received. Exiting...")
			return
		case <-ticker.C:
			runDarkDataPruning(ctx, pool, cfg)
		}
	}
}

func runDarkDataPruning(ctx context.Context, pool *pgxpool.Pool, cfg Config) {
	if !cfg.VerifiedPruneEnabled {
		log.Printf("[aerolake-bridge] Dark-data pruning bypassed: AEROLAKE_VERIFIED_PRUNE_ENABLED=false (safety kill-switch active)")
		return
	}

	watermarkStr := os.Getenv("AEROLAKE_DURABLE_WATERMARK_OVERRIDE")
	if watermarkStr == "" {
		log.Printf("[aerolake-bridge] Pruning halted: no verified Aerolake Parquet checkpoint watermark available")
		return
	}

	watermarkTime, err := time.Parse(time.RFC3339, watermarkStr)
	if err != nil {
		log.Printf("[aerolake-bridge] Pruning halted: invalid watermark format: %v", err)
		return
	}

	var deletedCount int64
	query := `SELECT prune_ambient_rf_telemetry_v45($1::interval, $2::timestamptz)`
	err = pool.QueryRow(ctx, query, cfg.TelemetryRetention, watermarkTime).Scan(&deletedCount)
	if err != nil {
		log.Printf("[aerolake-bridge] Error during verified dark-data pruning: %v", err)
		return
	}

	log.Printf("[aerolake-bridge] Verified dark-data pruning completed: %d rows deleted (bounded by watermark: %s)",
		deletedCount, watermarkTime.Format(time.RFC3339))
}
