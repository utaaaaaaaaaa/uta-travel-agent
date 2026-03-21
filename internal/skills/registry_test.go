package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkill_LoadTier(t *testing.T) {
	// Create a temporary skill file
	tmpDir := t.TempDir()
	skillPath := filepath.Join(tmpDir, "SKILL.md")

	content := `---
name: test-skill
description: A test skill for unit testing
---

# Test Skill

This is the full content of the test skill.

## Section 1
Some content here.

## Section 2
More content here that makes the skill longer.
`

	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create skill file: %v", err)
	}

	// Load skill from file (Tier 1)
	registry := NewRegistry()
	registry.AddDir(tmpDir)
	if err := registry.LoadSkills(); err != nil {
		t.Fatalf("Failed to load skills: %v", err)
	}

	skill := registry.Get("test-skill")
	if skill == nil {
		t.Fatal("test-skill not found")
	}

	// Test Tier 1 (should have frontmatter only)
	tier1 := skill.GetTier1Content()
	if tier1 == "" {
		t.Error("Tier 1 content should not be empty")
	}
	if !containsAll(tier1, "test-skill", "test skill") {
		t.Errorf("Tier 1 should contain name and description, got: %s", tier1)
	}

	// Test Tier 2 (partial content)
	tier2 := skill.LoadTier(Tier2)
	if tier2 == "" {
		t.Error("Tier 2 content should not be empty")
	}

	// Test Tier 3 (full content)
	tier3 := skill.LoadTier(Tier3)
	if tier3 == "" {
		t.Error("Tier 3 content should not be empty")
	}
	if !containsAll(tier3, "Test Skill", "Section 1", "Section 2") {
		t.Errorf("Tier 3 should contain full content, got: %s", tier3)
	}
}

func TestRegistry_LoadSkills(t *testing.T) {
	// Create temporary skill directories
	tmpDir := t.TempDir()

	// Skill 1: Direct SKILL.md
	skill1Content := `---
name: direct-skill
description: A skill in the root directory
---

# Direct Skill
Content here.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte(skill1Content), 0644); err != nil {
		t.Fatalf("Failed to create skill 1: %v", err)
	}

	// Skill 2: Subdirectory SKILL.md
	subDir := filepath.Join(tmpDir, "sub-skill")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	skill2Content := `---
name: sub-skill
description: A skill in a subdirectory
---

# Sub Skill
More content here.
`
	if err := os.WriteFile(filepath.Join(subDir, "SKILL.md"), []byte(skill2Content), 0644); err != nil {
		t.Fatalf("Failed to create skill 2: %v", err)
	}

	// Create registry and load skills
	registry := NewRegistry()
	registry.AddDir(tmpDir)

	if err := registry.LoadSkills(); err != nil {
		t.Fatalf("Failed to load skills: %v", err)
	}

	// Check skills were loaded
	skills := registry.List()
	if len(skills) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(skills))
	}

	// Check individual skills
	skill1 := registry.Get("direct-skill")
	if skill1 == nil {
		t.Error("direct-skill not found")
	} else if skill1.Description != "A skill in the root directory" {
		t.Errorf("Wrong description for direct-skill: %s", skill1.Description)
	}

	skill2 := registry.Get("sub-skill")
	if skill2 == nil {
		t.Error("sub-skill not found")
	} else if skill2.Description != "A skill in a subdirectory" {
		t.Errorf("Wrong description for sub-skill: %s", skill2.Description)
	}
}

func TestRegistry_MatchSkills(t *testing.T) {
	registry := NewRegistry()

	// Add test skills manually with realistic descriptions
	registry.mu.Lock()
	registry.skills["travel-planner"] = &Skill{
		Name:        "travel-planner",
		Description: "规划旅游行程，包括景点推荐、路线优化",
	}
	registry.skills["real-time-search"] = &Skill{
		Name:        "real-time-search",
		Description: "执行实时网络搜索，获取最新信息如价格、天气",
	}
	registry.skills["destination-research"] = &Skill{
		Name:        "destination-research",
		Description: "研究旅游目的地信息，包括景点、美食、文化",
	}
	registry.mu.Unlock()

	tests := []struct {
		query          string
		expectedSkills []string // Any of these should match
	}{
		{"帮我规划行程", []string{"travel-planner"}},
		{"今天天气", []string{"real-time-search"}},
		{"研究一下景点", []string{"destination-research", "travel-planner"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			matches := registry.MatchSkills(tt.query, 3)
			if len(matches) == 0 {
				t.Errorf("No skills matched for query: %s", tt.query)
				return
			}

			// Check if any expected skill is in top matches
			found := false
			for _, s := range matches {
				for _, expected := range tt.expectedSkills {
					if s.Name == expected {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Errorf("None of expected skills %v in top matches for query: %s", tt.expectedSkills, tt.query)
				t.Errorf("Got skills: %v", skillNames(matches))
			}
		})
	}
}

func TestRegistry_ListTier1(t *testing.T) {
	registry := NewRegistry()

	registry.mu.Lock()
	registry.skills["skill1"] = &Skill{
		Name:        "skill1",
		Description: "First skill",
	}
	registry.skills["skill2"] = &Skill{
		Name:        "skill2",
		Description: "Second skill",
	}
	registry.mu.Unlock()

	list := registry.ListTier1()
	if len(list) != 2 {
		t.Errorf("Expected 2 Tier 1 entries, got %d", len(list))
	}

	for _, entry := range list {
		if !containsAll(entry, "Skill:", "skill") {
			t.Errorf("Unexpected Tier 1 format: %s", entry)
		}
	}
}

func TestParseSkillFile(t *testing.T) {
	content := `---
name: parsed-skill
description: This skill was parsed
---

# Parsed Skill

This is the content after frontmatter.
`

	skill := &Skill{}
	result, err := parseSkillFile(content, skill)
	if err != nil {
		t.Fatalf("Failed to parse skill file: %v", err)
	}

	if skill.Name != "parsed-skill" {
		t.Errorf("Expected name 'parsed-skill', got '%s'", skill.Name)
	}

	if skill.Description != "This skill was parsed" {
		t.Errorf("Expected description 'This skill was parsed', got '%s'", skill.Description)
	}

	if !containsAll(result, "Parsed Skill", "content after frontmatter") {
		t.Errorf("Unexpected content: %s", result)
	}
}

// Helper functions

func containsAll(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if !contains(s, substr) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func skillNames(skills []*Skill) []string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return names
}
