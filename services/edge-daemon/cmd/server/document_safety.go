// Author: Sunil Jain, 2026-10-28 — Allow storage-only legacy XLS in development; keep active-content screening.
// Screen active document payloads while allowing inert PDF attachments and spreadsheet references.
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
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/document-intake/extract"
)

const maxOfficeEntries = 2000
const maxOfficeUnpacked = 96 << 20

// PDF attachments and navigation are inert for our text-only extractor. Do not
// confuse /EF or /EmbeddedFiles with executable PDF actions. Conversely,
// JavaScript, launch, dynamic form and multimedia actions remain disallowed.
// Production still requires AV; this is structural preflight, not a PDF sandbox.
var pdfActiveNames = map[string]bool{
	"JS": true, "JavaScript": true, "Launch": true, "RichMedia": true,
}

// pdfSyntaxNames walks PDF lexical syntax and skips literal/hex strings and
// binary streams. Naively searching every raw byte produces false positives in
// garment photos and compressed font/image data. We still decode PDF name
// escapes, e.g. /J#53, and examine names in object dictionaries.
func unsafePDF(raw []byte) bool {
	pendingAction := false
	for i := 0; i < len(raw); {
		switch raw[i] {
		case '%': // PDF comment: including the header line
			for i < len(raw) && raw[i] != '\n' && raw[i] != '\r' {
				i++
			}
		case '(': // Parenthesized PDF text string (may contain nested parentheses)
			depth := 1
			i++
			for i < len(raw) && depth > 0 {
				if raw[i] == '\\' {
					i += 2
					continue
				}
				if raw[i] == '(' {
					depth++
				}
				if raw[i] == ')' {
					depth--
				}
				i++
			}
		case '<':
			if i+1 < len(raw) && raw[i+1] == '<' {
				i += 2
				continue
			}
			i++ // hex string, not a dictionary
			for i < len(raw) && raw[i] != '>' {
				i++
			}
			if i < len(raw) {
				i++
			}
		case '/':
			i++
			var decoded strings.Builder
			for i < len(raw) && !isPDFDelimiter(raw[i]) {
				if raw[i] == '#' && i+2 < len(raw) {
					if a, ok := pdfHexNibble(raw[i+1]); ok {
						if b, ok := pdfHexNibble(raw[i+2]); ok {
							decoded.WriteByte(a<<4 | b)
							i += 3
							continue
						}
					}
				}
				decoded.WriteByte(raw[i])
				i++
			}
			name := decoded.String()
			// /XFA can be an inert AcroForm reference; the extractor does not
			// execute forms. Block actual submit/import actions (/S /Name),
			// rather than any occurrence of those names in a static resource.
			if pdfActiveNames[name] || (pendingAction && (name == "SubmitForm" || name == "ImportData")) {
				return true
			}
			pendingAction = name == "S"
		default:
			// PDF streams are binary and may randomly contain /JS, /XFA, etc.
			// Extraction never executes their content. Encoded object streams
			// are not a replacement for production antivirus inspection.
			if bytes.HasPrefix(raw[i:], []byte("stream")) && (i == 0 || isPDFDelimiter(raw[i-1])) {
				j := i + len("stream")
				if j < len(raw) && raw[j] == '\r' {
					j++
				}
				if j < len(raw) && raw[j] == '\n' {
					if k := bytes.Index(raw[j+1:], []byte("endstream")); k >= 0 {
						i = j + 1 + k + len("endstream")
						continue
					}
				}
			}
			i++
		}
	}
	return false
}

func isPDFDelimiter(c byte) bool {
	switch c {
	case 0, 9, 10, 12, 13, 32, '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}
func pdfHexNibble(c byte) (byte, bool) {
	if c >= '0' && c <= '9' {
		return c - '0', true
	}
	if c >= 'a' && c <= 'f' {
		return c - 'a' + 10, true
	}
	if c >= 'A' && c <= 'F' {
		return c - 'A' + 10, true
	}
	return 0, false
}

// The server's Office extractor reads ZIP XML only: it never fetches or follows
// external workbook links. Relative/file/UNC links to other workbooks therefore
// need not block ingestion. They remain in the immutable original; downloaded
// originals should be opened with normal desktop Office security protections.
func hasUnsafeExternalOfficeRelation(content []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(content))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return false
		}
		if err != nil {
			return true
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
		for _, forbidden := range []string{
			"attachedtemplate", "oleobject", "package", "vbaproject", "activex", "querytable", "connections", "ddelink", "olelink",
		} {
			if strings.Contains(relationType, forbidden) {
				return true
			}
		}
		target = strings.TrimSpace(target)
		if target == "" {
			return true
		}
		u, err := url.Parse(target)
		if err != nil {
			return true
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme == "https" || scheme == "http" {
			if u.Hostname() == "" {
				return true
			}
			continue
		}
		if scheme == "mailto" {
			continue
		}
		// Excel's non-executable externalLinkPath may refer to another local
		// spreadsheet (Windows drive, UNC, relative path, or file:// URL).
		// These links are stored, never dereferenced by the extraction worker.
		if strings.Contains(relationType, "externallinkpath") && (scheme == "" || scheme == "file" || len(scheme) == 1) {
			continue
		}
		return true
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
			return errors.New("Office file contains a macro, ActiveX control, embedded object, or executable part; export a clean XLSX or have antivirus inspect it")
		}
		// DDE and OLE links may live inside xl/externalLinks/*.xml rather
		// than a .rels file. Inspect their XML without evaluating them.
		if strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".rels") || path == "[content_types].xml" {
			limit := int64(24 << 20)
			if strings.HasSuffix(path, ".rels") || path == "[content_types].xml" {
				limit = 2 << 20
			}
			if entry.UncompressedSize64 > uint64(limit) {
				return errors.New("Office XML part exceeds inspection limit")
			}
			rc, e := entry.Open()
			if e != nil {
				return errors.New("Cannot inspect Office relationships")
			}
			content, e := io.ReadAll(io.LimitReader(rc, limit+1))
			_ = rc.Close()
			if e != nil {
				return errors.New("Cannot inspect Office relationships")
			}
			if int64(len(content)) > limit {
				return errors.New("Office XML part exceeds inspection limit")
			}
			text := strings.ToLower(string(content))
			if (strings.HasSuffix(path, ".rels") && hasUnsafeExternalOfficeRelation(content)) || strings.Contains(text, "macroenabled") || strings.Contains(text, "vbaproject") || strings.Contains(text, "activex") || strings.Contains(text, "<ddelink") || strings.Contains(text, "<olelink") {
				return errors.New("Office file contains an executable Office relationship or macro link; export a clean XLSX")
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
		return errors.New("PDF includes JavaScript, a launch action, or active media; export a flattened PDF")
	}
	if format == "XLSX" || format == "DOCX" {
		if err := unsafeOffice(raw); err != nil {
			return err
		}
	}
	address := strings.TrimSpace(os.Getenv("AGIPAREL_CLAMD_ADDR"))
	if format == "XLS_LEGACY" && address == "" {
		// Only an explicitly configured development instance may retain an
		// unscanned legacy binary as immutable evidence. The extractor marks
		// XLS_LEGACY UNSUPPORTED and never opens or executes its contents.
		// Do not extend this bypass to production or to active XLSX payloads.
		if !strings.EqualFold(os.Getenv("ENV"), "development") {
			return errors.New("legacy .xls requires antivirus outside development")
		}
		// A CFB signature alone does not establish a readable workbook. Require
		// its minimum header plus the standard endian and sector-size fields.
		// Inspection here is structural only, NOT an antivirus assertion.
		if len(raw) < 512 || raw[28] != 0xfe || raw[29] != 0xff ||
			(binary.LittleEndian.Uint16(raw[30:32]) != 9 && binary.LittleEndian.Uint16(raw[30:32]) != 12) {
			return errors.New("legacy .xls is damaged or has an invalid compound-file header")
		}
		return nil
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
