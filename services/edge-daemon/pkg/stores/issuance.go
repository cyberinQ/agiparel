package stores

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

type RollStock struct {
	RollID          string  `json:"roll_id"`
	OriginalLengthM float64 `json:"original_length_m"`
	RemainingLength float64 `json:"remaining_length_m"`
	ReservedOrderID string  `json:"reserved_order_id"`
	ReservedCutNo   int     `json:"reserved_cut_no"`
	Status          string  `json:"status"` // AVAILABLE, RESERVED, ISSUED, DEPLETED
}

type IssuanceRecord struct {
	IssuanceID       string  `json:"issuance_id"`
	RollID           string  `json:"roll_id"`
	OrderID          string  `json:"order_id"`
	CutNo            int     `json:"cut_no"`
	IssuedLengthM    float64 `json:"issued_length_m"`
	IssuerID         string  `json:"issuer_id"`
	RecipientID      string  `json:"recipient_id"`
	IssuedAt         int64   `json:"issued_at"`
	RemainingLengthM float64 `json:"remaining_length_m"`
	DigitalSigHash   string  `json:"digital_signature_hash"`
}

type StoresEngine struct {
	sync.Mutex
	inventory map[string]*RollStock
}

func NewStoresEngine() *StoresEngine {
	return &StoresEngine{
		inventory: make(map[string]*RollStock),
	}
}

func (s *StoresEngine) RegisterRoll(roll RollStock) {
	s.Lock()
	defer s.Unlock()
	s.inventory[roll.RollID] = &roll
}

func (s *StoresEngine) DoubleWitnessIssue(rollID, orderID string, cutNo int, issueLen float64, issuerID, recipientID, secretKey string) (*IssuanceRecord, error) {
	s.Lock()
	defer s.Unlock()

	roll, exists := s.inventory[rollID]
	if !exists {
		return nil, errors.New("ERR_ROLL_NOT_FOUND: Roll does not exist in store catalog")
	}

	if roll.Status == "DEPLETED" || roll.RemainingLength < issueLen {
		return nil, fmt.Errorf("ERR_INSUFFICIENT_STOCK: Requested %.2fm exceeds available %.2fm", issueLen, roll.RemainingLength)
	}

	if roll.ReservedOrderID != "" && (roll.ReservedOrderID != orderID || roll.ReservedCutNo != cutNo) {
		return nil, fmt.Errorf("ERR_SACROSANCT_RESERVATION: Roll reserved for %s (Cut %d), cannot issue to %s (Cut %d)",
			roll.ReservedOrderID, roll.ReservedCutNo, orderID, cutNo)
	}

	roll.RemainingLength -= issueLen
	if roll.RemainingLength == 0 {
		roll.Status = "DEPLETED"
	} else {
		roll.Status = "ISSUED"
	}

	now := time.Now().UTC().UnixMilli()
	claim := fmt.Sprintf("%s:%s:%d:%.2f:%s:%s:%d:%s", rollID, orderID, cutNo, issueLen, issuerID, recipientID, now, secretKey)
	sig := sha256.Sum256([]byte(claim))

	record := &IssuanceRecord{
		IssuanceID:       fmt.Sprintf("ISS-%d", now),
		RollID:           rollID,
		OrderID:          orderID,
		CutNo:            cutNo,
		IssuedLengthM:    issueLen,
		IssuerID:         issuerID,
		RecipientID:      recipientID,
		IssuedAt:         now,
		RemainingLengthM: roll.RemainingLength,
		DigitalSigHash:   hex.EncodeToString(sig[:]),
	}

	return record, nil
}