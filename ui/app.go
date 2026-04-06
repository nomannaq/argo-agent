package ui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nomanqureshi/argo/internal/agent"
	"github.com/nomanqureshi/argo/internal/commands"
	"github.com/nomanqureshi/argo/internal/llm"
	"github.com/nomanqureshi/argo/internal/permissions"
	"github.com/nomanqureshi/argo/internal/prompt"
	"github.com/nomanqureshi/argo/internal/tools"
	"github.com/nomanqureshi/argo/pkg/markdown"
)

// UI states
type appState int

const (
	stateInput      appState = iota // user is typing
	stateThinking                   // waiting for LLM
	statePermission                 // waiting for user to approve a tool call
)

// permissionInfo holds the details of a pending permission request.
type permissionInfo struct {
	toolName string
	level    tools.PermissionLevel
	input    string
}

// --- tea.Msg types ---

// agentEventMsg wraps an agent.Event received from the event channel.
type agentEventMsg struct {
	event agent.Event
}

// agentDoneMsg signals that the event channel has been closed.
type agentDoneMsg struct{}

// agentErrorMsg carries an error from the agent goroutine.
type agentErrorMsg struct {
	err error
}



// App is the main Bubbletea model for the Argo terminal UI.
type App struct {
	textarea textarea.Model
	viewport viewport.Model

	chatHistory     string // accumulated rendered chat
	currentResponse string // streaming assistant text

	agent  *agent.Agent
	state  appState
	width  int
	height int

	err       error
	statusMsg string

	totalInputTokens  int
	totalOutputTokens int

	// Cost tracking
	totalCost float64
	modelName string // to track which model for cost calculation
	provider  string // provider name (e.g. "anthropic")

	permissionPending *permissionInfo
	permissionCh      chan bool // channel for sending permission responses to the agent

	eventCh chan agent.Event // channel for receiving events from the agent
	cancel  context.CancelFunc

	threadStore   *agent.SQLiteStore         // for thread persistence
	policyHandler *permissions.PolicyHandler // for advanced permissions
	toolReg       *tools.Registry            // tool registry for listing tools
	cmdRegistry   *commands.Registry         // slash command registry
}



// NewApp creates a new App model suitable for use with tea.NewProgram.
// It takes a model name and provider name (e.g. "claude-sonnet-4-20250514", "anthropic").
func NewApp(model, provider string) tea.Model {
	// Resolve the API key from environment
	apiKey := resolveAPIKey(provider)

	// Create the LLM provider
	llmProvider, providerErr := llm.NewProvider(provider, apiKey)

	// Create tool registry and register core tools
	toolReg := tools.NewRegistry()
	toolReg.Register(&tools.ReadFileTool{})
	toolReg.Register(&tools.WriteFileTool{})
	toolReg.Register(&tools.ShellTool{})
	toolReg.Register(&tools.GrepTool{})
	toolReg.Register(&tools.FindFilesTool{})
	toolReg.Register(&tools.ListDirectoryTool{})
	toolReg.Register(&tools.EditFileTool{})

	// Permission channel for interactive approval flow
	permCh := make(chan bool, 1)

	// Event channel for receiving events from the agent goroutine.
	eventCh := make(chan agent.Event, 64)

	// Create the interactive permission handler.
	// The prompt function emits an EventPermissionRequest into the event channel
	// so the UI can display the permission prompt, then blocks on permCh until
	// the user responds with y/n.
	interactiveHandler := permissions.NewInteractiveHandler(
		func(toolName string, level tools.PermissionLevel, input string) (bool, error) {
			// This runs on the agent goroutine. Push a permission request event
			// into the event channel so the UI picks it up and shows the prompt.
			eventCh <- agent.Event{
				Type:      agent.EventPermissionRequest,
				ToolName:  toolName,
				ToolInput: input,
			}
			// Now block until the user responds via the permission channel.
			allowed, ok := <-permCh
			if !ok {
				return false, fmt.Errorf("permission channel closed")
			}
			return allowed, nil
		},
	)

	// Wrap the interactive handler in a PolicyHandler for advanced permissions.
	policyHandler := permissions.NewPolicyHandler(interactiveHandler)

	// Set up textarea
	ta := textarea.New()
	ta.Placeholder = "Type a message... (Ctrl+S to send, Ctrl+C to quit)"
	ta.CharLimit = 0 // no limit
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.Focus()
	ta.ShowLineNumbers = false

	// Set up viewport
	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to Argo! 🚀\n\nAn AI coding agent for your terminal.\n\nType a message and press Enter to send.\nType /help for available commands.\nPress Ctrl+C to quit.\n")

	// Create the command registry and register all built-in commands.
	cmdReg := commands.NewRegistry()
	commands.RegisterBuiltins(cmdReg)

	app := &App{
		textarea:      ta,
		viewport:      vp,
		chatHistory:   "",
		state:         stateInput,
		statusMsg:     "Ready",
		modelName:     model,
		provider:      provider,
		permissionCh:  permCh,
		eventCh:       eventCh,
		policyHandler: policyHandler,
		toolReg:       toolReg,
		cmdRegistry:   cmdReg,
	}

	if providerErr != nil {
		app.err = providerErr
		app.statusMsg = fmt.Sprintf("Provider error: %v", providerErr)
		return app
	}

	// Create the agent with an empty system prompt initially — we'll set it
	// after all tools (including spawn_agent and diagnostics) are registered.
	ag := agent.New(
		agent.Config{
			Model:    model,
			Provider: provider,
			APIKey:   apiKey,
			MaxTokens: 16384,
		},
		llmProvider,
		toolReg,
		policyHandler,
	)

	// Register spawn_agent and diagnostics tools (after agent creation so we can pass ag as the AgentSpawner)
	toolReg.Register(tools.NewSpawnAgentTool(ag))
	toolReg.Register(&tools.DiagnosticsTool{})

	// Now that ALL tools are registered, build the system prompt with the
	// complete tool list and inject it into the agent config.
	ag.SetSystemPrompt(prompt.BuildSystemPrompt(toolReg.ToolNames()))

	// Initialize thread store for thread persistence
	store, storeErr := agent.NewSQLiteStore()
	if storeErr != nil {
		// Log but don't fail — thread persistence is optional
		app.appendToChat(dimStyle.Render(fmt.Sprintf("Warning: thread persistence unavailable: %v", storeErr)) + "\n")
	} else {
		app.threadStore = store
		ag.SetStore(store)
	}

	// Set the event handler: push events into the channel for the UI to consume.
	ag.SetEventHandler(func(evt agent.Event) {
		app.eventCh <- evt
	})

	app.agent = ag

	return app
}

// resolveAPIKey looks up the API key for the given provider from env vars.
func resolveAPIKey(provider string) string {
	switch provider {
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	default:
		// Try a generic pattern
		key := strings.ToUpper(provider) + "_API_KEY"
		return os.Getenv(key)
	}
}

// --- tea.Model interface ---

func (a *App) Init() tea.Cmd {
	return textarea.Blink
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		headerHeight := 1
		statusBarHeight := 1
		inputHeight := 5 // textarea + padding
		vpHeight := a.height - headerHeight - statusBarHeight - inputHeight
		if vpHeight < 1 {
			vpHeight = 1
		}
		a.viewport.Width = a.width
		a.viewport.Height = vpHeight
		a.textarea.SetWidth(a.width)
		a.headerStyle() // update header width
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

	case agentEventMsg:
		return a.handleAgentEvent(msg.event)

	case agentDoneMsg:
		a.finishResponse()
		a.state = stateInput
		a.statusMsg = "Ready"
		return a, nil

	case agentErrorMsg:
		a.err = msg.err
		a.appendToChat(errorStyle.Render(fmt.Sprintf("Error: %v", msg.err)) + "\n\n")
		a.state = stateInput
		a.statusMsg = "Error — see above"
		return a, nil
	}

	// Pass through to textarea when in input state
	if a.state == stateInput {
		var cmd tea.Cmd
		a.textarea, cmd = a.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var sections []string

	// Header
	header := a.headerStyle().Render("🚀 Argo — AI Coding Agent")
	sections = append(sections, header)

	// Chat viewport
	sections = append(sections, a.viewport.View())

	// Status bar
	status := a.renderStatusBar()
	sections = append(sections, status)

	// Permission prompt (replaces input when active)
	if a.state == statePermission && a.permissionPending != nil {
		sections = append(sections, a.renderPermissionPrompt())
	} else {
		// Input area
		sections = append(sections, a.textarea.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// --- Key handling ---

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		// Cancel any running operation, then quit
		if a.cancel != nil {
			a.cancel()
		}
		if a.threadStore != nil {
			_ = a.threadStore.Close()
		}
		return a, tea.Quit

	case tea.KeyEscape:
		if a.state == stateThinking && a.cancel != nil {
			a.cancel()
			a.cancel = nil
			a.state = stateInput
			a.statusMsg = "Cancelled"
			a.appendToChat(dimStyle.Render("(cancelled)") + "\n\n")
			return a, nil
		}
		if a.state == statePermission {
			// Deny permission on escape
			a.permissionPending = nil
			a.state = stateThinking
			a.statusMsg = "Thinking..."
			a.permissionCh <- false
			return a, waitForEvent(a.eventCh)
		}
		return a, nil

	case tea.KeyCtrlS:
		return a.sendMessage()

	case tea.KeyEnter:
		// Send on Enter (plain enter without shift)
		if a.state == stateInput {
			return a.sendMessage()
		}
	}

	// Handle y/n for permission state
	if a.state == statePermission {
		switch msg.String() {
		case "y", "Y":
			a.permissionPending = nil
			a.state = stateThinking
			a.statusMsg = "Thinking..."
			a.appendToChat(dimStyle.Render("  ✓ Approved") + "\n")
			a.permissionCh <- true
			return a, waitForEvent(a.eventCh)
		case "n", "N":
			a.permissionPending = nil
			a.state = stateThinking
			a.statusMsg = "Thinking..."
			a.appendToChat(dimStyle.Render("  ✗ Denied") + "\n")
			a.permissionCh <- false
			return a, waitForEvent(a.eventCh)
		}
		// Ignore other keys in permission state
		return a, nil
	}

	// Pass through to textarea
	if a.state == stateInput {
		var cmd tea.Cmd
		a.textarea, cmd = a.textarea.Update(msg)
		return a, cmd
	}

	return a, nil
}

// handleSlashCommand processes a slash command and returns true if it was handled.
func (a *App) handleSlashCommand(input string) (tea.Model, tea.Cmd, bool) {
	cmd, args := a.cmdRegistry.Match(input)
	if cmd == nil {
		// No matching command found.
		parts := strings.Fields(input)
		if len(parts) > 0 {
			a.appendToChat(errorStyle.Render(fmt.Sprintf("Unknown command: %s", parts[0])) + "\n")
			a.appendToChat(dimStyle.Render("Type /help for available commands.") + "\n\n")
		}
		return a, nil, true
	}

	// Build the command context from current app state.
	var toolNames []string
	if a.toolReg != nil {
		toolNames = a.toolReg.ToolNames()
	}
	cmdCtx := &commands.Context{
		Ctx:          context.Background(),
		Agent:        a.agent,
		ThreadStore:  a.threadStore,
		ModelName:    a.modelName,
		Provider:     a.provider,
		InputTokens:  a.totalInputTokens,
		OutputTokens: a.totalOutputTokens,
		ToolNames:    toolNames,
	}

	result := cmd.Execute(cmdCtx, args)

	// Handle quit signal.
	if result.Quit {
		if a.cancel != nil {
			a.cancel()
		}
		if a.threadStore != nil {
			_ = a.threadStore.Close()
		}
		return a, tea.Quit, true
	}

	// Handle clear chat signal — reset UI accumulators and token counts.
	if result.ClearChat {
		a.chatHistory = ""
		a.currentResponse = ""
		a.totalInputTokens = 0
		a.totalOutputTokens = 0
		a.totalCost = 0
		a.viewport.SetContent("")
		a.viewport.GotoBottom()
	}

	// Handle resumed thread — replay messages in the chat viewport.
	if result.ResumedThread != nil {
		for _, msg := range result.ResumedThread.Messages() {
			switch msg.Role {
			case llm.RoleUser:
				a.chatHistory += userStyle.Render("You:") + " " + msg.Content + "\n\n"
			case llm.RoleAssistant:
				if msg.Content != "" {
					a.chatHistory += assistantStyle.Render("Argo:") + " " + msg.Content + "\n\n"
				}
			case llm.RoleTool:
				// Skip tool results in replay for cleanliness
			}
		}
		a.viewport.SetContent(a.chatHistory)
		a.viewport.GotoBottom()
	}

	// Handle model change.
	if result.NewModel != "" {
		a.modelName = result.NewModel
	}

	// Display output.
	if result.Output != "" {
		if result.IsError {
			a.appendToChat(errorStyle.Render(result.Output) + "\n\n")
		} else {
			a.appendToChat(dimStyle.Render(result.Output) + "\n\n")
		}
	}

	// Update status bar.
	if result.StatusMsg != "" {
		a.statusMsg = result.StatusMsg
	}

	return a, nil, true
}

// sendMessage sends the current textarea content to the agent.
func (a *App) sendMessage() (tea.Model, tea.Cmd) {
	if a.state != stateInput {
		return a, nil
	}

	msg := strings.TrimSpace(a.textarea.Value())
	if msg == "" {
		return a, nil
	}

	// Clear textarea first
	a.textarea.Reset()

	// Check for slash commands
	if strings.HasPrefix(msg, "/") {
		model, cmd, handled := a.handleSlashCommand(msg)
		if handled {
			return model, cmd
		}
	}

	if a.agent == nil {
		a.appendToChat(errorStyle.Render("Error: agent not initialized. Check your API key and provider.") + "\n\n")
		return a, nil
	}

	// Add user message to chat history
	a.appendToChat(userStyle.Render("You:") + " " + msg + "\n\n")

	// Set state to thinking
	a.state = stateThinking
	a.statusMsg = "Thinking..."
	a.currentResponse = ""

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// Start agent in a goroutine
	userMsg := msg
	go func() {
		err := a.agent.Run(ctx, userMsg)
		if err != nil {
			// The agent emits EventDone or EventError via the event handler,
			// but if Run itself returns an error that wasn't emitted, push it.
			select {
			case a.eventCh <- agent.Event{Type: agent.EventError, Error: err}:
			default:
			}
		}
	}()

	// Start listening for events
	return a, waitForEvent(a.eventCh)
}

// --- Agent event handling ---

func (a *App) handleAgentEvent(evt agent.Event) (tea.Model, tea.Cmd) {
	switch evt.Type {
	case agent.EventAssistantText:
		if a.currentResponse == "" {
			// Start of a new assistant message — add the prefix
			a.chatHistory += assistantStyle.Render("Argo:") + " "
		}
		a.currentResponse += evt.Content
		// Update the viewport with the streaming text
		a.viewport.SetContent(a.chatHistory + a.currentResponse)
		a.viewport.GotoBottom()
		return a, waitForEvent(a.eventCh)

	case agent.EventToolCallStart:
		a.finishResponse()
		toolDisplay := fmt.Sprintf("  🔧 %s", evt.ToolName)
		if evt.ToolInput != "" {
			// Show a truncated version of the input
			inputPreview := truncate(evt.ToolInput, 100)
			toolDisplay += fmt.Sprintf("(%s)", inputPreview)
		}
		a.appendToChat(toolCallStyle.Render(toolDisplay) + "\n")
		a.statusMsg = fmt.Sprintf("Running tool: %s", evt.ToolName)
		return a, waitForEvent(a.eventCh)

	case agent.EventToolResult:
		if evt.ToolResult != nil {
			var resultDisplay string
			if evt.ToolResult.IsError {
				resultDisplay = fmt.Sprintf("  ❌ %s: %s", evt.ToolName, truncate(evt.ToolResult.Error, 120))
				a.appendToChat(errorStyle.Render(resultDisplay) + "\n")
			} else {
				output := truncate(evt.ToolResult.Output, 200)
				resultDisplay = fmt.Sprintf("  ✅ %s: %s", evt.ToolName, output)
				a.appendToChat(toolResultStyle.Render(resultDisplay) + "\n")
			}
		}
		a.statusMsg = "Thinking..."
		return a, waitForEvent(a.eventCh)

	case agent.EventPermissionRequest:
		a.finishResponse()
		a.state = statePermission
		a.permissionPending = &permissionInfo{
			toolName: evt.ToolName,
			level:    tools.PermissionWrite, // default; the actual level is set by the handler
			input:    evt.ToolInput,
		}
		a.statusMsg = "Waiting for permission..."
		return a, nil // stop reading events until user responds

	case agent.EventUsageUpdate:
		if evt.Usage != nil {
			a.totalInputTokens += evt.Usage.InputTokens
			a.totalOutputTokens += evt.Usage.OutputTokens
			a.totalCost = commands.EstimateCost(a.modelName, a.totalInputTokens, a.totalOutputTokens)
			a.statusMsg = fmt.Sprintf("Tokens: %d in / %d out | ~$%.4f", a.totalInputTokens, a.totalOutputTokens, a.totalCost)
		}
		return a, waitForEvent(a.eventCh)

	case agent.EventSubagentStart:
		a.appendToChat(toolCallStyle.Render(fmt.Sprintf("  🤖 Sub-agent: %s", evt.Content)) + "\n")
		a.statusMsg = fmt.Sprintf("Sub-agent: %s", evt.Content)
		return a, waitForEvent(a.eventCh)

	case agent.EventSubagentEnd:
		a.appendToChat(toolResultStyle.Render(fmt.Sprintf("  ✅ Sub-agent completed: %s", evt.Content)) + "\n")
		a.statusMsg = "Thinking..."
		return a, waitForEvent(a.eventCh)

	case agent.EventDone:
		a.finishResponse()
		a.state = stateInput
		a.statusMsg = "Ready"
		return a, nil

	case agent.EventError:
		a.finishResponse()
		if evt.Error != nil {
			a.appendToChat(errorStyle.Render(fmt.Sprintf("Error: %v", evt.Error)) + "\n\n")
		}
		a.state = stateInput
		a.statusMsg = "Error — see above"
		return a, nil
	}

	// Unknown event type — keep listening
	return a, waitForEvent(a.eventCh)
}

// finishResponse flushes any accumulated streaming text into the chat history.
// When the response is complete, the raw text is passed through the markdown
// renderer to produce nicely formatted terminal output.
func (a *App) finishResponse() {
	if a.currentResponse != "" {
		// Render the completed response through the markdown renderer
		renderWidth := a.width - 4
		if renderWidth < 40 {
			renderWidth = 40
		}
		rendered := markdown.RenderToTerminal(a.currentResponse, renderWidth)
		a.chatHistory += rendered + "\n\n"
		a.currentResponse = ""
		a.viewport.SetContent(a.chatHistory)
		a.viewport.GotoBottom()
	}
}

// appendToChat adds text to the chat history and updates the viewport.
func (a *App) appendToChat(text string) {
	a.chatHistory += text
	a.viewport.SetContent(a.chatHistory + a.currentResponse)
	a.viewport.GotoBottom()
}

// --- Rendering helpers ---

func (a *App) headerStyle() lipgloss.Style {
	w := a.width
	if w == 0 {
		w = 80
	}
	return headerStyle.Width(w)
}

func (a *App) renderStatusBar() string {
	w := a.width
	if w == 0 {
		w = 80
	}

	var status string
	tokenInfo := ""
	if a.totalInputTokens > 0 || a.totalOutputTokens > 0 {
		tokenInfo = fmt.Sprintf("Tokens: %d in / %d out | ~$%.4f", a.totalInputTokens, a.totalOutputTokens, a.totalCost)
	}

	switch a.state {
	case stateThinking:
		if tokenInfo != "" {
			status = "⏳ Thinking... | " + tokenInfo
		} else {
			status = "⏳ Thinking..."
		}
	case statePermission:
		status = "⚠️  " + a.statusMsg
	default:
		if tokenInfo != "" {
			status = "Ready | " + tokenInfo
		} else if a.modelName != "" {
			status = "Ready | Model: " + a.modelName
		} else {
			status = "Ready"
		}
	}
	return statusBarStyle.Width(w).Render(status)
}

func (a *App) renderPermissionPrompt() string {
	if a.permissionPending == nil {
		return ""
	}

	w := a.width - 6 // account for border + padding
	if w < 20 {
		w = 20
	}

	var b strings.Builder
	fmt.Fprintf(&b, "⚠️  Permission required for tool: %s\n", a.permissionPending.toolName)
	fmt.Fprintf(&b, "Level: %s\n\n", a.permissionPending.level.String())

	if a.permissionPending.input != "" {
		inputPreview := truncate(a.permissionPending.input, 300)
		fmt.Fprintf(&b, "Input:\n%s\n\n", inputPreview)
	}

	b.WriteString("Allow this action? (y/n)")

	return permissionStyle.Width(w).Render(b.String())
}

// --- Command helpers ---

// waitForEvent returns a tea.Cmd that blocks until an event is received
// from the channel, then wraps it as an agentEventMsg. If the channel
// is closed, it returns agentDoneMsg.
func waitForEvent(ch <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return agentDoneMsg{}
		}
		return agentEventMsg{event: evt}
	}
}

// truncate shortens a string to maxLen, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	// Replace newlines for inline display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}
