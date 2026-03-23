// Package contextx provides context engineering for agents
package contextx

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/memory"
	"github.com/utaaa/uta-travel-agent/internal/tools"
)

// RAGService interface for RAG operations
type RAGService interface {
	Query(ctx context.Context, query string, limit int) ([]RAGResult, error)
}

// RAGResult represents a RAG query result
type RAGResult struct {
	Content string
	Source  string
	Score   float64
}

// ShortTermMemoryItem represents an item from short-term memory
type ShortTermMemoryItem struct {
	ID        string
	Type      string
	Content   string
	Metadata  map[string]any
	Timestamp time.Time
}

// NoteService interface for note operations
type NoteService interface {
	Search(ctx context.Context, query string, noteType string, tags []string, limit int) ([]*tools.Note, error)
}

// GSSCPipeline implements the Gather-Select-Structure-Compress pipeline
type GSSCPipeline struct {
	config      ContextConfig
	memory      *memory.PersistentMemory
	ragService  RAGService
	llmProvider llm.Provider
	noteService NoteService

	mu       sync.RWMutex
	cache    map[string][]*ContextPacket
}

// NewGSSCPipeline creates a new GSSC pipeline
func NewGSSCPipeline(config ContextConfig, mem *memory.PersistentMemory, ragService RAGService, llmProvider llm.Provider) *GSSCPipeline {
	if config.MaxTokens <= 0 {
		config = DefaultContextConfig()
	}

	return &GSSCPipeline{
		config:      config,
		memory:      mem,
		ragService:  ragService,
		llmProvider: llmProvider,
		cache:       make(map[string][]*ContextPacket),
	}
}

// SetNoteService sets the note service for the pipeline
func (p *GSSCPipeline) SetNoteService(ns NoteService) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.noteService = ns
}

// Gather collects context packets from multiple sources
func (p *GSSCPipeline) Gather(query string, mem *memory.PersistentMemory) []*ContextPacket {
	var packets []*ContextPacket
	now := time.Now()

	// 1. System instructions (highest priority)
	// Note: System prompt is added separately in Structure phase
	// This packet represents system-level context like current time, session state

	// 2. Long-term memory / User preferences
	if mem != nil {
		prefs, _ := mem.RecallPreferences()
		if prefs != nil && !prefs.IsEmpty() {
			prefsContext := prefs.FormatAsContext()
			if prefsContext != "" {
				packets = append(packets, NewContextPacket(
					prefsContext,
					PacketTypeLongTerm,
					WithRelevance(0.9),
					WithSource("preferences"),
				))
			}
		}

		// 3. Visited destinations (for context)
		destinations := mem.GetVisitedDestinations()
		if len(destinations) > 0 {
			destContext := fmt.Sprintf("[已访问目的地]\n%s", strings.Join(destinations, ", "))
			packets = append(packets, NewContextPacket(
				destContext,
				PacketTypeLongTerm,
				WithRelevance(0.7),
				WithSource("visited_destinations"),
			))
		}
	}

	// 4. RAG knowledge retrieval (if available)
	if p.ragService != nil && query != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		results, err := p.ragService.Query(ctx, query, 5)
		if err == nil && len(results) > 0 {
			var ragContent strings.Builder
			ragContent.WriteString("[知识库检索结果]\n")
			for i, r := range results {
				if i > 0 {
					ragContent.WriteString("\n---\n")
				}
				ragContent.WriteString(r.Content)
			}
			packets = append(packets, NewContextPacket(
				ragContent.String(),
				PacketTypeRAG,
				WithRelevance(results[0].Score),
				WithSource("rag"),
			))
		}
	}

	// 4.5 Notes from NoteTool (for long-horizon task context)
	if p.noteService != nil && query != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		notes, err := p.noteService.Search(ctx, query, "conclusion", nil, 3)
		if err == nil && len(notes) > 0 {
			for _, note := range notes {
				var noteContent strings.Builder
				noteContent.WriteString(fmt.Sprintf("[笔记: %s]\n", note.Title))
				if len(note.Content) > 500 {
					noteContent.WriteString(note.Content[:500] + "...")
				} else {
					noteContent.WriteString(note.Content)
				}
				packets = append(packets, NewContextPacket(
					noteContent.String(),
					PacketTypeLongTerm,
					WithRelevance(0.8),
					WithSource("note"),
					WithMetadata("note_id", note.ID),
				))
			}
		}
	}

	// 5. Conversation history from short-term memory
	if mem != nil {
		items := mem.ShortTerm().GetAll()
		totalItems := len(items)
		for i, item := range items {
			if item.Type == "message" {
				role := "user"
				if r, ok := item.Metadata["role"].(string); ok {
					role = r
				}

				// Calculate recency based on position (recent = higher score)
				recencyScore := float64(i+1) / float64(totalItems+1)

				pkt := NewContextPacket(
					fmt.Sprintf("[%s] %s", role, item.Content),
					PacketTypeConversation,
					WithRelevance(0.5),
					WithTimestamp(item.Timestamp),
					WithSource("conversation"),
					WithMetadata("role", role),
				)
				pkt.RecencyScore = recencyScore
				packets = append(packets, pkt)
			}
		}
	}

	// Calculate final scores for all packets
	for _, pkt := range packets {
		pkt.CalculateFinalScore(p.config, now)
	}

	return packets
}

// Select chooses the most relevant packets within token budget
func (p *GSSCPipeline) Select(packets []*ContextPacket, maxTokens int) []*ContextPacket {
	if len(packets) == 0 {
		return nil
	}

	// Sort by final score (descending)
	sort.Slice(packets, func(i, j int) bool {
		// Higher priority types always come first
		if packets[i].Type != packets[j].Type {
			return getDefaultPriority(packets[i].Type) > getDefaultPriority(packets[j].Type)
		}
		return packets[i].FinalScore > packets[j].FinalScore
	})

	// Filter out low relevance packets
	var filtered []*ContextPacket
	for _, pkt := range packets {
		if pkt.FinalScore >= p.config.MinRelevance {
			filtered = append(filtered, pkt)
		}
	}

	// Greedy selection within token budget
	var selected []*ContextPacket
	currentTokens := 0

	for _, pkt := range filtered {
		if currentTokens+pkt.TokenCount <= maxTokens {
			selected = append(selected, pkt)
			currentTokens += pkt.TokenCount
		} else {
			// Try to fit a compressed version if compression is enabled
			if p.config.EnableCompression && pkt.TokenCount > 100 {
				remaining := maxTokens - currentTokens
				if remaining > 50 {
					// Compress the packet
					compressed := p.compressPacket(pkt, remaining)
					if EstimateTokens(compressed.Content) <= remaining {
						selected = append(selected, compressed)
						break
					}
				}
			}
		}
	}

	return selected
}

// Structure organizes selected packets into LLM message format
func (p *GSSCPipeline) Structure(packets []*ContextPacket, query string) []llm.Message {
	if len(packets) == 0 {
		return nil
	}

	var messages []llm.Message
	var systemParts []string
	var conversationMessages []llm.Message

	// Group packets by type
	for _, pkt := range packets {
		switch pkt.Type {
		case PacketTypeLongTerm, PacketTypeRAG, PacketTypeSummary:
			// Add as system context
			systemParts = append(systemParts, pkt.Content)

		case PacketTypeConversation:
			// Extract role and content
			role := "user"
			if r, ok := pkt.Metadata["role"].(string); ok {
				role = r
			}

			// Parse content to extract message
			content := pkt.Content
			if strings.HasPrefix(content, "[user] ") {
				role = "user"
				content = strings.TrimPrefix(content, "[user] ")
			} else if strings.HasPrefix(content, "[assistant] ") {
				role = "assistant"
				content = strings.TrimPrefix(content, "[assistant] ")
			}

			conversationMessages = append(conversationMessages, llm.Message{
				Role:    role,
				Content: content,
			})
		}
	}

	// Add system context if any
	if len(systemParts) > 0 {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: strings.Join(systemParts, "\n\n---\n\n"),
		})
	}

	// Add conversation history
	messages = append(messages, conversationMessages...)

	// Add current query if provided
	if query != "" {
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: query,
		})
	}

	return messages
}

// Compress applies compression strategies to reduce context size
func (p *GSSCPipeline) Compress(packets []*ContextPacket, maxTokens int) []*ContextPacket {
	if len(packets) == 0 {
		return nil
	}

	totalTokens := 0
	for _, pkt := range packets {
		totalTokens += pkt.TokenCount
	}

	// No compression needed
	if totalTokens <= maxTokens {
		return packets
	}

	// Strategy 1: Remove tool results (lowest priority)
	var filtered []*ContextPacket
	for _, pkt := range packets {
		if pkt.Type != PacketTypeToolResult {
			filtered = append(filtered, pkt)
		}
	}

	// Recalculate tokens
	totalTokens = 0
	for _, pkt := range filtered {
		totalTokens += pkt.TokenCount
	}

	if totalTokens <= maxTokens {
		return filtered
	}

	// Strategy 2: Compress conversation history with LLM
	if p.llmProvider != nil {
		var conversationPackets []*ContextPacket
		var otherPackets []*ContextPacket

		for _, pkt := range packets {
			if pkt.Type == PacketTypeConversation {
				conversationPackets = append(conversationPackets, pkt)
			} else {
				otherPackets = append(otherPackets, pkt)
			}
		}

		if len(conversationPackets) > 3 {
			// Compress older conversation history
			keepRecent := 2 // Keep last 2 messages
			var toCompress []*ContextPacket
			var toKeep []*ContextPacket

			for i, pkt := range conversationPackets {
				if i < len(conversationPackets)-keepRecent {
					toCompress = append(toCompress, pkt)
				} else {
					toKeep = append(toKeep, pkt)
				}
			}

			// Create summary packet
			summary := p.summarizePackets(toCompress)
			if summary != nil {
				result := make([]*ContextPacket, 0, len(otherPackets)+len(toKeep)+1)
				result = append(result, otherPackets...)
				result = append(result, summary)
				result = append(result, toKeep...)
				return result
			}
		}
	}

	// Strategy 3: Truncation (fallback)
	for _, pkt := range filtered {
		if pkt.TokenCount > 500 {
			maxLen := 500 * 4 // Approximate chars
			if len(pkt.Content) > maxLen {
				pkt.Content = pkt.Content[:maxLen] + "..."
				pkt.TokenCount = EstimateTokens(pkt.Content)
			}
		}
	}

	return filtered
}

// compressPacket compresses a single packet
func (p *GSSCPipeline) compressPacket(pkt *ContextPacket, maxTokens int) *ContextPacket {
	if pkt.TokenCount <= maxTokens {
		return pkt
	}

	// Simple truncation for now
	maxLen := maxTokens * 4
	if len(pkt.Content) > maxLen {
		return &ContextPacket{
			Content:        pkt.Content[:maxLen] + "...",
			Type:           pkt.Type,
			RelevanceScore: pkt.RelevanceScore * 0.9, // Slight penalty for compression
			RecencyScore:   pkt.RecencyScore,
			Priority:       pkt.Priority,
			Timestamp:      pkt.Timestamp,
			Source:         pkt.Source + "_compressed",
			TokenCount:     EstimateTokens(pkt.Content[:maxLen] + "..."),
		}
	}

	return pkt
}

// summarizePackets creates a summary packet from multiple packets
func (p *GSSCPipeline) summarizePackets(packets []*ContextPacket) *ContextPacket {
	if len(packets) == 0 || p.llmProvider == nil {
		return nil
	}

	var content strings.Builder
	for _, pkt := range packets {
		content.WriteString(pkt.Content)
		content.WriteString("\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`Summarize the following conversation in 2-3 sentences, preserving key information:

%s

Summary:`, content.String())

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	response, err := p.llmProvider.Complete(ctx, messages)
	if err != nil {
		return nil
	}

	return NewContextPacket(
		"[Conversation Summary]\n"+response.Content,
		PacketTypeSummary,
		WithRelevance(0.8),
		WithSource("llm_compression"),
	)
}

// BuildContextWithGSSC is the main entry point for building context with GSSC pipeline
func (p *GSSCPipeline) BuildContextWithGSSC(mem *memory.PersistentMemory, query string, maxTokens int) []llm.Message {
	// 1. Gather
	packets := p.Gather(query, mem)

	// 2. Select
	selected := p.Select(packets, maxTokens)

	// 3. Structure
	messages := p.Structure(selected, query)

	// 4. Compress if needed
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += EstimateTokens(msg.Content)
	}

	if totalTokens > maxTokens && p.config.EnableCompression {
		// Re-select with compression
		selected = p.Compress(selected, maxTokens)
		messages = p.Structure(selected, query)
	}

	return messages
}

// Stats returns pipeline statistics
func (p *GSSCPipeline) Stats() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]any{
		"max_tokens":          p.config.MaxTokens,
		"relevance_weight":    p.config.RelevanceWeight,
		"recency_weight":      p.config.RecencyWeight,
		"min_relevance":       p.config.MinRelevance,
		"compression_enabled": p.config.EnableCompression,
		"cache_size":          len(p.cache),
	}
}
