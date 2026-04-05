package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// mockSpawner implements AgentSpawner for testing.
type mockSpawner struct {
	result    string
	err       error
	called    bool
	sessionID string
	label     string
	message   string
}

func (m *mockSpawner) SpawnSubAgent(ctx context.Context, sessionID, label, message string) (string, error) {
	m.called = true
	m.sessionID = sessionID
	m.label = label
	m.message = message
	return m.result, m.err
}

func TestSpawnAgentToolName(t *testing.T) {
	tool := NewSpawnAgentTool(nil)
	if got := tool.Name(); got != "spawn_agent" {
		t.Errorf("Name() = %q, want %q", got, "spawn_agent")
	}
}

func TestSpawnAgentToolDescription(t *testing.T) {
	tool := NewSpawnAgentTool(nil)
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "sub-agent") && !strings.Contains(strings.ToLower(desc), "agent") {
		t.Errorf("Description() = %q, expected it to mention agents", desc)
	}
}

func TestSpawnAgentToolPermission(t *testing.T) {
	tool := NewSpawnAgentTool(nil)
	if got := tool.Permission(); got != PermissionRead {
		t.Errorf("Permission() = %v, want %v (PermissionRead)", got, PermissionRead)
	}
}

func TestSpawnAgentToolInputSchema(t *testing.T) {
	tool := NewSpawnAgentTool(nil)
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

	for _, field := range []string{"label", "message", "session_id"} {
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
	if !requiredSet["label"] {
		t.Error("InputSchema() required does not include 'label'")
	}
	if !requiredSet["message"] {
		t.Error("InputSchema() required does not include 'message'")
	}
}

func TestSpawnAgentToolImplementsToolInterface(t *testing.T) {
	var _ Tool = NewSpawnAgentTool(nil)
}

func TestSpawnAgentToolExecuteNilSpawner(t *testing.T) {
	tool := NewSpawnAgentTool(nil)
	input := `{"label": "Test", "message": "Do something"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when spawner is nil")
	}
	if !strings.Contains(strings.ToLower(result.Error), "not configured") {
		t.Errorf("Error = %q, expected it to mention 'not configured'", result.Error)
	}
}

func TestSpawnAgentToolExecuteMissingLabel(t *testing.T) {
	spawner := &mockSpawner{result: "ok"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "", "message": "Do something"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when label is empty")
	}
	if !strings.Contains(strings.ToLower(result.Error), "label") && !strings.Contains(strings.ToLower(result.Error), "required") {
		t.Errorf("Error = %q, expected it to mention 'label' or 'required'", result.Error)
	}
	if spawner.called {
		t.Error("SpawnSubAgent should not have been called with empty label")
	}
}

func TestSpawnAgentToolExecuteMissingMessage(t *testing.T) {
	spawner := &mockSpawner{result: "ok"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Test", "message": ""}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when message is empty")
	}
	if !strings.Contains(strings.ToLower(result.Error), "message") && !strings.Contains(strings.ToLower(result.Error), "required") {
		t.Errorf("Error = %q, expected it to mention 'message' or 'required'", result.Error)
	}
	if spawner.called {
		t.Error("SpawnSubAgent should not have been called with empty message")
	}
}

func TestSpawnAgentToolExecuteMissingBothLabelAndMessage(t *testing.T) {
	spawner := &mockSpawner{result: "ok"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "", "message": ""}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when both fields are empty")
	}
	if spawner.called {
		t.Error("SpawnSubAgent should not have been called with empty fields")
	}
}

func TestSpawnAgentToolExecuteCallsSpawnerCorrectly(t *testing.T) {
	spawner := &mockSpawner{result: "sub-agent completed the task successfully"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Research task", "message": "Find info about Go testing", "session_id": "sess-123"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !spawner.called {
		t.Fatal("SpawnSubAgent was not called")
	}

	if spawner.label != "Research task" {
		t.Errorf("spawner.label = %q, want %q", spawner.label, "Research task")
	}
	if spawner.message != "Find info about Go testing" {
		t.Errorf("spawner.message = %q, want %q", spawner.message, "Find info about Go testing")
	}
	if spawner.sessionID != "sess-123" {
		t.Errorf("spawner.sessionID = %q, want %q", spawner.sessionID, "sess-123")
	}

	if result.Output != "sub-agent completed the task successfully" {
		t.Errorf("Output = %q, want %q", result.Output, "sub-agent completed the task successfully")
	}
}

func TestSpawnAgentToolExecuteCallsSpawnerWithoutSessionID(t *testing.T) {
	spawner := &mockSpawner{result: "done"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Quick task", "message": "Do this thing"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !spawner.called {
		t.Fatal("SpawnSubAgent was not called")
	}

	if spawner.sessionID != "" {
		t.Errorf("spawner.sessionID = %q, want empty string (no session_id provided)", spawner.sessionID)
	}

	if spawner.label != "Quick task" {
		t.Errorf("spawner.label = %q, want %q", spawner.label, "Quick task")
	}
	if spawner.message != "Do this thing" {
		t.Errorf("spawner.message = %q, want %q", spawner.message, "Do this thing")
	}

	if result.Output != "done" {
		t.Errorf("Output = %q, want %q", result.Output, "done")
	}
}

func TestSpawnAgentToolExecuteHandlesSpawnerError(t *testing.T) {
	spawner := &mockSpawner{
		result: "",
		err:    errors.New("connection timeout"),
	}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Failing task", "message": "This will fail"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when spawner returns an error")
	}

	if !strings.Contains(result.Error, "connection timeout") {
		t.Errorf("Error = %q, expected it to contain the spawner error message", result.Error)
	}

	if !spawner.called {
		t.Error("SpawnSubAgent should have been called")
	}
}

func TestSpawnAgentToolExecuteHandlesSpawnerErrorWithContext(t *testing.T) {
	spawner := &mockSpawner{
		result: "",
		err:    fmt.Errorf("model overloaded: %w", errors.New("rate limit exceeded")),
	}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Rate limited task", "message": "Try this"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when spawner returns a wrapped error")
	}

	if !strings.Contains(result.Error, "model overloaded") {
		t.Errorf("Error = %q, expected it to contain wrapped error message", result.Error)
	}
}

func TestSpawnAgentToolExecuteInvalidJSON(t *testing.T) {
	spawner := &mockSpawner{result: "ok"}
	tool := NewSpawnAgentTool(spawner)
	result, err := tool.Execute(context.Background(), "not valid json {{{")
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
	if spawner.called {
		t.Error("SpawnSubAgent should not have been called with invalid JSON input")
	}
}

func TestSpawnAgentToolExecuteEmptyJSON(t *testing.T) {
	spawner := &mockSpawner{result: "ok"}
	tool := NewSpawnAgentTool(spawner)
	result, err := tool.Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for empty JSON object (missing required fields)")
	}
	if spawner.called {
		t.Error("SpawnSubAgent should not have been called with empty JSON")
	}
}

func TestSpawnAgentToolExecuteOnlyLabelProvided(t *testing.T) {
	spawner := &mockSpawner{result: "ok"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Some label"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when message is missing")
	}
	if spawner.called {
		t.Error("SpawnSubAgent should not have been called without a message")
	}
}

func TestSpawnAgentToolExecuteOnlyMessageProvided(t *testing.T) {
	spawner := &mockSpawner{result: "ok"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"message": "Do something"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result when label is missing")
	}
	if spawner.called {
		t.Error("SpawnSubAgent should not have been called without a label")
	}
}

func TestSpawnAgentToolExecuteSpawnerReturnsEmptyString(t *testing.T) {
	spawner := &mockSpawner{result: "", err: nil}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Empty result", "message": "Get empty response"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if result.Output != "" {
		t.Errorf("Output = %q, want empty string", result.Output)
	}

	if !spawner.called {
		t.Error("SpawnSubAgent should have been called")
	}
}

func TestSpawnAgentToolExecuteSpawnerReturnsLargeOutput(t *testing.T) {
	largeOutput := strings.Repeat("This is a long response from the sub-agent. ", 1000)
	spawner := &mockSpawner{result: largeOutput}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Big task", "message": "Generate lots of output"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if result.Output != largeOutput {
		t.Errorf("Output length = %d, want %d", len(result.Output), len(largeOutput))
	}
}

func TestSpawnAgentToolExecuteContextPassedToSpawner(t *testing.T) {
	var receivedCtx context.Context
	spawner := &mockSpawner{result: "ok"}

	// Wrap the spawner to capture context
	type contextCapture struct {
		inner *mockSpawner
	}
	cc := &contextCapture{inner: spawner}

	// We can't easily wrap the interface, so let's use a custom spawner
	customSpawner := &contextCapturingSpawner{result: "context ok"}
	tool := NewSpawnAgentTool(customSpawner)

	ctx := context.WithValue(context.Background(), contextKey("test-key"), "test-value")
	input := `{"label": "Context test", "message": "Check context"}`
	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	receivedCtx = customSpawner.receivedCtx
	if receivedCtx == nil {
		t.Fatal("context was not passed to spawner")
	}

	val, ok := receivedCtx.Value(contextKey("test-key")).(string)
	if !ok || val != "test-value" {
		t.Errorf("context value = %q, want %q", val, "test-value")
	}

	_ = cc // suppress unused variable
}

type contextKey string

type contextCapturingSpawner struct {
	result      string
	receivedCtx context.Context
}

func (s *contextCapturingSpawner) SpawnSubAgent(ctx context.Context, sessionID, label, message string) (string, error) {
	s.receivedCtx = ctx
	return s.result, nil
}

func TestSpawnAgentToolExecuteSpecialCharactersInInput(t *testing.T) {
	spawner := &mockSpawner{result: "handled special chars"}
	tool := NewSpawnAgentTool(spawner)
	input := `{"label": "Special chars: <>&\"'", "message": "Handle unicode: 日本語 🎉 and newlines\nand\ttabs"}`
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Error)
	}

	if !spawner.called {
		t.Fatal("SpawnSubAgent was not called")
	}

	if !strings.Contains(spawner.label, "Special chars") {
		t.Errorf("spawner.label = %q, expected it to contain special characters", spawner.label)
	}
}

func TestSpawnAgentToolExecuteNilSpawnerDoesNotPanic(t *testing.T) {
	tool := NewSpawnAgentTool(nil)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Execute() panicked: %v", r)
		}
	}()

	result, err := tool.Execute(context.Background(), `{"label": "Test", "message": "msg"}`)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Execute() should return error result for nil spawner, not panic")
	}
}

func TestNewSpawnAgentToolReturnsNonNil(t *testing.T) {
	tool := NewSpawnAgentTool(nil)
	if tool == nil {
		t.Fatal("NewSpawnAgentTool(nil) returned nil")
	}

	spawner := &mockSpawner{result: "ok"}
	tool2 := NewSpawnAgentTool(spawner)
	if tool2 == nil {
		t.Fatal("NewSpawnAgentTool(spawner) returned nil")
	}
}

func TestSpawnAgentToolExecuteMultipleCallsWithSameSpawner(t *testing.T) {
	spawner := &mockSpawner{result: "first call result"}
	tool := NewSpawnAgentTool(spawner)

	// First call
	input1 := `{"label": "Task 1", "message": "First message", "session_id": "s1"}`
	result1, err := tool.Execute(context.Background(), input1)
	if err != nil {
		t.Fatalf("First Execute() returned error: %v", err)
	}
	if result1.IsError {
		t.Fatalf("First Execute() returned error result: %s", result1.Error)
	}
	if spawner.label != "Task 1" {
		t.Errorf("After first call, spawner.label = %q, want %q", spawner.label, "Task 1")
	}
	if spawner.sessionID != "s1" {
		t.Errorf("After first call, spawner.sessionID = %q, want %q", spawner.sessionID, "s1")
	}

	// Update spawner result for second call
	spawner.result = "second call result"

	// Second call
	input2 := `{"label": "Task 2", "message": "Second message", "session_id": "s2"}`
	result2, err := tool.Execute(context.Background(), input2)
	if err != nil {
		t.Fatalf("Second Execute() returned error: %v", err)
	}
	if result2.IsError {
		t.Fatalf("Second Execute() returned error result: %s", result2.Error)
	}
	if result2.Output != "second call result" {
		t.Errorf("Second call Output = %q, want %q", result2.Output, "second call result")
	}
	if spawner.label != "Task 2" {
		t.Errorf("After second call, spawner.label = %q, want %q", spawner.label, "Task 2")
	}
	if spawner.sessionID != "s2" {
		t.Errorf("After second call, spawner.sessionID = %q, want %q", spawner.sessionID, "s2")
	}
}
