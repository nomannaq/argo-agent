package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAuditLoggerAt_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}
	defer func() { _ = logger.Close() }()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("expected log file to be created at %s", logPath)
	}
}

func TestNewAuditLoggerAt_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sub", "dir", "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}
	defer func() { _ = logger.Close() }()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("expected log file to be created at %s", logPath)
	}
}

func TestLog_WritesValidJSONLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}

	logger.Log(AuditEntry{
		Tool:   "shell",
		Action: "allowed",
		Input:  "ls -la",
	})
	logger.Log(AuditEntry{
		Tool:   "read_file",
		Action: "denied",
		Input:  ".env",
		Reason: "sensitive file",
	})
	_ = logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
		if entry.Timestamp == "" {
			t.Errorf("line %d: expected non-empty timestamp", i)
		}
	}
}

func TestLogToolCall_WritesCorrectFields(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}

	logger.LogToolCall("terminal", "allowed", "go test ./...", "user approved")
	_ = logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry AuditEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}

	if entry.Tool != "terminal" {
		t.Errorf("expected tool %q, got %q", "terminal", entry.Tool)
	}
	if entry.Action != "allowed" {
		t.Errorf("expected action %q, got %q", "allowed", entry.Action)
	}
	if entry.Input != "go test ./..." {
		t.Errorf("expected input %q, got %q", "go test ./...", entry.Input)
	}
	if entry.Reason != "user approved" {
		t.Errorf("expected reason %q, got %q", "user approved", entry.Reason)
	}
	if entry.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestLogSecurityViolation_WritesViolations(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}

	violations := []string{"dangerous command pattern", "targets root filesystem"}
	logger.LogSecurityViolation("terminal", "rm -rf /", violations)
	_ = logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry AuditEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}

	if entry.Tool != "terminal" {
		t.Errorf("expected tool %q, got %q", "terminal", entry.Tool)
	}
	if entry.Action != "blocked" {
		t.Errorf("expected action %q, got %q", "blocked", entry.Action)
	}
	if entry.Input != "rm -rf /" {
		t.Errorf("expected input %q, got %q", "rm -rf /", entry.Input)
	}
	if entry.Reason != "security violation" {
		t.Errorf("expected reason %q, got %q", "security violation", entry.Reason)
	}
	if len(entry.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(entry.Violations))
	}
	if entry.Violations[0] != "dangerous command pattern" {
		t.Errorf("expected violation[0] %q, got %q", "dangerous command pattern", entry.Violations[0])
	}
	if entry.Violations[1] != "targets root filesystem" {
		t.Errorf("expected violation[1] %q, got %q", "targets root filesystem", entry.Violations[1])
	}
}

func TestMultipleLogCallsAppend(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		logger.Log(AuditEntry{
			Tool:   "shell",
			Action: "allowed",
			Input:  "command",
		})
	}
	_ = logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}

	// Verify each line is valid JSON and distinct
	for i, line := range lines {
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestClose_NoError(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}

	logger.Log(AuditEntry{Tool: "test", Action: "test"})

	if err := logger.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestClose_NilFile_NoError(t *testing.T) {
	logger := &AuditLogger{enabled: false}
	if err := logger.Close(); err != nil {
		t.Errorf("Close on disabled logger returned error: %v", err)
	}
}

func TestPath_ReturnsCorrectPath(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "my-audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}
	defer func() { _ = logger.Close() }()

	if logger.Path() != logPath {
		t.Errorf("expected path %q, got %q", logPath, logger.Path())
	}
}

func TestTruncateForAudit_TruncatesLongStrings(t *testing.T) {
	long := strings.Repeat("a", 600)
	result := truncateForAudit(long, 500)

	if len(result) != 503 { // 500 + len("...")
		t.Errorf("expected length 503, got %d", len(result))
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("expected truncated string to end with '...'")
	}
	if result[:500] != strings.Repeat("a", 500) {
		t.Error("expected first 500 chars to be preserved")
	}
}

func TestTruncateForAudit_DoesNotTruncateShortStrings(t *testing.T) {
	short := "hello world"
	result := truncateForAudit(short, 500)
	if result != short {
		t.Errorf("expected %q, got %q", short, result)
	}
}

func TestTruncateForAudit_ExactLength(t *testing.T) {
	exact := strings.Repeat("x", 500)
	result := truncateForAudit(exact, 500)
	if result != exact {
		t.Error("string at exact max length should not be truncated")
	}
}

func TestTruncateForAudit_EmptyString(t *testing.T) {
	result := truncateForAudit("", 500)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestLogToolCall_TruncatesLongInput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	logger, err := NewAuditLoggerAt(logPath)
	if err != nil {
		t.Fatalf("NewAuditLoggerAt failed: %v", err)
	}

	longInput := strings.Repeat("z", 1000)
	logger.LogToolCall("terminal", "allowed", longInput, "")
	_ = logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry AuditEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}

	if len(entry.Input) != 503 {
		t.Errorf("expected input length 503 (500 + '...'), got %d", len(entry.Input))
	}
	if !strings.HasSuffix(entry.Input, "...") {
		t.Error("expected truncated input to end with '...'")
	}
}

func TestDisabledLogger_DoesNotPanic(t *testing.T) {
	logger := NewAuditLogger()
	// Even if NewAuditLogger succeeded, this should not panic
	logger.Log(AuditEntry{Tool: "test", Action: "test"})
	logger.LogToolCall("test", "allowed", "input", "reason")
	logger.LogSecurityViolation("test", "input", []string{"v1"})
	_ = logger.Close()
}
