// Package router provides session HTTP handlers
package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/memory"
	"github.com/utaaa/uta-travel-agent/internal/session"
)

// SessionHandler handles session-related HTTP requests
type SessionHandler struct {
	sessionStore session.Storage
	memoryStore  memory.Storage
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sessionStore session.Storage, memoryStore memory.Storage) *SessionHandler {
	return &SessionHandler{
		sessionStore: sessionStore,
		memoryStore:  memoryStore,
	}
}

// handleListSessions returns all sessions
func (h *SessionHandler) handleListSessions(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Parse query parameters
	userID := req.URL.Query().Get("user_id")
	agentType := req.URL.Query().Get("agent_type")
	limitStr := req.URL.Query().Get("limit")

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var sessions []*session.Session
	var err error

	if userID != "" {
		sessions, err = h.sessionStore.ListByUser(ctx, userID, limit)
	} else if agentType != "" {
		sessions, err = h.sessionStore.ListByAgentType(ctx, agentType, limit)
	} else {
		// List all with default options
		result, err := h.sessionStore.List(ctx, session.ListOptions{
			Limit:      limit,
			OrderBy:    "last_active_at",
			Descending: true,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list sessions: %v", err))
			return
		}
		sessions = result.Sessions

		// Return grouped result
		writeJSON(w, http.StatusOK, map[string]any{
			"sessions": sessions,
			"grouped":  result.Grouped,
			"total":    result.Total,
		})
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list sessions: %v", err))
		return
	}

	// Group sessions by date
	grouped := groupSessionsByDate(sessions)

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"grouped":  grouped,
		"total":    len(sessions),
	})
}

// handleGetSession returns a specific session
func (h *SessionHandler) handleGetSession(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id := req.PathValue("id")

	sess, err := h.sessionStore.Get(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session not found: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, sess)
}

// CreateSessionRequest represents the request body for creating a session
type CreateSessionRequest struct {
	AgentType      string `json:"agent_type"`
	DestinationID  string `json:"destination_id,omitempty"`
	Title          string `json:"title,omitempty"`
	UserID         string `json:"user_id,omitempty"`
}

// handleCreateSession creates a new session
func (h *SessionHandler) handleCreateSession(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var reqBody CreateSessionRequest
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if reqBody.AgentType == "" {
		reqBody.AgentType = "main"
	}

	// Create session
	sess := session.New(generateID())
	sess.SetAgentType(reqBody.AgentType)
	if reqBody.Title != "" {
		sess.SetTitle(reqBody.Title)
	}
	if reqBody.UserID != "" {
		sess.SetMetadata("user_id", reqBody.UserID)
	}
	if reqBody.DestinationID != "" {
		sess.SetMetadata("destination_id", reqBody.DestinationID)
	}

	if err := h.sessionStore.Create(ctx, sess); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create session: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, sess.ToSnapshot())
}

// UpdateSessionRequest represents the request body for updating a session
type UpdateSessionRequest struct {
	Title string       `json:"title,omitempty"`
	State session.State `json:"state,omitempty"`
}

// handleUpdateSession updates a session
func (h *SessionHandler) handleUpdateSession(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id := req.PathValue("id")

	var reqBody UpdateSessionRequest
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get existing session
	sess, err := h.sessionStore.Get(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session not found: %v", err))
		return
	}

	// Update fields
	if reqBody.Title != "" {
		sess.SetTitle(reqBody.Title)
	}
	if reqBody.State != "" {
		sess.SetState(reqBody.State)
	}

	if err := h.sessionStore.Update(ctx, sess); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update session: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, sess.ToSnapshot())
}

// handleDeleteSession deletes a session
func (h *SessionHandler) handleDeleteSession(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id := req.PathValue("id")

	if err := h.sessionStore.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete session: %v", err))
		return
	}

	// Also delete memory
	if h.memoryStore != nil {
		h.memoryStore.Delete(ctx, id)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// handleGetSessionMessages returns messages for a session
func (h *SessionHandler) handleGetSessionMessages(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	sessionID := req.PathValue("id")

	if h.memoryStore == nil {
		writeError(w, http.StatusInternalServerError, "memory store not initialized")
		return
	}

	snapshot, err := h.memoryStore.Load(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load messages: %v", err))
		return
	}

	// Convert to message format
	messages := make([]map[string]any, 0)
	for _, item := range snapshot.ShortTerm {
		if item.Type == "message" {
			role := "user"
			if r, ok := item.Metadata["role"].(string); ok {
				role = r
			}
			messages = append(messages, map[string]any{
				"id":         item.ID,
				"role":       role,
				"content":    item.Content,
				"created_at": item.Timestamp,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"has_more": false,
	})
}

// handleSessionChat handles chat messages for a session
func (h *SessionHandler) handleSessionChat(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	sessionID := req.PathValue("id")

	var reqBody struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if reqBody.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Get session
	sess, err := h.sessionStore.Get(ctx, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session not found: %v", err))
		return
	}

	// Touch session
	sess.Touch()
	h.sessionStore.Update(ctx, sess)

	// For now, return a placeholder response
	// In a full implementation, this would use the appropriate agent
	writeJSON(w, http.StatusOK, map[string]any{
		"message": map[string]any{
			"id":         generateID(),
			"role":       "assistant",
			"content":    fmt.Sprintf("Received message for session %s: %s", sessionID, reqBody.Message),
			"created_at": time.Now(),
		},
	})
}

// Helper function to group sessions by date
func groupSessionsByDate(sessions []*session.Session) map[string][]*session.Session {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	grouped := map[string][]*session.Session{
		"today":     {},
		"yesterday": {},
		"previous":  {},
	}

	for _, sess := range sessions {
		createdAt := sess.CreatedAt()
		if createdAt.After(today) {
			grouped["today"] = append(grouped["today"], sess)
		} else if createdAt.After(yesterday) {
			grouped["yesterday"] = append(grouped["yesterday"], sess)
		} else {
			grouped["previous"] = append(grouped["previous"], sess)
		}
	}

	return grouped
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
