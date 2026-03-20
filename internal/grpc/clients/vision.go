package clients

import (
	"context"

	pb "github.com/utaaa/uta-travel-agent/internal/gen/go/agent/vision"
	"google.golang.org/grpc"
)

// VisionClient wraps the gRPC Vision service client
type VisionClient struct {
	client pb.VisionServiceClient
	conn   *grpc.ClientConn
}

// NewVisionClient creates a new Vision client
func NewVisionClient(conn *grpc.ClientConn) *VisionClient {
	return &VisionClient{
		client: pb.NewVisionServiceClient(conn),
		conn:   conn,
	}
}

// AnalyzeRequest for image analysis
type AnalyzeRequest struct {
	ImageData []byte
	MediaType string
	Prompt    string
}

// AnalyzeResponse from image analysis
type AnalyzeResponse struct {
	Description  string
	Model        string
	InputTokens  int32
	OutputTokens int32
}

// AnalyzeImage analyzes an image with a custom prompt
func (c *VisionClient) AnalyzeImage(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	protoReq := &pb.AnalyzeRequest{
		ImageData: req.ImageData,
		MediaType: req.MediaType,
		Prompt:    req.Prompt,
	}

	resp, err := c.client.AnalyzeImage(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	return &AnalyzeResponse{
		Description:  resp.Description,
		Model:        resp.Model,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}, nil
}

// LandmarkInfo contains information about a recognized landmark
type LandmarkInfo struct {
	Name              string
	Confidence        float64
	Description       string
	Category          string
	HistoricalPeriod  string
}

// RecognizeLandmarkRequest for landmark recognition
type RecognizeLandmarkRequest struct {
	ImageData   []byte
	MediaType   string
	Destination string
	Language    string
}

// RecognizeLandmarkResponse from landmark recognition
type RecognizeLandmarkResponse struct {
	Recognized bool
	Landmark   *LandmarkInfo
	RawAnalysis string
	Model       string
}

// RecognizeLandmark recognizes a landmark in an image
func (c *VisionClient) RecognizeLandmark(ctx context.Context, req RecognizeLandmarkRequest) (*RecognizeLandmarkResponse, error) {
	protoReq := &pb.RecognizeLandmarkRequest{
		ImageData: req.ImageData,
		MediaType: req.MediaType,
		Language:  req.Language,
	}

	if req.Destination != "" {
		protoReq.Destination = &req.Destination
	}

	resp, err := c.client.RecognizeLandmark(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	result := &RecognizeLandmarkResponse{
		Recognized:  resp.Recognized,
		RawAnalysis: resp.RawAnalysis,
		Model:       resp.Model,
	}

	if resp.Landmark != nil {
		result.Landmark = &LandmarkInfo{
			Name:             resp.Landmark.Name,
			Confidence:       float64(resp.Landmark.Confidence),
			Description:      resp.Landmark.Description,
			Category:         resp.Landmark.GetCategory(),
			HistoricalPeriod: resp.Landmark.GetHistoricalPeriod(),
		}
	}

	return result, nil
}

// HealthCheck checks the service health
func (c *VisionClient) HealthCheck(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":             resp.Status,
		"model":              resp.Model,
		"api_key_configured": resp.ApiKeyConfigured,
		"supported_formats":  resp.SupportedFormats,
	}, nil
}