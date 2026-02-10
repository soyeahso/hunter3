package store

// migration represents a single schema migration.
type migration struct {
	Version int
	Name    string
	SQL     string
}

// migrations is the ordered list of all schema migrations.
var migrations = []migration{
	{
		Version: 1,
		Name:    "create sessions and messages",
		SQL: `
			CREATE TABLE sessions (
				id          TEXT PRIMARY KEY,
				key_str     TEXT NOT NULL,
				channel_id  TEXT NOT NULL,
				account_id  TEXT NOT NULL DEFAULT '',
				chat_id     TEXT NOT NULL,
				sender_id   TEXT NOT NULL DEFAULT '',
				agent_id    TEXT NOT NULL,
				created_at  TEXT NOT NULL DEFAULT (datetime('now')),
				updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
			);

			CREATE UNIQUE INDEX idx_sessions_key ON sessions (key_str);
			CREATE INDEX idx_sessions_channel ON sessions (channel_id);
			CREATE INDEX idx_sessions_agent ON sessions (agent_id);

			CREATE TABLE messages (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
				role        TEXT NOT NULL,
				content     TEXT NOT NULL,
				timestamp   TEXT NOT NULL DEFAULT (datetime('now')),
				tool_calls  TEXT,
				FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
			);

			CREATE INDEX idx_messages_session ON messages (session_id, id);
		`,
	},
	{
		Version: 2,
		Name:    "create memory chunks with FTS5",
		SQL: `
			CREATE TABLE memory_chunks (
				id          TEXT PRIMARY KEY,
				agent_id    TEXT NOT NULL,
				session_id  TEXT NOT NULL DEFAULT '',
				category    TEXT NOT NULL DEFAULT 'general',
				content     TEXT NOT NULL,
				metadata    TEXT,
				created_at  TEXT NOT NULL DEFAULT (datetime('now')),
				updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
			);

			CREATE INDEX idx_memory_agent ON memory_chunks (agent_id);
			CREATE INDEX idx_memory_category ON memory_chunks (agent_id, category);

			CREATE VIRTUAL TABLE memory_fts USING fts5(
				content,
				category,
				content='memory_chunks',
				content_rowid='rowid'
			);

			CREATE TRIGGER memory_ai AFTER INSERT ON memory_chunks BEGIN
				INSERT INTO memory_fts(rowid, content, category)
				VALUES (new.rowid, new.content, new.category);
			END;

			CREATE TRIGGER memory_ad AFTER DELETE ON memory_chunks BEGIN
				INSERT INTO memory_fts(memory_fts, rowid, content, category)
				VALUES ('delete', old.rowid, old.content, old.category);
			END;

			CREATE TRIGGER memory_au AFTER UPDATE ON memory_chunks BEGIN
				INSERT INTO memory_fts(memory_fts, rowid, content, category)
				VALUES ('delete', old.rowid, old.content, old.category);
				INSERT INTO memory_fts(rowid, content, category)
				VALUES (new.rowid, new.content, new.category);
			END;
		`,
	},
}
