package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSessionTraceWriter(t *testing.T) {
	base := t.TempDir()
	id := ID("sess_123")

	w, err := NewSessionTraceWriter(base, id)
	if err != nil {
		t.Fatalf("NewSessionTraceWriter failed: %v", err)
	}
	defer w.Close()

	want := filepath.Join(base, "sess_123.trajectory.jsonl")
	if got := w.Path(); got != want {
		t.Fatalf("writer path = %s, want %s", got, want)
	}
}

func TestEventWriterWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := NewEventWriter(path)
	if err != nil {
		t.Fatalf("NewEventWriter failed: %v", err)
	}
	defer w.Close()

	if err := w.Write("run_started", map[string]any{"task": "test"}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var rec EventRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if rec.Type != "run_started" {
		t.Fatalf("record type = %s, want run_started", rec.Type)
	}
}
