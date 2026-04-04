locals {
  cloud_init_user_data = templatefile("${path.module}/templates/user-data.yaml.tftpl", {
    vm_name             = var.proxmox.vm_name
    ssh_username        = try(var.vm.ssh_username, "ubuntu")
    ssh_authorized_keys = var.ssh_authorized_keys
    dns_servers         = var.dns_servers
    ntp_servers         = var.ntp_servers
  })
}
