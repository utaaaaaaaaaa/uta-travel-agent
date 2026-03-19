package agent

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/llm"
)

// AgentFactory creates agents based on templates
type AgentFactory struct {
	templateRegistry *TemplateRegistry
	toolRegistry     ToolRegistry
	llmProvider      llm.Provider
	idCounter        int64
}

// NewAgentFactory creates a new agent factory
func NewAgentFactory(templateRegistry *TemplateRegistry, toolRegistry ToolRegistry, llmProvider llm.Provider) *AgentFactory {
	return &AgentFactory{
		templateRegistry: templateRegistry,
		toolRegistry:     toolRegistry,
		llmProvider:      llmProvider,
		idCounter:        time.Now().Unix(),
	}
}

// generateAgentID generates a unique agent ID
func (f *AgentFactory) generateAgentID() string {
	id := atomic.AddInt64(&f.idCounter, 1)
	return fmt.Sprintf("agent-%d", id)
}

// CreateAgent creates an agent of the specified type
func (f *AgentFactory) CreateAgent(agentType AgentType) (Agent, error) {
	template, err := f.templateRegistry.Get(agentType)
	if err != nil {
		return nil, fmt.Errorf("failed to get template for %s: %w", agentType, err)
	}

	return f.CreateAgentFromTemplate(agentType, template)
}

// CreateAgentFromTemplate creates an agent from a template
func (f *AgentFactory) CreateAgentFromTemplate(agentType AgentType, template *AgentTemplate) (Agent, error) {
	id := f.generateAgentID()

	var agent Agent

	switch agentType {
	case AgentTypeMain:
		agent = NewMainAgent(MainAgentConfig{
			ID:          id,
			Template:    template,
			LLMProvider: f.llmProvider,
		})
	case AgentTypeResearcher:
		agent = NewResearcherAgent(id, template)
	case AgentTypeCurator:
		agent = NewCuratorAgent(id, template)
	case AgentTypeIndexer:
		agent = NewIndexerAgent(id, template)
	case AgentTypeGuide:
		agent = NewGuideAgent(id, template, "")
	case AgentTypePlanner:
		agent = NewPlannerAgent(id, template)
	default:
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}

	// Set up tools if available
	if f.toolRegistry != nil {
		switch a := agent.(type) {
		case *MainAgent:
			a.BaseAgent.SetTools(f.toolRegistry)
		case *ResearcherAgent:
			a.BaseAgent.SetTools(f.toolRegistry)
		case *CuratorAgent:
			a.BaseAgent.SetTools(f.toolRegistry)
		case *IndexerAgent:
			a.BaseAgent.SetTools(f.toolRegistry)
		case *GuideAgent:
			a.BaseAgent.SetTools(f.toolRegistry)
		case *PlannerAgent:
			a.BaseAgent.SetTools(f.toolRegistry)
		}
	}

	return agent, nil
}

// CreateMainAgentWithSubagents creates a main agent with all its subagents
func (f *AgentFactory) CreateMainAgentWithSubagents() (*MainAgent, error) {
	// Create main agent
	mainAgentInterface, err := f.CreateAgent(AgentTypeMain)
	if err != nil {
		return nil, fmt.Errorf("failed to create main agent: %w", err)
	}

	mainAgent, ok := mainAgentInterface.(*MainAgent)
	if !ok {
		return nil, fmt.Errorf("unexpected agent type for main agent")
	}

	// Create and register subagents
	subagentTypes := []AgentType{
		AgentTypeResearcher,
		AgentTypeCurator,
		AgentTypeIndexer,
		AgentTypeGuide,
		AgentTypePlanner,
	}

	for _, subagentType := range subagentTypes {
		subagent, err := f.CreateAgent(subagentType)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s subagent: %w", subagentType, err)
		}

		if err := mainAgent.RegisterSubagent(subagent); err != nil {
			return nil, fmt.Errorf("failed to register %s subagent: %w", subagentType, err)
		}
	}

	return mainAgent, nil
}

// CreateDestinationAgent creates a complete destination agent system
func (f *AgentFactory) CreateDestinationAgent(destination string, userID string) (*DestinationAgent, *MainAgent, error) {
	// Create main agent with subagents
	mainAgent, err := f.CreateMainAgentWithSubagents()
	if err != nil {
		return nil, nil, err
	}

	// Create destination agent metadata
	now := time.Now()
	destAgent := &DestinationAgent{
		ID:                 f.generateAgentID(),
		UserID:             userID,
		Name:               fmt.Sprintf("%s旅游助手", destination),
		Description:        fmt.Sprintf("专业的%s旅游信息助手", destination),
		Destination:        destination,
		VectorCollectionID: fmt.Sprintf("%s-%d", destination, now.Unix()),
		DocumentCount:      0,
		Language:           "zh",
		Tags:               []string{},
		Theme:              "cultural",
		Status:             StatusCreating,
		CreatedAt:          now,
		UpdatedAt:          now,
		LastUsedAt:         nil,
		UsageCount:         0,
		Rating:             0,
	}

	return destAgent, mainAgent, nil
}

// CreateGuideAgentForDestination creates a guide agent for an existing destination
func (f *AgentFactory) CreateGuideAgentForDestination(destAgent *DestinationAgent) (Agent, error) {
	template, err := f.templateRegistry.Get(AgentTypeGuide)
	if err != nil {
		return nil, err
	}

	guide := NewGuideAgent(f.generateAgentID(), template, destAgent.VectorCollectionID)

	if f.toolRegistry != nil {
		guide.BaseAgent.SetTools(f.toolRegistry)
	}

	return guide, nil
}
