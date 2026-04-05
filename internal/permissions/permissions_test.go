package permissions

import (
	"errors"
	"testing"

	"github.com/nomanqureshi/argo/internal/tools"
)

func TestAutoApproveHandler_AlwaysReturnsTrue(t *testing.T) {
	h := &AutoApproveHandler{}

	tests := []struct {
		name  string
		tool  string
		level tools.PermissionLevel
		input string
	}{
		{"read operation", "read_file", tools.PermissionRead, "foo.txt"},
		{"write operation", "edit_file", tools.PermissionWrite, "bar.go"},
		{"dangerous operation", "shell", tools.PermissionDangerous, "rm -rf /tmp/test"},
		{"empty input", "something", tools.PermissionRead, ""},
		{"unknown tool", "nonexistent_tool", tools.PermissionWrite, "whatever"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := h.CheckPermission(tt.tool, tt.level, tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !allowed {
				t.Fatal("expected AutoApproveHandler to return true, got false")
			}
		})
	}
}

func TestInteractiveHandler_AutoApprovesReadWhenEnabled(t *testing.T) {
	called := false
	h := &InteractiveHandler{
		PromptFunc: func(toolName string, level tools.PermissionLevel, input string) (bool, error) {
			called = true
			return false, nil
		},
		AutoApproveRead: true,
	}

	allowed, err := h.CheckPermission("read_file", tools.PermissionRead, "some input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected read to be auto-approved, got false")
	}
	if called {
		t.Fatal("expected PromptFunc not to be called for read level with AutoApproveRead=true")
	}
}

func TestInteractiveHandler_CallsPromptFuncForWrite(t *testing.T) {
	var capturedTool string
	var capturedLevel tools.PermissionLevel
	var capturedInput string

	h := &InteractiveHandler{
		PromptFunc: func(toolName string, level tools.PermissionLevel, input string) (bool, error) {
			capturedTool = toolName
			capturedLevel = level
			capturedInput = input
			return true, nil
		},
		AutoApproveRead: true,
	}

	allowed, err := h.CheckPermission("edit_file", tools.PermissionWrite, "modify something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected PromptFunc return value true to be propagated")
	}
	if capturedTool != "edit_file" {
		t.Errorf("expected toolName %q, got %q", "edit_file", capturedTool)
	}
	if capturedLevel != tools.PermissionWrite {
		t.Errorf("expected level PermissionWrite, got %v", capturedLevel)
	}
	if capturedInput != "modify something" {
		t.Errorf("expected input %q, got %q", "modify something", capturedInput)
	}
}

func TestInteractiveHandler_CallsPromptFuncForDangerous(t *testing.T) {
	called := false
	h := &InteractiveHandler{
		PromptFunc: func(toolName string, level tools.PermissionLevel, input string) (bool, error) {
			called = true
			return false, nil
		},
		AutoApproveRead: true,
	}

	allowed, err := h.CheckPermission("shell", tools.PermissionDangerous, "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected PromptFunc return value false to be propagated")
	}
	if !called {
		t.Fatal("expected PromptFunc to be called for dangerous level")
	}
}

func TestInteractiveHandler_AutoApproveReadFalse_CallsPromptFuncForRead(t *testing.T) {
	called := false
	h := &InteractiveHandler{
		PromptFunc: func(toolName string, level tools.PermissionLevel, input string) (bool, error) {
			called = true
			return true, nil
		},
		AutoApproveRead: false,
	}

	allowed, err := h.CheckPermission("read_file", tools.PermissionRead, "foo.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed to be true")
	}
	if !called {
		t.Fatal("expected PromptFunc to be called for read level when AutoApproveRead=false")
	}
}

func TestNewInteractiveHandler_SetsAutoApproveReadTrue(t *testing.T) {
	h := NewInteractiveHandler(func(name string, level tools.PermissionLevel, input string) (bool, error) {
		return false, nil
	})

	if !h.AutoApproveRead {
		t.Fatal("expected NewInteractiveHandler to set AutoApproveRead=true by default")
	}
	if h.PromptFunc == nil {
		t.Fatal("expected PromptFunc to be set")
	}
}

func TestInteractiveHandler_PropagatesErrors(t *testing.T) {
	expectedErr := errors.New("prompt failed")
	h := &InteractiveHandler{
		PromptFunc: func(toolName string, level tools.PermissionLevel, input string) (bool, error) {
			return false, expectedErr
		},
		AutoApproveRead: true,
	}

	_, err := h.CheckPermission("edit_file", tools.PermissionWrite, "input")
	if err == nil {
		t.Fatal("expected error to be propagated from PromptFunc")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestInteractiveHandler_PromptFuncDenies(t *testing.T) {
	h := NewInteractiveHandler(func(name string, level tools.PermissionLevel, input string) (bool, error) {
		return false, nil
	})

	allowed, err := h.CheckPermission("shell", tools.PermissionDangerous, "dangerous command")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected denial from PromptFunc to be propagated")
	}
}

func TestAutoApproveHandler_ImplementsHandlerInterface(t *testing.T) {
	var h Handler = &AutoApproveHandler{}
	allowed, err := h.CheckPermission("test", tools.PermissionRead, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected true")
	}
}

func TestInteractiveHandler_ImplementsHandlerInterface(t *testing.T) {
	var h Handler = NewInteractiveHandler(func(name string, level tools.PermissionLevel, input string) (bool, error) {
		return true, nil
	})
	allowed, err := h.CheckPermission("test", tools.PermissionWrite, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected true")
	}
}
