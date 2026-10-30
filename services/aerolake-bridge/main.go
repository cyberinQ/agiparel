// Author: Sunil Jain, CyberinQ LLC, 2026-10-30.
// aerolake-bridge: Asynchronous ingestion adapter routing Agiparel CDC events into L3 Aerolake5 agentd
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	DatabaseURL        string
	AerolakeFlightAddr string
	PruneIntervalSec   int
	RetentionDays      int
}

func main() {
	log.Println("[Aerolake Bridge] Starting L3 Edge Data Mesh Adapter v45.0...")

	cfg := Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://agiparel_admin:dev_secret_vault_pg16@localhost:15434/agiparel_dev?sslmode=disable"),
		AerolakeFlightAddr: getEnv("AEROLAKE_FLIGHT_ADDR", "localhost:51051"),
		PruneIntervalSec:   3600,
		RetentionDays:      14,
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("ERR_DB_CONNECT: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[Aerolake Bridge] Shutdown signal received.")
		cancel()
	}()

	ticker := time.NewTicker(time.Duration(cfg.PruneIntervalSec) * time.Second)
	defer ticker.Stop()

	log.Printf("[Aerolake Bridge] Connected. Arrow Flight Target: %s. Prune Interval: %ds", cfg.AerolakeFlightAddr, cfg.PruneIntervalSec)

	for {
		select {
		case <-ctx.Done():
			log.Println("[Aerolake Bridge] Exiting cleanly.")
			return
		case <-ticker.C:
			runDarkDataPruning(ctx, db, cfg.RetentionDays)
		}
	}
}

func runDarkDataPruning(ctx context.Context, db *sql.DB, days int) {
	log.Printf("[Aerolake Bridge] Executing ambient dark data pruner (retention: %d days)...", days)
	var scans, moves, obs int64
	query := `SELECT pruned_scans, pruned_moves, pruned_observations FROM public.prune_ambient_rf_telemetry_v45($1)`
	err := db.QueryRowContext(ctx, query, days).Scan(&scans, &moves, &obs)
	if err != nil {
		log.Printf("[Aerolake Bridge] WARN: prune failed: %v", err)
		return
	}
	log.Printf("[Aerolake Bridge] Pruned: %d raw scans, %d movements, %d observations. Storage saved.", scans, moves, obs)
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
