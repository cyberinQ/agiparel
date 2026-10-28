// Author: Sunil Jain, 2026-10-27
// Reject unsafe document types and active payloads before immutable storage; optionally require ClamAV.
// These preflight checks are not a replacement for an updated AV engine or sandboxed extraction.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/document-intake/extract"
)

const maxOfficeEntries = 2000
const maxOfficeUnpacked = 96 << 20

var pdfNameToken = regexp.MustCompile(`/[A-Za-z0-9#]+`)
var pdfHexEscape = regexp.MustCompile(`#[0-9A-Fa-f]{2}`)
var pdfDangerousNames = map[string]bool{
	"JS": true, "JavaScript": true, "Launch": true,
	"EmbeddedFile": true, "EmbeddedFiles": true, "EF": true,
	"RichMedia": true, "XFA": true, "SubmitForm": true, "ImportData": true,
}

func unsafePDF(raw []byte) bool {
	// PDF name objects may hex-escape letters, e.g. /J#53 for /JS.
	for _, token := range pdfNameToken.FindAll(raw, -1) {
		name := string(token[1:])
		name = pdfHexEscape.ReplaceAllStringFunc(name, func(s string) string {
			v, e := strconv.ParseUint(s[1:], 16, 8)
			if e != nil {
				return s
			}
			return string(rune(v))
		})
		if pdfDangerousNames[name] {
			return true
		}
	}
	return false
}

// External HTTP(S) links in an Office source are inert to our XML-only extractor.
// Reject local paths, protocol handlers and active relationship types that could
// execute or disclose data if an original is later opened in desktop Office.
func hasUnsafeExternalOfficeRelation(content []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(content))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return false
		}
		if err != nil {
			return true // Malformed relationship XML is not trusted.
		}
		el, ok := token.(xml.StartElement)
		if !ok || !strings.EqualFold(el.Name.Local, "Relationship") {
			continue
		}
		var mode, target, relationType string
		for _, attr := range el.Attr {
			switch strings.ToLower(attr.Name.Local) {
			case "targetmode":
				mode = strings.TrimSpace(attr.Value)
			case "target":
				target = strings.TrimSpace(attr.Value)
			case "type":
				relationType = strings.ToLower(attr.Value)
			}
		}
		if !strings.EqualFold(mode, "External") {
			continue
		}
		if strings.Contains(relationType, "attachedtemplate") || strings.Contains(relationType, "oleobject") || strings.Contains(relationType, "package") {
			return true
		}
		u, err := url.Parse(target)
		if err != nil || u == nil {
			return true
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" && scheme != "mailto" {
			return true
		}
		if (scheme == "http" || scheme == "https") && u.Hostname() == "" {
			return true
		}
	}
}

func unsafeOffice(raw []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return errors.New("Office file is damaged")
	}
	if len(zr.File) > maxOfficeEntries {
		return errors.New("Office file has too many internal parts")
	}
	var total uint64
	for _, entry := range zr.File {
		path := strings.ToLower(strings.ReplaceAll(entry.Name, "\\", "/"))
		if strings.HasPrefix(path, "/") || strings.Contains(path, "../") || strings.HasPrefix(path, "../") {
			return errors.New("Office file contains an unsafe path")
		}
		if entry.UncompressedSize64 > 24<<20 {
			return errors.New("Office file has an oversized internal part")
		}
		total += entry.UncompressedSize64
		if total > maxOfficeUnpacked {
			return errors.New("Office file expands beyond the safe limit")
		}
		if entry.CompressedSize64 > 0 && entry.UncompressedSize64 > entry.CompressedSize64*250 && entry.UncompressedSize64 > 1<<20 {
			return errors.New("Office file contains excessively compressed content")
		}
		if strings.Contains(path, "vbaproject") || strings.Contains(path, "macrosheet") || strings.Contains(path, "activex") || strings.Contains(path, "/embeddings/") || strings.Contains(path, "/customui/") || strings.HasSuffix(path, ".exe") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".vbs") {
			return errors.New("Office file includes macros, embedded code, or linked content")
		}
		if strings.HasSuffix(path, ".rels") || path == "[content_types].xml" {
			if entry.UncompressedSize64 > 2<<20 {
				return errors.New("Office relationships are too large")
			}
			rc, e := entry.Open()
			if e != nil {
				return errors.New("Cannot inspect Office relationships")
			}
			content, e := io.ReadAll(io.LimitReader(rc, (2<<20)+1))
			_ = rc.Close()
			if e != nil {
				return errors.New("Cannot inspect Office relationships")
			}
			text := strings.ToLower(string(content))
			if (strings.HasSuffix(path, ".rels") && hasUnsafeExternalOfficeRelation(content)) || strings.Contains(text, "macroenabled") || strings.Contains(text, "vbaproject") || strings.Contains(text, "activex") {
				return errors.New("Office file contains active or external content")
			}
		}
	}
	return nil
}

// checkDocumentSafety runs before original bytes are stored or sent to parsers.
// Apparel product imagery is not rejected based on exposed skin or clothing type.
func checkDocumentSafety(ctx context.Context, name string, raw []byte) error {
	ext := strings.ToLower(filepath.Ext(name))
	format, _ := extract.Detect(name, raw)
	valid := false
	switch ext {
	case ".pdf":
		valid = format == "PDF"
	case ".xlsx":
		valid = format == "XLSX"
	case ".xls":
		valid = format == "XLS_LEGACY"
	case ".docx":
		valid = format == "DOCX"
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		valid = format == "IMAGE"
	case ".csv":
		valid = format == "CSV" || format == "TEXT"
	case ".tsv":
		valid = format == "TSV" || format == "TEXT"
	case ".txt", ".md":
		valid = format == "TEXT"
	}
	if !valid {
		return errors.New("unsupported or mismatched file type; use PDF, images, CSV, XLSX, DOCX, or scanned legacy XLS")
	}
	if format == "PDF" && unsafePDF(raw) {
		return errors.New("PDF contains active scripts or embedded content")
	}
	if format == "XLSX" || format == "DOCX" {
		if err := unsafeOffice(raw); err != nil {
			return err
		}
	}
	address := strings.TrimSpace(os.Getenv("AGIPAREL_CLAMD_ADDR"))
	if format == "XLS_LEGACY" && address == "" {
		return errors.New("old .xls files need antivirus scanning; save a copy as .xlsx or enable ClamAV")
	}
	if address == "" {
		if strings.EqualFold(os.Getenv("ENV"), "production") {
			return errors.New("antivirus service is required before production uploads")
		}
		return nil // Development: structural screening only; never claim AV-clean.
	}
	return scanClamAV(ctx, address, raw)
}

// ClamAV INSTREAM protocol, no shell, no file execution, bounded data and time.
// A scanner error or timeout is a rejection, not a clean result.
func scanClamAV(ctx context.Context, address string, raw []byte) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.New("antivirus service unavailable")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
	if _, err = io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return errors.New("antivirus scan failed")
	}
	var header [4]byte
	for pos := 0; pos < len(raw); {
		end := pos + 32*1024
		if end > len(raw) {
			end = len(raw)
		}
		binary.BigEndian.PutUint32(header[:], uint32(end-pos))
		if _, err = conn.Write(header[:]); err != nil {
			return errors.New("antivirus scan failed")
		}
		if _, err = conn.Write(raw[pos:end]); err != nil {
			return errors.New("antivirus scan failed")
		}
		pos = end
	}
	if _, err = conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return errors.New("antivirus scan failed")
	}
	response, err := bufio.NewReader(io.LimitReader(conn, 4096)).ReadString(0)
	if err != nil {
		return errors.New("antivirus service returned no result")
	}
	if strings.HasSuffix(strings.TrimRight(response, "\x00\r\n"), ": OK") {
		return nil
	}
	if strings.Contains(response, "FOUND") {
		return errors.New("malware detected; file not accepted")
	}
	return fmt.Errorf("antivirus service did not approve the file")
}
