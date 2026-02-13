package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
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

const (
	ToolSSHConnect      = "ssh_connect"
	ToolSSHExecute      = "ssh_execute"
	ToolSSHUpload       = "ssh_upload"
	ToolSSHDownload     = "ssh_download"
	ToolSSHListSessions = "ssh_list_sessions"
	ToolSSHDisconnect   = "ssh_disconnect"
)

var logger *log.Logger

func initLogger() {
	logsDir := filepath.Join(os.Getenv("HOME"), ".hunter3", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	logFile := filepath.Join(logsDir, "mcp-ssh.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-ssh] ", log.LstdFlags)
	logger.Println("MCP SSH server starting...")
}

func main() {
	initLogger()

	manager := NewSSHManager()
	server := &MCPServer{manager: manager}
	logger.Println("Server initialized")
	server.Run()
}

type MCPServer struct {
	manager *SSHManager
}

func (s *MCPServer) Run() {
	scanner := bufio.NewScanner(os.Stdin)
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
			Name:    "mcp-ssh",
			Version: "1.0.0",
		},
	}
	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        ToolSSHConnect,
			Description: "Connect to a remote server via SSH. Supports password and key-based authentication.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"host":           {Type: "string", Description: "Hostname or IP address of the remote server"},
					"port":           {Type: "number", Description: "SSH port (default: 22)", Default: "22"},
					"username":       {Type: "string", Description: "SSH username"},
					"password":       {Type: "string", Description: "SSH password (optional if using key)"},
					"key_path":       {Type: "string", Description: "Path to private key file (optional if using password)"},
					"key_passphrase": {Type: "string", Description: "Passphrase for encrypted private key (optional)"},
					"session_name":   {Type: "string", Description: "Name for this SSH session (optional, auto-generated if not provided)"},
				},
				Required: []string{"host", "username"},
			},
		},
		{
			Name:        ToolSSHExecute,
			Description: "Execute a command on a remote server via SSH.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_name": {Type: "string", Description: "Name of the SSH session to use"},
					"command":      {Type: "string", Description: "Command to execute on the remote server"},
					"working_dir":  {Type: "string", Description: "Working directory for command execution (optional)"},
					"timeout":      {Type: "number", Description: "Command timeout in seconds (default: 300)", Default: "300"},
				},
				Required: []string{"session_name", "command"},
			},
		},
		{
			Name:        ToolSSHUpload,
			Description: "Upload a file to a remote server via SFTP.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_name": {Type: "string", Description: "Name of the SSH session to use"},
					"local_path":   {Type: "string", Description: "Local file path to upload"},
					"remote_path":  {Type: "string", Description: "Remote destination path"},
					"permissions":  {Type: "string", Description: "File permissions in octal format (e.g., '0644', optional)"},
				},
				Required: []string{"session_name", "local_path", "remote_path"},
			},
		},
		{
			Name:        ToolSSHDownload,
			Description: "Download a file from a remote server via SFTP.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_name": {Type: "string", Description: "Name of the SSH session to use"},
					"remote_path":  {Type: "string", Description: "Remote file path to download"},
					"local_path":   {Type: "string", Description: "Local destination path"},
				},
				Required: []string{"session_name", "remote_path", "local_path"},
			},
		},
		{
			Name:        ToolSSHListSessions,
			Description: "List all active SSH sessions.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
				Required:   []string{},
			},
		},
		{
			Name:        ToolSSHDisconnect,
			Description: "Disconnect an SSH session.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_name": {Type: "string", Description: "Name of the SSH session to disconnect"},
				},
				Required: []string{"session_name"},
			},
		},
	}

	s.sendResponse(req.ID, ListToolsResult{Tools: tools})
}

func (s *MCPServer) handleCallTool(req JSONRPCRequest) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		logger.Printf("Invalid params: %v\n", err)
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	logger.Printf("Calling tool: %s\n", params.Name)

	var result ToolResult
	switch params.Name {
	case ToolSSHConnect:
		result = s.manager.handleConnect(params.Arguments)
	case ToolSSHExecute:
		result = s.manager.handleExecute(params.Arguments)
	case ToolSSHUpload:
		result = s.manager.handleUpload(params.Arguments)
	case ToolSSHDownload:
		result = s.manager.handleDownload(params.Arguments)
	case ToolSSHListSessions:
		result = s.manager.handleListSessions(params.Arguments)
	case ToolSSHDisconnect:
		result = s.manager.handleDisconnect(params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
		return
	}

	s.sendResponse(req.ID, result)
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
