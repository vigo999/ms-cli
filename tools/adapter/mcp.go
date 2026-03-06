package adapter

import (
	"encoding/json"
	"fmt"

	"github.com/vigo999/ms-cli/tools"
)

// MCPAdapter MCP协议适配器
type MCPAdapter struct {
	BaseAdapter
}

// NewMCPAdapter 创建新的MCP适配器
func NewMCPAdapter() Adapter {
	return &MCPAdapter{
		BaseAdapter: BaseAdapter{providerName: "mcp"},
	}
}

// MCPCallResult MCP调用结果
type MCPCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent MCP内容项
type MCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// ToProviderFormat 将ToolResult转换为MCP格式
func (a *MCPAdapter) ToProviderFormat(result *tools.ToolResult) (interface{}, error) {
	if result == nil {
		return MCPCallResult{
			Content: []MCPContent{},
			IsError: false,
		}, nil
	}

	contents := make([]MCPContent, 0, len(result.Parts))
	for _, part := range result.Parts {
		content := a.convertPartToMCP(part)
		contents = append(contents, content)
	}

	return MCPCallResult{
		Content: contents,
		IsError: !result.Success,
	}, nil
}

// FromProviderFormat 从MCP格式转换为ToolResult
func (a *MCPAdapter) FromProviderFormat(data interface{}) (*tools.ToolResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp data failed: %w", err)
	}

	var mcpResult MCPCallResult
	if err := json.Unmarshal(jsonData, &mcpResult); err != nil {
		return nil, fmt.Errorf("unmarshal mcp result failed: %w", err)
	}

	result := &tools.ToolResult{
		Success: !mcpResult.IsError,
		Parts:   make([]tools.Part, 0, len(mcpResult.Content)),
	}

	for _, content := range mcpResult.Content {
		part := a.convertMCPToPart(content)
		result.Parts = append(result.Parts, part)
	}

	return result, nil
}

// convertPartToMCP 将Part转换为MCPContent
func (a *MCPAdapter) convertPartToMCP(part tools.Part) MCPContent {
	switch part.Type {
	case tools.PartTypeText:
		return MCPContent{
			Type:     "text",
			Text:     part.Content,
			MimeType: part.MimeType,
		}
	case tools.PartTypeJSON:
		return MCPContent{
			Type: "json",
			Data: part.Data,
		}
	case tools.PartTypeBinary:
		return MCPContent{
			Type:     "binary",
			Text:     part.Content, // base64 encoded
			MimeType: part.MimeType,
		}
	case tools.PartTypeError:
		return MCPContent{
			Type: "error",
			Text: part.Content,
		}
	default:
		return MCPContent{
			Type: "text",
			Text: part.Content,
		}
	}
}

// convertMCPToPart 将MCPContent转换为Part
func (a *MCPAdapter) convertMCPToPart(content MCPContent) tools.Part {
	switch content.Type {
	case "text":
		return tools.Part{
			Type:     tools.PartTypeText,
			Content:  content.Text,
			MimeType: content.MimeType,
		}
	case "json":
		return tools.Part{
			Type: tools.PartTypeJSON,
			Data: content.Data,
		}
	case "binary":
		return tools.Part{
			Type:     tools.PartTypeBinary,
			Content:  content.Text,
			MimeType: content.MimeType,
		}
	case "error":
		return tools.Part{
			Type:    tools.PartTypeError,
			Content: content.Text,
		}
	default:
		return tools.Part{
			Type:    tools.PartTypeText,
			Content: content.Text,
		}
	}
}
