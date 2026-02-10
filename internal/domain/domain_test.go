package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SessionKey tests ---

func TestSessionKeyString(t *testing.T) {
	tests := []struct {
		name string
		key  SessionKey
		want string
	}{
		{
			name: "with sender",
			key:  SessionKey{ChannelID: "irc", ChatID: "#general", SenderID: "alice"},
			want: "irc:#general:alice",
		},
		{
			name: "without sender",
			key:  SessionKey{ChannelID: "irc", ChatID: "#general"},
			want: "irc:#general",
		},
		{
			name: "with account",
			key:  SessionKey{ChannelID: "discord", AccountID: "acc1", ChatID: "room1", SenderID: "bob"},
			want: "discord:room1:bob",
		},
		{
			name: "empty fields",
			key:  SessionKey{},
			want: ":",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.key.String())
		})
	}
}

func TestSessionKeyEquality(t *testing.T) {
	k1 := SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	k2 := SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "alice"}
	k3 := SessionKey{ChannelID: "irc", ChatID: "#test", SenderID: "bob"}

	assert.Equal(t, k1, k2)
	assert.NotEqual(t, k1, k3)
	assert.Equal(t, k1.String(), k2.String())
	assert.NotEqual(t, k1.String(), k3.String())
}

// --- ChatType tests ---

func TestChatTypeConstants(t *testing.T) {
	assert.Equal(t, ChatType("dm"), ChatTypeDM)
	assert.Equal(t, ChatType("group"), ChatTypeGroup)
	assert.Equal(t, ChatType("thread"), ChatTypeThread)
}

// --- JSON serialization tests ---

func TestInboundMessageJSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	msg := InboundMessage{
		ID:        "msg-1",
		ChannelID: "irc",
		AccountID: "acc-1",
		From:      "alice",
		FromName:  "Alice",
		ChatID:    "#general",
		ChatType:  ChatTypeDM,
		Body:      "hello world",
		Timestamp: now,
		ReplyToID: "msg-0",
		ThreadID:  "thread-1",
		Media: []Attachment{
			{ID: "att-1", URL: "https://example.com/file.png", MimeType: "image/png", Filename: "file.png", Size: 1024},
		},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded InboundMessage
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.ChannelID, decoded.ChannelID)
	assert.Equal(t, msg.AccountID, decoded.AccountID)
	assert.Equal(t, msg.From, decoded.From)
	assert.Equal(t, msg.FromName, decoded.FromName)
	assert.Equal(t, msg.ChatID, decoded.ChatID)
	assert.Equal(t, msg.ChatType, decoded.ChatType)
	assert.Equal(t, msg.Body, decoded.Body)
	assert.Equal(t, msg.ReplyToID, decoded.ReplyToID)
	assert.Equal(t, msg.ThreadID, decoded.ThreadID)
	assert.Len(t, decoded.Media, 1)
	assert.Equal(t, "att-1", decoded.Media[0].ID)
	assert.Equal(t, int64(1024), decoded.Media[0].Size)
}

func TestInboundMessageJSON_OmitsEmpty(t *testing.T) {
	msg := InboundMessage{
		ID:        "msg-1",
		ChannelID: "irc",
		From:      "alice",
		ChatID:    "#general",
		ChatType:  ChatTypeDM,
		Body:      "hello",
		Timestamp: time.Now().UTC(),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, "accountId")
	assert.NotContains(t, raw, "fromName")
	assert.NotContains(t, raw, "replyToId")
	assert.NotContains(t, raw, "threadId")
	assert.NotContains(t, raw, "media")
}

func TestOutboundMessageJSON(t *testing.T) {
	msg := OutboundMessage{
		ChannelID: "irc",
		To:        "#general",
		Body:      "response",
		ReplyToID: "msg-1",
		ThreadID:  "thread-1",
		Media: []Attachment{
			{URL: "https://example.com/img.jpg", MimeType: "image/jpeg"},
		},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded OutboundMessage
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, msg.ChannelID, decoded.ChannelID)
	assert.Equal(t, msg.To, decoded.To)
	assert.Equal(t, msg.Body, decoded.Body)
	assert.Equal(t, msg.ReplyToID, decoded.ReplyToID)
	assert.Len(t, decoded.Media, 1)
}

func TestOutboundMessageJSON_OmitsEmpty(t *testing.T) {
	msg := OutboundMessage{
		ChannelID: "irc",
		To:        "#general",
		Body:      "hi",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, "accountId")
	assert.NotContains(t, raw, "replyToId")
	assert.NotContains(t, raw, "threadId")
	assert.NotContains(t, raw, "media")
}

func TestAttachmentJSON(t *testing.T) {
	att := Attachment{
		ID:       "att-1",
		URL:      "https://example.com/doc.pdf",
		MimeType: "application/pdf",
		Filename: "doc.pdf",
		Size:     2048,
	}

	data, err := json.Marshal(att)
	require.NoError(t, err)

	var decoded Attachment
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, att, decoded)
}

func TestAttachmentJSON_OmitsEmpty(t *testing.T) {
	att := Attachment{}
	data, err := json.Marshal(att)
	require.NoError(t, err)

	raw := string(data)
	assert.Equal(t, "{}", raw)
}

// --- Session and Message tests ---

func TestSessionJSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	session := Session{
		ID:        "sess-1",
		Key:       SessionKey{ChannelID: "irc", ChatID: "#general", SenderID: "alice"},
		AgentID:   "agent-1",
		CreatedAt: now,
		UpdatedAt: now,
		Messages: []Message{
			{Role: "user", Content: "hello", Timestamp: now},
			{Role: "assistant", Content: "hi there", Timestamp: now},
		},
	}

	data, err := json.Marshal(session)
	require.NoError(t, err)

	var decoded Session
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, session.ID, decoded.ID)
	assert.Equal(t, session.Key, decoded.Key)
	assert.Equal(t, session.AgentID, decoded.AgentID)
	assert.Len(t, decoded.Messages, 2)
	assert.Equal(t, "user", decoded.Messages[0].Role)
	assert.Equal(t, "assistant", decoded.Messages[1].Role)
}

func TestSessionJSON_OmitsEmptyMessages(t *testing.T) {
	session := Session{
		ID:      "sess-1",
		Key:     SessionKey{ChannelID: "irc", ChatID: "#general"},
		AgentID: "agent-1",
	}

	data, err := json.Marshal(session)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, "messages")
}

func TestMessageJSON_WithToolCalls(t *testing.T) {
	msg := Message{
		Role:      "assistant",
		Content:   "calling tool",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		ToolCalls: []ToolCall{
			{
				ID:     "tc-1",
				Name:   "weather",
				Input:  `{"city":"NYC"}`,
				Output: `{"temp":72}`,
			},
		},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, msg.Role, decoded.Role)
	assert.Len(t, decoded.ToolCalls, 1)
	assert.Equal(t, "weather", decoded.ToolCalls[0].Name)
	assert.Equal(t, `{"city":"NYC"}`, decoded.ToolCalls[0].Input)
	assert.Equal(t, `{"temp":72}`, decoded.ToolCalls[0].Output)
}

func TestMessageJSON_OmitsEmptyToolCalls(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "hello",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, "toolCalls")
}

// --- Agent tests ---

func TestAgentJSON(t *testing.T) {
	agent := Agent{
		ID:        "agent-1",
		Name:      "TestBot",
		Model:     "claude",
		Workspace: "/tmp/workspace",
		IsDefault: true,
	}

	data, err := json.Marshal(agent)
	require.NoError(t, err)

	var decoded Agent
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, agent, decoded)
}

func TestAgentJSON_OmitsEmpty(t *testing.T) {
	agent := Agent{
		ID:    "agent-1",
		Name:  "Bot",
		Model: "claude",
	}

	data, err := json.Marshal(agent)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, "workspace")
	assert.NotContains(t, raw, "isDefault")
}

// --- ChannelCapabilities tests ---

func TestChannelCapabilitiesJSON(t *testing.T) {
	caps := ChannelCapabilities{
		ChatTypes: []ChatType{ChatTypeDM, ChatTypeGroup},
		Media:     true,
		Reactions: true,
		Edit:      true,
		Threads:   true,
		Reply:     true,
	}

	data, err := json.Marshal(caps)
	require.NoError(t, err)

	var decoded ChannelCapabilities
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, caps, decoded)
}

func TestChannelStatusJSON(t *testing.T) {
	status := ChannelStatus{
		ChannelID: "irc",
		AccountID: "acc-1",
		Connected: true,
		Running:   true,
		LastError: "",
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)

	var decoded ChannelStatus
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, status.ChannelID, decoded.ChannelID)
	assert.True(t, decoded.Connected)
	assert.True(t, decoded.Running)
}

func TestChannelStatusJSON_OmitsEmpty(t *testing.T) {
	status := ChannelStatus{
		ChannelID: "irc",
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)

	raw := string(data)
	assert.NotContains(t, raw, "accountId")
	assert.NotContains(t, raw, "lastError")
}

// --- ToolCall tests ---

func TestToolCallJSON(t *testing.T) {
	tc := ToolCall{
		ID:     "tc-1",
		Name:   "search",
		Input:  `{"query":"test"}`,
		Output: `{"results":["a","b"]}`,
	}

	data, err := json.Marshal(tc)
	require.NoError(t, err)

	var decoded ToolCall
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, tc, decoded)
}
