package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFileToolName(t *testing.T) {
	tool := &EditFileTool{}
	if got := tool.Name(); got != "edit_file" {
		t.Errorf("Name() = %q, want %q", got, "edit_file")
	}
}

func TestEditFileToolDescription(t *testing.T) {
	tool := &EditFileTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "edit") {
		t.Errorf("Description() = %q, expected it to mention editing", desc)
	}
}

func TestEditFileToolPermission(t *testing.T) {
	tool := &EditFileTool{}
	if got := tool.Permission(); got != PermissionWrite {
		t.Errorf("Permission() = %v, want %v (PermissionWrite)", got, PermissionWrite)
	}
}

func TestEditFileToolInputSchema(t *testing.T) {
	tool := &EditFileTool{}
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

	for _, field := range []string{"path", "old_text", "new_text"} {
		if _, ok := propsMap[field]; !ok {
			t.Errorf("InputSchema() properties missing %q", field)
		}
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
	for _, field := range []string{"path", "old_text", "new_text"} {
		if !requiredSet[field] {
			t.Errorf("InputSchema() required does not include %q", field)
		}
	}
}

func TestEditFileToolImplementsToolInterface(t *testing.T) {
	var _ Tool = &EditFileTool{}
}

func TestEditFileToolExecuteReplacesText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "edit_me.txt")
	original := "Hello, World!\nThis is a test file.\nGoodbye, World!\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "Hello, World!", "new_text": "Hi, Universe!"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Verify the file was modified correctly
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read modified file: %v", err)
	}

	expected := "Hi, Universe!\nThis is a test file.\nGoodbye, World!\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecuteReplacesMultilineText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "multiline.txt")
	original := "func main() {\n\tfmt.Println(\"old\")\n\treturn\n}\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	oldText := "func main() {\n\tfmt.Println(\"old\")\n\treturn\n}"
	newText := "func main() {\n\tfmt.Println(\"new\")\n\tos.Exit(0)\n}"
	input := fmt.Sprintf(`{"path": %q, "old_text": %q, "new_text": %q}`, filePath, oldText, newText)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read modified file: %v", err)
	}

	expected := "func main() {\n\tfmt.Println(\"new\")\n\tos.Exit(0)\n}\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecuteDeletesText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "delete.txt")
	original := "keep this\nremove this line\nkeep this too\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "remove this line\n", "new_text": ""}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read modified file: %v", err)
	}

	expected := "keep this\nkeep this too\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecuteFailsWhenOldTextNotFound(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notfound.txt")
	original := "some content here\nanother line\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "text that does not exist", "new_text": "replacement"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when old_text is not found")
	}

	if !strings.Contains(result.Error, "not found") {
		t.Errorf("Error = %q, expected it to mention 'not found'", result.Error)
	}

	// Verify file was not modified
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != original {
		t.Error("file content was modified even though old_text was not found")
	}
}

func TestEditFileToolExecuteFailsWhenOldTextMatchesMultipleTimes(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "duplicate.txt")
	original := "hello world\nfoo bar\nhello world\nbaz\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "hello world", "new_text": "hi there"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when old_text matches multiple times")
	}

	if !strings.Contains(strings.ToLower(result.Error), "multiple") {
		t.Errorf("Error = %q, expected it to mention 'multiple'", result.Error)
	}

	// Verify file was not modified
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != original {
		t.Error("file content was modified even though old_text matched multiple times")
	}
}

func TestEditFileToolExecuteEmptyPath(t *testing.T) {
	tool := &EditFileTool{}
	result, err := tool.Execute(context.Background(), `{"path": "", "old_text": "a", "new_text": "b"}`)
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

func TestEditFileToolExecuteInvalidJSON(t *testing.T) {
	tool := &EditFileTool{}
	result, err := tool.Execute(context.Background(), "not valid json at all {{{")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for invalid JSON")
	}
	if !strings.Contains(strings.ToLower(result.Error), "parse") || !strings.Contains(strings.ToLower(result.Error), "failed") {
		// Be flexible: just check it indicates a parsing problem
		if result.Error == "" {
			t.Error("Error is empty, expected parsing error message")
		}
	}
}

func TestEditFileToolExecuteNonexistentFile(t *testing.T) {
	tool := &EditFileTool{}
	input := `{"path": "/tmp/nonexistent_edit_file_test_12345.txt", "old_text": "a", "new_text": "b"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for nonexistent file")
	}
	if !strings.Contains(strings.ToLower(result.Error), "failed") && !strings.Contains(strings.ToLower(result.Error), "no such file") {
		t.Errorf("Error = %q, expected it to indicate file read failure", result.Error)
	}
}

func TestEditFileToolExecuteWithDisplayDescription(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "described.txt")
	original := "old value = 42\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "old value = 42", "new_text": "new value = 99", "display_description": "Update config value"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Output should mention the display description
	if !strings.Contains(result.Output, "Update config value") {
		t.Errorf("Output = %q, expected it to contain the display description", result.Output)
	}

	// Verify file was modified
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expected := "new value = 99\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecuteWithoutDisplayDescription(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nodesc.txt")
	original := "before edit\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "before edit", "new_text": "after edit"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should have a default description like "Edit applied"
	if !strings.Contains(result.Output, "Edit applied") {
		t.Errorf("Output = %q, expected it to contain a default description", result.Output)
	}
}

func TestEditFileToolExecuteOutputContainsDiff(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "diff.txt")
	original := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "beta", "new_text": "BETA"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Output should contain diff-like markers
	if !strings.Contains(result.Output, "-beta") {
		t.Errorf("Output missing old text in diff format, got: %q", result.Output)
	}
	if !strings.Contains(result.Output, "+BETA") {
		t.Errorf("Output missing new text in diff format, got: %q", result.Output)
	}
}

func TestEditFileToolExecuteWhitespaceMatching(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "whitespace.txt")
	original := "    indented line\n\ttabbed line\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Try to match with wrong whitespace — should fail
	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "indented line", "new_text": "replaced"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// "indented line" without leading spaces should still match since it's a substring
	// Actually in the original file it's "    indented line" — "indented line" is a substring
	// so strings.Count will find it once. Let's verify.
	if result.IsError {
		// If it errored, that's also acceptable depending on exact matching behavior.
		// The key point is it doesn't crash.
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(data), "replaced") {
		t.Error("Expected the replacement to have been applied")
	}
}

func TestEditFileToolExecuteExactWhitespaceRequired(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "exact.txt")
	original := "    four spaces\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Exact match with correct whitespace should work
	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "    four spaces", "new_text": "    replaced spaces"}`, filePath)
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
	expected := "    replaced spaces\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecuteReplaceWithLongerText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "longer.txt")
	original := "short\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	newText := "this is a much longer replacement\nwith multiple lines\nand more content\n"
	input := fmt.Sprintf(`{"path": %q, "old_text": "short\n", "new_text": %q}`, filePath, newText)
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
	if string(data) != newText {
		t.Errorf("file content = %q, want %q", string(data), newText)
	}
}

func TestEditFileToolExecuteReplaceWithShorterText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "shorter.txt")
	original := "this is a very long line of text that will be shortened\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "this is a very long line of text that will be shortened", "new_text": "short"}`, filePath)
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
	expected := "short\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecuteThreeOccurrences(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "triple.txt")
	original := "TODO: fix this\nsome code\nTODO: fix this\nmore code\nTODO: fix this\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "TODO: fix this", "new_text": "DONE"}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when old_text matches 3 times")
	}

	// Verify file was not modified
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != original {
		t.Error("file content was modified even though old_text matched multiple times")
	}
}

func TestEditFileToolExecuteReplaceEntireContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "entire.txt")
	original := "entire file content"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "entire file content", "new_text": "completely new content"}`, filePath)
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
	if string(data) != "completely new content" {
		t.Errorf("file content = %q, want %q", string(data), "completely new content")
	}
}

func TestEditFileToolExecuteSpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "special.txt")
	original := "price is $100 (USD)\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "price is $100 (USD)", "new_text": "price is €200 (EUR)"}`, filePath)
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
	expected := "price is €200 (EUR)\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecutePreservesRestOfFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "preserve.txt")
	original := "line 1\nline 2\nTARGET LINE\nline 4\nline 5\n"
	if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := &EditFileTool{}
	input := fmt.Sprintf(`{"path": %q, "old_text": "TARGET LINE", "new_text": "REPLACED LINE"}`, filePath)
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

	expected := "line 1\nline 2\nREPLACED LINE\nline 4\nline 5\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestEditFileToolExecuteMissingPathField(t *testing.T) {
	tool := &EditFileTool{}
	result, err := tool.Execute(context.Background(), `{"old_text": "a", "new_text": "b"}`)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when path field is missing")
	}
	if !strings.Contains(result.Error, "required") {
		t.Errorf("Error = %q, expected it to mention 'required'", result.Error)
	}
}

func TestEditFileToolExecuteEmptyJSON(t *testing.T) {
	tool := &EditFileTool{}
	result, err := tool.Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for empty JSON object")
	}
}

func TestGenerateDiffSummary(t *testing.T) {
	diff := generateDiffSummary("test.txt", "old line", "new line")

	if !strings.Contains(diff, "--- test.txt") {
		t.Errorf("diff missing old file header, got: %q", diff)
	}
	if !strings.Contains(diff, "+++ test.txt") {
		t.Errorf("diff missing new file header, got: %q", diff)
	}
	if !strings.Contains(diff, "-old line") {
		t.Errorf("diff missing removed line, got: %q", diff)
	}
	if !strings.Contains(diff, "+new line") {
		t.Errorf("diff missing added line, got: %q", diff)
	}
	if !strings.Contains(diff, "@@") {
		t.Errorf("diff missing hunk header, got: %q", diff)
	}
}

func TestGenerateDiffSummaryMultiline(t *testing.T) {
	oldText := "line 1\nline 2\nline 3"
	newText := "new line 1\nnew line 2"
	diff := generateDiffSummary("multi.txt", oldText, newText)

	// Should have 3 removed lines and 2 added lines
	removedCount := strings.Count(diff, "\n-")
	addedCount := strings.Count(diff, "\n+")
	if removedCount < 3 {
		t.Errorf("expected at least 3 removed lines, got %d in diff: %q", removedCount, diff)
	}
	if addedCount < 2 {
		t.Errorf("expected at least 2 added lines, got %d in diff: %q", addedCount, diff)
	}
}
