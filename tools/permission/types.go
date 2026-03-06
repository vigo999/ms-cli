package permission

// Action 权限决策
type Action string

const (
	// ActionAllow 允许
	ActionAllow Action = "allow"

	// ActionDeny 拒绝
	ActionDeny Action = "deny"

	// ActionAsk 询问用户
	ActionAsk Action = "ask"
)

// IsAllowed 是否允许
func (a Action) IsAllowed() bool {
	return a == ActionAllow
}

// IsDenied 是否被拒绝
func (a Action) IsDenied() bool {
	return a == ActionDeny
}

// NeedsAsk 是否需要询问
func (a Action) NeedsAsk() bool {
	return a == ActionAsk
}

// CheckLevel 检查层级 - 用于避免重复检查
type CheckLevel int

const (
	// CheckLevelResolver Resolver层静态检查
	CheckLevelResolver CheckLevel = iota

	// CheckLevelExecution Execution层动态检查
	CheckLevelExecution
)

func (c CheckLevel) String() string {
	switch c {
	case CheckLevelResolver:
		return "resolver"
	case CheckLevelExecution:
		return "execution"
	default:
		return "unknown"
	}
}

// Rule 权限规则
type Rule struct {
	// 规则ID
	ID string `json:"id,omitempty"`

	// 权限模式，如 "file:*" 或 "bash:execute"
	Permission string `json:"permission"`

	// 目标模式（如文件路径模式）
	Pattern string `json:"pattern,omitempty"`

	// 决策动作
	Action Action `json:"action"`

	// 适用的Agent类型（空表示适用所有）
	AgentTypes []string `json:"agentTypes,omitempty"`

	// 优先级（数字越大优先级越高）
	Priority int `json:"priority,omitempty"`

	// 是否启用
	Enabled bool `json:"enabled,omitempty"`

	// 规则描述
	Description string `json:"description,omitempty"`
}

// Ruleset 规则集合
type Ruleset struct {
	// 规则集版本
	Version string `json:"version"`

	// 规则列表
	Rules []Rule `json:"rules"`

	// 默认决策（当没有规则匹配时）
	DefaultAction Action `json:"defaultAction,omitempty"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// EvaluateResult 评估结果
type EvaluateResult struct {
	// 决策动作
	Action Action `json:"action"`

	// 匹配的规则ID
	RuleID string `json:"ruleId,omitempty"`

	// 匹配的规则
	MatchedRule *Rule `json:"-"`

	// 是否默认决策
	IsDefault bool `json:"isDefault,omitempty"`
}

// MatchInfo 匹配信息
type MatchInfo struct {
	// 是否匹配
	Matched bool

	// 匹配的模式
	Pattern string

	// 匹配的权限
	Permission string

	// 匹配度（用于排序）
	Score int
}

// NewRuleset 创建新的规则集
func NewRuleset() *Ruleset {
	return &Ruleset{
		Version:       "1.0",
		Rules:         make([]Rule, 0),
		DefaultAction: ActionAsk,
	}
}

// AddRule 添加规则
func (r *Ruleset) AddRule(rule Rule) *Ruleset {
	r.Rules = append(r.Rules, rule)
	return r
}

// SetDefaultAction 设置默认决策
func (r *Ruleset) SetDefaultAction(action Action) *Ruleset {
	r.DefaultAction = action
	return r
}

// GetEnabledRules 获取启用的规则
func (r *Ruleset) GetEnabledRules() []Rule {
	rules := make([]Rule, 0)
	for _, rule := range r.Rules {
		if rule.Enabled || rule.ID == "" {
			rules = append(rules, rule)
		}
	}
	return rules
}
