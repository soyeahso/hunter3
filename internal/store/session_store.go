package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
)

// SQLiteSessionStore implements agent.SessionStore backed by SQLite.
type SQLiteSessionStore struct {
	db *DB
}

// NewSQLiteSessionStore creates a session store using the given database.
func NewSQLiteSessionStore(db *DB) *SQLiteSessionStore {
	return &SQLiteSessionStore{db: db}
}

// GetOrCreate finds an existing session by key or creates a new one.
func (s *SQLiteSessionStore) GetOrCreate(key domain.SessionKey, agentID string) *domain.Session {
	keyStr := key.String()

	// Try to find existing session
	var sess domain.Session
	var createdAt, updatedAt string
	err := s.db.sql.QueryRow(
		`SELECT id, key_str, channel_id, account_id, chat_id, sender_id, agent_id, created_at, updated_at
		 FROM sessions WHERE key_str = ?`, keyStr,
	).Scan(
		&sess.ID, &keyStr, &sess.Key.ChannelID, &sess.Key.AccountID,
		&sess.Key.ChatID, &sess.Key.SenderID, &sess.AgentID,
		&createdAt, &updatedAt,
	)

	if err == nil {
		sess.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
		sess.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
		return &sess
	}

	// Create new session
	sess = domain.Session{
		ID:        uuid.New().String(),
		Key:       key,
		AgentID:   agentID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = s.db.sql.Exec(
		`INSERT INTO sessions (id, key_str, channel_id, account_id, chat_id, sender_id, agent_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, keyStr, key.ChannelID, key.AccountID, key.ChatID, key.SenderID, agentID,
		sess.CreatedAt.Format(time.DateTime), sess.UpdatedAt.Format(time.DateTime),
	)
	if err != nil {
		s.db.log.Error().Err(err).Str("key", keyStr).Msg("failed to create session")
	}

	return &sess
}

// Get returns a session by ID, or nil if not found.
func (s *SQLiteSessionStore) Get(id string) *domain.Session {
	var sess domain.Session
	var createdAt, updatedAt string

	err := s.db.sql.QueryRow(
		`SELECT id, key_str, channel_id, account_id, chat_id, sender_id, agent_id, created_at, updated_at
		 FROM sessions WHERE id = ?`, id,
	).Scan(
		&sess.ID, new(string), &sess.Key.ChannelID, &sess.Key.AccountID,
		&sess.Key.ChatID, &sess.Key.SenderID, &sess.AgentID,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil
	}

	sess.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
	sess.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)

	// Load messages
	sess.Messages = s.loadMessages(id)
	return &sess
}

// Append adds a message to a session.
func (s *SQLiteSessionStore) Append(sessionID string, msg domain.Message) {
	var toolCallsJSON sql.NullString
	if len(msg.ToolCalls) > 0 {
		if data, err := json.Marshal(msg.ToolCalls); err == nil {
			toolCallsJSON = sql.NullString{String: string(data), Valid: true}
		}
	}

	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	_, err := s.db.sql.Exec(
		`INSERT INTO messages (session_id, role, content, timestamp, tool_calls)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, msg.Role, msg.Content, ts.Format(time.DateTime), toolCallsJSON,
	)
	if err != nil {
		s.db.log.Error().Err(err).Str("session", sessionID).Msg("failed to append message")
		return
	}

	// Update session timestamp
	_, _ = s.db.sql.Exec(
		`UPDATE sessions SET updated_at = ? WHERE id = ?`,
		time.Now().Format(time.DateTime), sessionID,
	)
}

// History returns the message history for a session as LLM messages.
func (s *SQLiteSessionStore) History(sessionID string) []llm.Message {
	rows, err := s.db.sql.Query(
		`SELECT role, content FROM messages WHERE session_id = ? ORDER BY id`, sessionID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var msgs []llm.Message
	for rows.Next() {
		var m llm.Message
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs
}

// List returns all session IDs.
func (s *SQLiteSessionStore) List() []string {
	rows, err := s.db.sql.Query(`SELECT id FROM sessions ORDER BY updated_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// loadMessages loads all messages for a session.
func (s *SQLiteSessionStore) loadMessages(sessionID string) []domain.Message {
	rows, err := s.db.sql.Query(
		`SELECT role, content, timestamp, tool_calls
		 FROM messages WHERE session_id = ? ORDER BY id`, sessionID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var msg domain.Message
		var ts string
		var toolCallsJSON sql.NullString

		if err := rows.Scan(&msg.Role, &msg.Content, &ts, &toolCallsJSON); err != nil {
			continue
		}
		msg.Timestamp, _ = time.Parse(time.DateTime, ts)

		if toolCallsJSON.Valid && toolCallsJSON.String != "" {
			_ = json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls)
		}

		msgs = append(msgs, msg)
	}
	return msgs
}
