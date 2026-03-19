// Package skills provides the skill execution engine for UTA Travel Agent.
// Skills are document-based capabilities defined in SKILL.md files.
package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Skill represents a parsed skill definition
type Skill struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  []SkillParameter       `json:"parameters"`
	Execution   *ExecutionFlow         `json:"execution,omitempty"`
	Dependencies *SkillDependencies    `json:"dependencies,omitempty"`
	OutputSchema map[string]any        `json:"output_schema,omitempty"`
	Examples    []SkillExample         `json:"examples,omitempty"`
	Notes       []string               `json:"notes,omitempty"`
	FilePath    string                 `json:"file_path"`
}

// SkillParameter defines a skill parameter
type SkillParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Default     any    `json:"default,omitempty"`
}

// ExecutionFlow defines the execution steps
type ExecutionFlow struct {
	Steps []ExecutionStep `json:"steps"`
}

// ExecutionStep defines a single execution step
type ExecutionStep struct {
	Type        string         `json:"type"` // tool, skill, service, llm
	Name        string         `json:"name"`
	Params      map[string]any `json:"params,omitempty"`
	Description string         `json:"description,omitempty"`
}

// SkillDependencies defines skill dependencies
type SkillDependencies struct {
	MCPTtools []string `json:"mcp_tools,omitempty"`
	Skills    []string `json:"skills,omitempty"`
	Services  []string `json:"services,omitempty"`
}

// SkillExample defines a skill example
type SkillExample struct {
	Input  map[string]any `json:"input,omitempty"`
	Output map[string]any `json:"output,omitempty"`
}

// SkillResult represents the result of skill execution
type SkillResult struct {
	Success bool        `json:"success"`
	Data    any         `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ToolExecutor interface for executing tools
type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, params map[string]any) (*SkillResult, error)
}

// ServiceCaller interface for calling services
type ServiceCaller interface {
	Call(ctx context.Context, serviceName string, method string, params map[string]any) (*SkillResult, error)
}

// LLMCaller interface for LLM calls
type LLMCaller interface {
	Complete(ctx context.Context, prompt string, params map[string]any) (string, error)
}

// Engine manages skill loading and execution
type Engine struct {
	skills       map[string]*Skill
	registryPath string
	toolExec     ToolExecutor
	serviceCall  ServiceCaller
	llmCall      LLMCaller
}

// NewEngine creates a new skill engine
func NewEngine(registryPath string) *Engine {
	return &Engine{
		skills:       make(map[string]*Skill),
		registryPath: registryPath,
	}
}

// SetToolExecutor sets the tool executor
func (e *Engine) SetToolExecutor(exec ToolExecutor) {
	e.toolExec = exec
}

// SetServiceCaller sets the service caller
func (e *Engine) SetServiceCaller(caller ServiceCaller) {
	e.serviceCall = caller
}

// SetLLMCaller sets the LLM caller
func (e *Engine) SetLLMCaller(caller LLMCaller) {
	e.llmCall = caller
}

// LoadSkill loads a skill from a SKILL.md file
func (e *Engine) LoadSkill(skillPath string) error {
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return fmt.Errorf("failed to read skill file: %w", err)
	}

	skill, err := ParseSkillMarkdown(string(data))
	if err != nil {
		return fmt.Errorf("failed to parse skill: %w", err)
	}

	skill.FilePath = skillPath
	e.skills[skill.Name] = skill

	return nil
}

// LoadAll loads all skills from the registry path
func (e *Engine) LoadAll() error {
	return filepath.Walk(e.registryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "SKILL.md" {
			if err := e.LoadSkill(path); err != nil {
				return fmt.Errorf("failed to load skill %s: %w", path, err)
			}
		}

		return nil
	})
}

// Get retrieves a skill by name
func (e *Engine) Get(name string) (*Skill, bool) {
	skill, exists := e.skills[name]
	return skill, exists
}

// List returns all loaded skills
func (e *Engine) List() []*Skill {
	skills := make([]*Skill, 0, len(e.skills))
	for _, skill := range e.skills {
		skills = append(skills, skill)
	}
	return skills
}

// Execute runs a skill with the given parameters
func (e *Engine) Execute(ctx context.Context, skillName string, params map[string]any) (*SkillResult, error) {
	skill, exists := e.skills[skillName]
	if !exists {
		return nil, fmt.Errorf("skill %s not found", skillName)
	}

	// Validate required parameters
	for _, param := range skill.Parameters {
		if param.Required {
			if _, ok := params[param.Name]; !ok {
				return nil, fmt.Errorf("required parameter %s is missing", param.Name)
			}
		}
	}

	// If skill has execution flow, execute it
	if skill.Execution != nil && len(skill.Execution.Steps) > 0 {
		return e.executeFlow(ctx, skill, params)
	}

	// Otherwise, use LLM to interpret the skill
	return e.executeWithLLM(ctx, skill, params)
}

// executeFlow executes the defined execution flow
func (e *Engine) executeFlow(ctx context.Context, skill *Skill, params map[string]any) (*SkillResult, error) {
	var lastResult any

	for _, step := range skill.Execution.Steps {
		// Merge step params with input params
		mergedParams := make(map[string]any)
		for k, v := range step.Params {
			mergedParams[k] = v
		}
		for k, v := range params {
			mergedParams[k] = v
		}

		var result *SkillResult
		var err error

		switch step.Type {
		case "tool":
			if e.toolExec == nil {
				return nil, fmt.Errorf("tool executor not configured")
			}
			result, err = e.toolExec.Execute(ctx, step.Name, mergedParams)

		case "skill":
			result, err = e.Execute(ctx, step.Name, mergedParams)

		case "service":
			if e.serviceCall == nil {
				return nil, fmt.Errorf("service caller not configured")
			}
			result, err = e.serviceCall.Call(ctx, step.Name, "execute", mergedParams)

		case "llm":
			if e.llmCall == nil {
				return nil, fmt.Errorf("LLM caller not configured")
			}
			output, err := e.llmCall.Complete(ctx, step.Description, mergedParams)
			if err != nil {
				return nil, err
			}
			result = &SkillResult{Success: true, Data: output}

		default:
			return nil, fmt.Errorf("unknown step type: %s", step.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("step %s failed: %w", step.Name, err)
		}

		lastResult = result.Data
	}

	return &SkillResult{
		Success: true,
		Data:    lastResult,
	}, nil
}

// executeWithLLM executes the skill using LLM interpretation
func (e *Engine) executeWithLLM(ctx context.Context, skill *Skill, params map[string]any) (*SkillResult, error) {
	if e.llmCall == nil {
		return nil, fmt.Errorf("LLM caller not configured for skill execution")
	}

	// Build prompt from skill definition
	prompt := buildSkillPrompt(skill, params)

	output, err := e.llmCall.Complete(ctx, prompt, params)
	if err != nil {
		return nil, err
	}

	return &SkillResult{
		Success: true,
		Data:    output,
	}, nil
}

// ParseSkillMarkdown parses a SKILL.md file into a Skill struct
func ParseSkillMarkdown(content string) (*Skill, error) {
	skill := &Skill{
		Parameters: []SkillParameter{},
		Notes:      []string{},
		Examples:   []SkillExample{},
	}

	// Extract name from first heading
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			skill.Name = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	// Extract description
	descRegex := regexp.MustCompile(`(?s)## Description\s*\n(.+?)(?:\n##|$)`)
	if match := descRegex.FindStringSubmatch(content); len(match) > 1 {
		skill.Description = strings.TrimSpace(match[1])
	}

	// Extract parameters table
	skill.Parameters = parseParametersTable(content)

	// Extract dependencies
	skill.Dependencies = parseDependencies(content)

	// Extract notes
	notesRegex := regexp.MustCompile(`(?s)## Notes\s*\n(.+?)(?:\n##|$)`)
	if match := notesRegex.FindStringSubmatch(content); len(match) > 1 {
		notes := strings.Split(strings.TrimSpace(match[1]), "\n")
		for _, note := range notes {
			note = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(note), "- "))
			if note != "" {
				skill.Notes = append(skill.Notes, note)
			}
		}
	}

	return skill, nil
}

// parseParametersTable extracts parameters from markdown table
func parseParametersTable(content string) []SkillParameter {
	var params []SkillParameter

	// Find parameters section
	paramRegex := regexp.MustCompile(`(?s)## Parameters\s*\n\n\|.+\|\n\|[-|\s]+\|\n((?:\|.+\|\n?)+)`)
	match := paramRegex.FindStringSubmatch(content)
	if len(match) < 2 {
		return params
	}

	tableContent := match[1]
	rowRegex := regexp.MustCompile(`\|\s*` + "`?([^`|\n]+)`?" + `\s*\|\s*(\w+)\s*\|\s*([✓✗])\s*\|\s*(.+)`)

	rows := rowRegex.FindAllStringSubmatch(tableContent, -1)
	for _, row := range rows {
		param := SkillParameter{
			Name:        strings.TrimSpace(row[1]),
			Type:        strings.TrimSpace(row[2]),
			Required:    row[3] == "✓",
			Description: strings.TrimSpace(row[4]),
		}
		params = append(params, param)
	}

	return params
}

// parseDependencies extracts dependencies from content
func parseDependencies(content string) *SkillDependencies {
	deps := &SkillDependencies{}

	// Find dependencies section
	depsRegex := regexp.MustCompile(`(?s)## Dependencies\s*\n(.+?)(?:\n##|$)`)
	match := depsRegex.FindStringSubmatch(content)
	if len(match) < 2 {
		return deps
	}

	depsContent := match[1]

	// Parse MCP Tools
	mcpRegex := regexp.MustCompile(`(?s)### MCP Tools\s*\n(.+?)(?:\n###|$)`)
	if mcpMatch := mcpRegex.FindStringSubmatch(depsContent); len(mcpMatch) > 1 {
		deps.MCPTtools = parseListItems(mcpMatch[1])
	}

	// Parse Skills
	skillsRegex := regexp.MustCompile(`(?s)### Skills\s*\n(.+?)(?:\n###|$)`)
	if skillsMatch := skillsRegex.FindStringSubmatch(depsContent); len(skillsMatch) > 1 {
		deps.Skills = parseListItems(skillsMatch[1])
	}

	// Parse Services
	servicesRegex := regexp.MustCompile(`(?s)### Services\s*\n(.+?)(?:\n###|$)`)
	if servicesMatch := servicesRegex.FindStringSubmatch(depsContent); len(servicesMatch) > 1 {
		deps.Services = parseListItems(servicesMatch[1])
	}

	return deps
}

// parseListItems extracts list items from markdown
func parseListItems(content string) []string {
	var items []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(line, "- ")
			item = strings.TrimPrefix(item, "* ")
			item = strings.TrimPrefix(strings.TrimSpace(item), "`")
			item = strings.TrimSuffix(item, "`")
			if item != "" {
				items = append(items, item)
			}
		}
	}
	return items
}

// buildSkillPrompt builds a prompt for LLM execution
func buildSkillPrompt(skill *Skill, params map[string]any) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("# Task: Execute %s Skill\n\n", skill.Name))
	prompt.WriteString(fmt.Sprintf("## Description\n%s\n\n", skill.Description))

	if len(skill.Parameters) > 0 {
		prompt.WriteString("## Parameters\n")
		for _, param := range skill.Parameters {
			value := params[param.Name]
			prompt.WriteString(fmt.Sprintf("- %s (%s): %v\n", param.Name, param.Type, value))
		}
		prompt.WriteString("\n")
	}

	if len(skill.Notes) > 0 {
		prompt.WriteString("## Notes\n")
		for _, note := range skill.Notes {
			prompt.WriteString(fmt.Sprintf("- %s\n", note))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("## Output\n")
	prompt.WriteString("Please execute this skill and provide the result.")

	return prompt.String()
}
