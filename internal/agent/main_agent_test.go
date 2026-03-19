package agent

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// MockAgent is a mock implementation of Agent for testing
type MockAgent struct {
	id        string
	agentType AgentType
	state     AgentState
	memory    *AgentMemory
	tools     ToolRegistry
	runFunc   func(ctx context.Context, goal string) (*AgentResult, error)
	stopFunc  func() error
}

func NewMockAgent(id string, agentType AgentType) *MockAgent {
	return &MockAgent{
		id:        id,
		agentType: agentType,
		state:     StateIdle,
		memory:    NewAgentMemory(),
	}
}

func (m *MockAgent) ID() string                        { return m.id }
func (m *MockAgent) Type() AgentType                   { return m.agentType }
func (m *MockAgent) State() AgentState                 { return m.state }
func (m *MockAgent) Memory() *AgentMemory              { return m.memory }
func (m *MockAgent) SetTools(tools ToolRegistry)       { m.tools = tools }
func (m *MockAgent) SetState(state AgentState)         { m.state = state }
func (m *MockAgent) Stop() error {
	if m.stopFunc != nil {
		return m.stopFunc()
	}
	m.state = StateIdle
	return nil
}

func (m *MockAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, goal)
	}
	m.state = StateRunning
	defer func() { m.state = StateCompleted }()

	return &AgentResult{
		AgentID:   m.id,
		AgentType: m.agentType,
		Goal:      goal,
		Success:   true,
		Output:    fmt.Sprintf("completed: %s", goal),
		Duration:  100 * time.Millisecond,
	}, nil
}

func createTestTemplate(name string) *AgentTemplate {
	return &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:        name,
			Version:     "1.0.0",
			Description: "Test template",
		},
		Spec: TemplateSpec{
			Role:         "Test agent",
			Capabilities: []string{"test"},
			Decision: DecisionConfig{
				MaxIterations: 10,
				Timeout:       30 * time.Second,
			},
		},
	}
}

// TestMainAgentCreation tests main agent creation
func TestMainAgentCreation(t *testing.T) {
	template := createTestTemplate("main")

	config := MainAgentConfig{
		ID:       "main-001",
		Template: template,
	}

	agent := NewMainAgent(config)

	if agent.ID() != "main-001" {
		t.Errorf("expected ID main-001, got %s", agent.ID())
	}

	if agent.Type() != AgentTypeMain {
		t.Errorf("expected type main, got %s", agent.Type())
	}

	if agent.State() != StateIdle {
		t.Errorf("expected initial state idle, got %s", agent.State())
	}

	if agent.Memory() == nil {
		t.Error("expected memory to be initialized")
	}

	if len(agent.ListSubagents()) != 0 {
		t.Error("expected no subagents initially")
	}
}

// TestMainAgentRegisterSubagent tests subagent registration
func TestMainAgentRegisterSubagent(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	// Register researcher
	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	err := agent.RegisterSubagent(researcher)
	if err != nil {
		t.Fatalf("failed to register researcher: %v", err)
	}

	// Register curator
	curator := NewMockAgent("curator-001", AgentTypeCurator)
	err = agent.RegisterSubagent(curator)
	if err != nil {
		t.Fatalf("failed to register curator: %v", err)
	}

	// Verify subagents are registered
	subagents := agent.ListSubagents()
	if len(subagents) != 2 {
		t.Errorf("expected 2 subagents, got %d", len(subagents))
	}

	// Verify order is preserved
	if subagents[0].Type() != AgentTypeResearcher {
		t.Errorf("expected first subagent to be researcher, got %s", subagents[0].Type())
	}
	if subagents[1].Type() != AgentTypeCurator {
		t.Errorf("expected second subagent to be curator, got %s", subagents[1].Type())
	}

	// Test duplicate registration
	err = agent.RegisterSubagent(researcher)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

// TestMainAgentGetSubagent tests retrieving subagent by type
func TestMainAgentGetSubagent(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	agent.RegisterSubagent(researcher)

	// Test existing subagent
	sub, exists := agent.GetSubagent(AgentTypeResearcher)
	if !exists {
		t.Fatal("expected researcher to exist")
	}
	if sub.ID() != "researcher-001" {
		t.Errorf("expected ID researcher-001, got %s", sub.ID())
	}

	// Test non-existing subagent
	_, exists = agent.GetSubagent(AgentTypeCurator)
	if exists {
		t.Error("expected curator to not exist")
	}
}

// TestMainAgentRunCreationRequest tests running a destination creation workflow
func TestMainAgentRunCreationRequest(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	// Create mock subagents with success behavior
	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	researcher.runFunc = func(ctx context.Context, goal string) (*AgentResult, error) {
		return &AgentResult{
			AgentID:   "researcher-001",
			AgentType: AgentTypeResearcher,
			Goal:      goal,
			Success:   true,
			Output: map[string]any{
				"documents": []string{"doc1", "doc2"},
			},
			Duration: 100 * time.Millisecond,
		}, nil
	}

	curator := NewMockAgent("curator-001", AgentTypeCurator)
	curator.runFunc = func(ctx context.Context, goal string) (*AgentResult, error) {
		return &AgentResult{
			AgentID:   "curator-001",
			AgentType: AgentTypeCurator,
			Goal:      goal,
			Success:   true,
			Output: map[string]any{
				"knowledge_base": "organized",
			},
			Duration: 50 * time.Millisecond,
		}, nil
	}

	indexer := NewMockAgent("indexer-001", AgentTypeIndexer)
	indexer.runFunc = func(ctx context.Context, goal string) (*AgentResult, error) {
		return &AgentResult{
			AgentID:   "indexer-001",
			AgentType: AgentTypeIndexer,
			Goal:      goal,
			Success:   true,
			Output: map[string]any{
				"collection_id": "dest_kyoto",
				"doc_count":     2,
			},
			Duration: 150 * time.Millisecond,
		}, nil
	}

	// Register subagents
	agent.RegisterSubagent(researcher)
	agent.RegisterSubagent(curator)
	agent.RegisterSubagent(indexer)

	// Run with creation request
	ctx := context.Background()
	result, err := agent.Run(ctx, "创建京都旅游Agent")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected successful result")
	}

	if result.AgentID != "main-001" {
		t.Errorf("expected agent ID main-001, got %s", result.AgentID)
	}

	// Verify memory has all steps recorded
	memory := agent.Memory()
	items := memory.GetAll()

	// Should have: thought (goal), thought (plan), 3x thought (steps), 3x result
	if len(items) < 5 {
		t.Errorf("expected at least 5 memory items, got %d", len(items))
	}

	// Verify final state
	if agent.State() != StateIdle {
		t.Errorf("expected state idle after run, got %s", agent.State())
	}
}

// TestMainAgentRunRequiredSubagentFailure tests error handling for required subagent failure
func TestMainAgentRunRequiredSubagentFailure(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	// Create failing researcher
	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	researcher.runFunc = func(ctx context.Context, goal string) (*AgentResult, error) {
		return &AgentResult{
			AgentID:   "researcher-001",
			AgentType: AgentTypeResearcher,
			Goal:      goal,
			Success:   false,
			Error:     "search failed",
			Duration:  100 * time.Millisecond,
		}, fmt.Errorf("search failed")
	}

	curator := NewMockAgent("curator-001", AgentTypeCurator)
	indexer := NewMockAgent("indexer-001", AgentTypeIndexer)

	agent.RegisterSubagent(researcher)
	agent.RegisterSubagent(curator)
	agent.RegisterSubagent(indexer)

	ctx := context.Background()
	result, err := agent.Run(ctx, "创建京都旅游Agent")

	if err == nil {
		t.Error("expected error when required subagent fails")
	}

	if result.Success {
		t.Error("expected failure result")
	}

	if result.Error == "" {
		t.Error("expected error message in result")
	}

	// Verify state is error
	if agent.State() != StateIdle {
		t.Errorf("expected state idle after error, got %s", agent.State())
	}
}

// TestMainAgentRunPlanningRequest tests running a planning workflow
func TestMainAgentRunPlanningRequest(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	planner := NewMockAgent("planner-001", AgentTypePlanner)
	planner.runFunc = func(ctx context.Context, goal string) (*AgentResult, error) {
		return &AgentResult{
			AgentID:   "planner-001",
			AgentType: AgentTypePlanner,
			Goal:      goal,
			Success:   true,
			Output: map[string]any{
				"itinerary": "3-day Kyoto trip",
			},
			Duration: 200 * time.Millisecond,
		}, nil
	}

	agent.RegisterSubagent(planner)

	ctx := context.Background()
	result, err := agent.Run(ctx, "规划京都三日游行程")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected successful result")
	}
}

// TestMainAgentRunGuideRequest tests running a guide workflow
func TestMainAgentRunGuideRequest(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	guide := NewMockAgent("guide-001", AgentTypeGuide)
	guide.runFunc = func(ctx context.Context, goal string) (*AgentResult, error) {
		return &AgentResult{
			AgentID:   "guide-001",
			AgentType: AgentTypeGuide,
			Goal:      goal,
			Success:   true,
			Output: map[string]any{
				"explanation": "Kinkaku-ji history...",
			},
			Duration: 150 * time.Millisecond,
		}, nil
	}

	agent.RegisterSubagent(guide)

	ctx := context.Background()
	result, err := agent.Run(ctx, "导游讲解金阁寺")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected successful result")
	}
}

// TestMainAgentStop tests stopping all subagents
func TestMainAgentStop(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	curator := NewMockAgent("curator-001", AgentTypeCurator)

	agent.RegisterSubagent(researcher)
	agent.RegisterSubagent(curator)

	// Set running state
	researcher.SetState(StateRunning)
	curator.SetState(StateRunning)

	err := agent.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all subagents are idle
	if researcher.State() != StateIdle {
		t.Errorf("expected researcher to be idle, got %s", researcher.State())
	}
	if curator.State() != StateIdle {
		t.Errorf("expected curator to be idle, got %s", curator.State())
	}
}

// TestMainAgentSetSubagentTools tests setting tools for subagents
func TestMainAgentSetSubagentTools(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	agent.RegisterSubagent(researcher)

	registry := NewMockToolRegistry()

	// Test SetSubagentTools
	err := agent.SetSubagentTools(AgentTypeResearcher, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if researcher.tools != registry {
		t.Error("expected tools to be set on researcher")
	}

	// Test non-existing subagent
	err = agent.SetSubagentTools(AgentTypeCurator, registry)
	if err == nil {
		t.Error("expected error for non-existing subagent")
	}
}

// TestMainAgentSetAllSubagentTools tests setting tools for all subagents
func TestMainAgentSetAllSubagentTools(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	curator := NewMockAgent("curator-001", AgentTypeCurator)

	agent.RegisterSubagent(researcher)
	agent.RegisterSubagent(curator)

	registry := NewMockToolRegistry()
	agent.SetAllSubagentTools(registry)

	if researcher.tools != registry {
		t.Error("expected tools to be set on researcher")
	}
	if curator.tools != registry {
		t.Error("expected tools to be set on curator")
	}
}

// TestMainAgentContextCancellation tests context cancellation during execution
func TestMainAgentContextCancellation(t *testing.T) {
	agent := NewMainAgent(MainAgentConfig{
		ID:       "main-001",
		Template: createTestTemplate("main"),
	})

	// Create a slow researcher
	researcher := NewMockAgent("researcher-001", AgentTypeResearcher)
	researcher.runFunc = func(ctx context.Context, goal string) (*AgentResult, error) {
		select {
		case <-time.After(2 * time.Second):
			return &AgentResult{Success: true}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	agent.RegisterSubagent(researcher)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := agent.Run(ctx, "创建京都旅游Agent")

	if err == nil {
		t.Error("expected context cancellation error")
	}
}

// TestIsCreationRequest tests request type detection
func TestIsCreationRequest(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"创建京都Agent", true},
		{"建立东京旅游助手", true},
		{"生成目的地信息", true},
		{"create travel agent", true},
		{"build new destination", true},
		{"我想去京都旅游", false},
		{"规划行程", false},
		{"导游讲解", false},
	}

	for _, tt := range tests {
		result := isCreationRequest(tt.input)
		if result != tt.expected {
			t.Errorf("isCreationRequest(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

// TestIsPlanningRequest tests planning request detection
func TestIsPlanningRequest(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"规划京都三日游", true},
		{"帮我做行程计划", true},
		{"plan my trip", true},
		{"itinerary for Tokyo", true},
		{"推荐路线", true},
		{"创建Agent", false},
		{"导游讲解", false},
	}

	for _, tt := range tests {
		result := isPlanningRequest(tt.input)
		if result != tt.expected {
			t.Errorf("isPlanningRequest(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

// TestIsGuideRequest tests guide request detection
func TestIsGuideRequest(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"导游讲解金阁寺", true},
		{"介绍清水寺历史", true},
		{"guide me through", true},
		{"带我参观", true},
		{"explain this temple", true},
		{"创建Agent", false},
		{"规划行程", false},
	}

	for _, tt := range tests {
		result := isGuideRequest(tt.input)
		if result != tt.expected {
			t.Errorf("isGuideRequest(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

// TestContainsString tests the string contains helper
func TestContainsString(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		expect bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"京都旅游", "京都", true},
		{"京都旅游", "东京", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		result := containsString(tt.s, tt.substr)
		if result != tt.expect {
			t.Errorf("containsString(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expect)
		}
	}
}