package contextx

import (
	"context"
	"testing"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/memory"
)

// MockRAGService implements RAGService for testing
type MockRAGService struct {
	results []RAGResult
	err     error
}

func (m *MockRAGService) Query(ctx context.Context, query string, limit int) ([]RAGResult, error) {
	return m.results, m.err
}

// MockLLMProvider implements llm.Provider for testing
type MockLLMProvider struct {
	response *llm.Response
	err      error
}

func (m *MockLLMProvider) Complete(ctx context.Context, messages []llm.Message, opts ...llm.Option) (*llm.Response, error) {
	return m.response, m.err
}

func (m *MockLLMProvider) CompleteWithSystem(ctx context.Context, system string, messages []llm.Message, opts ...llm.Option) (*llm.Response, error) {
	return m.response, m.err
}

func (m *MockLLMProvider) RAGQuery(ctx context.Context, query, context string, opts ...llm.Option) (*llm.Response, error) {
	return m.response, m.err
}

func (m *MockLLMProvider) Stream(ctx context.Context, messages []llm.Message, opts ...llm.Option) (<-chan llm.StreamChunk, <-chan error) {
	chunkCh := make(chan llm.StreamChunk)
	errCh := make(chan error, 1)
	go func() {
		defer close(chunkCh)
		defer close(errCh)
	}()
	return chunkCh, errCh
}

func TestNewContextPacket(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		packetType  PacketType
		opts        []PacketOption
		wantType    PacketType
		wantContent string
	}{
		{
			name:        "basic packet",
			content:     "test content",
			packetType:  PacketTypeConversation,
			wantType:    PacketTypeConversation,
			wantContent: "test content",
		},
		{
			name:        "packet with relevance",
			content:     "important content",
			packetType:  PacketTypeRAG,
			opts:        []PacketOption{WithRelevance(0.9)},
			wantType:    PacketTypeRAG,
			wantContent: "important content",
		},
		{
			name:        "packet with source",
			content:     "sourced content",
			packetType:  PacketTypeLongTerm,
			opts:        []PacketOption{WithSource("test_source")},
			wantType:    PacketTypeLongTerm,
			wantContent: "sourced content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := NewContextPacket(tt.content, tt.packetType, tt.opts...)

			if pkt.Type != tt.wantType {
				t.Errorf("expected type %s, got %s", tt.wantType, pkt.Type)
			}
			if pkt.Content != tt.wantContent {
				t.Errorf("expected content %q, got %q", tt.wantContent, pkt.Content)
			}
			if pkt.TokenCount <= 0 {
				t.Errorf("expected positive token count, got %d", pkt.TokenCount)
			}
		})
	}
}

func TestCalculateFinalScore(t *testing.T) {
	config := DefaultContextConfig()
	now := time.Now()

	tests := []struct {
		name             string
		pkt              *ContextPacket
		minExpectedScore float64
		maxExpectedScore float64
	}{
		{
			name: "high relevance recent packet",
			pkt: NewContextPacket(
				"important content",
				PacketTypeRAG,
				WithRelevance(0.9),
				WithTimestamp(now.Add(-1*time.Hour)),
			),
			minExpectedScore: 0.5,
			maxExpectedScore: 1.5,
		},
		{
			name: "low relevance old packet",
			pkt: NewContextPacket(
				"old content",
				PacketTypeConversation,
				WithRelevance(0.3),
				WithTimestamp(now.Add(-48*time.Hour)),
			),
			minExpectedScore: 0.0,
			maxExpectedScore: 0.8,
		},
		{
			name: "system packet high priority",
			pkt: NewContextPacket(
				"system instructions",
				PacketTypeSystem,
				WithRelevance(1.0),
				WithTimestamp(now),
			),
			minExpectedScore: 0.8,
			maxExpectedScore: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pkt.CalculateFinalScore(config, now)

			if tt.pkt.FinalScore < tt.minExpectedScore {
				t.Errorf("final score %f < min expected %f", tt.pkt.FinalScore, tt.minExpectedScore)
			}
			if tt.pkt.FinalScore > tt.maxExpectedScore {
				t.Errorf("final score %f > max expected %f", tt.pkt.FinalScore, tt.maxExpectedScore)
			}
		})
	}
}

func TestGSSCPipelineGather(t *testing.T) {
	// Create memory with some data
	mem := memory.NewPersistentMemory(nil, 100)
	mem.ShortTerm().AddMessage("user", "Hello")
	mem.ShortTerm().AddMessage("assistant", "Hi there!")

	// Create pipeline
	config := DefaultContextConfig()
	pipeline := NewGSSCPipeline(config, mem, nil, nil)

	// Gather packets
	packets := pipeline.Gather("test query", mem)

	// Verify we got packets
	if len(packets) == 0 {
		t.Error("expected at least some packets from gather")
	}

	// Verify packet types
	for _, pkt := range packets {
		if pkt.Content == "" {
			t.Error("packet has empty content")
		}
		if pkt.TokenCount <= 0 {
			t.Errorf("packet has invalid token count: %d", pkt.TokenCount)
		}
		if pkt.FinalScore == 0 {
			t.Error("packet has zero final score (CalculateFinalScore not called)")
		}
	}
}

func TestGSSCPipelineSelect(t *testing.T) {
	config := DefaultContextConfig()
	config.MinRelevance = 0.3
	pipeline := NewGSSCPipeline(config, nil, nil, nil)

	// Create test packets
	packets := []*ContextPacket{
		NewContextPacket("high relevance", PacketTypeRAG, WithRelevance(0.9)),
		NewContextPacket("medium relevance", PacketTypeConversation, WithRelevance(0.5)),
		NewContextPacket("low relevance", PacketTypeToolResult, WithRelevance(0.2)),
	}

	// Calculate scores
	now := time.Now()
	for _, pkt := range packets {
		pkt.CalculateFinalScore(config, now)
	}

	// Select packets
	selected := pipeline.Select(packets, 1000)

	// Verify selection
	if len(selected) == 0 {
		t.Error("expected at least one selected packet")
	}

	// Verify low relevance packet was filtered out based on FinalScore
	for _, pkt := range selected {
		if pkt.FinalScore < config.MinRelevance {
			t.Errorf("selected packet has final score below threshold: %f < %f",
				pkt.FinalScore, config.MinRelevance)
		}
	}
}

func TestGSSCPipelineStructure(t *testing.T) {
	config := DefaultContextConfig()
	pipeline := NewGSSCPipeline(config, nil, nil, nil)

	// Create test packets
	packets := []*ContextPacket{
		NewContextPacket("System context", PacketTypeLongTerm, WithRelevance(0.9)),
		NewContextPacket("[user] Hello", PacketTypeConversation, WithRelevance(0.5),
			WithMetadata("role", "user")),
		NewContextPacket("[assistant] Hi there!", PacketTypeConversation, WithRelevance(0.5),
			WithMetadata("role", "assistant")),
	}

	// Structure packets
	messages := pipeline.Structure(packets, "What's the weather?")

	// Verify structure
	if len(messages) == 0 {
		t.Error("expected at least one message")
	}

	// Verify query is included
	foundQuery := false
	for _, msg := range messages {
		if msg.Content == "What's the weather?" {
			foundQuery = true
			break
		}
	}
	if !foundQuery {
		t.Error("query not included in structured messages")
	}

	// Verify system context is included
	foundSystem := false
	for _, msg := range messages {
		if msg.Role == "system" && msg.Content == "System context" {
			foundSystem = true
			break
		}
	}
	if !foundSystem {
		t.Error("system context not included in structured messages")
	}
}

func TestGSSCPipelineCompress(t *testing.T) {
	config := DefaultContextConfig()
	config.EnableCompression = true
	pipeline := NewGSSCPipeline(config, nil, nil, nil)

	// Create packets that exceed token limit
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	packets := []*ContextPacket{
		{
			Content:    string(largeContent),
			Type:       PacketTypeConversation,
			TokenCount: 5000,
		},
		{
			Content:    "small content",
			Type:       PacketTypeConversation,
			TokenCount: 10,
		},
	}

	// Compress to small token limit
	compressed := pipeline.Compress(packets, 100)

	// Verify compression occurred
	if len(compressed) == 0 {
		t.Error("expected at least one compressed packet")
	}

	// Verify total tokens is within limit
	totalTokens := 0
	for _, pkt := range compressed {
		totalTokens += pkt.TokenCount
	}

	// Allow some buffer for compression overhead
	if totalTokens > 150 {
		t.Errorf("total tokens %d exceeds limit (with buffer)", totalTokens)
	}
}

func TestGSSCPipelineFullFlow(t *testing.T) {
	// Create memory with conversation
	mem := memory.NewPersistentMemory(nil, 100)
	mem.ShortTerm().AddMessage("user", "I want to visit Tokyo")
	mem.ShortTerm().AddMessage("assistant", "Great choice! Tokyo has many attractions.")

	// Create mock RAG service
	mockRAG := &MockRAGService{
		results: []RAGResult{
			{Content: "Tokyo Tower is a famous landmark", Source: "wiki", Score: 0.9},
			{Content: "Shibuya Crossing is busy", Source: "wiki", Score: 0.8},
		},
	}

	// Create pipeline
	config := DefaultContextConfig()
	config.MaxTokens = 4000
	pipeline := NewGSSCPipeline(config, mem, mockRAG, nil)

	// Run full pipeline
	messages := pipeline.BuildContextWithGSSC(mem, "Tell me about Tokyo", 4000)

	// Verify results
	if len(messages) == 0 {
		t.Error("expected messages from GSSC pipeline")
	}

	// Verify messages have valid roles
	for _, msg := range messages {
		if msg.Role != "system" && msg.Role != "user" && msg.Role != "assistant" {
			t.Errorf("invalid message role: %s", msg.Role)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		minToken int
		maxToken int
	}{
		{
			name:     "English text",
			content:  "Hello world this is a test",
			minToken: 3,
			maxToken: 15,
		},
		{
			name:     "Chinese text",
			content:  "你好世界这是一个测试",
			minToken: 5,
			maxToken: 20,
		},
		{
			name:     "Mixed text",
			content:  "Hello 世界 this is 一个 test",
			minToken: 5,
			maxToken: 20,
		},
		{
			name:     "Empty text",
			content:  "",
			minToken: 0,
			maxToken: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := EstimateTokens(tt.content)

			if tokens < tt.minToken {
				t.Errorf("tokens %d < min expected %d", tokens, tt.minToken)
			}
			if tokens > tt.maxToken {
				t.Errorf("tokens %d > max expected %d", tokens, tt.maxToken)
			}
		})
	}
}

func TestContextConfig(t *testing.T) {
	config := DefaultContextConfig()

	if config.MaxTokens <= 0 {
		t.Error("default MaxTokens should be positive")
	}
	if config.RelevanceWeight <= 0 || config.RelevanceWeight > 1 {
		t.Errorf("invalid RelevanceWeight: %f", config.RelevanceWeight)
	}
	if config.RecencyWeight <= 0 || config.RecencyWeight > 1 {
		t.Errorf("invalid RecencyWeight: %f", config.RecencyWeight)
	}
	if config.MinRelevance < 0 || config.MinRelevance > 1 {
		t.Errorf("invalid MinRelevance: %f", config.MinRelevance)
	}
}

func TestPacketPriority(t *testing.T) {
	tests := []struct {
		packetType     PacketType
		expectedMinPri float64
		expectedMaxPri float64
	}{
		{PacketTypeSystem, 0.9, 1.1},
		{PacketTypeLongTerm, 0.8, 1.0},
		{PacketTypeRAG, 0.6, 0.8},
		{PacketTypeConversation, 0.4, 0.6},
		{PacketTypeToolResult, 0.2, 0.4},
	}

	for _, tt := range tests {
		t.Run(string(tt.packetType), func(t *testing.T) {
			priority := getDefaultPriority(tt.packetType)

			if priority < tt.expectedMinPri {
				t.Errorf("priority %f < min expected %f", priority, tt.expectedMinPri)
			}
			if priority > tt.expectedMaxPri {
				t.Errorf("priority %f > max expected %f", priority, tt.expectedMaxPri)
			}
		})
	}
}

func TestRecencyScoreDecay(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		age            time.Duration
		minScore       float64
		maxScore       float64
	}{
		{
			name:     "fresh packet",
			age:      0,
			minScore: 0.99,
			maxScore: 1.01,
		},
		{
			name:     "1 hour old",
			age:      1 * time.Hour,
			minScore: 0.9,
			maxScore: 1.0,
		},
		{
			name:     "24 hours old",
			age:      24 * time.Hour,
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name:     "1 week old",
			age:      7 * 24 * time.Hour,
			minScore: 0.0,
			maxScore: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := NewContextPacket("test", PacketTypeConversation,
				WithTimestamp(now.Add(-tt.age)))

			config := DefaultContextConfig()
			pkt.CalculateFinalScore(config, now)

			if pkt.RecencyScore < tt.minScore {
				t.Errorf("recency score %f < min expected %f", pkt.RecencyScore, tt.minScore)
			}
			if pkt.RecencyScore > tt.maxScore {
				t.Errorf("recency score %f > max expected %f", pkt.RecencyScore, tt.maxScore)
			}
		})
	}
}

// MockSemanticMemoryService implements SemanticMemoryService for testing
type MockSemanticMemoryService struct {
	results []memory.MemoryItem
	err     error
}

func (m *MockSemanticMemoryService) Retrieve(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return m.results, m.err
}

func TestGSSCPipelineWithSemanticMemory(t *testing.T) {
	config := DefaultContextConfig()
	pipeline := NewGSSCPipeline(config, nil, nil, nil)

	// Set mock semantic memory
	mockSemantic := &MockSemanticMemoryService{
		results: []memory.MemoryItem{
			{
				ID:      "sem-1",
				Content: "Tokyo Tower is a famous landmark in Tokyo, Japan.",
				Entities: []memory.ExtractedEntity{
					{Name: "Tokyo Tower", Type: "landmark", Confidence: 0.9},
					{Name: "Tokyo", Type: "destination", Confidence: 0.95},
				},
			},
			{
				ID:      "sem-2",
				Content: "Mount Fuji is Japan's highest mountain.",
				Entities: []memory.ExtractedEntity{
					{Name: "Mount Fuji", Type: "landmark", Confidence: 0.95},
				},
			},
		},
	}
	pipeline.SetSemanticMemory(mockSemantic)

	// Gather context
	packets := pipeline.Gather("Tokyo attractions", nil)

	// Verify semantic memory packet was added
	foundSemantic := false
	for _, pkt := range packets {
		if pkt.Source == "semantic_memory" {
			foundSemantic = true
			if !contains(pkt.Content, "语义记忆检索") {
				t.Error("expected semantic memory header in content")
			}
			if !contains(pkt.Content, "Tokyo Tower") {
				t.Error("expected Tokyo Tower in semantic memory content")
			}
			break
		}
	}

	if !foundSemantic {
		t.Error("expected semantic memory packet in gather results")
	}
}

func TestGSSCPipelineSemanticMemoryWithEntities(t *testing.T) {
	config := DefaultContextConfig()
	pipeline := NewGSSCPipeline(config, nil, nil, nil)

	// Set mock semantic memory with entities
	mockSemantic := &MockSemanticMemoryService{
		results: []memory.MemoryItem{
			{
				ID:      "sem-1",
				Content: "Kyoto has many temples.",
				Entities: []memory.ExtractedEntity{
					{Name: "Kyoto", Type: "destination", Confidence: 0.9},
					{Name: "temples", Type: "attraction", Confidence: 0.85},
				},
			},
		},
	}
	pipeline.SetSemanticMemory(mockSemantic)

	// Gather context
	packets := pipeline.Gather("Kyoto", nil)

	// Find semantic memory packet
	var semanticPkt *ContextPacket
	for _, pkt := range packets {
		if pkt.Source == "semantic_memory" {
			semanticPkt = pkt
			break
		}
	}

	if semanticPkt == nil {
		t.Fatal("expected semantic memory packet")
	}

	// Verify entities are included in content
	if !contains(semanticPkt.Content, "实体:") {
		t.Error("expected entities section in semantic memory content")
	}
	if !contains(semanticPkt.Content, "Kyoto(destination)") {
		t.Error("expected Kyoto entity in content")
	}
}

func TestGSSCPipelineSemanticMemoryError(t *testing.T) {
	config := DefaultContextConfig()
	pipeline := NewGSSCPipeline(config, nil, nil, nil)

	// Set mock semantic memory that returns error
	mockSemantic := &MockSemanticMemoryService{
		err: context.DeadlineExceeded,
	}
	pipeline.SetSemanticMemory(mockSemantic)

	// Gather context should not fail even if semantic memory fails
	packets := pipeline.Gather("test query", nil)

	// Should have no semantic memory packet
	for _, pkt := range packets {
		if pkt.Source == "semantic_memory" {
			t.Error("expected no semantic memory packet on error")
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
