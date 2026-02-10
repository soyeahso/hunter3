# MCP Gmail Plugin - Development Guide

## Architecture

The MCP Gmail plugin follows the Model Context Protocol (MCP) specification and provides Gmail integration through the Google Gmail API.

### Components

```
mcp-gmail/
├── main.go                      # Core plugin implementation
├── README.md                    # Full documentation
├── QUICKSTART.md               # Quick start guide
├── DEVELOPMENT.md              # This file
├── example-usage.md            # Usage examples
├── Makefile                    # Build automation
├── setup.sh                    # Setup script
├── credentials-template.json   # OAuth credentials template
└── .gitignore                  # Git ignore rules
```

## Code Structure

### Main Components

1. **MCP Protocol Implementation**
   - JSON-RPC 2.0 request/response handling
   - MCP protocol types (Tools, Capabilities, etc.)
   - Standard stdin/stdout communication

2. **Gmail Service Integration**
   - OAuth 2.0 authentication flow
   - Gmail API service initialization
   - Token management and refresh

3. **Tool Implementations**
   - `list_messages` - List/search messages
   - `read_message` - Read message content
   - `send_message` - Send emails with attachments
   - `search_messages` - Advanced search

### Key Functions

#### Authentication Flow
```go
initGmailService() -> reads credentials -> gets token -> creates service
```

#### Message Handling
```go
handleRequest() -> parse JSON-RPC -> route to handler -> send response
```

#### Tool Execution
```go
handleCallTool() -> validate params -> execute tool -> format result
```

## Gmail API Integration

### Required OAuth Scopes

```go
gmail.GmailReadonlyScope   // Read emails
gmail.GmailSendScope       // Send emails  
gmail.GmailComposeScope    // Compose/draft emails
gmail.GmailModifyScope     // Modify labels/properties
```

### API Calls Used

- `Users.Messages.List()` - List messages
- `Users.Messages.Get()` - Get message details
- `Users.Messages.Send()` - Send message

### Message Format

Emails are encoded using RFC 2822 format with base64 URL encoding:
```
To: recipient@example.com
Subject: Hello
MIME-Version: 1.0
Content-Type: text/plain

Body content
```

## Adding New Tools

To add a new tool:

1. **Define the tool** in `handleListTools()`:
```go
{
    Name:        "my_new_tool",
    Description: "Does something useful",
    InputSchema: InputSchema{
        Type: "object",
        Properties: map[string]Property{
            "param1": {
                Type:        "string",
                Description: "Parameter description",
            },
        },
        Required: []string{"param1"},
    },
}
```

2. **Add handler** in `handleCallTool()`:
```go
case "my_new_tool":
    s.myNewTool(req.ID, params.Arguments)
```

3. **Implement the function**:
```go
func (s *MCPServer) myNewTool(id interface{}, args map[string]interface{}) {
    // Extract parameters
    param1, ok := args["param1"].(string)
    if !ok {
        s.sendError(id, -32602, "Invalid arguments", "param1 is required")
        return
    }
    
    // Do work with Gmail API
    // ...
    
    // Return result
    result := ToolResult{
        Content: []ContentItem{
            {Type: "text", Text: "Success!"},
        },
    }
    s.sendResponse(id, result)
}
```

## Testing

### Unit Testing

Create test files following Go conventions:

```go
// main_test.go
package main

import "testing"

func TestExtractBody(t *testing.T) {
    // Test message body extraction
}

func TestBuildMessage(t *testing.T) {
    // Test message building
}
```

Run tests:
```bash
go test ./cmd/mcp-gmail/...
```

### Integration Testing

Test with real Gmail API (requires authentication):

```bash
# Set up test credentials
export GMAIL_CREDENTIALS_FILE=~/.hunter3/gmail-test-credentials.json

# Run integration tests
go test -tags=integration ./cmd/mcp-gmail/...
```

### Manual Testing

Use the test script:

```bash
# Create test-interactive.sh
cat > test-interactive.sh << 'EOF'
#!/bin/bash
./bin/mcp-gmail << 'JSONRPC'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
JSONRPC
EOF

chmod +x test-interactive.sh
./test-interactive.sh
```

## Debugging

### Enable Verbose Logging

Logs are written to `~/.hunter3/logs/mcp-gmail.log`:

```bash
# Tail logs in real-time
tail -f ~/.hunter3/logs/mcp-gmail.log

# Search for errors
grep ERROR ~/.hunter3/logs/mcp-gmail.log
```

### Debug Authentication Issues

```bash
# Check credentials file
cat ~/.hunter3/gmail-credentials.json | jq .

# Check token file
cat ~/.hunter3/gmail-token.json | jq .

# Test OAuth flow manually
go run ./cmd/mcp-gmail/main.go
```

### Debug MCP Protocol

```bash
# Capture stdin/stdout
./bin/mcp-gmail < input.json > output.json 2> errors.log

# Pretty print JSON
cat output.json | jq .
```

## Performance Considerations

### Rate Limiting

Gmail API has quotas:
- 250 quota units per user per second
- 1,000,000,000 quota units per day

Operations costs:
- `messages.list`: 5 units
- `messages.get`: 5 units
- `messages.send`: 100 units

### Caching

Consider implementing caching for:
- Message lists (short TTL)
- Message headers (medium TTL)
- Message bodies (longer TTL)

### Pagination

Large result sets should use pagination:

```go
call := s.gmailService.Users.Messages.List("me")
call = call.MaxResults(100)
call = call.PageToken(nextPageToken)
```

## Security Best Practices

1. **Credentials Storage**
   - Never commit credentials to git
   - Use file permissions 0600 for credential files
   - Store in user home directory only

2. **Token Management**
   - Tokens auto-refresh when expired
   - Store tokens securely (0600 permissions)
   - Implement token revocation on uninstall

3. **Input Validation**
   - Validate all user inputs
   - Sanitize file paths for attachments
   - Check attachment sizes before reading

4. **Error Handling**
   - Don't leak sensitive info in errors
   - Log errors appropriately
   - Return user-friendly error messages

## Dependencies

Core dependencies:
```go
golang.org/x/oauth2          // OAuth 2.0 authentication
google.golang.org/api/gmail  // Gmail API client
```

Update dependencies:
```bash
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
go get -u golang.org/x/oauth2
go get -u google.golang.org/api/gmail/v1
go mod tidy
```

## Building and Distribution

### Development Build
```bash
make build
```

### Production Build
```bash
# With optimizations
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
go build -ldflags="-s -w" -o bin/mcp-gmail ./cmd/mcp-gmail

# Strip binary further
strip bin/mcp-gmail
```

### Cross-compilation
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o bin/mcp-gmail-linux-amd64 ./cmd/mcp-gmail

# macOS
GOOS=darwin GOARCH=arm64 go build -o bin/mcp-gmail-darwin-arm64 ./cmd/mcp-gmail

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/mcp-gmail-windows-amd64.exe ./cmd/mcp-gmail
```

## Contributing

### Code Style

Follow standard Go conventions:
- Use `gofmt` for formatting
- Use `golint` for linting
- Add comments for exported functions
- Keep functions focused and small

```bash
# Format code
gofmt -w main.go

# Lint code
golint ./cmd/mcp-gmail/...

# Vet code
go vet ./cmd/mcp-gmail/...
```

### Commit Messages

Follow conventional commits:
```
feat: add draft email support
fix: handle empty message body
docs: update README with examples
refactor: simplify attachment handling
```

## Future Enhancements

Potential features to add:

1. **Draft Management**
   - Create drafts
   - Edit drafts
   - Delete drafts

2. **Label Management**
   - List labels
   - Apply labels
   - Remove labels

3. **Advanced Features**
   - Thread support
   - Calendar integration
   - Contact management
   - Spam filtering

4. **Performance**
   - Message caching
   - Batch operations
   - Streaming for large attachments

5. **UX Improvements**
   - Better error messages
   - Progress indicators
   - Retry logic

## Resources

- [Gmail API Documentation](https://developers.google.com/gmail/api)
- [MCP Specification](https://modelcontextprotocol.io)
- [OAuth 2.0 Flow](https://developers.google.com/identity/protocols/oauth2)
- [Go Gmail Library](https://pkg.go.dev/google.golang.org/api/gmail/v1)

## License

See main project LICENSE file.
