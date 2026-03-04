package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// Planner 规划器
type Planner struct {
	provider llm.Provider
	config   PlannerConfig
}

// PlannerConfig 规划器配置
type PlannerConfig struct {
	MaxSteps    int
	Temperature float32
}

// DefaultPlannerConfig 返回默认配置
func DefaultPlannerConfig() PlannerConfig {
	return PlannerConfig{
		MaxSteps:    10,
		Temperature: 0.3, // 较低温度，更确定性
	}
}

// NewPlanner 创建新规划器
func NewPlanner(provider llm.Provider, cfg PlannerConfig) *Planner {
	return &Planner{
		provider: provider,
		config:   cfg,
	}
}

// GeneratePlan 根据任务生成计划
func (p *Planner) GeneratePlan(ctx context.Context, goal string, availableTools []string) (*Plan, error) {
	if goal == "" {
		return nil, fmt.Errorf("goal cannot be empty")
	}

	prompt := GeneratePlanPrompt(goal, availableTools)

	resp, err := p.provider.Complete(ctx, &llm.CompletionRequest{
		Messages:    []llm.Message{llm.NewUserMessage(prompt)},
		Temperature: p.config.Temperature,
		MaxTokens:   2000,
	})
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}

	// 解析计划
	plan, err := p.parsePlan(goal, resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	// 验证计划
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("validate plan: %w", err)
	}

	return plan, nil
}

// parsePlan 解析 LLM 生成的计划
func (p *Planner) parsePlan(goal, content string) (*Plan, error) {
	plan := NewPlan(goal)

	// 尝试提取 JSON
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		// 如果没有 JSON，尝试按行解析
		return p.parsePlanFromLines(goal, content)
	}

	// 解析 JSON
	var steps []struct {
		Description string `json:"description"`
		Tool        string `json:"tool"`
		Params      map[string]any `json:"params,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &steps); err != nil {
		// JSON 解析失败，回退到行解析
		return p.parsePlanFromLines(goal, content)
	}

	// 限制步骤数
	if len(steps) > p.config.MaxSteps {
		steps = steps[:p.config.MaxSteps]
	}

	// 创建步骤
	for _, s := range steps {
		step := plan.AddStep(s.Description)
		if s.Tool != "" {
			step.Tool = s.Tool
		}
		if s.Params != nil {
			step.ToolParams = s.Params
		}
	}

	return plan, nil
}

// parsePlanFromLines 从文本行解析计划
func (p *Planner) parsePlanFromLines(goal, content string) (*Plan, error) {
	plan := NewPlan(goal)

	lines := strings.Split(content, "\n")
	stepNum := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 尝试匹配 "1. Step description" 或 "- Step description" 格式
		if matched := parseStepLine(line); matched != "" {
			if stepNum < p.config.MaxSteps {
				plan.AddStep(matched)
				stepNum++
			}
		}
	}

	return plan, nil
}

// parseStepLine 解析步骤行
func parseStepLine(line string) string {
	// 匹配 "1. Description" 或 "1) Description" 或 "- Description"
	patterns := []string{
		`^\d+[.\)]\s*(.+)$`, // "1. Description" or "1) Description"
		`^-\s*(.+)$`,        // "- Description"
		`^\*\s*(.+)$`,       // "* Description"
		`^Step\s+\d+[:\.]?\s*(.+)$`, // "Step 1: Description"
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(line); matches != nil {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// extractJSON 从文本中提取 JSON
func extractJSON(text string) string {
	// 查找 JSON 数组
	startIdx := strings.Index(text, "[")
	if startIdx == -1 {
		return ""
	}

	// 找到匹配的闭括号
	depth := 0
	endIdx := -1
	for i := startIdx; i < len(text); i++ {
		switch text[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				endIdx = i + 1
				break
			}
		}
		if endIdx != -1 {
			break
		}
	}

	if endIdx == -1 {
		return ""
	}

	return text[startIdx:endIdx]
}

// RefinePlan 优化计划
func (p *Planner) RefinePlan(ctx context.Context, plan *Plan, feedback string) (*Plan, error) {
	prompt := fmt.Sprintf(`Given the following plan and feedback, refine the plan.

Original Goal: %s

Current Plan:
%s

Feedback: %s

Please provide an improved plan in the same JSON format.`,
		plan.Goal, plan.ToMarkdown(), feedback)

	resp, err := p.provider.Complete(ctx, &llm.CompletionRequest{
		Messages:    []llm.Message{llm.NewUserMessage(prompt)},
		Temperature: p.config.Temperature,
		MaxTokens:   2000,
	})
	if err != nil {
		return nil, fmt.Errorf("refine plan: %w", err)
	}

	return p.parsePlan(plan.Goal, resp.Content)
}

// EstimatePlanComplexity 估算计划复杂度
func (p *Planner) EstimatePlanComplexity(plan *Plan) Complexity {
	if len(plan.Steps) <= 3 {
		return ComplexitySimple
	}
	if len(plan.Steps) <= 7 {
		return ComplexityModerate
	}
	return ComplexityComplex
}

// Complexity 复杂度级别
type Complexity int

const (
	ComplexitySimple Complexity = iota
	ComplexityModerate
	ComplexityComplex
)

// String 返回复杂度字符串
func (c Complexity) String() string {
	switch c {
	case ComplexitySimple:
		return "simple"
	case ComplexityModerate:
		return "moderate"
	case ComplexityComplex:
		return "complex"
	default:
		return "unknown"
	}
}

// SuggestExecutionMode 建议执行模式
func (p *Planner) SuggestExecutionMode(plan *Plan) RunMode {
	complexity := p.EstimatePlanComplexity(plan)

	switch complexity {
	case ComplexitySimple:
		return ModeStandard
	case ComplexityModerate:
		return ModePlan
	case ComplexityComplex:
		return ModeReview
	default:
		return ModeStandard
	}
}

// PlanValidator 计划验证器
type PlanValidator struct {
	allowedTools []string
}

// NewPlanValidator 创建计划验证器
func NewPlanValidator(allowedTools []string) *PlanValidator {
	return &PlanValidator{allowedTools: allowedTools}
}

// Validate 验证计划
func (v *PlanValidator) Validate(plan *Plan) []ValidationError {
	var errors []ValidationError

	// 检查步骤是否为空
	if len(plan.Steps) == 0 {
		errors = append(errors, ValidationError{
			Field:   "steps",
			Message: "plan has no steps",
		})
		return errors
	}

	// 检查每个步骤
	for i, step := range plan.Steps {
		// 检查描述是否为空
		if step.Description == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("steps[%d].description", i),
				Message: "step description is empty",
			})
		}

		// 检查工具是否允许
		if step.Tool != "" && !v.isToolAllowed(step.Tool) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("steps[%d].tool", i),
				Message: fmt.Sprintf("tool '%s' is not allowed", step.Tool),
			})
		}

		// 检查依赖是否存在
		for _, depID := range step.DependsOn {
			found := false
			for _, s := range plan.Steps {
				if s.ID == depID {
					found = true
					break
				}
			}
			if !found {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("steps[%d].dependsOn", i),
					Message: fmt.Sprintf("dependency '%s' not found", depID),
				})
			}
		}
	}

	return errors
}

// isToolAllowed 检查工具是否允许
func (v *PlanValidator) isToolAllowed(tool string) bool {
	if len(v.allowedTools) == 0 {
		return true
	}
	for _, t := range v.allowedTools {
		if t == tool {
			return true
		}
	}
	return false
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

// Error 实现 error 接口
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
