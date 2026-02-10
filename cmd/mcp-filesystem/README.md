# MCP Filesystem Server (Go)

A Go implementation of the Model Context Protocol filesystem server, providing secure file system operations within specified allowed directories.

## Features

This MCP server provides comprehensive file system operations:

### Read Operations
- **read_file** / **read_text_file** - Read complete file contents with optional head/tail
- **read_media_file** - Read images and audio files with base64 encoding
- **read_multiple_files** - Batch read multiple files efficiently
- **list_directory** - List directory contents with file/dir indicators
- **list_directory_with_sizes** - List with file sizes and sorting options
- **directory_tree** - Recursive tree view as JSON with exclusion patterns
- **search_files** - Glob pattern search with exclusions
- **get_file_info** - Detailed file/directory metadata

### Write Operations
- **write_file** - Create or overwrite files
- **edit_file** - Line-based editing with git-style diff output
- **create_directory** - Create directories recursively
- **move_file** - Move/rename files and directories

### Utility
- **list_allowed_directories** - Show accessible directory roots

## Usage

The server requires at least one allowed directory to be specified:

```bash
./mcp-filesystem /path/to/allowed/directory [additional/directories...]
```

### Example

```bash
./mcp-filesystem /home/user/projects /home/user/documents
```

## Security

- All file operations are restricted to allowed directories and their subdirectories
- Symlinks are resolved during validation to prevent escape attempts
- Paths are normalized and validated before any operation
- Parent directory traversal (../) is blocked if it would escape allowed directories

## Configuration

### In .mcp.json

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/path/to/dist/mcp-filesystem",
      "args": [
        "/home/user/workspace",
        "/home/user/documents"
      ]
    }
  }
}
```

## Building

```bash
# Build just the filesystem server
make mcp-filesystem

# Build all MCP servers
make all
```

## Logs

Logs are written to `~/.hunter3/logs/mcp-filesystem.log`

## Implementation Details

This is a native Go rewrite of the TypeScript `@modelcontextprotocol/server-filesystem`. Key differences:

- Written in Go for better performance and single-binary distribution
- Uses Go's standard library for file operations
- Implements the MCP 2024-11-05 protocol version
- Hot-reload support via `autorestart` during development
- Structured logging to both file and stderr

## Tools Reference

All tools support the standard MCP protocol format with JSON-RPC 2.0 over stdio transport.

### read_text_file
```json
{
  "path": "file.txt",
  "head": 10,  // optional: first N lines
  "tail": 10   // optional: last N lines
}
```

### read_media_file
```json
{
  "path": "image.png"
}
```

Returns base64-encoded data with MIME type detection.

### write_file
```json
{
  "path": "file.txt",
  "content": "Hello, world!"
}
```

### edit_file
```json
{
  "path": "file.txt",
  "edits": [
    {
      "oldText": "old content",
      "newText": "new content"
    }
  ],
  "dryRun": false
}
```

### search_files
```json
{
  "path": "/search/root",
  "pattern": "*.go",
  "excludePatterns": ["vendor", "node_modules"]
}
```

## License

Same as parent Hunter3 project.
