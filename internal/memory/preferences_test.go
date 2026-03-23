package memory

import (
	"testing"
	"time"
)

func TestUserPreferencesFormatAsContext(t *testing.T) {
	tests := []struct {
		name     string
		prefs    *UserPreferences
		expected string
	}{
		{
			name:     "nil preferences",
			prefs:    nil,
			expected: "",
		},
		{
			name:     "empty preferences",
			prefs:    &UserPreferences{},
			expected: "",
		},
		{
			name: "full preferences",
			prefs: &UserPreferences{
				TravelStyle:         "cultural",
				BudgetLevel:         "mid-range",
				DietaryRestrictions: []string{"不吃辣", "素食"},
				PreferredActivities: []string{"博物馆", "历史景点"},
				Dislikes:            []string{"购物"},
				TravelPace:          "moderate",
			},
			expected: "[用户偏好]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.prefs.FormatAsContext()
			if tt.expected == "" && result != "" {
				t.Errorf("expected empty, got %q", result)
			}
			if tt.expected != "" && result == "" {
				t.Errorf("expected non-empty, got empty")
			}
			if tt.expected != "" && result != "" {
				if !contains(result, tt.expected) {
					t.Errorf("expected to contain %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestUserPreferencesIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		prefs    *UserPreferences
		expected bool
	}{
		{
			name:     "nil preferences",
			prefs:    nil,
			expected: true,
		},
		{
			name:     "empty preferences",
			prefs:    &UserPreferences{},
			expected: true,
		},
		{
			name:     "with travel style",
			prefs:    &UserPreferences{TravelStyle: "cultural"},
			expected: false,
		},
		{
			name:     "with dietary restrictions",
			prefs:    &UserPreferences{DietaryRestrictions: []string{"素食"}},
			expected: false,
		},
		{
			name:     "with budget level",
			prefs:    &UserPreferences{BudgetLevel: "mid-range"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.prefs.IsEmpty()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMergePreferences(t *testing.T) {
	tests := []struct {
		name     string
		existing *UserPreferences
		newPrefs *UserPreferences
		expected *UserPreferences
	}{
		{
			name:     "nil existing",
			existing: nil,
			newPrefs: &UserPreferences{TravelStyle: "cultural"},
			expected: &UserPreferences{TravelStyle: "cultural"},
		},
		{
			name:     "nil new",
			existing: &UserPreferences{TravelStyle: "cultural"},
			newPrefs: nil,
			expected: &UserPreferences{TravelStyle: "cultural"},
		},
		{
			name:     "override travel style",
			existing: &UserPreferences{TravelStyle: "cultural", BudgetLevel: "economy"},
			newPrefs: &UserPreferences{TravelStyle: "food"},
			expected: &UserPreferences{TravelStyle: "food", BudgetLevel: "economy"},
		},
		{
			name:     "merge slices",
			existing: &UserPreferences{DietaryRestrictions: []string{"不吃辣"}},
			newPrefs: &UserPreferences{DietaryRestrictions: []string{"素食"}},
			expected: &UserPreferences{DietaryRestrictions: []string{"不吃辣", "素食"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergePreferences(tt.existing, tt.newPrefs)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected non-nil result")
				return
			}

			if tt.expected.TravelStyle != "" && result.TravelStyle != tt.expected.TravelStyle {
				t.Errorf("TravelStyle: expected %q, got %q", tt.expected.TravelStyle, result.TravelStyle)
			}

			if tt.expected.BudgetLevel != "" && result.BudgetLevel != tt.expected.BudgetLevel {
				t.Errorf("BudgetLevel: expected %q, got %q", tt.expected.BudgetLevel, result.BudgetLevel)
			}

			if len(tt.expected.DietaryRestrictions) > 0 {
				if len(result.DietaryRestrictions) != len(tt.expected.DietaryRestrictions) {
					t.Errorf("DietaryRestrictions: expected %d items, got %d", len(tt.expected.DietaryRestrictions), len(result.DietaryRestrictions))
				}
			}
		})
	}
}

func TestPersistentMemoryPreferences(t *testing.T) {
	mem := NewPersistentMemory(nil, 100)

	// Test recalling when no preferences exist
	prefs, err := mem.RecallPreferences()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if prefs != nil {
		t.Errorf("expected nil preferences, got %+v", prefs)
	}

	// Test saving preferences
	testPrefs := &UserPreferences{
		TravelStyle:         "cultural",
		BudgetLevel:         "mid-range",
		DietaryRestrictions: []string{"不吃辣"},
		LastUpdated:         time.Now(),
	}

	err = mem.RememberPreferences(testPrefs)
	if err != nil {
		t.Errorf("failed to save preferences: %v", err)
	}

	// Test recalling saved preferences
	prefs, err = mem.RecallPreferences()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if prefs == nil {
		t.Errorf("expected preferences, got nil")
		return
	}

	if prefs.TravelStyle != testPrefs.TravelStyle {
		t.Errorf("TravelStyle: expected %q, got %q", testPrefs.TravelStyle, prefs.TravelStyle)
	}
	if prefs.BudgetLevel != testPrefs.BudgetLevel {
		t.Errorf("BudgetLevel: expected %q, got %q", testPrefs.BudgetLevel, prefs.BudgetLevel)
	}

	// Test updating preferences
	updatedPrefs := &UserPreferences{
		TravelStyle: "food",
		BudgetLevel: "luxury",
	}
	err = mem.RememberPreferences(updatedPrefs)
	if err != nil {
		t.Errorf("failed to update preferences: %v", err)
	}

	prefs, err = mem.RecallPreferences()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if prefs.TravelStyle != "food" {
		t.Errorf("expected TravelStyle %q, got %q", "food", prefs.TravelStyle)
	}
}

func TestGetAllLongTermByKeyPrefix(t *testing.T) {
	mem := NewPersistentMemory(nil, 100)

	// Add some items to long-term memory
	mem.Remember("destination:hangzhou", "杭州")
	mem.Remember("destination:beijing", "北京")
	mem.Remember("preference:food", "喜欢辣")

	// Test prefix search
	destinations := mem.GetAllLongTermByKeyPrefix("destination:")
	if len(destinations) != 2 {
		t.Errorf("expected 2 destinations, got %d", len(destinations))
	}

	preferences := mem.GetAllLongTermByKeyPrefix("preference:")
	if len(preferences) != 1 {
		t.Errorf("expected 1 preference, got %d", len(preferences))
	}

	// Test non-matching prefix
	nonexistent := mem.GetAllLongTermByKeyPrefix("nonexistent:")
	if len(nonexistent) != 0 {
		t.Errorf("expected 0 items, got %d", len(nonexistent))
	}
}

func TestRememberDestination(t *testing.T) {
	mem := NewPersistentMemory(nil, 100)

	// Test saving destination
	err := mem.RememberDestination("杭州")
	if err != nil {
		t.Errorf("failed to save destination: %v", err)
	}

	// Test duplicate save (should not error)
	err = mem.RememberDestination("杭州")
	if err != nil {
		t.Errorf("unexpected error on duplicate: %v", err)
	}

	// Test retrieving destinations
	destinations := mem.GetVisitedDestinations()
	if len(destinations) != 1 {
		t.Errorf("expected 1 destination, got %d", len(destinations))
	}
	if len(destinations) > 0 && destinations[0] != "杭州" {
		t.Errorf("expected %q, got %q", "杭州", destinations[0])
	}

	// Add another destination
	mem.RememberDestination("北京")
	destinations = mem.GetVisitedDestinations()
	if len(destinations) != 2 {
		t.Errorf("expected 2 destinations, got %d", len(destinations))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
