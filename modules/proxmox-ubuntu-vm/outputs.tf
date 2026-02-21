output "vm_name" {
  description = "VM name."
  value       = proxmox_virtual_environment_vm.this.name
}

output "vm_id" {
  description = "VM ID."
  value       = proxmox_virtual_environment_vm.this.vm_id
}

output "node_ip" {
  description = "Configured node IPv4 address. Null when DHCP is used."
  value       = var.use_dhcp ? null : split("/", var.ipv4_address)[0]
}

