package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
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

// iCloud Mail Configuration
type ImailConfig struct {
	Email    string
	Password string
	IMAPHost string
	IMAPPort string
	SMTPHost string
	SMTPPort string
}

var logger *log.Logger

func initLogger() {
	logsDir := "/home/genoeg/.hunter3/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	logFile := filepath.Join(logsDir, "mcp-imail.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-imail] ", log.LstdFlags)
	logger.Println("MCP iCloud Mail server starting...")
}

func main() {
	initLogger()

	server := &MCPServer{}
	logger.Println("Server initialized")
	server.Run()
}

type MCPServer struct {
	config ImailConfig
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

	// Load configuration
	if err := s.loadConfig(); err != nil {
		logger.Printf("Failed to load configuration: %v\n", err)
		s.sendError(req.ID, -32603, "Internal error", fmt.Sprintf("Failed to load configuration: %v", err))
		return
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "imail",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) loadConfig() error {
	// Try to load from environment variables first
	email := os.Getenv("ICLOUD_EMAIL")
	password := os.Getenv("ICLOUD_PASSWORD")

	// If not in env, try to load from config file
	if email == "" || password == "" {
		configPath := filepath.Join(os.Getenv("HOME"), ".hunter3", "icloud-mail.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("no configuration found. Please set ICLOUD_EMAIL and ICLOUD_PASSWORD environment variables or create %s with {\"email\": \"your@icloud.com\", \"password\": \"app-specific-password\"}", configPath)
		}

		var config struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}

		email = config.Email
		password = config.Password
	}

	if email == "" || password == "" {
		return fmt.Errorf("email and password are required")
	}

	s.config = ImailConfig{
		Email:    email,
		Password: password,
		IMAPHost: "imap.mail.me.com",
		IMAPPort: "993",
		SMTPHost: "smtp.mail.me.com",
		SMTPPort: "587",
	}

	logger.Printf("Configuration loaded for email: %s\n", s.config.Email)
	return nil
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        "list_messages",
			Description: "List email messages from iCloud Mail inbox. Can specify mailbox and limit.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"mailbox": {
						Type:        "string",
						Description: "Mailbox name (default: INBOX). Common mailboxes: INBOX, Sent, Drafts, Trash",
						Default:     "INBOX",
					},
					"limit": {
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
			Description: "Read the full content of a specific email message by its sequence number.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"mailbox": {
						Type:        "string",
						Description: "Mailbox name (default: INBOX)",
						Default:     "INBOX",
					},
					"seq_num": {
						Type:        "string",
						Description: "Sequence number of the message to read",
					},
				},
				Required: []string{"seq_num"},
			},
		},
		{
			Name:        "send_message",
			Description: "Send an email message via iCloud Mail. Supports plain text and HTML content, multiple recipients, CC, BCC, and file attachments.",
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
			Description: "Search for email messages in iCloud Mail using IMAP search criteria.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"mailbox": {
						Type:        "string",
						Description: "Mailbox to search in (default: INBOX)",
						Default:     "INBOX",
					},
					"query": {
						Type:        "string",
						Description: "Search query. Examples: 'FROM user@example.com', 'SUBJECT meeting', 'UNSEEN' (for unread)",
					},
					"limit": {
						Type:        "string",
						Description: "Maximum number of results (default: 10, max: 100)",
						Default:     "10",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "list_mailboxes",
			Description: "List all available mailboxes in the iCloud Mail account.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
				Required:   []string{},
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
	case "list_messages":
		s.listMessages(req.ID, params.Arguments)
	case "read_message":
		s.readMessage(req.ID, params.Arguments)
	case "send_message":
		s.sendMessage(req.ID, params.Arguments)
	case "search_messages":
		s.searchMessages(req.ID, params.Arguments)
	case "list_mailboxes":
		s.listMailboxes(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) connectIMAP() (*client.Client, error) {
	addr := fmt.Sprintf("%s:%s", s.config.IMAPHost, s.config.IMAPPort)
	logger.Printf("Connecting to IMAP server: %s\n", addr)

	c, err := client.DialTLS(addr, &tls.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	if err := c.Login(s.config.Email, s.config.Password); err != nil {
		c.Logout()
		return nil, fmt.Errorf("login failed: %w", err)
	}

	logger.Println("IMAP connection established")
	return c, nil
}

func (s *MCPServer) listMessages(id interface{}, args map[string]interface{}) {
	mailboxName, _ := args["mailbox"].(string)
	if mailboxName == "" {
		mailboxName = "INBOX"
	}

	limit := 10
	if limitStr, ok := args["limit"].(string); ok && limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	logger.Printf("Listing messages from mailbox: %s, limit: %d\n", mailboxName, limit)

	c, err := s.connectIMAP()
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer c.Logout()

	mbox, err := c.Select(mailboxName, true)
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to select mailbox: %v", err))
		return
	}

	if mbox.Messages == 0 {
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: "No messages found in mailbox.",
				},
			},
		}
		s.sendResponse(id, result)
		return
	}

	// Get the last N messages
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > uint32(limit) {
		from = mbox.Messages - uint32(limit) + 1
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, section.FetchItem()}

	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	var output strings.Builder
	count := 0
	var msgList []*imap.Message

	for msg := range messages {
		msgList = append(msgList, msg)
	}

	// Reverse to show newest first
	for i := len(msgList) - 1; i >= 0; i-- {
		msg := msgList[i]
		count++
		
		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].Address()
		}

		flags := ""
		for _, f := range msg.Flags {
			if f == imap.SeenFlag {
				flags = "[READ] "
				break
			}
		}

		output.WriteString(fmt.Sprintf("%d. %sSeq: %d, UID: %d\n", count, flags, msg.SeqNum, msg.Uid))
		output.WriteString(fmt.Sprintf("   From: %s\n", from))
		output.WriteString(fmt.Sprintf("   Subject: %s\n", msg.Envelope.Subject))
		output.WriteString(fmt.Sprintf("   Date: %s\n\n", msg.Envelope.Date))
	}

	if err := <-done; err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to fetch messages: %v", err))
		return
	}

	if count == 0 {
		output.WriteString("No messages found.")
	} else {
		output.WriteString(fmt.Sprintf("Total: %d message(s)", count))
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
	mailboxName, _ := args["mailbox"].(string)
	if mailboxName == "" {
		mailboxName = "INBOX"
	}

	seqNumStr, ok := args["seq_num"].(string)
	if !ok || seqNumStr == "" {
		s.sendError(id, -32602, "Invalid arguments", "seq_num is required")
		return
	}

	seqNum, err := strconv.ParseUint(seqNumStr, 10, 32)
	if err != nil {
		s.sendError(id, -32602, "Invalid arguments", "seq_num must be a valid number")
		return
	}

	logger.Printf("Reading message seq %d from mailbox: %s\n", seqNum, mailboxName)

	c, err := s.connectIMAP()
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer c.Logout()

	_, err = c.Select(mailboxName, true)
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to select mailbox: %v", err))
		return
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uint32(seqNum))

	messages := make(chan *imap.Message, 1)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, section.FetchItem()}

	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	msg := <-messages
	if msg == nil {
		s.sendErrorResult(id, "Message not found")
		return
	}

	if err := <-done; err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to fetch message: %v", err))
		return
	}

	var output strings.Builder
	output.WriteString("=== Email Message ===\n\n")

	from := ""
	if len(msg.Envelope.From) > 0 {
		from = msg.Envelope.From[0].Address()
	}

	to := ""
	if len(msg.Envelope.To) > 0 {
		for i, addr := range msg.Envelope.To {
			if i > 0 {
				to += ", "
			}
			to += addr.Address()
		}
	}

	output.WriteString(fmt.Sprintf("From: %s\n", from))
	output.WriteString(fmt.Sprintf("To: %s\n", to))
	output.WriteString(fmt.Sprintf("Subject: %s\n", msg.Envelope.Subject))
	output.WriteString(fmt.Sprintf("Date: %s\n\n", msg.Envelope.Date))

	// Parse message body
	for _, value := range msg.Body {
		if value == nil {
			continue
		}
		
		mr, err := mail.ReadMessage(value)
		if err != nil {
			logger.Printf("Error reading message body: %v\n", err)
			continue
		}

		body, err := s.extractMessageBody(mr)
		if err != nil {
			logger.Printf("Error extracting body: %v\n", err)
			output.WriteString(fmt.Sprintf("Error reading body: %v\n", err))
		} else {
			output.WriteString("=== Body ===\n")
			output.WriteString(body)
			output.WriteString("\n")
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

func (s *MCPServer) extractMessageBody(msg *mail.Message) (string, error) {
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		// Try to read as plain text
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}

			partMediaType, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
			if strings.HasPrefix(partMediaType, "text/") {
				body, err := io.ReadAll(p)
				if err != nil {
					continue
				}
				return string(body), nil
			}
		}
	} else if strings.HasPrefix(mediaType, "text/") {
		encoding := msg.Header.Get("Content-Transfer-Encoding")
		if encoding == "quoted-printable" {
			qpr := quotedprintable.NewReader(msg.Body)
			body, err := io.ReadAll(qpr)
			if err != nil {
				return "", err
			}
			return string(body), nil
		} else {
			body, err := io.ReadAll(msg.Body)
			if err != nil {
				return "", err
			}
			return string(body), nil
		}
	}

	return "", fmt.Errorf("unsupported content type: %s", mediaType)
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

	// Build recipients list
	recipients := strings.Split(to, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}
	if cc != "" {
		ccList := strings.Split(cc, ",")
		for _, addr := range ccList {
			recipients = append(recipients, strings.TrimSpace(addr))
		}
	}
	if bcc != "" {
		bccList := strings.Split(bcc, ",")
		for _, addr := range bccList {
			recipients = append(recipients, strings.TrimSpace(addr))
		}
	}

	// Build message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", s.config.Email))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	if cc != "" {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")

	// Handle attachments
	var attachments []string
	if attachmentPaths != "" {
		attachments = strings.Split(attachmentPaths, ",")
		for i, path := range attachments {
			attachments[i] = strings.TrimSpace(path)
		}
	}

	if len(attachments) > 0 {
		boundary := fmt.Sprintf("boundary_%d", time.Now().Unix())
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
		msg.WriteString("\r\n")

		// Body part
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		if isHTML {
			msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		} else {
			msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		}
		msg.WriteString("\r\n")
		msg.WriteString(body)
		msg.WriteString("\r\n\r\n")

		// Attachment parts
		for _, attachmentPath := range attachments {
			data, err := os.ReadFile(attachmentPath)
			if err != nil {
				s.sendErrorResult(id, fmt.Sprintf("Failed to read attachment %s: %v", attachmentPath, err))
				return
			}

			filename := filepath.Base(attachmentPath)
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			msg.WriteString(fmt.Sprintf("Content-Type: %s\r\n", mimeType))
			msg.WriteString("Content-Transfer-Encoding: base64\r\n")
			msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", filename))
			msg.WriteString("\r\n")
			msg.WriteString(base64.StdEncoding.EncodeToString(data))
			msg.WriteString("\r\n\r\n")
		}

		msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		// Simple message
		if isHTML {
			msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		} else {
			msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		}
		msg.WriteString("\r\n")
		msg.WriteString(body)
	}

	// Send via SMTP
	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)
	auth := smtp.PlainAuth("", s.config.Email, s.config.Password, s.config.SMTPHost)

	logger.Printf("Connecting to SMTP server: %s\n", addr)

	err := smtp.SendMail(addr, auth, s.config.Email, recipients, []byte(msg.String()))
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to send message: %v", err))
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
	mailboxName, _ := args["mailbox"].(string)
	if mailboxName == "" {
		mailboxName = "INBOX"
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		s.sendError(id, -32602, "Invalid arguments", "query is required")
		return
	}

	limit := 10
	if limitStr, ok := args["limit"].(string); ok && limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	logger.Printf("Searching messages in mailbox: %s, query: %s, limit: %d\n", mailboxName, query, limit)

	c, err := s.connectIMAP()
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer c.Logout()

	_, err = c.Select(mailboxName, true)
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to select mailbox: %v", err))
		return
	}

	// Parse search criteria
	criteria := imap.NewSearchCriteria()
	queryUpper := strings.ToUpper(query)
	
	switch {
	case strings.HasPrefix(queryUpper, "FROM "):
		criteria.Header.Set("From", strings.TrimPrefix(query, "FROM "))
	case strings.HasPrefix(queryUpper, "SUBJECT "):
		criteria.Header.Set("Subject", strings.TrimPrefix(query, "SUBJECT "))
	case queryUpper == "UNSEEN":
		criteria.WithoutFlags = []string{imap.SeenFlag}
	case queryUpper == "SEEN":
		criteria.WithFlags = []string{imap.SeenFlag}
	default:
		// Try to use the query as-is for text search
		criteria.Text = []string{query}
	}

	uids, err := c.Search(criteria)
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Search failed: %v", err))
		return
	}

	if len(uids) == 0 {
		result := ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: "No messages found matching the search criteria.",
				},
			},
		}
		s.sendResponse(id, result)
		return
	}

	// Limit results
	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid}

	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d message(s):\n\n", len(uids)))

	count := 0
	var msgList []*imap.Message
	for msg := range messages {
		msgList = append(msgList, msg)
	}

	// Reverse to show newest first
	for i := len(msgList) - 1; i >= 0; i-- {
		msg := msgList[i]
		count++

		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].Address()
		}

		flags := ""
		for _, f := range msg.Flags {
			if f == imap.SeenFlag {
				flags = "[READ] "
				break
			}
		}

		output.WriteString(fmt.Sprintf("%d. %sSeq: %d, UID: %d\n", count, flags, msg.SeqNum, msg.Uid))
		output.WriteString(fmt.Sprintf("   From: %s\n", from))
		output.WriteString(fmt.Sprintf("   Subject: %s\n", msg.Envelope.Subject))
		output.WriteString(fmt.Sprintf("   Date: %s\n\n", msg.Envelope.Date))
	}

	if err := <-done; err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to fetch messages: %v", err))
		return
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

func (s *MCPServer) listMailboxes(id interface{}, args map[string]interface{}) {
	logger.Println("Listing mailboxes")

	c, err := s.connectIMAP()
	if err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer c.Logout()

	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	var output strings.Builder
	output.WriteString("Available mailboxes:\n\n")

	for m := range mailboxes {
		output.WriteString(fmt.Sprintf("- %s\n", m.Name))
	}

	if err := <-done; err != nil {
		s.sendErrorResult(id, fmt.Sprintf("Failed to list mailboxes: %v", err))
		return
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

func (s *MCPServer) sendErrorResult(id interface{}, errMsg string) {
	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: errMsg,
			},
		},
		IsError: true,
	}
	s.sendResponse(id, result)
}
