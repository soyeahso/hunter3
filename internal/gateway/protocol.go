package gateway

import "encoding/json"

// Frame types for the WebSocket protocol.
const (
	FrameTypeRequest  = "req"
	FrameTypeResponse = "res"
	FrameTypeEvent    = "event"
)

// Frame is the base envelope for all WebSocket messages.
// The Type field discriminates between request, response, and event frames.
type Frame struct {
	Type string `json:"type"`

	// Request fields
	ID     string          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`

	// Response fields
	OK      *bool           `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`

	// Event fields
	Event string          `json:"event,omitempty"`
	Seq   int64           `json:"seq,omitempty"`

	// Error (response only)
	Error *ErrorShape `json:"error,omitempty"`
}

// ErrorShape is the standard error format in response frames.
type ErrorShape struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    any    `json:"details,omitempty"`
	Retryable  bool   `json:"retryable,omitempty"`
	RetryAfter int    `json:"retryAfterMs,omitempty"`
}

// ConnectParams are sent by the client in the initial "connect" request.
type ConnectParams struct {
	MinProtocol int          `json:"minProtocol"`
	MaxProtocol int          `json:"maxProtocol"`
	Client      ClientInfo   `json:"client"`
	Auth        *ConnectAuth `json:"auth,omitempty"`
	Caps        []string     `json:"caps,omitempty"`
	Locale      string       `json:"locale,omitempty"`
	UserAgent   string       `json:"userAgent,omitempty"`
}

// ClientInfo identifies the connecting client.
type ClientInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	Mode        string `json:"mode"` // "app" | "node"
	InstanceID  string `json:"instanceId,omitempty"`
}

// ConnectAuth carries credentials in the connect request.
type ConnectAuth struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// HelloOK is the server's response payload after successful authentication.
type HelloOK struct {
	Protocol int          `json:"protocol"`
	Server   ServerInfo   `json:"server"`
	Features Features     `json:"features"`
	Policy   ServerPolicy `json:"policy"`
}

// ServerInfo identifies the gateway server.
type ServerInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Host    string `json:"host,omitempty"`
	ConnID  string `json:"connId"`
}

// Features advertises available RPC methods and events.
type Features struct {
	Methods []string `json:"methods"`
	Events  []string `json:"events"`
}

// ServerPolicy communicates protocol limits to the client.
type ServerPolicy struct {
	MaxPayload      int `json:"maxPayload"`
	MaxBufferedBytes int `json:"maxBufferedBytes"`
	TickIntervalMs  int `json:"tickIntervalMs"`
}

// NewRequest creates a request frame.
func NewRequest(id, method string, params any) (Frame, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return Frame{}, err
	}
	return Frame{
		Type:   FrameTypeRequest,
		ID:     id,
		Method: method,
		Params: raw,
	}, nil
}

// NewResponse creates a success response frame.
func NewResponse(id string, payload any) (Frame, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Frame{}, err
	}
	ok := true
	return Frame{
		Type:    FrameTypeResponse,
		ID:      id,
		OK:      &ok,
		Payload: raw,
	}, nil
}

// NewErrorResponse creates an error response frame.
func NewErrorResponse(id string, errShape ErrorShape) Frame {
	ok := false
	return Frame{
		Type:  FrameTypeResponse,
		ID:    id,
		OK:    &ok,
		Error: &errShape,
	}
}

// NewEvent creates an event frame.
func NewEvent(event string, payload any, seq int64) (Frame, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Frame{}, err
	}
	return Frame{
		Type:    FrameTypeEvent,
		Event:   event,
		Payload: raw,
		Seq:     seq,
	}, nil
}

// Protocol version supported by this server.
const ProtocolVersion = 1
