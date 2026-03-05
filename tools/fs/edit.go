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

// EditTool edits file contents by replacing text.
type EditTool struct {
	workDir string
	schema  tools.Schema
}

// NewEditTool creates a new edit tool.
func NewEditTool(workDir string) *EditTool {
	return &EditTool{
		workDir: workDir,
		schema: tools.NewSchema().
			String("path", "Relative path to the file to edit").
			String("old_string", "Exact text to replace (must match exactly including whitespace and newlines)").
			String("new_string", "New text to replace the old_string with").
			Required("path", "old_string", "new_string").
			Build(),
	}
}

// Name returns the tool name.
func (t *EditTool) Name() string {
	return "edit"
}

// Description returns the tool description.
func (t *EditTool) Description() string {
	return "Edit a file by replacing specific text. Use this for making targeted changes. The old_string must match exactly including whitespace."
}

// Schema returns the tool parameter schema.
func (t *EditTool) Schema() llm.ToolSchema {
	return tools.ToLLMToolSchema(t.schema)
}

// Validate validates the parameters against the schema.
func (t *EditTool) Validate(params json.RawMessage) []tools.ValidationError {
	return tools.ValidateAgainstSchema(params, t.schema)
}

type editParams struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// Execute executes the edit tool.
func (t *EditTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	// Validate parameters first
	if errs := t.Validate(params); len(errs) > 0 {
		return tools.ErrorResultf("validation failed: %v", errs), nil
	}

	var p editParams
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

	// Read existing file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return tools.ErrorResultf("file not found: %s", p.Path), nil
		}
		return tools.ErrorResultf("read file: %w", err), nil
	}

	contentStr := string(content)

	// Check if old_string exists
	if !strings.Contains(contentStr, p.OldString) {
		// Try to find similar content
		return tools.ErrorResultf("old_string not found in file. The text must match exactly (including whitespace and newlines)"), nil
	}

	// Count occurrences
	occurrences := strings.Count(contentStr, p.OldString)
	if occurrences > 1 {
		return tools.ErrorResultf("old_string appears %d times in the file. Please provide more context to make a unique match", occurrences), nil
	}

	// Replace
	newContent := strings.Replace(contentStr, p.OldString, p.NewString, 1)

	// Write back
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return tools.ErrorResultf("write file: %w", err), nil
	}

	// Build diff-style result
	oldLines := strings.Count(p.OldString, "\n")
	newLines := strings.Count(p.NewString, "\n")
	if !strings.HasSuffix(p.OldString, "\n") && p.OldString != "" {
		oldLines++
	}
	if !strings.HasSuffix(p.NewString, "\n") && p.NewString != "" {
		newLines++
	}

	result := fmt.Sprintf("Edited: %s\n- %s\n+ %s", p.Path, p.OldString, p.NewString)
	summary := fmt.Sprintf("%d lines → %d lines", oldLines, newLines)

	return tools.StringResultWithSummary(result, summary), nil
}
