package agent

import (
	"context"
	"fmt"
)

// ToolType defines the type of tool
type ToolType string

const (
	ToolTypeMCP     ToolType = "mcp"
	ToolTypeSkill   ToolType = "skill"
	ToolTypeService ToolType = "service"
)

// Tool represents a callable tool
type Tool struct {
	Name        string         `json:"name"`
	Type        ToolType       `json:"type"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Required    bool           `json:"required,omitempty"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Data    any         `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ToolExecutor defines how a tool is executed
type ToolExecutor interface {
	Execute(ctx context.Context, params map[string]any) (*ToolResult, error)
}

// MCPClient interface for MCP tool communication
type MCPClient interface {
	Call(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error)
	ListTools(ctx context.Context) ([]Tool, error)
}

// SkillExecutor handles skill execution based on SKILL.md definitions
type SkillExecutor interface {
	Execute(ctx context.Context, skillName string, params map[string]any) (*ToolResult, error)
	LoadSkill(skillPath string) error
	ListSkills() ([]Tool, error)
}

// ServiceClient interface for calling Python services
type ServiceClient interface {
	Call(ctx context.Context, serviceName string, method string, params map[string]any) (*ToolResult, error)
}

// ToolRegistry manages all available tools (MCP, Skills, Services)
type ToolRegistry interface {
	// Register adds a tool to the registry
	Register(tool Tool, executor ToolExecutor) error

	// Get retrieves a tool by name
	Get(toolName string) (Tool, bool)

	// Execute runs a tool with the given parameters
	Execute(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error)

	// ListTools returns all registered tools
	ListTools() []Tool

	// ListByType returns tools of a specific type
	ListByType(toolType ToolType) []Tool
}

// DefaultToolRegistry is the default implementation of ToolRegistry
type DefaultToolRegistry struct {
	tools     map[string]Tool
	executors map[string]ToolExecutor
	mcpClient MCPClient
	skillExec SkillExecutor
	svcClient ServiceClient
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(mcpClient MCPClient, skillExec SkillExecutor, svcClient ServiceClient) *DefaultToolRegistry {
	return &DefaultToolRegistry{
		tools:     make(map[string]Tool),
		executors: make(map[string]ToolExecutor),
		mcpClient: mcpClient,
		skillExec: skillExec,
		svcClient: svcClient,
	}
}

// Register adds a tool to the registry
func (r *DefaultToolRegistry) Register(tool Tool, executor ToolExecutor) error {
	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}
	r.tools[tool.Name] = tool
	r.executors[tool.Name] = executor
	return nil
}

// Get retrieves a tool by name
func (r *DefaultToolRegistry) Get(toolName string) (Tool, bool) {
	tool, exists := r.tools[toolName]
	return tool, exists
}

// Execute runs a tool with the given parameters
func (r *DefaultToolRegistry) Execute(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error) {
	tool, exists := r.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	// If there's a direct executor, use it
	if exec, ok := r.executors[toolName]; ok {
		return exec.Execute(ctx, params)
	}

	// Otherwise, route based on tool type
	switch tool.Type {
	case ToolTypeMCP:
		if r.mcpClient == nil {
			return nil, fmt.Errorf("MCP client not configured")
		}
		return r.mcpClient.Call(ctx, toolName, params)

	case ToolTypeSkill:
		if r.skillExec == nil {
			return nil, fmt.Errorf("skill executor not configured")
		}
		return r.skillExec.Execute(ctx, toolName, params)

	case ToolTypeService:
		if r.svcClient == nil {
			return nil, fmt.Errorf("service client not configured")
		}
		return r.svcClient.Call(ctx, toolName, "execute", params)

	default:
		return nil, fmt.Errorf("unknown tool type: %s", tool.Type)
	}
}

// ListTools returns all registered tools
func (r *DefaultToolRegistry) ListTools() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ListByType returns tools of a specific type
func (r *DefaultToolRegistry) ListByType(toolType ToolType) []Tool {
	var tools []Tool
	for _, tool := range r.tools {
		if tool.Type == toolType {
			tools = append(tools, tool)
		}
	}
	return tools
}

// LoadFromTemplate loads tools from an agent template
func (r *DefaultToolRegistry) LoadFromTemplate(template *AgentTemplate) error {
	// Load MCP tools
	for _, ref := range template.Spec.Tools.MCP {
		tool := Tool{
			Name:        ref.Name,
			Type:        ToolTypeMCP,
			Description: ref.Description,
			Required:    ref.Required,
		}
		// MCP tools don't have local executors, they use the mcpClient
		r.tools[tool.Name] = tool
	}

	// Load Skills
	for _, ref := range template.Spec.Tools.Skills {
		tool := Tool{
			Name:        ref.Name,
			Type:        ToolTypeSkill,
			Description: ref.Description,
			Required:    ref.Required,
		}
		r.tools[tool.Name] = tool
	}

	// Load Services
	for _, svcName := range template.Spec.Tools.Services {
		tool := Tool{
			Name: svcName,
			Type: ToolTypeService,
		}
		r.tools[tool.Name] = tool
	}

	return nil
}
