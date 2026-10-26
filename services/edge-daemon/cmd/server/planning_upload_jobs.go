// Author: Sunil Jain, 2026-10-25
// Asynchronous, per-request document intake. Keeps slow AI extraction off the HTTP request.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type intakeJob struct {
	ID        string                   `json:"job_id"`
	Status    string                   `json:"status"`
	Files     []string                 `json:"files"`
	Message   string                   `json:"message,omitempty"`
	Conflicts []map[string]interface{} `json:"conflicts,omitempty"`
	Created   time.Time                `json:"created_at"`
}

var intakeJobs = struct {
	sync.Mutex
	jobs map[string]intakeJob
}{jobs: make(map[string]intakeJob)}

func newIntakeJobID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func storeIntakeJob(j intakeJob) {
	intakeJobs.Lock()
	defer intakeJobs.Unlock()
	intakeJobs.jobs[j.ID] = j
}

func readIntakeJob(id string) (intakeJob, bool) {
	intakeJobs.Lock()
	defer intakeJobs.Unlock()
	j, ok := intakeJobs.jobs[id]
	return j, ok
}

// GET ?job=<id> returns processing status; POST files=<multipart files> queues work.
func (s *EdgeServer) handlePlanningUploadJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("job")
		if len(id) != 32 {
			http.Error(w, "Invalid job ID", http.StatusBadRequest)
			return
		}
		j, ok := readIntakeJob(id)
		if !ok {
			http.Error(w, "Upload job not found (jobs expire on restart)", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(j)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil || os.Getenv("DATABASE_URL") == "" {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "Invalid upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "Choose one or more files", http.StatusBadRequest)
		return
	}
	if len(files) > 15 {
		http.Error(w, "Upload at most 15 documents at a time", http.StatusBadRequest)
		return
	}
	id, err := newIntakeJobID()
	if err != nil {
		http.Error(w, "Could not create upload ID", http.StatusInternalServerError)
		return
	}
	jobFolder := filepath.Join("data", "staging", "intake-jobs", id)
	inputFolder := filepath.Join(jobFolder, "docs")
	if err := os.MkdirAll(inputFolder, 0700); err != nil {
		http.Error(w, "Could not create upload folder", http.StatusInternalServerError)
		return
	}
	names := make([]string, 0, len(files))
	seen := make(map[string]bool)
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(jobFolder)
		}
	}()
	for _, h := range files {
		if h.Size > 20<<20 {
			http.Error(w, "One file exceeds 20 MB", http.StatusRequestEntityTooLarge)
			return
		}
		name := filepath.Base(h.Filename)
		if name == "" || name == "." || name == ".." || strings.ContainsAny(h.Filename, "/\\") || seen[name] {
			http.Error(w, "Invalid or repeated file name", http.StatusBadRequest)
			return
		}
		seen[name] = true
		switch strings.ToLower(filepath.Ext(name)) {
		case ".csv", ".xlsx", ".pdf", ".png", ".jpg", ".jpeg", ".webp":
		default:
			http.Error(w, "Unsupported type: "+filepath.Ext(name), http.StatusUnsupportedMediaType)
			return
		}
		src, err := h.Open()
		if err != nil {
			http.Error(w, "Could not read upload", http.StatusBadRequest)
			return
		}
		dst, err := os.OpenFile(filepath.Join(inputFolder, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			src.Close()
			http.Error(w, "Could not save upload", http.StatusInternalServerError)
			return
		}
		n, copyErr := io.Copy(dst, io.LimitReader(src, (20<<20)+1))
		closeErr := dst.Close()
		_ = src.Close()
		if copyErr != nil || closeErr != nil || n > 20<<20 {
			http.Error(w, "Could not save complete document", http.StatusBadRequest)
			return
		}
		names = append(names, name)
	}
	job := intakeJob{ID: id, Status: "QUEUED", Files: names, Message: "Files received. Checking documents and factory records.", Created: time.Now().UTC()}
	storeIntakeJob(job)
	cleanup = false
	go s.runPlanningIntakeJob(job, inputFolder)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(job)
}

func (s *EdgeServer) runPlanningIntakeJob(job intakeJob, inputFolder string) {
	// The ingester writes a common cumulative workbook/manifest; serialize executions.
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	job.Status = "PROCESSING"
	job.Message = "Reading files and checking extracted details."
	storeIntakeJob(job)
	defer func() {
		if job.Status == "FAILED" {
			log.Printf("[INTAKE] Job %s failed: %s", job.ID, job.Message)
		}
		if job.Status == "COMPLETE" {
			_ = os.RemoveAll(filepath.Dir(inputFolder))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	host := strings.TrimRight(os.Getenv("OLLAMA_HOST"), "/")
	if host == "" {
		host = "http://host.docker.internal:11434"
	}
	model := os.Getenv("AGIE_MODEL")
	if model == "" {
		model = "qwen3-coder:30b"
	}
	// Do not use -force: older staged files and already committed factory data
	// must not be reprocessed merely because someone adds one new document.
	args := []string{"-apply-db=true", "-db-url", os.Getenv("DATABASE_URL"), "-staging-dir", inputFolder, "-output-dir", "./data/staging/output", "-ollama-host", host, "-model", model}
	captureOnly := os.Getenv("AGIPAREL_CAPTURE_FIRST") == "true"
	if captureOnly {
		args = append(args, "-capture-first=true")
	}
	command := exec.CommandContext(ctx, "/app/ingest-docs", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		job.Status = "FAILED"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			job.Message = "Document processing took too long. Try fewer or smaller files."
		} else {
			job.Message = "Document processing failed: " + intakeFailureSummary(string(output))
		}
		storeIntakeJob(job)
		return
	}
	// Keep originals as separately named evidence; never overwrite old uploads.
	archiveFolder := "./data/staging/docs"
	if err = os.MkdirAll(archiveFolder, 0700); err != nil {
		job.Status = "FAILED"
		job.Message = "Extraction completed but originals could not be archived"
		storeIntakeJob(job)
		return
	}
	for _, name := range job.Files {
		source := filepath.Join(inputFolder, name)
		target := filepath.Join(archiveFolder, fmt.Sprintf("%s_%s", job.ID, name))
		if e := copyIntakeFile(source, target); e != nil {
			job.Status = "FAILED"
			job.Message = "Document archive failed: " + name
			storeIntakeJob(job)
			return
		}
	}
	var conflicts []map[string]interface{}
	if data, e := os.ReadFile("./data/staging/output/conflicts.json"); e == nil {
		_ = json.Unmarshal(data, &conflicts)
	}
	job.Conflicts = conflicts
	job.Status = "COMPLETE"
	if captureOnly {
		job.Message = "Originals and proposed facts saved. Review and approve facts before planning; factory operations were not changed."
	} else {
		job.Message = "Documents processed into the existing factory workbook. Review changes and conflicts before approving a plan."
	}
	storeIntakeJob(job)
	log.Printf("[INTAKE] Job %s completed: %s", job.ID, strings.Join(job.Files, ", "))
}

func copyIntakeFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
