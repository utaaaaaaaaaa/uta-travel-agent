package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider defines the interface for LLM operations
type Provider interface {
	Complete(ctx context.Context, messages []Message, opts ...Option) (*Response, error)
	CompleteWithSystem(ctx context.Context, system string, messages []Message, opts ...Option) (*Response, error)
	RAGQuery(ctx context.Context, query, context string, opts ...Option) (*Response, error)
	Stream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, <-chan error)
}

// Message represents a chat message
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// Response from LLM
type Response struct {
	Content      string
	InputTokens  int
	OutputTokens int
	Model        string
	StopReason   string
}

// StreamChunk for streaming responses
type StreamChunk struct {
	Content string
	Done    bool
}

// Option for customizing LLM calls
type Option func(*Options)

// Options for LLM calls
type Options struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

// WithModel sets the model
func WithModel(model string) Option {
	return func(o *Options) {
		o.Model = model
	}
}

// WithTemperature sets the temperature
func WithTemperature(temp float64) Option {
	return func(o *Options) {
		o.Temperature = temp
	}
}

// WithMaxTokens sets max tokens
func WithMaxTokens(tokens int) Option {
	return func(o *Options) {
		o.MaxTokens = tokens
	}
}

// MockProvider implements Provider for testing
type MockProvider struct {
	Response string
}

// NewMockProvider creates a mock provider
func NewMockProvider(response string) *MockProvider {
	return &MockProvider{Response: response}
}

func (p *MockProvider) Complete(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	return &Response{
		Content:      p.Response,
		InputTokens:  100,
		OutputTokens: 50,
		Model:        "mock",
		StopReason:   "end_turn",
	}, nil
}

func (p *MockProvider) CompleteWithSystem(ctx context.Context, system string, messages []Message, opts ...Option) (*Response, error) {
	return p.Complete(ctx, messages, opts...)
}

func (p *MockProvider) RAGQuery(ctx context.Context, query, context string, opts ...Option) (*Response, error) {
	return &Response{
		Content:      fmt.Sprintf("关于「%s」的回答：\n\n基于上下文：%s\n\n%s", query, context, p.Response),
		InputTokens:  150,
		OutputTokens: 100,
		Model:        "mock",
		StopReason:   "end_turn",
	}, nil
}

func (p *MockProvider) Stream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, <-chan error) {
	chunkCh := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		words := strings.Fields(p.Response)
		for _, word := range words {
			select {
			case chunkCh <- StreamChunk{Content: word + " ", Done: false}:
			case <-ctx.Done():
				return
			}
		}
		chunkCh <- StreamChunk{Content: "", Done: true}
	}()

	return chunkCh, errCh
}

// GLMProvider implements Provider using GLM API (OpenAI-compatible)
type GLMProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// GLMConfig for GLM provider
type GLMConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// NewGLMProvider creates a new GLM provider
func NewGLMProvider(config GLMConfig) *GLMProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://open.bigmodel.cn/api/paas/v4"
	}
	if config.Model == "" {
		config.Model = "glm-4-flash"
	}

	return &GLMProvider{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// DeepSeekConfig for DeepSeek provider
type DeepSeekConfig struct {
	APIKey string
	Model  string
}

// NewDeepSeekProvider creates a new DeepSeek provider
func NewDeepSeekProvider(config DeepSeekConfig) *GLMProvider {
	if config.Model == "" {
		config.Model = "deepseek-chat"
	}

	return &GLMProvider{
		apiKey:  config.APIKey,
		baseURL: "https://api.deepseek.com/v1",
		model:   config.Model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// glmRequest represents OpenAI-compatible request
type glmRequest struct {
	Model       string          `json:"model"`
	Messages    []glmMessage    `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// glmMessage represents a message
type glmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// glmResponse represents OpenAI-compatible response
type glmResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int        `json:"index"`
		Message      glmMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
		Delta        *struct {
			Content string `json:"content"`
		} `json:"delta,omitempty"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Complete generates a completion
func (p *GLMProvider) Complete(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	return p.CompleteWithSystem(ctx, "", messages, opts...)
}

// CompleteWithSystem generates a completion with system prompt
func (p *GLMProvider) CompleteWithSystem(ctx context.Context, system string, messages []Message, opts ...Option) (*Response, error) {
	options := &Options{
		Temperature: 0.7,
		MaxTokens:   4096,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Build request
	glmMsgs := make([]glmMessage, 0)
	if system != "" {
		glmMsgs = append(glmMsgs, glmMessage{Role: "system", Content: system})
	}
	for _, m := range messages {
		glmMsgs = append(glmMsgs, glmMessage{Role: m.Role, Content: m.Content})
	}

	model := p.model
	if options.Model != "" {
		model = options.Model
	}

	req := glmRequest{
		Model:       model,
		Messages:    glmMsgs,
		Temperature: options.Temperature,
		MaxTokens:   options.MaxTokens,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var glmResp glmResponse
	if err := json.Unmarshal(respBody, &glmResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if glmResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", glmResp.Error.Message)
	}

	if len(glmResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := glmResp.Choices[0]
	return &Response{
		Content:      choice.Message.Content,
		InputTokens:  glmResp.Usage.PromptTokens,
		OutputTokens: glmResp.Usage.CompletionTokens,
		Model:        glmResp.Model,
		StopReason:   choice.FinishReason,
	}, nil
}

// RAGQuery executes a RAG-enhanced query
func (p *GLMProvider) RAGQuery(ctx context.Context, query, context string, opts ...Option) (*Response, error) {
	system := `你是一位专业的旅行导游助手。你的任务是根据提供的目的地知识库信息，为用户提供准确、有用的旅行建议。

规则:
1. 只使用提供的上下文信息回答问题
2. 如果信息不足，坦诚告知用户
3. 保持回答简洁但有价值
4. 适当添加文化背景和有趣的事实
5. 使用友好的语气，像一位本地导游一样交流`

	userMsg := fmt.Sprintf(`用户问题: %s

参考信息:
%s

请根据以上信息回答用户的问题。如果参考信息中没有相关内容，请坦诚告知。`, query, context)

	return p.CompleteWithSystem(ctx, system, []Message{{Role: "user", Content: userMsg}}, opts...)
}

// Stream generates a streaming completion
func (p *GLMProvider) Stream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, <-chan error) {
	chunkCh := make(chan StreamChunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		options := &Options{
			Temperature: 0.7,
			MaxTokens:   4096,
		}
		for _, opt := range opts {
			opt(options)
		}

		glmMsgs := make([]glmMessage, len(messages))
		for i, m := range messages {
			glmMsgs[i] = glmMessage{Role: m.Role, Content: m.Content}
		}

		model := p.model
		if options.Model != "" {
			model = options.Model
		}

		req := glmRequest{
			Model:       model,
			Messages:    glmMsgs,
			Temperature: options.Temperature,
			MaxTokens:   options.MaxTokens,
			Stream:      true,
		}

		body, err := json.Marshal(req)
		if err != nil {
			errCh <- fmt.Errorf("marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			errCh <- fmt.Errorf("create request: %w", err)
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := p.client.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("send request: %w", err)
			return
		}
		defer resp.Body.Close()

		reader := resp.Body
		buf := make([]byte, 4096)

		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					chunkCh <- StreamChunk{Content: "", Done: true}
				} else {
					errCh <- err
				}
				return
			}

			data := string(buf[:n])
			lines := strings.Split(data, "\n")

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				dataStr := strings.TrimPrefix(line, "data: ")
				if dataStr == "[DONE]" {
					chunkCh <- StreamChunk{Content: "", Done: true}
					return
				}

				var glmResp glmResponse
				if err := json.Unmarshal([]byte(dataStr), &glmResp); err != nil {
					continue
				}

				if glmResp.Error != nil {
					errCh <- fmt.Errorf("API error: %s", glmResp.Error.Message)
					return
				}

				if len(glmResp.Choices) > 0 && glmResp.Choices[0].Delta != nil {
					content := glmResp.Choices[0].Delta.Content
					if content != "" {
						select {
						case chunkCh <- StreamChunk{Content: content, Done: false}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	return chunkCh, errCh
}