package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type AuditEvent struct {
	Index        uint64 `json:"index"`
	Timestamp    int64  `json:"timestamp"`
	StationID    string `json:"station_id"`
	OperatorID   string `json:"operator_id"`
	EventType    string `json:"event_type"`
	PayloadHash  string `json:"payload_hash"`
	PreviousHash string `json:"prev_hash"`
	EntryHash    string `json:"entry_hash"`
}

type MerkleLedger struct {
	sync.Mutex
	secretKey    []byte
	currentIndex uint64
	lastHash     string
}

func NewMerkleLedger(seedKey string) *MerkleLedger {
	seedData := []byte("GENESIS_BLOCK_AGIPAREL_REV11_" + seedKey)
	sum := sha256.Sum256(seedData)
	genesisHash := hex.EncodeToString(sum[:])
	return &MerkleLedger{
		secretKey:    []byte(seedKey),
		currentIndex: 0,
		lastHash:     genesisHash,
	}
}

func (l *MerkleLedger) AppendEvent(stationID, operatorID, eventType, rawPayload string) (AuditEvent, error) {
	l.Lock()
	defer l.Unlock()

	l.currentIndex++
	now := time.Now().UTC().UnixMilli()

	payloadSha := sha256.Sum256([]byte(rawPayload))
	payloadHashStr := hex.EncodeToString(payloadSha[:])

	mac := hmac.New(sha256.New, l.secretKey)
	claim := fmt.Sprintf("%d:%d:%s:%s:%s:%s:%s",
		l.currentIndex, now, stationID, operatorID, eventType, payloadHashStr, l.lastHash)
	mac.Write([]byte(claim))
	entryHashStr := hex.EncodeToString(mac.Sum(nil))

	event := AuditEvent{
		Index:        l.currentIndex,
		Timestamp:    now,
		StationID:    stationID,
		OperatorID:   operatorID,
		EventType:    eventType,
		PayloadHash:  payloadHashStr,
		PreviousHash: l.lastHash,
		EntryHash:    entryHashStr,
	}

	l.lastHash = entryHashStr
	return event, nil
}

// Hydrate rebuilds the ledger state from a historical slice of audit events during boot.
func (l *MerkleLedger) Hydrate(events []AuditEvent) {
	l.Lock()
	defer l.Unlock()
	for _, ev := range events {
		if ev.Index > l.currentIndex {
			l.currentIndex = ev.Index
		}
		if ev.EntryHash != "" {
			l.lastHash = ev.EntryHash
		}
	}
}

// GetCurrentIndex returns the current sequence number of the ledger chain.
func (l *MerkleLedger) GetCurrentIndex() uint64 {
	l.Lock()
	defer l.Unlock()
	return l.currentIndex
}

func (l *MerkleLedger) GetLatestHash() string {
	l.Lock()
	defer l.Unlock()
	return l.lastHash
}