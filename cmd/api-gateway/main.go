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

	"github.com/utaaa/uta-travel-agent/internal/agent"
	"github.com/utaaa/uta-travel-agent/internal/llm"
	"github.com/utaaa/uta-travel-agent/internal/session"
)

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
	mainAgent     *agent.MainAgent
	httpPort      int
	sessionStore  *MemorySessionStore
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

// NewServer creates a new API server
func NewServer(config Config) *Server {
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

	// Create main agent
	mainAgent := agent.NewMainAgent(agent.MainAgentConfig{
		ID:          "main-agent-001",
		LLMProvider: llmProvider,
		Template: &agent.AgentTemplate{
			Name:         "UTA Main Agent",
			Description:  "Main orchestrator agent for UTA Travel",
			SystemPrompt: "你是 UTA Travel 的智能旅游助手。",
			Spec: agent.TemplateSpec{
				Role: "你是 UTA Travel 的智能旅游助手。你的任务是为用户提供专业、友好的旅游建议和信息。",
			},
		},
	})

	// Create mock tool registry
	toolRegistry := NewMockToolRegistry()

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
		mainAgent:    mainAgent,
		httpPort:     config.HTTPPort,
		sessionStore: NewMemorySessionStore(),
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
			fmt.Fprintf(w, "data: %s\n\n", chunk)
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

	// Create goal for the agent
	goal := fmt.Sprintf("创建 %s 的目的地 Agent", req.Destination)

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	result, err := s.mainAgent.Run(ctx, goal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate step count from result metadata
	stepCount := 0
	if result.Metadata != nil {
		if count, ok := result.Metadata["subagent_results"].(int); ok {
			stepCount = count
		}
	}
	if stepCount == 0 {
		stepCount = 3 // Default for creation workflow
	}

	response := CreateDestinationAgentResponse{
		AgentID:     result.AgentID,
		Destination: req.Destination,
		Status:      "created",
		Message:     fmt.Sprintf("目的地 Agent 创建成功，共执行 %d 个步骤，用时 %.1f 秒", stepCount, result.Duration.Seconds()),
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

	result, err := s.sessionStore.List(ctx, session.ListOptions{Limit: limit})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list sessions: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) createSession(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentType     string `json:"agent_type"`
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

	// Return empty messages for now (would load from memory store in full implementation)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": []interface{}{},
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

	// For now, use the main agent for chat
	// In a full implementation, this would route to the appropriate agent based on session type
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

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func main() {
	config := Config{
		HTTPPort:       getEnvInt("HTTP_PORT", 8080),
		LLMProvider:    getEnv("LLM_PROVIDER", "mock"),
		LLMGRPCAddr:    getEnv("LLM_GRPC_ADDR", "localhost:50051"),
		GLMAPIKey:      getEnv("GLM_API_KEY", ""),
		GLMModel:       getEnv("GLM_MODEL", "glm-4-flash"),
		DeepSeekAPIKey: getEnv("DEEPSEEK_API_KEY", ""),
		DeepSeekModel:  getEnv("DEEPSEEK_MODEL", "deepseek-chat"),
	}

	server := NewServer(config)
	log.Fatal(server.Start())
}