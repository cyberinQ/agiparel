package wal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Record struct {
	Index     uint64          `json:"index"`
	Timestamp int64           `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type LocalWAL struct {
	mu           sync.Mutex
	file         *os.File
	path         string
	currentIndex uint64
}

func Open(filePath string) (*LocalWAL, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create wal directory: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open wal file: %w", err)
	}

	wal := &LocalWAL{
		file: f,
		path: filePath,
	}

	records, err := wal.ReplayAll()
	if err == nil {
		wal.currentIndex = uint64(len(records))
	}

	return wal, nil
}

func (w *LocalWAL) Append(eventType string, payload json.RawMessage) (Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.currentIndex++
	rec := Record{
		Index:     w.currentIndex,
		Timestamp: time.Now().UTC().UnixMilli(),
		Type:      eventType,
		Payload:   payload,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return rec, fmt.Errorf("failed to marshal wal record: %w", err)
	}

	writer := bufio.NewWriter(w.file)
	if _, err := writer.Write(append(data, '\n')); err != nil {
		return rec, fmt.Errorf("failed to write wal record: %w", err)
	}

	if err := writer.Flush(); err != nil {
		return rec, fmt.Errorf("failed to flush wal buffer: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return rec, fmt.Errorf("failed to sync wal file to storage: %w", err)
	}

	return rec, nil
}

func (w *LocalWAL) ReplayAll() ([]Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek wal file: %w", err)
	}

	var records []Record
	scanner := bufio.NewScanner(w.file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal wal record: %w", err)
		}
		records = append(records, rec)
	}

	return records, scanner.Err()
}

func (w *LocalWAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
