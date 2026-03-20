package clients

import (
	"context"

	pb "github.com/utaaa/uta-travel-agent/internal/gen/go/agent/llm"
	"google.golang.org/grpc"
)

// LLMClient wraps the gRPC LLM service client
type LLMClient struct {
	client pb.LLMServiceClient
	conn   *grpc.ClientConn
}

// NewLLMClient creates a new LLM client
func NewLLMClient(conn *grpc.ClientConn) *LLMClient {
	return &LLMClient{
		client: pb.NewLLMServiceClient(conn),
		conn:   conn,
	}
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string
	Content string
}

// ChatRequest parameters
type ChatRequest struct {
	Messages    []ChatMessage
	Model       string
	System      string
	MaxTokens   int32
	Temperature float64
}

// ChatResponse from completion
type ChatResponse struct {
	Content    string
	Model      string
	InputTokens  int32
	OutputTokens int32
	StopReason string
}

// Complete generates a chat completion
func (c *LLMClient) Complete(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := make([]*pb.ChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = &pb.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	protoReq := &pb.ChatRequest{
		Messages:    messages,
		Temperature: req.Temperature,
	}

	if req.Model != "" {
		protoReq.Model = &req.Model
	}
	if req.System != "" {
		protoReq.System = &req.System
	}
	if req.MaxTokens > 0 {
		protoReq.MaxTokens = &req.MaxTokens
	}

	resp, err := c.client.Complete(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Content:      resp.Content,
		Model:        resp.Model,
		InputTokens:  resp.Usage.GetInputTokens(),
		OutputTokens: resp.Usage.GetOutputTokens(),
		StopReason:   resp.StopReason,
	}, nil
}

// RAGRequest for RAG-enhanced queries
type RAGRequest struct {
	Query        string
	Context      string
	SystemPrompt string
	Model        string
	Temperature  float64
}

// RAGQuery executes a RAG-enhanced query
func (c *LLMClient) RAGQuery(ctx context.Context, req RAGRequest) (*ChatResponse, error) {
	protoReq := &pb.RAGRequest{
		Query:       req.Query,
		Context:     req.Context,
		Temperature: req.Temperature,
	}

	if req.SystemPrompt != "" {
		protoReq.SystemPrompt = &req.SystemPrompt
	}
	if req.Model != "" {
		protoReq.Model = &req.Model
	}

	resp, err := c.client.RAGQuery(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Content:      resp.Content,
		Model:        resp.Model,
		InputTokens:  resp.Usage.GetInputTokens(),
		OutputTokens: resp.Usage.GetOutputTokens(),
		StopReason:   resp.StopReason,
	}, nil
}

// AnalyzeImageRequest for image analysis
type AnalyzeImageRequest struct {
	ImageData []byte
	MediaType string
	Prompt    string
	Model     string
}

// AnalyzeImageResponse from image analysis
type AnalyzeImageResponse struct {
	Description  string
	Model        string
	InputTokens  int32
	OutputTokens int32
}

// AnalyzeImage analyzes an image using vision
func (c *LLMClient) AnalyzeImage(ctx context.Context, req AnalyzeImageRequest) (*AnalyzeImageResponse, error) {
	protoReq := &pb.AnalyzeImageRequest{
		ImageData: req.ImageData,
		MediaType: req.MediaType,
		Prompt:    req.Prompt,
	}

	if req.Model != "" {
		protoReq.Model = &req.Model
	}

	resp, err := c.client.AnalyzeImage(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &AnalyzeImageResponse{
		Description:  resp.Description,
		Model:        resp.Model,
		InputTokens:  resp.Usage.GetInputTokens(),
		OutputTokens: resp.Usage.GetOutputTokens(),
	}, nil
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Content string
	Done    bool
}

// Stream generates a streaming chat completion
func (c *LLMClient) Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, <-chan error) {
	chunkCh := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		messages := make([]*pb.ChatMessage, len(req.Messages))
		for i, m := range req.Messages {
			messages[i] = &pb.ChatMessage{
				Role:    m.Role,
				Content: m.Content,
			}
		}

		protoReq := &pb.ChatRequest{
			Messages:    messages,
			Temperature: req.Temperature,
		}

		if req.Model != "" {
			protoReq.Model = &req.Model
		}
		if req.System != "" {
			protoReq.System = &req.System
		}
		if req.MaxTokens > 0 {
			protoReq.MaxTokens = &req.MaxTokens
		}

		stream, err := c.client.Stream(ctx, protoReq)
		if err != nil {
			errCh <- err
			return
		}

		for {
			chunk, err := stream.Recv()
			if err != nil {
				return
			}

			chunkCh <- StreamChunk{
				Content: chunk.Content,
				Done:    chunk.Done,
			}

			if chunk.Done {
				return
			}
		}
	}()

	return chunkCh, errCh
}

// HealthCheck checks the service health
func (c *LLMClient) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":            resp.Status,
		"model":             resp.Model,
		"api_key_configured": resp.ApiKeyConfigured,
	}, nil
}