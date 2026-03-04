package loop

import (
	"context"
	"fmt"
	"testing"

	ctxmanager "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

type captureProvider struct {
	lastReq *llm.CompletionRequest
}

func (p *captureProvider) Name() string {
	return "capture"
}

func (p *captureProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	copied := *req
	copied.Messages = append([]llm.Message(nil), req.Messages...)
	copied.Tools = append([]llm.Tool(nil), req.Tools...)
	p.lastReq = &copied

	return &llm.CompletionResponse{
		Content:      "ok",
		FinishReason: llm.FinishStop,
	}, nil
}

func (p *captureProvider) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *captureProvider) SupportsTools() bool {
	return true
}

func (p *captureProvider) AvailableModels() []llm.ModelInfo {
	return nil
}

func newEngineForContextTests(provider llm.Provider) *Engine {
	return NewEngine(EngineConfig{
		MaxIterations: 1,
		MaxTokens:     8000,
	}, provider, tools.NewRegistry())
}

func TestSetContextManagerPreservesSystemPrompt(t *testing.T) {
	engine := newEngineForContextTests(&captureProvider{})

	replacement := ctxmanager.NewManager(ctxmanager.ManagerConfig{
		MaxTokens:     8000,
		ReserveTokens: 4000,
	})
	if replacement.GetSystemPrompt() != nil {
		t.Fatal("replacement context manager should start without system prompt")
	}

	engine.SetContextManager(replacement)

	system := replacement.GetSystemPrompt()
	if system == nil {
		t.Fatal("expected system prompt to be preserved on context manager swap")
	}
	if system.Content != defaultSystemPrompt() {
		t.Fatalf("expected preserved system prompt to match default, got %q", system.Content)
	}
}

func TestSetContextManagerKeepsExistingSystemPrompt(t *testing.T) {
	engine := newEngineForContextTests(&captureProvider{})

	replacement := ctxmanager.NewManager(ctxmanager.ManagerConfig{
		MaxTokens:     8000,
		ReserveTokens: 4000,
	})
	const customPrompt = "custom system prompt"
	replacement.SetSystemPrompt(customPrompt)

	engine.SetContextManager(replacement)

	system := replacement.GetSystemPrompt()
	if system == nil {
		t.Fatal("expected replacement system prompt to remain set")
	}
	if system.Content != customPrompt {
		t.Fatalf("expected custom system prompt %q, got %q", customPrompt, system.Content)
	}
}

func TestRunUsesSystemPromptAfterContextManagerSwap(t *testing.T) {
	provider := &captureProvider{}
	engine := newEngineForContextTests(provider)

	replacement := ctxmanager.NewManager(ctxmanager.ManagerConfig{
		MaxTokens:     8000,
		ReserveTokens: 4000,
	})
	engine.SetContextManager(replacement)

	_, err := engine.Run(Task{
		ID:          "task-context-swap",
		Description: "say hello",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if provider.lastReq == nil {
		t.Fatal("expected provider to receive completion request")
	}
	if len(provider.lastReq.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(provider.lastReq.Messages))
	}

	first := provider.lastReq.Messages[0]
	if first.Role != "system" {
		t.Fatalf("expected first message role to be system, got %q", first.Role)
	}
	if first.Content != defaultSystemPrompt() {
		t.Fatalf("expected first message content to be default system prompt, got %q", first.Content)
	}

	second := provider.lastReq.Messages[1]
	if second.Role != "user" {
		t.Fatalf("expected second message role to be user, got %q", second.Role)
	}
	if second.Content != "say hello" {
		t.Fatalf("expected second message content to be user task, got %q", second.Content)
	}
}
