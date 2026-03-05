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

// WriteTool writes or creates file contents.
type WriteTool struct {
	workDir string
	schema  tools.Schema
}

// NewWriteTool creates a new write tool.
func NewWriteTool(workDir string) *WriteTool {
	return &WriteTool{
		workDir: workDir,
		schema: tools.NewSchema().
			String("path", "Relative path to the file to write").
			String("content", "Content to write to the file").
			Required("path", "content").
			Build(),
	}
}

// Name returns the tool name.
func (t *WriteTool) Name() string {
	return "write"
}

// Description returns the tool description.
func (t *WriteTool) Description() string {
	return "Create a new file or overwrite an existing file with new content."
}

// Schema returns the tool parameter schema.
func (t *WriteTool) Schema() llm.ToolSchema {
	return tools.ToLLMToolSchema(t.schema)
}

// Validate validates the parameters against the schema.
func (t *WriteTool) Validate(params json.RawMessage) []tools.ValidationError {
	return tools.ValidateAgainstSchema(params, t.schema)
}

type writeParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Execute executes the write tool.
func (t *WriteTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	// Validate parameters first
	if errs := t.Validate(params); len(errs) > 0 {
		return tools.ErrorResultf("validation failed: %v", errs), nil
	}

	var p writeParams
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

	// Check if file already exists
	exists := false
	if _, err := os.Stat(fullPath); err == nil {
		exists = true
	}

	// Ensure parent directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ErrorResultf("create directory: %w", err), nil
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(p.Content), 0644); err != nil {
		return tools.ErrorResultf("write file: %w", err), nil
	}

	// Build result
	lines := strings.Count(p.Content, "\n")
	if !strings.HasSuffix(p.Content, "\n") && p.Content != "" {
		lines++
	}

	action := "Created"
	if exists {
		action = "Updated"
	}

	content := fmt.Sprintf("%s: %s\n+ %s", action, p.Path, p.Content)
	summary := fmt.Sprintf("%s %d lines", action, lines)

	return tools.StringResultWithSummary(content, summary), nil
}
