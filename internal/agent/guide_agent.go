// Package agent provides the GuideAgent for destination-specific guidance
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/utaaa/uta-travel-agent/internal/llm"
)

// GuideAgent provides destination-specific guidance with RAG support
type GuideAgent struct {
	*BaseSessionAgent

	mu sync.RWMutex

	// Destination info
	destination  string
	collectionID string

	// RAG service (optional)
	ragService RAGService

	// Destination-specific knowledge
	knowledge map[string]any
}

// RAGService defines the interface for RAG operations
type RAGService interface {
	Query(ctx context.Context, collectionID, query string, limit int) (*RAGResult, error)
}

// RAGResult represents the result of a RAG query
type RAGResult struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources"`
	Score   float64  `json:"score"`
}

// GuideAgentConfig for creating a guide agent
type GuideAgentConfig struct {
	ID           string
	Destination  string
	CollectionID string
	LLMProvider  llm.Provider
	RAGService   RAGService
	SystemPrompt string
	MaxContext   int
}

// NewGuideAgent creates a new guide agent
func NewGuideAgent(config GuideAgentConfig) *GuideAgent {
	systemPrompt := config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = fmt.Sprintf(defaultGuidePrompt, config.Destination)
	}

	baseAgent := NewBaseSessionAgent(SessionAgentConfig{
		ID:          config.ID,
		AgentType:   AgentTypeGuide,
		LLMProvider: config.LLMProvider,
		SystemPrompt: systemPrompt,
		MaxContext:  config.MaxContext,
	})

	return &GuideAgent{
		BaseSessionAgent: baseAgent,
		destination:      config.Destination,
		collectionID:     config.CollectionID,
		ragService:       config.RAGService,
		knowledge:        make(map[string]any),
	}
}

// Destination returns the destination name
func (a *GuideAgent) Destination() string {
	return a.destination
}

// CollectionID returns the RAG collection ID
func (a *GuideAgent) CollectionID() string {
	return a.collectionID
}

// SetKnowledge sets destination-specific knowledge
func (a *GuideAgent) SetKnowledge(knowledge map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.knowledge = knowledge
}

// GetKnowledge returns destination-specific knowledge
func (a *GuideAgent) GetKnowledge() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]any, len(a.knowledge))
	for k, v := range a.knowledge {
		result[k] = v
	}
	return result
}

// Guide provides guidance based on RAG knowledge
func (a *GuideAgent) Guide(ctx context.Context, query string) (string, error) {
	a.SetState(StateThinking)
	defer a.SetState(StateIdle)

	// Touch session
	a.Session().Touch()

	// Add user message
	a.AddMessage("user", query)

	// Build messages
	var messages []llm.Message

	// If RAG service is available, query for context
	if a.ragService != nil && a.collectionID != "" {
		ragResult, err := a.ragService.Query(ctx, a.collectionID, query, 5)
		if err == nil && ragResult != nil {
			// Add RAG context as system message
			contextMsg := fmt.Sprintf("参考知识:\n%s", ragResult.Answer)
			messages = append(messages, llm.Message{
				Role:    "system",
				Content: contextMsg,
			})

			// Remember this interaction
			a.Remember("last_rag_query", query)
			if len(ragResult.Sources) > 0 {
				a.Remember("last_sources", ragResult.Sources)
			}
		}
	}

	// Add conversation history
	messages = append(messages, a.BuildContext()...)

	// Add current query
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: query,
	})

	// Call LLM
	response, err := a.llmProvider.CompleteWithSystem(ctx, a.GetSystemPrompt(), messages)
	if err != nil {
		return "", fmt.Errorf("LLM call: %w", err)
	}

	// Add assistant message
	a.AddMessage("assistant", response.Content)

	// Auto-save session
	go func() {
		_, _ = a.SaveSession(context.Background())
	}()

	return response.Content, nil
}

// GuideStream provides streaming guidance
func (a *GuideAgent) GuideStream(ctx context.Context, query string) (<-chan string, <-chan error) {
	outputCh := make(chan string, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)

		a.SetState(StateThinking)
		defer a.SetState(StateIdle)

		// Touch session
		a.Session().Touch()

		// Add user message
		a.AddMessage("user", query)

		// Build messages
		var messages []llm.Message

		// Query RAG if available
		if a.ragService != nil && a.collectionID != "" {
			ragResult, err := a.ragService.Query(ctx, a.collectionID, query, 5)
			if err == nil && ragResult != nil {
				contextMsg := fmt.Sprintf("参考知识:\n%s", ragResult.Answer)
				messages = append(messages, llm.Message{
					Role:    "system",
					Content: contextMsg,
				})
			}
		}

		// Add conversation history
		messages = append(messages, a.BuildContext()...)

		// Stream from LLM
		chunkCh, streamErrCh := a.llmProvider.Stream(ctx, messages)

		var fullResponse strings.Builder

		for {
			select {
			case chunk, ok := <-chunkCh:
				if !ok {
					// Save full response
					a.AddMessage("assistant", fullResponse.String())
					go func() {
						_, _ = a.SaveSession(context.Background())
					}()
					return
				}
				if chunk.Content != "" {
					fullResponse.WriteString(chunk.Content)
					select {
					case outputCh <- chunk.Content:
					case <-ctx.Done():
						return
					}
				}
			case err := <-streamErrCh:
				if err != nil {
					errCh <- err
				}
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return outputCh, errCh
}

// Chat implements SessionAgent interface (delegates to Guide)
func (a *GuideAgent) Chat(ctx context.Context, message string) (string, error) {
	return a.Guide(ctx, message)
}

// Attractions returns attractions from knowledge
func (a *GuideAgent) Attractions() []map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if attractions, ok := a.knowledge["attractions"].([]map[string]any); ok {
		return attractions
	}
	return nil
}

// Foods returns food recommendations from knowledge
func (a *GuideAgent) Foods() []map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if foods, ok := a.knowledge["foods"].([]map[string]any); ok {
		return foods
	}
	return nil
}

// Tips returns travel tips from knowledge
func (a *GuideAgent) Tips() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if tips, ok := a.knowledge["tips"].([]string); ok {
		return tips
	}
	return nil
}

const defaultGuidePrompt = `你是 %s 的专业导游助手。

你的职责:
1. 为游客提供专业、友好的导游服务
2. 介绍景点的历史背景、文化意义和游览建议
3. 推荐当地美食、特色体验
4. 解答游客关于目的地的各种问题

回答要求:
- 使用生动的语言，像一位本地导游
- 提供实用的建议和有趣的故事
- 如果知道具体信息，给出准确的数据（开放时间、门票价格等）
- 如果不确定，坦诚告知并建议游客核实

请用热情、专业的态度为游客服务！`