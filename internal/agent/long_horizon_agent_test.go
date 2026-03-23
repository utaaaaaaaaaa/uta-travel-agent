package agent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/tools"
)

// testTemplate creates a minimal template for testing
func testTemplate() *AgentTemplate {
	return &AgentTemplate{
		Spec: TemplateSpec{
			Decision: DecisionConfig{
				MaxIterations: 10,
				Timeout:       30 * time.Second,
			},
		},
	}
}

func TestLongHorizonAgentQuickComplexityCheck(t *testing.T) {
	agent := &LongHorizonAgent{
		BaseAgent: NewBaseAgent("test", AgentTypeMain, testTemplate()),
	}

	tests := []struct {
		task          string
		expectLong    bool
		minSteps      int
		description   string
	}{
		{
			task:        "Plan a 7-day trip to Japan",
			expectLong:  true,
			minSteps:    3,
			description: "Multi-day trip planning",
		},
		{
			task:        "Research Tokyo attractions and create itinerary",
			expectLong:  true,
			minSteps:    3,
			description: "Research and planning task",
		},
		{
			task:        "What is the capital of Japan?",
			expectLong:  false,
			minSteps:    1,
			description: "Simple question",
		},
		{
			task:        "Tell me about Mount Fuji",
			expectLong:  false,
			minSteps:    1,
			description: "Simple description request",
		},
		{
			task:        "Design a comprehensive travel plan for 5 cities in China",
			expectLong:  true,
			minSteps:    3,
			description: "Multi-city comprehensive planning",
		},
		{
			task:        "Compare hotels in Paris and recommend the best ones",
			expectLong:  true,
			minSteps:    3,
			description: "Comparison and recommendation task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			complexity := agent.quickComplexityCheck(tt.task)

			if complexity.IsLongHorizon != tt.expectLong {
				t.Errorf("expected IsLongHorizon=%v for '%s', got %v (reasoning: %s)",
					tt.expectLong, tt.task, complexity.IsLongHorizon, complexity.Reasoning)
			}

			if complexity.EstimatedSteps < tt.minSteps {
				t.Errorf("expected at least %d steps for '%s', got %d",
					tt.minSteps, tt.task, complexity.EstimatedSteps)
			}
		})
	}
}

func TestLongHorizonAgentGenerateTaskTag(t *testing.T) {
	agent := &LongHorizonAgent{
		BaseAgent: NewBaseAgent("test", AgentTypeMain, testTemplate()),
	}

	tests := []struct {
		task       string
		contains   string
		description string
	}{
		{
			task:        "Plan a trip to Tokyo",
			contains:    "plan-a-trip",
			description: "Simple task",
		},
		{
			task:        "Research Japanese temples and shrines",
			contains:    "research-japanese-temples",
			description: "Research task",
		},
		{
			task:        "What is the weather?",
			contains:    "what-is-the",
			description: "Question task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			tag := agent.generateTaskTag(tt.task)

			if !containsStr(tag, tt.contains) {
				t.Errorf("expected tag to contain '%s', got '%s'", tt.contains, tag)
			}

			// Check prefix
			if !hasPrefix(tag, "task-") {
				t.Errorf("expected tag to start with 'task-', got '%s'", tag)
			}
		})
	}
}

func TestLongHorizonAgentWithNoteTool(t *testing.T) {
	tmpDir := t.TempDir()
	noteTool, err := tools.NewNoteTool(tmpDir)
	if err != nil {
		t.Fatalf("failed to create note tool: %v", err)
	}

	mockAgent := &mockWrappedAgent{
		result: &AgentResult{
			Success:   true,
			Output:    "Task completed",
			Iterations: 5,
		},
	}

	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:           "test-long-agent",
		NoteTool:     noteTool,
		WrappedAgent: mockAgent,
		Template:     testTemplate(),
	})

	ctx := context.Background()

	// Run a long-horizon task
	result, err := agent.RunLongTask(ctx, "Plan a trip to Japan")
	if err != nil {
		t.Fatalf("failed to run long task: %v", err)
	}

	if !result.Success {
		t.Error("expected successful result")
	}

	if result.Metadata["long_horizon"] != true {
		t.Error("expected long_horizon metadata to be true")
	}

	// Check that progress was saved
	taskTag := result.Metadata["task_tag"].(string)
	progress, err := noteTool.GetTaskProgress(ctx, taskTag)
	if err != nil {
		t.Fatalf("failed to get task progress: %v", err)
	}

	if progress == nil {
		t.Error("expected progress note to be saved")
	}
}

func TestLongHorizonAgentCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	noteTool, _ := tools.NewNoteTool(tmpDir)

	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:       "test-checkpoint",
		NoteTool: noteTool,
		Template: testTemplate(),
	})

	ctx := context.Background()

	// Save a checkpoint
	err := agent.SaveCheckpoint(ctx, "test-task-123", "Checkpoint 1", "Progress so far...", map[string]any{
		"step": 1,
	})
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Verify checkpoint was saved
	notes, err := noteTool.Search(ctx, "", tools.NoteTypeTaskState, []string{"test-task-123"}, 10)
	if err != nil {
		t.Fatalf("failed to search notes: %v", err)
	}

	if len(notes) != 1 {
		t.Errorf("expected 1 checkpoint note, got %d", len(notes))
	}
}

func TestLongHorizonAgentCompressProgress(t *testing.T) {
	tmpDir := t.TempDir()
	noteTool, _ := tools.NewNoteTool(tmpDir)

	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:       "test-compress",
		NoteTool: noteTool,
		Template: testTemplate(),
	})

	// Add some memory state using proper methods
	agent.Memory().AddThought("Thought 1")
	agent.Memory().AddThought("Thought 2")
	agent.Memory().AddAction("Action 1", nil)
	agent.Memory().AddResult("Result 1", true, nil)

	ctx := context.Background()

	// Compress and save
	err := agent.CompressAndSaveProgress(ctx, "test-compress-123", "Summary of progress")
	if err != nil {
		t.Fatalf("failed to compress and save progress: %v", err)
	}

	// Verify
	progress, err := noteTool.GetTaskProgress(ctx, "test-compress-123")
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}

	if progress == nil {
		t.Fatal("expected progress note")
	}

	// Check metadata
	if progress.Metadata["thought_count"].(int) != 2 {
		t.Errorf("expected 2 thoughts, got %v", progress.Metadata["thought_count"])
	}
	if progress.Metadata["action_count"].(int) != 1 {
		t.Errorf("expected 1 action, got %v", progress.Metadata["action_count"])
	}
}

func TestLongHorizonAgentIsLongHorizonTask(t *testing.T) {
	agent := &LongHorizonAgent{
		BaseAgent: NewBaseAgent("test", AgentTypeMain, testTemplate()),
	}

	ctx := context.Background()

	tests := []struct {
		task     string
		expected bool
	}{
		{"Plan a 7-day trip", true},
		{"What is Tokyo?", false},
		{"Research and compare hotels", true},
		{"Tell me about Japanese food", false},
	}

	for _, tt := range tests {
		result := agent.IsLongHorizonTask(ctx, tt.task)
		if result != tt.expected {
			t.Errorf("IsLongHorizonTask(%q) = %v, expected %v", tt.task, result, tt.expected)
		}
	}
}

func TestLongHorizonAgentGetRelatedNotes(t *testing.T) {
	tmpDir := t.TempDir()
	noteTool, _ := tools.NewNoteTool(tmpDir)

	// Create some notes
	ctx := context.Background()
	noteTool.Create(ctx, "Tokyo Trip", "Notes about Tokyo", tools.NoteTypeTaskState, []string{"tokyo"}, nil)
	noteTool.Create(ctx, "Kyoto Trip", "Notes about Kyoto", tools.NoteTypeTaskState, []string{"kyoto"}, nil)
	noteTool.Create(ctx, "Osaka Food", "Food in Osaka", tools.NoteTypeInsight, []string{"osaka", "food"}, nil)

	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:       "test-related",
		NoteTool: noteTool,
		Template: testTemplate(),
	})

	// Search for related notes
	notes, err := agent.GetRelatedNotes(ctx, "Trip", 10)
	if err != nil {
		t.Fatalf("failed to get related notes: %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("expected 2 notes with 'Trip', got %d", len(notes))
	}
}

func TestLongHorizonAgentWithoutNoteTool(t *testing.T) {
	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:       "test-no-note-tool",
		Template: testTemplate(),
	})

	// Operations should gracefully handle missing note tool
	ctx := context.Background()

	_, err := agent.loadProgress(ctx, "test-tag")
	if err == nil {
		t.Error("expected error when note tool is nil")
	}

	err = agent.SaveCheckpoint(ctx, "test-tag", "title", "content", nil)
	if err == nil {
		t.Error("expected error when note tool is nil")
	}
}

func TestLongHorizonAgentInterface(t *testing.T) {
	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:       "test-interface",
		Template: testTemplate(),
	})

	// Verify Agent interface compliance
	var _ Agent = agent

	if agent.ID() != "test-interface" {
		t.Errorf("expected ID 'test-interface', got %s", agent.ID())
	}
	if agent.Type() != AgentTypeMain {
		t.Errorf("expected type Main, got %s", agent.Type())
	}
	if agent.Memory() == nil {
		t.Error("expected non-nil memory")
	}
}

func TestLongHorizonAgentRunShortTask(t *testing.T) {
	mockAgent := &mockWrappedAgent{
		result: &AgentResult{
			Success: true,
			Output:  "Quick answer",
		},
	}

	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:           "test-short",
		WrappedAgent: mockAgent,
		Template:     testTemplate(),
	})

	ctx := context.Background()

	// Run a simple question (should not use long-horizon path)
	result, err := agent.Run(ctx, "What is the capital of France?")
	if err != nil {
		t.Fatalf("failed to run: %v", err)
	}

	if !result.Success {
		t.Error("expected successful result")
	}

	// Should not have long_horizon metadata for short tasks
	if result.Metadata["long_horizon"] == true {
		t.Error("expected long_horizon to be false for short task")
	}
}

func TestLongHorizonAgentRunLongTask(t *testing.T) {
	mockAgent := &mockWrappedAgent{
		result: &AgentResult{
			Success:    true,
			Output:     "Comprehensive plan",
			Iterations: 10,
		},
	}

	tmpDir := t.TempDir()
	noteTool, _ := tools.NewNoteTool(tmpDir)

	agent := NewLongHorizonAgent(LongHorizonAgentConfig{
		ID:           "test-long-run",
		WrappedAgent: mockAgent,
		NoteTool:     noteTool,
		Template:     testTemplate(),
	})

	ctx := context.Background()

	// Run a multi-step planning task
	result, err := agent.Run(ctx, "Plan a comprehensive 10-day itinerary for Japan with daily activities")
	if err != nil {
		t.Fatalf("failed to run: %v", err)
	}

	if !result.Success {
		t.Error("expected successful result")
	}

	// Should have long_horizon metadata
	if result.Metadata["long_horizon"] != true {
		t.Error("expected long_horizon to be true for long task")
	}
}

// Mock wrapped agent for testing
type mockWrappedAgent struct {
	result *AgentResult
	err    error
}

func (m *mockWrappedAgent) ID() string {
	return "mock-agent"
}

func (m *mockWrappedAgent) Type() AgentType {
	return AgentTypeResearcher
}

func (m *mockWrappedAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result == nil {
		return &AgentResult{Success: true}, nil
	}
	return m.result, nil
}

func (m *mockWrappedAgent) State() AgentState {
	return StateIdle
}

func (m *mockWrappedAgent) Stop() error {
	return nil
}

func (m *mockWrappedAgent) Memory() *AgentMemory {
	return NewAgentMemory()
}

// Helper functions

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || containsStr(s[1:], substr))
}

func hasPrefix(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}

// Mock LLM provider for testing
type mockLLMProvider struct {
	response *llm.Response
	err      error
}

func (m *mockLLMProvider) Complete(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.response == nil {
		return &llm.Response{Content: "is_long_horizon: true\nestimated_steps: 5\nreasoning: Test response"}, nil
	}
	return m.response, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, messages []llm.Message) (<-chan llm.StreamChunk, <-chan error) {
	chunkCh := make(chan llm.StreamChunk)
	errCh := make(chan error)
	go func() {
		defer close(chunkCh)
		defer close(errCh)
		chunkCh <- llm.StreamChunk{Content: "test", Done: true}
	}()
	return chunkCh, errCh
}

func TestMain(m *testing.M) {
	// Tests are ready
	os.Exit(m.Run())
}