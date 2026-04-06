package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// estimateCost estimates the cost in USD based on model name and token counts.
func estimateCost(model string, inputTokens, outputTokens int) float64 {
	lowerModel := strings.ToLower(model)

	var inputPricePerM, outputPricePerM float64

	switch {
	case strings.HasPrefix(lowerModel, "claude-sonnet-4") ||
		strings.HasPrefix(lowerModel, "claude-3.5-sonnet") ||
		strings.HasPrefix(lowerModel, "claude-3-5-sonnet"):
		inputPricePerM = 3.0
		outputPricePerM = 15.0
	case strings.HasPrefix(lowerModel, "claude-opus-4") ||
		strings.HasPrefix(lowerModel, "claude-3-opus"):
		inputPricePerM = 15.0
		outputPricePerM = 75.0
	case strings.HasPrefix(lowerModel, "claude-3.5-haiku") ||
		strings.HasPrefix(lowerModel, "claude-3-5-haiku") ||
		strings.HasPrefix(lowerModel, "claude-3-haiku"):
		inputPricePerM = 0.25
		outputPricePerM = 1.25
	case strings.HasPrefix(lowerModel, "gpt-4o-mini"):
		inputPricePerM = 0.15
		outputPricePerM = 0.60
	case strings.HasPrefix(lowerModel, "gpt-4o"):
		inputPricePerM = 2.50
		outputPricePerM = 10.0
	case strings.HasPrefix(lowerModel, "gpt-4-turbo"):
		inputPricePerM = 10.0
		outputPricePerM = 30.0
	default:
		inputPricePerM = 1.0
		outputPricePerM = 3.0
	}

	cost := (float64(inputTokens) / 1_000_000.0 * inputPricePerM) +
		(float64(outputTokens) / 1_000_000.0 * outputPricePerM)
	return cost
}

// EstimateCost is the exported version for use by the UI package.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	return estimateCost(model, inputTokens, outputTokens)
}

// RegisterBuiltins registers all built-in slash commands into the given registry.
func RegisterBuiltins(reg *Registry) {
	reg.Register(helpCommand(reg))
	reg.Register(clearCommand())
	reg.Register(newCommand())
	reg.Register(modelCommand())
	reg.Register(compactCommand())
	reg.Register(tokensCommand())
	reg.Register(saveCommand())
	reg.Register(historyCommand())
	reg.Register(resumeCommand())
	reg.Register(toolsCommand())
	reg.Register(statusCommand())
	reg.Register(quitCommand())
}

// ---------------------------------------------------------------------------
// /help
// ---------------------------------------------------------------------------

func helpCommand(reg *Registry) *Command {
	return &Command{
		Name:        "help",
		Aliases:     []string{"?"},
		Description: "Show available commands",
		Usage:       "/help",
		Args:        "none",
		Execute: func(_ *Context, _ []string) Result {
			var b strings.Builder
			b.WriteString("Available commands:\n")

			cmds := reg.All()
			for _, cmd := range cmds {
				if cmd.Hidden {
					continue
				}
				name := "/" + cmd.Name
				if len(cmd.Aliases) > 0 {
					aliases := make([]string, len(cmd.Aliases))
					for i, a := range cmd.Aliases {
						aliases[i] = "/" + a
					}
					name += " (" + strings.Join(aliases, ", ") + ")"
				}
				fmt.Fprintf(&b, "  %-28s — %s\n", name, cmd.Description)
			}
			return Result{Output: b.String()}
		},
	}
}

// ---------------------------------------------------------------------------
// /clear
// ---------------------------------------------------------------------------

func clearCommand() *Command {
	return &Command{
		Name:        "clear",
		Aliases:     []string{"reset"},
		Description: "Clear conversation history and context",
		Usage:       "/clear",
		Args:        "none",
		Execute: func(ctx *Context, _ []string) Result {
			if ctx.Agent != nil {
				ctx.Agent.ResetThread()
			}
			return Result{
				Output:    "Chat cleared.",
				ClearChat: true,
				StatusMsg: "Ready",
			}
		},
	}
}

// ---------------------------------------------------------------------------
// /new
// ---------------------------------------------------------------------------

func newCommand() *Command {
	return &Command{
		Name:        "new",
		Description: "Start a new conversation thread",
		Usage:       "/new",
		Args:        "none",
		Execute: func(ctx *Context, _ []string) Result {
			if ctx.Agent != nil {
				ctx.Agent.ResetThread()
			}
			return Result{
				Output:    "New conversation started.",
				ClearChat: true,
				StatusMsg: "Ready",
			}
		},
	}
}

// ---------------------------------------------------------------------------
// /model
// ---------------------------------------------------------------------------

func modelCommand() *Command {
	return &Command{
		Name:        "model",
		Description: "Display or change the current model",
		Usage:       "/model [name]",
		Args:        "optional",
		Execute: func(ctx *Context, args []string) Result {
			if len(args) > 0 {
				newModel := args[0]
				if ctx.Agent != nil {
					ctx.Agent.SetModel(newModel)
				}
				return Result{
					Output:   fmt.Sprintf("Model changed to: %s", newModel),
					NewModel: newModel,
					StatusMsg: fmt.Sprintf("Model: %s", newModel),
				}
			}
			return Result{
				Output: fmt.Sprintf("Current model: %s", ctx.ModelName),
			}
		},
	}
}

// ---------------------------------------------------------------------------
// /compact
// ---------------------------------------------------------------------------

func compactCommand() *Command {
	return &Command{
		Name:        "compact",
		Description: "Compact older messages to save context",
		Usage:       "/compact [N]",
		Args:        "optional",
		Execute: func(ctx *Context, args []string) Result {
			if ctx.Agent == nil {
				return Result{Output: "Error: agent not initialized.", IsError: true}
			}
			keepRecent := 10
			if len(args) > 0 {
				if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
					keepRecent = n
				}
			}
			beforeCount := ctx.Agent.Thread().MessageCount()
			err := ctx.Agent.CompactMessages(context.Background(), keepRecent)
			if err != nil {
				return Result{
					Output:  fmt.Sprintf("Error compacting: %v", err),
					IsError: true,
				}
			}
			afterCount := ctx.Agent.Thread().MessageCount()
			return Result{
				Output: fmt.Sprintf("Compacted: %d → %d messages (kept last %d)", beforeCount, afterCount, keepRecent),
			}
		},
	}
}

// ---------------------------------------------------------------------------
// /tokens  (aliases: /cost, /usage)
// ---------------------------------------------------------------------------

func tokensCommand() *Command {
	return &Command{
		Name:        "tokens",
		Aliases:     []string{"cost", "usage"},
		Description: "Show token usage and estimated cost",
		Usage:       "/tokens",
		Args:        "none",
		Execute: func(ctx *Context, _ []string) Result {
			cost := estimateCost(ctx.ModelName, ctx.InputTokens, ctx.OutputTokens)
			var b strings.Builder
			b.WriteString("Token usage:\n")
			fmt.Fprintf(&b, "  Input tokens:  %d\n", ctx.InputTokens)
			fmt.Fprintf(&b, "  Output tokens: %d\n", ctx.OutputTokens)
			fmt.Fprintf(&b, "  Estimated cost: ~$%.4f", cost)
			return Result{Output: b.String()}
		},
	}
}

// ---------------------------------------------------------------------------
// /save
// ---------------------------------------------------------------------------

func saveCommand() *Command {
	return &Command{
		Name:        "save",
		Description: "Save the current conversation",
		Usage:       "/save",
		Args:        "none",
		Execute: func(ctx *Context, _ []string) Result {
			if ctx.Agent == nil {
				return Result{Output: "Error: agent not initialized.", IsError: true}
			}
			if ctx.ThreadStore == nil {
				return Result{Output: "Error: thread persistence is not available.", IsError: true}
			}
			err := ctx.ThreadStore.SaveThread(ctx.Agent.Thread())
			if err != nil {
				return Result{
					Output:  fmt.Sprintf("Error saving thread: %v", err),
					IsError: true,
				}
			}
			threadID := ctx.Agent.Thread().ID
			msgCount := ctx.Agent.Thread().MessageCount()
			return Result{
				Output: fmt.Sprintf("Thread saved: %s (%d messages)", threadID, msgCount),
			}
		},
	}
}

// ---------------------------------------------------------------------------
// /history  (aliases: /sessions)
// ---------------------------------------------------------------------------

func historyCommand() *Command {
	return &Command{
		Name:        "history",
		Aliases:     []string{"sessions"},
		Description: "List recent saved conversations",
		Usage:       "/history",
		Args:        "none",
		Execute: func(ctx *Context, _ []string) Result {
			if ctx.ThreadStore == nil {
				return Result{Output: "Thread persistence is not available.", IsError: true}
			}
			summaries, err := ctx.ThreadStore.ListThreads()
			if err != nil {
				return Result{
					Output:  fmt.Sprintf("Error listing threads: %v", err),
					IsError: true,
				}
			}
			if len(summaries) == 0 {
				return Result{Output: "No saved threads found."}
			}
			var b strings.Builder
			b.WriteString("Recent conversations:\n")
			limit := 10
			if len(summaries) < limit {
				limit = len(summaries)
			}
			for i, s := range summaries[:limit] {
				title := s.Title
				if len(title) > 60 {
					title = title[:60] + "…"
				}
				fmt.Fprintf(&b, "  %d. [%s] %s (%d msgs)\n",
					i+1, s.ID, title, s.MessageCount)
			}
			b.WriteString("\nUse /resume <id> to resume a conversation.")
			return Result{Output: b.String()}
		},
	}
}

// ---------------------------------------------------------------------------
// /resume  (aliases: /continue)
// ---------------------------------------------------------------------------

func resumeCommand() *Command {
	return &Command{
		Name:        "resume",
		Aliases:     []string{"continue"},
		Description: "Resume a saved conversation",
		Usage:       "/resume <id>",
		Args:        "required",
		Execute: func(ctx *Context, args []string) Result {
			if len(args) < 1 {
				return Result{Output: "Usage: /resume <thread-id>", IsError: true}
			}
			if ctx.ThreadStore == nil {
				return Result{Output: "Thread persistence is not available.", IsError: true}
			}
			threadID := args[0]
			thread, err := ctx.ThreadStore.LoadThread(threadID)
			if err != nil {
				return Result{
					Output:  fmt.Sprintf("Error loading thread: %v", err),
					IsError: true,
				}
			}
			if ctx.Agent != nil {
				ctx.Agent.SetThread(thread)
			}
			return Result{
				Output:        fmt.Sprintf("Resumed thread %s (%d messages)", threadID, thread.MessageCount()),
				ClearChat:     true,
				ResumedThread: thread,
				StatusMsg:     "Ready",
			}
		},
	}
}

// ---------------------------------------------------------------------------
// /tools
// ---------------------------------------------------------------------------

func toolsCommand() *Command {
	return &Command{
		Name:        "tools",
		Description: "List available tools",
		Usage:       "/tools",
		Args:        "none",
		Execute: func(ctx *Context, _ []string) Result {
			if len(ctx.ToolNames) == 0 {
				return Result{Output: "No tools registered."}
			}
			var b strings.Builder
			fmt.Fprintf(&b, "Available tools (%d):\n", len(ctx.ToolNames))
			for _, name := range ctx.ToolNames {
				fmt.Fprintf(&b, "  • %s\n", name)
			}
			return Result{Output: b.String()}
		},
	}
}

// ---------------------------------------------------------------------------
// /status  (aliases: /info)
// ---------------------------------------------------------------------------

func statusCommand() *Command {
	return &Command{
		Name:        "status",
		Aliases:     []string{"info"},
		Description: "Show current session info",
		Usage:       "/status",
		Args:        "none",
		Execute: func(ctx *Context, _ []string) Result {
			var b strings.Builder
			b.WriteString("Session info:\n")
			fmt.Fprintf(&b, "  Model:         %s\n", ctx.ModelName)
			fmt.Fprintf(&b, "  Provider:      %s\n", ctx.Provider)

			if ctx.Agent != nil {
				thread := ctx.Agent.Thread()
				fmt.Fprintf(&b, "  Thread ID:     %s\n", thread.ID)
				fmt.Fprintf(&b, "  Messages:      %d\n", thread.MessageCount())
			}

			fmt.Fprintf(&b, "  Input tokens:  %d\n", ctx.InputTokens)
			fmt.Fprintf(&b, "  Output tokens: %d\n", ctx.OutputTokens)

			cost := estimateCost(ctx.ModelName, ctx.InputTokens, ctx.OutputTokens)
			fmt.Fprintf(&b, "  Est. cost:     ~$%.4f\n", cost)
			fmt.Fprintf(&b, "  Tools:         %d registered", len(ctx.ToolNames))
			return Result{Output: b.String()}
		},
	}
}

// ---------------------------------------------------------------------------
// /quit  (aliases: /exit)
// ---------------------------------------------------------------------------

func quitCommand() *Command {
	return &Command{
		Name:        "quit",
		Aliases:     []string{"exit"},
		Description: "Quit the application",
		Usage:       "/quit",
		Args:        "none",
		Execute: func(_ *Context, _ []string) Result {
			return Result{Quit: true}
		},
	}
}
