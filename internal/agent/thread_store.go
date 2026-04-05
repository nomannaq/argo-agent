package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nomanqureshi/argo/internal/llm"
	_ "modernc.org/sqlite"
)

// ThreadStore defines the interface for persisting conversation threads.
type ThreadStore interface {
	SaveThread(thread *Thread) error
	LoadThread(id string) (*Thread, error)
	ListThreads() ([]ThreadSummary, error)
	DeleteThread(id string) error
	Close() error
}

// ThreadSummary holds lightweight metadata about a thread for listing purposes.
type ThreadSummary struct {
	ID           string
	CreatedAt    time.Time
	Title        string // first user message, truncated
	MessageCount int
}

// SQLiteStore implements ThreadStore using a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
	mu sync.Mutex
}

const (
	maxTitleLength = 100
	argoDir        = ".argo"
	dbFileName     = "threads.db"
)

const createTablesSQL = `
CREATE TABLE IF NOT EXISTS threads (
	id         TEXT PRIMARY KEY,
	created_at DATETIME NOT NULL,
	title      TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS messages (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	thread_id    TEXT NOT NULL,
	role         TEXT NOT NULL,
	content      TEXT NOT NULL DEFAULT '',
	tool_calls   TEXT NOT NULL DEFAULT '[]',
	tool_call_id TEXT NOT NULL DEFAULT '',
	position     INTEGER NOT NULL,
	FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_messages_position ON messages(thread_id, position);
`

// NewSQLiteStore creates a new SQLiteStore, initialising the database and tables
// at ~/.argo/threads.db. The ~/.argo directory is created if it does not exist.
func NewSQLiteStore() (*SQLiteStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}

	dirPath := filepath.Join(homeDir, argoDir)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create %s directory: %w", dirPath, err)
	}

	dbPath := filepath.Join(dirPath, dbFileName)
	return NewSQLiteStoreAt(dbPath)
}

// NewSQLiteStoreAt creates a new SQLiteStore at the specified database path.
// This is useful for testing with a custom or temporary database location.
func NewSQLiteStoreAt(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	// Verify the connection is usable.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables if they don't already exist.
	if _, err := db.Exec(createTablesSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialise database schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// SaveThread persists a thread and all of its messages to the database.
// The operation is performed inside a transaction: existing messages for the
// thread are deleted and re-inserted so the database always reflects the
// in-memory state exactly.
func (s *SQLiteStore) SaveThread(thread *Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	thread.mu.RLock()
	msgs := make([]llm.Message, len(thread.messages))
	copy(msgs, thread.messages)
	id := thread.ID
	createdAt := thread.CreatedAt
	thread.mu.RUnlock()

	title := deriveTitle(msgs)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after commit

	// Upsert the thread row.
	_, err = tx.Exec(
		`INSERT INTO threads (id, created_at, title) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET title = excluded.title`,
		id, createdAt.UTC().Format(time.RFC3339Nano), title,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert thread %s: %w", id, err)
	}

	// Remove all existing messages for this thread so we can re-insert them.
	if _, err := tx.Exec(`DELETE FROM messages WHERE thread_id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete old messages for thread %s: %w", id, err)
	}

	// Prepare the insert statement for messages.
	stmt, err := tx.Prepare(
		`INSERT INTO messages (thread_id, role, content, tool_calls, tool_call_id, position)
		 VALUES (?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("failed to prepare message insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for i, msg := range msgs {
		toolCallsJSON, err := json.Marshal(msg.ToolCalls)
		if err != nil {
			return fmt.Errorf("failed to marshal tool_calls at position %d: %w", i, err)
		}

		if _, err := stmt.Exec(
			id,
			string(msg.Role),
			msg.Content,
			string(toolCallsJSON),
			msg.ToolCallID,
			i,
		); err != nil {
			return fmt.Errorf("failed to insert message at position %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadThread reads a thread and all of its messages from the database,
// reconstructing a full Thread object. Returns an error if the thread is not found.
func (s *SQLiteStore) LoadThread(id string) (*Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load the thread metadata.
	var createdAtStr string
	err := s.db.QueryRow(
		`SELECT created_at FROM threads WHERE id = ?`, id,
	).Scan(&createdAtStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("thread %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query thread %s: %w", id, err)
	}

	createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at for thread %s: %w", id, err)
	}

	// Load all messages ordered by position.
	rows, err := s.db.Query(
		`SELECT role, content, tool_calls, tool_call_id
		 FROM messages
		 WHERE thread_id = ?
		 ORDER BY position ASC`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages for thread %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	var messages []llm.Message
	for rows.Next() {
		var (
			role         string
			content      string
			toolCallsRaw string
			toolCallID   string
		)
		if err := rows.Scan(&role, &content, &toolCallsRaw, &toolCallID); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}

		var toolCalls []llm.ToolCall
		if toolCallsRaw != "" && toolCallsRaw != "[]" {
			if err := json.Unmarshal([]byte(toolCallsRaw), &toolCalls); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool_calls: %w", err)
			}
		}

		messages = append(messages, llm.Message{
			Role:       llm.Role(role),
			Content:    content,
			ToolCalls:  toolCalls,
			ToolCallID: toolCallID,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	thread := &Thread{
		ID:        id,
		CreatedAt: createdAt,
		messages:  messages,
	}

	return thread, nil
}

// ListThreads returns lightweight summaries of all stored threads,
// sorted by creation time descending (most recent first).
func (s *SQLiteStore) ListThreads() ([]ThreadSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT t.id, t.created_at, t.title,
		       COALESCE(m.cnt, 0) AS message_count
		FROM threads t
		LEFT JOIN (
			SELECT thread_id, COUNT(*) AS cnt
			FROM messages
			GROUP BY thread_id
		) m ON m.thread_id = t.id
		ORDER BY t.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list threads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []ThreadSummary
	for rows.Next() {
		var (
			summary      ThreadSummary
			createdAtStr string
		)
		if err := rows.Scan(&summary.ID, &createdAtStr, &summary.Title, &summary.MessageCount); err != nil {
			return nil, fmt.Errorf("failed to scan thread summary: %w", err)
		}

		createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		summary.CreatedAt = createdAt

		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating thread rows: %w", err)
	}

	return summaries, nil
}

// DeleteThread removes a thread and all of its messages from the database.
func (s *SQLiteStore) DeleteThread(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete messages first (or rely on CASCADE, but be explicit).
	if _, err := tx.Exec(`DELETE FROM messages WHERE thread_id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete messages for thread %s: %w", id, err)
	}

	result, err := tx.Exec(`DELETE FROM threads WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete thread %s: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("thread %s not found", id)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// deriveTitle extracts a title from the first user message in the thread.
// The content is truncated to maxTitleLength characters.
func deriveTitle(messages []llm.Message) string {
	for _, msg := range messages {
		if msg.Role == llm.RoleUser && msg.Content != "" {
			title := msg.Content
			if len(title) > maxTitleLength {
				title = title[:maxTitleLength] + "…"
			}
			return title
		}
	}
	return ""
}
