output "vm_name" {
  description = "Provisioned VM name."
  value       = module.minecraft_vm.vm_name
}

output "vm_id" {
  description = "Provisioned VM ID."
  value       = module.minecraft_vm.vm_id
}

output "vm_ip" {
  description = "Provisioned VM IPv4 address (null when DHCP)."
  value       = module.minecraft_vm.node_ip
}
