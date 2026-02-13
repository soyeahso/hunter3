# MCP SSH Plugin

A Model Context Protocol (MCP) server that provides SSH connectivity and remote command execution capabilities.

## Features

- **SSH Connection Management**: Connect to remote servers using password or key-based authentication
- **Remote Command Execution**: Execute commands on remote servers with timeout support
- **File Transfer**: Upload and download files via SCP
- **Session Management**: Maintain multiple SSH sessions simultaneously
- **Secure Authentication**: Support for password and private key authentication (with passphrase support)

## Installation

```bash
go build -o mcp-ssh
```

## Configuration

Add to your MCP settings file (e.g., `~/.claude/mcp.json`):

```json
{
  "mcpServers": {
    "ssh": {
      "command": "/path/to/mcp-ssh"
    }
  }
}
```

## Tools

### ssh_connect

Connect to a remote server via SSH.

**Parameters:**
- `host` (required): Hostname or IP address
- `username` (required): SSH username
- `port` (optional): SSH port (default: 22)
- `password` (optional): SSH password
- `key_path` (optional): Path to private key file
- `key_passphrase` (optional): Passphrase for encrypted private key
- `session_name` (optional): Name for this session (auto-generated if not provided)

**Example:**
```json
{
  "host": "example.com",
  "username": "user",
  "password": "secret",
  "session_name": "my-server"
}
```

### ssh_execute

Execute a command on a remote server.

**Parameters:**
- `session_name` (required): Name of the SSH session
- `command` (required): Command to execute
- `working_dir` (optional): Working directory for command execution
- `timeout` (optional): Command timeout in seconds (default: 300)

**Example:**
```json
{
  "session_name": "my-server",
  "command": "ls -la /var/log",
  "timeout": 60
}
```

### ssh_upload

Upload a file to a remote server.

**Parameters:**
- `session_name` (required): Name of the SSH session
- `local_path` (required): Local file path
- `remote_path` (required): Remote destination path
- `permissions` (optional): File permissions in octal format (e.g., "0644")

**Example:**
```json
{
  "session_name": "my-server",
  "local_path": "/local/file.txt",
  "remote_path": "/remote/file.txt",
  "permissions": "0644"
}
```

### ssh_download

Download a file from a remote server.

**Parameters:**
- `session_name` (required): Name of the SSH session
- `remote_path` (required): Remote file path
- `local_path` (required): Local destination path

**Example:**
```json
{
  "session_name": "my-server",
  "remote_path": "/remote/file.txt",
  "local_path": "/local/file.txt"
}
```

### ssh_list_sessions

List all active SSH sessions.

**Parameters:** None

### ssh_disconnect

Disconnect an SSH session.

**Parameters:**
- `session_name` (required): Name of the session to disconnect

**Example:**
```json
{
  "session_name": "my-server"
}
```

## Security Considerations

⚠️ **Important Security Notes:**

1. **Host Key Verification**: The current implementation uses `InsecureIgnoreHostKey()` for simplicity. In production, implement proper host key verification.

2. **Credential Storage**: Credentials are not persisted. Sessions are kept in memory only.

3. **Private Keys**: Store private keys securely with appropriate file permissions (0600).

4. **Timeouts**: Command execution has configurable timeouts to prevent hanging operations.

## Usage Examples

### Connect with Password

```
Connect to server with:
- host: example.com
- username: admin
- password: mypassword
- session_name: prod-server
```

### Connect with SSH Key

```
Connect to server with:
- host: example.com
- username: admin
- key_path: /home/user/.ssh/id_rsa
- session_name: prod-server
```

### Execute Commands

```
Execute on prod-server:
- command: df -h
- timeout: 30
```

### File Operations

```
Upload file:
- session_name: prod-server
- local_path: /local/config.yml
- remote_path: /etc/app/config.yml
- permissions: 0644

Download file:
- session_name: prod-server
- remote_path: /var/log/app.log
- local_path: /tmp/app.log
```

## Dependencies

- `github.com/mark3labs/mcp-go` - MCP Go SDK
- `golang.org/x/crypto/ssh` - SSH client implementation

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
