package adapter

import (
	"github.com/vigo999/ms-cli/tools"
)

// Adapter 协议适配器接口
type Adapter interface {
	// ToProviderFormat 将ToolResult转换为协议特定格式
	ToProviderFormat(result *tools.ToolResult) (interface{}, error)

	// FromProviderFormat 从协议特定格式转换为ToolResult
	FromProviderFormat(data interface{}) (*tools.ToolResult, error)

	// GetProviderName 获取协议名称
	GetProviderName() string
}

// BaseAdapter 基础适配器
type BaseAdapter struct {
	providerName string
}

// GetProviderName 获取协议名称
func (a *BaseAdapter) GetProviderName() string {
	return a.providerName
}
