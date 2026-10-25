// Author: Sunil Jain, 2026-10-25
// Preserve original customer documents and all proposed extraction facts before planning.
package documentcapture

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const MaxOriginalBytes int64 = 32 << 20

type Fact struct {
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

type Record struct {
	TenantID   string
	Filename   string
	Path       string
	Mode       string
	Extraction any
}

// Flatten retains every leaf supplied by the extractor, including optional fields.
// It never invents a confidence score, unit, approval, or missing value.
func Flatten(v any) ([]Fact, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tree any
	if err := json.Unmarshal(b, &tree); err != nil {
		return nil, err
	}
	result := make([]Fact, 0)
	var walk func(string, any) error
	walk = func(path string, item any) error {
		switch x := item.(type) {
		case map[string]any:
			for k, value := range x {
				if err := walk(path+"/"+escape(k), value); err != nil {
					return err
				}
			}
		case []any:
			for i, value := range x {
				if err := walk(fmt.Sprintf("%s/%d", path, i), value); err != nil {
					return err
				}
			}
		default:
			if item == nil {
				return nil
			}
			value, err := json.Marshal(item)
			if err != nil {
				return err
			}
			result = append(result, Fact{Path: path, Value: value})
		}
		return nil
	}
	if err := walk("", tree); err != nil {
		return nil, err
	}
	return result, nil
}
func escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }

// Capture commits the unmodified source, raw extraction and flattened assertions
// as PROPOSED in one transaction. It does not approve or change factory inventory.
// Run migration 0004_document_capture.sql before enabling this path.
func Capture(ctx context.Context, db *sql.DB, rec Record) (string, error) {
	if db == nil {
		return "", errors.New("capture requires PostgreSQL")
	}
	if strings.TrimSpace(rec.TenantID) == "" || strings.TrimSpace(rec.Filename) == "" {
		return "", errors.New("tenant and filename are required")
	}
	info, err := os.Stat(rec.Path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxOriginalBytes {
		return "", errors.New("unsupported source size or file type")
	}
	original, err := os.ReadFile(rec.Path)
	if err != nil {
		return "", err
	}
	if int64(len(original)) > MaxOriginalBytes {
		return "", errors.New("source exceeds capture limit")
	}
	raw, err := json.Marshal(rec.Extraction)
	if err != nil {
		return "", err
	}
	facts, err := Flatten(rec.Extraction)
	if err != nil {
		return "", err
	}
	contentHash := sha256.Sum256(original)
	identityHash := sha256.Sum256([]byte(rec.TenantID + "\x00" + rec.Filename + "\x00" + hex.EncodeToString(contentHash[:])))
	id := hex.EncodeToString(identityHash[:])
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id',$1,true)", rec.TenantID); err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO intake_document_capture
        (id,tenant_id,filename,content_sha256,source_mode,original_bytes,extraction_json,status)
        VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,'PROPOSED')
        ON CONFLICT (id) DO NOTHING`, id, rec.TenantID, filepath.Base(rec.Filename), hex.EncodeToString(contentHash[:]), rec.Mode, original, string(raw))
	if err != nil {
		return "", fmt.Errorf("capture original: %w", err)
	}
	for _, fact := range facts {
		_, err = tx.ExecContext(ctx, `INSERT INTO intake_proposed_fact (capture_id,tenant_id,json_pointer,value_json)
            VALUES ($1,$2,$3,$4::jsonb) ON CONFLICT (capture_id,json_pointer) DO NOTHING`, id, rec.TenantID, fact.Path, string(fact.Value))
		if err != nil {
			return "", fmt.Errorf("capture fact %s: %w", fact.Path, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// MarkSynced is recorded only after the existing operational sync and metadata succeed.
func MarkSynced(ctx context.Context, db *sql.DB, tenant, id string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id',$1,true)", tenant); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE intake_document_capture SET status='SYNCED',synced_at=NOW()
        WHERE id=$1 AND tenant_id=$2 AND status IN ('PROPOSED','SYNCED')`, id, tenant)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("capture not found or not writable")
	}
	return tx.Commit()
}
