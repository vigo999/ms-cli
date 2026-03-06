package builtin

import (
	"encoding/json"
	"os"

	"github.com/vigo999/ms-cli/tools"
)

// ReadTool 文件读取工具
type ReadTool struct {
	definition *tools.ToolDefinition
}

// ReadParams 读取参数
type ReadParams struct {
	// 文件路径
	Path string `json:"path"`

	// 限制读取的最大字节数（0表示不限制）
	Limit int `json:"limit,omitempty"`

	// 起始偏移量
	Offset int `json:"offset,omitempty"`
}

// NewReadTool 创建新的读取工具
func NewReadTool() tools.ExecutableTool {
	return &ReadTool{
		definition: &tools.ToolDefinition{
			Name:        "read",
			DisplayName: "Read File",
			Description: "Read the contents of a file at the specified path.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to read",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of bytes to read (0 for unlimited)",
						"default":     0,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Start reading from this byte offset",
						"default":     0,
					},
				},
				"required": []string{"path"},
			},
			Meta: tools.ToolMeta{
				Category:   "filesystem",
				Cost:       tools.CostLevelFree,
				ReadOnly:   true,
				Idempotent: true,
				Permissions: []tools.Permission{
					{
						Name:        "file:read",
						Description: "Read file content",
					},
				},
			},
			Version: "1.0.0",
		},
	}
}

// Info 返回工具定义
func (t *ReadTool) Info() *tools.ToolDefinition {
	return t.definition
}

// Execute 执行读取操作
func (t *ReadTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	// 解析参数
	var params ReadParams
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
			Permission: "file:read",
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

	// 读取文件
	content, err := os.ReadFile(params.Path)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to read file: %v", err), nil
	}

	// 应用偏移量和限制
	if params.Offset > 0 {
		if params.Offset >= len(content) {
			return tools.NewErrorResult(tools.ErrCodeInvalidInput, "offset exceeds file size"), nil
		}
		content = content[params.Offset:]
	}

	if params.Limit > 0 && params.Limit < len(content) {
		content = content[:params.Limit]
	}

	// 返回结果
	result := tools.NewTextResult(string(content))
	result.Metadata = tools.ResultMetadata{
		Extra: map[string]interface{}{
			"path":     params.Path,
			"size":     info.Size(),
			"readSize": len(content),
		},
	}

	return result, nil
}

// ReadToolDefinition 返回读取工具的定义（用于注册）
func ReadToolDefinition() tools.ToolDefinition {
	tool := NewReadTool()
	return *tool.Info()
}
