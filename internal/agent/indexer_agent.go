// Package agent provides the core agent implementation
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
)

// IndexerAgent is a specialized agent for building vector indices
// It reads curated documents and builds vector indices for RAG
type IndexerAgent struct {
	mu           sync.RWMutex
	id           string
	agentType    AgentType
	state        AgentState
	llmProvider  llm.Provider
	systemPrompt string
	maxRounds    int
	currentRound int

	// Shared state reference
	sharedState *SharedKnowledgeState

	// Tool registry for indexing
	toolRegistry ToolRegistry

	// Indexing results
	collectionID   string
	totalChunks    int
	embeddingDim   int
	indexingTimeMs int64
}

// IndexerResult represents the indexing result
type IndexerResult struct {
	CollectionID     string `json:"collection_id"`
	TotalChunks      int    `json:"total_chunks"`
	EmbeddingDim     int    `json:"embedding_dimension"`
	IndexingTimeMs   int64  `json:"indexing_time_ms"`
	IsComplete       bool   `json:"is_complete"`
	DocumentsIndexed int    `json:"documents_indexed"`
}

// IndexerAgentConfig for creating an indexer agent
type IndexerAgentConfig struct {
	ID           string
	LLMProvider  llm.Provider
	SharedState  *SharedKnowledgeState
	ToolRegistry ToolRegistry
	MaxRounds    int
}

// NewIndexerAgent creates a new indexer agent
func NewIndexerAgent(config IndexerAgentConfig) *IndexerAgent {
	if config.MaxRounds == 0 {
		config.MaxRounds = 2
	}

	return &IndexerAgent{
		id:                config.ID,
		agentType:         AgentTypeIndexer,
		state:             StateIdle,
		llmProvider:       config.LLMProvider,
		systemPrompt:      GetSubagentPrompt(AgentTypeIndexer),
		maxRounds:         config.MaxRounds,
		currentRound:      0,
		sharedState:       config.SharedState,
		toolRegistry:      config.ToolRegistry,
		collectionID:      "",
		totalChunks:       0,
		embeddingDim:      768,
		indexingTimeMs:    0,
	}
}

// ID returns the agent ID
func (a *IndexerAgent) ID() string {
	return a.id
}

// Type returns the agent type
func (a *IndexerAgent) Type() AgentType {
	return a.agentType
}

// State returns the current state
func (a *IndexerAgent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState sets the agent state
func (a *IndexerAgent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// Run starts the indexer agent
func (a *IndexerAgent) Run(ctx context.Context, collectionID string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	a.collectionID = collectionID

	var totalTokensIn, totalTokensOut int

	for a.currentRound < a.maxRounds {
		a.currentRound++
		a.SetState(StateRunning)

		// 1. Read: Get high-quality documents from shared state
		documents := a.sharedState.GetDocuments()
		if len(documents) == 0 {
			a.SetState(StateError)
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      "build vector index",
				Success:   false,
				Error:     "no documents to index",
				Duration:  time.Since(startTime),
			}, fmt.Errorf("no documents to index")
		}

		// 2. Prepare documents for indexing
		indexDocs := make([]any, 0, len(documents))
		for _, doc := range documents {
			// Filter out low quality documents
			if doc.Quality < 0.5 {
				continue
			}
			indexDocs = append(indexDocs, map[string]any{
				"title":   doc.Title,
				"content": doc.Content,
				"url":     doc.URL,
				"source":  doc.Source,
				"topics":  doc.Topics,
			})
		}

		if len(indexDocs) == 0 {
			a.SetState(StateError)
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      "build vector index",
				Success:   false,
				Error:     "no high-quality documents to index",
				Duration:  time.Since(startTime),
			}, fmt.Errorf("no high-quality documents to index")
		}

		// 3. Build index using tool registry
		if a.toolRegistry == nil {
			a.SetState(StateError)
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      "build vector index",
				Success:   false,
				Error:     "tool registry not available",
				Duration:  time.Since(startTime),
			}, fmt.Errorf("tool registry not available")
		}

		result, err := a.toolRegistry.Execute(ctx, "build_knowledge_index", map[string]any{
			"documents":     indexDocs,
			"collection_id": a.collectionID,
		})
		if err != nil {
			a.SetState(StateError)
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      "build vector index",
				Success:   false,
				Error:     fmt.Sprintf("Indexing failed: %v", err),
				Duration:  time.Since(startTime),
			}, err
		}

		// 4. Extract results
		if result.Success && result.Data != nil {
			if data, ok := result.Data.(map[string]any); ok {
				if indexed, ok := data["indexed_count"].(int); ok {
					a.totalChunks = indexed
				}
				if dim, ok := data["embedding_dim"].(int); ok {
					a.embeddingDim = dim
				}
			}
		}

		a.indexingTimeMs = time.Since(startTime).Milliseconds()

		// Success - break out of loop
		break
	}

	// Mark as complete
	a.SetState(StateCompleted)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      "build vector index",
		Success:   true,
		Output: map[string]any{
			"collection_id":       a.collectionID,
			"total_chunks":        a.totalChunks,
			"embedding_dimension": a.embeddingDim,
			"indexing_time_ms":    a.indexingTimeMs,
			"documents_indexed":   len(a.sharedState.GetDocuments()),
			"is_complete":         true,
		},
		Duration: time.Since(startTime),
		Metadata: map[string]any{
			"tokens_in":  totalTokensIn,
			"tokens_out": totalTokensOut,
		},
	}, nil
}

// GetIndexStats returns the indexing statistics
func (a *IndexerAgent) GetIndexStats() *IndexerResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return &IndexerResult{
		CollectionID:     a.collectionID,
		TotalChunks:      a.totalChunks,
		EmbeddingDim:     a.embeddingDim,
		IndexingTimeMs:   a.indexingTimeMs,
		IsComplete:       a.state == StateCompleted,
		DocumentsIndexed: len(a.sharedState.GetDocuments()),
	}
}