package prompt

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	projectctx "github.com/nomanqureshi/argo/internal/context"
)

// BuildSystemPrompt constructs a comprehensive system prompt that includes
// identity, system info, project context, available tools, and detailed guidelines.
func BuildSystemPrompt(toolNames []string) string {
	var b strings.Builder

	// ── 1. Identity ──────────────────────────────────────────────────────
	b.WriteString("You are Argo, a powerful AI coding agent running in the user's terminal.\n")
	b.WriteString("You have extensive knowledge of programming languages, frameworks, design patterns, and best practices.\n")
	b.WriteString("You help users understand, write, debug, and improve their code by combining conversation with direct action through tools.\n\n")

	// ── 2. System Information ────────────────────────────────────────────
	b.WriteString("## System Information\n")
	fmt.Fprintf(&b, "- Operating System: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if u, err := user.Current(); err == nil {
		fmt.Fprintf(&b, "- User: %s\n", u.Username)
		fmt.Fprintf(&b, "- Home Directory: %s\n", u.HomeDir)
	}
	if cwd, err := os.Getwd(); err == nil {
		fmt.Fprintf(&b, "- Working Directory: %s\n", cwd)
		fmt.Fprintf(&b, "- Project: %s\n", filepath.Base(cwd))
	}
	fmt.Fprintf(&b, "- Default Shell: %s\n", detectShell())
	b.WriteString("\n")

	// ── 3. Project Context ───────────────────────────────────────────────
	pctx := projectctx.Detect()
	if pctx != nil {
		b.WriteString("## Project Context\n")
		if pctx.ProjectName != "" {
			fmt.Fprintf(&b, "- Project Name: %s\n", pctx.ProjectName)
		}
		if pctx.IsGitRepo {
			b.WriteString("- Version Control: git\n")
			if pctx.GitBranch != "" {
				fmt.Fprintf(&b, "- Current Branch: %s\n", pctx.GitBranch)
			}
			if pctx.GitRemote != "" {
				fmt.Fprintf(&b, "- Remote Origin: %s\n", pctx.GitRemote)
			}
		}
		if len(pctx.Languages) > 0 {
			fmt.Fprintf(&b, "- Detected Languages: %s\n", strings.Join(pctx.Languages, ", "))
		}
		if len(pctx.RootFiles) > 0 {
			fmt.Fprintf(&b, "- Root Files: %s\n", strings.Join(pctx.RootFiles, ", "))
		}
		if pctx.ReadmeSnippet != "" {
			b.WriteString("\n### README (first 30 lines)\n")
			b.WriteString("```\n")
			b.WriteString(pctx.ReadmeSnippet)
			if !strings.HasSuffix(pctx.ReadmeSnippet, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n")
		}
		b.WriteString("\n")
	}

	// ── 4. Available Tools ───────────────────────────────────────────────
	b.WriteString("## Available Tools\n")
	if len(toolNames) > 0 {
		b.WriteString("You have access to the following tools to interact with the user's project:\n")
		for _, name := range toolNames {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
	} else {
		b.WriteString("No tools are currently registered.\n")
	}
	b.WriteString("\n")

	// ── 5. Communication Guidelines ──────────────────────────────────────
	b.WriteString("## Communication Guidelines\n")
	b.WriteString("- Be conversational but professional.\n")
	b.WriteString("- Refer to the user in the second person and yourself in the first person.\n")
	b.WriteString("- Format your responses in markdown. Use backticks for file, directory, function, and class names.\n")
	b.WriteString("- NEVER lie or fabricate information. If you are unsure, say so.\n")
	b.WriteString("- Refrain from apologizing excessively. Instead, explain the situation and proceed.\n")
	b.WriteString("- When presenting code changes, explain what you changed and why.\n")
	b.WriteString("- Keep responses focused and avoid unnecessary verbosity.\n")
	b.WriteString("- When multiple approaches exist, briefly mention alternatives but recommend the best one.\n")
	b.WriteString("\n")

	// ── 6. Tool Use Best Practices ───────────────────────────────────────
	b.WriteString("## Tool Use Best Practices\n")
	b.WriteString("- ALWAYS read a file before editing it so you understand its current contents and context.\n")
	b.WriteString("- Use search/grep tools to explore the codebase before making changes. Do not guess file paths.\n")
	b.WriteString("- When you need to find symbols, prefer grep over listing directories.\n")
	b.WriteString("- You can call multiple tools in sequence to complete a task. Plan your tool use carefully.\n")
	b.WriteString("- Always check the result of tool calls and handle errors gracefully.\n")
	b.WriteString("- After making changes, verify they work if possible (e.g., run tests, check for compilation errors).\n")
	b.WriteString("- Prefer making minimal, targeted changes over large rewrites unless the user requests otherwise.\n")
	b.WriteString("- When running shell commands, be mindful of the working directory and environment.\n")
	b.WriteString("- Do not run commands that start long-lived servers or block indefinitely without informing the user.\n")
	b.WriteString("- Use appropriate timeouts for potentially long-running operations.\n")
	b.WriteString("- Never hardcode secrets or API keys. Point out when the user should use environment variables.\n")
	b.WriteString("\n")

	// ── 7. Debugging Guidelines ──────────────────────────────────────────
	b.WriteString("## Debugging Guidelines\n")
	b.WriteString("- When debugging, only make code changes if you are confident they address the root cause.\n")
	b.WriteString("- Address the root cause, not just the symptoms.\n")
	b.WriteString("- Add descriptive logging statements and error messages to track variable and code state.\n")
	b.WriteString("- Add test functions and assertions to isolate the problem.\n")
	b.WriteString("- Read error messages carefully and trace them back to the source.\n")
	b.WriteString("- If you cannot determine the root cause, explain your findings and suggest next steps to the user.\n")
	b.WriteString("- When fixing diagnostics or compiler errors, make 1-2 attempts, then defer to the user.\n")
	b.WriteString("- Never delete or simplify working code just to resolve a diagnostic. Correct, mostly-complete code is more valuable than empty code that compiles.\n")

	return b.String()
}

// detectShell returns the user's default shell from the SHELL environment variable,
// falling back to a sensible default.
func detectShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}
