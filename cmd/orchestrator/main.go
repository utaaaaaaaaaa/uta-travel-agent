package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/agent"
	"github.com/utaaa/uta-travel-agent/internal/grpc/clients"
	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/rag"
	"github.com/utaaa/uta-travel-agent/internal/router"
	"github.com/utaaa/uta-travel-agent/internal/scheduler"
	"github.com/utaaa/uta-travel-agent/internal/storage/postgres"
	"github.com/utaaa/uta-travel-agent/internal/storage/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log.Println("Starting UTA Travel Agent Orchestrator...")

	// Load configuration
	cfg := Load()

	// Initialize PostgreSQL database
	var pgClient *postgres.Client
	var agentRepo *postgres.AgentRepository

	if cfg.DatabaseURL != "" || cfg.DatabaseHost != "" {
		var err error
		pgClient, err = postgres.NewClient(postgres.Config{
			Host:     cfg.DatabaseHost,
			Port:     cfg.DatabasePort,
			User:     cfg.DatabaseUser,
			Password: cfg.DatabasePass,
			Database: cfg.DatabaseName,
			SSLMode:  cfg.DatabaseSSL,
		})
		if err != nil {
			log.Printf("Warning: Failed to connect to PostgreSQL: %v", err)
			log.Println("Agent persistence will not be available")
		} else {
			log.Printf("Connected to PostgreSQL at %s:%d", cfg.DatabaseHost, cfg.DatabasePort)
			agentRepo = postgres.NewAgentRepository(pgClient.DB())
		}
	}

	// Initialize embedding gRPC client
	var embeddingClient *clients.EmbeddingClient
	if cfg.EmbeddingAddr != "" {
		conn, err := grpc.NewClient(
			cfg.EmbeddingAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("Warning: Failed to connect to embedding service at %s: %v", cfg.EmbeddingAddr, err)
			log.Println("RAG and Indexer tools will not be available")
		} else {
			embeddingClient = clients.NewEmbeddingClient(conn)
			log.Printf("Connected to embedding service at %s", cfg.EmbeddingAddr)
		}
	}

	// Initialize Qdrant client
	var qdrantClient *qdrant.Client
	if cfg.QdrantHost != "" {
		var err error
		qdrantClient, err = qdrant.NewClient(qdrant.Config{
			Host: cfg.QdrantHost,
			Port: cfg.QdrantPort,
		})
		if err != nil {
			log.Printf("Warning: Failed to connect to Qdrant at %s:%d: %v", cfg.QdrantHost, cfg.QdrantPort, err)
			log.Println("Vector storage will not be available")
		} else {
			log.Printf("Connected to Qdrant at %s:%d", cfg.QdrantHost, cfg.QdrantPort)
		}
	}

	// Initialize LLM provider
	var llmProvider llm.Provider
	if cfg.LLMAPIKey != "" {
		llmProvider = llm.NewGLMProvider(llm.GLMConfig{
			APIKey:  cfg.LLMAPIKey,
			BaseURL: cfg.LLMBBaseURL,
			Model:   cfg.LLMModel,
		})
		log.Printf("LLM provider initialized with model: %s", cfg.LLMModel)
	} else {
		// Use mock provider if no API key
		llmProvider = llm.NewMockProvider("这是一个模拟回复。请配置 LLM_API_KEY 以启用真实的 AI 功能。")
		log.Println("Warning: No LLM API key provided, using mock provider")
	}

	// Initialize RAG service
	var ragSvc *rag.Service
	if embeddingClient != nil && qdrantClient != nil {
		ragSvc = rag.NewService(rag.Config{
			QdrantClient:    qdrantClient,
			LLMProvider:     llmProvider,
			EmbeddingClient: embeddingClient,
		})
		log.Println("RAG service initialized")
	} else {
		log.Println("Warning: RAG service not available (missing embedding or Qdrant client)")
	}

	// Initialize tool registry
	toolRegistry := agent.NewToolRegistry()

	// Register tools based on available services
	if embeddingClient != nil && qdrantClient != nil {
		// Build Knowledge Index Tool - wraps the gRPC client to ToolExecutor interface
		buildIndexTool := &BuildKnowledgeIndexToolAdapter{
			embeddingClient: embeddingClient,
			qdrantClient:    qdrantClient,
		}
		toolRegistry.Register(agent.Tool{
			Name:        "build_knowledge_index",
			Type:        agent.ToolTypeService,
			Description: "Builds vector index from documents for RAG queries",
			Parameters: map[string]any{
				"documents": map[string]any{
					"type":        "array",
					"description": "Array of documents to index",
				},
				"collection_id": map[string]any{
					"type":        "string",
					"description": "Collection ID for the vector index",
				},
			},
			Required: true,
		}, buildIndexTool)

		// RAG Query Tool
		ragQueryTool := &RAGQueryToolAdapter{
			embeddingClient: embeddingClient,
			qdrantClient:    qdrantClient,
			llmProvider:     llmProvider,
		}
		toolRegistry.Register(agent.Tool{
			Name:        "rag_query",
			Type:        agent.ToolTypeService,
			Description: "Queries the RAG knowledge base for relevant information",
			Parameters: map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The query to search for",
				},
				"collection": map[string]any{
					"type":        "string",
					"description": "The collection to search in",
				},
				"top_k": map[string]any{
					"type":        "integer",
					"description": "Number of results to return",
				},
			},
			Required: true,
		}, ragQueryTool)

		log.Println("Indexer tools registered")
	}

	// Register mock tools for research and web reading
	braveSearchTool := agent.NewBraveSearchTool("") // API key can be configured later
	toolRegistry.Register(agent.Tool{
		Name:        "brave_search",
		Type:        agent.ToolTypeMCP,
		Description: "Search the web for information",
		Parameters: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
		Required: true,
	}, braveSearchTool)

	// LLM Summarize Tool
	llmSummarizeTool := &LLMSummarizeToolAdapter{
		llmProvider: llmProvider,
	}
	toolRegistry.Register(agent.Tool{
		Name:        "llm_summarize",
		Type:        agent.ToolTypeSkill,
		Description: "Summarizes content using LLM",
		Parameters: map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "The content to summarize",
			},
		},
		Required: true,
	}, llmSummarizeTool)

	log.Println("Tool registry initialized with", len(toolRegistry.ListTools()), "tools")

	// Initialize template registry
	templateRegistry := agent.NewTemplateRegistry()

	// Register default templates for each agent type
	templateRegistry.Register(agent.AgentTypeMain, &agent.AgentTemplate{
		Kind:       "Agent",
		APIVersion: "v1",
		Metadata: agent.TemplateMetadata{
			Name:        "main",
			Version:     "1.0",
			Description: "Main orchestrating agent",
		},
		Spec: agent.TemplateSpec{
			Role:         agent.GetSubagentPrompt(agent.AgentTypeMain),
			Capabilities: []string{"orchestration", "task_routing", "progress_tracking"},
			AvailableSubagents: []string{"researcher", "curator", "indexer", "guide", "planner"},
			Decision: agent.DecisionConfig{
				Model:         cfg.LLMModel,
				Temperature:   0.7,
				MaxIterations: 20,
			},
		},
	})

	templateRegistry.Register(agent.AgentTypeResearcher, &agent.AgentTemplate{
		Kind:       "Agent",
		APIVersion: "v1",
		Metadata: agent.TemplateMetadata{
			Name:        "researcher",
			Version:     "1.0",
			Description: "Information research agent",
		},
		Spec: agent.TemplateSpec{
			Role:         agent.GetSubagentPrompt(agent.AgentTypeResearcher),
			Capabilities: []string{"web_search", "content_extraction", "data_collection"},
			Decision: agent.DecisionConfig{
				Model:         cfg.LLMModel,
				Temperature:   0.5,
				MaxIterations: 10,
			},
		},
	})

	templateRegistry.Register(agent.AgentTypeCurator, &agent.AgentTemplate{
		Kind:       "Agent",
		APIVersion: "v1",
		Metadata: agent.TemplateMetadata{
			Name:        "curator",
			Version:     "1.0",
			Description: "Content curation agent",
		},
		Spec: agent.TemplateSpec{
			Role:         agent.GetSubagentPrompt(agent.AgentTypeCurator),
			Capabilities: []string{"content_filtering", "summarization", "knowledge_organization"},
			Decision: agent.DecisionConfig{
				Model:         cfg.LLMModel,
				Temperature:   0.3,
				MaxIterations: 10,
			},
		},
	})

	templateRegistry.Register(agent.AgentTypeIndexer, &agent.AgentTemplate{
		Kind:       "Agent",
		APIVersion: "v1",
		Metadata: agent.TemplateMetadata{
			Name:        "indexer",
			Version:     "1.0",
			Description: "Knowledge indexing agent",
		},
		Spec: agent.TemplateSpec{
			Role:         agent.GetSubagentPrompt(agent.AgentTypeIndexer),
			Capabilities: []string{"text_chunking", "vector_indexing", "knowledge_storage"},
			Decision: agent.DecisionConfig{
				Model:         cfg.LLMModel,
				Temperature:   0.2,
				MaxIterations: 10,
			},
		},
	})

	templateRegistry.Register(agent.AgentTypeGuide, &agent.AgentTemplate{
		Kind:       "Agent",
		APIVersion: "v1",
		Metadata: agent.TemplateMetadata{
			Name:        "guide",
			Version:     "1.0",
			Description: "Real-time tour guide agent",
		},
		Spec: agent.TemplateSpec{
			Role:         agent.GetSubagentPrompt(agent.AgentTypeGuide),
			Capabilities: []string{"rag_query", "streaming_response", "location_awareness"},
			Decision: agent.DecisionConfig{
				Model:         cfg.LLMModel,
				Temperature:   0.7,
				MaxIterations: 5,
			},
		},
	})

	templateRegistry.Register(agent.AgentTypePlanner, &agent.AgentTemplate{
		Kind:       "Agent",
		APIVersion: "v1",
		Metadata: agent.TemplateMetadata{
			Name:        "planner",
			Version:     "1.0",
			Description: "Trip planning agent",
		},
		Spec: agent.TemplateSpec{
			Role:         agent.GetSubagentPrompt(agent.AgentTypePlanner),
			Capabilities: []string{"itinerary_generation", "time_optimization", "budget_planning"},
			Decision: agent.DecisionConfig{
				Model:         cfg.LLMModel,
				Temperature:   0.5,
				MaxIterations: 10,
			},
		},
	})

	log.Println("Template registry initialized with", len(templateRegistry.List()), "templates")

	// Initialize agent registry (with persistence if available)
	var registry *agent.Registry
	if agentRepo != nil {
		registry = agent.NewRegistryWithRepo(agentRepo)
		// Load existing agents from database
		ctx := context.Background()
		if err := registry.LoadFromRepository(ctx, ""); err != nil {
			log.Printf("Warning: Failed to load agents from database: %v", err)
		} else {
			log.Printf("Loaded %d agents from database", len(registry.List()))
		}
	} else {
		registry = agent.NewRegistry()
		log.Println("Using in-memory registry (no persistence)")
	}

	// Initialize agent factory (will be used for agent creation)
	_ = agent.NewAgentFactory(templateRegistry, toolRegistry, llmProvider)
	log.Println("Agent factory initialized")

	// Initialize scheduler
	sched := scheduler.NewScheduler(registry)

	// Initialize router with all services
	r := router.NewRouter(router.RouterConfig{
		Registry:  registry,
		Scheduler: sched,
		LLMClient: llmProvider,
		RAGSvc:    ragSvc,
	})

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.HTTPPort
	}

	log.Printf("Orchestrator initialized successfully")
	log.Printf("Starting HTTP server on port %s...", port)

	// Start the server
	if err := r.Start(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// BuildKnowledgeIndexToolAdapter wraps the gRPC clients to ToolExecutor interface
type BuildKnowledgeIndexToolAdapter struct {
	embeddingClient *clients.EmbeddingClient
	qdrantClient    *qdrant.Client
}

func (a *BuildKnowledgeIndexToolAdapter) Execute(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
	documents, ok := params["documents"].([]any)
	if !ok {
		return nil, errors.New("documents parameter required")
	}

	collectionID, ok := params["collection_id"].(string)
	if !ok {
		collectionID = fmt.Sprintf("collection-%d", time.Now().UnixNano())
	}

	// Extract text from documents
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
		return nil, errors.New("no text content found in documents")
	}

	// Generate embeddings via gRPC
	resp, err := a.embeddingClient.Embed(ctx, clients.EmbedRequest{
		Texts:    texts,
		UseCache: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Create collection if needed
	err = a.qdrantClient.CreateCollection(ctx, qdrant.CollectionConfig{
		Name:       collectionID,
		VectorSize: uint64(resp.Dimension),
		Distance:   "Cosine",
	})
	if err != nil {
		// Collection may already exist, continue
		log.Printf("Collection creation warning: %v", err)
	}

	// Build Qdrant points
	points := make([]qdrant.Point, 0, len(resp.Embeddings))
	for i, embedding := range resp.Embeddings {
		points = append(points, qdrant.Point{
			ID:      fmt.Sprintf("doc-%d-%d", i, time.Now().UnixNano()),
			Vector:  embedding,
			Payload: map[string]interface{}{"document": documents[i], "text": texts[i]},
		})
	}

	// Upsert to Qdrant
	if err := a.qdrantClient.Upsert(ctx, collectionID, points); err != nil {
		return nil, fmt.Errorf("failed to upsert to Qdrant: %w", err)
	}

	return &agent.ToolResult{
		Success: true,
		Data: map[string]any{
			"collection_id":    collectionID,
			"indexed_count":    len(points),
			"vector_dimension": resp.Dimension,
		},
	}, nil
}

// RAGQueryToolAdapter wraps the gRPC clients to ToolExecutor interface
type RAGQueryToolAdapter struct {
	embeddingClient *clients.EmbeddingClient
	qdrantClient    *qdrant.Client
	llmProvider     llm.Provider
}

func (a *RAGQueryToolAdapter) Execute(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, errors.New("query parameter required")
	}

	collection, ok := params["collection"].(string)
	if !ok {
		return nil, errors.New("collection parameter required")
	}

	topK := 5
	if val, ok := params["top_k"].(int); ok {
		topK = val
	}

	// Generate query embedding
	resp, err := a.embeddingClient.Embed(ctx, clients.EmbedRequest{
		Texts:    []string{query},
		UseCache: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search Qdrant
	results, err := a.qdrantClient.Search(ctx, collection, resp.Embeddings[0], topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Build context from results
	var contextBuilder strings.Builder
	for _, result := range results {
		if content, ok := result.Payload["content"].(string); ok {
			contextBuilder.WriteString(content)
			contextBuilder.WriteString("\n\n")
		} else if text, ok := result.Payload["text"].(string); ok {
			contextBuilder.WriteString(text)
			contextBuilder.WriteString("\n\n")
		}
	}

	return &agent.ToolResult{
		Success: true,
		Data: map[string]any{
			"query":        query,
			"context":      contextBuilder.String(),
			"sources":      results,
			"result_count": len(results),
		},
	}, nil
}

// LLMSummarizeToolAdapter wraps the LLM provider to ToolExecutor interface
type LLMSummarizeToolAdapter struct {
	llmProvider llm.Provider
}

func (a *LLMSummarizeToolAdapter) Execute(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
	content, ok := params["content"].(string)
	if !ok {
		return nil, errors.New("content parameter required")
	}

	systemPrompt := "You are a helpful assistant that summarizes content concisely and accurately."
	response, err := a.llmProvider.CompleteWithSystem(ctx, systemPrompt, []llm.Message{
		{Role: "user", Content: fmt.Sprintf("Please summarize the following content:\n\n%s", content)},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	return &agent.ToolResult{
		Success: true,
		Data: map[string]any{
			"summary":         response.Content,
			"original_length": len(content),
			"summary_length":  len(response.Content),
		},
	}, nil
}