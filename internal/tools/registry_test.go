package tools

import (
	"context"
	"sync"
	"testing"
)

// mockTool implements the Tool interface for testing purposes.
type mockTool struct {
	name        string
	description string
	permission  PermissionLevel
	inputSchema map[string]any
	executeFunc func(ctx context.Context, input string) (*Result, error)
}

func (m *mockTool) Name() string               { return m.name }
func (m *mockTool) Description() string         { return m.description }
func (m *mockTool) Permission() PermissionLevel { return m.permission }
func (m *mockTool) InputSchema() map[string]any { return m.inputSchema }
func (m *mockTool) Execute(ctx context.Context, input string) (*Result, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}
	return &Result{Output: "mock output"}, nil
}

func newMockTool(name, description string, perm PermissionLevel) *mockTool {
	return &mockTool{
		name:        name,
		description: description,
		permission:  perm,
		inputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	tools := r.List()
	if len(tools) != 0 {
		t.Errorf("NewRegistry().List() returned %d tools, want 0", len(tools))
	}

	names := r.ToolNames()
	if len(names) != 0 {
		t.Errorf("NewRegistry().ToolNames() returned %d names, want 0", len(names))
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := newMockTool("test_tool", "A test tool", PermissionRead)

	r.Register(tool)

	got, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("Get(\"test_tool\") returned ok=false, want true")
	}
	if got.Name() != "test_tool" {
		t.Errorf("Get(\"test_tool\").Name() = %q, want %q", got.Name(), "test_tool")
	}
	if got.Description() != "A test tool" {
		t.Errorf("Get(\"test_tool\").Description() = %q, want %q", got.Description(), "A test tool")
	}
}

func TestRegistryGetUnknownTool(t *testing.T) {
	r := NewRegistry()

	got, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") returned ok=true, want false")
	}
	if got != nil {
		t.Errorf("Get(\"nonexistent\") returned non-nil tool: %v", got)
	}
}

func TestRegistryGetAfterRegisteringOtherTools(t *testing.T) {
	r := NewRegistry()
	r.Register(newMockTool("alpha", "Alpha tool", PermissionRead))
	r.Register(newMockTool("beta", "Beta tool", PermissionWrite))

	_, ok := r.Get("gamma")
	if ok {
		t.Error("Get(\"gamma\") returned ok=true for unregistered tool")
	}
}

func TestRegistryRegisterOverwrite(t *testing.T) {
	r := NewRegistry()

	original := newMockTool("my_tool", "original description", PermissionRead)
	r.Register(original)

	got, ok := r.Get("my_tool")
	if !ok {
		t.Fatal("Get(\"my_tool\") returned ok=false after first registration")
	}
	if got.Description() != "original description" {
		t.Errorf("Description = %q, want %q", got.Description(), "original description")
	}

	replacement := newMockTool("my_tool", "updated description", PermissionWrite)
	r.Register(replacement)

	got, ok = r.Get("my_tool")
	if !ok {
		t.Fatal("Get(\"my_tool\") returned ok=false after overwrite")
	}
	if got.Description() != "updated description" {
		t.Errorf("Description after overwrite = %q, want %q", got.Description(), "updated description")
	}
	if got.Permission() != PermissionWrite {
		t.Errorf("Permission after overwrite = %v, want %v", got.Permission(), PermissionWrite)
	}

	// Ensure the total count is still 1
	tools := r.List()
	if len(tools) != 1 {
		t.Errorf("List() returned %d tools after overwrite, want 1", len(tools))
	}
}

func TestRegistryListSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(newMockTool("charlie", "C", PermissionRead))
	r.Register(newMockTool("alpha", "A", PermissionRead))
	r.Register(newMockTool("bravo", "B", PermissionRead))

	tools := r.List()
	if len(tools) != 3 {
		t.Fatalf("List() returned %d tools, want 3", len(tools))
	}

	expected := []string{"alpha", "bravo", "charlie"}
	for i, tool := range tools {
		if tool.Name() != expected[i] {
			t.Errorf("List()[%d].Name() = %q, want %q", i, tool.Name(), expected[i])
		}
	}
}

func TestRegistryListEmpty(t *testing.T) {
	r := NewRegistry()
	tools := r.List()
	if tools == nil {
		t.Fatal("List() returned nil, want empty slice")
	}
	if len(tools) != 0 {
		t.Errorf("List() returned %d tools, want 0", len(tools))
	}
}

func TestRegistryToolNamesSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(newMockTool("zebra", "Z", PermissionRead))
	r.Register(newMockTool("apple", "A", PermissionRead))
	r.Register(newMockTool("mango", "M", PermissionRead))

	names := r.ToolNames()
	if len(names) != 3 {
		t.Fatalf("ToolNames() returned %d names, want 3", len(names))
	}

	expected := []string{"apple", "mango", "zebra"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("ToolNames()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestRegistryToolNamesEmpty(t *testing.T) {
	r := NewRegistry()
	names := r.ToolNames()
	if names == nil {
		t.Fatal("ToolNames() returned nil, want empty slice")
	}
	if len(names) != 0 {
		t.Errorf("ToolNames() returned %d names, want 0", len(names))
	}
}

func TestRegistryToLLMDefinitions(t *testing.T) {
	r := NewRegistry()

	schema1 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	}
	schema2 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string"},
		},
	}

	tool1 := &mockTool{
		name:        "write_file",
		description: "Write a file",
		permission:  PermissionWrite,
		inputSchema: schema1,
	}
	tool2 := &mockTool{
		name:        "grep",
		description: "Search files",
		permission:  PermissionRead,
		inputSchema: schema2,
	}

	r.Register(tool1)
	r.Register(tool2)

	defs := r.ToLLMDefinitions()
	if len(defs) != 2 {
		t.Fatalf("ToLLMDefinitions() returned %d definitions, want 2", len(defs))
	}

	// Should be sorted alphabetically: grep before write_file
	if defs[0].Name != "grep" {
		t.Errorf("defs[0].Name = %q, want %q", defs[0].Name, "grep")
	}
	if defs[1].Name != "write_file" {
		t.Errorf("defs[1].Name = %q, want %q", defs[1].Name, "write_file")
	}

	// Check fields
	if defs[0].Description != "Search files" {
		t.Errorf("defs[0].Description = %q, want %q", defs[0].Description, "Search files")
	}
	if defs[1].Description != "Write a file" {
		t.Errorf("defs[1].Description = %q, want %q", defs[1].Description, "Write a file")
	}

	// Verify InputSchema is set (non-nil)
	if defs[0].InputSchema == nil {
		t.Error("defs[0].InputSchema is nil, want non-nil")
	}
	if defs[1].InputSchema == nil {
		t.Error("defs[1].InputSchema is nil, want non-nil")
	}
}

func TestRegistryToLLMDefinitionsEmpty(t *testing.T) {
	r := NewRegistry()
	defs := r.ToLLMDefinitions()
	if defs == nil {
		t.Fatal("ToLLMDefinitions() returned nil, want empty slice")
	}
	if len(defs) != 0 {
		t.Errorf("ToLLMDefinitions() returned %d definitions, want 0", len(defs))
	}
}

func TestRegistryToLLMDefinitionsReturnType(t *testing.T) {
	r := NewRegistry()
	r.Register(newMockTool("test", "Test tool", PermissionRead))

	defs := r.ToLLMDefinitions()

	// Verify the return type matches []llm.ToolDefinition
	_ = defs
	if len(defs) != 1 {
		t.Fatalf("len(defs) = %d, want 1", len(defs))
	}
	if defs[0].Name != "test" {
		t.Errorf("defs[0].Name = %q, want %q", defs[0].Name, "test")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	const numGoroutines = 50
	const numOpsPerGoroutine = 20

	var wg sync.WaitGroup

	// Spawn goroutines that concurrently register tools
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				name := "tool_" + string(rune('A'+id%26)) + "_" + string(rune('0'+j%10))
				tool := newMockTool(name, "concurrent tool", PermissionRead)
				r.Register(tool)
			}
		}(i)
	}

	// Spawn goroutines that concurrently read
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				r.Get("tool_A_0")
				r.List()
				r.ToolNames()
				r.ToLLMDefinitions()
			}
		}(i)
	}

	// If there's a race condition, the race detector will catch it
	wg.Wait()

	// Basic sanity check: registry should have some tools
	tools := r.List()
	if len(tools) == 0 {
		t.Error("expected some tools after concurrent registration, got 0")
	}
}

func TestRegistryConcurrentRegisterAndGet(t *testing.T) {
	r := NewRegistry()

	// Pre-register a tool
	r.Register(newMockTool("stable_tool", "always here", PermissionRead))

	var wg sync.WaitGroup
	const iterations = 100

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			r.Register(newMockTool("stable_tool", "updated", PermissionWrite))
		}
	}()

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tool, ok := r.Get("stable_tool")
			if !ok {
				t.Error("Get returned ok=false for tool that should always exist")
				return
			}
			if tool == nil {
				t.Error("Get returned nil tool for tool that should always exist")
				return
			}
		}
	}()

	wg.Wait()
}

func TestRegistryMultipleToolTypes(t *testing.T) {
	r := NewRegistry()

	r.Register(newMockTool("read_tool", "reads things", PermissionRead))
	r.Register(newMockTool("write_tool", "writes things", PermissionWrite))
	r.Register(newMockTool("danger_tool", "dangerous things", PermissionDangerous))

	tools := r.List()
	if len(tools) != 3 {
		t.Fatalf("List() returned %d tools, want 3", len(tools))
	}

	// Verify sorted order: danger_tool, read_tool, write_tool
	expectedNames := []string{"danger_tool", "read_tool", "write_tool"}
	expectedPerms := []PermissionLevel{PermissionDangerous, PermissionRead, PermissionWrite}

	for i, tool := range tools {
		if tool.Name() != expectedNames[i] {
			t.Errorf("tools[%d].Name() = %q, want %q", i, tool.Name(), expectedNames[i])
		}
		if tool.Permission() != expectedPerms[i] {
			t.Errorf("tools[%d].Permission() = %v, want %v", i, tool.Permission(), expectedPerms[i])
		}
	}
}
