package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nomanqureshi/argo/internal/security"
)

// EditFileTool makes targeted edits to existing files using search and replace.
type EditFileTool struct{}

type editFileInput struct {
	Path               string `json:"path"`
	OldText            string `json:"old_text"`
	NewText            string `json:"new_text"`
	DisplayDescription string `json:"display_description"`
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Make targeted edits to an existing file using search and replace. Specify the exact text to find and what to replace it with. For creating new files or full rewrites, use write_file instead."
}

func (t *EditFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit",
			},
			"old_text": map[string]any{
				"type":        "string",
				"description": "The exact text to find in the file. Must match exactly including whitespace and indentation.",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "The text to replace old_text with. Set to empty string to delete the matched text.",
			},
			"display_description": map[string]any{
				"type":        "string",
				"description": "A brief description of the edit being made",
			},
		},
		"required": []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFileTool) Permission() PermissionLevel {
	return PermissionWrite
}

func (t *EditFileTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params editFileInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Error:   fmt.Sprintf("Failed to parse input: %s", err),
			IsError: true,
		}, nil
	}

	if params.Path == "" {
		return &Result{
			Error:   "path is required",
			IsError: true,
		}, nil
	}

	// Security: check for sensitive file paths
	if security.IsSensitivePath(params.Path) {
		reason := security.DescribeSensitivePath(params.Path)
		return &Result{
			Error:   fmt.Sprintf("edit denied: %s. This file is protected by Argo's security policy.", reason),
			IsError: true,
		}, nil
	}

	// Read the existing file
	data, err := os.ReadFile(params.Path)
	if err != nil {
		return &Result{
			Error:   fmt.Sprintf("Failed to read file %s: %s", params.Path, err),
			IsError: true,
		}, nil
	}

	content := string(data)

	// Count occurrences of old_text
	count := strings.Count(content, params.OldText)

	if count == 0 {
		return &Result{
			Error:   "The specified old_text was not found in the file. Make sure it matches exactly, including whitespace and indentation.",
			IsError: true,
		}, nil
	}

	if count > 1 {
		return &Result{
			Error:   "The specified old_text matches multiple locations in the file. Please provide a more specific/larger text snippet to match uniquely.",
			IsError: true,
		}, nil
	}

	// Replace the single occurrence
	newContent := strings.Replace(content, params.OldText, params.NewText, 1)

	// Write the modified content back
	if err := os.WriteFile(params.Path, []byte(newContent), 0644); err != nil {
		return &Result{
			Error:   fmt.Sprintf("Failed to write file %s: %s", params.Path, err),
			IsError: true,
		}, nil
	}

	// Generate a simple unified diff summary
	diff := generateDiffSummary(params.Path, params.OldText, params.NewText)

	desc := params.DisplayDescription
	if desc == "" {
		desc = "Edit applied"
	}

	output := fmt.Sprintf("%s\n\n%s", desc, diff)

	return &Result{
		Output:  output,
		IsError: false,
	}, nil
}

// generateDiffSummary creates a simple unified-diff-style summary of the change.
func generateDiffSummary(path, oldText, newText string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "--- %s\n", path)
	fmt.Fprintf(&b, "+++ %s\n", path)

	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	fmt.Fprintf(&b, "@@ -%d +%d @@\n", len(oldLines), len(newLines))

	for _, line := range oldLines {
		b.WriteString("-" + line + "\n")
	}
	for _, line := range newLines {
		b.WriteString("+" + line + "\n")
	}

	return b.String()
}
