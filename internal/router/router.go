// Package router provides HTTP and gRPC routing
package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/utaaa/uta-travel-agent/internal/agent"
	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/rag"
	"github.com/utaaa/uta-travel-agent/internal/scheduler"
	"github.com/utaaa/uta-travel-agent/internal/tools"
)

// Router handles HTTP requests and routes to appropriate handlers
type Router struct {
	registry       *agent.Registry
	scheduler      *scheduler.Scheduler
	mux            *http.ServeMux
	llmClient      llm.Provider
	ragSvc         *rag.Service
	mainAgent      *agent.MainAgent
	toolRegistry   agent.ToolRegistry
	tavilyAPIKey   string
	sessionHandler *SessionHandler
}

// RouterConfig for creating a router
type RouterConfig struct {
	Registry       *agent.Registry
	Scheduler      *scheduler.Scheduler
	LLMClient      llm.Provider
	RAGSvc         *rag.Service
	MainAgent      *agent.MainAgent
	ToolRegistry   agent.ToolRegistry
	TavilyAPIKey   string
	SessionHandler *SessionHandler
}

// NewRouter creates a new router
func NewRouter(cfg RouterConfig) *Router {
	r := &Router{
		registry:       cfg.Registry,
		scheduler:      cfg.Scheduler,
		mux:            http.NewServeMux(),
		llmClient:      cfg.LLMClient,
		ragSvc:         cfg.RAGSvc,
		mainAgent:      cfg.MainAgent,
		toolRegistry:   cfg.ToolRegistry,
		tavilyAPIKey:   cfg.TavilyAPIKey,
		sessionHandler: cfg.SessionHandler,
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

	// Session endpoints
	if r.sessionHandler != nil {
		r.mux.HandleFunc("GET /api/v1/sessions", r.sessionHandler.handleListSessions)
		r.mux.HandleFunc("POST /api/v1/sessions", r.sessionHandler.handleCreateSession)
		r.mux.HandleFunc("GET /api/v1/sessions/{id}", r.sessionHandler.handleGetSession)
		r.mux.HandleFunc("PATCH /api/v1/sessions/{id}", r.sessionHandler.handleUpdateSession)
		r.mux.HandleFunc("DELETE /api/v1/sessions/{id}", r.sessionHandler.handleDeleteSession)
		r.mux.HandleFunc("GET /api/v1/sessions/{id}/messages", r.sessionHandler.handleGetSessionMessages)
		r.mux.HandleFunc("POST /api/v1/sessions/{id}/chat", r.sessionHandler.handleSessionChat)
	}

	// Agent endpoints
	r.mux.HandleFunc("GET /api/v1/agents", r.handleListAgents)
	r.mux.HandleFunc("GET /api/v1/agents/{id}", r.handleGetAgent)
	r.mux.HandleFunc("POST /api/v1/agents", r.handleCreateAgent)
	r.mux.HandleFunc("DELETE /api/v1/agents/{id}", r.handleDeleteAgent)

	// Chat endpoint for guide mode
	r.mux.HandleFunc("POST /api/v1/agents/{id}/chat", r.handleAgentChat)
	r.mux.HandleFunc("GET /api/v1/agents/{id}/chat/stream", r.handleAgentChatStream)

	// Attractions endpoint
	r.mux.HandleFunc("GET /api/v1/agents/{id}/attractions", r.handleAgentAttractions)

	// Get task by agent ID
	r.mux.HandleFunc("GET /api/v1/agents/{id}/task", r.handleGetTaskByAgent)

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

// handleGetTaskByAgent returns the creation task for an agent
func (r *Router) handleGetTaskByAgent(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")

	if r.scheduler == nil {
		writeError(w, http.StatusInternalServerError, "scheduler not initialized")
		return
	}

	task, exists := r.scheduler.GetByAgentID(agentID)
	if !exists {
		writeError(w, http.StatusNotFound, "task not found for this agent")
		return
	}

	writeJSON(w, http.StatusOK, task)
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
		TaskID:      taskID,
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

	// Track last sent log index
	lastLogIndex := 0

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

			// Send progress updates for new log entries
			if len(updatedTask.ExplorationLog) > lastLogIndex {
				for i := lastLogIndex; i < len(updatedTask.ExplorationLog); i++ {
					step := updatedTask.ExplorationLog[i]
					r.sendSSE(w, flusher, "step", map[string]any{
						"timestamp":   step.Timestamp,
						"direction":   step.Direction,
						"thought":     step.Thought,
						"action":      step.Action,
						"tool_name":   step.ToolName,
						"result":      step.Result,
						"success":     step.Success,
						"tokens_in":   step.TokensIn,
						"tokens_out":  step.TokensOut,
					})
				}
				lastLogIndex = len(updatedTask.ExplorationLog)
			}

			// Send overall progress update
			if updatedTask.Result != nil {
				r.sendSSE(w, flusher, "progress", map[string]any{
					"stage":          string(updatedTask.Status),
					"document_count": updatedTask.Result["document_count"],
					"covered_topics": updatedTask.Result["covered_topics"],
					"missing_topics": updatedTask.Result["missing_topics"],
					"researchers":    updatedTask.Result["researchers"],
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
					"result":       updatedTask.Result,
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
	Response  string                   `json:"response"`
	SessionID string                   `json:"session_id"`
	Sources   []map[string]interface{} `json:"sources,omitempty"`
	SearchType string                   `json:"search_type,omitempty"` // "rag" or "realtime"
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
	var sources []map[string]interface{}
	var searchType string

	// Check if this requires real-time search
	if r.needsRealtimeSearch(chatReq.Message) && r.tavilyAPIKey != "" {
		// Use Tavily for real-time information
		searchType = "realtime"
		result, err := r.performRealtimeSearch(req.Context(), ag.Destination, chatReq.Message)
		if err != nil {
			log.Printf("Realtime search failed: %v, falling back to RAG", err)
			searchType = "rag"
		} else {
			response = result.Response
			sources = result.Sources
		}
	}

	// Use RAG if not realtime or realtime failed
	if searchType != "realtime" {
		searchType = "rag"
		if r.ragSvc != nil && ag.VectorCollectionID != "" {
			result, err := r.ragSvc.Query(req.Context(), ag.VectorCollectionID, chatReq.Message, 5)
			if err != nil {
				log.Printf("RAG query failed: %v, falling back to LLM", err)
				searchType = "llm"
			} else {
				response = result.Answer
				// Convert sources to map format
				sources = make([]map[string]interface{}, 0, len(result.Sources))
				for _, s := range result.Sources {
					sourceMap := map[string]interface{}{
						"score":   s.Score,
						"content": truncateString(s.Content, 200),
					}
					if s.Metadata != nil {
						if url, ok := s.Metadata["url"].(string); ok {
							sourceMap["url"] = url
						}
						if title, ok := s.Metadata["title"].(string); ok {
							sourceMap["title"] = title
						}
						if src, ok := s.Metadata["source"].(string); ok {
							sourceMap["source"] = src
						}
					}
					sources = append(sources, sourceMap)
				}
			}
		}
	}

	// Use LLM directly if RAG is not available or failed
	if response == "" && r.llmClient != nil {
		searchType = "llm"
		systemPrompt := fmt.Sprintf(`你是一位专业的%s导游助手。请根据用户的问题，用专业、友好的语气回答。

规则：
1. 提供准确、有用的信息
2. 如果不确定，坦诚告知
3. 回答要简洁但有价值
4. 使用友好的语气，像一位本地导游一样交流`, ag.Destination)

		llmResp, err := r.llmClient.CompleteWithSystem(req.Context(), systemPrompt, []llm.Message{
			{Role: "user", Content: chatReq.Message},
		})
		if err != nil {
			log.Printf("LLM fallback failed: %v", err)
			response = fmt.Sprintf("抱歉，我暂时无法回答关于%s的问题。请稍后再试。", ag.Destination)
		} else {
			response = llmResp.Content
		}
	}

	// Final fallback if everything failed
	if response == "" {
		response = generateMockChatResponse(ag.Destination, chatReq.Message)
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		Response:   response,
		SessionID:  chatReq.SessionID,
		Sources:    sources,
		SearchType: searchType,
	})
}

// handleAgentChatStream handles streaming chat with SSE
func (r *Router) handleAgentChatStream(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")

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

	// For "default" agent, use MainAgent (general chat)
	if agentID == "default" && r.mainAgent != nil {
		ctx := req.Context()
		chunkChan, errChan := r.mainAgent.ChatStream(ctx, message)

		for {
			select {
			case chunk, ok := <-chunkChan:
				if !ok {
					// Stream finished normally
					fmt.Fprintf(w, "event: complete\ndata: done\n\n")
					flusher.Flush()
					return
				}
				// Properly escape JSON string
				jsonData, _ := json.Marshal(map[string]string{"content": chunk})
				fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", string(jsonData))
				flusher.Flush()
			case err, ok := <-errChan:
				if !ok {
					// Error channel closed, wait for chunk channel to close
					continue
				}
				if err != nil {
					log.Printf("Chat stream error: %v", err)
					jsonData, _ := json.Marshal(map[string]string{"error": err.Error()})
					fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(jsonData))
					flusher.Flush()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}

	// For specific destination agents, use RAG if available
	ag, exists := r.registry.Get(agentID)
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var response string
	var sources []map[string]interface{}
	var searchType string

	// Check if this requires real-time search
	if r.needsRealtimeSearch(message) && r.tavilyAPIKey != "" {
		searchType = "realtime"
		result, err := r.performRealtimeSearch(req.Context(), ag.Destination, message)
		if err != nil {
			log.Printf("Realtime search failed: %v, falling back to RAG", err)
			searchType = "rag"
		} else {
			response = result.Response
			sources = result.Sources
		}
	}

	// Use RAG if not realtime or realtime failed
	if searchType != "realtime" {
		searchType = "rag"
		if r.ragSvc != nil && ag.VectorCollectionID != "" {
			result, err := r.ragSvc.Query(req.Context(), ag.VectorCollectionID, message, 5)
			if err != nil {
				log.Printf("RAG query failed: %v, falling back to LLM", err)
				searchType = "llm"
			} else {
				response = result.Answer
				// Convert sources
				sources = make([]map[string]interface{}, 0, len(result.Sources))
				for _, s := range result.Sources {
					sourceMap := map[string]interface{}{
						"score":   s.Score,
						"content": truncateString(s.Content, 200),
					}
					if s.Metadata != nil {
						if url, ok := s.Metadata["url"].(string); ok {
							sourceMap["url"] = url
						}
						if title, ok := s.Metadata["title"].(string); ok {
							sourceMap["title"] = title
						}
					}
					sources = append(sources, sourceMap)
				}
			}
		}
	}

	// Use LLM directly if RAG is not available or failed
	if response == "" && r.llmClient != nil {
		searchType = "llm"
		systemPrompt := fmt.Sprintf(`你是一位专业的%s导游助手。请根据用户的问题，用专业、友好的语气回答。

规则：
1. 提供准确、有用的信息
2. 如果不确定，坦诚告知
3. 回答要简洁但有价值
4. 使用友好的语气，像一位本地导游一样交流`, ag.Destination)

		llmResp, err := r.llmClient.CompleteWithSystem(req.Context(), systemPrompt, []llm.Message{
			{Role: "user", Content: message},
		})
		if err != nil {
			log.Printf("LLM fallback failed: %v", err)
			response = fmt.Sprintf("抱歉，我暂时无法回答关于%s的问题。请稍后再试。", ag.Destination)
		} else {
			response = llmResp.Content
		}
	}

	// Final fallback if everything failed
	if response == "" {
		response = generateMockChatResponse(ag.Destination, message)
	}

	// Stream the response
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

	// Send completion event with sources
	r.sendSSE(w, flusher, "complete", map[string]any{
		"session_id":  sessionID,
		"agent_id":    agentID,
		"search_type": searchType,
		"sources":     sources,
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
	ctx := context.Background()

	// Update task status
	task.Status = agent.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	r.scheduler.Save(task)

	var searchErrors []string

	// Phase 1: Create SharedKnowledgeState
	r.addProgressStep(task, "researching", 5, fmt.Sprintf("初始化 %s 的研究团队...", ag.Destination), "")
	sharedState := agent.NewSharedKnowledgeState(ag.Destination)

	// Get proxy from environment
	proxyURL := os.Getenv("HTTP_PROXY")
	if proxyURL == "" {
		proxyURL = os.Getenv("HTTPS_PROXY")
	}

	// Create tools for researchers
	wikiTool := tools.NewWikipediaSearchToolWithProxy("zh", proxyURL)
	baikeTool := tools.NewBaiduBaikeSearchToolWithProxy(proxyURL)

	// Create tool adapter to match agent.ToolExecutor interface
	toolAdapter := func(t interface {
		Execute(ctx context.Context, params map[string]any) (map[string]any, error)
	}) agent.ToolExecutor {
		return &toolExecutorAdapter{tool: t}
	}

	// Create tool map for researchers
	researcherTools := map[string]agent.ToolExecutor{
		"wikipedia_search":   toolAdapter(wikiTool),
		"baidu_baike_search": toolAdapter(baikeTool),
	}

	// Phase 2: Create and run ResearcherAgents in parallel
	initialTopics := []string{"attractions", "food", "history", "transport"}
	numResearchers := 4

	r.addProgressStep(task, "researching", 10, fmt.Sprintf("启动 %d 个研究专家并行工作...", numResearchers), "")

	var wg sync.WaitGroup
	results := make([]*agent.AgentResult, numResearchers)
	var resultsMu sync.Mutex

	// Progress monitoring goroutine
	progressCtx, progressCancel := context.WithCancel(ctx)
	defer progressCancel()
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		lastProgress := 10
		lastDocCount := 0

		for {
			select {
			case <-progressCtx.Done():
				return
			case <-ticker.C:
				progress := sharedState.GetProgress()
				totalDocs := progress.TotalDocuments
				avgRound := 0
				if len(progress.Researchers) > 0 {
					for _, r := range progress.Researchers {
						avgRound += r.CurrentRound
					}
					avgRound /= len(progress.Researchers)
				}

				// Calculate progress percentage (10-60% for research phase)
				newProgress := 10 + (avgRound * 10)
				if newProgress > 60 {
					newProgress = 60
				}

				// Add progress step when we have new documents or progress change
				if newProgress > lastProgress || totalDocs > lastDocCount {
					// Pick a direction based on what's being researched
					direction := ""
					if len(progress.CoveredTopics) > 0 {
						// Map English topic to Chinese
						topicMap := map[string]string{
							"attractions":    "景点",
							"food":          "美食",
							"history":       "文化",
							"transport":     "交通",
							"accommodation": "住宿",
							"shopping":      "购物",
						}
						for topic := range progress.CoveredTopics {
							if cn, ok := topicMap[topic]; ok {
								direction = cn
								break
							}
						}
					}

					// Build covered topics list for display
					coveredList := make([]string, 0, len(progress.CoveredTopics))
					for topic := range progress.CoveredTopics {
						coveredList = append(coveredList, topic)
					}
					topicsStr := fmt.Sprintf("已覆盖: %v", coveredList)
					r.addProgressStep(task, "researching", newProgress,
						fmt.Sprintf("研究进行中 - 轮次 %d/5, 已收集 %d 篇文档, %s",
							avgRound, totalDocs, topicsStr), direction)
					lastProgress = newProgress
					lastDocCount = totalDocs
				}
			}
		}
	}()

	// Launch researchers in parallel
	for i := 0; i < numResearchers; i++ {
		wg.Add(1)
		go func(idx int, initialTopic string) {
			defer wg.Done()

			researcherID := fmt.Sprintf("researcher-%d", idx+1)
			researcher := agent.NewResearcherAgent(agent.ResearcherAgentConfig{
				ID:           researcherID,
				LLMProvider:  r.llmClient,
				SharedState:  sharedState,
				InitialTopic: initialTopic,
				MaxRounds:    5,
				Tools:        researcherTools,
			})

			result, err := researcher.Run(ctx, initialTopic)
			if err != nil {
				resultsMu.Lock()
				searchErrors = append(searchErrors, fmt.Sprintf("Researcher %s failed: %v", researcherID, err))
				resultsMu.Unlock()
				log.Printf("Researcher %s error: %v", researcherID, err)
				return
			}

			resultsMu.Lock()
			results[idx] = result
			resultsMu.Unlock()
		}(i, initialTopics[i])
	}

	// Wait for all researchers to complete
	wg.Wait()
	progressCancel()

	// Phase 3: Run CuratorAgent to evaluate document quality
	r.addProgressStep(task, "curating", 65, fmt.Sprintf("所有研究专家完成，启动策展专家评估文档质量..."), "")

	// Add curator to progress tracking
	sharedState.RegisterResearcher("curator-1", 3, "curating")

	curator := agent.NewCuratorAgent(agent.CuratorAgentConfig{
		ID:          "curator-1",
		LLMProvider: r.llmClient,
		SharedState: sharedState,
		MaxRounds:   3,
	})

	sharedState.UpdateResearcherRound("curator-1", 1, "searching")
	curatorResult, err := curator.Run(ctx, "")
	if err != nil {
		searchErrors = append(searchErrors, fmt.Sprintf("Curator evaluation failed: %v", err))
		log.Printf("Curator error: %v", err)
		r.addProgressStep(task, "curating", 70, fmt.Sprintf("策展评估失败: %v", err), "")
		sharedState.UpdateResearcherRound("curator-1", 1, "complete")
	} else {
		// Extract quality score
		var qualityScore float64
		var highQualityCount int
		if output, ok := curatorResult.Output.(map[string]any); ok {
			if qs, ok := output["quality_score"].(float64); ok {
				qualityScore = qs
			}
			if hq, ok := output["documents_evaluated"].(int); ok {
				highQualityCount = hq
			}
		}
		r.addProgressStep(task, "curating", 70,
			fmt.Sprintf("策展完成 - 整体质量: %.0f%%, 高质量文档: %d篇", qualityScore*100, highQualityCount), "")
		sharedState.UpdateResearcherRound("curator-1", 1, "complete")
	}

	// Phase 4: Run IndexerAgent to build vector index
	documents := sharedState.GetDocuments()
	finalProgress := sharedState.GetProgress()
	collectionID := fmt.Sprintf("dest_%s_%d", ag.Destination, startTime.Unix())

	r.addProgressStep(task, "indexing", 75, fmt.Sprintf("启动索引专家构建向量索引 (%d篇文档)...", len(documents)), "")

	// Add indexer to progress tracking
	sharedState.RegisterResearcher("indexer-1", 2, "indexing")
	sharedState.UpdateResearcherRound("indexer-1", 1, "searching")

	indexer := agent.NewIndexerAgent(agent.IndexerAgentConfig{
		ID:           "indexer-1",
		LLMProvider:  r.llmClient,
		SharedState:  sharedState,
		ToolRegistry: r.toolRegistry,
		MaxRounds:    2,
	})

	indexerResult, err := indexer.Run(ctx, collectionID)
	if err != nil {
		searchErrors = append(searchErrors, fmt.Sprintf("Indexing failed: %v", err))
		log.Printf("Indexing error: %v", err)
		r.addProgressStep(task, "indexing", 85, fmt.Sprintf("索引构建失败: %v", err), "")
		sharedState.UpdateResearcherRound("indexer-1", 1, "complete")
	} else {
		// Extract indexing stats
		var totalChunks int
		if output, ok := indexerResult.Output.(map[string]any); ok {
			if tc, ok := output["total_chunks"].(int); ok {
				totalChunks = tc
			}
		}
		r.addProgressStep(task, "indexing", 90, fmt.Sprintf("索引构建完成 - %d 个文档片段", totalChunks), "")
		sharedState.UpdateResearcherRound("indexer-1", 1, "complete")
	}

	// Get final progress with all agents
	finalProgress = sharedState.GetProgress()

	// Build exploration log from researchers
	var explorationLog []agent.ExplorationStep
	for _, result := range results {
		if result != nil {
			if output, ok := result.Output.(map[string]any); ok {
				if logData, ok := output["exploration_log"].([]agent.ExplorationStep); ok {
					explorationLog = append(explorationLog, logData...)
				}
			}
		}
	}

	// Complete
	task.Status = agent.TaskStatusCompleted
	task.DurationSeconds = time.Since(startTime).Seconds()
	task.TotalTokens = 0
	for _, result := range results {
		if result != nil {
			if tokensIn, ok := result.Metadata["tokens_in"].(int); ok {
				task.TotalTokens += tokensIn
			}
			if tokensOut, ok := result.Metadata["tokens_out"].(int); ok {
				task.TotalTokens += tokensOut
			}
		}
	}
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	task.Result = map[string]any{
		"collection_id":    collectionID,
		"document_count":   len(documents),
		"exploration_log":  explorationLog,
		"errors":           searchErrors,
		"covered_topics":   finalProgress.CoveredTopics,
		"missing_topics":   finalProgress.MissingTopics,
		"researchers":      finalProgress.Researchers,
	}

	r.scheduler.Save(task)

	// Update agent status
	ag.Status = agent.StatusReady
	ag.VectorCollectionID = collectionID
	ag.DocumentCount = len(documents)
	r.registry.Update(ag)

	log.Printf("Multi-Agent creation completed: %s, documents: %d, researchers: %d, errors: %d",
		ag.Destination, len(documents), numResearchers, len(searchErrors))
}

// addProgressStep adds a progress step to the task
func (r *Router) addProgressStep(task *agent.AgentTask, stage string, progress int, message string, direction string) {
	if direction == "" {
		direction = "综合"
	}
	step := agent.ExplorationStep{
		Timestamp:   time.Now(),
		Direction:   direction,
		Thought:     message,
		Action:      stage,
		DurationMs:  int64(progress * 100),
	}

	task.ExplorationLog = append(task.ExplorationLog, step)
	r.scheduler.Save(task)
}

// getThemeKeyword returns search keywords for a theme
func (r *Router) getThemeKeyword(theme string) string {
	keywords := map[string]string{
		"cultural":  "历史文化 景点 寺庙 博物馆",
		"food":      "美食 小吃 餐厅 料理",
		"adventure": "户外 徒步 自然 探险",
		"art":       "艺术 美术馆 画廊 展览",
		"general":   "旅游 攻略",
	}
	if kw, ok := keywords[theme]; ok {
		return kw
	}
	return "旅游"
}

// needsRealtimeSearch checks if the query requires real-time search
func (r *Router) needsRealtimeSearch(query string) bool {
	// Keywords that trigger real-time search
	realtimeKeywords := []string{
		"今天", "现在", "目前", "本周", "最近", "几点", "什么时候",
		"多少钱", "票价", "费用", "价格", "收费",
		"开放时间", "营业时间", "关门", "是否开门", "营业吗",
		"天气", "气温", "下雨", "晴天", "冷不冷", "热不热",
		"怎么去", "交通", "路线", "拥堵", "地铁", "公交",
		"有什么活动", "近期", "演出", "展览",
	}

	queryLower := strings.ToLower(query)
	for _, keyword := range realtimeKeywords {
		if strings.Contains(queryLower, keyword) {
			return true
		}
	}
	return false
}

// RealtimeSearchResult holds real-time search results
type RealtimeSearchResult struct {
	Response string
	Sources  []map[string]interface{}
}

// performRealtimeSearch uses Tavily for real-time search
func (r *Router) performRealtimeSearch(ctx context.Context, destination, query string) (*RealtimeSearchResult, error) {
	if r.tavilyAPIKey == "" {
		return nil, fmt.Errorf("tavily API key not configured")
	}

	tavilyTool := tools.NewTavilySearchTool(r.tavilyAPIKey)

	// Enhance query with destination context
	enhancedQuery := fmt.Sprintf("%s %s", destination, query)

	result, err := tavilyTool.Execute(ctx, map[string]any{
		"query":       enhancedQuery,
		"max_results": 5,
	})
	if err != nil {
		return nil, err
	}

	// Extract answer and sources
	var response string
	var sources []map[string]interface{}

	if answer, ok := result["answer"].(string); ok && answer != "" {
		response = answer
	}

	if results, ok := result["results"].([]map[string]any); ok {
		sources = make([]map[string]interface{}, 0, len(results))
		for _, res := range results {
			source := map[string]interface{}{
				"title":   res["title"],
				"url":     res["url"],
				"content": truncateString(fmt.Sprintf("%v", res["content"]), 200),
				"score":   res["score"],
			}
			sources = append(sources, source)
		}
	}

	// If no answer from Tavily, construct from results
	if response == "" && len(sources) > 0 {
		response = "根据实时搜索结果：\n"
		for i, src := range sources {
			if i >= 3 {
				break
			}
			response += fmt.Sprintf("- %s: %s\n", src["title"], src["content"])
		}
	}

	return &RealtimeSearchResult{
		Response: response,
		Sources:  sources,
	}, nil
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// handleAgentAttractions returns attractions for an agent
func (r *Router) handleAgentAttractions(w http.ResponseWriter, req *http.Request) {
	agentID := req.PathValue("id")

	ag, exists := r.registry.Get(agentID)
	if !exists {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Get limit from query parameter
	limit := 10
	if l := req.URL.Query().Get("limit"); l != "" {
		if parsed, err := fmt.Sscanf(l, "%d", &limit); err == nil && parsed == 1 {
			if limit <= 0 || limit > 50 {
				limit = 10
			}
		}
	}

	// Try to get attractions from RAG
	if r.ragSvc != nil && ag.VectorCollectionID != "" {
		attractions, err := r.ragSvc.GetAttractions(req.Context(), ag.VectorCollectionID, limit)
		if err != nil {
			log.Printf("Failed to get attractions from RAG: %v", err)
		} else if len(attractions) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"destination": ag.Destination,
				"count":       len(attractions),
				"attractions": attractions,
				"source":      "knowledge_base",
			})
			return
		}
	}

	// Fallback: return mock attractions
	mockAttractions := r.getMockAttractions(ag.Destination, limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"destination": ag.Destination,
		"count":       len(mockAttractions),
		"attractions": mockAttractions,
		"source":      "mock",
	})
}

// getMockAttractions returns mock attractions for a destination
func (r *Router) getMockAttractions(destination string, limit int) []map[string]any {
	// Mock data for common destinations
	mockData := map[string][]map[string]any{
		"京都": {
			{"name": "金阁寺", "description": "世界文化遗产，金碧辉煌的禅宗寺院", "type": "寺庙"},
			{"name": "清水寺", "description": "著名的木造寺院，可俯瞰京都市区", "type": "寺庙"},
			{"name": "伏见稻荷大社", "description": "以千本鸟居闻名的神社", "type": "神社"},
			{"name": "岚山", "description": "竹林小径和渡月桥的风景名胜", "type": "自然"},
			{"name": "二条城", "description": "德川幕府的权力象征", "type": "城堡"},
		},
		"东京": {
			{"name": "东京塔", "description": "东京的标志性建筑", "type": "地标"},
			{"name": "浅草寺", "description": "东京最古老的寺院", "type": "寺庙"},
			{"name": "明治神宫", "description": "供奉明治天皇的神社", "type": "神社"},
			{"name": "涩谷十字路口", "description": "世界最繁忙的十字路口", "type": "地标"},
		},
	}

	if attractions, ok := mockData[destination]; ok {
		if len(attractions) > limit {
			return attractions[:limit]
		}
		return attractions
	}

	// Generic response
	return []map[string]any{
		{"name": destination + "主要景点", "description": "请创建 Agent 获取详细信息", "type": "景点"},
	}
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

// toolExecutorAdapter adapts tools with (map[string]any, error) return type to agent.ToolExecutor
type toolExecutorAdapter struct {
	tool interface {
		Execute(ctx context.Context, params map[string]any) (map[string]any, error)
	}
}

func (a *toolExecutorAdapter) Execute(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
	result, err := a.tool.Execute(ctx, params)
	if err != nil {
		return &agent.ToolResult{Success: false, Error: err.Error()}, err
	}

	// Convert results to []any if it's a typed slice
	if result != nil {
		if results, ok := result["results"]; ok {
			// Convert typed slices to []any
			switch v := results.(type) {
			case []tools.WikipediaSearchResult:
				anyResults := make([]any, len(v))
				for i, r := range v {
					anyResults[i] = map[string]any{
						"title":   r.Title,
						"content": r.Content,
						"url":     r.URL,
						"source":  "wikipedia",
					}
				}
				result["results"] = anyResults
			case []tools.BaiduBaikeResult:
				anyResults := make([]any, len(v))
				for i, r := range v {
					anyResults[i] = map[string]any{
						"title":   r.Title,
						"content": r.Content,
						"url":     r.URL,
						"source":  "baidu_baike",
					}
				}
				result["results"] = anyResults
			}
		}
	}

	return &agent.ToolResult{Success: true, Data: result}, nil
}