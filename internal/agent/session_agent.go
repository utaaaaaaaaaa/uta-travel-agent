// Package agent provides session-based agent functionality
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/contextx"
	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/memory"
	"github.com/utaaa/uta-travel-agent/internal/session"
)

// SessionAgent is the interface for session-based agents
type SessionAgent interface {
	Agent

	// Session management
	SessionID() string
	SaveSession(ctx context.Context) (*SessionSnapshot, error)
	LoadSession(ctx context.Context, snapshot *SessionSnapshot) error

	// Context engineering
	GetContextWindow() int
	SetContextWindow(maxTokens int)
	CompressContext(ctx context.Context) error

	// Long-term memory
	Remember(key string, value any) error
	Recall(key string) (any, error)

	// Conversation
	Chat(ctx context.Context, message string) (string, error)
}

// SessionSnapshot represents a snapshot of session state
type SessionSnapshot struct {
	Session *session.Snapshot `json:"session"`
	Memory  *memory.Snapshot  `json:"memory"`
}

// SessionAgentConfig holds configuration for session-based agents
type SessionAgentConfig struct {
	ID           string
	AgentType    AgentType
	LLMProvider  llm.Provider
	SessionStore session.Storage
	MemoryStore  memory.Storage
	SystemPrompt string
	MaxContext   int
	MaxMemory    int
}

// BaseSessionAgent provides common functionality for session-based agents
type BaseSessionAgent struct {
	mu sync.RWMutex

	id         string
	agentType  AgentType
	state      AgentState

	// Session components
	session       *session.Session
	memory        *memory.PersistentMemory
	contextEngine *contextx.Engineer

	// LLM
	llmProvider  llm.Provider
	systemPrompt string
	tools        ToolRegistry

	// Configuration
	maxContext int
	maxMemory  int

	// Timestamps
	createdAt time.Time
}

// NewBaseSessionAgent creates a new session-based agent
func NewBaseSessionAgent(config SessionAgentConfig) *BaseSessionAgent {
	if config.MaxContext <= 0 {
		config.MaxContext = 8000
	}
	if config.MaxMemory <= 0 {
		config.MaxMemory = 100
	}

	// Create session
	sess := session.New(config.ID)

	// Create memory
	mem := memory.NewPersistentMemory(config.MemoryStore, config.MaxMemory)

	// Create context engineer
	contextEngine := contextx.NewEngineer(contextx.EngineerConfig{
		MaxTokens:   config.MaxContext,
		LLMProvider: config.LLMProvider,
	})

	return &BaseSessionAgent{
		id:            config.ID,
		agentType:     config.AgentType,
		state:         StateIdle,
		session:       sess,
		memory:        mem,
		contextEngine: contextEngine,
		llmProvider:   config.LLMProvider,
		systemPrompt:  config.SystemPrompt,
		maxContext:    config.MaxContext,
		maxMemory:     config.MaxMemory,
		createdAt:     time.Now(),
	}
}

// ID returns the agent ID
func (a *BaseSessionAgent) ID() string {
	return a.id
}

// Type returns the agent type
func (a *BaseSessionAgent) Type() AgentType {
	return a.agentType
}

// State returns the current state
func (a *BaseSessionAgent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState sets the agent state
func (a *BaseSessionAgent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// SessionID returns the session ID
func (a *BaseSessionAgent) SessionID() string {
	return a.session.ID()
}

// Memory returns the agent memory (for Agent interface compatibility)
func (a *BaseSessionAgent) Memory() *AgentMemory {
	// Convert persistent memory items to AgentMemory format
	// This provides backward compatibility with existing code
	return convertToAgentMemory(a.memory)
}

// PersistentMemory returns the persistent memory
func (a *BaseSessionAgent) PersistentMemory() *memory.PersistentMemory {
	return a.memory
}

// Session returns the session
func (a *BaseSessionAgent) Session() *session.Session {
	return a.session
}

// GetContextWindow returns the max context tokens
func (a *BaseSessionAgent) GetContextWindow() int {
	return a.contextEngine.GetMaxTokens()
}

// SetContextWindow sets the max context tokens
func (a *BaseSessionAgent) SetContextWindow(maxTokens int) {
	a.contextEngine.SetMaxTokens(maxTokens)
}

// Remember stores a value in long-term memory
func (a *BaseSessionAgent) Remember(key string, value any) error {
	return a.memory.Remember(key, value)
}

// Recall retrieves a value from long-term memory
func (a *BaseSessionAgent) Recall(key string) (any, error) {
	return a.memory.Recall(key)
}

// SaveSession saves the session state
func (a *BaseSessionAgent) SaveSession(ctx context.Context) (*SessionSnapshot, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Save memory to storage
	if err := a.memory.Save(ctx, a.session.ID()); err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}

	return &SessionSnapshot{
		Session: a.session.ToSnapshot(),
		Memory:  a.memory.ToSnapshot(a.session.ID()),
	}, nil
}

// LoadSession restores session from a snapshot
func (a *BaseSessionAgent) LoadSession(ctx context.Context, snapshot *SessionSnapshot) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Restore session
	sess, err := session.FromSnapshot(snapshot.Session)
	if err != nil {
		return fmt.Errorf("restore session: %w", err)
	}
	a.session = sess

	// Restore memory
	if err := a.memory.Load(ctx, a.session.ID()); err != nil {
		return fmt.Errorf("restore memory: %w", err)
	}

	return nil
}

// CompressContext compresses the context using the context engineer
func (a *BaseSessionAgent) CompressContext(ctx context.Context) error {
	items := a.memory.ShortTerm().GetRecent(20)
	if len(items) == 0 {
		return nil
	}

	_, err := a.contextEngine.Compress(ctx, items)
	return err
}

// AddMessage adds a message to the conversation
func (a *BaseSessionAgent) AddMessage(role, content string) {
	a.memory.ShortTerm().AddMessage(role, content)
	a.session.IncrementMessageCount()
	a.session.Touch()
}

// BuildContext builds the context for LLM
func (a *BaseSessionAgent) BuildContext() []llm.Message {
	return a.contextEngine.BuildContextWithSystem(a.memory, a.systemPrompt, a.maxContext)
}

// Chat sends a message and gets a response
func (a *BaseSessionAgent) Chat(ctx context.Context, message string) (string, error) {
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	// Touch session
	a.session.Touch()

	// Add user message
	a.AddMessage("user", message)

	// Build context
	messages := a.BuildContext()

	// Call LLM
	response, err := a.llmProvider.Complete(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM call: %w", err)
	}

	// Add assistant message
	a.AddMessage("assistant", response.Content)

	// Auto-save
	go func() {
		_, _ = a.SaveSession(context.Background())
	}()

	return response.Content, nil
}

// Run implements Agent interface (delegates to Chat)
func (a *BaseSessionAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	response, err := a.Chat(ctx, goal)
	if err != nil {
		return &AgentResult{
			AgentID:   a.id,
			AgentType: a.agentType,
			Goal:      goal,
			Success:   false,
			Error:     err.Error(),
		}, err
	}

	return &AgentResult{
		AgentID:   a.id,
		AgentType: a.agentType,
		Goal:      goal,
		Success:   true,
		Output:    response,
	}, nil
}

// Stop stops the agent (no-op for now)
func (a *BaseSessionAgent) Stop() error {
	a.SetState(StateIdle)
	return nil
}

// SetTools sets the tool registry
func (a *BaseSessionAgent) SetTools(tools ToolRegistry) {
	a.tools = tools
}

// SetSystemPrompt sets the system prompt
func (a *BaseSessionAgent) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.systemPrompt = prompt
}

// GetSystemPrompt returns the current system prompt
func (a *BaseSessionAgent) GetSystemPrompt() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.systemPrompt
}

// SetAgentType sets the agent type (for session)
func (a *BaseSessionAgent) SetAgentType(agentType AgentType) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.agentType = agentType
	a.session.SetAgentType(string(agentType))
}

// convertToAgentMemory converts PersistentMemory to AgentMemory for compatibility
func convertToAgentMemory(pm *memory.PersistentMemory) *AgentMemory {
	am := NewAgentMemory()
	items := pm.ShortTerm().GetAll()
	for _, item := range items {
		am.Add(item.Type, item.Content, item.Metadata)
	}
	return am
}