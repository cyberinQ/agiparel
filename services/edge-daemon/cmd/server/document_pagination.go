// Author: Sunil Jain, 2026-10-28. Bounded tenant-local catalog and source cursors.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

const docPageMax = 100
const docSegmentsPageMax = 200

type docPageCursor struct {
	CapturedAt time.Time `json:"captured_at"`
	ID         string    `json:"id"`
}

func docCursorEncode(at time.Time, id string) string {
	b, _ := json.Marshal(docPageCursor{CapturedAt: at.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}
func docCursorDecode(raw string) (docPageCursor, error) {
	var c docPageCursor
	if raw == "" {
		return c, nil
	}
	if len(raw) > 512 {
		return c, errors.New("cursor too long")
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return c, errors.New("malformed cursor")
	}
	if err = json.Unmarshal(b, &c); err != nil || c.CapturedAt.IsZero() || len(c.ID) != 36 || !strings.HasPrefix(c.ID, "DOC-") {
		return c, errors.New("invalid cursor")
	}
	for _, ch := range c.ID[4:] {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return c, errors.New("invalid cursor")
		}
	}
	return c, nil
}
func docPageSize(raw string) (int, error) {
	if raw == "" {
		return 100, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > docPageMax {
		return 0, errors.New("limit must be between 1 and 100")
	}
	return n, nil
}

func docNullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// docSegmentAfter limits the source cursor to the physical source ordinal
// domain; it is not a row offset and never changes evidence identity.
func docSegmentAfter(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > 10000 {
		return 0, errors.New("invalid source segment cursor")
	}
	return n, nil
}
