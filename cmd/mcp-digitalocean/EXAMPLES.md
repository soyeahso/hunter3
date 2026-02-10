# MCP DigitalOcean Examples

## Complete Workflows

### Example 1: Quick Development Server

Create a disposable Ubuntu server for testing:

```bash
# 1. List available regions to find the closest one
list_regions
# Pick a region close to you (e.g., "nyc3" for New York)

# 2. Create a minimal server
create_droplet(
  name="quick-test",
  region="nyc3",
  size="s-1vcpu-1gb",
  image="ubuntu-24-04-x64"
)

# Response includes droplet ID and IP address
# Example response:
# {
#   "id": 123456789,
#   "name": "quick-test",
#   "status": "new",
#   "networks": {
#     "v4": [{"ip_address": "192.0.2.1", "type": "public"}]
#   }
# }

# 3. Get the IP address
get_droplet(droplet_id=123456789)

# 4. SSH in (password will be emailed to your DO account email)
# ssh root@192.0.2.1

# 5. When done testing, delete it
delete_droplet(droplet_id=123456789)
```

**Cost:** ~$0.009/hour, ~$0.07 for 8 hours of testing

---

### Example 2: Production Web Server with SSH Key

Create a production-ready web server:

```bash
# 1. First, upload your SSH public key
create_ssh_key(
  name="laptop-2024",
  public_key="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... user@laptop"
)
# Response: {"id": 12345678, "fingerprint": "aa:bb:cc:..."}

# 2. Create the server with your SSH key
create_droplet(
  name="web-prod-1",
  region="nyc3",
  size="s-2vcpu-4gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["12345678"],
  tags=["production", "web"],
  monitoring=true,
  backups=true,
  ipv6=true
)

# 3. Wait for it to be ready (~60 seconds)
get_droplet(droplet_id=987654321)
# Status changes: "new" -> "active"

# 4. SSH in with your key (no password needed!)
# ssh root@<ip_address>

# 5. Install your app, configure firewall, etc.
```

**Cost:** $24/month + $4.80/month (backups) = $28.80/month

---

### Example 3: Docker Host with Cloud-Init

Create a server with Docker pre-installed:

```bash
create_droplet(
  name="docker-host",
  region="sfo3",
  size="s-2vcpu-2gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["your-key-id"],
  tags=["docker", "development"],
  user_data="#!/bin/bash
# Update system
apt-get update
apt-get upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh

# Install Docker Compose
apt-get install -y docker-compose-plugin

# Create a non-root docker user
useradd -m -s /bin/bash dockeruser
usermod -aG docker dockeruser

# Auto-update enabled
apt-get install -y unattended-upgrades
"
)

# Server will be ready with Docker installed in ~2-3 minutes
```

**Cost:** $18/month

---

### Example 4: Batch Server Creation

Create multiple servers for load testing:

```bash
# Create 5 web servers
for i in 1 2 3 4 5; do
  create_droplet(
    name="loadtest-${i}",
    region="nyc3",
    size="s-1vcpu-1gb",
    image="ubuntu-24-04-x64",
    tags=["loadtest", "temporary"]
  )
done

# Later, list all loadtest servers
list_droplets(tag="loadtest")

# Clean up all at once
# (You'd delete each by ID from the list)
delete_droplet(droplet_id=111)
delete_droplet(droplet_id=112)
delete_droplet(droplet_id=113)
delete_droplet(droplet_id=114)
delete_droplet(droplet_id=115)
```

**Cost:** $0.045/hour for all 5 = ~$1.08 for 24 hours of testing

---

### Example 5: Database Server with Snapshot Backup

Create a database server with regular backups:

```bash
# 1. Create database server
create_droplet(
  name="postgres-prod",
  region="nyc3",
  size="s-4vcpu-8gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["your-key"],
  tags=["database", "production"],
  monitoring=true,
  backups=true
)

# 2. After setting up PostgreSQL, create a snapshot
# First, shutdown gracefully
shutdown_droplet(droplet_id=555)

# 3. Wait for shutdown (check every few seconds)
get_droplet_action(droplet_id=555, action_id=777)

# 4. Create snapshot
snapshot_droplet(
  droplet_id=555,
  snapshot_name="postgres-prod-2024-02-09-initial"
)

# 5. Power back on
power_on_droplet(droplet_id=555)

# Later, create regular snapshots (weekly)
shutdown_droplet(droplet_id=555)
snapshot_droplet(
  droplet_id=555,
  snapshot_name="postgres-prod-2024-02-16-weekly"
)
power_on_droplet(droplet_id=555)
```

**Cost:** 
- Server: $48/month
- Backups: $9.60/month (automated weekly)
- Snapshots: ~$0.40/month (8GB disk × $0.05/GB)
- Total: ~$58/month

---

### Example 6: Multi-Region Deployment

Deploy across multiple regions for redundancy:

```bash
# Upload SSH key once
create_ssh_key(
  name="deploy-key",
  public_key="ssh-rsa AAAAB..."
)

# Create in New York
create_droplet(
  name="api-nyc",
  region="nyc3",
  size="s-1vcpu-2gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["deploy-key"],
  tags=["api", "production", "us-east"]
)

# Create in San Francisco
create_droplet(
  name="api-sfo",
  region="sfo3",
  size="s-1vcpu-2gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["deploy-key"],
  tags=["api", "production", "us-west"]
)

# Create in London
create_droplet(
  name="api-lon",
  region="lon1",
  size="s-1vcpu-2gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["deploy-key"],
  tags=["api", "production", "eu-west"]
)

# List all API servers
list_droplets(tag="api")
```

**Cost:** 3 × $12/month = $36/month total

---

### Example 7: Resize Workflow

Upgrade a server that's outgrown its resources:

```bash
# 1. Check current droplet info
get_droplet(droplet_id=999)
# Current: s-1vcpu-2gb

# 2. Power off (recommended for resize)
power_off_droplet(droplet_id=999)

# 3. Wait for power off
get_droplet_action(droplet_id=999, action_id=1111)

# 4. Resize WITHOUT disk resize (reversible)
resize_droplet(
  droplet_id=999,
  size="s-2vcpu-4gb",
  disk=false
)

# 5. Wait for resize to complete
get_droplet_action(droplet_id=999, action_id=2222)

# 6. Power back on
power_on_droplet(droplet_id=999)

# Note: To resize WITH disk (permanent):
# resize_droplet(droplet_id=999, size="s-2vcpu-4gb", disk=true)
```

---

### Example 8: Development Environment Setup

Create a complete development environment:

```bash
# Create development server
create_droplet(
  name="dev-env",
  region="nyc3",
  size="s-2vcpu-4gb",
  image="ubuntu-24-04-x64",
  ssh_keys=["dev-key"],
  tags=["development", "personal"],
  user_data="#!/bin/bash
# Development tools
apt-get update
apt-get install -y \
  git \
  vim \
  curl \
  wget \
  build-essential \
  python3-pip \
  nodejs \
  npm

# Docker
curl -fsSL https://get.docker.com | sh

# Go
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc

# VS Code Server
curl -fsSL https://code-server.dev/install.sh | sh
systemctl enable --now code-server@root
"
)

# After ~5 minutes, server is ready with:
# - Git, Docker, Go, Node.js
# - VS Code Server (access via browser)
```

**Cost:** $24/month (power off when not coding to save)

---

### Example 9: Temporary Testing with Auto-Cleanup

Create a server for quick testing, then clean up:

```bash
# Morning: Create test server
create_droplet(
  name="temp-test-2024-02-09",
  region="nyc3",
  size="s-1vcpu-1gb",
  image="ubuntu-24-04-x64",
  tags=["temporary", "testing"]
)

# Do your testing...

# Evening: Clean up
# List temporary servers
list_droplets(tag="temporary")

# Delete each one
delete_droplet(droplet_id=...)

# Cost for 8 hours: ~$0.07
```

---

### Example 10: Monitoring Setup

Check all your servers and their status:

```bash
# 1. List all droplets
list_droplets

# 2. Check a specific server's details
get_droplet(droplet_id=123)
# Shows: status, IP addresses, resources, tags

# 3. List by environment
list_droplets(tag="production")
list_droplets(tag="development")

# 4. Check your account info
get_account
# Shows: email, droplet limit, status

# 5. List all available images (including your snapshots)
list_images
# Your snapshots will have type="snapshot"
```

---

## Common Tasks

### Add SSH Key to Existing Account

```bash
# Read your public key
# cat ~/.ssh/id_rsa.pub

create_ssh_key(
  name="new-laptop-2024",
  public_key="ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ..."
)
```

### List All SSH Keys

```bash
list_ssh_keys
# Shows all your uploaded keys with IDs and fingerprints
```

### Power Cycle Frozen Server

```bash
# Hard reset (like pressing the power button)
power_cycle_droplet(droplet_id=123)

# Check action status
get_droplet_action(droplet_id=123, action_id=456)
```

### Tag Existing Resources

```bash
# Create tag first
create_tag(name="migrate-to-v2")

# Tag resources
tag_resources(
  tag="migrate-to-v2",
  resources=["do:droplet:111", "do:droplet:222"]
)
```

### Remove Tag from Resources

```bash
untag_resources(
  tag="old-version",
  resources=["do:droplet:111"]
)
```

### Check Available Regions and Sizes

```bash
# See all regions (shows availability and features)
list_regions

# See all sizes (shows pricing and specs)
list_sizes

# See OS images
list_images(type="distribution")

# See application images (e.g., WordPress, Docker)
list_images(type="application")
```

---

## Tips & Tricks

### Cost Optimization

1. **Development servers**: Power off when not using
   ```bash
   power_off_droplet(droplet_id=123)  # Stops billing for CPU/RAM
   ```

2. **Use snapshots wisely**: Delete old snapshots
   ```bash
   # Snapshots cost $0.05/GB/month
   # A 50GB snapshot = $2.50/month
   ```

3. **Right-size**: Start small, resize up if needed
   ```bash
   # Start: s-1vcpu-1gb ($6/mo)
   # Resize when needed: s-2vcpu-2gb ($18/mo)
   ```

### Security

1. **Always use SSH keys**:
   ```bash
   create_droplet(..., ssh_keys=["key-id"])
   # Never rely on emailed passwords
   ```

2. **Enable monitoring**:
   ```bash
   create_droplet(..., monitoring=true)
   # Free, provides valuable insights
   ```

3. **Use tags for access control**:
   ```bash
   # Tag by sensitivity level
   tags=["production", "pci-compliant", "high-security"]
   ```

### Organization

1. **Consistent naming**:
   ```bash
   # Pattern: <service>-<env>-<number>
   name="web-prod-1"
   name="db-staging-2"
   name="api-dev-1"
   ```

2. **Tag everything**:
   ```bash
   tags=["environment:production", "team:backend", "project:api"]
   ```

3. **Snapshot naming with dates**:
   ```bash
   snapshot_name="web-prod-1-2024-02-09-pre-deploy"
   ```

---

## Error Handling

### Check if operation completed

```bash
# Many operations return an action ID
power_on_droplet(droplet_id=123)
# Response: {"action": {"id": 456, "status": "in-progress"}}

# Check status
get_droplet_action(droplet_id=123, action_id=456)
# Response: {"status": "completed"} or {"status": "in-progress"}
```

### Retry failed operations

```bash
# If create fails, check why:
get_account  # Check limits
list_regions # Verify region exists
list_sizes   # Verify size exists

# Try again with valid parameters
```

---

These examples cover the most common use cases. Mix and match as needed for your specific infrastructure needs!
