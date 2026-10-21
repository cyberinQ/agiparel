package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrHashChainTampered = errors.New("ERR_AUDIT_BREACH: Hash chain broken at index")

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

type EventBlock struct {
	Index        uint64    `json:"index"`
	Timestamp    time.Time `json:"timestamp"`
	StationID    string    `json:"station_id"`
	OperatorID   string    `json:"operator_id"`
	EventType    string    `json:"event_type"`
	PayloadHash  string    `json:"payload_hash"`
	PreviousHash string    `json:"previous_hash"`
	BlockHash    string    `json:"block_hash"`
}

type MerkleLedger struct {
	sync.RWMutex
	secretKey    []byte
	currentIndex uint64
	lastHash     string
	chain        []EventBlock
}

func NewMerkleLedger(secretKey []byte) *MerkleLedger {
	h := sha256.Sum256([]byte("GENESIS_BLOCK_AGIPAREL_REV11"))
	genesisHash := hex.EncodeToString(h[:])

	genesisBlock := EventBlock{
		Index:        0,
		Timestamp:    time.Unix(0, 0).UTC(),
		StationID:    "SYSTEM",
		OperatorID:   "ROOT",
		EventType:    "GENESIS",
		PayloadHash:  "GENESIS_PAYLOAD",
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		BlockHash:    genesisHash,
	}

	return &MerkleLedger{
		secretKey:    secretKey,
		currentIndex: 0,
		lastHash:     genesisHash,
		chain:        []EventBlock{genesisBlock},
	}
}

func (l *MerkleLedger) GetCurrentIndex() uint64 {
	l.RLock()
	defer l.RUnlock()
	return l.currentIndex
}

func (l *MerkleLedger) GetLatestHash() string {
	l.RLock()
	defer l.RUnlock()
	return l.lastHash
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

	claim := fmt.Sprintf("%d:%d:%s:%s:%s:%s:%s",
		l.currentIndex, now, stationID, operatorID, eventType, payloadHashStr, l.lastHash)
	entryHashStr := computeHMAC(l.secretKey, claim)

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

	l.chain = append(l.chain, EventBlock{
		Index:        l.currentIndex,
		Timestamp:    time.UnixMilli(now).UTC(),
		StationID:    stationID,
		OperatorID:   operatorID,
		EventType:    eventType,
		PayloadHash:  payloadHashStr,
		PreviousHash: event.PreviousHash,
		BlockHash:    entryHashStr,
	})

	return event, nil
}

func (l *MerkleLedger) VerifyChain() (bool, error) {
	l.RLock()
	defer l.RUnlock()

	for i := 1; i < len(l.chain); i++ {
		curr := l.chain[i]
		prev := l.chain[i-1]

		if curr.PreviousHash != prev.BlockHash {
			return false, fmt.Errorf("%w: %d (broken link)", ErrHashChainTampered, curr.Index)
		}

		claim := fmt.Sprintf("%d:%d:%s:%s:%s:%s:%s",
			curr.Index, curr.Timestamp.UnixMilli(), curr.StationID, curr.OperatorID, curr.EventType, curr.PayloadHash, curr.PreviousHash)
		expectedHash := computeHMAC(l.secretKey, claim)

		if curr.BlockHash != expectedHash {
			return false, fmt.Errorf("%w: %d (payload alteration)", ErrHashChainTampered, curr.Index)
		}
	}
	return true, nil
}

func computeHMAC(key []byte, message string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}