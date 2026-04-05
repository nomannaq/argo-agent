package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileToolName(t *testing.T) {
	tool := &WriteFileTool{}
	if got := tool.Name(); got != "write_file" {
		t.Errorf("Name() = %q, want %q", got, "write_file")
	}
}

func TestWriteFileToolDescription(t *testing.T) {
	tool := &WriteFileTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestWriteFileToolPermission(t *testing.T) {
	tool := &WriteFileTool{}
	if got := tool.Permission(); got != PermissionWrite {
		t.Errorf("Permission() = %v, want %v (PermissionWrite)", got, PermissionWrite)
	}
}

func TestWriteFileToolInputSchema(t *testing.T) {
	tool := &WriteFileTool{}
	schema := tool.InputSchema()

	if schema == nil {
		t.Fatal("InputSchema() returned nil")
	}

	typ, ok := schema["type"]
	if !ok {
		t.Error("InputSchema() missing 'type' key")
	}
	if typ != "object" {
		t.Errorf("InputSchema() type = %v, want %q", typ, "object")
	}

	props, ok := schema["properties"]
	if !ok {
		t.Fatal("InputSchema() missing 'properties' key")
	}

	propsMap, ok := props.(map[string]any)
	if !ok {
		t.Fatalf("InputSchema() properties is not map[string]any, got %T", props)
	}

	if _, ok := propsMap["path"]; !ok {
		t.Error("InputSchema() properties missing 'path'")
	}
	if _, ok := propsMap["content"]; !ok {
		t.Error("InputSchema() properties missing 'content'")
	}

	required, ok := schema["required"]
	if !ok {
		t.Fatal("InputSchema() missing 'required' key")
	}

	requiredSlice, ok := required.([]string)
	if !ok {
		t.Fatalf("InputSchema() required is not []string, got %T", required)
	}

	requiredSet := make(map[string]bool)
	for _, r := range requiredSlice {
		requiredSet[r] = true
	}
	if !requiredSet["path"] {
		t.Error("InputSchema() required does not include 'path'")
	}
	if !requiredSet["content"] {
		t.Error("InputSchema() required does not include 'content'")
	}
}

func TestWriteFileToolImplementsToolInterface(t *testing.T) {
	var _ Tool = &WriteFileTool{}
}

func TestWriteFileToolExecuteCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "newfile.txt")

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": "hello world"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Verify the file was actually created on disk
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}

	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}

	// Verify the output message mentions success
	if !strings.Contains(result.Output, "Successfully") && !strings.Contains(result.Output, "success") {
		t.Errorf("Output = %q, expected it to mention success", result.Output)
	}

	// Verify the output mentions the byte count
	if !strings.Contains(result.Output, "11") {
		t.Errorf("Output = %q, expected it to mention byte count (11)", result.Output)
	}
}

func TestWriteFileToolExecuteOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "existing.txt")

	// Write initial content
	if err := os.WriteFile(filePath, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": "new content"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Verify the file was overwritten
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != "new content" {
		t.Errorf("file content = %q, want %q", string(data), "new content")
	}
}

func TestWriteFileToolExecuteCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a", "b", "c", "deep.txt")

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": "deeply nested"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Verify the parent directories were created
	parentDir := filepath.Join(dir, "a", "b", "c")
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("parent directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %q to be a directory", parentDir)
	}

	// Verify file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "deeply nested" {
		t.Errorf("file content = %q, want %q", string(data), "deeply nested")
	}
}

func TestWriteFileToolExecuteEmptyPath(t *testing.T) {
	tool := &WriteFileTool{}
	result, err := tool.Execute(context.Background(), `{"path": "", "content": "stuff"}`)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for empty path")
	}
	if !strings.Contains(result.Error, "required") {
		t.Errorf("Error = %q, expected it to mention 'required'", result.Error)
	}
}

func TestWriteFileToolExecuteInvalidJSON(t *testing.T) {
	tool := &WriteFileTool{}
	result, err := tool.Execute(context.Background(), "this is not json")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for invalid JSON")
	}
	if !strings.Contains(strings.ToLower(result.Error), "parse") && !strings.Contains(strings.ToLower(result.Error), "failed") {
		t.Errorf("Error = %q, expected it to mention parsing failure", result.Error)
	}
}

func TestWriteFileToolExecuteEmptyContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.txt")

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": ""}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// The file should exist with empty content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("file content length = %d, want 0", len(data))
	}
}

func TestWriteFileToolExecuteMultilineContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "multiline.txt")

	content := "line 1\nline 2\nline 3\n"
	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": %q}`, filePath, content)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

func TestWriteFileToolExecuteSpecialCharactersInContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "special.txt")

	content := "tabs\there\nnewlines\n\nand unicode: 日本語 🎉"
	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": %q}`, filePath, content)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

func TestWriteFileToolExecuteOutputMentionsPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "mentioned.txt")

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": "data"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, filePath) {
		t.Errorf("Output = %q, expected it to mention file path %q", result.Output, filePath)
	}
}

func TestWriteFileToolExecuteLargeContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.txt")

	// Create a large content string (~10KB)
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&sb, "line %d: some repeated content here\n", i)
	}
	content := sb.String()

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": %q}`, filePath, content)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("large file content mismatch: got %d bytes, want %d bytes", len(data), len(content))
	}
}

func TestWriteFileToolExecuteMissingContentField(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nofield.txt")

	tool := &WriteFileTool{}
	// JSON with path but no content field — content defaults to empty string
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// This should succeed (content defaults to "") or return an error about content being required.
	// Based on the implementation, empty content with a valid path is allowed.
	if result.IsError {
		// Acceptable if the implementation requires content
		return
	}

	// If it succeeded, verify the file was created
	_, statErr := os.Stat(filePath)
	if statErr != nil {
		t.Errorf("expected file to be created, got stat error: %v", statErr)
	}
}

func TestWriteFileToolExecuteCreatesSingleParentDir(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "subdir", "file.txt")

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": "in subdir"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "in subdir" {
		t.Errorf("file content = %q, want %q", string(data), "in subdir")
	}
}

func TestWriteFileToolExecutePreservesFilePermissions(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "perms.txt")

	tool := &WriteFileTool{}
	input := fmt.Sprintf(`{"path": %q, "content": "check perms"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	// File should be created with 0644 permissions
	perm := info.Mode().Perm()
	if perm != 0644 {
		t.Errorf("file permission = %o, want 0644", perm)
	}
}
