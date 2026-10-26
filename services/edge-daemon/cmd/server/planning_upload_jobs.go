// Legacy Planning upload is retired. General source intake uses /documents.
// Retain read-only lookup for old in-memory job identifiers until restart.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
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
			http.Error(w, "Old upload job not available; use /documents", http.StatusNotFound)
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
	w.Header().Set("Link", "</documents>; rel=alternate")
	http.Error(w, "Legacy Planning file upload retired: use /documents. No factory records were changed.", http.StatusGone)
}
