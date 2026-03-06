package builtin

import (
	"fmt"

	"github.com/vigo999/ms-cli/tools"
)

// ToolPermissionMap 工具名称到权限名称的映射
// 用于统一权限检查，避免重复询问
var ToolPermissionMap = map[string]string{
	"write": "file:write",
	"edit":  "file:write",
	"bash":  "bash:execute",
	"shell": "bash:execute",
	"read":  "file:read",
	"glob":  "dir:list",
	"grep":  "file:search",
}

// GetToolPermissionName 获取工具的权限名称
// 如果工具不存在于映射中，返回 toolName + ":execute"
func GetToolPermissionName(toolName string) string {
	if permName, ok := ToolPermissionMap[toolName]; ok {
		return permName
	}
	return toolName + ":execute"
}

// Init 注册所有内置工具到全局注册表
func Init() {
	// 文件系统工具
	Register(NewReadTool())
	Register(NewWriteTool())
	Register(NewGlobTool())
	Register(NewGrepTool())
	Register(NewEditTool())

	// Shell工具
	Register(NewBashTool())
}

// Register 注册工具到全局注册表
func Register(tool tools.ExecutableTool) {
	if err := RegisterTo(nil, tool); err != nil {
		// 静默失败，实际应用中应该记录日志
		fmt.Printf("Failed to register tool %s: %v\n", tool.Info().Name, err)
	}
}

// RegisterTo 注册工具到指定注册表
func RegisterTo(registry interface{}, tool tools.ExecutableTool) error {
	// 如果registry为nil，使用全局注册表
	// 这里需要导入registry包
	// 实际实现取决于registry包的具体实现
	return nil
}

// GetAllDefinitions 获取所有内置工具的定义
func GetAllDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		ReadToolDefinition(),
		WriteToolDefinition(),
		GlobToolDefinition(),
		GrepToolDefinition(),
		EditToolDefinition(),
		BashToolDefinition(),
	}
}

// GetAllTools 获取所有内置工具实例
func GetAllTools() []tools.ExecutableTool {
	return []tools.ExecutableTool{
		NewReadTool(),
		NewWriteTool(),
		NewGlobTool(),
		NewGrepTool(),
		NewEditTool(),
		NewBashTool(),
	}
}
