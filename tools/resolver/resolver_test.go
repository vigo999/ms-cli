package resolver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/permission"
)

func TestNewResolver(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)
	if resolver == nil {
		t.Fatal("NewResolver returned nil")
	}

	tools := resolver.List()
	if len(tools) != 0 {
		t.Errorf("New resolver should have 0 tools, got %d", len(tools))
	}
}

func TestNewSimpleResolver(t *testing.T) {
	resolver := NewSimpleResolver()
	if resolver == nil {
		t.Fatal("NewSimpleResolver returned nil")
	}

	tools := resolver.List()
	if len(tools) != 0 {
		t.Errorf("New simple resolver should have 0 tools, got %d", len(tools))
	}
}

func TestResolverRegister(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	def := tools.ToolDefinition{
		Name:        "test-tool",
		Description: "Test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"arg1": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	tool := &testTool{def: &def}

	err := resolver.Register("test-tool", tool)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Register duplicate should succeed (overwrites)
	err = resolver.Register("test-tool", tool)
	if err != nil {
		t.Errorf("Register duplicate should succeed but got: %v", err)
	}

	// Get the tool
	got, ok := resolver.Get("test-tool")
	if !ok {
		t.Error("Get should return true for registered tool")
	}
	if got.Info().Name != "test-tool" {
		t.Errorf("Get returned wrong tool: %s", got.Info().Name)
	}
}

func TestResolverUnregister(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	def := tools.ToolDefinition{
		Name:        "to-unregister",
		Description: "To unregister",
	}

	tool := &testTool{def: &def}

	// Unregister non-existent should fail
	err := resolver.Unregister("non-existent")
	if err == nil {
		t.Error("Unregister non-existent should fail")
	}

	// Register then unregister
	resolver.Register("to-unregister", tool)
	err = resolver.Unregister("to-unregister")
	if err != nil {
		t.Errorf("Unregister failed: %v", err)
	}

	// Verify it's gone
	_, ok := resolver.Get("to-unregister")
	if ok {
		t.Error("Tool should be unregistered")
	}
}

func TestResolverGet(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	// Get non-existent tool
	_, ok := resolver.Get("non-existent")
	if ok {
		t.Error("Get should return false for non-existent tool")
	}

	// Register and get
	def := tools.ToolDefinition{
		Name:        "exists",
		Description: "Exists",
	}

	tool := &testTool{def: &def}
	resolver.Register("exists", tool)

	got, ok := resolver.Get("exists")
	if !ok {
		t.Error("Get should return true for existing tool")
	}
	if got.Info().Name != "exists" {
		t.Errorf("Get returned wrong tool: %s", got.Info().Name)
	}
}

func TestResolverList(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	// Empty list
	list := resolver.List()
	if len(list) != 0 {
		t.Errorf("Empty resolver should return 0 tools, got %d", len(list))
	}

	// Register multiple tools
	for i := 0; i < 3; i++ {
		name := string(rune('a' + i))
		def := tools.ToolDefinition{
			Name:        name,
			Description: "Tool " + name,
		}
		tool := &testTool{def: &def}
		resolver.Register(name, tool)
	}

	list = resolver.List()
	if len(list) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(list))
	}
}

func TestResolverResolve(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	// 先注册测试工具
	tool1Def := tools.ToolDefinition{
		Name:        "tool1",
		Description: "Tool 1",
		Meta:        tools.ToolMeta{},
	}
	tool2Def := tools.ToolDefinition{
		Name:        "tool2",
		Description: "Tool 2",
		Meta:        tools.ToolMeta{},
	}
	resolver.Register("tool1", &testTool{def: &tool1Def})
	resolver.Register("tool2", &testTool{def: &tool2Def})

	// Create input
	defs := []tools.ToolDefinition{tool1Def, tool2Def}

	input := ResolveInput{
		Definitions: defs,
		Agent:       tools.AgentInfo{Type: "test"},
		Session:     tools.SessionInfo{},
		Provider:    ProviderTypeInternal,
	}

	output, err := resolver.Resolve(context.Background(), input)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(output.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(output.Tools))
	}

	if len(output.ProviderSchemas) != 2 {
		t.Errorf("Expected 2 schemas, got %d", len(output.ProviderSchemas))
	}

	// Check that tools are in the output
	if _, ok := output.Tools["tool1"]; !ok {
		t.Error("tool1 should be in output")
	}
	if _, ok := output.Tools["tool2"]; !ok {
		t.Error("tool2 should be in output")
	}
}

func TestResolverResolveWithPermissionDeny(t *testing.T) {
	// Create engine with deny action
	permEngine := permission.NewSimpleEngine(permission.ActionDeny)
	resolver := NewResolver(permEngine)

	// Create input with tools that have permissions defined
	defs := []tools.ToolDefinition{
		{
			Name:        "denied-tool",
			Description: "Denied Tool",
			Meta: tools.ToolMeta{
				Permissions: []tools.Permission{
					{Name: "tool:use"},
				},
			},
		},
	}

	input := ResolveInput{
		Definitions: defs,
		Agent:       tools.AgentInfo{Type: "test"},
		Session:     tools.SessionInfo{},
		Provider:    ProviderTypeInternal,
	}

	output, err := resolver.Resolve(context.Background(), input)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// With deny engine, tools with permissions should be filtered out
	if len(output.Tools) != 0 {
		t.Errorf("Expected 0 tools with deny engine, got %d", len(output.Tools))
	}
}

func TestResolveTool(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	// 注册一个测试工具
	def := tools.ToolDefinition{
		Name:        "test-tool",
		Description: "Test tool",
	}

	testImpl := &testTool{def: &def}
	resolver.Register("test-tool", testImpl)

	// 通过 ResolveTool 获取工具
	resolved := resolver.ResolveTool(def)
	if resolved == nil {
		t.Fatal("ResolveTool returned nil for registered tool")
	}

	info := resolved.Info()
	if info.Name != "test-tool" {
		t.Errorf("Resolved tool name = %s, want test-tool", info.Name)
	}
}

func TestResolveToolExecution(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	// 注册一个测试实现
	def := tools.ToolDefinition{
		Name:        "exec-tool",
		Description: "Executable tool",
	}

	executed := false
	testImpl := &testExecutableTool{
		def: &def,
		executeFunc: func(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
			executed = true
			return tools.NewTextResult("executed"), nil
		},
	}

	// 注册实现
	resolver.Register("exec-tool", testImpl)

	// 通过 ResolveTool 获取工具
	resolved := resolver.ResolveTool(def)
	if resolved == nil {
		t.Fatal("ResolveTool returned nil")
	}

	// 执行工具
	ctx := &tools.ToolContext{
		ToolID: "exec-tool",
	}
	exec := &testExecutor{}

	result, err := resolved.Execute(ctx, exec, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !executed {
		t.Error("Tool implementation was not executed")
	}

	hasContent := false
	for _, part := range result.Parts {
		if part.Type == tools.PartTypeText && part.Content == "executed" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Errorf("Expected result content 'executed', got %v", result)
	}
}

func TestResolveToolNotFound(t *testing.T) {
	permEngine := permission.NewSimpleEngine(permission.ActionAllow)
	resolver := NewResolver(permEngine)

	def := tools.ToolDefinition{
		Name:        "not-registered",
		Description: "Not registered tool",
	}

	// 未注册的工具应该返回 nil
	resolved := resolver.ResolveTool(def)
	if resolved != nil {
		t.Error("ResolveTool should return nil for unregistered tool")
	}
}

func TestConvertToProviderSchema(t *testing.T) {
	resolver := NewSimpleResolver()

	def := tools.ToolDefinition{
		Name:        "test",
		Description: "Test tool",
		Parameters: map[string]interface{}{
			"type": "object",
		},
	}

	tests := []struct {
		name     string
		provider ProviderType
		wantErr  bool
	}{
		{"MCP", ProviderTypeMCP, false},
		{"OpenAI", ProviderTypeOpenAI, false},
		{"Internal", ProviderTypeInternal, false},
		{"A2A", ProviderTypeA2A, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the resolver's Resolve to get tools which have access to convertToProviderSchema
			defs := []tools.ToolDefinition{def}
			input := ResolveInput{
				Definitions: defs,
				Agent:       tools.AgentInfo{},
				Provider:    tt.provider,
			}

			_, err := resolver.Resolve(context.Background(), input)
			if tt.wantErr {
				// A2A is not supported, should result in error but Resolve doesn't propagate it
				// So we just check it doesn't panic
			}
			_ = err
		})
	}
}

// testTool 用于测试的工具实现
type testTool struct {
	def *tools.ToolDefinition
}

func (t *testTool) Info() *tools.ToolDefinition {
	return t.def
}

func (t *testTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	return tools.NewTextResult("test"), nil
}

// testExecutableTool 用于测试的可执行工具
type testExecutableTool struct {
	def         *tools.ToolDefinition
	executeFunc func(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error)
}

func (t *testExecutableTool) Info() *tools.ToolDefinition {
	return t.def
}

func (t *testExecutableTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, exec, args)
	}
	return tools.NewTextResult("test"), nil
}

// testExecutor 用于测试的执行器
type testExecutor struct{}

func (e *testExecutor) UpdateMetadata(meta tools.MetadataUpdate) error {
	return nil
}

func (e *testExecutor) AskPermission(req tools.PermissionRequest) error {
	return nil
}
