package tools

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestWikipediaSearchTool(t *testing.T) {
	tool := NewWikipediaSearchTool("zh")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, map[string]any{
		"query":         "京都",
		"limit":         3,
		"fetch_content": true,
	})

	if err != nil {
		t.Fatalf("Wikipedia search failed: %v", err)
	}

	// Verify result structure
	if result["query"] != "京都" {
		t.Errorf("Expected query '京都', got %v", result["query"])
	}

	results, ok := result["results"].([]WikipediaSearchResult)
	if !ok {
		t.Fatalf("Expected results to be []WikipediaSearchResult, got %T", result["results"])
	}

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Check first result has content
	if results[0].Content == "" {
		t.Error("Expected content in first result")
	}

	t.Logf("✓ Wikipedia search returned %d results", len(results))
	t.Logf("  First result: %s (content length: %d)", results[0].Title, len(results[0].Content))
}

func TestWikipediaSearchToolEnglish(t *testing.T) {
	tool := NewWikipediaSearchTool("en")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, map[string]any{
		"query":         "Tokyo",
		"limit":         2,
		"fetch_content": false, // Don't fetch content for speed
	})

	if err != nil {
		t.Fatalf("Wikipedia search failed: %v", err)
	}

	results, ok := result["results"].([]WikipediaSearchResult)
	if !ok {
		t.Fatalf("Expected results to be []WikipediaSearchResult, got %T", result["results"])
	}

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	t.Logf("✓ English Wikipedia search returned %d results", len(results))
}

func TestTavilySearchTool(t *testing.T) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping Tavily test")
	}

	tool := NewTavilySearchTool(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, map[string]any{
		"query":       "京都 天气 今天",
		"max_results": 3,
	})

	if err != nil {
		t.Fatalf("Tavily search failed: %v", err)
	}

	// Verify result structure
	if result["answer"] == "" && len(result["results"].([]map[string]any)) == 0 {
		t.Error("Expected either answer or results")
	}

	t.Logf("✓ Tavily search returned answer: %v", result["answer"] != "")
	if results, ok := result["results"].([]map[string]any); ok {
		t.Logf("  Results count: %d", len(results))
	}
}

func TestWebReaderTool(t *testing.T) {
	tool := NewWebReaderTool()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, map[string]any{
		"url": "https://en.wikipedia.org/wiki/Kyoto",
	})

	if err != nil {
		t.Fatalf("Web reader failed: %v", err)
	}

	// Verify result structure
	if result["title"] == "" {
		t.Error("Expected title in result")
	}

	content, ok := result["content"].(string)
	if !ok || content == "" {
		t.Error("Expected content in result")
	}

	t.Logf("✓ Web reader returned content (length: %d)", len(content))
	t.Logf("  Title: %s", result["title"])
}

func TestWebReaderToolWithProxy(t *testing.T) {
	// Set proxy if available
	proxy := os.Getenv("HTTP_PROXY")
	if proxy == "" {
		proxy = os.Getenv("http_proxy")
	}
	if proxy == "" {
		t.Skip("No proxy configured, skipping proxy test")
	}

	// This test verifies the tool works with proxy
	tool := NewWebReaderTool()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := tool.Execute(ctx, map[string]any{
		"url": "https://www.example.com",
	})

	if err != nil {
		t.Fatalf("Web reader with proxy failed: %v", err)
	}

	t.Logf("✓ Web reader works with proxy")
	t.Logf("  Content length: %d", result["content_length"])
}
