// Package mcpclient provides a simplified MCP (Model Context Protocol) client
// that supports both stdio-based local MCP servers and HTTP-based remote servers.
//
// Usage (stdio - local MCP server):
//
//	tool := mcpclient.NewMCPTool(mcpclient.StdioConfig{
//	    Command: "npx",
//	    Args:    []string{"-y", "@modelcontextprotocol/server-github"},
//	})
//	result, err := tool.Run(ctx, mcpclient.ActionListTools)
//	result, err := tool.Run(ctx, mcpclient.ActionCall("search_repos", map[string]any{"query": "mcp"}))
//
// Usage (HTTP - remote MCP server):
//
//	tool := mcpclient.NewMCPTool(mcpclient.HTTPConfig{
//	    URL:     "https://mcp.tavily.com/mcp/?tavilyApiKey=xxx",
//	    Proxy:   os.Getenv("HTTPS_PROXY"),
//	})
package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Action represents an MCP action to perform
type Action struct {
	Type      string         `json:"type"`
	ToolName  string         `json:"tool_name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Action types
var (
	// ActionListTools lists all available tools from the MCP server
	ActionListTools = Action{Type: "list_tools"}
	// ActionInitialized sends the initialized notification
	ActionInitialized = Action{Type: "initialized"}
)

// ActionCall creates a call_tool action
func ActionCall(toolName string, arguments map[string]any) Action {
	return Action{
		Type:      "call_tool",
		ToolName:  toolName,
		Arguments: arguments,
	}
}

// ToolInfo represents information about an MCP tool
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Result represents the result of an MCP action
type Result struct {
	Success bool     `json:"success"`
	Data    any      `json:"data,omitempty"`
	Error   string   `json:"error,omitempty"`
	Tools   []ToolInfo `json:"tools,omitempty"`
	Content string   `json:"content,omitempty"`
}

// MCPTool is a simplified MCP client that works with any MCP server
type MCPTool struct {
	config    Config
	client    MCPClient
	initOnce  sync.Once
	initError error
}

// Config is the common interface for MCP configurations
type Config interface {
	isConfig()
}

// StdioConfig configures a stdio-based MCP server (local process)
type StdioConfig struct {
	Command string   // Command to run (e.g., "npx", "python")
	Args    []string // Command arguments
	Env     []string // Optional environment variables
}

func (StdioConfig) isConfig() {}

// HTTPConfig configures an HTTP-based MCP server (remote)
type HTTPConfig struct {
	URL       string        // MCP server URL
	Proxy     string        // Optional HTTP proxy URL
	Timeout   time.Duration // Request timeout (default 60s)
	Insecure  bool          // Skip TLS verification
}

func (HTTPConfig) isConfig() {}

// MCPClient is the internal interface for MCP communication
type MCPClient interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]ToolInfo, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*Result, error)
	Close() error
}

// NewMCPTool creates a new MCP tool with the given configuration
// This is the one-line registration pattern
func NewMCPTool(cfg Config) *MCPTool {
	return &MCPTool{config: cfg}
}

// Run executes an action on the MCP server
// This is the single method for all MCP operations
func (t *MCPTool) Run(ctx context.Context, action Action) (*Result, error) {
	// Initialize on first use
	if err := t.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	switch action.Type {
	case "list_tools":
		tools, err := t.client.ListTools(ctx)
		if err != nil {
			return &Result{Success: false, Error: err.Error()}, nil
		}
		return &Result{Success: true, Tools: tools}, nil

	case "call_tool":
		if action.ToolName == "" {
			return &Result{Success: false, Error: "tool_name is required for call_tool"}, nil
		}
		return t.client.CallTool(ctx, action.ToolName, action.Arguments)

	case "initialized":
		// Already handled during initialization
		return &Result{Success: true}, nil

	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action type: %s", action.Type)}, nil
	}
}

// ensureInitialized lazily initializes the MCP connection
func (t *MCPTool) ensureInitialized(ctx context.Context) error {
	t.initOnce.Do(func() {
		switch cfg := t.config.(type) {
		case StdioConfig:
			t.client = newStdioClient(cfg)
		case HTTPConfig:
			t.client = newHTTPClient(cfg)
		default:
			t.initError = fmt.Errorf("unsupported config type: %T", cfg)
			return
		}
		t.initError = t.client.Initialize(ctx)
	})
	return t.initError
}

// Close closes the MCP connection
func (t *MCPTool) Close() error {
	if t.client != nil {
		return t.client.Close()
	}
	return nil
}

// ==================== Stdio Client ====================

type stdioClient struct {
	config  StdioConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.Reader
	mu      sync.Mutex
	nextID  int64
	tools   []ToolInfo
}

func newStdioClient(cfg StdioConfig) *stdioClient {
	return &stdioClient{config: cfg}
}

func (c *stdioClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Start the MCP server process
	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)
	c.cmd.Env = append(os.Environ(), c.config.Env...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Send initialize request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"clientInfo":      map[string]any{"name": "uta-mcp-client", "version": "1.0"},
		},
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error [%d]: %s", resp.Error.Code, resp.Error.Message)
	}

	// Send initialized notification
	notifyReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	_, _ = c.sendRequest(notifyReq)

	// Discover tools
	tools, err := c.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover tools: %w", err)
	}
	c.tools = tools

	return nil
}

func (c *stdioClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      atomic.AddInt64(&c.nextID, 1),
		Method:  "tools/list",
		Params:  map[string]any{},
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error [%d]: %s", resp.Error.Code, resp.Error.Message)
	}

	var tools []ToolInfo
	if result, ok := resp.Result.(map[string]any); ok {
		if toolList, ok := result["tools"].([]any); ok {
			for _, t := range toolList {
				if toolMap, ok := t.(map[string]any); ok {
					tool := ToolInfo{
						Name:        fmt.Sprintf("%v", toolMap["name"]),
						Description: fmt.Sprintf("%v", toolMap["description"]),
					}
					if schema, ok := toolMap["inputSchema"].(map[string]any); ok {
						tool.InputSchema = schema
					}
					tools = append(tools, tool)
				}
			}
		}
	}

	return tools, nil
}

func (c *stdioClient) CallTool(ctx context.Context, name string, args map[string]any) (*Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      atomic.AddInt64(&c.nextID, 1),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return &Result{Success: false, Error: fmt.Sprintf("[%d] %s", resp.Error.Code, resp.Error.Message)}, nil
	}

	// Parse result
	result := &Result{Success: true}
	if resultMap, ok := resp.Result.(map[string]any); ok {
		if isError, ok := resultMap["isError"].(bool); ok && isError {
			result.Success = false
		}
		if content, ok := resultMap["content"].([]any); ok {
			var texts []string
			for _, c := range content {
				if contentMap, ok := c.(map[string]any); ok {
					if text, ok := contentMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
			result.Content = strings.Join(texts, "\n")
		}
		result.Data = resultMap
	}

	return result, nil
}

func (c *stdioClient) sendRequest(req JSONRPCRequest) (*JSONRPCResponse, error) {
	// Write request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Add newline as message delimiter
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, err
	}

	// Read response
	scanner := bufio.NewScanner(c.stdout)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		return nil, fmt.Errorf("no response from MCP server")
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

func (c *stdioClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		return c.cmd.Wait()
	}
	return nil
}

// ==================== HTTP Client ====================

type httpClient struct {
	config     HTTPConfig
	httpClient *http.Client
	tools      []ToolInfo
	mu         sync.RWMutex
	nextID     int64
}

func newHTTPClient(cfg HTTPConfig) *httpClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		},
	}

	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &httpClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

func (c *httpClient) Initialize(ctx context.Context) error {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      atomic.AddInt64(&c.nextID, 1),
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"clientInfo":      map[string]any{"name": "uta-mcp-client", "version": "1.0"},
		},
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error [%d]: %s", resp.Error.Code, resp.Error.Message)
	}

	// Send initialized notification
	notifyReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	_, _ = c.sendRequest(ctx, notifyReq)

	// Discover tools
	tools, err := c.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover tools: %w", err)
	}
	c.tools = tools

	return nil
}

func (c *httpClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      atomic.AddInt64(&c.nextID, 1),
		Method:  "tools/list",
		Params:  map[string]any{},
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error [%d]: %s", resp.Error.Code, resp.Error.Message)
	}

	var tools []ToolInfo
	if result, ok := resp.Result.(map[string]any); ok {
		if toolList, ok := result["tools"].([]any); ok {
			for _, t := range toolList {
				if toolMap, ok := t.(map[string]any); ok {
					tool := ToolInfo{
						Name:        fmt.Sprintf("%v", toolMap["name"]),
						Description: fmt.Sprintf("%v", toolMap["description"]),
					}
					if schema, ok := toolMap["inputSchema"].(map[string]any); ok {
						tool.InputSchema = schema
					}
					tools = append(tools, tool)
				}
			}
		}
	}

	return tools, nil
}

func (c *httpClient) CallTool(ctx context.Context, name string, args map[string]any) (*Result, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      atomic.AddInt64(&c.nextID, 1),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return &Result{Success: false, Error: fmt.Sprintf("[%d] %s", resp.Error.Code, resp.Error.Message)}, nil
	}

	// Parse result
	result := &Result{Success: true}
	if resultMap, ok := resp.Result.(map[string]any); ok {
		if isError, ok := resultMap["isError"].(bool); ok && isError {
			result.Success = false
		}
		if content, ok := resultMap["content"].([]any); ok {
			var texts []string
			for _, c := range content {
				if contentMap, ok := c.(map[string]any); ok {
					if text, ok := contentMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
			result.Content = strings.Join(texts, "\n")
		}
		result.Data = resultMap
	}

	return result, nil
}

func (c *httpClient) sendRequest(ctx context.Context, req JSONRPCRequest) (*JSONRPCResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(c.config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", parsedURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Handle SSE or JSON response
	contentType := httpResp.Header.Get("Content-Type")
	var resp JSONRPCResponse

	if strings.Contains(contentType, "text/event-stream") {
		resp, err = c.parseSSEResponse(httpResp.Body)
		if err != nil {
			return nil, err
		}
	} else {
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return nil, err
		}
	}

	return &resp, nil
}

func (c *httpClient) parseSSEResponse(reader io.Reader) (JSONRPCResponse, error) {
	var resp JSONRPCResponse
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		if data, found := strings.CutPrefix(line, "data: "); found {
			if data == "" || data == " " {
				continue
			}
			if err := json.Unmarshal([]byte(data), &resp); err != nil {
				continue
			}
			if resp.ID != nil {
				break
			}
		}
	}

	return resp, scanner.Err()
}

func (c *httpClient) Close() error {
	return nil
}

// ==================== Common Types ====================

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
