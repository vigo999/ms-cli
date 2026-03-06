package context

import (
	"testing"

	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestNewManager(t *testing.T) {
	cfg := DefaultManagerConfig()
	mgr := NewManager(cfg)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.config.MaxTokens != 24000 {
		t.Errorf("Expected MaxTokens to be 24000, got %d", mgr.config.MaxTokens)
	}

	if mgr.budget == nil {
		t.Error("Budget should be initialized")
	}

	if mgr.tokenizer == nil {
		t.Error("Tokenizer should be initialized")
	}

	if mgr.compactor == nil {
		t.Error("Compactor should be initialized")
	}
}

func TestSetSystemPrompt(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	prompt := "You are a helpful assistant."
	mgr.SetSystemPrompt(prompt)

	systemMsg := mgr.GetSystemPrompt()
	if systemMsg == nil {
		t.Fatal("System prompt should not be nil")
	}

	if systemMsg.Content != prompt {
		t.Errorf("Expected system prompt '%s', got '%s'", prompt, systemMsg.Content)
	}

	if systemMsg.Role != "system" {
		t.Errorf("Expected role 'system', got '%s'", systemMsg.Role)
	}
}

func TestAddMessage(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	msg := llm.NewUserMessage("Hello")
	err := mgr.AddMessage(msg)
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	messages := mgr.GetNonSystemMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Content != "Hello" {
		t.Errorf("Expected message content 'Hello', got '%s'", messages[0].Content)
	}
}

func TestAddToolResult(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	err := mgr.AddToolResult("call_123", "Result content")
	if err != nil {
		t.Fatalf("AddToolResult failed: %v", err)
	}

	messages := mgr.GetNonSystemMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", messages[0].Role)
	}

	if messages[0].ToolCallID != "call_123" {
		t.Errorf("Expected ToolCallID 'call_123', got '%s'", messages[0].ToolCallID)
	}
}

func TestGetMessages(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	// Set system prompt
	mgr.SetSystemPrompt("System prompt")

	// Add user message
	mgr.AddMessage(llm.NewUserMessage("Hello"))

	// Get all messages
	messages := mgr.GetMessages()

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages (system + user), got %d", len(messages))
	}

	if messages[0].Role != "system" {
		t.Errorf("First message should be system, got %s", messages[0].Role)
	}

	if messages[1].Role != "user" {
		t.Errorf("Second message should be user, got %s", messages[1].Role)
	}
}

func TestClear(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	mgr.SetSystemPrompt("System")
	mgr.AddMessage(llm.NewUserMessage("Hello"))
	mgr.AddMessage(llm.NewAssistantMessage("Hi"))

	mgr.Clear()

	nonSystem := mgr.GetNonSystemMessages()
	if len(nonSystem) != 0 {
		t.Errorf("Expected 0 non-system messages after clear, got %d", len(nonSystem))
	}

	// System prompt should still exist
	system := mgr.GetSystemPrompt()
	if system == nil {
		t.Error("System prompt should persist after Clear()")
	}
}

func TestTokenUsage(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	initialUsage := mgr.TokenUsage()
	if initialUsage.Current != 0 {
		t.Errorf("Expected initial usage to be 0, got %d", initialUsage.Current)
	}

	// Add messages
	mgr.AddMessage(llm.NewUserMessage("Hello world"))

	usage := mgr.TokenUsage()
	if usage.Current == 0 {
		t.Error("Token usage should increase after adding message")
	}

	if usage.Max != 24000 {
		t.Errorf("Expected Max to be 24000, got %d", usage.Max)
	}
}

func TestIsWithinBudget(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.MaxTokens = 100
	cfg.ReserveTokens = 20

	mgr := NewManager(cfg)

	// Small message should be within budget
	smallMsg := llm.NewUserMessage("Hi")
	if !mgr.IsWithinBudget(smallMsg) {
		t.Error("Small message should be within budget")
	}
}

func TestEstimateTokens(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	msgs := []llm.Message{
		llm.NewUserMessage("Hello"),
		llm.NewAssistantMessage("World"),
	}

	tokens := mgr.EstimateTokens(msgs)
	if tokens <= 0 {
		t.Error("Estimated tokens should be positive")
	}
}

func TestGetStats(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())
	mgr.SetSystemPrompt("System")
	mgr.AddMessage(llm.NewUserMessage("Hello"))

	stats := mgr.GetStats()

	if stats["total_messages"] != 1 {
		t.Errorf("Expected 1 message in stats, got %v", stats["total_messages"])
	}

	if stats["has_system_prompt"] != true {
		t.Error("Expected has_system_prompt to be true")
	}
}

func TestCompact(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.MaxHistoryRounds = 2

	mgr := NewManager(cfg)

	// Add many messages
	for i := 0; i < 10; i++ {
		mgr.AddMessage(llm.NewUserMessage("Message"))
	}

	err := mgr.Compact()
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// After compaction, message count should be reduced
	messages := mgr.GetNonSystemMessages()
	if len(messages) > 10 {
		t.Errorf("Expected messages to be compacted, got %d", len(messages))
	}
}

func TestSetCompactStrategy(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	mgr.SetCompactStrategy(CompactStrategySummarize)

	if mgr.config.CompactStrategy != CompactStrategySummarize {
		t.Errorf("Expected strategy to be CompactStrategySummarize, got %v", mgr.config.CompactStrategy)
	}
}

func TestGetMessagePriority(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())
	mgr.SetSystemPrompt("System")
	mgr.AddMessage(llm.NewUserMessage("Hello"))

	priority := mgr.GetMessagePriority(0)
	if priority <= 0 {
		t.Error("Priority should be positive")
	}
}

func TestTruncateTo(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	// Add messages
	for i := 0; i < 5; i++ {
		mgr.AddMessage(llm.NewUserMessage("Message"))
	}

	mgr.TruncateTo(2)

	messages := mgr.GetNonSystemMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages after truncate, got %d", len(messages))
	}
}

func TestBudgetAllocation(t *testing.T) {
	allocation := DefaultBudgetAllocation()

	if allocation.SystemPercent+allocation.HistoryPercent+
		allocation.ToolResultPercent+allocation.ReservePercent != 100 {
		t.Error("Budget allocation percentages should sum to 100")
	}

	budget, err := NewBudget(10000, allocation)
	if err != nil {
		t.Fatalf("NewBudget failed: %v", err)
	}

	if budget.GetSystemBudget() <= 0 {
		t.Error("System budget should be positive")
	}

	if budget.GetHistoryBudget() <= 0 {
		t.Error("History budget should be positive")
	}
}

func TestBudgetValidation(t *testing.T) {
	// Invalid: sum != 100
	invalidAllocation := BudgetAllocation{
		SystemPercent:     50,
		HistoryPercent:    40,
		ToolResultPercent: 0,
		ReservePercent:    0,
	}

	_, err := NewBudget(10000, invalidAllocation)
	if err == nil {
		t.Error("Expected error for invalid budget allocation")
	}
}

func TestPriorityScorer(t *testing.T) {
	scorer := NewPriorityScorer()

	// System message should have high priority
	systemMsg := llm.NewSystemMessage("System prompt")
	priority := scorer.ScoreMessage(systemMsg, 0, 1)

	if priority < PriorityHigh {
		t.Errorf("System message should have high priority, got %d", priority)
	}

	// User message
	userMsg := llm.NewUserMessage("Hello")
	priority = scorer.ScoreMessage(userMsg, 0, 1)

	if priority < PriorityMedium {
		t.Errorf("User message should have at least medium priority, got %d", priority)
	}
}

func TestCompactResult(t *testing.T) {
	result := CompactResult{
		Kept:     5,
		Removed:  3,
		Strategy: CompactStrategySimple,
		Summary:  "Test summary",
	}

	str := result.String()
	if str == "" {
		t.Error("CompactResult.String() should not be empty")
	}
}

func TestReplaceMessages(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())
	mgr.SetSystemPrompt("system")

	recovered := []llm.Message{
		llm.NewUserMessage("u1"),
		llm.NewAssistantMessage("a1"),
		llm.NewToolMessage("call_1", "tool output"),
	}
	mgr.ReplaceMessages(recovered)

	msgs := mgr.GetNonSystemMessages()
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3", len(msgs))
	}
	if msgs[2].Role != "tool" {
		t.Fatalf("third message role = %s, want tool", msgs[2].Role)
	}

	stats := mgr.GetStats()
	total, _ := stats["total_messages"].(int)
	if total != 3 {
		t.Fatalf("total_messages = %d, want 3", total)
	}
	toolCalls, _ := stats["tool_call_count"].(int)
	if toolCalls != 1 {
		t.Fatalf("tool_call_count = %d, want 1", toolCalls)
	}
}
