package commands

import (
	"context"
	"sort"
	"strings"

	"github.com/nomanqureshi/argo/internal/agent"
)

// Command represents a slash command that can be executed in the UI.
type Command struct {
	Name        string   // e.g. "clear"
	Aliases     []string // e.g. ["reset"]
	Description string   // short one-line description shown in /help
	Usage       string   // e.g. "/model [name]" - shown in detailed help
	Args        string   // "none", "optional", "required"
	Hidden      bool     // if true, not shown in /help but still works
	Execute     func(ctx *Context, args []string) Result
}

// Context provides commands with access to application state.
type Context struct {
	Ctx          context.Context
	Agent        *agent.Agent
	ThreadStore  *agent.SQLiteStore
	ModelName    string
	Provider     string
	InputTokens  int
	OutputTokens int
	ToolNames    []string // names of all registered tools, for /tools
}

// Result is what a command returns to the UI.
type Result struct {
	Output        string        // text to display
	IsError       bool          // if true, output is rendered as an error
	Quit          bool          // signals the app should quit
	ClearChat     bool          // signals the UI should clear the chat viewport
	StatusMsg     string        // if non-empty, updates the status bar
	ResumedThread *agent.Thread // if set, UI should replay this thread
	NewModel      string        // if non-empty, UI should update its model name
}

// Registry holds all registered commands.
type Registry struct {
	commands map[string]*Command // lookup by name and aliases
	ordered  []*Command          // ordered for /help display (primary entries only)
}

// NewRegistry creates a new empty command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
	}
}

// Register adds a command to the registry, indexed by its name and all aliases.
func (r *Registry) Register(cmd *Command) {
	// Store by primary name.
	r.commands[cmd.Name] = cmd
	// Store by each alias.
	for _, alias := range cmd.Aliases {
		r.commands[alias] = cmd
	}
	// Add to ordered list (primary entries only, sorted on retrieval).
	r.ordered = append(r.ordered, cmd)
}

// Get looks up a command by name or alias. Returns nil if not found.
func (r *Registry) Get(name string) *Command {
	return r.commands[name]
}

// All returns all registered commands sorted alphabetically by name.
// Only primary entries are returned (not duplicates from aliases).
func (r *Registry) All() []*Command {
	sorted := make([]*Command, len(r.ordered))
	copy(sorted, r.ordered)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

// Match parses a raw input string (e.g. "/model gpt-4o"), looks up the
// command by name or alias, and returns the matched command along with
// any remaining arguments. Returns nil if no command matches.
func (r *Registry) Match(input string) (*Command, []string) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil, nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil, nil
	}

	// Strip the leading "/" from the command name.
	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))

	cmd := r.commands[name]
	if cmd == nil {
		return nil, nil
	}

	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	return cmd, args
}
