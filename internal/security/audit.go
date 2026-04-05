package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	Timestamp  string   `json:"timestamp"`
	Tool       string   `json:"tool"`
	Action     string   `json:"action"` // "allowed", "denied", "confirmed", "blocked"
	Input      string   `json:"input,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	Violations []string `json:"violations,omitempty"`
}

// AuditLogger writes security-relevant events to a persistent log file.
type AuditLogger struct {
	mu       sync.Mutex
	file     *os.File
	filePath string
	enabled  bool
}

// NewAuditLogger creates a new AuditLogger that writes to ~/.argo/audit.log.
// If the file cannot be opened, logging is disabled silently.
func NewAuditLogger() *AuditLogger {
	logger := &AuditLogger{enabled: false}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return logger
	}

	dirPath := filepath.Join(homeDir, ".argo")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return logger
	}

	logPath := filepath.Join(dirPath, "audit.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return logger
	}

	logger.file = f
	logger.filePath = logPath
	logger.enabled = true

	return logger
}

// NewAuditLoggerAt creates an AuditLogger writing to a specific path.
// Useful for testing.
func NewAuditLoggerAt(path string) (*AuditLogger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &AuditLogger{
		file:     f,
		filePath: path,
		enabled:  true,
	}, nil
}

// Log writes an audit entry to the log file.
func (l *AuditLogger) Log(entry AuditEntry) {
	if !l.enabled {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	data = append(data, '\n')
	_, _ = l.file.Write(data)
}

// LogToolCall is a convenience method for logging a tool call.
func (l *AuditLogger) LogToolCall(tool, action, input, reason string) {
	l.Log(AuditEntry{
		Tool:   tool,
		Action: action,
		Input:  truncateForAudit(input, 500),
		Reason: reason,
	})
}

// LogSecurityViolation logs a security violation with details.
func (l *AuditLogger) LogSecurityViolation(tool, input string, violations []string) {
	l.Log(AuditEntry{
		Tool:       tool,
		Action:     "blocked",
		Input:      truncateForAudit(input, 500),
		Reason:     "security violation",
		Violations: violations,
	})
}

// Close closes the audit log file.
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Path returns the path to the audit log file.
func (l *AuditLogger) Path() string {
	return l.filePath
}

// truncateForAudit truncates a string for inclusion in audit logs.
func truncateForAudit(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
