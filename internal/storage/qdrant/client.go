// Package qdrant provides Qdrant vector database operations
package qdrant

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds Qdrant configuration
type Config struct {
	Host string
	Port int
}

// Client wraps Qdrant gRPC connection
type Client struct {
	conn    *grpc.ClientConn
	client  qdrant.CollectionsClient
	points  qdrant.PointsClient
	host    string
	port    int
}

// NewClient creates a new Qdrant client
func NewClient(cfg Config) (*Client, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 6334 // Default gRPC port
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant at %s: %w", addr, err)
	}

	return &Client{
		conn:    conn,
		client:  qdrant.NewCollectionsClient(conn),
		points:  qdrant.NewPointsClient(conn),
		host:    cfg.Host,
		port:    cfg.Port,
	}, nil
}

// CollectionConfig holds collection creation parameters
type CollectionConfig struct {
	Name          string
	VectorSize    uint64
	Distance      string // "Cosine", "Euclid", "Dot"
	OnDisk        bool
	SparseVectors bool
}

// CreateCollection creates a new vector collection
func (c *Client) CreateCollection(ctx context.Context, cfg CollectionConfig) error {
	if cfg.VectorSize == 0 {
		return fmt.Errorf("vector size must be greater than 0")
	}

	distance := qdrant.Distance_Cosine
	switch cfg.Distance {
	case "Euclid", "euclid":
		distance = qdrant.Distance_Euclid
	case "Dot", "dot":
		distance = qdrant.Distance_Dot
	}

	req := &qdrant.CreateCollection{
		CollectionName: cfg.Name,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     cfg.VectorSize,
					Distance: distance,
					OnDisk:   &cfg.OnDisk,
				},
			},
		},
	}

	_, err := c.client.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", cfg.Name, err)
	}

	return nil
}

// DeleteCollection deletes a vector collection
func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	req := &qdrant.DeleteCollection{
		CollectionName: name,
	}

	_, err := c.client.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete collection %s: %w", name, err)
	}

	return nil
}

// ListCollections lists all collections
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	resp, err := c.client.List(ctx, &qdrant.ListCollectionsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	names := make([]string, len(resp.GetCollections()))
	for i, col := range resp.GetCollections() {
		names[i] = col.GetName()
	}

	return names, nil
}

// CollectionInfo returns information about a collection
func (c *Client) CollectionInfo(ctx context.Context, name string) (*CollectionInfo, error) {
	resp, err := c.client.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	result := resp.GetResult()
	if result == nil {
		return nil, fmt.Errorf("collection %s not found", name)
	}

	// Try to get vector size from config
	var vectorSize uint64
	config := result.GetConfig()
	if config != nil {
		params := config.GetParams()
		if params != nil {
			vectorsConfig := params.GetVectorsConfig()
			if vectorsConfig != nil {
				vectorParams := vectorsConfig.GetParams()
				if vectorParams != nil {
					vectorSize = vectorParams.GetSize()
				}
			}
		}
	}

	return &CollectionInfo{
		Name:       name,
		VectorSize: vectorSize,
		PointCount: result.GetPointsCount(),
		Status:     result.GetStatus().String(),
	}, nil
}

// CollectionInfo holds collection metadata
type CollectionInfo struct {
	Name       string
	VectorSize uint64
	PointCount uint64
	Status     string
}

// Point represents a vector point with metadata
type Point struct {
	ID       string
	Vector   []float32
	Payload  map[string]interface{}
}

// Upsert inserts or updates points in a collection
func (c *Client) Upsert(ctx context.Context, collection string, points []Point) error {
	if len(points) == 0 {
		return nil
	}

	qdrantPoints := make([]*qdrant.PointStruct, len(points))
	for i, p := range points {
		// Convert ID to valid UUID format (use MD5 hash for deterministic UUID)
		uuid := toUUID(p.ID)

		// Store original ID in payload for reference
		payload := p.Payload
		if payload == nil {
			payload = make(map[string]interface{})
		}
		payload["_id"] = p.ID

		qdrantPoints[i] = &qdrant.PointStruct{
			Id: &qdrant.PointId{
				PointIdOptions: &qdrant.PointId_Uuid{
					Uuid: uuid,
				},
			},
			Vectors: &qdrant.Vectors{
				VectorsOptions: &qdrant.Vectors_Vector{
					Vector: &qdrant.Vector{
						Data: p.Vector,
					},
				},
			},
			Payload: convertToPayload(payload),
		}
	}

	req := &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         qdrantPoints,
	}

	_, err := c.points.Upsert(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}

	return nil
}

// DeletePoints deletes points from a collection
func (c *Client) DeletePoints(ctx context.Context, collection string, ids []string) error {
	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Uuid{
				Uuid: toUUID(id),
			},
		}
	}

	req := &qdrant.DeletePoints{
		CollectionName: collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: pointIDs,
				},
			},
		},
	}

	_, err := c.points.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}

	return nil
}

// SearchResult represents a vector search result
type SearchResult struct {
	ID       string
	Score    float32
	Payload  map[string]interface{}
}

// Search performs vector similarity search
func (c *Client) Search(ctx context.Context, collection string, vector []float32, limit int) ([]SearchResult, error) {
	return c.SearchWithFilter(ctx, collection, vector, limit, nil)
}

// SearchWithFilter performs vector similarity search with optional filter
func (c *Client) SearchWithFilter(ctx context.Context, collection string, vector []float32, limit int, filter *qdrant.Filter) ([]SearchResult, error) {
	req := &qdrant.SearchPoints{
		CollectionName: collection,
		Vector:         vector,
		Limit:          uint64(limit),
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
	}

	if filter != nil {
		req.Filter = filter
	}

	resp, err := c.points.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search points: %w", err)
	}

	results := make([]SearchResult, len(resp.GetResult()))
	for i, hit := range resp.GetResult() {
		results[i] = SearchResult{
			ID:      hit.GetId().GetUuid(),
			Score:   hit.GetScore(),
			Payload: convertFromPayload(hit.GetPayload()),
		}
	}

	return results, nil
}

// Scroll retrieves points by scrolling through collection
func (c *Client) Scroll(ctx context.Context, collection string, limit int, offset string) ([]Point, string, error) {
	limitUint32 := uint32(limit)
	req := &qdrant.ScrollPoints{
		CollectionName: collection,
		Limit:          &limitUint32,
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
		WithVectors: &qdrant.WithVectorsSelector{
			SelectorOptions: &qdrant.WithVectorsSelector_Enable{
				Enable: true,
			},
		},
	}

	if offset != "" {
		req.Offset = &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Uuid{
				Uuid: offset,
			},
		}
	}

	resp, err := c.points.Scroll(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to scroll points: %w", err)
	}

	points := make([]Point, len(resp.GetResult()))
	for i, p := range resp.GetResult() {
		points[i] = Point{
			ID:      p.GetId().GetUuid(),
			Vector:  p.GetVectors().GetVector().GetData(),
			Payload: convertFromPayload(p.GetPayload()),
		}
	}

	var nextPageOffset string
	if resp.GetNextPageOffset() != nil {
		nextPageOffset = resp.GetNextPageOffset().GetUuid()
	}

	return points, nextPageOffset, nil
}

// Close closes the Qdrant connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// convertToPayload converts map to Qdrant payload
func convertToPayload(m map[string]interface{}) map[string]*qdrant.Value {
	if m == nil {
		return nil
	}

	payload := make(map[string]*qdrant.Value)
	for k, v := range m {
		payload[k] = convertValue(v)
	}
	return payload
}

// convertValue converts Go value to Qdrant Value
func convertValue(v interface{}) *qdrant.Value {
	switch val := v.(type) {
	case string:
		return &qdrant.Value{
			Kind: &qdrant.Value_StringValue{
				StringValue: val,
			},
		}
	case int:
		return &qdrant.Value{
			Kind: &qdrant.Value_IntegerValue{
				IntegerValue: int64(val),
			},
		}
	case int64:
		return &qdrant.Value{
			Kind: &qdrant.Value_IntegerValue{
				IntegerValue: val,
			},
		}
	case float64:
		return &qdrant.Value{
			Kind: &qdrant.Value_DoubleValue{
				DoubleValue: val,
			},
		}
	case float32:
		return &qdrant.Value{
			Kind: &qdrant.Value_DoubleValue{
				DoubleValue: float64(val),
			},
		}
	case bool:
		return &qdrant.Value{
			Kind: &qdrant.Value_BoolValue{
				BoolValue: val,
			},
		}
	case []string:
		values := make([]*qdrant.Value, len(val))
		for i, s := range val {
			values[i] = convertValue(s)
		}
		return &qdrant.Value{
			Kind: &qdrant.Value_ListValue{
				ListValue: &qdrant.ListValue{
					Values: values,
				},
			},
		}
	default:
		return &qdrant.Value{
			Kind: &qdrant.Value_StringValue{
				StringValue: fmt.Sprintf("%v", v),
			},
		}
	}
}

// convertFromPayload converts Qdrant payload to map
func convertFromPayload(payload map[string]*qdrant.Value) map[string]interface{} {
	if payload == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range payload {
		result[k] = extractValue(v)
	}
	return result
}

// extractValue extracts Go value from Qdrant Value
func extractValue(v *qdrant.Value) interface{} {
	if v == nil {
		return nil
	}

	switch kind := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return kind.StringValue
	case *qdrant.Value_IntegerValue:
		return kind.IntegerValue
	case *qdrant.Value_DoubleValue:
		return kind.DoubleValue
	case *qdrant.Value_BoolValue:
		return kind.BoolValue
	case *qdrant.Value_ListValue:
		values := make([]interface{}, len(kind.ListValue.Values))
		for i, val := range kind.ListValue.Values {
			values[i] = extractValue(val)
		}
		return values
	default:
		return nil
	}
}

// toUUID converts any string to a valid UUID v4 format
// Uses MD5 hash for deterministic conversion
func toUUID(id string) string {
	// If already a valid UUID format, return as-is
	if len(id) == 36 && id[8] == '-' && id[13] == '-' && id[18] == '-' && id[23] == '-' {
		return id
	}

	// Generate MD5 hash and format as UUID
	hash := md5.Sum([]byte(id))
	hexStr := hex.EncodeToString(hash[:])

	// Format as UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexStr[0:8],
		hexStr[8:12],
		hexStr[12:16],
		hexStr[16:20],
		hexStr[20:32],
	)
}