package projectctx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectContext holds detected information about the current project.
type ProjectContext struct {
	// GitBranch is the current git branch, if in a git repo.
	GitBranch string
	// GitRemote is the origin remote URL, if available.
	GitRemote string
	// Languages detected in the project (based on files present).
	Languages []string
	// RootFiles lists the top-level files in the project directory.
	RootFiles []string
	// ReadmeSnippet is the first ~30 lines of README.md, if present.
	ReadmeSnippet string
	// ProjectName is the basename of the working directory.
	ProjectName string
	// IsGitRepo indicates whether the project is a git repository.
	IsGitRepo bool
}

// Detect inspects the current working directory and returns a ProjectContext
// with as much information as can be gathered. Errors in individual detection
// steps are silently ignored so a partial context is always returned.
func Detect() *ProjectContext {
	ctx := &ProjectContext{}

	cwd, err := os.Getwd()
	if err != nil {
		return ctx
	}
	ctx.ProjectName = filepath.Base(cwd)

	// Detect git info
	detectGit(ctx)

	// Detect root files
	detectRootFiles(ctx, cwd)

	// Detect languages from root files and common config files
	detectLanguages(ctx, cwd)

	// Read README snippet
	detectReadme(ctx, cwd)

	return ctx
}

func detectGit(ctx *ProjectContext) {
	// Check if we're in a git repo
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return
	}
	ctx.IsGitRepo = true

	// Get current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if out, err := cmd.Output(); err == nil {
		ctx.GitBranch = strings.TrimSpace(string(out))
	}

	// Get remote origin URL
	cmd = exec.Command("git", "remote", "get-url", "origin")
	if out, err := cmd.Output(); err == nil {
		ctx.GitRemote = strings.TrimSpace(string(out))
	}
}

func detectRootFiles(ctx *ProjectContext, cwd string) {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files/dirs
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			ctx.RootFiles = append(ctx.RootFiles, name+"/")
		} else {
			ctx.RootFiles = append(ctx.RootFiles, name)
		}
	}
}

func detectLanguages(ctx *ProjectContext, cwd string) {
	langSignals := map[string][]string{
		"Go":         {"go.mod", "go.sum"},
		"Python":     {"pyproject.toml", "setup.py", "requirements.txt", "Pipfile"},
		"JavaScript": {"package.json", "tsconfig.json", ".eslintrc.json", ".eslintrc.js"},
		"TypeScript": {"tsconfig.json"},
		"Rust":       {"Cargo.toml", "Cargo.lock"},
		"Ruby":       {"Gemfile", "Rakefile", ".ruby-version"},
		"Java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
		"C#":         {"*.csproj", "*.sln"},
		"Swift":      {"Package.swift", "*.xcodeproj"},
		"Elixir":     {"mix.exs"},
		"PHP":        {"composer.json"},
		"Dart":       {"pubspec.yaml"},
	}

	seen := map[string]bool{}
	for lang, markers := range langSignals {
		for _, marker := range markers {
			if strings.Contains(marker, "*") {
				// Glob pattern
				matches, err := filepath.Glob(filepath.Join(cwd, marker))
				if err == nil && len(matches) > 0 && !seen[lang] {
					ctx.Languages = append(ctx.Languages, lang)
					seen[lang] = true
				}
			} else {
				if _, err := os.Stat(filepath.Join(cwd, marker)); err == nil && !seen[lang] {
					ctx.Languages = append(ctx.Languages, lang)
					seen[lang] = true
				}
			}
		}
	}
}

func detectReadme(ctx *ProjectContext, cwd string) {
	candidates := []string{"README.md", "readme.md", "README", "README.txt", "README.rst"}
	for _, name := range candidates {
		path := filepath.Join(cwd, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		maxLines := 30
		if len(lines) < maxLines {
			maxLines = len(lines)
		}
		ctx.ReadmeSnippet = strings.Join(lines[:maxLines], "\n")
		return
	}
}
