package permission

import (
	"fmt"
	"sync"
	"time"

	"github.com/vigo999/ms-cli/tools/events"
)

// Engine 权限引擎接口
type Engine interface {
	// Evaluate 评估权限请求
	Evaluate(permission, pattern, agentType string) EvaluateResult

	// Ask 权限检查入口
	Ask(req PermissionRequest) error

	// CanExecute 统一检查方法
	CanExecute(permission, pattern, agentType string) EvaluateResult

	// GetRuleset 获取规则集
	GetRuleset() *Ruleset

	// UpdateRuleset 更新规则集
	UpdateRuleset(ruleset *Ruleset)

	// Reload 重新加载配置
	Reload() error

	// Close 关闭引擎
	Close() error

	// SetDefaultTimeout 设置默认超时（仅DefaultEngine实现）
	SetDefaultTimeout(timeout time.Duration)
}

// DefaultEngine 默认权限引擎实现
type DefaultEngine struct {
	// 配置
	config *Config

	// 规则集
	ruleset *Ruleset

	// 评估缓存
	cache *EvaluationCache

	// 匹配缓存
	matchCache *MatchCache

	// 等待中的权限请求
	pendingRequests map[string]chan PermissionResponse

	// 请求锁
	pendingMu sync.RWMutex

	// 是否已关闭
	closed bool

	// 关闭通道
	closeCh chan struct{}

	// 事件总线（可选，用于异步权限确认）
	eventBus events.EventBus

	// 默认权限确认超时
	defaultTimeout time.Duration
}

// NewEngine 创建新的权限引擎
func NewEngine(config *Config) (Engine, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	engine := &DefaultEngine{
		config:          config,
		ruleset:         config.Ruleset,
		cache:           NewEvaluationCache(config.Cache),
		matchCache:      NewMatchCache(config.Cache.MaxSize),
		pendingRequests: make(map[string]chan PermissionResponse),
		closeCh:         make(chan struct{}),
		defaultTimeout:  60 * time.Second,
	}

	return engine, nil
}

// SetEventBus 设置事件总线
func (e *DefaultEngine) SetEventBus(bus events.EventBus) {
	e.eventBus = bus
}

// SetDefaultTimeout 设置默认超时
func (e *DefaultEngine) SetDefaultTimeout(timeout time.Duration) {
	e.defaultTimeout = timeout
}

// Evaluate 评估权限请求（后匹配优先）
func (e *DefaultEngine) Evaluate(permission, pattern, agentType string) EvaluateResult {
	if e.closed {
		return EvaluateResult{Action: ActionDeny, IsDefault: true}
	}

	// 检查缓存
	cacheKey := CacheKey{Permission: permission, Pattern: pattern, AgentType: agentType}
	if result, ok := e.cache.Get(cacheKey); ok {
		return result
	}

	var matchedRule *Rule

	// 遍历所有启用的规则，后匹配优先
	rules := e.ruleset.GetEnabledRules()
	for i := len(rules) - 1; i >= 0; i-- {
		rule := rules[i]

		// 检查Agent类型
		if len(rule.AgentTypes) > 0 && !contains(rule.AgentTypes, agentType) {
			continue
		}

		// 检查权限匹配
		if !e.matchPermission(rule.Permission, permission) {
			continue
		}

		// 检查模式匹配（如果规则有指定模式）
		if rule.Pattern != "" && !e.matchPattern(rule.Pattern, pattern) {
			continue
		}

		// 找到匹配的规则
		matchedRule = &rule
		break
	}

	var result EvaluateResult
	if matchedRule != nil {
		result = EvaluateResult{
			Action:      matchedRule.Action,
			RuleID:      matchedRule.ID,
			MatchedRule: matchedRule,
			IsDefault:   false,
		}
	} else {
		result = EvaluateResult{
			Action:    e.ruleset.DefaultAction,
			IsDefault: true,
		}
	}

	// 缓存结果
	e.cache.Set(cacheKey, result)

	return result
}

// Ask 权限检查入口
func (e *DefaultEngine) Ask(req PermissionRequest) error {
	if e.closed {
		return &PermissionError{Code: ErrCodeEngineNotReady, Message: "engine is closed"}
	}

	// 检查所有模式
	patternsNeedAsk := make([]string, 0)
	for _, pattern := range req.Patterns {
		result := e.Evaluate(req.Permission, pattern, "")

		switch result.Action {
		case ActionDeny:
			return NewDeniedError(req.Permission, req.Patterns)

		case ActionAsk:
			patternsNeedAsk = append(patternsNeedAsk, pattern)

		case ActionAllow:
			// 继续检查其他模式
			continue
		}
	}

	// 有需要询问的模式
	if len(patternsNeedAsk) > 0 {
		// 如果静默模式，直接返回等待错误
		if req.Silent {
			return NewPendingError(req.Permission, patternsNeedAsk)
		}

		// 如果有事件总线，使用异步确认
		if e.eventBus != nil {
			return e.waitForPermission(req, patternsNeedAsk)
		}

		// 没有事件总线，返回待处理错误
		return NewPendingError(req.Permission, patternsNeedAsk)
	}

	return nil
}

// waitForPermission 等待权限确认
func (e *DefaultEngine) waitForPermission(req PermissionRequest, patterns []string) error {
	requestID := req.ID
	if requestID == "" {
		requestID = generateRequestID()
	}

	// 创建响应通道
	responseCh := make(chan PermissionResponse, 1)

	// 注册到等待列表
	e.pendingMu.Lock()
	e.pendingRequests[requestID] = responseCh
	e.pendingMu.Unlock()

	// 清理
	defer func() {
		e.pendingMu.Lock()
		delete(e.pendingRequests, requestID)
		e.pendingMu.Unlock()
		close(responseCh)
	}()

	// 发布权限请求事件
	event := &PermissionRequestedEvent{
		RequestID:  requestID,
		SessionID:  req.SessionID,
		ToolID:     req.ToolID,
		CallID:     req.CallID,
		Permission: req.Permission,
		Patterns:   patterns,
		Metadata:   req.Metadata,
		Timestamp:  time.Now(),
		ResponseCh: responseCh,
	}

	if err := e.eventBus.Publish(event.ToEvent()); err != nil {
		return fmt.Errorf("failed to publish permission request: %w", err)
	}

	// 等待响应或超时
	timeout := e.defaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	select {
	case response := <-responseCh:
		switch response.Action {
		case ActionAllow:
			return nil
		case ActionDeny:
			return NewDeniedError(req.Permission, patterns)
		default:
			return NewPendingError(req.Permission, patterns)
		}
	case <-time.After(timeout):
		return NewTimeoutError(req.Permission)
	case <-e.closeCh:
		return NewCancelledError(req.Permission)
	}
}

// HandlePermissionResponse 处理权限响应（由UI层调用）
func (e *DefaultEngine) HandlePermissionResponse(requestID string, response PermissionResponse) error {
	e.pendingMu.RLock()
	ch, ok := e.pendingRequests[requestID]
	e.pendingMu.RUnlock()

	if !ok {
		return fmt.Errorf("permission request not found: %s", requestID)
	}

	select {
	case ch <- response:
		return nil
	default:
		return fmt.Errorf("permission response channel is full")
	}
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("perm_%d", time.Now().UnixNano())
}

// CanExecute 统一检查方法
func (e *DefaultEngine) CanExecute(permission, pattern, agentType string) EvaluateResult {
	return e.Evaluate(permission, pattern, agentType)
}

// GetRuleset 获取规则集
func (e *DefaultEngine) GetRuleset() *Ruleset {
	return e.ruleset
}

// UpdateRuleset 更新规则集
func (e *DefaultEngine) UpdateRuleset(ruleset *Ruleset) {
	e.ruleset = ruleset
	// 清除缓存
	e.cache.Clear()
	e.matchCache.Clear()
}

// Reload 重新加载配置
func (e *DefaultEngine) Reload() error {
	if e.config.StoragePath == "" {
		return nil
	}

	config, err := LoadConfig(e.config.StoragePath)
	if err != nil {
		return err
	}

	if err := config.Validate(); err != nil {
		return err
	}

	e.config = config
	e.UpdateRuleset(config.Ruleset)
	return nil
}

// Close 关闭引擎
func (e *DefaultEngine) Close() error {
	if e.closed {
		return nil
	}

	e.closed = true
	close(e.closeCh)
	e.cache.Stop()
	return nil
}

// matchPermission 匹配权限
func (e *DefaultEngine) matchPermission(rulePattern, permission string) bool {
	return CompileWildcard(rulePattern).Match(permission)
}

// matchPattern 匹配目标模式
func (e *DefaultEngine) matchPattern(rulePattern, target string) bool {
	// 检查匹配缓存
	if result, ok := e.matchCache.Get(rulePattern, target); ok {
		return result
	}

	result := CompileWildcard(rulePattern).Match(target)
	e.matchCache.Set(rulePattern, target, result)
	return result
}

// contains 检查字符串切片是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SimpleEngine 简单权限引擎（用于测试）
type SimpleEngine struct {
	defaultAction Action
	rules         []Rule
}

// NewSimpleEngine 创建简单权限引擎
func NewSimpleEngine(defaultAction Action) Engine {
	return &SimpleEngine{
		defaultAction: defaultAction,
		rules:         make([]Rule, 0),
	}
}

// Evaluate 评估权限
func (e *SimpleEngine) Evaluate(permission, pattern, agentType string) EvaluateResult {
	for i := len(e.rules) - 1; i >= 0; i-- {
		rule := e.rules[i]
		if CompileWildcard(rule.Permission).Match(permission) {
			if rule.Pattern == "" || CompileWildcard(rule.Pattern).Match(pattern) {
				return EvaluateResult{Action: rule.Action, RuleID: rule.ID}
			}
		}
	}
	return EvaluateResult{Action: e.defaultAction, IsDefault: true}
}

// Ask 权限检查
func (e *SimpleEngine) Ask(req PermissionRequest) error {
	for _, pattern := range req.Patterns {
		result := e.Evaluate(req.Permission, pattern, "")
		if result.Action == ActionDeny {
			return NewDeniedError(req.Permission, req.Patterns)
		}
		if result.Action == ActionAsk {
			return NewPendingError(req.Permission, req.Patterns)
		}
	}
	return nil
}

// CanExecute 检查是否可以执行
func (e *SimpleEngine) CanExecute(permission, pattern, agentType string) EvaluateResult {
	return e.Evaluate(permission, pattern, agentType)
}

// GetRuleset 获取规则集
func (e *SimpleEngine) GetRuleset() *Ruleset {
	return &Ruleset{
		Version:       "1.0",
		Rules:         e.rules,
		DefaultAction: e.defaultAction,
	}
}

// UpdateRuleset 更新规则集
func (e *SimpleEngine) UpdateRuleset(ruleset *Ruleset) {
	e.rules = ruleset.Rules
	e.defaultAction = ruleset.DefaultAction
}

// Reload 重新加载
func (e *SimpleEngine) Reload() error {
	return nil
}

// Close 关闭
func (e *SimpleEngine) Close() error {
	return nil
}

// SetDefaultTimeout 设置默认超时（SimpleEngine无操作）
func (e *SimpleEngine) SetDefaultTimeout(timeout time.Duration) {
}
