package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct {
	workDir string
	schema  tools.Schema
}

// NewGlobTool creates a new glob tool.
func NewGlobTool(workDir string) *GlobTool {
	return &GlobTool{
		workDir: workDir,
		schema: tools.NewSchema().
			String("pattern", "Glob pattern (e.g., '*.go', '**/*.yaml', 'cmd/*')").
			String("path", "Base directory to search from (default: current directory)").
			Required("pattern").
			Build(),
	}
}

// Name returns the tool name.
func (t *GlobTool) Name() string {
	return "glob"
}

// Description returns the tool description.
func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern. Use this to explore project structure and find specific file types."
}

// Schema returns the tool parameter schema.
func (t *GlobTool) Schema() llm.ToolSchema {
	return tools.ToLLMToolSchema(t.schema)
}

// Validate validates the parameters against the schema.
func (t *GlobTool) Validate(params json.RawMessage) []tools.ValidationError {
	return tools.ValidateAgainstSchema(params, t.schema)
}

type globParams struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// Execute executes the glob tool.
func (t *GlobTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	// Validate parameters first
	if errs := t.Validate(params); len(errs) > 0 {
		return tools.ErrorResultf("validation failed: %v", errs), nil
	}

	var p globParams
	if err := tools.ParseParams(params, &p); err != nil {
		return tools.ErrorResult(err), nil
	}

	// Resolve base path
	basePath := t.workDir
	if p.Path != "" {
		path := filepath.Clean(p.Path)
		if filepath.IsAbs(path) {
			return tools.ErrorResultf("absolute paths are not allowed: %s", p.Path), nil
		}
		fullPath := filepath.Join(t.workDir, path)
		if !strings.HasPrefix(fullPath, t.workDir) {
			return tools.ErrorResultf("path escapes working directory: %s", p.Path), nil
		}
		basePath = fullPath
	}

	// Check if base path exists
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return tools.ErrorResultf("path not found: %s", p.Path), nil
		}
		return tools.ErrorResult(err), nil
	}

	// Handle **/ prefix for recursive search
	pattern := p.Pattern
	recursive := false
	if strings.HasPrefix(pattern, "**/") {
		recursive = true
		pattern = pattern[3:]
	}

	// Find matches
	var matches []string
	if recursive {
		matches, err = t.globRecursive(basePath, pattern)
	} else {
		matches, err = t.globSingle(basePath, pattern)
	}
	if err != nil {
		return tools.ErrorResult(err), nil
	}

	// If base path is a file (not directory), check it directly
	if !info.IsDir() {
		matched, _ := filepath.Match(pattern, filepath.Base(basePath))
		if matched {
			relPath, _ := filepath.Rel(t.workDir, basePath)
			matches = append(matches, relPath)
		}
	}

	// Sort and deduplicate
	matches = uniqueStrings(matches)

	if len(matches) == 0 {
		return tools.StringResultWithSummary("No files found", "0 files"), nil
	}

	result := strings.Join(matches, "\n")
	summary := fmt.Sprintf("%d files", len(matches))

	return tools.StringResultWithSummary(result, summary), nil
}

func (t *GlobTool) globSingle(root, pattern string) ([]string, error) {
	var matches []string

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		name := entry.Name()
		matched, _ := filepath.Match(pattern, name)
		if matched {
			relPath, _ := filepath.Rel(t.workDir, filepath.Join(root, name))
			matches = append(matches, relPath)
		}
	}

	return matches, nil
}

func (t *GlobTool) globRecursive(root, pattern string) ([]string, error) {
	var matches []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if d.IsDir() {
			return nil
		}

		name := filepath.Base(path)
		matched, _ := filepath.Match(pattern, name)
		if matched {
			relPath, _ := filepath.Rel(t.workDir, path)
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
