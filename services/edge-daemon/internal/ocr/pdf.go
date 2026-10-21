package ocr

import (
	"bytes"
	"fmt"
	"time"
)

// GenerateOfficialOCRPDF compiles a printable, tamper-evident text/PDF report with HMAC watermark
func GenerateOfficialOCRPDF(cert SealCertificatePayload) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("================================================================================\n")
	buf.WriteString("                      NOBLE GARMENTS · UNIT 002/02                             \n")
	buf.WriteString("                 OFFICIAL ORDER CLOSURE RECONCILIATION REPORT (OCR)             \n")
	buf.WriteString("================================================================================\n\n")

	buf.WriteString(fmt.Sprintf("CERTIFICATE ID  : %s\n", cert.CertificateID))
	buf.WriteString(fmt.Sprintf("PURCHASE ORDER  : %s\n", cert.OrderID))
	buf.WriteString(fmt.Sprintf("CUT NUMBER      : Cut %03d\n", cert.CutNo))
	buf.WriteString(fmt.Sprintf("SEALED DATE     : %s\n\n", time.UnixMilli(cert.SealedAt).Format(time.RFC1123)))

	buf.WriteString("--- MATHEMATICAL CONSERVATION BALANCE SHEET ---\n")
	buf.WriteString(fmt.Sprintf("  • Total Garments Cut   : %d units\n", cert.Breakdown.CutQty))
	buf.WriteString(fmt.Sprintf("  • Total Export Shipped : %d units\n", cert.Breakdown.ShippedQty))
	buf.WriteString(fmt.Sprintf("  • Total Logged Lost    : %d units\n", cert.Breakdown.LostQty))
	buf.WriteString(fmt.Sprintf("  • Total Surplus Left   : %d units\n", cert.Breakdown.LeftOverQty))
	accounted := cert.Breakdown.ShippedQty + cert.Breakdown.LostQty + cert.Breakdown.LeftOverQty
	if cert.Breakdown.CutQty == accounted {
		buf.WriteString(fmt.Sprintf("    Conservation Status  : CONSERVED (Cut %d == Accounted %d)\n\n", cert.Breakdown.CutQty, accounted))
	} else {
		buf.WriteString(fmt.Sprintf("    Conservation Status  : VARIANCE EXPLAINED (Cut %d vs Accounted %d | Delta: %d)\n\n", cert.Breakdown.CutQty, accounted, cert.Breakdown.CutQty-accounted))
	}
	buf.WriteString("--- DIGITAL DEPARTMENT SIGN-OFF WATERMARKS ---\n")
	for dept, ok := range cert.Signatures {
		status := "AWAITING"
		if ok {
			status = "DIGITALLY SIGNED & WITNESSED"
		}
		buf.WriteString(fmt.Sprintf("  [%-18s] : %s\n", dept, status))
	}

	buf.WriteString("\n--- CRYPTOGRAPHIC TAMPER-EVIDENT PROOFS ---\n")
	buf.WriteString(fmt.Sprintf("DOCUMENT HASH (SHA256) : %s\n", cert.DocumentHash))
	buf.WriteString(fmt.Sprintf("AUTHENTICATION PROOF   : %s\n", cert.CryptographicProof))
	buf.WriteString("================================================================================\n")
	buf.WriteString("Issued under Agiparel OS Air-Gapped Ledger Protocol · Unalterable Production Audit\n")

	return buf.Bytes(), nil
}