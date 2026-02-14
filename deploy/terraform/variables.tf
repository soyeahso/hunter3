variable "aws_region" {
  description = "AWS region for the Lightsail instance"
  type        = string
  default     = "us-east-1"
}

variable "instance_name" {
  description = "Name of the Lightsail instance"
  type        = string
  default     = "hunter3"
}

variable "blueprint_id" {
  description = "Lightsail OS blueprint (Debian 12)"
  type        = string
  default     = "debian_12"
}

variable "bundle_id" {
  description = "Lightsail instance size"
  type        = string
  default     = "small_3_0"
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key for instance access"
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}

variable "ssh_private_key_path" {
  description = "Path to SSH private key for Ansible"
  type        = string
  default     = "~/.ssh/id_ed25519"
}
