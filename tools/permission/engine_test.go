package permission

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	config := DefaultConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if engine == nil {
		t.Fatal("Engine is nil")
	}
}

func TestEvaluate(t *testing.T) {
	config := DefaultConfig()
	config.Ruleset = NewRuleset().
		AddRule(Rule{
			ID:         "allow-read",
			Permission: "file:read",
			Action:     ActionAllow,
			Enabled:    true,
		}).
		AddRule(Rule{
			ID:         "deny-write",
			Permission: "file:write",
			Action:     ActionDeny,
			Enabled:    true,
		}).
		SetDefaultAction(ActionAsk)

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	tests := []struct {
		name       string
		permission string
		wantAction Action
	}{
		{"allow read", "file:read", ActionAllow},
		{"deny write", "file:write", ActionDeny},
		{"ask delete", "file:delete", ActionAsk},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(tt.permission, "*", "")
			if result.Action != tt.wantAction {
				t.Errorf("Evaluate() = %v, want %v", result.Action, tt.wantAction)
			}
		})
	}
}

func TestAsk(t *testing.T) {
	config := DefaultConfig()
	config.Ruleset = NewRuleset().
		AddRule(Rule{
			ID:         "allow-read",
			Permission: "file:read",
			Action:     ActionAllow,
			Enabled:    true,
		}).
		AddRule(Rule{
			ID:         "deny-write",
			Permission: "file:write",
			Action:     ActionDeny,
			Enabled:    true,
		})

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	tests := []struct {
		name    string
		req     PermissionRequest
		wantErr bool
		errCode ErrorCode
	}{
		{
			name: "allow read",
			req: PermissionRequest{
				Permission: "file:read",
				Patterns:   []string{"/test.txt"},
			},
			wantErr: false,
		},
		{
			name: "deny write",
			req: PermissionRequest{
				Permission: "file:write",
				Patterns:   []string{"/test.txt"},
			},
			wantErr: true,
			errCode: ErrCodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.Ask(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Ask() expected error, got nil")
					return
				}
				if permErr, ok := err.(*PermissionError); ok {
					if permErr.Code != tt.errCode {
						t.Errorf("Ask() error code = %v, want %v", permErr.Code, tt.errCode)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Ask() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSimpleEngine(t *testing.T) {
	engine := NewSimpleEngine(ActionAsk)

	// Test default action
	result := engine.Evaluate("unknown:action", "*", "")
	if result.Action != ActionAsk {
		t.Errorf("Default action = %v, want %v", result.Action, ActionAsk)
	}

	// Add a rule
	ruleset := NewRuleset()
	ruleset.AddRule(Rule{
		ID:         "allow-bash",
		Permission: "bash:execute",
		Action:     ActionAllow,
	})
	engine.UpdateRuleset(ruleset)

	result = engine.Evaluate("bash:execute", "*", "")
	if result.Action != ActionAllow {
		t.Errorf("After adding rule, action = %v, want %v", result.Action, ActionAllow)
	}
}

func TestEngineTimeout(t *testing.T) {
	config := DefaultConfig()
	config.Ruleset = NewRuleset().SetDefaultAction(ActionAsk)

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Set short timeout
	engine.SetDefaultTimeout(1 * time.Millisecond)

	// This should timeout if event bus is set
	req := PermissionRequest{
		Permission: "file:write",
		Patterns:   []string{"/test.txt"},
		Silent:     false,
	}

	// Without event bus, should return pending error immediately
	err = engine.Ask(req)
	if err == nil {
		t.Error("Expected error for ActionAsk without event bus")
	}
}
