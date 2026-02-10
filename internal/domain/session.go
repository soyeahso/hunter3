package domain

import "time"

// SessionKey uniquely identifies a conversation session.
type SessionKey struct {
	ChannelID string `json:"channelId"`
	AccountID string `json:"accountId,omitempty"`
	ChatID    string `json:"chatId"`
	SenderID  string `json:"senderId,omitempty"`
}

// String returns a canonical string form of the session key.
func (k SessionKey) String() string {
	s := k.ChannelID + ":" + k.ChatID
	if k.SenderID != "" {
		s += ":" + k.SenderID
	}
	return s
}

// Session tracks a conversation between a user and the agent.
type Session struct {
	ID        string     `json:"id"`
	Key       SessionKey `json:"key"`
	AgentID   string     `json:"agentId"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Messages  []Message  `json:"messages,omitempty"`
}

// Message is a single turn in a conversation (used in session history).
type Message struct {
	Role      string    `json:"role"` // "user", "assistant", "system", "tool"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
}

// ToolCall represents an LLM tool invocation within a message.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"`    // JSON string
	Output   string `json:"output"`   // JSON string
}
