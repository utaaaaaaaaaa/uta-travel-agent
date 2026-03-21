// Package rag provides RAG (Retrieval-Augmented Generation) functionality
package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/utaaa/uta-travel-agent/internal/grpc/clients"
	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/storage/qdrant"
)

// Service provides RAG functionality
type Service struct {
	qdrantClient   *qdrant.Client
	llmProvider    llm.Provider
	embeddingClient *clients.EmbeddingClient // Use existing gRPC client
}

// Config for RAG service
type Config struct {
	QdrantClient    *qdrant.Client
	LLMProvider     llm.Provider
	EmbeddingClient *clients.EmbeddingClient
}

// NewService creates a new RAG service
func NewService(cfg Config) *Service {
	return &Service{
		qdrantClient:    cfg.QdrantClient,
		llmProvider:     cfg.LLMProvider,
		embeddingClient: cfg.EmbeddingClient,
	}
}

// Embed creates embedding for a single text
func (s *Service) Embed(ctx context.Context, text string) ([]float32, error) {
	if s.embeddingClient == nil {
		return nil, fmt.Errorf("embedding client not configured")
	}

	resp, err := s.embeddingClient.Embed(ctx, clients.EmbedRequest{
		Texts:    []string{text},
		UseCache: true,
	})
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return resp.Embeddings[0], nil
}

// EmbedBatch creates embeddings for multiple texts
func (s *Service) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if s.embeddingClient == nil {
		return nil, fmt.Errorf("embedding client not configured")
	}

	resp, err := s.embeddingClient.BatchEmbed(ctx, texts, "")
	if err != nil {
		return nil, fmt.Errorf("batch embedding failed: %w", err)
	}

	return resp.Embeddings, nil
}

// QueryResult represents a RAG query result
type QueryResult struct {
	Answer     string
	Sources    []Source
	TokensUsed int
}

// Source represents a source document
type Source struct {
	Content  string
	Score    float32
	Metadata map[string]interface{}
}

// Query performs RAG query: retrieve relevant docs and generate answer
func (s *Service) Query(ctx context.Context, collectionID, query string, topK int) (*QueryResult, error) {
	if s.qdrantClient == nil {
		return nil, fmt.Errorf("qdrant client not configured")
	}

	if s.embeddingClient == nil {
		return nil, fmt.Errorf("embedding client not configured")
	}

	// Step 1: Generate embedding for query
	queryVector, err := s.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Step 2: Search for relevant documents
	searchResults, err := s.qdrantClient.Search(ctx, collectionID, queryVector, topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	if len(searchResults) == 0 {
		return &QueryResult{
			Answer:  "抱歉，我没有找到相关的信息。请尝试其他问题。",
			Sources: nil,
		}, nil
	}

	// Step 3: Build context from search results
	var contextBuilder strings.Builder
	sources := make([]Source, 0, len(searchResults))

	for i, result := range searchResults {
		content := extractContent(result.Payload)
		if content == "" {
			continue
		}

		contextBuilder.WriteString(fmt.Sprintf("\n[文档 %d]\n%s\n", i+1, content))
		sources = append(sources, Source{
			Content:  content,
			Score:    result.Score,
			Metadata: result.Payload,
		})
	}

	context := contextBuilder.String()

	// Step 4: Generate answer using LLM
	answer, err := s.generateAnswer(ctx, query, context)
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	return &QueryResult{
		Answer:  answer,
		Sources: sources,
	}, nil
}

// QueryStream performs RAG query with streaming response
func (s *Service) QueryStream(ctx context.Context, collectionID, query string, topK int) (<-chan string, <-chan error) {
	outputCh := make(chan string, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)

		result, err := s.Query(ctx, collectionID, query, topK)
		if err != nil {
			errCh <- err
			return
		}

		// Stream the answer word by word
		words := strings.Fields(result.Answer)
		for _, word := range words {
			select {
			case outputCh <- word + " ":
			case <-ctx.Done():
				return
			}
		}
	}()

	return outputCh, errCh
}

// generateAnswer uses LLM to generate answer from context
func (s *Service) generateAnswer(ctx context.Context, query, context string) (string, error) {
	if s.llmProvider == nil {
		// Fallback: return context directly
		return fmt.Sprintf("根据知识库信息：\n\n%s", truncate(context, 500)), nil
	}

	systemPrompt := `你是一位专业的旅行导游助手。你的任务是根据提供的目的地知识库信息，为用户提供准确、有用的旅行建议。

规则:
1. 只使用提供的上下文信息回答问题
2. 如果信息不足，坦诚告知用户
3. 保持回答简洁但有价值
4. 适当添加文化背景和有趣的事实
5. 使用友好的语气，像一位本地导游一样交流
6. 用中文回答`

	userMsg := fmt.Sprintf(`用户问题: %s

参考信息:
%s

请根据以上信息回答用户的问题。如果参考信息中没有相关内容，请坦诚告知。`, query, context)

	response, err := s.llmProvider.CompleteWithSystem(ctx, systemPrompt, []llm.Message{
		{Role: "user", Content: userMsg},
	})
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

// extractContent extracts content from payload
func extractContent(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}

	// Try common field names
	for _, field := range []string{"content", "text", "body", "description"} {
		if v, ok := payload[field]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}

	return ""
}

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GetAttractions retrieves attraction information from the knowledge base
func (s *Service) GetAttractions(ctx context.Context, collectionID string, limit int) ([]map[string]interface{}, error) {
	if s.qdrantClient == nil {
		return nil, fmt.Errorf("qdrant client not configured")
	}

	// Use a generic query to find attractions
	attractionQueries := []string{
		"景点 景胜 旅游胜地",
		"寺庙 神社 教堂",
		"博物馆 美术馆",
		"公园 自然景观",
	}

	allResults := make([]map[string]interface{}, 0)
	seenNames := make(map[string]bool)

	for _, query := range attractionQueries {
		// Get embedding for query
		queryVector, err := s.Embed(ctx, query)
		if err != nil {
			continue
		}

		// Search
		results, err := s.qdrantClient.Search(ctx, collectionID, queryVector, 5)
		if err != nil {
			continue
		}

		for _, result := range results {
			// Extract attraction info from payload
			name := extractAttractionName(result.Payload)
			if name == "" || seenNames[name] {
				continue
			}
			seenNames[name] = true

			content := extractContent(result.Payload)
			attraction := map[string]interface{}{
				"name":        name,
				"description": truncate(content, 300),
				"score":       result.Score,
			}

			// Add metadata if available
			if url, ok := result.Payload["url"].(string); ok {
				attraction["url"] = url
			}
			if title, ok := result.Payload["title"].(string); ok {
				attraction["title"] = title
			}

			allResults = append(allResults, attraction)
		}

		if len(allResults) >= limit {
			break
		}
	}

	// Sort by score and limit
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults, nil
}

// extractAttractionName tries to extract an attraction name from payload
func extractAttractionName(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}

	// Try title field first
	if title, ok := payload["title"].(string); ok && title != "" {
		return title
	}

	// Try to extract from content
	content := extractContent(payload)
	if content == "" {
		return ""
	}

	// Try to find a name pattern (first sentence or line)
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// If first line is short enough, use as name
		if len(firstLine) > 0 && len(firstLine) < 50 {
			// Remove common prefixes
			for _, prefix := range []string{"是", "位于", "名为"} {
				if idx := strings.Index(firstLine, prefix); idx > 0 {
					return strings.TrimSpace(firstLine[:idx])
				}
			}
			return firstLine
		}
	}

	return ""
}
