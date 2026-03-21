// Package session provides session storage implementations
package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Storage defines the interface for session persistence
type Storage interface {
	// Session operations
	Create(ctx context.Context, session *Session) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Update(ctx context.Context, session *Session) error
	Delete(ctx context.Context, sessionID string) error
	ListByUser(ctx context.Context, userID string, limit int) ([]*Session, error)
	ListByAgentType(ctx context.Context, agentType string, limit int) ([]*Session, error)

	// Batch operations
	List(ctx context.Context, opts ListOptions) (*ListResult, error)
}

// ListOptions for querying sessions
type ListOptions struct {
	UserID     string
	AgentType  string
	State      State
	Limit      int
	Offset     int
	OrderBy    string // "created_at", "last_active_at", "updated_at"
	Descending bool
}

// ListResult contains paginated session results
type ListResult struct {
	Sessions  []*Session
	Total     int
	HasMore   bool
	Grouped   map[string][]*Session // Grouped by date: "today", "yesterday", "previous"
}

// PostgreSQLStorage implements Storage using PostgreSQL
type PostgreSQLStorage struct {
	db *sql.DB
}

// NewPostgreSQLStorage creates a new PostgreSQL storage
func NewPostgreSQLStorage(db *sql.DB) *PostgreSQLStorage {
	return &PostgreSQLStorage{db: db}
}

// Create creates a new session
func (s *PostgreSQLStorage) Create(ctx context.Context, session *Session) error {
	snapshot := session.ToSnapshot()

	metadata, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO sessions (id, agent_type, state, created_at, updated_at, last_active_at, title, message_count, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = s.db.ExecContext(ctx, query,
		snapshot.ID,
		snapshot.AgentType,
		snapshot.State,
		snapshot.CreatedAt,
		snapshot.UpdatedAt,
		snapshot.LastActiveAt,
		snapshot.Title,
		snapshot.MessageCount,
		metadata,
	)

	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// Get retrieves a session by ID
func (s *PostgreSQLStorage) Get(ctx context.Context, sessionID string) (*Session, error) {
	query := `
		SELECT id, agent_type, state, created_at, updated_at, last_active_at, title, message_count, metadata
		FROM sessions
		WHERE id = $1
	`

	var snapshot Snapshot
	var metadata []byte

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&snapshot.ID,
		&snapshot.AgentType,
		&snapshot.State,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
		&snapshot.LastActiveAt,
		&snapshot.Title,
		&snapshot.MessageCount,
		&metadata,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &snapshot.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	return FromSnapshot(&snapshot)
}

// Update updates an existing session
func (s *PostgreSQLStorage) Update(ctx context.Context, session *Session) error {
	snapshot := session.ToSnapshot()

	metadata, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		UPDATE sessions
		SET agent_type = $2, state = $3, updated_at = $4, last_active_at = $5,
		    title = $6, message_count = $7, metadata = $8
		WHERE id = $1
	`

	result, err := s.db.ExecContext(ctx, query,
		snapshot.ID,
		snapshot.AgentType,
		snapshot.State,
		snapshot.UpdatedAt,
		snapshot.LastActiveAt,
		snapshot.Title,
		snapshot.MessageCount,
		metadata,
	)

	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("session not found: %s", snapshot.ID)
	}

	return nil
}

// Delete removes a session
func (s *PostgreSQLStorage) Delete(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// ListByUser retrieves sessions for a user
func (s *PostgreSQLStorage) ListByUser(ctx context.Context, userID string, limit int) ([]*Session, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, agent_type, state, created_at, updated_at, last_active_at, title, message_count, metadata
		FROM sessions
		WHERE metadata->>'user_id' = $1 AND state != 'archived'
		ORDER BY last_active_at DESC
		LIMIT $2
	`

	return s.querySessions(ctx, query, userID, limit)
}

// ListByAgentType retrieves sessions by agent type
func (s *PostgreSQLStorage) ListByAgentType(ctx context.Context, agentType string, limit int) ([]*Session, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, agent_type, state, created_at, updated_at, last_active_at, title, message_count, metadata
		FROM sessions
		WHERE agent_type = $1 AND state != 'archived'
		ORDER BY last_active_at DESC
		LIMIT $2
	`

	return s.querySessions(ctx, query, agentType, limit)
}

// List retrieves sessions with filtering and pagination
func (s *PostgreSQLStorage) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	// Build query
	orderBy := "last_active_at"
	if opts.OrderBy != "" {
		orderBy = opts.OrderBy
	}
	orderDir := "ASC"
	if opts.Descending {
		orderDir = "DESC"
	}

	whereClauses := []string{"state != 'archived'"}
	args := []any{}
	argIdx := 1

	if opts.UserID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("metadata->>'user_id' = $%d", argIdx))
		args = append(args, opts.UserID)
		argIdx++
	}

	if opts.AgentType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("agent_type = $%d", argIdx))
		args = append(args, opts.AgentType)
		argIdx++
	}

	if opts.State != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("state = $%d", argIdx))
		args = append(args, opts.State)
		argIdx++
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM sessions WHERE %s", joinClauses(whereClauses))
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count sessions: %w", err)
	}

	// Data query with pagination
	args = append(args, opts.Limit, opts.Offset)
	query := fmt.Sprintf(`
		SELECT id, agent_type, state, created_at, updated_at, last_active_at, title, message_count, metadata
		FROM sessions
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, joinClauses(whereClauses), orderBy, orderDir, argIdx, argIdx+1)

	sessions, err := s.querySessions(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// Group by date
	grouped := s.groupByDate(sessions)

	return &ListResult{
		Sessions: sessions,
		Total:    total,
		HasMore:  opts.Offset+opts.Limit < total,
		Grouped:  grouped,
	}, nil
}

// querySessions is a helper to query sessions
func (s *PostgreSQLStorage) querySessions(ctx context.Context, query string, args ...any) ([]*Session, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session

	for rows.Next() {
		var snapshot Snapshot
		var metadata []byte

		err := rows.Scan(
			&snapshot.ID,
			&snapshot.AgentType,
			&snapshot.State,
			&snapshot.CreatedAt,
			&snapshot.UpdatedAt,
			&snapshot.LastActiveAt,
			&snapshot.Title,
			&snapshot.MessageCount,
			&metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &snapshot.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}

		session, err := FromSnapshot(&snapshot)
		if err != nil {
			return nil, fmt.Errorf("restore session: %w", err)
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

// groupByDate groups sessions by date
func (s *PostgreSQLStorage) groupByDate(sessions []*Session) map[string][]*Session {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)

	grouped := map[string][]*Session{
		"today":     {},
		"yesterday": {},
		"previous":  {},
	}

	for _, session := range sessions {
		createdAt := session.CreatedAt()
		if createdAt.After(today) {
			grouped["today"] = append(grouped["today"], session)
		} else if createdAt.After(yesterday) {
			grouped["yesterday"] = append(grouped["yesterday"], session)
		} else if createdAt.After(weekAgo) {
			grouped["previous"] = append(grouped["previous"], session)
		}
	}

	return grouped
}

func joinClauses(clauses []string) string {
	result := ""
	for i, clause := range clauses {
		if i > 0 {
			result += " AND "
		}
		result += clause
	}
	return result
}
