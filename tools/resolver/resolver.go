package resolver

import (
	"context"
	"fmt"

	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/permission"
	"github.com/vigo999/ms-cli/tools/registry/builtin"
)

// Resolver 工具解析器接口
type Resolver interface {
	// Resolve 解析可用工具列表（含权限预过滤）
	Resolve(ctx context.Context, input ResolveInput) (ResolveOutput, error)

	// ResolveTool 根据ToolDefinition解析可执行工具
	ResolveTool(def tools.ToolDefinition) tools.ExecutableTool

	// Register 注册可执行工具
	Register(name string, tool tools.ExecutableTool) error

	// Unregister 注销工具
	Unregister(name string) error

	// Get 获取指定工具
	Get(name string) (tools.ExecutableTool, bool)

	// List 列出所有可用工具
	List() []string
}

// DefaultResolver 默认解析器实现
type DefaultResolver struct {
	// 已注册的工具
	tools map[string]tools.ExecutableTool

	// 权限引擎
	permEngine permission.Engine
}

// NewResolver 创建新的解析器
func NewResolver(permEngine permission.Engine) Resolver {
	return &DefaultResolver{
		tools:      make(map[string]tools.ExecutableTool),
		permEngine: permEngine,
	}
}

// Resolve 解析可用工具列表
func (r *DefaultResolver) Resolve(ctx context.Context, input ResolveInput) (ResolveOutput, error) {
	result := ResolveOutput{
		Tools:           make(map[string]tools.ExecutableTool),
		ProviderSchemas: make([]ProviderToolSchema, 0),
	}

	// 处理每个工具定义
	for _, def := range input.Definitions {
		// Resolver层权限预过滤
		if !r.checkToolPermission(def, input.Agent.Type) {
			continue
		}

		// 获取或创建可执行工具
		var execTool tools.ExecutableTool
		if existing, ok := r.tools[def.Name]; ok {
			execTool = existing
		} else {
			execTool = r.ResolveTool(def)
			if execTool == nil {
				continue // 跳过未找到的工具
			}
			r.tools[def.Name] = execTool
		}

		// 添加到结果
		result.Tools[def.Name] = execTool

		// 转换为协议Schema
		schema, err := r.convertToProviderSchema(def, input.Provider)
		if err != nil {
			// 记录错误但继续处理其他工具
			continue
		}
		result.ProviderSchemas = append(result.ProviderSchemas, schema)
	}

	return result, nil
}

// ResolveTool 根据ToolDefinition解析可执行工具
func (r *DefaultResolver) ResolveTool(def tools.ToolDefinition) tools.ExecutableTool {
	// 先查注册表
	if tool, ok := r.tools[def.Name]; ok {
		return tool
	}
	// 再查内置工具
	for _, tool := range builtin.GetAllTools() {
		if tool.Info().Name == def.Name {
			return tool
		}
	}
	// 未找到时返回nil
	return nil
}

// Register 注册可执行工具
func (r *DefaultResolver) Register(name string, tool tools.ExecutableTool) error {
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	r.tools[name] = tool
	return nil
}

// Unregister 注销工具
func (r *DefaultResolver) Unregister(name string) error {
	if _, ok := r.tools[name]; !ok {
		return &ResolutionError{
			Type:     ErrorTypeNotFound,
			Message:  "tool not found",
			ToolName: name,
		}
	}
	delete(r.tools, name)
	return nil
}

// Get 获取指定工具
func (r *DefaultResolver) Get(name string) (tools.ExecutableTool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List 列出所有可用工具
func (r *DefaultResolver) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// checkToolPermission Resolver层权限检查（粗粒度）
func (r *DefaultResolver) checkToolPermission(def tools.ToolDefinition, agentType string) bool {
	if r.permEngine == nil {
		return true
	}

	// 检查工具类型权限
	for _, perm := range def.Meta.Permissions {
		result := r.permEngine.CanExecute(perm.Name, "*", agentType)
		if result.Action == permission.ActionDeny {
			return false
		}
	}

	return true
}

// convertToProviderSchema 转换为协议Schema
func (r *DefaultResolver) convertToProviderSchema(def tools.ToolDefinition, provider ProviderType) (ProviderToolSchema, error) {
	switch provider {
	case ProviderTypeMCP:
		return r.toMCPSchema(def)
	case ProviderTypeOpenAI:
		return r.toOpenAISchema(def)
	case ProviderTypeInternal, "":
		return r.toInternalSchema(def)
	default:
		return ProviderToolSchema{}, &ResolutionError{
			Type:    ErrorTypeInvalidSchema,
			Message: fmt.Sprintf("unsupported provider type: %s", provider),
		}
	}
}

// toMCPSchema 转换为MCP Schema
func (r *DefaultResolver) toMCPSchema(def tools.ToolDefinition) (ProviderToolSchema, error) {
	return ProviderToolSchema{
		Name:        def.Name,
		Description: def.Description,
		Parameters:  def.Parameters,
		Definition:  &def,
		Provider:    ProviderTypeMCP,
	}, nil
}

// toOpenAISchema 转换为OpenAI Schema
func (r *DefaultResolver) toOpenAISchema(def tools.ToolDefinition) (ProviderToolSchema, error) {
	// OpenAI使用特定的function calling格式
	parameters := def.Parameters
	if parameters == nil {
		parameters = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	return ProviderToolSchema{
		Name:        def.Name,
		Description: def.Description,
		Parameters:  parameters,
		Definition:  &def,
		Provider:    ProviderTypeOpenAI,
	}, nil
}

// toInternalSchema 转换为内部Schema
func (r *DefaultResolver) toInternalSchema(def tools.ToolDefinition) (ProviderToolSchema, error) {
	return ProviderToolSchema{
		Name:        def.Name,
		Description: def.Description,
		Parameters:  def.Parameters,
		Definition:  &def,
		Provider:    ProviderTypeInternal,
	}, nil
}

// SimpleResolver 简单解析器（用于测试）
type SimpleResolver struct {
	tools map[string]tools.ExecutableTool
}

// NewSimpleResolver 创建简单解析器
func NewSimpleResolver() Resolver {
	return &SimpleResolver{
		tools: make(map[string]tools.ExecutableTool),
	}
}

// Resolve 解析
func (r *SimpleResolver) Resolve(ctx context.Context, input ResolveInput) (ResolveOutput, error) {
	result := ResolveOutput{
		Tools:           r.tools,
		ProviderSchemas: make([]ProviderToolSchema, 0),
	}

	for _, def := range input.Definitions {
		schema, _ := r.convertToProviderSchema(def, input.Provider)
		result.ProviderSchemas = append(result.ProviderSchemas, schema)
	}

	return result, nil
}

// ResolveTool 根据ToolDefinition解析可执行工具
func (r *SimpleResolver) ResolveTool(def tools.ToolDefinition) tools.ExecutableTool {
	// 查注册表
	if tool, ok := r.tools[def.Name]; ok {
		return tool
	}
	// SimpleResolver 不支持内置工具，返回 nil
	return nil
}

// Register 注册
func (r *SimpleResolver) Register(name string, tool tools.ExecutableTool) error {
	r.tools[name] = tool
	return nil
}

// Unregister 注销
func (r *SimpleResolver) Unregister(name string) error {
	delete(r.tools, name)
	return nil
}

// Get 获取
func (r *SimpleResolver) Get(name string) (tools.ExecutableTool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List 列表
func (r *SimpleResolver) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// convertToProviderSchema 转换
func (r *SimpleResolver) convertToProviderSchema(def tools.ToolDefinition, provider ProviderType) (ProviderToolSchema, error) {
	return ProviderToolSchema{
		Name:        def.Name,
		Description: def.Description,
		Parameters:  def.Parameters,
		Definition:  &def,
		Provider:    provider,
	}, nil
}
