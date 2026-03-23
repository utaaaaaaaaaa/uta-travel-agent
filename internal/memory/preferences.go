// Package memory provides memory management for agents
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// UserPreferences stores user's travel preferences extracted from conversations
type UserPreferences struct {
	UserID               string   `json:"user_id,omitempty"`
	FavoriteDestinations []string `json:"favorite_destinations,omitempty"`
	TravelStyle          string   `json:"travel_style,omitempty"` // cultural, food, adventure, art, relaxation
	Language             string   `json:"language,omitempty"`
	DietaryRestrictions  []string `json:"dietary_restrictions,omitempty"`
	BudgetLevel          string   `json:"budget_level,omitempty"` // economy, mid-range, luxury
	PreferredActivities  []string `json:"preferred_activities,omitempty"`
	Dislikes             []string `json:"dislikes,omitempty"`
	Accessibility        []string `json:"accessibility,omitempty"`
	TravelPace           string   `json:"travel_pace,omitempty"` // slow, moderate, fast
	AccommodationStyle   string   `json:"accommodation_style,omitempty"` // hotel, hostel, airbnb, resort
	TransportPreference  string   `json:"transport_preference,omitempty"` // public, rental, walking, mixed
	LastUpdated          time.Time `json:"last_updated,omitempty"`
}

// PreferenceExtractor extracts user preferences from conversations using LLM
type PreferenceExtractor struct {
	llmProvider LLMProvider
	mu          sync.RWMutex
}

// LLMProvider interface for preference extraction
type LLMProvider interface {
	Complete(ctx context.Context, messages []LLMMessage) (*LLMResponse, error)
}

// LLMMessage represents a message for LLM
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse represents LLM response
type LLMResponse struct {
	Content string `json:"content"`
}

// NewPreferenceExtractor creates a new preference extractor
func NewPreferenceExtractor(llmProvider LLMProvider) *PreferenceExtractor {
	return &PreferenceExtractor{
		llmProvider: llmProvider,
	}
}

// ExtractPreferences analyzes a conversation and extracts user preferences
func (e *PreferenceExtractor) ExtractPreferences(ctx context.Context, conversation string) (*UserPreferences, error) {
	if e.llmProvider == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}

	prompt := buildExtractionPrompt(conversation)

	messages := []LLMMessage{
		{Role: "system", Content: extractionSystemPrompt},
		{Role: "user", Content: prompt},
	}

	response, err := e.llmProvider.Complete(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	prefs, err := parsePreferencesFromResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("parse preferences: %w", err)
	}

	prefs.LastUpdated = time.Now()
	return prefs, nil
}

// MergePreferences combines existing preferences with newly extracted ones
// New preferences take precedence for non-empty values
func MergePreferences(existing, newPrefs *UserPreferences) *UserPreferences {
	if existing == nil {
		return newPrefs
	}
	if newPrefs == nil {
		return existing
	}

	merged := &UserPreferences{
		UserID:               existing.UserID,
		FavoriteDestinations: existing.FavoriteDestinations,
		TravelStyle:          existing.TravelStyle,
		Language:             existing.Language,
		BudgetLevel:          existing.BudgetLevel,
		TravelPace:           existing.TravelPace,
		AccommodationStyle:   existing.AccommodationStyle,
		TransportPreference:  existing.TransportPreference,
		LastUpdated:          time.Now(),
	}

	// Merge slices (append unique values)
	merged.DietaryRestrictions = mergeStringSlices(existing.DietaryRestrictions, newPrefs.DietaryRestrictions)
	merged.PreferredActivities = mergeStringSlices(existing.PreferredActivities, newPrefs.PreferredActivities)
	merged.Dislikes = mergeStringSlices(existing.Dislikes, newPrefs.Dislikes)
	merged.Accessibility = mergeStringSlices(existing.Accessibility, newPrefs.Accessibility)

	// Override with new non-empty values
	if newPrefs.UserID != "" {
		merged.UserID = newPrefs.UserID
	}
	if len(newPrefs.FavoriteDestinations) > 0 {
		merged.FavoriteDestinations = mergeStringSlices(existing.FavoriteDestinations, newPrefs.FavoriteDestinations)
	}
	if newPrefs.TravelStyle != "" {
		merged.TravelStyle = newPrefs.TravelStyle
	}
	if newPrefs.Language != "" {
		merged.Language = newPrefs.Language
	}
	if newPrefs.BudgetLevel != "" {
		merged.BudgetLevel = newPrefs.BudgetLevel
	}
	if newPrefs.TravelPace != "" {
		merged.TravelPace = newPrefs.TravelPace
	}
	if newPrefs.AccommodationStyle != "" {
		merged.AccommodationStyle = newPrefs.AccommodationStyle
	}
	if newPrefs.TransportPreference != "" {
		merged.TransportPreference = newPrefs.TransportPreference
	}

	return merged
}

// FormatAsContext formats preferences as a context string for LLM
func (p *UserPreferences) FormatAsContext() string {
	if p == nil {
		return ""
	}

	var parts []string

	if p.TravelStyle != "" {
		parts = append(parts, fmt.Sprintf("- 旅行风格: %s", formatTravelStyle(p.TravelStyle)))
	}

	if p.BudgetLevel != "" {
		parts = append(parts, fmt.Sprintf("- 预算级别: %s", formatBudgetLevel(p.BudgetLevel)))
	}

	if len(p.DietaryRestrictions) > 0 {
		parts = append(parts, fmt.Sprintf("- 饮食限制: %s", strings.Join(p.DietaryRestrictions, ", ")))
	}

	if len(p.PreferredActivities) > 0 {
		parts = append(parts, fmt.Sprintf("- 喜欢的活动: %s", strings.Join(p.PreferredActivities, ", ")))
	}

	if len(p.Dislikes) > 0 {
		parts = append(parts, fmt.Sprintf("- 不喜欢: %s", strings.Join(p.Dislikes, ", ")))
	}

	if p.TravelPace != "" {
		parts = append(parts, fmt.Sprintf("- 旅行节奏: %s", formatTravelPace(p.TravelPace)))
	}

	if p.AccommodationStyle != "" {
		parts = append(parts, fmt.Sprintf("- 住宿偏好: %s", formatAccommodationStyle(p.AccommodationStyle)))
	}

	if p.TransportPreference != "" {
		parts = append(parts, fmt.Sprintf("- 交通偏好: %s", formatTransportPreference(p.TransportPreference)))
	}

	if len(p.FavoriteDestinations) > 0 {
		parts = append(parts, fmt.Sprintf("- 喜欢的目的地: %s", strings.Join(p.FavoriteDestinations, ", ")))
	}

	if len(parts) == 0 {
		return ""
	}

	return "[用户偏好]\n" + strings.Join(parts, "\n")
}

// IsEmpty returns true if no preferences are set
func (p *UserPreferences) IsEmpty() bool {
	if p == nil {
		return true
	}

	return p.TravelStyle == "" &&
		p.BudgetLevel == "" &&
		p.TravelPace == "" &&
		p.AccommodationStyle == "" &&
		p.TransportPreference == "" &&
		p.Language == "" &&
		len(p.FavoriteDestinations) == 0 &&
		len(p.DietaryRestrictions) == 0 &&
		len(p.PreferredActivities) == 0 &&
		len(p.Dislikes) == 0 &&
		len(p.Accessibility) == 0
}

// Helper functions

func buildExtractionPrompt(conversation string) string {
	return fmt.Sprintf(`请分析以下对话，提取用户的旅游偏好。

对话内容:
%s

请以 JSON 格式返回提取的偏好信息。只返回 JSON，不要有其他内容。
如果某个字段无法从对话中确定，请省略该字段。

示例输出格式:
{
  "travel_style": "cultural",
  "budget_level": "mid-range",
  "dietary_restrictions": ["不吃辣"],
  "preferred_activities": ["博物馆", "历史景点"],
  "dislikes": ["购物"]
}`, conversation)
}

const extractionSystemPrompt = `你是一个旅游偏好分析助手。你的任务是从用户的对话中提取他们的旅游偏好。

请识别以下类型的偏好:
1. 旅行风格 (travel_style): cultural(文化), food(美食), adventure(冒险), art(艺术), relaxation(休闲)
2. 预算级别 (budget_level): economy(经济), mid-range(中等), luxury(奢华)
3. 饮食限制 (dietary_restrictions): 如不吃辣、素食、过敏等
4. 喜欢的活动 (preferred_activities): 如博物馆、徒步、购物等
5. 不喜欢 (dislikes): 用户明确表示不喜欢的
6. 旅行节奏 (travel_pace): slow(慢), moderate(适中), fast(快)
7. 住宿偏好 (accommodation_style): hotel, hostel, airbnb, resort
8. 交通偏好 (transport_preference): public, rental, walking, mixed

规则:
- 只提取用户明确表达的偏好
- 如果没有明确提到，不要猜测
- 使用中文标注饮食限制和活动
- 返回标准 JSON 格式`

func parsePreferencesFromResponse(response string) (*UserPreferences, error) {
	// Try to extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return &UserPreferences{}, nil
	}

	var prefs UserPreferences
	if err := json.Unmarshal([]byte(jsonStr), &prefs); err != nil {
		return nil, fmt.Errorf("unmarshal JSON: %w", err)
	}

	return &prefs, nil
}

func extractJSON(s string) string {
	// Find JSON object in response
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

func mergeStringSlices(existing, new []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, s := range existing {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	for _, s := range new {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

func formatTravelStyle(style string) string {
	styles := map[string]string{
		"cultural":    "文化历史",
		"food":        "美食探索",
		"adventure":   "冒险户外",
		"art":         "艺术人文",
		"relaxation":  "休闲度假",
	}
	if s, ok := styles[style]; ok {
		return s
	}
	return style
}

func formatBudgetLevel(level string) string {
	levels := map[string]string{
		"economy":   "经济实惠",
		"mid-range": "中等预算",
		"luxury":    "奢华享受",
	}
	if s, ok := levels[level]; ok {
		return s
	}
	return level
}

func formatTravelPace(pace string) string {
	paces := map[string]string{
		"slow":     "慢节奏深度游",
		"moderate": "适中节奏",
		"fast":     "快节奏打卡",
	}
	if s, ok := paces[pace]; ok {
		return s
	}
	return pace
}

func formatAccommodationStyle(style string) string {
	styles := map[string]string{
		"hotel":  "酒店",
		"hostel": "青旅",
		"airbnb": "民宿",
		"resort": "度假村",
	}
	if s, ok := styles[style]; ok {
		return s
	}
	return style
}

func formatTransportPreference(pref string) string {
	prefs := map[string]string{
		"public":  "公共交通",
		"rental":  "租车自驾",
		"walking": "步行探索",
		"mixed":   "混合方式",
	}
	if s, ok := prefs[pref]; ok {
		return s
	}
	return pref
}
