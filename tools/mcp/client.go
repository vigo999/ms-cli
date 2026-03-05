package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vigo999/ms-cli/tools"
)

// Client is the MCP client interface
type Client interface {
	// ListTools returns available tools from the MCP server
	ListTools(ctx context.Context) ([]MCPTool, error)
	// CallTool calls a tool on the MCP server
	CallTool(ctx context.Context, name string, params map[string]any) (*CallResult, error)
	// Close closes the client connection
	Close() error
}

// MCPTool represents a tool from an MCP server
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"` // JSON Schema
}

// CallResult is the result of an MCP tool call
type CallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem represents content in the result
type ContentItem struct {
	Type string `json:"type"` // "text", "image", etc.
	Text string `json:"text,omitempty"`
}

// Adapter wraps an MCP tool as a local Tool
type Adapter struct {
	name        string
	description string
	schema      tools.Schema
	client      Client
}

// NewAdapter creates a new MCP tool adapter
func NewAdapter(tool MCPTool, client Client) (*Adapter, error) {
	// Convert JSON Schema to our Schema format
	schema, err := convertJSONSchema(tool.Schema)
	if err != nil {
		return nil, fmt.Errorf("convert schema: %w", err)
	}

	return &Adapter{
		name:        tool.Name,
		description: tool.Description,
		schema:      schema,
		client:      client,
	}, nil
}

func (a *Adapter) Name() string {
	return a.name
}

func (a *Adapter) Description() string {
	return a.description
}

func (a *Adapter) Schema() tools.Schema {
	return a.schema
}

func (a *Adapter) Validate(params json.RawMessage) []tools.ValidationError {
	return tools.ValidateAgainstSchema(params, a.schema)
}

func (a *Adapter) Execute(ctx context.Context, params json.RawMessage) (*tools.EnhancedResult, error) {
	// Parse params
	var paramMap map[string]any
	if err := json.Unmarshal(params, &paramMap); err != nil {
		return &tools.EnhancedResult{
			Type:    tools.ResultTypeError,
			Content: err.Error(),
			Error: &tools.ErrorInfo{
				Code:    "parse_error",
				Message: err.Error(),
			},
		}, nil
	}

	// Call MCP server
	result, err := a.client.CallTool(ctx, a.name, paramMap)
	if err != nil {
		return &tools.EnhancedResult{
			Type:    tools.ResultTypeError,
			Content: err.Error(),
			Error: &tools.ErrorInfo{
				Code:    "mcp_error",
				Message: err.Error(),
			},
		}, nil
	}

	// Convert result
	if result.IsError {
		content := ""
		for _, item := range result.Content {
			if item.Type == "text" {
				content += item.Text
			}
		}
		return &tools.EnhancedResult{
			Type:    tools.ResultTypeError,
			Content: content,
			Error: &tools.ErrorInfo{
				Code:    "tool_error",
				Message: content,
			},
		}, nil
	}

	// Build content
	var content string
	for _, item := range result.Content {
		if item.Type == "text" {
			content += item.Text + "\n"
		}
	}

	return &tools.EnhancedResult{
		Type:    tools.ResultTypeText,
		Content: content,
		Summary: fmt.Sprintf("MCP %s executed", a.name),
	}, nil
}

// convertJSONSchema converts JSON Schema to our Schema format
func convertJSONSchema(raw json.RawMessage) (tools.Schema, error) {
	// Parse JSON Schema
	var jsonSchema struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}

	if err := json.Unmarshal(raw, &jsonSchema); err != nil {
		return tools.Schema{}, fmt.Errorf("parse JSON schema: %w", err)
	}

	schema := tools.Schema{
		Type:       tools.SchemaType(jsonSchema.Type),
		Properties: make(map[string]*tools.SchemaProperty),
		Required:   jsonSchema.Required,
	}

	// Convert properties
	for name, propRaw := range jsonSchema.Properties {
		var prop struct {
			Type        string   `json:"type"`
			Description string   `json:"description"`
			Enum        []string `json:"enum"`
		}
		if err := json.Unmarshal(propRaw, &prop); err != nil {
			continue // Skip invalid properties
		}

		schema.Properties[name] = &tools.SchemaProperty{
			Type:        tools.SchemaType(prop.Type),
			Description: prop.Description,
			Enum:        prop.Enum,
		}
	}

	return schema, nil
}
