package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vigo999/ms-cli/tools"
)

// EditTool 文件编辑工具
type EditTool struct {
	definition *tools.ToolDefinition
}

// EditParams 编辑参数
type EditParams struct {
	// 文件路径
	Path string `json:"path"`

	// 要查找的旧文本（用于定位）
	OldText string `json:"oldText"`

	// 新文本（用于替换）
	NewText string `json:"newText"`

	// 是否替换所有匹配项
	All bool `json:"all,omitempty"`

	// 是否区分大小写
	CaseSensitive bool `json:"caseSensitive,omitempty"`

	// 多行模式（允许OldText/NewText包含换行）
	Multiline bool `json:"multiline,omitempty"`
}

// EditResult 编辑结果
type EditResult struct {
	Path        string `json:"path"`
	Replacements int   `json:"replacements"`
	OriginalSize int64 `json:"originalSize"`
	NewSize     int64  `json:"newSize"`
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
}

// NewEditTool 创建新的编辑工具
func NewEditTool() tools.ExecutableTool {
	return &EditTool{
		definition: &tools.ToolDefinition{
			Name:        "edit",
			DisplayName: "Edit File",
			Description: "Edit a file by replacing text. Can replace single or multiple occurrences of a text pattern.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to edit",
					},
					"oldText": map[string]interface{}{
						"type":        "string",
						"description": "The text to find and replace",
					},
					"newText": map[string]interface{}{
						"type":        "string",
						"description": "The new text to insert",
					},
					"all": map[string]interface{}{
						"type":        "boolean",
						"description": "Replace all occurrences (default: false)",
						"default":     false,
					},
					"caseSensitive": map[string]interface{}{
						"type":        "boolean",
						"description": "Case-sensitive matching (default: true)",
						"default":     true,
					},
					"multiline": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable multiline mode for pattern matching (default: false)",
						"default":     false,
					},
				},
				"required": []string{"path", "oldText", "newText"},
			},
			Meta: tools.ToolMeta{
				Category:   "filesystem",
				Cost:       tools.CostLevelMedium,
				ReadOnly:   false,
				Idempotent: false,
				Permissions: []tools.Permission{
					{
						Name:           "file:write",
						Description:    "Modify file content",
						RequireConfirm: true,
					},
				},
			},
			Version: "1.0.0",
		},
	}
}

// Info 返回工具定义
func (t *EditTool) Info() *tools.ToolDefinition {
	return t.definition
}

// Execute 执行编辑操作
func (t *EditTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	// 解析参数
	var params EditParams
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "invalid parameters: %v", err), nil
	}

	if params.Path == "" {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "path is required"), nil
	}

	if params.OldText == "" {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "oldText is required"), nil
	}

	// 执行层权限检查
	if exec != nil {
		req := tools.PermissionRequest{
			ID:         ctx.CallID,
			SessionID:  ctx.SessionID,
			ToolID:     ctx.ToolID,
			CallID:     ctx.CallID,
			Permission: "file:write",
			Patterns:   []string{params.Path},
			CheckLevel: tools.CheckLevelExecution,
		}
		if err := exec.AskPermission(req); err != nil {
			return tools.NewErrorResult(tools.ErrCodePermissionDenied, "permission denied: %v", err), nil
		}
	}

	// 检查文件是否存在
	info, err := os.Stat(params.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return tools.NewErrorResult(tools.ErrCodeNotFound, "file not found: %s", params.Path), nil
		}
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to stat file: %v", err), nil
	}

	if info.IsDir() {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "path is a directory: %s", params.Path), nil
	}

	// 读取文件内容
	content, err := os.ReadFile(params.Path)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to read file: %v", err), nil
	}

	originalContent := string(content)
	originalSize := int64(len(content))

	// 执行替换
	newContent, replacements, err := t.performReplace(originalContent, params)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "replace failed: %v", err), nil
	}

	if replacements == 0 {
		return tools.NewErrorResult(tools.ErrCodeNotFound, "text not found in file: %s", truncate(params.OldText, 50)), nil
	}

	// 写回文件
	if err := os.WriteFile(params.Path, []byte(newContent), info.Mode()); err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to write file: %v", err), nil
	}

	// 构建结果
	result := EditResult{
		Path:         params.Path,
		Replacements: replacements,
		OriginalSize: originalSize,
		NewSize:      int64(len(newContent)),
		Success:      true,
		Message:      fmt.Sprintf("Successfully made %d replacement(s)", replacements),
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	toolResult := tools.NewJSONResult(result)
	toolResult.AddPart(tools.Part{
		Type:    tools.PartTypeText,
		Content: string(jsonData),
	})

	return toolResult, nil
}

// performReplace 执行替换操作
func (t *EditTool) performReplace(content string, params EditParams) (string, int, error) {
	oldText := params.OldText
	newText := params.NewText

	replacements := 0

	if params.All {
		// 替换所有匹配项
		if params.CaseSensitive {
			count := strings.Count(content, oldText)
			content = strings.ReplaceAll(content, oldText, newText)
			replacements = count
		} else {
			// 不区分大小写 - 需要逐个替换
			content, replacements = replaceAllCaseInsensitive(content, oldText, newText)
		}
	} else {
		// 只替换第一个匹配项
		if params.CaseSensitive {
			idx := strings.Index(content, oldText)
			if idx >= 0 {
				content = content[:idx] + newText + content[idx+len(oldText):]
				replacements = 1
			}
		} else {
			content, replacements = replaceFirstCaseInsensitive(content, oldText, newText)
		}
	}

	return content, replacements, nil
}

// replaceAllCaseInsensitive 不区分大小写替换所有
func replaceAllCaseInsensitive(content, oldText, newText string) (string, int) {
	count := 0
	lowerContent := strings.ToLower(content)
	lowerOldText := strings.ToLower(oldText)

	for {
		idx := strings.Index(lowerContent, lowerOldText)
		if idx < 0 {
			break
		}
		content = content[:idx] + newText + content[idx+len(oldText):]
		lowerContent = strings.ToLower(content)
		count++
	}

	return content, count
}

// replaceFirstCaseInsensitive 不区分大小写替换第一个
func replaceFirstCaseInsensitive(content, oldText, newText string) (string, int) {
	lowerContent := strings.ToLower(content)
	lowerOldText := strings.ToLower(oldText)

	idx := strings.Index(lowerContent, lowerOldText)
	if idx >= 0 {
		return content[:idx] + newText + content[idx+len(oldText):], 1
	}

	return content, 0
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// EditToolDefinition 返回编辑工具的定义（用于注册）
func EditToolDefinition() tools.ToolDefinition {
	tool := NewEditTool()
	return *tool.Info()
}
