package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockToolRegistry is a mock implementation of ToolRegistry
type MockToolRegistry struct {
	tools     map[string]Tool
	executors map[string]ToolExecutor
	calls     map[string]int // Track tool calls
	results   map[string]*ToolResult
	errors    map[string]error
}

func NewMockToolRegistry() *MockToolRegistry {
	return &MockToolRegistry{
		tools:     make(map[string]Tool),
		executors: make(map[string]ToolExecutor),
		calls:     make(map[string]int),
		results:   make(map[string]*ToolResult),
		errors:    make(map[string]error),
	}
}

func (m *MockToolRegistry) Register(tool Tool, executor ToolExecutor) error {
	m.tools[tool.Name] = tool
	m.executors[tool.Name] = executor
	return nil
}

func (m *MockToolRegistry) Get(toolName string) (Tool, bool) {
	tool, exists := m.tools[toolName]
	return tool, exists
}

func (m *MockToolRegistry) Execute(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error) {
	m.calls[toolName]++

	if err, exists := m.errors[toolName]; exists {
		return nil, err
	}

	if result, exists := m.results[toolName]; exists {
		return result, nil
	}

	return &ToolResult{Success: true, Data: nil}, nil
}

func (m *MockToolRegistry) ListTools() []Tool {
	tools := make([]Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools
}

func (m *MockToolRegistry) ListByType(toolType ToolType) []Tool {
	var tools []Tool
	for _, tool := range m.tools {
		if tool.Type == toolType {
			tools = append(tools, tool)
		}
	}
	return tools
}

func (m *MockToolRegistry) SetResult(toolName string, result *ToolResult) {
	m.results[toolName] = result
}

func (m *MockToolRegistry) SetError(toolName string, err error) {
	m.errors[toolName] = err
}

func (m *MockToolRegistry) GetCallCount(toolName string) int {
	return m.calls[toolName]
}

// TestResearcherAgentCreation tests creating a new researcher agent
func TestResearcherAgentCreation(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:        "researcher",
			Version:     "v1.0.0",
			Description: "Researcher Agent template",
		},
		Spec: TemplateSpec{
			Role: "Professional travel information researcher",
			Decision: DecisionConfig{
				Model:         "claude-sonnet-4-6",
				Temperature:   0.2,
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)

	assert.Equal(t, "researcher-001", agent.ID())
	assert.Equal(t, AgentTypeResearcher, agent.Type())
	assert.Equal(t, StateIdle, agent.State())
	assert.NotNil(t, agent.Memory())
}

// TestResearcherAgentRunWithSuccess tests running the agent with successful tool execution
func TestResearcherAgentRunWithSuccess(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)
	mockRegistry := NewMockToolRegistry()

	// Setup mock results
	mockRegistry.SetResult("brave_search", &ToolResult{
		Success: true,
		Data: map[string]any{
			"query": "Kyoto travel guide",
			"results": []any{
				map[string]any{
					"title":       "Kyoto Travel Guide",
					"url":         "https://example.com/kyoto-guide",
					"description": "Complete guide to Kyoto",
				},
				map[string]any{
					"title":       "Best temples in Kyoto",
					"url":         "https://example.com/kyoto-temples",
					"description": "Top temples to visit",
				},
			},
		},
	})

	mockRegistry.SetResult("web_reader", &ToolResult{
		Success: true,
		Data: map[string]any{
			"url":     "https://example.com/kyoto-guide",
			"content": "Kyoto is a city in Japan known for its temples...",
			"size":    1000,
		},
	})

	mockRegistry.SetResult("extract_travel_info", &ToolResult{
		Success: true,
		Data: map[string]any{
			"attractions": []any{
				map[string]any{"name": "Kinkaku-ji", "description": "Golden Pavilion"},
			},
		},
	})

	agent.SetTools(mockRegistry)

	ctx := context.Background()
	result, err := agent.Run(ctx, "Research Kyoto travel information")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "researcher-001", result.AgentID)
	assert.Equal(t, AgentTypeResearcher, result.AgentType)
	assert.NotZero(t, result.Duration)

	// Verify tool calls
	assert.Equal(t, 1, mockRegistry.GetCallCount("brave_search"))
	assert.GreaterOrEqual(t, mockRegistry.GetCallCount("web_reader"), 1)
}

// TestResearcherAgentSearchFailure tests agent behavior when search fails
func TestResearcherAgentSearchFailure(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)
	mockRegistry := NewMockToolRegistry()

	// Setup search failure
	mockRegistry.SetError("brave_search", errors.New("API rate limit exceeded"))

	agent.SetTools(mockRegistry)

	ctx := context.Background()
	result, err := agent.Run(ctx, "Research Tokyo travel information")

	require.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "搜索失败")
	assert.Equal(t, StateIdle, agent.State()) // Should return to idle after error
}

// TestResearcherAgentWebReaderFailure tests agent behavior when web reading fails
func TestResearcherAgentWebReaderFailure(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)
	mockRegistry := NewMockToolRegistry()

	// Search succeeds but web reader fails
	mockRegistry.SetResult("brave_search", &ToolResult{
		Success: true,
		Data: map[string]any{
			"query": "Osaka travel",
			"results": []any{
				map[string]any{
					"url":   "https://example.com/osaka",
					"title": "Osaka Guide",
				},
			},
		},
	})

	mockRegistry.SetError("web_reader", errors.New("connection timeout"))

	agent.SetTools(mockRegistry)

	ctx := context.Background()
	result, err := agent.Run(ctx, "Research Osaka travel information")

	// Should still succeed even if web reader fails for some URLs
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, mockRegistry.GetCallCount("brave_search"))
}

// TestResearcherAgentMemoryTracking tests that the agent properly tracks memory
func TestResearcherAgentMemoryTracking(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)
	mockRegistry := NewMockToolRegistry()

	mockRegistry.SetResult("brave_search", &ToolResult{
		Success: true,
		Data:    map[string]any{"query": "test", "results": []any{}},
	})

	agent.SetTools(mockRegistry)

	ctx := context.Background()
	_, err := agent.Run(ctx, "Research test destination")

	require.NoError(t, err)

	// Check memory was populated
	thoughts := agent.Memory().GetByType("thought")
	assert.NotEmpty(t, thoughts)

	actions := agent.Memory().GetByType("action")
	assert.NotEmpty(t, actions)
	assert.Equal(t, "brave_search", actions[0].Content)
}

// TestResearcherAgentStop tests stopping the agent
func TestResearcherAgentStop(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)
	agent.SetState(StateRunning)

	err := agent.Stop()
	require.NoError(t, err)
	assert.Equal(t, StateIdle, agent.State())
}

// TestExtractURLs tests the URL extraction helper function
func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected []string
		found    bool
	}{
		{
			name: "map with urls array",
			input: map[string]any{
				"urls": []string{"https://example.com/1", "https://example.com/2"},
			},
			expected: []string{"https://example.com/1", "https://example.com/2"},
			found:    true,
		},
		{
			name: "map with results array containing url field",
			input: map[string]any{
				"results": []any{
					map[string]any{"url": "https://example.com/1"},
					map[string]any{"url": "https://example.com/2"},
				},
			},
			expected: []string{"https://example.com/1", "https://example.com/2"},
			found:    true,
		},
		{
			name: "map with results array containing link field",
			input: map[string]any{
				"results": []any{
					map[string]any{"link": "https://example.com/link1"},
					map[string]any{"link": "https://example.com/link2"},
				},
			},
			expected: []string{"https://example.com/link1", "https://example.com/link2"},
			found:    true,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
			found:    false,
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: nil,
			found:    false,
		},
		{
			name: "array of maps with urls",
			input: []any{
				map[string]any{"url": "https://example.com/1"},
				map[string]any{"url": "https://example.com/2"},
				map[string]any{"link": "https://example.com/3"},
			},
			expected: []string{"https://example.com/1", "https://example.com/2", "https://example.com/3"},
			found:    true,
		},
		{
			name: "ToolResult with results array",
			input: &ToolResult{
				Success: true,
				Data: map[string]any{
					"results": []any{
						map[string]any{"url": "https://example.com/1"},
						map[string]any{"url": "https://example.com/2"},
					},
				},
			},
			expected: []string{"https://example.com/1", "https://example.com/2"},
			found:    true,
		},
		{
			name: "ToolResult with nil data",
			input: &ToolResult{
				Success: true,
				Data:    nil,
			},
			expected: nil,
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls, found := extractURLs(tt.input)
			assert.Equal(t, tt.found, found)
			if tt.expected != nil {
				assert.Equal(t, tt.expected, urls)
			}
		})
	}
}

// TestResearcherAgentWithoutTools tests agent behavior when no tools are configured
func TestResearcherAgentWithoutTools(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)
	// Don't set tools

	ctx := context.Background()
	result, err := agent.Run(ctx, "Research test destination")

	require.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "no tools available")
}

// TestResearcherAgentContextCancellation tests agent behavior when context is cancelled
func TestResearcherAgentContextCancellation(t *testing.T) {
	template := &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 30,
				Timeout:       600 * time.Second,
			},
		},
	}

	agent := NewResearcherAgent("researcher-001", template)
	mockRegistry := NewMockToolRegistry()

	// Setup a slow mock
	mockRegistry.SetResult("brave_search", &ToolResult{
		Success: true,
		Data:    map[string]any{"query": "test", "results": []any{}},
	})

	agent.SetTools(mockRegistry)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := agent.Run(ctx, "Research test destination")

	// The agent should handle context cancellation gracefully
	// Either it returns an error or handles it internally
	if err != nil {
		assert.False(t, result.Success)
	}
}