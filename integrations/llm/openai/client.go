// Package openai provides an OpenAI API compatible provider implementation.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

const (
	defaultEndpoint = "https://api.openai.com/v1"
	defaultTimeout  = 180 * time.Second // 3 minutes for longer conversations
)

// Config holds the OpenAI client configuration.
type Config struct {
	APIKey     string
	Endpoint   string
	Model      string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// Client implements the llm.Provider interface for OpenAI.
type Client struct {
	apiKey     string
	endpoint   string
	model      string
	httpClient *http.Client
}

// NewClient creates a new OpenAI client.
func NewClient(cfg Config) (*Client, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		apiKey:     apiKey,
		endpoint:   strings.TrimRight(endpoint, "/"),
		model:      cfg.Model,
		httpClient: httpClient,
	}, nil
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "openai"
}

// SupportsTools returns whether the provider supports tool calls.
func (c *Client) SupportsTools() bool {
	return true
}

// Complete performs a non-streaming completion request.
func (c *Client) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	body, err := c.buildRequestBody(req, false)
	if err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("request timeout: the operation took too long (>%v). Try reducing context size or increasing timeout", c.httpClient.Timeout)
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("response timeout: server took too long to respond. Try with a shorter conversation or increase timeout")
		}
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertResponse(&result), nil
}

// CompleteStream performs a streaming completion request.
func (c *Client) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	body, err := c.buildRequestBody(req, true)
	if err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, c.parseError(resp)
	}

	return &streamIterator{reader: bufio.NewReader(resp.Body), closer: resp.Body}, nil
}

// AvailableModels returns the list of available models.
func (c *Client) AvailableModels() []llm.ModelInfo {
	return []llm.ModelInfo{
		{ID: "gpt-4o", Provider: "openai", MaxTokens: 128000},
		{ID: "gpt-4o-mini", Provider: "openai", MaxTokens: 128000},
		{ID: "gpt-4-turbo", Provider: "openai", MaxTokens: 128000},
		{ID: "gpt-4", Provider: "openai", MaxTokens: 8192},
		{ID: "gpt-3.5-turbo", Provider: "openai", MaxTokens: 16385},
	}
}

func (c *Client) buildRequestBody(req *llm.CompletionRequest, stream bool) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	body := map[string]any{
		"model":       model,
		"messages":    c.convertMessages(req.Messages),
		"temperature": req.Temperature,
		"stream":      stream,
	}

	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.TopP > 0 {
		body["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		body["stop"] = req.Stop
	}
	if len(req.Tools) > 0 {
		body["tools"] = c.convertTools(req.Tools)
	}

	return json.Marshal(body)
}

func (c *Client) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return c.httpClient.Do(req)
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
}

func (c *Client) convertMessages(msgs []llm.Message) []message {
	result := make([]message, len(msgs))
	for i, m := range msgs {
		result[i] = message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCalls:  c.convertToolCalls(m.ToolCalls),
			ToolCallID: m.ToolCallID,
		}
	}
	return result
}

func (c *Client) convertToolCalls(calls []llm.ToolCall) []toolCall {
	if len(calls) == 0 {
		return nil
	}
	result := make([]toolCall, len(calls))
	for i, tc := range calls {
		result[i] = toolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: toolCallFunc{
				Name:      tc.Function.Name,
				Arguments: string(tc.Function.Arguments),
			},
		}
	}
	return result
}

func (c *Client) convertTools(tools []llm.Tool) []tool {
	result := make([]tool, len(tools))
	for i, t := range tools {
		result[i] = tool{
			Type: t.Type,
			Function: toolFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		}
	}
	return result
}

func (c *Client) convertResponse(resp *chatCompletionResponse) *llm.CompletionResponse {
	if len(resp.Choices) == 0 {
		return &llm.CompletionResponse{
			ID:   resp.ID,
			Model: resp.Model,
		}
	}

	choice := resp.Choices[0]
	result := &llm.CompletionResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		Content:      choice.Message.Content,
		FinishReason: llm.FinishReason(choice.FinishReason),
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]llm.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = llm.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: llm.ToolCallFunc{
					Name:      tc.Function.Name,
					Arguments: json.RawMessage(tc.Function.Arguments),
				},
			}
		}
		result.FinishReason = llm.FinishToolCalls
	}

	return result
}

// Request/Response types for OpenAI API.

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  llm.ToolSchema `json:"parameters"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolCallFunc `json:"function"`
}

type toolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float32   `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	Tools       []tool    `json:"tools,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type chatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

type choice struct {
	Index        int      `json:"index"`
	Message      message  `json:"message"`
	FinishReason string   `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Streaming types.

type streamResponse struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []streamChoice `json:"choices"`
	Usage   *usage         `json:"usage,omitempty"`
}

type streamChoice struct {
	Index        int             `json:"index"`
	Delta        delta           `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

type delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []toolCall `json:"tool_calls,omitempty"`
}

// streamIterator implements llm.StreamIterator.
type streamIterator struct {
	reader *bufio.Reader
	closer io.Closer
	buffer llm.StreamChunk
	done   bool
}

func (it *streamIterator) Next() (*llm.StreamChunk, error) {
	if it.done {
		return nil, io.EOF
	}

	for {
		line, err := it.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				it.done = true
				if it.buffer.Content != "" || len(it.buffer.ToolCalls) > 0 {
					return &it.buffer, io.EOF
				}
			}
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" || line == "data: [DONE]" {
			if line == "data: [DONE]" {
				it.done = true
				if it.buffer.Content != "" || len(it.buffer.ToolCalls) > 0 {
					return &it.buffer, nil
				}
				return nil, io.EOF
			}
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		var resp streamResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		if len(resp.Choices) == 0 {
			continue
		}

		choice := resp.Choices[0]
		delta := choice.Delta

		// Build chunk from accumulated buffer
		chunk := &llm.StreamChunk{
			Content: delta.Content,
		}

		// Handle tool calls
		if len(delta.ToolCalls) > 0 {
			chunk.ToolCalls = convertStreamToolCalls(delta.ToolCalls)
		}

		// Check finish reason
		if choice.FinishReason != nil {
			chunk.FinishReason = llm.FinishReason(*choice.FinishReason)
			it.done = true
		}

		if resp.Usage != nil {
			chunk.Usage = &llm.Usage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			}
		}

		return chunk, nil
	}
}

func (it *streamIterator) Close() error {
	if it.closer != nil {
		return it.closer.Close()
	}
	return nil
}

func convertStreamToolCalls(calls []toolCall) []llm.ToolCall {
	result := make([]llm.ToolCall, len(calls))
	for i, tc := range calls {
		result[i] = llm.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: llm.ToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			},
		}
	}
	return result
}
