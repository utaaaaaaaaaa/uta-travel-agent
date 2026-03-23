package neo4j

import (
	"context"
	"testing"
	"time"
)

// Note: These tests require a running Neo4j instance.
// For CI/CD, use testcontainers or mock the driver.

func TestEntityCreation(t *testing.T) {
	entity := &Entity{
		ID:   "test-destination-1",
		Name: "Tokyo",
		Type: EntityTypeDestination,
		Properties: map[string]any{
			"country":    "Japan",
			"population": 14000000,
		},
	}

	if entity.ID == "" {
		t.Error("entity ID should not be empty")
	}
	if entity.Name != "Tokyo" {
		t.Errorf("expected name Tokyo, got %s", entity.Name)
	}
	if entity.Type != EntityTypeDestination {
		t.Errorf("expected type Destination, got %s", entity.Type)
	}
	if entity.Properties["country"] != "Japan" {
		t.Errorf("expected country Japan, got %v", entity.Properties["country"])
	}
}

func TestRelationCreation(t *testing.T) {
	relation := &Relation{
		FromID: "entity-1",
		ToID:   "entity-2",
		Type:   RelationLocatedIn,
		Weight: 1.0,
	}

	if relation.FromID == "" || relation.ToID == "" {
		t.Error("relation should have both from and to IDs")
	}
	if relation.Type != RelationLocatedIn {
		t.Errorf("expected type LOCATED_IN, got %s", relation.Type)
	}
	if relation.Weight <= 0 {
		t.Error("relation weight should be positive")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.URI == "" {
		t.Error("default URI should not be empty")
	}
	if config.Username == "" {
		t.Error("default username should not be empty")
	}
	if config.Password == "" {
		t.Error("default password should not be empty")
	}
	if config.Database == "" {
		t.Error("default database should not be empty")
	}
}

func TestEntityTypeValidation(t *testing.T) {
	validTypes := []EntityType{
		EntityTypeDestination,
		EntityTypeAttraction,
		EntityTypeFood,
		EntityTypeActivity,
		EntityTypeHotel,
		EntityTypeRestaurant,
		EntityTypeCulturalSite,
		EntityTypeUser,
		EntityTypeTag,
	}

	for _, et := range validTypes {
		if et == "" {
			t.Error("entity type should not be empty")
		}
	}
}

func TestRelationTypeValidation(t *testing.T) {
	validTypes := []RelationType{
		RelationLocatedIn,
		RelationFamousFor,
		RelationSimilarTo,
		RelationHasAttraction,
		RelationHasFood,
		RelationHasActivity,
		RelationNearby,
		RelationPartOf,
		RelationRelatedTo,
		RelationPrefers,
		RelationVisited,
		RelationTaggedWith,
	}

	for _, rt := range validTypes {
		if rt == "" {
			t.Error("relation type should not be empty")
		}
	}
}

func TestSearchResult(t *testing.T) {
	result := SearchResult{
		Entity: Entity{
			ID:   "test-1",
			Name: "Test Entity",
			Type: EntityTypeAttraction,
		},
		Score:      0.85,
		PathLength: 2,
		RelatedIDs: []string{"related-1", "related-2"},
	}

	if result.Score < 0 || result.Score > 1 {
		t.Errorf("score should be between 0 and 1, got %f", result.Score)
	}
	if result.PathLength < 0 {
		t.Errorf("path length should be non-negative, got %d", result.PathLength)
	}
	if len(result.RelatedIDs) == 0 {
		t.Error("related IDs should not be empty for test")
	}
}

// MockClient for testing without real Neo4j
type MockClient struct {
	healthy bool
	err     error
}

func NewMockClient() *MockClient {
	return &MockClient{healthy: true}
}

func (m *MockClient) Close() error {
	return nil
}

func (m *MockClient) Ping(ctx context.Context) error {
	return m.err
}

func (m *MockClient) IsHealthy() bool {
	return m.healthy
}

func (m *MockClient) Stats() map[string]any {
	return map[string]any{
		"healthy": m.healthy,
	}
}

func TestMockClient(t *testing.T) {
	client := NewMockClient()

	if !client.IsHealthy() {
		t.Error("mock client should be healthy")
	}

	stats := client.Stats()
	if stats["healthy"] != true {
		t.Error("stats should show healthy")
	}

	err := client.Close()
	if err != nil {
		t.Errorf("close should not error: %v", err)
	}
}

func TestEntityTimestamps(t *testing.T) {
	entity := &Entity{
		ID:   "test-time",
		Name: "Time Test",
		Type: EntityTypeDestination,
	}

	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	if entity.CreatedAt.IsZero() {
		t.Error("created_at should not be zero")
	}
	if entity.UpdatedAt.IsZero() {
		t.Error("updated_at should not be zero")
	}

	// Simulate update
	entity.UpdatedAt = time.Now()
	if entity.UpdatedAt.Before(entity.CreatedAt) {
		t.Error("updated_at should not be before created_at")
	}
}

func TestEntityProperties(t *testing.T) {
	entity := &Entity{
		ID:   "test-props",
		Name: "Properties Test",
		Type: EntityTypeAttraction,
		Properties: map[string]any{
			"opening_hours": "9:00-18:00",
			"ticket_price":  50,
			"is_unesco":     true,
			"tags":          []string{"temple", "historic"},
		},
	}

	// Test string property
	if hours, ok := entity.Properties["opening_hours"].(string); !ok || hours == "" {
		t.Error("opening_hours should be a non-empty string")
	}

	// Test numeric property
	if price, ok := entity.Properties["ticket_price"].(int); !ok || price <= 0 {
		t.Error("ticket_price should be a positive integer")
	}

	// Test boolean property
	if isUnesco, ok := entity.Properties["is_unesco"].(bool); !ok {
		t.Error("is_unesco should be a boolean")
	} else if !isUnesco {
		t.Error("is_unesco should be true for test entity")
	}

	// Test slice property
	if tags, ok := entity.Properties["tags"].([]string); !ok || len(tags) == 0 {
		t.Error("tags should be a non-empty string slice")
	}
}

func TestRelationWeight(t *testing.T) {
	tests := []struct {
		weight    float64
		shouldErr bool
	}{
		{1.0, false},
		{0.5, false},
		{2.0, false},
		{0.0, true}, // Zero weight might indicate missing initialization
	}

	for _, tt := range tests {
		relation := &Relation{
			FromID: "a",
			ToID:   "b",
			Type:   RelationRelatedTo,
			Weight: tt.weight,
		}

		if tt.shouldErr && relation.Weight > 0 {
			t.Errorf("expected validation error for weight %f", tt.weight)
		}
		if !tt.shouldErr && relation.Weight <= 0 {
			t.Errorf("unexpected validation error for weight %f", tt.weight)
		}
	}
}

func TestGraphStoreInterface(t *testing.T) {
	// Verify GraphStore interface compliance
	var _ interface {
		CreateEntity(ctx context.Context, entity *Entity) error
		GetEntity(ctx context.Context, id string) (*Entity, error)
		CreateRelation(ctx context.Context, relation *Relation) error
		FindRelated(ctx context.Context, entityIDs []string, maxDepth int, limit int) ([]SearchResult, error)
		DeleteEntity(ctx context.Context, id string) error
		SearchByName(ctx context.Context, name string, entityType EntityType, limit int) ([]Entity, error)
	} = (*GraphStore)(nil)
}