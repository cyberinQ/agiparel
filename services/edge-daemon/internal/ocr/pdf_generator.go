package ocr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type OCRCertificateGenerator struct {
	SecretKey string
}

type SealCertificatePayload struct {
	CertificateID      string         `json:"certificate_id"`
	OrderID            string         `json:"order_id"`
	CutNo              int            `json:"cut_no"`
	SealedAt           int64          `json:"sealed_at"`
	Breakdown          StageBreakdown `json:"breakdown"`
	Signatures         map[string]bool `json:"signatures"`
	DocumentHash       string         `json:"document_hash"`
	CryptographicProof string         `json:"cryptographic_proof"`
}

func NewOCRCertificateGenerator(secretKey string) *OCRCertificateGenerator {
	return &OCRCertificateGenerator{SecretKey: secretKey}
}

// GenerateSealCertificate validates conservation and produces a tamper-evident certificate payload.
func (g *OCRCertificateGenerator) GenerateSealCertificate(orderID string, cutNo int, b StageBreakdown, signatures map[string]bool) (SealCertificatePayload, error) {
	// 1. Verify Conservation Equation: CutQty = ShippedQty + LostQty + LeftOverQty
	accountedFor := b.ShippedQty + b.LostQty + b.LeftOverQty
	if b.CutQty != accountedFor {
		return SealCertificatePayload{}, fmt.Errorf("ERR_CONSERVATION_VIOLATION: Cut qty (%d) != Accounted (%d)", b.CutQty, accountedFor)
	}

	// 2. Verify all 6 department sign-offs
	for _, dept := range RequiredDepartments {
		if signed, ok := signatures[dept]; !ok || !signed {
			return SealCertificatePayload{}, fmt.Errorf("ERR_MISSING_SIGNOFF: Department %s has not signed off", dept)
		}
	}

	now := time.Now().UTC().UnixMilli()
	certID := fmt.Sprintf("OCR-%s-CUT%03d-%d", orderID, cutNo, now)

	rawPayload, _ := json.Marshal(map[string]interface{}{
		"cert_id":   certID,
		"order_id":  orderID,
		"cut_no":    cutNo,
		"breakdown": b,
		"sealed_at": now,
	})

	docHashBytes := sha256.Sum256(rawPayload)
	docHashStr := hex.EncodeToString(docHashBytes[:])

	mac := hmac.New(sha256.New, []byte(g.SecretKey))
	mac.Write([]byte(docHashStr))
	proofStr := hex.EncodeToString(mac.Sum(nil))

	return SealCertificatePayload{
		CertificateID:      certID,
		OrderID:            orderID,
		CutNo:              cutNo,
		SealedAt:           now,
		Breakdown:          b,
		Signatures:         signatures,
		DocumentHash:       docHashStr,
		CryptographicProof: proofStr,
	}, nil
}
