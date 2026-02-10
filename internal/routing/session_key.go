package routing

import "github.com/soyeahso/hunter3/internal/domain"

// ResolveSessionKey builds a session key from an inbound message and the configured scope.
//
// Scopes:
//   - "per-sender": separate session per user per chat (default)
//   - "global": single session per chat, shared among all users
func ResolveSessionKey(msg domain.InboundMessage, scope string) domain.SessionKey {
	key := domain.SessionKey{
		ChannelID: msg.ChannelID,
		AccountID: msg.AccountID,
		ChatID:    msg.ChatID,
	}

	switch scope {
	case "global":
		// No sender ID — all users in a chat share one session
	default:
		// per-sender (default)
		key.SenderID = msg.From
	}

	return key
}
