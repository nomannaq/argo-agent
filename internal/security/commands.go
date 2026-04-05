package security

import (
	"fmt"
	"regexp"
	"strings"
)

// ShellSubstitutionPatterns detects shell variable expansion and command substitution
// patterns that could be used to exfiltrate environment variables or run hidden commands.
// Inspired by Zed's approach but applied proactively.
var shellSubstitutionPatterns = []*regexp.Regexp{
	// $VAR, ${VAR} - variable expansion
	regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*`),
	regexp.MustCompile(`\$\{[^}]+\}`),

	// $0-$9, $?, $$, $@, $* - special parameters
	regexp.MustCompile(`\$[0-9?$@*#!-]`),

	// $(command) - command substitution
	regexp.MustCompile(`\$\(`),

	// `command` - backtick command substitution
	regexp.MustCompile("`"),

	// $((expr)) - arithmetic expansion
	regexp.MustCompile(`\$\(\(`),

	// <(cmd), >(cmd) - process substitution
	regexp.MustCompile(`[<>]\(`),
}

// ExfiltrationCommands are commands that can send data to external servers.
// These require explicit confirmation even if the user has auto-approved shell commands.
var exfiltrationCommands = []string{
	"curl",
	"wget",
	"nc",
	"ncat",
	"netcat",
	"ssh",
	"scp",
	"rsync",
	"ftp",
	"sftp",
	"telnet",
	"socat",
	"nmap",
	"dig",      // DNS exfiltration
	"nslookup", // DNS exfiltration
	"base64",   // often used in exfiltration chains
}

// CommandValidation contains the result of validating a command.
type CommandValidation struct {
	// IsValid is true if the command passes all security checks.
	IsValid bool

	// Blocked is true if the command should be unconditionally blocked.
	Blocked bool

	// RequiresConfirmation is true if the command needs explicit user approval.
	RequiresConfirmation bool

	// Reason explains why the command was blocked or flagged.
	Reason string

	// Violations lists all security violations found.
	Violations []string
}

// ValidateCommand performs comprehensive security validation on a shell command.
// It checks for:
// 1. Shell substitutions that could leak env vars
// 2. Known dangerous patterns (rm -rf /, etc.)
// 3. Exfiltration commands that could send data externally
// 4. Chained commands where any sub-command is problematic
func ValidateCommand(command string) CommandValidation {
	result := CommandValidation{IsValid: true}

	// Check dangerous patterns against the full command first, before splitting.
	// This catches patterns like "curl ... | bash" and fork bombs that span
	// shell operators and would be fragmented by splitCommand.
	if violation := checkDangerousPatterns(command); violation != "" {
		result.Violations = append(result.Violations, violation)
		result.Blocked = true
	}

	// Also check each segment between && / || / ; separators, preserving
	// pipes within each segment so pipe-to-shell patterns are still caught.
	chainedParts := splitChainedCommand(command)
	for _, part := range chainedParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if violation := checkDangerousPatterns(part); violation != "" {
			result.Violations = append(result.Violations, violation)
			result.Blocked = true
		}
	}

	// Split by common command separators to analyze each sub-command
	subCommands := splitCommand(command)

	for _, subCmd := range subCommands {
		subCmd = strings.TrimSpace(subCmd)
		if subCmd == "" {
			continue
		}

		// Check for shell substitutions
		if violation := checkShellSubstitutions(subCmd); violation != "" {
			result.Violations = append(result.Violations, violation)
			result.RequiresConfirmation = true
		}

		// Check for dangerous patterns
		if violation := checkDangerousPatterns(subCmd); violation != "" {
			result.Violations = append(result.Violations, violation)
			result.Blocked = true
		}

		// Check for exfiltration commands
		if violation := checkExfiltrationCommand(subCmd); violation != "" {
			result.Violations = append(result.Violations, violation)
			result.RequiresConfirmation = true
		}

		// Check for attempts to read sensitive files
		if violation := checkSensitiveFileAccess(subCmd); violation != "" {
			result.Violations = append(result.Violations, violation)
			result.RequiresConfirmation = true
		}
	}

	if result.Blocked {
		result.IsValid = false
		result.Reason = fmt.Sprintf("command blocked by security policy: %s", strings.Join(result.Violations, "; "))
	} else if result.RequiresConfirmation {
		result.Reason = fmt.Sprintf("command requires confirmation: %s", strings.Join(result.Violations, "; "))
	}

	return result
}

// splitChainedCommand splits a command string by chaining operators (&&, ||, ;, newline)
// but preserves pipes within each segment. This allows dangerous-pattern checks to see
// full pipe chains like "curl ... | bash".
func splitChainedCommand(command string) []string {
	separators := regexp.MustCompile(`\s*(?:&&|\|\||;|\n)\s*`)
	parts := separators.Split(command, -1)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// splitCommand splits a command string by shell operators (&&, ||, ;, |, newline).
func splitCommand(command string) []string {
	// Split on common command separators
	separators := regexp.MustCompile(`\s*(?:&&|\|\||;|\n)\s*`)
	parts := separators.Split(command, -1)

	// Also handle pipe chains - but we want to analyze the whole pipe
	var result []string
	for _, part := range parts {
		// For pipes, each command in the pipe should be checked
		pipeParts := strings.Split(part, "|")
		for _, p := range pipeParts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
	}

	return result
}

// checkShellSubstitutions checks for shell variable expansion and command substitution.
func checkShellSubstitutions(command string) string {
	// Allow environment variable assignments at the start of a command
	// (e.g., "PAGER=less git log" is OK, "echo $HOME" is not)
	stripped := stripEnvPrefixes(command)

	for _, pattern := range shellSubstitutionPatterns {
		if pattern.MatchString(stripped) {
			return fmt.Sprintf("contains shell substitution pattern %q which could leak secrets", pattern.String())
		}
	}
	return ""
}

// stripEnvPrefixes removes leading VAR=value assignments from a command.
// "FOO=bar BAZ=qux git log" → "git log"
func stripEnvPrefixes(command string) string {
	envAssignment := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=[^\s]*\s+`)
	for envAssignment.MatchString(command) {
		command = envAssignment.ReplaceAllString(command, "")
	}
	return command
}

// dangerousCommandPatterns checks for commands that are unconditionally dangerous.
var dangerousCommandPatterns = []*regexp.Regexp{
	// rm -rf / variants (any flag ordering)
	regexp.MustCompile(`\brm\s+(-[^\s]*r[^\s]*f|-[^\s]*f[^\s]*r)[^\s]*\s+/\s*$`),
	regexp.MustCompile(`\brm\s+(-[^\s]*r[^\s]*f|-[^\s]*f[^\s]*r)[^\s]*\s+~/?\s*$`),
	regexp.MustCompile(`\brm\s+--recursive\s+--force\s+/`),
	regexp.MustCompile(`\brm\s+--force\s+--recursive\s+/`),

	// Fork bombs
	regexp.MustCompile(`:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`),

	// Filesystem formatting
	regexp.MustCompile(`\bmkfs\b`),

	// Raw disk writes
	regexp.MustCompile(`\bdd\s+if=.+of=/dev/`),
	regexp.MustCompile(`>\s*/dev/sd[a-z]`),
	regexp.MustCompile(`>\s*/dev/nvme`),
	regexp.MustCompile(`>\s*/dev/vd[a-z]`),

	// Recursive chmod 777 on root
	regexp.MustCompile(`\bchmod\s+(-[^\s]*R[^\s]*\s+)?777\s+/\s*$`),

	// Overwriting critical system files
	regexp.MustCompile(`>\s*/etc/passwd`),
	regexp.MustCompile(`>\s*/etc/shadow`),

	// Downloading and piping to shell
	regexp.MustCompile(`\bcurl\b.*\|\s*(ba)?sh`),
	regexp.MustCompile(`\bwget\b.*\|\s*(ba)?sh`),
}

func checkDangerousPatterns(command string) string {
	normalized := strings.TrimSpace(command)
	for _, pattern := range dangerousCommandPatterns {
		if pattern.MatchString(normalized) {
			return fmt.Sprintf("matches dangerous command pattern: %s", pattern.String())
		}
	}
	return ""
}

// checkExfiltrationCommand checks if a command starts with a known data exfiltration tool.
func checkExfiltrationCommand(command string) string {
	// Get the first word (the command name)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	cmdName := parts[0]
	// Handle paths like /usr/bin/curl
	if idx := strings.LastIndex(cmdName, "/"); idx >= 0 {
		cmdName = cmdName[idx+1:]
	}

	for _, exfilCmd := range exfiltrationCommands {
		if strings.EqualFold(cmdName, exfilCmd) {
			return fmt.Sprintf("command %q can send data to external servers", cmdName)
		}
	}
	return ""
}

// checkSensitiveFileAccess checks if a command attempts to read sensitive files.
func checkSensitiveFileAccess(command string) string {
	// Common file-reading commands
	readCommands := []string{"cat", "less", "more", "head", "tail", "grep", "awk", "sed", "sort", "wc"}

	parts := strings.Fields(command)
	if len(parts) < 2 {
		return ""
	}

	cmdName := parts[0]
	if idx := strings.LastIndex(cmdName, "/"); idx >= 0 {
		cmdName = cmdName[idx+1:]
	}

	isReadCmd := false
	for _, rc := range readCommands {
		if cmdName == rc {
			isReadCmd = true
			break
		}
	}
	if !isReadCmd {
		return ""
	}

	// Check if any argument looks like a sensitive file path
	for _, arg := range parts[1:] {
		if strings.HasPrefix(arg, "-") {
			continue // skip flags
		}
		if IsSensitivePath(arg) {
			return fmt.Sprintf("command %q accesses sensitive file %q", cmdName, arg)
		}
	}

	return ""
}

// ContainsShellSubstitution returns true if the command contains any shell
// variable expansion or command substitution patterns.
func ContainsShellSubstitution(command string) bool {
	stripped := stripEnvPrefixes(command)
	for _, pattern := range shellSubstitutionPatterns {
		if pattern.MatchString(stripped) {
			return true
		}
	}
	return false
}
