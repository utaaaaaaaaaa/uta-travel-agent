// Package memory provides memory management for agents
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Item represents a single item in agent memory
type Item struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"` // thought, observation, action, result, message
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// ConversationMessage represents a message in conversation history
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Storage defines the interface for persistent memory storage
type Storage interface {
	Save(ctx context.Context, sessionID string, snapshot *Snapshot) error
	Load(ctx context.Context, sessionID string) (*Snapshot, error)
	Delete(ctx context.Context, sessionID string) error
}

// Snapshot is a serializable snapshot of memory state
type Snapshot struct {
	SessionID   string         `json:"session_id"`
	ShortTerm   []Item         `json:"short_term"`
	LongTerm    []Item         `json:"long_term"`
	Embeddings  map[string][]float32 `json:"embeddings,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ShortTerm provides in-memory working memory for agents
type ShortTerm struct {
	mu      sync.RWMutex
	items   []Item
	maxSize int
	byType  map[string][]int
}

// NewShortTerm creates a new short-term memory
func NewShortTerm(maxSize int) *ShortTerm {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &ShortTerm{
		items:   make([]Item, 0),
		maxSize: maxSize,
		byType:  make(map[string][]int),
	}
}

// Add adds a new memory item
func (m *ShortTerm) Add(itemType, content string, metadata map[string]any) *Item {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := Item{
		ID:        generateID(),
		Type:      itemType,
		Content:   content,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	if len(m.items) >= m.maxSize {
		m.trim()
	}

	idx := len(m.items)
	m.items = append(m.items, item)
	m.byType[itemType] = append(m.byType[itemType], idx)

	return &item
}

// AddThought adds a thought to memory
func (m *ShortTerm) AddThought(thought string) *Item {
	return m.Add("thought", thought, nil)
}

// AddObservation adds an observation to memory
func (m *ShortTerm) AddObservation(observation, source string) *Item {
	return m.Add("observation", observation, map[string]any{"source": source})
}

// AddAction adds an action to memory
func (m *ShortTerm) AddAction(action string, params map[string]any) *Item {
	return m.Add("action", action, params)
}

// AddResult adds a result to memory
func (m *ShortTerm) AddResult(result string, success bool, metadata map[string]any) *Item {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["success"] = success
	return m.Add("result", result, metadata)
}

// AddMessage adds a conversation message to memory
func (m *ShortTerm) AddMessage(role, content string) *Item {
	return m.Add("message", content, map[string]any{"role": role})
}

// GetAll returns all memory items
func (m *ShortTerm) GetAll() []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Item, len(m.items))
	copy(result, m.items)
	return result
}

// GetByType returns memory items of a specific type
func (m *ShortTerm) GetByType(itemType string) []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indices, ok := m.byType[itemType]
	if !ok {
		return nil
	}

	result := make([]Item, len(indices))
	for i, idx := range indices {
		result[i] = m.items[idx]
	}
	return result
}

// GetConversationHistory returns the conversation history
func (m *ShortTerm) GetConversationHistory() []ConversationMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indices, ok := m.byType["message"]
	if !ok {
		return nil
	}

	result := make([]ConversationMessage, len(indices))
	for i, idx := range indices {
		item := m.items[idx]
		role := "user"
		if r, ok := item.Metadata["role"].(string); ok {
			role = r
		}
		result[i] = ConversationMessage{
			Role:    role,
			Content: item.Content,
		}
	}
	return result
}

// GetRecent returns the most recent n items
func (m *ShortTerm) GetRecent(n int) []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n >= len(m.items) {
		return m.GetAll()
	}

	result := make([]Item, n)
	copy(result, m.items[len(m.items)-n:])
	return result
}

// Clear clears all memory
func (m *ShortTerm) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make([]Item, 0)
	m.byType = make(map[string][]int)
}

// Size returns the number of items in memory
func (m *ShortTerm) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// trim removes old items when memory is full
func (m *ShortTerm) trim() {
	if len(m.items) <= m.maxSize/2 {
		return
	}

	keep := m.maxSize / 2
	m.items = m.items[len(m.items)-keep:]

	m.byType = make(map[string][]int)
	for i, item := range m.items {
		m.byType[item.Type] = append(m.byType[item.Type], i)
	}
}

// PersistentMemory provides both short-term and long-term memory with persistence
type PersistentMemory struct {
	shortTerm *ShortTerm

	mu       sync.RWMutex
	longTerm []Item
	storage  Storage

	// Embeddings for semantic search (optional)
	embeddings map[string][]float32
}

// NewPersistentMemory creates a new persistent memory
func NewPersistentMemory(storage Storage, maxShortTerm int) *PersistentMemory {
	return &PersistentMemory{
		shortTerm:  NewShortTerm(maxShortTerm),
		longTerm:   make([]Item, 0),
		storage:    storage,
		embeddings: make(map[string][]float32),
	}
}

// ShortTerm returns the short-term memory
func (m *PersistentMemory) ShortTerm() *ShortTerm {
	return m.shortTerm
}

// Remember stores a key-value pair in long-term memory
func (m *PersistentMemory) Remember(key string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	item := Item{
		ID:        generateID(),
		Type:      "long_term",
		Content:   string(content),
		Metadata:  map[string]any{"key": key},
		Timestamp: time.Now(),
	}

	m.longTerm = append(m.longTerm, item)
	return nil
}

// Recall retrieves a value from long-term memory by key
func (m *PersistentMemory) Recall(key string) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := len(m.longTerm) - 1; i >= 0; i-- {
		item := m.longTerm[i]
		if k, ok := item.Metadata["key"].(string); ok && k == key {
			var value any
			if err := json.Unmarshal([]byte(item.Content), &value); err != nil {
				return nil, fmt.Errorf("failed to unmarshal value: %w", err)
			}
			return value, nil
		}
	}

	return nil, fmt.Errorf("key not found: %s", key)
}

// AddToLongTerm adds an item directly to long-term memory
func (m *PersistentMemory) AddToLongTerm(item Item) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.longTerm = append(m.longTerm, item)
}

// GetLongTerm returns all long-term memory items
func (m *PersistentMemory) GetLongTerm() []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Item, len(m.longTerm))
	copy(result, m.longTerm)
	return result
}

// GetAllLongTermByKeyPrefix returns long-term memory items with keys matching a prefix
func (m *PersistentMemory) GetAllLongTermByKeyPrefix(prefix string) []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Item
	for _, item := range m.longTerm {
		if k, ok := item.Metadata["key"].(string); ok && strings.HasPrefix(k, prefix) {
			result = append(result, item)
		}
	}
	return result
}

// RememberPreferences stores user preferences in long-term memory
func (m *PersistentMemory) RememberPreferences(prefs *UserPreferences) error {
	if prefs == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	content, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}

	// Remove old preferences
	var newLongTerm []Item
	for _, item := range m.longTerm {
		if k, ok := item.Metadata["key"].(string); ok && k == "user_preferences" {
			continue
		}
		newLongTerm = append(newLongTerm, item)
	}

	// Add new preferences
	item := Item{
		ID:        generateID(),
		Type:      "preferences",
		Content:   string(content),
		Metadata:  map[string]any{"key": "user_preferences"},
		Timestamp: time.Now(),
	}

	m.longTerm = append(newLongTerm, item)
	return nil
}

// RecallPreferences retrieves user preferences from long-term memory
func (m *PersistentMemory) RecallPreferences() (*UserPreferences, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Search from newest to oldest
	for i := len(m.longTerm) - 1; i >= 0; i-- {
		item := m.longTerm[i]
		if k, ok := item.Metadata["key"].(string); ok && k == "user_preferences" {
			var prefs UserPreferences
			if err := json.Unmarshal([]byte(item.Content), &prefs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal preferences: %w", err)
			}
			return &prefs, nil
		}
	}

	return nil, nil // No preferences found
}

// RememberDestination stores a destination in long-term memory
func (m *PersistentMemory) RememberDestination(destination string) error {
	if destination == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	for _, item := range m.longTerm {
		if k, ok := item.Metadata["key"].(string); ok && k == "destination:"+destination {
			return nil // Already stored
		}
	}

	item := Item{
		ID:        generateID(),
		Type:      "destination",
		Content:   destination,
		Metadata:  map[string]any{"key": "destination:" + destination},
		Timestamp: time.Now(),
	}

	m.longTerm = append(m.longTerm, item)
	return nil
}

// GetVisitedDestinations returns all visited destinations
func (m *PersistentMemory) GetVisitedDestinations() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var destinations []string
	for _, item := range m.longTerm {
		if item.Type == "destination" {
			destinations = append(destinations, item.Content)
		}
	}
	return destinations
}

// SetEmbedding sets the embedding vector for an item
func (m *PersistentMemory) SetEmbedding(itemID string, embedding []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embeddings[itemID] = embedding
}

// GetEmbedding gets the embedding vector for an item
func (m *PersistentMemory) GetEmbedding(itemID string) ([]float32, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	emb, ok := m.embeddings[itemID]
	return emb, ok
}

// Save persists the memory state to storage
func (m *PersistentMemory) Save(ctx context.Context, sessionID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &Snapshot{
		SessionID:  sessionID,
		ShortTerm:  m.shortTerm.GetAll(),
		LongTerm:   m.longTerm,
		Embeddings: m.embeddings,
		UpdatedAt:  time.Now(),
	}

	if m.storage != nil {
		return m.storage.Save(ctx, sessionID, snapshot)
	}
	return nil
}

// Load restores the memory state from storage
func (m *PersistentMemory) Load(ctx context.Context, sessionID string) error {
	if m.storage == nil {
		return nil
	}

	snapshot, err := m.storage.Load(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load memory: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.shortTerm.Clear()
	for _, item := range snapshot.ShortTerm {
		m.shortTerm.Add(item.Type, item.Content, item.Metadata)
	}

	m.longTerm = snapshot.LongTerm
	m.embeddings = snapshot.Embeddings
	if m.embeddings == nil {
		m.embeddings = make(map[string][]float32)
	}

	return nil
}

// ToSnapshot creates a snapshot of the current memory state
func (m *PersistentMemory) ToSnapshot(sessionID string) *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	embeddings := make(map[string][]float32, len(m.embeddings))
	for k, v := range m.embeddings {
		embeddings[k] = v
	}

	return &Snapshot{
		SessionID:  sessionID,
		ShortTerm:  m.shortTerm.GetAll(),
		LongTerm:   m.longTerm,
		Embeddings: embeddings,
		UpdatedAt:  time.Now(),
	}
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
