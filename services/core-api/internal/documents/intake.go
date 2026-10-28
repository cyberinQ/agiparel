package documents

import (
	"time"
)

type DocumentRecord struct {
	DocumentID           string    `json:"document_id"`
	DocumentType         string    `json:"document_type"` // e.g., "BUYER_PO", "TECH_PACK", "OB"
	Issuer               string    `json:"issuer"`
	ScopeTenantID        string    `json:"scope_tenant_id"`
	Revision             int       `json:"revision"`
	EffectiveFrom        time.Time `json:"effective_from"`
	FileChecksum         string    `json:"file_checksum"`
	ApprovalStatus       string    `json:"approval_status"` // "PROPOSED", "APPROVED", "SUPERSEDED"
	SupersedesDocumentID *string   `json:"supersedes_document_id,omitempty"`
}
