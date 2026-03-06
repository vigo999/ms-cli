package main

import (
	"context"
	"fmt"

	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/integrations/llm"
)

// unconfiguredProvider lets the app boot without an API key and fails lazily on first request.
type unconfiguredProvider struct {
	protocol string
	model    string
}

func newUnconfiguredProvider(protocol, model string) llm.Provider {
	return &unconfiguredProvider{
		protocol: configs.NormalizeProtocol(protocol),
		model:    model,
	}
}

func (p *unconfiguredProvider) Name() string {
	return p.protocol
}

func (p *unconfiguredProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	_ = ctx
	_ = req
	return nil, p.missingKeyError()
}

func (p *unconfiguredProvider) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	_ = ctx
	_ = req
	return nil, p.missingKeyError()
}

func (p *unconfiguredProvider) SupportsTools() bool {
	return true
}

func (p *unconfiguredProvider) AvailableModels() []llm.ModelInfo {
	provider := p.protocol
	if provider == "" {
		provider = configs.ProtocolOpenAI
	}
	if p.model == "" {
		return nil
	}
	return []llm.ModelInfo{{ID: p.model, Provider: provider}}
}

func (p *unconfiguredProvider) missingKeyError() error {
	envVar := providerAPIKeyEnvVar(p.protocol)
	return fmt.Errorf("API key is not configured for %s. Use /model key <KEY> or set MSCLI_API_KEY/%s", p.protocol, envVar)
}
