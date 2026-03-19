// Package agent provides agent management functionality
package agent

import (
	"sync"
	"time"
)

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	StatusCreating AgentStatus = "creating"
	StatusReady    AgentStatus = "ready"
	StatusBusy     AgentStatus = "busy"
	StatusArchived AgentStatus = "archived"
)

// DestinationAgent represents a persisted destination agent
type DestinationAgent struct {
	ID                 string      `json:"id"`
	UserID             string      `json:"user_id"`
	Name               string      `json:"name"`
	Description        string      `json:"description"`
	Destination        string      `json:"destination"`
	VectorCollectionID string      `json:"vector_collection_id"`
	DocumentCount      int         `json:"document_count"`
	Language           string      `json:"language"`
	Tags               []string    `json:"tags"`
	Theme              string      `json:"theme"`
	Status             AgentStatus `json:"status"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	LastUsedAt         *time.Time  `json:"last_used_at"`
	UsageCount         int         `json:"usage_count"`
	Rating             float64     `json:"rating"`
}

// Registry manages all agents in the system
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*DestinationAgent
}

// NewRegistry creates a new agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*DestinationAgent),
	}
}

// Register adds a new agent to the registry
func (r *Registry) Register(agent *DestinationAgent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()
	agent.Status = StatusReady

	r.agents[agent.ID] = agent
	return nil
}

// Get retrieves an agent by ID
func (r *Registry) Get(id string) (*DestinationAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[id]
	return agent, exists
}

// GetByUserID retrieves all agents for a user
func (r *Registry) GetByUserID(userID string) []*DestinationAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agents []*DestinationAgent
	for _, agent := range r.agents {
		if agent.UserID == userID {
			agents = append(agents, agent)
		}
	}
	return agents
}

// Update modifies an existing agent
func (r *Registry) Update(agent *DestinationAgent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID]; !exists {
		return ErrAgentNotFound
	}

	agent.UpdatedAt = time.Now()
	r.agents[agent.ID] = agent
	return nil
}

// Delete removes an agent from the registry
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[id]; !exists {
		return ErrAgentNotFound
	}

	delete(r.agents, id)
	return nil
}

// List returns all agents
func (r *Registry) List() []*DestinationAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*DestinationAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}
