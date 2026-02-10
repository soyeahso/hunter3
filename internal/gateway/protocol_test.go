package gateway

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameTypeConstants(t *testing.T) {
	assert.Equal(t, "req", FrameTypeRequest)
	assert.Equal(t, "res", FrameTypeResponse)
	assert.Equal(t, "event", FrameTypeEvent)
}

func TestProtocolVersion(t *testing.T) {
	assert.Equal(t, 1, ProtocolVersion)
}

// --- NewRequest tests ---

func TestNewRequest(t *testing.T) {
	frame, err := NewRequest("req-1", "health", nil)
	require.NoError(t, err)

	assert.Equal(t, FrameTypeRequest, frame.Type)
	assert.Equal(t, "req-1", frame.ID)
	assert.Equal(t, "health", frame.Method)
}

func TestNewRequest_WithParams(t *testing.T) {
	params := map[string]string{"key": "gateway.port"}
	frame, err := NewRequest("req-2", "config.get", params)
	require.NoError(t, err)

	assert.Equal(t, FrameTypeRequest, frame.Type)
	assert.Equal(t, "req-2", frame.ID)
	assert.Equal(t, "config.get", frame.Method)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(frame.Params, &decoded))
	assert.Equal(t, "gateway.port", decoded["key"])
}

func TestNewRequest_MarshalRoundTrip(t *testing.T) {
	frame, err := NewRequest("req-3", "chat.send", map[string]string{"message": "hello"})
	require.NoError(t, err)

	data, err := json.Marshal(frame)
	require.NoError(t, err)

	var decoded Frame
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, FrameTypeRequest, decoded.Type)
	assert.Equal(t, "req-3", decoded.ID)
	assert.Equal(t, "chat.send", decoded.Method)
}

// --- NewResponse tests ---

func TestNewResponse(t *testing.T) {
	frame, err := NewResponse("req-1", map[string]string{"status": "ok"})
	require.NoError(t, err)

	assert.Equal(t, FrameTypeResponse, frame.Type)
	assert.Equal(t, "req-1", frame.ID)
	require.NotNil(t, frame.OK)
	assert.True(t, *frame.OK)
	assert.Nil(t, frame.Error)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(frame.Payload, &payload))
	assert.Equal(t, "ok", payload["status"])
}

func TestNewResponse_NilPayload(t *testing.T) {
	frame, err := NewResponse("req-1", nil)
	require.NoError(t, err)
	assert.Equal(t, FrameTypeResponse, frame.Type)
	require.NotNil(t, frame.OK)
	assert.True(t, *frame.OK)
}

// --- NewErrorResponse tests ---

func TestNewErrorResponse(t *testing.T) {
	frame := NewErrorResponse("req-1", ErrorShape{
		Code:    "unauthorized",
		Message: "invalid token",
	})

	assert.Equal(t, FrameTypeResponse, frame.Type)
	assert.Equal(t, "req-1", frame.ID)
	require.NotNil(t, frame.OK)
	assert.False(t, *frame.OK)
	require.NotNil(t, frame.Error)
	assert.Equal(t, "unauthorized", frame.Error.Code)
	assert.Equal(t, "invalid token", frame.Error.Message)
}

func TestNewErrorResponse_WithRetry(t *testing.T) {
	frame := NewErrorResponse("req-2", ErrorShape{
		Code:       "rate_limited",
		Message:    "too many requests",
		Retryable:  true,
		RetryAfter: 5000,
	})

	require.NotNil(t, frame.Error)
	assert.True(t, frame.Error.Retryable)
	assert.Equal(t, 5000, frame.Error.RetryAfter)
}

func TestNewErrorResponse_MarshalRoundTrip(t *testing.T) {
	frame := NewErrorResponse("req-1", ErrorShape{
		Code:    "not_found",
		Message: "key not found",
		Details: map[string]string{"key": "gateway.foo"},
	})

	data, err := json.Marshal(frame)
	require.NoError(t, err)

	var decoded Frame
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NotNil(t, decoded.OK)
	assert.False(t, *decoded.OK)
	require.NotNil(t, decoded.Error)
	assert.Equal(t, "not_found", decoded.Error.Code)
}

// --- NewEvent tests ---

func TestNewEvent(t *testing.T) {
	frame, err := NewEvent("chat.delta", map[string]string{"content": "hello"}, 42)
	require.NoError(t, err)

	assert.Equal(t, FrameTypeEvent, frame.Type)
	assert.Equal(t, "chat.delta", frame.Event)
	assert.Equal(t, int64(42), frame.Seq)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(frame.Payload, &payload))
	assert.Equal(t, "hello", payload["content"])
}

func TestNewEvent_ZeroSeq(t *testing.T) {
	frame, err := NewEvent("connect.challenge", map[string]string{"nonce": "abc"}, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), frame.Seq)
}

func TestNewEvent_NilPayload(t *testing.T) {
	frame, err := NewEvent("shutdown", nil, 1)
	require.NoError(t, err)
	assert.Equal(t, FrameTypeEvent, frame.Type)
}

// --- ConnectParams tests ---

func TestConnectParams_Marshal(t *testing.T) {
	params := ConnectParams{
		MinProtocol: 1,
		MaxProtocol: 1,
		Client: ClientInfo{
			ID:          "my-client",
			DisplayName: "Test Client",
			Version:     "1.0.0",
			Platform:    "linux",
			Mode:        "app",
			InstanceID:  "inst-1",
		},
		Auth:      &ConnectAuth{Token: "secret"},
		Caps:      []string{"streaming"},
		Locale:    "en-US",
		UserAgent: "hunter3-test/1.0",
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var decoded ConnectParams
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, 1, decoded.MinProtocol)
	assert.Equal(t, "my-client", decoded.Client.ID)
	assert.Equal(t, "Test Client", decoded.Client.DisplayName)
	require.NotNil(t, decoded.Auth)
	assert.Equal(t, "secret", decoded.Auth.Token)
	assert.Equal(t, []string{"streaming"}, decoded.Caps)
}

func TestConnectParams_OmitsNilAuth(t *testing.T) {
	params := ConnectParams{
		MinProtocol: 1,
		MaxProtocol: 1,
		Client: ClientInfo{
			ID:       "client",
			Version:  "1.0.0",
			Platform: "linux",
			Mode:     "app",
		},
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"auth"`)
}

// --- HelloOK tests ---

func TestHelloOK_Marshal(t *testing.T) {
	hello := HelloOK{
		Protocol: 1,
		Server: ServerInfo{
			Version: "1.0.0",
			Commit:  "abc1234",
			Host:    "localhost",
			ConnID:  "conn-1",
		},
		Features: Features{
			Methods: []string{"health", "config.get", "chat.send"},
			Events:  []string{"connect.challenge", "chat.delta"},
		},
		Policy: ServerPolicy{
			MaxPayload:       4194304,
			MaxBufferedBytes: 16777216,
			TickIntervalMs:   30000,
		},
	}

	data, err := json.Marshal(hello)
	require.NoError(t, err)

	var decoded HelloOK
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, 1, decoded.Protocol)
	assert.Equal(t, "conn-1", decoded.Server.ConnID)
	assert.Len(t, decoded.Features.Methods, 3)
	assert.Equal(t, 4194304, decoded.Policy.MaxPayload)
}

// --- ErrorShape tests ---

func TestErrorShape_OmitsEmpty(t *testing.T) {
	err := ErrorShape{
		Code:    "bad_request",
		Message: "missing params",
	}

	data, e := json.Marshal(err)
	require.NoError(t, e)
	assert.NotContains(t, string(data), "details")
	assert.NotContains(t, string(data), "retryable")
	assert.NotContains(t, string(data), "retryAfterMs")
}
