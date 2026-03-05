package tools

import (
	"fmt"
)

// ResultType represents the type of tool result
type ResultType string

const (
	ResultTypeText  ResultType = "text"
	ResultTypeJSON  ResultType = "json"
	ResultTypeError ResultType = "error"
)

// ErrorInfo contains structured error information
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// FileRef references a file affected by the tool
type FileRef struct {
	Path   string `json:"path"`
	Action string `json:"action"` // "read", "written", "modified"
}

// EnhancedResult is the enhanced result of tool execution
// Note: This is a new type to avoid breaking existing code.
// Eventually, this should replace the simple Result type.
type EnhancedResult struct {
	Type    ResultType `json:"type"`
	Content string     `json:"content"`
	Data    any        `json:"data,omitempty"`      // Structured data for JSON results
	Summary string     `json:"summary"`             // Brief summary for UI
	Error   *ErrorInfo `json:"error,omitempty"`     // Error details if failed
	Files   []FileRef  `json:"files,omitempty"`     // Referenced files
}

// NewSuccessResult creates a successful result
func NewSuccessResult(content, summary string) *EnhancedResult {
	return &EnhancedResult{
		Type:    ResultTypeText,
		Content: content,
		Summary: summary,
	}
}

// NewJSONResult creates a JSON result with structured data
func NewJSONResult(data any, summary string) *EnhancedResult {
	return &EnhancedResult{
		Type:    ResultTypeJSON,
		Data:    data,
		Summary: summary,
	}
}

// NewErrorResult creates an error result
func NewErrorResult(code, message string) *EnhancedResult {
	return &EnhancedResult{
		Type:    ResultTypeError,
		Content: message,
		Summary: fmt.Sprintf("Error: %s", code),
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}

// NewErrorResultWithDetails creates an error result with additional details
func NewErrorResultWithDetails(code, message, details string) *EnhancedResult {
	return &EnhancedResult{
		Type:    ResultTypeError,
		Content: message,
		Summary: fmt.Sprintf("Error: %s", code),
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// AddFile adds a file reference to the result
func (r *EnhancedResult) AddFile(path, action string) *EnhancedResult {
	r.Files = append(r.Files, FileRef{Path: path, Action: action})
	return r
}

// SetContent sets the content of the result (for builder pattern)
func (r *EnhancedResult) SetContent(content string) *EnhancedResult {
	r.Content = content
	return r
}

// ToLegacyResult converts EnhancedResult to legacy Result for backward compatibility
func (r *EnhancedResult) ToLegacyResult() *Result {
	var err error
	if r.Error != nil {
		err = fmt.Errorf("%s: %s", r.Error.Code, r.Error.Message)
	}
	return &Result{
		Content: r.Content,
		Summary: r.Summary,
		Error:   err,
	}
}

// LegacyResultAdapter adapts legacy Result to EnhancedResult
func LegacyResultAdapter(r *Result) *EnhancedResult {
	if r == nil {
		return nil
	}

	resultType := ResultTypeText
	var errorInfo *ErrorInfo

	if r.Error != nil {
		resultType = ResultTypeError
		errorInfo = &ErrorInfo{
			Code:    "execution_error",
			Message: r.Error.Error(),
		}
	}

	return &EnhancedResult{
		Type:    resultType,
		Content: r.Content,
		Summary: r.Summary,
		Error:   errorInfo,
	}
}
