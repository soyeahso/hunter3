package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

)

// MCP Protocol Types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default,omitempty"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools map[string]interface{} `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

var logger *log.Logger

func initLogger() {
	// Create logs directory if it doesn't exist
	logsDir := "/home/genoeg/.hunter3/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	// Open log file
	logFile := filepath.Join(logsDir, "mcp-fetch-website.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-fetch-website] ", log.LstdFlags)
	logger.Println("MCP Fetch Website server starting...")
}

func main() {
	initLogger()

	server := &MCPServer{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				if err := validateURLTarget(req.URL.Hostname()); err != nil {
					return fmt.Errorf("redirect blocked: %w", err)
				}
				return nil
			},
		},
	}
	logger.Println("Server initialized")
	server.Run()
}

const maxResponseSize = 10 * 1024 * 1024 // 10MB

// isPrivateIP checks if an IP address belongs to a private/reserved range.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, r := range privateRanges {
		_, cidr, err := net.ParseCIDR(r)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// validateURLTarget resolves a hostname and rejects private/internal IPs.
func validateURLTarget(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %q: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to private/internal IP address %s", ip)
		}
	}
	return nil
}

type MCPServer struct {
	httpClient *http.Client
}

func (s *MCPServer) Run() {
	scanner := bufio.NewScanner(os.Stdin)

	// Increase buffer size for large inputs
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	logger.Println("Listening for requests on stdin...")

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		logger.Printf("Received request: %s\n", line)
		s.handleRequest(line)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		logger.Printf("Error reading stdin: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
	}
	logger.Println("Server shutting down")
}

func (s *MCPServer) handleRequest(line string) {
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		logger.Printf("Parse error: %v\n", err)
		s.sendError(nil, -32700, "Parse error", err.Error())
		return
	}

	logger.Printf("Handling method: %s\n", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleListTools(req)
	case "tools/call":
		s.handleCallTool(req)
	case "notifications/initialized":
		// Ignore this notification
		logger.Println("Received initialized notification")
		return
	default:
		logger.Printf("Unknown method: %s\n", req.Method)
		s.sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", req.Method))
	}
}

func (s *MCPServer) handleInitialize(req JSONRPCRequest) {
	logger.Println("Handling initialize request")
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "fetch-website",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        "fetch",
			Description: "Fetches a URL from the internet and returns the response. Can fetch HTML pages, JSON APIs, images, and other web resources.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        "string",
						Description: "URL to fetch (must start with http:// or https://)",
					},
					"method": {
						Type:        "string",
						Description: "HTTP method to use (GET, POST, PUT, DELETE, etc.)",
						Enum:        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
						Default:     "GET",
					},
					"headers": {
						Type:        "string",
						Description: "Optional HTTP headers as JSON object string (e.g., '{\"Authorization\": \"Bearer token\"}')",
					},
					"body": {
						Type:        "string",
						Description: "Optional request body for POST/PUT/PATCH requests",
					},
				},
				Required: []string{"url"},
			},
		},
	}

	result := ListToolsResult{
		Tools: tools,
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleCallTool(req JSONRPCRequest) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		logger.Printf("Invalid params: %v\n", err)
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	logger.Printf("Calling tool: %s\n", params.Name)

	switch params.Name {
	case "fetch":
		s.fetchURL(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) fetchURL(id interface{}, args map[string]interface{}) {
	// Extract URL
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		logger.Println("Missing or invalid URL parameter")
		s.sendError(id, -32602, "Invalid arguments", "url parameter is required")
		return
	}

	logger.Printf("Fetching URL: %s\n", urlStr)

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		logger.Printf("Failed to parse URL: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Invalid URL: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		logger.Printf("Invalid URL scheme: %s\n", parsedURL.Scheme)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: "URL must start with http:// or https://",
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// SSRF protection: block requests to private/internal IPs
	if err := validateURLTarget(parsedURL.Hostname()); err != nil {
		logger.Printf("SSRF check failed: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Blocked: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Extract method (default to GET)
	method := "GET"
	if m, ok := args["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}
	logger.Printf("Using HTTP method: %s\n", method)

	// Create request
	ctx := context.Background()
	var bodyReader io.Reader
	if body, ok := args["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to create request: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Add headers
	req.Header.Set("User-Agent", "Hunter3-MCP-Fetch/1.0")

	if headersStr, ok := args["headers"].(string); ok && headersStr != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersStr), &headers); err != nil {
			result := ToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Invalid headers JSON: %v", err),
					},
				},
				IsError: true,
			}
			s.sendResponse(id, result)
			return
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	// Perform request
	logger.Printf("Performing HTTP request to %s\n", urlStr)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Printf("HTTP request failed: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to fetch URL: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}
	defer resp.Body.Close()

	logger.Printf("Received response: %d %s\n", resp.StatusCode, resp.Status)

	// Read response body
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		logger.Printf("Failed to read response body: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to read response body: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Build response headers
	var headerLines []string
	for key, values := range resp.Header {
		for _, value := range values {
			headerLines = append(headerLines, fmt.Sprintf("%s: %s", key, value))
		}
	}

	contentType := resp.Header.Get("Content-Type")

	// Check if the response is an image
	if strings.HasPrefix(contentType, "image/") {
		logger.Printf("Detected image response: %s, size: %d bytes\n", contentType, len(body))

		// Extract just the mime type (strip charset or other params)
		mimeType := strings.SplitN(contentType, ";", 2)[0]
		base64Data := base64.StdEncoding.EncodeToString(body)

		metaText := fmt.Sprintf("HTTP %s\nStatus: %d %s\nContent-Type: %s\nSize: %d bytes\n\nImage fetched successfully. The image content is provided below for analysis.",
			resp.Proto,
			resp.StatusCode,
			resp.Status,
			mimeType,
			len(body),
		)

		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: metaText,
				},
				{
					Type:     "image",
					Data:     base64Data,
					MimeType: mimeType,
				},
			},
			IsError: resp.StatusCode >= 400,
		}
		s.sendResponse(id, result)
		return
	}

	// Format text response
	responseText := fmt.Sprintf("HTTP %s\nStatus: %d %s\n\nHeaders:\n%s\n\nBody:\n%s",
		resp.Proto,
		resp.StatusCode,
		resp.Status,
		strings.Join(headerLines, "\n"),
		string(body),
	)

	// Success
	logger.Printf("Fetch completed successfully, body size: %d bytes\n", len(body))
	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: responseText,
			},
		},
		IsError: resp.StatusCode >= 400,
	}

	s.sendResponse(id, result)
}

func (s *MCPServer) sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		logger.Printf("Error marshaling response: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error marshaling response: %v\n", err)
		return
	}

	fmt.Println(string(data))
	logger.Printf("Sent response for request ID: %v\n", id)
}

func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) {
	logger.Printf("Sending error response: code=%d, message=%s\n", code, message)
	
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		logger.Printf("Error marshaling error response: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error marshaling error response: %v\n", err)
		return
	}

	fmt.Println(string(jsonData))
}
