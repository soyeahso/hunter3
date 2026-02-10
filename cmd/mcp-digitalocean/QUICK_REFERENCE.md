# MCP DigitalOcean Quick Reference

## Setup
```bash
export DIGITALOCEAN_TOKEN="your_api_token_here"
```

## Common Operations

### Droplets (VMs)

| Operation | Command | Parameters |
|-----------|---------|------------|
| List all droplets | `list_droplets` | `tag` (optional) |
| Get droplet details | `get_droplet` | `droplet_id` (required) |
| Create droplet | `create_droplet` | `name`, `region`, `size`, `image` (required)<br>`ssh_keys`, `backups`, `ipv6`, `monitoring`, `tags`, `user_data`, `vpc_uuid` (optional) |
| Delete droplet | `delete_droplet` | `droplet_id` (required) |
| Power on | `power_on_droplet` | `droplet_id` (required) |
| Power off | `power_off_droplet` | `droplet_id` (required) |
| Reboot | `reboot_droplet` | `droplet_id` (required) |
| Shutdown | `shutdown_droplet` | `droplet_id` (required) |
| Power cycle | `power_cycle_droplet` | `droplet_id` (required) |
| Resize | `resize_droplet` | `droplet_id`, `size` (required)<br>`disk` (optional) |
| Snapshot | `snapshot_droplet` | `droplet_id`, `snapshot_name` (required) |
| Get action status | `get_droplet_action` | `droplet_id`, `action_id` (required) |

### SSH Keys

| Operation | Command | Parameters |
|-----------|---------|------------|
| List SSH keys | `list_ssh_keys` | None |
| Create SSH key | `create_ssh_key` | `name`, `public_key` (required) |
| Delete SSH key | `delete_ssh_key` | `key_id` (required) |

### Discovery

| Operation | Command | Parameters |
|-----------|---------|------------|
| List regions | `list_regions` | None |
| List sizes | `list_sizes` | None |
| List images | `list_images` | `type` (optional: "distribution", "application") |

### Tags

| Operation | Command | Parameters |
|-----------|---------|------------|
| List tags | `list_tags` | None |
| Create tag | `create_tag` | `name` (required) |
| Delete tag | `delete_tag` | `name` (required) |
| Tag resources | `tag_resources` | `tag`, `resources` (required) |
| Untag resources | `untag_resources` | `tag`, `resources` (required) |

### Account

| Operation | Command | Parameters |
|-----------|---------|------------|
| Get account info | `get_account` | None |

## Quick Examples

### Create a Basic Ubuntu Server
```
create_droplet(
  name="my-server",
  region="nyc3",
  size="s-1vcpu-1gb",
  image="ubuntu-24-04-x64"
)
```

### Create with SSH Key and User Data
```
create_droplet(
  name="web-server",
  region="nyc3",
  size="s-1vcpu-2gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["12345"],
  tags=["web", "production"],
  user_data="#!/bin/bash\napt update && apt install -y nginx"
)
```

### List All Droplets with Tag
```
list_droplets(tag="production")
```

### Power Cycle and Check Status
```
# Start power cycle
power_cycle_droplet(droplet_id=12345)
# Returns action_id in response

# Check if complete
get_droplet_action(droplet_id=12345, action_id=67890)
```

### Backup Workflow
```
# Shutdown gracefully
shutdown_droplet(droplet_id=12345)

# Wait for shutdown, then snapshot
snapshot_droplet(droplet_id=12345, snapshot_name="backup-2024-01-15")

# Power back on
power_on_droplet(droplet_id=12345)
```

## Common Values

### Regions (US)
- `nyc1`, `nyc3` - New York
- `sfo2`, `sfo3` - San Francisco

### Regions (EU)
- `lon1` - London
- `ams3` - Amsterdam
- `fra1` - Frankfurt

### Regions (Asia)
- `sgp1` - Singapore
- `blr1` - Bangalore

### Popular Sizes
- `s-1vcpu-1gb` - $6/mo - 1 CPU, 1GB RAM, 25GB SSD
- `s-1vcpu-2gb` - $12/mo - 1 CPU, 2GB RAM, 50GB SSD
- `s-2vcpu-2gb` - $18/mo - 2 CPU, 2GB RAM, 60GB SSD
- `s-2vcpu-4gb` - $24/mo - 2 CPU, 4GB RAM, 80GB SSD

### Popular Images
- `ubuntu-24-04-x64` - Ubuntu 24.04 LTS (recommended)
- `ubuntu-22-04-x64` - Ubuntu 22.04 LTS
- `debian-12-x64` - Debian 12
- `fedora-40-x64` - Fedora 40
