package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileToolName(t *testing.T) {
	tool := &ReadFileTool{}
	if got := tool.Name(); got != "read_file" {
		t.Errorf("Name() = %q, want %q", got, "read_file")
	}
}

func TestReadFileToolDescription(t *testing.T) {
	tool := &ReadFileTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(desc, "Read") && !strings.Contains(desc, "read") {
		t.Errorf("Description() = %q, expected it to mention reading", desc)
	}
}

func TestReadFileToolPermission(t *testing.T) {
	tool := &ReadFileTool{}
	if got := tool.Permission(); got != PermissionRead {
		t.Errorf("Permission() = %v, want %v (PermissionRead)", got, PermissionRead)
	}
}

func TestReadFileToolInputSchema(t *testing.T) {
	tool := &ReadFileTool{}
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
	if _, ok := propsMap["start_line"]; !ok {
		t.Error("InputSchema() properties missing 'start_line'")
	}
	if _, ok := propsMap["end_line"]; !ok {
		t.Error("InputSchema() properties missing 'end_line'")
	}

	required, ok := schema["required"]
	if !ok {
		t.Fatal("InputSchema() missing 'required' key")
	}

	requiredSlice, ok := required.([]string)
	if !ok {
		t.Fatalf("InputSchema() required is not []string, got %T", required)
	}

	found := false
	for _, r := range requiredSlice {
		if r == "path" {
			found = true
			break
		}
	}
	if !found {
		t.Error("InputSchema() required does not include 'path'")
	}
}

func TestReadFileToolExecuteValidFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should contain line numbers
	if !strings.Contains(result.Output, "1 |") {
		t.Error("Output does not contain line number 1")
	}
	if !strings.Contains(result.Output, "line one") {
		t.Error("Output does not contain 'line one'")
	}
	if !strings.Contains(result.Output, "line two") {
		t.Error("Output does not contain 'line two'")
	}
	if !strings.Contains(result.Output, "line three") {
		t.Error("Output does not contain 'line three'")
	}
}

func TestReadFileToolExecuteLineNumbers(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "numbered.txt")

	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("content line %d", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	result, err := tool.Execute(context.Background(), fmt.Sprintf(`{"path": %q}`, filePath))
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Check that each line has a line number prefix
	outputLines := strings.Split(result.Output, "\n")
	for _, line := range outputLines {
		if line == "" {
			continue
		}
		if !strings.Contains(line, " | ") {
			t.Errorf("output line missing line number separator: %q", line)
		}
	}
}

func TestReadFileToolExecuteWithStartAndEndLine(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "range.txt")

	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d here", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q, "start_line": 3, "end_line": 5}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should contain lines 3-5
	if !strings.Contains(result.Output, "line 3 here") {
		t.Error("Output missing 'line 3 here'")
	}
	if !strings.Contains(result.Output, "line 4 here") {
		t.Error("Output missing 'line 4 here'")
	}
	if !strings.Contains(result.Output, "line 5 here") {
		t.Error("Output missing 'line 5 here'")
	}

	// Should NOT contain lines outside the range
	if strings.Contains(result.Output, "line 2 here") {
		t.Error("Output contains 'line 2 here' which is outside the requested range")
	}
	if strings.Contains(result.Output, "line 6 here") {
		t.Error("Output contains 'line 6 here' which is outside the requested range")
	}

	// Verify line numbers in output start at 3
	if !strings.Contains(result.Output, "3 |") {
		t.Error("Output line numbers should start at 3")
	}
}

func TestReadFileToolExecuteWithStartLineOnly(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "startonly.txt")

	var lines []string
	for i := 1; i <= 5; i++ {
		lines = append(lines, fmt.Sprintf("row %d", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q, "start_line": 3}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should contain lines 3 onwards
	if !strings.Contains(result.Output, "row 3") {
		t.Error("Output missing 'row 3'")
	}
	if !strings.Contains(result.Output, "row 5") {
		t.Error("Output missing 'row 5'")
	}
	// Should NOT contain lines before start_line
	if strings.Contains(result.Output, "row 2") {
		t.Error("Output contains 'row 2' which is before start_line")
	}
}

func TestReadFileToolExecuteWithEndLineOnly(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "endonly.txt")

	var lines []string
	for i := 1; i <= 5; i++ {
		lines = append(lines, fmt.Sprintf("item %d", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q, "end_line": 3}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should contain lines 1-3
	if !strings.Contains(result.Output, "item 1") {
		t.Error("Output missing 'item 1'")
	}
	if !strings.Contains(result.Output, "item 3") {
		t.Error("Output missing 'item 3'")
	}
	// Should NOT contain lines after end_line
	if strings.Contains(result.Output, "item 4") {
		t.Error("Output contains 'item 4' which is after end_line")
	}
}

func TestReadFileToolExecuteNonexistentFile(t *testing.T) {
	tool := &ReadFileTool{}
	input := `{"path": "/tmp/this_file_does_not_exist_at_all_12345.txt"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for nonexistent file")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("Error = %q, expected it to mention 'not found'", result.Error)
	}
}

func TestReadFileToolExecuteInvalidJSON(t *testing.T) {
	tool := &ReadFileTool{}
	result, err := tool.Execute(context.Background(), "not valid json at all")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for invalid JSON")
	}
	if !strings.Contains(result.Error, "parse") {
		t.Errorf("Error = %q, expected it to mention 'parse'", result.Error)
	}
}

func TestReadFileToolExecuteEmptyPath(t *testing.T) {
	tool := &ReadFileTool{}
	result, err := tool.Execute(context.Background(), `{"path": ""}`)
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

func TestReadFileToolExecuteEmptyJSONObject(t *testing.T) {
	tool := &ReadFileTool{}
	result, err := tool.Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for missing path")
	}
	if !strings.Contains(result.Error, "required") {
		t.Errorf("Error = %q, expected it to mention 'required'", result.Error)
	}
}

func TestReadFileToolExecuteLargeFileWithoutRange(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.txt")

	var lines []string
	for i := 1; i <= 600; i++ {
		lines = append(lines, fmt.Sprintf("line number %d with some content", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// Should NOT be an error
	if result.IsError {
		t.Fatalf("Execute() returned error result for large file: %s", result.Error)
	}

	// Should contain a warning/info message about the file being too large
	if !strings.Contains(result.Output, "too large") && !strings.Contains(result.Output, "large") {
		t.Errorf("Output should contain a warning about file size, got: %q", result.Output[:min(200, len(result.Output))])
	}

	// Should mention the line count
	if !strings.Contains(result.Output, "600") && !strings.Contains(result.Output, "601") {
		t.Error("Output should mention the number of lines")
	}

	// Should suggest using start_line/end_line
	if !strings.Contains(result.Output, "start_line") || !strings.Contains(result.Output, "end_line") {
		t.Error("Output should suggest using start_line and end_line parameters")
	}

	// Should NOT contain the actual file content lines
	if strings.Contains(result.Output, "line number 1 with some content") {
		t.Error("Output should not contain the actual file content for large files without range")
	}
}

func TestReadFileToolExecuteLargeFileWithRange(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large_range.txt")

	var lines []string
	for i := 1; i <= 600; i++ {
		lines = append(lines, fmt.Sprintf("data line %d", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q, "start_line": 10, "end_line": 20}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should contain lines from the range
	if !strings.Contains(result.Output, "data line 10") {
		t.Error("Output missing 'data line 10'")
	}
	if !strings.Contains(result.Output, "data line 20") {
		t.Error("Output missing 'data line 20'")
	}

	// Should NOT contain lines outside the range
	if strings.Contains(result.Output, "data line 9") {
		t.Error("Output should not contain 'data line 9'")
	}
	if strings.Contains(result.Output, "data line 21") {
		t.Error("Output should not contain 'data line 21'")
	}
}

func TestReadFileToolExecuteSmallFileFullContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "small.txt")
	content := "hello\nworld\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Error("Output missing 'hello'")
	}
	if !strings.Contains(result.Output, "world") {
		t.Error("Output missing 'world'")
	}
}

func TestReadFileToolExecuteEmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}
}

func TestReadFileToolExecuteSingleLine(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "single.txt")
	if err := os.WriteFile(filePath, []byte("only one line"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "1 |") {
		t.Error("Output missing line number 1")
	}
	if !strings.Contains(result.Output, "only one line") {
		t.Error("Output missing file content")
	}
}

func TestReadFileToolExecuteStartLineExceedsFileLength(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "short.txt")
	if err := os.WriteFile(filePath, []byte("one\ntwo\nthree"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q, "start_line": 100}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when start_line exceeds file length")
	}
	if !strings.Contains(result.Error, "exceeds") {
		t.Errorf("Error = %q, expected it to mention 'exceeds'", result.Error)
	}
}

func TestReadFileToolExecuteStartLineGreaterThanEndLine(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "inverted.txt")
	if err := os.WriteFile(filePath, []byte("aaa\nbbb\nccc\nddd\neee"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q, "start_line": 4, "end_line": 2}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when start_line > end_line")
	}
	if !strings.Contains(result.Error, "greater than") {
		t.Errorf("Error = %q, expected it to mention 'greater than'", result.Error)
	}
}

func TestReadFileToolExecuteExactly500Lines(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "exactly500.txt")

	var lines []string
	for i := 1; i <= 500; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// 500 lines should be displayed fully (boundary: > 500 triggers the warning)
	if !strings.Contains(result.Output, "line 1") {
		t.Error("Output should contain file content for exactly 500 lines")
	}
	if !strings.Contains(result.Output, "line 500") {
		t.Error("Output should contain line 500")
	}
}

func TestReadFileToolExecute501Lines(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "over500.txt")

	var lines []string
	for i := 1; i <= 501; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// 501 lines should trigger the large file warning
	if !strings.Contains(result.Output, "start_line") {
		t.Error("Output should suggest using start_line for file with >500 lines")
	}
}

func TestReadFileToolExecuteFileInSubdirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	filePath := filepath.Join(subDir, "deep.txt")
	if err := os.WriteFile(filePath, []byte("deep content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &ReadFileTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "deep content") {
		t.Error("Output missing 'deep content'")
	}
}

func TestReadFileToolImplementsToolInterface(t *testing.T) {
	var _ Tool = &ReadFileTool{}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
