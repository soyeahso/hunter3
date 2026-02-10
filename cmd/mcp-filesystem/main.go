package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	Type       string                 `json:"type"`
	Properties map[string]Property    `json:"properties"`
	Required   []string               `json:"required,omitempty"`
	AdditionalProperties interface{} `json:"additionalProperties,omitempty"`
}

type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Items       *Items      `json:"items,omitempty"`
	MinItems    *int        `json:"minItems,omitempty"`
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

type DirectoryEntry struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Children    []DirectoryEntry  `json:"children,omitempty"`
}

var logger *log.Logger
var allowedDirectories []string

func initLogger() {
	logsDir := filepath.Join(os.Getenv("HOME"), ".hunter3", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	logFile := filepath.Join(logsDir, "mcp-filesystem.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-filesystem] ", log.LstdFlags)
	logger.Println("MCP Filesystem server starting...")
}

func main() {
	initLogger()

	// Parse allowed directories from command-line arguments
	if len(os.Args) < 2 {
		logger.Fatal("Usage: mcp-filesystem <allowed-directory> [additional-directories...]")
	}

	for _, dir := range os.Args[1:] {
		// Expand home directory
		if strings.HasPrefix(dir, "~/") {
			dir = filepath.Join(os.Getenv("HOME"), dir[2:])
		}

		// Get absolute path
		absDir, err := filepath.Abs(dir)
		if err != nil {
			logger.Printf("Warning: Could not resolve absolute path for %s: %v\n", dir, err)
			continue
		}

		// Resolve symlinks
		resolvedDir, err := filepath.EvalSymlinks(absDir)
		if err != nil {
			// If it doesn't exist yet, use the absolute path
			resolvedDir = absDir
		}

		// Check if it's accessible
		info, err := os.Stat(resolvedDir)
		if err != nil {
			logger.Printf("Warning: Cannot access directory %s, skipping: %v\n", resolvedDir, err)
			continue
		}

		if !info.IsDir() {
			logger.Printf("Warning: %s is not a directory, skipping\n", resolvedDir)
			continue
		}

		// Normalize path
		normalizedDir := filepath.Clean(resolvedDir)
		allowedDirectories = append(allowedDirectories, normalizedDir)
		logger.Printf("Allowed directory: %s\n", normalizedDir)
	}

	if len(allowedDirectories) == 0 {
		logger.Fatal("Error: None of the specified directories are accessible")
	}

	server := &MCPServer{}
	logger.Println("Server initialized")
	server.Run()
}

type MCPServer struct{}

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
			Name:    "filesystem",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	
	minOne := 1
	
	tools := []Tool{
		{
			Name:        "read_file",
			Description: "Read the complete contents of a file as text. DEPRECATED: Use read_text_file instead.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string"},
					"head": {Type: "number", Description: "If provided, returns only the first N lines of the file"},
					"tail": {Type: "number", Description: "If provided, returns only the last N lines of the file"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "read_text_file",
			Description: "Read the complete contents of a file from the file system as text. Handles various text encodings and provides detailed error messages if the file cannot be read. Use this tool when you need to examine the contents of a single file. Use the 'head' parameter to read only the first N lines of a file, or the 'tail' parameter to read only the last N lines of a file. Operates on the file as text regardless of extension. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string"},
					"head": {Type: "number", Description: "If provided, returns only the first N lines of the file"},
					"tail": {Type: "number", Description: "If provided, returns only the last N lines of the file"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "read_media_file",
			Description: "Read an image or audio file. Returns the base64 encoded data and MIME type. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "read_multiple_files",
			Description: "Read the contents of multiple files simultaneously. This is more efficient than reading files one by one when you need to analyze or compare multiple files. Each file's content is returned with its path as a reference. Failed reads for individual files won't stop the entire operation. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"paths": {
						Type:        "array",
						Description: "Array of file paths to read. Each path must be a string pointing to a valid file within allowed directories.",
						Items:       &Items{Type: "string"},
						MinItems:    &minOne,
					},
				},
				Required: []string{"paths"},
			},
		},
		{
			Name:        "write_file",
			Description: "Create a new file or completely overwrite an existing file with new content. Use with caution as it will overwrite existing files without warning. Handles text content with proper encoding. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":    {Type: "string"},
					"content": {Type: "string"},
				},
				Required: []string{"path", "content"},
			},
		},
		{
			Name:        "edit_file",
			Description: "Make line-based edits to a text file. Each edit replaces exact line sequences with new content. Returns a git-style diff showing the changes made. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string"},
					"edits": {
						Type: "array",
						Items: &Items{Type: "object"},
					},
					"dryRun": {Type: "boolean", Default: false, Description: "Preview changes using git-style diff format"},
				},
				Required: []string{"path", "edits"},
			},
		},
		{
			Name:        "create_directory",
			Description: "Create a new directory or ensure a directory exists. Can create multiple nested directories in one operation. If the directory already exists, this operation will succeed silently. Perfect for setting up directory structures for projects or ensuring required paths exist. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "list_directory",
			Description: "Get a detailed listing of all files and directories in a specified path. Results clearly distinguish between files and directories with [FILE] and [DIR] prefixes. This tool is essential for understanding directory structure and finding specific files within a directory. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "list_directory_with_sizes",
			Description: "Get a detailed listing of all files and directories in a specified path, including sizes. Results clearly distinguish between files and directories with [FILE] and [DIR] prefixes. This tool is useful for understanding directory structure and finding specific files within a directory. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":   {Type: "string"},
					"sortBy": {Type: "string", Enum: []string{"name", "size"}, Default: "name", Description: "Sort entries by name or size"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "directory_tree",
			Description: "Get a recursive tree view of files and directories as a JSON structure. Each entry includes 'name', 'type' (file/directory), and 'children' for directories. Files have no children array, while directories always have a children array (which may be empty). The output is formatted with 2-space indentation for readability. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":            {Type: "string"},
					"excludePatterns": {Type: "array", Items: &Items{Type: "string"}, Default: []string{}},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "move_file",
			Description: "Move or rename files and directories. Can move files between directories and rename them in a single operation. If the destination exists, the operation will fail. Works across different directories and can be used for simple renaming within the same directory. Both source and destination must be within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"source":      {Type: "string"},
					"destination": {Type: "string"},
				},
				Required: []string{"source", "destination"},
			},
		},
		{
			Name:        "search_files",
			Description: "Recursively search for files and directories matching a pattern. The patterns should be glob-style patterns that match paths relative to the working directory. Use pattern like '*.ext' to match files in current directory, and '**/*.ext' to match files in all subdirectories. Returns full paths to all matching items. Great for finding files when you don't know their exact location. Only searches within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":            {Type: "string"},
					"pattern":         {Type: "string"},
					"excludePatterns": {Type: "array", Items: &Items{Type: "string"}, Default: []string{}},
				},
				Required: []string{"path", "pattern"},
			},
		},
		{
			Name:        "get_file_info",
			Description: "Retrieve detailed metadata about a file or directory. Returns comprehensive information including size, creation time, last modified time, permissions, and type. This tool is perfect for understanding file characteristics without reading the actual content. Only works within allowed directories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "list_allowed_directories",
			Description: "Returns the list of directories that this server is allowed to access. Subdirectories within these allowed directories are also accessible. Use this to understand which directories and their nested paths are available before trying to access files.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
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
	case "read_file", "read_text_file":
		s.readTextFile(req.ID, params.Arguments)
	case "read_media_file":
		s.readMediaFile(req.ID, params.Arguments)
	case "read_multiple_files":
		s.readMultipleFiles(req.ID, params.Arguments)
	case "write_file":
		s.writeFile(req.ID, params.Arguments)
	case "edit_file":
		s.editFile(req.ID, params.Arguments)
	case "create_directory":
		s.createDirectory(req.ID, params.Arguments)
	case "list_directory":
		s.listDirectory(req.ID, params.Arguments)
	case "list_directory_with_sizes":
		s.listDirectoryWithSizes(req.ID, params.Arguments)
	case "directory_tree":
		s.directoryTree(req.ID, params.Arguments)
	case "move_file":
		s.moveFile(req.ID, params.Arguments)
	case "search_files":
		s.searchFiles(req.ID, params.Arguments)
	case "get_file_info":
		s.getFileInfo(req.ID, params.Arguments)
	case "list_allowed_directories":
		s.listAllowedDirectories(req.ID)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

// resolvePartialSymlinks finds the longest existing prefix of a path,
// resolves symlinks on it, then appends the remaining components.
// This prevents symlink-based escapes even for non-existent target paths.
func resolvePartialSymlinks(absPath string) (string, error) {
	parts := strings.Split(absPath, string(filepath.Separator))

	// Start from root, build up path until we find the non-existent part
	existing := string(filepath.Separator)
	var remaining []string
	foundBreak := false

	for _, part := range parts {
		if part == "" {
			continue
		}
		if foundBreak {
			remaining = append(remaining, part)
			continue
		}

		candidate := filepath.Join(existing, part)
		if _, err := os.Lstat(candidate); err != nil {
			foundBreak = true
			remaining = append(remaining, part)
		} else {
			existing = candidate
		}
	}

	// Resolve symlinks on the existing prefix
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}

	// Append remaining (non-existent) components
	if len(remaining) > 0 {
		resolved = filepath.Join(append([]string{resolved}, remaining...)...)
	}

	return resolved, nil
}

// validatePath ensures a path is within allowed directories
func validatePath(path string) (string, error) {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(os.Getenv("HOME"), path[2:])
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve symlinks — for non-existent paths, resolve the longest
	// existing prefix to prevent symlink-based directory escapes.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		resolvedPath, err = resolvePartialSymlinks(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}
	}

	// Normalize
	normalizedPath := filepath.Clean(resolvedPath)

	// Check if path is within allowed directories
	allowed := false
	for _, allowedDir := range allowedDirectories {
		if normalizedPath == allowedDir || strings.HasPrefix(normalizedPath, allowedDir+string(filepath.Separator)) {
			allowed = true
			break
		}
	}

	if !allowed {
		return "", fmt.Errorf("access denied: path is outside allowed directories")
	}

	return normalizedPath, nil
}

func (s *MCPServer) readTextFile(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	content, err := os.ReadFile(validPath)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to read file: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	text := string(content)

	// Handle head/tail parameters
	if head, ok := args["head"].(float64); ok {
		lines := strings.Split(text, "\n")
		if int(head) < len(lines) {
			lines = lines[:int(head)]
		}
		text = strings.Join(lines, "\n")
	} else if tail, ok := args["tail"].(float64); ok {
		lines := strings.Split(text, "\n")
		if int(tail) < len(lines) {
			lines = lines[len(lines)-int(tail):]
		}
		text = strings.Join(lines, "\n")
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: text}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) readMediaFile(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	content, err := os.ReadFile(validPath)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to read file: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Determine MIME type from extension
	ext := strings.ToLower(filepath.Ext(validPath))
	mimeTypes := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
		".svg":  "image/svg+xml",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".flac": "audio/flac",
	}

	mimeType := mimeTypes[ext]
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	contentType := "image"
	if strings.HasPrefix(mimeType, "audio/") {
		contentType = "audio"
	} else if !strings.HasPrefix(mimeType, "image/") {
		contentType = "blob"
	}

	base64Data := base64.StdEncoding.EncodeToString(content)

	result := ToolResult{
		Content: []ContentItem{{
			Type:     contentType,
			Data:     base64Data,
			MimeType: mimeType,
		}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) readMultipleFiles(id interface{}, args map[string]interface{}) {
	pathsInterface, ok := args["paths"].([]interface{})
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "paths parameter is required and must be an array")
		return
	}

	var results []string
	for _, pathInterface := range pathsInterface {
		pathStr, ok := pathInterface.(string)
		if !ok {
			results = append(results, "Error: invalid path in array")
			continue
		}

		validPath, err := validatePath(pathStr)
		if err != nil {
			s.sendError(id, -32602, "Access denied", fmt.Sprintf("%s: %v", pathStr, err))
			return
		}

		content, err := os.ReadFile(validPath)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: Error - %v", pathStr, err))
			continue
		}

		results = append(results, fmt.Sprintf("%s:\n%s\n", pathStr, string(content)))
	}

	text := strings.Join(results, "\n---\n")
	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: text}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) writeFile(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	content, ok := args["content"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "content parameter is required")
		return
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(validPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to create parent directory: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	if err := os.WriteFile(validPath, []byte(content), 0644); err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to write file: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Successfully wrote to %s", pathStr)}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) editFile(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	editsInterface, ok := args["edits"].([]interface{})
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "edits parameter is required and must be an array")
		return
	}

	dryRun := false
	if dr, ok := args["dryRun"].(bool); ok {
		dryRun = dr
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	content, err := os.ReadFile(validPath)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to read file: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	originalContent := string(content)
	modifiedContent := originalContent

	// Apply edits
	for _, editInterface := range editsInterface {
		edit, ok := editInterface.(map[string]interface{})
		if !ok {
			continue
		}

		oldText, ok1 := edit["oldText"].(string)
		newText, ok2 := edit["newText"].(string)

		if !ok1 || !ok2 {
			continue
		}

		modifiedContent = strings.ReplaceAll(modifiedContent, oldText, newText)
	}

	// Generate diff
	diff := generateDiff(originalContent, modifiedContent, pathStr)

	if !dryRun {
		if err := os.WriteFile(validPath, []byte(modifiedContent), 0644); err != nil {
			result := ToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to write file: %v", err)}},
				IsError: true,
			}
			s.sendResponse(id, result)
			return
		}
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: diff}},
	}
	s.sendResponse(id, result)
}

func generateDiff(original, modified, filename string) string {
	origLines := strings.Split(original, "\n")
	modLines := strings.Split(modified, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s\n", filename))
	diff.WriteString(fmt.Sprintf("+++ %s\n", filename))

	// Simple line-by-line diff
	maxLen := len(origLines)
	if len(modLines) > maxLen {
		maxLen = len(modLines)
	}

	for i := 0; i < maxLen; i++ {
		var origLine, modLine string
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(modLines) {
			modLine = modLines[i]
		}

		if origLine != modLine {
			if origLine != "" {
				diff.WriteString(fmt.Sprintf("-%s\n", origLine))
			}
			if modLine != "" {
				diff.WriteString(fmt.Sprintf("+%s\n", modLine))
			}
		}
	}

	return diff.String()
}

func (s *MCPServer) createDirectory(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	if err := os.MkdirAll(validPath, 0755); err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to create directory: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Successfully created directory %s", pathStr)}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) listDirectory(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	entries, err := os.ReadDir(validPath)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to read directory: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	var lines []string
	for _, entry := range entries {
		prefix := "[FILE]"
		if entry.IsDir() {
			prefix = "[DIR]"
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, entry.Name()))
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: strings.Join(lines, "\n")}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) listDirectoryWithSizes(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	sortBy := "name"
	if sb, ok := args["sortBy"].(string); ok {
		sortBy = sb
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	entries, err := os.ReadDir(validPath)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to read directory: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	type entryInfo struct {
		name  string
		isDir bool
		size  int64
	}

	var infos []entryInfo
	var totalSize int64
	var totalFiles, totalDirs int

	for _, entry := range entries {
		info := entryInfo{
			name:  entry.Name(),
			isDir: entry.IsDir(),
		}

		if !entry.IsDir() {
			fileInfo, err := entry.Info()
			if err == nil {
				info.size = fileInfo.Size()
				totalSize += info.size
			}
			totalFiles++
		} else {
			totalDirs++
		}

		infos = append(infos, info)
	}

	// Sort
	if sortBy == "size" {
		sort.Slice(infos, func(i, j int) bool {
			return infos[i].size > infos[j].size
		})
	} else {
		sort.Slice(infos, func(i, j int) bool {
			return infos[i].name < infos[j].name
		})
	}

	var lines []string
	for _, info := range infos {
		prefix := "[FILE]"
		sizeStr := ""
		if info.isDir {
			prefix = "[DIR]"
		} else {
			sizeStr = fmt.Sprintf("%10s", formatSize(info.size))
		}
		lines = append(lines, fmt.Sprintf("%s %-30s %s", prefix, info.name, sizeStr))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Total: %d files, %d directories", totalFiles, totalDirs))
	lines = append(lines, fmt.Sprintf("Combined size: %s", formatSize(totalSize)))

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: strings.Join(lines, "\n")}},
	}
	s.sendResponse(id, result)
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (s *MCPServer) directoryTree(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	excludePatterns := []string{}
	if ep, ok := args["excludePatterns"].([]interface{}); ok {
		for _, p := range ep {
			if pattern, ok := p.(string); ok {
				excludePatterns = append(excludePatterns, pattern)
			}
		}
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	tree, err := buildDirectoryTree(validPath, validPath, excludePatterns)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to build directory tree: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	jsonData, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to marshal tree: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: string(jsonData)}},
	}
	s.sendResponse(id, result)
}

func buildDirectoryTree(rootPath, currentPath string, excludePatterns []string) ([]DirectoryEntry, error) {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	var result []DirectoryEntry

	for _, entry := range entries {
		entryPath := filepath.Join(currentPath, entry.Name())
		relPath, _ := filepath.Rel(rootPath, entryPath)

		// Check exclusions
		excluded := false
		for _, pattern := range excludePatterns {
			matched, _ := filepath.Match(pattern, entry.Name())
			if matched {
				excluded = true
				break
			}
			// Also check if the relative path matches
			matched, _ = filepath.Match(pattern, relPath)
			if matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		dirEntry := DirectoryEntry{
			Name: entry.Name(),
		}

		if entry.IsDir() {
			dirEntry.Type = "directory"
			children, err := buildDirectoryTree(rootPath, entryPath, excludePatterns)
			if err == nil {
				dirEntry.Children = children
			} else {
				dirEntry.Children = []DirectoryEntry{}
			}
		} else {
			dirEntry.Type = "file"
		}

		result = append(result, dirEntry)
	}

	return result, nil
}

func (s *MCPServer) moveFile(id interface{}, args map[string]interface{}) {
	sourceStr, ok := args["source"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "source parameter is required")
		return
	}

	destStr, ok := args["destination"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "destination parameter is required")
		return
	}

	validSource, err := validatePath(sourceStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", fmt.Sprintf("source: %v", err))
		return
	}

	validDest, err := validatePath(destStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", fmt.Sprintf("destination: %v", err))
		return
	}

	if err := os.Rename(validSource, validDest); err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to move file: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Successfully moved %s to %s", sourceStr, destStr)}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) searchFiles(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	pattern, ok := args["pattern"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "pattern parameter is required")
		return
	}

	excludePatterns := []string{}
	if ep, ok := args["excludePatterns"].([]interface{}); ok {
		for _, p := range ep {
			if pat, ok := p.(string); ok {
				excludePatterns = append(excludePatterns, pat)
			}
		}
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	var matches []string
	err = filepath.WalkDir(validPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		relPath, _ := filepath.Rel(validPath, path)

		// Check exclusions
		for _, excl := range excludePatterns {
			matched, _ := filepath.Match(excl, relPath)
			if matched {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check pattern match
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Search failed: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	text := "No matches found"
	if len(matches) > 0 {
		text = strings.Join(matches, "\n")
	}

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: text}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) getFileInfo(id interface{}, args map[string]interface{}) {
	pathStr, ok := args["path"].(string)
	if !ok {
		s.sendError(id, -32602, "Invalid arguments", "path parameter is required")
		return
	}

	validPath, err := validatePath(pathStr)
	if err != nil {
		s.sendError(id, -32602, "Access denied", err.Error())
		return
	}

	info, err := os.Stat(validPath)
	if err != nil {
		result := ToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to get file info: %v", err)}},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("name: %s", info.Name()))
	lines = append(lines, fmt.Sprintf("size: %s", formatSize(info.Size())))
	lines = append(lines, fmt.Sprintf("modified: %s", info.ModTime().Format(time.RFC3339)))
	lines = append(lines, fmt.Sprintf("mode: %s", info.Mode().String()))
	lines = append(lines, fmt.Sprintf("isDirectory: %t", info.IsDir()))

	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: strings.Join(lines, "\n")}},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) listAllowedDirectories(id interface{}) {
	text := "Allowed directories:\n" + strings.Join(allowedDirectories, "\n")
	result := ToolResult{
		Content: []ContentItem{{Type: "text", Text: text}},
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
