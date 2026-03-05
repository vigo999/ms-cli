package domain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatCompletionsClient_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("missing authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "choices":[{"message":{"role":"assistant","content":"ok"}}],
  "usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
}`))
	}))
	defer srv.Close()

	c := &chatCompletionsClient{
		httpClient: srv.Client(),
		provider:   "openai",
		endpoint:   srv.URL,
		apiKey:     "test-key",
	}

	resp, err := c.Generate(context.Background(), GenerateRequest{
		Model:        "gpt-4o-mini",
		SystemPrompt: "system",
		Input:        "hello",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Text != "ok" || resp.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestChatCompletionsClient_GenerateAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer srv.Close()

	c := &chatCompletionsClient{
		httpClient: srv.Client(),
		provider:   "openai",
		endpoint:   srv.URL,
		apiKey:     "bad-key",
	}
	_, err := c.Generate(context.Background(), GenerateRequest{
		Model:        "gpt-4o-mini",
		SystemPrompt: "system",
		Input:        "hello",
	})
	if err == nil {
		t.Fatalf("expected API error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: %d", apiErr.StatusCode)
	}
}

func TestChatCompletionsClient_GenerateOpenRouterHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("HTTP-Referer") == "" {
			t.Fatalf("missing HTTP-Referer header")
		}
		if r.Header.Get("X-Title") == "" {
			t.Fatalf("missing X-Title header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "choices":[{"message":{"role":"assistant","content":"ok"}}],
  "usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
}`))
	}))
	defer srv.Close()

	c := &chatCompletionsClient{
		httpClient: srv.Client(),
		provider:   "openrouter",
		endpoint:   srv.URL,
		apiKey:     "test-key",
	}

	_, err := c.Generate(context.Background(), GenerateRequest{
		Model:        "deepseek/deepseek-r1",
		SystemPrompt: "system",
		Input:        "hello",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
}

func TestIsOpenRouterEndpoint(t *testing.T) {
	cases := []struct {
		endpoint string
		want     bool
	}{
		{endpoint: "https://openrouter.ai/api/v1", want: true},
		{endpoint: "https://api.openrouter.ai/v1", want: true},
		{endpoint: "https://api.openai.com/v1", want: false},
		{endpoint: "not-a-url", want: false},
	}
	for _, tc := range cases {
		got := isOpenRouterEndpoint(tc.endpoint)
		if got != tc.want {
			t.Fatalf("isOpenRouterEndpoint(%q)=%v want %v", tc.endpoint, got, tc.want)
		}
	}
}
