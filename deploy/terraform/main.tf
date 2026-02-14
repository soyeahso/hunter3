terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

resource "aws_lightsail_key_pair" "hunter3" {
  name       = "${var.instance_name}-key"
  public_key = file(var.ssh_public_key_path)
}

resource "aws_lightsail_instance" "hunter3" {
  name              = var.instance_name
  availability_zone = "${var.aws_region}a"
  blueprint_id      = var.blueprint_id
  bundle_id         = var.bundle_id
  key_pair_name     = aws_lightsail_key_pair.hunter3.name

  tags = {
    Project = "hunter3"
  }
}

resource "aws_lightsail_instance_public_ports" "hunter3" {
  instance_name = aws_lightsail_instance.hunter3.name

  port_info {
    protocol  = "tcp"
    from_port = 22
    to_port   = 22
  }
}

resource "local_file" "ansible_inventory" {
  content = templatefile("${path.module}/inventory.tftpl", {
    ip   = aws_lightsail_instance.hunter3.public_ip_address
    user = "admin"
    key  = var.ssh_private_key_path
  })
  filename = "${path.module}/../ansible/inventory.ini"
}
