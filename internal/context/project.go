package projectctx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectInfo contains detected information about the current project.
type ProjectInfo struct {
	RootDir       string   // absolute path to project root
	Name          string   // project name (directory name)
	GitRoot       string   // git repository root (if any)
	GitBranch     string   // current git branch
	Languages     []string // detected programming languages
	HasReadme     bool
	ReadmeSnippet string   // first ~500 chars of README
	RootFiles     []string // files in root directory
}

// Detect gathers project context from the current working directory.
func DetectProjectInfo() *ProjectInfo {
	info := &ProjectInfo{}

	cwd, err := os.Getwd()
	if err != nil {
		return info
	}
	info.RootDir = cwd
	info.Name = filepath.Base(cwd)

	// Detect git
	info.detectGit()

	// Detect languages from file extensions
	info.detectLanguages()

	// Read README
	info.readReadme()

	// List root files (top-level only, limit to 50)
	info.listRootFiles()

	return info
}

func (p *ProjectInfo) detectGit() {
	// Use `git rev-parse --show-toplevel` to find git root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = p.RootDir
	if out, err := cmd.Output(); err == nil {
		p.GitRoot = strings.TrimSpace(string(out))
	}

	// Get current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = p.RootDir
	if out, err := cmd.Output(); err == nil {
		p.GitBranch = strings.TrimSpace(string(out))
	}
}

func (p *ProjectInfo) detectLanguages() {
	// Map file extensions to language names
	extMap := map[string]string{
		".go":    "Go",
		".rs":    "Rust",
		".py":    "Python",
		".js":    "JavaScript",
		".ts":    "TypeScript",
		".tsx":   "TypeScript/React",
		".jsx":   "JavaScript/React",
		".java":  "Java",
		".kt":    "Kotlin",
		".rb":    "Ruby",
		".php":   "PHP",
		".c":     "C",
		".cpp":   "C++",
		".h":     "C/C++",
		".cs":    "C#",
		".swift": "Swift",
		".m":     "Objective-C",
		".html":  "HTML",
		".css":   "CSS",
		".scss":  "SCSS",
		".sh":    "Shell",
		".bash":  "Shell",
		".sql":   "SQL",
		".md":    "Markdown",
		".yaml":  "YAML",
		".yml":   "YAML",
		".json":  "JSON",
		".toml":  "TOML",
		".xml":   "XML",
		".proto": "Protocol Buffers",
		".zig":   "Zig",
		".nim":   "Nim",
		".ex":    "Elixir",
		".erl":   "Erlang",
		".lua":   "Lua",
		".r":     "R",
		".jl":    "Julia",
	}

	seen := map[string]bool{}
	// Walk the directory (limit depth to avoid being too slow)
	_ = filepath.WalkDir(p.RootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and common non-source dirs
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "target" || name == "__pycache__" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			// Limit depth to 3 levels
			rel, _ := filepath.Rel(p.RootDir, path)
			if strings.Count(rel, string(filepath.Separator)) >= 3 {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if lang, ok := extMap[ext]; ok {
			if !seen[lang] {
				seen[lang] = true
				p.Languages = append(p.Languages, lang)
			}
		}
		return nil
	})
}

func (p *ProjectInfo) readReadme() {
	// Try common README filenames
	for _, name := range []string{"README.md", "README.txt", "README", "readme.md", "Readme.md"} {
		path := filepath.Join(p.RootDir, name)
		data, err := os.ReadFile(path)
		if err == nil {
			p.HasReadme = true
			content := string(data)
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			p.ReadmeSnippet = content
			return
		}
	}
}

func (p *ProjectInfo) listRootFiles() {
	entries, err := os.ReadDir(p.RootDir)
	if err != nil {
		return
	}
	count := 0
	for _, e := range entries {
		if count >= 50 {
			break
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			p.RootFiles = append(p.RootFiles, name+"/")
		} else {
			p.RootFiles = append(p.RootFiles, name)
		}
		count++
	}
}

// Summary returns a formatted summary string for use in system prompts.
func (p *ProjectInfo) Summary() string {
	var b strings.Builder

	b.WriteString("## Project Context\n")
	if p.Name != "" {
		b.WriteString("- Project: " + p.Name + "\n")
	}
	if p.RootDir != "" {
		b.WriteString("- Root: " + p.RootDir + "\n")
	}
	if p.GitRoot != "" {
		b.WriteString("- Git Repository: yes\n")
		if p.GitBranch != "" {
			b.WriteString("- Branch: " + p.GitBranch + "\n")
		}
	}
	if len(p.Languages) > 0 {
		b.WriteString("- Languages: " + strings.Join(p.Languages, ", ") + "\n")
	}
	if len(p.RootFiles) > 0 {
		b.WriteString("- Root files: " + strings.Join(p.RootFiles, ", ") + "\n")
	}
	if p.HasReadme {
		b.WriteString("\n### README\n")
		b.WriteString(p.ReadmeSnippet + "\n")
	}

	return b.String()
}
