package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientConfig holds configuration for gRPC clients
type ClientConfig struct {
	LLMHost       string
	LLMPort       int
	EmbeddingHost string
	EmbeddingPort int
	VisionHost    string
	VisionPort    int
	Timeout       time.Duration
}

// DefaultClientConfig returns default configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		LLMHost:       "localhost",
		LLMPort:       50061,
		EmbeddingHost: "localhost",
		EmbeddingPort: 50062,
		VisionHost:    "localhost",
		VisionPort:    50063,
		Timeout:       30 * time.Second,
	}
}

// ClientManager manages gRPC client connections
type ClientManager struct {
	config ClientConfig

	llmConn       *grpc.ClientConn
	embeddingConn *grpc.ClientConn
	visionConn    *grpc.ClientConn

	mu sync.RWMutex
}

// NewClientManager creates a new client manager
func NewClientManager(config ClientConfig) *ClientManager {
	return &ClientManager{
		config: config,
	}
}

// Connect establishes all connections
func (m *ClientManager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error

	// Connect to LLM service
	m.llmConn, err = m.dial(ctx, m.config.LLMHost, m.config.LLMPort)
	if err != nil {
		return fmt.Errorf("failed to connect to LLM service: %w", err)
	}

	// Connect to Embedding service
	m.embeddingConn, err = m.dial(ctx, m.config.EmbeddingHost, m.config.EmbeddingPort)
	if err != nil {
		m.llmConn.Close()
		return fmt.Errorf("failed to connect to Embedding service: %w", err)
	}

	// Connect to Vision service
	m.visionConn, err = m.dial(ctx, m.config.VisionHost, m.config.VisionPort)
	if err != nil {
		m.llmConn.Close()
		m.embeddingConn.Close()
		return fmt.Errorf("failed to connect to Vision service: %w", err)
	}

	return nil
}

// Close closes all connections
func (m *ClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	if m.llmConn != nil {
		if err := m.llmConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.embeddingConn != nil {
		if err := m.embeddingConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.visionConn != nil {
		if err := m.visionConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}

	return nil
}

func (m *ClientManager) dial(ctx context.Context, host string, port int) (*grpc.ClientConn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(m.config.Timeout),
	}

	conn, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	return conn, nil
}

// GetLLMConn returns the LLM service connection
func (m *ClientManager) GetLLMConn() *grpc.ClientConn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.llmConn
}

// GetEmbeddingConn returns the Embedding service connection
func (m *ClientManager) GetEmbeddingConn() *grpc.ClientConn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.embeddingConn
}

// GetVisionConn returns the Vision service connection
func (m *ClientManager) GetVisionConn() *grpc.ClientConn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.visionConn
}