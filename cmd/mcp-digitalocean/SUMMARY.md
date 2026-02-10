# MCP DigitalOcean - Summary

**MCP server for managing DigitalOcean infrastructure via the DigitalOcean API.**

## What it does

This MCP plugin provides comprehensive control over DigitalOcean resources, with a primary focus on Droplet (VM) management. It uses the official DigitalOcean Go client library (`godo`) to interact with the DigitalOcean API.

## Key Features

### Droplet Management (Core)
- **Create** - Spin up new VMs with custom configurations
- **List** - View all droplets or filter by tag
- **Get** - Detailed information about specific droplets
- **Delete** - Tear down droplets
- **Power Control** - Power on/off, reboot, shutdown, power cycle
- **Resize** - Change droplet size (CPU/RAM/disk)
- **Snapshot** - Create backup snapshots
- **Action Tracking** - Monitor asynchronous operations

### SSH Key Management
- List, create, and delete SSH keys
- Use keys when creating droplets for secure access

### Resource Discovery
- List available regions (datacenters)
- List available sizes (VM configurations)
- List available images (OS distributions)

### Tagging & Organization
- Create and manage tags
- Tag/untag resources for organization
- Filter droplets by tags

### Account
- Get account information and details

## Authentication

Requires a DigitalOcean API token set via environment variable:
```bash
export DIGITALOCEAN_TOKEN="your_api_token_here"
```

Get a token from: https://cloud.digitalocean.com/account/api/tokens

## Common Use Cases

1. **Quick Development Server** - Spin up a VM in seconds for testing
2. **Batch Operations** - Create/manage multiple servers with tags
3. **Backup Workflows** - Snapshot droplets before major changes
4. **Resource Management** - List and clean up unused resources
5. **Infrastructure Automation** - Programmatically manage cloud infrastructure

## Example: Create and SSH into a Server

```bash
# 1. Upload SSH key
create_ssh_key(name="my-laptop", public_key="ssh-rsa AAAAB3...")

# 2. Create droplet
create_droplet(
  name="dev-server",
  region="nyc3",
  size="s-1vcpu-1gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["my-laptop"]
)

# 3. Get droplet details (including IP)
get_droplet(droplet_id=12345)

# 4. SSH in
# ssh root@<ip_address_from_step_3>
```

## Tool Categories

- **9 Droplet Operations** - Full lifecycle management
- **3 SSH Key Operations** - Key management
- **3 Discovery Tools** - Regions, sizes, images
- **4 Tagging Tools** - Organization and filtering
- **1 Account Tool** - Account information

Total: **20 tools**

## Technology

- **Language**: Go
- **Client Library**: `github.com/digitalocean/godo` (official DO client)
- **Protocol**: MCP (Model Context Protocol)
- **API**: DigitalOcean REST API v2

## Safety Features

- Read-only operations (list, get) are safe
- Destructive operations (delete, power_off) require explicit droplet_id
- Actions return action IDs for tracking completion
- All API calls are logged

## Pricing Note

DigitalOcean charges by the hour while droplets are running. Remember to delete or power off droplets when not in use to avoid charges!

Smallest droplet: $6/month ($0.009/hour) - 1 vCPU, 1GB RAM, 25GB SSD
