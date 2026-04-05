package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirectoryToolName(t *testing.T) {
	tool := &ListDirectoryTool{}
	if got := tool.Name(); got != "list_directory" {
		t.Errorf("Name() = %q, want %q", got, "list_directory")
	}
}

func TestListDirectoryToolDescription(t *testing.T) {
	tool := &ListDirectoryTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "list") {
		t.Errorf("Description() = %q, expected it to mention listing", desc)
	}
}

func TestListDirectoryToolPermission(t *testing.T) {
	tool := &ListDirectoryTool{}
	if got := tool.Permission(); got != PermissionRead {
		t.Errorf("Permission() = %v, want %v (PermissionRead)", got, PermissionRead)
	}
}

func TestListDirectoryToolInputSchema(t *testing.T) {
	tool := &ListDirectoryTool{}
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
}

func TestListDirectoryToolImplementsToolInterface(t *testing.T) {
	var _ Tool = &ListDirectoryTool{}
}

func TestListDirectoryToolExecuteListsContents(t *testing.T) {
	dir := t.TempDir()

	// Create some directories
	for _, d := range []string{"alpha_dir", "beta_dir"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", d, err)
		}
	}

	// Create some files
	for _, f := range []string{"charlie.txt", "delta.go"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Verify directories are listed
	if !strings.Contains(result.Output, "alpha_dir") {
		t.Error("Output missing 'alpha_dir'")
	}
	if !strings.Contains(result.Output, "beta_dir") {
		t.Error("Output missing 'beta_dir'")
	}

	// Verify files are listed
	if !strings.Contains(result.Output, "charlie.txt") {
		t.Error("Output missing 'charlie.txt'")
	}
	if !strings.Contains(result.Output, "delta.go") {
		t.Error("Output missing 'delta.go'")
	}

	// Verify directories are marked as dirs
	if !strings.Contains(result.Output, "[dir]") {
		t.Error("Output missing '[dir]' marker for directories")
	}

	// Verify files are marked as files
	if !strings.Contains(result.Output, "[file]") {
		t.Error("Output missing '[file]' marker for files")
	}
}

func TestListDirectoryToolExecuteDirsBeforeFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a file that alphabetically comes before the directory
	if err := os.WriteFile(filepath.Join(dir, "aaa_file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a directory that alphabetically comes after the file
	if err := os.Mkdir(filepath.Join(dir, "zzz_dir"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Directories should appear before files in the output
	dirPos := strings.Index(result.Output, "zzz_dir")
	filePos := strings.Index(result.Output, "aaa_file.txt")

	if dirPos == -1 {
		t.Fatal("Output missing 'zzz_dir'")
	}
	if filePos == -1 {
		t.Fatal("Output missing 'aaa_file.txt'")
	}

	if dirPos > filePos {
		t.Errorf("Directories should be listed before files: dir at pos %d, file at pos %d", dirPos, filePos)
	}
}

func TestListDirectoryToolExecuteDirsSortedAlphabetically(t *testing.T) {
	dir := t.TempDir()

	dirs := []string{"charlie", "alpha", "bravo"}
	for _, d := range dirs {
		if err := os.Mkdir(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", d, err)
		}
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	alphaPos := strings.Index(result.Output, "alpha")
	bravoPos := strings.Index(result.Output, "bravo")
	charliePos := strings.Index(result.Output, "charlie")

	if alphaPos > bravoPos || bravoPos > charliePos {
		t.Errorf("Directories not sorted alphabetically: alpha=%d, bravo=%d, charlie=%d", alphaPos, bravoPos, charliePos)
	}
}

func TestListDirectoryToolExecuteFilesSortedAlphabetically(t *testing.T) {
	dir := t.TempDir()

	files := []string{"zebra.txt", "apple.txt", "mango.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("data"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	applePos := strings.Index(result.Output, "apple.txt")
	mangoPos := strings.Index(result.Output, "mango.txt")
	zebraPos := strings.Index(result.Output, "zebra.txt")

	if applePos > mangoPos || mangoPos > zebraPos {
		t.Errorf("Files not sorted alphabetically: apple=%d, mango=%d, zebra=%d", applePos, mangoPos, zebraPos)
	}
}

func TestListDirectoryToolExecuteNonexistentDirectory(t *testing.T) {
	tool := &ListDirectoryTool{}
	input := `{"path": "/tmp/this_directory_absolutely_does_not_exist_xyz_12345"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for nonexistent directory")
	}

	if !strings.Contains(strings.ToLower(result.Error), "not found") && !strings.Contains(strings.ToLower(result.Error), "not exist") {
		t.Errorf("Error = %q, expected it to mention 'not found' or 'not exist'", result.Error)
	}
}

func TestListDirectoryToolExecuteDefaultEmptyPath(t *testing.T) {
	tool := &ListDirectoryTool{}
	input := `{"path": ""}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// Empty path defaults to "." (current directory), which should work
	if result.IsError {
		t.Fatalf("Execute() returned error result for empty path (defaults to '.'): %s", result.Error)
	}

	// Should contain the "Directory:" header
	if !strings.Contains(result.Output, "Directory:") {
		t.Errorf("Output missing 'Directory:' header, got: %q", result.Output[:min(200, len(result.Output))])
	}
}

func TestListDirectoryToolExecuteEmptyJSONObject(t *testing.T) {
	tool := &ListDirectoryTool{}
	input := `{}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// Missing path defaults to "." which should succeed
	if result.IsError {
		t.Fatalf("Execute() returned error result for missing path field: %s", result.Error)
	}
}

func TestListDirectoryToolExecuteEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "empty") {
		t.Errorf("Output should indicate empty directory, got: %q", result.Output)
	}
}

func TestListDirectoryToolExecuteInvalidJSON(t *testing.T) {
	tool := &ListDirectoryTool{}
	result, err := tool.Execute(context.Background(), "not valid json {{")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for invalid JSON")
	}
	if !strings.Contains(strings.ToLower(result.Error), "parse") && !strings.Contains(strings.ToLower(result.Error), "failed") {
		if result.Error == "" {
			t.Error("Error is empty, expected parsing error message")
		}
	}
}

func TestListDirectoryToolExecutePathIsFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notadir.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, filePath)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when path is a file, not a directory")
	}

	if !strings.Contains(strings.ToLower(result.Error), "not a directory") {
		t.Errorf("Error = %q, expected it to mention 'not a directory'", result.Error)
	}
}

func TestListDirectoryToolExecuteShowsFileSize(t *testing.T) {
	dir := t.TempDir()
	content := "hello world!" // 12 bytes
	filePath := filepath.Join(dir, "sized.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should show file size (12 B)
	if !strings.Contains(result.Output, "12 B") {
		t.Errorf("Output should contain file size '12 B', got: %q", result.Output)
	}
}

func TestListDirectoryToolExecuteShowsSummaryCount(t *testing.T) {
	dir := t.TempDir()

	// Create 2 directories and 3 files
	for _, d := range []string{"dir1", "dir2"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
	}
	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should have summary: "2 directories, 3 files"
	if !strings.Contains(result.Output, "2 directories") {
		t.Errorf("Output should contain '2 directories', got: %q", result.Output)
	}
	if !strings.Contains(result.Output, "3 files") {
		t.Errorf("Output should contain '3 files', got: %q", result.Output)
	}
}

func TestListDirectoryToolExecuteDirectoriesHaveTrailingSlash(t *testing.T) {
	dir := t.TempDir()

	if err := os.Mkdir(filepath.Join(dir, "mydir"), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Directory names should have a trailing slash
	if !strings.Contains(result.Output, "mydir/") {
		t.Errorf("Output should show directory with trailing slash, got: %q", result.Output)
	}
}

func TestListDirectoryToolExecuteOnlyDirs(t *testing.T) {
	dir := t.TempDir()

	for _, d := range []string{"adir", "bdir", "cdir"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "3 directories, 0 files") {
		t.Errorf("Output should contain '3 directories, 0 files', got: %q", result.Output)
	}
}

func TestListDirectoryToolExecuteOnlyFiles(t *testing.T) {
	dir := t.TempDir()

	for _, f := range []string{"x.txt", "y.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("data"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "0 directories, 2 files") {
		t.Errorf("Output should contain '0 directories, 2 files', got: %q", result.Output)
	}
}

func TestListDirectoryToolExecuteNestedDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "parent", "child")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	// Create a file inside the nested directory
	if err := os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("deep"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// List the nested directory
	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, nested)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "deep.txt") {
		t.Error("Output missing 'deep.txt' in nested directory listing")
	}
}

func TestListDirectoryToolExecuteDoesNotRecurse(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory with a file inside
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "hidden_inside.txt"), []byte("hidden"), 0644); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	// Create a file at the top level
	if err := os.WriteFile(filepath.Join(dir, "top.txt"), []byte("top"), 0644); err != nil {
		t.Fatalf("failed to create top-level file: %v", err)
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	// Should list the subdirectory name but NOT its contents
	if !strings.Contains(result.Output, "subdir") {
		t.Error("Output should list the 'subdir' directory")
	}
	if strings.Contains(result.Output, "hidden_inside.txt") {
		t.Error("Output should NOT list contents of subdirectories (no recursion)")
	}
	if !strings.Contains(result.Output, "top.txt") {
		t.Error("Output should list 'top.txt' at the top level")
	}
}

func TestListDirectoryToolExecuteShowsDirectoryHeader(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "Directory:") {
		t.Errorf("Output should contain 'Directory:' header, got: %q", result.Output)
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 100, "100 B"},
		{"just under 1KB", 1023, "1023 B"},
		{"exactly 1KB", 1024, "1.0 KB"},
		{"kilobytes", 2048, "2.0 KB"},
		{"just under 1MB", 1024*1024 - 1, "1024.0 KB"},
		{"exactly 1MB", 1024 * 1024, "1.0 MB"},
		{"megabytes", 5 * 1024 * 1024, "5.0 MB"},
		{"just under 1GB", 1024*1024*1024 - 1, "1024.0 MB"},
		{"exactly 1GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"gigabytes", 2 * 1024 * 1024 * 1024, "2.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFileSize(tt.size)
			if got != tt.expected {
				t.Errorf("formatFileSize(%d) = %q, want %q", tt.size, got, tt.expected)
			}
		})
	}
}

func TestFormatFileSizeKBRange(t *testing.T) {
	// 1.5 KB = 1536 bytes
	got := formatFileSize(1536)
	if got != "1.5 KB" {
		t.Errorf("formatFileSize(1536) = %q, want %q", got, "1.5 KB")
	}
}

func TestListDirectoryToolExecuteLargerFileSize(t *testing.T) {
	dir := t.TempDir()

	// Create a file with known larger size
	content := strings.Repeat("x", 2048) // 2 KB
	filePath := filepath.Join(dir, "bigfile.bin")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "2.0 KB") {
		t.Errorf("Output should show '2.0 KB' for 2048-byte file, got: %q", result.Output)
	}
}

func TestListDirectoryToolExecuteManyEntries(t *testing.T) {
	dir := t.TempDir()

	// Create many files and directories
	for i := 0; i < 20; i++ {
		fname := fmt.Sprintf("file_%02d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, fname), []byte("x"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", fname, err)
		}
	}
	for i := 0; i < 5; i++ {
		dname := fmt.Sprintf("dir_%02d", i)
		if err := os.Mkdir(filepath.Join(dir, dname), 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dname, err)
		}
	}

	tool := &ListDirectoryTool{}
	input := fmt.Sprintf(`{"path": %q}`, dir)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !strings.Contains(result.Output, "5 directories, 20 files") {
		t.Errorf("Output should contain '5 directories, 20 files', got: %q", result.Output)
	}
}
