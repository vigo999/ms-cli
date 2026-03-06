package main

import (
	"os"
	"strings"

	"github.com/vigo999/ms-cli/configs"
)

const (
	defaultOpenAIURL    = "https://api.openai.com/v1"
	defaultAnthropicURL = "https://api.anthropic.com/v1"
)

func defaultURLForProtocol(protocol string) string {
	switch configs.NormalizeProtocol(protocol) {
	case configs.ProtocolAnthropic:
		return defaultAnthropicURL
	default:
		return defaultOpenAIURL
	}
}

func providerAPIKeyEnvVar(protocol string) string {
	switch configs.NormalizeProtocol(protocol) {
	case configs.ProtocolAnthropic:
		return "ANTHROPIC_API_KEY"
	default:
		return "OPENAI_API_KEY"
	}
}

func hasConfiguredAPIKey(cfgProtocol, cfgKey string) bool {
	if strings.TrimSpace(cfgKey) != "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("MSCLI_API_KEY")) != "" {
		return true
	}
	return strings.TrimSpace(os.Getenv(providerAPIKeyEnvVar(cfgProtocol))) != ""
}
