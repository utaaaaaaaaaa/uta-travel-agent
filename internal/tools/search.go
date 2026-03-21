// Package tools provides tool implementations for UTA agents.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WikipediaSearchTool searches Wikipedia for information.
// It uses the Wikipedia API to search and retrieve page content.
type WikipediaSearchTool struct {
	client *http.Client
	lang   string // "zh" or "en"
}

// NewWikipediaSearchTool creates a new Wikipedia search tool.
func NewWikipediaSearchTool(lang string) *WikipediaSearchTool {
	if lang == "" {
		lang = "zh"
	}
	return &WikipediaSearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		lang: lang,
	}
}

// NewWikipediaSearchToolWithProxy creates a new Wikipedia search tool with proxy support.
func NewWikipediaSearchToolWithProxy(lang, proxyURL string) *WikipediaSearchTool {
	if lang == "" {
		lang = "zh"
	}

	transport := &http.Transport{}
	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyParsed)
		}
	}

	return &WikipediaSearchTool{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		lang: lang,
	}
}

// WikipediaSearchResult represents a search result from Wikipedia.
type WikipediaSearchResult struct {
	Title       string `json:"title"`
	PageID      int    `json:"pageid"`
	Snippet     string `json:"snippet"`
	URL         string `json:"url"`
	Content     string `json:"content,omitempty"`
	LastUpdated string `json:"last_updated,omitempty"`
}

// wikipediaSearchResponse represents the API response for search.
type wikipediaSearchResponse struct {
	Query struct {
		Search []struct {
			Title   string `json:"title"`
			PageID  int    `json:"pageid"`
			Snippet string `json:"snippet"`
		} `json:"search"`
	} `json:"query"`
}

// wikipediaContentResponse represents the API response for page content.
type wikipediaContentResponse struct {
	Query struct {
		Pages map[string]struct {
			Title     string `json:"title"`
			PageID    int    `json:"pageid"`
			Extract   string `json:"extract"`
			FullURL   string `json:"fullurl"`
			Touched   string `json:"touched"`
			Redirects []struct {
				Title string `json:"title"`
			} `json:"redirects,omitempty"`
		} `json:"pages"`
	} `json:"query"`
}

// Execute performs a Wikipedia search.
func (t *WikipediaSearchTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	// Get optional parameters
	limit := 5
	if l, ok := params["limit"].(int); ok && l > 0 {
		limit = l
	}
	if l, ok := params["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	fetchContent := true
	if fc, ok := params["fetch_content"].(bool); ok {
		fetchContent = fc
	}

	// Step 1: Search for pages
	results, err := t.searchPages(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Step 2: Fetch content for top results if requested
	if fetchContent && len(results) > 0 {
		for i := range results {
			if results[i].PageID > 0 {
				content, err := t.getPageContent(ctx, results[i].PageID)
				if err == nil {
					results[i].Content = content
				}
			}
		}
	}

	return map[string]any{
		"query":      query,
		"lang":       t.lang,
		"count":      len(results),
		"results":    results,
		"source":     "wikipedia",
		"source_url": fmt.Sprintf("https://%s.wikipedia.org", t.lang),
	}, nil
}

// searchPages searches Wikipedia for pages matching the query.
func (t *WikipediaSearchTool) searchPages(ctx context.Context, query string, limit int) ([]WikipediaSearchResult, error) {
	apiURL := fmt.Sprintf(
		"https://%s.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&format=json&utf8=&srlimit=%d",
		t.lang,
		url.QueryEscape(query),
		limit,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "UTA-Travel-Agent/1.0 (https://github.com/uta-travel-agent)")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia API returned status %d", resp.StatusCode)
	}

	var searchResp wikipediaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]WikipediaSearchResult, 0, len(searchResp.Query.Search))
	for _, item := range searchResp.Query.Search {
		results = append(results, WikipediaSearchResult{
			Title:   item.Title,
			PageID:  item.PageID,
			Snippet: t.cleanHTML(item.Snippet),
			URL:     fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", t.lang, url.PathEscape(item.Title)),
		})
	}

	return results, nil
}

// getPageContent fetches the full content of a Wikipedia page.
func (t *WikipediaSearchTool) getPageContent(ctx context.Context, pageID int) (string, error) {
	apiURL := fmt.Sprintf(
		"https://%s.wikipedia.org/w/api.php?action=query&prop=extracts|info&pageids=%d&format=json&explaintext=&inprop=url",
		t.lang,
		pageID,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "UTA-Travel-Agent/1.0 (https://github.com/uta-travel-agent)")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wikipedia API returned status %d", resp.StatusCode)
	}

	var contentResp wikipediaContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&contentResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	for _, page := range contentResp.Query.Pages {
		if page.PageID == pageID {
			return page.Extract, nil
		}
	}

	return "", fmt.Errorf("page not found")
}

// cleanHTML removes HTML tags from the snippet.
func (t *WikipediaSearchTool) cleanHTML(s string) string {
	// Remove HTML tags
	s = strings.ReplaceAll(s, "<span class=\"searchmatch\">", "")
	s = strings.ReplaceAll(s, "</span>", "")
	return s
}

// GetName returns the tool name.
func (t *WikipediaSearchTool) GetName() string {
	return "wikipedia_search"
}

// GetDescription returns the tool description.
func (t *WikipediaSearchTool) GetDescription() string {
	return "Search Wikipedia for authoritative information. Use for historical facts, cultural background, and general knowledge. Supports multiple languages."
}

// GetParameters returns the tool parameters schema.
func (t *WikipediaSearchTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results (default: 5)",
				"default":     5,
			},
			"fetch_content": map[string]any{
				"type":        "boolean",
				"description": "Whether to fetch full page content (default: true)",
				"default":     true,
			},
		},
		"required": []string{"query"},
	}
}

// TavilySearchTool performs real-time web search using Tavily API.
// Use this for dynamic information like prices, schedules, weather, etc.
type TavilySearchTool struct {
	apiKey string
	client *http.Client
}

// NewTavilySearchTool creates a new Tavily search tool.
func NewTavilySearchTool(apiKey string) *TavilySearchTool {
	return &TavilySearchTool{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewTavilySearchToolWithProxy creates a new Tavily search tool with proxy support.
func NewTavilySearchToolWithProxy(apiKey, proxyURL string) *TavilySearchTool {
	transport := &http.Transport{}
	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyParsed)
		}
	}

	return &TavilySearchTool{
		apiKey: apiKey,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// tavilySearchRequest represents the request body for Tavily API.
type tavilySearchRequest struct {
	APIKey        string `json:"api_key"`
	Query         string `json:"query"`
	SearchDepth   string `json:"search_depth,omitempty"`
	IncludeAnswer bool   `json:"include_answer,omitempty"`
	MaxResults    int    `json:"max_results,omitempty"`
}

// tavilySearchResponse represents the response from Tavily API.
type tavilySearchResponse struct {
	Answer string `json:"answer,omitempty"`
	Query  string `json:"query"`
	Result []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
}

// Execute performs a Tavily search.
func (t *TavilySearchTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	if t.apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY not configured")
	}

	maxResults := 5
	if mr, ok := params["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	}
	if mr, ok := params["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	searchDepth := "basic"
	if sd, ok := params["search_depth"].(string); ok {
		searchDepth = sd
	}

	reqBody := tavilySearchRequest{
		APIKey:        t.apiKey,
		Query:         query,
		SearchDepth:   searchDepth,
		IncludeAnswer: true,
		MaxResults:    maxResults,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tavily API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tavilyResp tavilySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert results
	results := make([]map[string]any, 0, len(tavilyResp.Result))
	for _, r := range tavilyResp.Result {
		results = append(results, map[string]any{
			"title":   r.Title,
			"url":     r.URL,
			"content": r.Content,
			"score":   r.Score,
		})
	}

	return map[string]any{
		"query":        query,
		"answer":       tavilyResp.Answer,
		"count":        len(results),
		"results":      results,
		"source":       "tavily",
		"search_depth": searchDepth,
	}, nil
}

// GetName returns the tool name.
func (t *TavilySearchTool) GetName() string {
	return "tavily_search"
}

// GetDescription returns the tool description.
func (t *TavilySearchTool) GetDescription() string {
	return "Real-time web search for dynamic information. Use for current prices, opening hours, weather, traffic, and recent news. Triggers: 'today', 'now', 'price', 'schedule', 'weather', 'traffic'."
}

// GetParameters returns the tool parameters schema.
func (t *TavilySearchTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results (default: 5)",
				"default":     5,
			},
			"search_depth": map[string]any{
				"type":        "string",
				"description": "Search depth: 'basic' or 'advanced'",
				"enum":        []string{"basic", "advanced"},
				"default":     "basic",
			},
		},
		"required": []string{"query"},
	}
}

// WebReaderTool reads and extracts content from web pages.
type WebReaderTool struct {
	client *http.Client
}

// NewWebReaderTool creates a new web reader tool.
func NewWebReaderTool() *WebReaderTool {
	return &WebReaderTool{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// NewWebReaderToolWithProxy creates a new web reader tool with proxy support.
func NewWebReaderToolWithProxy(proxyURL string) *WebReaderTool {
	transport := &http.Transport{}
	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyParsed)
		}
	}

	return &WebReaderTool{
		client: &http.Client{
			Timeout:   20 * time.Second,
			Transport: transport,
		},
	}
}

// Execute reads content from a URL.
func (t *WebReaderTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	urlStr, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; UTA-Bot/1.0; +https://github.com/uta-travel-agent)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	// Read and parse content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	content := string(body)
	title := t.extractTitle(content)
	textContent := t.extractTextContent(content)

	return map[string]any{
		"url":           urlStr,
		"title":         title,
		"content":       textContent,
		"content_length": len(textContent),
		"status":        resp.StatusCode,
	}, nil
}

// extractTitle extracts the title from HTML.
func (t *WebReaderTool) extractTitle(html string) string {
	// Simple title extraction
	start := strings.Index(html, "<title>")
	end := strings.Index(html, "</title>")
	if start != -1 && end != -1 && end > start {
		title := html[start+7 : end]
		return strings.TrimSpace(title)
	}
	return ""
}

// extractTextContent extracts readable text content from HTML.
func (t *WebReaderTool) extractTextContent(html string) string {
	// Remove script and style blocks
	html = t.removeTagContent(html, "script")
	html = t.removeTagContent(html, "style")
	html = t.removeTagContent(html, "nav")
	html = t.removeTagContent(html, "header")
	html = t.removeTagContent(html, "footer")

	// Remove HTML tags
	text := t.stripTags(html)

	// Clean up whitespace
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	text = strings.ReplaceAll(text, "  ", " ")

	// Limit content length
	maxLen := 10000
	if len(text) > maxLen {
		text = text[:maxLen] + "... [truncated]"
	}

	return strings.TrimSpace(text)
}

// removeTagContent removes content between tags.
func (t *WebReaderTool) removeTagContent(html, tag string) string {
	openTag := "<" + tag
	closeTag := "</" + tag + ">"

	for {
		start := strings.Index(html, openTag)
		if start == -1 {
			break
		}
		end := strings.Index(html, closeTag)
		if end == -1 || end < start {
			break
		}
		html = html[:start] + html[end+len(closeTag):]
	}
	return html
}

// stripTags removes HTML tags.
func (t *WebReaderTool) stripTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// GetName returns the tool name.
func (t *WebReaderTool) GetName() string {
	return "web_reader"
}

// GetDescription returns the tool description.
func (t *WebReaderTool) GetDescription() string {
	return "Read and extract text content from a web page URL. Use to get detailed information from specific pages found via search."
}

// GetParameters returns the tool parameters schema.
func (t *WebReaderTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to read",
			},
		},
		"required": []string{"url"},
	}
}

// BaiduBaikeSearchTool searches Baidu Baike (百度百科) for Chinese encyclopedic content.
// This is particularly useful for Chinese destinations, culture, and history.
type BaiduBaikeSearchTool struct {
	client *http.Client
}

// NewBaiduBaikeSearchTool creates a new Baidu Baike search tool.
func NewBaiduBaikeSearchTool() *BaiduBaikeSearchTool {
	return &BaiduBaikeSearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewBaiduBaikeSearchToolWithProxy creates a new Baidu Baike search tool with proxy support.
func NewBaiduBaikeSearchToolWithProxy(proxyURL string) *BaiduBaikeSearchTool {
	transport := &http.Transport{}
	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyParsed)
		}
	}

	return &BaiduBaikeSearchTool{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// BaiduBaikeResult represents a search result from Baidu Baike.
type BaiduBaikeResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	Content     string `json:"content,omitempty"`
	Description string `json:"description,omitempty"`
}

// Execute performs the search and returns results.
func (t *BaiduBaikeSearchTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	query, ok := params["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter required")
	}

	limit := 5
	if l, ok := params["limit"].(int); ok {
		limit = l
	}

	fetchContent := true
	if fc, ok := params["fetch_content"].(bool); ok {
		fetchContent = fc
	}

	// Step 1: Search for entries
	results, err := t.searchBaike(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("baike search failed: %w", err)
	}

	// Step 2: Fetch content for top results if requested
	if fetchContent && len(results) > 0 {
		for i := range results {
			if results[i].URL != "" {
				content, err := t.getPageContent(ctx, results[i].URL)
				if err == nil {
					results[i].Content = content
				}
			}
		}
	}

	return map[string]any{
		"query":      query,
		"count":      len(results),
		"results":    results,
		"source":     "baidu_baike",
		"source_url": "https://baike.baidu.com",
	}, nil
}

// searchBaike searches Baidu Baike for entries matching the query.
func (t *BaiduBaikeSearchTool) searchBaike(ctx context.Context, query string, limit int) ([]BaiduBaikeResult, error) {
	// Baidu Baike search API
	apiURL := fmt.Sprintf(
		"https://baike.baidu.com/api/openapi/BaikeLemmaCardApi?scope=1034&format=json&appid=379020&bk_key=%s&bk_length=%d",
		url.QueryEscape(query),
		limit,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("baike API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var baikeResp struct {
		ErrNo  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
		Data   []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Desc  string `json:"desc"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &baikeResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]BaiduBaikeResult, 0, len(baikeResp.Data))
	for _, item := range baikeResp.Data {
		results = append(results, BaiduBaikeResult{
			Title:       item.Title,
			URL:         item.URL,
			Snippet:     item.Desc,
			Description: item.Desc,
		})
	}

	// If API didn't return results, try direct search
	if len(results) == 0 {
		return t.searchBaikeDirect(ctx, query, limit)
	}

	return results, nil
}

// searchBaikeDirect performs a direct search on Baidu Baike website.
func (t *BaiduBaikeSearchTool) searchBaikeDirect(ctx context.Context, query string, limit int) ([]BaiduBaikeResult, error) {
	searchURL := fmt.Sprintf("https://baike.baidu.com/search?word=%s&pn=0&rn=%d", url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)
	results := make([]BaiduBaikeResult, 0)

	// Parse search results from HTML
	// Look for result items
	titleStart := strings.Index(html, "class=\"result-list\"")
	if titleStart == -1 {
		return results, nil
	}

	// Extract titles and URLs
	items := strings.Split(html[titleStart:], "class=\"result-title\"")
	for i := 1; i < len(items) && len(results) < limit; i++ {
		item := items[i]

		// Extract title
		titleStart := strings.Index(item, ">")
		titleEnd := strings.Index(item, "</a>")
		if titleStart != -1 && titleEnd != -1 && titleEnd > titleStart {
			title := strings.TrimSpace(item[titleStart+1 : titleEnd])
			title = t.stripHTMLTags(title)

			// Extract URL
			hrefStart := strings.Index(item, "href=\"")
			hrefEnd := strings.Index(item[hrefStart+6:], "\"")
			if hrefStart != -1 && hrefEnd != -1 {
				href := item[hrefStart+6 : hrefStart+6+hrefEnd]
				if !strings.HasPrefix(href, "http") {
					href = "https://baike.baidu.com" + href
				}

				// Extract snippet
				snippet := ""
				snippetStart := strings.Index(item, "class=\"result-summary\"")
				if snippetStart != -1 {
					snippetEnd := strings.Index(item[snippetStart:], "</div>")
					if snippetEnd != -1 {
						snippet = t.stripHTMLTags(item[snippetStart : snippetStart+snippetEnd])
						snippet = strings.TrimSpace(snippet)
					}
				}

				results = append(results, BaiduBaikeResult{
					Title:   title,
					URL:     href,
					Snippet: snippet,
				})
			}
		}
	}

	return results, nil
}

// getPageContent fetches the content of a Baidu Baike page.
func (t *BaiduBaikeSearchTool) getPageContent(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	html := string(body)

	// Extract main content from Baidu Baike page
	// Look for lemma-summary and lemma-content
	var content strings.Builder

	// Extract summary
	summaryStart := strings.Index(html, "class=\"lemma-summary\"")
	if summaryStart != -1 {
		summaryEnd := strings.Index(html[summaryStart:], "</div>")
		if summaryEnd != -1 {
			summary := t.stripHTMLTags(html[summaryStart : summaryStart+summaryEnd])
			content.WriteString("摘要: ")
			content.WriteString(strings.TrimSpace(summary))
			content.WriteString("\n\n")
		}
	}

	// Extract main content paragraphs
	contentStart := strings.Index(html, "class=\"lemma-content\"")
	if contentStart != -1 {
		contentEnd := strings.Index(html[contentStart:], "class=\"bottom-part\"")
		if contentEnd == -1 {
			contentEnd = strings.Index(html[contentStart:], "class=\"side-content\"")
		}
		if contentEnd == -1 {
			contentEnd = 20000 // Limit to ~20k chars
		}
		mainContent := html[contentStart : contentStart+contentEnd]
		mainContent = t.stripHTMLTags(mainContent)
		content.WriteString(strings.TrimSpace(mainContent))
	}

	result := content.String()
	if len(result) > 10000 {
		result = result[:10000] + "... [truncated]"
	}

	return result, nil
}

// stripHTMLTags removes HTML tags from text.
func (t *BaiduBaikeSearchTool) stripHTMLTags(s string) string {
	// Remove common HTML entities
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")

	// Remove HTML tags
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// Clean up whitespace
	text := result.String()
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	text = strings.ReplaceAll(text, "  ", " ")

	return text
}

// GetName returns the tool name.
func (t *BaiduBaikeSearchTool) GetName() string {
	return "baidu_baike_search"
}

// GetDescription returns the tool description.
func (t *BaiduBaikeSearchTool) GetDescription() string {
	return "Search Baidu Baike (百度百科) for Chinese encyclopedic content. Best for Chinese destinations, culture, history, and local knowledge. Use together with Wikipedia for comprehensive coverage."
}

// GetParameters returns the tool parameters schema.
func (t *BaiduBaikeSearchTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query (Chinese keywords work best)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results (default: 5)",
				"default":     5,
			},
			"fetch_content": map[string]any{
				"type":        "boolean",
				"description": "Whether to fetch full page content (default: true)",
				"default":     true,
			},
		},
		"required": []string{"query"},
	}
}
