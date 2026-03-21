// Package mcpclient provides a simplified MCP (Model Context Protocol) client
package mcpclient

import (
	"context"

	"github.com/utaaa/uta-travel-agent/internal/agent"
)

// MCPToolAdapter adapts MCPTool to the agent.ToolExecutor interface
// This allows MCP tools to be used seamlessly in the agent system
type MCPToolAdapter struct {
	tool     *MCPTool
	toolName string // The specific tool to call on the MCP server
}

// NewMCPToolAdapter creates an adapter for a specific MCP tool
func NewMCPToolAdapter(mcpTool *MCPTool, toolName string) *MCPToolAdapter {
	return &MCPToolAdapter{
		tool:     mcpTool,
		toolName: toolName,
	}
}

// Execute implements agent.ToolExecutor
func (a *MCPToolAdapter) Execute(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
	result, err := a.tool.Run(ctx, ActionCall(a.toolName, params))
	if err != nil {
		return &agent.ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if !result.Success {
		return &agent.ToolResult{
			Success: false,
			Error:   result.Error,
		}, nil
	}

	return &agent.ToolResult{
		Success: true,
		Data:    result.Data,
	}, nil
}

// DiscoverAndRegisterTools discovers all tools from an MCP server and registers them
// This is the one-line pattern: mcpclient.DiscoverAndRegisterTools(registry, mcpTool)
func DiscoverAndRegisterTools(ctx context.Context, registry agent.ToolRegistry, mcpTool *MCPTool) error {
	// List available tools
	result, err := mcpTool.Run(ctx, ActionListTools)
	if err != nil {
		return err
	}
	if !result.Success {
		return &MCPError{Message: result.Error}
	}

	// Register each tool
	for _, toolInfo := range result.Tools {
		tool := agent.Tool{
			Name:        toolInfo.Name,
			Type:        agent.ToolTypeMCP,
			Description: toolInfo.Description,
			Parameters:  toolInfo.InputSchema,
		}

		adapter := NewMCPToolAdapter(mcpTool, toolInfo.Name)
		if err := registry.Register(tool, adapter); err != nil {
			return err
		}
	}

	return nil
}

// MCPError represents an MCP-related error
type MCPError struct {
	Message string
}

func (e *MCPError) Error() string {
	return e.Message
}
