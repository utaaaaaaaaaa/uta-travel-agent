// Package contextx provides context engineering for agents
package contextx

import (
	"math"
	"time"
)

// PacketType defines the type of context packet
type PacketType string

const (
	PacketTypeSystem       PacketType = "system"        // System instructions
	PacketTypeLongTerm     PacketType = "long_term"     // Long-term memory / user preferences
	PacketTypeRAG          PacketType = "rag"           // RAG knowledge retrieval
	PacketTypeConversation PacketType = "conversation"  // Conversation history
	PacketTypeSummary      PacketType = "summary"       // Compressed summary
	PacketTypeNote         PacketType = "note"          // Long-horizon task notes
	PacketTypeToolResult   PacketType = "tool_result"   // Tool execution results
	PacketTypeMessage      PacketType = "message"       // Single message
)

// ContextPacket represents a unit of context information
type ContextPacket struct {
	// Content is the actual text content
	Content string `json:"content"`

	// Type indicates the packet type
	Type PacketType `json:"type"`

	// RelevanceScore measures relevance to current query (0.0 - 1.0)
	RelevanceScore float64 `json:"relevance_score"`

	// RecencyScore measures temporal relevance (0.0 - 1.0, decayed by age)
	RecencyScore float64 `json:"recency_score"`

	// FinalScore is the combined score used for selection
	FinalScore float64 `json:"final_score"`

	// Priority is the base priority level (higher = more important)
	Priority float64 `json:"priority"`

	// Timestamp when the packet was created
	Timestamp time.Time `json:"timestamp"`

	// Source identifies where the packet came from
	Source string `json:"source"`

	// Metadata for additional information
	Metadata map[string]any `json:"metadata,omitempty"`

	// TokenCount is estimated token count
	TokenCount int `json:"token_count"`
}

// ContextConfig holds configuration for GSSC pipeline
type ContextConfig struct {
	// MaxTokens is the maximum context window size
	MaxTokens int `json:"max_tokens"`

	// RelevanceWeight is the weight for relevance score (default 0.6)
	RelevanceWeight float64 `json:"relevance_weight"`

	// RecencyWeight is the weight for recency score (default 0.4)
	RecencyWeight float64 `json:"recency_weight"`

	// MinRelevance is the minimum relevance threshold (default 0.3)
	MinRelevance float64 `json:"min_relevance"`

	// EnableCompression enables intelligent compression
	EnableCompression bool `json:"enable_compression"`

	// CompressionThreshold is the token threshold to trigger compression
	CompressionThreshold int `json:"compression_threshold"`

	// TimeDecayFactor controls recency decay rate
	TimeDecayFactor float64 `json:"time_decay_factor"`
}

// DefaultContextConfig returns default configuration
func DefaultContextConfig() ContextConfig {
	return ContextConfig{
		MaxTokens:            8000,
		RelevanceWeight:      0.6,
		RecencyWeight:        0.4,
		MinRelevance:         0.3,
		EnableCompression:    true,
		CompressionThreshold: 6000,
		TimeDecayFactor:      0.1,
	}
}

// NewContextPacket creates a new context packet
func NewContextPacket(content string, packetType PacketType, opts ...PacketOption) *ContextPacket {
	p := &ContextPacket{
		Content:      content,
		Type:         packetType,
		RelevanceScore: 0.5,
		RecencyScore:  1.0,
		Priority:      getDefaultPriority(packetType),
		Timestamp:     time.Now(),
		Metadata:      make(map[string]any),
		TokenCount:    EstimateTokens(content),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// PacketOption is a functional option for ContextPacket
type PacketOption func(*ContextPacket)

// WithRelevance sets the relevance score
func WithRelevance(score float64) PacketOption {
	return func(p *ContextPacket) {
		p.RelevanceScore = score
	}
}

// WithPriority sets the priority
func WithPriority(priority float64) PacketOption {
	return func(p *ContextPacket) {
		p.Priority = priority
	}
}

// WithSource sets the source
func WithSource(source string) PacketOption {
	return func(p *ContextPacket) {
		p.Source = source
	}
}

// WithMetadata sets metadata
func WithMetadata(key string, value any) PacketOption {
	return func(p *ContextPacket) {
		if p.Metadata == nil {
			p.Metadata = make(map[string]any)
		}
		p.Metadata[key] = value
	}
}

// WithTimestamp sets the timestamp
func WithTimestamp(t time.Time) PacketOption {
	return func(p *ContextPacket) {
		p.Timestamp = t
	}
}

// CalculateFinalScore computes the final score based on relevance, recency, and priority
func (p *ContextPacket) CalculateFinalScore(config ContextConfig, queryTime time.Time) {
	// Calculate recency score with time decay
	ageHours := queryTime.Sub(p.Timestamp).Hours()
	decayFactor := config.TimeDecayFactor
	if decayFactor == 0 {
		decayFactor = 0.1
	}
	p.RecencyScore = calculateRecencyScore(ageHours, decayFactor)

	// Combine scores with weights
	relevanceWeight := config.RelevanceWeight
	recencyWeight := config.RecencyWeight

	// Normalize weights
	totalWeight := relevanceWeight + recencyWeight
	if totalWeight > 0 {
		relevanceWeight /= totalWeight
		recencyWeight /= totalWeight
	}

	// Final score = weighted sum + priority bonus
	p.FinalScore = (p.RelevanceScore * relevanceWeight) +
		(p.RecencyScore * recencyWeight) +
		(p.Priority * 0.1) // Priority as bonus
}

// getDefaultPriority returns default priority for packet type
func getDefaultPriority(packetType PacketType) float64 {
	switch packetType {
	case PacketTypeSystem:
		return 1.0
	case PacketTypeLongTerm:
		return 0.9
	case PacketTypeRAG:
		return 0.7
	case PacketTypeConversation:
		return 0.5
	case PacketTypeSummary:
		return 0.6
	case PacketTypeNote:
		return 0.8
	case PacketTypeToolResult:
		return 0.3
	default:
		return 0.5
	}
}

// calculateRecencyScore calculates recency score with exponential decay
func calculateRecencyScore(ageHours, decayFactor float64) float64 {
	if ageHours <= 0 {
		return 1.0
	}
	// Exponential decay: score = exp(-decayFactor * age / 24)
	// Normalized to 24 hours as baseline
	return math.Exp(-decayFactor * ageHours / 24.0)
}