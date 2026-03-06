package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/agent/session"
	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/permission"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestBootstrapResumeRestoresModelPermissionAndContext(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer os.Chdir(wd)

	t.Setenv("MSCLI_API_KEY", "test-key")

	storePath := filepath.Join(tempDir, ".mscli", "sessions")
	store, err := session.NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	mgr := session.NewManager(store, session.DefaultConfig())
	defer mgr.Close()

	sess, err := mgr.CreateAndSetCurrent("resume-target", tempDir)
	if err != nil {
		t.Fatalf("CreateAndSetCurrent failed: %v", err)
	}
	sess.AddMessage(llm.NewUserMessage("hello"))
	sess.AddMessage(llm.NewAssistantMessage("world"))
	sess.Runtime = session.RuntimeSnapshot{
		Model: session.ModelSnapshot{
			Model: "gpt-resume",
			URL:   "https://example.com/v1",
		},
		Permission: session.PermissionSnapshot{
			ToolPolicies: map[string]string{
				"shell": "deny",
			},
			CommandPolicies: map[string]string{
				"rm": "deny",
			},
			PathPolicies: []session.PathPolicySnapshot{
				{Pattern: "*.secret", Level: "deny"},
			},
		},
	}
	if err := mgr.Save(sess.ID); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	app, err := Bootstrap(BootstrapConfig{
		ResumeSessionID: string(sess.ID),
	})
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}
	defer func() {
		if c, ok := app.traceWriter.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		if app.sessionManager != nil {
			_ = app.sessionManager.Close()
		}
	}()

	if got := app.Config.Model.Model; got != "gpt-resume" {
		t.Fatalf("restored model = %s, want gpt-resume", got)
	}
	if got := len(app.ctxManager.GetNonSystemMessages()); got != 2 {
		t.Fatalf("restored messages = %d, want 2", got)
	}
	if got := len(app.initialUIMessages); got != 2 {
		t.Fatalf("initial ui messages = %d, want 2", got)
	}

	permSvc, ok := app.permService.(*permission.DefaultPermissionService)
	if !ok {
		t.Fatalf("permission service type = %T", app.permService)
	}
	if level := permSvc.Check("shell", ""); level != permission.PermissionDeny {
		t.Fatalf("shell permission = %s, want deny", level)
	}
	if level := permSvc.GetCommandPolicies()["rm"]; level != permission.PermissionDeny {
		t.Fatalf("rm command permission = %s, want deny", level)
	}
}

func TestCmdClearClearsContextAndSession(t *testing.T) {
	store, err := session.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	mgr := session.NewManager(store, session.DefaultConfig())
	defer mgr.Close()

	s, err := mgr.CreateAndSetCurrent("clear-test", t.TempDir())
	if err != nil {
		t.Fatalf("CreateAndSetCurrent failed: %v", err)
	}
	_ = mgr.AddMessageToCurrent(llm.NewUserMessage("hello"))

	ctx := context.NewManager(context.DefaultManagerConfig())
	_ = ctx.AddMessage(llm.NewUserMessage("hello"))

	app := &Application{
		EventCh:          make(chan model.Event, 4),
		ctxManager:       ctx,
		sessionManager:   mgr,
		currentSessionID: s.ID,
	}
	app.cmdClear()

	if got := len(ctx.GetNonSystemMessages()); got != 0 {
		t.Fatalf("context messages = %d, want 0", got)
	}
	msgs, err := mgr.GetMessages(s.ID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("session messages = %d, want 0", len(msgs))
	}
}

func TestBootstrapAllowsNoKeyAndFailsLazily(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer os.Chdir(wd)

	t.Setenv("MSCLI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	app, err := Bootstrap(BootstrapConfig{})
	if err != nil {
		t.Fatalf("Bootstrap failed without key: %v", err)
	}
	defer func() {
		if c, ok := app.traceWriter.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		if app.sessionManager != nil {
			_ = app.sessionManager.Close()
		}
	}()

	_, err = app.Engine.Run(loop.Task{
		ID:          "no-key",
		Description: "say hello",
	})
	if err == nil {
		t.Fatal("expected error when running task without api key")
	}
	if !strings.Contains(err.Error(), "API key is not configured") {
		t.Fatalf("error = %v, want missing key guidance", err)
	}
}
