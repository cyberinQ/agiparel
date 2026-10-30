package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyWatermarkGuard_FailsClosedOnEmptyOrUncompacted(t *testing.T) {
	producer := NewFlightProducer("127.0.0.1:51051")
	now := time.Now().UTC()

	// Empty bytes
	if err := producer.VerifyWatermarkGuard(now, nil); err == nil {
		t.Fatalf("expected error on nil watermark bytes, got nil")
	}

	// Empty JSON object
	if err := producer.VerifyWatermarkGuard(now, []byte("{}")); err == nil {
		t.Fatalf("expected error on empty watermark object, got nil")
	}

	// Invalid timestamp
	badTS, _ := json.Marshal(CheckpointWatermark{
		TableName:                "edge_ambient_rf_telemetry_v45",
		LastCompactedTimestampMs: 0,
	})
	if err := producer.VerifyWatermarkGuard(now, badTS); err == nil {
		t.Fatalf("expected error on zero timestamp watermark, got nil")
	}
}

func TestVerifyWatermarkGuard_RejectsPruningAheadOfCompaction(t *testing.T) {
	producer := NewFlightProducer("127.0.0.1:51051")

	// Aerolake compacted up to 10 minutes ago
	watermarkTime := time.Now().Add(-10 * time.Minute)
	wmBytes, err := json.Marshal(CheckpointWatermark{
		TableName:                "edge_ambient_rf_telemetry_v45",
		LastCompactedTimestampMs: watermarkTime.UnixMilli(),
		TotalCompactedRows:       50000,
	})
	if err != nil {
		t.Fatalf("failed to marshal test watermark: %v", err)
	}

	// Attempting to prune up to 5 minutes ago (ahead of aerolake's 10m watermark) -> MUST REJECT
	pruneTargetAhead := time.Now().Add(-5 * time.Minute)
	if err := producer.VerifyWatermarkGuard(pruneTargetAhead, wmBytes); err == nil {
		t.Fatalf("expected safety violation when pruning ahead of watermark, got nil")
	}

	// Attempting to prune up to 15 minutes ago (safely behind aerolake's 10m watermark) -> MUST PASS
	pruneTargetBehind := time.Now().Add(-15 * time.Minute)
	if err := producer.VerifyWatermarkGuard(pruneTargetBehind, wmBytes); err != nil {
		t.Fatalf("unexpected error when pruning safely behind watermark: %v", err)
	}
}
