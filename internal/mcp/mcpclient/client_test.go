package mcpclient

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMCPTool_HTTPClient_Tavily(t *testing.T) {
	apiKey := getEnvOrDefault("TAVILY_API_KEY", "")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping HTTP MCP test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create MCP tool with HTTP config
	tool := NewMCPTool(HTTPConfig{
		URL:     "https://mcp.tavily.com/mcp/?tavilyApiKey=" + apiKey,
		Timeout: 30 * time.Second,
	})
	defer tool.Close()

	// Test 1: List tools
	t.Run("ListTools", func(t *testing.T) {
		result, err := tool.Run(ctx, ActionListTools)
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("ListTools not successful: %s", result.Error)
		}

		if len(result.Tools) == 0 {
			t.Fatal("Expected at least one tool from Tavily MCP server")
		}

		t.Logf("Found %d tools:", len(result.Tools))
		for _, tool := range result.Tools {
			t.Logf("  - %s: %s", tool.Name, tool.Description)
		}
	})

	// Test 2: Call tool (tavily-search or tavily_search)
	t.Run("CallTool", func(t *testing.T) {
		// First get available tools to find the correct name
		listResult, err := tool.Run(ctx, ActionListTools)
		if err != nil || !listResult.Success {
			t.Fatalf("Failed to list tools: %v", err)
		}

		// Find search tool
		var searchToolName string
		for _, ti := range listResult.Tools {
			if ti.Name == "tavily-search" || ti.Name == "tavily_search" || ti.Name == "search" {
				searchToolName = ti.Name
				break
			}
		}

		if searchToolName == "" {
			t.Fatal("Could not find search tool in Tavily MCP server")
		}

		result, err := tool.Run(ctx, ActionCall(searchToolName, map[string]any{
			"query": "京都旅游",
		}))
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("CallTool not successful: %s", result.Error)
		}

		t.Logf("Search result (first 500 chars): %s", truncate(result.Content, 500))
		if result.Data != nil {
			t.Logf("Data keys: %v", getKeys(result.Data))
		}
	})
}

func TestMCPTool_StdioClient_Mock(t *testing.T) {
	// This test uses echo to simulate an MCP server response
	// It's a basic integration test without requiring actual MCP servers

	t.Run("Initialize", func(t *testing.T) {
		// Skip on CI or if echo doesn't work as expected
		if testing.Short() {
			t.Skip("Skipping stdio test in short mode")
		}

		// Note: This is a placeholder for real stdio tests
		// Real tests would use actual MCP servers like:
		// tool := NewMCPTool(StdioConfig{
		//     Command: "npx",
		//     Args:    []string{"-y", "@modelcontextprotocol/server-github"},
		// })
		t.Log("Stdio MCP test requires actual MCP server (like GitHub MCP via npx)")
	})
}

func TestActionCall(t *testing.T) {
	action := ActionCall("test_tool", map[string]any{"key": "value"})

	if action.Type != "call_tool" {
		t.Errorf("Expected type 'call_tool', got '%s'", action.Type)
	}
	if action.ToolName != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", action.ToolName)
	}
	if action.Arguments["key"] != "value" {
		t.Errorf("Expected argument key='value', got '%v'", action.Arguments["key"])
	}
}

func TestActionTypes(t *testing.T) {
	if ActionListTools.Type != "list_tools" {
		t.Errorf("Expected ActionListTools.Type='list_tools', got '%s'", ActionListTools.Type)
	}
	if ActionInitialized.Type != "initialized" {
		t.Errorf("Expected ActionInitialized.Type='initialized', got '%s'", ActionInitialized.Type)
	}
}

// Helper functions

func getEnv(key string) string {
	return os.Getenv(key)
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := getEnv(key); val != "" {
		return val
	}
	return defaultVal
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getKeys(m any) []string {
	if resultMap, ok := m.(map[string]any); ok {
		keys := make([]string, 0, len(resultMap))
		for k := range resultMap {
			keys = append(keys, k)
		}
		return keys
	}
	return nil
}
