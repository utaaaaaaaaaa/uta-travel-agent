// Package session provides session management for agents
package session

import (
	"fmt"
	"sync"
	"time"
)

// State defines the state of a session
type State string

const (
	StateActive   State = "active"
	StatePaused   State = "paused"
	StateArchived State = "archived"
)

// Session represents a conversation session
type Session struct {
	mu           sync.RWMutex
	id           string
	agentType    string
	state        State
	createdAt    time.Time
	updatedAt    time.Time
	lastActiveAt time.Time
	metadata     map[string]any
	title        string
	messageCount int
}

// New creates a new session
func New(id string) *Session {
	now := time.Now()
	return &Session{
		id:           id,
		state:        StateActive,
		createdAt:    now,
		updatedAt:    now,
		lastActiveAt: now,
		metadata:     make(map[string]any),
	}
}

// ID returns the session ID
func (s *Session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// State returns the current session state
func (s *Session) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState sets the session state
func (s *Session) SetState(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.updatedAt = time.Now()
}

// AgentType returns the agent type
func (s *Session) AgentType() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agentType
}

// SetAgentType sets the agent type
func (s *Session) SetAgentType(agentType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentType = agentType
	s.updatedAt = time.Now()
}

// Touch updates the last active time
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActiveAt = time.Now()
	s.updatedAt = time.Now()
}

// CreatedAt returns the creation time
func (s *Session) CreatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.createdAt
}

// LastActiveAt returns the last active time
func (s *Session) LastActiveAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActiveAt
}

// Title returns the session title
func (s *Session) Title() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.title
}

// SetTitle sets the session title
func (s *Session) SetTitle(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.title = title
	s.updatedAt = time.Now()
}

// MessageCount returns the message count
func (s *Session) MessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messageCount
}

// IncrementMessageCount increments the message counter
func (s *Session) IncrementMessageCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageCount++
	s.updatedAt = time.Now()
}

// Metadata returns all metadata
func (s *Session) Metadata() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]any, len(s.metadata))
	for k, v := range s.metadata {
		result[k] = v
	}
	return result
}

// SetMetadata sets a metadata key-value pair
func (s *Session) SetMetadata(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = value
	s.updatedAt = time.Now()
}

// GetMetadata gets a metadata value
func (s *Session) GetMetadata(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.metadata[key]
	return val, ok
}

// IsActive returns true if the session is active
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateActive
}

// Archive marks the session as archived
func (s *Session) Archive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateArchived
	s.updatedAt = time.Now()
}

// Pause pauses the session
func (s *Session) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StatePaused
	s.updatedAt = time.Now()
}

// Resume resumes a paused session
func (s *Session) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateActive
	s.lastActiveAt = time.Now()
	s.updatedAt = time.Now()
}

// Duration returns the duration since session creation
func (s *Session) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.createdAt)
}

// IdleDuration returns the duration since last activity
func (s *Session) IdleDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.lastActiveAt)
}

// Snapshot is a serializable snapshot of a session
type Snapshot struct {
	ID           string         `json:"id"`
	AgentType    string         `json:"agent_type"`
	State        State          `json:"state"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	LastActiveAt time.Time      `json:"last_active_at"`
	Title        string         `json:"title"`
	MessageCount int            `json:"message_count"`
	Metadata     map[string]any `json:"metadata"`
}

// ToSnapshot creates a snapshot of the session
func (s *Session) ToSnapshot() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata := make(map[string]any, len(s.metadata))
	for k, v := range s.metadata {
		metadata[k] = v
	}

	return &Snapshot{
		ID:           s.id,
		AgentType:    s.agentType,
		State:        s.state,
		CreatedAt:    s.createdAt,
		UpdatedAt:    s.updatedAt,
		LastActiveAt: s.lastActiveAt,
		Title:        s.title,
		MessageCount: s.messageCount,
		Metadata:     metadata,
	}
}

// FromSnapshot restores session from a snapshot
func FromSnapshot(snapshot *Snapshot) (*Session, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("snapshot is nil")
	}

	metadata := make(map[string]any)
	if snapshot.Metadata != nil {
		for k, v := range snapshot.Metadata {
			metadata[k] = v
		}
	}

	return &Session{
		id:           snapshot.ID,
		agentType:    snapshot.AgentType,
		state:        snapshot.State,
		createdAt:    snapshot.CreatedAt,
		updatedAt:    snapshot.UpdatedAt,
		lastActiveAt: snapshot.LastActiveAt,
		title:        snapshot.Title,
		messageCount: snapshot.MessageCount,
		metadata:     metadata,
	}, nil
}