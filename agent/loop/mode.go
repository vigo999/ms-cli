package loop

import (
	"strings"
)

// RunMode 运行模式
type RunMode int

const (
	// ModeStandard 标准模式：直接执行
	ModeStandard RunMode = iota
	// ModePlan 计划模式：先制定计划
	ModePlan
	// ModeReview 审核模式：每步需要确认
	ModeReview
)

// String 返回模式名称
func (m RunMode) String() string {
	switch m {
	case ModeStandard:
		return "standard"
	case ModePlan:
		return "plan"
	case ModeReview:
		return "review"
	default:
		return "unknown"
	}
}

// ParseRunMode 解析运行模式
func ParseRunMode(s string) RunMode {
	switch strings.ToLower(s) {
	case "standard", "default":
		return ModeStandard
	case "plan", "planning":
		return ModePlan
	case "review", "confirm":
		return ModeReview
	default:
		return ModeStandard
	}
}

// ModeConfig 模式配置
type ModeConfig struct {
	Mode          RunMode
	PlanConfig    PlanModeConfig
	ReviewConfig  ReviewModeConfig
}

// DefaultModeConfig 返回默认模式配置
func DefaultModeConfig() ModeConfig {
	return ModeConfig{
		Mode: ModeStandard,
		PlanConfig: DefaultPlanModeConfig(),
		ReviewConfig: DefaultReviewModeConfig(),
	}
}

// PlanModeConfig 计划模式配置
type PlanModeConfig struct {
	RequireApproval bool   // 是否需要用户批准计划
	MaxSteps        int    // 最大计划步骤数
	AllowEdit       bool   // 允许用户编辑计划
	AutoExecute     bool   // 批准后自动执行
	ShowProgress    bool   // 显示执行进度
}

// DefaultPlanModeConfig 返回默认计划模式配置
func DefaultPlanModeConfig() PlanModeConfig {
	return PlanModeConfig{
		RequireApproval: true,
		MaxSteps:        10,
		AllowEdit:       true,
		AutoExecute:     true,
		ShowProgress:    true,
	}
}

// ReviewModeConfig 审核模式配置
type ReviewModeConfig struct {
	ConfirmEachStep bool           // 每步确认
	ConfirmTools    []string       // 需要确认的工具
	AutoConfirmRead bool           // 自动确认读操作
	TimeoutSec      int            // 确认超时时间
}

// DefaultReviewModeConfig 返回默认审核模式配置
func DefaultReviewModeConfig() ReviewModeConfig {
	return ReviewModeConfig{
		ConfirmEachStep: true,
		ConfirmTools:    []string{"write", "edit", "shell"},
		AutoConfirmRead: true,
		TimeoutSec:      60,
	}
}

// ModeTransition 模式转换
type ModeTransition struct {
	From   RunMode
	To     RunMode
	Reason string
}

// CanTransition 检查是否可以转换
func (mt ModeTransition) CanTransition() bool {
	// 允许任何转换
	return true
}

// ModeCallback 模式回调接口
type ModeCallback interface {
	// OnPlanCreated 计划创建时调用
	OnPlanCreated(plan *Plan) error

	// OnPlanApproved 计划批准时调用
	OnPlanApproved(plan *Plan) error

	// OnPlanRejected 计划被拒绝时调用
	OnPlanRejected(plan *Plan, reason string) error

	// OnStepStarted 步骤开始时调用
	OnStepStarted(step *PlanStep, index int) error

	// OnStepCompleted 步骤完成时调用
	OnStepCompleted(step *PlanStep, index int, result string) error

	// OnStepNeedsConfirmation 步骤需要确认时调用
	OnStepNeedsConfirmation(step *PlanStep, index int) (bool, error)
}

// DefaultModeCallback 默认模式回调
type DefaultModeCallback struct{}

// OnPlanCreated implements ModeCallback
func (d *DefaultModeCallback) OnPlanCreated(plan *Plan) error { return nil }

// OnPlanApproved implements ModeCallback
func (d *DefaultModeCallback) OnPlanApproved(plan *Plan) error { return nil }

// OnPlanRejected implements ModeCallback
func (d *DefaultModeCallback) OnPlanRejected(plan *Plan, reason string) error { return nil }

// OnStepStarted implements ModeCallback
func (d *DefaultModeCallback) OnStepStarted(step *PlanStep, index int) error { return nil }

// OnStepCompleted implements ModeCallback
func (d *DefaultModeCallback) OnStepCompleted(step *PlanStep, index int, result string) error { return nil }

// OnStepNeedsConfirmation implements ModeCallback
func (d *DefaultModeCallback) OnStepNeedsConfirmation(step *PlanStep, index int) (bool, error) {
	return true, nil
}
