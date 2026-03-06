package main

import (
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/permission"
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/ui/model"
)

func TestCmdModelKeyUpdatesProvider(t *testing.T) {
	cfg := configs.DefaultConfig()
	cfg.Model.Key = ""

	app := &Application{
		EventCh:      make(chan model.Event, 8),
		Config:       cfg,
		toolRegistry: tools.NewRegistry(),
		ctxManager:   context.NewManager(context.DefaultManagerConfig()),
		permService:  permission.NewNoOpPermissionService(),
	}

	app.cmdModel([]string{"key", "sk-test"})

	if app.Config.Model.Key != "sk-test" {
		t.Fatalf("key = %s, want sk-test", app.Config.Model.Key)
	}

	events := drainEvents(app.EventCh)
	for _, ev := range events {
		if ev.Type == model.ToolError {
			t.Fatalf("unexpected ToolError event: %s", ev.Message)
		}
	}

	if !containsMessage(events, "API key updated") {
		t.Fatalf("events do not include success message: %#v", events)
	}
}

func TestCmdModelAnthropicPrefixSwitchesProtocolAndDefaultURL(t *testing.T) {
	cfg := configs.DefaultConfig()
	cfg.Model.Protocol = configs.ProtocolOpenAI
	cfg.Model.URL = defaultOpenAIURL
	cfg.Model.Key = "test-key"

	app := &Application{
		EventCh:      make(chan model.Event, 8),
		Config:       cfg,
		toolRegistry: tools.NewRegistry(),
		ctxManager:   context.NewManager(context.DefaultManagerConfig()),
		permService:  permission.NewNoOpPermissionService(),
	}

	app.cmdModel([]string{"anthropic:claude-3-5-sonnet-latest"})

	if app.Config.Model.Protocol != configs.ProtocolAnthropic {
		t.Fatalf("protocol = %s, want anthropic", app.Config.Model.Protocol)
	}
	if app.Config.Model.Model != "claude-3-5-sonnet-latest" {
		t.Fatalf("model = %s, want claude-3-5-sonnet-latest", app.Config.Model.Model)
	}
	if app.Config.Model.URL != defaultAnthropicURL {
		t.Fatalf("url = %s, want %s", app.Config.Model.URL, defaultAnthropicURL)
	}
}

func TestCmdModelRejectsUnsupportedProviderPrefix(t *testing.T) {
	cfg := configs.DefaultConfig()

	app := &Application{
		EventCh: make(chan model.Event, 4),
		Config:  cfg,
	}

	app.cmdModel([]string{"foo:bar"})

	events := drainEvents(app.EventCh)
	if !containsMessage(events, "Unsupported provider prefix") {
		t.Fatalf("expected unsupported provider error, events = %#v", events)
	}
}

func drainEvents(ch <-chan model.Event) []model.Event {
	out := make([]model.Event, 0)
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		default:
			return out
		}
	}
}

func containsMessage(events []model.Event, expected string) bool {
	for _, ev := range events {
		if strings.Contains(ev.Message, expected) {
			return true
		}
	}
	return false
}
