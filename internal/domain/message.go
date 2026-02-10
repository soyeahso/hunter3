package domain

import "time"

// ChatType classifies the conversation context.
type ChatType string

const (
	ChatTypeDM     ChatType = "dm"
	ChatTypeGroup  ChatType = "group"
	ChatTypeThread ChatType = "thread"
)

// Attachment represents a file or media attachment on a message.
type Attachment struct {
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// InboundMessage is a message received from a channel.
type InboundMessage struct {
	ID        string       `json:"id"`
	ChannelID string       `json:"channelId"`
	AccountID string       `json:"accountId,omitempty"`
	From      string       `json:"from"`
	FromName  string       `json:"fromName,omitempty"`
	ChatID    string       `json:"chatId"`
	ChatType  ChatType     `json:"chatType"`
	Body      string       `json:"body"`
	Timestamp time.Time    `json:"timestamp"`
	ReplyToID string       `json:"replyToId,omitempty"`
	ThreadID  string       `json:"threadId,omitempty"`
	Media     []Attachment `json:"media,omitempty"`
	Raw       any          `json:"raw,omitempty"`
}

// OutboundMessage is a message to be sent via a channel.
type OutboundMessage struct {
	ChannelID string       `json:"channelId"`
	AccountID string       `json:"accountId,omitempty"`
	To        string       `json:"to"`
	Body      string       `json:"body"`
	ReplyToID string       `json:"replyToId,omitempty"`
	ThreadID  string       `json:"threadId,omitempty"`
	Media     []Attachment `json:"media,omitempty"`
}
