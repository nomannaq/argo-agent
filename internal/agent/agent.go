package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nomanqureshi/argo/internal/llm"
	"github.com/nomanqureshi/argo/internal/permissions"
	"github.com/nomanqureshi/argo/internal/prompt"
	"github.com/nomanqureshi/argo/internal/tools"
)

const (
	// maxRetryAttempts is the maximum number of times to retry a failed LLM call.
	maxRetryAttempts = 3
	// baseRetryDelay is the starting delay for exponential backoff.
	baseRetryDelay = 5 * time.Second
)

// Config holds the configuration for an Agent instance.
type Config struct {
	Model        string
	Provider     string
	APIKey       string
	SystemPrompt string
	MaxTokens    int
}

// Agent orchestrates the conversation loop between the user, the LLM, and tools.
type Agent struct {
	config      Config
	provider    llm.Provider
	toolReg     *tools.Registry
	permHandler permissions.Handler
	thread      *Thread
	store       ThreadStore // optional — when non-nil, threads are auto-saved
	onEvent     func(Event)

	// subagent tracking
	subagentsMu sync.Mutex
	subagents   map[string]*Agent // keyed by session ID for resumption
}

// Event types sent to the UI
type EventType int

const (
	EventAssistantText     EventType = iota
	EventToolCallStart                // a tool call has been received
	EventToolResult                   // a tool finished executing
	EventError                        // an error occurred
	EventDone                         // the agent loop completed
	EventPermissionRequest            // waiting for user permission
	EventUsageUpdate                  // token usage update
	EventSubagentStart                // a sub-agent has been spawned
	EventSubagentEnd                  // a sub-agent has finished
)

// Event carries information from the agent goroutine to the UI.
type Event struct {
	Type       EventType
	Content    string
	ToolName   string
	ToolInput  string
	ToolResult *tools.Result
	Usage      *llm.Usage
	Error      error
}

// New creates a new Agent. If no system prompt is configured, one is built
// automatically from the registered tool names and project context.
func New(cfg Config, provider llm.Provider, toolReg *tools.Registry, permHandler permissions.Handler) *Agent {
	if cfg.SystemPrompt == "" {
		toolList := toolReg.List()
		names := make([]string, len(toolList))
		for i, t := range toolList {
			names[i] = t.Name()
		}
		cfg.SystemPrompt = prompt.BuildSystemPrompt(names)
	}

	return &Agent{
		config:      cfg,
		provider:    provider,
		toolReg:     toolReg,
		permHandler: permHandler,
		thread:      NewThread(),
		subagents:   make(map[string]*Agent),
	}
}

// SetEventHandler registers a callback that receives agent events (used by the UI).
func (a *Agent) SetEventHandler(handler func(Event)) {
	a.onEvent = handler
}

// SetStore attaches a ThreadStore for automatic thread persistence.
// When set, threads are saved after each completed Run invocation.
func (a *Agent) SetStore(store ThreadStore) {
	a.store = store
}

// SetSystemPrompt updates the agent's system prompt. This is useful when
// the prompt needs to be rebuilt after all tools have been registered.
func (a *Agent) SetSystemPrompt(prompt string) {
	a.config.SystemPrompt = prompt
}

// SetModel updates the model used for LLM requests.
func (a *Agent) SetModel(model string) {
	a.config.Model = model
}

func (a *Agent) emit(evt Event) {
	if a.onEvent != nil {
		a.onEvent(evt)
	}
}

// Thread returns the agent's current conversation thread.
func (a *Agent) Thread() *Thread {
	return a.thread
}

// SetThread replaces the current thread (used for session resume).
func (a *Agent) SetThread(t *Thread) {
	a.thread = t
}

// ResetThread replaces the current thread with a fresh one,
// effectively clearing the conversation history.
func (a *Agent) ResetThread() {
	a.thread = NewThread()
}

// autoSave persists the current thread to the store if one is configured.
// Errors are emitted as events but do not interrupt the agent flow.
func (a *Agent) autoSave() {
	if a.store == nil {
		return
	}
	if err := a.store.SaveThread(a.thread); err != nil {
		a.emit(Event{
			Type:    EventError,
			Content: fmt.Sprintf("failed to save thread: %v", err),
			Error:   err,
		})
	}
}

// Run processes a user message through the agent loop.
// It sends the message to the LLM, processes tool calls, and iterates until done.
// Failed LLM calls are retried with exponential backoff for transient errors.
// Cumulative token usage across the entire Run invocation is tracked and emitted
// as a final EventUsageUpdate when the loop completes.
func (a *Agent) Run(ctx context.Context, userMessage string) error {
	// 1. Add user message to thread
	a.thread.AddMessage(llm.Message{
		Role:    llm.RoleUser,
		Content: userMessage,
	})

	// Track total token usage across all LLM calls in this Run invocation.
	totalUsage := &llm.Usage{}

	// 2. Agentic loop
	for {
		select {
		case <-ctx.Done():
			a.autoSave()
			return ctx.Err()
		default:
		}

		// Build request
		req := &llm.Request{
			Model:        a.config.Model,
			SystemPrompt: a.config.SystemPrompt,
			Messages:     a.thread.Messages(),
			Tools:        a.toolReg.ToLLMDefinitions(),
			MaxTokens:    a.config.MaxTokens,
		}
		if req.MaxTokens == 0 {
			req.MaxTokens = 16384
		}

		// Stream response from LLM with retry logic
		stream, err := a.streamWithRetry(ctx, req)
		if err != nil {
			a.emit(Event{Type: EventError, Error: err})
			a.autoSave()
			return err
		}

		// Collect the full response
		var assistantMsg llm.Message
		assistantMsg.Role = llm.RoleAssistant
		var currentToolCalls []llm.ToolCall
		var textContent string
		var streamErr error

		for evt := range stream {
			switch evt.Type {
			case llm.EventText:
				textContent += evt.Content
				a.emit(Event{Type: EventAssistantText, Content: evt.Content})
			case llm.EventToolCallComplete:
				if evt.ToolCall != nil {
					currentToolCalls = append(currentToolCalls, *evt.ToolCall)
					a.emit(Event{
						Type:      EventToolCallStart,
						ToolName:  evt.ToolCall.Name,
						ToolInput: evt.ToolCall.Arguments,
					})
				}
			case llm.EventDone:
				if evt.Usage != nil {
					totalUsage.InputTokens += evt.Usage.InputTokens
					totalUsage.OutputTokens += evt.Usage.OutputTokens
					// Emit per-turn usage as well
					a.emit(Event{Type: EventUsageUpdate, Usage: evt.Usage})
				}
			case llm.EventError:
				streamErr = evt.Error
			}
		}

		// If we got a stream-level error, surface it directly.
		if streamErr != nil {
			a.emit(Event{Type: EventError, Error: streamErr})
			a.autoSave()
			return streamErr
		}

		assistantMsg.Content = textContent
		assistantMsg.ToolCalls = currentToolCalls
		a.thread.AddMessage(assistantMsg)

		// If no tool calls, we're done
		if len(currentToolCalls) == 0 {
			// Emit cumulative usage for the entire Run
			a.emit(Event{
				Type:  EventUsageUpdate,
				Usage: totalUsage,
			})
			a.emit(Event{Type: EventDone})
			a.autoSave()
			return nil
		}

		// 3. Execute tool calls
		for _, tc := range currentToolCalls {
			tool, ok := a.toolReg.Get(tc.Name)
			if !ok {
				// Unknown tool — send error back
				a.thread.AddMessage(llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Content:    "Error: unknown tool " + tc.Name,
				})
				continue
			}

			// Check permissions
			allowed, err := a.permHandler.CheckPermission(tool.Name(), tool.Permission(), tc.Arguments)
			if err != nil {
				return err
			}
			if !allowed {
				a.thread.AddMessage(llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: tc.ID,
					Content:    "Error: permission denied for tool " + tc.Name,
				})
				a.emit(Event{Type: EventToolResult, ToolName: tc.Name, ToolResult: &tools.Result{Output: "Permission denied", IsError: true}})
				continue
			}

			// Execute tool
			result, err := tool.Execute(ctx, tc.Arguments)
			if err != nil {
				result = &tools.Result{Output: err.Error(), IsError: true, Error: err.Error()}
			}

			a.emit(Event{Type: EventToolResult, ToolName: tc.Name, ToolResult: result})

			content := result.Output
			if result.IsError {
				content = "Error: " + result.Error
			}
			a.thread.AddMessage(llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: tc.ID,
				Content:    content,
			})
		}
		// Loop back to send tool results to LLM
	}
}

// ---------------------------------------------------------------------------
// Sub-agent spawning (implements tools.AgentSpawner)
// ---------------------------------------------------------------------------

// SpawnSubAgent creates or resumes a sub-agent, sends it a message, and blocks
// until it completes. Returns the final assistant text response.
//
// This method is called by the SpawnAgentTool and implements tools.AgentSpawner.
//
// Architecture (inspired by Zed):
//   - Sub-agents get their own Thread, tools, and system prompt
//   - Depth is limited to MaxSubagentDepth (prevents recursive spawning)
//   - Sub-agents at max depth do NOT get the spawn_agent tool
//   - The parent blocks until the child finishes (fire-and-wait)
//   - Only the final assistant text is returned (no structured data)
//   - Session ID enables multi-turn follow-ups with the same child
func (a *Agent) SpawnSubAgent(ctx context.Context, sessionID string, label string, message string) (string, error) {
	currentDepth := a.thread.Depth()

	// Depth guard
	if currentDepth >= MaxSubagentDepth {
		return "", fmt.Errorf("cannot spawn sub-agent: maximum depth (%d) reached", MaxSubagentDepth)
	}

	a.emit(Event{
		Type:    EventSubagentStart,
		Content: label,
	})

	a.subagentsMu.Lock()

	var child *Agent
	var isResume bool
	if sessionID != "" {
		child = a.subagents[sessionID]
		if child != nil {
			isResume = true
		}
	}

	if child == nil {
		// Create a new sub-agent with its own thread and tool registry.
		childThread := NewSubagentThread(a.thread.ID, currentDepth)

		// Build a child tool registry. Copy all tools from parent EXCEPT
		// spawn_agent if the child would be at max depth.
		childToolReg := tools.NewRegistry()
		for _, t := range a.toolReg.List() {
			if t.Name() == "spawn_agent" && (currentDepth+1) >= MaxSubagentDepth {
				continue // don't give spawn_agent to max-depth children
			}
			childToolReg.Register(t)
		}

		// Build the child system prompt with its available tools.
		childToolNames := childToolReg.ToolNames()
		childSystemPrompt := prompt.BuildSystemPrompt(childToolNames)

		child = &Agent{
			config: Config{
				Model:        a.config.Model,
				Provider:     a.config.Provider,
				APIKey:       a.config.APIKey,
				SystemPrompt: childSystemPrompt,
				MaxTokens:    a.config.MaxTokens,
			},
			provider:    a.provider,
			toolReg:     childToolReg,
			permHandler: a.permHandler,
			thread:      childThread,
			store:       a.store, // share the store so child threads are persisted
			subagents:   make(map[string]*Agent),
		}

		// Determine the key under which this child is stored.
		// If the caller provided a sessionID (even though no child existed
		// yet), use that as the key so follow-up calls with the same
		// sessionID will find the child. Otherwise key on the new thread ID.
		storeKey := childThread.ID
		if sessionID != "" {
			storeKey = sessionID
		}
		a.subagents[storeKey] = child
		sessionID = storeKey
	}

	_ = isResume // reserved for future logging

	a.subagentsMu.Unlock()

	// Run the sub-agent synchronously (blocks until done).
	// The child's events are NOT forwarded to the parent's UI — the parent
	// only gets the final text result. This keeps the UX clean.
	err := child.Run(ctx, message)
	if err != nil {
		a.emit(Event{
			Type:    EventSubagentEnd,
			Content: label,
		})
		return "", fmt.Errorf("sub-agent %q failed: %w", label, err)
	}

	a.emit(Event{
		Type:    EventSubagentEnd,
		Content: label,
	})

	// Extract the last assistant text from the child thread.
	result := child.thread.LastAssistantText()
	if result == "" {
		result = "(sub-agent produced no output)"
	}

	// Prepend the session ID so the LLM can use it for follow-up calls.
	result = fmt.Sprintf("[session_id: %s]\n\n%s", sessionID, result)

	return result, nil
}

// ---------------------------------------------------------------------------
// Retry logic
// ---------------------------------------------------------------------------

// streamWithRetry wraps provider.StreamMessage with exponential backoff retry
// logic for transient / rate-limit errors.
func (a *Agent) streamWithRetry(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetryAttempts; attempt++ {
		stream, err := a.provider.StreamMessage(ctx, req)
		if err == nil {
			return stream, nil
		}

		lastErr = err

		// If this is not a retryable error, bail out immediately.
		if !isRetryableError(err) {
			return nil, err
		}

		// If we've exhausted all retry attempts, give up.
		if attempt == maxRetryAttempts {
			break
		}

		// Calculate backoff delay: baseRetryDelay * 2^attempt
		delay := baseRetryDelay * time.Duration(1<<uint(attempt))

		a.emit(Event{
			Type:    EventError,
			Content: fmt.Sprintf("Retrying in %s... (attempt %d/%d)", delay, attempt+1, maxRetryAttempts),
			Error:   err,
		})

		// Wait with context awareness so we can be cancelled.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt.
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetryAttempts, lastErr)
}

// isRetryableError returns true if the error looks like a transient failure
// that is worth retrying (rate limits, server overload, temporary outages).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	retryablePatterns := []string{
		"429",                   // HTTP 429 Too Many Requests
		"500",                   // HTTP 500 Internal Server Error
		"503",                   // HTTP 503 Service Unavailable
		"529",                   // HTTP 529 Overloaded (Anthropic)
		"rate limit",            // Generic rate limit message
		"rate_limit",            // Underscore variant
		"too many requests",     // Human-readable 429
		"overloaded",            // Anthropic overloaded
		"temporarily",           // "temporarily unavailable" etc.
		"internal server error", // Generic 500 message
		"service unavailable",   // Generic 503 message
		"server error",          // Broad server error
		"capacity",              // "at capacity" messages
		"timeout",               // Request timeouts
		"connection reset",      // Network-level transient errors
		"eof",                   // Unexpected EOF from server
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	return false
}
