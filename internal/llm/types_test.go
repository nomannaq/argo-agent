package llm

import (
	"fmt"
	"testing"
)

func TestRoleConstants(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		expected string
	}{
		{"RoleUser", RoleUser, "user"},
		{"RoleAssistant", RoleAssistant, "assistant"},
		{"RoleSystem", RoleSystem, "system"},
		{"RoleTool", RoleTool, "tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.role) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.role))
			}
		})
	}
}

func TestRoleIsDistinct(t *testing.T) {
	roles := []Role{RoleUser, RoleAssistant, RoleSystem, RoleTool}
	seen := make(map[Role]bool)
	for _, r := range roles {
		if seen[r] {
			t.Errorf("duplicate role value: %q", r)
		}
		seen[r] = true
	}
}

func TestMessageBasicFields(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "Hello, world!",
	}
	if msg.Role != RoleUser {
		t.Errorf("expected role %q, got %q", RoleUser, msg.Role)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("expected content %q, got %q", "Hello, world!", msg.Content)
	}
	if msg.ToolCalls != nil {
		t.Errorf("expected nil ToolCalls, got %v", msg.ToolCalls)
	}
	if msg.ToolCallID != "" {
		t.Errorf("expected empty ToolCallID, got %q", msg.ToolCallID)
	}
}

func TestMessageCanHoldToolCalls(t *testing.T) {
	tc1 := ToolCall{
		ID:        "call_1",
		Name:      "read_file",
		Arguments: `{"path": "/tmp/test.txt"}`,
	}
	tc2 := ToolCall{
		ID:        "call_2",
		Name:      "write_file",
		Arguments: `{"path": "/tmp/out.txt", "content": "data"}`,
	}

	msg := Message{
		Role:      RoleAssistant,
		Content:   "",
		ToolCalls: []ToolCall{tc1, tc2},
	}

	if len(msg.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected first tool call ID %q, got %q", "call_1", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected first tool call name %q, got %q", "read_file", msg.ToolCalls[0].Name)
	}
	if msg.ToolCalls[1].Name != "write_file" {
		t.Errorf("expected second tool call name %q, got %q", "write_file", msg.ToolCalls[1].Name)
	}
	if msg.ToolCalls[1].Arguments != `{"path": "/tmp/out.txt", "content": "data"}` {
		t.Errorf("unexpected arguments: %s", msg.ToolCalls[1].Arguments)
	}
}

func TestMessageToolCallID(t *testing.T) {
	msg := Message{
		Role:       RoleTool,
		Content:    "file contents here",
		ToolCallID: "call_abc123",
	}
	if msg.ToolCallID != "call_abc123" {
		t.Errorf("expected ToolCallID %q, got %q", "call_abc123", msg.ToolCallID)
	}
}

func TestToolCallFields(t *testing.T) {
	tc := ToolCall{
		ID:        "tc_001",
		Name:      "search",
		Arguments: `{"query": "test"}`,
	}
	if tc.ID != "tc_001" {
		t.Errorf("unexpected ID: %s", tc.ID)
	}
	if tc.Name != "search" {
		t.Errorf("unexpected Name: %s", tc.Name)
	}
	if tc.Arguments != `{"query": "test"}` {
		t.Errorf("unexpected Arguments: %s", tc.Arguments)
	}
}

func TestToolDefinitionFields(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	}
	td := ToolDefinition{
		Name:        "read_file",
		Description: "Reads a file from disk",
		InputSchema: schema,
	}
	if td.Name != "read_file" {
		t.Errorf("unexpected Name: %s", td.Name)
	}
	if td.Description != "Reads a file from disk" {
		t.Errorf("unexpected Description: %s", td.Description)
	}
	if td.InputSchema == nil {
		t.Error("expected non-nil InputSchema")
	}
}

func TestToolDefinitionNilSchema(t *testing.T) {
	td := ToolDefinition{
		Name:        "no_args_tool",
		Description: "A tool with no arguments",
	}
	if td.InputSchema != nil {
		t.Errorf("expected nil InputSchema, got %v", td.InputSchema)
	}
}

func TestUsageZeroValues(t *testing.T) {
	var u Usage
	if u.InputTokens != 0 {
		t.Errorf("expected InputTokens=0, got %d", u.InputTokens)
	}
	if u.OutputTokens != 0 {
		t.Errorf("expected OutputTokens=0, got %d", u.OutputTokens)
	}
}

func TestUsageWithValues(t *testing.T) {
	u := Usage{
		InputTokens:  1500,
		OutputTokens: 300,
	}
	if u.InputTokens != 1500 {
		t.Errorf("expected InputTokens=1500, got %d", u.InputTokens)
	}
	if u.OutputTokens != 300 {
		t.Errorf("expected OutputTokens=300, got %d", u.OutputTokens)
	}
}

func TestStreamEventTypeValues(t *testing.T) {
	// Verify iota ordering: EventText=0, EventToolCallStart=1, etc.
	if EventText != 0 {
		t.Errorf("expected EventText=0, got %d", EventText)
	}
	if EventToolCallStart != 1 {
		t.Errorf("expected EventToolCallStart=1, got %d", EventToolCallStart)
	}
	if EventToolCallDelta != 2 {
		t.Errorf("expected EventToolCallDelta=2, got %d", EventToolCallDelta)
	}
	if EventToolCallComplete != 3 {
		t.Errorf("expected EventToolCallComplete=3, got %d", EventToolCallComplete)
	}
	if EventDone != 4 {
		t.Errorf("expected EventDone=4, got %d", EventDone)
	}
	if EventError != 5 {
		t.Errorf("expected EventError=5, got %d", EventError)
	}
}

func TestStreamEventTypeDistinct(t *testing.T) {
	types := []StreamEventType{
		EventText,
		EventToolCallStart,
		EventToolCallDelta,
		EventToolCallComplete,
		EventDone,
		EventError,
	}
	seen := make(map[StreamEventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate StreamEventType value: %d", et)
		}
		seen[et] = true
	}
}

func TestStreamEventTextEvent(t *testing.T) {
	evt := StreamEvent{
		Type:    EventText,
		Content: "Hello",
	}
	if evt.Type != EventText {
		t.Errorf("expected EventText, got %d", evt.Type)
	}
	if evt.Content != "Hello" {
		t.Errorf("expected content %q, got %q", "Hello", evt.Content)
	}
	if evt.Done {
		t.Error("expected Done=false for text event")
	}
	if evt.Error != nil {
		t.Errorf("expected nil Error, got %v", evt.Error)
	}
}

func TestStreamEventDoneEvent(t *testing.T) {
	usage := &Usage{InputTokens: 100, OutputTokens: 50}
	evt := StreamEvent{
		Type:  EventDone,
		Done:  true,
		Usage: usage,
	}
	if evt.Type != EventDone {
		t.Errorf("expected EventDone, got %d", evt.Type)
	}
	if !evt.Done {
		t.Error("expected Done=true")
	}
	if evt.Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if evt.Usage.InputTokens != 100 {
		t.Errorf("expected InputTokens=100, got %d", evt.Usage.InputTokens)
	}
	if evt.Usage.OutputTokens != 50 {
		t.Errorf("expected OutputTokens=50, got %d", evt.Usage.OutputTokens)
	}
}

func TestStreamEventErrorEvent(t *testing.T) {
	testErr := fmt.Errorf("something went wrong")
	evt := StreamEvent{
		Type:  EventError,
		Error: testErr,
	}
	if evt.Type != EventError {
		t.Errorf("expected EventError, got %d", evt.Type)
	}
	if evt.Error == nil {
		t.Fatal("expected non-nil Error")
	}
	if evt.Error.Error() != "something went wrong" {
		t.Errorf("unexpected error message: %s", evt.Error.Error())
	}
}

func TestStreamEventToolCallEvent(t *testing.T) {
	tc := &ToolCall{
		ID:        "tc_stream_1",
		Name:      "bash",
		Arguments: `{"command": "ls"}`,
	}
	evt := StreamEvent{
		Type:     EventToolCallComplete,
		ToolCall: tc,
	}
	if evt.Type != EventToolCallComplete {
		t.Errorf("expected EventToolCallComplete, got %d", evt.Type)
	}
	if evt.ToolCall == nil {
		t.Fatal("expected non-nil ToolCall")
	}
	if evt.ToolCall.Name != "bash" {
		t.Errorf("expected tool call name %q, got %q", "bash", evt.ToolCall.Name)
	}
}

func TestStreamEventZeroValue(t *testing.T) {
	var evt StreamEvent
	if evt.Type != EventText {
		t.Errorf("expected zero-value Type to be EventText (0), got %d", evt.Type)
	}
	if evt.Content != "" {
		t.Errorf("expected empty Content, got %q", evt.Content)
	}
	if evt.ToolCall != nil {
		t.Errorf("expected nil ToolCall, got %v", evt.ToolCall)
	}
	if evt.Done {
		t.Error("expected Done=false")
	}
	if evt.Error != nil {
		t.Errorf("expected nil Error, got %v", evt.Error)
	}
	if evt.Usage != nil {
		t.Errorf("expected nil Usage, got %v", evt.Usage)
	}
}

func TestRequestFields(t *testing.T) {
	tools := []ToolDefinition{
		{Name: "tool1", Description: "desc1"},
	}
	msgs := []Message{
		{Role: RoleUser, Content: "hi"},
	}
	req := Request{
		Model:        "claude-3-opus",
		SystemPrompt: "You are a helpful assistant.",
		Messages:     msgs,
		Tools:        tools,
		MaxTokens:    4096,
		Temperature:  0.7,
	}
	if req.Model != "claude-3-opus" {
		t.Errorf("unexpected Model: %s", req.Model)
	}
	if req.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("unexpected SystemPrompt: %s", req.SystemPrompt)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}
	if req.Messages[0].Content != "hi" {
		t.Errorf("unexpected message content: %s", req.Messages[0].Content)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(req.Tools))
	}
	if req.MaxTokens != 4096 {
		t.Errorf("expected MaxTokens=4096, got %d", req.MaxTokens)
	}
	if req.Temperature != 0.7 {
		t.Errorf("expected Temperature=0.7, got %f", req.Temperature)
	}
}

func TestRequestZeroValue(t *testing.T) {
	var req Request
	if req.Model != "" {
		t.Errorf("expected empty Model, got %q", req.Model)
	}
	if req.MaxTokens != 0 {
		t.Errorf("expected MaxTokens=0, got %d", req.MaxTokens)
	}
	if req.Temperature != 0.0 {
		t.Errorf("expected Temperature=0.0, got %f", req.Temperature)
	}
	if req.Messages != nil {
		t.Errorf("expected nil Messages, got %v", req.Messages)
	}
	if req.Tools != nil {
		t.Errorf("expected nil Tools, got %v", req.Tools)
	}
}

// fmt is used for error creation in test
var _ = fmt.Errorf
