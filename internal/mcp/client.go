// Package mcp provides MCP (Model Context Protocol) client implementations
// for external tools like Brave Search, Web Reader, Maps, and Qdrant.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds MCP client configuration
type Config struct {
	BraveSearchAPIKey string `json:"brave_search_api_key"`
	UserAgent         string `json:"user_agent"`
	Timeout           int    `json:"timeout"` // seconds
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		UserAgent: "UTA-Travel-Agent/1.0",
		Timeout:   30,
	}
}

// ToolResult represents the result of an MCP tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Data    any         `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Client is the main MCP client
type Client struct {
	config     *Config
	httpClient *http.Client
	tools      map[string]MCPTool
}

// MCPTool defines the interface for MCP tools
type MCPTool interface {
	Name() string
	Execute(ctx context.Context, params map[string]any) (*ToolResult, error)
}

// NewClient creates a new MCP client
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		tools: make(map[string]MCPTool),
	}
}

// RegisterTool adds a tool to the client
func (c *Client) RegisterTool(tool MCPTool) {
	c.tools[tool.Name()] = tool
}

// Call executes an MCP tool
func (c *Client) Call(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error) {
	tool, exists := c.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("MCP tool %s not found", toolName)
	}
	return tool.Execute(ctx, params)
}

// ListTools returns all registered tools
func (c *Client) ListTools() []map[string]any {
	var tools []map[string]any
	for name := range c.tools {
		tools = append(tools, map[string]any{
			"name": name,
			"type": "mcp",
		})
	}
	return tools
}

// doRequest performs an HTTP request
func (c *Client) doRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	req = req.WithContext(ctx)
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.config.UserAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// doJSONRequest performs an HTTP request expecting JSON response
func (c *Client) doJSONRequest(ctx context.Context, req *http.Request, result any) error {
	body, err := c.doRequest(ctx, req)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}