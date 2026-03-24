package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/utaaa/uta-travel-agent/internal/agent"
	"github.com/utaaa/uta-travel-agent/internal/grpc/clients"
	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/memory"
	"github.com/utaaa/uta-travel-agent/internal/rag"
	"github.com/utaaa/uta-travel-agent/internal/session"
	"github.com/utaaa/uta-travel-agent/internal/storage/postgres"
	"github.com/utaaa/uta-travel-agent/internal/storage/qdrant"
	"github.com/utaaa/uta-travel-agent/internal/tools"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ToolExecutorAdapter wraps a simple Execute function to implement agent.ToolExecutor
type ToolExecutorAdapter struct {
	executeFunc func(ctx context.Context, params map[string]any) (*agent.ToolResult, error)
}

func (a *ToolExecutorAdapter) Execute(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
	return a.executeFunc(ctx, params)
}

// RealToolRegistry uses actual search tools
type RealToolRegistry struct {
	tools     map[string]agent.Tool
	executors map[string]agent.ToolExecutor
}

// NewRealToolRegistry creates a tool registry with real search tools
func NewRealToolRegistry(tavilyAPIKey, proxyURL string) *RealToolRegistry {
	registry := &RealToolRegistry{
		tools:     make(map[string]agent.Tool),
		executors: make(map[string]agent.ToolExecutor),
	}

	// Wikipedia search
	wikiSearch := tools.NewWikipediaSearchTool("zh")
	registry.tools["wikipedia_search"] = agent.Tool{
		Name:        "wikipedia_search",
		Type:        agent.ToolTypeMCP,
		Description: "Search Wikipedia for authoritative knowledge",
	}
	registry.executors["wikipedia_search"] = &ToolExecutorAdapter{
		executeFunc: func(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
			result, err := wikiSearch.Execute(ctx, params)
			if err != nil {
				return &agent.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &agent.ToolResult{Success: true, Data: result}, nil
		},
	}

	// Tavily search
	tavilySearch := tools.NewTavilySearchTool(tavilyAPIKey)
	registry.tools["tavily_search"] = agent.Tool{
		Name:        "tavily_search",
		Type:        agent.ToolTypeMCP,
		Description: "Search the web for real-time information using Tavily",
	}
	registry.executors["tavily_search"] = &ToolExecutorAdapter{
		executeFunc: func(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
			result, err := tavilySearch.Execute(ctx, params)
			if err != nil {
				return &agent.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &agent.ToolResult{Success: true, Data: result}, nil
		},
	}

	// Web reader
	webReader := tools.NewWebReaderTool()
	registry.tools["web_reader"] = agent.Tool{
		Name:        "web_reader",
		Type:        agent.ToolTypeMCP,
		Description: "Read and extract content from any web page",
	}
	registry.executors["web_reader"] = &ToolExecutorAdapter{
		executeFunc: func(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
			result, err := webReader.Execute(ctx, params)
			if err != nil {
				return &agent.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &agent.ToolResult{Success: true, Data: result}, nil
		},
	}

	// Baidu Baike search
	baiduSearch := tools.NewBaiduBaikeSearchTool()
	registry.tools["baidu_baike_search"] = agent.Tool{
		Name:        "baidu_baike_search",
		Type:        agent.ToolTypeMCP,
		Description: "Search Baidu Baike for Chinese knowledge",
	}
	registry.executors["baidu_baike_search"] = &ToolExecutorAdapter{
		executeFunc: func(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
			result, err := baiduSearch.Execute(ctx, params)
			if err != nil {
				return &agent.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &agent.ToolResult{Success: true, Data: result}, nil
		},
	}

	// brave_search as alias to tavily_search
	registry.tools["brave_search"] = agent.Tool{
		Name:        "brave_search",
		Type:        agent.ToolTypeMCP,
		Description: "Search the web for real-time information",
	}
	registry.executors["brave_search"] = &ToolExecutorAdapter{
		executeFunc: func(ctx context.Context, params map[string]any) (*agent.ToolResult, error) {
			result, err := tavilySearch.Execute(ctx, params)
			if err != nil {
				return &agent.ToolResult{Success: false, Error: err.Error()}, err
			}
			return &agent.ToolResult{Success: true, Data: result}, nil
		},
	}

	// Skill tools without real executors (fallback to mock)
	registry.tools["extract_travel_info"] = agent.Tool{
		Name:        "extract_travel_info",
		Type:        agent.ToolTypeSkill,
		Description: "Extract travel information from documents",
	}
	registry.tools["build_knowledge_base"] = agent.Tool{
		Name:        "build_knowledge_base",
		Type:        agent.ToolTypeSkill,
		Description: "Build knowledge base from curated content",
	}
	registry.tools["build_knowledge_index"] = agent.Tool{
		Name:        "build_knowledge_index",
		Type:        agent.ToolTypeSkill,
		Description: "Build vector index for RAG",
	}
	registry.tools["rag_query"] = agent.Tool{
		Name:        "rag_query",
		Type:        agent.ToolTypeSkill,
		Description: "Query the RAG knowledge base",
	}
	registry.tools["itinerary_planner"] = agent.Tool{
		Name:        "itinerary_planner",
		Type:        agent.ToolTypeSkill,
		Description: "Plan travel itineraries",
	}

	return registry
}

func (r *RealToolRegistry) Register(tool agent.Tool, executor agent.ToolExecutor) error {
	r.tools[tool.Name] = tool
	if executor != nil {
		r.executors[tool.Name] = executor
	}
	return nil
}

func (r *RealToolRegistry) Get(toolName string) (agent.Tool, bool) {
	tool, ok := r.tools[toolName]
	return tool, ok
}

func (r *RealToolRegistry) Execute(ctx context.Context, toolName string, params map[string]any) (*agent.ToolResult, error) {
	log.Printf("[TOOL] Execute called: toolName=%s", toolName)

	if executor, ok := r.executors[toolName]; ok {
		log.Printf("[TOOL] Found executor for %s, calling real implementation", toolName)
		result, err := executor.Execute(ctx, params)
		if err != nil {
			log.Printf("[TOOL] Executor error for %s: %v", toolName, err)
		} else {
			log.Printf("[TOOL] Executor success for %s", toolName)
		}
		return result, err
	}

	// Fallback for tools without executors
	log.Printf("[TOOL] No executor for %s, returning empty success", toolName)
	return &agent.ToolResult{Success: true, Data: map[string]any{"status": "processed"}}, nil
}

func (r *RealToolRegistry) ListTools() []agent.Tool {
	result := make([]agent.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

func (r *RealToolRegistry) ListByType(toolType agent.ToolType) []agent.Tool {
	result := make([]agent.Tool, 0)
	for _, tool := range r.tools {
		if tool.Type == toolType {
			result = append(result, tool)
		}
	}
	return result
}

// MockToolRegistry is a mock implementation for development
type MockToolRegistry struct {
	tools map[string]agent.Tool
}

// NewMockToolRegistry creates a mock tool registry with predefined tools
func NewMockToolRegistry() *MockToolRegistry {
	tools := map[string]agent.Tool{
		"brave_search": {
			Name:        "brave_search",
			Type:        agent.ToolTypeMCP,
			Description: "Search the web for information",
		},
		"web_reader": {
			Name:        "web_reader",
			Type:        agent.ToolTypeMCP,
			Description: "Read content from web pages",
		},
		"extract_travel_info": {
			Name:        "extract_travel_info",
			Type:        agent.ToolTypeSkill,
			Description: "Extract travel information from documents",
		},
		"build_knowledge_base": {
			Name:        "build_knowledge_base",
			Type:        agent.ToolTypeSkill,
			Description: "Build knowledge base from curated content",
		},
		"build_knowledge_index": {
			Name:        "build_knowledge_index",
			Type:        agent.ToolTypeSkill,
			Description: "Build vector index for RAG",
		},
		"rag_query": {
			Name:        "rag_query",
			Type:        agent.ToolTypeSkill,
			Description: "Query the RAG knowledge base",
		},
		"itinerary_planner": {
			Name:        "itinerary_planner",
			Type:        agent.ToolTypeSkill,
			Description: "Plan travel itineraries",
		},
	}
	return &MockToolRegistry{tools: tools}
}

func (r *MockToolRegistry) Register(tool agent.Tool, executor agent.ToolExecutor) error {
	r.tools[tool.Name] = tool
	return nil
}

func (r *MockToolRegistry) Get(toolName string) (agent.Tool, bool) {
	tool, ok := r.tools[toolName]
	return tool, ok
}

func (r *MockToolRegistry) Execute(ctx context.Context, toolName string, params map[string]any) (*agent.ToolResult, error) {
	// Return mock results based on tool name
	switch toolName {
	case "brave_search":
		return &agent.ToolResult{
			Success: true,
			Data: map[string]any{
				"results": []map[string]any{
					{"title": "京都旅游攻略", "url": "https://example.com/kyoto-guide", "description": "京都热门景点和美食推荐"},
					{"title": "金阁寺", "url": "https://example.com/kinkakuji", "description": "世界文化遗产，金阁寺介绍"},
					{"title": "清水寺", "url": "https://example.com/kiyomizudera", "description": "京都必去景点清水寺"},
				},
			},
		}, nil
	case "web_reader":
		return &agent.ToolResult{
			Success: true,
			Data: map[string]any{
				"content": "京都拥有众多世界文化遗产，包括金阁寺、清水寺、伏见稻荷大社等著名景点。春季的樱花和秋季的红叶是最受欢迎的旅游季节。",
			},
		}, nil
	case "extract_travel_info":
		return &agent.ToolResult{
			Success: true,
			Data: map[string]any{
				"attractions": []string{"金阁寺", "清水寺", "伏见稻荷大社"},
				"food":        []string{"抹茶甜点", "京都料理", "豆腐料理"},
				"culture":     []string{"茶道", "花道", "和服体验"},
			},
		}, nil
	case "build_knowledge_base":
		return &agent.ToolResult{
			Success: true,
			Data: map[string]any{
				"categories":  []string{"景点", "美食", "文化", "交通"},
				"document_id": "kb-001",
			},
		}, nil
	case "build_knowledge_index":
		return &agent.ToolResult{
			Success: true,
			Data: map[string]any{
				"collection_id": "col-kyoto-001",
				"vector_count":  128,
			},
		}, nil
	case "rag_query":
		return &agent.ToolResult{
			Success: true,
			Data: map[string]any{
				"answer":      "金阁寺是京都最著名的景点之一，正式名称为鹿苑寺，因其金碧辉煌的外观而闻名。",
				"confidence":  0.95,
				"sources":     []string{"kb-kyoto-001"},
			},
		}, nil
	case "itinerary_planner":
		return &agent.ToolResult{
			Success: true,
			Data: map[string]any{
				"itinerary": "京都三日游行程规划",
				"days": []map[string]any{
					{"day": 1, "activities": []string{"金阁寺", "岚山", "竹林小径"}},
					{"day": 2, "activities": []string{"清水寺", "祇园", "花见小路"}},
					{"day": 3, "activities": []string{"伏见稻荷大社", "千本鸟居", "奈良一日游"}},
				},
			},
		}, nil
	default:
		return &agent.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}
}

func (r *MockToolRegistry) ListTools() []agent.Tool {
	tools := make([]agent.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

func (r *MockToolRegistry) ListByType(toolType agent.ToolType) []agent.Tool {
	var tools []agent.Tool
	for _, tool := range r.tools {
		if tool.Type == toolType {
			tools = append(tools, tool)
		}
	}
	return tools
}

// MemorySessionStore is an in-memory implementation of session storage
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*session.Session
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*session.Session),
	}
}

func (s *MemorySessionStore) Create(ctx context.Context, sess *session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID()] = sess
	return nil
}

func (s *MemorySessionStore) Get(ctx context.Context, id string) (*session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return sess, nil
}

func (s *MemorySessionStore) Update(ctx context.Context, sess *session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID()] = sess
	return nil
}

func (s *MemorySessionStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func (s *MemorySessionStore) List(ctx context.Context, opts session.ListOptions) (*session.ListResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*session.Session
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}

	// Group by date
	grouped := session.GroupSessionsByDate(sessions)

	return &session.ListResult{
		Sessions: sessions,
		Grouped:  grouped,
		Total:    len(sessions),
	}, nil
}

func (s *MemorySessionStore) ListByUser(ctx context.Context, userID string, limit int) ([]*session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*session.Session
	for _, sess := range s.sessions {
		if sess.Metadata()["user_id"] == userID {
			sessions = append(sessions, sess)
		}
	}
	return sessions, nil
}

func (s *MemorySessionStore) ListByAgentType(ctx context.Context, agentType string, limit int) ([]*session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*session.Session
	for _, sess := range s.sessions {
		if sess.AgentType() == agentType {
			sessions = append(sessions, sess)
		}
	}
	return sessions, nil
}

// Server is the main API server
type Server struct {
	mainAgent       *agent.MainAgent
	httpPort        int
	sessionStore    session.Storage  // 使用接口，支持 PostgreSQL 持久化
	memoryStorage   session.MemoryStorage  // 消息持久化存储
	agentRepo       AgentRepository

	// User-scoped memory cache for cross-session preferences
	// Key: userID, Value: *memory.PersistentMemory
	userMemoryCache sync.Map
	memoryStorageBackend *memoryStorageAdapter  // Adapter for memory.Storage interface
}

// memoryStorageAdapter adapts session.MemoryStorage to memory.Storage interface
type memoryStorageAdapter struct {
	backend session.MemoryStorage
}

func (a *memoryStorageAdapter) Save(ctx context.Context, sessionID string, snapshot *memory.Snapshot) error {
	return a.backend.Save(ctx, sessionID, snapshot)
}

func (a *memoryStorageAdapter) Load(ctx context.Context, sessionID string) (*memory.Snapshot, error) {
	return a.backend.Load(ctx, sessionID)
}

func (a *memoryStorageAdapter) Delete(ctx context.Context, sessionID string) error {
	return a.backend.Delete(ctx, sessionID)
}

// ragServiceAdapter adapts rag.Service to agent.RAGService interface
type ragServiceAdapter struct {
	*rag.Service
}

func (a *ragServiceAdapter) Query(ctx context.Context, collectionID, query string, limit int) (*agent.RAGResult, error) {
	result, err := a.Service.Query(ctx, collectionID, query, limit)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	// Convert rag.QueryResult to agent.RAGResult
	agentResult := &agent.RAGResult{
		Answer:  result.Answer,
		Sources: make([]string, len(result.Sources)),
		Score:   float64(result.TokensUsed),
	}

	for i, src := range result.Sources {
		agentResult.Sources[i] = src.Content
	}

	return agentResult, nil
}

// getUserMemory returns or creates a user-scoped PersistentMemory for cross-session preferences.
// The userID is used as the key prefix for storing long-term memory items.
// If userID is empty, returns nil (no user-scoped memory available).
func (s *Server) getUserMemory(userID string) *memory.PersistentMemory {
	if userID == "" {
		return nil
	}

	// Check cache first
	if cached, ok := s.userMemoryCache.Load(userID); ok {
		return cached.(*memory.PersistentMemory)
	}

	// Create new user-scoped memory with storage backend
	userMemKey := "user:" + userID
	userMem := memory.NewPersistentMemory(s.memoryStorageBackend, 100)

	// Load existing long-term memory from storage if available
	if s.memoryStorageBackend != nil {
		ctx := context.Background()
		if snapshot, err := s.memoryStorageBackend.Load(ctx, userMemKey); err == nil {
			// Restore long-term memory from snapshot
			for _, item := range snapshot.LongTerm {
				userMem.AddToLongTerm(item)
			}
			log.Printf("[Memory] Loaded %d long-term items for user %s", len(snapshot.LongTerm), userID)
		}
	}

	// Cache the memory
	s.userMemoryCache.Store(userID, userMem)
	return userMem
}

// saveUserMemory persists user-scoped memory to storage.
// This should be called after updating user preferences.
// It ensures a user session record exists before saving to satisfy foreign key constraints.
func (s *Server) saveUserMemory(userID string, mem *memory.PersistentMemory) {
	if userID == "" || mem == nil || s.memoryStorageBackend == nil {
		return
	}

	userMemKey := "user:" + userID
	ctx := context.Background()

	// Ensure user session record exists (required for foreign key constraint)
	// This creates a special "user preferences" session that persists across all user's sessions
	if s.sessionStore != nil {
		userSession, err := s.sessionStore.Get(ctx, userMemKey)
		if err != nil || userSession == nil {
			// Create new user session for preferences storage
			userSession = session.New(userMemKey)
			userSession.SetMetadata("type", "user_preferences")
			userSession.SetMetadata("user_id", userID)
			if err := s.sessionStore.Create(ctx, userSession); err != nil {
				log.Printf("[Memory] Failed to create user session for %s: %v", userID, err)
				// Continue anyway - the save might still work if session exists
			} else {
				log.Printf("[Memory] Created user session for %s", userID)
			}
		}
	}

	if err := mem.Save(ctx, userMemKey); err != nil {
		log.Printf("[Memory] Failed to save user memory for %s: %v", userID, err)
	}
}

// AgentRepository interface for agent persistence
type AgentRepository interface {
	SaveAgent(ctx context.Context, ag *agent.DestinationAgent) error
	GetAgent(ctx context.Context, id string) (*agent.DestinationAgent, error)
	ListAgentsByUser(ctx context.Context, userID string) ([]*agent.DestinationAgent, error)
	DeleteAgent(ctx context.Context, id string) error

	// Task operations
	SaveTask(ctx context.Context, task *agent.AgentTask) error
	GetTask(ctx context.Context, id string) (*agent.AgentTask, error)
	UpdateTask(ctx context.Context, task *agent.AgentTask) error
}

// MemoryAgentRepository is an in-memory implementation of AgentRepository
type MemoryAgentRepository struct {
	sync.RWMutex
	agents map[string]*agent.DestinationAgent
	tasks  map[string]*agent.AgentTask
}

// NewMemoryAgentRepository creates a new in-memory agent repository
func NewMemoryAgentRepository() *MemoryAgentRepository {
	return &MemoryAgentRepository{
		agents: make(map[string]*agent.DestinationAgent),
		tasks:  make(map[string]*agent.AgentTask),
	}
}

func (r *MemoryAgentRepository) SaveAgent(ctx context.Context, ag *agent.DestinationAgent) error {
	r.Lock()
	defer r.Unlock()
	r.agents[ag.ID] = ag
	return nil
}

func (r *MemoryAgentRepository) GetAgent(ctx context.Context, id string) (*agent.DestinationAgent, error) {
	r.RLock()
	defer r.RUnlock()
	ag, ok := r.agents[id]
	if !ok {
		return nil, agent.ErrAgentNotFound
	}
	return ag, nil
}

func (r *MemoryAgentRepository) ListAgentsByUser(ctx context.Context, userID string) ([]*agent.DestinationAgent, error) {
	r.RLock()
	defer r.RUnlock()
	result := make([]*agent.DestinationAgent, 0)
	for _, ag := range r.agents {
		if ag.UserID == userID {
			result = append(result, ag)
		}
	}
	return result, nil
}

func (r *MemoryAgentRepository) DeleteAgent(ctx context.Context, id string) error {
	r.Lock()
	defer r.Unlock()
	delete(r.agents, id)
	return nil
}

func (r *MemoryAgentRepository) SaveTask(ctx context.Context, task *agent.AgentTask) error {
	r.Lock()
	defer r.Unlock()
	r.tasks[task.ID] = task
	return nil
}

func (r *MemoryAgentRepository) GetTask(ctx context.Context, id string) (*agent.AgentTask, error) {
	r.RLock()
	defer r.RUnlock()
	task, ok := r.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	return task, nil
}

func (r *MemoryAgentRepository) UpdateTask(ctx context.Context, task *agent.AgentTask) error {
	r.Lock()
	defer r.Unlock()
	r.tasks[task.ID] = task
	return nil
}

// taskRepo is a fallback in-memory task repository for tasks without database storage
var taskRepo = NewMemoryTaskRepository()

// MemoryTaskRepository is a simple in-memory task store
type MemoryTaskRepository struct {
	sync.RWMutex
	tasks map[string]*agent.AgentTask
}

func NewMemoryTaskRepository() *MemoryTaskRepository {
	return &MemoryTaskRepository{
		tasks: make(map[string]*agent.AgentTask),
	}
}

func (r *MemoryTaskRepository) GetTask(ctx context.Context, id string) (*agent.AgentTask, error) {
	r.RLock()
	defer r.RUnlock()
	task, ok := r.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	return task, nil
}

// InMemoryMessageStore is an in-memory implementation of session.MemoryStorage
type InMemoryMessageStore struct {
	sync.RWMutex
	snapshots map[string]*memory.Snapshot
}

// NewInMemoryMessageStore creates a new in-memory message store
func NewInMemoryMessageStore() *InMemoryMessageStore {
	return &InMemoryMessageStore{
		snapshots: make(map[string]*memory.Snapshot),
	}
}

// Save saves a memory snapshot
func (s *InMemoryMessageStore) Save(ctx context.Context, sessionID string, snapshot *memory.Snapshot) error {
	s.Lock()
	defer s.Unlock()
	snapshot.UpdatedAt = time.Now()
	s.snapshots[sessionID] = snapshot
	return nil
}

// Load loads a memory snapshot
func (s *InMemoryMessageStore) Load(ctx context.Context, sessionID string) (*memory.Snapshot, error) {
	s.RLock()
	defer s.RUnlock()
	snapshot, ok := s.snapshots[sessionID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", sessionID)
	}
	return snapshot, nil
}

// Delete deletes a memory snapshot
func (s *InMemoryMessageStore) Delete(ctx context.Context, sessionID string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.snapshots, sessionID)
	return nil
}

// Config for the server
type Config struct {
	HTTPPort       int
	LLMProvider    string // "mock", "glm", "deepseek", "grpc"
	LLMGRPCAddr    string
	GLMAPIKey      string
	GLMModel       string
	DeepSeekAPIKey string
	DeepSeekModel  string
	TavilyAPIKey   string
	ProxyURL       string
	// PostgreSQL config
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDatabase string
	PostgresSSLMode  string
	// Feature flags
	EnableRAG bool // Enable RAG knowledge retrieval
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as int with default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// NewServer creates a new API server
func NewServer(config Config) *Server {
	// Connect to PostgreSQL
	var agentRepo AgentRepository
	var pgClient *postgres.Client

	if config.PostgresHost != "" {
		var err error
		pgClient, err = postgres.NewClient(postgres.Config{
			Host:     config.PostgresHost,
			Port:     config.PostgresPort,
			User:     config.PostgresUser,
			Password: config.PostgresPassword,
			Database: config.PostgresDatabase,
			SSLMode:  config.PostgresSSLMode,
		})
		if err != nil {
			log.Printf("Failed to connect to PostgreSQL: %v, using in-memory storage", err)
		} else {
			log.Println("Connected to PostgreSQL successfully")
			agentRepo = postgres.NewAgentRepository(pgClient.DB())
		}
	}

	// Fallback to in-memory repository if no database
	if agentRepo == nil {
		agentRepo = NewMemoryAgentRepository()
	}

	// Create session storage - use PostgreSQL if available
	var sessionStore session.Storage
	var memoryStorage session.MemoryStorage

	if pgClient != nil {
		sessionStore = session.NewPostgreSQLStorage(pgClient.DB())
		memoryStorage = session.NewPostgreSQLMemoryStorage(pgClient.DB())
		log.Println("Using PostgreSQL session storage (persistent)")
	} else {
		sessionStore = NewMemorySessionStore()
		memoryStorage = NewInMemoryMessageStore()
		log.Println("Using in-memory session storage (not persistent)")
	}

	// Create LLM provider
	var llmProvider llm.Provider
	if config.LLMProvider == "mock" {
		log.Println("Using Mock LLM Provider")
		llmProvider = llm.NewMockProvider("你好！我是 UTA Travel 智能旅游助手。我可以帮助你规划旅行、了解目的地信息。请问有什么可以帮助你的？")
	} else if config.LLMProvider == "glm" {
		log.Printf("Using GLM Provider with model %s", config.GLMModel)
		llmProvider = llm.NewGLMProvider(llm.GLMConfig{
			APIKey: config.GLMAPIKey,
			Model:  config.GLMModel,
		})
	} else if config.LLMProvider == "deepseek" {
		log.Printf("Using DeepSeek Provider with model %s", config.DeepSeekModel)
		llmProvider = llm.NewDeepSeekProvider(llm.DeepSeekConfig{
			APIKey: config.DeepSeekAPIKey,
			Model:  config.DeepSeekModel,
		})
	} else {
		log.Printf("Using gRPC LLM Provider at %s", config.LLMGRPCAddr)
		llmProvider = llm.NewMockProvider("gRPC LLM 服务连接中...")
	}

	// Create RAG service with Qdrant (only if enabled)
	var ragService agent.RAGService
	if config.EnableRAG {
		qdrantClient, err := qdrant.NewClient(qdrant.Config{
			Host: "localhost",
			Port: 6334, // Qdrant gRPC port
		})
		if err != nil {
			log.Printf("Failed to create Qdrant client: %v, RAG will be disabled", err)
		} else {
			// Create embedding client
			embeddingAddr := getEnv("EMBEDDING_ADDR", "localhost:50052")
			embeddingConn, err := grpc.NewClient(embeddingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Printf("Failed to connect to embedding service: %v, RAG will be disabled", err)
			} else {
				embeddingClient := clients.NewEmbeddingClient(embeddingConn)
				ragSvc := rag.NewService(rag.Config{
					QdrantClient:    qdrantClient,
					LLMProvider:     llmProvider,
					EmbeddingClient: embeddingClient,
				})
				ragService = &ragServiceAdapter{ragSvc}
				log.Printf("RAG service initialized with Qdrant and Embedding service at %s", embeddingAddr)
			}
		}
	} else {
		log.Println("RAG is disabled (set ENABLE_RAG=true to enable)")
	}

	// Create main agent with RAG support
	mainAgent := agent.NewMainAgent(agent.MainAgentConfig{
		ID:          "main-agent-001",
		LLMProvider: llmProvider,
		RAGService:  ragService,
		Template: &agent.AgentTemplate{
			Name:         "UTA Main Agent",
			Description:  "Main orchestrator agent for UTA Travel",
			SystemPrompt: "你是 UTA Travel 的智能旅游助手。",
			Spec: agent.TemplateSpec{
				Role: "你是 UTA Travel 的智能旅游助手。你的任务是为用户提供专业、友好的旅游建议和信息。",
			},
		},
	})

	// Create real tool registry with actual search tools
	toolRegistry := NewRealToolRegistry(config.TavilyAPIKey, config.ProxyURL)

	// Create and register subagents - NOW WITH LLM BRAIN!
	// Each subagent is a complete Agent with: Memory, Context, Prompt, Action Flow, LLM Brain

	researcherPrompt := `你是一位专业的旅游信息研究员。你的任务是搜集目的地的旅游信息。

## 角色
你是一名经验丰富的旅游信息收集专家，擅长从各种渠道获取准确、有价值的旅游信息。

## 职责
1. 搜索目的地的景点、美食、文化、交通等信息
2. 评估信息的可靠性和价值
3. 发现值得深入探索的新方向

## 工具
- brave_search: 搜索网络信息
- web_reader: 读取网页内容
- extract_travel_info: 提取结构化旅游信息

## 工作流程
1. 先思考需要收集哪些信息
2. 使用工具搜索和获取信息
3. 评估结果，决定是否需要继续探索
4. 当收集到足够信息时，主动结束任务

## 输出格式
完成任务时，请总结你收集到的信息概要。`

	researcher := agent.NewLLMAgent(agent.LLMAgentConfig{
		ID:          "researcher-001",
		AgentType:   agent.AgentTypeResearcher,
		LLMProvider: llmProvider,
		SystemPrompt: researcherPrompt,
		Tools:       toolRegistry,
		MaxIterations: 10,
	})
	if err := mainAgent.RegisterSubagent(researcher); err != nil {
		log.Printf("Failed to register researcher: %v", err)
	} else {
		log.Println("Registered Researcher Agent (with LLM Brain)")
	}

	curatorPrompt := `你是一位专业的旅游信息整理师。你的任务是整理和结构化收集到的旅游信息。

## 角色
你是一名细心的信息整理专家，擅长将杂乱的信息组织成结构化的知识库。

## 职责
1. 对收集的信息进行分类（景点、美食、文化、交通等）
2. 去重和验证信息质量
3. 构建信息之间的关联关系

## 工具
- build_knowledge_base: 构建知识库

## 工作流程
1. 分析输入的原始信息
2. 按类别整理信息
3. 标记高质量信息和待验证信息
4. 输出结构化的知识库

## 输出格式
完成任务时，请输出整理后的知识库结构和统计信息。`

	curator := agent.NewLLMAgent(agent.LLMAgentConfig{
		ID:          "curator-001",
		AgentType:   agent.AgentTypeCurator,
		LLMProvider: llmProvider,
		SystemPrompt: curatorPrompt,
		Tools:       toolRegistry,
		MaxIterations: 5,
	})
	if err := mainAgent.RegisterSubagent(curator); err != nil {
		log.Printf("Failed to register curator: %v", err)
	} else {
		log.Println("Registered Curator Agent (with LLM Brain)")
	}

	indexerPrompt := `你是一位专业的向量索引工程师。你的任务是将整理好的信息构建成向量索引。

## 角色
你是一名技术专家，擅长处理文本向量和构建高效的检索系统。

## 职责
1. 将文本切分成合适的块
2. 生成向量嵌入
3. 存入向量数据库

## 工具
- build_knowledge_index: 构建向量索引

## 工作流程
1. 分析输入的文本内容
2. 按语义边界切分文本
3. 生成向量并存储
4. 验证索引质量

## 输出格式
完成任务时，请输出索引的统计信息（文档数、向量数、索引ID等）。`

	indexer := agent.NewLLMAgent(agent.LLMAgentConfig{
		ID:          "indexer-001",
		AgentType:   agent.AgentTypeIndexer,
		LLMProvider: llmProvider,
		SystemPrompt: indexerPrompt,
		Tools:       toolRegistry,
		MaxIterations: 5,
	})
	if err := mainAgent.RegisterSubagent(indexer); err != nil {
		log.Printf("Failed to register indexer: %v", err)
	} else {
		log.Println("Registered Indexer Agent (with LLM Brain)")
	}

	plannerPrompt := `你是一位专业的旅行规划师。你的任务是根据用户需求制定个性化的旅行行程。

## 角色
你是一名经验丰富的旅行规划专家，熟悉世界各地的旅游资源。

## 职责
1. 理解用户的旅行偏好和约束条件
2. 规划合理的行程路线
3. 平衡时间、预算和体验

## 工具
- itinerary_planner: 生成行程规划

## 工作流程
1. 分析用户需求
2. 筛选合适的景点和活动
3. 安排合理的时间表
4. 优化行程路线

## 输出格式
输出详细的行程安排，包括每天的活动、时间安排和注意事项。`

	planner := agent.NewLLMAgent(agent.LLMAgentConfig{
		ID:          "planner-001",
		AgentType:   agent.AgentTypePlanner,
		LLMProvider: llmProvider,
		SystemPrompt: plannerPrompt,
		Tools:       toolRegistry,
		MaxIterations: 5,
	})
	if err := mainAgent.RegisterSubagent(planner); err != nil {
		log.Printf("Failed to register planner: %v", err)
	} else {
		log.Println("Registered Planner Agent (with LLM Brain)")
	}

	return &Server{
		mainAgent:            mainAgent,
		httpPort:             config.HTTPPort,
		sessionStore:         sessionStore,
		memoryStorage:        memoryStorage,
		agentRepo:            agentRepo,
		memoryStorageBackend: &memoryStorageAdapter{backend: memoryStorage},
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	// Agent endpoints
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/chat/stream", s.handleChatStream)
	mux.HandleFunc("/api/v1/agent/status", s.handleAgentStatus)
	mux.HandleFunc("/api/v1/agent/create", s.handleCreateDestinationAgent)
	mux.HandleFunc("/api/v1/agent/task/", s.handleTaskDetails) // Task details with exploration log

	// Agents list endpoint (for destinations page)
	mux.HandleFunc("/api/v1/agents", s.handleAgents)
	mux.HandleFunc("/api/v1/agents/", s.handleAgentByID)

	// Task endpoints (for progress tracking)
	mux.HandleFunc("/api/v1/tasks/", s.handleTaskByID)

	// Session endpoints
	mux.HandleFunc("/api/v1/sessions", s.handleSessions)
	mux.HandleFunc("/api/v1/sessions/", s.handleSessionByID)

	// CORS middleware
	handler := s.corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", s.httpPort)
	log.Printf("Starting API server on %s", addr)
	return http.ListenAndServe(addr, handler)
}

// CORS middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[REQUEST] %s %s", r.Method, r.URL.Path)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "uta-api-gateway",
	})
}

// ChatRequest for chat API
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse for chat API
type ChatResponse struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id"`
	Timestamp int64  `json:"timestamp"`
}

// handleChat handles chat requests
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	response, err := s.mainAgent.Chat(ctx, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{
		Response:  response,
		SessionID: s.mainAgent.ID(),
		Timestamp: time.Now().Unix(),
	})
}

// handleChatStream handles streaming chat requests
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	chunkCh, errCh := s.mainAgent.ChatStream(ctx, req.Message)

	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				fmt.Fprintf(w, "data: [DONE]\n\n")
				w.(http.Flusher).Flush()
				return
			}
			// Use JSON encoding to properly handle newlines and special characters
			jsonData, _ := json.Marshal(map[string]string{"content": chunk})
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			w.(http.Flusher).Flush()
		case err := <-errCh:
			if err != nil {
				fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", err.Error())
				w.(http.Flusher).Flush()
			}
			return
		case <-ctx.Done():
			return
		}
	}
}

// AgentStatusResponse for status API
type AgentStatusResponse struct {
	AgentID    string      `json:"agent_id"`
	Type       string      `json:"type"`
	State      string      `json:"state"`
	Subagents  []string    `json:"subagents"`
	MemorySize int         `json:"memory_size"`
	Metadata   interface{} `json:"metadata,omitempty"`
}

// handleAgentStatus returns agent status
func (s *Server) handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	subagents := s.mainAgent.ListSubagents()
	subagentTypes := make([]string, len(subagents))
	for i, sa := range subagents {
		subagentTypes[i] = string(sa.Type())
	}

	response := AgentStatusResponse{
		AgentID:    s.mainAgent.ID(),
		Type:       string(s.mainAgent.Type()),
		State:      string(s.mainAgent.State()),
		Subagents:  subagentTypes,
		MemorySize: s.mainAgent.Memory().Size(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateDestinationAgentRequest for creating destination agents
type CreateDestinationAgentRequest struct {
	Destination string   `json:"destination"`
	Theme       string   `json:"theme,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CreateDestinationAgentResponse for creation response
type CreateDestinationAgentResponse struct {
	AgentID     string `json:"agent_id"`
	Destination string `json:"destination"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	TaskID      string `json:"task_id"`
}

// handleCreateDestinationAgent creates a new destination agent
func (s *Server) handleCreateDestinationAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateDestinationAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Destination == "" {
		http.Error(w, "destination is required", http.StatusBadRequest)
		return
	}

	// Generate agent ID upfront
	agentID := generateID()
	now := time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	log.Printf("Starting parallel research for %s...", req.Destination)

	// Use RunParallelResearch to actually collect documents
	// This creates dedicated ResearcherAgents that store documents in SharedKnowledgeState
	researchResult, err := s.mainAgent.RunParallelResearch(ctx, req.Destination, req.Theme, func(progress string) {
		log.Printf("[Progress] %s", progress)
	})
	if err != nil {
		log.Printf("Parallel research failed: %v", err)
		// Fallback to simple Run if parallel research fails
		goal := fmt.Sprintf("创建 %s 的目的地 Agent", req.Destination)
		_, fallbackErr := s.mainAgent.Run(ctx, goal)
		if fallbackErr != nil {
			log.Printf("Fallback also failed: %v", fallbackErr)
		}
	}

	documentCount := 0
	if researchResult != nil {
		documentCount = researchResult.TotalDocuments
		log.Printf("Research completed: %d documents collected for %s", documentCount, req.Destination)
	}

	// Create the destination agent record
	ag := &agent.DestinationAgent{
		ID:            agentID,
		UserID:        "default",
		Name:          fmt.Sprintf("%s导游助手", req.Destination),
		Description:   fmt.Sprintf("%s智能导游", req.Destination),
		Destination:   req.Destination,
		Theme:         req.Theme,
		Status:        "ready",
		DocumentCount: documentCount,
		Language:      "zh",
		Tags:          req.Languages,
		CreatedAt:     now,
		UpdatedAt:     now,
		UsageCount:    0,
		Rating:        0,
	}

	// Save to repository
	if s.agentRepo != nil {
		if err := s.agentRepo.SaveAgent(ctx, ag); err != nil {
			log.Printf("Failed to save agent to database: %v", err)
		} else {
			log.Printf("Agent saved to database: %s", agentID)
		}
	} else {
		log.Printf("Warning: agentRepo is nil, agent not saved to database")
	}

	// Create and save task record
	taskID := fmt.Sprintf("task-%s", agentID)

	// Extract data from research result
	var totalTokens int
	var explorationLog []agent.ExplorationStep
	var coveredTopics map[string]int
	var researchers []map[string]any

	if researchResult != nil {
		totalTokens = researchResult.TotalTokensIn + researchResult.TotalTokensOut
		explorationLog = researchResult.ExplorationLog
		coveredTopics = researchResult.CoveredTopics

		// Generate researchers data for radar chart
		researchers = make([]map[string]any, 0)
		for i, topic := range researchResult.Topics {
			// Map topic name to English key for frontend
			topicKey := agent.TopicNameToKey(topic.Topic.Name)
			researchers = append(researchers, map[string]any{
				"ID":             fmt.Sprintf("researcher-%d", i+1),
				"CurrentRound":   5,
				"MaxRounds":      5,
				"CurrentTopic":   topicKey,
				"DocumentsFound": len(topic.Documents),
				"Status":         "complete",
			})
		}

		log.Printf("[CreateDestinationAgent] Task data: tokens=%d, exploration_steps=%d, topics=%v, researchers=%d",
			totalTokens, len(explorationLog), coveredTopics, len(researchers))
	} else {
		coveredTopics = make(map[string]int)
		researchers = []map[string]any{}
	}

	task := &agent.AgentTask{
		ID:              taskID,
		AgentID:         agentID,
		UserID:          "default",
		Status:          "completed",
		Goal:            fmt.Sprintf("创建%s导游助手", req.Destination),
		DurationSeconds: time.Since(now).Seconds(),
		TotalTokens:     totalTokens,
		ExplorationLog:  explorationLog,
		Result: map[string]any{
			"document_count":  documentCount,
			"destination":     req.Destination,
			"theme":           req.Theme,
			"covered_topics":  coveredTopics,
			"missing_topics":  []string{},
			"collection_id":   agentID,
			"researchers":     researchers,
		},
		CreatedAt:   now,
		CompletedAt: &now,
	}

	if s.agentRepo != nil {
		if err := s.agentRepo.SaveTask(ctx, task); err != nil {
			log.Printf("[CreateDestinationAgent] Warning: Failed to save task: %v", err)
		} else {
			log.Printf("[CreateDestinationAgent] Task saved: %s", taskID)
		}
	}

	response := CreateDestinationAgentResponse{
		AgentID:     agentID,
		Destination: req.Destination,
		Status:      "created",
		Message:     fmt.Sprintf("目的地 Agent 创建成功，收集 %d 篇文档， 用 %.1f 秒", documentCount, time.Since(now).Seconds()),
		TaskID:      taskID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// TaskDetailsResponse for task details API
type TaskDetailsResponse struct {
	TaskID         string                   `json:"task_id"`
	AgentID        string                   `json:"agent_id"`
	Status         string                   `json:"status"`
	Duration       float64                  `json:"duration_seconds"`
	TotalTokens    int                      `json:"total_tokens"`
	ExplorationLog []agent.ExplorationStep  `json:"exploration_log"`
	RadarData      *RadarDataResponse       `json:"radar_data,omitempty"`
	Metadata       map[string]any           `json:"metadata,omitempty"`
}

// RadarDataResponse for radar chart visualization
type RadarDataResponse struct {
	Directions []RadarDirection `json:"directions"`
}

// RadarDirection represents one direction on the radar chart
type RadarDirection struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`     // 0-100, how much exploration in this direction
	AgentID   string  `json:"agent_id"`
	LastUpdate string `json:"last_update"`
}

// handleTaskDetails returns task details with exploration log for radar chart
func (s *Server) handleTaskDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from URL
	taskID := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/task/")
	if taskID == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}

	// Get agent status
	agentStatus := s.mainAgent.State()
	memorySize := s.mainAgent.Memory().Size()

	// Get exploration logs from subagents
	var allExplorations []agent.ExplorationStep
	for _, subagent := range s.mainAgent.ListSubagents() {
		if llmAg, ok := subagent.(*agent.LLMAgent); ok {
			allExplorations = append(allExplorations, llmAg.GetExplorationLog()...)
		}
	}

	// Calculate radar data from explorations
	radarData := calculateRadarData(allExplorations)

	// Calculate totals
	var totalTokensIn, totalTokensOut int
	for _, exp := range allExplorations {
		totalTokensIn += exp.TokensIn
		totalTokensOut += exp.TokensOut
	}

	response := TaskDetailsResponse{
		TaskID:         taskID,
		AgentID:        s.mainAgent.ID(),
		Status:         string(agentStatus),
		Duration:       0, // Would be tracked per task
		TotalTokens:    totalTokensIn + totalTokensOut,
		ExplorationLog: allExplorations,
		RadarData:      radarData,
		Metadata: map[string]any{
			"memory_size":     memorySize,
			"subagent_count":  len(s.mainAgent.ListSubagents()),
			"exploration_count": len(allExplorations),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// calculateRadarData calculates radar chart data from exploration logs
func calculateRadarData(explorations []agent.ExplorationStep) *RadarDataResponse {
	directionCounts := make(map[string]int)
	directionLastUpdate := make(map[string]string)

	// Count explorations per direction
	for _, exp := range explorations {
		directionCounts[exp.Direction]++
		if exp.Timestamp.Format(time.RFC3339) > directionLastUpdate[exp.Direction] {
			directionLastUpdate[exp.Direction] = exp.Timestamp.Format(time.RFC3339)
		}
	}

	// Ensure all directions exist with at least 0
	allDirections := []string{"景点", "美食", "文化", "交通", "住宿", "购物"}
	for _, dir := range allDirections {
		if _, exists := directionCounts[dir]; !exists {
			directionCounts[dir] = 0
			directionLastUpdate[dir] = ""
		}
	}

	// Calculate max for normalization
	maxCount := 1
	for _, count := range directionCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	// Build radar data
	directions := make([]RadarDirection, 0, len(directionCounts))
	for _, dir := range allDirections {
		value := float64(directionCounts[dir]) / float64(maxCount) * 100
		directions = append(directions, RadarDirection{
			Name:       dir,
			Value:      value,
			LastUpdate: directionLastUpdate[dir],
		})
	}

	return &RadarDataResponse{
		Directions: directions,
	}
}

// Agent API handlers

// In-memory store for created agents (simple implementation)
var agentsStore = struct {
	sync.RWMutex
	agents map[string]AgentInfo
}{
	agents: make(map[string]AgentInfo),
}

// AgentInfo represents a destination agent
type AgentInfo struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Destination   string    `json:"destination"`
	Theme         string    `json:"theme"`
	Status        string    `json:"status"`
	DocumentCount int       `json:"document_count"`
	Language      string    `json:"language"`
	Tags          []string  `json:"tags"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	UsageCount    int       `json:"usage_count"`
	Rating        float64   `json:"rating"`
}

// handleAgents handles GET (list) and POST (create) for agents
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listAgents(w, r)
	case http.MethodPost:
		s.createAgent(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAgentByID handles operations on a specific agent
func (s *Server) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	if path == "" || strings.Contains(path, "/") {
		// Check if it's a sub-resource
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			agentID := parts[0]
			subResource := parts[1]
			switch subResource {
			case "chat":
				s.chatWithAgent(r.Context(), w, r, agentID)
			case "chat/stream":
				s.chatStreamWithAgent(r.Context(), w, r, agentID)
			case "attractions":
				s.getAgentAttractions(r.Context(), w, r, agentID)
			case "task":
				s.getAgentTask(r.Context(), w, r, agentID)
			case "tasks":
				s.listAgentTasks(r.Context(), w, r, agentID)
			case "sessions":
				if r.Method == http.MethodPost {
					s.createSessionForAgent(r.Context(), w, r, agentID)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			default:
				http.Error(w, "Not found", http.StatusNotFound)
			}
			return
		}
		http.Error(w, "Invalid agent ID", http.StatusBadRequest)
		return
	}
	agentID := path

	switch r.Method {
	case http.MethodGet:
		s.getAgent(w, r, agentID)
	case http.MethodDelete:
		s.deleteAgent(w, r, agentID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agents, err := s.agentRepo.ListAgentsByUser(ctx, "default")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list agents: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	result := make([]map[string]interface{}, len(agents))
	for i, ag := range agents {
		result[i] = map[string]interface{}{
			"id":             ag.ID,
			"user_id":        ag.UserID,
			"name":           ag.Name,
			"description":    ag.Description,
			"destination":    ag.Destination,
			"theme":          ag.Theme,
			"status":         ag.Status,
			"document_count": ag.DocumentCount,
			"language":       ag.Language,
			"tags":           ag.Tags,
			"created_at":     ag.CreatedAt,
			"updated_at":     ag.UpdatedAt,
			"usage_count":    ag.UsageCount,
			"rating":         ag.Rating,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": result,
		"total":  len(result),
	})
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		Destination string   `json:"destination"`
		Theme       string   `json:"theme"`
		Languages   []string `json:"languages"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Destination == "" {
		http.Error(w, "Destination is required", http.StatusBadRequest)
		return
	}

	// Create agent
	agentID := generateID()
	now := time.Now()
	ag := &agent.DestinationAgent{
		ID:            agentID,
		UserID:        "default",
		Name:          fmt.Sprintf("%s导游助手", req.Destination),
		Description:   fmt.Sprintf("%s智能导游", req.Destination),
		Destination:   req.Destination,
		Theme:         req.Theme,
		Status:        "ready",
		DocumentCount: 0,
		Language:      "zh",
		Tags:          req.Languages,
		CreatedAt:     now,
		UpdatedAt:     now,
		UsageCount:    0,
		Rating:        0,
	}

	// Save agent to repository
	if err := s.agentRepo.SaveAgent(ctx, ag); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save agent: %v", err), http.StatusInternalServerError)
		return
	}

	// Create and save task record
	taskID := fmt.Sprintf("task-%s", agentID)
	task := &agent.AgentTask{
		ID:              taskID,
		AgentID:         agentID,
		UserID:          "default",
		Status:          "completed",
		Goal:            fmt.Sprintf("创建%s导游助手", req.Destination),
		DurationSeconds: 0,
		TotalTokens:     0,
		ExplorationLog:  []agent.ExplorationStep{},
		Result: map[string]any{
			"document_count":  0,
			"destination":     req.Destination,
			"theme":           req.Theme,
			"covered_topics":  map[string]int{},
			"missing_topics":  []string{},
			"collection_id":   agentID,
		},
		CreatedAt:   now,
		CompletedAt: &now,
	}

	if err := s.agentRepo.SaveTask(ctx, task); err != nil {
		log.Printf("[CreateAgent] Warning: Failed to save task: %v", err)
		// Don't fail the request, just log the warning
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent": map[string]interface{}{
			"id":             ag.ID,
			"user_id":        ag.UserID,
			"name":           ag.Name,
			"description":    ag.Description,
			"destination":    ag.Destination,
			"theme":          ag.Theme,
			"status":         ag.Status,
			"document_count": ag.DocumentCount,
			"language":       ag.Language,
			"tags":           ag.Tags,
			"created_at":     ag.CreatedAt,
			"updated_at":     ag.UpdatedAt,
			"usage_count":    ag.UsageCount,
			"rating":         ag.Rating,
		},
		"agent_id": agentID,
		"message":  "目的地 Agent 创建成功",
		"task_id":  taskID,
	})
}

func (s *Server) getAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	ctx := r.Context()
	ag, err := s.agentRepo.GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             ag.ID,
		"user_id":        ag.UserID,
		"name":           ag.Name,
		"description":    ag.Description,
		"destination":    ag.Destination,
		"theme":          ag.Theme,
		"status":         ag.Status,
		"document_count": ag.DocumentCount,
		"language":       ag.Language,
		"tags":           ag.Tags,
		"created_at":     ag.CreatedAt,
		"updated_at":     ag.UpdatedAt,
		"usage_count":    ag.UsageCount,
		"rating":         ag.Rating,
		"task_id":        fmt.Sprintf("task-%s", agentID),
	})
}

func (s *Server) deleteAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	ctx := r.Context()

	// Check if agent exists
	_, err := s.agentRepo.GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	if err := s.agentRepo.DeleteAgent(ctx, agentID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete agent: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Agent deleted successfully",
	})
}

// getAgentAttractions returns attractions for a specific agent
func (s *Server) getAgentAttractions(ctx context.Context, w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get agent from repository
	ag, err := s.agentRepo.GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent not found: %v", err), http.StatusNotFound)
		return
	}

	// Return default attractions based on destination
	// In a full implementation, these would come from the RAG knowledge base
	attractions := []map[string]interface{}{
		{
			"id":          agentID + "-1",
			"name":        ag.Destination + "热门景点",
			"category":    "景点",
			"description": ag.Destination + "的主要旅游景点",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"attractions": attractions,
		"total":       len(attractions),
	})
}

// getAgentTask returns task status for agent creation
func (s *Server) getAgentTask(ctx context.Context, w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get agent to check status
	ag, err := s.agentRepo.GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent not found: %v", err), http.StatusNotFound)
		return
	}

	// Return task-like status with correct task_id format
	taskId := fmt.Sprintf("task-%s", agentID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":              taskId,
		"task_id":         taskId,
		"agent_id":        agentID,
		"status":          "completed",
		"goal":            fmt.Sprintf("创建%s导游助手", ag.Destination),
		"duration_seconds": 0,
		"total_tokens":    0,
		"exploration_log": []interface{}{},
		"created_at":      ag.CreatedAt,
		"completed_at":    ag.UpdatedAt,
		"progress": map[string]interface{}{
			"current":  100,
			"total":    100,
			"message":  "Agent ready",
		},
	})
}

// listAgentTasks returns all tasks for an agent
func (s *Server) listAgentTasks(ctx context.Context, w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return mock task list for now
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": []map[string]interface{}{
			{
				"id":     agentID + "-research",
				"type":   "research",
				"status": "completed",
			},
		},
	})
}

func (s *Server) chatWithAgent(ctx context.Context, w http.ResponseWriter, r *http.Request, agentID string) {
	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Use main agent for chat
	response, err := s.mainAgent.Chat(ctx, req.Message)
	if err != nil {
		http.Error(w, fmt.Sprintf("Chat failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": map[string]interface{}{
			"id":         generateID(),
			"role":       "assistant",
			"content":    response,
			"created_at": time.Now(),
		},
	})
}

func (s *Server) chatStreamWithAgent(ctx context.Context, w http.ResponseWriter, r *http.Request, agentID string) {
	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Get agent info to retrieve destination and user_id
	var destination string
	var userID string
	if s.agentRepo != nil {
		ag, err := s.agentRepo.GetAgent(ctx, agentID)
		if err == nil && ag != nil {
			destination = ag.Destination
			userID = ag.UserID
			log.Printf("[Chat] Agent %s destination: %s, user: %s", agentID, destination, userID)
		}
	}

	// Get user-scoped memory for cross-session preferences
	// Use default user if agent's user_id is empty
	effectiveUserID := userID
	if effectiveUserID == "" {
		effectiveUserID = "default"
		log.Printf("[Chat] Agent has no user_id, using default user")
	}
	userMem := s.getUserMemory(effectiveUserID)
	if userMem != nil {
		log.Printf("[Chat] Loaded user memory for user %s", effectiveUserID)
	}

	// Get stream from main agent with destination context
	// Use empty history to avoid cross-contamination between different agents
	// Each agent should have its own conversation context
	// Pass user memory for preferences
	outputCh, errCh, _ := s.mainAgent.ChatStreamWithDestinationAndHistory(ctx, req.Message, destination, []llm.Message{}, userMem)

	var fullResponse strings.Builder
	streamDone := false

	for !streamDone {
		select {
		case chunk, ok := <-outputCh:
			if !ok {
				streamDone = true
				break
			}
			fullResponse.WriteString(chunk)
			// Send SSE event with JSON encoding for proper newline handling
			jsonData, _ := json.Marshal(map[string]string{"content": chunk})
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		case err, ok := <-errCh:
			if !ok {
				// Error channel closed, continue reading chunks
				continue
			}
			if err != nil {
				fmt.Fprintf(w, "data: [ERROR] %v\n\n", err)
				flusher.Flush()
				streamDone = true
			}
		case <-ctx.Done():
			streamDone = true
		}
	}

	// Send done event
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Save user-scoped memory (preferences) after chat completes
	if userMem != nil && effectiveUserID != "" {
		s.saveUserMemory(effectiveUserID, userMem)
		log.Printf("[Chat] Saved user memory for user %s", effectiveUserID)
	}
}

// handleTaskByID handles task status and streaming
func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	// Extract task ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	if path == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	// Check if it's a sub-resource like /stream
	parts := strings.SplitN(path, "/", 2)
	taskID := parts[0]
	subResource := ""
	if len(parts) == 2 {
		subResource = parts[1]
	}

	ctx := r.Context()

	switch subResource {
	case "stream":
		s.streamTaskProgress(ctx, w, r, taskID)
	case "":
		// Get task details
		s.getTaskDetails(ctx, w, r, taskID)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// TaskProgress represents task progress information
type TaskProgress struct {
	TaskID      string                   `json:"task_id"`
	Status      string                   `json:"status"`
	Progress    int                      `json:"progress"`
	Stage       string                   `json:"stage"`
	Message     string                   `json:"message"`
	Step        *agent.ExplorationStep   `json:"step,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	Duration    int64                    `json:"duration_seconds"`
	TotalTokens int                      `json:"total_tokens"`
}

// getTaskDetails returns task status
func (s *Server) getTaskDetails(ctx context.Context, w http.ResponseWriter, r *http.Request, taskID string) {
	// Try to find agent by task ID (task ID might be agent ID)
	var agentID string
	if s.agentRepo != nil {
		ag, err := s.agentRepo.GetAgent(ctx, taskID)
		if err == nil && ag != nil {
			agentID = ag.ID
		}
	}

	// If no agent found, use task ID as agent ID
	if agentID == "" {
		agentID = taskID
	}

	// Try to get task from repository
	var task *agent.AgentTask
	var err error
	if s.agentRepo != nil {
		task, err = s.agentRepo.GetTask(ctx, taskID)
		if err != nil {
			log.Printf("[GetTaskDetails] Task not found in agentRepo: %v", err)
			task = nil
		}
	}

	// Fallback to memory task repo if available
	if task == nil && taskRepo != nil {
		task, err = taskRepo.GetTask(ctx, taskID)
		if err != nil {
			log.Printf("[GetTaskDetails] Task not found in taskRepo: %v", err)
		}
	}

	// If task found, return it
	if task != nil {
		updatedAt := task.CreatedAt
		if task.CompletedAt != nil {
			updatedAt = *task.CompletedAt
		}
		response := map[string]interface{}{
			"task_id":          task.ID,
			"agent_id":         task.AgentID,
			"status":           task.Status,
			"goal":             task.Goal,
			"duration_seconds": task.DurationSeconds,
			"total_tokens":      task.TotalTokens,
			"created_at":       task.CreatedAt,
			"updated_at":       updatedAt,
			"exploration_log":   task.ExplorationLog,
			"result":           task.Result,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Fallback for old agents without task records
	progress := &TaskProgress{
		TaskID:    taskID,
		Status:    "completed",
		Progress:  100,
		Stage:     "complete",
		Message:   "任务完成",
		CreatedAt: time.Now().Add(-90 * time.Second),
		UpdatedAt: time.Now(),
		Duration:  90,
	}

	// Return response with agent_id for frontend compatibility
	response := map[string]interface{}{
		"task_id":          progress.TaskID,
		"agent_id":         agentID,
		"status":           progress.Status,
		"progress":         progress.Progress,
		"stage":            progress.Stage,
		"message":          progress.Message,
		"duration_seconds": progress.Duration,
		"total_tokens":     progress.TotalTokens,
		"created_at":       progress.CreatedAt,
		"updated_at":       progress.UpdatedAt,
		"exploration_log":  []interface{}{},
		"result": map[string]interface{}{
			"document_count":  100,
			"covered_topics":  map[string]int{"attractions": 30, "food": 25, "culture": 25, "transport": 20},
			"missing_topics":  []string{},
			"collection_id":   taskID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// streamTaskProgress streams task progress via SSE
func (s *Server) streamTaskProgress(ctx context.Context, w http.ResponseWriter, r *http.Request, taskID string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Simulate progress updates
	stages := []struct {
		stage   string
		message string
		delay   time.Duration
	}{
		{"research", "正在搜索目的地信息...", 2 * time.Second},
		{"research", "正在分析搜索结果...", 3 * time.Second},
		{"research", "正在提取旅游信息...", 2 * time.Second},
		{"curate", "正在整理和分类信息...", 3 * time.Second},
		{"curate", "正在验证信息质量...", 2 * time.Second},
		{"index", "正在构建知识库索引...", 3 * time.Second},
		{"index", "正在生成向量嵌入...", 2 * time.Second},
		{"complete", "任务完成！", 1 * time.Second},
	}

	for i, stage := range stages {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress := (i + 1) * 100 / len(stages)
		data := map[string]interface{}{
			"task_id":  taskID,
			"status":   "running",
			"progress": progress,
			"stage":    stage.stage,
			"message":  stage.message,
		}

		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", jsonData)
		flusher.Flush()

		time.Sleep(stage.delay)
	}

	// Send completion event
	completeData := map[string]interface{}{
		"task_id":  taskID,
		"status":   "completed",
		"progress": 100,
		"message":  "目的地 Agent 创建成功！",
		"agent_id": strings.TrimPrefix(taskID, "task-"),
	}
	jsonData, _ := json.Marshal(completeData)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", jsonData)
	flusher.Flush()
}

// Session API handlers

// handleSessions handles GET (list) and POST (create) for sessions
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		s.listSessions(ctx, w, r)
	case http.MethodPost:
		s.createSession(ctx, w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionByID handles operations on a specific session
func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
	if path == "" || strings.Contains(path, "/") {
		// Check if it's a sub-resource like /messages or /chat
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			sessionID := parts[0]
			subResource := parts[1]
			switch subResource {
			case "messages":
				s.getSessionMessages(r.Context(), w, r, sessionID)
			case "chat":
				s.chatSession(r.Context(), w, r, sessionID)
			case "chat/stream":
				s.chatSessionStream(r.Context(), w, r, sessionID)
			default:
				http.Error(w, "Not found", http.StatusNotFound)
			}
			return
		}
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}
	sessionID := path

	switch r.Method {
	case http.MethodGet:
		s.getSession(r.Context(), w, r, sessionID)
	case http.MethodPatch:
		s.updateSession(r.Context(), w, r, sessionID)
	case http.MethodDelete:
		s.deleteSession(r.Context(), w, r, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listSessions(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Support agent_id filtering
	agentID := r.URL.Query().Get("agent_id")

	opts := session.ListOptions{
		Limit:      limit,
		Descending: true, // Show most recent sessions first
	}
	if agentID != "" {
		opts.AgentID = agentID
	}

	result, err := s.sessionStore.List(ctx, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list sessions: %v", err), http.StatusInternalServerError)
		return
	}

	// Filter out user-type sessions (used only for storing preferences)
	filtered := session.ListResult{
		Sessions: make([]*session.Session, 0),
		Grouped:  make(map[string][]*session.Session),
	}
	for _, sess := range result.Sessions {
		if sess.AgentType() != "user" {
			filtered.Sessions = append(filtered.Sessions, sess)
		}
	}
	for key, sessions := range result.Grouped {
		for _, sess := range sessions {
			if sess.AgentType() != "user" {
				filtered.Grouped[key] = append(filtered.Grouped[key], sess)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

func (s *Server) createSession(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentType     string `json:"agent_type"`
		AgentID       string `json:"agent_id,omitempty"`
		DestinationID string `json:"destination_id,omitempty"`
		Title         string `json:"title,omitempty"`
		UserID        string `json:"user_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentType == "" {
		req.AgentType = "main"
	}

	sess := session.New(generateID())
	sess.SetAgentType(req.AgentType)
	if req.Title != "" {
		sess.SetTitle(req.Title)
	}
	if req.UserID != "" {
		sess.SetMetadata("user_id", req.UserID)
	}
	if req.AgentID != "" {
		sess.SetAgentID(req.AgentID)
		sess.SetMetadata("agent_id", req.AgentID)
	}
	if req.DestinationID != "" {
		sess.SetMetadata("destination_id", req.DestinationID)
	}

	if err := s.sessionStore.Create(ctx, sess); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sess.ToSnapshot())
}

// createSessionForAgent creates a new session for a specific agent (guide agent)
func (s *Server) createSessionForAgent(ctx context.Context, w http.ResponseWriter, r *http.Request, agentID string) {
	var req struct {
		Title string `json:"title,omitempty"`
	}

	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	// Get agent info to determine destination
	ag, err := s.agentRepo.GetAgent(ctx, agentID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent not found: %v", err), http.StatusNotFound)
		return
	}

	// Create session with agent_type="guide" and agent_id set
	sess := session.New(generateID())
	sess.SetAgentType("guide")
	sess.SetAgentID(agentID)
	sess.SetMetadata("agent_id", agentID)

	if req.Title != "" {
		sess.SetTitle(req.Title)
	} else {
		// Default title based on agent destination
		sess.SetTitle(fmt.Sprintf("%s导游对话", ag.Destination))
	}

	if err := s.sessionStore.Create(ctx, sess); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sess.ToSnapshot())
}

func (s *Server) getSession(ctx context.Context, w http.ResponseWriter, r *http.Request, id string) {
	sess, err := s.sessionStore.Get(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess.ToSnapshot())
}

func (s *Server) updateSession(ctx context.Context, w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Title string        `json:"title,omitempty"`
		State session.State `json:"state,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	sess, err := s.sessionStore.Get(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	if req.Title != "" {
		sess.SetTitle(req.Title)
	}
	if req.State != "" {
		sess.SetState(req.State)
	}

	if err := s.sessionStore.Update(ctx, sess); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess.ToSnapshot())
}

func (s *Server) deleteSession(ctx context.Context, w http.ResponseWriter, r *http.Request, id string) {
	if err := s.sessionStore.Delete(ctx, id); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) getSessionMessages(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	// Verify session exists
	_, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	// Load messages from memory storage
	var messages []interface{}
	if s.memoryStorage != nil {
		snapshot, err := s.memoryStorage.Load(ctx, sessionID)
		if err == nil && snapshot != nil {
			// Filter items by type "message" and convert to response format
			for _, item := range snapshot.ShortTerm {
				if item.Type == "message" {
					role := "user"
					if r, ok := item.Metadata["role"].(string); ok {
						role = r
					}
					messages = append(messages, map[string]interface{}{
						"id":        item.ID,
						"role":      role,
						"content":   item.Content,
						"created_at": item.Timestamp,
					})
				}
			}
		}
	}

	if messages == nil {
		messages = []interface{}{}
	}

	log.Printf("[GetMessages] Session %s: returning %d messages", sessionID, len(messages))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
		"has_more": false,
	})
}

func (s *Server) chatSession(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Verify session exists
	sess, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	// Touch session
	sess.Touch()
	s.sessionStore.Update(ctx, sess)

	// Get destination from session's associated agent
	var destination string
	if agentID, ok := sess.GetMetadata("agent_id"); ok {
		if agentIDStr, ok := agentID.(string); ok && agentIDStr != "" && s.agentRepo != nil {
			ag, err := s.agentRepo.GetAgent(ctx, agentIDStr)
			if err == nil && ag != nil {
				destination = ag.Destination
			}
		}
	}

	// Get user-scoped memory for cross-session preferences
	var userMem *memory.PersistentMemory
	userID := ""
	if uid, ok := sess.GetMetadata("user_id"); ok {
		if userIDStr, ok := uid.(string); ok && userIDStr != "" {
			userID = userIDStr
		}
	}
	// If no user_id, use default user for preferences
	if userID == "" {
		userID = "demo-user-001"
	}
	userMem = s.getUserMemory(userID)
	log.Printf("[ChatSession] Non-streaming chat using user memory for user %s", userID)

	// Load conversation history from session's memory storage
	var conversationHistory []llm.Message
	if s.memoryStorage != nil {
		snapshot, err := s.memoryStorage.Load(ctx, sessionID)
		if err == nil && snapshot != nil {
			for _, item := range snapshot.ShortTerm {
				if item.Type == "message" {
					role := "user"
					if r, ok := item.Metadata["role"].(string); ok {
						role = r
					}
					conversationHistory = append(conversationHistory, llm.Message{
						Role:    role,
						Content: item.Content,
					})
				}
			}
		}
	}

	// Use ChatWithDestinationAndHistory for proper memory handling
	var response string
	if userMem != nil {
		// Always use ChatWithDestinationAndHistory when user memory is available
		// to ensure preferences are loaded
		response, err = s.mainAgent.ChatWithDestinationAndHistory(ctx, req.Message, destination, conversationHistory, userMem)
	} else {
		// Fallback to basic chat when no user memory
		response, err = s.mainAgent.Chat(ctx, req.Message)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Chat failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Save messages to memory storage
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer saveCancel()

	if s.memoryStorage != nil {
		snapshot, err := s.memoryStorage.Load(saveCtx, sessionID)
		if err != nil {
			snapshot = &memory.Snapshot{
				SessionID: sessionID,
				ShortTerm: []memory.Item{},
				LongTerm:  []memory.Item{},
				CreatedAt: time.Now(),
			}
		}
		now := time.Now()
		snapshot.ShortTerm = append(snapshot.ShortTerm, memory.Item{
			ID:        generateID(),
			Type:      "message",
			Content:   req.Message,
			Metadata:  map[string]any{"role": "user"},
			Timestamp: now,
		})
		snapshot.ShortTerm = append(snapshot.ShortTerm, memory.Item{
			ID:        generateID(),
			Type:      "message",
			Content:   response,
			Metadata:  map[string]any{"role": "assistant"},
			Timestamp: now,
		})
		snapshot.UpdatedAt = now
		s.memoryStorage.Save(saveCtx, sessionID, snapshot)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": map[string]interface{}{
			"id":         generateID(),
			"role":       "assistant",
			"content":    response,
			"created_at": time.Now(),
		},
	})
}

// chatSessionStream handles streaming chat for a session
func (s *Server) chatSessionStream(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Verify session exists
	sess, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	// Touch session
	sess.Touch()
	s.sessionStore.Update(ctx, sess)

	// Get destination from session's associated agent
	var destination string
	if agentID, ok := sess.GetMetadata("agent_id"); ok {
		if agentIDStr, ok := agentID.(string); ok && agentIDStr != "" && s.agentRepo != nil {
			ag, err := s.agentRepo.GetAgent(ctx, agentIDStr)
			if err == nil && ag != nil {
				destination = ag.Destination
				log.Printf("[ChatSession] Session %s using destination: %s (agent: %s)", sessionID, destination, agentIDStr)
			}
		}
	}

	// Get user-scoped memory for cross-session preferences
	// Use default user if no user_id is set in session
	var userMem *memory.PersistentMemory
	userID := ""
	if uid, ok := sess.GetMetadata("user_id"); ok {
		if userIDStr, ok := uid.(string); ok && userIDStr != "" {
			userID = userIDStr
		}
	}
	// If no user_id, use default user for preferences
	if userID == "" {
		userID = "default"
		log.Printf("[ChatSession] No user_id in session, using default user: %s", userID)
	}
	userMem = s.getUserMemory(userID)
	log.Printf("[ChatSession] Loaded user memory for user %s", userID)

	// Load conversation history from session's memory storage
	var conversationHistory []llm.Message
	if s.memoryStorage != nil {
		snapshot, err := s.memoryStorage.Load(ctx, sessionID)
		if err == nil && snapshot != nil {
			// Convert memory items to llm.Message format
			for _, item := range snapshot.ShortTerm {
				if item.Type == "message" {
					role := "user"
					if r, ok := item.Metadata["role"].(string); ok {
						role = r
					}
					conversationHistory = append(conversationHistory, llm.Message{
						Role:    role,
						Content: item.Content,
					})
				}
			}
			log.Printf("[ChatSession] Loaded %d messages from session memory", len(conversationHistory))
		}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Get stream from main agent with destination context and session history
	// Pass user memory for preferences loading/saving
	var outputCh <-chan string
	var errCh <-chan error
	if userMem != nil {
		// Always use ChatStreamWithDestinationAndHistory when user memory is available
		outputCh, errCh, _ = s.mainAgent.ChatStreamWithDestinationAndHistory(ctx, req.Message, destination, conversationHistory, userMem)
	} else {
		// Fallback to basic chat stream when no user memory
		outputCh, errCh = s.mainAgent.ChatStream(ctx, req.Message)
	}

	var fullResponse strings.Builder
	streamDone := false

	for !streamDone {
		select {
		case chunk, ok := <-outputCh:
			if !ok {
				streamDone = true
				break
			}
			fullResponse.WriteString(chunk)
			// Use JSON encoding to properly handle newlines and special characters
			jsonData, _ := json.Marshal(map[string]string{"content": chunk})
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		case err, ok := <-errCh:
			if !ok {
				continue
			}
			if err != nil {
				fmt.Fprintf(w, "data: [ERROR] %v\n\n", err)
				flusher.Flush()
				streamDone = true
			}
		case <-ctx.Done():
			streamDone = true
		}
	}

	// Save messages to memory storage
	// Use a new context with timeout to avoid context canceled issues
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer saveCancel()

	if s.memoryStorage != nil && fullResponse.Len() > 0 {
		// Load existing snapshot or create new one
		snapshot, err := s.memoryStorage.Load(saveCtx, sessionID)
		if err != nil {
			snapshot = &memory.Snapshot{
				SessionID: sessionID,
				ShortTerm: []memory.Item{},
				LongTerm:  []memory.Item{},
				CreatedAt: time.Now(),
			}
		}

		// Add user message
		userItem := memory.Item{
			ID:        generateID(),
			Type:      "message",
			Content:   req.Message,
			Metadata:  map[string]any{"role": "user"},
			Timestamp: time.Now(),
		}
		snapshot.ShortTerm = append(snapshot.ShortTerm, userItem)

		// Add assistant message
		assistantItem := memory.Item{
			ID:        generateID(),
			Type:      "message",
			Content:   fullResponse.String(),
			Metadata:  map[string]any{"role": "assistant"},
			Timestamp: time.Now(),
		}
		snapshot.ShortTerm = append(snapshot.ShortTerm, assistantItem)

		// Save snapshot
		if err := s.memoryStorage.Save(saveCtx, sessionID, snapshot); err != nil {
			log.Printf("[Chat] Failed to save messages: %v", err)
		} else {
			log.Printf("[Chat] Saved %d messages to session %s memory", len(snapshot.ShortTerm), sessionID)
		}

		// Update session message count
		sess.IncrementMessageCount()
		s.sessionStore.Update(saveCtx, sess)
	}

	// Save user-scoped memory (preferences) after chat completes
	// Wait briefly for async preference extraction to complete
	if userMem != nil {
		if userID, ok := sess.GetMetadata("user_id"); ok {
			if userIDStr, ok := userID.(string); ok && userIDStr != "" {
				// Wait for async preference extraction (runs in MainAgent goroutine)
				// This is a temporary solution - ideally we'd use a channel or callback
				time.Sleep(3 * time.Second)
				s.saveUserMemory(userIDStr, userMem)
				log.Printf("[ChatSession] Saved user memory for user %s", userIDStr)
			}
		}
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := Config{
		HTTPPort:       getEnvInt("HTTP_PORT", 8080),
		LLMProvider:    getEnv("LLM_PROVIDER", "mock"),
		LLMGRPCAddr:    getEnv("LLM_GRPC_ADDR", "localhost:50051"),
		GLMAPIKey:      getEnv("GLM_API_KEY", getEnv("LLM_API_KEY", "")),
		GLMModel:       getEnv("GLM_MODEL", "glm-4-flash"),
		DeepSeekAPIKey: getEnv("DEEPSEEK_API_KEY", getEnv("LLM_API_KEY", "")),
		DeepSeekModel:  getEnv("DEEPSEEK_MODEL", "deepseek-chat"),
		TavilyAPIKey:   getEnv("TAVILY_API_KEY", ""),
		ProxyURL:       getEnv("PROXY_URL", ""),
		// PostgreSQL config
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnvInt("POSTGRES_PORT", 5432),
		PostgresUser:     getEnv("POSTGRES_USER", "postgres"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "postgres"),
		PostgresDatabase: getEnv("POSTGRES_DB", "uta_travel"),
		PostgresSSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		// Feature flags - RAG disabled by default
		EnableRAG: getEnvBool("ENABLE_RAG", false),
	}

	server := NewServer(config)
	log.Fatal(server.Start())
}