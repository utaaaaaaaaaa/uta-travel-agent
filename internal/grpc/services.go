package grpc

import (
	"context"

	"github.com/utaaa/uta-travel-agent/internal/grpc/clients"
)

// PythonServices provides access to all Python service clients
type PythonServices struct {
	LLM       *clients.LLMClient
	Embedding *clients.EmbeddingClient
	Vision    *clients.VisionClient
}

// ServiceClients holds all gRPC clients for Python services
type ServiceClients struct {
	llm       *clients.LLMClient
	embedding *clients.EmbeddingClient
	vision    *clients.VisionClient
}

// NewServiceClients creates new service clients from a client manager
func NewServiceClients(manager *ClientManager) *ServiceClients {
	return &ServiceClients{
		llm:       clients.NewLLMClient(manager.GetLLMConn()),
		embedding: clients.NewEmbeddingClient(manager.GetEmbeddingConn()),
		vision:    clients.NewVisionClient(manager.GetVisionConn()),
	}
}

// LLM returns the LLM service client
func (c *ServiceClients) LLM() *clients.LLMClient {
	return c.llm
}

// Embedding returns the Embedding service client
func (c *ServiceClients) Embedding() *clients.EmbeddingClient {
	return c.embedding
}

// Vision returns the Vision service client
func (c *ServiceClients) Vision() *clients.VisionClient {
	return c.vision
}

// HealthCheckAll checks health of all services
func (c *ServiceClients) HealthCheckAll(ctx context.Context) (map[string]interface{}, error) {
	results := make(map[string]interface{})

	// Check LLM
	llmHealth, err := c.llm.HealthCheck(ctx)
	if err != nil {
		results["llm"] = map[string]interface{}{"error": err.Error()}
	} else {
		results["llm"] = llmHealth
	}

	// Check Embedding
	embeddingHealth, err := c.embedding.HealthCheck(ctx)
	if err != nil {
		results["embedding"] = map[string]interface{}{"error": err.Error()}
	} else {
		results["embedding"] = embeddingHealth
	}

	// Check Vision
	visionHealth, err := c.vision.HealthCheck(ctx)
	if err != nil {
		results["vision"] = map[string]interface{}{"error": err.Error()}
	} else {
		results["vision"] = visionHealth
	}

	return results, nil
}

// EmbedText is a helper to create embeddings for text
func (c *ServiceClients) EmbedText(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.embedding.Embed(ctx, clients.EmbedRequest{
		Texts:    []string{text},
		UseCache: true,
	})
	if err != nil {
		return nil, err
	}
	return resp.Embeddings[0], nil
}

// EmbedTexts is a helper to create embeddings for multiple texts
func (c *ServiceClients) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.embedding.Embed(ctx, clients.EmbedRequest{
		Texts:    texts,
		UseCache: true,
	})
	if err != nil {
		return nil, err
	}
	return resp.Embeddings, nil
}

// Chat is a helper for simple chat completion
func (c *ServiceClients) Chat(ctx context.Context, messages []clients.ChatMessage, system string) (string, error) {
	resp, err := c.llm.Complete(ctx, clients.ChatRequest{
		Messages: messages,
		System:   system,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// RAGQuery is a helper for RAG queries
func (c *ServiceClients) RAGQuery(ctx context.Context, query, context string) (string, error) {
	resp, err := c.llm.RAGQuery(ctx, clients.RAGRequest{
		Query:    query,
		Context:  context,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// AnalyzeImage is a helper for image analysis
func (c *ServiceClients) AnalyzeImage(ctx context.Context, imageData []byte, mediaType, prompt string) (string, error) {
	resp, err := c.llm.AnalyzeImage(ctx, clients.AnalyzeImageRequest{
		ImageData: imageData,
		MediaType: mediaType,
		Prompt:    prompt,
	})
	if err != nil {
		return "", err
	}
	return resp.Description, nil
}

// RecognizeLandmark is a helper for landmark recognition
func (c *ServiceClients) RecognizeLandmark(ctx context.Context, imageData []byte, mediaType string) (*clients.RecognizeLandmarkResponse, error) {
	return c.vision.RecognizeLandmark(ctx, clients.RecognizeLandmarkRequest{
		ImageData: imageData,
		MediaType: mediaType,
		Language:  "zh",
	})
}