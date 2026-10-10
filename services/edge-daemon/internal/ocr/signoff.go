package ocr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

type SealCertificate struct {
	CertificateID string              `json:"certificate_id"`
	OrderID       string              `json:"order_id"`
	CutNo         int                 `json:"cut_no"`
	SealedAt      int64               `json:"sealed_at"`
	DocumentHash  string              `json:"document_hash"`
	Signatures    map[string]string   `json:"signatures"`
	Report        ReconciliationReport `json:"report"`
}

type SignOffSession struct {
	sync.Mutex
	Report    *ReconciliationReport
	Signatures map[string]string
	SecretKey  string
	IsSealed   bool
	Cert       *SealCertificate
}

func NewSignOffSession(report *ReconciliationReport, secretKey string) *SignOffSession {
	return &SignOffSession{
		Report:     report,
		Signatures: make(map[string]string),
		SecretKey:  secretKey,
	}
}

func (s *SignOffSession) Sign(dept, signerID string) error {
	s.Lock()
	defer s.Unlock()

	if s.IsSealed {
		return errors.New("ERR_SESSION_SEALED: Cannot sign an already sealed certificate")
	}

	if err := s.Report.SignOff(dept); err != nil {
		return err
	}

	now := time.Now().UTC().UnixMilli()
	claim := fmt.Sprintf("%s:%d:%s:%s:%d", s.Report.OrderID, s.Report.CutNo, dept, signerID, now)
	mac := hmac.New(sha256.New, []byte(s.SecretKey))
	mac.Write([]byte(claim))
	s.Signatures[dept] = hex.EncodeToString(mac.Sum(nil))

	return nil
}

func (s *SignOffSession) Seal() (*SealCertificate, error) {
	s.Lock()
	defer s.Unlock()

	if s.IsSealed {
		return s.Cert, nil
	}

	if !s.Report.CanSeal() {
		return nil, errors.New("ERR_INCOMPLETE_SIGNOFFS: All 6 department heads must sign before sealing")
	}

	now := time.Now().UTC().UnixMilli()
	certID := fmt.Sprintf("OCR-%s-CUT%03d", s.Report.OrderID, s.Report.CutNo)

	docPayload := fmt.Sprintf("%s:%d:%d:%d:%d",
		certID,
		s.Report.Breakdown.CutQty,
		s.Report.Breakdown.ShippedQty,
		s.Report.Breakdown.LostQty,
		s.Report.Breakdown.LeftOverQty,
	)
	docHash := sha256.Sum256([]byte(docPayload))

	s.Cert = &SealCertificate{
		CertificateID: certID,
		OrderID:       s.Report.OrderID,
		CutNo:         s.Report.CutNo,
		SealedAt:      now,
		DocumentHash:  hex.EncodeToString(docHash[:]),
		Signatures:    s.Signatures,
		Report:        *s.Report,
	}
	s.IsSealed = true

	return s.Cert, nil
}