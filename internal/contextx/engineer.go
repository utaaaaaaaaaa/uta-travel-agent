// Package contextx provides context engineering for agents
package contextx

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/memory"
)

// Priority levels for context items
type Priority int

const (
	PriorityCritical Priority = 100 // Current query, system prompt
	PriorityHigh     Priority = 75  // Recent messages
	PriorityMedium   Priority = 50  // Summaries, important context
	PriorityLow      Priority = 25  // Old messages
)

// EngineerConfig holds configuration for ContextEngineer
type EngineerConfig struct {
	MaxTokens        int
	LLMProvider      llm.Provider
	CompressionModel string // Model to use for compression
	UseLongTerm      bool   `json:"use_long_term"`      // Enable long-term memory context
	UsePreferences   bool   `json:"use_preferences"`    // Enable user preferences context
	UseGSSC          bool   `json:"use_gssc"`           // Enable GSSC pipeline
}

// Engineer manages context window for agents
type Engineer struct {
	maxTokens   int
	llmProvider llm.Provider
	useLongTerm bool
	usePrefs    bool
	useGSSC     bool
	gsscConfig  ContextConfig
	ragService  RAGService

	mu         sync.RWMutex
	priorities map[string]Priority
	compressed map[string]string // Cache of compressed content
}

// NewEngineer creates a new context engineer
func NewEngineer(config EngineerConfig) *Engineer {
	if config.MaxTokens <= 0 {
		config.MaxTokens = 8000
	}

	gsscConfig := DefaultContextConfig()
	gsscConfig.MaxTokens = config.MaxTokens
	if config.LLMProvider != nil {
		gsscConfig.EnableCompression = true
	}

	return &Engineer{
		maxTokens:   config.MaxTokens,
		llmProvider: config.LLMProvider,
		useLongTerm: config.UseLongTerm,
		usePrefs:    config.UsePreferences,
		useGSSC:     config.UseGSSC,
		gsscConfig:  gsscConfig,
		priorities:  make(map[string]Priority),
		compressed:  make(map[string]string),
	}
}

// SetRAGService sets the RAG service for GSSC pipeline
func (e *Engineer) SetRAGService(service RAGService) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ragService = service
}

// SetGSSCConfig sets the GSSC configuration
func (e *Engineer) SetGSSCConfig(config ContextConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.gsscConfig = config
}

// SetMaxTokens sets the maximum context window size
func (e *Engineer) SetMaxTokens(max int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.maxTokens = max
}

// GetMaxTokens returns the current max tokens
func (e *Engineer) GetMaxTokens() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.maxTokens
}

// SetPriority sets the priority for a specific item type
func (e *Engineer) SetPriority(itemType string, priority Priority) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.priorities[itemType] = priority
}

// GetPriority returns the priority for an item type
func (e *Engineer) getPriority(item memory.Item) Priority {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check explicit priority
	if p, ok := e.priorities[item.Type]; ok {
		return p
	}

	// Default priorities by type
	switch item.Type {
	case "message":
		return PriorityHigh
	case "thought":
		return PriorityMedium
	case "observation":
		return PriorityMedium
	case "action":
		return PriorityLow
	case "result":
		return PriorityLow
	default:
		return PriorityMedium
	}
}

// EstimateTokens estimates the number of tokens in content
// Rough estimation: ~4 chars per token for Chinese, ~0.75 words per token for English
func EstimateTokens(content string) int {
	// Count Chinese characters
	chineseCount := 0
	englishWords := 0
	inWord := false

	for _, r := range content {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
			if inWord {
				englishWords++
				inWord = false
			}
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			inWord = true
		} else {
			if inWord {
				englishWords++
				inWord = false
			}
		}
	}
	if inWord {
		englishWords++
	}

	// Chinese: ~1.5 tokens per character
	// English: ~1.3 tokens per word
	return int(float64(chineseCount)*1.5 + float64(englishWords)*1.3)
}

// prioritizedItem wraps a memory item with priority for sorting
type prioritizedItem struct {
	item     memory.Item
	priority Priority
	index    int // Original index for stable sort
}

// BuildContext builds an optimized context for LLM from memory
func (e *Engineer) BuildContext(mem *memory.PersistentMemory, maxTokens int) []llm.Message {
	if maxTokens <= 0 {
		maxTokens = e.GetMaxTokens()
	}

	items := mem.ShortTerm().GetAll()

	// Wrap items with priorities
	prioritized := make([]prioritizedItem, len(items))
	for i, item := range items {
		prioritized[i] = prioritizedItem{
			item:     item,
			priority: e.getPriority(item),
			index:    i,
		}
	}

	// Sort by priority (descending) and then by recency (later items first within same priority)
	sort.Slice(prioritized, func(i, j int) bool {
		if prioritized[i].priority != prioritized[j].priority {
			return prioritized[i].priority > prioritized[j].priority
		}
		return prioritized[i].index > prioritized[j].index
	})

	// Build messages until token limit
	var messages []llm.Message
	currentTokens := 0

	for _, pi := range prioritized {
		item := pi.item

		// Only include messages in context
		if item.Type != "message" {
			continue
		}

		tokens := EstimateTokens(item.Content)

		if currentTokens+tokens > maxTokens {
			// Try to fit a compressed summary instead
			remaining := maxTokens - currentTokens
			if remaining > 100 {
				summary := e.getCompressedSummary(item, remaining)
				if EstimateTokens(summary) <= remaining {
					messages = append(messages, llm.Message{
						Role:    "system",
						Content: "[Summary] " + summary,
					})
					break
				}
			}
			break
		}

		role := "user"
		if r, ok := item.Metadata["role"].(string); ok {
			role = r
		}

		messages = append(messages, llm.Message{
			Role:    role,
			Content: item.Content,
		})
		currentTokens += tokens
	}

	return messages
}

// BuildContextWithSystem builds context including system message and user preferences
func (e *Engineer) BuildContextWithSystem(mem *memory.PersistentMemory, systemPrompt string, maxTokens int) []llm.Message {
	// Reserve tokens for system prompt
	systemTokens := EstimateTokens(systemPrompt)

	// Load user preferences if enabled
	var prefsContext string
	var prefsTokens int
	if e.usePrefs {
		prefs, _ := mem.RecallPreferences()
		if prefs != nil {
			prefsContext = prefs.FormatAsContext()
			prefsTokens = EstimateTokens(prefsContext)
		}
	}

	// Calculate remaining tokens for conversation history
	remainingTokens := maxTokens - systemTokens - prefsTokens - 200 // Buffer for response

	messages := e.BuildContext(mem, remainingTokens)

	// Build final messages array
	result := make([]llm.Message, 0, len(messages)+3)

	// Add system prompt
	result = append(result, llm.Message{Role: "system", Content: systemPrompt})

	// Add user preferences as context if available
	if prefsContext != "" {
		result = append(result, llm.Message{
			Role:    "system",
			Content: prefsContext,
		})
	}

	// Add conversation history
	result = append(result, messages...)

	return result
}

// BuildContextWithSystemAndPrefs builds context with explicit preferences
func (e *Engineer) BuildContextWithSystemAndPrefs(mem *memory.PersistentMemory, systemPrompt string, prefs *memory.UserPreferences, maxTokens int) []llm.Message {
	// Reserve tokens for system prompt
	systemTokens := EstimateTokens(systemPrompt)

	// Use provided preferences
	var prefsContext string
	var prefsTokens int
	if prefs != nil {
		prefsContext = prefs.FormatAsContext()
		prefsTokens = EstimateTokens(prefsContext)
	}

	// Calculate remaining tokens for conversation history
	remainingTokens := maxTokens - systemTokens - prefsTokens - 200 // Buffer for response

	messages := e.BuildContext(mem, remainingTokens)

	// Build final messages array
	result := make([]llm.Message, 0, len(messages)+3)

	// Add system prompt
	result = append(result, llm.Message{Role: "system", Content: systemPrompt})

	// Add user preferences as context if available
	if prefsContext != "" {
		result = append(result, llm.Message{
			Role:    "system",
			Content: prefsContext,
		})
	}

	// Add conversation history
	result = append(result, messages...)

	return result
}

// getCompressedSummary gets or creates a compressed summary for an item
func (e *Engineer) getCompressedSummary(item memory.Item, maxTokens int) string {
	e.mu.RLock()
	if summary, ok := e.compressed[item.ID]; ok {
		e.mu.RUnlock()
		return summary
	}
	e.mu.RUnlock()

	// Simple truncation for now (LLM compression can be added later)
	targetLen := maxTokens * 3 // Approximate chars per token
	if len(item.Content) > targetLen {
		return item.Content[:targetLen] + "..."
	}
	return item.Content
}

// Compress uses LLM to compress a list of memory items into a summary
func (e *Engineer) Compress(ctx context.Context, items []memory.Item) (string, error) {
	if e.llmProvider == nil {
		return e.simpleCompress(items), nil
	}

	var content strings.Builder
	for _, item := range items {
		role := "user"
		if r, ok := item.Metadata["role"].(string); ok {
			role = r
		}
		content.WriteString(fmt.Sprintf("%s: %s\n", role, item.Content))
	}

	prompt := fmt.Sprintf("Summarize the following conversation in 2-3 sentences, preserving key information:\n\n%s", content.String())

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	response, err := e.llmProvider.Complete(ctx, messages)
	if err != nil {
		return e.simpleCompress(items), nil
	}

	// Cache the compression
	if len(items) > 0 {
		key := items[0].ID + "_compressed"
		e.mu.Lock()
		e.compressed[key] = response.Content
		e.mu.Unlock()
	}

	return response.Content, nil
}

// simpleCompress provides a fallback compression without LLM
func (e *Engineer) simpleCompress(items []memory.Item) string {
	if len(items) == 0 {
		return ""
	}

	var summary strings.Builder
	summary.WriteString("[Summary of earlier conversation]: ")

	for i, item := range items {
		if i > 0 {
			summary.WriteString("; ")
		}

		role := "User"
		if r, ok := item.Metadata["role"].(string); ok {
			if r == "assistant" {
				role = "Assistant"
			}
		}

		// Truncate long content
		content := item.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}

		summary.WriteString(fmt.Sprintf("%s asked about %s", role, content))
	}

	return summary.String()
}

// ClearCache clears the compression cache
func (e *Engineer) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.compressed = make(map[string]string)
}

// Stats returns statistics about the context engineer
func (e *Engineer) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]any{
		"max_tokens":          e.maxTokens,
		"cached_compressions": len(e.compressed),
		"priority_rules":      len(e.priorities),
		"use_gssc":            e.useGSSC,
		"use_long_term":       e.useLongTerm,
		"use_preferences":     e.usePrefs,
	}
}

// BuildContextWithGSSC builds context using the GSSC pipeline
// This method uses the advanced Gather-Select-Structure-Compress pipeline
func (e *Engineer) BuildContextWithGSSC(mem *memory.PersistentMemory, query string, maxTokens int) []llm.Message {
	if maxTokens <= 0 {
		maxTokens = e.GetMaxTokens()
	}

	// Create GSSC pipeline
	pipeline := NewGSSCPipeline(e.gsscConfig, mem, e.ragService, e.llmProvider)

	// Build context using GSSC
	return pipeline.BuildContextWithGSSC(mem, query, maxTokens)
}

// BuildContextOptimized builds context with automatic GSSC selection
// If GSSC is enabled, uses GSSC pipeline; otherwise uses legacy method
func (e *Engineer) BuildContextOptimized(mem *memory.PersistentMemory, systemPrompt, query string, maxTokens int) []llm.Message {
	if e.useGSSC {
		// Use GSSC pipeline
		messages := e.BuildContextWithGSSC(mem, query, maxTokens)

		// Prepend system prompt
		if systemPrompt != "" {
			messagesWithSystem := make([]llm.Message, 0, len(messages)+1)
			messagesWithSystem = append(messagesWithSystem, llm.Message{
				Role:    "system",
				Content: systemPrompt,
			})
			messagesWithSystem = append(messagesWithSystem, messages...)
			return messagesWithSystem
		}
		return messages
	}

	// Use legacy method
	return e.BuildContextWithSystem(mem, systemPrompt, maxTokens)
}

// IsGSSCEnabled returns whether GSSC pipeline is enabled
func (e *Engineer) IsGSSCEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.useGSSC
}

// SetGSSCEnabled enables or disables GSSC pipeline
func (e *Engineer) SetGSSCEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.useGSSC = enabled
}
