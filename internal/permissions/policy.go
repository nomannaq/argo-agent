package permissions

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/nomanqureshi/argo/internal/tools"
)

// Decision represents the outcome of a policy rule evaluation.
type Decision int

const (
	// Allow permits the tool invocation without prompting.
	Allow Decision = iota
	// Deny blocks the tool invocation silently.
	Deny
	// Ask defers to the interactive prompt for a decision.
	Ask
)

// String returns a human-readable name for the decision.
func (d Decision) String() string {
	switch d {
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	case Ask:
		return "ask"
	default:
		return "unknown"
	}
}

// PolicyRule defines a single permission policy entry. Rules are evaluated
// in order and the first matching rule wins (Zed-inspired first-match semantics).
type PolicyRule struct {
	// ToolPattern is a glob pattern matched against tool names.
	// Examples: "*", "shell", "edit_*", "read_*"
	ToolPattern string

	// InputPattern is an optional regex pattern matched against the tool input.
	// If empty, the rule matches any input for the given tool pattern.
	InputPattern string

	// Decision is the action to take when this rule matches.
	Decision Decision
}

// matches reports whether the rule matches the given tool name and input.
func (r *PolicyRule) matches(toolName string, input string) bool {
	matched, err := filepath.Match(r.ToolPattern, toolName)
	if err != nil || !matched {
		return false
	}

	if r.InputPattern == "" {
		return true
	}

	re, err := regexp.Compile(r.InputPattern)
	if err != nil {
		return false
	}
	return re.MatchString(input)
}

// dangerousShellPatterns contains regex patterns for commands that should never
// be executed. These are compiled once and reused for every check.
var dangerousShellPatterns = []string{
	// Recursive force-remove of root or home
	`rm\s+-[^\s]*r[^\s]*f[^\s]*\s+/\s*$`,
	`rm\s+-[^\s]*f[^\s]*r[^\s]*\s+/\s*$`,
	`rm\s+-[^\s]*r[^\s]*f[^\s]*\s+~/?\s*$`,
	`rm\s+-[^\s]*f[^\s]*r[^\s]*\s+~/?\s*$`,

	// Fork bomb
	`:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`,

	// Filesystem formatting
	`mkfs\b`,

	// Raw disk writes
	`dd\s+if=`,

	// Redirect to block device
	`>\s*/dev/sd[a-z]`,
	`>\s*/dev/nvme`,
	`>\s*/dev/vd[a-z]`,

	// Dangerous recursive chmod on root
	`chmod\s+-[^\s]*R[^\s]*\s+777\s+/\s*$`,
	`chmod\s+777\s+-[^\s]*R[^\s]*\s+/\s*$`,

	// Overwriting boot sector / MBR
	`dd\s+.*of=/dev/[snv]`,
}

// dangerousPathPrefixes lists system directories that should never be written to.
var dangerousPathPrefixes = []string{
	"/etc/",
	"/usr/",
	"/bin/",
	"/sbin/",
	"/boot/",
	"/etc",
	"/usr",
	"/bin",
	"/sbin",
	"/boot",
}

// compiledDangerousPatterns holds the pre-compiled regexes for dangerous commands.
var compiledDangerousPatterns []*regexp.Regexp

func init() {
	compiledDangerousPatterns = make([]*regexp.Regexp, 0, len(dangerousShellPatterns))
	for _, p := range dangerousShellPatterns {
		compiledDangerousPatterns = append(compiledDangerousPatterns, regexp.MustCompile(p))
	}
}

// DefaultSecurityRules returns the hardcoded safety rules that are always
// evaluated before any user-defined policy. These cannot be overridden.
func DefaultSecurityRules() []PolicyRule {
	rules := make([]PolicyRule, 0, len(dangerousShellPatterns)+len(dangerousPathPrefixes))

	// Deny dangerous shell commands.
	for _, pattern := range dangerousShellPatterns {
		rules = append(rules, PolicyRule{
			ToolPattern:  "*",
			InputPattern: pattern,
			Decision:     Deny,
		})
	}

	// Deny writes to system directories.
	for _, prefix := range dangerousPathPrefixes {
		escaped := regexp.QuoteMeta(prefix)
		rules = append(rules, PolicyRule{
			ToolPattern:  "*",
			InputPattern: escaped,
			Decision:     Deny,
		})
	}

	return rules
}

// PolicyHandler implements Handler with a layered, rule-based permission system
// inspired by Zed's settings model. Permission checks follow this order:
//
//  1. Hardcoded security rules (always deny dangerous patterns)
//  2. Session decisions (remembered "always allow"/"always deny" for a tool)
//  3. User-defined policy rules (first match wins)
//  4. Default behavior based on permission level
//
// For the "ask" path it delegates to an embedded InteractiveHandler.
type PolicyHandler struct {
	// interactive is the fallback handler used when a decision is "Ask".
	interactive *InteractiveHandler

	// mu protects rules and sessionDecisions from concurrent access.
	mu sync.RWMutex

	// rules is the ordered list of user-defined policy rules.
	rules []PolicyRule

	// sessionDecisions remembers per-tool decisions for the lifetime of a session.
	// Keyed by tool name; value is Allow or Deny.
	sessionDecisions map[string]Decision
}

// NewPolicyHandler creates a PolicyHandler that delegates interactive prompts
// to the given InteractiveHandler.
func NewPolicyHandler(interactive *InteractiveHandler) *PolicyHandler {
	return &PolicyHandler{
		interactive:      interactive,
		rules:            make([]PolicyRule, 0),
		sessionDecisions: make(map[string]Decision),
	}
}

// AddRule appends a user-defined policy rule. Rules are evaluated in the order
// they are added, and the first matching rule wins.
func (p *PolicyHandler) AddRule(rule PolicyRule) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = append(p.rules, rule)
}

// RememberDecision stores a session-scoped decision for a specific tool name.
// Subsequent calls to CheckPermission for that tool will use this decision
// instead of prompting (unless a hardcoded security rule takes precedence).
func (p *PolicyHandler) RememberDecision(toolName string, decision Decision) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessionDecisions[toolName] = decision
}

// ClearSessionDecisions removes all remembered decisions, useful when
// resetting a session.
func (p *PolicyHandler) ClearSessionDecisions() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessionDecisions = make(map[string]Decision)
}

// CheckPermission evaluates whether the given tool invocation should proceed.
// The evaluation order is:
//
//  1. Hardcoded security rules – always checked first, cannot be bypassed.
//  2. Session decisions – previously remembered allow/deny for this tool.
//  3. User-defined policy rules – first matching rule wins.
//  4. Default behavior – read→allow, write/dangerous→ask via interactive handler.
func (p *PolicyHandler) CheckPermission(toolName string, level tools.PermissionLevel, input string) (bool, error) {
	// ---------------------------------------------------------------
	// Step 1: Hardcoded security rules (always deny dangerous input).
	// ---------------------------------------------------------------
	if denied, reason := p.checkSecurityRules(input); denied {
		return false, fmt.Errorf("blocked by security policy: %s", reason)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// ---------------------------------------------------------------
	// Step 2: Session-scoped remembered decisions.
	// ---------------------------------------------------------------
	if decision, ok := p.sessionDecisions[toolName]; ok {
		switch decision {
		case Allow:
			return true, nil
		case Deny:
			return false, fmt.Errorf("denied by session policy for tool %q", toolName)
		// Ask falls through to the interactive handler below.
		}
	}

	// ---------------------------------------------------------------
	// Step 3: User-defined policy rules (first match wins).
	// ---------------------------------------------------------------
	for i := range p.rules {
		rule := &p.rules[i]
		if rule.matches(toolName, input) {
			switch rule.Decision {
			case Allow:
				return true, nil
			case Deny:
				return false, fmt.Errorf("denied by policy rule (tool=%q, pattern=%q)",
					toolName, rule.ToolPattern)
			case Ask:
				return p.interactive.CheckPermission(toolName, level, input)
			}
		}
	}

	// ---------------------------------------------------------------
	// Step 4: Default behaviour based on permission level.
	// ---------------------------------------------------------------
	switch level {
	case tools.PermissionRead:
		return true, nil
	case tools.PermissionWrite, tools.PermissionDangerous:
		return p.interactive.CheckPermission(toolName, level, input)
	default:
		return p.interactive.CheckPermission(toolName, level, input)
	}
}

// checkSecurityRules tests the input against all hardcoded dangerous patterns.
// It returns true (and a reason) if the input should be blocked.
func (p *PolicyHandler) checkSecurityRules(input string) (bool, string) {
	normalized := strings.TrimSpace(input)

	// Check dangerous command patterns.
	for _, re := range compiledDangerousPatterns {
		if re.MatchString(normalized) {
			return true, fmt.Sprintf("input matches dangerous pattern %q", re.String())
		}
	}

	// Check for writes targeting system directories.
	for _, prefix := range dangerousPathPrefixes {
		if strings.Contains(normalized, prefix) {
			// Only flag if it looks like a write operation — we inspect the raw
			// input for path references combined with write-like keywords.
			lower := strings.ToLower(normalized)
			if containsWriteIntent(lower) {
				return true, fmt.Sprintf("write operation targeting system directory %q", prefix)
			}
		}
	}

	return false, ""
}

// containsWriteIntent performs a lightweight heuristic check for write-like
// actions in an input string. It is intentionally conservative so that read
// operations referencing system paths are not blocked.
func containsWriteIntent(input string) bool {
	writeIndicators := []string{
		"write", "create", "delete", "remove", "mv ", "move",
		"cp ", "copy", "chmod", "chown", "chgrp",
		"truncate", "append", "overwrite", "> ", ">>",
		"edit", "modify", "patch", "install", "uninstall",
		"mkdir", "rmdir", "ln ", "link",
	}
	for _, w := range writeIndicators {
		if strings.Contains(input, w) {
			return true
		}
	}
	return false
}
