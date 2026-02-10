package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
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
	logsDir := filepath.Join(os.Getenv("HOME"), ".hunter3", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	logFile := filepath.Join(logsDir, "mcp-gdrive.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-gdrive] ", log.LstdFlags)
	logger.Println("MCP Google Drive server starting...")
}

func main() {
	initLogger()

	// Check for --auth flag for interactive OAuth flow
	for _, arg := range os.Args[1:] {
		if arg == "--auth" {
			runAuth()
			return
		}
	}

	server := &MCPServer{}
	logger.Println("Server initialized")
	server.Run()
}

func runAuth() {
	credentialsPath := os.Getenv("GDRIVE_CREDENTIALS_FILE")
	if credentialsPath == "" {
		credentialsPath = filepath.Join(os.Getenv("HOME"), ".hunter3", "gdrive-credentials.json")
	}

	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read credentials file at %s: %v\n", credentialsPath, err)
		fmt.Fprintf(os.Stderr, "See QUICKSTART.md Step 1-2 for setup instructions.\n")
		os.Exit(1)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveScope, drive.DriveFileScope, drive.DriveMetadataReadonlyScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse credentials: %v\n", err)
		os.Exit(1)
	}

	tokenPath := filepath.Join(os.Getenv("HOME"), ".hunter3", "gdrive-token.json")

	// Check if token already exists
	if _, err := tokenFromFile(tokenPath); err == nil {
		fmt.Println("Already authenticated. Token exists at", tokenPath)
		fmt.Println("To re-authenticate, delete the token first:")
		fmt.Println("  rm", tokenPath)
		return
	}

	token, err := getTokenFromWeb(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		os.Exit(1)
	}

	saveToken(tokenPath, token)
	fmt.Println("\nAuthentication successful! Token saved to", tokenPath)
	fmt.Println("You can now use mcp-gdrive as an MCP server.")
}

type MCPServer struct {
	driveService *drive.Service
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

	// Initialize Google Drive service
	if err := s.initDriveService(); err != nil {
		logger.Printf("Failed to initialize Drive service: %v\n", err)
		s.sendError(req.ID, -32603, "Internal error", fmt.Sprintf("Failed to initialize Drive service: %v", err))
		return
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "gdrive",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) initDriveService() error {
	ctx := context.Background()

	// Look for credentials file
	credentialsPath := os.Getenv("GDRIVE_CREDENTIALS_FILE")
	if credentialsPath == "" {
		credentialsPath = filepath.Join(os.Getenv("HOME"), ".hunter3", "gdrive-credentials.json")
	}

	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveScope, drive.DriveFileScope, drive.DriveMetadataReadonlyScope)
	if err != nil {
		return fmt.Errorf("unable to parse credentials: %w", err)
	}

	tokenPath := filepath.Join(os.Getenv("HOME"), ".hunter3", "gdrive-token.json")
	token, err := tokenFromFile(tokenPath)
	if err != nil {
		return fmt.Errorf("no auth token found at %s - run 'mcp-gdrive --auth' to authenticate first", tokenPath)
	}

	client := config.Client(ctx, token)
	s.driveService, err = drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to create Drive service: %w", err)
	}

	return nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	return tok, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) error {
	logger.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
	return nil
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        "list_files",
			Description: "List files and folders in Google Drive. Can filter by query (e.g., 'name contains \"report\"', 'mimeType = \"application/pdf\"').",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Search query using Google Drive query syntax (optional). Examples: 'name contains \"budget\"', 'mimeType = \"application/pdf\"', 'trashed = false'",
					},
					"max_results": {
						Type:        "string",
						Description: "Maximum number of files to return (default: 20, max: 100)",
						Default:     "20",
					},
					"folder_id": {
						Type:        "string",
						Description: "List files in a specific folder by folder ID (optional)",
					},
				},
				Required: []string{},
			},
		},
		{
			Name:        "get_file_info",
			Description: "Get detailed information about a specific file or folder by its ID.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_id": {
						Type:        "string",
						Description: "The ID of the file or folder",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "download_file",
			Description: "Download a file from Google Drive to local storage. Returns the content for text files or saves binary files to disk.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_id": {
						Type:        "string",
						Description: "The ID of the file to download",
					},
					"output_path": {
						Type:        "string",
						Description: "Local path to save the file (optional for text files)",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "upload_file",
			Description: "Upload a file to Google Drive from local storage.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_path": {
						Type:        "string",
						Description: "Local path to the file to upload",
					},
					"name": {
						Type:        "string",
						Description: "Name for the file in Google Drive (optional, defaults to filename)",
					},
					"folder_id": {
						Type:        "string",
						Description: "ID of the folder to upload to (optional, defaults to root)",
					},
					"description": {
						Type:        "string",
						Description: "Description for the file (optional)",
					},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "create_folder",
			Description: "Create a new folder in Google Drive.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {
						Type:        "string",
						Description: "Name of the folder to create",
					},
					"parent_id": {
						Type:        "string",
						Description: "ID of the parent folder (optional, defaults to root)",
					},
					"description": {
						Type:        "string",
						Description: "Description for the folder (optional)",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "delete_file",
			Description: "Delete a file or folder from Google Drive (moves to trash).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_id": {
						Type:        "string",
						Description: "The ID of the file or folder to delete",
					},
				},
				Required: []string{"file_id"},
			},
		},
		{
			Name:        "search_files",
			Description: "Search for files in Google Drive using advanced query syntax.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Search query. Examples: 'fullText contains \"meeting notes\"', 'modifiedTime > \"2024-01-01\"'",
					},
					"max_results": {
						Type:        "string",
						Description: "Maximum number of results (default: 20, max: 100)",
						Default:     "20",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "share_file",
			Description: "Share a file or folder with specific users or make it publicly accessible.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_id": {
						Type:        "string",
						Description: "The ID of the file or folder to share",
					},
					"email": {
						Type:        "string",
						Description: "Email address to share with (optional if making public)",
					},
					"role": {
						Type:        "string",
						Description: "Permission role: reader, writer, commenter, or owner",
						Enum:        []string{"reader", "writer", "commenter", "owner"},
						Default:     "reader",
					},
					"type": {
						Type:        "string",
						Description: "Permission type: user, group, domain, or anyone (for public)",
						Enum:        []string{"user", "group", "domain", "anyone"},
						Default:     "user",
					},
				},
				Required: []string{"file_id"},
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

	if s.driveService == nil {
		s.sendError(req.ID, -32603, "Internal error", "Drive service not initialized")
		return
	}

	switch params.Name {
	case "list_files":
		s.listFiles(req.ID, params.Arguments)
	case "get_file_info":
		s.getFileInfo(req.ID, params.Arguments)
	case "download_file":
		s.downloadFile(req.ID, params.Arguments)
	case "upload_file":
		s.uploadFile(req.ID, params.Arguments)
	case "create_folder":
		s.createFolder(req.ID, params.Arguments)
	case "delete_file":
		s.deleteFile(req.ID, params.Arguments)
	case "search_files":
		s.searchFiles(req.ID, params.Arguments)
	case "share_file":
		s.shareFile(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) listFiles(id interface{}, args map[string]interface{}) {
	query, _ := args["query"].(string)
	folderID, _ := args["folder_id"].(string)
	maxResults := int64(20)

	if maxStr, ok := args["max_results"].(string); ok && maxStr != "" {
		fmt.Sscanf(maxStr, "%d", &maxResults)
		if maxResults > 100 {
			maxResults = 100
		}
	}

	logger.Printf("Listing files with query: %s, folder: %s, max: %d\n", query, folderID, maxResults)

	call := s.driveService.Files.List().
		PageSize(maxResults).
		Fields("files(id, name, mimeType, size, createdTime, modifiedTime, owners, webViewLink)")

	// Build query
	var queryParts []string
	if query != "" {
		queryParts = append(queryParts, query)
	}
	if folderID != "" {
		queryParts = append(queryParts, fmt.Sprintf("'%s' in parents", folderID))
	}
	if len(queryParts) > 0 {
		call = call.Q(strings.Join(queryParts, " and "))
	}

	r, err := call.Do()
	if err != nil {
		logger.Printf("Failed to list files: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to list files: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	if len(r.Files) == 0 {
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: "No files found.",
				},
			},
		}
		s.sendResponse(id, result)
		return
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d file(s):\n\n", len(r.Files)))

	for i, file := range r.Files {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, file.Name))
		output.WriteString(fmt.Sprintf("   ID: %s\n", file.Id))
		output.WriteString(fmt.Sprintf("   Type: %s\n", file.MimeType))
		if file.Size > 0 {
			output.WriteString(fmt.Sprintf("   Size: %d bytes\n", file.Size))
		}
		if len(file.Owners) > 0 {
			output.WriteString(fmt.Sprintf("   Owner: %s\n", file.Owners[0].DisplayName))
		}
		output.WriteString(fmt.Sprintf("   Modified: %s\n", file.ModifiedTime))
		output.WriteString(fmt.Sprintf("   Link: %s\n\n", file.WebViewLink))
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: output.String(),
			},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) getFileInfo(id interface{}, args map[string]interface{}) {
	fileID, ok := args["file_id"].(string)
	if !ok || fileID == "" {
		s.sendError(id, -32602, "Invalid arguments", "file_id is required")
		return
	}

	logger.Printf("Getting file info for: %s\n", fileID)

	file, err := s.driveService.Files.Get(fileID).
		Fields("id, name, mimeType, size, createdTime, modifiedTime, description, owners, parents, webViewLink, webContentLink, permissions").
		Do()
	if err != nil {
		logger.Printf("Failed to get file info: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to get file info: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	var output strings.Builder
	output.WriteString("=== File Information ===\n\n")
	output.WriteString(fmt.Sprintf("Name: %s\n", file.Name))
	output.WriteString(fmt.Sprintf("ID: %s\n", file.Id))
	output.WriteString(fmt.Sprintf("Type: %s\n", file.MimeType))
	if file.Size > 0 {
		output.WriteString(fmt.Sprintf("Size: %d bytes\n", file.Size))
	}
	if file.Description != "" {
		output.WriteString(fmt.Sprintf("Description: %s\n", file.Description))
	}
	output.WriteString(fmt.Sprintf("Created: %s\n", file.CreatedTime))
	output.WriteString(fmt.Sprintf("Modified: %s\n", file.ModifiedTime))
	if len(file.Owners) > 0 {
		output.WriteString(fmt.Sprintf("Owner: %s (%s)\n", file.Owners[0].DisplayName, file.Owners[0].EmailAddress))
	}
	if len(file.Parents) > 0 {
		output.WriteString(fmt.Sprintf("Parent Folder ID: %s\n", file.Parents[0]))
	}
	output.WriteString(fmt.Sprintf("View Link: %s\n", file.WebViewLink))
	if file.WebContentLink != "" {
		output.WriteString(fmt.Sprintf("Download Link: %s\n", file.WebContentLink))
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: output.String(),
			},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) downloadFile(id interface{}, args map[string]interface{}) {
	fileID, ok := args["file_id"].(string)
	if !ok || fileID == "" {
		s.sendError(id, -32602, "Invalid arguments", "file_id is required")
		return
	}

	outputPath, _ := args["output_path"].(string)

	logger.Printf("Downloading file: %s to: %s\n", fileID, outputPath)

	// Get file metadata first
	file, err := s.driveService.Files.Get(fileID).Fields("name, mimeType, size").Do()
	if err != nil {
		logger.Printf("Failed to get file metadata: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to get file metadata: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Download file content
	resp, err := s.driveService.Files.Get(fileID).Download()
	if err != nil {
		logger.Printf("Failed to download file: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to download file: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Printf("Failed to read file content: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to read file content: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// If output path specified, save to disk
	if outputPath != "" {
		if err := os.WriteFile(outputPath, content, 0644); err != nil {
			logger.Printf("Failed to write file: %v\n", err)
			result := ToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Failed to write file: %v", err),
					},
				},
				IsError: true,
			}
			s.sendResponse(id, result)
			return
		}

		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("File '%s' downloaded successfully to %s (%d bytes)", file.Name, outputPath, len(content)),
				},
			},
		}
		s.sendResponse(id, result)
		return
	}

	// For text files, return content
	if strings.HasPrefix(file.MimeType, "text/") || 
	   strings.Contains(file.MimeType, "json") || 
	   strings.Contains(file.MimeType, "xml") {
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("=== File: %s ===\n\n%s", file.Name, string(content)),
				},
			},
		}
		s.sendResponse(id, result)
		return
	}

	// For binary files, suggest saving to disk
	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("File '%s' is a binary file (%s, %d bytes). Please specify an output_path to save it.", file.Name, file.MimeType, len(content)),
			},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) uploadFile(id interface{}, args map[string]interface{}) {
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		s.sendError(id, -32602, "Invalid arguments", "file_path is required")
		return
	}

	name, _ := args["name"].(string)
	if name == "" {
		name = filepath.Base(filePath)
	}

	folderID, _ := args["folder_id"].(string)
	description, _ := args["description"].(string)

	logger.Printf("Uploading file: %s as: %s to folder: %s\n", filePath, name, folderID)

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		logger.Printf("Failed to read file: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to read file: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Create file metadata
	file := &drive.File{
		Name:        name,
		Description: description,
	}

	if folderID != "" {
		file.Parents = []string{folderID}
	}

	// Upload file
	uploadedFile, err := s.driveService.Files.Create(file).Media(strings.NewReader(string(content))).Do()
	if err != nil {
		logger.Printf("Failed to upload file: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to upload file: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("File '%s' uploaded successfully!\nFile ID: %s\nSize: %d bytes", uploadedFile.Name, uploadedFile.Id, len(content)),
			},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) createFolder(id interface{}, args map[string]interface{}) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		s.sendError(id, -32602, "Invalid arguments", "name is required")
		return
	}

	parentID, _ := args["parent_id"].(string)
	description, _ := args["description"].(string)

	logger.Printf("Creating folder: %s in parent: %s\n", name, parentID)

	// Create folder metadata
	folder := &drive.File{
		Name:        name,
		MimeType:    "application/vnd.google-apps.folder",
		Description: description,
	}

	if parentID != "" {
		folder.Parents = []string{parentID}
	}

	// Create folder
	createdFolder, err := s.driveService.Files.Create(folder).Do()
	if err != nil {
		logger.Printf("Failed to create folder: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to create folder: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Folder '%s' created successfully!\nFolder ID: %s", createdFolder.Name, createdFolder.Id),
			},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) deleteFile(id interface{}, args map[string]interface{}) {
	fileID, ok := args["file_id"].(string)
	if !ok || fileID == "" {
		s.sendError(id, -32602, "Invalid arguments", "file_id is required")
		return
	}

	logger.Printf("Deleting file: %s\n", fileID)

	// Get file name first
	file, err := s.driveService.Files.Get(fileID).Fields("name").Do()
	if err != nil {
		logger.Printf("Failed to get file info: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to get file info: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	// Delete file (moves to trash)
	err = s.driveService.Files.Delete(fileID).Do()
	if err != nil {
		logger.Printf("Failed to delete file: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to delete file: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("File '%s' moved to trash successfully!", file.Name),
			},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) searchFiles(id interface{}, args map[string]interface{}) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		s.sendError(id, -32602, "Invalid arguments", "query is required")
		return
	}

	maxResults := int64(20)
	if maxStr, ok := args["max_results"].(string); ok && maxStr != "" {
		fmt.Sscanf(maxStr, "%d", &maxResults)
		if maxResults > 100 {
			maxResults = 100
		}
	}

	logger.Printf("Searching files with query: %s, max: %d\n", query, maxResults)

	// Use list_files implementation
	s.listFiles(id, args)
}

func (s *MCPServer) shareFile(id interface{}, args map[string]interface{}) {
	fileID, ok := args["file_id"].(string)
	if !ok || fileID == "" {
		s.sendError(id, -32602, "Invalid arguments", "file_id is required")
		return
	}

	email, _ := args["email"].(string)
	role, _ := args["role"].(string)
	if role == "" {
		role = "reader"
	}
	permType, _ := args["type"].(string)
	if permType == "" {
		permType = "user"
	}

	logger.Printf("Sharing file: %s with: %s, role: %s, type: %s\n", fileID, email, role, permType)

	// Create permission
	permission := &drive.Permission{
		Type: permType,
		Role: role,
	}

	if email != "" && permType != "anyone" {
		permission.EmailAddress = email
	}

	// Share file
	_, err := s.driveService.Permissions.Create(fileID, permission).Do()
	if err != nil {
		logger.Printf("Failed to share file: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to share file: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	var msg string
	if email != "" {
		msg = fmt.Sprintf("File shared successfully with %s as %s!", email, role)
	} else {
		msg = fmt.Sprintf("File shared publicly as %s!", role)
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: msg,
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
