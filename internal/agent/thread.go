package agent

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/nomanqureshi/argo/internal/llm"
)

// MaxSubagentDepth is the maximum nesting depth for sub-agents.
// Sub-agents at this depth do NOT get the spawn_agent tool,
// preventing recursive spawning (inspired by Zed's MAX_SUBAGENT_DEPTH = 1).
const MaxSubagentDepth = 1

// SubagentContext holds metadata linking a child thread to its parent.
// When non-nil on a Thread, it indicates this thread is a sub-agent session.
type SubagentContext struct {
	ParentThreadID string // ID of the parent thread that spawned this sub-agent
	Depth          int    // nesting depth (0 = top-level, 1 = first sub-agent, etc.)
}

// Thread represents a conversation between the user and the agent.
// It holds the ordered list of messages and metadata about the session.
type Thread struct {
	ID        string
	CreatedAt time.Time
	Title     string // derived from the first user message, or set explicitly

	// Subagent tracks parent-child relationships between threads.
	// nil for top-level (user-initiated) threads.
	Subagent *SubagentContext

	messages []llm.Message
	mu       sync.RWMutex
}

// NewThread creates a new thread with a unique ID and the current timestamp.
func NewThread() *Thread {
	return &Thread{
		ID:        generateUUID(),
		CreatedAt: time.Now(),
		messages:  make([]llm.Message, 0),
	}
}

// NewSubagentThread creates a new thread that is a child of the given parent.
// The depth is set to parentDepth + 1.
func NewSubagentThread(parentThreadID string, parentDepth int) *Thread {
	return &Thread{
		ID:        generateUUID(),
		CreatedAt: time.Now(),
		messages:  make([]llm.Message, 0),
		Subagent: &SubagentContext{
			ParentThreadID: parentThreadID,
			Depth:          parentDepth + 1,
		},
	}
}

// AddMessage appends a message to the thread.
func (t *Thread) AddMessage(msg llm.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messages = append(t.messages, msg)

	// Auto-derive title from the first user message if not already set.
	if t.Title == "" && msg.Role == llm.RoleUser && msg.Content != "" {
		title := msg.Content
		if len(title) > 100 {
			title = title[:100] + "…"
		}
		t.Title = title
	}
}

// Messages returns a snapshot copy of all messages in the thread.
func (t *Thread) Messages() []llm.Message {
	t.mu.RLock()
	defer t.mu.RUnlock()
	msgs := make([]llm.Message, len(t.messages))
	copy(msgs, t.messages)
	return msgs
}

// SetMessages replaces the thread's messages with the given slice.
// This is used when loading a thread from persistence.
func (t *Thread) SetMessages(msgs []llm.Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messages = make([]llm.Message, len(msgs))
	copy(t.messages, msgs)
}

// MessageCount returns the number of messages in the thread.
func (t *Thread) MessageCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.messages)
}

// LastMessage returns a copy of the last message, or nil if empty.
func (t *Thread) LastMessage() *llm.Message {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.messages) == 0 {
		return nil
	}
	msg := t.messages[len(t.messages)-1]
	return &msg
}

// LastAssistantText returns the text content of the last assistant message,
// or an empty string if there is none. This is used to extract sub-agent results.
func (t *Thread) LastAssistantText() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for i := len(t.messages) - 1; i >= 0; i-- {
		if t.messages[i].Role == llm.RoleAssistant && t.messages[i].Content != "" {
			return t.messages[i].Content
		}
	}
	return ""
}

// Clear removes all messages from the thread but preserves metadata.
func (t *Thread) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messages = make([]llm.Message, 0)
}

// Depth returns the sub-agent nesting depth of this thread.
// Top-level threads return 0.
func (t *Thread) Depth() int {
	if t.Subagent != nil {
		return t.Subagent.Depth
	}
	return 0
}

// IsSubagent returns true if this thread was spawned as a sub-agent.
func (t *Thread) IsSubagent() bool {
	return t.Subagent != nil
}

// generateUUID produces a version-4 UUID string.
// Falls back to a timestamp-based ID if crypto/rand fails (extremely unlikely).
func generateUUID() string {
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	if err != nil {
		// Fallback: use timestamp + simple counter for uniqueness.
		return time.Now().Format("20060102-150405.000000000")
	}
	// Set version 4 bits.
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant bits.
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
