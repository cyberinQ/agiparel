// Author: Sunil Jain, 2026-10-28 — Test development-only XLS retention and active-content rejection.
// Test document preflight allowing inert garment evidence while rejecting executable payloads.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func makeOffice(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, data := range entries {
		w, e := zw.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(data)); e != nil {
			t.Fatal(e)
		}
	}
	if e := zw.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}

func TestDocumentSafetyPreflight(t *testing.T) {
	ctx := context.Background()
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_CLAMD_ADDR", "")
	xlsx := makeOffice(t, map[string]string{"xl/workbook.xml": "<workbook/>", "xl/worksheets/sheet1.xml": "<sheet/>"})
	docx := makeOffice(t, map[string]string{"word/document.xml": "<document/>"})
	macro := makeOffice(t, map[string]string{"xl/workbook.xml": "<workbook/>", "xl/vbaProject.bin": "macro"})
	link := makeOffice(t, map[string]string{"word/document.xml": "<document/>", "word/_rels/document.xml.rels": `<Relationships><Relationship TargetMode = "External" Target="https://example.org/size-chart"/></Relationships>`})
	workbookLink := makeOffice(t, map[string]string{"xl/workbook.xml": "<workbook/>", "xl/externalLinks/externalLink1.xml": "<externalLink/>", "xl/_rels/workbook.xml.rels": `<Relationships><Relationship TargetMode="External" Target="https://example.org/catalog"/></Relationships>`})
	localLink := makeOffice(t, map[string]string{"xl/workbook.xml": "<workbook/>", "xl/_rels/workbook.xml.rels": `<Relationships><Relationship TargetMode="External" Target="file:///etc/passwd" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"/></Relationships>`})
	localExcelBook := makeOffice(t, map[string]string{"xl/workbook.xml": "<workbook/>", "xl/externalLinks/externalLink1.xml": "<externalLink><externalBook/></externalLink>", "xl/externalLinks/_rels/externalLink1.xml.rels": `<Relationships><Relationship TargetMode="External" Target="file:///C:/supplier/measurements.xlsx" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/externalLinkPath"/></Relationships>`})
	activeTemplate := makeOffice(t, map[string]string{"word/document.xml": "<document/>", "word/_rels/document.xml.rels": `<Relationships><Relationship TargetMode="External" Target="https://example.org/template.dotm" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/attachedTemplate"/></Relationships>`})
	activeDDE := makeOffice(t, map[string]string{"xl/workbook.xml": "<workbook/>", "xl/externalLinks/externalLink1.xml": "<externalLink><ddeLink/></externalLink>"})
	legacy := make([]byte, 512)
	copy(legacy, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	legacy[28] = 0xfe
	legacy[29] = 0xff
	binary.LittleEndian.PutUint16(legacy[30:32], 9)
	// Magic-byte valid PNG, not a content classifier: swimsuit and underwear photography are permitted.
	png := append([]byte{137, 80, 78, 71, 13, 10, 26, 10}, bytes.Repeat([]byte{0}, 24)...)
	for _, x := range []struct {
		name    string
		data    []byte
		allowed bool
	}{
		{"sizes.csv", []byte("material_sku,rate\nCOTTON,1.5\n"), true},
		{"buyer tech pack.pdf", []byte("%PDF-1.4\n1 0 obj <</Type /Catalog>> endobj\n%%EOF"), true},
		{"ordinary-catalog.pdf", []byte("%PDF-1.7\n1 0 obj <</OpenAction [2 0 R /Fit] /AA <</O <</S /GoTo /D [2 0 R /Fit]>>>>>> endobj\n%%EOF"), true},
		{"swimwear.png", png, true},
		{"machines.xlsx", xlsx, true},
		{"buyer.docx", docx, true},
		{"macros.xlsx", macro, false},
		{"spoofed.xlsm", macro, false},
		{"external.docx", link, true},
		{"linked-workbook.xlsx", workbookLink, true},
		{"linked-local-workbook.xlsx", localExcelBook, true},
		{"active-template.docx", activeTemplate, false},
		{"dangerous-dde.xlsx", activeDDE, false},
		{"local-file-link.xlsx", localLink, false},
		{"active.pdf", []byte("%PDF-1.5\n<</OpenAction <</S /J#53>>>>"), false},
		{"active2.pdf", []byte("%PDF-1.4\n<</JavaScript 1 0 R>>"), false},
		{"static-acroform.pdf", []byte("%PDF-1.7\n<</AcroForm <</XFA 9 0 R>> /SubmitForm (Printed instructions) >>"), true},
		{"active-submit.pdf", []byte("%PDF-1.7\n<</S /SubmitForm /F (https://example.com)>>"), false},
		{"active-import.pdf", []byte("%PDF-1.7\n<</S /ImportData /F (file:///etc/passwd)>>"), false},
		{"tech-pack-attachments.pdf", []byte("%PDF-1.7\n<</Names <</EmbeddedFiles 5 0 R>> /EF 1 0 R>>"), true},
		{"stream-false-positive.pdf", []byte("%PDF-1.7\n<</Length 19>>stream\n/JS /XFA /JavaScript\nendstream"), true},
		{"program.exe", []byte("MZabcdef"), false},
		{"disguised.png", xlsx, false},
		{"unknown.svg", []byte(`<svg onload="alert(1)"></svg>`), false},
		{"old.xls", legacy, true}, // Storage only in development; worker marks unsupported.
		{"truncated.xls", legacy[:8], false},
	} {
		e := checkDocumentSafety(ctx, x.name, x.data)
		if (e == nil) != x.allowed {
			t.Errorf("%s: allowed=%t err=%v", x.name, x.allowed, e)
		}
	}
}

// Legacy binary workbooks can be retained only in explicit development mode.
// Production and unset environments must not bypass antivirus.
func TestLegacyXLSStorageOnlyDevelopmentGate(t *testing.T) {
	legacy := make([]byte, 512)
	copy(legacy, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	legacy[28] = 0xfe
	legacy[29] = 0xff
	binary.LittleEndian.PutUint16(legacy[30:32], 9)
	t.Setenv("AGIPAREL_CLAMD_ADDR", "")
	for _, mode := range []struct {
		env     string
		allowed bool
	}{
		{"development", true}, {"production", false}, {"", false}, {"staging", false},
	} {
		t.Setenv("ENV", mode.env)
		err := checkDocumentSafety(context.Background(), "sample.xls", legacy)
		if (err == nil) != mode.allowed {
			t.Errorf("ENV=%q allowed=%t err=%v", mode.env, mode.allowed, err)
		}
	}
}

func TestProductionUploadsRequireAntivirus(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("AGIPAREL_CLAMD_ADDR", "")
	if err := checkDocumentSafety(context.Background(), "a.csv", []byte("a,b\n1,2")); err == nil || !strings.Contains(err.Error(), "antivirus") {
		t.Fatalf("production must fail closed: %v", err)
	}
}

func TestClamAVStreamProtocol(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_CLAMD_ADDR", ln.Addr().String())
	done := make(chan error, 1)
	go func() {
		c, e := ln.Accept()
		if e != nil {
			done <- e
			return
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		start := make([]byte, 10)
		_, e = io.ReadFull(c, start)
		if e != nil {
			done <- e
			return
		}
		if string(start) != "zINSTREAM\x00" {
			done <- io.ErrUnexpectedEOF
			return
		}
		var body bytes.Buffer
		for {
			var header [4]byte
			_, e = io.ReadFull(c, header[:])
			if e != nil {
				done <- e
				return
			}
			n := binary.BigEndian.Uint32(header[:])
			if n == 0 {
				break
			}
			_, e = io.CopyN(&body, c, int64(n))
			if e != nil {
				done <- e
				return
			}
		}
		if string(body.Bytes()) != "item,qty\nTHREAD,1\n" {
			done <- io.ErrUnexpectedEOF
			return
		}
		_, e = c.Write([]byte("stream: OK\x00"))
		done <- e
	}()
	if e := checkDocumentSafety(context.Background(), "items.csv", []byte("item,qty\nTHREAD,1\n")); e != nil {
		t.Fatal(e)
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}

func TestClamAVUnreachableFailsClosed(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_CLAMD_ADDR", "127.0.0.1:1")
	if e := checkDocumentSafety(context.Background(), "a.csv", []byte("a,b\n1,2")); e == nil {
		t.Fatal("scanner outage must reject")
	}
}

func TestClamAVDetectedMalwareRejected(t *testing.T) {
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_CLAMD_ADDR", ln.Addr().String())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		header := make([]byte, 10)
		if _, err = io.ReadFull(c, header); err != nil {
			return
		}
		for {
			var block [4]byte
			if _, err = io.ReadFull(c, block[:]); err != nil {
				return
			}
			n := binary.BigEndian.Uint32(block[:])
			if n == 0 {
				break
			}
			if _, err = io.CopyN(io.Discard, c, int64(n)); err != nil {
				return
			}
		}
		_, _ = c.Write([]byte("stream: Eicar-Test-Signature FOUND\x00"))
	}()
	e = checkDocumentSafety(context.Background(), "report.csv", []byte("item,qty\nTHREAD,1\n"))
	<-done
	if e == nil || !strings.Contains(e.Error(), "malware detected") {
		t.Fatalf("infected file must be rejected: %v", e)
	}
}

// OpenAction commonly selects a starting page; it is not itself JavaScript.
func TestPDFNavigationVersusActivePayloads(t *testing.T) {
	for _, tc := range []struct {
		name, pdf string
		blocked   bool
	}{
		{"page-navigation", "%PDF-1.7\n<</OpenAction [3 0 R /Fit]>>", false},
		{"js-escaped", "%PDF-1.7\n<</OpenAction <</S /J#53>>>>", true},
		{"embedded-file", "%PDF-1.7\n<</Names <</EmbeddedFiles 5 0 R>>>>", false},
		{"launch", "%PDF-1.7\n<</S /Launch>>", true},
		{"metadata-string", "%PDF-1.7\n<</Title (Code /JS not executable) >>", false},
		{"font-stream", "%PDF-1.7\n<</Length 21>>\nstream\n/JavaScript /JS /XFA\nendstream", false},
	} {
		if got := unsafePDF([]byte(tc.pdf)); got != tc.blocked {
			t.Errorf("%s: blocked=%v, want %v", tc.name, got, tc.blocked)
		}
	}
}
