package adapter

import (
	"encoding/json"
	"fmt"

	"github.com/vigo999/ms-cli/tools"
)

// A2AAdapter A2A协议适配器（预留）
type A2AAdapter struct {
	BaseAdapter
}

// NewA2AAdapter 创建新的A2A适配器
func NewA2AAdapter() Adapter {
	return &A2AAdapter{
		BaseAdapter: BaseAdapter{providerName: "a2a"},
	}
}

// A2ATaskResult A2A任务结果
type A2ATaskResult struct {
	ID       string      `json:"id"`
	Status   string      `json:"status"`
	Output   interface{} `json:"output,omitempty"`
	Error    *A2AError   `json:"error,omitempty"`
	Artifacts []A2AArtifact `json:"artifacts,omitempty"`
}

// A2AError A2A错误
type A2AError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// A2AArtifact A2A产物
type A2AArtifact struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Content     interface{} `json:"content"`
	MimeType    string      `json:"mimeType,omitempty"`
	Description string      `json:"description,omitempty"`
}

// ToProviderFormat 将ToolResult转换为A2A格式
func (a *A2AAdapter) ToProviderFormat(result *tools.ToolResult) (interface{}, error) {
	if result == nil {
		return A2ATaskResult{
			ID:     "",
			Status: "completed",
			Output: "",
		}, nil
	}

	a2aResult := A2ATaskResult{
		ID:     "",
		Status: "completed",
	}

	if !result.Success {
		a2aResult.Status = "failed"
		if result.Error != nil {
			a2aResult.Error = &A2AError{
				Code:    string(result.Error.Code),
				Message: result.Error.Message,
			}
		}
	}

	// 转换各部分为产物
	artifacts := make([]A2AArtifact, 0, len(result.Parts))
	for i, part := range result.Parts {
		artifact := a.convertPartToArtifact(i, part)
		artifacts = append(artifacts, artifact)
	}
	a2aResult.Artifacts = artifacts

	// 第一个文本部分作为主输出
	for _, part := range result.Parts {
		if part.Type == tools.PartTypeText {
			a2aResult.Output = part.Content
			break
		}
	}

	return a2aResult, nil
}

// FromProviderFormat 从A2A格式转换为ToolResult
func (a *A2AAdapter) FromProviderFormat(data interface{}) (*tools.ToolResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal a2a data failed: %w", err)
	}

	var a2aResult A2ATaskResult
	if err := json.Unmarshal(jsonData, &a2aResult); err != nil {
		return nil, fmt.Errorf("unmarshal a2a result failed: %w", err)
	}

	result := &tools.ToolResult{
		Success: a2aResult.Status == "completed",
		Parts:   make([]tools.Part, 0),
	}

	// 转换产物为Part
	for _, artifact := range a2aResult.Artifacts {
		part := a.convertArtifactToPart(artifact)
		result.Parts = append(result.Parts, part)
	}

	// 转换错误
	if a2aResult.Error != nil {
		result.Success = false
		result.Error = &tools.ToolError{
			Code:    tools.ToolErrorCode(a2aResult.Error.Code),
			Message: a2aResult.Error.Message,
		}
	}

	return result, nil
}

// convertPartToArtifact 将Part转换为A2AArtifact
func (a *A2AAdapter) convertPartToArtifact(index int, part tools.Part) A2AArtifact {
	artifact := A2AArtifact{
		Name: fmt.Sprintf("part_%d", index),
		Type: string(part.Type),
	}

	switch part.Type {
	case tools.PartTypeText:
		artifact.Content = part.Content
		artifact.MimeType = "text/plain"
	case tools.PartTypeJSON:
		artifact.Content = part.Data
		artifact.MimeType = "application/json"
	case tools.PartTypeBinary:
		artifact.Content = part.Content
		artifact.MimeType = part.MimeType
		if artifact.MimeType == "" {
			artifact.MimeType = "application/octet-stream"
		}
	case tools.PartTypeError:
		artifact.Content = part.Content
		artifact.MimeType = "text/plain"
		artifact.Type = "error"
	case tools.PartTypeArtifact:
		artifact.Content = part.Content
		artifact.MimeType = part.MimeType
		if metadata, ok := part.Metadata["name"].(string); ok {
			artifact.Name = metadata
		}
		if desc, ok := part.Metadata["description"].(string); ok {
			artifact.Description = desc
		}
	}

	return artifact
}

// convertArtifactToPart 将A2AArtifact转换为Part
func (a *A2AAdapter) convertArtifactToPart(artifact A2AArtifact) tools.Part {
	partType := tools.PartType(artifact.Type)
	if partType == "" {
		partType = tools.PartTypeArtifact
	}

	part := tools.Part{
		Type:     partType,
		MimeType: artifact.MimeType,
		Metadata: map[string]interface{}{
			"name": artifact.Name,
		},
	}

	if artifact.Description != "" {
		part.Metadata["description"] = artifact.Description
	}

	switch artifact.Type {
	case "text":
		part.Type = tools.PartTypeText
		if content, ok := artifact.Content.(string); ok {
			part.Content = content
		}
	case "json":
		part.Type = tools.PartTypeJSON
		part.Data = artifact.Content
	case "binary":
		part.Type = tools.PartTypeBinary
		if content, ok := artifact.Content.(string); ok {
			part.Content = content
		}
	case "error":
		part.Type = tools.PartTypeError
		if content, ok := artifact.Content.(string); ok {
			part.Content = content
		}
	default:
		part.Type = tools.PartTypeArtifact
		if content, ok := artifact.Content.(string); ok {
			part.Content = content
		}
	}

	return part
}
