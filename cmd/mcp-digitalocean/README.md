# MCP DigitalOcean

Model Context Protocol (MCP) server for managing DigitalOcean Droplets (VMs) and resources.

## Features

- **Droplet Management**: Create, list, get, delete, power on/off, reboot, resize, snapshot
- **SSH Key Management**: List, create, and delete SSH keys
- **Resource Discovery**: List available regions, sizes, and images
- **Tagging**: Create tags and tag/untag resources
- **Account Info**: Get account information

## Setup

### 1. Get a DigitalOcean API Token

1. Go to [DigitalOcean API Tokens](https://cloud.digitalocean.com/account/api/tokens)
2. Click "Generate New Token"
3. Give it a name and select read/write permissions
4. Copy the token (you won't be able to see it again!)

### 2. Set Environment Variable

```bash
export DIGITALOCEAN_TOKEN="your_api_token_here"
```

Or add it to your shell configuration file (~/.bashrc, ~/.zshrc, etc.):

```bash
echo 'export DIGITALOCEAN_TOKEN="your_api_token_here"' >> ~/.bashrc
source ~/.bashrc
```

### 3. Build and Install

```bash
cd cmd/mcp-digitalocean
go build
```

## Usage

### List All Droplets

```
list_droplets
```

Optional parameters:
- `tag`: Filter by tag name

### Create a Droplet

```
create_droplet(name="my-server", region="nyc3", size="s-1vcpu-1gb", image="ubuntu-24-04-x64")
```

Required parameters:
- `name`: Name for the droplet
- `region`: Region slug (e.g., "nyc3", "sfo3", "lon1", "ams3")
- `size`: Size slug (e.g., "s-1vcpu-1gb", "s-2vcpu-2gb")
- `image`: Image slug (e.g., "ubuntu-24-04-x64", "debian-12-x64")

Optional parameters:
- `ssh_keys`: Array of SSH key IDs or fingerprints
- `backups`: Enable automated backups (boolean)
- `ipv6`: Enable IPv6 (boolean)
- `monitoring`: Enable monitoring (boolean)
- `tags`: Array of tags to apply
- `user_data`: Cloud-init script to run on first boot
- `vpc_uuid`: UUID of VPC to create the droplet in

### Get Droplet Details

```
get_droplet(droplet_id=12345)
```

### Delete a Droplet

```
delete_droplet(droplet_id=12345)
```

### Power Operations

```
power_on_droplet(droplet_id=12345)
power_off_droplet(droplet_id=12345)
reboot_droplet(droplet_id=12345)
shutdown_droplet(droplet_id=12345)
power_cycle_droplet(droplet_id=12345)
```

### Resize a Droplet

```
resize_droplet(droplet_id=12345, size="s-2vcpu-4gb", disk=false)
```

Parameters:
- `droplet_id`: The droplet ID
- `size`: New size slug
- `disk`: Resize disk (permanent, cannot be reversed)

### Create a Snapshot

```
snapshot_droplet(droplet_id=12345, snapshot_name="my-backup")
```

### Check Action Status

```
get_droplet_action(droplet_id=12345, action_id=67890)
```

### SSH Key Management

```
list_ssh_keys
create_ssh_key(name="my-key", public_key="ssh-rsa AAAAB3...")
delete_ssh_key(key_id="123456")
```

### List Available Resources

```
list_regions    # List all available regions
list_sizes      # List all available droplet sizes
list_images     # List all available images
list_images(type="distribution")  # Filter by type
```

### Tagging

```
list_tags
create_tag(name="production")
delete_tag(name="old-tag")
tag_resources(tag="production", resources=["do:droplet:12345", "do:droplet:67890"])
untag_resources(tag="production", resources=["do:droplet:12345"])
```

### Account Information

```
get_account
```

## Common Region Slugs

- `nyc1`, `nyc3` - New York
- `sfo2`, `sfo3` - San Francisco
- `lon1` - London
- `ams3` - Amsterdam
- `sgp1` - Singapore
- `fra1` - Frankfurt
- `tor1` - Toronto
- `blr1` - Bangalore

## Common Size Slugs

- `s-1vcpu-1gb` - 1 vCPU, 1GB RAM, 25GB SSD - $6/month
- `s-1vcpu-2gb` - 1 vCPU, 2GB RAM, 50GB SSD - $12/month
- `s-2vcpu-2gb` - 2 vCPU, 2GB RAM, 60GB SSD - $18/month
- `s-2vcpu-4gb` - 2 vCPU, 4GB RAM, 80GB SSD - $24/month
- `s-4vcpu-8gb` - 4 vCPU, 8GB RAM, 160GB SSD - $48/month

## Common Image Slugs

- `ubuntu-24-04-x64` - Ubuntu 24.04 LTS
- `ubuntu-22-04-x64` - Ubuntu 22.04 LTS
- `ubuntu-20-04-x64` - Ubuntu 20.04 LTS
- `debian-12-x64` - Debian 12
- `debian-11-x64` - Debian 11
- `fedora-40-x64` - Fedora 40
- `centos-stream-9-x64` - CentOS Stream 9
- `rocky-9-x64` - Rocky Linux 9

## Example Workflow

### 1. Create a web server

```bash
# First, upload your SSH key
create_ssh_key(name="laptop", public_key="ssh-rsa AAAAB3...")

# Create a droplet with the SSH key
create_droplet(
  name="web-server-1",
  region="nyc3",
  size="s-1vcpu-2gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["laptop"],
  tags=["web", "production"],
  user_data="#!/bin/bash\napt update && apt install -y nginx"
)

# Wait for it to be created, then get details
get_droplet(droplet_id=12345)

# SSH into your new server using the public IP from get_droplet
```

### 2. Manage server lifecycle

```bash
# Power off for maintenance
power_off_droplet(droplet_id=12345)

# Create a snapshot backup
snapshot_droplet(droplet_id=12345, snapshot_name="before-upgrade")

# Power back on
power_on_droplet(droplet_id=12345)

# Resize if needed
resize_droplet(droplet_id=12345, size="s-2vcpu-4gb")

# When done, delete
delete_droplet(droplet_id=12345)
```

## Troubleshooting

### "DIGITALOCEAN_TOKEN environment variable not set"

Make sure you've exported the token:
```bash
export DIGITALOCEAN_TOKEN="your_token_here"
```

### "Failed to create droplet"

Common issues:
- Invalid region/size/image combination (not all sizes are available in all regions)
- Invalid SSH key ID or fingerprint
- Quota limits reached (check your account limits)

### Check action status

Many operations return an action ID. You can check the status:
```bash
get_droplet_action(droplet_id=12345, action_id=67890)
```

## Security Notes

- **Never commit your API token to version control**
- API tokens have full account access - protect them like passwords
- Use SSH keys instead of passwords for droplet access
- Enable monitoring and backups for production droplets
- Use tags to organize resources and manage costs
- Consider using VPCs to isolate network traffic

## Resource URN Format

For tagging operations, use the format: `do:<resource_type>:<resource_id>`

Examples:
- `do:droplet:12345` - A droplet
- `do:volume:67890` - A volume
- `do:loadbalancer:abcde` - A load balancer
