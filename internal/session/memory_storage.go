// Package session provides session memory storage implementations
package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/utaaa/uta-travel-agent/internal/memory"
)

// MemoryStorage defines the interface for session memory persistence
type MemoryStorage interface {
	Save(ctx context.Context, sessionID string, snapshot *memory.Snapshot) error
	Load(ctx context.Context, sessionID string) (*memory.Snapshot, error)
	Delete(ctx context.Context, sessionID string) error
}

// PostgreSQLMemoryStorage implements MemoryStorage using PostgreSQL
type PostgreSQLMemoryStorage struct {
	db *sql.DB
}

// NewPostgreSQLMemoryStorage creates a new PostgreSQL memory storage
func NewPostgreSQLMemoryStorage(db *sql.DB) *PostgreSQLMemoryStorage {
	return &PostgreSQLMemoryStorage{db: db}
}

// Save persists memory snapshot to database
func (s *PostgreSQLMemoryStorage) Save(ctx context.Context, sessionID string, snapshot *memory.Snapshot) error {
	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing memory items for this session
	_, err = tx.ExecContext(ctx, "DELETE FROM session_memory WHERE session_id = $1", sessionID)
	if err != nil {
		return fmt.Errorf("delete old memory: %w", err)
	}

	// Insert short-term memory
	for _, item := range snapshot.ShortTerm {
		if err := s.insertMemoryItem(ctx, tx, sessionID, item); err != nil {
			return fmt.Errorf("insert short-term memory: %w", err)
		}
	}

	// Insert long-term memory
	for _, item := range snapshot.LongTerm {
		if err := s.insertMemoryItem(ctx, tx, sessionID, item); err != nil {
			return fmt.Errorf("insert long-term memory: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// insertMemoryItem inserts a single memory item
func (s *PostgreSQLMemoryStorage) insertMemoryItem(ctx context.Context, tx *sql.Tx, sessionID string, item memory.Item) error {
	metadata, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	role := ""
	if r, ok := item.Metadata["role"].(string); ok {
		role = r
	}

	query := `
		INSERT INTO session_memory (id, session_id, type, content, role, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = tx.ExecContext(ctx, query,
		item.ID,
		sessionID,
		item.Type,
		item.Content,
		role,
		metadata,
		item.Timestamp,
	)

	return err
}

// Load restores memory snapshot from database
func (s *PostgreSQLMemoryStorage) Load(ctx context.Context, sessionID string) (*memory.Snapshot, error) {
	query := `
		SELECT id, type, content, role, metadata, created_at
		FROM session_memory
		WHERE session_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query memory: %w", err)
	}
	defer rows.Close()

	snapshot := &memory.Snapshot{
		SessionID:  sessionID,
		ShortTerm:  []memory.Item{},
		LongTerm:   []memory.Item{},
		Embeddings: make(map[string][]float32),
	}

	for rows.Next() {
		var item memory.Item
		var role string
		var metadata []byte

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Content,
			&role,
			&metadata,
			&item.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("scan memory item: %w", err)
		}

		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &item.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}

		// Ensure role is in metadata
		if item.Metadata == nil {
			item.Metadata = make(map[string]any)
		}
		if role != "" {
			item.Metadata["role"] = role
		}

		// Separate short-term and long-term memory
		if item.Type == "long_term" {
			snapshot.LongTerm = append(snapshot.LongTerm, item)
		} else {
			snapshot.ShortTerm = append(snapshot.ShortTerm, item)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory: %w", err)
	}

	return snapshot, nil
}

// Delete removes all memory for a session
func (s *PostgreSQLMemoryStorage) Delete(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM session_memory WHERE session_id = $1", sessionID)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	return nil
}
