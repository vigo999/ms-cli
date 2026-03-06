package adapter

import (
	"encoding/json"
	"fmt"

	"github.com/vigo999/ms-cli/tools"
)

// OpenAIAdapter OpenAI协议适配器
type OpenAIAdapter struct {
	BaseAdapter
}

// NewOpenAIAdapter 创建新的OpenAI适配器
func NewOpenAIAdapter() Adapter {
	return &OpenAIAdapter{
		BaseAdapter: BaseAdapter{providerName: "openai"},
	}
}

// OpenAIFunctionResult OpenAI函数调用结果
type OpenAIFunctionResult struct {
	Role        string       `json:"role"`
	Name        string       `json:"name"`
	Content     string       `json:"content"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID  string       `json:"tool_call_id,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function FunctionCallInfo `json:"function"`
}

// FunctionCallInfo 函数调用信息
type FunctionCallInfo struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToProviderFormat 将ToolResult转换为OpenAI格式
func (a *OpenAIAdapter) ToProviderFormat(result *tools.ToolResult) (interface{}, error) {
	if result == nil {
		return OpenAIFunctionResult{
			Role:    "tool",
			Content: "",
		}, nil
	}

	// 合并所有部分为字符串
	content := ""
	for _, part := range result.Parts {
		switch part.Type {
		case tools.PartTypeText:
			content += part.Content
		case tools.PartTypeJSON:
			jsonBytes, _ := json.Marshal(part.Data)
			content += string(jsonBytes)
		case tools.PartTypeError:
			content += fmt.Sprintf("Error: %s", part.Content)
		default:
			content += part.Content
		}
	}

	// 如果失败，添加错误信息
	if !result.Success && result.Error != nil {
		content = fmt.Sprintf("Error [%s]: %s", result.Error.Code, result.Error.Message)
	}

	return OpenAIFunctionResult{
		Role:    "tool",
		Content: content,
	}, nil
}

// FromProviderFormat 从OpenAI格式转换为ToolResult
func (a *OpenAIAdapter) FromProviderFormat(data interface{}) (*tools.ToolResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal openai data failed: %w", err)
	}

	var openaiResult OpenAIFunctionResult
	if err := json.Unmarshal(jsonData, &openaiResult); err != nil {
		return nil, fmt.Errorf("unmarshal openai result failed: %w", err)
	}

	return tools.NewTextResult(openaiResult.Content), nil
}

// ConvertToFunctionDefinition 将ToolDefinition转换为OpenAI函数定义
func (a *OpenAIAdapter) ConvertToFunctionDefinition(def tools.ToolDefinition) map[string]interface{} {
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        def.Name,
			"description": def.Description,
			"parameters":  def.Parameters,
		},
	}
}

// ExtractFunctionCall 从消息中提取函数调用
func (a *OpenAIAdapter) ExtractFunctionCall(data interface{}) (*ToolCall, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var msg struct {
		ToolCalls []ToolCall `json:"tool_calls"`
	}
	if err := json.Unmarshal(jsonData, &msg); err != nil {
		return nil, err
	}

	if len(msg.ToolCalls) > 0 {
		return &msg.ToolCalls[0], nil
	}

	return nil, fmt.Errorf("no function call found")
}
