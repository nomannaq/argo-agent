package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	diagnosticsTimeoutSeconds = 60
	diagnosticsMaxOutput      = 100 * 1024 // 100KB
)

// DiagnosticsTool runs language-specific diagnostic/lint commands to check code
// for errors and warnings.
type DiagnosticsTool struct{}

type diagnosticsInput struct {
	Path     string `json:"path"`
	Language string `json:"language"`
}

func (t *DiagnosticsTool) Name() string {
	return "diagnostics"
}

func (t *DiagnosticsTool) Description() string {
	return "Run language-specific diagnostic/lint commands to check code for errors and warnings. " +
		"Auto-detects the project language based on marker files (go.mod, package.json, Cargo.toml, etc.) " +
		"or uses an explicit language hint. Returns diagnostic output even when the linter reports issues."
}

func (t *DiagnosticsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Optional file or directory path to check diagnostics for. If not provided, runs project-wide diagnostics.",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Optional language hint to select the right diagnostic tool. Auto-detected if not provided.",
			},
		},
	}
}

func (t *DiagnosticsTool) Permission() PermissionLevel {
	return PermissionRead
}

func (t *DiagnosticsTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params diagnosticsInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Error:   fmt.Sprintf("failed to parse input: %s", err),
			IsError: true,
		}, nil
	}

	targetPath := params.Path
	language := strings.ToLower(strings.TrimSpace(params.Language))

	// Determine the working directory for detection and command execution.
	workDir, err := resolveWorkDir(targetPath)
	if err != nil {
		return &Result{
			Error:   fmt.Sprintf("failed to resolve working directory: %s", err),
			IsError: true,
		}, nil
	}

	// Detect language if not explicitly provided.
	if language == "" {
		language = detectLanguage(workDir, targetPath)
	}

	// Build the diagnostic command for the detected language.
	cmdName, cmdArgs, dir := buildDiagnosticCommand(language, targetPath, workDir)
	if cmdName == "" {
		return &Result{
			Output:  "No diagnostic tool detected for this project.",
			IsError: false,
		}, nil
	}

	// Run with a 60-second timeout.
	timeout := diagnosticsTimeoutSeconds * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, cmdName, cmdArgs...)
	cmd.Dir = dir

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	runErr := cmd.Run()

	// Capture and potentially truncate output.
	raw := combined.Bytes()
	truncated := false
	if len(raw) > diagnosticsMaxOutput {
		raw = raw[:diagnosticsMaxOutput]
		truncated = true
	}

	outputStr := string(raw)
	if truncated {
		outputStr += "\n\n... [output truncated at 100KB]"
	}

	// Format the header showing what command was run.
	fullCmd := cmdName + " " + strings.Join(cmdArgs, " ")
	header := fmt.Sprintf("Command: %s\nDirectory: %s\n", fullCmd, dir)

	// Handle timeout.
	if cmdCtx.Err() == context.DeadlineExceeded {
		return &Result{
			Output:  header + "\n" + outputStr,
			Error:   fmt.Sprintf("diagnostic command timed out after %d seconds", diagnosticsTimeoutSeconds),
			IsError: true,
		}, nil
	}

	// Exit code 0 means clean.
	if runErr == nil {
		body := "No errors or warnings found."
		if strings.TrimSpace(outputStr) != "" {
			body = outputStr
		}
		return &Result{
			Output:  header + "\n" + body,
			IsError: false,
		}, nil
	}

	// Non-zero exit — diagnostic output is still valuable, not treated as an
	// error from the tool's perspective.
	if strings.TrimSpace(outputStr) == "" {
		outputStr = fmt.Sprintf("Command exited with: %s", runErr)
	}

	return &Result{
		Output:  header + "\n" + outputStr,
		IsError: false,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveWorkDir determines the directory context for running diagnostics.
func resolveWorkDir(targetPath string) (string, error) {
	if targetPath != "" {
		abs, err := filepath.Abs(targetPath)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return abs, nil
		}
		return filepath.Dir(abs), nil
	}

	return os.Getwd()
}

// detectLanguage inspects the directory tree (walking upward) for known marker
// files and falls back to the file extension of targetPath.
func detectLanguage(workDir, targetPath string) string {
	// Walk up from workDir looking for project marker files.
	dir := workDir
	for {
		if lang := detectByMarkerFiles(dir); lang != "" {
			return lang
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback: use file extension.
	if targetPath != "" {
		if lang := detectByExtension(targetPath); lang != "" {
			return lang
		}
	}

	return ""
}

// detectByMarkerFiles checks a single directory for language marker files.
func detectByMarkerFiles(dir string) string {
	markers := []struct {
		file string
		lang string
	}{
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"tsconfig.json", "typescript"},
		{"package.json", "javascript"},
		{"pyproject.toml", "python"},
		{"setup.py", "python"},
		{"requirements.txt", "python"},
		{"Gemfile", "ruby"},
	}

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			return m.lang
		}
	}
	return ""
}

// detectByExtension maps common file extensions to languages.
func detectByExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	default:
		return ""
	}
}

// buildDiagnosticCommand returns the command name, arguments, and working
// directory for the given language. An empty cmdName signals that no suitable
// command was found.
func buildDiagnosticCommand(language, targetPath, workDir string) (cmdName string, args []string, dir string) {
	switch language {
	case "go":
		return buildGoCommand(targetPath, workDir)
	case "python":
		return buildPythonCommand(targetPath, workDir)
	case "javascript", "typescript":
		return buildJSTSCommand(language, targetPath, workDir)
	case "rust":
		return buildRustCommand(workDir)
	case "ruby":
		return buildRubyCommand(targetPath, workDir)
	default:
		return "", nil, ""
	}
}

// --- Go ---

func buildGoCommand(targetPath, workDir string) (string, []string, string) {
	// Find the module root (directory containing go.mod) so `go vet` works
	// correctly regardless of where targetPath is.
	modRoot := findAncestorWithFile(workDir, "go.mod")
	if modRoot == "" {
		modRoot = workDir
	}

	if targetPath != "" {
		info, err := os.Stat(targetPath)
		if err == nil && !info.IsDir() {
			// go vet on the package that contains the file.
			rel, relErr := filepath.Rel(modRoot, filepath.Dir(mustAbs(targetPath)))
			if relErr == nil && rel != "" {
				return "go", []string{"vet", "./" + rel}, modRoot
			}
		}
		return "go", []string{"vet", targetPath}, modRoot
	}

	return "go", []string{"vet", "./..."}, modRoot
}

// --- Python ---

func buildPythonCommand(targetPath, workDir string) (string, []string, string) {
	// For a single file, use py_compile.
	if targetPath != "" {
		abs := mustAbs(targetPath)
		info, err := os.Stat(abs)
		if err == nil && !info.IsDir() {
			return "python", []string{"-m", "py_compile", abs}, workDir
		}
	}

	// Project-wide: prefer ruff, then flake8, then py_compile is not useful.
	if _, err := exec.LookPath("ruff"); err == nil {
		target := "."
		if targetPath != "" {
			target = targetPath
		}
		return "ruff", []string{"check", target}, workDir
	}

	if _, err := exec.LookPath("flake8"); err == nil {
		target := "."
		if targetPath != "" {
			target = targetPath
		}
		return "flake8", []string{target}, workDir
	}

	// Last resort for project-wide: not much we can do generically.
	return "python", []string{"-m", "py_compile"}, workDir
}

// --- JavaScript / TypeScript ---

func buildJSTSCommand(language, targetPath, workDir string) (string, []string, string) {
	projRoot := findAncestorWithFile(workDir, "package.json")
	if projRoot == "" {
		projRoot = workDir
	}

	// If TypeScript or tsconfig exists, prefer tsc --noEmit for type checking.
	if language == "typescript" || fileExists(filepath.Join(projRoot, "tsconfig.json")) {
		if targetPath == "" {
			return "npx", []string{"tsc", "--noEmit"}, projRoot
		}
	}

	// Use eslint for a specific path (or as fallback for JS).
	target := "."
	if targetPath != "" {
		target = targetPath
	}
	return "npx", []string{"eslint", target}, projRoot
}

// --- Rust ---

func buildRustCommand(workDir string) (string, []string, string) {
	cargoRoot := findAncestorWithFile(workDir, "Cargo.toml")
	if cargoRoot == "" {
		cargoRoot = workDir
	}
	return "cargo", []string{"check"}, cargoRoot
}

// --- Ruby ---

func buildRubyCommand(targetPath, workDir string) (string, []string, string) {
	if targetPath != "" {
		abs := mustAbs(targetPath)
		info, err := os.Stat(abs)
		if err == nil && !info.IsDir() {
			return "ruby", []string{"-c", abs}, workDir
		}
	}

	// Project-wide: use rubocop if available, otherwise syntax-check isn't
	// very useful across a whole tree.
	if _, err := exec.LookPath("rubocop"); err == nil {
		return "rubocop", nil, workDir
	}

	return "ruby", []string{"-c"}, workDir
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

// findAncestorWithFile walks up from dir looking for a directory containing
// the named file. Returns the directory path or "".
func findAncestorWithFile(dir, filename string) string {
	dir = mustAbs(dir)
	for {
		if fileExists(filepath.Join(dir, filename)) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// mustAbs returns the absolute version of path, falling back to path itself on
// error (should never happen in practice).
func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
