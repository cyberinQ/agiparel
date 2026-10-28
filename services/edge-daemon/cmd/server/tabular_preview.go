// Author: Sunil Jain, 2026-10-28
// Read a simple cutting worksheet for operator review; never stages or approves a cut.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var cuttingSheetHeaders = map[string][]string{
	"marker_positions":  {"position", "color_code", "size_code"},
	"pattern_parts":     {"part_code", "pieces_per_garment"},
	"issue_allocations": {"issue_id", "used_mm"},
}

func (s *EdgeServer) handleTabularPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if e := r.ParseMultipartForm(8 << 20); e != nil {
		http.Error(w, "Spreadsheet upload is too large or incomplete", 400)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	headers, ok := cuttingSheetHeaders[r.FormValue("kind")]
	if !ok {
		http.Error(w, "Unknown worksheet type", 400)
		return
	}
	if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		http.Error(w, "Choose exactly one worksheet", 400)
		return
	}
	src, e := readSourceTable(r.MultipartForm.File["file"][0], headers, false)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	rows, e := parseCSV(src.CSV, headers)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		for _, value := range row {
			if strings.ContainsAny(value, ",\r\n") || strings.TrimSpace(value) == "" {
				e = errors.New("blank or comma-containing cells are not allowed")
				break
			}
		}
		if e != nil {
			break
		}
		lines = append(lines, strings.Join(row, ","))
		if len(lines) > intakeMaxRows {
			e = fmt.Errorf("too many worksheet rows (%d)", i+1)
			break
		}
	}
	if e != nil {
		http.Error(w, fmt.Sprintf("Cannot use worksheet: %v", e), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(struct {
		Lines    []string `json:"lines"`
		Filename string   `json:"filename"`
	}{lines, src.Name})
}
