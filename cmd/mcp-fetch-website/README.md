# MCP Fetch Website Plugin

A Model Context Protocol (MCP) server that provides web fetching capabilities.

## Features

- Fetch URLs via HTTP/HTTPS
- Support for multiple HTTP methods (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)
- Custom headers support
- Request body support for POST/PUT/PATCH
- Returns full HTTP response including headers and status

## Tool: fetch

### Parameters

- `url` (required): URL to fetch (must start with http:// or https://)
- `method` (optional): HTTP method (default: GET)
- `headers` (optional): JSON string of headers (e.g., `{"Authorization": "Bearer token"}`)
- `body` (optional): Request body for POST/PUT/PATCH requests

### Examples

#### Simple GET request
```json
{
  "url": "https://api.example.com/data"
}
```

#### POST with headers and body
```json
{
  "url": "https://api.example.com/create",
  "method": "POST",
  "headers": "{\"Content-Type\": \"application/json\", \"Authorization\": \"Bearer token123\"}",
  "body": "{\"name\": \"test\", \"value\": 123}"
}
```

## Building

```bash
make mcp-fetch-website
```

## Configuration

Add to `.mcp.json`:

```json
{
  "mcpServers": {
    "fetch": {
      "command": "/path/to/dist/mcp-fetch-website",
      "args": []
    }
  }
}
```
