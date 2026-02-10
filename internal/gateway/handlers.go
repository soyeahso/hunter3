package gateway

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is returned by health endpoints. The public HTTP endpoint
// only populates Status; the authenticated RPC handler populates all fields.
type HealthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version,omitempty"`
	Clients  int    `json:"clients,omitempty"`
}

// handleHealth returns the server health status. Only status is exposed
// publicly; detailed info is available via the authenticated RPC health method.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

// handleNotFound returns a 404 for unknown routes.
func handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not found",
		"path":  r.URL.Path,
	})
}

// RequestHandler processes an incoming RPC request frame from a client.
type RequestHandler func(ctx *RequestContext)

// RequestContext carries everything a handler needs.
type RequestContext struct {
	Client  *Client
	Frame   Frame
	Server  *Server
}

// Respond sends a success response.
func (rc *RequestContext) Respond(payload any) {
	if err := rc.Client.Respond(rc.Frame.ID, payload); err != nil {
		rc.Server.log.Warn().Err(err).Str("method", rc.Frame.Method).Msg("failed to send response")
	}
}

// RespondError sends an error response.
func (rc *RequestContext) RespondError(code, message string) {
	rc.Client.RespondError(rc.Frame.ID, ErrorShape{
		Code:    code,
		Message: message,
	})
}

// Params unmarshals the request params into the given target.
func (rc *RequestContext) Params(target any) error {
	if rc.Frame.Params == nil {
		return nil
	}
	return json.Unmarshal(rc.Frame.Params, target)
}
