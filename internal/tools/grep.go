package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GrepTool searches file contents using regular expressions.
type GrepTool struct{}

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Include string `json:"include"`
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search file contents using a regular expression pattern. Returns matching lines with file paths and line numbers."
}

func (t *GrepTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory or file path to search in (default: current directory)",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g. '*.go')",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *GrepTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params grepInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Failed to parse input: %s", err),
			IsError: true,
		}, nil
	}

	if params.Pattern == "" {
		return &Result{
			Output:  "",
			Error:   "Pattern is required",
			IsError: true,
		}, nil
	}

	searchPath := params.Path
	if searchPath == "" {
		searchPath = "."
	}

	// Build grep command arguments
	args := []string{"-rn", "--color=never"}

	if params.Include != "" {
		args = append(args, fmt.Sprintf("--include=%s", params.Include))
	}

	// Use -E for extended regex support
	args = append(args, "-E", params.Pattern, searchPath)

	cmd := exec.CommandContext(ctx, "grep", args...)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// grep returns exit code 1 when no matches are found — that's not an error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return &Result{
					Output:  "No matches found.",
					IsError: false,
				}, nil
			}
			// Exit code 2 means an actual error (bad regex, etc.)
			return &Result{
				Output:  "",
				Error:   fmt.Sprintf("grep error: %s", strings.TrimSpace(outputStr)),
				IsError: true,
			}, nil
		}
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Failed to execute grep: %s", err),
			IsError: true,
		}, nil
	}

	// Limit results to 50 matches
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	const maxMatches = 50
	truncated := false

	if len(lines) > maxMatches {
		lines = lines[:maxMatches]
		truncated = true
	}

	var result strings.Builder
	for _, line := range lines {
		if line != "" {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	if truncated {
		fmt.Fprintf(&result, "\n... (results truncated, showing first %d of many matches)\n", maxMatches)
	}

	return &Result{
		Output:  result.String(),
		IsError: false,
	}, nil
}
