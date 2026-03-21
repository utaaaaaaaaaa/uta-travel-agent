package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/tools"
)

// Tool types
type ToolType string

const (
	ToolTypeMCP     ToolType = "mcp"
	ToolTypeSkill   ToolType = "skill"
	ToolTypeService ToolType = "service"
)

// Tool represents a callable tool
type Tool struct {
	Name        string         `json:"name"`
	Type        ToolType       `json:"type"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Required    bool           `json:"required,omitempty"`
}

// ToolExecutor defines how a tool is executed
type ToolExecutor interface {
	Execute(ctx context.Context, params map[string]any) (*ToolResult, error)
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool           `json:"success"`
	Data    any            `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// ToolRegistry manages all available tools
type ToolRegistry interface {
	Register(tool Tool, executor ToolExecutor) error
	Get(toolName string) (Tool, bool)
	Execute(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error)
	ListTools() []Tool
}

// DefaultToolRegistry is the default implementation
type DefaultToolRegistry struct {
	tools     map[string]Tool
	executors map[string]ToolExecutor
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *DefaultToolRegistry {
	return &DefaultToolRegistry{
		tools:     make(map[string]Tool),
		executors: make(map[string]ToolExecutor),
	}
}

// Register adds a tool to the registry
func (r *DefaultToolRegistry) Register(tool Tool, executor ToolExecutor) error {
	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}
	r.tools[tool.Name] = tool
	r.executors[tool.Name] = executor
	return nil
}

// Get retrieves a tool by name
func (r *DefaultToolRegistry) Get(toolName string) (Tool, bool) {
	tool, exists := r.tools[toolName]
	return tool, exists
}

// Execute runs a tool with given parameters
func (r *DefaultToolRegistry) Execute(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error) {
	executor, exists := r.executors[toolName]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}
	return executor.Execute(ctx, params)
}

// ListTools returns all registered tools
func (r *DefaultToolRegistry) ListTools() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ToolConfig holds configuration for tool initialization.
type ToolConfig struct {
	TavilyAPIKey string
}

// SetupResearchTools registers tools for Research Agent (high-quality data sources).
func SetupResearchTools(registry ToolRegistry, config ToolConfig) error {
	// Wikipedia Search - for authoritative static knowledge
	wikiTool := tools.NewWikipediaSearchTool("zh") // Default to Chinese
	if err := registry.Register(Tool{
		Name:        wikiTool.GetName(),
		Type:        ToolTypeMCP,
		Description: wikiTool.GetDescription(),
		Parameters:  wikiTool.GetParameters(),
	}, &toolExecutorAdapter{impl: wikiTool}); err != nil {
		return fmt.Errorf("failed to register wikipedia_search: %w", err)
	}

	// Web Reader - for reading specific pages
	webReader := tools.NewWebReaderTool()
	if err := registry.Register(Tool{
		Name:        webReader.GetName(),
		Type:        ToolTypeMCP,
		Description: webReader.GetDescription(),
		Parameters:  webReader.GetParameters(),
	}, &toolExecutorAdapter{impl: webReader}); err != nil {
		return fmt.Errorf("failed to register web_reader: %w", err)
	}

	return nil
}

// SetupDestinationTools registers tools for Destination Agent (RAG + real-time search).
func SetupDestinationTools(registry ToolRegistry, config ToolConfig) error {
	// Tavily Search - for real-time information
	if config.TavilyAPIKey != "" {
		tavilyTool := tools.NewTavilySearchTool(config.TavilyAPIKey)
		if err := registry.Register(Tool{
			Name:        tavilyTool.GetName(),
			Type:        ToolTypeMCP,
			Description: tavilyTool.GetDescription(),
			Parameters:  tavilyTool.GetParameters(),
		}, &toolExecutorAdapter{impl: tavilyTool}); err != nil {
			return fmt.Errorf("failed to register tavily_search: %w", err)
		}
	}

	return nil
}

// SetupMainTools registers tools for Main Agent (general assistant).
func SetupMainTools(registry ToolRegistry, config ToolConfig) error {
	// Tavily Search - for real-time information
	if config.TavilyAPIKey != "" {
		tavilyTool := tools.NewTavilySearchTool(config.TavilyAPIKey)
		if err := registry.Register(Tool{
			Name:        tavilyTool.GetName(),
			Type:        ToolTypeMCP,
			Description: tavilyTool.GetDescription(),
			Parameters:  tavilyTool.GetParameters(),
		}, &toolExecutorAdapter{impl: tavilyTool}); err != nil {
			return fmt.Errorf("failed to register tavily_search: %w", err)
		}
	}

	return nil
}

// SetupAllTools registers all available tools.
func SetupAllTools(registry ToolRegistry, config ToolConfig) error {
	if err := SetupResearchTools(registry, config); err != nil {
		return err
	}
	if err := SetupDestinationTools(registry, config); err != nil {
		return err
	}
	if err := SetupMainTools(registry, config); err != nil {
		return err
	}
	return nil
}

// toolExecutorAdapter adapts our tools.Tool interface to agent.ToolExecutor.
type toolExecutorAdapter struct {
	impl interface {
		Execute(ctx context.Context, params map[string]any) (map[string]any, error)
	}
}

func (a *toolExecutorAdapter) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	result, err := a.impl.Execute(ctx, params)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}
	return &ToolResult{
		Success: true,
		Data:    result,
	}, nil
}

// --- MCP Tools ---

// BraveSearchTool performs web search
type BraveSearchTool struct {
	apiKey string
}

// NewBraveSearchTool creates a new brave search tool
func NewBraveSearchTool(apiKey string) *BraveSearchTool {
	return &BraveSearchTool{apiKey: apiKey}
}

// Execute performs a brave search
func (t *BraveSearchTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	// TODO: Implement actual Brave Search API call
	// For now, return mock results
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"query":   query,
			"results": []any{
				map[string]any{
					"title": "京都旅游攻略",
					"url":   "https://example.com/kyoto",
					"snippet": "京都最佳旅游时间、交通指南",
				},
			},
		},
	}, nil
}

// WebReaderTool reads web content
type WebReaderTool struct {
	mcpClient MCPClient
}

// MCPClient interface for MCP communication
type MCPClient interface {
	Call(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error)
}

// NewWebReaderTool creates a new web reader tool
func NewWebReaderTool(client MCPClient) *WebReaderTool {
	return &WebReaderTool{mcpClient: client}
}

// Execute reads web content
func (t *WebReaderTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	url, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter required")
	}

	// TODO: Implement actual MCP call
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"url":     url,
			"content": "京都，日本的文化古都，拥有众多世界文化遗产...",
			"size":    1024,
		},
	}, nil
}

// --- Skill Tools ---

// LLMSummarizeTool summarizes content using LLM
type LLMSummarizeTool struct {
	llmProvider LLMProvider
}

// LLMProvider interface for LLM calls
type LLMProvider interface {
	Complete(ctx context.Context, prompt string, messages []LLMMessage, opts ...LLMOption) (*LLMResponse, error)
}

// LLMMessage represents a chat message
type LLMMessage struct {
	Role    string
	Content string
}

// LLMResponse represents LLM response
type LLMResponse struct {
	Content      string
	TokensUsed   int
}

// LLMOption is a functional option for LLM calls
type LLMOption func(*LLMOptions)

// LLMOptions holds LLM options
type LLMOptions struct {
	Temperature float64
	MaxTokens   int
}

// NewLLMSummarizeTool creates a new LLM summarize tool
func NewLLMSummarizeTool(provider LLMProvider) *LLMSummarizeTool {
	return &LLMSummarizeTool{llmProvider: provider}
}

// Execute summarizes content
func (t *LLMSummarizeTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	content, ok := params["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter required")
	}

	// TODO: Implement actual LLM call
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"summary": "京都旅游攻略摘要...",
			"original_length": len(content),
		},
	}, nil
}

// BuildKnowledgeBaseTool builds knowledge base from documents
type BuildKnowledgeBaseTool struct {
	llmProvider LLMProvider
}

// NewBuildKnowledgeBaseTool creates a new build knowledge base tool
func NewBuildKnowledgeBaseTool(provider LLMProvider) *BuildKnowledgeBaseTool {
	return &BuildKnowledgeBaseTool{llmProvider: provider}
}

// Execute builds knowledge base
func (t *BuildKnowledgeBaseTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	documents, ok := params["documents"].([]any)
	if !ok {
		return nil, fmt.Errorf("documents parameter required")
	}

	// TODO: Implement knowledge base building
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"knowledge_base": map[string]any{
				"categories": []string{"景点", "美食", "文化"},
				"total_items": len(documents),
			},
		},
	}, nil
}

// BuildKnowledgeIndexTool builds vector index from documents
type BuildKnowledgeIndexTool struct {
	embeddingClient EmbeddingClient
	qdrantClient   QdrantClient
	collectionID   string
}

// EmbeddingClient interface for embedding service
type EmbeddingClient interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
}

// QdrantClient interface for Qdrant operations
type QdrantClient interface {
	CreateCollection(ctx context.Context, name string, vectorSize uint64) error
	Upsert(ctx context.Context, collection string, points []QdrantPoint) error
	Search(ctx context.Context, collection string, vector []float32, limit int) ([]QdrantSearchResult, error)
}

// QdrantPoint represents a point to store
type QdrantPoint struct {
	ID      string
	Vector  []float32
	Payload map[string]any
}

// QdrantSearchResult represents search result
type QdrantSearchResult struct {
	ID      string
	Score   float32
	Payload map[string]any
}

// NewBuildKnowledgeIndexTool creates a new build knowledge index tool
func NewBuildKnowledgeIndexTool(embeddingClient EmbeddingClient, qdrantClient QdrantClient, collectionID string) *BuildKnowledgeIndexTool {
	return &BuildKnowledgeIndexTool{
		embeddingClient: embeddingClient,
		qdrantClient:   qdrantClient,
		collectionID:   collectionID,
	}
}

// Execute builds vector index
func (t *BuildKnowledgeIndexTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	documents, ok := params["documents"].([]any)
	if !ok {
		return nil, fmt.Errorf("documents parameter required")
	}

	// Step 1: Extract text from documents
	texts := make([]string, 0, len(documents))
	for _, doc := range documents {
		if m, ok := doc.(map[string]any); ok {
			if text, ok := m["text"].(string); ok {
				texts = append(texts, text)
			} else if content, ok := m["content"].(string); ok {
				texts = append(texts, content)
			}
		}
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no text content found in documents")
	}

	// Step 2: Generate embeddings
	embeddings, err := t.embeddingClient.Embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Step 3: Create Qdrant points
	points := make([]QdrantPoint, 0, len(embeddings))
	for i, embedding := range embeddings {
		points = append(points, QdrantPoint{
			ID:     fmt.Sprintf("doc-%d-%d", i, time.Now().UnixNano()),
			Vector: embedding,
			Payload: map[string]any{
				"document":    documents[i],
				"text":        texts[i],
				"indexed_at":  time.Now().Format(time.RFC3339),
			},
		})
	}

	// Step 4: Upsert to Qdrant
	if err := t.qdrantClient.Upsert(ctx, t.collectionID, points); err != nil {
		return nil, fmt.Errorf("failed to upsert to Qdrant: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"collection_id":    t.collectionID,
			"indexed_count":    len(points),
			"vector_dimension": len(embeddings[0]),
		},
	}, nil
}

// RAGQueryTool queries RAG knowledge base
type RAGQueryTool struct {
	embeddingClient EmbeddingClient
	qdrantClient   QdrantClient
	llmProvider    LLMProvider
}

// NewRAGQueryTool creates a new RAG query tool
func NewRAGQueryTool(embeddingClient EmbeddingClient, qdrantClient QdrantClient, llmProvider LLMProvider) *RAGQueryTool {
	return &RAGQueryTool{
		embeddingClient: embeddingClient,
		qdrantClient:   qdrantClient,
		llmProvider:    llmProvider,
	}
}

// Execute queries RAG knowledge base
func (t *RAGQueryTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	collection, ok := params["collection"].(string)
	if !ok {
		return nil, fmt.Errorf("collection parameter required")
	}

	topK := 5
	if val, ok := params["top_k"].(int); ok {
		topK = val
	}

	// Step 1: Generate query embedding
	embeddings, err := t.embeddingClient.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Step 2: Search Qdrant
	results, err := t.qdrantClient.Search(ctx, collection, embeddings[0], topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Step 3: Build context from results
	var contextBuilder strings.Builder
	for _, result := range results {
		if content, ok := result.Payload["content"].(string); ok {
			contextBuilder.WriteString(content)
			contextBuilder.WriteString("\n\n")
		}
	}

	// Step 4: Generate answer with LLM
	if t.llmProvider != nil {
		// TODO: Call LLM with context
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"answer":       "基于知识库的回答...",
				"sources":      results,
				"context_used": contextBuilder.String(),
			},
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"context": contextBuilder.String(),
			"sources": results,
		},
	}, nil
}
