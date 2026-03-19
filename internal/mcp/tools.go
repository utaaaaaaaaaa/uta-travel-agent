package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// BraveSearchTool implements the Brave Search MCP tool
type BraveSearchTool struct {
	apiKey     string
	client     *Client
	baseURL    string
}

// NewBraveSearchTool creates a new Brave Search tool
func NewBraveSearchTool(client *Client, apiKey string) *BraveSearchTool {
	return &BraveSearchTool{
		apiKey:  apiKey,
		client:  client,
		baseURL: "https://api.search.brave.com/res/v1/web/search",
	}
}

// Name returns the tool name
func (t *BraveSearchTool) Name() string {
	return "brave_search"
}

// BraveSearchParams defines search parameters
type BraveSearchParams struct {
	Query       string `json:"query"`
	Count       int    `json:"count,omitempty"`       // Number of results (default 20)
	Offset      int    `json:"offset,omitempty"`      // Pagination offset
	SearchLang  string `json:"search_lang,omitempty"` // Search language
	Country     string `json:"country,omitempty"`     // Country code
	Freshness   string `json:"freshness,omitempty"`   // pd, pw, pm, py
	TextDecorations bool `json:"text_decorations,omitempty"`
}

// BraveSearchResult represents a search result
type BraveSearchResult struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age,omitempty"`
	Language    string `json:"language,omitempty"`
}

// BraveSearchResponse represents the API response
type BraveSearchResponse struct {
	Query struct {
		Original string `json:"original"`
	} `json:"query"`
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Age         string `json:"age,omitempty"`
			Language    string `json:"language,omitempty"`
		} `json:"results"`
	} `json:"web"`
}

// Execute performs a Brave Search
func (t *BraveSearchTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s?q=%s", t.baseURL, url.QueryEscape(query))

	// Add optional parameters
	if count, ok := params["count"].(int); ok {
		reqURL += fmt.Sprintf("&count=%d", count)
	}
	if lang, ok := params["search_lang"].(string); ok {
		reqURL += fmt.Sprintf("&search_lang=%s", lang)
	}
	if country, ok := params["country"].(string); ok {
		reqURL += fmt.Sprintf("&country=%s", country)
	}

	// Create request
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	if t.apiKey != "" {
		req.Header.Set("X-Subscription-Token", t.apiKey)
	}

	// Execute request
	var response BraveSearchResponse
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}

	// Transform results
	var results []BraveSearchResult
	for _, r := range response.Web.Results {
		results = append(results, BraveSearchResult{
			Type:        "web",
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Description,
			Age:         r.Age,
			Language:    r.Language,
		})
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"query":   response.Query.Original,
			"results": results,
			"count":   len(results),
		},
	}, nil
}

// WebReaderTool implements web page reading
type WebReaderTool struct {
	client *Client
}

// NewWebReaderTool creates a new Web Reader tool
func NewWebReaderTool(client *Client) *WebReaderTool {
	return &WebReaderTool{
		client: client,
	}
}

// Name returns the tool name
func (t *WebReaderTool) Name() string {
	return "web_reader"
}

// Execute reads and parses a web page
func (t *WebReaderTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	urlStr, ok := params["url"].(string)
	if !ok || urlStr == "" {
		return nil, fmt.Errorf("url parameter is required")
	}

	// Validate URL
	_, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Create request
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	// Execute request
	body, err := t.client.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	// For now, return raw content
	// TODO: Implement proper HTML parsing and text extraction
	content := string(body)

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"url":     urlStr,
			"content": content,
			"size":    len(body),
		},
	}, nil
}

// MapsTool implements maps and geocoding functionality
type MapsTool struct {
	client  *Client
	apiKey  string
	baseURL string
}

// NewMapsTool creates a new Maps tool
func NewMapsTool(client *Client, apiKey string) *MapsTool {
	return &MapsTool{
		client:  client,
		apiKey:  apiKey,
		baseURL: "https://api.mapbox.com",
	}
}

// Name returns the tool name
func (t *MapsTool) Name() string {
	return "maps"
}

// Execute performs map operations
func (t *MapsTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action, _ := params["action"].(string)

	switch action {
	case "geocode", "search":
		return t.geocode(ctx, params)
	case "reverse":
		return t.reverseGeocode(ctx, params)
	case "directions":
		return t.directions(ctx, params)
	case "distance":
		return t.distance(ctx, params)
	default:
		return t.geocode(ctx, params) // Default to geocode
	}
}

func (t *MapsTool) geocode(ctx context.Context, params map[string]any) (*ToolResult, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	// Use Mapbox Geocoding API
	endpoint := fmt.Sprintf("%s/geocoding/v5/mapbox.places/%s.json", t.baseURL, url.QueryEscape(query))

	if t.apiKey != "" {
		endpoint += fmt.Sprintf("?access_token=%s", t.apiKey)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var response map[string]any
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("geocoding failed: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    response,
	}, nil
}

func (t *MapsTool) reverseGeocode(ctx context.Context, params map[string]any) (*ToolResult, error) {
	lng, _ := params["lng"].(float64)
	lat, _ := params["lat"].(float64)

	endpoint := fmt.Sprintf("%s/geocoding/v5/mapbox.places/%f,%f.json", t.baseURL, lng, lat)
	if t.apiKey != "" {
		endpoint += fmt.Sprintf("?access_token=%s", t.apiKey)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var response map[string]any
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("reverse geocoding failed: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    response,
	}, nil
}

func (t *MapsTool) directions(ctx context.Context, params map[string]any) (*ToolResult, error) {
	// TODO: Implement directions
	return &ToolResult{
		Success: false,
		Error:   "directions not yet implemented",
	}, nil
}

func (t *MapsTool) distance(ctx context.Context, params map[string]any) (*ToolResult, error) {
	// TODO: Implement distance matrix
	return &ToolResult{
		Success: false,
		Error:   "distance calculation not yet implemented",
	}, nil
}

// QdrantTool implements Qdrant vector database operations
type QdrantTool struct {
	client  *Client
	baseURL string
	apiKey  string
}

// NewQdrantTool creates a new Qdrant tool
func NewQdrantTool(client *Client, baseURL string, apiKey string) *QdrantTool {
	return &QdrantTool{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Name returns the tool name
func (t *QdrantTool) Name() string {
	return "qdrant"
}

// Execute performs Qdrant operations
func (t *QdrantTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action, _ := params["action"].(string)

	switch action {
	case "search":
		return t.search(ctx, params)
	case "upsert":
		return t.upsert(ctx, params)
	case "create_collection":
		return t.createCollection(ctx, params)
	case "delete_collection":
		return t.deleteCollection(ctx, params)
	case "list_collections":
		return t.listCollections(ctx)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *QdrantTool) search(ctx context.Context, params map[string]any) (*ToolResult, error) {
	collection, _ := params["collection"].(string)
	vector, _ := params["vector"].([]float64)
	limit, _ := params["limit"].(int)
	if limit == 0 {
		limit = 10
	}

	if collection == "" {
		return nil, fmt.Errorf("collection parameter is required")
	}
	if len(vector) == 0 {
		return nil, fmt.Errorf("vector parameter is required")
	}

	endpoint := fmt.Sprintf("%s/collections/%s/points/search", t.baseURL, collection)

	body := map[string]any{
		"vector": vector,
		"limit":  limit,
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("api-key", t.apiKey)
	}

	var response map[string]any
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    response,
	}, nil
}

func (t *QdrantTool) upsert(ctx context.Context, params map[string]any) (*ToolResult, error) {
	collection, _ := params["collection"].(string)
	points, _ := params["points"].([]any)

	if collection == "" || len(points) == 0 {
		return nil, fmt.Errorf("collection and points parameters are required")
	}

	endpoint := fmt.Sprintf("%s/collections/%s/points", t.baseURL, collection)

	body := map[string]any{
		"points": points,
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("PUT", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("api-key", t.apiKey)
	}

	var response map[string]any
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("upsert failed: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    response,
	}, nil
}

func (t *QdrantTool) createCollection(ctx context.Context, params map[string]any) (*ToolResult, error) {
	name, _ := params["name"].(string)
	vectorSize, _ := params["vector_size"].(int)
	if vectorSize == 0 {
		vectorSize = 1536 // Default to OpenAI embedding size
	}

	if name == "" {
		return nil, fmt.Errorf("name parameter is required")
	}

	endpoint := fmt.Sprintf("%s/collections/%s", t.baseURL, name)

	body := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("PUT", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("api-key", t.apiKey)
	}

	var response map[string]any
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("create collection failed: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    response,
	}, nil
}

func (t *QdrantTool) deleteCollection(ctx context.Context, params map[string]any) (*ToolResult, error) {
	name, _ := params["name"].(string)

	if name == "" {
		return nil, fmt.Errorf("name parameter is required")
	}

	endpoint := fmt.Sprintf("%s/collections/%s", t.baseURL, name)

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if t.apiKey != "" {
		req.Header.Set("api-key", t.apiKey)
	}

	var response map[string]any
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("delete collection failed: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    response,
	}, nil
}

func (t *QdrantTool) listCollections(ctx context.Context) (*ToolResult, error) {
	endpoint := fmt.Sprintf("%s/collections", t.baseURL)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if t.apiKey != "" {
		req.Header.Set("api-key", t.apiKey)
	}

	var response map[string]any
	if err := t.client.doJSONRequest(ctx, req, &response); err != nil {
		return nil, fmt.Errorf("list collections failed: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    response,
	}, nil
}
