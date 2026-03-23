// Package agent provides agent implementations for UTA Travel Agent
package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/tools"
)

// TaskComplexity represents analysis of a task's complexity
type TaskComplexity struct {
	IsLongHorizon  bool     `json:"is_long_horizon"`
	EstimatedSteps int      `json:"estimated_steps"`
	Keywords       []string `json:"keywords"`
	Reasoning      string   `json:"reasoning"`
}

// LongHorizonAgent wraps another agent to provide long-horizon task support
type LongHorizonAgent struct {
	*BaseAgent
	noteTool     *tools.NoteTool
	llmProvider  llm.Provider
	wrappedAgent Agent
	taskNotes    map[string]*tools.Note
	mu           sync.RWMutex
}

// LongHorizonAgentConfig for creating a long-horizon agent
type LongHorizonAgentConfig struct {
	ID           string
	LLMProvider  llm.Provider
	NoteTool     *tools.NoteTool
	WrappedAgent Agent
	Template     *AgentTemplate
}

// NewLongHorizonAgent creates a new long-horizon agent
func NewLongHorizonAgent(config LongHorizonAgentConfig) *LongHorizonAgent {
	agent := &LongHorizonAgent{
		BaseAgent:    NewBaseAgent(config.ID, AgentTypeMain, config.Template),
		llmProvider:  config.LLMProvider,
		noteTool:     config.NoteTool,
		wrappedAgent: config.WrappedAgent,
		taskNotes:    make(map[string]*tools.Note),
	}
	return agent
}

// AnalyzeTaskComplexity analyzes whether a task is long-horizon
func (a *LongHorizonAgent) AnalyzeTaskComplexity(ctx context.Context, task string) (*TaskComplexity, error) {
	// Use heuristics first for quick classification
	complexity := a.quickComplexityCheck(task)

	// If LLM is available, get more accurate analysis
	if a.llmProvider != nil && complexity.EstimatedSteps == 0 {
		llmComplexity, err := a.llmComplexityAnalysis(ctx, task)
		if err == nil {
			return llmComplexity, nil
		}
		log.Printf("[LongHorizonAgent] LLM complexity analysis failed: %v", err)
	}

	return complexity, nil
}

// quickComplexityCheck uses heuristics to estimate task complexity
func (a *LongHorizonAgent) quickComplexityCheck(task string) *TaskComplexity {
	taskLower := strings.ToLower(task)
	complexity := &TaskComplexity{
		IsLongHorizon:  false,
		EstimatedSteps: 1,
		Keywords:       []string{},
		Reasoning:      "Quick heuristic check",
	}

	// Keywords indicating multi-step tasks
	multiStepKeywords := []string{
		"plan", "schedule", "itinerary", "trip", "travel", "multi-day",
		"research", "explore", "analyze", "compare", "design",
		"week", "days", "cities", "destinations", "attractions",
		"complete", "comprehensive", "detailed", "full",
	}

	// Keywords indicating single-step tasks
	singleStepKeywords := []string{
		"what is", "tell me about", "describe", "explain",
		"how do i", "where is", "when", "who",
	}

	// Check for multi-step indicators
	for _, kw := range multiStepKeywords {
		if strings.Contains(taskLower, kw) {
			complexity.Keywords = append(complexity.Keywords, kw)
			complexity.EstimatedSteps += 2
		}
	}

	// Check for single-step indicators
	for _, kw := range singleStepKeywords {
		if strings.Contains(taskLower, kw) {
			complexity.EstimatedSteps = 1
			break
		}
	}

	// Determine if long-horizon
	if complexity.EstimatedSteps >= 3 || len(complexity.Keywords) >= 2 {
		complexity.IsLongHorizon = true
		complexity.Reasoning = fmt.Sprintf("Task contains %d multi-step keywords: %v",
			len(complexity.Keywords), complexity.Keywords)
	}

	return complexity
}

// llmComplexityAnalysis uses LLM for more accurate complexity analysis
func (a *LongHorizonAgent) llmComplexityAnalysis(ctx context.Context, task string) (*TaskComplexity, error) {
	prompt := fmt.Sprintf(`Analyze this task and determine if it's a long-horizon task that requires multiple steps.

Task: %s

Respond in this format:
is_long_horizon: true/false
estimated_steps: <number>
reasoning: <brief explanation>

Consider it long-horizon if it requires:
- Multiple research steps
- Planning across multiple days/locations
- Gathering and synthesizing information from multiple sources
- Creating a comprehensive output (itinerary, plan, report)`, task)

	messages := []llm.Message{
		{Role: "system", Content: "You are a task complexity analyzer. Respond concisely."},
		{Role: "user", Content: prompt},
	}

	resp, err := a.llmProvider.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}

	complexity := &TaskComplexity{
		IsLongHorizon:  strings.Contains(strings.ToLower(resp.Content), "is_long_horizon: true"),
		EstimatedSteps: 1,
		Reasoning:      "LLM analysis",
	}

	// Parse estimated steps
	if strings.Contains(resp.Content, "estimated_steps:") {
		var steps int
		fmt.Sscanf(resp.Content, "estimated_steps: %d", &steps)
		if steps > 0 {
			complexity.EstimatedSteps = steps
		}
	}

	return complexity, nil
}

// RunLongTask executes a long-horizon task with progress tracking
func (a *LongHorizonAgent) RunLongTask(ctx context.Context, task string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateRunning)

	// Generate task tag for note storage
	taskTag := a.generateTaskTag(task)

	log.Printf("[LongHorizonAgent] Starting long-horizon task: %s (tag: %s)", task, taskTag)

	// Check for existing progress
	existingProgress, err := a.loadProgress(ctx, taskTag)
	if err == nil && existingProgress != nil {
		log.Printf("[LongHorizonAgent] Resuming from existing progress")
	}

	// Execute the wrapped agent
	result, err := a.wrappedAgent.Run(ctx, task)
	if err != nil {
		// Save progress on failure
		a.saveProgressOnError(ctx, taskTag, task, err)
		return nil, err
	}

	// Save completion progress
	if err := a.saveCompletionProgress(ctx, taskTag, task, result); err != nil {
		log.Printf("[LongHorizonAgent] Warning: failed to save completion progress: %v", err)
	}

	result.Duration = time.Since(startTime)
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["long_horizon"] = true
	result.Metadata["task_tag"] = taskTag

	return result, nil
}

// Run implements the Agent interface
func (a *LongHorizonAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	// Analyze task complexity
	complexity, err := a.AnalyzeTaskComplexity(ctx, goal)
	if err != nil {
		log.Printf("[LongHorizonAgent] Complexity analysis failed: %v", err)
		complexity = &TaskComplexity{IsLongHorizon: false, EstimatedSteps: 1}
	}

	if complexity.IsLongHorizon {
		return a.RunLongTask(ctx, goal)
	}

	// Run normally for short tasks
	if a.wrappedAgent != nil {
		return a.wrappedAgent.Run(ctx, goal)
	}

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   false,
		Error:     "No wrapped agent configured",
	}, nil
}

// Stop stops the agent
func (a *LongHorizonAgent) Stop() error {
	if a.wrappedAgent != nil {
		return a.wrappedAgent.Stop()
	}
	a.SetState(StateIdle)
	return nil
}

// generateTaskTag creates a unique tag for the task
func (a *LongHorizonAgent) generateTaskTag(task string) string {
	// Create a simple tag from the task
	words := strings.Fields(strings.ToLower(task))
	if len(words) > 3 {
		words = words[:3]
	}
	tag := strings.Join(words, "-")

	// Clean up tag
	tag = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, tag)

	return fmt.Sprintf("task-%s-%d", tag, time.Now().Unix())
}

// loadProgress loads existing progress for a task
func (a *LongHorizonAgent) loadProgress(ctx context.Context, taskTag string) (*tools.Note, error) {
	if a.noteTool == nil {
		return nil, fmt.Errorf("note tool not configured")
	}

	note, err := a.noteTool.GetTaskProgress(ctx, taskTag)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.taskNotes[taskTag] = note
	a.mu.Unlock()

	return note, nil
}

// saveProgressOnError saves progress when an error occurs
func (a *LongHorizonAgent) saveProgressOnError(ctx context.Context, taskTag, task string, err error) {
	if a.noteTool == nil {
		return
	}

	content := fmt.Sprintf(`# Task Progress (Error)

## Task
%s

## Status
Error: %s

## Timestamp
%s
`, task, err.Error(), time.Now().Format(time.RFC3339))

	a.noteTool.SaveTaskProgress(ctx, taskTag, fmt.Sprintf("Error: %s", task), content, map[string]any{
		"status":  "error",
		"error":   err.Error(),
		"agent":   a.ID(),
	})
}

// saveCompletionProgress saves progress when task completes successfully
func (a *LongHorizonAgent) saveCompletionProgress(ctx context.Context, taskTag, task string, result *AgentResult) error {
	if a.noteTool == nil {
		return nil
	}

	content := fmt.Sprintf(`# Task Completed

## Task
%s

## Result
Success: %v
Iterations: %d
Duration: %v

## Output
%v

## Timestamp
%s
`, task, result.Success, result.Iterations, result.Duration, result.Output, time.Now().Format(time.RFC3339))

	_, err := a.noteTool.SaveTaskProgress(ctx, taskTag, fmt.Sprintf("Completed: %s", task), content, map[string]any{
		"status":     "completed",
		"success":    result.Success,
		"iterations": result.Iterations,
		"duration":   result.Duration.String(),
		"agent":      a.ID(),
	})

	return err
}

// SaveCheckpoint saves a checkpoint during long task execution
func (a *LongHorizonAgent) SaveCheckpoint(ctx context.Context, taskTag, title, content string, metadata map[string]any) error {
	if a.noteTool == nil {
		return fmt.Errorf("note tool not configured")
	}

	_, err := a.noteTool.Create(ctx, title, content, tools.NoteTypeTaskState, []string{taskTag, "checkpoint"}, metadata)
	return err
}

// GetRelatedNotes retrieves notes related to a task
func (a *LongHorizonAgent) GetRelatedNotes(ctx context.Context, query string, limit int) ([]*tools.Note, error) {
	if a.noteTool == nil {
		return nil, fmt.Errorf("note tool not configured")
	}

	return a.noteTool.Search(ctx, query, "", nil, limit)
}

// CompressAndSaveProgress compresses current memory state and saves as a note
func (a *LongHorizonAgent) CompressAndSaveProgress(ctx context.Context, taskTag, summary string) error {
	if a.noteTool == nil {
		return fmt.Errorf("note tool not configured")
	}

	// Get counts from memory
	thoughts := a.Memory().GetByType("thought")
	actions := a.Memory().GetByType("action")
	results := a.Memory().GetByType("result")

	content := fmt.Sprintf(`# Task Progress Summary

## Summary
%s

## Memory State
- Total thoughts: %d
- Total actions: %d
- Total results: %d

## Timestamp
%s
`, summary, len(thoughts), len(actions), len(results), time.Now().Format(time.RFC3339))

	_, err := a.noteTool.SaveTaskProgress(ctx, taskTag, "Progress Summary", content, map[string]any{
		"summary":       summary,
		"thought_count": len(thoughts),
		"action_count":  len(actions),
		"result_count":  len(results),
	})

	return err
}

// IsLongHorizonTask is a convenience function to check if a task is long-horizon
func (a *LongHorizonAgent) IsLongHorizonTask(ctx context.Context, task string) bool {
	complexity, err := a.AnalyzeTaskComplexity(ctx, task)
	if err != nil {
		return false
	}
	return complexity.IsLongHorizon
}
