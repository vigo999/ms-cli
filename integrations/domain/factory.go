package domain

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type HTTPFactory struct {
	providers map[string]ProviderConfig
	client    *http.Client
}

func NewFactory(cfg FactoryConfig) *HTTPFactory {
	providers := map[string]ProviderConfig{
		"openai": {
			Endpoint:  "https://api.openai.com/v1",
			BaseURL:   "https://api.openai.com/v1",
			APIKeyEnv: "OPENAI_API_KEY",
		},
		"openrouter": {
			Endpoint:  "https://openrouter.ai/api/v1",
			BaseURL:   "https://openrouter.ai/api/v1",
			APIKeyEnv: "OPENROUTER_API_KEY",
		},
	}
	for k, v := range cfg.Providers {
		name := strings.ToLower(strings.TrimSpace(k))
		if name == "" {
			continue
		}
		if strings.TrimSpace(v.Endpoint) == "" && strings.TrimSpace(v.BaseURL) != "" {
			v.Endpoint = strings.TrimSpace(v.BaseURL)
		}
		if strings.TrimSpace(v.BaseURL) == "" && strings.TrimSpace(v.Endpoint) != "" {
			v.BaseURL = strings.TrimSpace(v.Endpoint)
		}
		if strings.TrimSpace(v.Endpoint) == "" {
			v.Endpoint = providers[name].Endpoint
		}
		if strings.TrimSpace(v.BaseURL) == "" {
			v.BaseURL = providers[name].BaseURL
		}
		if strings.TrimSpace(v.APIKeyEnv) == "" {
			v.APIKeyEnv = providers[name].APIKeyEnv
		}
		if strings.TrimSpace(v.APIKey) == "" {
			v.APIKey = providers[name].APIKey
		}
		providers[name] = v
	}

	for name, v := range providers {
		if strings.TrimSpace(v.Endpoint) == "" && strings.TrimSpace(v.BaseURL) != "" {
			v.Endpoint = strings.TrimSpace(v.BaseURL)
		}
		if strings.TrimSpace(v.BaseURL) == "" && strings.TrimSpace(v.Endpoint) != "" {
			v.BaseURL = strings.TrimSpace(v.Endpoint)
		}
		providers[name] = v
	}

	return &HTTPFactory{
		providers: providers,
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (f *HTTPFactory) Providers() []ProviderInfo {
	names := make([]string, 0, len(f.providers))
	for k := range f.providers {
		names = append(names, k)
	}
	sort.Strings(names)

	out := make([]ProviderInfo, 0, len(names))
	for _, name := range names {
		out = append(out, ProviderInfo{
			Name:     name,
			Endpoint: f.providers[name].Endpoint,
		})
	}
	return out
}

func (f *HTTPFactory) ClientFor(spec ModelSpec) (ModelClient, error) {
	provider := strings.ToLower(strings.TrimSpace(spec.Provider))
	if provider == "" {
		provider = "openai"
	}
	pcfg, ok := f.providers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported model provider: %s", provider)
	}

	endpoint := strings.TrimSpace(spec.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(pcfg.Endpoint)
		if endpoint == "" {
			endpoint = strings.TrimSpace(pcfg.BaseURL)
		}
	}
	if endpoint == "" {
		return nil, fmt.Errorf("provider %s endpoint is empty", provider)
	}

	keyEnv := strings.TrimSpace(pcfg.APIKeyEnv)
	if keyEnv == "" {
		return nil, fmt.Errorf("provider %s api key env is empty", provider)
	}
	apiKey := strings.TrimSpace(pcfg.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv(keyEnv))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("%s is required for provider %s", keyEnv, provider)
	}

	return &chatCompletionsClient{
		httpClient: f.client,
		provider:   provider,
		endpoint:   strings.TrimRight(endpoint, "/"),
		apiKey:     apiKey,
	}, nil
}
