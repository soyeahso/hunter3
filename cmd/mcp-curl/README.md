# MCP Curl Plugin

A Model Context Protocol (MCP) plugin that wraps the curl command-line tool, providing access to all curl features through a structured API.

## Features

- Full curl command support with all standard options
- HTTP methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
- Custom headers support
- Request body data (raw and form-data)
- Authentication (Basic, Bearer, etc.)
- Cookie handling (send and save)
- Proxy support
- SSL/TLS options
- Timeout controls
- Redirect following
- Verbose mode for debugging
- Response header inclusion
- File upload via form-data
- Compressed response handling
- Custom User-Agent
- Extra flags for advanced curl features

## Installation

Build and register the plugin:

```bash
make mcp-curl
make mcp-register
```

## Usage

The plugin provides a single `curl` tool with extensive configuration options.

### Basic GET Request

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://api.example.com/data"
  }
}
```

### POST with JSON Data

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://api.example.com/data",
    "method": "POST",
    "headers": [
      "Content-Type: application/json",
      "Authorization: Bearer token123"
    ],
    "data": "{\"key\":\"value\"}"
  }
}
```

### Form Data Upload

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://api.example.com/upload",
    "method": "POST",
    "form_data": [
      "name=John Doe",
      "file=@/path/to/file.txt"
    ]
  }
}
```

### Authentication

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://api.example.com/secure",
    "auth": "username:password"
  }
}
```

### With Proxy

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://api.example.com/data",
    "proxy": "http://proxy.example.com:8080"
  }
}
```

### Verbose Mode for Debugging

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://api.example.com/data",
    "verbose": true,
    "include_headers": true
  }
}
```

### Save to File

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://example.com/file.zip",
    "output": "/path/to/save/file.zip"
  }
}
```

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | URL to fetch |
| `method` | string | No | HTTP method (default: GET) |
| `headers` | array[string] | No | Array of headers in 'Key: Value' format |
| `data` | string | No | Request body data |
| `form_data` | array[string] | No | Form data fields (supports file uploads with @) |
| `output` | string | No | Save response to file |
| `user_agent` | string | No | Custom User-Agent string |
| `follow_redirects` | boolean | No | Follow redirects (default: true) |
| `insecure` | boolean | No | Allow insecure SSL connections |
| `verbose` | boolean | No | Enable verbose output |
| `timeout` | number | No | Connection timeout in seconds (default: 30) |
| `max_time` | number | No | Maximum time for operation in seconds |
| `proxy` | string | No | Proxy server URL |
| `cookie` | string | No | Send cookies from string/file |
| `cookie_jar` | string | No | Save cookies to file |
| `auth` | string | No | Server authentication (user:password) |
| `include_headers` | boolean | No | Include response headers in output |
| `show_error` | boolean | No | Show error messages (default: true) |
| `silent` | boolean | No | Silent mode |
| `compressed` | boolean | No | Request compressed response |
| `extra_flags` | array[string] | No | Additional curl flags not covered above |

## Extra Flags

The `extra_flags` parameter allows you to pass any curl flags not explicitly covered by the structured parameters. Examples:

```json
{
  "name": "curl",
  "arguments": {
    "url": "https://api.example.com/data",
    "extra_flags": ["--http2", "--ipv4", "--limit-rate", "1M"]
  }
}
```

## Response Format

The plugin returns the curl command output as text, including:
- HTTP status and headers (if `include_headers: true`)
- Response body
- Error messages (if any)

If the curl command exits with a non-zero status, the response will have `isError: true`.

## Logging

Logs are written to `~/.hunter3/logs/mcp-curl.log` and include:
- All curl commands executed
- Command parameters
- Output lengths
- Error conditions

## Security Notes

- The plugin executes system curl commands directly
- Be cautious with file paths in `form_data` and `output` parameters
- Validate URLs and headers when accepting user input
- Use `insecure: true` only when necessary (development/testing)

## Requirements

- System curl command must be installed and in PATH
- Go 1.19 or higher for building

## License

Part of the Hunter3 project - see main LICENSE file
