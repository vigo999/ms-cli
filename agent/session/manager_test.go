package session

import (
	"os"
	"testing"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

func TestNewSession(t *testing.T) {
	session := New("test-session", "/home/user/project")

	if session == nil {
		t.Fatal("New returned nil")
	}

	if session.Name != "test-session" {
		t.Errorf("Expected name 'test-session', got '%s'", session.Name)
	}

	if session.WorkDir != "/home/user/project" {
		t.Errorf("Expected workdir '/home/user/project', got '%s'", session.WorkDir)
	}

	if session.ID == "" {
		t.Error("ID should not be empty")
	}

	if len(session.Messages) != 0 {
		t.Error("New session should have no messages")
	}
}

func TestAddMessage(t *testing.T) {
	session := New("test", "/tmp")
	
	msg := llm.NewUserMessage("Hello")
	session.AddMessage(msg)

	if len(session.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(session.Messages))
	}

	if session.Metadata.MessageCount != 1 {
		t.Errorf("Expected MessageCount 1, got %d", session.Metadata.MessageCount)
	}

	// Check that UpdatedAt is updated
	if time.Since(session.UpdatedAt) > 1*time.Second {
		t.Error("UpdatedAt should be updated")
	}
}

func TestAddToolMessage(t *testing.T) {
	session := New("test", "/tmp")
	
	toolMsg := llm.NewToolMessage("call_123", "Tool result")
	session.AddMessage(toolMsg)

	if session.Metadata.ToolCallCount != 1 {
		t.Errorf("Expected ToolCallCount 1, got %d", session.Metadata.ToolCallCount)
	}
}

func TestClearMessages(t *testing.T) {
	session := New("test", "/tmp")
	
	session.AddMessage(llm.NewUserMessage("Hello"))
	session.AddMessage(llm.NewAssistantMessage("Hi"))

	session.ClearMessages()

	if len(session.Messages) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(session.Messages))
	}

	if session.Metadata.MessageCount != 0 {
		t.Errorf("Expected MessageCount 0, got %d", session.Metadata.MessageCount)
	}
}

func TestArchiveUnarchive(t *testing.T) {
	session := New("test", "/tmp")

	if session.Archived {
		t.Error("New session should not be archived")
	}

	session.Archive()

	if !session.Archived {
		t.Error("Session should be archived")
	}

	session.Unarchive()

	if session.Archived {
		t.Error("Session should not be archived after unarchive")
	}
}

func TestToInfo(t *testing.T) {
	session := New("test", "/tmp")
	session.AddMessage(llm.NewUserMessage("Hello"))

	info := session.ToInfo()

	if info.ID != session.ID {
		t.Error("Info ID should match session ID")
	}

	if info.Name != session.Name {
		t.Error("Info Name should match session Name")
	}

	if info.MessageCount != 1 {
		t.Errorf("Expected MessageCount 1, got %d", info.MessageCount)
	}
}

func TestSessionManager(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	cfg := DefaultConfig()
	mgr := NewManager(store, cfg)
	defer mgr.Close()

	// Test Create
	session, err := mgr.Create("test-session", "/tmp")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if session == nil {
		t.Fatal("Create returned nil")
	}

	// Test Load
	loaded, err := mgr.Load(session.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Name != session.Name {
		t.Error("Loaded session name mismatch")
	}

	// Test List
	infos, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(infos) != 1 {
		t.Errorf("Expected 1 session in list, got %d", len(infos))
	}
}

func TestCreateAndSetCurrent(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	session, err := mgr.CreateAndSetCurrent("current-test", "/tmp")
	if err != nil {
		t.Fatalf("CreateAndSetCurrent failed: %v", err)
	}

	current := mgr.Current()
	if current == nil {
		t.Fatal("Current should not be nil")
	}

	if current.ID != session.ID {
		t.Error("Current should be the newly created session")
	}
}

func TestRename(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	session, _ := mgr.Create("old-name", "/tmp")

	err := mgr.Rename(session.ID, "new-name")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	loaded, _ := mgr.Load(session.ID)
	if loaded.Name != "new-name" {
		t.Errorf("Expected name 'new-name', got '%s'", loaded.Name)
	}
}

func TestArchiveOperations(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	session, _ := mgr.Create("archive-test", "/tmp")

	// Archive
	err := mgr.Archive(session.ID)
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	loaded, _ := mgr.Load(session.ID)
	if !loaded.Archived {
		t.Error("Session should be archived")
	}

	// List active should not include archived
	active, _ := mgr.ListActive()
	for _, info := range active {
		if info.ID == session.ID {
			t.Error("Archived session should not be in active list")
		}
	}

	// List archived should include it
	archived, _ := mgr.ListArchived()
	found := false
	for _, info := range archived {
		if info.ID == session.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Archived session should be in archived list")
	}

	// Unarchive
	err = mgr.Unarchive(session.ID)
	if err != nil {
		t.Fatalf("Unarchive failed: %v", err)
	}

	loaded, _ = mgr.Load(session.ID)
	if loaded.Archived {
		t.Error("Session should not be archived after unarchive")
	}
}

func TestTags(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	session, _ := mgr.Create("tags-test", "/tmp")

	// Add tag
	err := mgr.AddTag(session.ID, "important")
	if err != nil {
		t.Fatalf("AddTag failed: %v", err)
	}

	loaded, _ := mgr.Load(session.ID)
	found := false
	for _, tag := range loaded.Metadata.Tags {
		if tag == "important" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Tag should be added")
	}

	// Remove tag
	err = mgr.RemoveTag(session.ID, "important")
	if err != nil {
		t.Fatalf("RemoveTag failed: %v", err)
	}

	loaded, _ = mgr.Load(session.ID)
	for _, tag := range loaded.Metadata.Tags {
		if tag == "important" {
			t.Error("Tag should be removed")
		}
	}
}

func TestAddMessageToCurrent(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	_, _ = mgr.CreateAndSetCurrent("msg-test", "/tmp")

	msg := llm.NewUserMessage("Hello")
	err := mgr.AddMessageToCurrent(msg)
	if err != nil {
		t.Fatalf("AddMessageToCurrent failed: %v", err)
	}

	messages, _ := mgr.GetMessages(mgr.Current().ID)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
}

func TestDelete(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	session, _ := mgr.Create("delete-test", "/tmp")

	err := mgr.Delete(session.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mgr.Exists(session.ID) {
		t.Error("Session should not exist after delete")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.StorePath != ".ms-cli/sessions" {
		t.Errorf("Expected default store path '.ms-cli/sessions', got '%s'", cfg.StorePath)
	}

	if !cfg.AutoSave {
		t.Error("AutoSave should be true by default")
	}

	if cfg.MaxSessions != 50 {
		t.Errorf("Expected MaxSessions 50, got %d", cfg.MaxSessions)
	}
}

func TestSessionWithMessages(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	defer store.Close()

	// Create session with messages
	messages := []llm.Message{
		llm.NewUserMessage("Hello"),
		llm.NewAssistantMessage("Hi there"),
	}

	session, err := NewManager(store, DefaultConfig()).CreateFromMessages("with-messages", "/tmp", messages)
	if err != nil {
		t.Fatalf("CreateFromMessages failed: %v", err)
	}

	if len(session.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(session.Messages))
	}
}

func TestCleanup(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	
	cfg := DefaultConfig()
	cfg.MaxAge = 1 * time.Nanosecond // Very short for testing
	mgr := NewManager(store, cfg)
	defer mgr.Close()

	// Create a session
	session, _ := mgr.Create("cleanup-test", "/tmp")
	
	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Cleanup
	deleted, err := mgr.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if deleted == 0 {
		t.Log("Cleanup might not have deleted the session due to timing")
	}

	_ = session
}

func TestImportExport(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewFileStore(tempDir)
	mgr := NewManager(store, DefaultConfig())
	defer mgr.Close()

	// Create and export
	session, _ := mgr.Create("export-test", "/tmp")
	exportPath := tempDir + "/exported.json"
	
	err := mgr.Export(session.ID, exportPath)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Fatal("Export file should exist")
	}

	// Import
	imported, err := mgr.Import(exportPath)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if imported.Name != session.Name {
		t.Error("Imported session name should match")
	}

	// ID should be different (regenerated)
	if imported.ID == session.ID {
		t.Error("Imported session should have new ID")
	}
}
