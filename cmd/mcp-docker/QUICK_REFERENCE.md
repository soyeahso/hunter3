# MCP Docker Quick Reference

## Build & Install
```bash
cd /home/genoeg/go/src/github.com/soyeahso/hunter3
make mcp-docker
```

## Tool Categories

### 📦 Containers (10 tools)
| Tool | Purpose |
|------|---------|
| `docker_ps` | List containers |
| `docker_run` | Create & run container |
| `docker_start` | Start stopped container |
| `docker_stop` | Stop running container |
| `docker_restart` | Restart container |
| `docker_rm` | Remove container |
| `docker_exec` | Execute command in container |
| `docker_logs` | View container logs |
| `docker_inspect` | Get detailed info |
| `docker_stats` | Show resource usage |

### 🖼️ Images (6 tools)
| Tool | Purpose |
|------|---------|
| `docker_images` | List images |
| `docker_pull` | Pull from registry |
| `docker_push` | Push to registry |
| `docker_rmi` | Remove image |
| `docker_build` | Build from Dockerfile |
| `docker_tag` | Tag image |

### 🌐 Networks (5 tools)
| Tool | Purpose |
|------|---------|
| `docker_network_ls` | List networks |
| `docker_network_create` | Create network |
| `docker_network_rm` | Remove network |
| `docker_network_connect` | Connect container |
| `docker_network_disconnect` | Disconnect container |

### 💾 Volumes (4 tools)
| Tool | Purpose |
|------|---------|
| `docker_volume_ls` | List volumes |
| `docker_volume_create` | Create volume |
| `docker_volume_rm` | Remove volume |
| `docker_volume_inspect` | Inspect volume |

### 🎼 Compose (4 tools)
| Tool | Purpose |
|------|---------|
| `docker_compose_up` | Start services |
| `docker_compose_down` | Stop services |
| `docker_compose_ps` | List services |
| `docker_compose_logs` | View service logs |

### ⚙️ System (4 tools)
| Tool | Purpose |
|------|---------|
| `docker_info` | System info |
| `docker_version` | Docker version |
| `docker_system_df` | Disk usage |
| `docker_system_prune` | Clean up |

## Common Examples

### List all containers
```json
{"name": "docker_ps", "arguments": {"all": true}}
```

### Run nginx
```json
{
  "name": "docker_run",
  "arguments": {
    "image": "nginx:latest",
    "detach": true,
    "name": "web",
    "ports": ["8080:80"]
  }
}
```

### Execute shell
```json
{
  "name": "docker_exec",
  "arguments": {
    "container": "web",
    "command": ["sh"],
    "interactive": true,
    "tty": true
  }
}
```

### View logs (last 100 lines)
```json
{
  "name": "docker_logs",
  "arguments": {
    "container": "web",
    "tail": "100"
  }
}
```

### Build image
```json
{
  "name": "docker_build",
  "arguments": {
    "path": "./app",
    "tag": ["myapp:latest"]
  }
}
```

### Create network
```json
{
  "name": "docker_network_create",
  "arguments": {
    "name": "backend",
    "driver": "bridge"
  }
}
```

### Create volume
```json
{
  "name": "docker_volume_create",
  "arguments": {
    "name": "data"
  }
}
```

### Compose up
```json
{
  "name": "docker_compose_up",
  "arguments": {
    "detach": true,
    "build": true
  }
}
```

### Clean up
```json
{
  "name": "docker_system_prune",
  "arguments": {
    "all": true,
    "force": true
  }
}
```

## Response Format

Every command returns:
```json
{
  "command": "docker ...",
  "success": true/false,
  "stdout": "...",
  "stderr": "...",
  "error": "..."
}
```

## Logs

View logs:
```bash
tail -f ~/.hunter3/logs/mcp-docker.log
```

## Testing

Test the plugin:
```bash
# Direct test
echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | ./dist/mcp-docker

# With Claude CLI
claude mcp test mcp-docker

# Run unit tests
go test ./cmd/mcp-docker
```

## Tips

1. **Always use `detach: true`** for long-running containers
2. **Use `force: true`** carefully - it can remove running containers
3. **Check logs** at `~/.hunter3/logs/mcp-docker.log` for debugging
4. **Use filters** with ps/images/volumes for cleaner output
5. **Use `flags` array** for advanced Docker features not explicitly supported

## Additional Flags

All tools support a `flags` array for passing additional Docker flags:

```json
{
  "name": "docker_run",
  "arguments": {
    "image": "nginx",
    "flags": ["--restart=always", "--cpus=2", "--memory=1g"]
  }
}
```

## Error Handling

If a command fails:
- `success` will be `false`
- `error` will contain the error message
- `stderr` will contain Docker's error output
- Check logs for full command that was executed

## Common Patterns

### Development Container
```json
{
  "name": "docker_run",
  "arguments": {
    "image": "node:18",
    "interactive": true,
    "tty": true,
    "remove": true,
    "volumes": ["./app:/app"],
    "workdir": "/app",
    "command": ["npm", "run", "dev"]
  }
}
```

### Database Container
```json
{
  "name": "docker_run",
  "arguments": {
    "image": "postgres:15",
    "detach": true,
    "name": "db",
    "env": ["POSTGRES_PASSWORD=secret"],
    "volumes": ["pgdata:/var/lib/postgresql/data"],
    "ports": ["5432:5432"]
  }
}
```

### Inspect Container IP
```json
{
  "name": "docker_inspect",
  "arguments": {
    "objects": ["web"],
    "format": "{{.NetworkSettings.IPAddress}}"
  }
}
```
