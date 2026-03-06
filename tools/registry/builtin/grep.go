package builtin

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/vigo999/ms-cli/tools"
)

// GrepTool 文本搜索工具
type GrepTool struct {
	definition *tools.ToolDefinition
}

// GrepParams 搜索参数
type GrepParams struct {
	// 搜索模式（正则表达式）
	Pattern string `json:"pattern"`

	// 要搜索的路径（文件或目录）
	Path string `json:"path,omitempty"`

	// 搜索内容（如果未提供path，则搜索此内容）
	Content string `json:"content,omitempty"`

	// 是否大小写敏感
	CaseSensitive bool `json:"caseSensitive,omitempty"`

	// 是否只匹配整行
	WholeLine bool `json:"wholeLine,omitempty"`

	// 返回匹配的最大行数
	MaxResults int `json:"maxResults,omitempty"`

	// 是否包含行号
	IncludeLineNumbers bool `json:"includeLineNumbers,omitempty"`
}

// GrepMatch 匹配结果
type GrepMatch struct {
	Line       int    `json:"line,omitempty"`
	Content    string `json:"content"`
	File       string `json:"file,omitempty"`
	MatchStart int    `json:"matchStart,omitempty"`
	MatchEnd   int    `json:"matchEnd,omitempty"`
}

// GrepResult 搜索结果
type GrepResult struct {
	Matches    []GrepMatch `json:"matches"`
	TotalCount int         `json:"totalCount"`
	Truncated  bool        `json:"truncated,omitempty"`
}

// NewGrepTool 创建新的Grep工具
func NewGrepTool() tools.ExecutableTool {
	return &GrepTool{
		definition: &tools.ToolDefinition{
			Name:        "grep",
			DisplayName: "Search Text",
			Description: "Search for text patterns in files or content using regular expressions.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Regular expression pattern to search for",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File or directory path to search in",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Text content to search within (alternative to path)",
					},
					"caseSensitive": map[string]interface{}{
						"type":        "boolean",
						"description": "Case sensitive matching",
						"default":     true,
					},
					"wholeLine": map[string]interface{}{
						"type":        "boolean",
						"description": "Match whole lines only",
						"default":     false,
					},
					"maxResults": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of matches to return (0 for unlimited)",
						"default":     100,
					},
					"includeLineNumbers": map[string]interface{}{
						"type":        "boolean",
						"description": "Include line numbers in results",
						"default":     true,
					},
				},
				"required": []string{"pattern"},
			},
			Meta: tools.ToolMeta{
				Category:   "search",
				Cost:       tools.CostLevelLow,
				ReadOnly:   true,
				Idempotent: true,
				Permissions: []tools.Permission{
					{
						Name:        "file:read",
						Description: "Read file content",
					},
				},
			},
			Version: "1.0.0",
		},
	}
}

// Info 返回工具定义
func (t *GrepTool) Info() *tools.ToolDefinition {
	return t.definition
}

// Execute 执行搜索操作
func (t *GrepTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	// 解析参数
	var params GrepParams
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "invalid parameters: %v", err), nil
	}

	if params.Pattern == "" {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "pattern is required"), nil
	}

	// 设置默认值
	if params.MaxResults < 0 {
		params.MaxResults = 100
	}

	// 编译正则表达式
	re, err := t.compileRegex(params.Pattern, params.CaseSensitive, params.WholeLine)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "invalid regex pattern: %v", err), nil
	}

	// 执行搜索
	var result GrepResult

	if params.Content != "" {
		// 搜索提供的文本内容
		result = t.searchInContent(params.Content, re, params)
	} else if params.Path != "" {
		// 执行层权限检查
		if exec != nil {
			req := tools.PermissionRequest{
				ID:         ctx.CallID,
				SessionID:  ctx.SessionID,
				ToolID:     ctx.ToolID,
				CallID:     ctx.CallID,
				Permission: "file:read",
				Patterns:   []string{params.Path},
				CheckLevel: tools.CheckLevelExecution,
			}
			if err := exec.AskPermission(req); err != nil {
				return tools.NewErrorResult(tools.ErrCodePermissionDenied, "permission denied: %v", err), nil
			}
		}

		// 搜索文件或目录
		result, err = t.searchInPath(params.Path, re, params)
		if err != nil {
			return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "search failed: %v", err), nil
		}
	} else {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "either path or content must be provided"), nil
	}

	// 格式化结果
	output := t.formatResult(result)
	toolResult := tools.NewJSONResult(result)
	toolResult.AddPart(tools.Part{
		Type:    tools.PartTypeText,
		Content: output,
	})

	return toolResult, nil
}

// compileRegex 编译正则表达式
func (t *GrepTool) compileRegex(pattern string, caseSensitive, wholeLine bool) (*regexp.Regexp, error) {
	flags := ""
	if !caseSensitive {
		flags = "(?i)"
	}

	if wholeLine {
		pattern = "^" + pattern + "$"
	}

	return regexp.Compile(flags + pattern)
}

// searchInContent 在文本内容中搜索
func (t *GrepTool) searchInContent(content string, re *regexp.Regexp, params GrepParams) GrepResult {
	result := GrepResult{
		Matches: make([]GrepMatch, 0),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if loc := re.FindStringIndex(line); loc != nil {
			match := GrepMatch{
				Content:    line,
				MatchStart: loc[0],
				MatchEnd:   loc[1],
			}

			if params.IncludeLineNumbers {
				match.Line = lineNum
			}

			result.Matches = append(result.Matches, match)
			result.TotalCount++

			if params.MaxResults > 0 && len(result.Matches) >= params.MaxResults {
				result.Truncated = true
				break
			}
		}
	}

	return result
}

// searchInPath 在文件或目录中搜索
func (t *GrepTool) searchInPath(path string, re *regexp.Regexp, params GrepParams) (GrepResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return GrepResult{}, err
	}

	if info.IsDir() {
		return t.searchInDir(path, re, params)
	}

	return t.searchInFile(path, re, params)
}

// searchInFile 在单个文件中搜索
func (t *GrepTool) searchInFile(filePath string, re *regexp.Regexp, params GrepParams) (GrepResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return GrepResult{}, err
	}

	result := t.searchInContent(string(content), re, params)

	// 添加文件信息
	for i := range result.Matches {
		result.Matches[i].File = filePath
	}

	return result, nil
}

// searchInDir 在目录中搜索
func (t *GrepTool) searchInDir(dirPath string, re *regexp.Regexp, params GrepParams) (GrepResult, error) {
	result := GrepResult{
		Matches: make([]GrepMatch, 0),
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return result, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // 跳过子目录，简化处理
		}

		filePath := dirPath + "/" + entry.Name()
		fileResult, err := t.searchInFile(filePath, re, params)
		if err != nil {
			continue // 跳过无法读取的文件
		}

		result.Matches = append(result.Matches, fileResult.Matches...)
		result.TotalCount += fileResult.TotalCount

		if params.MaxResults > 0 && len(result.Matches) >= params.MaxResults {
			result.Truncated = true
			break
		}
	}

	return result, nil
}

// formatResult 格式化搜索结果
func (t *GrepTool) formatResult(result GrepResult) string {
	if result.TotalCount == 0 {
		return "No matches found."
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Found %d match(es):\n\n", result.TotalCount))

	for _, match := range result.Matches {
		if match.File != "" {
			buf.WriteString(fmt.Sprintf("%s:", match.File))
		}
		if match.Line > 0 {
			buf.WriteString(fmt.Sprintf("%d:", match.Line))
		}
		buf.WriteString(fmt.Sprintf(" %s\n", match.Content))
	}

	if result.Truncated {
		buf.WriteString("\n... (results truncated)")
	}

	return buf.String()
}

// GrepToolDefinition 返回Grep工具的定义（用于注册）
func GrepToolDefinition() tools.ToolDefinition {
	tool := NewGrepTool()
	return *tool.Info()
}
