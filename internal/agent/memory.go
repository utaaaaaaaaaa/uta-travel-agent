package agent

import (
	"fmt"
	"sync"
	"time"
)

// MemoryItem represents a single item in agent memory
type MemoryItem struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"` // thought, observation, action, result
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// AgentMemory manages the working memory of an agent
type AgentMemory struct {
	mu       sync.RWMutex
	items    []MemoryItem
	maxSize  int
	byType   map[string][]int // index by type
	byTag    map[string][]int // index by tag
}

// NewAgentMemory creates a new agent memory
func NewAgentMemory() *AgentMemory {
	return &AgentMemory{
		items:   make([]MemoryItem, 0),
		maxSize: 100,
		byType:  make(map[string][]int),
		byTag:   make(map[string][]int),
	}
}

// Add adds a new memory item
func (m *AgentMemory) Add(itemType, content string, metadata map[string]any) *MemoryItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := MemoryItem{
		ID:        generateID(),
		Type:      itemType,
		Content:   content,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	// Check size limit
	if len(m.items) >= m.maxSize {
		m.trim()
	}

	idx := len(m.items)
	m.items = append(m.items, item)

	// Update indexes
	m.byType[itemType] = append(m.byType[itemType], idx)

	// Index by tags if present
	if tags, ok := metadata["tags"].([]string); ok {
		for _, tag := range tags {
			m.byTag[tag] = append(m.byTag[tag], idx)
		}
	}

	return &item
}

// AddThought adds a thought to memory
func (m *AgentMemory) AddThought(thought string) *MemoryItem {
	return m.Add("thought", thought, nil)
}

// AddObservation adds an observation to memory
func (m *AgentMemory) AddObservation(observation string, source string) *MemoryItem {
	return m.Add("observation", observation, map[string]any{
		"source": source,
	})
}

// AddAction adds an action to memory
func (m *AgentMemory) AddAction(action string, params map[string]any) *MemoryItem {
	return m.Add("action", action, params)
}

// AddResult adds a result to memory
func (m *AgentMemory) AddResult(result string, success bool, metadata map[string]any) *MemoryItem {
	metadata["success"] = success
	return m.Add("result", result, metadata)
}

// ConversationMessage represents a message in conversation history
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AddMessage adds a conversation message to memory
func (m *AgentMemory) AddMessage(role, content string) *MemoryItem {
	return m.Add("message", content, map[string]any{
		"role": role,
	})
}

// GetConversationHistory returns the conversation history
func (m *AgentMemory) GetConversationHistory() []ConversationMessage {
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

// GetAll returns all memory items
func (m *AgentMemory) GetAll() []MemoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]MemoryItem, len(m.items))
	copy(result, m.items)
	return result
}

// GetByType returns memory items of a specific type
func (m *AgentMemory) GetByType(itemType string) []MemoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indices, ok := m.byType[itemType]
	if !ok {
		return nil
	}

	result := make([]MemoryItem, len(indices))
	for i, idx := range indices {
		result[i] = m.items[idx]
	}
	return result
}

// GetRecent returns the most recent n items
func (m *AgentMemory) GetRecent(n int) []MemoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n >= len(m.items) {
		return m.GetAll()
	}

	result := make([]MemoryItem, n)
	copy(result, m.items[len(m.items)-n:])
	return result
}

// GetContext builds a context string from memory for LLM
func (m *AgentMemory) GetContext(maxItems int) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := m.GetRecent(maxItems)
	var context string
	for _, item := range items {
		switch item.Type {
		case "thought":
			context += "思考: " + item.Content + "\n"
		case "observation":
			context += "观察: " + item.Content + "\n"
		case "action":
			context += "行动: " + item.Content + "\n"
		case "result":
			context += "结果: " + item.Content + "\n"
		}
	}
	return context
}

// Clear clears all memory
func (m *AgentMemory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make([]MemoryItem, 0)
	m.byType = make(map[string][]int)
	m.byTag = make(map[string][]int)
}

// trim removes old items when memory is full
func (m *AgentMemory) trim() {
	if len(m.items) <= m.maxSize/2 {
		return
	}

	// Keep the more recent half
	keep := m.maxSize / 2
	m.items = m.items[len(m.items)-keep:]

	// Rebuild indexes
	m.byType = make(map[string][]int)
	m.byTag = make(map[string][]int)
	for i, item := range m.items {
		m.byType[item.Type] = append(m.byType[item.Type], i)
		if tags, ok := item.Metadata["tags"].([]string); ok {
			for _, tag := range tags {
				m.byTag[tag] = append(m.byTag[tag], i)
			}
		}
	}
}

// Size returns the number of items in memory
func (m *AgentMemory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
