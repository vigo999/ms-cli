package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type chatCompletionsClient struct {
	httpClient *http.Client
	provider   string
	endpoint   string
	apiKey     string
}

type chatCompletionsRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature,omitempty"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *chatCompletionsClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	body, err := json.Marshal(chatCompletionsRequest{
		Model: model,
		Messages: []chatCompletionMessage{
			{
				Role:    "system",
				Content: req.SystemPrompt,
			},
			{
				Role:    "user",
				Content: req.Input,
			},
		},
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.provider == "openrouter" || isOpenRouterEndpoint(c.endpoint) {
		httpReq.Header.Set("HTTP-Referer", "https://github.com/vigo999/ms-cli")
		httpReq.Header.Set("X-Title", "ms-cli")
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode model response: %w", err)
	}

	if httpResp.StatusCode >= 300 {
		msg := strings.TrimSpace(httpResp.Status)
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			msg = parsed.Error.Message
		}
		return nil, &APIError{
			StatusCode: httpResp.StatusCode,
			Message:    msg,
		}
	}

	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("empty response choices from %s", c.provider)
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	return &GenerateResponse{
		Text: content,
		Usage: Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		},
	}, nil
}

func isOpenRouterEndpoint(endpoint string) bool {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	return host == "openrouter.ai" || strings.HasSuffix(host, ".openrouter.ai")
}
