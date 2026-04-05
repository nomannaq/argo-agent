package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/nomanqureshi/argo/internal/llm"
)

func TestNewThread_UniqueID(t *testing.T) {
	t1 := NewThread()
	t2 := NewThread()
	if t1.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if t1.ID == t2.ID {
		t.Fatalf("expected unique IDs, got %s and %s", t1.ID, t2.ID)
	}
}

func TestNewThread_CurrentTimestamp(t *testing.T) {
	before := time.Now().Add(-time.Second)
	th := NewThread()
	after := time.Now().Add(time.Second)
	if th.CreatedAt.Before(before) || th.CreatedAt.After(after) {
		t.Fatalf("CreatedAt %v not within expected range [%v, %v]", th.CreatedAt, before, after)
	}
}

func TestNewThread_EmptyMessages(t *testing.T) {
	th := NewThread()
	if count := th.MessageCount(); count != 0 {
		t.Fatalf("expected 0 messages, got %d", count)
	}
	msgs := th.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected empty messages slice, got %d", len(msgs))
	}
}

func TestNewThread_EmptyTitle(t *testing.T) {
	th := NewThread()
	if th.Title != "" {
		t.Fatalf("expected empty title, got %q", th.Title)
	}
}

func TestNewSubagentThread_SetsDepth(t *testing.T) {
	th := NewSubagentThread("parent-123", 0)
	if th.Subagent == nil {
		t.Fatal("expected Subagent to be non-nil")
	}
	if th.Subagent.Depth != 1 {
		t.Fatalf("expected depth 1, got %d", th.Subagent.Depth)
	}
}

func TestNewSubagentThread_SetsParentThreadID(t *testing.T) {
	th := NewSubagentThread("parent-456", 2)
	if th.Subagent == nil {
		t.Fatal("expected Subagent to be non-nil")
	}
	if th.Subagent.ParentThreadID != "parent-456" {
		t.Fatalf("expected parent ID %q, got %q", "parent-456", th.Subagent.ParentThreadID)
	}
	if th.Subagent.Depth != 3 {
		t.Fatalf("expected depth 3, got %d", th.Subagent.Depth)
	}
}

func TestNewSubagentThread_HasUniqueID(t *testing.T) {
	t1 := NewSubagentThread("parent", 0)
	t2 := NewSubagentThread("parent", 0)
	if t1.ID == t2.ID {
		t.Fatal("expected unique IDs for subagent threads")
	}
}

func TestAddMessage_AppendsMessage(t *testing.T) {
	th := NewThread()
	msg := llm.Message{Role: llm.RoleUser, Content: "hello"}
	th.AddMessage(msg)
	if th.MessageCount() != 1 {
		t.Fatalf("expected 1 message, got %d", th.MessageCount())
	}
	msgs := th.Messages()
	if msgs[0].Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", msgs[0].Content)
	}
}

func TestAddMessage_AppendsMultiple(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "first"})
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "second"})
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "third"})
	if th.MessageCount() != 3 {
		t.Fatalf("expected 3 messages, got %d", th.MessageCount())
	}
	msgs := th.Messages()
	if msgs[0].Content != "first" || msgs[1].Content != "second" || msgs[2].Content != "third" {
		t.Fatal("messages not in expected order")
	}
}

func TestAddMessage_AutoDerivesTitle(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "How do I write tests?"})
	if th.Title != "How do I write tests?" {
		t.Fatalf("expected title %q, got %q", "How do I write tests?", th.Title)
	}
}

func TestAddMessage_TitleFromFirstUserMessage_IgnoresSystemMessages(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleSystem, Content: "You are a helpful assistant"})
	if th.Title != "" {
		t.Fatalf("expected empty title after system message, got %q", th.Title)
	}
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "Hello"})
	if th.Title != "Hello" {
		t.Fatalf("expected title %q, got %q", "Hello", th.Title)
	}
}

func TestAddMessage_TruncatesLongTitles(t *testing.T) {
	th := NewThread()
	longContent := strings.Repeat("a", 200)
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: longContent})
	expectedTitle := strings.Repeat("a", 100) + "…"
	if th.Title != expectedTitle {
		t.Fatalf("expected truncated title of length %d, got length %d", len(expectedTitle), len(th.Title))
	}
	if !strings.HasSuffix(th.Title, "…") {
		t.Fatal("expected title to end with ellipsis")
	}
}

func TestAddMessage_ExactlyMaxLengthTitle(t *testing.T) {
	th := NewThread()
	exactContent := strings.Repeat("b", 100)
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: exactContent})
	if th.Title != exactContent {
		t.Fatal("title of exactly 100 chars should not be truncated")
	}
}

func TestAddMessage_DoesNotOverwriteExistingTitle(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "First question"})
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "Second question"})
	if th.Title != "First question" {
		t.Fatalf("expected title %q, got %q", "First question", th.Title)
	}
}

func TestAddMessage_DoesNotDeriveTitleFromEmptyContent(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: ""})
	if th.Title != "" {
		t.Fatalf("expected empty title for empty content, got %q", th.Title)
	}
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "Actual question"})
	if th.Title != "Actual question" {
		t.Fatalf("expected title %q, got %q", "Actual question", th.Title)
	}
}

func TestMessages_ReturnsCopy(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "original"})
	msgs := th.Messages()
	msgs[0].Content = "modified"
	// The original should remain unchanged.
	original := th.Messages()
	if original[0].Content != "original" {
		t.Fatal("modifying returned slice should not affect the thread's messages")
	}
}

func TestMessages_AppendToCopyDoesNotAffectOriginal(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "one"})
	msgs := th.Messages()
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: "two"})
	if th.MessageCount() != 1 {
		t.Fatalf("appending to copy should not affect thread, got %d messages", th.MessageCount())
	}
	_ = msgs
}

func TestSetMessages_ReplacesMessages(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "old"})
	newMsgs := []llm.Message{
		{Role: llm.RoleUser, Content: "new1"},
		{Role: llm.RoleAssistant, Content: "new2"},
	}
	th.SetMessages(newMsgs)
	if th.MessageCount() != 2 {
		t.Fatalf("expected 2 messages after SetMessages, got %d", th.MessageCount())
	}
	msgs := th.Messages()
	if msgs[0].Content != "new1" || msgs[1].Content != "new2" {
		t.Fatal("SetMessages did not replace messages correctly")
	}
}

func TestSetMessages_MakesCopy(t *testing.T) {
	th := NewThread()
	input := []llm.Message{{Role: llm.RoleUser, Content: "hello"}}
	th.SetMessages(input)
	input[0].Content = "modified"
	msgs := th.Messages()
	if msgs[0].Content != "hello" {
		t.Fatal("SetMessages should copy the input, not keep a reference")
	}
}

func TestMessageCount(t *testing.T) {
	th := NewThread()
	if th.MessageCount() != 0 {
		t.Fatalf("expected 0, got %d", th.MessageCount())
	}
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "one"})
	if th.MessageCount() != 1 {
		t.Fatalf("expected 1, got %d", th.MessageCount())
	}
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "two"})
	if th.MessageCount() != 2 {
		t.Fatalf("expected 2, got %d", th.MessageCount())
	}
}

func TestLastMessage_NilForEmptyThread(t *testing.T) {
	th := NewThread()
	if msg := th.LastMessage(); msg != nil {
		t.Fatal("expected nil for empty thread")
	}
}

func TestLastMessage_ReturnsLastMessage(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "first"})
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "second"})
	msg := th.LastMessage()
	if msg == nil {
		t.Fatal("expected non-nil last message")
	}
	if msg.Content != "second" {
		t.Fatalf("expected content %q, got %q", "second", msg.Content)
	}
	if msg.Role != llm.RoleAssistant {
		t.Fatalf("expected role %q, got %q", llm.RoleAssistant, msg.Role)
	}
}

func TestLastMessage_ReturnsCopy(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "original"})
	msg := th.LastMessage()
	msg.Content = "modified"
	again := th.LastMessage()
	if again.Content != "original" {
		t.Fatal("modifying returned LastMessage should not affect the thread")
	}
}

func TestLastAssistantText_ReturnsLastAssistantContent(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "question"})
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "answer1"})
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "follow up"})
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "answer2"})
	result := th.LastAssistantText()
	if result != "answer2" {
		t.Fatalf("expected %q, got %q", "answer2", result)
	}
}

func TestLastAssistantText_EmptyWhenNoAssistant(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "hello"})
	result := th.LastAssistantText()
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestLastAssistantText_EmptyThread(t *testing.T) {
	th := NewThread()
	result := th.LastAssistantText()
	if result != "" {
		t.Fatalf("expected empty string for empty thread, got %q", result)
	}
}

func TestLastAssistantText_SkipsAssistantWithEmptyContent(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "real answer"})
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: ""})
	result := th.LastAssistantText()
	if result != "real answer" {
		t.Fatalf("expected %q, got %q", "real answer", result)
	}
}

func TestClear_RemovesMessages(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "hello"})
	th.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "hi"})
	th.Clear()
	if th.MessageCount() != 0 {
		t.Fatalf("expected 0 messages after Clear, got %d", th.MessageCount())
	}
	msgs := th.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected empty messages after Clear, got %d", len(msgs))
	}
}

func TestClear_KeepsMetadata(t *testing.T) {
	th := NewThread()
	th.AddMessage(llm.Message{Role: llm.RoleUser, Content: "My Title"})
	originalID := th.ID
	originalCreatedAt := th.CreatedAt
	originalTitle := th.Title
	th.Clear()
	if th.ID != originalID {
		t.Fatal("Clear should preserve ID")
	}
	if th.CreatedAt != originalCreatedAt {
		t.Fatal("Clear should preserve CreatedAt")
	}
	if th.Title != originalTitle {
		t.Fatal("Clear should preserve Title")
	}
}

func TestDepth_TopLevel(t *testing.T) {
	th := NewThread()
	if depth := th.Depth(); depth != 0 {
		t.Fatalf("expected depth 0 for top-level thread, got %d", depth)
	}
}

func TestDepth_SubagentThread(t *testing.T) {
	th := NewSubagentThread("parent-1", 0)
	if depth := th.Depth(); depth != 1 {
		t.Fatalf("expected depth 1, got %d", depth)
	}

	th2 := NewSubagentThread("parent-2", 3)
	if depth := th2.Depth(); depth != 4 {
		t.Fatalf("expected depth 4, got %d", depth)
	}
}

func TestIsSubagent_FalseForTopLevel(t *testing.T) {
	th := NewThread()
	if th.IsSubagent() {
		t.Fatal("expected IsSubagent() to be false for top-level thread")
	}
}

func TestIsSubagent_TrueForSubagent(t *testing.T) {
	th := NewSubagentThread("parent-1", 0)
	if !th.IsSubagent() {
		t.Fatal("expected IsSubagent() to be true for subagent thread")
	}
}

func TestMaxSubagentDepthConstant(t *testing.T) {
	if MaxSubagentDepth != 1 {
		t.Fatalf("expected MaxSubagentDepth to be 1, got %d", MaxSubagentDepth)
	}
}

func TestAddMessage_WithToolCalls(t *testing.T) {
	th := NewThread()
	msg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: "",
		ToolCalls: []llm.ToolCall{
			{ID: "tc1", Name: "read_file", Arguments: `{"path":"foo.go"}`},
		},
	}
	th.AddMessage(msg)
	msgs := th.Messages()
	if len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msgs[0].ToolCalls))
	}
	if msgs[0].ToolCalls[0].Name != "read_file" {
		t.Fatalf("expected tool call name %q, got %q", "read_file", msgs[0].ToolCalls[0].Name)
	}
}

func TestAddMessage_WithToolCallID(t *testing.T) {
	th := NewThread()
	msg := llm.Message{
		Role:       llm.RoleTool,
		Content:    "file contents here",
		ToolCallID: "tc1",
	}
	th.AddMessage(msg)
	msgs := th.Messages()
	if msgs[0].ToolCallID != "tc1" {
		t.Fatalf("expected ToolCallID %q, got %q", "tc1", msgs[0].ToolCallID)
	}
}
