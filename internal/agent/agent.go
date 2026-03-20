// Package agent provides the core agent runtime for UTA Travel Agent.
// It implements a multi-agent architecture with Main Agent and Subagents.
package agent

import (
	"context"
	"fmt"
	"time"
)

// AgentType defines the type of agent
type AgentType string

const (
	AgentTypeMain       AgentType = "main"
	AgentTypeResearcher AgentType = "researcher"
	AgentTypeCurator    AgentType = "curator"
	AgentTypeIndexer    AgentType = "indexer"
	AgentTypeGuide      AgentType = "guide"
	AgentTypePlanner    AgentType = "planner"
)

// AgentState defines the current state of an agent
type AgentState string

const (
	StateIdle       AgentState = "idle"
	StateThinking   AgentState = "thinking"
	StateRunning    AgentState = "running"
	StateWaiting    AgentState = "waiting"
	StateCompleted  AgentState = "completed"
	StateError      AgentState = "error"
)

// Agent is the core interface for all agents
type Agent interface {
	// ID returns the unique identifier of the agent
	ID() string

	// Type returns the agent type
	Type() AgentType

	// Run starts the agent with the given goal
	Run(ctx context.Context, goal string) (*AgentResult, error)

	// State returns the current state of the agent
	State() AgentState

	// Stop stops the agent execution
	Stop() error

	// Memory returns the agent's memory
	Memory() *AgentMemory
}

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	id          string
	agentType   AgentType
	state       AgentState
	template    *AgentTemplate
	memory      *AgentMemory
	tools       ToolRegistry
	maxIter     int
	timeout     time.Duration
	createdAt   time.Time
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(id string, agentType AgentType, template *AgentTemplate) *BaseAgent {
	return &BaseAgent{
		id:        id,
		agentType: agentType,
		state:     StateIdle,
		template:  template,
		memory:    NewAgentMemory(),
		maxIter:   template.Spec.Decision.MaxIterations,
		timeout:   template.Spec.Decision.Timeout,
		createdAt: time.Now(),
	}
}

func (a *BaseAgent) ID() string {
	return a.id
}

func (a *BaseAgent) Type() AgentType {
	return a.agentType
}

func (a *BaseAgent) State() AgentState {
	return a.state
}

func (a *BaseAgent) Memory() *AgentMemory {
	return a.memory
}

func (a *BaseAgent) SetState(state AgentState) {
	a.state = state
}

func (a *BaseAgent) SetTools(tools ToolRegistry) {
	a.tools = tools
}

// ExecuteTool executes a tool by name with the given parameters
func (a *BaseAgent) ExecuteTool(ctx context.Context, toolName string, params map[string]any) (any, error) {
	if a.tools == nil {
		return nil, fmt.Errorf("no tools available")
	}
	return a.tools.Execute(ctx, toolName, params)
}

// AgentResult represents the result of an agent execution
type AgentResult struct {
	AgentID    string         `json:"agent_id"`
	AgentType  AgentType      `json:"agent_type"`
	Goal       string         `json:"goal"`
	Success    bool           `json:"success"`
	Output     any            `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	Iterations int            `json:"iterations"`
	Duration   time.Duration  `json:"duration"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// AgentDecision represents a decision made by the agent
type AgentDecision struct {
	Thought     string         `json:"thought"`
	Action      string         `json:"action"`
	ToolName    string         `json:"tool_name,omitempty"`
	ToolParams  map[string]any `json:"tool_params,omitempty"`
	ToolArgs    map[string]any `json:"tool_args,omitempty"` // Alias for ToolParams
	IsComplete  bool           `json:"is_complete"`
	Output      any            `json:"output,omitempty"`
	Result      string         `json:"result,omitempty"` // Result summary when complete
}
