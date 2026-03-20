package agent

import (
	"context"
	"time"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// AgentTask represents a task execution record
type AgentTask struct {
	ID              string             `json:"id"`
	AgentID         string             `json:"agent_id"`
	UserID          string             `json:"user_id"`
	Status          TaskStatus         `json:"status"`
	Goal            string             `json:"goal"`
	Result          map[string]any     `json:"result,omitempty"`
	Error           string             `json:"error,omitempty"`
	DurationSeconds float64            `json:"duration_seconds"`
	TotalTokens     int                `json:"total_tokens"`
	ExplorationLog  []ExplorationStep  `json:"exploration_log,omitempty"`
	RadarData       *RadarData         `json:"radar_data,omitempty"`
	Metadata        map[string]any     `json:"metadata,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	StartedAt       *time.Time         `json:"started_at,omitempty"`
	CompletedAt     *time.Time         `json:"completed_at,omitempty"`
}

// RadarData represents aggregated radar chart data
type RadarData struct {
	Directions []RadarDirection `json:"directions"`
}

// RadarDirection represents one direction in the radar chart
type RadarDirection struct {
	Name       string    `json:"name"`
	Value      int       `json:"value"`
	LastUpdate time.Time `json:"last_update"`
}

// Repository defines the interface for agent persistence
type Repository interface {
	// Agent operations
	SaveAgent(ctx context.Context, agent *DestinationAgent) error
	GetAgent(ctx context.Context, id string) (*DestinationAgent, error)
	ListAgentsByUser(ctx context.Context, userID string) ([]*DestinationAgent, error)
	ListAllAgents(ctx context.Context) ([]*DestinationAgent, error)
	ListAgents(ctx context.Context, limit, offset int) ([]*DestinationAgent, error)
	UpdateAgent(ctx context.Context, agent *DestinationAgent) error
	DeleteAgent(ctx context.Context, id string) error

	// Task operations
	SaveTask(ctx context.Context, task *AgentTask) error
	GetTask(ctx context.Context, id string) (*AgentTask, error)
	ListTasksByAgent(ctx context.Context, agentID string, limit int) ([]*AgentTask, error)
	ListTasksByUser(ctx context.Context, userID string, limit int) ([]*AgentTask, error)
	UpdateTask(ctx context.Context, task *AgentTask) error
	DeleteTask(ctx context.Context, id string) error
}
