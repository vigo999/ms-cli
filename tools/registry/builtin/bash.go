package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/tools"
)

// BashTool Bash命令执行工具
type BashTool struct {
	definition *tools.ToolDefinition
}

// BashParams Bash命令参数
type BashParams struct {
	// 要执行的命令
	Command string `json:"command"`

	// 工作目录
	WorkingDir string `json:"workingDir,omitempty"`

	// 环境变量
	Env map[string]string `json:"env,omitempty"`

	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`

	// 是否返回stderr
	CaptureStderr bool `json:"captureStderr,omitempty"`
}

// BashResult Bash命令结果
type BashResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exitCode"`
}

// NewBashTool 创建新的Bash工具
func NewBashTool() tools.ExecutableTool {
	return &BashTool{
		definition: &tools.ToolDefinition{
			Name:        "bash",
			DisplayName: "Execute Bash Command",
			Description: "Execute a bash command in the shell. Use with caution as this can modify the system.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The bash command to execute",
					},
					"workingDir": map[string]interface{}{
						"type":        "string",
						"description": "Working directory for command execution",
					},
					"env": map[string]interface{}{
						"type":        "object",
						"description": "Environment variables to set",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
					},
					"timeout": map[string]interface{}{
						"type":        "integer",
						"description": "Command timeout in seconds (default: 60)",
						"default":     60,
					},
					"captureStderr": map[string]interface{}{
						"type":        "boolean",
						"description": "Include stderr in the output",
						"default":     true,
					},
				},
				"required": []string{"command"},
			},
			Meta: tools.ToolMeta{
				Category:   "shell",
				Cost:       tools.CostLevelHigh,
				ReadOnly:   false,
				Idempotent: false,
				Timeout:    60,
				Permissions: []tools.Permission{
					{
						Name:           "bash:execute",
						Description:    "Execute bash commands",
						RequireConfirm: true,
					},
				},
			},
			Version: "1.0.0",
		},
	}
}

// Info 返回工具定义
func (t *BashTool) Info() *tools.ToolDefinition {
	return t.definition
}

// Execute 执行Bash命令
func (t *BashTool) Execute(ctx *tools.ToolContext, executor tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	// 解析参数
	var params BashParams
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "invalid parameters: %v", err), nil
	}

	if strings.TrimSpace(params.Command) == "" {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "command is required"), nil
	}

	// 安全检查：禁止危险命令
	if isDangerousCommand(params.Command) {
		return tools.NewErrorResult(
			tools.ErrCodePermissionDenied,
			"command contains potentially dangerous operations",
		), nil
	}

	// 执行层权限检查
	if executor != nil {
		req := tools.PermissionRequest{
			ID:         ctx.CallID,
			SessionID:  ctx.SessionID,
			ToolID:     ctx.ToolID,
			CallID:     ctx.CallID,
			Permission: "bash:execute",
			Patterns:   []string{params.Command},
			CheckLevel: tools.CheckLevelExecution,
			Metadata: map[string]interface{}{
				"command": params.Command,
			},
		}
		if err := executor.AskPermission(req); err != nil {
			return tools.NewErrorResult(tools.ErrCodePermissionDenied, "permission denied: %v", err), nil
		}
	}

	// 设置超时
	timeout := params.Timeout
	if timeout <= 0 {
		timeout = 60
	}

	execCtx, cancel := context.WithTimeout(ctx.ContextOrBackground(), time.Duration(timeout)*time.Second)
	defer cancel()

	// 创建命令
	cmd := exec.CommandContext(execCtx, "bash", "-c", params.Command)

	// 设置工作目录
	if params.WorkingDir != "" {
		cmd.Dir = params.WorkingDir
	}

	// 设置环境变量
	if len(params.Env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range params.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// 执行命令
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// 构建结果
	result := BashResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return tools.NewErrorResult(tools.ErrCodeTimeout, "command execution timed out after %d seconds", timeout), nil
		}
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "command execution failed: %v", err), nil
	}

	// 构建输出
	output := result.Stdout
	if params.CaptureStderr && result.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += "[stderr]\n" + result.Stderr
	}

	toolResult := tools.NewTextResult(output)
	toolResult.Metadata = tools.ResultMetadata{
		Extra: map[string]interface{}{
			"exitCode": result.ExitCode,
			"command":  params.Command,
		},
	}

	return toolResult, nil
}

// isDangerousCommand 检查命令是否包含危险操作
func isDangerousCommand(command string) bool {
	// 转换为小写进行检查
	lower := strings.ToLower(command)

	// 定义危险命令模式
	dangerousPatterns := []string{
		"rm -rf /",
		"> /dev/sda",
		"mkfs",
		"dd if=/dev/zero",
		":(){ :|:& };:", // fork bomb
		"chmod -R 777 /",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// BashToolDefinition 返回Bash工具的定义（用于注册）
func BashToolDefinition() tools.ToolDefinition {
	tool := NewBashTool()
	return *tool.Info()
}
