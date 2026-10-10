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
	h := sha256.Sum256([]byte("GENESIS_BLOCK_AGIPAREL_REV11"))
	return &MerkleLedger{
		secretKey:    []byte(seedKey),
		currentIndex: 0,
		lastHash:     hex.EncodeToString(h[:]),
	}
}

func (l *MerkleLedger) Hydrate(events []AuditEvent) {
	l.Lock()
	defer l.Unlock()
	for _, ev := range events {
		if ev.Index > l.currentIndex {
			l.currentIndex = ev.Index
			l.lastHash = ev.EntryHash
		}
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

func (l *MerkleLedger) GetLatestHash() string {
	l.Lock()
	defer l.Unlock()
	return l.lastHash
}

func (l *MerkleLedger) GetCurrentIndex() uint64 {
	l.Lock()
	defer l.Unlock()
	return l.currentIndex
}