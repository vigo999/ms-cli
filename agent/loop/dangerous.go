package loop

import (
	"regexp"
	"strings"
)

// DangerousCommand 定义危险命令
type DangerousCommand struct {
	Pattern     string          // 正则表达式模式
	Level       PermissionLevel // 默认权限级别
	Category    string          // 类别
	Description string          // 描述
}

// 预定义的危险命令列表
var defaultDangerousCommands = []DangerousCommand{
	// 删除类命令
	{
		Pattern:     `rm\s+-rf\s+/`,
		Level:       PermissionDeny,
		Category:    "destructive",
		Description: "Recursive force delete from root",
	},
	{
		Pattern:     `rm\s+-rf\s+~`,
		Level:       PermissionDeny,
		Category:    "destructive",
		Description: "Recursive force delete home directory",
	},
	{
		Pattern:     `rm\s+(-rf|-fr)\s*`,
		Level:       PermissionAsk,
		Category:    "destructive",
		Description: "Recursive force delete",
	},

	// 磁盘操作
	{
		Pattern:     `dd\s+if=.*of=/dev/`,
		Level:       PermissionDeny,
		Category:    "disk",
		Description: "Direct disk write",
	},
	{
		Pattern:     `mkfs\.`,
		Level:       PermissionDeny,
		Category:    "disk",
		Description: "Format filesystem",
	},
	{
		Pattern:     `fdisk`,
		Level:       PermissionAsk,
		Category:    "disk",
		Description: "Partition manipulation",
	},

	// 权限提升
	{
		Pattern:     `sudo`,
		Level:       PermissionAsk,
		Category:    "privilege",
		Description: "Elevated privileges",
	},
	{
		Pattern:     `su\s+-`,
		Level:       PermissionAsk,
		Category:    "privilege",
		Description: "Switch user",
	},

	// 权限修改
	{
		Pattern:     `chmod\s+777`,
		Level:       PermissionAsk,
		Category:    "permission",
		Description: "Wide permissions",
	},
	{
		Pattern:     `chmod\s+-R\s+777`,
		Level:       PermissionAsk,
		Category:    "permission",
		Description: "Recursive wide permissions",
	},
	{
		Pattern:     `chown\s+-R`,
		Level:       PermissionAsk,
		Category:    "permission",
		Description: "Recursive ownership change",
	},

	// 网络操作
	{
		Pattern:     `curl\s+.*\|\s*sh`,
		Level:       PermissionAsk,
		Category:    "network",
		Description: "Pipe curl to shell",
	},
	{
		Pattern:     `wget\s+.*\|\s*sh`,
		Level:       PermissionAsk,
		Category:    "network",
		Description: "Pipe wget to shell",
	},
	{
		Pattern:     `nc\s+-[lL]`,
		Level:       PermissionAsk,
		Category:    "network",
		Description: "Netcat listen mode",
	},

	// 系统修改
	{
		Pattern:     `>\s*/etc/`,
		Level:       PermissionAsk,
		Category:    "system",
		Description: "Write to system config",
	},
	{
		Pattern:     `>\s*/dev/`,
		Level:       PermissionDeny,
		Category:    "system",
		Description: "Direct device write",
	},
	{
		Pattern:     `:(){ :|:& };:`,
		Level:       PermissionDeny,
		Category:    "system",
		Description: "Fork bomb",
	},

	// Git 危险操作
	{
		Pattern:     `git\s+push\s+.*--force`,
		Level:       PermissionAsk,
		Category:    "git",
		Description: "Force push",
	},
	{
		Pattern:     `git\s+reset\s+--hard`,
		Level:       PermissionAsk,
		Category:    "git",
		Description: "Hard reset",
	},
	{
		Pattern:     `git\s+clean\s+-fd`,
		Level:       PermissionAsk,
		Category:    "git",
		Description: "Force clean untracked files",
	},

	// 环境变量修改
	{
		Pattern:     `export\s+PATH=`,
		Level:       PermissionAsk,
		Category:    "env",
		Description: "Modify PATH",
	},

	// 文件移动/重命名
	{
		Pattern:     `mv\s+/`,
		Level:       PermissionAsk,
		Category:    "file",
		Description: "Move system files",
	},
}

// DangerousCommandChecker 危险命令检查器
type DangerousCommandChecker struct {
	commands []DangerousCommand
}

// NewDangerousCommandChecker 创建危险命令检查器
func NewDangerousCommandChecker() *DangerousCommandChecker {
	return &DangerousCommandChecker{
		commands: defaultDangerousCommands,
	}
}

// AddCommand 添加危险命令
func (c *DangerousCommandChecker) AddCommand(cmd DangerousCommand) {
	c.commands = append(c.commands, cmd)
}

// Check 检查命令是否危险
func (c *DangerousCommandChecker) Check(command string) *DangerousCommand {
	for _, cmd := range c.commands {
		re, err := regexp.Compile(cmd.Pattern)
		if err != nil {
			continue // 跳过无效的正则
		}
		if re.MatchString(command) {
			return &cmd
		}
	}
	return nil
}

// IsDangerous 检查命令是否危险
func (c *DangerousCommandChecker) IsDangerous(command string) bool {
	return c.Check(command) != nil
}

// GetCategory 获取命令的危险类别
func (c *DangerousCommandChecker) GetCategory(command string) string {
	if cmd := c.Check(command); cmd != nil {
		return cmd.Category
	}
	return ""
}

// GlobalChecker 全局检查器实例
var GlobalChecker = NewDangerousCommandChecker()

// IsDangerousCommand 检查命令是否危险
func IsDangerousCommand(command string) bool {
	return GlobalChecker.IsDangerous(command)
}

// GetDangerousCommandInfo 获取危险命令信息
func GetDangerousCommandInfo(command string) *DangerousCommand {
	return GlobalChecker.Check(command)
}

// ValidateCommand 验证命令并返回建议的权限级别
func ValidateCommand(command string) PermissionLevel {
	if cmd := GlobalChecker.Check(command); cmd != nil {
		return cmd.Level
	}
	return PermissionAllowAlways
}

// CommandCategory 命令类别
type CommandCategory struct {
	Name        string
	Description string
	Level       PermissionLevel
}

// Categories 返回所有类别
func Categories() []CommandCategory {
	return []CommandCategory{
		{Name: "destructive", Description: "Destructive operations", Level: PermissionAsk},
		{Name: "disk", Description: "Disk operations", Level: PermissionAsk},
		{Name: "privilege", Description: "Privilege escalation", Level: PermissionAsk},
		{Name: "permission", Description: "Permission changes", Level: PermissionAsk},
		{Name: "network", Description: "Network operations", Level: PermissionAsk},
		{Name: "system", Description: "System modifications", Level: PermissionAsk},
		{Name: "git", Description: "Git operations", Level: PermissionAsk},
		{Name: "env", Description: "Environment changes", Level: PermissionAsk},
		{Name: "file", Description: "File operations", Level: PermissionAllowAlways},
	}
}

// SafeCommand 安全命令包装器
type SafeCommand struct {
	Command      string
	Sanitized    string
	IsDangerous  bool
	Warning      string
	SuggestedLevel PermissionLevel
}

// SanitizeCommand 清理命令并检查安全性
func SanitizeCommand(command string) SafeCommand {
	safe := SafeCommand{
		Command: command,
	}

	// 检查是否危险
	if dangerous := GlobalChecker.Check(command); dangerous != nil {
		safe.IsDangerous = true
		safe.Warning = dangerous.Description
		safe.SuggestedLevel = dangerous.Level
	}

	// 简单清理：去除前后空格
	safe.Sanitized = strings.TrimSpace(command)

	return safe
}

// CommandWhitelist 命令白名单
type CommandWhitelist struct {
	commands map[string]bool
}

// NewCommandWhitelist 创建命令白名单
func NewCommandWhitelist(commands []string) *CommandWhitelist {
	w := &CommandWhitelist{
		commands: make(map[string]bool),
	}
	for _, cmd := range commands {
		w.commands[cmd] = true
	}
	return w
}

// IsAllowed 检查命令是否在白名单中
func (w *CommandWhitelist) IsAllowed(command string) bool {
	cmd := extractCommandName(command)
	return w.commands[cmd]
}

// Add 添加命令到白名单
func (w *CommandWhitelist) Add(command string) {
	w.commands[command] = true
}

// Remove 从白名单移除命令
func (w *CommandWhitelist) Remove(command string) {
	delete(w.commands, command)
}

// CommandBlacklist 命令黑名单
type CommandBlacklist struct {
	commands map[string]bool
}

// NewCommandBlacklist 创建命令黑名单
func NewCommandBlacklist(commands []string) *CommandBlacklist {
	b := &CommandBlacklist{
		commands: make(map[string]bool),
	}
	for _, cmd := range commands {
		b.commands[cmd] = true
	}
	return b
}

// IsBlocked 检查命令是否在黑名单中
func (b *CommandBlacklist) IsBlocked(command string) bool {
	cmd := extractCommandName(command)
	return b.commands[cmd]
}

// Add 添加命令到黑名单
func (b *CommandBlacklist) Add(command string) {
	b.commands[command] = true
}

// Remove 从黑名单移除命令
func (b *CommandBlacklist) Remove(command string) {
	delete(b.commands, command)
}

// IsAllowedCommand 检查命令是否是常见的安全命令
func IsAllowedCommand(command string) bool {
	allowedCommands := []string{
		"ls", "ll", "cat", "pwd", "echo", "cd", "pwd",
		"head", "tail", "less", "more", "grep", "find",
		"wc", "sort", "uniq", "diff", "which", "whoami",
		"date", "cal", "clear", "history", "exit",
	}

	cmd := extractCommandName(command)
	for _, allowed := range allowedCommands {
		if cmd == allowed {
			return true
		}
	}
	return false
}
