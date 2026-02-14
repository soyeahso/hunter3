# hunter3 Deployment

Terraform + Ansible deployment for hunter3 on AWS Lightsail with GitHub Copilot CLI.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.0
- [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/) >= 2.12
- AWS credentials configured (`aws configure` or env vars)
- SSH key pair (`~/.ssh/id_ed25519` by default)

## Quick Start

### 1. Provision the VPS

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your settings
terraform init
terraform plan
terraform apply
```

This creates a Lightsail instance and generates `ansible/inventory.ini`.

### 2. Install hunter3

```bash
cd ../ansible
ansible-playbook -i inventory.ini playbook.yml \
  -e github_token=ghp_YOUR_TOKEN \
  -e brave_api_key=YOUR_BRAVE_KEY \
  -e irc_server=irc.example.com \
  -e irc_port=6697 \
  -e irc_nick=mybot \
  -e 'irc_channels=["#general"]' \
  -e irc_use_tls=true
```

### 3. Connect

```bash
# Terraform prints the SSH command:
terraform -chdir=../terraform output ssh_command
```

## Customization

### Terraform Variables

| Variable | Default | Description |
|---|---|---|
| `aws_region` | `us-east-1` | AWS region |
| `instance_name` | `hunter3` | Lightsail instance name |
| `blueprint_id` | `debian_12` | OS blueprint |
| `bundle_id` | `small_3_0` | Instance size (2 GB RAM) |
| `ssh_public_key_path` | `~/.ssh/id_ed25519.pub` | SSH public key |
| `ssh_private_key_path` | `~/.ssh/id_ed25519` | SSH private key |

### Ansible Variables

| Variable | Default | Description |
|---|---|---|
| `hunter3_user` | `genoeg` | System user |
| `github_token` | (empty) | GitHub PAT for `gh auth` |
| `brave_api_key` | (empty) | Brave Search API key |
| `irc_server` | `irc.libera.chat` | IRC server |
| `irc_port` | `6667` | IRC port |
| `irc_nick` | `hunter3` | IRC nick |
| `irc_channels` | `["#hunter3"]` | IRC channels |
| `irc_use_tls` | `false` | TLS for IRC |
| `irc_password` | (empty) | IRC server password |
| `irc_sasl` | `false` | SASL authentication |

## What Gets Installed

1. **System packages** — git, gh, make, curl, etc.
2. **Go** — via goinstall.sh
3. **Node.js 22** — for Copilot CLI
4. **GitHub Copilot CLI** — via npm
5. **hunter3** — cloned, built with MCP servers
6. **Copilot MCP config** — generated at `~/.hunter3/mcp-servers.json`
7. **Systemd service** — `hunter3.service` enabled and started

## Teardown

```bash
cd terraform
terraform destroy
```
