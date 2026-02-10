# Google Drive MCP Plugin - Development Guide

## Development Setup

### Prerequisites

- Go 1.25.7 or later
- Google Cloud Platform account
- Google Drive API enabled
- OAuth 2.0 credentials (Desktop app)

### Initial Setup

1. Clone the repository:
```bash
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
```

2. Build the plugin:
```bash
make mcp-gdrive
```

3. Set up credentials (see QUICKSTART.md)

## Code Structure

### Main Components

```
main.go
├── MCP Protocol Types (JSONRPCRequest, JSONRPCResponse, etc.)
├── MCPServer struct
│   ├── driveService *drive.Service
│   └── Run() - Main event loop
├── Request Handlers
│   ├── handleInitialize()
│   ├── handleListTools()
│   └── handleCallTool()
├── Tool Implementations
│   ├── listFiles()
│   ├── getFileInfo()
│   ├── downloadFile()
│   ├── uploadFile()
│   ├── createFolder()
│   ├── deleteFile()
│   ├── searchFiles()
│   └── shareFile()
├── OAuth Helpers
│   ├── initDriveService()
│   ├── getTokenFromWeb()
│   ├── tokenFromFile()
│   └── saveToken()
└── Response Helpers
    ├── sendResponse()
    └── sendError()
```

### MCP Protocol Flow

```
Client                          Server
  │                               │
  ├─── initialize ───────────────>│
  │<─── InitializeResult ─────────┤
  │                               │
  ├─── tools/list ───────────────>│
  │<─── ListToolsResult ──────────┤
  │                               │
  ├─── tools/call ───────────────>│
  │    (with tool name & args)    │
  │<─── ToolResult ───────────────┤
  │                               │
```

## Adding New Tools

### 1. Define the Tool

Add to `handleListTools()`:

```go
{
    Name:        "new_tool",
    Description: "Description of what the tool does",
    InputSchema: InputSchema{
        Type: "object",
        Properties: map[string]Property{
            "param1": {
                Type:        "string",
                Description: "Description of param1",
            },
            "param2": {
                Type:        "string",
                Description: "Description of param2",
            },
        },
        Required: []string{"param1"},
    },
}
```

### 2. Implement the Handler

Add to `handleCallTool()` switch:

```go
case "new_tool":
    s.newTool(req.ID, params.Arguments)
```

### 3. Create the Implementation

```go
func (s *MCPServer) newTool(id interface{}, args map[string]interface{}) {
    // Extract arguments
    param1, ok := args["param1"].(string)
    if !ok || param1 == "" {
        s.sendError(id, -32602, "Invalid arguments", "param1 is required")
        return
    }

    param2, _ := args["param2"].(string)

    logger.Printf("Executing new_tool: param1=%s, param2=%s\n", param1, param2)

    // Call Google Drive API
    // ... your implementation ...

    // Send response
    result := ToolResult{
        Content: []ContentItem{
            {
                Type: "text",
                Text: "Success message",
            },
        },
    }
    s.sendResponse(id, result)
}
```

## Testing

### Manual Testing

1. Start the server manually:
```bash
./dist/mcp-gdrive
```

2. Send JSON-RPC requests via stdin:
```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_files","arguments":{}}}
```

### Testing with Claude CLI

```bash
# Register the plugin
make mcp-register

# Start Claude
claude

# Test the plugin
> List files in my Google Drive
```

### Debugging

1. Check logs:
```bash
tail -f ~/.hunter3/logs/mcp-gdrive.log
```

2. Add debug logging:
```go
logger.Printf("Debug: variable=%v\n", variable)
```

3. Test individual functions:
```go
// Add to main() for testing
func main() {
    initLogger()
    server := &MCPServer{}
    server.initDriveService()
    
    // Test specific functionality
    args := map[string]interface{}{
        "query": "name contains 'test'",
    }
    server.listFiles(1, args)
}
```

## Google Drive API Reference

### Common Operations

#### List Files
```go
s.driveService.Files.List().
    PageSize(20).
    Q("name contains 'test'").
    Fields("files(id, name, mimeType)").
    Do()
```

#### Get File
```go
s.driveService.Files.Get(fileID).
    Fields("id, name, size, mimeType").
    Do()
```

#### Download File
```go
resp, err := s.driveService.Files.Get(fileID).Download()
content, _ := io.ReadAll(resp.Body)
```

#### Upload File
```go
file := &drive.File{
    Name: "filename.txt",
}
s.driveService.Files.Create(file).
    Media(strings.NewReader(content)).
    Do()
```

#### Create Folder
```go
folder := &drive.File{
    Name:     "Folder Name",
    MimeType: "application/vnd.google-apps.folder",
}
s.driveService.Files.Create(folder).Do()
```

#### Share File
```go
permission := &drive.Permission{
    Type:         "user",
    Role:         "reader",
    EmailAddress: "user@example.com",
}
s.driveService.Permissions.Create(fileID, permission).Do()
```

### Query Syntax

```go
// Name queries
"name = 'exact name'"
"name contains 'substring'"

// Type queries
"mimeType = 'application/pdf'"

// Date queries
"modifiedTime > '2024-01-01'"
"createdTime < '2024-12-31'"

// Size queries
"size > 1000000"  // bytes

// Folder queries
"'folderID' in parents"

// Trash queries
"trashed = false"

// Combine with operators
"name contains 'report' and mimeType = 'application/pdf' and not trashed"
```

### MIME Types

```go
const (
    TypePDF         = "application/pdf"
    TypeWord        = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    TypeExcel       = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
    TypePowerPoint  = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
    TypeFolder      = "application/vnd.google-apps.folder"
    TypeGoogleDoc   = "application/vnd.google-apps.document"
    TypeGoogleSheet = "application/vnd.google-apps.spreadsheet"
    TypeGoogleSlide = "application/vnd.google-apps.presentation"
)
```

## Error Handling

### MCP Error Codes

```go
const (
    ParseError     = -32700  // Invalid JSON
    InvalidRequest = -32600  // Invalid request object
    MethodNotFound = -32601  // Method doesn't exist
    InvalidParams  = -32602  // Invalid method parameters
    InternalError  = -32603  // Internal JSON-RPC error
)
```

### Best Practices

1. Always validate required parameters:
```go
param, ok := args["param"].(string)
if !ok || param == "" {
    s.sendError(id, -32602, "Invalid arguments", "param is required")
    return
}
```

2. Handle API errors gracefully:
```go
result, err := s.driveService.Files.List().Do()
if err != nil {
    logger.Printf("API error: %v\n", err)
    s.sendError(id, -32603, "API error", err.Error())
    return
}
```

3. Log all operations:
```go
logger.Printf("Operation: param1=%s, param2=%s\n", param1, param2)
```

4. Provide helpful error messages:
```go
s.sendError(id, -32602, "Invalid file ID", 
    "The file ID must be a non-empty string. Get file IDs using list_files or search_files.")
```

## Performance Considerations

### Pagination

Always use pagination for list operations:
```go
call := s.driveService.Files.List().PageSize(maxResults)
```

### Field Selection

Only request needed fields:
```go
.Fields("files(id, name, mimeType)")  // Good
.Fields("*")                          // Bad - returns everything
```

### Batching

For multiple operations, consider batch requests:
```go
// TODO: Implement batch operations
```

### Rate Limits

Google Drive API has rate limits:
- 1,000 queries per 100 seconds per user
- 10,000 queries per 100 seconds per project

Implement exponential backoff for rate limit errors.

## Security Best Practices

1. **Never log sensitive data**:
```go
// Bad
logger.Printf("Token: %s\n", token.AccessToken)

// Good
logger.Printf("Token refreshed successfully\n")
```

2. **Validate all inputs**:
```go
// Sanitize file paths
if strings.Contains(path, "..") {
    return errors.New("invalid path")
}
```

3. **Use minimal scopes**:
```go
// Only request needed scopes
config, err := google.ConfigFromJSON(b, 
    drive.DriveScope,
    drive.DriveFileScope,
    drive.DriveMetadataReadonlyScope)
```

4. **Secure credential storage**:
```go
// Use 0600 permissions
os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
```

## Building and Deployment

### Local Build

```bash
make mcp-gdrive
```

### Build All Plugins

```bash
make mcp-all
```

### Clean Build

```bash
make clean && make mcp-gdrive
```

### Cross-Platform Build

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o dist/mcp-gdrive-linux ./cmd/mcp-gdrive

# macOS
GOOS=darwin GOARCH=amd64 go build -o dist/mcp-gdrive-mac ./cmd/mcp-gdrive

# Windows
GOOS=windows GOARCH=amd64 go build -o dist/mcp-gdrive.exe ./cmd/mcp-gdrive
```

## Contributing

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `go vet` before committing
- Add comments for exported functions
- Keep functions focused and small

### Commit Messages

```
Add feature: Brief description

Detailed explanation of what changed and why.
Include any relevant issue numbers.
```

### Pull Request Process

1. Create a feature branch
2. Implement changes
3. Test thoroughly
4. Update documentation
5. Submit PR with clear description

## Troubleshooting

### Common Issues

**"Credentials file not found"**
- Check `~/.hunter3/gdrive-credentials.json` exists
- Verify file permissions

**"Token expired"**
- Delete `~/.hunter3/gdrive-token.json`
- Re-authenticate

**"API rate limit exceeded"**
- Implement exponential backoff
- Reduce request frequency
- Check quota in Google Cloud Console

**"File not found"**
- Verify file ID is correct
- Check file hasn't been deleted
- Ensure user has access permissions

## Resources

- [Google Drive API Documentation](https://developers.google.com/drive/api/v3/reference)
- [MCP Protocol Specification](https://modelcontextprotocol.io/)
- [Go Google API Client](https://pkg.go.dev/google.golang.org/api/drive/v3)
- [OAuth 2.0 Guide](https://developers.google.com/identity/protocols/oauth2)

## License

See LICENSE file in the root of the repository.
