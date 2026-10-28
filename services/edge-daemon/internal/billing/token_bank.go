package billing

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type HardwareTokenBank struct {
	mu         sync.Mutex
	tenantID   string
	plantID    string
	balance    int64
	publicKey  ed25519.PublicKey
	usedNonces map[string]bool
}

func NewHardwareTokenBank(tenantID, plantID string, pubKey ed25519.PublicKey) *HardwareTokenBank {
	return &HardwareTokenBank{
		tenantID:   tenantID,
		plantID:    plantID,
		balance:    1000, // Seed pilot allowance
		publicKey:  pubKey,
		usedNonces: make(map[string]bool),
	}
}

func (b *HardwareTokenBank) GetBalance() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.balance
}

// DeductPieces decrements credits per piece/bundle mapped at cutting. Locks if balance is 0.
func (b *HardwareTokenBank) DeductPieces(pieces int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if pieces <= 0 {
		return errors.New("ERR_INVALID_DEDUCTION: Piece count must be strictly positive")
	}
	if b.balance < pieces {
		return fmt.Errorf("ERR_METER_EXHAUSTED: Remaining credit balance (%d) insufficient for %d pieces. Top up via signed voucher", b.balance, pieces)
	}
	b.balance -= pieces
	return nil
}

// RedeemVoucher applies an ed25519-signed offline top-up voucher (Format: "TENANT:PLANT:CREDITS:NONCE:SIG_B64")
func (b *HardwareTokenBank) RedeemVoucher(voucher string) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	parts := strings.Split(voucher, ":")
	if len(parts) != 5 {
		return b.balance, errors.New("ERR_INVALID_VOUCHER_FORMAT")
	}
	tenant, plant, credStr, nonce, sigB64 := parts[0], parts[1], parts[2], parts[3], parts[4]

	if tenant != b.tenantID || plant != b.plantID {
		return b.balance, errors.New("ERR_VOUCHER_TENANT_MISMATCH")
	}
	if b.usedNonces[nonce] {
		return b.balance, errors.New("ERR_NONCE_ALREADY_REDEEMED")
	}

	credits, err := strconv.ParseInt(credStr, 10, 64)
	if err != nil || credits <= 0 {
		return b.balance, errors.New("ERR_INVALID_VOUCHER_CREDITS: Credits must be a positive integer")
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		sigBytes, err = base64.StdEncoding.DecodeString(sigB64)
		if err != nil {
			return b.balance, fmt.Errorf("ERR_INVALID_SIG_ENCODING: %w", err)
		}
	}

	claim := []byte(fmt.Sprintf("%s:%s:%s:%s", tenant, plant, credStr, nonce))
	if len(b.publicKey) != ed25519.PublicKeySize || !ed25519.Verify(b.publicKey, claim, sigBytes) {
		return b.balance, errors.New("ERR_CRYPTOGRAPHIC_SIGNATURE_INVALID")
	}

	b.balance += credits
	b.usedNonces[nonce] = true
	return b.balance, nil
}
