package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadLegacySessionWithoutRuntime(t *testing.T) {
	base := t.TempDir()
	store, err := NewFileStore(base)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	const id = ID("sess_legacy")
	legacyJSON := `{
  "ID": "sess_legacy",
  "Name": "legacy",
  "WorkDir": "/tmp",
  "Messages": [{"role":"user","content":"hello"}],
  "Metadata": {"MessageCount": 1},
  "CreatedAt": "2026-03-05T00:00:00Z",
  "UpdatedAt": "2026-03-05T00:00:00Z",
  "Archived": false
}`
	if err := os.WriteFile(filepath.Join(base, string(id)+".json"), []byte(legacyJSON), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	s, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s.Name != "legacy" {
		t.Fatalf("session name = %s, want legacy", s.Name)
	}

	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()
	if _, err := mgr.Load(id); err != nil {
		t.Fatalf("manager load failed: %v", err)
	}
	if err := mgr.UpdateCurrentPermission(PermissionSnapshot{}); err != nil {
		t.Fatalf("UpdateCurrentPermission failed: %v", err)
	}
}

func TestSessionTraceWriterAppendOnResume(t *testing.T) {
	base := t.TempDir()
	id := ID("sess_resume")
	path := TracePathForSession(base, id)

	w1, err := NewSessionTraceWriter(base, id)
	if err != nil {
		t.Fatalf("NewSessionTraceWriter(1) failed: %v", err)
	}
	if err := w1.Write("event_one", map[string]any{"n": 1}); err != nil {
		t.Fatalf("Write(1) failed: %v", err)
	}
	_ = w1.Close()

	time.Sleep(10 * time.Millisecond)

	w2, err := NewSessionTraceWriter(base, id)
	if err != nil {
		t.Fatalf("NewSessionTraceWriter(2) failed: %v", err)
	}
	if err := w2.Write("event_two", map[string]any{"n": 2}); err != nil {
		t.Fatalf("Write(2) failed: %v", err)
	}
	_ = w2.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("trace lines = %d, want 2", len(lines))
	}
}
