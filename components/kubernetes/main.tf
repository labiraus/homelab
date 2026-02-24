module "kubernetes_vm" {
  source = "../../modules/proxmox-ubuntu-vm"

  node_name             = var.proxmox.node_name
  vm_id                 = var.proxmox.vm_id
  vm_name               = var.proxmox.vm_name
  bridge                = var.proxmox.bridge
  datastore_id          = var.proxmox.datastore_id
  image_datastore_id    = try(var.proxmox.image_datastore_id, null)
  snippets_datastore_id = try(var.proxmox.snippets_datastore_id, null)

  vm_cpu_cores    = var.vm.cpu_cores
  vm_memory_mb    = var.vm.memory_mb
  vm_disk_size_gb = var.vm.disk_size_gb

  use_dhcp             = var.network.use_dhcp
  ipv4_address         = try(var.network.ipv4_address, "")
  ipv4_gateway         = try(var.network.ipv4_gateway, "")
  ssh_username         = try(var.vm.ssh_username, "ubuntu")
  ssh_authorized_keys  = var.ssh_authorized_keys
  cloud_init_user_data = local.cloud_init_user_data

  ubuntu_image_url        = var.ubuntu_image_url
  ubuntu_image_file_name  = var.ubuntu_image_file_name
  download_ubuntu_image   = var.download_ubuntu_image
  enable_drain_on_destroy = var.enable_drain_on_destroy
  drain_command           = var.drain_command
  tags                    = ["kubernetes", "worker", var.cluster_name]
}
