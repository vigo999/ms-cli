// Package anthropic provides an Anthropic messages API provider implementation.
package anthropic

import (
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
	defaultEndpoint  = "https://api.anthropic.com/v1"
	defaultTimeout   = 180 * time.Second
	defaultVersion   = "2023-06-01"
	defaultMaxTokens = 4096
)

// Config holds Anthropic client configuration.
type Config struct {
	Key        string
	URL        string
	Model      string
	MaxTokens  int
	Timeout    time.Duration
	Version    string
	HTTPClient *http.Client
}

// Client implements llm.Provider for Anthropic messages API.
type Client struct {
	apiKey     string
	endpoint   string
	model      string
	maxTokens  int
	version    string
	httpClient *http.Client
}

// NewClient creates a new Anthropic client.
func NewClient(cfg Config) (*Client, error) {
	apiKey := strings.TrimSpace(cfg.Key)
	if apiKey == "" {
		return nil, fmt.Errorf("key is required")
	}

	endpoint := strings.TrimSpace(cfg.URL)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = defaultVersion
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	return &Client{
		apiKey:     apiKey,
		endpoint:   strings.TrimRight(endpoint, "/"),
		model:      strings.TrimSpace(cfg.Model),
		maxTokens:  maxTokens,
		version:    version,
		httpClient: httpClient,
	}, nil
}

// Name returns provider name.
func (c *Client) Name() string {
	return "anthropic"
}

// SupportsTools reports tool-call support.
func (c *Client) SupportsTools() bool {
	return true
}

// Complete performs a non-streaming completion request.
func (c *Client) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	body, err := c.buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var out messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertResponse(&out)
}

// CompleteStream is intentionally not implemented yet.
func (c *Client) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	_ = ctx
	_ = req
	return nil, fmt.Errorf("anthropic stream is not implemented yet")
}

// AvailableModels returns a static model list.
func (c *Client) AvailableModels() []llm.ModelInfo {
	return []llm.ModelInfo{
		{ID: "claude-3-7-sonnet-latest", Provider: "anthropic", MaxTokens: 200000},
		{ID: "claude-3-5-sonnet-latest", Provider: "anthropic", MaxTokens: 200000},
		{ID: "claude-3-5-haiku-latest", Provider: "anthropic", MaxTokens: 200000},
	}
}

func (c *Client) buildRequestBody(req *llm.CompletionRequest) ([]byte, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.model
	}
	if model == "" {
		model = "claude-3-5-sonnet-latest"
	}

	system, messages, err := c.convertMessages(req.Messages)
	if err != nil {
		return nil, err
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.maxTokens
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	payload := messageRequest{
		Model:       model,
		System:      system,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	if len(req.Stop) > 0 {
		payload.StopSequences = req.Stop
	}
	if len(req.Tools) > 0 {
		payload.Tools = c.convertTools(req.Tools)
	}

	return json.Marshal(payload)
}

func (c *Client) convertMessages(messages []llm.Message) (string, []message, error) {
	systemParts := make([]string, 0)
	result := make([]message, 0, len(messages))

	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		switch role {
		case "system":
			if content := strings.TrimSpace(msg.Content); content != "" {
				systemParts = append(systemParts, content)
			}
		case "assistant":
			blocks := make([]contentBlock, 0, 1+len(msg.ToolCalls))
			if text := msg.Content; text != "" {
				blocks = append(blocks, contentBlock{Type: "text", Text: text})
			}
			for _, tc := range msg.ToolCalls {
				input := map[string]any{}
				if len(tc.Function.Arguments) > 0 {
					if err := json.Unmarshal(tc.Function.Arguments, &input); err != nil {
						return "", nil, fmt.Errorf("decode tool call arguments for %s: %w", tc.Function.Name, err)
					}
				}
				blocks = append(blocks, contentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			if len(blocks) > 0 {
				result = append(result, message{Role: "assistant", Content: blocks})
			}
		case "tool":
			if msg.ToolCallID == "" {
				continue
			}
			result = append(result, message{
				Role: "user",
				Content: []contentBlock{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				}},
			})
		default:
			if msg.Content == "" {
				continue
			}
			result = append(result, message{Role: "user", Content: []contentBlock{{Type: "text", Text: msg.Content}}})
		}
	}

	return strings.Join(systemParts, "\n\n"), result, nil
}

func (c *Client) convertTools(tools []llm.Tool) []tool {
	out := make([]tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, tool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	return out
}

func (c *Client) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	// Some Anthropic-compatible gateways require Bearer auth instead of x-api-key.
	// Send both for compatibility.
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("anthropic-version", c.version)

	return c.httpClient.Do(req)
}

func (c *Client) parseError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return fmt.Errorf("anthropic API error (status %d): failed to read body: %w", resp.StatusCode, err)
	}

	if len(body) == 0 {
		return fmt.Errorf("anthropic API error (status %d): empty response", resp.StatusCode)
	}

	var er struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &er) == nil && er.Error.Message != "" {
		return fmt.Errorf("anthropic API error (status %d, %s): %s", resp.StatusCode, er.Error.Type, er.Error.Message)
	}

	return fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
}

func (c *Client) convertResponse(resp *messageResponse) (*llm.CompletionResponse, error) {
	result := &llm.CompletionResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		FinishReason: mapFinishReason(resp.StopReason),
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	var textBuilder strings.Builder
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textBuilder.WriteString(block.Text)
		case "tool_use":
			args, err := json.Marshal(block.Input)
			if err != nil {
				return nil, fmt.Errorf("marshal tool input: %w", err)
			}
			result.ToolCalls = append(result.ToolCalls, llm.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      block.Name,
					Arguments: json.RawMessage(args),
				},
			})
		}
	}
	result.Content = textBuilder.String()
	if len(result.ToolCalls) > 0 && result.FinishReason == llm.FinishStop {
		result.FinishReason = llm.FinishToolCalls
	}

	return result, nil
}

func mapFinishReason(reason string) llm.FinishReason {
	switch reason {
	case "tool_use":
		return llm.FinishToolCalls
	case "max_tokens":
		return llm.FinishLength
	default:
		return llm.FinishStop
	}
}

type messageRequest struct {
	Model         string    `json:"model"`
	System        string    `json:"system,omitempty"`
	Messages      []message `json:"messages"`
	MaxTokens     int       `json:"max_tokens"`
	Temperature   float32   `json:"temperature,omitempty"`
	TopP          float32   `json:"top_p,omitempty"`
	StopSequences []string  `json:"stop_sequences,omitempty"`
	Tools         []tool    `json:"tools,omitempty"`
}

type message struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema llm.ToolSchema `json:"input_schema"`
}

type messageResponse struct {
	ID         string                 `json:"id"`
	Model      string                 `json:"model"`
	Content    []responseContentBlock `json:"content"`
	StopReason string                 `json:"stop_reason"`
	Usage      usage                  `json:"usage"`
}

type responseContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
