package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AgentSpawner is implemented by the agent layer and injected into the tool.
// This interface breaks the circular dependency between the tools and agent
// packages: the tool depends only on this interface, while the agent package
// provides the concrete implementation.
type AgentSpawner interface {
	// SpawnSubAgent creates or resumes a sub-agent, sends it a message,
	// and blocks until it completes. Returns the final assistant text.
	SpawnSubAgent(ctx context.Context, sessionID string, label string, message string) (string, error)
}

// SpawnAgentTool lets the LLM spawn sub-agents for parallel task delegation.
// Each sub-agent runs in its own conversation and inherits the parent agent's
// tool permissions. The tool blocks until the sub-agent finishes and returns
// the sub-agent's final response.
type SpawnAgentTool struct {
	spawner AgentSpawner
}

type spawnAgentInput struct {
	Label     string `json:"label"`
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

// NewSpawnAgentTool creates a SpawnAgentTool backed by the given AgentSpawner.
// If spawner is nil the tool will return a configuration error on every call.
func NewSpawnAgentTool(spawner AgentSpawner) *SpawnAgentTool {
	return &SpawnAgentTool{spawner: spawner}
}

func (t *SpawnAgentTool) Name() string {
	return "spawn_agent"
}

func (t *SpawnAgentTool) Description() string {
	return strings.TrimSpace(`
Spawn a sub-agent to perform a task independently. The sub-agent runs in its
own conversation with its own context window and has access to the same set of
tools as you.

Use this tool when you need to delegate a well-scoped, independent unit of
work — for example researching a topic, editing a specific file, running a
build, or investigating a bug. You can spawn multiple sub-agents in parallel
by making several tool calls at once, which is a great way to speed up work
that can be done concurrently.

Guidelines:
- Include ALL relevant context in the message. The sub-agent has NO access to
  your conversation history; it only sees the message you provide.
- Give the sub-agent a clear, specific objective. Vague prompts lead to vague
  results.
- Use "session_id" to send follow-up messages to an existing sub-agent instead
  of creating a new one. The sub-agent retains its full conversation history
  across calls.
- Avoid spawning sub-agents for trivial tasks you can accomplish directly with
  a single tool call — the overhead is not worth it.
- Each sub-agent inherits your permission level, so it can perform the same
  operations you can.
`)
}

func (t *SpawnAgentTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"label": map[string]any{
				"type":        "string",
				"description": "Short label displayed while the agent runs (e.g., 'Researching alternatives')",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "The prompt for the agent. Include all relevant context needed for the task.",
			},
			"session_id": map[string]any{
				"type":        "string",
				"description": "Optional session ID to resume an existing sub-agent conversation instead of creating a new one.",
			},
		},
		"required": []string{"label", "message"},
	}
}

// Permission returns PermissionRead because spawning a sub-agent is inherently
// safe — the sub-agent itself will request permission for any dangerous
// operations it performs through its own tool calls.
func (t *SpawnAgentTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *SpawnAgentTool) Execute(ctx context.Context, input string) (*Result, error) {
	if t.spawner == nil {
		return &Result{
			Error:   "sub-agent spawning is not configured",
			IsError: true,
		}, nil
	}

	var params spawnAgentInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Error:   fmt.Sprintf("failed to parse input: %s", err),
			IsError: true,
		}, nil
	}

	if params.Label == "" {
		return &Result{
			Error:   "label is required",
			IsError: true,
		}, nil
	}

	if params.Message == "" {
		return &Result{
			Error:   "message is required",
			IsError: true,
		}, nil
	}

	output, err := t.spawner.SpawnSubAgent(ctx, params.SessionID, params.Label, params.Message)
	if err != nil {
		return &Result{
			Error:   fmt.Sprintf("sub-agent error: %s", err),
			IsError: true,
		}, nil
	}

	return &Result{
		Output:  output,
		IsError: false,
	}, nil
}
