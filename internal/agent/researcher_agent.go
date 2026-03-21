// Package agent provides the core agent implementation
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
)

// ResearcherAgent is a specialized agent for researching topics
// It reads from and writes to a shared knowledge state
type ResearcherAgent struct {
	mu           sync.RWMutex
	id           string
	agentType    AgentType
	state        AgentState
	llmProvider  llm.Provider
	systemPrompt string
	maxRounds    int
	currentRound int

	// Shared state
	sharedState *SharedKnowledgeState

	// Tools for searching
	tools map[string]ToolExecutor

	// Private memory for this researcher
	roundSummaries []RoundSummary
	explorationLog []ExplorationStep
}

// RoundSummary is a summary of a single round
type RoundSummary struct {
	Round          int      `json:"round"`
	SearchedQuery  string   `json:"searched_query"`
	FoundDocuments int      `json:"found_documents"`
	CoveredTopics  []string `json:"covered_topics"`
	QualityScore   float64  `json:"quality_score"`
	SuggestedNext  string   `json:"suggested_next"`
	Summary        string   `json:"summary"`
}

// RoundSummary is a summary of a single round
type ResearcherAgentConfig struct {
	ID           string
	LLMProvider  llm.Provider
	SharedState  *SharedKnowledgeState
	InitialTopic string
	MaxRounds    int
	Tools        map[string]ToolExecutor
}

// NewResearcherAgent creates a new researcher agent
func NewResearcherAgent(config ResearcherAgentConfig) *ResearcherAgent {
	if config.MaxRounds == 0 {
		config.MaxRounds = 5
	}

	return &ResearcherAgent{
		id:             config.ID,
		agentType:      AgentTypeResearcher,
		state:          StateIdle,
		llmProvider:    config.LLMProvider,
		systemPrompt:   GetSubagentPrompt(AgentTypeResearcher),
		maxRounds:      config.MaxRounds,
		currentRound:   0,
		sharedState:    config.SharedState,
		tools:          config.Tools,
		roundSummaries: make([]RoundSummary, 0),
		explorationLog: make([]ExplorationStep, 0),
	}
}

// ID returns the agent ID
func (a *ResearcherAgent) ID() string {
	return a.id
}

// Type returns the agent type
func (a *ResearcherAgent) Type() AgentType {
	return a.agentType
}

// State returns the current state
func (a *ResearcherAgent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState sets the agent state
func (a *ResearcherAgent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// GetExplorationLog returns the exploration log
func (a *ResearcherAgent) GetExplorationLog() []ExplorationStep {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.explorationLog
}

// Run starts the researcher with an initial topic
func (a *ResearcherAgent) Run(ctx context.Context, initialTopic string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)

	// Register with shared state
	a.sharedState.RegisterResearcher(a.id, a.maxRounds, initialTopic)

	var totalDocsFound int
	var totalTokensIn, totalTokensOut int
	var consecutiveNoNewDocs int

	for a.currentRound < a.maxRounds {
		a.currentRound++
		a.SetState(StateRunning)
		a.sharedState.UpdateResearcherRound(a.id, a.currentRound, "searching")

		// 1. Read: Get current state
		state := a.sharedState.Read()
		stateSnapshot := a.buildStateSnapshot(state, a.currentRound)

		// 2. Think: LLM decides what to search
		decision, tokensIn, tokensOut, err := a.thinkAndDecide(ctx, stateSnapshot, a.currentRound)
		if err != nil {
			a.SetState(StateError)
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      initialTopic,
				Success:   false,
				Error:     fmt.Sprintf("LLM 思考失败: %v", err),
				Duration:  time.Since(startTime),
			}, err
		}

		totalTokensIn += tokensIn
		totalTokensOut += tokensOut

		// 3. Act: Execute the search
		a.sharedState.UpdateResearcherRound(a.id, a.currentRound, "processing")
		result, err := a.executeSearch(ctx, decision)
		if err != nil {
			// Record the error and continue
			a.roundSummaries = append(a.roundSummaries, RoundSummary{
				Round:          a.currentRound,
				SearchedQuery:  decision.Query,
				FoundDocuments: 0,
				QualityScore:   0,
				Summary:        fmt.Sprintf("搜索失败: %v", err),
			})
			continue
		}

		// 4. Observe: Analyze and extract documents
		docs, topics, quality := a.analyzeResult(result, decision.Query)
		newDocs := 0
		for i, doc := range docs {
			a.sharedState.Update(a.id, &doc, topics[i], quality[i])
			newDocs++
		}

		// Track consecutive rounds with no new documents
		if newDocs == 0 {
			consecutiveNoNewDocs++
		} else {
			consecutiveNoNewDocs = 0
		}

		// 5. Update: Record the round summary
		summary := a.summarizeRound(a.currentRound, decision, docs, topics, quality)
		a.roundSummaries = append(a.roundSummaries, summary)
		totalDocsFound += len(docs)

		// Log exploration step
		a.mu.Lock()
		a.explorationLog = append(a.explorationLog, ExplorationStep{
			Timestamp: time.Now(),
			Direction: a.inferDirection(decision.Query),
			Thought:   decision.Analysis,
			Action:    "search",
			ToolName:  decision.ToolName,
			Result:    summary.Summary,
			TokensIn:  tokensIn,
			TokensOut: tokensOut,
		})
		a.mu.Unlock()

		// 6. Check: Should we continue?
		if a.shouldComplete(consecutiveNoNewDocs) {
			break
		}
	}

	// Mark as complete
	a.SetState(StateCompleted)
	a.sharedState.UpdateResearcherRound(a.id, a.currentRound, "complete")

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      initialTopic,
		Success:   true,
		Output: map[string]any{
			"total_documents":  totalDocsFound,
			"total_rounds":     a.currentRound,
			"round_summaries":  a.roundSummaries,
			"exploration_log":  a.explorationLog,
		},
		Duration: time.Since(startTime),
		Metadata: map[string]any{
			"tokens_in":  totalTokensIn,
			"tokens_out": totalTokensOut,
		},
	}, nil
}

// ResearcherDecision is the decision made by the researcher
type ResearcherDecision struct {
	Analysis string `json:"analysis"`  // Analysis of current state
	Decision string `json:"decision"`  // Decision on what to do
	Query    string `json:"query"`     // Search query
	ToolName string `json:"tool_name"` // Tool to use
}

// StateSnapshot is a compact representation of the shared state
type StateSnapshot struct {
	Destination   string
	CoveredTopics []string
	MissingTopics []string
	CurrentRound  int
	MaxRounds     int
	PreviousWork  string
}

// buildStateSnapshot builds a compact snapshot of the shared state
func (a *ResearcherAgent) buildStateSnapshot(state *StateSummary, currentRound int) *StateSnapshot {
	var covered []string
	for _, t := range state.CoveredTopics {
		covered = append(covered, fmt.Sprintf("%s(%.0f%%, %d篇)", t.Name, t.Quality*100, t.DocumentCount))
	}

	var prevWork strings.Builder
	if len(a.roundSummaries) > 0 {
		for _, rs := range a.roundSummaries {
			prevWork.WriteString(fmt.Sprintf("轮次%d: %s -> 找到%d篇, 质量%.0f%%\n",
				rs.Round, rs.SearchedQuery, rs.FoundDocuments, rs.QualityScore*100))
		}
	} else {
		prevWork.WriteString("这是第一轮")
	}

	return &StateSnapshot{
		Destination:   state.Destination,
		CoveredTopics: covered,
		MissingTopics: state.MissingTopics,
		CurrentRound:  currentRound,
		MaxRounds:     a.maxRounds,
		PreviousWork:  prevWork.String(),
	}
}

// thinkAndDecide uses LLM to decide the next action
func (a *ResearcherAgent) thinkAndDecide(ctx context.Context, state *StateSnapshot, round int) (*ResearcherDecision, int, int, error) {
	// Topic name to Chinese keyword mapping with sub-topics for progressive exploration
	topicSubQueries := map[string][]string{
		"attractions": {
			"景点 名胜 古迹",      // Round 1: Main topic
			"著名景点 园林",       // Round 2: Famous attractions
			"古镇 水乡",          // Round 3: Ancient towns
			"博物馆 寺庙",        // Round 4: Museums and temples
			"自然风光 公园",      // Round 5: Nature and parks
		},
		"food": {
			"美食 小吃 料理",
			"特色菜 地方菜",
			"小吃街 美食街",
			"老字号 餐厅推荐",
			"特产 伴手礼",
		},
		"history": {
			"历史 文化 古迹",
			"历史人物 名人",
			"历史事件 故事",
			"文化遗产 非遗",
			"传统节日 习俗",
		},
		"transport": {
			"交通 机场 火车站",
			"地铁 公交",
			"出租车 网约车",
			"自驾 停车",
			"轮渡 游船",
		},
		"accommodation": {
			"住宿 酒店 民宿",
			"豪华酒店 星级酒店",
			"特色民宿 客栈",
			"青年旅舍 经济酒店",
			"酒店推荐 住宿攻略",
		},
		"entertainment": {
			"娱乐 夜生活 演出",
			"酒吧 夜店",
			"演出 表演",
			"游乐园 主题公园",
			"户外活动 体验",
		},
		"shopping": {
			"购物 商场 特产",
			"购物中心 百货",
			"步行街 商业街",
			"特产店 手工艺品",
			"免税店 奥特莱斯",
		},
		"practical": {
			"攻略 注意事项 实用信息",
			"最佳旅游时间 天气",
			"门票 开放时间",
			"旅游路线 行程安排",
			"安全 紧急电话",
		},
	}

	// Get this researcher's assigned topic
	assignedTopic := a.sharedState.GetResearcherTopic(a.id)

	// Progressive sub-topic search: Use different sub-queries for each round
	if subQueries, ok := topicSubQueries[assignedTopic]; ok {
		// Determine which sub-query to use based on round (0-indexed)
		queryIdx := (round - 1) % len(subQueries)
		if queryIdx < len(subQueries) {
			subQuery := subQueries[queryIdx]
			query := fmt.Sprintf("%s %s", state.Destination, subQuery)

			// Check if we already searched this exact query
			if !a.isAlreadySearched(query) {
				return &ResearcherDecision{
					Analysis: fmt.Sprintf("第%d轮探索 %s 的子主题: %s", round, state.Destination, subQuery),
					Decision:  fmt.Sprintf("搜索 %s 的 %s 信息", state.Destination, subQuery),
					Query:     query,
					ToolName:  "wikipedia_search",
				}, 0, 0, nil
			}
		}
	}

	// Fallback: Check missing topics from other areas
	if len(state.MissingTopics) > 0 {
		for _, t := range state.MissingTopics {
			if !a.isAlreadySearched(t) {
				// Get Chinese keywords for the topic
				keywords := t
				if subQueries, ok := topicSubQueries[t]; ok && len(subQueries) > 0 {
					keywords = subQueries[0]
				}
				query := fmt.Sprintf("%s %s", state.Destination, keywords)
				return &ResearcherDecision{
					Analysis: fmt.Sprintf("发现缺失主题: %s", t),
					Decision:  fmt.Sprintf("搜索 %s 的 %s 信息", state.Destination, t),
					Query:     query,
					ToolName:  "wikipedia_search",
				}, 0, 0, nil
			}
		}
	}

	// Use LLM for more complex decisions as final fallback
	prompt := fmt.Sprintf(`# 当前状态

目的地: %s
当前轮次: %d/%d
我负责的主题: %s

## 已覆盖主题
%s

## 缺失主题
%s

## 之前的工作
%s

## 决策
作为一个研究专家，分析当前状态并决定下一步搜索什么。

**重要**:
1. 继续深入探索我负责的主题，寻找更详细的信息
2. 如果我负责的主题已经很完善，可以探索相关子主题
3. 避免重复搜索相同的内容

输出 JSON:
{
  "analysis": "分析当前状态",
  "decision": "决定搜索什么",
  "query": "具体的搜索词（包含目的地名称）",
  "tool_name": "wikipedia_search 或 baidu_baike_search"
}`,
		state.Destination,
		round, state.MaxRounds,
		assignedTopic,
		strings.Join(state.CoveredTopics, ", "),
		strings.Join(state.MissingTopics, ", "),
		state.PreviousWork,
	)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := a.llmProvider.Complete(ctx, messages)
	if err != nil {
		return nil, 0, 0, err
	}

	// Parse the decision
	var decision ResearcherDecision
	if err := json.Unmarshal([]byte(resp.Content), &decision); err != nil {
		// Try to extract the JSON
		if idx := strings.Index(resp.Content, "{"); idx != -1 {
			endIdx := strings.LastIndex(resp.Content, "}")
			if endIdx > idx {
				json.Unmarshal([]byte(resp.Content[idx:endIdx+1]), &decision)
			}
		}
	}

	return &decision, resp.InputTokens, resp.OutputTokens, nil
}

// isAlreadySearched checks if a topic has been searched
func (a *ResearcherAgent) isAlreadySearched(topic string) bool {
	for _, rs := range a.roundSummaries {
		if strings.Contains(strings.ToLower(rs.SearchedQuery), strings.ToLower(topic)) {
			return true
		}
	}
	return false
}

// executeSearch executes the search using the available tools
func (a *ResearcherAgent) executeSearch(ctx context.Context, decision *ResearcherDecision) (map[string]any, error) {
	if a.tools == nil {
		return nil, fmt.Errorf("no tools available")
	}

	tool, exists := a.tools[decision.ToolName]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", decision.ToolName)
	}

	params := map[string]any{
		"query": decision.Query,
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		return nil, err
	}

	// Convert ToolResult to map[string]any
	if result.Data != nil {
		if data, ok := result.Data.(map[string]any); ok {
			return data, nil
		}
	}

	return map[string]any{
		"success": result.Success,
		"data":    result.Data,
		"error":   result.Error,
	}, nil
}

// analyzeResult analyzes search results and extracts documents
func (a *ResearcherAgent) analyzeResult(result map[string]any, query string) ([]Document, [][]string, []float64) {
	var docs []Document
	var topicsList [][]string
	var qualities []float64

	// Extract results from the search result
	// Results can be []any or a typed slice
	resultsRaw, ok := result["results"]
	if !ok {
		return docs, topicsList, qualities
	}

	// Convert to []any if needed
	var results []any
	switch v := resultsRaw.(type) {
	case []any:
		results = v
	case []map[string]any:
		for _, m := range v {
			results = append(results, m)
		}
	default:
		// Try reflection for typed slices
		results = convertToSlice(resultsRaw)
	}

	for _, r := range results {
		item, ok := r.(map[string]any)
		if !ok {
			continue
		}

		doc := Document{
			ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
			Title:   getString(item, "title"),
			Content: getString(item, "content"),
			URL:     getString(item, "url"),
			Source:  getString(item, "source"),
		}

		// Infer topics from the content
		topics := a.inferTopics(doc.Content, query)
		quality := a.assessQuality(doc.Content)

		docs = append(docs, doc)
		topicsList = append(topicsList, topics)
		qualities = append(qualities, quality)
	}

	return docs, topicsList, qualities
}

// convertToSlice converts a typed slice to []any using reflection
func convertToSlice(v any) []any {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	result := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i).Interface()
		// Convert struct to map[string]any
		itemMap := structToMap(item)
		result[i] = itemMap
	}
	return result
}

// structToMap converts a struct to map[string]any
func structToMap(v any) map[string]any {
	result := make(map[string]any)
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return result
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		result[strings.ToLower(field.Name)] = rv.Field(i).Interface()
	}
	return result
}

// getString safely extracts a string from a map
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// inferTopics infers topics from content
func (a *ResearcherAgent) inferTopics(content string, query string) []string {
	var topics []string
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)

	// Map of topic keywords
	topicKeywords := map[string][]string{
		"attractions": {"景点", "名胜", "古迹", "景区", "公园", "寺", "塔", "楼"},
		"food":       {"美食", "小吃", "菜", "餐厅", "料理", "特色"},
		"history":    {"历史", "文化", "朝代", "古代", "传说", "故事"},
		"transport":  {"交通", "地铁", "公交", "机场", "火车站", "巴士"},
		"accommodation": {"酒店", "住宿", "民宿", "宾馆"},
		"entertainment": {"娱乐", "演出", "夜生活", "酒吧"},
		"shopping":   {"购物", "商场", "特产", "市场"},
		"practical":  {"签证", "货币", "通讯", "安全", "注意事项"},
	}

	for topic, keywords := range topicKeywords {
		for _, kw := range keywords {
			if strings.Contains(contentLower, kw) || strings.Contains(queryLower, kw) {
				topics = append(topics, topic)
				break
			}
		}
	}

	if len(topics) == 0 {
		topics = append(topics, "general")
	}

	return topics
}

// assessQuality assesses the quality of a document
func (a *ResearcherAgent) assessQuality(content string) float64 {
	if len(content) < 100 {
		return 0.3
	}
	if len(content) < 300 {
		return 0.5
	}
	if len(content) < 500 {
		return 0.7
	}
	return 0.85
}

// summarizeRound creates a summary of the round
func (a *ResearcherAgent) summarizeRound(round int, decision *ResearcherDecision, docs []Document, topics [][]string, quality []float64) RoundSummary {
	var avgQuality float64
	if len(quality) > 0 {
		for _, q := range quality {
			avgQuality += q
		}
		avgQuality /= float64(len(quality))
	}

	// Get unique topics
	topicSet := make(map[string]bool)
	for _, t := range topics {
		for _, topic := range t {
			topicSet[topic] = true
		}
	}
	var uniqueTopics []string
	for t := range topicSet {
		uniqueTopics = append(uniqueTopics, t)
	}

	summary := fmt.Sprintf("搜索'%s'找到%d篇文档，覆盖主题: %s",
		decision.Query, len(docs), strings.Join(uniqueTopics, ", "))

	suggestedNext := ""
	state := a.sharedState.Read()
	if len(state.MissingTopics) > 0 {
		suggestedNext = state.MissingTopics[0]
	}

	return RoundSummary{
		Round:          round,
		SearchedQuery:  decision.Query,
		FoundDocuments: len(docs),
		CoveredTopics:  uniqueTopics,
		QualityScore:   avgQuality,
		SuggestedNext:  suggestedNext,
		Summary:        summary,
	}
}

// shouldComplete checks if the researcher should complete
func (a *ResearcherAgent) shouldComplete(consecutiveNoNewDocs int) bool {
	// Stop if we've had 2 consecutive rounds with no new documents
	if consecutiveNoNewDocs >= 2 {
		return true
	}

	// Don't stop too early - need at least 3 rounds to do thorough research
	if a.currentRound < 3 {
		return false
	}

	// Check if the researcher's OWN initial topic is well-covered
	// (not all topics - each researcher focuses on their own topic)
	state := a.sharedState.Read()

	// Find the coverage for this researcher's assigned topic
	for _, covered := range state.CoveredTopics {
		if covered.Name == a.sharedState.GetResearcherTopic(a.id) {
			// Only stop if our own topic has enough documents AND good quality
			if covered.DocumentCount >= 8 && covered.Quality >= 0.7 {
				return true
			}
		}
	}

	// Continue if we haven't covered our topic well enough
	return false
}

// inferDirection infers the research direction from the query
func (a *ResearcherAgent) inferDirection(query string) string {
	queryLower := strings.ToLower(query)

	directions := map[string][]string{
		"景点搜索": {"景点", "名胜", "景区", "公园"},
		"美食探索": {"美食", "小吃", "菜", "餐厅"},
		"历史研究": {"历史", "文化", "传说"},
		"交通查询": {"交通", "地铁", "公交", "机场"},
		"住宿信息": {"酒店", "住宿", "民宿"},
	}

	for dir, keywords := range directions {
		for _, kw := range keywords {
			if strings.Contains(queryLower, kw) {
				return dir
			}
		}
	}

	return "综合搜索"
}
