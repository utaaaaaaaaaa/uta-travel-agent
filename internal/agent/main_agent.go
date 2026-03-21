package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
)

// MainAgent is the primary orchestrator agent
type MainAgent struct {
	*BaseAgent
	llmProvider llm.Provider
	subagents   map[AgentType]Agent
	subagentOrder []AgentType
	mu          sync.RWMutex
}

// MainAgentConfig for creating a main agent
type MainAgentConfig struct {
	ID          string
	Template    *AgentTemplate
	LLMProvider llm.Provider
}

// NewMainAgent creates a new main agent with LLM support
func NewMainAgent(config MainAgentConfig) *MainAgent {
	return &MainAgent{
		BaseAgent:   NewBaseAgent(config.ID, AgentTypeMain, config.Template),
		llmProvider: config.LLMProvider,
		subagents:   make(map[AgentType]Agent),
		subagentOrder: []AgentType{},
	}
}

// RegisterSubagent adds a subagent to the main agent
func (a *MainAgent) RegisterSubagent(subagent Agent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	agentType := subagent.Type()
	if _, exists := a.subagents[agentType]; exists {
		return fmt.Errorf("subagent %s already registered", agentType)
	}

	a.subagents[agentType] = subagent
	a.subagentOrder = append(a.subagentOrder, agentType)
	return nil
}

// GetSubagent retrieves a subagent by type
func (a *MainAgent) GetSubagent(agentType AgentType) (Agent, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	subagent, exists := a.subagents[agentType]
	return subagent, exists
}

// ListSubagents returns all registered subagent types
func (a *MainAgent) ListSubagents() []Agent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	subagents := make([]Agent, 0, len(a.subagents))
	for _, agentType := range a.subagentOrder {
		if subagent, exists := a.subagents[agentType]; exists {
			subagents = append(subagents, subagent)
		}
	}
	return subagents
}

// ID returns the agent ID
func (a *MainAgent) ID() string {
	return a.BaseAgent.id
}

// Type returns the agent type
func (a *MainAgent) Type() AgentType {
	return AgentTypeMain
}

// State returns the current state
func (a *MainAgent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.BaseAgent.state
}

// Memory returns the agent memory
func (a *MainAgent) Memory() *AgentMemory {
	return a.BaseAgent.memory
}

// SetState sets the agent state
func (a *MainAgent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.BaseAgent.state = state
}

// Run starts the main agent with a goal
func (a *MainAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	// Add goal to memory
	a.Memory().AddThought(fmt.Sprintf("用户目标: %s", goal))

	// Analyze the goal and decide which subagents to use
	plan, err := a.planExecution(ctx, goal)
	if err != nil {
		a.SetState(StateError)
		return &AgentResult{
			AgentID:   a.ID(),
			AgentType: a.Type(),
			Goal:      goal,
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
		}, err
	}

	a.Memory().AddThought(fmt.Sprintf("执行计划: %d 个步骤", len(plan)))

	// Execute the plan
	results := make(map[AgentType]*AgentResult)
	for i, step := range plan {
		a.SetState(StateRunning)

		subagent, exists := a.GetSubagent(step.AgentType)
		if !exists {
			err := fmt.Errorf("subagent %s not found", step.AgentType)
			a.Memory().AddResult(err.Error(), false, nil)
			continue
		}

		a.Memory().AddThought(fmt.Sprintf("步骤 %d/%d: 执行 %s", i+1, len(plan), step.AgentType))

		// Run subagent
		result, err := subagent.Run(ctx, step.Goal)
		if err != nil {
			a.Memory().AddResult(fmt.Sprintf("Subagent %s failed: %v", step.AgentType, err), false, nil)
			results[step.AgentType] = &AgentResult{
				Success: false,
				Error:   err.Error(),
			}
			if step.Required {
				a.SetState(StateError)
				return &AgentResult{
					AgentID:   a.ID(),
					AgentType: a.Type(),
					Goal:      goal,
					Success:   false,
					Error:     fmt.Sprintf("required subagent %s failed", step.AgentType),
					Duration:  time.Since(startTime),
				}, err
			}
			continue
		}

		results[step.AgentType] = result
		a.Memory().AddResult(fmt.Sprintf("Subagent %s completed", step.AgentType), true, map[string]any{
			"output": result.Output,
		})
	}

	a.SetState(StateCompleted)
	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   true,
		Output:    results,
		Duration:  time.Since(startTime),
		Metadata: map[string]any{
			"plan":             plan,
			"subagent_results": len(results),
		},
	}, nil
}

// Chat handles a simple chat message
func (a *MainAgent) Chat(ctx context.Context, message string) (string, error) {
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	if a.llmProvider == nil {
		return "", fmt.Errorf("no LLM provider configured")
	}

	// Get conversation history from memory
	history := a.Memory().GetConversationHistory()
	messages := make([]llm.Message, len(history)+1)
	for i, h := range history {
		messages[i] = llm.Message{Role: h.Role, Content: h.Content}
	}
	messages[len(history)] = llm.Message{Role: "user", Content: message}

	// Add to memory
	a.Memory().AddMessage("user", message)

	// Call LLM
	systemPrompt := a.getSystemPrompt()
	response, err := a.llmProvider.CompleteWithSystem(ctx, systemPrompt, messages)
	if err != nil {
		return "", err
	}

	// Add response to memory
	a.Memory().AddMessage("assistant", response.Content)

	return response.Content, nil
}

// ChatStream handles a chat message with streaming
func (a *MainAgent) ChatStream(ctx context.Context, message string) (<-chan string, <-chan error) {
	outputCh := make(chan string, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)

		a.SetState(StateThinking)
		defer a.SetState(StateIdle)

		if a.llmProvider == nil {
			errCh <- fmt.Errorf("no LLM provider configured")
			return
		}

		// Get conversation history
		history := a.Memory().GetConversationHistory()
		messages := make([]llm.Message, len(history)+1)
		for i, h := range history {
			messages[i] = llm.Message{Role: h.Role, Content: h.Content}
		}
		messages[len(history)] = llm.Message{Role: "user", Content: message}

		// Add to memory
		a.Memory().AddMessage("user", message)

		// Stream from LLM
		systemPrompt := a.getSystemPrompt()
		messagesWithSystem := make([]llm.Message, 0, len(messages)+1)
		messagesWithSystem = append(messagesWithSystem, llm.Message{Role: "system", Content: systemPrompt})
		messagesWithSystem = append(messagesWithSystem, messages...)

		chunkCh, streamErrCh := a.llmProvider.Stream(ctx, messagesWithSystem)

		var fullResponse strings.Builder

		for {
			select {
			case chunk, ok := <-chunkCh:
				if !ok {
					// Stream finished, save full response to memory
					a.Memory().AddMessage("assistant", fullResponse.String())
					return
				}
				if chunk.Content != "" {
					fullResponse.WriteString(chunk.Content)
					select {
					case outputCh <- chunk.Content:
					case <-ctx.Done():
						return
					}
				}
			case err := <-streamErrCh:
				if err != nil {
					errCh <- err
				}
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return outputCh, errCh
}

// RAGQuery executes a RAG-enhanced query
func (a *MainAgent) RAGQuery(ctx context.Context, query, context string) (string, error) {
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	if a.llmProvider == nil {
		return "", fmt.Errorf("no LLM provider configured")
	}

	response, err := a.llmProvider.RAGQuery(ctx, query, context)
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

// SetToolRegistry sets the tool registry for the main agent
func (a *MainAgent) SetToolRegistry(registry ToolRegistry) {
	a.BaseAgent.tools = registry
}

// SetSubagentTools sets the tool registry for a specific subagent
func (a *MainAgent) SetSubagentTools(agentType AgentType, registry ToolRegistry) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	subagent, exists := a.subagents[agentType]
	if !exists {
		return fmt.Errorf("subagent %s not found", agentType)
	}

	// Use type assertion to access SetTools method
	if baseAgent, ok := subagent.(interface{ SetTools(ToolRegistry) }); ok {
		baseAgent.SetTools(registry)
		return nil
	}

	return fmt.Errorf("subagent %s does not support SetTools", agentType)
}

// RunParallelResearch runs multiple researcher agents in parallel
// Each researcher focuses on a different topic (attractions, food, culture, etc.)
func (a *MainAgent) RunParallelResearch(ctx context.Context, destination, theme string, onProgress func(string)) (*ParallelResearchResult, error) {
	startTime := time.Now()
	a.SetState(StateRunning)
	defer a.SetState(StateIdle)

	// Define research topics based on theme
	topics := a.getResearchTopics(destination, theme)

	// Create channels for results
	resultCh := make(chan *ResearchTopicResult, len(topics))
	errCh := make(chan error, len(topics))

	// Launch parallel research
	for _, topic := range topics {
		go func(t ResearchTopic) {
			result := a.researchTopic(ctx, t)
			if result.Error != nil {
				errCh <- result.Error
			} else {
				resultCh <- result
			}
		}(topic)
	}

	// Collect results
	results := make([]*ResearchTopicResult, 0, len(topics))
	var errors []error

	for i := 0; i < len(topics); i++ {
		select {
		case result := <-resultCh:
			results = append(results, result)
			if onProgress != nil {
				onProgress(fmt.Sprintf("完成 %s 的研究 (%d/%d)", result.Topic.Name, len(results), len(topics)))
			}
		case err := <-errCh:
			errors = append(errors, err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Merge all documents
	allDocs := make([]map[string]any, 0)
	for _, r := range results {
		allDocs = append(allDocs, r.Documents...)
	}

	return &ParallelResearchResult{
		Destination:    destination,
		Theme:          theme,
		Topics:         results,
		TotalDocuments: len(allDocs),
		AllDocuments:   allDocs,
		Errors:         errors,
		Duration:       time.Since(startTime),
	}, nil
}

// ResearchTopic defines a research topic
type ResearchTopic struct {
	Name        string
	Query       string
	Description string
}

// ResearchTopicResult holds results for a single topic
type ResearchTopicResult struct {
	Topic     ResearchTopic
	Documents []map[string]any
	Error     error
}

// ParallelResearchResult holds results from parallel research
type ParallelResearchResult struct {
	Destination    string
	Theme          string
	Topics         []*ResearchTopicResult
	TotalDocuments int
	AllDocuments   []map[string]any
	Errors         []error
	Duration       time.Duration
}

// getResearchTopics returns research topics based on destination and theme
func (a *MainAgent) getResearchTopics(destination, theme string) []ResearchTopic {
	// Base topics for all destinations
	topics := []ResearchTopic{
		{
			Name:        "景点",
			Query:       fmt.Sprintf("%s 景点 旅游", destination),
			Description: "主要景点和名胜",
		},
		{
			Name:        "历史与文化",
			Query:       fmt.Sprintf("%s 历史 文化", destination),
			Description: "历史背景和文化特色",
		},
		{
			Name:        "美食",
			Query:       fmt.Sprintf("%s 美食 特色", destination),
			Description: "当地美食和特色菜肴",
		},
	}

	// Add theme-specific topics
	switch theme {
	case "cultural":
		topics = append(topics, ResearchTopic{
			Name:        "文化景点",
			Query:       fmt.Sprintf("%s 寺庙 博物馆 世界遗产", destination),
			Description: "寺庙、博物馆、文化景点",
		})
	case "food":
		topics = append(topics, ResearchTopic{
			Name:        "美食推荐",
			Query:       fmt.Sprintf("%s 餐厅 小吃 市场", destination),
			Description: "餐厅、小吃、美食市场",
		})
	case "adventure":
		topics = append(topics, ResearchTopic{
			Name:        "户外活动",
			Query:       fmt.Sprintf("%s 徒步 自然 公园", destination),
			Description: "徒步、自然景观、户外活动",
		})
	case "art":
		topics = append(topics, ResearchTopic{
			Name:        "艺术场所",
			Query:       fmt.Sprintf("%s 美术馆 画廊 艺术", destination),
			Description: "美术馆、画廊、艺术展览",
		})
	}

	return topics
}

// researchTopic executes research for a single topic
func (a *MainAgent) researchTopic(ctx context.Context, topic ResearchTopic) *ResearchTopicResult {
	result := &ResearchTopicResult{
		Topic: topic,
	}

	// Get researcher subagent
	researcher, exists := a.GetSubagent(AgentTypeResearcher)
	if !exists {
		result.Error = fmt.Errorf("researcher subagent not found")
		return result
	}

	// Run researcher with topic query
	agentResult, err := researcher.Run(ctx, topic.Query)
	if err != nil {
		result.Error = err
		return result
	}

	// Extract documents from result
	if agentResult != nil && agentResult.Output != nil {
		if docs, ok := agentResult.Output.([]map[string]any); ok {
			result.Documents = docs
		} else if outputMap, ok := agentResult.Output.(map[string]any); ok {
			if docs, ok := outputMap["documents"].([]map[string]any); ok {
				result.Documents = docs
			}
		}
	}

	return result
}

// SetAllSubagentTools sets the tool registry for all registered subagents
func (a *MainAgent) SetAllSubagentTools(registry ToolRegistry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, subagent := range a.subagents {
		if baseAgent, ok := subagent.(interface{ SetTools(ToolRegistry) }); ok {
			baseAgent.SetTools(registry)
		}
	}
}

// Stop stops the main agent and all subagents
func (a *MainAgent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, subagent := range a.subagents {
		if err := subagent.Stop(); err != nil {
			return err
		}
	}

	// Directly set state without calling SetState to avoid deadlock
	a.BaseAgent.state = StateIdle
	return nil
}

// ExecutionStep represents a step in the execution plan
type ExecutionStep struct {
	AgentType AgentType `json:"agent_type"`
	Goal      string    `json:"goal"`
	Required  bool      `json:"required"`
	// ParallelSteps allows running multiple subagents in parallel
	// Each step in this slice will be executed concurrently
	ParallelSteps []ExecutionStep `json:"parallel_steps,omitempty"`
}

// planExecution analyzes the goal and creates an execution plan
func (a *MainAgent) planExecution(ctx context.Context, goal string) ([]ExecutionStep, error) {
	// Simple rule-based planning for now
	// TODO: Use LLM for intelligent planning

	var plan []ExecutionStep

	// Check if this is a destination agent creation request
	if isCreationRequest(goal) {
		plan = []ExecutionStep{
			{AgentType: AgentTypeResearcher, Goal: goal, Required: true},
			{AgentType: AgentTypeCurator, Goal: "整理研究信息", Required: true},
			{AgentType: AgentTypeIndexer, Goal: "构建知识索引", Required: true},
		}
		return plan, nil
	}

	// Check if this is an itinerary planning request
	if isPlanningRequest(goal) {
		plan = []ExecutionStep{
			{AgentType: AgentTypePlanner, Goal: goal, Required: true},
		}
		return plan, nil
	}

	// Check if this is a guide request
	if isGuideRequest(goal) {
		plan = []ExecutionStep{
			{AgentType: AgentTypeGuide, Goal: goal, Required: true},
		}
		return plan, nil
	}

	// Default: simple chat response
	return nil, nil
}

func (a *MainAgent) getSystemPrompt() string {
	// Check convenience field first
	if a.template != nil && a.template.SystemPrompt != "" {
		return a.template.SystemPrompt
	}
	// Then check spec role
	if a.template != nil && a.template.Spec.Role != "" {
		return a.template.Spec.Role
	}
	return `你是 UTA Travel 的智能旅游助手。
你的任务是为用户提供专业、友好的旅游建议和信息。
请用清晰、有组织的格式回答问题。
如果用户想创建一个目的地Agent，告诉他们可以输入"创建 [目的地名] Agent"。`
}

// Helper functions to detect request type
func isCreationRequest(goal string) bool {
	keywords := []string{"创建", "建立", "生成", "制作", "create", "build", "agent"}
	for _, kw := range keywords {
		if containsString(goal, kw) {
			return true
		}
	}
	return false
}

func isPlanningRequest(goal string) bool {
	keywords := []string{"规划", "行程", "计划", "plan", "itinerary", "路线"}
	for _, kw := range keywords {
		if containsString(goal, kw) {
			return true
		}
	}
	return false
}

func isGuideRequest(goal string) bool {
	keywords := []string{"导游", "讲解", "介绍", "guide", "explain", "带我"}
	for _, kw := range keywords {
		if containsString(goal, kw) {
			return true
		}
	}
	return false
}

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}