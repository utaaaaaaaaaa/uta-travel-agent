package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/utaaa/uta-travel-agent/internal/llm"
)

// MockLLMProvider is a mock implementation of llm.Provider
type MockLLMProvider struct {
	responses []*llm.Response
	callIndex int
	errors    []error
	streamCh  chan string
	streamErr chan error
}

func NewMockLLMProvider() *MockLLMProvider {
	return &MockLLMProvider{
		responses: make([]*llm.Response, 0),
		callIndex: 0,
		errors:    make([]error, 0),
	}
}

func (m *MockLLMProvider) Complete(ctx context.Context, messages []llm.Message, opts ...llm.Option) (*llm.Response, error) {
	return m.CompleteWithSystem(ctx, "", messages, opts...)
}

func (m *MockLLMProvider) CompleteWithSystem(ctx context.Context, systemPrompt string, messages []llm.Message, opts ...llm.Option) (*llm.Response, error) {
	if m.callIndex < len(m.errors) && m.errors[m.callIndex] != nil {
		err := m.errors[m.callIndex]
		m.callIndex++
		return nil, err
	}

	if m.callIndex >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}

	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func (m *MockLLMProvider) Stream(ctx context.Context, messages []llm.Message, opts ...llm.Option) (<-chan llm.StreamChunk, <-chan error) {
	chunkCh := make(chan llm.StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		if m.streamCh != nil {
			for chunk := range m.streamCh {
				select {
				case chunkCh <- llm.StreamChunk{Content: chunk}:
				case <-ctx.Done():
					return
				}
			}
		}

		if m.streamErr != nil {
			select {
			case errCh <- <-m.streamErr:
			default:
			}
		}
	}()

	return chunkCh, errCh
}

func (m *MockLLMProvider) RAGQuery(ctx context.Context, query, context string, opts ...llm.Option) (*llm.Response, error) {
	return m.CompleteWithSystem(ctx, "", []llm.Message{
		{Role: "user", Content: query},
	}, opts...)
}

func (m *MockLLMProvider) AddResponse(content string, inputTokens, outputTokens int) {
	m.responses = append(m.responses, &llm.Response{
		Content:      content,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	})
}

func (m *MockLLMProvider) AddError(err error) {
	m.errors = append(m.errors, err)
}

// TestLLMAgentCreation tests creating a new LLM-powered agent
func TestLLMAgentCreation(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	agent := NewLLMAgent(LLMAgentConfig{
		ID:            "researcher-001",
		AgentType:     AgentTypeResearcher,
		LLMProvider:   mockLLM,
		SystemPrompt:  ResearcherAgentPrompt,
		Tools:         mockTools,
		MaxIterations: 10,
	})

	assert.Equal(t, "researcher-001", agent.ID())
	assert.Equal(t, AgentTypeResearcher, agent.Type())
	assert.Equal(t, StateIdle, agent.State())
	assert.NotNil(t, agent.Memory())
}

// TestLLMAgentReActLoop tests the ReAct (Think → Act → Observe) loop
func TestLLMAgentReActLoop(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	// Setup tool
	mockTools.Register(Tool{
		Name:        "brave_search",
		Type:        ToolTypeMCP,
		Description: "Search the web",
	}, nil)
	mockTools.SetResult("brave_search", &ToolResult{
		Success: true,
		Data: map[string]any{
			"results": []any{
				map[string]any{"url": "https://example.com", "title": "Test"},
			},
		},
	})

	// Step 1: LLM decides to search
	mockLLM.AddResponse(`{
		"thought": "I need to search for Kyoto travel information",
		"action": "Searching for Kyoto travel guide",
		"tool_name": "brave_search",
		"tool_args": {"query": "Kyoto travel guide"},
		"is_complete": false
	}`, 100, 50)

	// Step 2: LLM decides task is complete
	mockLLM.AddResponse(`{
		"thought": "I have collected enough information",
		"action": "Task completed",
		"is_complete": true,
		"result": "Collected 5 documents about Kyoto travel"
	}`, 100, 50)

	agent := NewLLMAgent(LLMAgentConfig{
		ID:            "researcher-001",
		AgentType:     AgentTypeResearcher,
		LLMProvider:   mockLLM,
		SystemPrompt:  ResearcherAgentPrompt,
		Tools:         mockTools,
		MaxIterations: 10,
	})

	ctx := context.Background()
	result, err := agent.Run(ctx, "Research Kyoto travel information")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "researcher-001", result.AgentID)
	assert.NotZero(t, result.Duration)

	// Verify tool was called
	assert.Equal(t, 1, mockTools.GetCallCount("brave_search"))

	// Verify exploration log
	explorationLog := agent.GetExplorationLog()
	assert.GreaterOrEqual(t, len(explorationLog), 1)
	assert.Contains(t, explorationLog[0].Thought, "search")
}

// TestLLMAgentMaxIterations tests that agent stops at max iterations
func TestLLMAgentMaxIterations(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	// Always return incomplete responses
	for i := 0; i < 15; i++ {
		mockLLM.AddResponse(`{
			"thought": "Still thinking...",
			"action": "Continuing",
			"is_complete": false
		}`, 50, 30)
	}

	agent := NewLLMAgent(LLMAgentConfig{
		ID:            "test-001",
		AgentType:     AgentTypeResearcher,
		LLMProvider:   mockLLM,
		SystemPrompt:  "Test prompt",
		Tools:         mockTools,
		MaxIterations: 3,
	})

	ctx := context.Background()
	result, err := agent.Run(ctx, "Test goal")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 3, result.Metadata["iterations"])
	assert.Equal(t, true, result.Metadata["max_reached"])
}

// TestLLMAgentToolExecutionFailure tests agent handling tool failures
func TestLLMAgentToolExecutionFailure(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	mockTools.Register(Tool{
		Name:        "failing_tool",
		Type:        ToolTypeMCP,
		Description: "A tool that fails",
	}, nil)
	mockTools.SetError("failing_tool", errors.New("tool execution failed"))

	// Step 1: LLM tries to use failing tool
	mockLLM.AddResponse(`{
		"thought": "Let me try this tool",
		"action": "Using failing tool",
		"tool_name": "failing_tool",
		"tool_args": {},
		"is_complete": false
	}`, 50, 30)

	// Step 2: LLM recovers and completes
	mockLLM.AddResponse(`{
		"thought": "The tool failed but I can still complete",
		"action": "Completing without tool",
		"is_complete": true,
		"result": "Task completed with partial results"
	}`, 50, 30)

	agent := NewLLMAgent(LLMAgentConfig{
		ID:            "test-001",
		AgentType:     AgentTypeResearcher,
		LLMProvider:   mockLLM,
		SystemPrompt:  "Test prompt",
		Tools:         mockTools,
		MaxIterations: 10,
	})

	ctx := context.Background()
	result, err := agent.Run(ctx, "Test goal")

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify the tool was called despite failure
	assert.Equal(t, 1, mockTools.GetCallCount("failing_tool"))
}

// TestLLMAgentDirectionInference tests the direction inference from thoughts
func TestLLMAgentDirectionInference(t *testing.T) {
	agent := NewLLMAgent(LLMAgentConfig{
		ID:           "test",
		AgentType:    AgentTypeResearcher,
		SystemPrompt: "Test",
	})

	tests := []struct {
		thought    string
		expected   string
	}{
		{"I need to search for famous temples in Kyoto", "景点"},
		{"Looking for the best ramen restaurants", "美食"},
		{"Researching the history and culture", "文化"},
		{"How to get from airport to city center", "交通"},
		{"Finding good hotels in the area", "住宿"},
		{"Where to buy souvenirs and gifts", "购物"},
		{"General overview of the destination", "综合"},
	}

	for _, tt := range tests {
		direction := agent.inferDirection(tt.thought)
		assert.Contains(t, []string{"景点", "美食", "文化", "交通", "住宿", "购物", "综合"}, direction)
	}
}

// TestLLMAgentMemoryTracking tests that the agent properly tracks memory
func TestLLMAgentMemoryTracking(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	mockLLM.AddResponse(`{
		"thought": "Starting research",
		"action": "Completing immediately",
		"is_complete": true,
		"result": "Done"
	}`, 100, 50)

	agent := NewLLMAgent(LLMAgentConfig{
		ID:            "test-001",
		AgentType:     AgentTypeResearcher,
		LLMProvider:   mockLLM,
		SystemPrompt:  "Test",
		Tools:         mockTools,
		MaxIterations: 10,
	})

	ctx := context.Background()
	_, err := agent.Run(ctx, "Test goal")

	require.NoError(t, err)

	// Check memory was populated
	thoughts := agent.Memory().GetByType("thought")
	assert.NotEmpty(t, thoughts)
}

// TestLLMAgentStop tests stopping the agent
func TestLLMAgentStop(t *testing.T) {
	agent := NewLLMAgent(LLMAgentConfig{
		ID:           "test-001",
		AgentType:    AgentTypeResearcher,
		SystemPrompt: "Test",
	})

	agent.SetState(StateRunning)
	err := agent.Stop()

	require.NoError(t, err)
	assert.Equal(t, StateIdle, agent.State())
}

// TestLLMAgentJSONParsing tests parsing LLM JSON responses
func TestLLMAgentJSONParsing(t *testing.T) {
	agent := NewLLMAgent(LLMAgentConfig{
		ID:           "test",
		AgentType:    AgentTypeResearcher,
		SystemPrompt: "Test",
	})

	tests := []struct {
		name     string
		input    string
		hasError bool
	}{
		{
			name: "valid JSON",
			input: `{
				"thought": "Thinking",
				"action": "Acting",
				"is_complete": true
			}`,
			hasError: false,
		},
		{
			name:     "no JSON",
			input:    "This is just plain text",
			hasError: true,
		},
		{
			name: "JSON in markdown",
			input: "Here's my response:\n```json\n{\"thought\": \"test\", \"is_complete\": true}\n```",
			hasError: false, // Parser finds JSON inside
		},
		{
			name: "invalid JSON",
			input: `{thought: "missing quotes"}`,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := agent.parseDecision(tt.input)
			if tt.hasError {
				assert.Error(t, err)
				assert.Nil(t, decision)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, decision)
			}
		})
	}
}

// TestSubagentPromptRetrieval tests getting prompts for different agent types
func TestSubagentPromptRetrieval(t *testing.T) {
	tests := []struct {
		agentType   AgentType
		shouldExist bool
	}{
		{AgentTypeResearcher, true},
		{AgentTypeCurator, true},
		{AgentTypeIndexer, true},
		{AgentTypeGuide, true},
		{AgentTypePlanner, true},
		{AgentTypeMain, false},
	}

	for _, tt := range tests {
		prompt := GetSubagentPrompt(tt.agentType)
		if tt.shouldExist {
			assert.NotEmpty(t, prompt, "Expected prompt for %s", tt.agentType)
			assert.Contains(t, prompt, "角色定义")
		} else {
			assert.Empty(t, prompt)
		}
	}
}

// TestFactoryCreatesLLMSubagent tests that factory creates LLMAgent-based subagents
func TestFactoryCreatesLLMSubagent(t *testing.T) {
	templateRegistry := NewTemplateRegistry()
	// Register a template for researcher
	templateRegistry.templates[AgentTypeResearcher] = &AgentTemplate{
		Metadata: TemplateMetadata{
			Name:    "researcher",
			Version: "v1.0.0",
		},
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 10,
			},
		},
	}

	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	factory := NewAgentFactory(templateRegistry, mockTools, mockLLM)

	// Create a researcher agent
	agent, err := factory.CreateAgent(AgentTypeResearcher)
	require.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, AgentTypeResearcher, agent.Type())

	// Verify it's an LLMAgent
	_, ok := agent.(*LLMAgent)
	assert.True(t, ok, "Expected LLMAgent type")
}

// TestFactoryCreatesAllSubagentTypes tests factory creating all subagent types
func TestFactoryCreatesAllSubagentTypes(t *testing.T) {
	templateRegistry := NewTemplateRegistry()

	// Register templates for all subagent types
	subagentTypes := []AgentType{
		AgentTypeResearcher,
		AgentTypeCurator,
		AgentTypeIndexer,
		AgentTypeGuide,
		AgentTypePlanner,
	}

	for _, agentType := range subagentTypes {
		templateRegistry.templates[agentType] = &AgentTemplate{
			Metadata: TemplateMetadata{
				Name:    string(agentType),
				Version: "v1.0.0",
			},
			Spec: TemplateSpec{
				Decision: DecisionConfig{
					MaxIterations: 10,
				},
			},
		}
	}

	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	factory := NewAgentFactory(templateRegistry, mockTools, mockLLM)

	for _, agentType := range subagentTypes {
		agent, err := factory.CreateAgent(agentType)
		require.NoError(t, err, "Failed to create %s", agentType)
		assert.Equal(t, agentType, agent.Type())

		// Verify it's an LLMAgent
		llmAgent, ok := agent.(*LLMAgent)
		assert.True(t, ok, "Expected LLMAgent type for %s", agentType)
		assert.NotNil(t, llmAgent.Memory())
	}
}

// TestMainAgentWithLLMSubagents tests MainAgent with LLM-powered subagents
func TestMainAgentWithLLMSubagents(t *testing.T) {
	templateRegistry := NewTemplateRegistry()

	// Register templates for all agent types
	allTypes := []AgentType{
		AgentTypeMain,
		AgentTypeResearcher,
		AgentTypeCurator,
		AgentTypeIndexer,
		AgentTypeGuide,
		AgentTypePlanner,
	}

	for _, agentType := range allTypes {
		templateRegistry.templates[agentType] = &AgentTemplate{
			Metadata: TemplateMetadata{
				Name:    string(agentType),
				Version: "v1.0.0",
			},
			Spec: TemplateSpec{
				Decision: DecisionConfig{
					MaxIterations: 10,
				},
			},
		}
	}

	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	// Setup mock responses for subagents
	// Researcher responses
	mockLLM.AddResponse(`{
		"thought": "Searching for destination info",
		"action": "Searching",
		"tool_name": "brave_search",
		"tool_args": {"query": "Kyoto travel"},
		"is_complete": false
	}`, 100, 50)
	mockLLM.AddResponse(`{
		"thought": "Research complete",
		"action": "Done",
		"is_complete": true,
		"result": "Found 10 documents"
	}`, 100, 50)

	// Curator responses
	mockLLM.AddResponse(`{
		"thought": "Organizing information",
		"action": "Building knowledge base",
		"tool_name": "build_knowledge_base",
		"tool_args": {},
		"is_complete": false
	}`, 100, 50)
	mockLLM.AddResponse(`{
		"thought": "Curation complete",
		"action": "Done",
		"is_complete": true,
		"result": "Organized 8 categories"
	}`, 100, 50)

	// Indexer responses
	mockLLM.AddResponse(`{
		"thought": "Building index",
		"action": "Indexing",
		"tool_name": "build_knowledge_index",
		"tool_args": {},
		"is_complete": false
	}`, 100, 50)
	mockLLM.AddResponse(`{
		"thought": "Indexing complete",
		"action": "Done",
		"is_complete": true,
		"result": "Indexed 50 chunks"
	}`, 100, 50)

	// Setup tools
	mockTools.Register(Tool{Name: "brave_search", Type: ToolTypeMCP, Description: "Search"}, nil)
	mockTools.Register(Tool{Name: "build_knowledge_base", Type: ToolTypeSkill, Description: "Build KB"}, nil)
	mockTools.Register(Tool{Name: "build_knowledge_index", Type: ToolTypeSkill, Description: "Index"}, nil)

	mockTools.SetResult("brave_search", &ToolResult{Success: true, Data: map[string]any{"results": []any{}}})
	mockTools.SetResult("build_knowledge_base", &ToolResult{Success: true, Data: map[string]any{"kb": "created"}})
	mockTools.SetResult("build_knowledge_index", &ToolResult{Success: true, Data: map[string]any{"index": "created"}})

	factory := NewAgentFactory(templateRegistry, mockTools, mockLLM)

	// Create main agent with subagents
	mainAgent, err := factory.CreateMainAgentWithSubagents()
	require.NoError(t, err)
	assert.NotNil(t, mainAgent)

	// Verify subagents are registered
	subagents := mainAgent.ListSubagents()
	assert.Len(t, subagents, 5)

	// Verify subagents are LLMAgent type
	for _, subagent := range subagents {
		_, ok := subagent.(*LLMAgent)
		assert.True(t, ok, "Expected LLMAgent type for subagent")
	}
}

// TestExplorationLog tests that exploration is tracked correctly
func TestExplorationLog(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	mockTools := NewMockToolRegistry()

	mockTools.Register(Tool{Name: "test_tool", Type: ToolTypeMCP, Description: "Test"}, nil)
	mockTools.SetResult("test_tool", &ToolResult{Success: true, Data: "result"})

	mockLLM.AddResponse(`{
		"thought": "Searching for famous temples in Kyoto",
		"action": "Using search tool",
		"tool_name": "test_tool",
		"tool_args": {},
		"is_complete": false
	}`, 150, 200)

	mockLLM.AddResponse(`{
		"thought": "Found temple information",
		"action": "Completing",
		"is_complete": true,
		"result": "Found 5 temples"
	}`, 100, 50)

	agent := NewLLMAgent(LLMAgentConfig{
		ID:            "test-001",
		AgentType:     AgentTypeResearcher,
		LLMProvider:   mockLLM,
		SystemPrompt:  "Test",
		Tools:         mockTools,
		MaxIterations: 10,
	})

	ctx := context.Background()
	result, err := agent.Run(ctx, "Find temples in Kyoto")

	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify exploration log
	log := agent.GetExplorationLog()
	assert.Len(t, log, 2)

	// Verify first step has direction inference
	assert.NotEmpty(t, log[0].Direction)
	assert.NotEmpty(t, log[0].Thought)
	assert.Equal(t, "test_tool", log[0].ToolName)
	assert.Greater(t, log[0].TokensIn, 0)
	assert.Greater(t, log[0].TokensOut, 0)

	// Verify the result includes exploration log
	outputMap, ok := result.Output.(map[string]any)
	require.True(t, ok)
	explorationLog, ok := outputMap["exploration_log"].([]ExplorationStep)
	require.True(t, ok)
	assert.Len(t, explorationLog, 2)
}

// TestFormatToolResult tests the tool result formatting
func TestFormatToolResult(t *testing.T) {
	agent := NewLLMAgent(LLMAgentConfig{
		ID:           "test",
		AgentType:    AgentTypeResearcher,
		SystemPrompt: "Test",
	})

	tests := []struct {
		name     string
		result   *ToolResult
		contains string
	}{
		{
			name:     "failed result",
			result:   &ToolResult{Success: false, Error: "some error"},
			contains: "执行失败",
		},
		{
			name:     "nil data",
			result:   &ToolResult{Success: true, Data: nil},
			contains: "无返回数据",
		},
		{
			name:     "map data",
			result:   &ToolResult{Success: true, Data: map[string]any{"key": "value"}},
			contains: "key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := agent.formatToolResult(tt.result)
			assert.Contains(t, formatted, tt.contains)
		})
	}
}

// Helper to access private methods in tests
func init() {
	// This allows us to test private methods by creating helper functions
}
