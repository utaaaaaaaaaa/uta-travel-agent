// Package agent provides the core agent implementation
package agent

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// SharedKnowledgeState is the shared state for all Researcher agents
type SharedKnowledgeState struct {
	mu sync.RWMutex

	Destination string

	// Covered topics and their coverage details
	CoveredTopics map[string]*TopicCoverage

	// All collected documents
	Documents []Document

	// Topics that need to be searched
	MissingTopics []string

	// Current researchers and their status
	Researchers map[string]*ResearcherStatus

	// When the state was last updated
	LastUpdated time.Time
}

// TopicCoverage represents the coverage of a specific topic
type TopicCoverage struct {
	Name          string
	DocumentIDs   []string
	Quality       float64 // 0-1
	DocumentCount int
	LastUpdated   time.Time
}

// Document represents a collected document
type Document struct {
	ID           string
	Title        string
	Content      string
	URL          string
	Source       string
	Topics       []string
	Quality      float64
	Summary      string
	CollectedBy  string
	CollectedAt  time.Time
}

// ResearcherStatus represents the status of a researcher agent
type ResearcherStatus struct {
	ID             string
	CurrentRound   int
	MaxRounds      int
	CurrentTopic   string
	DocumentsFound int
	Status         string // "ready", "searching", "processing", "complete"
	LastActivity   time.Time
}

// StateSummary is a read-only summary of the shared state
type StateSummary struct {
	Destination    string
	CoveredTopics  []TopicSummary
	MissingTopics  []string
	Researchers    []ResearcherSummary
	TotalDocuments int
	LastUpdated    time.Time
}

// TopicSummary is a summary of topic coverage
type TopicSummary struct {
	Name          string
	DocumentCount int
	Quality       float64
}

// ResearcherSummary is a summary of researcher status
type ResearcherSummary struct {
	ID             string
	CurrentRound   int
	MaxRounds      int
	CurrentTopic   string
	DocumentsFound int
	Status         string
}

// CreationProgress represents the progress of agent creation
type CreationProgress struct {
	Destination    string
	CoveredTopics  map[string]float64
	MissingTopics  []string
	Researchers    []ResearchProgress
	TotalDocuments int
	TotalRounds    int
	IsComplete     bool
}

// ResearchProgress represents a researcher's progress
type ResearchProgress struct {
	ID             string
	CurrentRound   int
	MaxRounds      int
	CurrentTopic   string
	DocumentsFound int
	Status         string
}

// NewSharedKnowledgeState creates a new shared state
func NewSharedKnowledgeState(destination string) *SharedKnowledgeState {
	return &SharedKnowledgeState{
		Destination:   destination,
		CoveredTopics: make(map[string]*TopicCoverage),
		Documents:     make([]Document, 0),
		MissingTopics: GetKnowledgeTopics(),
		Researchers:   make(map[string]*ResearcherStatus),
		LastUpdated:   time.Now(),
	}
}

// GetKnowledgeTopics returns all possible knowledge topics
func GetKnowledgeTopics() []string {
	return []string{
		"attractions", // 景点
		"food",        // 美食
		"history",     // 历史文化
		"transport",   // 交通
		"accommodation", // 住宿
		"entertainment", // 娱乐
		"shopping",    // 购物
		"practical",   // 实用信息
	}
}

// GetCoreTopics returns the core topics that must be covered
func GetCoreTopics() []string {
	return []string{"attractions", "food", "history", "transport"}
}

// Read returns the shared state summary
func (s *SharedKnowledgeState) Read() *StateSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	covered := make([]TopicSummary, 0)
	for _, t := range s.CoveredTopics {
		covered = append(covered, TopicSummary{
			Name:          t.Name,
			DocumentCount: t.DocumentCount,
			Quality:       t.Quality,
		})
	}

	researchers := make([]ResearcherSummary, 0)
	for _, r := range s.Researchers {
		researchers = append(researchers, ResearcherSummary{
			ID:             r.ID,
			CurrentRound:   r.CurrentRound,
			MaxRounds:      r.MaxRounds,
			CurrentTopic:   r.CurrentTopic,
			DocumentsFound: r.DocumentsFound,
			Status:         r.Status,
		})
	}

	return &StateSummary{
		Destination:    s.Destination,
		CoveredTopics:  covered,
		MissingTopics:  s.MissingTopics,
		Researchers:    researchers,
		TotalDocuments: len(s.Documents),
		LastUpdated:    s.LastUpdated,
	}
}

// Update adds a document and updates topic coverage
func (s *SharedKnowledgeState) Update(researcherID string, doc *Document, topics []string, quality float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add document
	doc.ID = uuid.New().String()
	doc.CollectedBy = researcherID
	doc.CollectedAt = time.Now()
	doc.Quality = quality
	doc.Topics = topics
	s.Documents = append(s.Documents, *doc)

	// Update topic coverage
	for _, topic := range topics {
		if _, exists := s.CoveredTopics[topic]; !exists {
			s.CoveredTopics[topic] = &TopicCoverage{
				Name:          topic,
				DocumentIDs:   []string{doc.ID},
				DocumentCount: 1,
				Quality:       quality,
				LastUpdated:   time.Now(),
			}
		} else {
			tc := s.CoveredTopics[topic]
			tc.DocumentIDs = append(tc.DocumentIDs, doc.ID)
			tc.DocumentCount++
			tc.Quality = (tc.Quality*float64(tc.DocumentCount-1) + quality) / float64(tc.DocumentCount)
			tc.LastUpdated = time.Now()
		}
	}

	s.updateMissingTopicsLocked()

	if rs, exists := s.Researchers[researcherID]; exists {
		rs.DocumentsFound++
		rs.LastActivity = time.Now()
	}

	s.LastUpdated = time.Now()
}

// RegisterResearcher registers a new researcher
func (s *SharedKnowledgeState) RegisterResearcher(id string, maxRounds int, initialTopic string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Researchers[id] = &ResearcherStatus{
		ID:           id,
		CurrentRound: 0,
		MaxRounds:    maxRounds,
		CurrentTopic: initialTopic,
		Status:       "ready",
		LastActivity: time.Now(),
	}
}

// UpdateResearcherRound updates the researcher's current round
func (s *SharedKnowledgeState) UpdateResearcherRound(id string, round int, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rs, exists := s.Researchers[id]; exists {
		rs.CurrentRound = round
		rs.Status = status
		rs.LastActivity = time.Now()
	}
}

// UpdateResearcherTopic updates the topic a researcher is searching
func (s *SharedKnowledgeState) UpdateResearcherTopic(id, topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rs, exists := s.Researchers[id]; exists {
		rs.CurrentTopic = topic
		rs.LastActivity = time.Now()
	}
}

// GetResearcherTopic returns the topic assigned to a researcher
func (s *SharedKnowledgeState) GetResearcherTopic(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if rs, exists := s.Researchers[id]; exists {
		return rs.CurrentTopic
	}
	return ""
}

// GetMissingTopic returns a topic that needs to be searched
func (s *SharedKnowledgeState) GetMissingTopic(researcherID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find missing core topics
	var missing []string
	for _, topic := range GetCoreTopics() {
		if tc, exists := s.CoveredTopics[topic]; !exists || tc.Quality < 0.6 {
			missing = append(missing, topic)
		}
	}
	if len(missing) == 0 {
		return "", false
	}

	// Find a topic not being worked on by another researcher
	for _, topic := range missing {
		alreadyWorking := false
		for _, rs := range s.Researchers {
			if rs.CurrentTopic == topic && rs.ID != researcherID {
				alreadyWorking = true
				break
			}
		}
		if !alreadyWorking {
			return topic, true
		}
	}

	// If all missing topics are being worked on, pick the first one
	return missing[0], true
}

// GetProgress returns the current progress for the frontend
func (s *SharedKnowledgeState) GetProgress() *CreationProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	covered := make(map[string]float64)
	for _, topic := range GetCoreTopics() {
		if tc, exists := s.CoveredTopics[topic]; exists {
			covered[topic] = tc.Quality
		} else {
			covered[topic] = 0
		}
	}

	researchers := make([]ResearchProgress, 0)
	for _, r := range s.Researchers {
		researchers = append(researchers, ResearchProgress{
			ID:             r.ID,
			CurrentRound:   r.CurrentRound,
			MaxRounds:      r.MaxRounds,
			CurrentTopic:   r.CurrentTopic,
			DocumentsFound: r.DocumentsFound,
			Status:         r.Status,
		})
	}

	return &CreationProgress{
		Destination:    s.Destination,
		CoveredTopics:  covered,
		MissingTopics:  s.MissingTopics,
		Researchers:    researchers,
		TotalDocuments: len(s.Documents),
		TotalRounds:    s.getTotalRounds(),
		IsComplete:     s.isCompleteLocked(),
	}
}

// AllResearchersComplete returns true if all researchers have completed
func (s *SharedKnowledgeState) AllResearchersComplete() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Researchers) == 0 {
		return false
	}

	for _, rs := range s.Researchers {
		if rs.Status != "complete" {
			return false
		}
	}
	return true
}

// GetDocuments returns all collected documents
func (s *SharedKnowledgeState) GetDocuments() []Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := make([]Document, len(s.Documents))
	copy(docs, s.Documents)
	return docs
}

// getTotalRounds returns the total number of rounds completed
func (s *SharedKnowledgeState) getTotalRounds() int {
	total := 0
	for _, rs := range s.Researchers {
		total += rs.CurrentRound
	}
	return total
}

// isCompleteLocked checks if the creation is complete
func (s *SharedKnowledgeState) isCompleteLocked() bool {
	if len(s.Researchers) == 0 {
		return false
	}

	for _, rs := range s.Researchers {
		if rs.Status != "complete" {
			return false
		}
	}
	return true
}

// updateMissingTopicsLocked updates the list of missing topics
func (s *SharedKnowledgeState) updateMissingTopicsLocked() {
	var missing []string
	for _, topic := range GetKnowledgeTopics() {
		if tc, exists := s.CoveredTopics[topic]; !exists || tc.Quality < 0.5 {
			missing = append(missing, topic)
		}
	}
	s.MissingTopics = missing
}