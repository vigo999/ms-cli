package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vigo999/ms-cli/agent/session"
	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestParseCLIArgsRunDefault(t *testing.T) {
	opts, err := parseCLIArgs(nil)
	if err != nil {
		t.Fatalf("parseCLIArgs failed: %v", err)
	}
	if opts.Command != "run" {
		t.Fatalf("command = %s, want run", opts.Command)
	}
}

func TestParseCLIArgsResume(t *testing.T) {
	opts, err := parseCLIArgs([]string{"resume", "sess_1", "-model", "gpt-4o"})
	if err != nil {
		t.Fatalf("parseCLIArgs failed: %v", err)
	}
	if opts.Command != "resume" {
		t.Fatalf("command = %s, want resume", opts.Command)
	}
	if opts.Bootstrap.ResumeSessionID != "sess_1" {
		t.Fatalf("resume id = %s, want sess_1", opts.Bootstrap.ResumeSessionID)
	}
	if opts.Bootstrap.Model != "gpt-4o" {
		t.Fatalf("model = %s, want gpt-4o", opts.Bootstrap.Model)
	}
}

func TestParseCLIArgsResumeMissingID(t *testing.T) {
	_, err := parseCLIArgs([]string{"resume"})
	if err == nil {
		t.Fatalf("expected error for missing resume id")
	}
}

func TestParseCLIArgsSessionsList(t *testing.T) {
	opts, err := parseCLIArgs([]string{"sessions", "list"})
	if err != nil {
		t.Fatalf("parseCLIArgs failed: %v", err)
	}
	if opts.Command != "sessions" {
		t.Fatalf("command = %s, want sessions", opts.Command)
	}
}

func TestParseCLIArgsUnknownCommand(t *testing.T) {
	_, err := parseCLIArgs([]string{"unknown"})
	if err == nil {
		t.Fatalf("expected error for unknown command")
	}
}

func TestRunSessionsList(t *testing.T) {
	workDir := t.TempDir()
	storePath := filepath.Join(workDir, ".mscli", "sessions")
	store, err := session.NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	mgr := session.NewManager(store, session.DefaultConfig())
	defer mgr.Close()

	first, err := mgr.CreateAndSetCurrent("first", workDir)
	if err != nil {
		t.Fatalf("create first failed: %v", err)
	}
	first.AddMessage(sessionMessage("user", "hello"))
	if err := mgr.Save(first.ID); err != nil {
		t.Fatalf("save first failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	second, err := mgr.CreateAndSetCurrent("second", workDir)
	if err != nil {
		t.Fatalf("create second failed: %v", err)
	}
	second.AddMessage(sessionMessage("user", "newer"))
	if err := mgr.Save(second.ID); err != nil {
		t.Fatalf("save second failed: %v", err)
	}

	var buf bytes.Buffer
	if err := runSessionsList(workDir, &buf); err != nil {
		t.Fatalf("runSessionsList failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Fatalf("output missing header: %s", out)
	}
	if strings.Contains(strings.SplitN(out, "\n", 2)[0], "Name") {
		t.Fatalf("header should not contain Name column: %s", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		t.Fatalf("output lines too short: %q", out)
	}
	if !strings.Contains(lines[1], string(second.ID)) {
		t.Fatalf("expected newest session first, got: %s", lines[1])
	}
}

func sessionMessage(role, content string) llm.Message {
	return llm.Message{
		Role:    role,
		Content: content,
	}
}
