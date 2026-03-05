// Package tools provides executable tools for the agent.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// Tool is the interface for executable tools.
// This is the legacy interface maintained for backward compatibility.
// New implementations should consider implementing ValidatingTool.
type Tool interface {
	// Name returns the tool name (English, no spaces).
	Name() string

	// Description returns the tool description for LLM understanding.
	Description() string

	// Schema returns the JSON schema for tool parameters.
	Schema() llm.ToolSchema

	// Execute executes the tool with the given parameters.
	// Note: Tools that need permission checking should use GetPermissionChecker(ctx)
	// to obtain a PermissionChecker from the context.
	Execute(ctx context.Context, params json.RawMessage) (*Result, error)
}

// ValidatingTool is an enhanced tool interface with parameter validation.
// This is the recommended interface for new tool implementations.
type ValidatingTool interface {
	Tool

	// Validate validates the raw parameters against the tool's schema.
	// Returns a list of validation errors, or nil/empty if valid.
	Validate(params json.RawMessage) []ValidationError
}

// ToolWithSchema is a tool that uses the new Schema type instead of llm.ToolSchema.
// This allows for more detailed schema definitions and validation.
type ToolWithSchema interface {
	// Name returns the tool name (English, no spaces).
	Name() string

	// Description returns the tool description for LLM understanding.
	Description() string

	// Schema returns the detailed parameter schema for validation.
	Schema() Schema

	// Validate validates the raw parameters against the schema.
	Validate(params json.RawMessage) []ValidationError

	// Execute executes the tool with the given parameters.
	// Tools should use GetPermissionChecker(ctx) to obtain permission checking capabilities.
	Execute(ctx context.Context, params json.RawMessage) (*EnhancedResult, error)
}

// Result is the result of a tool execution.
// This is the legacy result type maintained for backward compatibility.
type Result struct {
	Content string // Main output content
	Summary string // Summary for UI display (e.g., "42 lines", "5 matches")
	Error   error  // Execution error
}

// StringResult creates a result with just content.
func StringResult(content string) *Result {
	return &Result{Content: content}
}

// StringResultWithSummary creates a result with content and summary.
func StringResultWithSummary(content, summary string) *Result {
	return &Result{Content: content, Summary: summary}
}

// ErrorResult creates an error result.
func ErrorResult(err error) *Result {
	return &Result{Error: err}
}

// ErrorResultf creates an error result with formatted message.
func ErrorResultf(format string, args ...any) *Result {
	return &Result{Error: fmt.Errorf(format, args...)}
}

// ParseParams parses the raw JSON parameters into a struct.
func ParseParams(data json.RawMessage, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse params: %w", err)
	}
	return nil
}

// ToLLMToolSchema converts our Schema to llm.ToolSchema for LLM compatibility.
func ToLLMToolSchema(schema Schema) llm.ToolSchema {
	// Convert properties
	properties := make(map[string]llm.Property)
	for name, prop := range schema.Properties {
		properties[name] = llm.Property{
			Type:        string(prop.Type),
			Description: prop.Description,
			Enum:        prop.Enum,
		}
	}

	return llm.ToolSchema{
		Type:       string(schema.Type),
		Properties: properties,
		Required:   schema.Required,
	}
}
