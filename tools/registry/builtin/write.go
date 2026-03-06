package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vigo999/ms-cli/tools"
)

// WriteTool 文件写入工具
type WriteTool struct {
	definition *tools.ToolDefinition
}

// WriteParams 写入参数
type WriteParams struct {
	// 文件路径
	Path string `json:"path"`

	// 文件内容
	Content string `json:"content"`

	// 是否追加模式
	Append bool `json:"append,omitempty"`

	// 文件权限（八进制，如 0644）
	Mode int `json:"mode,omitempty"`
}

// NewWriteTool 创建新的写入工具
func NewWriteTool() tools.ExecutableTool {
	return &WriteTool{
		definition: &tools.ToolDefinition{
			Name:        "write",
			DisplayName: "Write File",
			Description: "Write content to a file at the specified path. Creates the file if it doesn't exist, or overwrites it if it does.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file",
					},
					"append": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, append to the file instead of overwriting",
						"default":     false,
					},
					"mode": map[string]interface{}{
						"type":        "integer",
						"description": "File permissions in octal (default: 0644)",
						"default":     420, // 0644 in decimal
					},
				},
				"required": []string{"path", "content"},
			},
			Meta: tools.ToolMeta{
				Category:   "filesystem",
				Cost:       tools.CostLevelLow,
				ReadOnly:   false,
				Idempotent: false,
				Permissions: []tools.Permission{
					{
						Name:           "file:write",
						Description:    "Write to file",
						RequireConfirm: true,
					},
				},
			},
			Version: "1.0.0",
		},
	}
}

// Info 返回工具定义
func (t *WriteTool) Info() *tools.ToolDefinition {
	return t.definition
}

// Execute 执行写入操作
func (t *WriteTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	// 解析参数
	var params WriteParams
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "invalid parameters: %v", err), nil
	}

	if params.Path == "" {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "path is required"), nil
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

	// 检查是否为目录
	info, err := os.Stat(params.Path)
	if err == nil && info.IsDir() {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "path is a directory: %s", params.Path), nil
	}

	// 确保父目录存在
	dir := filepath.Dir(params.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to create directory: %v", err), nil
	}

	// 设置默认权限
	mode := os.FileMode(0644)
	if params.Mode > 0 {
		mode = os.FileMode(params.Mode)
	}

	// 写入文件
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if params.Append {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}

	file, err := os.OpenFile(params.Path, flag, mode)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to open file: %v", err), nil
	}
	defer file.Close()

	written, err := file.WriteString(params.Content)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to write file: %v", err), nil
	}

	// 返回结果
	result := tools.NewTextResult(fmt.Sprintf("Successfully wrote %d bytes to %s", written, params.Path))
	result.Metadata = tools.ResultMetadata{
		Extra: map[string]interface{}{
			"path":    params.Path,
			"written": written,
		},
	}

	return result, nil
}

// WriteToolDefinition 返回写入工具的定义（用于注册）
func WriteToolDefinition() tools.ToolDefinition {
	tool := NewWriteTool()
	return *tool.Info()
}
