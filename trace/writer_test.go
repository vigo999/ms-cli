package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestNewTimestampWriter(t *testing.T) {
	cacheDir := t.TempDir()

	w, err := NewTimestampWriter(cacheDir)
	if err != nil {
		t.Fatalf("NewTimestampWriter failed: %v", err)
	}
	defer w.Close()

	path := w.Path()
	if filepath.Dir(path) != cacheDir {
		t.Fatalf("writer path dir = %s, want %s", filepath.Dir(path), cacheDir)
	}

	filename := filepath.Base(path)
	pattern := regexp.MustCompile(`^\d{8}-\d{6}\.\d{9}\.trajectory\.jsonl$`)
	if !pattern.MatchString(filename) {
		t.Fatalf("filename %q does not match expected timestamp pattern", filename)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("trace file should exist: %v", err)
	}
}

func TestWritePersistsImmediately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")

	w, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter failed: %v", err)
	}
	defer w.Close()

	payload := map[string]any{
		"task": "test",
		"step": 1,
	}
	if err := w.Write("event", payload); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if record.Type != "event" {
		t.Fatalf("record type = %q, want %q", record.Type, "event")
	}
	if record.Timestamp.IsZero() {
		t.Fatalf("record timestamp should not be zero")
	}

	gotPayload, ok := record.Payload.(map[string]any)
	if !ok {
		t.Fatalf("payload should be map[string]any, got %T", record.Payload)
	}
	if gotPayload["task"] != "test" {
		t.Fatalf("payload task = %v, want %v", gotPayload["task"], "test")
	}
	if gotPayload["step"] != float64(1) {
		t.Fatalf("payload step = %v, want %v", gotPayload["step"], 1)
	}
}
