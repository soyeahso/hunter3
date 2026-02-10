package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
	Text string `json:"text"`
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
var stdout *bufio.Writer

func initLogger() {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join(os.Getenv("HOME"), ".hunter3", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	// Open log file
	logFile := filepath.Join(logsDir, "mcp-make.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-make] ", log.LstdFlags)
	logger.Println("MCP Make server starting...")
}

func main() {
	initLogger()
	stdout = bufio.NewWriter(os.Stdout)

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
			Name:    "mcp-make",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        "build",
			Description: "Execute 'make all' to rebuild the project. Use this whenever you need to rebuild or recompile the project.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"rule": {
						Type:        "string",
						Description: "The make rule to execute (e.g. 'all', 'test', 'clean'). Defaults to 'all' if not specified.",
					},
				},
				Required: []string{},
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
	case "build":
		rule := "all"
		if r, ok := params.Arguments["rule"]; ok {
			if rs, ok := r.(string); ok && rs != "" {
				rule = rs
			}
		}
		s.executeMake(req.ID, rule)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

// projectRoot returns the project root directory. Checks HUNTER3_PROJECT_ROOT
// env first, then walks up from the executable's directory looking for a Makefile.
func projectRoot() (string, error) {
	if root := os.Getenv("HUNTER3_PROJECT_ROOT"); root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(abs, "Makefile")); err != nil {
			return "", fmt.Errorf("HUNTER3_PROJECT_ROOT %q does not contain a Makefile", abs)
		}
		return abs, nil
	}

	// Binary is typically in dist/, so project root is one level up
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(execPath)
	for dir != "/" && dir != "." {
		if _, err := os.Stat(filepath.Join(dir, "Makefile")); err == nil {
			return dir, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", fmt.Errorf("could not determine project root; set HUNTER3_PROJECT_ROOT")
}

func (s *MCPServer) executeMake(id interface{}, rule string) {
	logger.Printf("Executing make %s...\n", rule)

	// Flush all output to the client before running make, since make all
	// rebuilds this binary and triggers autorestart.
	stdout.Flush()

	ctx := context.Background()

	// Create the make command with the specified rule
	cmd := exec.CommandContext(ctx, "make", rule)

	// Set working directory to project root to avoid running a foreign Makefile.
	root, err := projectRoot()
	if err != nil {
		logger.Printf("Failed to determine project root: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Build failed: could not determine project root: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}
	cmd.Dir = root

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	outputStr := string(output)

	if err != nil {
		logger.Printf("make %s failed: %v\n", rule, err)
		// Include error information
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("make %s failed\n\nError: %v\n\nOutput:\n%s",
						rule, err, outputStr),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	logger.Printf("make %s completed successfully\n", rule)
	// Success
	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("make %s successful\n\nOutput:\n%s", rule, outputStr),
			},
		},
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

	fmt.Fprintln(stdout, string(data))
	stdout.Flush()
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

	fmt.Fprintln(stdout, string(jsonData))
	stdout.Flush()

}
