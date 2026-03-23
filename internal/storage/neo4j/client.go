// Package neo4j provides Neo4j graph database client for knowledge graph storage
package neo4j

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Config holds Neo4j connection configuration
type Config struct {
	URI      string `json:"uri"`      // e.g., "bolt://localhost:7687"
	Username string `json:"username"` // default: "neo4j"
	Password string `json:"password"` // default: "uta-password"
	Database string `json:"database"` // default: "neo4j"
}

// DefaultConfig returns default Neo4j configuration
func DefaultConfig() Config {
	return Config{
		URI:      "bolt://localhost:7687",
		Username: "neo4j",
		Password: "uta-password",
		Database: "neo4j",
	}
}

// Client wraps Neo4j driver with connection management
type Client struct {
	config   Config
	driver   neo4j.DriverWithContext
	mu       sync.RWMutex
	healthy  bool
	lastPing time.Time
}

// NewClient creates a new Neo4j client
func NewClient(config Config) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(
		config.URI,
		neo4j.BasicAuth(config.Username, config.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	client := &Client{
		config:  config,
		driver:  driver,
		healthy: true,
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("failed to connect to Neo4j: %w", err)
	}

	log.Printf("[Neo4j] Connected to %s", config.URI)
	return client, nil
}

// Close closes the Neo4j driver connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.driver != nil {
		c.healthy = false
		return c.driver.Close(context.Background())
	}
	return nil
}

// Ping checks if the Neo4j server is reachable
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.driver == nil {
		return fmt.Errorf("driver not initialized")
	}

	err := c.driver.VerifyConnectivity(ctx)
	if err != nil {
		c.healthy = false
		return fmt.Errorf("connection verification failed: %w", err)
	}

	c.healthy = true
	c.lastPing = time.Now()
	return nil
}

// IsHealthy returns true if the client is healthy
func (c *Client) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthy
}

// GetSession returns a new Neo4j session
func (c *Client) GetSession(ctx context.Context, accessMode neo4j.AccessMode) (neo4j.SessionWithContext, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.driver == nil {
		return nil, fmt.Errorf("driver not initialized")
	}

	return c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   accessMode,
		DatabaseName: c.config.Database,
	}), nil
}

// RunQuery executes a Cypher query and returns results
func (c *Client) RunQuery(ctx context.Context, query string, params map[string]any) ([]*neo4j.Record, error) {
	session, err := c.GetSession(ctx, neo4j.AccessModeRead)
	if err != nil {
		return nil, err
	}
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	records, err := result.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect results: %w", err)
	}

	return records, nil
}

// RunWrite executes a write transaction
func (c *Client) RunWrite(ctx context.Context, work func(tx neo4j.ManagedTransaction) (any, error)) error {
	session, err := c.GetSession(ctx, neo4j.AccessModeWrite)
	if err != nil {
		return err
	}
	defer session.Close(ctx)

	_, err = session.ExecuteWrite(ctx, work)
	return err
}

// RunRead executes a read transaction
func (c *Client) RunRead(ctx context.Context, work func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	session, err := c.GetSession(ctx, neo4j.AccessModeRead)
	if err != nil {
		return nil, err
	}
	defer session.Close(ctx)

	return session.ExecuteRead(ctx, work)
}

// InitializeSchema creates indexes and constraints for the knowledge graph
func (c *Client) InitializeSchema(ctx context.Context) error {
	// Create indexes for entities
	queries := []string{
		// Entity indexes
		"CREATE INDEX entity_id_index IF NOT EXISTS FOR (e:Entity) ON (e.id)",
		"CREATE INDEX entity_name_index IF NOT EXISTS FOR (e:Entity) ON (e.name)",
		"CREATE INDEX entity_type_index IF NOT EXISTS FOR (e:Entity) ON (e.type)",
		"CREATE INDEX entity_created_index IF NOT EXISTS FOR (e:Entity) ON (e.created_at)",

		// Destination indexes
		"CREATE INDEX destination_id_index IF NOT EXISTS FOR (d:Destination) ON (d.id)",
		"CREATE INDEX destination_name_index IF NOT EXISTS FOR (d:Destination) ON (d.name)",

		// Attraction indexes
		"CREATE INDEX attraction_id_index IF NOT EXISTS FOR (a:Attraction) ON (a.id)",
		"CREATE INDEX attraction_name_index IF NOT EXISTS FOR (a:Attraction) ON (a.name)",

		// Food indexes
		"CREATE INDEX food_id_index IF NOT EXISTS FOR (f:Food) ON (f.id)",
		"CREATE INDEX food_name_index IF NOT EXISTS FOR (f:Food) ON (f.name)",

		// User preference indexes
		"CREATE INDEX user_id_index IF NOT EXISTS FOR (u:User) ON (u.id)",

		// Constraints
		"CREATE CONSTRAINT entity_id_unique IF NOT EXISTS FOR (e:Entity) REQUIRE e.id IS UNIQUE",
		"CREATE CONSTRAINT destination_id_unique IF NOT EXISTS FOR (d:Destination) REQUIRE d.id IS UNIQUE",
		"CREATE CONSTRAINT user_id_unique IF NOT EXISTS FOR (u:User) REQUIRE u.id IS UNIQUE",
	}

	for _, query := range queries {
		err := c.RunWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, query, nil)
			return nil, err
		})
		if err != nil {
			log.Printf("[Neo4j] Warning: schema initialization error for query %s: %v", query, err)
			// Continue with other queries
		}
	}

	log.Printf("[Neo4j] Schema initialization complete")
	return nil
}

// Stats returns Neo4j statistics
func (c *Client) Stats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]any{
		"uri":        c.config.URI,
		"database":   c.config.Database,
		"healthy":    c.healthy,
		"last_ping":  c.lastPing.Format(time.RFC3339),
	}
}

// ClearDatabase clears all data (use with caution!)
func (c *Client) ClearDatabase(ctx context.Context) error {
	return c.RunWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
		return nil, err
	})
}
