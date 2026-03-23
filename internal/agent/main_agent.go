package agent

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/memory"
)

// MainAgent is the primary orchestrator agent
type MainAgent struct {
	*BaseAgent
	llmProvider         llm.Provider
	subagents           map[AgentType]Agent
	subagentOrder       []AgentType
	preferenceExtractor *memory.PreferenceExtractor
	mu                  sync.RWMutex
}

// MainAgentConfig for creating a main agent
type MainAgentConfig struct {
	ID          string
	Template    *AgentTemplate
	LLMProvider llm.Provider
}

// NewMainAgent creates a new main agent with LLM support
func NewMainAgent(config MainAgentConfig) *MainAgent {
	agent := &MainAgent{
		BaseAgent:     NewBaseAgent(config.ID, AgentTypeMain, config.Template),
		llmProvider:   config.LLMProvider,
		subagents:     make(map[AgentType]Agent),
		subagentOrder: []AgentType{},
	}

	// Create preference extractor if LLM provider is available
	if config.LLMProvider != nil {
		agent.preferenceExtractor = memory.NewPreferenceExtractor(&preferenceLLMAdapter{provider: config.LLMProvider})
	}

	return agent
}

// preferenceLLMAdapter adapts llm.Provider to memory.LLMProvider interface
type preferenceLLMAdapter struct {
	provider llm.Provider
}

func (a *preferenceLLMAdapter) Complete(ctx context.Context, messages []memory.LLMMessage) (*memory.LLMResponse, error) {
	llmMsgs := make([]llm.Message, len(messages))
	for i, m := range messages {
		llmMsgs[i] = llm.Message{Role: m.Role, Content: m.Content}
	}

	resp, err := a.provider.Complete(ctx, llmMsgs)
	if err != nil {
		return nil, err
	}

	return &memory.LLMResponse{Content: resp.Content}, nil
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
		defer func() {
			close(outputCh)
			close(errCh)
		}()

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
		streamDone := false

		for !streamDone {
			select {
			case chunk, ok := <-chunkCh:
				if !ok {
					// Stream channel closed, finish
					a.Memory().AddMessage("assistant", fullResponse.String())
					streamDone = true
					break
				}
				if chunk.Content != "" {
					fullResponse.WriteString(chunk.Content)
					select {
					case outputCh <- chunk.Content:
					case <-ctx.Done():
						streamDone = true
						break
					}
				}
				// Check for Done signal
				if chunk.Done {
					a.Memory().AddMessage("assistant", fullResponse.String())
					streamDone = true
					break
				}
			case err, ok := <-streamErrCh:
				if !ok {
					// Error channel closed, stream finished
					streamDone = true
					break
				}
				if err != nil {
					errCh <- err
					streamDone = true
					break
				}
			case <-ctx.Done():
				streamDone = true
				break
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

	// Aggregate exploration logs, tokens, and covered topics
	var allExplorationLog []ExplorationStep
	var totalTokensIn, totalTokensOut int
	coveredTopics := make(map[string]int)

	for _, r := range results {
		// Aggregate tokens
		totalTokensIn += r.TokensIn
		totalTokensOut += r.TokensOut

		// Aggregate exploration log
		allExplorationLog = append(allExplorationLog, r.ExplorationLog...)

		// Track covered topics - use English key for frontend compatibility
		topicKey := TopicNameToKey(r.Topic.Name)
		coveredTopics[topicKey] = len(r.Documents)
	}

	// Sort exploration log by timestamp
	sort.Slice(allExplorationLog, func(i, j int) bool {
		return allExplorationLog[i].Timestamp.Before(allExplorationLog[j].Timestamp)
	})

	return &ParallelResearchResult{
		Destination:    destination,
		Theme:          theme,
		Topics:         results,
		TotalDocuments: len(allDocs),
		AllDocuments:   allDocs,
		Errors:         errors,
		Duration:       time.Since(startTime),
		TotalTokensIn:  totalTokensIn,
		TotalTokensOut: totalTokensOut,
		ExplorationLog: allExplorationLog,
		CoveredTopics:  coveredTopics,
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
	Topic           ResearchTopic
	Documents       []map[string]any
	Error           error
	ExplorationLog  []ExplorationStep
	TokensIn        int
	TokensOut       int
	DurationMs      int64
}

// ParallelResearchResult holds results from parallel research
type ParallelResearchResult struct {
	Destination     string
	Theme           string
	Topics          []*ResearchTopicResult
	TotalDocuments  int
	AllDocuments    []map[string]any
	Errors          []error
	Duration        time.Duration
	TotalTokensIn   int
	TotalTokensOut  int
	ExplorationLog  []ExplorationStep
	CoveredTopics   map[string]int
}

// TopicNameToKey maps Chinese topic names to English keys for frontend
func TopicNameToKey(name string) string {
	mapping := map[string]string{
		"景点":     "attractions",
		"历史与文化":  "history",
		"美食":     "food",
		"交通":     "transport",
		"文化景点":   "cultural",
		"美食推荐":   "food",
		"户外活动":   "adventure",
		"艺术场所":   "art",
		"住宿":     "accommodation",
		"购物":     "shopping",
	}
	if key, ok := mapping[name]; ok {
		return key
	}
	return name
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
	startTime := time.Now()
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

	// Record duration
	result.DurationMs = time.Since(startTime).Milliseconds()

	// Extract tokens from metadata
	if agentResult != nil && agentResult.Metadata != nil {
		if tokensIn, ok := agentResult.Metadata["tokens_in"].(int); ok {
			result.TokensIn = tokensIn
		}
		if tokensOut, ok := agentResult.Metadata["tokens_out"].(int); ok {
			result.TokensOut = tokensOut
		}
	}

	// Extract exploration log from output
	if agentResult != nil && agentResult.Output != nil {
		log.Printf("[researchTopic] Agent result output type: %T", agentResult.Output)
		if outputMap, ok := agentResult.Output.(map[string]any); ok {
			// Extract documents
			if docs, ok := outputMap["documents"].([]map[string]any); ok {
				result.Documents = docs
				log.Printf("[researchTopic] Extracted %d documents for topic %s", len(docs), topic.Name)
			} else if docsAny, ok := outputMap["documents"].([]any); ok {
				// Handle []any case and convert to []map[string]any
				for _, d := range docsAny {
					if m, ok := d.(map[string]any); ok {
						result.Documents = append(result.Documents, m)
					}
				}
				log.Printf("[researchTopic] Converted %d documents from []any for topic %s", len(result.Documents), topic.Name)
			} else {
				log.Printf("[researchTopic] documents key not found or wrong type, keys: %v", getKeys(outputMap))
			}

			// Extract exploration log
			if explorationLog, ok := outputMap["exploration_log"].([]ExplorationStep); ok {
				result.ExplorationLog = explorationLog
			} else if explorationLogAny, ok := outputMap["exploration_log"].([]any); ok {
				// Convert []any to []ExplorationStep
				for _, step := range explorationLogAny {
					if stepMap, ok := step.(map[string]any); ok {
						es := ExplorationStep{}
						if ts, ok := stepMap["timestamp"].(string); ok {
							if t, err := time.Parse(time.RFC3339, ts); err == nil {
								es.Timestamp = t
							}
						} else if t, ok := stepMap["timestamp"].(time.Time); ok {
							es.Timestamp = t
						}
						if d, ok := stepMap["direction"].(string); ok {
							es.Direction = d
						}
						if th, ok := stepMap["thought"].(string); ok {
							es.Thought = th
						}
						if ac, ok := stepMap["action"].(string); ok {
							es.Action = ac
						}
						if tn, ok := stepMap["tool_name"].(string); ok {
							es.ToolName = tn
						}
						if r, ok := stepMap["result"].(string); ok {
							es.Result = r
						}
						if ti, ok := stepMap["tokens_in"].(int); ok {
							es.TokensIn = ti
						} else if ti, ok := stepMap["tokens_in"].(float64); ok {
							es.TokensIn = int(ti)
						}
						if to, ok := stepMap["tokens_out"].(int); ok {
							es.TokensOut = to
						} else if to, ok := stepMap["tokens_out"].(float64); ok {
							es.TokensOut = int(to)
						}
						if dm, ok := stepMap["duration_ms"].(int64); ok {
							es.DurationMs = dm
						} else if dm, ok := stepMap["duration_ms"].(int); ok {
							es.DurationMs = int64(dm)
						} else if dm, ok := stepMap["duration_ms"].(float64); ok {
							es.DurationMs = int64(dm)
						}
						result.ExplorationLog = append(result.ExplorationLog, es)
					}
				}
				log.Printf("[researchTopic] Extracted %d exploration steps for topic %s", len(result.ExplorationLog), topic.Name)
			}
		}
	}

	return result
}

// getKeys returns keys from a map
func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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

// getGuideSystemPrompt returns a destination-specific system prompt
func (a *MainAgent) getGuideSystemPrompt(destination string) string {
	if destination == "" {
		return a.getSystemPrompt()
	}
	return fmt.Sprintf(`你是 %s 的专业导游助手。

你的职责:
1. 为游客提供专业、友好的导游服务
2. 介绍景点的历史背景、文化意义和游览建议
3. 推荐当地美食、特色体验
4. 解答游客关于 %s 的各种问题

重要规则:
- 你是 %s 的导游，当用户问"当地"、"这里"时，指的是 %s
- 不需要询问用户位置，你已经知道用户要游览 %s
- 使用生动的语言，像一位本地导游
- 提供实用的建议和有趣的故事
- 如果知道具体信息，给出准确的数据（开放时间、门票价格等）

【输出格式要求 - 必须严格遵守】
你必须使用标准 Markdown 格式输出，具体要求：

1. 标题格式：
   - 一级标题: # 标题内容
   - 二级标题: ## 标题内容
   - 三级标题: ### 标题内容
   - 标题前后必须有换行

2. 表格格式（用于对比信息）：
   | 列1 | 列2 | 列3 |
   |-----|-----|-----|
   | 内容 | 内容 | 内容 |
   - 表格前后必须有换行
   - 分隔行使用 |---|---| 格式

3. 列表格式：
   - 无序列表: - 项目内容 （推荐使用）
   - 有序列表: 1. 项目内容
   - 列表项之间不需要空行

【重要禁止事项】
- 不要使用数字编号（1. 2. 3.）作为标题或段落开头
- 不要延续之前对话的数字编号
- 如果要表示顺序，使用 emoji（1️⃣ 2️⃣ 3️⃣）或无序列表（-）
- 每个新回复都应该是独立的内容，不要引用或继续之前的编号

4. 强调格式：
   - **粗体** 用于强调重点
   - *斜体* 用于术语或特指

5. 分隔线：
   - 使用 --- 分隔不同部分
   - 分隔线前后必须有换行

示例输出格式：

# 🍜 %s美食全攻略

%s美食以"鲜、甜、细、雅"著称...

---

## 必吃美食榜

| 菜品 | 特点 | 推荐餐厅 |
|------|------|----------|
| 松鼠鳜鱼 | 外酥里嫩 | 松鹤楼 |

---

### 街头小吃

- **生煎馒头**: 底部焦脆...
- **糖粥**: 赤豆甜糯...

请用热情、专业的态度为游客服务！`, destination, destination, destination, destination, destination, destination, destination)
}

// ChatStreamWithDestination streams a chat response with destination context
func (a *MainAgent) ChatStreamWithDestination(ctx context.Context, message, destination string) (<-chan string, <-chan error) {
	outputCh := make(chan string, 10)
	errCh := make(chan error, 1)

	go func() {
		defer func() {
			close(outputCh)
			close(errCh)
		}()

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

		// Get destination-specific system prompt
		systemPrompt := a.getGuideSystemPrompt(destination)
		messagesWithSystem := make([]llm.Message, 0, len(messages)+1)
		messagesWithSystem = append(messagesWithSystem, llm.Message{Role: "system", Content: systemPrompt})
		messagesWithSystem = append(messagesWithSystem, messages...)

		// Stream from LLM
		chunkCh, streamErrCh := a.llmProvider.Stream(ctx, messagesWithSystem)

		var fullResponse strings.Builder
		streamDone := false

		for !streamDone {
			select {
			case chunk, ok := <-chunkCh:
				if !ok {
					// Stream channel closed, finish
					a.Memory().AddMessage("assistant", fullResponse.String())
					streamDone = true
					break
				}
				if chunk.Content != "" {
					fullResponse.WriteString(chunk.Content)
					select {
					case outputCh <- chunk.Content:
					case <-ctx.Done():
						streamDone = true
						break
					}
				}
				// Check for Done signal
				if chunk.Done {
					a.Memory().AddMessage("assistant", fullResponse.String())
					streamDone = true
					break
				}
			case err, ok := <-streamErrCh:
				if !ok {
					// Error channel closed, stream finished
					streamDone = true
					break
				}
				if err != nil {
					errCh <- err
					streamDone = true
					break
				}
			case <-ctx.Done():
				streamDone = true
				break
			}
		}
	}()

	return outputCh, errCh
}

// ChatStreamWithDestinationAndHistory streams a response with destination context and external conversation history
// This is used by sessions which manage their own memory separately
// mem parameter is optional - if provided, preferences will be loaded/saved
func (a *MainAgent) ChatStreamWithDestinationAndHistory(ctx context.Context, message, destination string, history []llm.Message, mem *memory.PersistentMemory) (<-chan string, <-chan error, []llm.Message) {
	outputCh := make(chan string, 10)
	errCh := make(chan error, 1)
	updatedHistory := make([]llm.Message, 0)

	go func() {
		defer func() {
			close(outputCh)
			close(errCh)
		}()

		a.SetState(StateThinking)
		defer a.SetState(StateIdle)

		if a.llmProvider == nil {
			errCh <- fmt.Errorf("no LLM provider configured")
			return
		}

		// Load user preferences if memory is provided
		var prefs *memory.UserPreferences
		if mem != nil {
			prefs, _ = mem.RecallPreferences()
		}

		// Build messages from provided history
		messages := make([]llm.Message, len(history)+1)
		copy(messages, history)
		messages[len(history)] = llm.Message{Role: "user", Content: message}

		// Get destination-specific system prompt with preferences
		systemPrompt := a.getGuideSystemPromptWithPrefs(destination, prefs)
		messagesWithSystem := make([]llm.Message, 0, len(messages)+2)

		// Add system prompt
		messagesWithSystem = append(messagesWithSystem, llm.Message{Role: "system", Content: systemPrompt})

		// Add user preferences context if available
		if prefs != nil && !prefs.IsEmpty() {
			prefsContext := prefs.FormatAsContext()
			if prefsContext != "" {
				messagesWithSystem = append(messagesWithSystem, llm.Message{
					Role:    "system",
					Content: prefsContext,
				})
			}
		}

		messagesWithSystem = append(messagesWithSystem, messages...)

		// Stream from LLM
		chunkCh, streamErrCh := a.llmProvider.Stream(ctx, messagesWithSystem)

		var fullResponse strings.Builder
		streamDone := false

		for !streamDone {
			select {
			case chunk, ok := <-chunkCh:
				if !ok {
					// Stream channel closed, finish
					streamDone = true
					break
				}
				if chunk.Content != "" {
					fullResponse.WriteString(chunk.Content)
					select {
					case outputCh <- chunk.Content:
					case <-ctx.Done():
						streamDone = true
						break
					}
				}
				// Check for Done signal
				if chunk.Done {
					streamDone = true
					break
				}
			case err, ok := <-streamErrCh:
				if !ok {
					// Error channel closed, stream finished
					streamDone = true
					break
				}
				if err != nil {
					errCh <- err
					streamDone = true
					break
				}
			case <-ctx.Done():
				streamDone = true
				break
			}
		}

		// Return updated history (for caller to save)
		updatedHistory = append(history, llm.Message{Role: "user", Content: message})
		updatedHistory = append(updatedHistory, llm.Message{Role: "assistant", Content: fullResponse.String()})

		// Asynchronously extract and save preferences if memory is provided
		if mem != nil && a.preferenceExtractor != nil {
			go a.extractAndSavePreferences(context.Background(), mem, message, fullResponse.String())
		}
	}()

	// Wait for goroutine to complete and return updated history
	go func() {
		<-ctx.Done()
	}()

	return outputCh, errCh, updatedHistory
}

// extractAndSavePreferences extracts preferences from conversation and saves to memory
func (a *MainAgent) extractAndSavePreferences(ctx context.Context, mem *memory.PersistentMemory, userMsg, assistantMsg string) {
	// Build conversation for extraction
	conversation := fmt.Sprintf("用户: %s\n助手: %s", userMsg, assistantMsg)

	// Extract new preferences
	newPrefs, err := a.preferenceExtractor.ExtractPreferences(ctx, conversation)
	if err != nil {
		log.Printf("[MainAgent] Failed to extract preferences: %v", err)
		return
	}

	// Skip if no preferences extracted
	if newPrefs.IsEmpty() {
		return
	}

	// Load existing preferences and merge
	existingPrefs, _ := mem.RecallPreferences()
	mergedPrefs := memory.MergePreferences(existingPrefs, newPrefs)

	// Save merged preferences
	if err := mem.RememberPreferences(mergedPrefs); err != nil {
		log.Printf("[MainAgent] Failed to save preferences: %v", err)
		return
	}

	log.Printf("[MainAgent] Saved updated preferences: travel_style=%s, budget=%s",
		mergedPrefs.TravelStyle, mergedPrefs.BudgetLevel)
}

// getGuideSystemPromptWithPrefs returns a destination-specific system prompt with preference awareness
func (a *MainAgent) getGuideSystemPromptWithPrefs(destination string, prefs *memory.UserPreferences) string {
	if destination == "" {
		return a.getSystemPrompt()
	}

	// Base prompt
	basePrompt := a.getGuideSystemPrompt(destination)

	// Add preference awareness section if preferences exist
	if prefs != nil && !prefs.IsEmpty() {
		var prefGuidance strings.Builder
		prefGuidance.WriteString("\n\n【用户偏好参考】\n")
		prefGuidance.WriteString("请根据以下用户偏好调整推荐内容:\n")

		if prefs.TravelStyle != "" {
			prefGuidance.WriteString(fmt.Sprintf("- 旅行风格: %s\n", formatTravelStyle(prefs.TravelStyle)))
		}
		if prefs.BudgetLevel != "" {
			prefGuidance.WriteString(fmt.Sprintf("- 预算级别: %s\n", formatBudgetLevel(prefs.BudgetLevel)))
		}
		if len(prefs.DietaryRestrictions) > 0 {
			prefGuidance.WriteString(fmt.Sprintf("- 饮食限制: %s (推荐餐厅时请注意)\n", strings.Join(prefs.DietaryRestrictions, ", ")))
		}
		if len(prefs.PreferredActivities) > 0 {
			prefGuidance.WriteString(fmt.Sprintf("- 喜欢的活动: %s\n", strings.Join(prefs.PreferredActivities, ", ")))
		}
		if len(prefs.Dislikes) > 0 {
			prefGuidance.WriteString(fmt.Sprintf("- 不喜欢: %s (请避免推荐)\n", strings.Join(prefs.Dislikes, ", ")))
		}
		if prefs.TravelPace != "" {
			prefGuidance.WriteString(fmt.Sprintf("- 旅行节奏: %s\n", formatTravelPace(prefs.TravelPace)))
		}

		return basePrompt + prefGuidance.String()
	}

	return basePrompt
}

// Helper functions for preference formatting (duplicated from preferences.go to avoid import cycles)
func formatTravelStyle(style string) string {
	styles := map[string]string{
		"cultural":   "文化历史",
		"food":       "美食探索",
		"adventure":  "冒险户外",
		"art":        "艺术人文",
		"relaxation": "休闲度假",
	}
	if s, ok := styles[style]; ok {
		return s
	}
	return style
}

func formatBudgetLevel(level string) string {
	levels := map[string]string{
		"economy":   "经济实惠",
		"mid-range": "中等预算",
		"luxury":    "奢华享受",
	}
	if s, ok := levels[level]; ok {
		return s
	}
	return level
}

func formatTravelPace(pace string) string {
	paces := map[string]string{
		"slow":     "慢节奏深度游",
		"moderate": "适中节奏",
		"fast":     "快节奏打卡",
	}
	if s, ok := paces[pace]; ok {
		return s
	}
	return pace
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