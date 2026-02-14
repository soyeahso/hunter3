output "instance_ip" {
  description = "Public IP of the Lightsail instance"
  value       = aws_lightsail_instance.hunter3.public_ip_address
}

output "instance_name" {
  description = "Name of the Lightsail instance"
  value       = aws_lightsail_instance.hunter3.name
}

output "ssh_command" {
  description = "SSH command to connect"
  value       = "ssh -i ${var.ssh_private_key_path} admin@${aws_lightsail_instance.hunter3.public_ip_address}"
}
