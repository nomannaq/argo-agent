package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nomanqureshi/argo/internal/security"
)

// ReadFileTool reads the contents of a file, optionally extracting a line range.
type ReadFileTool struct{}

type readFileInput struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file at the given path. Can optionally read specific line ranges."
}

func (t *ReadFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional start line (1-based)",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional end line (1-based, inclusive)",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *ReadFileTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params readFileInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("failed to parse input: %v", err),
			IsError: true,
		}, nil
	}

	if params.Path == "" {
		return &Result{
			Output:  "",
			Error:   "path is required",
			IsError: true,
		}, nil
	}

	// Security: check for sensitive file paths
	if security.IsSensitivePath(params.Path) {
		reason := security.DescribeSensitivePath(params.Path)
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("access denied: %s. This file is protected by Argo's security policy.", reason),
			IsError: true,
		}, nil
	}

	data, err := os.ReadFile(params.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{
				Output:  "",
				Error:   fmt.Sprintf("file not found: %s", params.Path),
				IsError: true,
			}, nil
		}
		if os.IsPermission(err) {
			return &Result{
				Output:  "",
				Error:   fmt.Sprintf("permission denied: %s", params.Path),
				IsError: true,
			}, nil
		}
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("failed to read file: %v", err),
			IsError: true,
		}, nil
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	hasRange := params.StartLine > 0 || params.EndLine > 0

	// For large files with no line range specified, return an outline message
	if !hasRange && totalLines > 500 {
		return &Result{
			Output: fmt.Sprintf(
				"File %s has %d lines, which is too large to display in full.\n"+
					"Please specify a line range using start_line and end_line parameters.\n"+
					"For example, start_line=1 and end_line=100 to read the first 100 lines.",
				params.Path, totalLines,
			),
			IsError: false,
		}, nil
	}

	// If a line range is specified, extract it
	if hasRange {
		startLine := params.StartLine
		endLine := params.EndLine

		if startLine <= 0 {
			startLine = 1
		}
		if endLine <= 0 || endLine > totalLines {
			endLine = totalLines
		}
		if startLine > totalLines {
			return &Result{
				Output:  "",
				Error:   fmt.Sprintf("start_line %d exceeds file length of %d lines", startLine, totalLines),
				IsError: true,
			}, nil
		}
		if startLine > endLine {
			return &Result{
				Output:  "",
				Error:   fmt.Sprintf("start_line %d is greater than end_line %d", startLine, endLine),
				IsError: true,
			}, nil
		}

		// Convert from 1-based to 0-based indexing
		selectedLines := lines[startLine-1 : endLine]

		var sb strings.Builder
		for i, line := range selectedLines {
			lineNum := startLine + i
			fmt.Fprintf(&sb, "%4d | %s", lineNum, line)
			if i < len(selectedLines)-1 {
				sb.WriteByte('\n')
			}
		}

		return &Result{
			Output:  sb.String(),
			IsError: false,
		}, nil
	}

	// Return the full file content with line numbers
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%4d | %s", i+1, line)
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}

	return &Result{
		Output:  sb.String(),
		IsError: false,
	}, nil
}
