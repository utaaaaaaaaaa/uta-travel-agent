// Package agent provides agent management functionality
package agent

import (
	"context"
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
	StatusError    AgentStatus = "error"
)

// DestinationAgent represents a persisted destination agent
type DestinationAgent struct {
	ID                 string      `json:"id"`
	UserID             string      `json:"user_id"`
	Name               string      `json:"name"`
	Description        string      `json:"description"`
	Destination        string      `json:"destination"`
	VectorCollectionID string      `json:"vector_collection_id"`
	TaskID             string      `json:"task_id"`
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
// It supports both in-memory and database-backed storage
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*DestinationAgent

	// Optional repository for persistent storage
	repo Repository
}

// NewRegistry creates a new agent registry (in-memory only)
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*DestinationAgent),
	}
}

// NewRegistryWithRepo creates a new agent registry with database backend
func NewRegistryWithRepo(repo Repository) *Registry {
	return &Registry{
		agents: make(map[string]*DestinationAgent),
		repo:   repo,
	}
}

// Register adds a new agent to the registry
func (r *Registry) Register(agent *DestinationAgent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	agent.Status = StatusReady

	r.agents[agent.ID] = agent

	// Persist to database if repository is set
	if r.repo != nil {
		ctx := context.Background()
		return r.repo.SaveAgent(ctx, agent)
	}

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

	// Persist to database if repository is set
	if r.repo != nil {
		ctx := context.Background()
		return r.repo.UpdateAgent(ctx, agent)
	}

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

	// Delete from database if repository is set
	if r.repo != nil {
		ctx := context.Background()
		return r.repo.DeleteAgent(ctx, id)
	}

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

// LoadFromRepository loads all agents from the database into memory
// This should be called on startup when using persistent storage
func (r *Registry) LoadFromRepository(ctx context.Context, userID string) error {
	if r.repo == nil {
		return nil
	}

	var agents []*DestinationAgent
	var err error

	if userID != "" {
		agents, err = r.repo.ListAgentsByUser(ctx, userID)
	} else {
		// Load all agents if userID is empty
		agents, err = r.repo.ListAllAgents(ctx)
	}

	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, agent := range agents {
		r.agents[agent.ID] = agent
	}

	return nil
}

// LoadAllFromRepository loads all agents from the database
func (r *Registry) LoadAllFromRepository(ctx context.Context) error {
	return r.LoadFromRepository(ctx, "")
}
