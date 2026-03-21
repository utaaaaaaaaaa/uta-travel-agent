// Package skills provides Claude skill management with progressive disclosure
//
// Skills are Markdown files with YAML frontmatter that provide instructions to LLMs.
// Unlike tools (which execute code), skills provide declarative knowledge.
//
// Progressive Disclosure:
// - Tier 1: Frontmatter only (~100 tokens) - always loaded
// - Tier 2: Partial content (~500 tokens) - loaded when potentially relevant
// - Tier 3: Full content (~5000+ tokens) - loaded when actively invoked
package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Skill represents a Claude skill with progressive disclosure support
type Skill struct {
	// Tier 1: Always loaded (~100 tokens)
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Tier 2/3: Loaded on demand
	Content     string `yaml:"-"` // Full markdown content (after frontmatter)
	Partial     string `yaml:"-"` // Partial content for Tier 2
	FilePath    string `yaml:"-"` // Source file path

	// State
	loadedTier int `yaml:"-"` // Current loaded tier (1, 2, or 3)

	// Computed
	contentHash string `yaml:"-"` // SHA256 hash of content for caching
}

// Tier constants for progressive disclosure
const (
	Tier1 = 1 // Frontmatter only (~100 tokens)
	Tier2 = 2 // Partial content (~500 tokens)
	Tier3 = 3 // Full content (~5000+ tokens)
)

// LoadTier loads content up to the specified tier
func (s *Skill) LoadTier(tier int) string {
	switch tier {
	case Tier1:
		return s.GetTier1Content()
	case Tier2:
		if s.loadedTier < Tier2 {
			s.loadPartial()
		}
		return s.GetTier2Content()
	case Tier3:
		if s.loadedTier < Tier3 {
			s.loadFull()
		}
		return s.GetTier3Content()
	}
	return ""
}

// GetTier1Content returns Tier 1 content (frontmatter summary)
func (s *Skill) GetTier1Content() string {
	return fmt.Sprintf("Skill: %s\n%s", s.Name, s.Description)
}

// GetTier2Content returns Tier 2 content (partial)
func (s *Skill) GetTier2Content() string {
	if s.Partial != "" {
		return s.Partial
	}
	return s.GetTier1Content()
}

// GetTier3Content returns Tier 3 content (full)
func (s *Skill) GetTier3Content() string {
	if s.Content != "" {
		return s.Content
	}
	return s.GetTier2Content()
}

// loadPartial loads Tier 2 content (first ~500 chars)
func (s *Skill) loadPartial() {
	if s.Content == "" {
		if err := s.readContent(); err != nil {
			return
		}
	}

	// Extract first 500 characters or first section
	maxPartial := 500
	if len(s.Content) <= maxPartial {
		s.Partial = s.Content
	} else {
		// Try to cut at a paragraph boundary
		cut := strings.LastIndex(s.Content[:maxPartial], "\n\n")
		if cut > maxPartial/2 {
			s.Partial = s.Content[:cut]
		} else {
			s.Partial = s.Content[:maxPartial] + "..."
		}
	}
	s.loadedTier = Tier2
}

// loadFull ensures full content is loaded
func (s *Skill) loadFull() {
	if s.Content == "" {
		s.readContent()
	}
	s.loadedTier = Tier3
}

// readContent reads the skill file and parses it
func (s *Skill) readContent() error {
	if s.FilePath == "" {
		return fmt.Errorf("no file path set for skill %s", s.Name)
	}

	data, err := os.ReadFile(s.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read skill file %s: %w", s.FilePath, err)
	}

	// Parse frontmatter and extract content
	content, err := parseSkillFile(string(data), s)
	if err != nil {
		return err
	}

	s.Content = content
	s.computeHash()
	s.loadedTier = Tier3

	return nil
}

// computeHash computes SHA256 hash of content
func (s *Skill) computeHash() {
	hash := sha256.Sum256([]byte(s.Content))
	s.contentHash = hex.EncodeToString(hash[:])
}

// parseSkillFile parses a SKILL.md file and returns the content
func parseSkillFile(data string, skill *Skill) (string, error) {
	// Match YAML frontmatter
	re := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n(.*)$`)
	matches := re.FindStringSubmatch(data)

	if len(matches) != 3 {
		return "", fmt.Errorf("invalid skill file format: missing frontmatter")
	}

	// Parse YAML frontmatter
	frontmatter := matches[1]
	content := strings.TrimSpace(matches[2])

	// Parse frontmatter into skill
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return "", fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	if skill.Name == "" {
		skill.Name = fm.Name
	}
	if skill.Description == "" {
		skill.Description = fm.Description
	}

	return content, nil
}

// Registry manages all skills with progressive disclosure
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill // name -> skill

	// Skill directories to scan
	dirs []string

	// Embedding service for semantic matching (optional)
	embedder Embedder
}

// Embedder interface for semantic skill matching
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// NewRegistry creates a new skill registry
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
		dirs:   []string{},
	}
}

// AddDir adds a directory to scan for skills
func (r *Registry) AddDir(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirs = append(r.dirs, dir)
}

// SetEmbedder sets the embedding service for semantic matching
func (r *Registry) SetEmbedder(e Embedder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.embedder = e
}

// LoadSkills scans all directories and loads skill metadata (Tier 1 only)
func (r *Registry) LoadSkills() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, dir := range r.dirs {
		if err := r.scanDirectory(dir); err != nil {
			return fmt.Errorf("failed to scan directory %s: %w", dir, err)
		}
	}

	return nil
}

// scanDirectory scans a directory for SKILL.md files
func (r *Registry) scanDirectory(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, skip
	}

	// Look for SKILL.md in the directory itself
	skillFile := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil {
		skill, err := r.loadSkillFile(skillFile)
		if err != nil {
			return err
		}
		if skill.Name != "" {
			r.skills[skill.Name] = skill
		}
	}

	// Look for subdirectories with SKILL.md
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // Can't read directory, skip
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			skillFile := filepath.Join(subDir, "SKILL.md")
			if _, err := os.Stat(skillFile); err == nil {
				skill, err := r.loadSkillFile(skillFile)
				if err != nil {
					return err
				}
				if skill.Name != "" {
					r.skills[skill.Name] = skill
				}
			}
		}
	}

	return nil
}

// loadSkillFile loads a SKILL.md file (Tier 1 only - just frontmatter)
func (r *Registry) loadSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter only for Tier 1
	re := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---`)
	matches := re.FindStringSubmatch(string(data))

	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid skill file %s: missing frontmatter", path)
	}

	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(matches[1]), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter in %s: %w", path, err)
	}

	return &Skill{
		Name:        fm.Name,
		Description: fm.Description,
		FilePath:    path,
		loadedTier:  Tier1,
	}, nil
}

// Get retrieves a skill by name
func (r *Registry) Get(name string) *Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills[name]
}

// List returns all skills (Tier 1 info only)
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	return skills
}

// ListTier1 returns Tier 1 content for all skills (for context-efficient listing)
func (r *Registry) ListTier1() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s.GetTier1Content())
	}
	return result
}

// MatchSkills finds relevant skills based on query
func (r *Registry) MatchSkills(query string, topK int) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	queryLower := strings.ToLower(query)

	type scored struct {
		skill *Skill
		score float64
	}

	var candidates []scored

	// Keyword matching
	for _, skill := range r.skills {
		score := r.calculateRelevance(queryLower, skill)
		if score > 0.2 { // Lower threshold for Chinese
			candidates = append(candidates, scored{skill, score})
		}
	}

	// Sort by score (bubble sort for simplicity)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Return top K
	result := make([]*Skill, 0, topK)
	for i := 0; i < topK && i < len(candidates); i++ {
		result = append(result, candidates[i].skill)
	}

	return result
}

// calculateRelevance calculates relevance score between query and skill
func (r *Registry) calculateRelevance(queryLower string, skill *Skill) float64 {
	nameLower := strings.ToLower(skill.Name)
	descLower := strings.ToLower(skill.Description)

	score := 0.0

	// Exact name match (highest weight)
	if strings.Contains(queryLower, nameLower) || strings.Contains(nameLower, queryLower) {
		score += 0.9
	}

	// Partial name match (for hyphenated names like "travel-planner")
	nameParts := strings.Split(nameLower, "-")
	for _, part := range nameParts {
		if len(part) > 1 && strings.Contains(queryLower, part) {
			score += 0.5
		}
	}

	// Description keyword match - check for common words
	// Chinese text matching: check if any 2+ character substring matches
	queryRunes := []rune(queryLower)
	descRunes := []rune(descLower)

	// Check for character overlaps
	matchedChars := 0
	for _, qr := range queryRunes {
		for _, dr := range descRunes {
			if qr == dr {
				matchedChars++
				break
			}
		}
	}

	// Normalize by query length
	if len(queryRunes) > 0 {
		charScore := float64(matchedChars) / float64(len(queryRunes))
		score += charScore * 0.5
	}

	// Also check for phrase matches (2-4 character phrases)
	for i := 0; i < len(queryRunes)-1; i++ {
		for j := i + 2; j <= len(queryRunes) && j <= i+5; j++ {
			phrase := string(queryRunes[i:j])
			if strings.Contains(descLower, phrase) {
				score += 0.2 * float64(j-i) // Longer phrases score higher
			}
		}
	}

	return score
}

// MatchSkillsSemantic finds relevant skills using embeddings
func (r *Registry) MatchSkillsSemantic(ctx context.Context, query string, topK int) ([]*Skill, error) {
	r.mu.RLock()
	embedder := r.embedder
	skills := r.skills
	r.mu.RUnlock()

	if embedder == nil {
		// Fallback to keyword matching
		return r.MatchSkills(query, topK), nil
	}

	// Get query embedding
	queryVec, err := embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	type scored struct {
		skill *Skill
		score float64
	}

	var candidates []scored

	// Match against skill descriptions
	for _, skill := range skills {
		// Use description + name for matching
		text := skill.Name + " " + skill.Description
		skillVec, err := embedder.Embed(ctx, text)
		if err != nil {
			continue
		}

		score := cosineSimilarity(queryVec, skillVec)
		if score > 0.5 {
			candidates = append(candidates, scored{skill, score})
		}
	}

	// Sort by score
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Return top K
	result := make([]*Skill, 0, topK)
	for i := 0; i < topK && i < len(candidates); i++ {
		result = append(result, candidates[i].skill)
	}

	return result, nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt computes square root using Newton's method
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// GetSkillInstructions gets instructions from matching skills
// This is the main entry point for agents
func (r *Registry) GetSkillInstructions(ctx context.Context, query string, maxTokens int) (string, error) {
	// Find relevant skills
	skills := r.MatchSkills(query, 5)

	if len(skills) == 0 {
		return "", nil
	}

	var instructions strings.Builder
	currentTokens := 0

	for _, skill := range skills {
		// Load Tier 2 for confirmation
		content := skill.LoadTier(Tier2)

		// Estimate tokens (rough: 4 chars per token)
		tokens := len(content) / 4

		if currentTokens+tokens > maxTokens {
			// If we can fit Tier 1, do that
			tier1 := skill.GetTier1Content()
			tier1Tokens := len(tier1) / 4
			if currentTokens+tier1Tokens <= maxTokens {
				instructions.WriteString("\n---\n")
				instructions.WriteString(tier1)
				currentTokens += tier1Tokens
			}
			continue
		}

		instructions.WriteString("\n---\n")
		instructions.WriteString(content)
		currentTokens += tokens
	}

	return instructions.String(), nil
}
