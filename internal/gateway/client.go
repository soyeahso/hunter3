package gateway

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/soyeahso/hunter3/internal/logging"
)

// Client represents an authenticated WebSocket connection.
type Client struct {
	ConnID      string
	Info        ClientInfo
	Socket      *websocket.Conn
	AuthResult  AuthResult
	ConnectedAt time.Time

	mu     sync.Mutex
	closed bool
	log    *logging.Logger
}

// NewClient creates a Client for a newly authenticated WebSocket connection.
func NewClient(conn *websocket.Conn, info ClientInfo, authResult AuthResult, log *logging.Logger) *Client {
	return &Client{
		ConnID:      uuid.New().String(),
		Info:        info,
		Socket:      conn,
		AuthResult:  authResult,
		ConnectedAt: time.Now(),
		log:         log,
	}
}

// Send sends a frame to the client. Thread-safe.
func (c *Client) Send(frame Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClientClosed
	}

	return c.Socket.WriteJSON(frame)
}

// SendEvent sends a named event with payload.
func (c *Client) SendEvent(event string, payload any, seq int64) error {
	f, err := NewEvent(event, payload, seq)
	if err != nil {
		return err
	}
	return c.Send(f)
}

// Respond sends a success response for the given request ID.
func (c *Client) Respond(reqID string, payload any) error {
	f, err := NewResponse(reqID, payload)
	if err != nil {
		return err
	}
	return c.Send(f)
}

// RespondError sends an error response for the given request ID.
func (c *Client) RespondError(reqID string, errShape ErrorShape) error {
	return c.Send(NewErrorResponse(reqID, errShape))
}

// ReadFrame reads the next frame from the WebSocket.
func (c *Client) ReadFrame() (Frame, error) {
	_, msg, err := c.Socket.ReadMessage()
	if err != nil {
		return Frame{}, err
	}
	var f Frame
	if err := json.Unmarshal(msg, &f); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// Close closes the WebSocket connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.Socket.Close()
}

// ClientRegistry manages connected clients.
type ClientRegistry struct {
	mu      sync.RWMutex
	clients map[string]*Client // connID → Client
	log     *logging.Logger
}

// NewClientRegistry creates an empty client registry.
func NewClientRegistry(log *logging.Logger) *ClientRegistry {
	return &ClientRegistry{
		clients: make(map[string]*Client),
		log:     log,
	}
}

// Add registers a connected client.
func (r *ClientRegistry) Add(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c.ConnID] = c
	r.log.Info().Str("connId", c.ConnID).Str("client", c.Info.ID).Msg("client connected")
}

// Remove unregisters a client by connection ID.
func (r *ClientRegistry) Remove(connID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, connID)
	r.log.Info().Str("connId", connID).Msg("client disconnected")
}

// Get returns a client by connection ID.
func (r *ClientRegistry) Get(connID string) (*Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clients[connID]
	return c, ok
}

// Count returns the number of connected clients.
func (r *ClientRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// Broadcast sends an event frame to all connected clients.
func (r *ClientRegistry) Broadcast(event string, payload any, seq int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.clients {
		if err := c.SendEvent(event, payload, seq); err != nil {
			r.log.Warn().Err(err).Str("connId", c.ConnID).Msg("broadcast send failed")
		}
	}
}

// CloseAll closes all connected clients.
func (r *ClientRegistry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, c := range r.clients {
		c.Close()
		delete(r.clients, id)
	}
}
