package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/nomanqureshi/argo/internal/security"
)

const (
	defaultTimeoutMs = 30000
	maxOutputBytes   = 100 * 1024 // 100KB
)

// ShellTool executes shell commands and returns their output.
type ShellTool struct{}

type shellInput struct {
	Command   string `json:"command"`
	TimeoutMs int    `json:"timeout_ms"`
}

func (t *ShellTool) Name() string {
	return "shell"
}

func (t *ShellTool) Description() string {
	return "Execute a shell command and return the output. Commands run in the user's default shell."
}

func (t *ShellTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Optional timeout in milliseconds (default: 30000)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ShellTool) Permission() PermissionLevel {
	return PermissionDangerous
}

func (t *ShellTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params shellInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("failed to parse input: %s", err),
			IsError: true,
		}, nil
	}

	if params.Command == "" {
		return &Result{
			Output:  "",
			Error:   "command is required",
			IsError: true,
		}, nil
	}

	// Security: validate command before execution
	validation := security.ValidateCommand(params.Command)
	if validation.Blocked {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("command blocked by security policy: %s", validation.Reason),
			IsError: true,
		}, nil
	}

	// Security: warn about shell substitutions
	var securityWarnings []string
	if security.ContainsShellSubstitution(params.Command) {
		securityWarnings = append(securityWarnings,
			"Warning: command contains shell variable expansion ($VAR, $(cmd), etc.) which may expose sensitive environment variables.")
	}
	if validation.RequiresConfirmation {
		for _, v := range validation.Violations {
			securityWarnings = append(securityWarnings, "Security notice: "+v)
		}
	}

	timeoutMs := params.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultTimeoutMs
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", params.Command)

	// Security: sanitize environment to prevent secret leakage
	cmd.Env = security.SanitizeEnvironment()

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	err := cmd.Run()

	output := combined.Bytes()
	truncated := false
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes]
		truncated = true
	}

	outputStr := string(output)
	if truncated {
		outputStr += "\n\n... [output truncated at 100KB]"
	}

	if cmdCtx.Err() == context.DeadlineExceeded {
		return &Result{
			Output:  outputStr,
			Error:   fmt.Sprintf("command timed out after %dms", timeoutMs),
			IsError: true,
		}, nil
	}

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			return &Result{
				Output:  outputStr,
				Error:   fmt.Sprintf("command exited with status %d", exitErr.ExitCode()),
				IsError: true,
			}, nil
		}
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("failed to execute command: %s", err),
			IsError: true,
		}, nil
	}

	if len(securityWarnings) > 0 {
		prefix := strings.Join(securityWarnings, "\n") + "\n\n"
		outputStr = prefix + outputStr
	}

	return &Result{
		Output:  outputStr,
		IsError: false,
	}, nil
}
