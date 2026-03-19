// Package router provides HTTP and gRPC routing
package router

import (
	"log"
	"net/http"

	"github.com/utaaa/uta-travel-agent/internal/agent"
	"github.com/utaaa/uta-travel-agent/internal/scheduler"
)

// Router handles HTTP requests and routes to appropriate handlers
type Router struct {
	registry  *agent.Registry
	scheduler *scheduler.Scheduler
	mux       *http.ServeMux
}

// NewRouter creates a new router
func NewRouter(registry *agent.Registry, scheduler *scheduler.Scheduler) *Router {
	r := &Router{
		registry:  registry,
		scheduler: scheduler,
		mux:       http.NewServeMux(),
	}

	r.setupRoutes()
	return r
}

func (r *Router) setupRoutes() {
	// Health check
	r.mux.HandleFunc("GET /health", r.handleHealth)

	// Agent endpoints
	r.mux.HandleFunc("GET /api/v1/agents", r.handleListAgents)
	r.mux.HandleFunc("GET /api/v1/agents/{id}", r.handleGetAgent)
	r.mux.HandleFunc("POST /api/v1/agents", r.handleCreateAgent)
	r.mux.HandleFunc("DELETE /api/v1/agents/{id}", r.handleDeleteAgent)

	// Task endpoints
	r.mux.HandleFunc("GET /api/v1/tasks/{id}", r.handleGetTask)
	r.mux.HandleFunc("POST /api/v1/tasks", r.handleCreateTask)

	// WebSocket for real-time communication
	r.mux.HandleFunc("GET /ws", r.handleWebSocket)
}

// Start begins serving requests
func (r *Router) Start(addr string) error {
	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, r.mux)
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

func (r *Router) handleListAgents(w http.ResponseWriter, req *http.Request) {
	// TODO: Implement with proper JSON response
	agents := r.registry.List()
	log.Printf("Listing %d agents", len(agents))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"agents":[]}`))
}

func (r *Router) handleGetAgent(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	agent, exists := r.registry.Get(id)
	if !exists {
		http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
		return
	}
	log.Printf("Getting agent: %s", agent.ID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"id":"` + id + `"}`))
}

func (r *Router) handleCreateAgent(w http.ResponseWriter, req *http.Request) {
	// TODO: Parse request body and create agent
	log.Println("Creating new agent")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"creating"}`))
}

func (r *Router) handleDeleteAgent(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if err := r.registry.Delete(id); err != nil {
		http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
		return
	}
	log.Printf("Deleted agent: %s", id)
	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) handleGetTask(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	task, exists := r.scheduler.Get(id)
	if !exists {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}
	log.Printf("Getting task: %s", task.ID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"id":"` + id + `"}`))
}

func (r *Router) handleCreateTask(w http.ResponseWriter, req *http.Request) {
	// TODO: Parse request body and create task
	log.Println("Creating new task")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"pending"}`))
}

func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request) {
	// TODO: Implement WebSocket upgrade and handling
	log.Println("WebSocket connection requested")
	http.Error(w, "WebSocket not implemented yet", http.StatusNotImplemented)
}
