package fs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

// ReadTool reads file contents.
type ReadTool struct {
	workDir string
	schema  tools.Schema
}

// NewReadTool creates a new read tool.
func NewReadTool(workDir string) *ReadTool {
	return &ReadTool{
		workDir: workDir,
		schema: tools.NewSchema().
			String("path", "Relative path to the file to read").
			Int("offset", "Line number to start reading from (1-indexed, 0 means from start)").
			Int("limit", "Maximum number of lines to read (0 means no limit)").
			Required("path").
			Build(),
	}
}

// Name returns the tool name.
func (t *ReadTool) Name() string {
	return "read"
}

// Description returns the tool description.
func (t *ReadTool) Description() string {
	return "Read the contents of a file. Use this when you need to examine file contents."
}

// Schema returns the tool parameter schema.
func (t *ReadTool) Schema() llm.ToolSchema {
	return tools.ToLLMToolSchema(t.schema)
}

// Validate validates the parameters against the schema.
func (t *ReadTool) Validate(params json.RawMessage) []tools.ValidationError {
	return tools.ValidateAgainstSchema(params, t.schema)
}

type readParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// Execute executes the read tool.
func (t *ReadTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	// Validate parameters first
	if errs := t.Validate(params); len(errs) > 0 {
		return tools.ErrorResultf("validation failed: %v", errs), nil
	}

	var p readParams
	if err := tools.ParseParams(params, &p); err != nil {
		return tools.ErrorResult(err), nil
	}

	// Clean and resolve path
	path := filepath.Clean(p.Path)
	if filepath.IsAbs(path) {
		return tools.ErrorResultf("absolute paths are not allowed: %s", p.Path), nil
	}

	fullPath := filepath.Join(t.workDir, path)

	// Security check: ensure path is within workDir
	if !strings.HasPrefix(fullPath, t.workDir) {
		return tools.ErrorResultf("path escapes working directory: %s", p.Path), nil
	}

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return tools.ErrorResultf("file not found: %s", p.Path), nil
		}
		return tools.ErrorResultf("stat file: %w", err), nil
	}

	if info.IsDir() {
		return tools.ErrorResultf("path is a directory: %s", p.Path), nil
	}

	// Read file
	content, err := t.readFile(fullPath, p.Offset, p.Limit)
	if err != nil {
		return tools.ErrorResult(err), nil
	}

	// Count lines for summary
	lines := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") && content != "" {
		lines++
	}

	summary := fmt.Sprintf("%d lines", lines)
	if p.Offset > 0 || p.Limit > 0 {
		summary = fmt.Sprintf("%d lines (offset=%d, limit=%d)", lines, p.Offset, p.Limit)
	}

	return tools.StringResultWithSummary(content, summary), nil
}

func (t *ReadTool) readFile(path string, offset, limit int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if offset > 0 && lineNum < offset {
			continue
		}
		lines = append(lines, scanner.Text())
		if limit > 0 && len(lines) >= limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
