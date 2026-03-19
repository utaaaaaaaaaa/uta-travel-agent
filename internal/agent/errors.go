package agent

import "errors"

// Error definitions for agent package
var (
	ErrAgentNotFound    = errors.New("agent not found")
	ErrAgentExists      = errors.New("agent already exists")
	ErrInvalidAgentID   = errors.New("invalid agent ID")
	ErrInvalidAgentData = errors.New("invalid agent data")
)
