package permission

import (
	"regexp"
	"strings"
	"sync"
)

// WildcardMatcher 通配符匹配器
type WildcardMatcher struct {
	// 原始模式
	pattern string

	// 编译后的正则表达式
	regex *regexp.Regexp

	// 是否为精确匹配
	exact bool

	// 是否为前缀匹配
	prefix bool

	// 是否为后缀匹配
	suffix bool
}

// matcherCache 匹配器缓存
var matcherCache = struct {
	sync.RWMutex
	matchers map[string]*WildcardMatcher
}{
	matchers: make(map[string]*WildcardMatcher),
}

// CompileWildcard 编译通配符模式
func CompileWildcard(pattern string) *WildcardMatcher {
	// 尝试从缓存获取
	matcherCache.RLock()
	if m, ok := matcherCache.matchers[pattern]; ok {
		matcherCache.RUnlock()
		return m
	}
	matcherCache.RUnlock()

	m := &WildcardMatcher{
		pattern: pattern,
	}

	// 判断匹配类型
	hasStar := strings.Contains(pattern, "*")
	hasQuestion := strings.Contains(pattern, "?")

	if !hasStar && !hasQuestion {
		// 精确匹配
		m.exact = true
	} else if strings.HasSuffix(pattern, "*") && !strings.Contains(strings.TrimSuffix(pattern, "*"), "*") {
		// 前缀匹配，如 "/home/user/*"
		m.prefix = true
	} else if strings.HasPrefix(pattern, "*") && !strings.Contains(strings.TrimPrefix(pattern, "*"), "*") {
		// 后缀匹配，如 "*.go"
		m.suffix = true
	} else {
		// 复杂模式，使用正则
		regexPattern := wildcardToRegex(pattern)
		m.regex = regexp.MustCompile(regexPattern)
	}

	// 存入缓存
	matcherCache.Lock()
	matcherCache.matchers[pattern] = m
	matcherCache.Unlock()

	return m
}

// Match 检查字符串是否匹配模式
func (m *WildcardMatcher) Match(s string) bool {
	if m.exact {
		return m.pattern == s
	}

	if m.prefix {
		prefix := strings.TrimSuffix(m.pattern, "*")
		return strings.HasPrefix(s, prefix)
	}

	if m.suffix {
		suffix := strings.TrimPrefix(m.pattern, "*")
		return strings.HasSuffix(s, suffix)
	}

	if m.regex != nil {
		return m.regex.MatchString(s)
	}

	return false
}

// MatchAny 检查是否匹配任意一个模式
func MatchAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if CompileWildcard(pattern).Match(s) {
			return true
		}
	}
	return false
}

// MatchAll 检查是否匹配所有模式
func MatchAll(s string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if !CompileWildcard(pattern).Match(s) {
			return false
		}
	}
	return true
}

// wildcardToRegex 将通配符转换为正则表达式
func wildcardToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")

	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			result.WriteString("\\")
			result.WriteByte(c)
		default:
			result.WriteByte(c)
		}
	}

	result.WriteString("$")
	return result.String()
}

// ClearMatcherCache 清除匹配器缓存
func ClearMatcherCache() {
	matcherCache.Lock()
	matcherCache.matchers = make(map[string]*WildcardMatcher)
	matcherCache.Unlock()
}

// GetMatcherCacheSize 获取缓存大小
func GetMatcherCacheSize() int {
	matcherCache.RLock()
	defer matcherCache.RUnlock()
	return len(matcherCache.matchers)
}
