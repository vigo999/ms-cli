package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Writer writes structured runtime events.
type Writer interface {
	Write(eventType string, payload any) error
}

// Record is one trace event persisted to disk.
type Record struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Payload   any       `json:"payload,omitempty"`
}

// FileWriter writes JSONL trace records to disk.
type FileWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
	enc  *json.Encoder
}

// NewFileWriter creates a writer that appends to a JSONL file.
func NewFileWriter(path string) (*FileWriter, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("trace path cannot be empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create trace directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}

	return &FileWriter{
		path: path,
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

// NewTimestampWriter creates a writer under cacheDir with timestamp filename.
func NewTimestampWriter(cacheDir string) (*FileWriter, error) {
	if strings.TrimSpace(cacheDir) == "" {
		return nil, fmt.Errorf("cache directory cannot be empty")
	}

	filename := fmt.Sprintf("%s.trajectory.jsonl", time.Now().Format("20060102-150405.000000000"))
	return NewFileWriter(filepath.Join(cacheDir, filename))
}

// Write appends one trace record and syncs it to disk immediately.
func (w *FileWriter) Write(eventType string, payload any) error {
	if w == nil {
		return fmt.Errorf("trace writer is nil")
	}
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("event type cannot be empty")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	record := Record{
		Timestamp: time.Now(),
		Type:      eventType,
		Payload:   payload,
	}

	if err := w.enc.Encode(record); err != nil {
		return fmt.Errorf("encode trace record: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync trace file: %w", err)
	}
	return nil
}

// Close closes the underlying file.
func (w *FileWriter) Close() error {
	if w == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close trace file: %w", err)
	}
	w.file = nil
	return nil
}

// Path returns the output file path.
func (w *FileWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}
