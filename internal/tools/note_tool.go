// Package tools provides tool implementations for UTA agents.
package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// NoteType defines types of notes for long-horizon tasks.
type NoteType string

const (
	NoteTypeTaskState  NoteType = "task_state"  // Task progress state
	NoteTypeConclusion NoteType = "conclusion"  // Key findings/conclusions
	NoteTypeBlocker    NoteType = "blocker"     // Obstacles or issues
	NoteTypeAction     NoteType = "action"      // Planned actions
	NoteTypeInsight    NoteType = "insight"     // Important insights
)

// Note represents a persistent note for long-horizon tasks.
type Note struct {
	ID        string         `yaml:"id"`
	Title     string         `yaml:"title"`
	Type      NoteType       `yaml:"type"`
	Tags      []string       `yaml:"tags,omitempty"`
	CreatedAt time.Time      `yaml:"created_at"`
	UpdatedAt time.Time      `yaml:"updated_at"`
	Content   string         `yaml:"-"` // Markdown body (not in front matter)
	Metadata  map[string]any `yaml:"metadata,omitempty"`
}

// NoteMetadata is used for indexing notes without loading full content.
type NoteMetadata struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      NoteType  `json:"type"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	FilePath  string    `json:"file_path"`
}

// NoteTool provides persistent note management for long-horizon tasks.
type NoteTool struct {
	workspace string
	index     map[string]*NoteMetadata
	mu        sync.RWMutex
}

// NewNoteTool creates a new note tool with the given workspace directory.
func NewNoteTool(workspace string) (*NoteTool, error) {
	if workspace == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		workspace = filepath.Join(home, ".uta", "notes")
	}

	// Ensure workspace exists
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	tool := &NoteTool{
		workspace: workspace,
		index:     make(map[string]*NoteMetadata),
	}

	// Build initial index
	if err := tool.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("failed to build index: %w", err)
	}

	return tool, nil
}

// Create creates a new note with the given parameters.
func (t *NoteTool) Create(ctx context.Context, title, content string, noteType NoteType, tags []string, metadata map[string]any) (*Note, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	now := time.Now()
	id := t.generateID(title, now)

	note := &Note{
		ID:        id,
		Title:     title,
		Type:      noteType,
		Tags:      tags,
		CreatedAt: now,
		UpdatedAt: now,
		Content:   content,
		Metadata:  metadata,
	}

	// Save to file
	if err := t.saveNote(note); err != nil {
		return nil, err
	}

	// Update index
	t.mu.Lock()
	t.index[id] = &NoteMetadata{
		ID:        note.ID,
		Title:     note.Title,
		Type:      note.Type,
		Tags:      note.Tags,
		CreatedAt: note.CreatedAt,
		UpdatedAt: note.UpdatedAt,
		FilePath:  t.notePath(note.ID),
	}
	t.mu.Unlock()

	return note, nil
}

// Read retrieves a note by ID.
func (t *NoteTool) Read(ctx context.Context, id string) (*Note, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	filePath := t.notePath(id)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("note not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read note: %w", err)
	}

	note, err := t.parseNote(data)
	if err != nil {
		return nil, err
	}

	note.ID = id
	return note, nil
}

// Update updates an existing note's content.
func (t *NoteTool) Update(ctx context.Context, id, content string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	note, err := t.Read(ctx, id)
	if err != nil {
		return err
	}

	note.Content = content
	note.UpdatedAt = time.Now()

	if err := t.saveNote(note); err != nil {
		return err
	}

	// Update index timestamp
	t.mu.Lock()
	if meta, ok := t.index[id]; ok {
		meta.UpdatedAt = note.UpdatedAt
	}
	t.mu.Unlock()

	return nil
}

// UpdateMetadata updates a note's metadata.
func (t *NoteTool) UpdateMetadata(ctx context.Context, id string, metadata map[string]any) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	note, err := t.Read(ctx, id)
	if err != nil {
		return err
	}

	if note.Metadata == nil {
		note.Metadata = make(map[string]any)
	}
	for k, v := range metadata {
		note.Metadata[k] = v
	}
	note.UpdatedAt = time.Now()

	if err := t.saveNote(note); err != nil {
		return err
	}

	t.mu.Lock()
	if meta, ok := t.index[id]; ok {
		meta.UpdatedAt = note.UpdatedAt
	}
	t.mu.Unlock()

	return nil
}

// Delete removes a note by ID.
func (t *NoteTool) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	filePath := t.notePath(id)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("note not found: %s", id)
		}
		return fmt.Errorf("failed to delete note: %w", err)
	}

	t.mu.Lock()
	delete(t.index, id)
	t.mu.Unlock()

	return nil
}

// Search finds notes matching the query.
func (t *NoteTool) Search(ctx context.Context, query string, noteType NoteType, tags []string, limit int) ([]*Note, error) {
	if limit <= 0 {
		limit = 10
	}

	t.mu.RLock()
	var candidates []*NoteMetadata
	for _, meta := range t.index {
		// Filter by type
		if noteType != "" && meta.Type != noteType {
			continue
		}
		// Filter by tags
		if len(tags) > 0 {
			if !t.hasAllTags(meta.Tags, tags) {
				continue
			}
		}
		// Filter by query (title match)
		if query != "" {
			if !strings.Contains(strings.ToLower(meta.Title), strings.ToLower(query)) {
				continue
			}
		}
		candidates = append(candidates, meta)
	}
	t.mu.RUnlock()

	// Sort by update time (most recent first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
	})

	// Limit results
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	// Load full notes
	var results []*Note
	for _, meta := range candidates {
		note, err := t.Read(ctx, meta.ID)
		if err != nil {
			continue
		}

		// Also search in content if query is specified
		if query != "" && !strings.Contains(strings.ToLower(note.Title), strings.ToLower(query)) {
			if !strings.Contains(strings.ToLower(note.Content), strings.ToLower(query)) {
				continue
			}
		}

		results = append(results, note)
	}

	return results, nil
}

// List returns all notes, optionally filtered by type.
func (t *NoteTool) List(ctx context.Context, noteType NoteType, limit int) ([]*NoteMetadata, error) {
	if limit <= 0 {
		limit = 100
	}

	t.mu.RLock()
	var results []*NoteMetadata
	for _, meta := range t.index {
		if noteType != "" && meta.Type != noteType {
			continue
		}
		results = append(results, meta)
	}
	t.mu.RUnlock()

	// Sort by update time (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetByTag returns notes with a specific tag.
func (t *NoteTool) GetByTag(ctx context.Context, tag string, limit int) ([]*Note, error) {
	return t.Search(ctx, "", "", []string{tag}, limit)
}

// GetTaskProgress retrieves the latest task state for a given task.
func (t *NoteTool) GetTaskProgress(ctx context.Context, taskTag string) (*Note, error) {
	notes, err := t.Search(ctx, "", NoteTypeTaskState, []string{taskTag}, 1)
	if err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		return nil, fmt.Errorf("no task progress found for: %s", taskTag)
	}
	return notes[0], nil
}

// SaveTaskProgress saves or updates task progress.
func (t *NoteTool) SaveTaskProgress(ctx context.Context, taskTag, title, content string, metadata map[string]any) (*Note, error) {
	// Try to find existing task note
	existing, err := t.GetTaskProgress(ctx, taskTag)
	if err == nil {
		// Update existing
		if err := t.Update(ctx, existing.ID, content); err != nil {
			return nil, err
		}
		if metadata != nil {
			if err := t.UpdateMetadata(ctx, existing.ID, metadata); err != nil {
				return nil, err
			}
		}
		return t.Read(ctx, existing.ID)
	}

	// Create new
	tags := []string{taskTag, "long-horizon"}
	return t.Create(ctx, title, content, NoteTypeTaskState, tags, metadata)
}

// Helper methods

func (t *NoteTool) notePath(id string) string {
	return filepath.Join(t.workspace, id+".md")
}

func (t *NoteTool) generateID(title string, timestamp time.Time) string {
	data := fmt.Sprintf("%s-%d", title, timestamp.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

func (t *NoteTool) saveNote(note *Note) error {
	// Build YAML front matter
	frontMatter := map[string]any{
		"id":         note.ID,
		"title":      note.Title,
		"type":       note.Type,
		"created_at": note.CreatedAt.Format(time.RFC3339),
		"updated_at": note.UpdatedAt.Format(time.RFC3339),
	}
	if len(note.Tags) > 0 {
		frontMatter["tags"] = note.Tags
	}
	if len(note.Metadata) > 0 {
		frontMatter["metadata"] = note.Metadata
	}

	yamlData, err := yaml.Marshal(frontMatter)
	if err != nil {
		return fmt.Errorf("failed to marshal front matter: %w", err)
	}

	// Build full content
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(string(yamlData))
	sb.WriteString("---\n\n")
	sb.WriteString(note.Content)

	// Write to file
	filePath := t.notePath(note.ID)
	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write note: %w", err)
	}

	return nil
}

func (t *NoteTool) parseNote(data []byte) (*Note, error) {
	// Parse YAML front matter
	content := string(data)

	// Check for front matter delimiters
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("invalid note format: missing front matter")
	}

	// Find end of front matter
	endIndex := strings.Index(content[4:], "\n---\n")
	if endIndex == -1 {
		return nil, fmt.Errorf("invalid note format: unterminated front matter")
	}

	frontMatterStr := content[4 : 4+endIndex]
	noteContent := strings.TrimSpace(content[4+endIndex+5:])

	// Parse YAML
	var frontMatter struct {
		ID        string         `yaml:"id"`
		Title     string         `yaml:"title"`
		Type      NoteType       `yaml:"type"`
		Tags      []string       `yaml:"tags"`
		CreatedAt string         `yaml:"created_at"`
		UpdatedAt string         `yaml:"updated_at"`
		Metadata  map[string]any `yaml:"metadata"`
	}

	if err := yaml.Unmarshal([]byte(frontMatterStr), &frontMatter); err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}

	// Parse timestamps
	createdAt, _ := time.Parse(time.RFC3339, frontMatter.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, frontMatter.UpdatedAt)

	return &Note{
		ID:        frontMatter.ID,
		Title:     frontMatter.Title,
		Type:      frontMatter.Type,
		Tags:      frontMatter.Tags,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Content:   noteContent,
		Metadata:  frontMatter.Metadata,
	}, nil
}

func (t *NoteTool) rebuildIndex() error {
	entries, err := os.ReadDir(t.workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(t.workspace, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		note, err := t.parseNote(data)
		if err != nil {
			continue
		}

		t.mu.Lock()
		t.index[note.ID] = &NoteMetadata{
			ID:        note.ID,
			Title:     note.Title,
			Type:      note.Type,
			Tags:      note.Tags,
			CreatedAt: note.CreatedAt,
			UpdatedAt: note.UpdatedAt,
			FilePath:  filePath,
		}
		t.mu.Unlock()
	}

	return nil
}

func (t *NoteTool) hasAllTags(noteTags, requiredTags []string) bool {
	noteTagSet := make(map[string]bool)
	for _, t := range noteTags {
		noteTagSet[strings.ToLower(t)] = true
	}
	for _, req := range requiredTags {
		if !noteTagSet[strings.ToLower(req)] {
			return false
		}
	}
	return true
}

// Execute implements the Tool interface for NoteTool.
func (t *NoteTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	action, ok := params["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action parameter required")
	}

	switch action {
	case "create":
		title, _ := params["title"].(string)
		content, _ := params["content"].(string)
		noteType := NoteTypeTaskState
		if nt, ok := params["type"].(string); ok {
			noteType = NoteType(nt)
		}
		tags := getStringSlice(params, "tags")
		metadata := getMapAny(params, "metadata")

		note, err := t.Create(ctx, title, content, noteType, tags, metadata)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id":      note.ID,
			"title":   note.Title,
			"type":    note.Type,
			"created": true,
		}, nil

	case "read":
		id, _ := params["id"].(string)
		note, err := t.Read(ctx, id)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id":        note.ID,
			"title":     note.Title,
			"type":      note.Type,
			"content":   note.Content,
			"tags":      note.Tags,
			"metadata":  note.Metadata,
			"created_at": note.CreatedAt.Format(time.RFC3339),
			"updated_at": note.UpdatedAt.Format(time.RFC3339),
		}, nil

	case "update":
		id, _ := params["id"].(string)
		content, _ := params["content"].(string)
		if err := t.Update(ctx, id, content); err != nil {
			return nil, err
		}
		return map[string]any{
			"id":      id,
			"updated": true,
		}, nil

	case "delete":
		id, _ := params["id"].(string)
		if err := t.Delete(ctx, id); err != nil {
			return nil, err
		}
		return map[string]any{
			"id":      id,
			"deleted": true,
		}, nil

	case "search":
		query, _ := params["query"].(string)
		noteType := NoteType("")
		if nt, ok := params["type"].(string); ok {
			noteType = NoteType(nt)
		}
		tags := getStringSlice(params, "tags")
		limit := 10
		if l, ok := params["limit"].(int); ok {
			limit = l
		}
		if l, ok := params["limit"].(float64); ok {
			limit = int(l)
		}

		notes, err := t.Search(ctx, query, noteType, tags, limit)
		if err != nil {
			return nil, err
		}

		results := make([]map[string]any, len(notes))
		for i, note := range notes {
			results[i] = map[string]any{
				"id":         note.ID,
				"title":      note.Title,
				"type":       note.Type,
				"tags":       note.Tags,
				"content":    note.Content,
				"updated_at": note.UpdatedAt.Format(time.RFC3339),
			}
		}
		return map[string]any{
			"count":   len(results),
			"results": results,
		}, nil

	case "list":
		noteType := NoteType("")
		if nt, ok := params["type"].(string); ok {
			noteType = NoteType(nt)
		}
		limit := 100
		if l, ok := params["limit"].(int); ok {
			limit = l
		}
		if l, ok := params["limit"].(float64); ok {
			limit = int(l)
		}

		metas, err := t.List(ctx, noteType, limit)
		if err != nil {
			return nil, err
		}

		results := make([]map[string]any, len(metas))
		for i, meta := range metas {
			results[i] = map[string]any{
				"id":         meta.ID,
				"title":      meta.Title,
				"type":       meta.Type,
				"tags":       meta.Tags,
				"updated_at": meta.UpdatedAt.Format(time.RFC3339),
			}
		}
		return map[string]any{
			"count":   len(results),
			"results": results,
		}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// GetName returns the tool name.
func (t *NoteTool) GetName() string {
	return "note"
}

// GetDescription returns the tool description.
func (t *NoteTool) GetDescription() string {
	return "Manage persistent notes for long-horizon tasks. Actions: create, read, update, delete, search, list. Use for saving task progress, conclusions, blockers, and insights."
}

// GetParameters returns the tool parameters schema.
func (t *NoteTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action to perform: create, read, update, delete, search, list",
				"enum":        []string{"create", "read", "update", "delete", "search", "list"},
			},
			"id": map[string]any{
				"type":        "string",
				"description": "Note ID (required for read, update, delete)",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Note title (required for create)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Note content in Markdown",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "Note type: task_state, conclusion, blocker, action, insight",
				"enum":        []string{"task_state", "conclusion", "blocker", "action", "insight"},
				"default":     "task_state",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tags for categorization",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (for search action)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results",
				"default":     10,
			},
			"metadata": map[string]any{
				"type":        "object",
				"description": "Additional metadata",
			},
		},
		"required": []string{"action"},
	}
}

// Stats returns statistics about notes.
func (t *NoteTool) Stats() map[string]any {
	t.mu.RLock()
	defer t.mu.RUnlock()

	typeCounts := make(map[NoteType]int)
	tagCounts := make(map[string]int)

	for _, meta := range t.index {
		typeCounts[meta.Type]++
		for _, tag := range meta.Tags {
			tagCounts[tag]++
		}
	}

	return map[string]any{
		"total_notes":   len(t.index),
		"by_type":       typeCounts,
		"top_tags":      getTopTags(tagCounts, 10),
		"workspace":     t.workspace,
	}
}

// Helper functions

func getStringSlice(params map[string]any, key string) []string {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []any:
			result := make([]string, len(v))
			for i, item := range v {
				if s, ok := item.(string); ok {
					result[i] = s
				}
			}
			return result
		}
	}
	return nil
}

func getMapAny(params map[string]any, key string) map[string]any {
	if val, ok := params[key]; ok {
		if m, ok := val.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func getTopTags(tagCounts map[string]int, limit int) []map[string]any {
	type tagCount struct {
		tag   string
		count int
	}

	var counts []tagCount
	for tag, count := range tagCounts {
		counts = append(counts, tagCount{tag, count})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	result := make([]map[string]any, 0, limit)
	for i := 0; i < len(counts) && i < limit; i++ {
		result = append(result, map[string]any{
			"tag":   counts[i].tag,
			"count": counts[i].count,
		})
	}
	return result
}

// CompactNoteRegex matches content for summarization
var CompactNoteRegex = regexp.MustCompile(`(?m)^#{1,3}\s+.+$`)