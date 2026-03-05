package fs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

// GrepTool searches for patterns in files.
type GrepTool struct {
	workDir string
	schema  tools.Schema
}

// NewGrepTool creates a new grep tool.
func NewGrepTool(workDir string) *GrepTool {
	return &GrepTool{
		workDir: workDir,
		schema: tools.NewSchema().
			String("pattern", "Regular expression pattern to search for (e.g., 'func.*main', 'TODO|FIXME')").
			String("path", "Directory or file to search in (default: current directory)").
			String("include", "File pattern to include using glob syntax (e.g., '*.go', '*.md')").
			Bool("case_sensitive", "Whether the search is case sensitive (default: true)").
			Required("pattern").
			Build(),
	}
}

// Name returns the tool name.
func (t *GrepTool) Name() string {
	return "grep"
}

// Description returns the tool description.
func (t *GrepTool) Description() string {
	return "Search for patterns in files using regular expressions. Returns matching lines with file names and line numbers."
}

// Schema returns the tool parameter schema.
func (t *GrepTool) Schema() llm.ToolSchema {
	return tools.ToLLMToolSchema(t.schema)
}

// Validate validates the parameters against the schema.
func (t *GrepTool) Validate(params json.RawMessage) []tools.ValidationError {
	return tools.ValidateAgainstSchema(params, t.schema)
}

type grepParams struct {
	Pattern       string `json:"pattern"`
	Path          string `json:"path"`
	Include       string `json:"include"`
	CaseSensitive bool   `json:"case_sensitive"`
}

// Match represents a single grep match.
type Match struct {
	File   string
	Line   int
	Column int
	Text   string
}

// Execute executes the grep tool.
func (t *GrepTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	// Validate parameters first
	if errs := t.Validate(params); len(errs) > 0 {
		return tools.ErrorResultf("validation failed: %v", errs), nil
	}

	var p grepParams
	if err := tools.ParseParams(params, &p); err != nil {
		return tools.ErrorResult(err), nil
	}

	// Default case sensitive
	if !p.CaseSensitive {
		// Pattern will be handled with case-insensitive flag
	}

	// Resolve search path
	searchPath := "."
	if p.Path != "" {
		searchPath = filepath.Clean(p.Path)
		if filepath.IsAbs(searchPath) {
			return tools.ErrorResultf("absolute paths are not allowed: %s", p.Path), nil
		}
	}
	fullPath := filepath.Join(t.workDir, searchPath)

	// Security check
	if !strings.HasPrefix(fullPath, t.workDir) {
		return tools.ErrorResultf("path escapes working directory: %s", p.Path), nil
	}

	// Compile regex
	pattern := p.Pattern
	if !p.CaseSensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return tools.ErrorResultf("invalid pattern: %w", err), nil
	}

	// Find files and search
	matches, err := t.grep(ctx, fullPath, p.Include, re)
	if err != nil {
		return tools.ErrorResult(err), nil
	}

	// Format results
	if len(matches) == 0 {
		return tools.StringResultWithSummary("No matches found", "0 matches"), nil
	}

	var lines []string
	for _, m := range matches {
		relPath, _ := filepath.Rel(t.workDir, m.File)
		lines = append(lines, fmt.Sprintf("%s:%d:%s", relPath, m.Line, m.Text))
	}

	result := strings.Join(lines, "\n")
	summary := fmt.Sprintf("%d matches", len(matches))

	return tools.StringResultWithSummary(result, summary), nil
}

func (t *GrepTool) grep(ctx context.Context, root, include string, re *regexp.Regexp) ([]Match, error) {
	var matches []Match

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		return t.searchFile(root, re)
	}

	// Walk directory
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if d.IsDir() {
			return nil
		}

		// Check include pattern
		if include != "" {
			matched, _ := filepath.Match(include, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		fileMatches, err := t.searchFile(path, re)
		if err != nil {
			return nil // Skip file errors
		}

		matches = append(matches, fileMatches...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

func (t *GrepTool) searchFile(path string, re *regexp.Regexp) ([]Match, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []Match
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if loc := re.FindStringIndex(line); loc != nil {
			matches = append(matches, Match{
				File:   path,
				Line:   lineNum,
				Column: loc[0] + 1,
				Text:   line,
			})
		}
	}

	return matches, scanner.Err()
}
