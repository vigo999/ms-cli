package loop

import (
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/test/mocks"
	"github.com/vigo999/ms-cli/tools"
)

func TestEngineMessageSinkReceivesUserAssistantMessages(t *testing.T) {
	provider := mocks.NewMockProvider()
	provider.AddResponse("done")

	engine := NewEngine(EngineConfig{
		MaxIterations: 3,
		MaxTokens:     8000,
	}, provider, tools.NewRegistry())

	got := make([]llm.Message, 0, 2)
	engine.SetMessageSink(func(msg llm.Message) error {
		got = append(got, msg)
		return nil
	})

	_, err := engine.Run(Task{
		ID:          "task_1",
		Description: "say hello",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(got) < 2 {
		t.Fatalf("sink messages = %d, want >= 2", len(got))
	}
	if got[0].Role != "user" {
		t.Fatalf("first sink role = %s, want user", got[0].Role)
	}
	if got[1].Role != "assistant" {
		t.Fatalf("second sink role = %s, want assistant", got[1].Role)
	}
}

func TestEngineMessageSinkReceivesToolMessages(t *testing.T) {
	provider := mocks.NewMockProvider()
	provider.AddToolCallResponse([]llm.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: llm.ToolCallFunc{
				Name:      "missing_tool",
				Arguments: []byte(`{}`),
			},
		},
	})
	provider.AddResponse("done")

	engine := NewEngine(EngineConfig{
		MaxIterations: 4,
		MaxTokens:     8000,
	}, provider, tools.NewRegistry())

	toolMessages := 0
	engine.SetMessageSink(func(msg llm.Message) error {
		if msg.Role == "tool" {
			toolMessages++
		}
		return nil
	})

	_, err := engine.Run(Task{
		ID:          "task_2",
		Description: "call a missing tool",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if toolMessages == 0 {
		t.Fatalf("expected sink to receive at least one tool message")
	}
}
