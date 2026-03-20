package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
)

// LLMAgent is a complete Agent with LLM as brain
// It follows the Agent paradigm: Memory, Context, Prompt, Action Flow, LLM Brain
type LLMAgent struct {
	mu            sync.RWMutex
	id            string
	agentType     AgentType
	state         AgentState
	llmProvider   llm.Provider
	systemPrompt  string
	memory        *AgentMemory
	context       []llm.Message // Conversation context
	tools         ToolRegistry
	maxIterations int
	explorationLog []ExplorationStep // Track exploration for radar chart
}

// LLMAgentConfig for creating an LLM-powered agent
type LLMAgentConfig struct {
	ID            string
	AgentType     AgentType
	LLMProvider   llm.Provider
	SystemPrompt  string
	Tools         ToolRegistry
	MaxIterations int
}

// ExplorationStep tracks agent's exploration for visualization
type ExplorationStep struct {
	Timestamp  time.Time `json:"timestamp"`
	Direction  string    `json:"direction"`  // What direction is being explored (景点/美食/文化/交通/住宿/购物)
	Thought    string    `json:"thought"`    // Agent's thought
	Action     string    `json:"action"`     // Action taken
	ToolName   string    `json:"tool_name"`  // Tool used
	ToolArgs   map[string]any `json:"tool_args,omitempty"`
	Result     string    `json:"result"`     // Result summary
	TokensIn   int       `json:"tokens_in"`  // Input tokens
	TokensOut  int       `json:"tokens_out"` // Output tokens
	DurationMs int64     `json:"duration_ms"`
	Success    bool      `json:"success"`    // Whether this step succeeded
}

// NewLLMAgent creates a new LLM-powered agent
func NewLLMAgent(config LLMAgentConfig) *LLMAgent {
	if config.MaxIterations == 0 {
		config.MaxIterations = 10
	}

	return &LLMAgent{
		id:            config.ID,
		agentType:     config.AgentType,
		state:         StateIdle,
		llmProvider:   config.LLMProvider,
		systemPrompt:  config.SystemPrompt,
		memory:        NewAgentMemory(),
		context:       make([]llm.Message, 0),
		tools:         config.Tools,
		maxIterations: config.MaxIterations,
		explorationLog: make([]ExplorationStep, 0),
	}
}

// ID returns the agent ID
func (a *LLMAgent) ID() string {
	return a.id
}

// Type returns the agent type
func (a *LLMAgent) Type() AgentType {
	return a.agentType
}

// State returns the current state
func (a *LLMAgent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState sets the agent state
func (a *LLMAgent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// Memory returns the agent memory
func (a *LLMAgent) Memory() *AgentMemory {
	return a.memory
}

// GetExplorationLog returns the exploration log for radar chart
func (a *LLMAgent) GetExplorationLog() []ExplorationStep {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.explorationLog
}

// Stop stops the agent
func (a *LLMAgent) Stop() error {
	a.SetState(StateIdle)
	return nil
}

// Run starts the agent with a goal using ReAct pattern
func (a *LLMAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	// Initialize context with the goal
	a.memory.AddThought(fmt.Sprintf("开始任务: %s", goal))
	a.context = append(a.context, llm.Message{
		Role:    "user",
		Content: fmt.Sprintf("请完成以下任务:\n\n%s\n\n请使用 ReAct 格式思考和行动。每一步都要先思考(Thought)，然后决定行动(Action)。如果任务完成，设置 is_complete 为 true。", goal),
	})

	var totalTokensIn, totalTokensOut int
	iteration := 0

	for iteration < a.maxIterations {
		iteration++
		a.SetState(StateRunning)

		// LLM thinks and decides next action
		decision, tokensIn, tokensOut, err := a.thinkAndDecide(ctx)
		if err != nil {
			a.SetState(StateError)
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      goal,
				Success:   false,
				Error:     fmt.Sprintf("LLM 思考失败: %v", err),
				Duration:  time.Since(startTime),
			}, err
		}

		totalTokensIn += tokensIn
		totalTokensOut += tokensOut

		// Record thought
		a.memory.AddThought(decision.Thought)

		// Log exploration step
		explorationStep := ExplorationStep{
			Timestamp:  time.Now(),
			Direction:  a.inferDirection(decision.Thought),
			Thought:    decision.Thought,
			Action:     decision.Action,
			ToolName:   decision.ToolName,
			TokensIn:   tokensIn,
			TokensOut:  tokensOut,
			DurationMs: time.Since(startTime).Milliseconds(),
		}

		// Execute tool if specified
		if decision.ToolName != "" && a.tools != nil {
			toolStart := time.Now()
			// Use ToolParams, fallback to ToolArgs for compatibility
			params := decision.ToolParams
			if params == nil {
				params = decision.ToolArgs
			}
			result, err := a.tools.Execute(ctx, decision.ToolName, params)
			execTime := time.Since(toolStart)

			if err != nil {
				observation := fmt.Sprintf("工具 %s 执行失败: %v", decision.ToolName, err)
				a.memory.AddObservation(observation, decision.ToolName)
				a.context = append(a.context, llm.Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Thought: %s\nAction: %s(%v)\nResult: %s", decision.Thought, decision.ToolName, params, observation),
				})
				explorationStep.Result = observation
			} else {
				resultStr := a.formatToolResult(result)
				a.memory.AddObservation(resultStr, decision.ToolName)
				a.context = append(a.context, llm.Message{
					Role:    "assistant",
					Content: fmt.Sprintf("Thought: %s\nAction: %s(%v)\nResult: %s", decision.Thought, decision.ToolName, params, resultStr),
				})
				explorationStep.Result = resultStr
				explorationStep.DurationMs = execTime.Milliseconds()
			}
		}

		// Record exploration
		a.mu.Lock()
		a.explorationLog = append(a.explorationLog, explorationStep)
		a.mu.Unlock()

		// Check if task is complete
		if decision.IsComplete {
			a.SetState(StateCompleted)
			a.memory.AddResult(decision.Result, true, map[string]any{
				"iterations":   iteration,
				"tokens_in":    totalTokensIn,
				"tokens_out":   totalTokensOut,
				"explorations": len(a.explorationLog),
			})

			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      goal,
				Success:   true,
				Output: map[string]any{
					"result":        decision.Result,
					"exploration_log": a.explorationLog,
				},
				Duration: time.Since(startTime),
				Metadata: map[string]any{
					"iterations":   iteration,
					"tokens_in":    totalTokensIn,
					"tokens_out":   totalTokensOut,
					"explorations": len(a.explorationLog),
				},
			}, nil
		}
	}

	// Max iterations reached
	a.SetState(StateCompleted)
	a.memory.AddResult(fmt.Sprintf("达到最大迭代次数 %d，任务可能未完全完成", a.maxIterations), false, nil)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   true,
		Output: map[string]any{
			"result":          "达到最大迭代次数",
			"exploration_log": a.explorationLog,
		},
		Duration: time.Since(startTime),
		Metadata: map[string]any{
			"iterations":     iteration,
			"tokens_in":      totalTokensIn,
			"tokens_out":     totalTokensOut,
			"max_reached":    true,
			"explorations":   len(a.explorationLog),
		},
	}, nil
}

// thinkAndDecide uses LLM to think and decide next action
func (a *LLMAgent) thinkAndDecide(ctx context.Context) (*AgentDecision, int, int, error) {
	// Build available tools description
	toolsDesc := ""
	if a.tools != nil {
		toolsDesc = "\n\n可用工具:\n"
		for _, tool := range a.tools.ListTools() {
			toolsDesc += fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description)
		}
	}

	systemPrompt := a.systemPrompt + toolsDesc + `

## 响应格式
你必须以 JSON 格式响应:
{
  "thought": "你的思考过程",
  "action": "你的行动描述",
  "tool_name": "要使用的工具名(可选)",
  "tool_args": {"参数名": "参数值"},
  "is_complete": false,
  "result": "任务完成时的总结(仅在 is_complete=true 时)"
}`

	// Call LLM
	response, err := a.llmProvider.CompleteWithSystem(ctx, systemPrompt, a.context,
		llm.WithTemperature(0.7),
		llm.WithMaxTokens(2048),
	)
	if err != nil {
		return nil, 0, 0, err
	}

	// Parse response
	decision, err := a.parseDecision(response.Content)
	if err != nil {
		// If parsing fails, treat as a thought without action
		return &AgentDecision{
			Thought:    response.Content,
			Action:     "思考",
			IsComplete: false,
		}, response.InputTokens, response.OutputTokens, nil
	}

	return decision, response.InputTokens, response.OutputTokens, nil
}

// parseDecision parses LLM response into AgentDecision
func (a *LLMAgent) parseDecision(content string) (*AgentDecision, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)

	// Find JSON in the response
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := content[startIdx : endIdx+1]

	var decision AgentDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return &decision, nil
}

// inferDirection infers exploration direction from thought
func (a *LLMAgent) inferDirection(thought string) string {
	thought = strings.ToLower(thought)

	directions := map[string][]string{
		"景点": {"景点", "景观", "名胜", "寺庙", "神社", "公园", "tower", "temple", "shrine"},
		"美食": {"美食", "料理", "餐厅", "小吃", "抹茶", "寿司", "food", "restaurant", "cuisine"},
		"文化": {"文化", "历史", "传统", "艺术", "文化", "culture", "history", "art"},
		"交通": {"交通", "地铁", "巴士", "车站", "transport", "train", "bus"},
		"住宿": {"住宿", "酒店", "民宿", "hotel", "ryokan"},
		"购物": {"购物", "商店", "市场", "shopping", "market"},
	}

	for direction, keywords := range directions {
		for _, keyword := range keywords {
			if strings.Contains(thought, keyword) {
				return direction
			}
		}
	}

	return "综合"
}

// formatToolResult formats tool result for context
func (a *LLMAgent) formatToolResult(result *ToolResult) string {
	if !result.Success {
		return fmt.Sprintf("执行失败: %s", result.Error)
	}

	// Convert result data to string
	if result.Data == nil {
		return "执行成功，无返回数据"
	}

	// Try to format as JSON
	if data, err := json.MarshalIndent(result.Data, "", "  "); err == nil {
		if len(data) > 500 {
			return string(data[:500]) + "\n...(内容过长已截断)"
		}
		return string(data)
	}

	return fmt.Sprintf("%v", result.Data)
}
