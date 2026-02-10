package store

import (
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	log := logging.New(nil, "silent")
	db, err := Open(":memory:", log)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// --- DB/Migration tests ---

func TestOpen_InMemory(t *testing.T) {
	db := testDB(t)
	assert.NotNil(t, db)
	assert.NotNil(t, db.SQL())
}

func TestMigrations_Applied(t *testing.T) {
	db := testDB(t)

	var count int
	err := db.sql.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, len(migrations), count)
}

func TestMigrations_Idempotent(t *testing.T) {
	db := testDB(t)

	// Running migrate again should be a no-op
	err := db.migrate()
	require.NoError(t, err)

	var count int
	err = db.sql.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, len(migrations), count)
}

func TestSchema_TablesExist(t *testing.T) {
	db := testDB(t)

	tables := []string{"sessions", "messages", "memory_chunks", "memory_fts"}
	for _, table := range tables {
		var name string
		err := db.sql.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

// --- Session Store tests ---

func TestSessionStore_GetOrCreate_New(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	sess := ss.GetOrCreate(key, "agent-1")

	require.NotNil(t, sess)
	assert.NotEmpty(t, sess.ID)
	assert.Equal(t, "irc", sess.Key.ChannelID)
	assert.Equal(t, "#test", sess.Key.ChatID)
	assert.Equal(t, "alice", sess.Key.SenderID)
	assert.Equal(t, "agent-1", sess.AgentID)
}

func TestSessionStore_GetOrCreate_Existing(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	sess1 := ss.GetOrCreate(key, "agent-1")
	sess2 := ss.GetOrCreate(key, "agent-1")

	assert.Equal(t, sess1.ID, sess2.ID)
}

func TestSessionStore_GetOrCreate_DifferentKeys(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key1 := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	key2 := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "bob"}

	sess1 := ss.GetOrCreate(key1, "agent-1")
	sess2 := ss.GetOrCreate(key2, "agent-1")

	assert.NotEqual(t, sess1.ID, sess2.ID)
}

func TestSessionStore_Get(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	created := ss.GetOrCreate(key, "agent-1")

	got := ss.Get(created.ID)
	require.NotNil(t, got)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "irc", got.Key.ChannelID)
}

func TestSessionStore_Get_NotFound(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	got := ss.Get("nonexistent")
	assert.Nil(t, got)
}

func TestSessionStore_Append(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	sess := ss.GetOrCreate(key, "agent-1")

	ss.Append(sess.ID, domain.Message{
		Role:      "user",
		Content:   "Hello!",
		Timestamp: time.Now(),
	})
	ss.Append(sess.ID, domain.Message{
		Role:      "assistant",
		Content:   "Hi there!",
		Timestamp: time.Now(),
	})

	// Verify messages are in the database
	got := ss.Get(sess.ID)
	require.NotNil(t, got)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "Hello!", got.Messages[0].Content)
	assert.Equal(t, "assistant", got.Messages[1].Role)
	assert.Equal(t, "Hi there!", got.Messages[1].Content)
}

func TestSessionStore_Append_WithToolCalls(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	sess := ss.GetOrCreate(key, "agent-1")

	ss.Append(sess.ID, domain.Message{
		Role:    "assistant",
		Content: "Let me check.",
		ToolCalls: []domain.ToolCall{
			{ID: "tc-1", Name: "search", Input: `{"q":"test"}`, Output: `{"result":"found"}`},
		},
		Timestamp: time.Now(),
	})

	got := ss.Get(sess.ID)
	require.NotNil(t, got)
	require.Len(t, got.Messages, 1)
	require.Len(t, got.Messages[0].ToolCalls, 1)
	assert.Equal(t, "search", got.Messages[0].ToolCalls[0].Name)
	assert.Equal(t, `{"q":"test"}`, got.Messages[0].ToolCalls[0].Input)
}

func TestSessionStore_History(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key := domain.SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	sess := ss.GetOrCreate(key, "agent-1")

	ss.Append(sess.ID, domain.Message{Role: "user", Content: "Question", Timestamp: time.Now()})
	ss.Append(sess.ID, domain.Message{Role: "assistant", Content: "Answer", Timestamp: time.Now()})

	history := ss.History(sess.ID)
	require.Len(t, history, 2)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "Question", history[0].Content)
	assert.Equal(t, "assistant", history[1].Role)
	assert.Equal(t, "Answer", history[1].Content)
}

func TestSessionStore_History_Empty(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	history := ss.History("nonexistent")
	assert.Nil(t, history)
}

func TestSessionStore_List(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	key1 := domain.SessionKey{ChannelID: "irc", ChatID: "#a", SenderID: "alice"}
	key2 := domain.SessionKey{ChannelID: "irc", ChatID: "#b", SenderID: "bob"}

	ss.GetOrCreate(key1, "agent-1")
	ss.GetOrCreate(key2, "agent-1")

	ids := ss.List()
	assert.Len(t, ids, 2)
}

func TestSessionStore_List_Empty(t *testing.T) {
	db := testDB(t)
	ss := NewSQLiteSessionStore(db)

	ids := ss.List()
	assert.Nil(t, ids)
}

// --- Memory Store tests ---

func TestMemoryStore_Store(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	chunk, err := ms.Store(MemoryChunk{
		AgentID:  "agent-1",
		Category: "facts",
		Content:  "The sky is blue.",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, chunk.ID)
	assert.Equal(t, "facts", chunk.Category)
}

func TestMemoryStore_Store_DefaultCategory(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	chunk, err := ms.Store(MemoryChunk{
		AgentID: "agent-1",
		Content: "Something general.",
	})
	require.NoError(t, err)
	assert.Equal(t, "general", chunk.Category)
}

func TestMemoryStore_Store_Upsert(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	chunk1, err := ms.Store(MemoryChunk{
		AgentID:  "agent-1",
		Category: "facts",
		Content:  "Version 1",
	})
	require.NoError(t, err)

	// Update same chunk
	_, err = ms.Store(MemoryChunk{
		ID:       chunk1.ID,
		AgentID:  "agent-1",
		Category: "facts",
		Content:  "Version 2",
	})
	require.NoError(t, err)

	// Verify only one chunk exists
	chunks, err := ms.ListByAgent("agent-1", "", 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "Version 2", chunks[0].Content)
}

func TestMemoryStore_Search(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	_, err := ms.Store(MemoryChunk{
		AgentID:  "agent-1",
		Category: "facts",
		Content:  "Go is a statically typed compiled language.",
	})
	require.NoError(t, err)

	_, err = ms.Store(MemoryChunk{
		AgentID:  "agent-1",
		Category: "facts",
		Content:  "Python is an interpreted language.",
	})
	require.NoError(t, err)

	_, err = ms.Store(MemoryChunk{
		AgentID:  "agent-1",
		Category: "prefs",
		Content:  "User prefers dark mode.",
	})
	require.NoError(t, err)

	// Search for Go-related content
	results, err := ms.Search("agent-1", "Go compiled", 10)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Contains(t, results[0].Content, "Go")
}

func TestMemoryStore_Search_NoResults(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	_, err := ms.Store(MemoryChunk{
		AgentID: "agent-1",
		Content: "The sky is blue.",
	})
	require.NoError(t, err)

	results, err := ms.Search("agent-1", "nonexistent xyzzy", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestMemoryStore_Search_WrongAgent(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	_, err := ms.Store(MemoryChunk{
		AgentID: "agent-1",
		Content: "Secret knowledge.",
	})
	require.NoError(t, err)

	results, err := ms.Search("agent-2", "secret", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestMemoryStore_SearchByCategory(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	_, err := ms.Store(MemoryChunk{
		AgentID:  "agent-1",
		Category: "facts",
		Content:  "Go is a programming language.",
	})
	require.NoError(t, err)

	_, err = ms.Store(MemoryChunk{
		AgentID:  "agent-1",
		Category: "prefs",
		Content:  "User prefers Go over Python.",
	})
	require.NoError(t, err)

	results, err := ms.SearchByCategory("agent-1", "facts", "Go", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "facts", results[0].Category)
}

func TestMemoryStore_ListByAgent(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	_, _ = ms.Store(MemoryChunk{AgentID: "agent-1", Content: "chunk 1"})
	_, _ = ms.Store(MemoryChunk{AgentID: "agent-1", Content: "chunk 2"})
	_, _ = ms.Store(MemoryChunk{AgentID: "agent-2", Content: "chunk 3"})

	chunks, err := ms.ListByAgent("agent-1", "", 100)
	require.NoError(t, err)
	assert.Len(t, chunks, 2)
}

func TestMemoryStore_ListByAgent_WithCategory(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	_, _ = ms.Store(MemoryChunk{AgentID: "agent-1", Category: "facts", Content: "fact 1"})
	_, _ = ms.Store(MemoryChunk{AgentID: "agent-1", Category: "prefs", Content: "pref 1"})

	chunks, err := ms.ListByAgent("agent-1", "facts", 100)
	require.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "fact 1", chunks[0].Content)
}

func TestMemoryStore_Delete(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	chunk, _ := ms.Store(MemoryChunk{AgentID: "agent-1", Content: "to delete"})

	err := ms.Delete(chunk.ID)
	require.NoError(t, err)

	chunks, err := ms.ListByAgent("agent-1", "", 100)
	require.NoError(t, err)
	assert.Empty(t, chunks)
}

func TestMemoryStore_DeleteByAgent(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	_, _ = ms.Store(MemoryChunk{AgentID: "agent-1", Content: "chunk 1"})
	_, _ = ms.Store(MemoryChunk{AgentID: "agent-1", Content: "chunk 2"})
	_, _ = ms.Store(MemoryChunk{AgentID: "agent-2", Content: "chunk 3"})

	err := ms.DeleteByAgent("agent-1")
	require.NoError(t, err)

	chunks1, _ := ms.ListByAgent("agent-1", "", 100)
	assert.Empty(t, chunks1)

	chunks2, _ := ms.ListByAgent("agent-2", "", 100)
	assert.Len(t, chunks2, 1)
}

func TestMemoryStore_FTS_AfterDelete(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	chunk, _ := ms.Store(MemoryChunk{
		AgentID: "agent-1",
		Content: "unique searchable content xyzzy",
	})

	// Should find it
	results, err := ms.Search("agent-1", "xyzzy", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Delete it
	err = ms.Delete(chunk.ID)
	require.NoError(t, err)

	// Should no longer find it
	results, err = ms.Search("agent-1", "xyzzy", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestMemoryStore_FTS_AfterUpdate(t *testing.T) {
	db := testDB(t)
	ms := NewMemoryStore(db)

	chunk, _ := ms.Store(MemoryChunk{
		AgentID: "agent-1",
		Content: "original content alpha",
	})

	// Update content
	_, err := ms.Store(MemoryChunk{
		ID:      chunk.ID,
		AgentID: "agent-1",
		Content: "updated content beta",
	})
	require.NoError(t, err)

	// Should not find old content
	results, err := ms.Search("agent-1", "alpha", 10)
	require.NoError(t, err)
	assert.Empty(t, results)

	// Should find new content
	results, err = ms.Search("agent-1", "beta", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "beta")
}
