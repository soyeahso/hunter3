package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
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
	logsDir := "/home/genoeg/.hunter3/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	logFile := filepath.Join(logsDir, "mcp-gmail.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-gmail] ", log.LstdFlags)
	logger.Println("MCP Gmail server starting...")
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
	credentialsPath := os.Getenv("GMAIL_CREDENTIALS_FILE")
	if credentialsPath == "" {
		credentialsPath = filepath.Join(os.Getenv("HOME"), ".hunter3", "gmail-credentials.json")
	}

	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read credentials file at %s: %v\n", credentialsPath, err)
		fmt.Fprintf(os.Stderr, "See QUICKSTART.md Step 1-2 for setup instructions.\n")
		os.Exit(1)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope, gmail.GmailSendScope, gmail.GmailComposeScope, gmail.GmailModifyScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse credentials: %v\n", err)
		os.Exit(1)
	}

	tokenPath := filepath.Join(os.Getenv("HOME"), ".hunter3", "gmail-token.json")

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
	fmt.Println("You can now use mcp-gmail as an MCP server.")
}

type MCPServer struct {
	gmailService *gmail.Service
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
	
	// Initialize Gmail service
	if err := s.initGmailService(); err != nil {
		logger.Printf("Failed to initialize Gmail service: %v\n", err)
		s.sendError(req.ID, -32603, "Internal error", fmt.Sprintf("Failed to initialize Gmail service: %v", err))
		return
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "gmail",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) initGmailService() error {
	ctx := context.Background()
	
	// Look for credentials file
	credentialsPath := os.Getenv("GMAIL_CREDENTIALS_FILE")
	if credentialsPath == "" {
		credentialsPath = filepath.Join(os.Getenv("HOME"), ".hunter3", "gmail-credentials.json")
	}

	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope, gmail.GmailSendScope, gmail.GmailComposeScope, gmail.GmailModifyScope)
	if err != nil {
		return fmt.Errorf("unable to parse credentials: %w", err)
	}

	tokenPath := filepath.Join(os.Getenv("HOME"), ".hunter3", "gmail-token.json")
	token, err := tokenFromFile(tokenPath)
	if err != nil {
		return fmt.Errorf("no auth token found at %s - run 'mcp-gmail --auth' to authenticate first", tokenPath)
	}

	client := config.Client(ctx, token)
	s.gmailService, err = gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to create Gmail service: %w", err)
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
			Name:        "list_messages",
			Description: "List email messages from Gmail inbox. Can filter by query string (e.g., 'is:unread', 'from:user@example.com', 'subject:important').",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Gmail search query (optional). Examples: 'is:unread', 'from:user@example.com', 'subject:meeting', 'after:2024/01/01'",
					},
					"max_results": {
						Type:        "string",
						Description: "Maximum number of messages to return (default: 10, max: 100)",
						Default:     "10",
					},
				},
				Required: []string{},
			},
		},
		{
			Name:        "read_message",
			Description: "Read the full content of a specific email message by its ID.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"message_id": {
						Type:        "string",
						Description: "The ID of the message to read",
					},
				},
				Required: []string{"message_id"},
			},
		},
		{
			Name:        "send_message",
			Description: "Send an email message. Supports plain text and HTML content, multiple recipients, CC, BCC, and file attachments.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"to": {
						Type:        "string",
						Description: "Recipient email address(es), comma-separated for multiple recipients",
					},
					"subject": {
						Type:        "string",
						Description: "Email subject line",
					},
					"body": {
						Type:        "string",
						Description: "Email body content (plain text or HTML)",
					},
					"cc": {
						Type:        "string",
						Description: "CC recipients, comma-separated (optional)",
					},
					"bcc": {
						Type:        "string",
						Description: "BCC recipients, comma-separated (optional)",
					},
					"is_html": {
						Type:        "string",
						Description: "Whether the body is HTML (true/false, default: false)",
						Default:     "false",
					},
					"attachment_paths": {
						Type:        "string",
						Description: "Comma-separated list of file paths to attach (optional)",
					},
				},
				Required: []string{"to", "subject", "body"},
			},
		},
		{
			Name:        "search_messages",
			Description: "Search for email messages using Gmail's advanced search syntax. Returns message summaries.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Gmail search query. Examples: 'has:attachment larger:10M', 'is:starred', 'category:primary'",
					},
					"max_results": {
						Type:        "string",
						Description: "Maximum number of results (default: 10, max: 100)",
						Default:     "10",
					},
				},
				Required: []string{"query"},
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

	if s.gmailService == nil {
		s.sendError(req.ID, -32603, "Internal error", "Gmail service not initialized")
		return
	}

	switch params.Name {
	case "list_messages":
		s.listMessages(req.ID, params.Arguments)
	case "read_message":
		s.readMessage(req.ID, params.Arguments)
	case "send_message":
		s.sendMessage(req.ID, params.Arguments)
	case "search_messages":
		s.searchMessages(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) listMessages(id interface{}, args map[string]interface{}) {
	query, _ := args["query"].(string)
	maxResults := int64(10)
	
	if maxStr, ok := args["max_results"].(string); ok && maxStr != "" {
		if parsed, err := fmt.Sscanf(maxStr, "%d", &maxResults); err == nil && parsed > 0 {
			if maxResults > 100 {
				maxResults = 100
			}
		}
	}

	logger.Printf("Listing messages with query: %s, max: %d\n", query, maxResults)

	call := s.gmailService.Users.Messages.List("me").MaxResults(maxResults)
	if query != "" {
		call = call.Q(query)
	}

	r, err := call.Do()
	if err != nil {
		logger.Printf("Failed to list messages: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to list messages: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	if len(r.Messages) == 0 {
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: "No messages found.",
				},
			},
		}
		s.sendResponse(id, result)
		return
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d message(s):\n\n", len(r.Messages)))

	for i, msg := range r.Messages {
		// Get message details
		msgDetail, err := s.gmailService.Users.Messages.Get("me", msg.Id).Format("metadata").MetadataHeaders("From", "Subject", "Date").Do()
		if err != nil {
			logger.Printf("Failed to get message details for %s: %v\n", msg.Id, err)
			continue
		}

		from := ""
		subject := ""
		date := ""
		for _, header := range msgDetail.Payload.Headers {
			switch header.Name {
			case "From":
				from = header.Value
			case "Subject":
				subject = header.Value
			case "Date":
				date = header.Value
			}
		}

		output.WriteString(fmt.Sprintf("%d. ID: %s\n", i+1, msg.Id))
		output.WriteString(fmt.Sprintf("   From: %s\n", from))
		output.WriteString(fmt.Sprintf("   Subject: %s\n", subject))
		output.WriteString(fmt.Sprintf("   Date: %s\n", date))
		output.WriteString(fmt.Sprintf("   Snippet: %s\n\n", msgDetail.Snippet))
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

func (s *MCPServer) readMessage(id interface{}, args map[string]interface{}) {
	messageID, ok := args["message_id"].(string)
	if !ok || messageID == "" {
		s.sendError(id, -32602, "Invalid arguments", "message_id is required")
		return
	}

	logger.Printf("Reading message: %s\n", messageID)

	msg, err := s.gmailService.Users.Messages.Get("me", messageID).Format("full").Do()
	if err != nil {
		logger.Printf("Failed to read message: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to read message: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	var output strings.Builder
	output.WriteString("=== Email Message ===\n\n")

	// Extract headers
	for _, header := range msg.Payload.Headers {
		if header.Name == "From" || header.Name == "To" || header.Name == "Cc" || 
		   header.Name == "Subject" || header.Name == "Date" {
			output.WriteString(fmt.Sprintf("%s: %s\n", header.Name, header.Value))
		}
	}
	output.WriteString("\n")

	// Extract body
	body := extractBody(msg.Payload)
	if body != "" {
		output.WriteString("=== Body ===\n")
		output.WriteString(body)
		output.WriteString("\n")
	}

	// List attachments
	attachments := extractAttachments(msg.Payload)
	if len(attachments) > 0 {
		output.WriteString("\n=== Attachments ===\n")
		for _, att := range attachments {
			output.WriteString(fmt.Sprintf("- %s (%d bytes)\n", att.Filename, att.Body.Size))
		}
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

func extractBody(payload *gmail.MessagePart) string {
	if payload.MimeType == "text/plain" || payload.MimeType == "text/html" {
		if payload.Body.Data != "" {
			data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
			if err == nil {
				return string(data)
			}
		}
	}

	// Recursively search for body in parts
	for _, part := range payload.Parts {
		if part.MimeType == "text/plain" || part.MimeType == "text/html" {
			if part.Body.Data != "" {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err == nil {
					return string(data)
				}
			}
		}
		// Check nested parts
		if body := extractBody(part); body != "" {
			return body
		}
	}

	return ""
}

func extractAttachments(payload *gmail.MessagePart) []*gmail.MessagePart {
	var attachments []*gmail.MessagePart
	
	if payload.Filename != "" && payload.Body.AttachmentId != "" {
		attachments = append(attachments, payload)
	}

	for _, part := range payload.Parts {
		attachments = append(attachments, extractAttachments(part)...)
	}

	return attachments
}

func (s *MCPServer) sendMessage(id interface{}, args map[string]interface{}) {
	to, ok := args["to"].(string)
	if !ok || to == "" {
		s.sendError(id, -32602, "Invalid arguments", "to is required")
		return
	}

	subject, ok := args["subject"].(string)
	if !ok || subject == "" {
		s.sendError(id, -32602, "Invalid arguments", "subject is required")
		return
	}

	body, ok := args["body"].(string)
	if !ok || body == "" {
		s.sendError(id, -32602, "Invalid arguments", "body is required")
		return
	}

	cc, _ := args["cc"].(string)
	bcc, _ := args["bcc"].(string)
	isHTML := false
	if isHTMLStr, ok := args["is_html"].(string); ok && (isHTMLStr == "true" || isHTMLStr == "True") {
		isHTML = true
	}
	attachmentPaths, _ := args["attachment_paths"].(string)

	logger.Printf("Sending message to: %s, subject: %s\n", to, subject)

	// Build email message
	var message strings.Builder
	message.WriteString(fmt.Sprintf("To: %s\r\n", to))
	if cc != "" {
		message.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}
	if bcc != "" {
		message.WriteString(fmt.Sprintf("Bcc: %s\r\n", bcc))
	}
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

	// Handle attachments
	var attachments []string
	if attachmentPaths != "" {
		attachments = strings.Split(attachmentPaths, ",")
		for i, path := range attachments {
			attachments[i] = strings.TrimSpace(path)
		}
	}

	if len(attachments) > 0 {
		// Multipart message with attachments
		boundary := fmt.Sprintf("boundary_%d", time.Now().Unix())
		message.WriteString(fmt.Sprintf("MIME-Version: 1.0\r\n"))
		message.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
		message.WriteString("\r\n")

		// Body part
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		if isHTML {
			message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		} else {
			message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		}
		message.WriteString("\r\n")
		message.WriteString(body)
		message.WriteString("\r\n\r\n")

		// Attachment parts
		for _, attachmentPath := range attachments {
			data, err := os.ReadFile(attachmentPath)
			if err != nil {
				logger.Printf("Failed to read attachment %s: %v\n", attachmentPath, err)
				result := ToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Failed to read attachment %s: %v", attachmentPath, err),
						},
					},
					IsError: true,
				}
				s.sendResponse(id, result)
				return
			}

			filename := filepath.Base(attachmentPath)
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			message.WriteString(fmt.Sprintf("Content-Type: %s\r\n", mimeType))
			message.WriteString("Content-Transfer-Encoding: base64\r\n")
			message.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", filename))
			message.WriteString("\r\n")
			message.WriteString(base64.StdEncoding.EncodeToString(data))
			message.WriteString("\r\n\r\n")
		}

		message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		// Simple message without attachments
		if isHTML {
			message.WriteString("MIME-Version: 1.0\r\n")
			message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		} else {
			message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		}
		message.WriteString("\r\n")
		message.WriteString(body)
	}

	// Encode message
	encodedMessage := base64.URLEncoding.EncodeToString([]byte(message.String()))

	gmailMessage := &gmail.Message{
		Raw: encodedMessage,
	}

	_, err := s.gmailService.Users.Messages.Send("me", gmailMessage).Do()
	if err != nil {
		logger.Printf("Failed to send message: %v\n", err)
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to send message: %v", err),
				},
			},
			IsError: true,
		}
		s.sendResponse(id, result)
		return
	}

	logger.Println("Message sent successfully")
	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Email sent successfully to %s", to),
			},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) searchMessages(id interface{}, args map[string]interface{}) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		s.sendError(id, -32602, "Invalid arguments", "query is required")
		return
	}

	maxResults := int64(10)
	if maxStr, ok := args["max_results"].(string); ok && maxStr != "" {
		if parsed, err := fmt.Sscanf(maxStr, "%d", &maxResults); err == nil && parsed > 0 {
			if maxResults > 100 {
				maxResults = 100
			}
		}
	}

	logger.Printf("Searching messages with query: %s, max: %d\n", query, maxResults)

	// This is essentially the same as list_messages but the name makes it clearer
	s.listMessages(id, args)
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
