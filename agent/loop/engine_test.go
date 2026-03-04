package loop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vigo999/ms-cli/integrations/domain"
)

func TestShouldDirectReply(t *testing.T) {
	if !shouldDirectReply("hello") {
		t.Fatal("hello should trigger direct reply")
	}
	if !shouldDirectReply("你好") {
		t.Fatal("你好 should trigger direct reply")
	}
	if shouldDirectReply("fix bug in app/run.go") {
		t.Fatal("task-like input should not trigger direct reply")
	}
}

type testFactory struct{}

func (f testFactory) ClientFor(spec domain.ModelSpec) (domain.ModelClient, error) {
	return testClient{}, nil
}

func (f testFactory) Providers() []domain.ProviderInfo {
	return nil
}

type testClient struct{}

func (c testClient) Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResponse, error) {
	return &domain.GenerateResponse{Text: "ok"}, nil
}

type shellLoopFactory struct{}

func (f shellLoopFactory) ClientFor(spec domain.ModelSpec) (domain.ModelClient, error) {
	return shellLoopClient{}, nil
}

func (f shellLoopFactory) Providers() []domain.ProviderInfo {
	return nil
}

type shellLoopClient struct{}

func (c shellLoopClient) Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResponse, error) {
	return &domain.GenerateResponse{
		Text: `{"action":"shell","command":"ls -la"}`,
	}, nil
}

type globLoopFactory struct{}

func (f globLoopFactory) ClientFor(spec domain.ModelSpec) (domain.ModelClient, error) {
	return globLoopClient{}, nil
}

func (f globLoopFactory) Providers() []domain.ProviderInfo { return nil }

type globLoopClient struct{}

func (c globLoopClient) Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResponse, error) {
	return &domain.GenerateResponse{
		Text: `{"action":"glob","pattern":"*","path":"."}`,
	}, nil
}

type nopFS struct{}

func (nopFS) Read(path string) (string, error) { return "", nil }
func (nopFS) Glob(path, pattern string, maxMatches int) ([]string, error) {
	return nil, nil
}
func (nopFS) Grep(path, pattern string, maxMatches int) ([]string, error) {
	return nil, nil
}
func (nopFS) Edit(path, oldText, newText string) (string, error) { return "", nil }
func (nopFS) Write(path, content string) (int, error)            { return len(content), nil }

type nopShell struct{}

func (nopShell) Run(ctx context.Context, command string) (string, int, error) {
	return "ok", 0, nil
}

func TestRunWithContext_Canceled(t *testing.T) {
	engine := NewEngine(Config{
		ModelFactory: testFactory{},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events, err := engine.RunWithContext(ctx, Task{
		Description: "analyze code",
		Model: ModelSpec{
			Provider: "openrouter",
			Name:     "deepseek/deepseek-chat",
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected paused event")
	}
	if events[0].Type != EventReply {
		t.Fatalf("expected reply event, got %s", events[0].Type)
	}
}

func TestRunWithContextStream_EmitsEvents(t *testing.T) {
	engine := NewEngine(Config{
		ModelFactory:   testFactory{},
		DefaultMaxStep: 0, // unlimited
	})

	got := make([]EventType, 0, 4)
	err := engine.RunWithContextStream(context.Background(), Task{
		Description: "analyze code structure",
		Model: ModelSpec{
			Provider: "openrouter",
			Name:     "deepseek/deepseek-chat",
		},
	}, func(ev Event) {
		got = append(got, ev.Type)
	})
	if err != nil {
		t.Fatalf("RunWithContextStream failed: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("expected multiple streamed events, got %d", len(got))
	}
	if got[0] != EventThinking {
		t.Fatalf("first streamed event=%s want %s", got[0], EventThinking)
	}
	if got[len(got)-1] != EventReply {
		t.Fatalf("last streamed event=%s want %s", got[len(got)-1], EventReply)
	}
}

func TestRunWithContext_RepeatedShellLoopGuardStops(t *testing.T) {
	engine := NewEngine(Config{
		FS:               nopFS{},
		Shell:            nopShell{},
		ModelFactory:     shellLoopFactory{},
		DefaultMaxStep:   0, // unlimited
		MaxRepeatedShell: 2,
	})

	events, err := engine.RunWithContext(context.Background(), Task{
		Description: "分析代码结构",
		Model: ModelSpec{
			Provider: "openrouter",
			Name:     "deepseek/deepseek-chat",
		},
	})
	if err != nil {
		t.Fatalf("RunWithContext failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected events")
	}

	last := events[len(events)-1]
	if last.Type != EventReply {
		t.Fatalf("expected final reply, got %s", last.Type)
	}
	if !strings.Contains(last.Message, "重复命令循环") {
		t.Fatalf("expected loop-guard stop message, got %q", last.Message)
	}
}

func TestRunWithContext_RepeatedGlobLoopGuardStops(t *testing.T) {
	engine := NewEngine(Config{
		FS:             nopFS{},
		Shell:          nopShell{},
		ModelFactory:   globLoopFactory{},
		DefaultMaxStep: 0, // unlimited
	})

	events, err := engine.RunWithContext(context.Background(), Task{
		Description: "scan repository",
		Model: ModelSpec{
			Provider: "openrouter",
			Name:     "deepseek/deepseek-chat",
		},
	})
	if err != nil {
		t.Fatalf("RunWithContext failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected events")
	}

	last := events[len(events)-1]
	if last.Type != EventReply {
		t.Fatalf("expected final reply, got %s", last.Type)
	}
	if !strings.Contains(last.Message, "重复 Glob 循环") {
		t.Fatalf("expected glob loop-guard stop message, got %q", last.Message)
	}
}

type budgetFactory struct{}

func (f budgetFactory) ClientFor(spec domain.ModelSpec) (domain.ModelClient, error) {
	return budgetClient{}, nil
}

func (f budgetFactory) Providers() []domain.ProviderInfo { return nil }

type budgetClient struct{}

func (c budgetClient) Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResponse, error) {
	return &domain.GenerateResponse{
		Text: `{"action":"final","final":"done"}`,
		Usage: domain.Usage{
			PromptTokens:     120,
			CompletionTokens: 40,
			TotalTokens:      160,
		},
	}, nil
}

func TestRunWithContext_StopsOnTokenBudget(t *testing.T) {
	engine := NewEngine(Config{
		ModelFactory:   budgetFactory{},
		DefaultMaxStep: 1,
		MaxTotalTokens: 100,
	})

	events, err := engine.RunWithContext(context.Background(), Task{
		Description: "analyze quickly",
		Model: ModelSpec{
			Provider: "openai",
			Name:     "gpt-4o-mini",
		},
	})
	if err != nil {
		t.Fatalf("RunWithContext failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected events")
	}
	last := events[len(events)-1]
	if last.Type != EventReply || !strings.Contains(last.Message, "token budget exceeded") {
		t.Fatalf("expected budget stop reply, got %#v", last)
	}
}

type approvalFactory struct{}

func (f approvalFactory) ClientFor(spec domain.ModelSpec) (domain.ModelClient, error) {
	return approvalClient{}, nil
}

func (f approvalFactory) Providers() []domain.ProviderInfo { return nil }

type approvalClient struct{}

func (c approvalClient) Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResponse, error) {
	return &domain.GenerateResponse{
		Text: `{"action":"shell","command":"rm -rf /tmp/x"}`,
		Usage: domain.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func TestRunWithContext_BlocksOnApproval(t *testing.T) {
	pm := NewPermissionManager(false, nil)
	engine := NewEngine(Config{
		FS:                   nopFS{},
		Shell:                nopShell{},
		ModelFactory:         approvalFactory{},
		Permission:           pm,
		RequireApprovalBlock: true,
		DefaultMaxStep:       1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	events, err := engine.RunWithContext(ctx, Task{
		Description: "dangerous task",
		Model: ModelSpec{
			Provider: "openai",
			Name:     "gpt-4o-mini",
		},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded while waiting approval, got %v", err)
	}
	foundApproval := false
	for _, ev := range events {
		if ev.Type == EventApprovalRequired {
			foundApproval = true
			break
		}
	}
	if !foundApproval {
		t.Fatalf("expected approval required event, got %#v", events)
	}
}
