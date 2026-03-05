package tools

import (
	"context"
)

// contextKey is the type for context keys to avoid collisions
type contextKey int

const (
	permissionCheckerKey contextKey = iota
)

// PermissionRequest represents a permission request from a tool
type PermissionRequest struct {
	Operation string // e.g., "write", "delete", "execute", "shell"
	Resource  string // e.g., file path, command
	Reason    string // Human-readable explanation
}

// PermissionResponse represents the response to a permission request
type PermissionResponse struct {
	Granted bool
	Error   error
}

// PermissionChecker is the interface for tools to request permission checks.
// This is passed via context to allow tools to check permissions internally.
type PermissionChecker interface {
	// CheckPermission checks if an operation is allowed without user interaction.
	// Returns true if allowed, false if denied or needs user confirmation.
	CheckPermission(ctx context.Context, operation string, resource string) (bool, error)

	// AskPermission asks user for permission interactively.
	// This should be called for operations that require user confirmation.
	AskPermission(ctx context.Context, operation string, resource string, reason string) (bool, error)
}

// PermissionUI is the interface for UI-based permission requests.
// The engine or application layer should implement this interface.
type PermissionUI interface {
	// RequestPermission asks the user for permission.
	// Returns (granted, error).
	RequestPermission(ctx context.Context, req PermissionRequest) (bool, error)
}

// WithPermissionChecker adds a permission checker to the context.
// This should be called by the engine before executing a tool.
func WithPermissionChecker(ctx context.Context, checker PermissionChecker) context.Context {
	return context.WithValue(ctx, permissionCheckerKey, checker)
}

// GetPermissionChecker retrieves the permission checker from context.
// Tools should call this at the beginning of Execute to get permission checking capabilities.
// Returns (checker, true) if found, (nil, false) otherwise.
func GetPermissionChecker(ctx context.Context) (PermissionChecker, bool) {
	checker, ok := ctx.Value(permissionCheckerKey).(PermissionChecker)
	return checker, ok
}

// internalPermissionChecker implements PermissionChecker using a PermissionUI.
type internalPermissionChecker struct {
	ui PermissionUI
}

// NewPermissionChecker creates a new permission checker with the given UI.
// If ui is nil, all permission checks will default to allowed.
func NewPermissionChecker(ui PermissionUI) PermissionChecker {
	return &internalPermissionChecker{ui: ui}
}

// CheckPermission checks permission without user interaction.
// For now, this returns false to indicate that AskPermission should be used.
// In the future, this could check cached permissions or policy rules.
func (c *internalPermissionChecker) CheckPermission(ctx context.Context, operation, resource string) (bool, error) {
	// Non-interactive check - could check cached permissions or policies
	// For now, return false to indicate need for AskPermission
	return false, nil
}

// AskPermission asks user for permission interactively.
func (c *internalPermissionChecker) AskPermission(ctx context.Context, operation, resource, reason string) (bool, error) {
	if c.ui == nil {
		// No UI available, default to allow
		return true, nil
	}

	return c.ui.RequestPermission(ctx, PermissionRequest{
		Operation: operation,
		Resource:  resource,
		Reason:    reason,
	})
}

// AlwaysAllowChecker is a permission checker that always grants permission.
// Useful for testing or when permissions are disabled.
type AlwaysAllowChecker struct{}

// NewAlwaysAllowChecker creates a permission checker that always allows operations.
func NewAlwaysAllowChecker() PermissionChecker {
	return &AlwaysAllowChecker{}
}

func (c *AlwaysAllowChecker) CheckPermission(ctx context.Context, operation, resource string) (bool, error) {
	return true, nil
}

func (c *AlwaysAllowChecker) AskPermission(ctx context.Context, operation, resource, reason string) (bool, error) {
	return true, nil
}

// AlwaysDenyChecker is a permission checker that always denies permission.
// Useful for testing or in restricted environments.
type AlwaysDenyChecker struct{}

// NewAlwaysDenyChecker creates a permission checker that always denies operations.
func NewAlwaysDenyChecker() PermissionChecker {
	return &AlwaysDenyChecker{}
}

func (c *AlwaysDenyChecker) CheckPermission(ctx context.Context, operation, resource string) (bool, error) {
	return false, nil
}

func (c *AlwaysDenyChecker) AskPermission(ctx context.Context, operation, resource, reason string) (bool, error) {
	return false, nil
}
