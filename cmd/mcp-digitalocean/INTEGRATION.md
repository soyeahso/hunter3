# MCP DigitalOcean Integration Guide

## Prerequisites

1. **DigitalOcean Account** - Sign up at https://www.digitalocean.com/
2. **API Token** - Generate at https://cloud.digitalocean.com/account/api/tokens
3. **Go 1.25+** - For building the plugin

## Installation

### 1. Build the Plugin

From the hunter3 root directory:

```bash
make mcp-digitalocean
```

Or build all MCP plugins:

```bash
make mcp-all
```

The binary will be created at `dist/mcp-digitalocean`.

### 2. Set Environment Variable

Add your DigitalOcean API token to your shell configuration:

**For Bash (~/.bashrc):**
```bash
echo 'export DIGITALOCEAN_TOKEN="dop_v1_xxxxxxxxxxxxxxxxxxxxx"' >> ~/.bashrc
source ~/.bashrc
```

**For Zsh (~/.zshrc):**
```bash
echo 'export DIGITALOCEAN_TOKEN="dop_v1_xxxxxxxxxxxxxxxxxxxxx"' >> ~/.zshrc
source ~/.zshrc
```

**For Fish (~/.config/fish/config.fish):**
```fish
echo 'set -x DIGITALOCEAN_TOKEN "dop_v1_xxxxxxxxxxxxxxxxxxxxx"' >> ~/.config/fish/config.fish
source ~/.config/fish/config.fish
```

### 3. Register with Claude CLI (Optional)

If using with Claude Desktop or CLI:

```bash
make mcp-register
```

Or manually:

```bash
claude mcp add --transport stdio mcp-digitalocean -- $(pwd)/dist/mcp-digitalocean
```

### 4. Verify Installation

Test the plugin:

```bash
# List available regions (should work if token is valid)
./dist/mcp-digitalocean
# Then send JSON-RPC commands via stdin
```

Or if integrated with Claude:

```
list_regions
```

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DIGITALOCEAN_TOKEN` | Yes | Your DigitalOcean API token |

### Token Permissions

Your API token needs **read and write** permissions to manage resources. When creating the token:

- ✅ Enable "Read" scope
- ✅ Enable "Write" scope
- Set an appropriate expiration (or no expiration for long-term use)

## Usage Examples

### Basic Workflow

```bash
# 1. Check available regions
list_regions

# 2. Check available sizes
list_sizes

# 3. Upload SSH key for access
create_ssh_key(
  name="my-laptop",
  public_key="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC..."
)

# 4. Create a droplet
create_droplet(
  name="test-server",
  region="nyc3",
  size="s-1vcpu-1gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["my-laptop"]
)
# Returns droplet info with ID and IP address

# 5. Get droplet details
get_droplet(droplet_id=12345)

# 6. SSH into the server
# ssh root@<ip_from_step_5>

# 7. When done, delete the droplet
delete_droplet(droplet_id=12345)
```

### Advanced Usage

#### Create Tagged Production Server

```bash
create_droplet(
  name="web-prod-1",
  region="nyc3",
  size="s-2vcpu-4gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["12345"],
  tags=["production", "web", "api"],
  monitoring=true,
  backups=true,
  ipv6=true
)
```

#### Create Server with Cloud-Init

```bash
create_droplet(
  name="nginx-server",
  region="sfo3",
  size="s-1vcpu-2gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["my-key"],
  user_data="#!/bin/bash
apt-get update
apt-get install -y nginx
systemctl enable nginx
systemctl start nginx
"
)
```

#### Backup Workflow

```bash
# 1. Graceful shutdown
shutdown_droplet(droplet_id=12345)

# 2. Wait for shutdown (check action status)
get_droplet_action(droplet_id=12345, action_id=67890)

# 3. Create snapshot
snapshot_droplet(droplet_id=12345, snapshot_name="pre-upgrade-2024-02-09")

# 4. Power back on
power_on_droplet(droplet_id=12345)
```

#### Resize Server

```bash
# 1. Power off first (recommended)
power_off_droplet(droplet_id=12345)

# 2. Resize (without disk resize for reversibility)
resize_droplet(droplet_id=12345, size="s-2vcpu-4gb", disk=false)

# 3. Power back on
power_on_droplet(droplet_id=12345)
```

#### List Tagged Resources

```bash
# List all production droplets
list_droplets(tag="production")
```

## Troubleshooting

### "DIGITALOCEAN_TOKEN environment variable not set"

**Problem:** Token not exported in current shell.

**Solution:**
```bash
export DIGITALOCEAN_TOKEN="your_token_here"
# Or restart your terminal after adding to ~/.bashrc
```

### "401 Unauthorized"

**Problem:** Invalid or expired token.

**Solutions:**
1. Generate a new token at https://cloud.digitalocean.com/account/api/tokens
2. Ensure token has read/write permissions
3. Check for typos in the token string

### "Failed to create droplet: resource not found"

**Problem:** Invalid region, size, or image combination.

**Solutions:**
1. Run `list_regions` to see available regions
2. Run `list_sizes` to see available sizes
3. Run `list_images` to see available images
4. Verify the combination is valid (not all sizes available in all regions)

### "SSH key not found"

**Problem:** Invalid SSH key ID or fingerprint.

**Solutions:**
1. Run `list_ssh_keys` to see your keys
2. Use the exact key ID or fingerprint
3. Create a new key with `create_ssh_key` if needed

### "Quota exceeded"

**Problem:** You've hit your account limits.

**Solutions:**
1. Delete unused droplets: `delete_droplet(droplet_id=...)`
2. Check your account limits in the DigitalOcean dashboard
3. Contact DigitalOcean support to increase limits

### Check Action Status

Many operations are asynchronous and return an action ID. Check completion:

```bash
get_droplet_action(droplet_id=12345, action_id=67890)
# Status will be "in-progress" or "completed"
```

## Best Practices

### Security

1. **Never commit tokens** - Keep `DIGITALOCEAN_TOKEN` out of version control
2. **Use SSH keys** - Always add SSH keys to droplets instead of passwords
3. **Rotate tokens** - Periodically regenerate your API token
4. **Least privilege** - Create separate tokens for different purposes
5. **Enable monitoring** - Use `monitoring=true` for production droplets

### Cost Management

1. **Delete unused droplets** - Droplets charge by the hour while running
2. **Power off when not needed** - Still charges for reserved resources, but less
3. **Use tags** - Tag resources by project for cost tracking
4. **Snapshots cost money** - Delete old snapshots you don't need
5. **Right-size** - Start small and resize up as needed

### Reliability

1. **Enable backups** - Set `backups=true` for production (20% extra cost)
2. **Use monitoring** - Set `monitoring=true` for alerts
3. **Multiple regions** - Distribute across regions for redundancy
4. **Snapshot before changes** - Always snapshot before major upgrades
5. **Test in development** - Create test droplets before production changes

### Organization

1. **Naming convention** - Use consistent names (e.g., `web-prod-1`, `db-dev-1`)
2. **Tags everywhere** - Tag by environment, project, purpose
3. **Document snapshots** - Use descriptive snapshot names with dates
4. **Resource cleanup** - Regularly audit and remove unused resources

## API Rate Limits

DigitalOcean enforces rate limits:
- **5,000 requests per hour** per token
- Some endpoints have lower limits

The plugin handles pagination automatically and will respect rate limits.

## Cost Reference

### Droplet Pricing (Monthly/Hourly)

| Size Slug | vCPUs | RAM | Disk | Transfer | Monthly | Hourly |
|-----------|-------|-----|------|----------|---------|--------|
| s-1vcpu-1gb | 1 | 1GB | 25GB | 1TB | $6 | $0.009 |
| s-1vcpu-2gb | 1 | 2GB | 50GB | 2TB | $12 | $0.018 |
| s-2vcpu-2gb | 2 | 2GB | 60GB | 3TB | $18 | $0.027 |
| s-2vcpu-4gb | 2 | 4GB | 80GB | 4TB | $24 | $0.036 |
| s-4vcpu-8gb | 4 | 8GB | 160GB | 5TB | $48 | $0.071 |

### Additional Costs

- **Backups**: +20% of droplet cost (automated weekly backups)
- **Snapshots**: $0.05/GB per month (manual backups)
- **Monitoring**: Free (basic metrics included)
- **Bandwidth**: First 1TB free, then $0.01/GB

**Always check current pricing at:** https://www.digitalocean.com/pricing

## Support

- **DigitalOcean API Docs**: https://docs.digitalocean.com/reference/api/
- **Community Tutorials**: https://www.digitalocean.com/community/tutorials
- **Support Tickets**: https://cloud.digitalocean.com/support/tickets

## Next Steps

1. ✅ Build and install the plugin
2. ✅ Set your API token
3. ✅ Test with `list_regions`
4. Create your first droplet
5. Add SSH keys for secure access
6. Explore tagging and organization
7. Set up backup workflows
8. Automate infrastructure management

Happy cloud computing! 🚀
