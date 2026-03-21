// Package agent provides the core agent implementation
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

// CuratorAgent is a specialized agent for curating and evaluating documents
// It reads from SharedKnowledgeState and evaluates document quality
type CuratorAgent struct {
	mu           sync.RWMutex
	id           string
	agentType    AgentType
	state        AgentState
	llmProvider  llm.Provider
	systemPrompt string
	maxRounds    int
	currentRound int

	// Shared state reference
	sharedState *SharedKnowledgeState

	// Evaluation results
	evaluationResults []DocumentEvaluation
	qualityScore      float64
}

// DocumentEvaluation represents the evaluation result for a document
type DocumentEvaluation struct {
	DocumentID    string   `json:"document_id"`
	Title         string   `json:"title"`
	QualityScore  float64  `json:"quality_score"`
	CoveredTopics []string `json:"covered_topics"`
	IsHighQuality bool     `json:"is_high_quality"`
	Issues        []string `json:"issues,omitempty"`
}

// CuratorResult represents the overall curation result
type CuratorResult struct {
	OverallQuality    float64              `json:"overall_quality"`
	DocumentsEvaluated int                 `json:"documents_evaluated"`
	HighQualityCount  int                  `json:"high_quality_count"`
	DuplicatesFound   int                  `json:"duplicates_found"`
	NeedsMoreSearch   bool                 `json:"needs_more_search"`
	MissingAspects    []string             `json:"missing_aspects"`
	IsComplete        bool                 `json:"is_complete"`
	Evaluations       []DocumentEvaluation `json:"evaluations"`
}

// CuratorAgentConfig for creating a curator agent
type CuratorAgentConfig struct {
	ID           string
	LLMProvider  llm.Provider
	SharedState  *SharedKnowledgeState
	MaxRounds    int
}

// NewCuratorAgent creates a new curator agent
func NewCuratorAgent(config CuratorAgentConfig) *CuratorAgent {
	if config.MaxRounds == 0 {
		config.MaxRounds = 3
	}

	return &CuratorAgent{
		id:                config.ID,
		agentType:         AgentTypeCurator,
		state:             StateIdle,
		llmProvider:       config.LLMProvider,
		systemPrompt:      GetSubagentPrompt(AgentTypeCurator),
		maxRounds:         config.MaxRounds,
		currentRound:      0,
		sharedState:       config.SharedState,
		evaluationResults: make([]DocumentEvaluation, 0),
		qualityScore:      0,
	}
}

// ID returns the agent ID
func (a *CuratorAgent) ID() string {
	return a.id
}

// Type returns the agent type
func (a *CuratorAgent) Type() AgentType {
	return a.agentType
}

// State returns the current state
func (a *CuratorAgent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState sets the agent state
func (a *CuratorAgent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// Run starts the curator agent
func (a *CuratorAgent) Run(ctx context.Context, _ string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)

	var totalTokensIn, totalTokensOut int
	var lastQualityScore float64
	var noImprovementCount int

	for a.currentRound < a.maxRounds {
		a.currentRound++
		a.SetState(StateRunning)

		// 1. Read: Get all documents from shared state
		documents := a.sharedState.GetDocuments()
		state := a.sharedState.Read()

		if len(documents) == 0 {
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      "curate documents",
				Success:   false,
				Error:     "no documents to curate",
				Duration:  time.Since(startTime),
			}, fmt.Errorf("no documents to curate")
		}

		// 2. Evaluate: Use LLM to evaluate document quality
		result, tokensIn, tokensOut, err := a.evaluateDocuments(ctx, documents, state)
		if err != nil {
			a.SetState(StateError)
			return &AgentResult{
				AgentID:   a.ID(),
				AgentType: a.Type(),
				Goal:      "curate documents",
				Success:   false,
				Error:     fmt.Sprintf("Evaluation failed: %v", err),
				Duration:  time.Since(startTime),
			}, err
		}

		totalTokensIn += tokensIn
		totalTokensOut += tokensOut
		a.qualityScore = result.OverallQuality
		a.evaluationResults = result.Evaluations

		// 3. Check: Should we continue?
		if result.OverallQuality >= 0.7 {
			result.IsComplete = true
			break
		}

		// Check for no improvement
		if result.OverallQuality <= lastQualityScore {
			noImprovementCount++
			if noImprovementCount >= 2 {
				break
			}
		} else {
			noImprovementCount = 0
		}
		lastQualityScore = result.OverallQuality

		// 4. If needs more search, we could trigger additional research
		// For now, we just mark it and continue
		if !result.NeedsMoreSearch {
			break
		}
	}

	// Mark as complete
	a.SetState(StateCompleted)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      "curate documents",
		Success:   true,
		Output: map[string]any{
			"quality_score":       a.qualityScore,
			"documents_evaluated": len(a.evaluationResults),
			"evaluations":         a.evaluationResults,
			"rounds":              a.currentRound,
		},
		Duration: time.Since(startTime),
		Metadata: map[string]any{
			"tokens_in":  totalTokensIn,
			"tokens_out": totalTokensOut,
		},
	}, nil
}

// evaluateDocuments evaluates all documents using LLM
func (a *CuratorAgent) evaluateDocuments(ctx context.Context, documents []Document, state *StateSummary) (*CuratorResult, int, int, error) {
	// Build document summaries for evaluation
	var docSummaries strings.Builder
	for i, doc := range documents {
		if i >= 20 { // Limit to first 20 documents for evaluation
			break
		}
		docSummaries.WriteString(fmt.Sprintf("- [%d] %s: %s (来源: %s, 长度: %d字)\n",
			i+1, doc.Title, truncate(doc.Content, 100), doc.Source, len(doc.Content)))
	}

	// Build topic coverage summary
	var topicSummaries strings.Builder
	for _, t := range state.CoveredTopics {
		topicSummaries.WriteString(fmt.Sprintf("- %s: %d篇, 质量%.0f%%\n",
			t.Name, t.DocumentCount, t.Quality*100))
	}

	prompt := fmt.Sprintf(`# 旅游信息整理专家

## 当前任务
目的地: %s
已收集文档: %d篇

## 主题覆盖情况
%s

缺失主题: %s

## 文档摘要 (前20篇)
%s

## 你的任务
1. 评估文档整体质量 (0-1分)
2. 识别重复或低质量内容
3. 判断知识库是否完整

## 质量评估标准
- 准确性: 信息是否具体、可验证
- 完整性: 是否覆盖主题的关键信息
- 时效性: 信息是否仍然有效
- 来源可靠性: 来源是否权威

## 输出格式 (JSON)
{
  "quality_score": 0.75,
  "documents_evaluated": 12,
  "high_quality_count": 8,
  "duplicates_found": 2,
  "needs_more_search": true,
  "missing_aspects": ["交通详情", "住宿推荐"],
  "is_complete": false
}`,
		state.Destination,
		len(documents),
		topicSummaries.String(),
		strings.Join(state.MissingTopics, ", "),
		docSummaries.String(),
	)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := a.llmProvider.Complete(ctx, messages)
	if err != nil {
		return nil, 0, 0, err
	}

	// Parse the result
	var result CuratorResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		// Try to extract JSON
		if idx := strings.Index(resp.Content, "{"); idx != -1 {
			endIdx := strings.LastIndex(resp.Content, "}")
			if endIdx > idx {
				json.Unmarshal([]byte(resp.Content[idx:endIdx+1]), &result)
			}
		}
	}

	// Generate document evaluations based on documents
	result.Evaluations = a.generateEvaluations(documents, result.OverallQuality)
	result.DocumentsEvaluated = len(documents)

	// Count high quality documents
	highQuality := 0
	for _, eval := range result.Evaluations {
		if eval.IsHighQuality {
			highQuality++
		}
	}
	result.HighQualityCount = highQuality

	return &result, resp.InputTokens, resp.OutputTokens, nil
}

// generateEvaluations generates evaluation results for documents
func (a *CuratorAgent) generateEvaluations(documents []Document, overallQuality float64) []DocumentEvaluation {
	evaluations := make([]DocumentEvaluation, 0, len(documents))

	for _, doc := range documents {
		eval := DocumentEvaluation{
			DocumentID:    doc.ID,
			Title:         doc.Title,
			QualityScore:  doc.Quality,
			CoveredTopics: doc.Topics,
			IsHighQuality: doc.Quality >= 0.7,
		}

		// Identify issues
		if len(doc.Content) < 100 {
			eval.Issues = append(eval.Issues, "内容过短")
			eval.IsHighQuality = false
		}
		if doc.Source == "" {
			eval.Issues = append(eval.Issues, "来源不明")
		}

		evaluations = append(evaluations, eval)
	}

	return evaluations
}

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}