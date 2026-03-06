package builtin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/vigo999/ms-cli/tools"
)

// GlobTool 文件搜索工具
type GlobTool struct {
	definition *tools.ToolDefinition
}

// GlobParams 搜索参数
type GlobParams struct {
	// 搜索模式（支持通配符）
	Pattern string `json:"pattern"`

	// 搜索起始目录
	Path string `json:"path,omitempty"`

	// 是否递归搜索
	Recursive bool `json:"recursive,omitempty"`

	// 限制返回结果数量
	Limit int `json:"limit,omitempty"`
}

// GlobResult 搜索结果
type GlobResult struct {
	Files       []string `json:"files"`
	TotalCount  int      `json:"totalCount"`
	Truncated   bool     `json:"truncated,omitempty"`
	SearchPath  string   `json:"searchPath"`
	SearchPattern string `json:"searchPattern"`
}

// NewGlobTool 创建新的搜索工具
func NewGlobTool() tools.ExecutableTool {
	return &GlobTool{
		definition: &tools.ToolDefinition{
			Name:        "glob",
			DisplayName: "Search Files",
			Description: "Search for files matching a pattern. Supports glob patterns like '*.go' or '**/*.json'.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern to match files (e.g., '*.go', '**/*.json')",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory to start searching from (default: current directory)",
						"default":     ".",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "Search recursively in subdirectories",
						"default":     true,
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (0 for unlimited)",
						"default":     100,
					},
				},
				"required": []string{"pattern"},
			},
			Meta: tools.ToolMeta{
				Category:   "filesystem",
				Cost:       tools.CostLevelLow,
				ReadOnly:   true,
				Idempotent: true,
				Permissions: []tools.Permission{
					{
						Name:        "file:read",
						Description: "Read directory and file information",
					},
				},
			},
			Version: "1.0.0",
		},
	}
}

// Info 返回工具定义
func (t *GlobTool) Info() *tools.ToolDefinition {
	return t.definition
}

// Execute 执行搜索操作
func (t *GlobTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	// 解析参数
	var params GlobParams
	if err := json.Unmarshal(args, &params); err != nil {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "invalid parameters: %v", err), nil
	}

	if params.Pattern == "" {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "pattern is required"), nil
	}

	// 设置默认值
	if params.Path == "" {
		params.Path = "."
	}

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
			return tools.NewErrorResult(tools.ErrCodePermissionDenied,
				"permission denied: %v", err), nil
		}
	}

	// 检查路径是否存在
	info, err := os.Stat(params.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return tools.NewErrorResult(tools.ErrCodeNotFound, "path not found: %s", params.Path), nil
		}
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "failed to stat path: %v", err), nil
	}

	if !info.IsDir() {
		return tools.NewErrorResult(tools.ErrCodeInvalidInput, "path is not a directory: %s", params.Path), nil
	}

	// 执行搜索
	files, err := t.searchFiles(params.Path, params.Pattern, params.Recursive, params.Limit)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "search failed: %v", err), nil
	}

	// 构建结果
	result := GlobResult{
		Files:         files,
		TotalCount:    len(files),
		SearchPath:    params.Path,
		SearchPattern: params.Pattern,
	}

	if params.Limit > 0 && len(files) >= params.Limit {
		result.Truncated = true
	}

	// 转换为JSON
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	toolResult := tools.NewJSONResult(result)
	toolResult.AddPart(tools.Part{
		Type:    tools.PartTypeText,
		Content: string(jsonData),
	})

	return toolResult, nil
}

// searchFiles 搜索文件
func (t *GlobTool) searchFiles(root, pattern string, recursive bool, limit int) ([]string, error) {
	var files []string

	// 解析pattern，分离目录部分和文件模式部分
	// 例如："executor/**" -> dirPattern="executor", filePattern="**"
	// 例如："**/*.go" -> dirPattern="", filePattern="*.go"
	// 例如："executor/*.go" -> dirPattern="executor", filePattern="*.go"
	dirPattern, filePattern := splitPattern(pattern)

	// 确定搜索的起始目录
	searchRoot := root
	if dirPattern != "" && dirPattern != "." && dirPattern != "*" && dirPattern != "**" {
		// 如果dirPattern是具体的目录（不含通配符），则直接拼接
		if !strings.ContainsAny(dirPattern, "*?[]") {
			searchRoot = filepath.Join(root, dirPattern)
			// 检查目录是否存在
			if info, err := os.Stat(searchRoot); err != nil || !info.IsDir() {
				return files, nil // 目录不存在，返回空结果
			}
			dirPattern = "" // 已经定位到具体目录
		}
	}

	// 执行搜索
	err := t.walkDir(searchRoot, root, dirPattern, filePattern, &files, limit)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// splitPattern 将pattern分离为目录部分和文件名模式部分
// 返回 (目录模式, 文件模式)
func splitPattern(pattern string) (string, string) {
	// 处理 ** 模式
	if pattern == "**" {
		return "**", "*"
	}

	// 找到最后一个路径分隔符
	lastSep := strings.LastIndexAny(pattern, "/\\")
	if lastSep == -1 {
		// 没有路径分隔符，整个pattern是文件匹配模式
		return "", pattern
	}

	dirPart := pattern[:lastSep]
	filePart := pattern[lastSep+1:]

	// 如果目录部分包含 **，需要特殊处理
	if strings.Contains(dirPart, "**") {
		// 保持完整的pattern，让walkDir处理
		return dirPart, filePart
	}

	return dirPart, filePart
}

// walkDir 递归遍历目录
// root: 用于计算相对路径的根目录
// current: 当前遍历的目录
// dirPattern: 目录匹配模式（可能包含通配符）
// filePattern: 文件匹配模式
func (t *GlobTool) walkDir(current, root, dirPattern, filePattern string, files *[]string, limit int) error {
	if limit > 0 && len(*files) >= limit {
		return nil
	}

	entries, err := os.ReadDir(current)
	if err != nil {
		// 忽略无权限访问的目录
		return nil
	}

	for _, entry := range entries {
		if limit > 0 && len(*files) >= limit {
			return nil
		}

		fullPath := filepath.Join(current, entry.Name())
		relPath, _ := filepath.Rel(root, fullPath)

		if entry.IsDir() {
			// 检查是否需要进入子目录
			if shouldEnterDir(dirPattern, relPath, entry.Name()) {
				if err := t.walkDir(fullPath, root, dirPattern, filePattern, files, limit); err != nil {
					return err
				}
			}
		} else {
			// 检查文件是否匹配
			if matchFile(relPath, dirPattern, filePattern, entry.Name()) {
				*files = append(*files, fullPath)
			}
		}
	}

	return nil
}

// shouldEnterDir 判断是否应该进入子目录
func shouldEnterDir(dirPattern, relPath, dirName string) bool {
	// 如果dirPattern为空或为**或*，则进入所有子目录
	if dirPattern == "" || dirPattern == "**" || dirPattern == "*" {
		return true
	}

	// 检查目录名是否匹配dirPattern的对应部分
	// 简化处理：如果dirPattern不包含通配符，直接比较
	if !strings.ContainsAny(dirPattern, "*?[]") {
		// dirPattern是具体路径，检查relPath是否以其开头
		return strings.HasPrefix(dirPattern, relPath) || strings.HasPrefix(relPath, dirName)
	}

	// 尝试匹配目录
	matched, _ := filepath.Match(dirPattern, relPath)
	if matched {
		return true
	}

	// 如果relPath是dirPattern的前缀，也应该进入
	if strings.HasPrefix(dirPattern, relPath) {
		return true
	}

	return true // 默认进入所有子目录（保守策略）
}

// matchFile 检查文件是否匹配模式
func matchFile(relPath, dirPattern, filePattern, fileName string) bool {
	// 如果没有目录模式，直接匹配文件名
	if dirPattern == "" {
		matched, _ := filepath.Match(filePattern, fileName)
		return matched
	}

	// 如果filePattern是**，匹配所有文件
	if filePattern == "**" || filePattern == "*" {
		// 检查目录部分是否匹配
		if dirPattern == "**" || dirPattern == "*" {
			return true
		}
		// 检查文件路径是否匹配dirPattern作为前缀
		dir := filepath.Dir(relPath)
		matched, _ := filepath.Match(dirPattern, dir)
		return matched
	}

	// 处理 **/*.ext 模式
	if dirPattern == "**" {
		matched, _ := filepath.Match(filePattern, fileName)
		return matched
	}

	// 尝试匹配完整路径
	fullPattern := dirPattern
	if filePattern != "" {
		fullPattern = dirPattern + string(filepath.Separator) + filePattern
	}
	matched, _ := filepath.Match(fullPattern, relPath)
	if matched {
		return true
	}

	// 如果dirPattern是相对路径，也尝试只匹配文件名
	dir := filepath.Dir(relPath)
	matched, _ = filepath.Match(dirPattern, dir)
	if matched {
		matched, _ = filepath.Match(filePattern, fileName)
		return matched
	}

	return false
}

// GlobToolDefinition 返回搜索工具的定义（用于注册）
func GlobToolDefinition() tools.ToolDefinition {
	tool := NewGlobTool()
	return *tool.Info()
}
