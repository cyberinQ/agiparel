package stores

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
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

	if issueLen <= 0 {
		return nil, errors.New("ERR_INVALID_ISSUE_LEN: Issued length must be greater than zero")
	}
	if strings.TrimSpace(issuerID) == "" || strings.TrimSpace(recipientID) == "" || issuerID == recipientID {
		return nil, errors.New("ERR_INVALID_WITNESS: Distinct issuer and recipient digital IDs are required")
	}

	roll, exists := s.inventory[rollID]
	if !exists {
		return nil, errors.New("ERR_ROLL_NOT_FOUND: Roll does not exist in store catalog")
	}

	// Round to nearest millimeter to eliminate IEEE-754 representation drift
	availMM := math.Round(roll.RemainingLength * 1000)
	reqMM := math.Round(issueLen * 1000)

	if roll.Status == "DEPLETED" || availMM < reqMM {
		return nil, fmt.Errorf("ERR_INSUFFICIENT_STOCK: Requested %.2fm exceeds available %.2fm", issueLen, roll.RemainingLength)
	}

	if roll.ReservedOrderID != "" && (roll.ReservedOrderID != orderID || roll.ReservedCutNo != cutNo) {
		return nil, fmt.Errorf("ERR_SACROSANCT_RESERVATION: Roll reserved for %s (Cut %d), cannot issue to %s (Cut %d)",
			roll.ReservedOrderID, roll.ReservedCutNo, orderID, cutNo)
	}

	remMM := availMM - reqMM
	roll.RemainingLength = remMM / 1000.0
	if remMM <= 0 {
		roll.RemainingLength = 0
		roll.Status = "DEPLETED"
	} else {
		roll.Status = "ISSUED"
	}

	now := time.Now().UTC().UnixMilli()
	claim := fmt.Sprintf("%s:%s:%d:%.3f:%s:%s:%d", rollID, orderID, cutNo, issueLen, issuerID, recipientID, now)
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(claim))

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
		DigitalSigHash:   hex.EncodeToString(mac.Sum(nil)),
	}
	return record, nil
}

// VerifyIssuanceSignature allows offline downstream stations (e.g., cutting tables)
// to cryptographically verify double-witness signatures without reaching the cloud.
func VerifyIssuanceSignature(record IssuanceRecord, secretKey string) bool {
	claim := fmt.Sprintf("%s:%s:%d:%.3f:%s:%s:%d",
		record.RollID,
		record.OrderID,
		record.CutNo,
		record.IssuedLengthM,
		record.IssuerID,
		record.RecipientID,
		record.IssuedAt,
	)

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(claim))
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(record.DigitalSigHash), []byte(expectedHash))
}

// CalculateFourPointScore computes the ASTM D5430 fabric defect score per 100 sq. yards.
// A roll fails if points exceed threshold (standard: 28 to 40 points per 100 sq. yards).
func CalculateFourPointScore(totalDefectPoints int, rollLengthMeters, fabricWidthCm float64) (float64, bool) {
	if rollLengthMeters <= 0 || fabricWidthCm <= 0 {
		return 0.0, false
	}
	widthInches := fabricWidthCm / 2.54
	lengthYards := rollLengthMeters * 1.09361
	pointsPer100SqYards := (float64(totalDefectPoints) * 3600.0) / (widthInches * lengthYards)
	// Industry threshold: <= 28 points per 100 sq. yd passes
	isAcceptable := pointsPer100SqYards <= 28.0
	return math.Round(pointsPer100SqYards*10) / 10, isAcceptable
}
