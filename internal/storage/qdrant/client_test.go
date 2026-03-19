// Package qdrant_test provides integration tests for Qdrant client
package qdrant_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/utaaa/uta-travel-agent/internal/storage/qdrant"
)

// TestQdrantConnection tests basic connection to Qdrant
func TestQdrantConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := getEnvOrDefault("QDRANT_HOST", "localhost")
	port := 6334

	client, err := qdrant.NewClient(qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test listing collections (should work even if empty)
	collections, err := client.ListCollections(ctx)
	if err != nil {
		t.Fatalf("Failed to list collections: %v", err)
	}
	t.Logf("Found %d collections", len(collections))
}

// TestCreateAndDeleteCollection tests collection lifecycle
func TestCreateAndDeleteCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := getEnvOrDefault("QDRANT_HOST", "localhost")
	client, err := qdrant.NewClient(qdrant.Config{
		Host: host,
		Port: 6334,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	collectionName := fmt.Sprintf("test_collection_%d", time.Now().UnixNano())

	// Cleanup after test
	defer func() {
		_ = client.DeleteCollection(ctx, collectionName)
	}()

	// Create collection
	err = client.CreateCollection(ctx, qdrant.CollectionConfig{
		Name:       collectionName,
		VectorSize: 384,
		Distance:   "Cosine",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}
	t.Logf("Created collection: %s", collectionName)

	// Verify collection exists
	info, err := client.CollectionInfo(ctx, collectionName)
	if err != nil {
		t.Fatalf("Failed to get collection info: %v", err)
	}
	if info.Name != collectionName {
		t.Errorf("Expected collection name %s, got %s", collectionName, info.Name)
	}
	if info.VectorSize != 384 {
		t.Errorf("Expected vector size 384, got %d", info.VectorSize)
	}
	t.Logf("Collection info: %+v", info)

	// Delete collection
	err = client.DeleteCollection(ctx, collectionName)
	if err != nil {
		t.Fatalf("Failed to delete collection: %v", err)
	}
	t.Logf("Deleted collection: %s", collectionName)
}

// TestUpsertAndSearch tests vector upsert and search operations
func TestUpsertAndSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := getEnvOrDefault("QDRANT_HOST", "localhost")
	client, err := qdrant.NewClient(qdrant.Config{
		Host: host,
		Port: 6334,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	collectionName := fmt.Sprintf("test_vectors_%d", time.Now().UnixNano())

	// Cleanup after test
	defer func() {
		_ = client.DeleteCollection(ctx, collectionName)
	}()

	// Create collection with small vectors for testing
	err = client.CreateCollection(ctx, qdrant.CollectionConfig{
		Name:       collectionName,
		VectorSize: 4,
		Distance:   "Cosine",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	// Wait for collection to be ready
	time.Sleep(500 * time.Millisecond)

	// Insert test vectors
	points := []qdrant.Point{
		{
			ID:     "doc-1",
			Vector: []float32{0.1, 0.2, 0.3, 0.4},
			Payload: map[string]interface{}{
				"title":    "Document 1",
				"category": "travel",
			},
		},
		{
			ID:     "doc-2",
			Vector: []float32{0.5, 0.6, 0.7, 0.8},
			Payload: map[string]interface{}{
				"title":    "Document 2",
				"category": "food",
			},
		},
		{
			ID:     "doc-3",
			Vector: []float32{0.15, 0.25, 0.35, 0.45},
			Payload: map[string]interface{}{
				"title":    "Document 3",
				"category": "travel",
			},
		},
	}

	err = client.Upsert(ctx, collectionName, points)
	if err != nil {
		t.Fatalf("Failed to upsert points: %v", err)
	}
	t.Logf("Upserted %d points", len(points))

	// Wait for indexing
	time.Sleep(500 * time.Millisecond)

	// Search for similar vectors
	queryVector := []float32{0.1, 0.2, 0.3, 0.4} // Same as doc-1
	results, err := client.Search(ctx, collectionName, queryVector, 3)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected search results, got none")
	}

	t.Logf("Search results (%d):", len(results))
	for i, r := range results {
		t.Logf("  %d. ID=%s Score=%.4f Payload=%+v", i+1, r.ID, r.Score, r.Payload)
	}

	// First result should be doc-1 (exact match)
	// Check via payload _id field since actual ID is UUID
	firstID, ok := results[0].Payload["_id"].(string)
	if !ok || firstID != "doc-1" {
		t.Errorf("Expected first result _id to be doc-1, got %v", firstID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("Expected score close to 1.0, got %.4f", results[0].Score)
	}
}

// TestScrollPagination tests scrolling through points
func TestScrollPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := getEnvOrDefault("QDRANT_HOST", "localhost")
	client, err := qdrant.NewClient(qdrant.Config{
		Host: host,
		Port: 6334,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	collectionName := fmt.Sprintf("test_scroll_%d", time.Now().UnixNano())

	defer func() {
		_ = client.DeleteCollection(ctx, collectionName)
	}()

	err = client.CreateCollection(ctx, qdrant.CollectionConfig{
		Name:       collectionName,
		VectorSize: 4,
		Distance:   "Cosine",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Insert multiple points
	var points []qdrant.Point
	for i := 0; i < 5; i++ {
		points = append(points, qdrant.Point{
			ID:     fmt.Sprintf("point-%d", i),
			Vector: []float32{float32(i) * 0.1, float32(i) * 0.2, float32(i) * 0.3, float32(i) * 0.4},
			Payload: map[string]interface{}{
				"index": i,
			},
		})
	}

	err = client.Upsert(ctx, collectionName, points)
	if err != nil {
		t.Fatalf("Failed to upsert points: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Scroll through points
	page1, nextOffset, err := client.Scroll(ctx, collectionName, 2, "")
	if err != nil {
		t.Fatalf("Failed to scroll: %v", err)
	}

	if len(page1) != 2 {
		t.Errorf("Expected 2 points in first page, got %d", len(page1))
	}
	t.Logf("Page 1: %d points, nextOffset=%s", len(page1), nextOffset)

	if nextOffset == "" {
		t.Fatal("Expected nextOffset to be set for pagination")
	}

	// Get second page
	page2, _, err := client.Scroll(ctx, collectionName, 2, nextOffset)
	if err != nil {
		t.Fatalf("Failed to scroll second page: %v", err)
	}

	t.Logf("Page 2: %d points", len(page2))
}

// TestDeletePoints tests point deletion
func TestDeletePoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := getEnvOrDefault("QDRANT_HOST", "localhost")
	client, err := qdrant.NewClient(qdrant.Config{
		Host: host,
		Port: 6334,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	collectionName := fmt.Sprintf("test_delete_%d", time.Now().UnixNano())

	defer func() {
		_ = client.DeleteCollection(ctx, collectionName)
	}()

	err = client.CreateCollection(ctx, qdrant.CollectionConfig{
		Name:       collectionName,
		VectorSize: 4,
		Distance:   "Cosine",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Insert points
	points := []qdrant.Point{
		{ID: "del-1", Vector: []float32{0.1, 0.2, 0.3, 0.4}},
		{ID: "del-2", Vector: []float32{0.5, 0.6, 0.7, 0.8}},
		{ID: "del-3", Vector: []float32{0.9, 1.0, 1.1, 1.2}},
	}
	err = client.Upsert(ctx, collectionName, points)
	if err != nil {
		t.Fatalf("Failed to upsert points: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Delete one point
	err = client.DeletePoints(ctx, collectionName, []string{"del-2"})
	if err != nil {
		t.Fatalf("Failed to delete points: %v", err)
	}
	t.Log("Deleted point del-2")

	time.Sleep(500 * time.Millisecond)

	// Verify deletion - search should only return 2 points
	results, err := client.Search(ctx, collectionName, []float32{0.5, 0.5, 0.5, 0.5}, 10)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	for _, r := range results {
		if r.ID == "del-2" {
			t.Error("Point del-2 should have been deleted")
		}
	}
	t.Logf("Found %d points after deletion", len(results))
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
