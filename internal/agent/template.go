package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentTemplate defines the structure of an agent template
type AgentTemplate struct {
	Kind        string           `yaml:"kind"`
	APIVersion  string           `yaml:"apiVersion"`
	Metadata    TemplateMetadata `yaml:"metadata"`
	Spec        TemplateSpec     `yaml:"spec"`
	// Convenience fields for programmatic use
	Name        string `yaml:"-" json:"name,omitempty"`
	Description string `yaml:"-" json:"description,omitempty"`
	SystemPrompt string `yaml:"-" json:"system_prompt,omitempty"`
}

// TemplateMetadata contains metadata about the template
type TemplateMetadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

// TemplateSpec contains the template specification
type TemplateSpec struct {
	Role               string                 `yaml:"role"`
	Capabilities       []string               `yaml:"capabilities"`
	AvailableSubagents []string               `yaml:"availableSubagents,omitempty"`
	Tools              ToolsConfig            `yaml:"tools"`
	Decision           DecisionConfig         `yaml:"decision"`
	States             []string               `yaml:"states"`
	StopConditions     []StopCondition        `yaml:"stopConditions,omitempty"`
	Memory             MemoryConfig           `yaml:"memory,omitempty"`
	InputSource        *InputSourceConfig     `yaml:"inputSource,omitempty"`
	OutputFormat       OutputFormatConfig     `yaml:"outputFormat,omitempty"`
	Requires           *RequiresConfig        `yaml:"requires,omitempty"`
	IndexConfig        *IndexConfig           `yaml:"indexConfig,omitempty"`
}

// ToolsConfig defines the tools available to an agent
type ToolsConfig struct {
	MCP      []ToolReference `yaml:"mcp,omitempty"`
	Skills   []ToolReference `yaml:"skills,omitempty"`
	Services []string        `yaml:"services,omitempty"`
}

// ToolReference references a tool with optional configuration
type ToolReference struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

// DecisionConfig configures the agent's decision making
type DecisionConfig struct {
	Model         string        `yaml:"model"`
	Temperature   float64       `yaml:"temperature"`
	MaxIterations int           `yaml:"max_iterations"`
	Timeout       time.Duration `yaml:"timeout"`
}

// StopCondition defines when an agent should stop
type StopCondition struct {
	Type        string `yaml:"type"`
	Value       any    `yaml:"value,omitempty"`
	Description string `yaml:"description,omitempty"`
	CheckPrompt string `yaml:"checkPrompt,omitempty"`
}

// MemoryConfig configures the agent's memory
type MemoryConfig struct {
	Type       string   `yaml:"type"`
	MaxSize    int      `yaml:"maxSize"`
	Retention  string   `yaml:"retention"`
	Store      []string `yaml:"store"`
}

// InputSourceConfig defines where the agent gets its input
type InputSourceConfig struct {
	From   string `yaml:"from"`
	Format string `yaml:"format"`
}

// OutputFormatConfig defines the expected output format
type OutputFormatConfig struct {
	Type   string `yaml:"type"`
	Schema any    `yaml:"schema,omitempty"`
}

// RequiresConfig defines dependencies
type RequiresConfig struct {
	DestinationAgent bool `yaml:"destination_agent"`
	KnowledgeBase    bool `yaml:"knowledge_base"`
}

// IndexConfig configures indexing behavior
type IndexConfig struct {
	VectorSize      int    `yaml:"vectorSize"`
	DistanceMetric  string `yaml:"distanceMetric"`
	ChunkSize       int    `yaml:"chunkSize"`
	ChunkOverlap    int    `yaml:"chunkOverlap"`
	MinChunkSize    int    `yaml:"minChunkSize"`
}

// TemplateRegistry manages agent templates
type TemplateRegistry struct {
	templates map[AgentType]*AgentTemplate
	paths     []string // search paths for templates
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{
		templates: make(map[AgentType]*AgentTemplate),
		paths:     []string{"agent-templates"},
	}
}

// AddPath adds a search path for templates
func (r *TemplateRegistry) AddPath(path string) {
	r.paths = append(r.paths, path)
}

// Load loads a template from a file
func (r *TemplateRegistry) Load(path string) (*AgentTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	var template AgentTemplate
	if err := yaml.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &template, nil
}

// LoadAll loads all templates from search paths
func (r *TemplateRegistry) LoadAll() error {
	for _, path := range r.paths {
		files, err := filepath.Glob(filepath.Join(path, "*.yaml"))
		if err != nil {
			continue
		}

		for _, file := range files {
			template, err := r.Load(file)
			if err != nil {
				return fmt.Errorf("failed to load template %s: %w", file, err)
			}

			agentType := AgentType(template.Metadata.Name)
			r.templates[agentType] = template
		}
	}

	return nil
}

// Get retrieves a template by agent type
func (r *TemplateRegistry) Get(agentType AgentType) (*AgentTemplate, error) {
	template, ok := r.templates[agentType]
	if !ok {
		return nil, fmt.Errorf("template not found for agent type: %s", agentType)
	}
	return template, nil
}

// List returns all loaded template types
func (r *TemplateRegistry) List() []AgentType {
	types := make([]AgentType, 0, len(r.templates))
	for t := range r.templates {
		types = append(types, t)
	}
	return types
}