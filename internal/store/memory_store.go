package store

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// MemoryChunk is a piece of knowledge stored in the memory system.
type MemoryChunk struct {
	ID        string `json:"id"`
	AgentID   string `json:"agentId"`
	SessionID string `json:"sessionId,omitempty"`
	Category  string `json:"category"`
	Content   string `json:"content"`
	Metadata  string `json:"metadata,omitempty"` // JSON blob
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Rank      float64 `json:"rank,omitempty"` // FTS5 rank score (search results only)
}

// MemoryStore manages knowledge chunks with full-text search via SQLite FTS5.
type MemoryStore struct {
	db *DB
}

// NewMemoryStore creates a memory store using the given database.
func NewMemoryStore(db *DB) *MemoryStore {
	return &MemoryStore{db: db}
}

// Store inserts or updates a memory chunk.
func (m *MemoryStore) Store(chunk MemoryChunk) (*MemoryChunk, error) {
	if chunk.ID == "" {
		chunk.ID = uuid.New().String()
	}
	if chunk.Category == "" {
		chunk.Category = "general"
	}

	now := time.Now()
	chunk.CreatedAt = now
	chunk.UpdatedAt = now

	var metadata sql.NullString
	if chunk.Metadata != "" {
		metadata = sql.NullString{String: chunk.Metadata, Valid: true}
	}

	_, err := m.db.sql.Exec(
		`INSERT INTO memory_chunks (id, agent_id, session_id, category, content, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   content = excluded.content,
		   category = excluded.category,
		   metadata = excluded.metadata,
		   updated_at = excluded.updated_at`,
		chunk.ID, chunk.AgentID, chunk.SessionID, chunk.Category,
		chunk.Content, metadata,
		now.Format(time.DateTime), now.Format(time.DateTime),
	)
	if err != nil {
		return nil, err
	}

	return &chunk, nil
}

// Search finds memory chunks matching the query using FTS5.
// Results are ranked by relevance. Limit of 0 defaults to 20.
func (m *MemoryStore) Search(agentID, query string, limit int) ([]MemoryChunk, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := m.db.sql.Query(
		`SELECT mc.id, mc.agent_id, mc.session_id, mc.category, mc.content, mc.metadata,
		        mc.created_at, mc.updated_at, rank
		 FROM memory_fts
		 JOIN memory_chunks mc ON mc.rowid = memory_fts.rowid
		 WHERE memory_fts MATCH ?
		   AND mc.agent_id = ?
		 ORDER BY rank
		 LIMIT ?`,
		query, agentID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChunks(rows)
}

// SearchByCategory searches within a specific category.
func (m *MemoryStore) SearchByCategory(agentID, category, query string, limit int) ([]MemoryChunk, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := m.db.sql.Query(
		`SELECT mc.id, mc.agent_id, mc.session_id, mc.category, mc.content, mc.metadata,
		        mc.created_at, mc.updated_at, rank
		 FROM memory_fts
		 JOIN memory_chunks mc ON mc.rowid = memory_fts.rowid
		 WHERE memory_fts MATCH ?
		   AND mc.agent_id = ?
		   AND mc.category = ?
		 ORDER BY rank
		 LIMIT ?`,
		query, agentID, category, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChunks(rows)
}

// ListByAgent returns all chunks for an agent, optionally filtered by category.
func (m *MemoryStore) ListByAgent(agentID, category string, limit int) ([]MemoryChunk, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error

	if category != "" {
		rows, err = m.db.sql.Query(
			`SELECT id, agent_id, session_id, category, content, metadata, created_at, updated_at, 0
			 FROM memory_chunks WHERE agent_id = ? AND category = ?
			 ORDER BY updated_at DESC LIMIT ?`,
			agentID, category, limit,
		)
	} else {
		rows, err = m.db.sql.Query(
			`SELECT id, agent_id, session_id, category, content, metadata, created_at, updated_at, 0
			 FROM memory_chunks WHERE agent_id = ?
			 ORDER BY updated_at DESC LIMIT ?`,
			agentID, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChunks(rows)
}

// Delete removes a memory chunk by ID.
func (m *MemoryStore) Delete(id string) error {
	_, err := m.db.sql.Exec(`DELETE FROM memory_chunks WHERE id = ?`, id)
	return err
}

// DeleteByAgent removes all memory chunks for an agent.
func (m *MemoryStore) DeleteByAgent(agentID string) error {
	_, err := m.db.sql.Exec(`DELETE FROM memory_chunks WHERE agent_id = ?`, agentID)
	return err
}

func scanChunks(rows *sql.Rows) ([]MemoryChunk, error) {
	var chunks []MemoryChunk
	for rows.Next() {
		var chunk MemoryChunk
		var createdAt, updatedAt string
		var metadata sql.NullString

		if err := rows.Scan(
			&chunk.ID, &chunk.AgentID, &chunk.SessionID, &chunk.Category,
			&chunk.Content, &metadata, &createdAt, &updatedAt, &chunk.Rank,
		); err != nil {
			continue
		}

		chunk.CreatedAt, _ = time.Parse(time.DateTime, createdAt)
		chunk.UpdatedAt, _ = time.Parse(time.DateTime, updatedAt)
		if metadata.Valid {
			chunk.Metadata = metadata.String
		}

		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}
