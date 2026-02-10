package agent

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/soyeahso/hunter3/internal/domain"
	"github.com/soyeahso/hunter3/internal/llm"
)

// SessionStore manages conversation sessions.
type SessionStore interface {
	// GetOrCreate finds an existing session by key or creates a new one.
	GetOrCreate(key domain.SessionKey, agentID string) *domain.Session

	// Get returns a session by ID, or nil if not found.
	Get(id string) *domain.Session

	// Append adds a message to a session.
	Append(sessionID string, msg domain.Message)

	// History returns the message history for a session as LLM messages.
	History(sessionID string) []llm.Message

	// List returns all session IDs.
	List() []string
}

// MemorySessionStore is an in-memory SessionStore implementation.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*domain.Session    // id → session
	byKey    map[string]string             // key string → session id
}

// NewMemorySessionStore creates an in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*domain.Session),
		byKey:    make(map[string]string),
	}
}

func (s *MemorySessionStore) GetOrCreate(key domain.SessionKey, agentID string) *domain.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyStr := key.String()
	if id, ok := s.byKey[keyStr]; ok {
		if sess, ok := s.sessions[id]; ok {
			return sess
		}
	}

	sess := &domain.Session{
		ID:        uuid.New().String(),
		Key:       key,
		AgentID:   agentID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[sess.ID] = sess
	s.byKey[keyStr] = sess.ID
	return sess
}

func (s *MemorySessionStore) Get(id string) *domain.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

func (s *MemorySessionStore) Append(sessionID string, msg domain.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[sessionID]; ok {
		sess.Messages = append(sess.Messages, msg)
		sess.UpdatedAt = time.Now()
	}
}

func (s *MemorySessionStore) History(sessionID string) []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}

	msgs := make([]llm.Message, 0, len(sess.Messages))
	for _, m := range sess.Messages {
		msgs = append(msgs, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return msgs
}

func (s *MemorySessionStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids
}
