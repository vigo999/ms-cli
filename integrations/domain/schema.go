package domain

type ModelSpec struct {
	Provider string
	Model    string
	Endpoint string
}

type GenerateRequest struct {
	Model        string
	SystemPrompt string
	Input        string
	Temperature  float64
	MaxTokens    int
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type GenerateResponse struct {
	Text  string
	Usage Usage
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

type ProviderConfig struct {
	Endpoint  string
	BaseURL   string
	APIKeyEnv string
	APIKey    string
}

type ProviderInfo struct {
	Name     string
	Endpoint string
}

type FactoryConfig struct {
	Providers map[string]ProviderConfig
}
