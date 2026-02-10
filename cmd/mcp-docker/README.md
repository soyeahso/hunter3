# MCP Docker Plugin

A Model Context Protocol (MCP) server for managing Docker containers, images, networks, volumes, and Docker Compose projects.

## Features

### Container Management
- **docker_ps** - List containers with filtering and formatting
- **docker_run** - Run commands in new containers with full configuration
- **docker_start** - Start stopped containers
- **docker_stop** - Stop running containers
- **docker_restart** - Restart containers
- **docker_rm** - Remove containers
- **docker_exec** - Execute commands in running containers
- **docker_logs** - Fetch container logs with filtering
- **docker_inspect** - Get detailed information about containers
- **docker_stats** - Display resource usage statistics

### Image Management
- **docker_images** - List images with filtering
- **docker_pull** - Pull images from registries
- **docker_push** - Push images to registries
- **docker_rmi** - Remove images
- **docker_build** - Build images from Dockerfiles
- **docker_tag** - Tag images

### Network Management
- **docker_network_ls** - List networks
- **docker_network_create** - Create networks
- **docker_network_rm** - Remove networks
- **docker_network_connect** - Connect containers to networks
- **docker_network_disconnect** - Disconnect containers from networks

### Volume Management
- **docker_volume_ls** - List volumes
- **docker_volume_create** - Create volumes
- **docker_volume_rm** - Remove volumes
- **docker_volume_inspect** - Inspect volumes

### Docker Compose
- **docker_compose_up** - Create and start services
- **docker_compose_down** - Stop and remove services
- **docker_compose_ps** - List containers in compose project
- **docker_compose_logs** - View compose service logs

### System Commands
- **docker_info** - Display system-wide information
- **docker_version** - Show Docker version
- **docker_system_df** - Show disk usage
- **docker_system_prune** - Remove unused data

## Installation

### Build from source

```bash
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
make mcp-docker
```

This will:
1. Build the binary to `dist/mcp-docker`
2. Register it with the Claude CLI

### Manual registration

```bash
claude mcp add --transport stdio mcp-docker -- /path/to/dist/mcp-docker
```

## Usage Examples

### Container Operations

**List running containers:**
```json
{
  "name": "docker_ps"
}
```

**List all containers (including stopped):**
```json
{
  "name": "docker_ps",
  "arguments": {
    "all": true
  }
}
```

**Run a container:**
```json
{
  "name": "docker_run",
  "arguments": {
    "image": "nginx:latest",
    "detach": true,
    "name": "my-nginx",
    "ports": ["8080:80"],
    "env": ["DEBUG=1"]
  }
}
```

**Execute command in container:**
```json
{
  "name": "docker_exec",
  "arguments": {
    "container": "my-nginx",
    "command": ["ls", "-la", "/etc/nginx"]
  }
}
```

**View container logs:**
```json
{
  "name": "docker_logs",
  "arguments": {
    "container": "my-nginx",
    "tail": "100",
    "timestamps": true
  }
}
```

### Image Operations

**Pull an image:**
```json
{
  "name": "docker_pull",
  "arguments": {
    "image": "ubuntu:22.04"
  }
}
```

**Build an image:**
```json
{
  "name": "docker_build",
  "arguments": {
    "path": "./myapp",
    "tag": ["myapp:latest", "myapp:v1.0"],
    "file": "Dockerfile",
    "build_arg": ["HTTP_PROXY=http://proxy.example.com"]
  }
}
```

**Tag an image:**
```json
{
  "name": "docker_tag",
  "arguments": {
    "source": "myapp:latest",
    "target": "myrepo/myapp:v1.0"
  }
}
```

### Network Operations

**Create a network:**
```json
{
  "name": "docker_network_create",
  "arguments": {
    "name": "my-network",
    "driver": "bridge",
    "subnet": "172.20.0.0/16"
  }
}
```

**Connect container to network:**
```json
{
  "name": "docker_network_connect",
  "arguments": {
    "network": "my-network",
    "container": "my-nginx",
    "alias": ["web"]
  }
}
```

### Volume Operations

**Create a volume:**
```json
{
  "name": "docker_volume_create",
  "arguments": {
    "name": "my-data",
    "driver": "local"
  }
}
```

**List volumes:**
```json
{
  "name": "docker_volume_ls",
  "arguments": {
    "filter": ["dangling=true"]
  }
}
```

### Docker Compose

**Start services:**
```json
{
  "name": "docker_compose_up",
  "arguments": {
    "file": "docker-compose.yml",
    "detach": true,
    "build": true
  }
}
```

**Stop services:**
```json
{
  "name": "docker_compose_down",
  "arguments": {
    "volumes": true
  }
}
```

### System Commands

**Show disk usage:**
```json
{
  "name": "docker_system_df",
  "arguments": {
    "verbose": true
  }
}
```

**Clean up unused resources:**
```json
{
  "name": "docker_system_prune",
  "arguments": {
    "all": true,
    "volumes": true,
    "force": true
  }
}
```

## Response Format

All commands return a JSON response with the following structure:

```json
{
  "command": "docker ps -a",
  "success": true,
  "stdout": "CONTAINER ID   IMAGE     COMMAND   ...",
  "stderr": "",
  "error": ""
}
```

- `command`: The exact Docker command that was executed
- `success`: Boolean indicating if the command succeeded
- `stdout`: Standard output from the command
- `stderr`: Standard error output (if any)
- `error`: Error message (if command failed)

## Logging

Logs are written to `~/.hunter3/logs/mcp-docker.log`

To monitor logs in real-time:
```bash
tail -f ~/.hunter3/logs/mcp-docker.log
```

Or use the Makefile target:
```bash
make tail_logs
```

## Requirements

- Docker installed and accessible via the `docker` command
- Docker Compose (for compose commands)
- Go 1.25+ (for building from source)

## Security Considerations

The plugin executes Docker commands directly on the host system. Users should:
- Be aware of what commands are being executed
- Review container configurations before running
- Be cautious with `docker_system_prune` and `docker_rm` with force flags
- Ensure proper Docker permissions are configured

## Development

### Building
```bash
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
make mcp-docker
```

### Testing
```bash
# Test the binary directly
echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | ./dist/mcp-docker

# Test with Claude CLI
claude mcp test mcp-docker
```

### Debugging
Enable verbose logging by checking `~/.hunter3/logs/mcp-docker.log`

## License

Same as the Hunter3 project.
