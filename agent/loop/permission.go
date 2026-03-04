package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vigo999/ms-cli/configs"
)

// PermissionLevel represents the permission level for a tool.
type PermissionLevel int

const (
	// PermissionDeny always denies the tool.
	PermissionDeny PermissionLevel = iota
	// PermissionAsk asks user for permission each time.
	PermissionAsk
	// PermissionAllowOnce allows once without asking.
	PermissionAllowOnce
	// PermissionAllowSession allows for the current session.
	PermissionAllowSession
	// PermissionAllowAlways always allows.
	PermissionAllowAlways
)

// String returns the string representation.
func (p PermissionLevel) String() string {
	switch p {
	case PermissionDeny:
		return "deny"
	case PermissionAsk:
		return "ask"
	case PermissionAllowOnce:
		return "allow_once"
	case PermissionAllowSession:
		return "allow_session"
	case PermissionAllowAlways:
		return "allow_always"
	default:
		return "unknown"
	}
}

// ParsePermissionLevel parses a permission level string.
func ParsePermissionLevel(s string) PermissionLevel {
	switch strings.ToLower(s) {
	case "deny":
		return PermissionDeny
	case "ask":
		return PermissionAsk
	case "allow_once":
		return PermissionAllowOnce
	case "allow_session":
		return PermissionAllowSession
	case "allow_always", "allow":
		return PermissionAllowAlways
	default:
		return PermissionAsk
	}
}

// PermissionService controls tool-call permissions.
type PermissionService interface {
	// Request requests permission to execute a tool.
	// Returns (granted, error).
	Request(ctx context.Context, tool, action, path string) (bool, error)

	// Check checks the current permission level without triggering interaction.
	Check(tool, action string) PermissionLevel

	// CheckCommand checks permission for a specific command.
	CheckCommand(command string) PermissionLevel

	// CheckPath checks permission for a specific path.
	CheckPath(path string) PermissionLevel

	// Grant grants permission for a tool.
	Grant(tool string, level PermissionLevel)

	// GrantCommand grants permission for a specific command.
	GrantCommand(command string, level PermissionLevel)

	// GrantPath grants permission for a specific path pattern.
	GrantPath(pattern string, level PermissionLevel)

	// Revoke revokes permission for a tool.
	Revoke(tool string)

	// RevokeCommand revokes permission for a specific command.
	RevokeCommand(command string)

	// RevokePath revokes permission for a specific path pattern.
	RevokePath(pattern string)
}

// DefaultPermissionService is the default permission service implementation.
type DefaultPermissionService struct {
	mu              sync.RWMutex
	policies        map[string]PermissionLevel
	commandPolicies map[string]PermissionLevel
	pathPatterns    []PathPermission
	default_        PermissionLevel
	skipAsk         bool
	ui              PermissionUI
	store           PermissionStore
}

// PathPermission 路径权限
type PathPermission struct {
	Pattern string
	Level   PermissionLevel
}

// PermissionUI is the interface for permission UI interaction.
type PermissionUI interface {
	// RequestPermission asks the user for permission.
	// Returns (granted, remember, error).
	RequestPermission(tool, action, path string) (bool, bool, error)
}

// NewDefaultPermissionService creates a new permission service.
func NewDefaultPermissionService(cfg configs.PermissionsConfig) *DefaultPermissionService {
	svc := &DefaultPermissionService{
		policies:        make(map[string]PermissionLevel),
		commandPolicies: make(map[string]PermissionLevel),
		pathPatterns:    make([]PathPermission, 0),
		default_:        ParsePermissionLevel(cfg.DefaultLevel),
		skipAsk:         cfg.SkipRequests,
	}

	// Load tool policies
	for tool, level := range cfg.ToolPolicies {
		svc.policies[tool] = ParsePermissionLevel(level)
	}

	// Load allowed tools as allow_always
	for _, tool := range cfg.AllowedTools {
		svc.policies[tool] = PermissionAllowAlways
	}

	// Load blocked tools as deny
	for _, tool := range cfg.BlockedTools {
		svc.policies[tool] = PermissionDeny
	}

	return svc
}

// SetUI sets the permission UI.
func (s *DefaultPermissionService) SetUI(ui PermissionUI) {
	s.ui = ui
}

// SetStore sets the permission store.
func (s *DefaultPermissionService) SetStore(store PermissionStore) {
	s.store = store
}

// Request requests permission.
func (s *DefaultPermissionService) Request(ctx context.Context, tool, action, path string) (bool, error) {
	// 1. 检查工具级别权限
	level := s.Check(tool, action)

	// 2. 检查命令级别权限（如果是 shell 工具）
	if tool == "shell" && action != "" {
		cmdLevel := s.CheckCommand(action)
		if cmdLevel < level {
			level = cmdLevel
		}
	}

	// 3. 检查路径级别权限
	if path != "" {
		pathLevel := s.CheckPath(path)
		if pathLevel < level {
			level = pathLevel
		}
	}

	switch level {
	case PermissionDeny:
		return false, fmt.Errorf("tool %q is blocked", tool)

	case PermissionAllowAlways, PermissionAllowSession:
		return true, nil

	case PermissionAllowOnce:
		// Grant once then revert to ask
		s.Grant(tool, PermissionAsk)
		return true, nil

	case PermissionAsk:
		if s.skipAsk {
			return true, nil
		}

		// Interactive permission request
		if s.ui != nil {
			granted, remember, err := s.ui.RequestPermission(tool, action, path)
			if err != nil {
				return false, err
			}

			if granted && remember {
				s.Grant(tool, PermissionAllowSession)
				// 持久化决策
				if s.store != nil {
					s.store.SaveDecision(PermissionDecision{
						Tool:      tool,
						Action:    action,
						Path:      path,
						Level:     PermissionAllowSession,
						Timestamp: time.Now(),
					})
				}
			}

			return granted, nil
		}

		// No UI, default to allow
		return true, nil
	}

	return false, fmt.Errorf("unknown permission level")
}

// Check checks the permission level.
func (s *DefaultPermissionService) Check(tool, action string) PermissionLevel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check specific tool policy
	if level, ok := s.policies[tool]; ok {
		return level
	}

	// Check read/write patterns
	if tool == "read" || tool == "glob" {
		return PermissionAllowAlways
	}

	// Check destructive commands
	if tool == "write" || tool == "edit" {
		return maxPermission(s.default_, PermissionAsk)
	}

	if tool == "shell" {
		return maxPermission(s.default_, PermissionAsk)
	}

	return s.default_
}

// CheckCommand checks permission for a specific command.
func (s *DefaultPermissionService) CheckCommand(command string) PermissionLevel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	command = normalizeCommandInput(command)

	// 解析命令名
	cmd := extractCommandName(command)

	// 检查是否有该命令的特定策略
	if level, ok := s.commandPolicies[cmd]; ok {
		return level
	}

	// 检查是否是危险命令
	if IsDangerousCommand(command) {
		return minPermission(s.default_, PermissionAsk)
	}

	return s.default_
}

// CheckPath checks permission for a specific path.
func (s *DefaultPermissionService) CheckPath(path string) PermissionLevel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 检查路径模式匹配
	for _, pp := range s.pathPatterns {
		if matched, _ := filepath.Match(pp.Pattern, path); matched {
			return pp.Level
		}
	}

	return PermissionAllowAlways
}

// Grant grants permission.
func (s *DefaultPermissionService) Grant(tool string, level PermissionLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.policies[tool] = level
}

// GrantCommand grants permission for a specific command.
func (s *DefaultPermissionService) GrantCommand(command string, level PermissionLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := extractCommandName(command)
	s.commandPolicies[cmd] = level
}

// GrantPath grants permission for a specific path pattern.
func (s *DefaultPermissionService) GrantPath(pattern string, level PermissionLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 查找是否已存在
	for i, pp := range s.pathPatterns {
		if pp.Pattern == pattern {
			s.pathPatterns[i].Level = level
			return
		}
	}

	// 添加新的
	s.pathPatterns = append(s.pathPatterns, PathPermission{
		Pattern: pattern,
		Level:   level,
	})
}

// Revoke revokes permission.
func (s *DefaultPermissionService) Revoke(tool string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.policies, tool)
}

// RevokeCommand revokes permission for a specific command.
func (s *DefaultPermissionService) RevokeCommand(command string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := extractCommandName(command)
	delete(s.commandPolicies, cmd)
}

// RevokePath revokes permission for a specific path pattern.
func (s *DefaultPermissionService) RevokePath(pattern string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, pp := range s.pathPatterns {
		if pp.Pattern == pattern {
			s.pathPatterns = append(s.pathPatterns[:i], s.pathPatterns[i+1:]...)
			return
		}
	}
}

// GetPolicies returns a copy of all policies.
func (s *DefaultPermissionService) GetPolicies() map[string]PermissionLevel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]PermissionLevel, len(s.policies))
	for k, v := range s.policies {
		result[k] = v
	}
	return result
}

// GetCommandPolicies returns command policies.
func (s *DefaultPermissionService) GetCommandPolicies() map[string]PermissionLevel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]PermissionLevel, len(s.commandPolicies))
	for k, v := range s.commandPolicies {
		result[k] = v
	}
	return result
}

// GetPathPolicies returns path policies.
func (s *DefaultPermissionService) GetPathPolicies() []PathPermission {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PathPermission, len(s.pathPatterns))
	copy(result, s.pathPatterns)
	return result
}

func maxPermission(a, b PermissionLevel) PermissionLevel {
	if a > b {
		return a
	}
	return b
}

func minPermission(a, b PermissionLevel) PermissionLevel {
	if a < b {
		return a
	}
	return b
}

// extractCommandName 从命令字符串中提取命令名
func extractCommandName(command string) string {
	// 去除前导空格
	command = strings.TrimLeft(command, " \t")

	// 提取第一个词
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

func normalizeCommandInput(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	var payload struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(command), &payload); err == nil {
		if cmd := strings.TrimSpace(payload.Command); cmd != "" {
			return cmd
		}
	}

	return command
}

// NoOpPermissionService is a permission service that always allows.
type NoOpPermissionService struct{}

// NewNoOpPermissionService creates a no-op permission service.
func NewNoOpPermissionService() *NoOpPermissionService {
	return &NoOpPermissionService{}
}

// Request always grants permission.
func (s *NoOpPermissionService) Request(ctx context.Context, tool, action, path string) (bool, error) {
	return true, nil
}

// Check always returns allow always.
func (s *NoOpPermissionService) Check(tool, action string) PermissionLevel {
	return PermissionAllowAlways
}

// CheckCommand always returns allow always.
func (s *NoOpPermissionService) CheckCommand(command string) PermissionLevel {
	return PermissionAllowAlways
}

// CheckPath always returns allow always.
func (s *NoOpPermissionService) CheckPath(path string) PermissionLevel {
	return PermissionAllowAlways
}

// Grant is a no-op.
func (s *NoOpPermissionService) Grant(tool string, level PermissionLevel) {}

// GrantCommand is a no-op.
func (s *NoOpPermissionService) GrantCommand(command string, level PermissionLevel) {}

// GrantPath is a no-op.
func (s *NoOpPermissionService) GrantPath(pattern string, level PermissionLevel) {}

// Revoke is a no-op.
func (s *NoOpPermissionService) Revoke(tool string) {}

// RevokeCommand is a no-op.
func (s *NoOpPermissionService) RevokeCommand(command string) {}

// RevokePath is a no-op.
func (s *NoOpPermissionService) RevokePath(pattern string) {}

// PermissionDecision 权限决策记录
type PermissionDecision struct {
	Tool      string
	Action    string
	Path      string
	Level     PermissionLevel
	Timestamp time.Time
}

// PermissionStore 权限存储接口
type PermissionStore interface {
	SaveDecision(decision PermissionDecision) error
	LoadDecisions() ([]PermissionDecision, error)
	ClearDecisions() error
}
