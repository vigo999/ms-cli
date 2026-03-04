// Package mocks provides mock implementations for testing.
package mocks

import (
	"context"
	"fmt"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// MockProvider is a mock LLM provider for testing.
type MockProvider struct {
	Name_           string
	Responses       []llm.CompletionResponse
	StreamResponses []llm.StreamChunk
	CallCount       int
	SupportsTools_  bool
	Models          []llm.ModelInfo
}

// NewMockProvider creates a new mock provider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		Name_:          "mock",
		Responses:      make([]llm.CompletionResponse, 0),
		SupportsTools_: true,
		Models: []llm.ModelInfo{
			{ID: "mock-model", Provider: "mock", MaxTokens: 4096},
		},
	}
}

// AddResponse adds a response to the mock.
func (m *MockProvider) AddResponse(content string) {
	m.Responses = append(m.Responses, llm.CompletionResponse{
		Content:      content,
		FinishReason: llm.FinishStop,
	})
}

// AddToolCallResponse adds a tool call response.
func (m *MockProvider) AddToolCallResponse(toolCalls []llm.ToolCall) {
	m.Responses = append(m.Responses, llm.CompletionResponse{
		ToolCalls:    toolCalls,
		FinishReason: llm.FinishToolCalls,
	})
}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	return m.Name_
}

// Complete performs a completion request.
func (m *MockProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if m.CallCount >= len(m.Responses) {
		return &llm.CompletionResponse{
			Content:      "mock response",
			FinishReason: llm.FinishStop,
		}, nil
	}

	resp := m.Responses[m.CallCount]
	m.CallCount++
	return &resp, nil
}

// CompleteStream performs a streaming completion request.
func (m *MockProvider) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	return &mockStreamIterator{chunks: m.StreamResponses}, nil
}

// SupportsTools returns whether the provider supports tool calls.
func (m *MockProvider) SupportsTools() bool {
	return m.SupportsTools_
}

// AvailableModels returns available models.
func (m *MockProvider) AvailableModels() []llm.ModelInfo {
	return m.Models
}

// mockStreamIterator is a mock stream iterator.
type mockStreamIterator struct {
	chunks []llm.StreamChunk
	index  int
}

// Next returns the next chunk.
func (m *mockStreamIterator) Next() (*llm.StreamChunk, error) {
	if m.index >= len(m.chunks) {
		return nil, fmt.Errorf("EOF")
	}

	chunk := m.chunks[m.index]
	m.index++
	return &chunk, nil
}

// Close closes the iterator.
func (m *mockStreamIterator) Close() error {
	return nil
}
