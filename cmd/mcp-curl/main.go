package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	Items       *Items   `json:"items,omitempty"`
}

type Items struct {
	Type string `json:"type"`
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
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
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
	logFile := filepath.Join(logsDir, "mcp-curl.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-curl] ", log.LstdFlags)
	logger.Println("MCP Curl server starting...")
}

func main() {
	initLogger()

	server := &MCPServer{}
	logger.Println("Server initialized")
	server.Run()
}

type MCPServer struct{}

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
			Name:    "mcp-curl",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        "curl",
			Description: "Execute curl commands with support for all standard curl options. Wraps the system curl command for maximum compatibility and feature support.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url": {
						Type:        "string",
						Description: "URL to fetch (required)",
					},
					"method": {
						Type:        "string",
						Description: "HTTP method to use (e.g., GET, POST, PUT, DELETE, PATCH, HEAD)",
						Default:     "GET",
					},
					"headers": {
						Type:        "array",
						Description: "Array of headers in 'Key: Value' format (e.g., ['Content-Type: application/json', 'Authorization: Bearer token'])",
						Items: &Items{
							Type: "string",
						},
					},
					"data": {
						Type:        "string",
						Description: "Data to send in the request body (for POST, PUT, PATCH)",
					},
					"output": {
						Type:        "string",
						Description: "Save response to file instead of returning it (path to output file)",
					},
					"user_agent": {
						Type:        "string",
						Description: "Custom User-Agent string",
					},
					"follow_redirects": {
						Type:        "boolean",
						Description: "Follow HTTP redirects (default: true)",
					},
					"insecure": {
						Type:        "boolean",
						Description: "Allow insecure SSL connections (default: false)",
					},
					"verbose": {
						Type:        "boolean",
						Description: "Enable verbose output for debugging (default: false)",
					},
					"timeout": {
						Type:        "number",
						Description: "Maximum time in seconds to wait for the request to complete (default: 30)",
					},
					"max_time": {
						Type:        "number",
						Description: "Maximum time in seconds for the whole operation (includes connection time)",
					},
					"proxy": {
						Type:        "string",
						Description: "Use proxy server (e.g., 'http://proxy.example.com:8080')",
					},
					"cookie": {
						Type:        "string",
						Description: "Send cookies from string/file",
					},
					"cookie_jar": {
						Type:        "string",
						Description: "Write cookies to file after operation",
					},
					"auth": {
						Type:        "string",
						Description: "Server authentication user:password",
					},
					"form_data": {
						Type:        "array",
						Description: "Send form data as multipart/form-data (e.g., ['field1=value1', 'file=@/path/to/file'])",
						Items: &Items{
							Type: "string",
						},
					},
					"include_headers": {
						Type:        "boolean",
						Description: "Include response headers in output (default: false)",
					},
					"show_error": {
						Type:        "boolean",
						Description: "Show error messages (default: true)",
					},
					"silent": {
						Type:        "boolean",
						Description: "Silent mode (default: false)",
					},
					"compressed": {
						Type:        "boolean",
						Description: "Request compressed response (default: false)",
					},
					"extra_flags": {
						Type:        "array",
						Description: "Additional curl flags not covered by other parameters (e.g., ['--http2', '--ipv4'])",
						Items: &Items{
							Type: "string",
						},
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
	case "curl":
		s.executeCurl(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) executeCurl(id interface{}, args map[string]interface{}) {
	// Extract URL (required)
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		logger.Println("Missing or invalid URL parameter")
		s.sendError(id, -32602, "Invalid arguments", "url parameter is required")
		return
	}

	logger.Printf("Executing curl for URL: %s\n", urlStr)

	// Build curl command
	curlArgs := []string{}

	// Method
	if method, ok := args["method"].(string); ok && method != "" {
		curlArgs = append(curlArgs, "-X", strings.ToUpper(method))
	}

	// Headers
	if headers, ok := args["headers"].([]interface{}); ok {
		for _, h := range headers {
			if headerStr, ok := h.(string); ok {
				curlArgs = append(curlArgs, "-H", headerStr)
			}
		}
	}

	// Data
	if data, ok := args["data"].(string); ok && data != "" {
		curlArgs = append(curlArgs, "-d", data)
	}

	// Form data
	if formData, ok := args["form_data"].([]interface{}); ok {
		for _, fd := range formData {
			if formStr, ok := fd.(string); ok {
				curlArgs = append(curlArgs, "-F", formStr)
			}
		}
	}

	// Output
	if output, ok := args["output"].(string); ok && output != "" {
		curlArgs = append(curlArgs, "-o", output)
	}

	// User-Agent
	if userAgent, ok := args["user_agent"].(string); ok && userAgent != "" {
		curlArgs = append(curlArgs, "-A", userAgent)
	}

	// Follow redirects (default true)
	followRedirects := true
	if fr, ok := args["follow_redirects"].(bool); ok {
		followRedirects = fr
	}
	if followRedirects {
		curlArgs = append(curlArgs, "-L")
	}

	// Insecure
	if insecure, ok := args["insecure"].(bool); ok && insecure {
		curlArgs = append(curlArgs, "-k")
	}

	// Verbose
	if verbose, ok := args["verbose"].(bool); ok && verbose {
		curlArgs = append(curlArgs, "-v")
	}

	// Timeout
	if timeout, ok := args["timeout"].(float64); ok {
		curlArgs = append(curlArgs, "--connect-timeout", fmt.Sprintf("%.0f", timeout))
	} else {
		// Default timeout of 30 seconds
		curlArgs = append(curlArgs, "--connect-timeout", "30")
	}

	// Max time
	if maxTime, ok := args["max_time"].(float64); ok {
		curlArgs = append(curlArgs, "--max-time", fmt.Sprintf("%.0f", maxTime))
	}

	// Proxy
	if proxy, ok := args["proxy"].(string); ok && proxy != "" {
		curlArgs = append(curlArgs, "-x", proxy)
	}

	// Cookie
	if cookie, ok := args["cookie"].(string); ok && cookie != "" {
		curlArgs = append(curlArgs, "-b", cookie)
	}

	// Cookie jar
	if cookieJar, ok := args["cookie_jar"].(string); ok && cookieJar != "" {
		curlArgs = append(curlArgs, "-c", cookieJar)
	}

	// Auth
	if auth, ok := args["auth"].(string); ok && auth != "" {
		curlArgs = append(curlArgs, "-u", auth)
	}

	// Include headers
	if includeHeaders, ok := args["include_headers"].(bool); ok && includeHeaders {
		curlArgs = append(curlArgs, "-i")
	}

	// Show error (default true)
	showError := true
	if se, ok := args["show_error"].(bool); ok {
		showError = se
	}
	if showError {
		curlArgs = append(curlArgs, "-S")
	}

	// Silent
	if silent, ok := args["silent"].(bool); ok && silent {
		curlArgs = append(curlArgs, "-s")
	}

	// Compressed
	if compressed, ok := args["compressed"].(bool); ok && compressed {
		curlArgs = append(curlArgs, "--compressed")
	}

	// Extra flags
	if extraFlags, ok := args["extra_flags"].([]interface{}); ok {
		for _, flag := range extraFlags {
			if flagStr, ok := flag.(string); ok {
				curlArgs = append(curlArgs, flagStr)
			}
		}
	}

	// Add URL as the last argument
	curlArgs = append(curlArgs, urlStr)

	// Execute curl command
	logger.Printf("Executing: curl %s\n", strings.Join(curlArgs, " "))
	
	cmd := exec.Command("curl", curlArgs...)
	output, err := cmd.CombinedOutput()
	
	outputStr := string(output)
	logger.Printf("Curl command completed, output length: %d bytes\n", len(output))

	if err != nil {
		logger.Printf("Curl command failed: %v\n", err)
		
		// Check if this is a non-zero exit code (which might be expected for HTTP errors)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Include the output even on error as it might contain useful info
			errorMsg := fmt.Sprintf("Curl exited with code %d\n\nOutput:\n%s", exitErr.ExitCode(), outputStr)
			
			result := ToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: errorMsg,
					},
				},
				IsError: true,
			}
			s.sendResponse(id, result)
			return
		}
		
		// Other execution errors
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to execute curl: %v\n\nOutput:\n%s", err, outputStr),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Success
	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: outputStr,
			},
		},
		IsError: false,
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
