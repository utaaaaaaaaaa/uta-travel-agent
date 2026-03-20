// Package postgres provides PostgreSQL database operations
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/agent"
)

// AgentRepository implements agent.Repository using PostgreSQL
type AgentRepository struct {
	db *sql.DB
}

// NewAgentRepository creates a new agent repository
func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// SaveAgent saves a new agent to the database
func (r *AgentRepository) SaveAgent(ctx context.Context, ag *agent.DestinationAgent) error {
	query := `
		INSERT INTO destination_agents (
			id, user_id, name, description, destination,
			vector_collection_id, document_count, language, theme,
			status, tags, created_at, updated_at, last_used_at,
			usage_count, rating
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	tagsJSON, _ := json.Marshal(ag.Tags)

	var lastUsedAt interface{}
	if ag.LastUsedAt != nil {
		lastUsedAt = ag.LastUsedAt
	}

	_, err := r.db.ExecContext(ctx, query,
		ag.ID, ag.UserID, ag.Name, ag.Description, ag.Destination,
		ag.VectorCollectionID, ag.DocumentCount, ag.Language, ag.Theme,
		ag.Status, tagsJSON, ag.CreatedAt, ag.UpdatedAt, lastUsedAt,
		ag.UsageCount, ag.Rating,
	)

	return err
}

// GetAgent retrieves an agent by ID
func (r *AgentRepository) GetAgent(ctx context.Context, id string) (*agent.DestinationAgent, error) {
	query := `
		SELECT id, user_id, name, description, destination,
			   vector_collection_id, document_count, language, theme,
			   status, tags, created_at, updated_at, last_used_at,
			   usage_count, rating
		FROM destination_agents
		WHERE id = $1
	`

	var ag agent.DestinationAgent
	var tagsJSON []byte
	var lastUsedAt sql.NullTime
	var description, vectorCollectionID sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&ag.ID, &ag.UserID, &ag.Name, &description, &ag.Destination,
		&vectorCollectionID, &ag.DocumentCount, &ag.Language, &ag.Theme,
		&ag.Status, &tagsJSON, &ag.CreatedAt, &ag.UpdatedAt, &lastUsedAt,
		&ag.UsageCount, &ag.Rating,
	)

	if err == sql.ErrNoRows {
		return nil, agent.ErrAgentNotFound
	}
	if err != nil {
		return nil, err
	}

	ag.Description = description.String
	ag.VectorCollectionID = vectorCollectionID.String
	json.Unmarshal(tagsJSON, &ag.Tags)

	if lastUsedAt.Valid {
		ag.LastUsedAt = &lastUsedAt.Time
	}

	return &ag, nil
}

// ListAgentsByUser retrieves all agents for a user
func (r *AgentRepository) ListAgentsByUser(ctx context.Context, userID string) ([]*agent.DestinationAgent, error) {
	query := `
		SELECT id, user_id, name, description, destination,
			   vector_collection_id, document_count, language, theme,
			   status, tags, created_at, updated_at, last_used_at,
			   usage_count, rating
		FROM destination_agents
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	return r.queryAgents(ctx, query, userID)
}

// ListAllAgents retrieves all agents (for admin/system use)
func (r *AgentRepository) ListAllAgents(ctx context.Context) ([]*agent.DestinationAgent, error) {
	query := `
		SELECT id, user_id, name, description, destination,
			   vector_collection_id, document_count, language, theme,
			   status, tags, created_at, updated_at, last_used_at,
			   usage_count, rating
		FROM destination_agents
		ORDER BY created_at DESC
	`

	return r.queryAgents(ctx, query)
}

// ListAgents retrieves all agents with pagination
func (r *AgentRepository) ListAgents(ctx context.Context, limit, offset int) ([]*agent.DestinationAgent, error) {
	query := `
		SELECT id, user_id, name, description, destination,
			   vector_collection_id, document_count, language, theme,
			   status, tags, created_at, updated_at, last_used_at,
			   usage_count, rating
		FROM destination_agents
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	return r.queryAgents(ctx, query, limit, offset)
}

// UpdateAgent updates an existing agent
func (r *AgentRepository) UpdateAgent(ctx context.Context, ag *agent.DestinationAgent) error {
	query := `
		UPDATE destination_agents SET
			name = $2, description = $3, vector_collection_id = $4,
			document_count = $5, language = $6, theme = $7,
			status = $8, tags = $9, updated_at = $10,
			last_used_at = $11, usage_count = $12, rating = $13
		WHERE id = $1
	`

	tagsJSON, _ := json.Marshal(ag.Tags)

	var lastUsedAt interface{}
	if ag.LastUsedAt != nil {
		lastUsedAt = ag.LastUsedAt
	}

	result, err := r.db.ExecContext(ctx, query,
		ag.ID, ag.Name, ag.Description, ag.VectorCollectionID,
		ag.DocumentCount, ag.Language, ag.Theme, ag.Status,
		tagsJSON, time.Now(), lastUsedAt, ag.UsageCount, ag.Rating,
	)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return agent.ErrAgentNotFound
	}

	return nil
}

// DeleteAgent removes an agent from the database
func (r *AgentRepository) DeleteAgent(ctx context.Context, id string) error {
	query := `DELETE FROM destination_agents WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return agent.ErrAgentNotFound
	}

	return nil
}

// SaveTask saves a new task to the database
func (r *AgentRepository) SaveTask(ctx context.Context, task *agent.AgentTask) error {
	query := `
		INSERT INTO agent_tasks (
			id, agent_id, user_id, status, goal, result, error,
			duration_seconds, total_tokens, exploration_log, radar_data,
			metadata, created_at, started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	resultJSON, _ := json.Marshal(task.Result)
	explorationLogJSON, _ := json.Marshal(task.ExplorationLog)
	radarDataJSON, _ := json.Marshal(task.RadarData)
	metadataJSON, _ := json.Marshal(task.Metadata)

	_, err := r.db.ExecContext(ctx, query,
		task.ID, task.AgentID, task.UserID, task.Status, task.Goal,
		resultJSON, task.Error, task.DurationSeconds, task.TotalTokens,
		explorationLogJSON, radarDataJSON, metadataJSON,
		task.CreatedAt, task.StartedAt, task.CompletedAt,
	)

	return err
}

// GetTask retrieves a task by ID
func (r *AgentRepository) GetTask(ctx context.Context, id string) (*agent.AgentTask, error) {
	query := `
		SELECT id, agent_id, user_id, status, goal, result, error,
			   duration_seconds, total_tokens, exploration_log, radar_data,
			   metadata, created_at, started_at, completed_at
		FROM agent_tasks
		WHERE id = $1
	`

	var task agent.AgentTask
	var resultJSON, explorationLogJSON, radarDataJSON, metadataJSON []byte
	var goal, taskError sql.NullString
	var startedAt, completedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.AgentID, &task.UserID, &task.Status, &goal,
		&resultJSON, &taskError, &task.DurationSeconds, &task.TotalTokens,
		&explorationLogJSON, &radarDataJSON, &metadataJSON,
		&task.CreatedAt, &startedAt, &completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found")
	}
	if err != nil {
		return nil, err
	}

	task.Goal = goal.String
	task.Error = taskError.String

	if len(resultJSON) > 0 {
		json.Unmarshal(resultJSON, &task.Result)
	}
	if len(explorationLogJSON) > 0 {
		json.Unmarshal(explorationLogJSON, &task.ExplorationLog)
	}
	if len(radarDataJSON) > 0 {
		json.Unmarshal(radarDataJSON, &task.RadarData)
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &task.Metadata)
	}

	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	return &task, nil
}

// ListTasksByAgent retrieves tasks for an agent
func (r *AgentRepository) ListTasksByAgent(ctx context.Context, agentID string, limit int) ([]*agent.AgentTask, error) {
	query := `
		SELECT id, agent_id, user_id, status, goal, result, error,
			   duration_seconds, total_tokens, exploration_log, radar_data,
			   metadata, created_at, started_at, completed_at
		FROM agent_tasks
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	return r.queryTasks(ctx, query, agentID, limit)
}

// ListTasksByUser retrieves tasks for a user
func (r *AgentRepository) ListTasksByUser(ctx context.Context, userID string, limit int) ([]*agent.AgentTask, error) {
	query := `
		SELECT id, agent_id, user_id, status, goal, result, error,
			   duration_seconds, total_tokens, exploration_log, radar_data,
			   metadata, created_at, started_at, completed_at
		FROM agent_tasks
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	return r.queryTasks(ctx, query, userID, limit)
}

// UpdateTask updates an existing task
func (r *AgentRepository) UpdateTask(ctx context.Context, task *agent.AgentTask) error {
	query := `
		UPDATE agent_tasks SET
			status = $2, result = $3, error = $4,
			duration_seconds = $5, total_tokens = $6,
			exploration_log = $7, radar_data = $8, metadata = $9,
			started_at = $10, completed_at = $11
		WHERE id = $1
	`

	resultJSON, _ := json.Marshal(task.Result)
	explorationLogJSON, _ := json.Marshal(task.ExplorationLog)
	radarDataJSON, _ := json.Marshal(task.RadarData)
	metadataJSON, _ := json.Marshal(task.Metadata)

	_, err := r.db.ExecContext(ctx, query,
		task.ID, task.Status, resultJSON, task.Error,
		task.DurationSeconds, task.TotalTokens,
		explorationLogJSON, radarDataJSON, metadataJSON,
		task.StartedAt, task.CompletedAt,
	)

	return err
}

// DeleteTask removes a task from the database
func (r *AgentRepository) DeleteTask(ctx context.Context, id string) error {
	query := `DELETE FROM agent_tasks WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// Helper: query multiple agents
func (r *AgentRepository) queryAgents(ctx context.Context, query string, args ...any) ([]*agent.DestinationAgent, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*agent.DestinationAgent

	for rows.Next() {
		var ag agent.DestinationAgent
		var tagsJSON []byte
		var lastUsedAt sql.NullTime
		var description, vectorCollectionID sql.NullString

		err := rows.Scan(
			&ag.ID, &ag.UserID, &ag.Name, &description, &ag.Destination,
			&vectorCollectionID, &ag.DocumentCount, &ag.Language, &ag.Theme,
			&ag.Status, &tagsJSON, &ag.CreatedAt, &ag.UpdatedAt, &lastUsedAt,
			&ag.UsageCount, &ag.Rating,
		)
		if err != nil {
			return nil, err
		}

		ag.Description = description.String
		ag.VectorCollectionID = vectorCollectionID.String
		json.Unmarshal(tagsJSON, &ag.Tags)

		if lastUsedAt.Valid {
			ag.LastUsedAt = &lastUsedAt.Time
		}

		agents = append(agents, &ag)
	}

	return agents, nil
}

// Helper: query multiple tasks
func (r *AgentRepository) queryTasks(ctx context.Context, query string, args ...any) ([]*agent.AgentTask, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*agent.AgentTask

	for rows.Next() {
		var task agent.AgentTask
		var resultJSON, explorationLogJSON, radarDataJSON, metadataJSON []byte
		var goal, taskError sql.NullString
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&task.ID, &task.AgentID, &task.UserID, &task.Status, &goal,
			&resultJSON, &taskError, &task.DurationSeconds, &task.TotalTokens,
			&explorationLogJSON, &radarDataJSON, &metadataJSON,
			&task.CreatedAt, &startedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		task.Goal = goal.String
		task.Error = taskError.String

		if len(resultJSON) > 0 {
			json.Unmarshal(resultJSON, &task.Result)
		}
		if len(explorationLogJSON) > 0 {
			json.Unmarshal(explorationLogJSON, &task.ExplorationLog)
		}
		if len(radarDataJSON) > 0 {
			json.Unmarshal(radarDataJSON, &task.RadarData)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &task.Metadata)
		}

		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		tasks = append(tasks, &task)
	}

	return tasks, nil
}