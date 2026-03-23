// Package memory provides semantic memory with hybrid retrieval
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// EmbeddingService interface for vector embeddings
type EmbeddingService interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// VectorStore interface for vector storage
type VectorStore interface {
	Search(ctx context.Context, vector []float32, limit int) ([]VectorResult, error)
	Add(ctx context.Context, id string, vector []float32, metadata map[string]any) error
	Delete(ctx context.Context, id string) error
}

// VectorResult represents a vector search result
type VectorResult struct {
	ID       string         `json:"id"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata"`
	Content  string         `json:"content"`
}

// GraphStore interface for graph storage
type GraphStore interface {
	CreateEntity(ctx context.Context, entity *GraphEntity) error
	GetEntity(ctx context.Context, id string) (*GraphEntity, error)
	CreateRelation(ctx context.Context, relation *GraphRelation) error
	FindRelated(ctx context.Context, entityIDs []string, maxDepth int, limit int) ([]GraphSearchResult, error)
	DeleteEntity(ctx context.Context, id string) error
	SearchByName(ctx context.Context, name string, entityType string, limit int) ([]GraphEntity, error)
}

// GraphEntity represents an entity in the knowledge graph
type GraphEntity struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// GraphRelation represents a relationship in the knowledge graph
type GraphRelation struct {
	FromID     string         `json:"from_id"`
	ToID       string         `json:"to_id"`
	Type       string         `json:"type"`
	Weight     float64        `json:"weight"`
	Properties map[string]any `json:"properties"`
}

// GraphSearchResult represents a graph search result
type GraphSearchResult struct {
	Entity     GraphEntity `json:"entity"`
	Score      float64     `json:"score"`
	PathLength int         `json:"path_length"`
	RelatedIDs []string    `json:"related_ids"`
}

// MemoryItem represents a semantic memory item
type MemoryItem struct {
	ID         string         `json:"id"`
	Content    string         `json:"content"`
	Vector     []float32      `json:"vector,omitempty"`
	Entities   []ExtractedEntity `json:"entities"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ExtractedEntity represents an entity extracted from content
type ExtractedEntity struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Confidence float64  `json:"confidence"`
	Positions  []int    `json:"positions"`
}

// SemanticMemory combines vector and graph storage for hybrid retrieval
type SemanticMemory struct {
	vectorStore VectorStore
	graphStore  GraphStore
	embedder    EmbeddingService

	mu       sync.RWMutex
	items    map[string]*MemoryItem
	maxSize  int
}

// SemanticMemoryConfig holds configuration for semantic memory
type SemanticMemoryConfig struct {
	VectorStore VectorStore
	GraphStore  GraphStore
	Embedder    EmbeddingService
	MaxSize     int
}

// NewSemanticMemory creates a new semantic memory
func NewSemanticMemory(config SemanticMemoryConfig) *SemanticMemory {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	return &SemanticMemory{
		vectorStore: config.VectorStore,
		graphStore:  config.GraphStore,
		embedder:    config.Embedder,
		items:       make(map[string]*MemoryItem),
		maxSize:     config.MaxSize,
	}
}

// Add adds content to semantic memory with automatic entity extraction
func (m *SemanticMemory) Add(ctx context.Context, content string, metadata map[string]any) (*MemoryItem, error) {
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// Generate ID
	id := fmt.Sprintf("mem-%d", time.Now().UnixNano())

	// Extract entities
	entities := m.extractEntities(content)

	// Create memory item
	item := &MemoryItem{
		ID:        id,
		Content:   content,
		Entities:  entities,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	// Generate embedding if embedder is available
	if m.embedder != nil {
		vectors, err := m.embedder.Embed(ctx, []string{content})
		if err == nil && len(vectors) > 0 {
			item.Vector = vectors[0]

			// Store in vector store
			if m.vectorStore != nil {
				vecMetadata := make(map[string]any)
				for k, v := range metadata {
					vecMetadata[k] = v
				}
				vecMetadata["content"] = content
				vecMetadata["entities"] = entities

				if err := m.vectorStore.Add(ctx, id, item.Vector, vecMetadata); err != nil {
					log.Printf("[SemanticMemory] Failed to add to vector store: %v", err)
				}
			}
		}
	}

	// Create entities in graph store
	if m.graphStore != nil {
		for _, entity := range entities {
			graphEntity := &GraphEntity{
				ID:   fmt.Sprintf("entity-%s-%d", strings.ToLower(strings.ReplaceAll(entity.Name, " ", "-")), time.Now().UnixNano()),
				Name: entity.Name,
				Type: entity.Type,
				Properties: map[string]any{
					"source_id":  id,
					"confidence": entity.Confidence,
				},
			}

			if err := m.graphStore.CreateEntity(ctx, graphEntity); err != nil {
				log.Printf("[SemanticMemory] Failed to create entity %s: %v", entity.Name, err)
			}
		}
	}

	// Store in local cache
	m.mu.Lock()
	m.items[id] = item

	// Trim if over capacity
	if len(m.items) > m.maxSize {
		m.trim()
	}
	m.mu.Unlock()

	return item, nil
}

// Retrieve performs hybrid retrieval combining vector and graph search
func (m *SemanticMemory) Retrieve(ctx context.Context, query string, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []MemoryItem
	resultIDs := make(map[string]bool)

	// 1. Vector search (if available)
	if m.embedder != nil && m.vectorStore != nil {
		vectors, err := m.embedder.Embed(ctx, []string{query})
		if err == nil && len(vectors) > 0 {
			vectorResults, err := m.vectorStore.Search(ctx, vectors[0], limit*2)
			if err == nil {
				for _, vr := range vectorResults {
					if !resultIDs[vr.ID] {
						resultIDs[vr.ID] = true
						results = append(results, MemoryItem{
							ID:       vr.ID,
							Content:  vr.Content,
							Metadata: vr.Metadata,
						})
					}
				}
			}
		}
	}

	// 2. Graph search (if available)
	if m.graphStore != nil {
		// Extract entities from query
		queryEntities := m.extractEntities(query)

		var entityIDs []string
		for _, e := range queryEntities {
			entityIDs = append(entityIDs, e.Name)
		}

		if len(entityIDs) > 0 {
			graphResults, err := m.graphStore.FindRelated(ctx, entityIDs, 2, limit*2)
			if err == nil {
				for _, gr := range graphResults {
					// Add related content
					// For now, just mark the entity as related
					log.Printf("[SemanticMemory] Found related entity: %s (score: %.2f)", gr.Entity.Name, gr.Score)
				}
			}
		}
	}

	// 3. Fallback to local cache search
	m.mu.RLock()
	for _, item := range m.items {
		if len(results) >= limit {
			break
		}

		if !resultIDs[item.ID] {
			// Simple text matching
			if strings.Contains(strings.ToLower(item.Content), strings.ToLower(query)) {
				results = append(results, *item)
				resultIDs[item.ID] = true
			}
		}
	}
	m.mu.RUnlock()

	return results[:min(len(results), limit)], nil
}

// Get retrieves a memory item by ID
func (m *SemanticMemory) Get(id string) (*MemoryItem, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.items[id]
	return item, ok
}

// Delete removes a memory item
func (m *SemanticMemory) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from vector store
	if m.vectorStore != nil {
		if err := m.vectorStore.Delete(ctx, id); err != nil {
			log.Printf("[SemanticMemory] Failed to delete from vector store: %v", err)
		}
	}

	// Remove from local cache
	delete(m.items, id)

	return nil
}

// extractEntities extracts entities from content using pattern matching
func (m *SemanticMemory) extractEntities(content string) []ExtractedEntity {
	var entities []ExtractedEntity
	seen := make(map[string]bool)

	// Chinese destination patterns - using \x{4e00}-\x{9fff} for Chinese characters
	destinationPatterns := []*regexp.Regexp{
		regexp.MustCompile(`([\x{4e00}-\x{9fff}]{2,}(?:市|省|县|区|镇|村))`),
		regexp.MustCompile(`([\x{4e00}-\x{9fff}]{2,}(?:景点|公园|寺|庙|塔|楼|阁|院|宫|殿|陵|墓|桥|门|城墙))`),
		regexp.MustCompile(`([\x{4e00}-\x{9fff}]{2,}(?:博物馆|纪念馆|展览馆|美术馆))`),
		regexp.MustCompile(`([\x{4e00}-\x{9fff}]{2,}(?:山|河|湖|海|江|溪|瀑布|峡谷))`),
	}

	// English patterns
	englishPatterns := []*regexp.Regexp{
		regexp.MustCompile(`([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)`),
	}

	entityTypes := []string{"destination", "attraction", "landmark"}

	for i, pattern := range destinationPatterns {
		matches := pattern.FindAllStringSubmatchIndex(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				name := content[match[2]:match[3]]
				if !seen[name] && len(name) > 1 {
					seen[name] = true
					entityType := entityTypes[min(i, len(entityTypes)-1)]
					entities = append(entities, ExtractedEntity{
						Name:       name,
						Type:       entityType,
						Confidence: 0.8,
						Positions:  []int{match[2], match[3]},
					})
				}
			}
		}
	}

	// Extract English names (lower confidence)
	for _, pattern := range englishPatterns {
		matches := pattern.FindAllStringSubmatchIndex(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				name := content[match[2]:match[3]]
				if !seen[name] && len(name) > 3 {
					seen[name] = true
					entities = append(entities, ExtractedEntity{
						Name:       name,
						Type:       "location",
						Confidence: 0.6,
						Positions:  []int{match[2], match[3]},
					})
				}
			}
		}
	}

	// Extract food-related entities
	foodPattern := regexp.MustCompile(`([\x{4e00}-\x{9fff}]{2,}(?:面|饭|粥|汤|菜|饼|糕|茶|酒|肉|鱼|虾|蟹|豆腐))`)
	matches := foodPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			name := content[match[2]:match[3]]
			if !seen[name] {
				seen[name] = true
				entities = append(entities, ExtractedEntity{
					Name:       name,
					Type:       "food",
					Confidence: 0.7,
					Positions:  []int{match[2], match[3]},
				})
			}
		}
	}

	return entities
}

// trim removes old items when memory is full
func (m *SemanticMemory) trim() {
	// Simple FIFO trim - remove oldest items
	if len(m.items) <= m.maxSize {
		return
	}

	// Find and remove oldest items
	toRemove := len(m.items) - m.maxSize
	count := 0

	for id := range m.items {
		if count >= toRemove {
			break
		}
		delete(m.items, id)
		count++
	}
}

// Stats returns memory statistics
func (m *SemanticMemory) Stats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entityCount := 0
	entityTypes := make(map[string]int)

	for _, item := range m.items {
		for _, e := range item.Entities {
			entityCount++
			entityTypes[e.Type]++
		}
	}

	return map[string]any{
		"total_items":    len(m.items),
		"total_entities": entityCount,
		"entity_types":   entityTypes,
		"max_size":       m.maxSize,
	}
}

// ToJSON serializes the memory item
func (item *MemoryItem) ToJSON() string {
	data, _ := json.Marshal(item)
	return string(data)
}

// FromJSON deserializes a memory item
func MemoryItemFromJSON(data string) (*MemoryItem, error) {
	var item MemoryItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}