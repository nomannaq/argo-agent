package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindFilesTool finds files matching a glob pattern in the project.
type FindFilesTool struct{}

type findFilesInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func (t *FindFilesTool) Name() string {
	return "find_files"
}

func (t *FindFilesTool) Description() string {
	return "Find files matching a glob pattern in the project."
}

func (t *FindFilesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern to match (e.g. '**/*.go')",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Base directory to search from (default: current directory)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *FindFilesTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *FindFilesTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params findFilesInput
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

	basePath := params.Path
	if basePath == "" {
		basePath = "."
	}

	// Verify the base path exists
	info, err := os.Stat(basePath)
	if err != nil {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Path error: %s", err),
			IsError: true,
		}, nil
	}
	if !info.IsDir() {
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Path is not a directory: %s", basePath),
			IsError: true,
		}, nil
	}

	pattern := params.Pattern
	hasDoublestar := strings.Contains(pattern, "**")

	// Split pattern into directory prefix and file glob parts for ** support
	// e.g., "**/*.go" means match *.go at any depth
	// e.g., "src/**/*.go" means match *.go at any depth under src/
	var matches []string
	const maxResults = 100

	err = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't access
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if len(matches) >= maxResults {
			return filepath.SkipAll
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != basePath {
			return filepath.SkipDir
		}

		// Skip directories themselves, we only want files
		if d.IsDir() {
			return nil
		}

		// Get relative path from base
		relPath, relErr := filepath.Rel(basePath, path)
		if relErr != nil {
			return nil
		}

		matched := false
		if hasDoublestar {
			matched = matchDoublestar(pattern, relPath)
		} else {
			// Simple glob match against the relative path
			var matchErr error
			matched, matchErr = filepath.Match(pattern, relPath)
			if matchErr != nil {
				// Try matching against just the filename
				matched, _ = filepath.Match(pattern, d.Name())
			}
		}

		if matched {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		if ctx.Err() != nil {
			return &Result{
				Output:  "",
				Error:   "Search cancelled: context deadline exceeded",
				IsError: true,
			}, nil
		}
		return &Result{
			Output:  "",
			Error:   fmt.Sprintf("Error walking directory: %s", err),
			IsError: true,
		}, nil
	}

	if len(matches) == 0 {
		return &Result{
			Output:  fmt.Sprintf("No files found matching pattern: %s", pattern),
			IsError: false,
		}, nil
	}

	var sb strings.Builder
	for i, m := range matches {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(m)
	}

	if len(matches) >= maxResults {
		fmt.Fprintf(&sb, "\n\n(Results limited to %d files)", maxResults)
	}

	return &Result{
		Output:  sb.String(),
		IsError: false,
	}, nil
}

// matchDoublestar matches a pattern containing ** against a file path.
// ** matches zero or more directory levels.
func matchDoublestar(pattern, path string) bool {
	// Normalize separators
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	return doMatchDoublestar(pattern, path)
}

func doMatchDoublestar(pattern, path string) bool {
	// Split pattern by **
	parts := strings.SplitN(pattern, "**", 2)

	if len(parts) == 1 {
		// No ** found, do a regular match
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	prefix := parts[0]
	suffix := parts[1]

	// Remove leading separator from suffix if present
	suffix = strings.TrimPrefix(suffix, "/")

	// The prefix must match the beginning of the path (if non-empty)
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/")
		if !strings.HasPrefix(path, prefix+"/") && path != prefix {
			return false
		}
		// Strip the prefix from path
		path = strings.TrimPrefix(path, prefix+"/")
	}

	// If no suffix, ** matches everything remaining
	if suffix == "" {
		return true
	}

	// Try matching suffix against every possible sub-path
	// ** can match zero or more path segments
	pathParts := strings.Split(path, "/")
	for i := 0; i <= len(pathParts); i++ {
		remaining := strings.Join(pathParts[i:], "/")
		if strings.Contains(suffix, "**") {
			if doMatchDoublestar(suffix, remaining) {
				return true
			}
		} else {
			matched, _ := filepath.Match(suffix, remaining)
			if matched {
				return true
			}
		}
	}

	return false
}
