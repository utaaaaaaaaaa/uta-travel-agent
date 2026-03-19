package clients

import (
	"context"

	pb "github.com/utaaa/uta-travel-agent/internal/gen/go/agent"
	"google.golang.org/grpc"
)

// EmbeddingClient wraps the gRPC Embedding service client
type EmbeddingClient struct {
	client pb.EmbeddingServiceClient
	conn   *grpc.ClientConn
}

// NewEmbeddingClient creates a new Embedding client
func NewEmbeddingClient(conn *grpc.ClientConn) *EmbeddingClient {
	return &EmbeddingClient{
		client: pb.NewEmbeddingServiceClient(conn),
		conn:   conn,
	}
}

// EmbedRequest parameters
type EmbedRequest struct {
	Texts       []string
	UseCache    bool
	ModelPreset string
}

// EmbedResponse from embedding
type EmbedResponse struct {
	Embeddings  [][]float32
	Model       string
	Dimension   int32
	CachedCount int32
}

// Embed creates embeddings for texts
func (c *EmbeddingClient) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	protoReq := &pb.EmbedRequest{
		Texts:    req.Texts,
		UseCache: req.UseCache,
	}

	if req.ModelPreset != "" {
		protoReq.ModelPreset = &req.ModelPreset
	}

	resp, err := c.client.Embed(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		embeddings[i] = e.Values
	}

	return &EmbedResponse{
		Embeddings:  embeddings,
		Model:       resp.Model,
		Dimension:   resp.Dimension,
		CachedCount: resp.CachedCount,
	}, nil
}

// BatchEmbed creates embeddings for a large batch
func (c *EmbeddingClient) BatchEmbed(ctx context.Context, texts []string, modelPreset string) (*EmbedResponse, error) {
	protoReq := &pb.BatchEmbedRequest{
		Texts: texts,
	}

	if modelPreset != "" {
		protoReq.ModelPreset = &modelPreset
	}

	resp, err := c.client.BatchEmbed(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		embeddings[i] = e.Values
	}

	return &EmbedResponse{
		Embeddings:  embeddings,
		Model:       resp.Model,
		Dimension:   resp.Dimension,
		CachedCount: resp.CachedCount,
	}, nil
}

// ModelInfo returns model information
func (c *EmbeddingClient) ModelInfo(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.GetModelInfo(ctx, &pb.ModelInfoRequest{})
	if err != nil {
		return nil, err
	}

	presets := make(map[string]string)
	for k, v := range resp.Presets {
		presets[k] = v
	}

	return map[string]interface{}{
		"model":     resp.Model,
		"dimension": resp.Dimension,
		"presets":   presets,
	}, nil
}

// ClearCache clears the embedding cache
func (c *EmbeddingClient) ClearCache(ctx context.Context) (int32, error) {
	resp, err := c.client.ClearCache(ctx, &pb.ClearCacheRequest{})
	if err != nil {
		return 0, err
	}

	return resp.Cleared, nil
}

// HealthCheck checks the service health
func (c *EmbeddingClient) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":     resp.Status,
		"model":      resp.Model,
		"dimension":  resp.Dimension,
		"cache_size": resp.CacheSize,
	}, nil
}