package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CheckpointWatermark models the durable Parquet state emitted by aerolake-agentd.
type CheckpointWatermark struct {
	TableName                 string `json:"table_name"`
	LastCompactedSequence     uint64 `json:"last_compacted_sequence"`
	LastCompactedTimestampMs  int64  `json:"last_compacted_timestamp_ms"`
	TotalCompactedRows        uint64 `json:"total_compacted_rows"`
}

// FlightProducer manages the client stream and watermark safety assertions.
type FlightProducer struct {
	endpoint string
	conn     *grpc.ClientConn
	mu       sync.Mutex
}

// NewFlightProducer creates a new managed flight producer client.
func NewFlightProducer(endpoint string) *FlightProducer {
	return &FlightProducer{
		endpoint: endpoint,
	}
}

// Connect dials the aerolake gRPC Arrow Flight endpoint with a fail-closed timeout.
func (p *FlightProducer) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		return nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, p.endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("aerolake flight unreachable at %s: %w", p.endpoint, err)
	}

	p.conn = conn
	return nil
}

// Close tears down active connection handles cleanly.
func (p *FlightProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil
		return err
	}
	return nil
}

// VerifyWatermarkGuard ensures local pruning does not advance past aerolake's durable Parquet timestamp.
func (p *FlightProducer) VerifyWatermarkGuard(localPruneBefore time.Time, rawWatermarkJSON []byte) error {
	if len(rawWatermarkJSON) == 0 || string(rawWatermarkJSON) == "{}" {
		return fmt.Errorf("fail-closed: aerolake has not yet committed any durable parquet chunks")
	}

	var wm CheckpointWatermark
	if err := json.Unmarshal(rawWatermarkJSON, &wm); err != nil {
		return fmt.Errorf("malformed watermark metadata: %w", err)
	}

	if wm.LastCompactedTimestampMs <= 0 {
		return fmt.Errorf("invalid watermark timestamp (%d): unverified compaction state", wm.LastCompactedTimestampMs)
	}

	watermarkTime := time.UnixMilli(wm.LastCompactedTimestampMs)
	if localPruneBefore.After(watermarkTime) {
		return fmt.Errorf("safety violation: requested prune horizon %s is ahead of aerolake durable watermark %s",
			localPruneBefore.UTC().Format(time.RFC3339),
			watermarkTime.UTC().Format(time.RFC3339))
	}

	return nil
}

// CheckSocketLive probes whether the port listener is bound before streaming.
func (p *FlightProducer) CheckSocketLive(timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", p.endpoint, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
