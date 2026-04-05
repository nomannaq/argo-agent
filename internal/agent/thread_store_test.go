package agent

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nomanqureshi/argo/internal/llm"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_threads.db")
	store, err := NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStoreAt(%q) failed: %v", dbPath, err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestNewSQLiteStoreAt_CreatesDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStoreAt failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	if store.db == nil {
		t.Fatal("expected db to be non-nil")
	}

	// Verify that the tables exist by querying them.
	var count int
	err = store.db.QueryRow(`SELECT COUNT(*) FROM threads`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query threads table: %v", err)
	}

	err = store.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query messages table: %v", err)
	}
}

func TestSaveAndLoadThread_RoundTrip(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()
	thread.AddMessage(llm.Message{Role: llm.RoleUser, Content: "hello"})
	thread.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "hi there"})

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	loaded, err := store.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("LoadThread failed: %v", err)
	}

	if loaded.ID != thread.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, thread.ID)
	}

	// CreatedAt should be close (within 1 second) since we serialize to RFC3339Nano.
	diff := loaded.CreatedAt.Sub(thread.CreatedAt.UTC())
	if diff < -time.Second || diff > time.Second {
		t.Errorf("CreatedAt mismatch: got %v, want ~%v", loaded.CreatedAt, thread.CreatedAt)
	}

	loadedMsgs := loaded.Messages()
	origMsgs := thread.Messages()
	if len(loadedMsgs) != len(origMsgs) {
		t.Fatalf("message count mismatch: got %d, want %d", len(loadedMsgs), len(origMsgs))
	}

	for i := range origMsgs {
		if loadedMsgs[i].Role != origMsgs[i].Role {
			t.Errorf("message[%d].Role: got %q, want %q", i, loadedMsgs[i].Role, origMsgs[i].Role)
		}
		if loadedMsgs[i].Content != origMsgs[i].Content {
			t.Errorf("message[%d].Content: got %q, want %q", i, loadedMsgs[i].Content, origMsgs[i].Content)
		}
	}
}

func TestSaveThread_PreservesMessageOrder(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()
	roles := []llm.Role{llm.RoleSystem, llm.RoleUser, llm.RoleAssistant, llm.RoleUser, llm.RoleAssistant}
	for i, role := range roles {
		thread.AddMessage(llm.Message{
			Role:    role,
			Content: strings.Repeat("x", i+1),
		})
	}

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	loaded, err := store.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("LoadThread failed: %v", err)
	}

	loadedMsgs := loaded.Messages()
	if len(loadedMsgs) != len(roles) {
		t.Fatalf("message count: got %d, want %d", len(loadedMsgs), len(roles))
	}

	for i, role := range roles {
		if loadedMsgs[i].Role != role {
			t.Errorf("message[%d].Role: got %q, want %q", i, loadedMsgs[i].Role, role)
		}
		expectedContent := strings.Repeat("x", i+1)
		if loadedMsgs[i].Content != expectedContent {
			t.Errorf("message[%d].Content: got %q, want %q", i, loadedMsgs[i].Content, expectedContent)
		}
	}
}

func TestSaveThread_PreservesToolCalls(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()
	thread.AddMessage(llm.Message{Role: llm.RoleUser, Content: "use the tool"})
	thread.AddMessage(llm.Message{
		Role:    llm.RoleAssistant,
		Content: "",
		ToolCalls: []llm.ToolCall{
			{ID: "call_1", Name: "read_file", Arguments: `{"path":"main.go"}`},
			{ID: "call_2", Name: "write_file", Arguments: `{"path":"out.go","content":"pkg"}`},
		},
	})
	thread.AddMessage(llm.Message{
		Role:       llm.RoleTool,
		Content:    "file contents here",
		ToolCallID: "call_1",
	})

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	loaded, err := store.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("LoadThread failed: %v", err)
	}

	msgs := loaded.Messages()
	if len(msgs) != 3 {
		t.Fatalf("message count: got %d, want 3", len(msgs))
	}

	// Check tool calls on assistant message.
	assistantMsg := msgs[1]
	if len(assistantMsg.ToolCalls) != 2 {
		t.Fatalf("tool calls count: got %d, want 2", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].ID != "call_1" {
		t.Errorf("ToolCalls[0].ID: got %q, want %q", assistantMsg.ToolCalls[0].ID, "call_1")
	}
	if assistantMsg.ToolCalls[0].Name != "read_file" {
		t.Errorf("ToolCalls[0].Name: got %q, want %q", assistantMsg.ToolCalls[0].Name, "read_file")
	}
	if assistantMsg.ToolCalls[0].Arguments != `{"path":"main.go"}` {
		t.Errorf("ToolCalls[0].Arguments: got %q, want %q", assistantMsg.ToolCalls[0].Arguments, `{"path":"main.go"}`)
	}
	if assistantMsg.ToolCalls[1].ID != "call_2" {
		t.Errorf("ToolCalls[1].ID: got %q, want %q", assistantMsg.ToolCalls[1].ID, "call_2")
	}

	// Check tool result message.
	toolMsg := msgs[2]
	if toolMsg.ToolCallID != "call_1" {
		t.Errorf("ToolCallID: got %q, want %q", toolMsg.ToolCallID, "call_1")
	}
	if toolMsg.Content != "file contents here" {
		t.Errorf("tool result Content: got %q, want %q", toolMsg.Content, "file contents here")
	}
}

func TestSaveThread_Upserts(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()
	thread.AddMessage(llm.Message{Role: llm.RoleUser, Content: "first message"})

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("first SaveThread failed: %v", err)
	}

	// Add another message and save again.
	thread.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "response"})

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("second SaveThread failed: %v", err)
	}

	loaded, err := store.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("LoadThread failed: %v", err)
	}

	msgs := loaded.Messages()
	if len(msgs) != 2 {
		t.Errorf("message count after upsert: got %d, want 2", len(msgs))
	}

	// Also verify that there's only one thread entry.
	summaries, err := store.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Errorf("thread count after upsert: got %d, want 1", len(summaries))
	}
}

func TestLoadThread_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.LoadThread("non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent thread, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestListThreads_SortedByCreatedAtDesc(t *testing.T) {
	store := newTestStore(t)

	// Create threads with staggered timestamps.
	thread1 := NewThread()
	thread1.CreatedAt = time.Now().Add(-3 * time.Hour)
	thread1.AddMessage(llm.Message{Role: llm.RoleUser, Content: "oldest"})

	thread2 := NewThread()
	thread2.CreatedAt = time.Now().Add(-1 * time.Hour)
	thread2.AddMessage(llm.Message{Role: llm.RoleUser, Content: "middle"})

	thread3 := NewThread()
	thread3.CreatedAt = time.Now()
	thread3.AddMessage(llm.Message{Role: llm.RoleUser, Content: "newest"})

	for _, th := range []*Thread{thread1, thread2, thread3} {
		if err := store.SaveThread(th); err != nil {
			t.Fatalf("SaveThread failed: %v", err)
		}
	}

	summaries, err := store.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("summary count: got %d, want 3", len(summaries))
	}

	// Should be newest first.
	if summaries[0].ID != thread3.ID {
		t.Errorf("first summary should be newest thread, got ID %q, want %q", summaries[0].ID, thread3.ID)
	}
	if summaries[1].ID != thread2.ID {
		t.Errorf("second summary should be middle thread, got ID %q, want %q", summaries[1].ID, thread2.ID)
	}
	if summaries[2].ID != thread1.ID {
		t.Errorf("third summary should be oldest thread, got ID %q, want %q", summaries[2].ID, thread1.ID)
	}
}

func TestListThreads_CorrectMessageCounts(t *testing.T) {
	store := newTestStore(t)

	thread1 := NewThread()
	thread1.AddMessage(llm.Message{Role: llm.RoleUser, Content: "one"})

	thread2 := NewThread()
	thread2.AddMessage(llm.Message{Role: llm.RoleUser, Content: "two"})
	thread2.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "reply"})
	thread2.AddMessage(llm.Message{Role: llm.RoleUser, Content: "three"})

	if err := store.SaveThread(thread1); err != nil {
		t.Fatalf("SaveThread 1 failed: %v", err)
	}
	if err := store.SaveThread(thread2); err != nil {
		t.Fatalf("SaveThread 2 failed: %v", err)
	}

	summaries, err := store.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}

	counts := map[string]int{}
	for _, s := range summaries {
		counts[s.ID] = s.MessageCount
	}

	if counts[thread1.ID] != 1 {
		t.Errorf("thread1 message count: got %d, want 1", counts[thread1.ID])
	}
	if counts[thread2.ID] != 3 {
		t.Errorf("thread2 message count: got %d, want 3", counts[thread2.ID])
	}
}

func TestListThreads_EmptyStore(t *testing.T) {
	store := newTestStore(t)

	summaries, err := store.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}

	if len(summaries) != 0 {
		t.Errorf("expected empty or nil summaries, got %d", len(summaries))
	}
}

func TestDeleteThread_RemovesThreadAndMessages(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()
	thread.AddMessage(llm.Message{Role: llm.RoleUser, Content: "to be deleted"})
	thread.AddMessage(llm.Message{Role: llm.RoleAssistant, Content: "will be gone"})

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	if err := store.DeleteThread(thread.ID); err != nil {
		t.Fatalf("DeleteThread failed: %v", err)
	}

	// LoadThread should fail now.
	_, err := store.LoadThread(thread.ID)
	if err == nil {
		t.Fatal("expected error loading deleted thread, got nil")
	}

	// ListThreads should be empty.
	summaries, err := store.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected no threads after delete, got %d", len(summaries))
	}

	// Verify messages are also gone by checking the DB directly.
	var msgCount int
	err = store.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE thread_id = ?`, thread.ID).Scan(&msgCount)
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}
	if msgCount != 0 {
		t.Errorf("expected 0 orphan messages, got %d", msgCount)
	}
}

func TestDeleteThread_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.DeleteThread("non-existent-id")
	if err == nil {
		t.Fatal("expected error for deleting non-existent thread, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestClose_ClosesDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "close_test.db")
	store, err := NewSQLiteStoreAt(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStoreAt failed: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// After closing, database operations should fail.
	err = store.db.Ping()
	if err == nil {
		t.Error("expected error pinging closed database, got nil")
	}
}

func TestDeriveTitle_FromFirstUserMessage(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: "Hello, what can you do?"},
		{Role: llm.RoleAssistant, Content: "I can help with many things."},
	}

	title := deriveTitle(msgs)
	if title != "Hello, what can you do?" {
		t.Errorf("deriveTitle: got %q, want %q", title, "Hello, what can you do?")
	}
}

func TestDeriveTitle_TruncatesLongMessages(t *testing.T) {
	longContent := strings.Repeat("a", 150)
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: longContent},
	}

	title := deriveTitle(msgs)
	expectedPrefix := strings.Repeat("a", 100)
	if !strings.HasPrefix(title, expectedPrefix) {
		t.Errorf("expected title to start with 100 'a's")
	}
	if !strings.HasSuffix(title, "…") {
		t.Errorf("expected title to end with ellipsis, got %q", title[len(title)-4:])
	}
}

func TestDeriveTitle_EmptyWhenNoUserMessages(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleAssistant, Content: "hello"},
	}

	title := deriveTitle(msgs)
	if title != "" {
		t.Errorf("deriveTitle: got %q, want empty string", title)
	}
}

func TestDeriveTitle_EmptySlice(t *testing.T) {
	title := deriveTitle(nil)
	if title != "" {
		t.Errorf("deriveTitle(nil): got %q, want empty string", title)
	}
}

func TestSaveThread_PreservesTitle(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()
	thread.AddMessage(llm.Message{Role: llm.RoleUser, Content: "What is Go?"})

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	summaries, err := store.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	if summaries[0].Title != "What is Go?" {
		t.Errorf("title: got %q, want %q", summaries[0].Title, "What is Go?")
	}
}

func TestDeleteThread_DoesNotAffectOtherThreads(t *testing.T) {
	store := newTestStore(t)

	thread1 := NewThread()
	thread1.AddMessage(llm.Message{Role: llm.RoleUser, Content: "keep me"})

	thread2 := NewThread()
	thread2.AddMessage(llm.Message{Role: llm.RoleUser, Content: "delete me"})

	if err := store.SaveThread(thread1); err != nil {
		t.Fatalf("SaveThread 1 failed: %v", err)
	}
	if err := store.SaveThread(thread2); err != nil {
		t.Fatalf("SaveThread 2 failed: %v", err)
	}

	if err := store.DeleteThread(thread2.ID); err != nil {
		t.Fatalf("DeleteThread failed: %v", err)
	}

	// thread1 should still be loadable.
	loaded, err := store.LoadThread(thread1.ID)
	if err != nil {
		t.Fatalf("LoadThread for surviving thread failed: %v", err)
	}
	if loaded.ID != thread1.ID {
		t.Errorf("loaded thread ID: got %q, want %q", loaded.ID, thread1.ID)
	}

	msgs := loaded.Messages()
	if len(msgs) != 1 || msgs[0].Content != "keep me" {
		t.Errorf("surviving thread messages are incorrect")
	}
}

func TestSaveThread_EmptyMessages(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	loaded, err := store.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("LoadThread failed: %v", err)
	}

	msgs := loaded.Messages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestSaveThread_UserMessageWithEmptyContent(t *testing.T) {
	store := newTestStore(t)

	thread := NewThread()
	thread.AddMessage(llm.Message{Role: llm.RoleUser, Content: ""})
	thread.AddMessage(llm.Message{Role: llm.RoleUser, Content: "actual content"})

	if err := store.SaveThread(thread); err != nil {
		t.Fatalf("SaveThread failed: %v", err)
	}

	loaded, err := store.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("LoadThread failed: %v", err)
	}

	msgs := loaded.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "" {
		t.Errorf("msg[0].Content: got %q, want empty", msgs[0].Content)
	}
	if msgs[1].Content != "actual content" {
		t.Errorf("msg[1].Content: got %q, want %q", msgs[1].Content, "actual content")
	}
}
