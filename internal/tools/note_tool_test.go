package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewNoteTool(t *testing.T) {
	// Create temp workspace
	tmpDir := t.TempDir()

	tool, err := NewNoteTool(tmpDir)
	if err != nil {
		t.Fatalf("failed to create note tool: %v", err)
	}

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.workspace != tmpDir {
		t.Errorf("expected workspace %s, got %s", tmpDir, tool.workspace)
	}
}

func TestNewNoteToolDefaultWorkspace(t *testing.T) {
	tool, err := NewNoteTool("")
	if err != nil {
		t.Fatalf("failed to create note tool with default workspace: %v", err)
	}

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.workspace == "" {
		t.Error("expected non-empty workspace")
	}
	// Should contain .uta/notes
	if !filepath.IsAbs(tool.workspace) {
		t.Error("expected absolute path")
	}
}

func TestNoteCreate(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	note, err := tool.Create(ctx, "Test Note", "This is test content.", NoteTypeTaskState, []string{"test"}, nil)
	if err != nil {
		t.Fatalf("failed to create note: %v", err)
	}

	if note.ID == "" {
		t.Error("expected note ID to be set")
	}
	if note.Title != "Test Note" {
		t.Errorf("expected title 'Test Note', got %s", note.Title)
	}
	if note.Content != "This is test content." {
		t.Errorf("expected content 'This is test content.', got %s", note.Content)
	}
	if note.Type != NoteTypeTaskState {
		t.Errorf("expected type task_state, got %s", note.Type)
	}
	if len(note.Tags) != 1 || note.Tags[0] != "test" {
		t.Errorf("expected tags ['test'], got %v", note.Tags)
	}
	if note.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Check file was created
	filePath := filepath.Join(tmpDir, note.ID+".md")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("expected note file to be created")
	}
}

func TestNoteRead(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	created, _ := tool.Create(ctx, "Read Test", "Content to read.", NoteTypeConclusion, []string{"read"}, nil)

	read, err := tool.Read(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to read note: %v", err)
	}

	if read.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, read.ID)
	}
	if read.Title != "Read Test" {
		t.Errorf("expected title 'Read Test', got %s", read.Title)
	}
	if read.Content != "Content to read." {
		t.Errorf("expected content 'Content to read.', got %s", read.Content)
	}
}

func TestNoteReadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	_, err := tool.Read(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent note")
	}
}

func TestNoteUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	created, _ := tool.Create(ctx, "Update Test", "Original content.", NoteTypeTaskState, nil, nil)

	// Wait to ensure different timestamp
	time.Sleep(100 * time.Millisecond)

	err := tool.Update(ctx, created.ID, "Updated content.")
	if err != nil {
		t.Fatalf("failed to update note: %v", err)
	}

	read, _ := tool.Read(ctx, created.ID)
	if read.Content != "Updated content." {
		t.Errorf("expected content 'Updated content.', got %s", read.Content)
	}
	// UpdatedAt should be >= CreatedAt (may be equal if very fast)
	if read.UpdatedAt.Before(read.CreatedAt) {
		t.Errorf("updated_at should not be before created_at: created=%v, updated=%v", read.CreatedAt, read.UpdatedAt)
	}
}

func TestNoteDelete(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	created, _ := tool.Create(ctx, "Delete Test", "Content to delete.", NoteTypeTaskState, nil, nil)

	err := tool.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to delete note: %v", err)
	}

	// Verify deleted
	_, err = tool.Read(ctx, created.ID)
	if err == nil {
		t.Error("expected error reading deleted note")
	}

	// Verify file deleted
	filePath := filepath.Join(tmpDir, created.ID+".md")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected note file to be deleted")
	}
}

func TestNoteSearch(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	tool.Create(ctx, "Search Test 1", "Content about apples.", NoteTypeTaskState, []string{"fruit"}, nil)
	tool.Create(ctx, "Search Test 2", "Content about bananas.", NoteTypeConclusion, []string{"fruit"}, nil)
	tool.Create(ctx, "Other Note", "Content about cars.", NoteTypeTaskState, []string{"vehicle"}, nil)

	// Search by query
	results, err := tool.Search(ctx, "Search", "", nil, 10)
	if err != nil {
		t.Fatalf("failed to search notes: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Search by type
	results, _ = tool.Search(ctx, "", NoteTypeConclusion, nil, 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result for conclusion type, got %d", len(results))
	}

	// Search by tag
	results, _ = tool.Search(ctx, "", "", []string{"fruit"}, 10)
	if len(results) != 2 {
		t.Errorf("expected 2 results for fruit tag, got %d", len(results))
	}

	// Search with limit
	results, _ = tool.Search(ctx, "", "", nil, 1)
	if len(results) != 1 {
		t.Errorf("expected 1 result with limit, got %d", len(results))
	}
}

func TestNoteList(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	tool.Create(ctx, "List Test 1", "Content 1.", NoteTypeTaskState, nil, nil)
	tool.Create(ctx, "List Test 2", "Content 2.", NoteTypeConclusion, nil, nil)
	tool.Create(ctx, "List Test 3", "Content 3.", NoteTypeTaskState, nil, nil)

	results, err := tool.List(ctx, "", 10)
	if err != nil {
		t.Fatalf("failed to list notes: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// List by type
	results, _ = tool.List(ctx, NoteTypeTaskState, 10)
	if len(results) != 2 {
		t.Errorf("expected 2 results for task_state type, got %d", len(results))
	}
}

func TestNoteGetByTag(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	tool.Create(ctx, "Tag Test 1", "Content 1.", NoteTypeTaskState, []string{"important", "project-a"}, nil)
	tool.Create(ctx, "Tag Test 2", "Content 2.", NoteTypeTaskState, []string{"important", "project-b"}, nil)
	tool.Create(ctx, "Tag Test 3", "Content 3.", NoteTypeTaskState, []string{"project-a"}, nil)

	results, err := tool.GetByTag(ctx, "important", 10)
	if err != nil {
		t.Fatalf("failed to get notes by tag: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'important' tag, got %d", len(results))
	}
}

func TestTaskProgress(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()

	// Save initial progress
	note, err := tool.SaveTaskProgress(ctx, "hangzhou-trip", "Hangzhou Trip Progress", "## Completed\n- Research done", map[string]any{"step": 1})
	if err != nil {
		t.Fatalf("failed to save task progress: %v", err)
	}

	if note.Type != NoteTypeTaskState {
		t.Errorf("expected type task_state, got %s", note.Type)
	}

	// Get progress
	progress, err := tool.GetTaskProgress(ctx, "hangzhou-trip")
	if err != nil {
		t.Fatalf("failed to get task progress: %v", err)
	}
	if progress.Content != "## Completed\n- Research done" {
		t.Errorf("unexpected content: %s", progress.Content)
	}

	// Update progress
	time.Sleep(10 * time.Millisecond)
	tool.SaveTaskProgress(ctx, "hangzhou-trip", "Hangzhou Trip Progress", "## Completed\n- Research done\n- Hotels booked", map[string]any{"step": 2})

	updated, _ := tool.GetTaskProgress(ctx, "hangzhou-trip")
	if updated.Content != "## Completed\n- Research done\n- Hotels booked" {
		t.Errorf("expected updated content, got %s", updated.Content)
	}
}

func TestNoteExecute(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)
	ctx := context.Background()

	// Create
	result, err := tool.Execute(ctx, map[string]any{
		"action":  "create",
		"title":   "Execute Test",
		"content": "Test content",
		"type":    "task_state",
		"tags":    []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to execute create: %v", err)
	}
	noteID := result["id"].(string)

	// Read
	result, err = tool.Execute(ctx, map[string]any{
		"action": "read",
		"id":     noteID,
	})
	if err != nil {
		t.Fatalf("failed to execute read: %v", err)
	}
	if result["title"] != "Execute Test" {
		t.Errorf("expected title 'Execute Test', got %v", result["title"])
	}

	// Update
	_, err = tool.Execute(ctx, map[string]any{
		"action":  "update",
		"id":      noteID,
		"content": "Updated content",
	})
	if err != nil {
		t.Fatalf("failed to execute update: %v", err)
	}

	// Search
	result, err = tool.Execute(ctx, map[string]any{
		"action": "search",
		"query":  "Execute",
	})
	if err != nil {
		t.Fatalf("failed to execute search: %v", err)
	}
	if result["count"].(int) != 1 {
		t.Errorf("expected 1 search result, got %d", result["count"])
	}

	// List
	result, err = tool.Execute(ctx, map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("failed to execute list: %v", err)
	}
	if result["count"].(int) != 1 {
		t.Errorf("expected 1 list result, got %d", result["count"])
	}

	// Delete
	_, err = tool.Execute(ctx, map[string]any{
		"action": "delete",
		"id":     noteID,
	})
	if err != nil {
		t.Fatalf("failed to execute delete: %v", err)
	}

	// Verify deleted
	result, _ = tool.Execute(ctx, map[string]any{
		"action": "list",
	})
	if result["count"].(int) != 0 {
		t.Errorf("expected 0 results after delete, got %d", result["count"])
	}
}

func TestNoteStats(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	tool.Create(ctx, "Stats 1", "Content 1.", NoteTypeTaskState, []string{"a", "b"}, nil)
	tool.Create(ctx, "Stats 2", "Content 2.", NoteTypeConclusion, []string{"a"}, nil)
	tool.Create(ctx, "Stats 3", "Content 3.", NoteTypeBlocker, []string{"b"}, nil)

	stats := tool.Stats()

	if stats["total_notes"].(int) != 3 {
		t.Errorf("expected 3 total notes, got %d", stats["total_notes"])
	}

	byType := stats["by_type"].(map[NoteType]int)
	if byType[NoteTypeTaskState] != 1 {
		t.Errorf("expected 1 task_state, got %d", byType[NoteTypeTaskState])
	}
	if byType[NoteTypeConclusion] != 1 {
		t.Errorf("expected 1 conclusion, got %d", byType[NoteTypeConclusion])
	}
	if byType[NoteTypeBlocker] != 1 {
		t.Errorf("expected 1 blocker, got %d", byType[NoteTypeBlocker])
	}
}

func TestNoteToolInterface(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	// Verify interface compliance
	_ = tool.GetName()
	_ = tool.GetDescription()
	_ = tool.GetParameters()

	if tool.GetName() != "note" {
		t.Errorf("expected name 'note', got %s", tool.GetName())
	}
	if tool.GetDescription() == "" {
		t.Error("expected non-empty description")
	}
	if tool.GetParameters() == nil {
		t.Error("expected non-nil parameters")
	}
}

func TestNotePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create tool and note
	tool1, _ := NewNoteTool(tmpDir)
	ctx := context.Background()
	note, _ := tool1.Create(ctx, "Persistence Test", "Content to persist.", NoteTypeTaskState, []string{"persist"}, nil)

	// Create new tool instance (should load existing notes)
	tool2, err := NewNoteTool(tmpDir)
	if err != nil {
		t.Fatalf("failed to create second tool instance: %v", err)
	}

	// Verify note exists in new instance
	read, err := tool2.Read(ctx, note.ID)
	if err != nil {
		t.Fatalf("failed to read note from new instance: %v", err)
	}
	if read.Title != "Persistence Test" {
		t.Errorf("expected title 'Persistence Test', got %s", read.Title)
	}
	if read.Content != "Content to persist." {
		t.Errorf("expected content 'Content to persist.', got %s", read.Content)
	}
}

func TestNoteMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := NewNoteTool(tmpDir)

	ctx := context.Background()
	metadata := map[string]any{
		"priority":  1,
		"assignee":  "agent-1",
		"due_date":  "2026-03-30",
	}

	note, err := tool.Create(ctx, "Metadata Test", "Content.", NoteTypeTaskState, nil, metadata)
	if err != nil {
		t.Fatalf("failed to create note with metadata: %v", err)
	}

	read, _ := tool.Read(ctx, note.ID)
	if read.Metadata["priority"].(int) != 1 {
		t.Errorf("expected priority 1, got %v", read.Metadata["priority"])
	}
	if read.Metadata["assignee"].(string) != "agent-1" {
		t.Errorf("expected assignee 'agent-1', got %v", read.Metadata["assignee"])
	}

	// Update metadata
	tool.UpdateMetadata(ctx, note.ID, map[string]any{
		"status": "completed",
	})

	updated, _ := tool.Read(ctx, note.ID)
	if updated.Metadata["status"].(string) != "completed" {
		t.Errorf("expected status 'completed', got %v", updated.Metadata["status"])
	}
	// Original metadata should still exist
	if updated.Metadata["priority"].(int) != 1 {
		t.Errorf("expected priority to remain 1, got %v", updated.Metadata["priority"])
	}
}
