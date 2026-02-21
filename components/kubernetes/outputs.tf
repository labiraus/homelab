output "vm_name" {
  description = "Provisioned VM name."
  value       = module.kubernetes_vm.vm_name
}

output "vm_id" {
  description = "Provisioned VM ID."
  value       = module.kubernetes_vm.vm_id
}

output "node_ip" {
  description = "Provisioned node IP address (null when DHCP)."
  value       = module.kubernetes_vm.node_ip
}

