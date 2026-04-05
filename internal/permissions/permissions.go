package permissions

import "github.com/nomanqureshi/argo/internal/tools"

// Handler decides whether a tool invocation should be allowed.
type Handler interface {
	// CheckPermission returns true if the tool call is allowed.
	// For interactive mode, this may prompt the user via a callback.
	CheckPermission(toolName string, level tools.PermissionLevel, input string) (bool, error)
}

// AutoApproveHandler always approves tool calls (useful for testing or trusted mode).
type AutoApproveHandler struct{}

func (h *AutoApproveHandler) CheckPermission(toolName string, level tools.PermissionLevel, input string) (bool, error) {
	return true, nil
}

// InteractiveHandler prompts the user for permission on write/dangerous operations.
// The PromptFunc is called when permission is needed; it should return true to allow.
type InteractiveHandler struct {
	PromptFunc func(toolName string, level tools.PermissionLevel, input string) (bool, error)
	// AutoApproveRead auto-approves read-only tool calls
	AutoApproveRead bool
}

func NewInteractiveHandler(promptFunc func(string, tools.PermissionLevel, string) (bool, error)) *InteractiveHandler {
	return &InteractiveHandler{
		PromptFunc:      promptFunc,
		AutoApproveRead: true,
	}
}

func (h *InteractiveHandler) CheckPermission(toolName string, level tools.PermissionLevel, input string) (bool, error) {
	if h.AutoApproveRead && level == tools.PermissionRead {
		return true, nil
	}
	return h.PromptFunc(toolName, level, input)
}
