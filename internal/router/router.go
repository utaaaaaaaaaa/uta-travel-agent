// Package router provides HTTP and gRPC routing
package router

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/utaaa/uta-travel-agent/internal/agent"
	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/rag"
	"github.com/utaaa/uta-travel-agent/internal/scheduler"
)

// Router handles HTTP requests and routes to appropriate handlers
type Router struct {
	registry  *agent.Registry
	scheduler *scheduler.Scheduler
	mux       *http.ServeMux
	llmClient llm.Provider
	ragSvc    *rag.Service
}

// RouterConfig for creating a router
type RouterConfig struct {
	Registry  *agent.Registry
	Scheduler *scheduler.Scheduler
	LLMClient llm.Provider
	RAGSvc    *rag.Service
}

// NewRouter creates a new router
func NewRouter(cfg RouterConfig) *Router {
	r := &Router{
		registry:  cfg.Registry,
		scheduler: cfg.Scheduler,
		mux:       http.NewServeMux(),
		llmClient: cfg.LLMClient,
		ragSvc:    cfg.RAGSvc,
	}

	r.setupRoutes()
	return r
}

// NewRouterWithDefaults creates a router with defaults (for backward compatibility)
func NewRouterWithDefaults(registry *agent.Registry, scheduler *scheduler.Scheduler) *Router {
	return NewRouter(RouterConfig{
		Registry:  registry,
		Scheduler: scheduler,
	})
}

func (r *Router) setupRoutes() {
	// Health check
	r.mux.HandleFunc("GET /health", r.handleHealth)

	// Agent endpoints
	r.mux.HandleFunc("GET /api/v1/agents", r.handleListAgents)
	r.mux.HandleFunc("GET /api/v1/agents/{id}", r.handleGetAgent)
	r.mux.HandleFunc("POST /api/v1/agents", r.handleCreateAgent)
	r.mux.HandleFunc("DELETE /api/v1/agents/{id}", r.handleDeleteAgent)

	// Chat endpoint for guide mode
	r.mux.HandleFunc("POST /api/v1/agents/{id}/chat", r.handleAgentChat)
	r.mux.HandleFunc("GET /api/v1/agents/{id}/chat/stream", r.handleAgentChatStream)

	// Task endpoints
	r.mux.HandleFunc("POST /api/v1/agents/{id}/tasks", r.handleCreateTask)
	r.mux.HandleFunc("GET /api/v1/tasks/{id}", r.handleGetTask)
	r.mux.HandleFunc("GET /api/v1/tasks/{id}/stream", r.handleTaskStream)

	// WebSocket for real-time communication
	r.mux.HandleFunc("GET /ws", r.handleWebSocket)
}

// Start begins serving requests
func (r *Router) Start(addr string) error {
	// Add CORS middleware
	handler := corsMiddleware(r.mux)

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, handler)
}

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if req.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// handleListAgents returns all agents for a user
func (r *Router) handleListAgents(w http.ResponseWriter, req *http.Request) {
	userID := req.URL.Query().Get("user_id")
	if userID == "" {
		userID = "default-user" // Default user for demo
	}

	agents := r.registry.GetByUserID(userID)

	response := map[string]any{
		"agents": agents,
		"count":  len(agents),
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetAgent returns a specific agent
func (r *Router) handleGetAgent(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	ag, exists := r.registry.Get(id)
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, ag)
}

// CreateAgentRequest represents the request body for creating an agent
type CreateAgentRequest struct {
	Destination string   `json:"destination"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Theme       string   `json:"theme,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	UserID      string   `json:"user_id,omitempty"`
}

// CreateAgentResponse represents the response for creating an agent
type CreateAgentResponse struct {
	AgentID string `json:"agent_id"`
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// handleCreateAgent creates a new destination agent
func (r *Router) handleCreateAgent(w http.ResponseWriter, req *http.Request) {
	var reqBody CreateAgentRequest
	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if reqBody.Destination == "" {
		writeError(w, http.StatusBadRequest, "destination is required")
		return
	}

	// Generate IDs
	agentID := uuid.New().String()
	taskID := uuid.New().String()

	// Default values
	if reqBody.UserID == "" {
		reqBody.UserID = "default-user"
	}
	if reqBody.Name == "" {
		reqBody.Name = fmt.Sprintf("%s旅游助手", reqBody.Destination)
	}
	if reqBody.Theme == "" {
		reqBody.Theme = "cultural"
	}

	// Create agent
	now := time.Now()
	ag := &agent.DestinationAgent{
		ID:          agentID,
		UserID:      reqBody.UserID,
		Name:        reqBody.Name,
		Description: reqBody.Description,
		Destination: reqBody.Destination,
		Theme:       reqBody.Theme,
		Language:    "zh",
		Tags:        []string{},
		Status:      agent.StatusCreating,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Register agent
	if err := r.registry.Register(ag); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create agent: %v", err))
		return
	}

	// Create task
	task := &agent.AgentTask{
		ID:        taskID,
		AgentID:   agentID,
		UserID:    reqBody.UserID,
		Status:    agent.TaskStatusPending,
		Goal:      fmt.Sprintf("创建 %s 导游 Agent", reqBody.Destination),
		CreatedAt: now,
	}

	// Save task to scheduler
	if r.scheduler != nil {
		r.scheduler.Save(task)
	}

	// Start background task execution
	go r.executeAgentCreation(task, ag)

	// Return response
	response := CreateAgentResponse{
		AgentID: agentID,
		TaskID:  taskID,
		Status:  "creating",
		Message: fmt.Sprintf("正在创建 %s 导游 Agent", reqBody.Destination),
	}

	writeJSON(w, http.StatusCreated, response)
}

// handleDeleteAgent deletes an agent
func (r *Router) handleDeleteAgent(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	if err := r.registry.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCreateTask creates a new task for an agent
func (r *Router) handleCreateTask(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")

	_, exists := r.registry.Get(agentID)
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var reqBody struct {
		Goal  string         `json:"goal"`
		Input map[string]any `json:"input,omitempty"`
	}

	if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	taskID := uuid.New().String()
	task := &agent.AgentTask{
		ID:        taskID,
		AgentID:   agentID,
		UserID:    "default-user",
		Status:    agent.TaskStatusPending,
		Goal:      reqBody.Goal,
		CreatedAt: time.Now(),
	}

	if r.scheduler != nil {
		r.scheduler.Save(task)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"task_id": taskID,
		"status":  "pending",
	})
}

// handleGetTask returns task details
func (r *Router) handleGetTask(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	if r.scheduler == nil {
		writeError(w, http.StatusInternalServerError, "scheduler not initialized")
		return
	}

	task, exists := r.scheduler.Get(id)
	if !exists {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// handleTaskStream streams task progress via SSE
func (r *Router) handleTaskStream(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Get task
	if r.scheduler == nil {
		writeError(w, http.StatusInternalServerError, "scheduler not initialized")
		return
	}

	task, exists := r.scheduler.Get(id)
	if !exists {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// Send initial status
	r.sendSSE(w, flusher, "status", map[string]any{
		"task_id": id,
		"status":  task.Status,
	})

	// Poll for updates
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := req.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updatedTask, exists := r.scheduler.Get(id)
			if !exists {
				return
			}

			// Send progress update
			if len(updatedTask.ExplorationLog) > 0 {
				latestStep := updatedTask.ExplorationLog[len(updatedTask.ExplorationLog)-1]
				r.sendSSE(w, flusher, "progress", map[string]any{
					"stage":   "exploring",
					"step":    latestStep,
					"message": latestStep.Thought,
				})
			}

			// Check completion
			if updatedTask.Status == agent.TaskStatusCompleted ||
				updatedTask.Status == agent.TaskStatusFailed {
				r.sendSSE(w, flusher, "complete", map[string]any{
					"task_id":      id,
					"status":       updatedTask.Status,
					"agent_id":     updatedTask.AgentID,
					"error":        updatedTask.Error,
					"duration_sec": updatedTask.DurationSeconds,
					"tokens":       updatedTask.TotalTokens,
				})
				return
			}
		}
	}
}

// sendSSE sends a Server-Sent Event
func (r *Router) sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	dataJSON, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(dataJSON))
	flusher.Flush()
}

// handleWebSocket handles WebSocket connections (placeholder)
func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request) {
	// TODO: Implement WebSocket upgrade and handling
	log.Println("WebSocket connection requested")
	http.Error(w, "WebSocket not implemented yet", http.StatusNotImplemented)
}

// ChatRequest represents a chat message request
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id"`
}

// handleAgentChat handles synchronous chat with an agent
func (r *Router) handleAgentChat(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")

	// Verify agent exists
	ag, exists := r.registry.Get(agentID)
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var chatReq ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chatReq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if chatReq.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Generate session ID if not provided
	if chatReq.SessionID == "" {
		chatReq.SessionID = uuid.New().String()
	}

	var response string

	// Try RAG if available and agent has a vector collection
	if r.ragSvc != nil && ag.VectorCollectionID != "" {
		result, err := r.ragSvc.Query(req.Context(), ag.VectorCollectionID, chatReq.Message, 5)
		if err != nil {
			log.Printf("RAG query failed: %v, falling back to mock", err)
			response = generateMockChatResponse(ag.Destination, chatReq.Message)
		} else {
			response = result.Answer
		}
	} else {
		// Fallback to mock response
		response = generateMockChatResponse(ag.Destination, chatReq.Message)
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		Response:  response,
		SessionID: chatReq.SessionID,
	})
}

// handleAgentChatStream handles streaming chat with SSE
func (r *Router) handleAgentChatStream(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")

	// Verify agent exists
	ag, exists := r.registry.Get(agentID)
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Get message from query parameter for GET streaming
	message := req.URL.Query().Get("message")
	if message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	sessionID := req.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Stream the response
	response := generateMockChatResponse(ag.Destination, message)
	words := splitIntoChunks(response, 10) // Split into chunks of ~10 chars

	for i, chunk := range words {
		select {
		case <-req.Context().Done():
			return
		default:
			r.sendSSE(w, flusher, "chunk", map[string]any{
				"content": chunk,
				"done":    i == len(words)-1,
			})
			time.Sleep(50 * time.Millisecond) // Simulate streaming delay
		}
	}

	// Send completion event
	r.sendSSE(w, flusher, "complete", map[string]any{
		"session_id": sessionID,
		"agent_id":   agentID,
	})
}

// generateMockChatResponse generates a mock chat response
func generateMockChatResponse(destination, message string) string {
	responses := map[string]string{
		"京都": fmt.Sprintf("关于您的问题「%s」：\n\n京都是日本的文化古都，拥有众多世界文化遗产。%s\n\n如果您想了解更多关于京都的信息，比如金阁寺、清水寺、伏见稻荷大社等著名景点，或者抹茶、京料理等美食文化，请随时问我！", message, getAttractionInfo(destination)),
		"东京": fmt.Sprintf("关于您的问题「%s」：\n\n东京是日本的首都，融合了现代与传统。%s\n\n您想了解东京塔、浅草寺、涩谷十字路口等景点吗？或者东京的美食、购物体验？", message, getAttractionInfo(destination)),
		"大阪": fmt.Sprintf("关于您的问题「%s」：\n\n大阪被称为\"天下厨房\"，是美食爱好者的天堂。%s\n\n道顿堀、大阪城、通天阁都是必去景点。章鱼烧、大阪烧更是不能错过的美食！", message, getAttractionInfo(destination)),
	}

	if resp, ok := responses[destination]; ok {
		return resp
	}
	return fmt.Sprintf("关于您的问题「%s」：\n\n%s是一个很棒的旅游目的地！\n\n我可以为您介绍当地的景点、美食、文化等信息。请告诉我您想了解什么？", message, destination)
}

// getAttractionInfo returns mock attraction information
func getAttractionInfo(destination string) string {
	attractions := map[string]string{
		"京都": "推荐景点包括：金阁寺（世界文化遗产）、清水寺（著名古寺）、伏见稻荷大社（千本�的居）、岚山（竹林小径）、银阁寺等。",
		"东京": "推荐景点包括：东京塔、浅草寺、明治神宫、涩谷十字路口、秋叶原电器街、银座购物区等。",
		"大阪": "推荐景点包括：大阪城、道顿堀、通天阁、环球影城、心斋桥购物街等。",
	}

	if info, ok := attractions[destination]; ok {
		return info
	}
	return "这里有许多值得探索的景点和文化体验。"
}

// splitIntoChunks splits text into chunks for streaming
func splitIntoChunks(text string, chunkSize int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

// executeAgentCreation runs the agent creation task in background
func (r *Router) executeAgentCreation(task *agent.AgentTask, ag *agent.DestinationAgent) {
	startTime := time.Now()

	// Update task status
	task.Status = agent.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	r.scheduler.Save(task)

	// Simulate creation process (will be replaced with real agent execution)
	// This is a placeholder for the actual agent creation logic

	// Phase 1: Research
	r.simulateProgress(task, "researching", 20, "正在搜索目的地信息...")
	time.Sleep(2 * time.Second)

	// Phase 2: Curate
	r.simulateProgress(task, "curating", 50, "正在整理信息...")
	time.Sleep(2 * time.Second)

	// Phase 3: Index
	r.simulateProgress(task, "indexing", 80, "正在构建向量索引...")
	time.Sleep(2 * time.Second)

	// Complete
	task.Status = agent.TaskStatusCompleted
	task.DurationSeconds = time.Since(startTime).Seconds()
	task.TotalTokens = 5000 // Placeholder
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	task.Result = map[string]any{
		"collection_id":   fmt.Sprintf("%s-%d", ag.Destination, startTime.Unix()),
		"document_count":  42,
		"exploration_log": task.ExplorationLog,
	}

	r.scheduler.Save(task)

	// Update agent status
	ag.Status = agent.StatusReady
	ag.VectorCollectionID = fmt.Sprintf("%s-%d", ag.Destination, startTime.Unix())
	ag.DocumentCount = 42
	r.registry.Update(ag)
}

// simulateProgress adds a progress step to the task
func (r *Router) simulateProgress(task *agent.AgentTask, stage string, progress int, message string) {
	step := agent.ExplorationStep{
		Timestamp: time.Now(),
		Direction: "综合",
		Thought:   message,
		Action:    stage,
		DurationMs: int64(progress * 100),
	}

	task.ExplorationLog = append(task.ExplorationLog, step)
	r.scheduler.Save(task)
}

// Helper functions
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}