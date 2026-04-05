package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nomanqureshi/argo/internal/security"
)

// WriteFileTool creates or overwrites files with given content.
type WriteFileTool struct{}

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Create a new file or overwrite an existing file with the given content."
}

func (t *WriteFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Permission() PermissionLevel {
	return PermissionWrite
}

func (t *WriteFileTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params writeFileInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Failed to parse input: %s", err),
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

	if params.Content == "" && params.Path == "" {
		return &Result{
			Output:  "",
			Error:   "content is required",
			IsError: true,
		}, nil
	}

	// Security: check for sensitive file paths
	if security.IsSensitivePath(params.Path) {
		reason := security.DescribeSensitivePath(params.Path)
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("write denied: %s. This file is protected by Argo's security policy.", reason),
			IsError: true,
		}, nil
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(params.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &Result{
				Output:  "",
				Error:   fmt.Sprintf("Failed to create directories %s: %s", dir, err),
				IsError: true,
			}, nil
		}
	}

	// Write the file
	content := []byte(params.Content)
	if err := os.WriteFile(params.Path, content, 0644); err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Failed to write file %s: %s", params.Path, err),
			IsError: true,
		}, nil
	}

	return &Result{
		Output:  fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), params.Path),
		IsError: false,
	}, nil
}
