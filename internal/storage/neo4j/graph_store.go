// Package neo4j provides graph storage for knowledge graph
package neo4j

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// EntityType defines types of entities in the knowledge graph
type EntityType string

const (
	EntityTypeDestination  EntityType = "Destination"
	EntityTypeAttraction   EntityType = "Attraction"
	EntityTypeFood         EntityType = "Food"
	EntityTypeActivity     EntityType = "Activity"
	EntityTypeHotel        EntityType = "Hotel"
	EntityTypeRestaurant   EntityType = "Restaurant"
	EntityTypeCulturalSite EntityType = "CulturalSite"
	EntityTypeUser         EntityType = "User"
	EntityTypeTag          EntityType = "Tag"
)

// RelationType defines types of relationships in the knowledge graph
type RelationType string

const (
	RelationLocatedIn    RelationType = "LOCATED_IN"
	RelationFamousFor    RelationType = "FAMOUS_FOR"
	RelationSimilarTo    RelationType = "SIMILAR_TO"
	RelationHasAttraction RelationType = "HAS_ATTRACTION"
	RelationHasFood      RelationType = "HAS_FOOD"
	RelationHasActivity  RelationType = "HAS_ACTIVITY"
	RelationNearby       RelationType = "NEARBY"
	RelationPartOf       RelationType = "PART_OF"
	RelationRelatedTo    RelationType = "RELATED_TO"
	RelationPrefers      RelationType = "PREFERS"
	RelationVisited      RelationType = "VISITED"
	RelationTaggedWith   RelationType = "TAGGED_WITH"
)

// Entity represents a node in the knowledge graph
type Entity struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Type       EntityType     `json:"type"`
	Properties map[string]any `json:"properties"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// Relation represents an edge in the knowledge graph
type Relation struct {
	FromID    string       `json:"from_id"`
	ToID      string       `json:"to_id"`
	Type      RelationType `json:"type"`
	Weight    float64      `json:"weight"`
	Properties map[string]any `json:"properties"`
}

// SearchResult represents a graph search result
type SearchResult struct {
	Entity      Entity   `json:"entity"`
	Score       float64  `json:"score"`
	PathLength  int      `json:"path_length"`
	RelatedIDs  []string `json:"related_ids"`
}

// GraphStore provides graph storage operations
type GraphStore struct {
	client *Client
}

// NewGraphStore creates a new graph store
func NewGraphStore(client *Client) *GraphStore {
	return &GraphStore{client: client}
}

// CreateEntity creates a new entity in the graph
func (s *GraphStore) CreateEntity(ctx context.Context, entity *Entity) error {
	if entity.ID == "" {
		return fmt.Errorf("entity ID is required")
	}
	if entity.Name == "" {
		return fmt.Errorf("entity name is required")
	}
	if entity.Type == "" {
		entity.Type = EntityTypeDestination
	}
	if entity.Properties == nil {
		entity.Properties = make(map[string]any)
	}
	now := time.Now()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	return s.client.RunWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		params := map[string]any{
			"id":         entity.ID,
			"name":       entity.Name,
			"type":       string(entity.Type),
			"properties": entity.Properties,
			"created_at": entity.CreatedAt,
			"updated_at": entity.UpdatedAt,
		}

		query := fmt.Sprintf(`
			MERGE (e:%s {id: $id})
			SET e.name = $name,
			    e.type = $type,
			    e.properties = $properties,
			    e.created_at = $created_at,
			    e.updated_at = $updated_at
			RETURN e
		`, entity.Type)

		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
}

// GetEntity retrieves an entity by ID
func (s *GraphStore) GetEntity(ctx context.Context, id string) (*Entity, error) {
	result, err := s.client.RunRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, `
			MATCH (e:Entity {id: $id})
			RETURN e.id, e.name, e.type, e.properties, e.created_at, e.updated_at
			LIMIT 1
		`, map[string]any{"id": id})
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			record := result.Record()
			return &Entity{
				ID:         record.Values[0].(string),
				Name:       record.Values[1].(string),
				Type:       EntityType(record.Values[2].(string)),
				Properties: record.Values[3].(map[string]any),
				CreatedAt:  record.Values[4].(time.Time),
				UpdatedAt:  record.Values[5].(time.Time),
			}, nil
		}

		return nil, nil
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	return result.(*Entity), nil
}

// CreateRelation creates a relationship between two entities
func (s *GraphStore) CreateRelation(ctx context.Context, relation *Relation) error {
	if relation.FromID == "" || relation.ToID == "" {
		return fmt.Errorf("both from_id and to_id are required")
	}
	if relation.Type == "" {
		relation.Type = RelationRelatedTo
	}
	if relation.Weight == 0 {
		relation.Weight = 1.0
	}
	if relation.Properties == nil {
		relation.Properties = make(map[string]any)
	}

	return s.client.RunWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		params := map[string]any{
			"from_id":    relation.FromID,
			"to_id":      relation.ToID,
			"weight":     relation.Weight,
			"properties": relation.Properties,
		}

		query := fmt.Sprintf(`
			MATCH (from:Entity {id: $from_id})
			MATCH (to:Entity {id: $to_id})
			MERGE (from)-[r:%s]->(to)
			SET r.weight = $weight, r.properties = $properties
			RETURN r
		`, relation.Type)

		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
}

// FindRelated finds entities related to the given entity IDs
func (s *GraphStore) FindRelated(ctx context.Context, entityIDs []string, maxDepth int, limit int) ([]SearchResult, error) {
	if len(entityIDs) == 0 {
		return nil, nil
	}
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if limit <= 0 {
		limit = 10
	}

	result, err := s.client.RunRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (start:Entity)-[r*1..` + fmt.Sprintf("%d", maxDepth) + `]-(related:Entity)
			WHERE start.id IN $entity_ids
			WITH related, length(r) as pathLength, collect(DISTINCT start.id) as sources
			RETURN related.id, related.name, related.type, related.properties,
			       pathLength, count(sources) as score, sources
			ORDER BY score DESC, pathLength ASC
			LIMIT $limit
		`

		params := map[string]any{
			"entity_ids": entityIDs,
			"limit":      limit,
		}

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var results []SearchResult
		for result.Next(ctx) {
			record := result.Record()

			var relatedIDs []string
			if sources, ok := record.Values[6].([]any); ok {
				for _, s := range sources {
					if id, ok := s.(string); ok {
						relatedIDs = append(relatedIDs, id)
					}
				}
			}

			// Handle score as int64 and convert to float64
			var score float64
			if s, ok := record.Values[5].(int64); ok {
				score = float64(s)
			}

			// Handle path length as int64 and convert to int
			var pathLength int
			if p, ok := record.Values[4].(int64); ok {
				pathLength = int(p)
			}

			results = append(results, SearchResult{
				Entity: Entity{
					ID:         record.Values[0].(string),
					Name:       record.Values[1].(string),
					Type:       EntityType(record.Values[2].(string)),
					Properties: record.Values[3].(map[string]any),
				},
				Score:      score,
				PathLength: pathLength,
				RelatedIDs: relatedIDs,
			})
		}

		return results, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]SearchResult), nil
}

// DeleteEntity deletes an entity and all its relationships
func (s *GraphStore) DeleteEntity(ctx context.Context, id string) error {
	return s.client.RunWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MATCH (e:Entity {id: $id})
			DETACH DELETE e
		`, map[string]any{"id": id})
		return nil, err
	})
}

// SearchByName searches entities by name (fuzzy match)
func (s *GraphStore) SearchByName(ctx context.Context, name string, entityType EntityType, limit int) ([]Entity, error) {
	if limit <= 0 {
		limit = 10
	}

	result, err := s.client.RunRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		var query string
		params := map[string]any{
			"name":  name,
			"limit": limit,
		}

		if entityType != "" {
			query = fmt.Sprintf(`
				MATCH (e:%s)
				WHERE toLower(e.name) CONTAINS toLower($name)
				RETURN e.id, e.name, e.type, e.properties, e.created_at, e.updated_at
				LIMIT $limit
			`, entityType)
		} else {
			query = `
				MATCH (e:Entity)
				WHERE toLower(e.name) CONTAINS toLower($name)
				RETURN e.id, e.name, e.type, e.properties, e.created_at, e.updated_at
				LIMIT $limit
			`
		}

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var entities []Entity
		for result.Next(ctx) {
			record := result.Record()
			entities = append(entities, Entity{
				ID:         record.Values[0].(string),
				Name:       record.Values[1].(string),
				Type:       EntityType(record.Values[2].(string)),
				Properties: record.Values[3].(map[string]any),
				CreatedAt:  record.Values[4].(time.Time),
				UpdatedAt:  record.Values[5].(time.Time),
			})
		}

		return entities, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]Entity), nil
}

// GetEntityCount returns the total number of entities
func (s *GraphStore) GetEntityCount(ctx context.Context) (int64, error) {
	result, err := s.client.RunRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, "MATCH (e:Entity) RETURN count(e) as count", nil)
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			return result.Record().Values[0].(int64), nil
		}

		return int64(0), nil
	})

	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// GetRelationCount returns the total number of relationships
func (s *GraphStore) GetRelationCount(ctx context.Context) (int64, error) {
	result, err := s.client.RunRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, "MATCH ()-[r]->() RETURN count(r) as count", nil)
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			return result.Record().Values[0].(int64), nil
		}

		return int64(0), nil
	})

	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// GetEntitiesByType returns all entities of a specific type
func (s *GraphStore) GetEntitiesByType(ctx context.Context, entityType EntityType, limit int) ([]Entity, error) {
	if limit <= 0 {
		limit = 100
	}

	result, err := s.client.RunRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := fmt.Sprintf(`
			MATCH (e:%s)
			RETURN e.id, e.name, e.type, e.properties, e.created_at, e.updated_at
			LIMIT $limit
		`, entityType)

		params := map[string]any{"limit": limit}

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var entities []Entity
		for result.Next(ctx) {
			record := result.Record()
			entities = append(entities, Entity{
				ID:         record.Values[0].(string),
				Name:       record.Values[1].(string),
				Type:       EntityType(record.Values[2].(string)),
				Properties: record.Values[3].(map[string]any),
				CreatedAt:  record.Values[4].(time.Time),
				UpdatedAt:  record.Values[5].(time.Time),
			})
		}

		return entities, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]Entity), nil
}