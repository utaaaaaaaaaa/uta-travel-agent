package agent

import (
	"context"
	"fmt"
	"time"
)

// ResearcherAgent collects travel information using MCP tools
type ResearcherAgent struct {
	*BaseAgent
}

// NewResearcherAgent creates a new researcher agent
func NewResearcherAgent(id string, template *AgentTemplate) *ResearcherAgent {
	return &ResearcherAgent{
		BaseAgent: NewBaseAgent(id, AgentTypeResearcher, template),
	}
}

// Stop stops the researcher agent
func (a *ResearcherAgent) Stop() error {
	a.SetState(StateIdle)
	return nil
}

// Run starts the researcher agent
func (a *ResearcherAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	a.Memory().AddThought(fmt.Sprintf("研究目标: %s", goal))

	// Step 1: Search for information
	a.SetState(StateRunning)
	a.Memory().AddAction("brave_search", map[string]any{"query": goal})

	searchResult, err := a.ExecuteTool(ctx, "brave_search", map[string]any{
		"query": goal,
	})
	if err != nil {
		a.SetState(StateError)
		return &AgentResult{
			AgentID:   a.ID(),
			AgentType: a.Type(),
			Goal:      goal,
			Success:   false,
			Error:     fmt.Sprintf("搜索失败: %v", err),
			Duration:  time.Since(startTime),
		}, err
	}

	a.Memory().AddObservation(fmt.Sprintf("搜索结果: %v", searchResult), "brave_search")

	// Step 2: Read web pages for detailed information
	var documents []any
	if urls, ok := extractURLs(searchResult); ok && len(urls) > 0 {
		for i, url := range urls {
			if i >= 5 { // Limit to 5 URLs
				break
			}
			webResult, err := a.ExecuteTool(ctx, "web_reader", map[string]any{
				"url": url,
			})
			if err == nil {
				documents = append(documents, webResult)
				a.Memory().AddObservation(fmt.Sprintf("读取网页: %s", url), "web_reader")
			}
		}
	}

	// Step 3: Extract travel information using skill
	extractResult, err := a.ExecuteTool(ctx, "extract_travel_info", map[string]any{
		"documents": documents,
		"goal":      goal,
	})
	if err != nil {
		a.Memory().AddThought(fmt.Sprintf("信息提取失败: %v，使用原始文档", err))
		extractResult = documents
	} else {
		a.Memory().AddObservation("信息提取完成", "extract_travel_info")
	}

	a.SetState(StateCompleted)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   true,
		Output: map[string]any{
			"search_results": searchResult,
			"documents":      documents,
			"extracted":      extractResult,
		},
		Duration: time.Since(startTime),
		Metadata: map[string]any{
			"urls_processed": len(documents),
		},
	}, nil
}

// CuratorAgent organizes and structures collected information
type CuratorAgent struct {
	*BaseAgent
}

// NewCuratorAgent creates a new curator agent
func NewCuratorAgent(id string, template *AgentTemplate) *CuratorAgent {
	return &CuratorAgent{
		BaseAgent: NewBaseAgent(id, AgentTypeCurator, template),
	}
}

// Run starts the curator agent
func (a *CuratorAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	a.Memory().AddThought(fmt.Sprintf("整理目标: %s", goal))

	// The curator receives data from the researcher and organizes it
	// This would typically involve:
	// 1. Categorizing information (attractions, restaurants, transport)
	// 2. Removing duplicates
	// 3. Building a knowledge graph
	// 4. Validating data quality

	a.SetState(StateRunning)

	// Use build_knowledge_base skill
	result, err := a.ExecuteTool(ctx, "build_knowledge_base", map[string]any{
		"goal": goal,
	})
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

	a.Memory().AddResult("知识库整理完成", true, nil)
	a.SetState(StateCompleted)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   true,
		Output:    result,
		Duration:  time.Since(startTime),
	}, nil
}

// Stop stops the curator agent
func (a *CuratorAgent) Stop() error {
	a.SetState(StateIdle)
	return nil
}

// IndexerAgent builds vector indexes for RAG
type IndexerAgent struct {
	*BaseAgent
}

// NewIndexerAgent creates a new indexer agent
func NewIndexerAgent(id string, template *AgentTemplate) *IndexerAgent {
	return &IndexerAgent{
		BaseAgent: NewBaseAgent(id, AgentTypeIndexer, template),
	}
}

// Run starts the indexer agent
func (a *IndexerAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	a.Memory().AddThought(fmt.Sprintf("索引目标: %s", goal))

	a.SetState(StateRunning)

	// Use build_knowledge_index skill
	result, err := a.ExecuteTool(ctx, "build_knowledge_index", map[string]any{
		"goal": goal,
	})
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

	a.Memory().AddResult("向量索引构建完成", true, nil)
	a.SetState(StateCompleted)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   true,
		Output:    result,
		Duration:  time.Since(startTime),
	}, nil
}

// Stop stops the indexer agent
func (a *IndexerAgent) Stop() error {
	a.SetState(StateIdle)
	return nil
}

// GuideAgent provides real-time tour guidance
type GuideAgent struct {
	*BaseAgent
	collectionID string
}

// NewGuideAgent creates a new guide agent
func NewGuideAgent(id string, template *AgentTemplate, collectionID string) *GuideAgent {
	return &GuideAgent{
		BaseAgent:     NewBaseAgent(id, AgentTypeGuide, template),
		collectionID: collectionID,
	}
}

// Run starts the guide agent
func (a *GuideAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	a.Memory().AddThought(fmt.Sprintf("导游请求: %s", goal))

	a.SetState(StateRunning)

	// Use RAG query to get relevant information
	result, err := a.ExecuteTool(ctx, "rag_query", map[string]any{
		"query":      goal,
		"collection": a.collectionID,
	})
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

	a.Memory().AddResult("导游讲解完成", true, nil)
	a.SetState(StateCompleted)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   true,
		Output:    result,
		Duration:  time.Since(startTime),
	}, nil
}

// Stop stops the guide agent
func (a *GuideAgent) Stop() error {
	a.SetState(StateIdle)
	return nil
}

// PlannerAgent generates travel itineraries
type PlannerAgent struct {
	*BaseAgent
}

// NewPlannerAgent creates a new planner agent
func NewPlannerAgent(id string, template *AgentTemplate) *PlannerAgent {
	return &PlannerAgent{
		BaseAgent: NewBaseAgent(id, AgentTypePlanner, template),
	}
}

// Run starts the planner agent
func (a *PlannerAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
	startTime := time.Now()
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	a.Memory().AddThought(fmt.Sprintf("行程规划: %s", goal))

	a.SetState(StateRunning)

	// Use itinerary_planner skill
	result, err := a.ExecuteTool(ctx, "itinerary_planner", map[string]any{
		"goal": goal,
	})
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

	a.Memory().AddResult("行程规划完成", true, nil)
	a.SetState(StateCompleted)

	return &AgentResult{
		AgentID:   a.ID(),
		AgentType: a.Type(),
		Goal:      goal,
		Success:   true,
		Output:    result,
		Duration:  time.Since(startTime),
	}, nil
}

// Stop stops the planner agent
func (a *PlannerAgent) Stop() error {
	a.SetState(StateIdle)
	return nil
}

// Helper function to extract URLs from search results
func extractURLs(result any) ([]string, bool) {
	if result == nil {
		return nil, false
	}

	// Handle different result formats
	switch v := result.(type) {
	case map[string]any:
		if urls, ok := v["urls"].([]string); ok {
			return urls, true
		}
		if urls, ok := v["urls"].([]any); ok {
			var result []string
			for _, u := range urls {
				if s, ok := u.(string); ok {
					result = append(result, s)
				}
			}
			return result, true
		}
	case []any:
		var urls []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if url, ok := m["url"].(string); ok {
					urls = append(urls, url)
				}
				if link, ok := m["link"].(string); ok {
					urls = append(urls, link)
				}
			}
		}
		return urls, true
	}

	return nil, false
}
