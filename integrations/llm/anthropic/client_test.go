package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestCompleteBuildsAnthropicRequestAndParsesResponse(t *testing.T) {
	var capturedReq messageRequest
	var capturedHeaders http.Header

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			capturedHeaders = r.Header.Clone()
			if r.URL.Path != "/v1/messages" {
				t.Fatalf("path = %s, want /v1/messages", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&capturedReq); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			payload := map[string]any{
				"id":          "msg_1",
				"model":       "claude-3-5-sonnet-latest",
				"stop_reason": "tool_use",
				"usage": map[string]any{
					"input_tokens":  12,
					"output_tokens": 8,
				},
				"content": []map[string]any{
					{"type": "text", "text": "done"},
					{"type": "tool_use", "id": "toolu_1", "name": "read", "input": map[string]any{"path": "a"}},
				},
			}
			body, _ := json.Marshal(payload)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client, err := NewClient(Config{
		Key:        "anth-key",
		URL:        "https://anthropic.test/v1",
		Model:      "claude-3-5-sonnet-latest",
		HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	toolArgs, _ := json.Marshal(map[string]any{"path": "README.md"})
	resp, err := client.Complete(context.Background(), &llm.CompletionRequest{
		Messages: []llm.Message{
			llm.NewSystemMessage("system rules"),
			llm.NewUserMessage("read file"),
			{Role: "assistant", Content: "calling tool", ToolCalls: []llm.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "read",
					Arguments: toolArgs,
				},
			}}},
			llm.NewToolMessage("call_1", "file-content"),
		},
		Tools: []llm.Tool{{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "read",
				Description: "read file",
				Parameters: llm.ToolSchema{
					Type: "object",
					Properties: map[string]llm.Property{
						"path": {Type: "string", Description: "file path"},
					},
					Required: []string{"path"},
				},
			},
		}},
		Temperature: 0.3,
		MaxTokens:   1024,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if got := capturedHeaders.Get("x-api-key"); got != "anth-key" {
		t.Fatalf("x-api-key = %q, want anth-key", got)
	}
	if got := capturedHeaders.Get("Authorization"); got != "Bearer anth-key" {
		t.Fatalf("authorization = %q, want Bearer anth-key", got)
	}
	if got := capturedHeaders.Get("anthropic-version"); got == "" {
		t.Fatal("anthropic-version header should be set")
	}
	if capturedReq.System != "system rules" {
		t.Fatalf("system = %q, want system rules", capturedReq.System)
	}
	if capturedReq.Model != "claude-3-5-sonnet-latest" {
		t.Fatalf("model = %q", capturedReq.Model)
	}
	if capturedReq.MaxTokens != 1024 {
		t.Fatalf("max_tokens = %d, want 1024", capturedReq.MaxTokens)
	}
	if len(capturedReq.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(capturedReq.Messages))
	}
	if capturedReq.Messages[0].Role != "user" || capturedReq.Messages[0].Content[0].Text != "read file" {
		t.Fatalf("unexpected first message: %#v", capturedReq.Messages[0])
	}
	if capturedReq.Messages[1].Role != "assistant" {
		t.Fatalf("unexpected second role: %s", capturedReq.Messages[1].Role)
	}
	if len(capturedReq.Messages[1].Content) < 2 || capturedReq.Messages[1].Content[1].Type != "tool_use" {
		t.Fatalf("assistant tool_use block missing: %#v", capturedReq.Messages[1].Content)
	}
	if capturedReq.Messages[2].Content[0].Type != "tool_result" {
		t.Fatalf("tool_result block missing: %#v", capturedReq.Messages[2].Content)
	}

	if resp.Content != "done" {
		t.Fatalf("content = %q, want done", resp.Content)
	}
	if resp.FinishReason != llm.FinishToolCalls {
		t.Fatalf("finish reason = %s, want %s", resp.FinishReason, llm.FinishToolCalls)
	}
	if resp.Usage.TotalTokens != 20 {
		t.Fatalf("total tokens = %d, want 20", resp.Usage.TotalTokens)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls len = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "read" {
		t.Fatalf("tool call name = %s, want read", resp.ToolCalls[0].Function.Name)
	}
}

func TestCompleteParsesAnthropicError(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			payload := map[string]any{
				"type": "error",
				"error": map[string]any{
					"type":    "authentication_error",
					"message": "invalid api key",
				},
			}
			body, _ := json.Marshal(payload)
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client, err := NewClient(Config{
		Key:        "bad",
		URL:        "https://anthropic.test/v1",
		Model:      "claude-3-5-sonnet-latest",
		HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.Complete(context.Background(), &llm.CompletionRequest{Messages: []llm.Message{llm.NewUserMessage("hello")}})
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error = %v, want anthropic error message", err)
	}
}

func TestCompleteStreamNotImplemented(t *testing.T) {
	client, err := NewClient(Config{Key: "k", Model: "claude-3-5-sonnet-latest"})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.CompleteStream(context.Background(), &llm.CompletionRequest{})
	if err == nil {
		t.Fatal("expected not implemented error")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("error = %v, want not implemented", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
