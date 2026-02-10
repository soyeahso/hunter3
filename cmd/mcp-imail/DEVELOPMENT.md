# iCloud Mail MCP Plugin - Development Guide

## Architecture

### Protocol Stack
```
Claude AI
   ↓
MCP (JSON-RPC over stdio)
   ↓
mcp-imail server (Go)
   ↓
IMAP/SMTP (iCloud Mail)
```

### Components

1. **MCP Server (`main.go`)**
   - JSON-RPC request/response handling
   - Tool registration and execution
   - Error handling and logging

2. **IMAP Client**
   - Connection management (TLS on port 993)
   - Mailbox operations
   - Message fetching and searching
   - Email parsing

3. **SMTP Client**
   - Email composition
   - Multi-part MIME messages
   - Attachment encoding
   - SMTP delivery (STARTTLS on port 587)

## Project Structure

```
cmd/mcp-imail/
├── main.go              # Main server implementation
├── README.md            # User documentation
├── QUICKSTART.md        # Quick setup guide
├── SUMMARY.md           # Project summary
├── DEVELOPMENT.md       # This file
├── example-usage.md     # Usage examples
└── .gitignore          # Git ignore rules
```

## Code Organization

### Type Definitions

**MCP Protocol Types**:
- `JSONRPCRequest` - Incoming JSON-RPC requests
- `JSONRPCResponse` - Outgoing JSON-RPC responses  
- `Tool` - Tool definition for MCP
- `ToolResult` - Tool execution result

**Application Types**:
- `MCPServer` - Main server struct
- `ImailConfig` - iCloud Mail configuration

### Key Functions

**Server Lifecycle**:
- `main()` - Entry point
- `initLogger()` - Log setup
- `Run()` - Main event loop
- `handleRequest()` - Request dispatcher

**MCP Protocol Handlers**:
- `handleInitialize()` - Initialize server
- `handleListTools()` - List available tools
- `handleCallTool()` - Execute tool

**Email Operations**:
- `connectIMAP()` - Establish IMAP connection
- `listMessages()` - List emails in mailbox
- `readMessage()` - Read email content
- `sendMessage()` - Send email via SMTP
- `searchMessages()` - Search emails
- `listMailboxes()` - List available folders

## Dependencies

### Core Libraries

```go
// IMAP client
github.com/emersion/go-imap v1.2.1
github.com/emersion/go-imap/client

// SMTP (standard library)
net/smtp
crypto/tls
```

### Standard Library
- `encoding/json` - JSON parsing
- `encoding/base64` - Base64 encoding
- `mime` - MIME type handling
- `net/mail` - Email parsing
- `bufio` - Buffered I/O
- `log` - Logging

## Configuration

### Config File Format
```json
{
  "email": "user@icloud.com",
  "password": "xxxx-xxxx-xxxx-xxxx"
}
```

### Environment Variables
- `ICLOUD_EMAIL` - iCloud email address
- `ICLOUD_PASSWORD` - App-Specific Password

### Hardcoded Settings
```go
IMAPHost: "imap.mail.me.com"
IMAPPort: "993"
SMTPHost: "smtp.mail.me.com"
SMTPPort: "587"
```

## Error Handling

### Connection Errors
- IMAP connection failures → User-friendly error message
- SMTP failures → Detailed error with server response
- Authentication failures → Prompt for correct credentials

### Protocol Errors
- Invalid JSON-RPC requests → JSON-RPC error response
- Missing required parameters → Parameter validation error
- Unknown methods/tools → Method not found error

### Email Errors
- Mailbox not found → List available mailboxes
- Message not found → Verify sequence number
- Attachment not found → File path validation

## Logging

### Log Location
```
~/.hunter3/logs/mcp-imail.log
```

### Log Levels
- Info: Normal operations (connections, tool calls)
- Error: Failures and exceptions
- Debug: Request/response details

### Log Format
```
[mcp-imail] 2024/01/15 10:30:00 Message
```

## Testing

### Manual Testing

1. **Start server**:
   ```bash
   ./dist/mcp-imail
   ```

2. **Send JSON-RPC requests via stdin**:
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./dist/mcp-imail
   ```

3. **Check logs**:
   ```bash
   tail -f ~/.hunter3/logs/mcp-imail.log
   ```

### Integration Testing

Test with Claude CLI:
```bash
claude mcp add --transport stdio mcp-imail -- $(pwd)/dist/mcp-imail
claude
```

Then try commands like:
- "List my recent iCloud emails"
- "Send a test email via iCloud"

## Building

### Local Build
```bash
make mcp-imail
```

### Build All
```bash
make all
```

### Clean Build
```bash
make clean
make all
```

## Deployment

### Install for User
```bash
# Build
make all

# Copy to user bin
cp dist/mcp-imail ~/.local/bin/

# Register with Claude
claude mcp add --transport stdio mcp-imail -- ~/.local/bin/mcp-imail
```

### System-wide Install
```bash
# Build
make all

# Install (requires sudo)
sudo cp dist/mcp-imail /usr/local/bin/

# Register for all users
claude mcp add --transport stdio mcp-imail -- /usr/local/bin/mcp-imail
```

## Debugging

### Enable Verbose Logging

Modify `initLogger()` to log to stdout:
```go
logger = log.New(os.Stdout, "[mcp-imail] ", log.LstdFlags)
```

### Test IMAP Connection

```go
// Add test connection in main():
c, err := client.DialTLS("imap.mail.me.com:993", nil)
if err != nil {
    log.Fatal(err)
}
log.Println("Connected!")
c.Login(email, password)
log.Println("Logged in!")
```

### Test SMTP Sending

```go
// Test SMTP directly:
auth := smtp.PlainAuth("", email, password, "smtp.mail.me.com")
msg := []byte("Subject: Test\r\n\r\nTest body")
err := smtp.SendMail("smtp.mail.me.com:587", auth, email, []string{"test@example.com"}, msg)
```

## Performance Considerations

### Connection Management
- IMAP connections are created per-request (stateless)
- Connections are properly closed with defer
- TLS handshake overhead on each connection

### Memory Usage
- Messages buffered in channels (10 message buffer)
- Large attachments are read into memory
- Consider streaming for very large attachments

### Limits
- Default message list limit: 10 (max: 100)
- Search result limit: 10 (max: 100)
- No hard attachment size limit (limited by memory)

## Security

### Credential Storage
- Config file: `~/.hunter3/icloud-mail.json` (mode 600)
- Never log passwords
- App-Specific Password recommended

### TLS/SSL
- IMAP: TLS from start (port 993)
- SMTP: STARTTLS (port 587)
- No certificate validation warnings

### Input Validation
- Email address format validation
- Path traversal prevention for attachments
- Sanitize user input in search queries

## Future Enhancements

### High Priority
- [ ] Mark messages as read/unread
- [ ] Delete messages
- [ ] Move messages between mailboxes
- [ ] Download attachments

### Medium Priority
- [ ] Batch operations
- [ ] Advanced search with multiple criteria
- [ ] Draft message management
- [ ] Email signatures

### Low Priority
- [ ] Connection pooling
- [ ] Async operations
- [ ] Progress reporting for large operations
- [ ] Email templates

## Contributing

### Code Style
- Follow Go conventions
- Use `gofmt` for formatting
- Add comments for exported functions
- Keep functions focused and testable

### Testing Requirements
- Test with real iCloud account
- Verify all tools work correctly
- Check error handling paths
- Test with various mailbox configurations

### Documentation
- Update README.md for user-facing changes
- Update this file for architectural changes
- Add examples for new features
- Keep SUMMARY.md current

## References

- [IMAP RFC 3501](https://tools.ietf.org/html/rfc3501)
- [SMTP RFC 5321](https://tools.ietf.org/html/rfc5321)
- [MIME RFC 2045](https://tools.ietf.org/html/rfc2045)
- [go-imap Documentation](https://github.com/emersion/go-imap)
- [MCP Specification](https://modelcontextprotocol.io/)
- [iCloud Mail Settings](https://support.apple.com/en-us/HT202304)
