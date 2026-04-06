package commands

import (
	"strings"
	"testing"

	"github.com/nomanqureshi/argo/internal/agent"
	"github.com/nomanqureshi/argo/internal/tools"
)

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if len(reg.commands) != 0 {
		t.Errorf("new registry should have 0 commands, got %d", len(reg.commands))
	}
	if len(reg.ordered) != 0 {
		t.Errorf("new registry should have 0 ordered entries, got %d", len(reg.ordered))
	}
}

func TestRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	cmd := &Command{
		Name:        "test",
		Aliases:     []string{"t", "tst"},
		Description: "a test command",
	}
	reg.Register(cmd)

	// Lookup by primary name.
	got := reg.Get("test")
	if got == nil {
		t.Fatal("Get(\"test\") returned nil")
	}
	if got.Name != "test" {
		t.Errorf("Get(\"test\").Name = %q, want %q", got.Name, "test")
	}

	// Lookup by alias.
	for _, alias := range []string{"t", "tst"} {
		got = reg.Get(alias)
		if got == nil {
			t.Fatalf("Get(%q) returned nil", alias)
		}
		if got.Name != "test" {
			t.Errorf("Get(%q).Name = %q, want %q", alias, got.Name, "test")
		}
	}

	// Lookup non-existent.
	if reg.Get("nope") != nil {
		t.Error("Get(\"nope\") should return nil")
	}
}

func TestAllSortedAlphabetically(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "zebra", Description: "z"})
	reg.Register(&Command{Name: "alpha", Description: "a"})
	reg.Register(&Command{Name: "middle", Description: "m"})

	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d commands, want 3", len(all))
	}
	expected := []string{"alpha", "middle", "zebra"}
	for i, cmd := range all {
		if cmd.Name != expected[i] {
			t.Errorf("All()[%d].Name = %q, want %q", i, cmd.Name, expected[i])
		}
	}
}

func TestAllDoesNotIncludeDuplicatesFromAliases(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "foo", Aliases: []string{"f", "fo"}})
	reg.Register(&Command{Name: "bar", Aliases: []string{"b"}})

	all := reg.All()
	if len(all) != 2 {
		t.Errorf("All() returned %d commands, want 2 (no alias duplicates)", len(all))
	}
}

// ---------------------------------------------------------------------------
// Match tests
// ---------------------------------------------------------------------------

func TestMatchByName(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "help", Description: "show help"})

	cmd, args := reg.Match("/help")
	if cmd == nil {
		t.Fatal("Match(\"/help\") returned nil command")
	}
	if cmd.Name != "help" {
		t.Errorf("matched command name = %q, want %q", cmd.Name, "help")
	}
	if len(args) != 0 {
		t.Errorf("args should be empty, got %v", args)
	}
}

func TestMatchByAlias(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "quit", Aliases: []string{"exit"}})

	cmd, _ := reg.Match("/exit")
	if cmd == nil {
		t.Fatal("Match(\"/exit\") returned nil")
	}
	if cmd.Name != "quit" {
		t.Errorf("matched command name = %q, want %q", cmd.Name, "quit")
	}
}

func TestMatchWithArgs(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "model"})

	cmd, args := reg.Match("/model gpt-4o")
	if cmd == nil {
		t.Fatal("Match returned nil")
	}
	if len(args) != 1 || args[0] != "gpt-4o" {
		t.Errorf("args = %v, want [\"gpt-4o\"]", args)
	}
}

func TestMatchWithMultipleArgs(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "resume"})

	cmd, args := reg.Match("/resume abc-123 extra")
	if cmd == nil {
		t.Fatal("Match returned nil")
	}
	if len(args) != 2 {
		t.Fatalf("args length = %d, want 2", len(args))
	}
	if args[0] != "abc-123" || args[1] != "extra" {
		t.Errorf("args = %v, want [\"abc-123\", \"extra\"]", args)
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "help"})

	cmd, _ := reg.Match("/HELP")
	if cmd == nil {
		t.Fatal("Match should be case-insensitive")
	}
	if cmd.Name != "help" {
		t.Errorf("matched command name = %q, want %q", cmd.Name, "help")
	}
}

func TestMatchNoSlashPrefix(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "help"})

	cmd, _ := reg.Match("help")
	if cmd != nil {
		t.Error("Match without leading / should return nil")
	}
}

func TestMatchEmptyInput(t *testing.T) {
	reg := NewRegistry()
	cmd, _ := reg.Match("")
	if cmd != nil {
		t.Error("Match on empty string should return nil")
	}
}

func TestMatchWhitespace(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "help"})

	cmd, _ := reg.Match("   /help   ")
	if cmd == nil {
		t.Fatal("Match should trim whitespace")
	}
}

func TestMatchUnknownCommand(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Command{Name: "help"})

	cmd, _ := reg.Match("/unknown")
	if cmd != nil {
		t.Error("Match for unknown command should return nil")
	}
}

// ---------------------------------------------------------------------------
// estimateCost tests
// ---------------------------------------------------------------------------

func TestEstimateCostSonnet(t *testing.T) {
	cost := estimateCost("claude-sonnet-4-20250514", 1_000_000, 1_000_000)
	// input: 3.0, output: 15.0 → total 18.0
	if cost < 17.99 || cost > 18.01 {
		t.Errorf("cost = %f, want ~18.0", cost)
	}
}

func TestEstimateCostOpus(t *testing.T) {
	cost := estimateCost("claude-opus-4-20250514", 1_000_000, 1_000_000)
	// input: 15.0, output: 75.0 → total 90.0
	if cost < 89.99 || cost > 90.01 {
		t.Errorf("cost = %f, want ~90.0", cost)
	}
}

func TestEstimateCostHaiku(t *testing.T) {
	cost := estimateCost("claude-3-5-haiku-20241022", 1_000_000, 1_000_000)
	// input: 0.25, output: 1.25 → total 1.50
	if cost < 1.49 || cost > 1.51 {
		t.Errorf("cost = %f, want ~1.50", cost)
	}
}

func TestEstimateCostGPT4oMini(t *testing.T) {
	cost := estimateCost("gpt-4o-mini", 1_000_000, 1_000_000)
	// input: 0.15, output: 0.60 → total 0.75
	if cost < 0.74 || cost > 0.76 {
		t.Errorf("cost = %f, want ~0.75", cost)
	}
}

func TestEstimateCostGPT4o(t *testing.T) {
	cost := estimateCost("gpt-4o", 1_000_000, 1_000_000)
	// input: 2.50, output: 10.0 → total 12.50
	if cost < 12.49 || cost > 12.51 {
		t.Errorf("cost = %f, want ~12.50", cost)
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	cost := estimateCost("some-unknown-model", 1_000_000, 1_000_000)
	// default: input 1.0, output 3.0 → total 4.0
	if cost < 3.99 || cost > 4.01 {
		t.Errorf("cost = %f, want ~4.0", cost)
	}
}

func TestEstimateCostZeroTokens(t *testing.T) {
	cost := estimateCost("claude-sonnet-4-20250514", 0, 0)
	if cost != 0 {
		t.Errorf("cost with zero tokens = %f, want 0", cost)
	}
}

func TestEstimateCostExported(t *testing.T) {
	// Verify exported function delegates correctly.
	got := EstimateCost("gpt-4o", 100, 200)
	want := estimateCost("gpt-4o", 100, 200)
	if got != want {
		t.Errorf("EstimateCost = %f, estimateCost = %f, should be equal", got, want)
	}
}

// ---------------------------------------------------------------------------
// RegisterBuiltins tests
// ---------------------------------------------------------------------------

func TestRegisterBuiltinsRegistersAllCommands(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	expectedNames := []string{
		"help", "clear", "new", "model", "compact",
		"tokens", "save", "history", "resume", "tools",
		"status", "quit",
	}
	for _, name := range expectedNames {
		if reg.Get(name) == nil {
			t.Errorf("builtin command %q not registered", name)
		}
	}
}

func TestRegisterBuiltinsAliases(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	aliases := map[string]string{
		"?":        "help",
		"reset":    "clear",
		"exit":     "quit",
		"cost":     "tokens",
		"usage":    "tokens",
		"sessions": "history",
		"continue": "resume",
		"info":     "status",
	}
	for alias, expectedName := range aliases {
		cmd := reg.Get(alias)
		if cmd == nil {
			t.Errorf("alias %q not registered", alias)
			continue
		}
		if cmd.Name != expectedName {
			t.Errorf("alias %q maps to %q, want %q", alias, cmd.Name, expectedName)
		}
	}
}

func TestAllReturnsNonHiddenForHelp(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	all := reg.All()
	for _, cmd := range all {
		if cmd.Hidden {
			t.Errorf("All() included hidden command %q", cmd.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Command execution tests (no agent/store needed)
// ---------------------------------------------------------------------------

func TestHelpCommandOutput(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("help")
	if cmd == nil {
		t.Fatal("help command not registered")
	}

	result := cmd.Execute(&Context{}, nil)
	if result.IsError {
		t.Errorf("help should not be an error")
	}
	if !strings.Contains(result.Output, "Available commands:") {
		t.Errorf("help output missing 'Available commands:', got: %s", result.Output)
	}
	// Should list some known commands.
	for _, name := range []string{"/clear", "/help", "/model", "/quit"} {
		if !strings.Contains(result.Output, name) {
			t.Errorf("help output missing %q", name)
		}
	}
}

func TestHelpCommandShowsAliases(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("help")
	result := cmd.Execute(&Context{}, nil)

	// Verify that at least some aliases show up in the help text.
	for _, alias := range []string{"/reset", "/exit", "/?", "/cost"} {
		if !strings.Contains(result.Output, alias) {
			t.Errorf("help output missing alias %q", alias)
		}
	}
}

func TestClearCommand(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("clear")
	result := cmd.Execute(&Context{}, nil)

	if result.IsError {
		t.Error("clear should not return error")
	}
	if !result.ClearChat {
		t.Error("clear should set ClearChat = true")
	}
	if result.StatusMsg != "Ready" {
		t.Errorf("clear StatusMsg = %q, want %q", result.StatusMsg, "Ready")
	}
	if !strings.Contains(result.Output, "cleared") {
		t.Errorf("clear output should mention 'cleared', got: %s", result.Output)
	}
}

func TestClearCommandAlias(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd, _ := reg.Match("/reset")
	if cmd == nil {
		t.Fatal("/reset should match clear command")
	}
	if cmd.Name != "clear" {
		t.Errorf("matched command = %q, want %q", cmd.Name, "clear")
	}
}

func TestNewCommand(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("new")
	result := cmd.Execute(&Context{}, nil)

	if !result.ClearChat {
		t.Error("new should set ClearChat = true")
	}
	if !strings.Contains(result.Output, "New conversation") {
		t.Errorf("new output should mention 'New conversation', got: %s", result.Output)
	}
}

func TestModelCommandDisplay(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("model")
	result := cmd.Execute(&Context{ModelName: "gpt-4o"}, nil)

	if result.IsError {
		t.Error("model display should not be an error")
	}
	if !strings.Contains(result.Output, "gpt-4o") {
		t.Errorf("model output should show current model name, got: %s", result.Output)
	}
	if result.NewModel != "" {
		t.Error("model display should not set NewModel")
	}
}

func TestModelCommandChange(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("model")
	result := cmd.Execute(&Context{ModelName: "gpt-4o"}, []string{"claude-sonnet-4-20250514"})

	if result.IsError {
		t.Error("model change should not be an error")
	}
	if result.NewModel != "claude-sonnet-4-20250514" {
		t.Errorf("NewModel = %q, want %q", result.NewModel, "claude-sonnet-4-20250514")
	}
	if !strings.Contains(result.Output, "claude-sonnet-4-20250514") {
		t.Errorf("model output should mention new model, got: %s", result.Output)
	}
}

func TestTokensCommand(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("tokens")
	result := cmd.Execute(&Context{
		ModelName:    "claude-sonnet-4-20250514",
		InputTokens:  5000,
		OutputTokens: 1000,
	}, nil)

	if result.IsError {
		t.Error("tokens should not return error")
	}
	if !strings.Contains(result.Output, "5000") {
		t.Errorf("tokens output should show input token count, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "1000") {
		t.Errorf("tokens output should show output token count, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "$") {
		t.Errorf("tokens output should show cost, got: %s", result.Output)
	}
}

func TestTokensAliases(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	for _, alias := range []string{"/cost", "/usage"} {
		cmd, _ := reg.Match(alias)
		if cmd == nil {
			t.Errorf("%s should match tokens command", alias)
			continue
		}
		if cmd.Name != "tokens" {
			t.Errorf("%s matched %q, want %q", alias, cmd.Name, "tokens")
		}
	}
}

func TestQuitCommand(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("quit")
	result := cmd.Execute(&Context{}, nil)

	if !result.Quit {
		t.Error("quit should set Quit = true")
	}
}

func TestExitAlias(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd, _ := reg.Match("/exit")
	if cmd == nil {
		t.Fatal("/exit should match quit command")
	}
	if cmd.Name != "quit" {
		t.Errorf("matched command = %q, want %q", cmd.Name, "quit")
	}
}

func TestToolsCommandEmpty(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("tools")
	result := cmd.Execute(&Context{ToolNames: nil}, nil)

	if result.IsError {
		t.Error("tools with no tools should not be an error")
	}
	if !strings.Contains(result.Output, "No tools") {
		t.Errorf("tools output should say no tools, got: %s", result.Output)
	}
}

func TestToolsCommandWithTools(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("tools")
	result := cmd.Execute(&Context{
		ToolNames: []string{"read_file", "write_file", "shell"},
	}, nil)

	if result.IsError {
		t.Error("tools should not return error")
	}
	if !strings.Contains(result.Output, "3") {
		t.Errorf("tools output should show count 3, got: %s", result.Output)
	}
	for _, name := range []string{"read_file", "write_file", "shell"} {
		if !strings.Contains(result.Output, name) {
			t.Errorf("tools output should list %q, got: %s", name, result.Output)
		}
	}
}

func TestStatusCommand(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("status")
	result := cmd.Execute(&Context{
		ModelName:    "claude-sonnet-4-20250514",
		Provider:     "anthropic",
		InputTokens:  1234,
		OutputTokens: 5678,
		ToolNames:    []string{"a", "b", "c"},
	}, nil)

	if result.IsError {
		t.Error("status should not return error")
	}
	for _, expected := range []string{
		"claude-sonnet-4-20250514",
		"anthropic",
		"1234",
		"5678",
		"3 registered",
	} {
		if !strings.Contains(result.Output, expected) {
			t.Errorf("status output missing %q, got: %s", expected, result.Output)
		}
	}
}

func TestStatusAlias(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd, _ := reg.Match("/info")
	if cmd == nil {
		t.Fatal("/info should match status command")
	}
	if cmd.Name != "status" {
		t.Errorf("matched command = %q, want %q", cmd.Name, "status")
	}
}

// ---------------------------------------------------------------------------
// Commands that need agent/store (error paths without them)
// ---------------------------------------------------------------------------

func TestSaveCommandNoAgent(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("save")
	result := cmd.Execute(&Context{Agent: nil}, nil)

	if !result.IsError {
		t.Error("save without agent should be an error")
	}
	if !strings.Contains(result.Output, "agent not initialized") {
		t.Errorf("save output should mention agent not initialized, got: %s", result.Output)
	}
}

func TestSaveCommandNoStore(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	ag := newTestAgent(t)
	cmd := reg.Get("save")
	result := cmd.Execute(&Context{Agent: ag, ThreadStore: nil}, nil)

	if !result.IsError {
		t.Error("save without store should be an error")
	}
	if !strings.Contains(result.Output, "persistence") {
		t.Errorf("save output should mention persistence, got: %s", result.Output)
	}
}

func TestCompactCommandNoAgent(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("compact")
	result := cmd.Execute(&Context{Agent: nil}, nil)

	if !result.IsError {
		t.Error("compact without agent should be an error")
	}
}

func TestCompactCommandWithAgent(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	ag := newTestAgent(t)
	cmd := reg.Get("compact")
	result := cmd.Execute(&Context{Agent: ag}, nil)

	if result.IsError {
		t.Errorf("compact should not error on empty thread, got: %s", result.Output)
	}
}

func TestResumeCommandNoArgs(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("resume")
	result := cmd.Execute(&Context{}, nil)

	if !result.IsError {
		t.Error("resume without args should be an error")
	}
	if !strings.Contains(result.Output, "Usage") {
		t.Errorf("resume output should show usage, got: %s", result.Output)
	}
}

func TestResumeCommandNoStore(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("resume")
	result := cmd.Execute(&Context{ThreadStore: nil}, []string{"some-id"})

	if !result.IsError {
		t.Error("resume without store should be an error")
	}
}

func TestHistoryCommandNoStore(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd := reg.Get("history")
	result := cmd.Execute(&Context{ThreadStore: nil}, nil)

	if !result.IsError {
		t.Error("history without store should be an error")
	}
}

func TestHistoryAlias(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd, _ := reg.Match("/sessions")
	if cmd == nil {
		t.Fatal("/sessions should match history command")
	}
	if cmd.Name != "history" {
		t.Errorf("matched command = %q, want %q", cmd.Name, "history")
	}
}

func TestResumeAlias(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd, _ := reg.Match("/continue")
	if cmd == nil {
		t.Fatal("/continue should match resume command")
	}
	if cmd.Name != "resume" {
		t.Errorf("matched command = %q, want %q", cmd.Name, "resume")
	}
}

// ---------------------------------------------------------------------------
// Integration: save + history + resume with a real SQLite store
// ---------------------------------------------------------------------------

func TestSaveHistoryResumeIntegration(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	store, cleanup := newTestStore(t)
	defer cleanup()

	ag := newTestAgent(t)

	// Save an empty thread.
	saveCmd := reg.Get("save")
	saveResult := saveCmd.Execute(&Context{Agent: ag, ThreadStore: store}, nil)
	if saveResult.IsError {
		t.Fatalf("save failed: %s", saveResult.Output)
	}
	if !strings.Contains(saveResult.Output, "Thread saved") {
		t.Errorf("save output should confirm save, got: %s", saveResult.Output)
	}

	threadID := ag.Thread().ID

	// List history.
	historyCmd := reg.Get("history")
	historyResult := historyCmd.Execute(&Context{ThreadStore: store}, nil)
	if historyResult.IsError {
		t.Fatalf("history failed: %s", historyResult.Output)
	}
	if !strings.Contains(historyResult.Output, threadID) {
		t.Errorf("history should contain thread ID %s, got: %s", threadID, historyResult.Output)
	}

	// Resume the thread.
	resumeCmd := reg.Get("resume")
	resumeResult := resumeCmd.Execute(&Context{Agent: ag, ThreadStore: store}, []string{threadID})
	if resumeResult.IsError {
		t.Fatalf("resume failed: %s", resumeResult.Output)
	}
	if resumeResult.ResumedThread == nil {
		t.Fatal("resume should return a ResumedThread")
	}
	if resumeResult.ResumedThread.ID != threadID {
		t.Errorf("resumed thread ID = %q, want %q", resumeResult.ResumedThread.ID, threadID)
	}
	if !resumeResult.ClearChat {
		t.Error("resume should set ClearChat = true")
	}
}

func TestResumeNonExistentThread(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	store, cleanup := newTestStore(t)
	defer cleanup()

	ag := newTestAgent(t)

	cmd := reg.Get("resume")
	result := cmd.Execute(&Context{Agent: ag, ThreadStore: store}, []string{"does-not-exist"})

	if !result.IsError {
		t.Error("resuming non-existent thread should be an error")
	}
	if !strings.Contains(result.Output, "Error loading thread") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestHistoryEmptyStore(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	store, cleanup := newTestStore(t)
	defer cleanup()

	cmd := reg.Get("history")
	result := cmd.Execute(&Context{ThreadStore: store}, nil)

	if result.IsError {
		t.Errorf("empty history should not be an error, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "No saved threads") {
		t.Errorf("empty history output should mention no saved threads, got: %s", result.Output)
	}
}

// ---------------------------------------------------------------------------
// Clear and new reset agent thread
// ---------------------------------------------------------------------------

func TestClearResetsAgentThread(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	ag := newTestAgent(t)
	originalThreadID := ag.Thread().ID

	cmd := reg.Get("clear")
	_ = cmd.Execute(&Context{Agent: ag}, nil)

	if ag.Thread().ID == originalThreadID {
		t.Error("clear should have reset the agent thread to a new ID")
	}
}

func TestNewResetsAgentThread(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	ag := newTestAgent(t)
	originalThreadID := ag.Thread().ID

	cmd := reg.Get("new")
	_ = cmd.Execute(&Context{Agent: ag}, nil)

	if ag.Thread().ID == originalThreadID {
		t.Error("new should have reset the agent thread to a new ID")
	}
}

// ---------------------------------------------------------------------------
// Model change sets model on agent
// ---------------------------------------------------------------------------

func TestModelCommandSetsOnAgent(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	ag := newTestAgent(t)

	cmd := reg.Get("model")
	result := cmd.Execute(&Context{Agent: ag, ModelName: "old-model"}, []string{"new-model"})

	if result.IsError {
		t.Errorf("model change should not error: %s", result.Output)
	}
	// We can't easily inspect the agent's internal model, but we verify
	// the result signals a model change.
	if result.NewModel != "new-model" {
		t.Errorf("NewModel = %q, want %q", result.NewModel, "new-model")
	}
}

// ---------------------------------------------------------------------------
// Compact with custom keep-recent
// ---------------------------------------------------------------------------

func TestCompactCommandWithKeepRecentArg(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	ag := newTestAgent(t)

	cmd := reg.Get("compact")
	// Should not error even when there are no messages.
	result := cmd.Execute(&Context{Agent: ag}, []string{"5"})

	if result.IsError {
		t.Errorf("compact should not error: %s", result.Output)
	}
}

func TestCompactCommandInvalidArg(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	ag := newTestAgent(t)

	cmd := reg.Get("compact")
	// Invalid arg should fall back to default keepRecent=10, not error.
	result := cmd.Execute(&Context{Agent: ag}, []string{"notanumber"})

	if result.IsError {
		t.Errorf("compact with invalid arg should not error: %s", result.Output)
	}
}

// ---------------------------------------------------------------------------
// Match edge cases
// ---------------------------------------------------------------------------

func TestMatchOnlySlash(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd, _ := reg.Match("/")
	// "/" with no command name should match nothing (empty name).
	if cmd != nil {
		t.Errorf("Match(\"/\") should return nil, got %q", cmd.Name)
	}
}

func TestMatchSlashWithSpaces(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)

	cmd, args := reg.Match("  /model   gpt-4o  ")
	if cmd == nil {
		t.Fatal("Match should handle leading/trailing whitespace")
	}
	if cmd.Name != "model" {
		t.Errorf("matched %q, want %q", cmd.Name, "model")
	}
	if len(args) != 1 || args[0] != "gpt-4o" {
		t.Errorf("args = %v, want [\"gpt-4o\"]", args)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestAgent creates a minimal Agent for testing (no provider, no tools).
func newTestAgent(t *testing.T) *agent.Agent {
	t.Helper()
	return agent.New(
		agent.Config{Model: "test-model", Provider: "test", SystemPrompt: "test"},
		nil,                  // no LLM provider needed for command tests
		tools.NewRegistry(),  // empty registry to avoid nil dereference
		nil,                  // no permission handler needed
	)
}

// newTestStore creates a temporary SQLiteStore and returns it along with
// a cleanup function.
func newTestStore(t *testing.T) (*agent.SQLiteStore, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := agent.NewSQLiteStoreAt(tmpDir + "/test-threads.db")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store, func() { _ = store.Close() }
}
