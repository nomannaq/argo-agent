package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListDirectoryTool lists files and directories at the given path.
type ListDirectoryTool struct{}

type listDirectoryInput struct {
	Path string `json:"path"`
}

func (t *ListDirectoryTool) Name() string {
	return "list_directory"
}

func (t *ListDirectoryTool) Description() string {
	return "List files and directories at the given path."
}

func (t *ListDirectoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path to list (default: current directory)",
			},
		},
	}
}

func (t *ListDirectoryTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *ListDirectoryTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params listDirectoryInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Failed to parse input: %s", err),
			IsError: true,
		}, nil
	}

	dirPath := params.Path
	if dirPath == "" {
		dirPath = "."
	}

	dirPath = filepath.Clean(dirPath)

	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{
				Output:  "",
				Error:   fmt.Sprintf("Directory not found: %s", dirPath),
				IsError: true,
			}, nil
		}
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Cannot access directory: %s", err),
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Path is not a directory: %s", dirPath),
			IsError: true,
		}, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Failed to read directory: %s", err),
			IsError: true,
		}, nil
	}

	type entryInfo struct {
		name  string
		isDir bool
		size  int64
	}

	var dirs []entryInfo
	var files []entryInfo

	for _, entry := range entries {
		ei := entryInfo{
			name:  entry.Name(),
			isDir: entry.IsDir(),
		}
		if !entry.IsDir() {
			if fi, err := entry.Info(); err == nil {
				ei.size = fi.Size()
			}
		}
		if ei.isDir {
			dirs = append(dirs, ei)
		} else {
			files = append(files, ei)
		}
	}

	// Sort directories and files alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].name < dirs[j].name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].name < files[j].name
	})

	var sb strings.Builder
	fmt.Fprintf(&sb, "Directory: %s\n\n", dirPath)

	totalEntries := len(dirs) + len(files)
	if totalEntries == 0 {
		sb.WriteString("(empty directory)\n")
		return &Result{
			Output: sb.String(),
		}, nil
	}

	// List directories first
	for _, d := range dirs {
		fmt.Fprintf(&sb, "  [dir]  %s/\n", d.name)
	}

	// Then list files
	for _, f := range files {
		fmt.Fprintf(&sb, "  [file] %s (%s)\n", f.name, formatFileSize(f.size))
	}

	fmt.Fprintf(&sb, "\n%d directories, %d files\n", len(dirs), len(files))

	return &Result{
		Output: sb.String(),
	}, nil
}

// formatFileSize returns a human-readable file size string.
func formatFileSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	case size < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
	}
}
