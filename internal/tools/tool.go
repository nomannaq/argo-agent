package tools

import "context"

// PermissionLevel indicates how dangerous a tool operation is.
type PermissionLevel int

const (
	// PermissionRead is for safe read-only operations.
	PermissionRead PermissionLevel = iota
	// PermissionWrite is for operations that modify files.
	PermissionWrite
	// PermissionDangerous is for shell commands, network access, etc.
	PermissionDangerous
)

// String returns a human-readable name for the permission level.
func (p PermissionLevel) String() string {
	switch p {
	case PermissionRead:
		return "read"
	case PermissionWrite:
		return "write"
	case PermissionDangerous:
		return "dangerous"
	default:
		return "unknown"
	}
}

// Tool is the interface all tools must implement.
type Tool interface {
	// Name returns the unique identifier for this tool.
	Name() string
	// Description returns a human-readable description of what this tool does.
	Description() string
	// InputSchema returns the JSON Schema for the tool's parameters.
	InputSchema() map[string]any
	// Permission returns the permission level required to use this tool.
	Permission() PermissionLevel
	// Execute runs the tool with the given raw JSON input and returns a result.
	Execute(ctx context.Context, input string) (*Result, error)
}

// Result represents the output of a tool execution.
type Result struct {
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
	IsError bool   `json:"is_error"`
}
