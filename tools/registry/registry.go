package registry

import (
	"fmt"
	"sync"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

// Registry 工具注册表接口
type Registry interface {
	// Register 注册工具
	Register(tool tools.ExecutableTool) error

	// Unregister 注销工具
	Unregister(name string) error

	// Get 获取工具
	Get(name string) (tools.ExecutableTool, bool)

	// GetDefinition 获取工具定义
	GetDefinition(name string) (*tools.ToolDefinition, bool)

	// List 列出所有工具
	List() []tools.ExecutableTool

	// ListDefinitions 列出所有工具定义
	ListDefinitions() []tools.ToolDefinition

	// Names 返回所有工具名称
	Names() []string

	// Count 返回工具数量
	Count() int

	// Clear 清空注册表
	Clear()

	// ToLLMTools 转换为 LLM 工具列表
	ToLLMTools() []llm.Tool
}

// DefaultRegistry 默认注册表实现
type DefaultRegistry struct {
	tools map[string]tools.ExecutableTool
	mu    sync.RWMutex
}

// NewRegistry 创建新的注册表
func NewRegistry() Registry {
	return &DefaultRegistry{
		tools: make(map[string]tools.ExecutableTool),
	}
}

// Register 注册工具
func (r *DefaultRegistry) Register(tool tools.ExecutableTool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	info := tool.Info()
	if info == nil {
		return fmt.Errorf("tool info cannot be nil")
	}

	if info.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[info.Name] = tool
	return nil
}

// Unregister 注销工具
func (r *DefaultRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tools[name]; !ok {
		return fmt.Errorf("tool not found: %s", name)
	}

	delete(r.tools, name)
	return nil
}

// Get 获取工具
func (r *DefaultRegistry) Get(name string) (tools.ExecutableTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// GetDefinition 获取工具定义
func (r *DefaultRegistry) GetDefinition(name string) (*tools.ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok {
		return nil, false
	}

	return tool.Info(), true
}

// List 列出所有工具
func (r *DefaultRegistry) List() []tools.ExecutableTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]tools.ExecutableTool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// ListDefinitions 列出所有工具定义
func (r *DefaultRegistry) ListDefinitions() []tools.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]tools.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, *tool.Info())
	}
	return result
}

// Names 返回所有工具名称
func (r *DefaultRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Count 返回工具数量
func (r *DefaultRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Clear 清空注册表
func (r *DefaultRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = make(map[string]tools.ExecutableTool)
}

// ToLLMTools 转换为 LLM 工具列表
func (r *DefaultRegistry) ToLLMTools() []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]llm.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		info := tool.Info()
		tool := llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        info.Name,
				Description: info.Description,
				Parameters:  convertToToolSchema(info.Parameters),
			},
		}
		result = append(result, tool)
	}
	return result
}

// convertToToolSchema 转换参数为 LLM 工具 schema
func convertToToolSchema(params map[string]interface{}) llm.ToolSchema {
	schema := llm.ToolSchema{
		Type:       "object",
		Properties: make(map[string]llm.Property),
	}

	if params == nil {
		return schema
	}

	if t, ok := params["type"].(string); ok {
		schema.Type = t
	}

	if props, ok := params["properties"].(map[string]interface{}); ok {
		for key, val := range props {
			if propMap, ok := val.(map[string]interface{}); ok {
				prop := llm.Property{Type: "string"}
				if t, ok := propMap["type"].(string); ok {
					prop.Type = t
				}
				if desc, ok := propMap["description"].(string); ok {
					prop.Description = desc
				}
				schema.Properties[key] = prop
			}
		}
	}

	if required, ok := params["required"].([]string); ok {
		schema.Required = required
	}

	return schema
}

// BuiltInRegistry 内置工具注册表
type BuiltInRegistry struct {
	DefaultRegistry
	definitions []tools.ToolDefinition
}

// NewBuiltInRegistry 创建新的内置工具注册表
func NewBuiltInRegistry() *BuiltInRegistry {
	return &BuiltInRegistry{
		DefaultRegistry: DefaultRegistry{
			tools: make(map[string]tools.ExecutableTool),
		},
		definitions: make([]tools.ToolDefinition, 0),
	}
}

// AddDefinition 添加工具定义
func (r *BuiltInRegistry) AddDefinition(def tools.ToolDefinition) *BuiltInRegistry {
	r.definitions = append(r.definitions, def)
	return r
}

// GetDefinitions 获取所有定义
func (r *BuiltInRegistry) GetDefinitions() []tools.ToolDefinition {
	return r.definitions
}

// RegisterFromFactory 使用工厂函数注册工具
func (r *BuiltInRegistry) RegisterFromFactory(name string, factory func() tools.ExecutableTool) error {
	tool := factory()
	return r.Register(tool)
}

// GlobalRegistry 全局注册表实例
var GlobalRegistry = NewRegistry()

// Register 向全局注册表注册工具
func Register(tool tools.ExecutableTool) error {
	return GlobalRegistry.Register(tool)
}

// Unregister 从全局注册表注销工具
func Unregister(name string) error {
	return GlobalRegistry.Unregister(name)
}

// Get 从全局注册表获取工具
func Get(name string) (tools.ExecutableTool, bool) {
	return GlobalRegistry.Get(name)
}

// List 列出全局注册表中的所有工具
func List() []tools.ExecutableTool {
	return GlobalRegistry.List()
}
