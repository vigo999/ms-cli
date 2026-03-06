package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventRecord is one trace event persisted to disk.
type EventRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Payload   any       `json:"payload,omitempty"`
}

// EventWriter writes JSONL event records to disk.
type EventWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
	enc  *json.Encoder
}

// NewEventWriter creates a writer that appends to a JSONL file.
func NewEventWriter(path string) (*EventWriter, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("event path cannot be empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create event directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open event file: %w", err)
	}

	return &EventWriter{
		path: path,
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

// TracePathForSession builds the canonical trace path for a session.
func TracePathForSession(storePath string, id ID) string {
	return filepath.Join(storePath, fmt.Sprintf("%s.trajectory.jsonl", id))
}

// NewSessionTraceWriter creates a writer for a specific session ID.
func NewSessionTraceWriter(storePath string, id ID) (*EventWriter, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, fmt.Errorf("session id cannot be empty")
	}
	return NewEventWriter(TracePathForSession(storePath, id))
}

// Write appends one event record and syncs it to disk immediately.
func (w *EventWriter) Write(eventType string, payload any) error {
	if w == nil {
		return fmt.Errorf("event writer is nil")
	}
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("event type cannot be empty")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	record := EventRecord{
		Timestamp: time.Now(),
		Type:      eventType,
		Payload:   payload,
	}

	if err := w.enc.Encode(record); err != nil {
		return fmt.Errorf("encode event record: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync event file: %w", err)
	}
	return nil
}

// Close closes the underlying file.
func (w *EventWriter) Close() error {
	if w == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close event file: %w", err)
	}
	w.file = nil
	return nil
}

// Path returns the output file path.
func (w *EventWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}
