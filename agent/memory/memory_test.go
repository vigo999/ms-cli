package memory

import (
	"testing"
	"time"
)

func TestNewMemoryItem(t *testing.T) {
	item := NewMemoryItem(MemoryTypeFact, "Test content")

	if item == nil {
		t.Fatal("NewMemoryItem returned nil")
	}

	if item.Type != MemoryTypeFact {
		t.Errorf("Expected type %s, got %s", MemoryTypeFact, item.Type)
	}

	if item.Content != "Test content" {
		t.Errorf("Expected content 'Test content', got '%s'", item.Content)
	}

	if item.ID == "" {
		t.Error("ID should not be empty")
	}

	if item.Importance != 5 {
		t.Errorf("Expected default importance 5, got %d", item.Importance)
	}
}

func TestSetTTL(t *testing.T) {
	item := NewMemoryItem(MemoryTypeFact, "Test")

	item.SetTTL(1 * time.Hour)

	if item.ExpiresAt == nil {
		t.Fatal("ExpiresAt should be set")
	}

	if !item.ExpiresAt.After(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestIsExpired(t *testing.T) {
	item := NewMemoryItem(MemoryTypeFact, "Test")

	// Not expired initially
	if item.IsExpired() {
		t.Error("New item should not be expired")
	}

	// Set expired TTL
	item.SetTTL(-1 * time.Hour)

	if !item.IsExpired() {
		t.Error("Item with past TTL should be expired")
	}
}

func TestTags(t *testing.T) {
	item := NewMemoryItem(MemoryTypeFact, "Test")

	item.AddTag("tag1")
	item.AddTag("tag2")

	if len(item.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(item.Tags))
	}

	// Duplicate tag should not be added
	item.AddTag("tag1")
	if len(item.Tags) != 2 {
		t.Error("Duplicate tag should not be added")
	}

	// Remove tag
	item.RemoveTag("tag1")
	if len(item.Tags) != 1 {
		t.Errorf("Expected 1 tag after removal, got %d", len(item.Tags))
	}
}

func TestMetadata(t *testing.T) {
	item := NewMemoryItem(MemoryTypeFact, "Test")

	item.SetMetadata("key1", "value1")
	item.SetMetadata("key2", 123)

	val, ok := item.GetMetadata("key1")
	if !ok || val != "value1" {
		t.Error("Expected to get metadata value")
	}

	_, ok = item.GetMetadata("nonexistent")
	if ok {
		t.Error("Should return false for nonexistent key")
	}
}

func TestRecordAccess(t *testing.T) {
	item := NewMemoryItem(MemoryTypeFact, "Test")

	initialCount := item.AccessCount
	item.RecordAccess()

	if item.AccessCount != initialCount+1 {
		t.Errorf("Expected access count %d, got %d", initialCount+1, item.AccessCount)
	}

	if item.LastAccess == nil {
		t.Error("LastAccess should be set")
	}
}

func TestMemoryTypeValidation(t *testing.T) {
	tests := []struct {
		memType MemoryType
		valid   bool
	}{
		{MemoryTypeSession, true},
		{MemoryTypeFact, true},
		{MemoryTypeTask, true},
		{MemoryTypePreference, true},
		{MemoryTypeCode, true},
		{MemoryTypeDecision, true},
		{MemoryType("invalid"), false},
	}

	for _, test := range tests {
		if test.memType.IsValid() != test.valid {
			t.Errorf("Expected %s validity to be %v, got %v", test.memType, test.valid, !test.valid)
		}
	}
}

func TestDefaultQuery(t *testing.T) {
	q := DefaultQuery()

	if q.Limit != 10 {
		t.Errorf("Expected default limit 10, got %d", q.Limit)
	}

	if q.Offset != 0 {
		t.Errorf("Expected default offset 0, got %d", q.Offset)
	}

	if q.OrderBy != OrderByCreatedAt {
		t.Errorf("Expected default order by created_at, got %s", q.OrderBy)
	}
}

func TestMemoryManager(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	store, err := NewSQLiteStore(tempDir+"/test.db", DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	// Test Save
	item := NewMemoryItem(MemoryTypeFact, "Test fact")
	item.Importance = 8
	if err := mgr.Save(item); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Test Get
	retrieved, err := mgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Content != item.Content {
		t.Error("Retrieved item content mismatch")
	}

	// Test Query
	q := DefaultQuery()
	q.Types = []MemoryType{MemoryTypeFact}
	results, err := mgr.Query(q)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestSaveDifferentTypes(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewSQLiteStore(tempDir+"/test.db", DefaultConfig())
	defer store.Close()

	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	// Save session memory
	if err := mgr.SaveSessionMemory("sess_123", "Session content", 5); err != nil {
		t.Errorf("SaveSessionMemory failed: %v", err)
	}

	// Save fact
	if err := mgr.SaveFact("Test fact", []string{"tag1"}); err != nil {
		t.Errorf("SaveFact failed: %v", err)
	}

	// Save preference
	if err := mgr.SavePreference("theme", "dark"); err != nil {
		t.Errorf("SavePreference failed: %v", err)
	}

	// Save code snippet
	if err := mgr.SaveCodeSnippet("func main() {}", "go", "Main function"); err != nil {
		t.Errorf("SaveCodeSnippet failed: %v", err)
	}

	// Save task
	if err := mgr.SaveTask("Do something", "pending"); err != nil {
		t.Errorf("SaveTask failed: %v", err)
	}
}

func TestPolicy(t *testing.T) {
	policy := DefaultPolicy()

	if policy.MaxItems != 1000 {
		t.Errorf("Expected MaxItems 1000, got %d", policy.MaxItems)
	}

	// Test ShouldKeep
	item := NewMemoryItem(MemoryTypeFact, "Test")
	item.Importance = 10
	if !policy.ShouldKeep(item) {
		t.Error("High importance item should be kept")
	}

	// Test with low importance
	item.Importance = 0
	if policy.ShouldKeep(item) {
		t.Error("Low importance item should not be kept")
	}
}

func TestPolicyEvaluator(t *testing.T) {
	policy := DefaultPolicy()
	evaluator := NewPolicyEvaluator(policy)

	// Test with valid item
	item := NewMemoryItem(MemoryTypeFact, "Test")
	item.Importance = 8
	result := evaluator.Evaluate(item)

	if !result.ShouldKeep {
		t.Error("Valid item should be kept")
	}

	// Test with expired item
	item.SetTTL(-1 * time.Hour)
	result = evaluator.Evaluate(item)

	if result.ShouldKeep {
		t.Error("Expired item should not be kept")
	}
}

func TestCompactionPolicy(t *testing.T) {
	policy := DefaultCompactionPolicy()

	item := NewMemoryItem(MemoryTypeFact, "Test")
	item.Importance = 8

	score := policy.CalculatePriority(item)
	if score <= 0 {
		t.Error("Priority score should be positive")
	}

	if score > 1 {
		t.Error("Priority score should be <= 1")
	}
}

func TestConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxItems != 1000 {
		t.Errorf("Expected MaxItems 1000, got %d", cfg.MaxItems)
	}

	if !cfg.AutoCompact {
		t.Error("AutoCompact should be true by default")
	}
}

func TestMemoryStats(t *testing.T) {
	stats := MemoryStats{
		TotalCount:     100,
		ByType:         make(map[MemoryType]int64),
		AvgImportance:  5.5,
		ExpiredCount:   10,
		TotalSizeBytes: 1024,
	}

	if stats.TotalCount != 100 {
		t.Errorf("Expected TotalCount 100, got %d", stats.TotalCount)
	}
}

func TestRetrieveRecent(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewSQLiteStore(tempDir+"/test.db", DefaultConfig())
	defer store.Close()

	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	// Save items
	for i := 0; i < 5; i++ {
		item := NewMemoryItem(MemoryTypeSession, "Session")
		mgr.Save(item)
		time.Sleep(10 * time.Millisecond)
	}

	// Retrieve recent
	items, err := mgr.RetrieveRecent(1*time.Hour, 10)
	if err != nil {
		t.Fatalf("RetrieveRecent failed: %v", err)
	}

	if len(items) != 5 {
		t.Errorf("Expected 5 items, got %d", len(items))
	}
}

func TestDelete(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewSQLiteStore(tempDir+"/test.db", DefaultConfig())
	defer store.Close()

	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	item := NewMemoryItem(MemoryTypeFact, "Test")
	mgr.Save(item)

	// Delete
	if err := mgr.Delete(item.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	got, err := mgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("Should not be able to get deleted item")
	}
}

func TestDeleteExpired(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewSQLiteStore(tempDir+"/test.db", DefaultConfig())
	defer store.Close()

	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	// Create expired item
	item := NewMemoryItem(MemoryTypeFact, "Expired")
	item.SetTTL(-1 * time.Hour)
	mgr.Save(item)

	// Delete expired
	if err := mgr.DeleteExpired(); err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}

	// Verify deleted
	got, err := mgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("Expired item should be deleted")
	}
}
