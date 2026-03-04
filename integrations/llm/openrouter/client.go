// Package openrouter provides an OpenRouter API compatible provider implementation.
// DEPRECATED: This package is kept for backward compatibility.
// Please use the openai package directly with OpenRouter endpoint.
//
// Example:
//   client, err := openai.NewClient(openai.Config{
//       APIKey:   "your-key",
//       Endpoint: "https://openrouter.ai/api/v1",
//       Model:    "openai/gpt-4o",
//   })
package openrouter

import (
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/integrations/llm/openai"
)

// Config holds the OpenRouter client configuration.
// DEPRECATED: Use openai.Config instead.
type Config struct {
	APIKey     string
	Endpoint   string
	Model      string
	Timeout    time.Duration
	HTTPClient interface{} // kept for compatibility, ignored
	SiteURL    string // kept for compatibility, ignored
	SiteName   string // kept for compatibility, ignored
}

// Client is an alias to openai.Client for backward compatibility.
// DEPRECATED: Use openai.Client instead.
type Client = openai.Client

// NewClient creates a new OpenRouter client.
// DEPRECATED: Use openai.NewClient with OpenRouter endpoint instead.
func NewClient(cfg Config) (*Client, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://openrouter.ai/api/v1"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return openai.NewClient(openai.Config{
		APIKey:   cfg.APIKey,
		Endpoint: endpoint,
		Model:    cfg.Model,
		Timeout:  timeout,
	})
}

// AvailableModels returns the list of available models from OpenRouter.
// DEPRECATED: Use openai.Client.AvailableModels instead.
func AvailableModels() []llm.ModelInfo {
	return []llm.ModelInfo{
		{ID: "openai/gpt-4o", Provider: "openrouter", MaxTokens: 128000},
		{ID: "openai/gpt-4o-mini", Provider: "openrouter", MaxTokens: 128000},
		{ID: "anthropic/claude-3.5-sonnet", Provider: "openrouter", MaxTokens: 200000},
		{ID: "anthropic/claude-3-opus", Provider: "openrouter", MaxTokens: 200000},
		{ID: "anthropic/claude-3-haiku", Provider: "openrouter", MaxTokens: 200000},
		{ID: "google/gemini-1.5-pro", Provider: "openrouter", MaxTokens: 2000000},
		{ID: "meta-llama/llama-3.1-405b-instruct", Provider: "openrouter", MaxTokens: 128000},
		{ID: "deepseek/deepseek-chat", Provider: "openrouter", MaxTokens: 64000},
	}
}
