locals {
  join_discovery_hash = trimsuffix(trimprefix(trimspace(var.kubeadm_discovery_token_ca_cert_hash), "sha256:"), "\n")
  computed_kubeadm_join_command = (
    trimspace(var.kube_api_server_host) != "" &&
    trimspace(var.kubeadm_join_token) != "" &&
    local.join_discovery_hash != ""
  ) ? "kubeadm join ${var.kube_api_server_host}:${var.kube_api_server_port} --token ${var.kubeadm_join_token} --discovery-token-ca-cert-hash sha256:${local.join_discovery_hash}" : ""
  effective_kubeadm_join_command = trimspace(var.kubeadm_join_command) != "" ? var.kubeadm_join_command : local.computed_kubeadm_join_command

  kubelet_extra_args_parts = concat(
    length(var.kubelet_node_labels) > 0 ? [
      "--node-labels=${join(",", [for key, value in var.kubelet_node_labels : "${key}=${value}"])}"
    ] : [],
    length(var.kubelet_register_taints) > 0 ? [
      "--register-with-taints=${join(",", var.kubelet_register_taints)}"
    ] : []
  )

  kubelet_extra_args = join(" ", local.kubelet_extra_args_parts)

  cloud_init_user_data = templatefile("${path.module}/templates/user-data.yaml.tftpl", {
    cluster_name         = var.cluster_name
    vm_name              = var.proxmox.vm_name
    ssh_username         = try(var.vm.ssh_username, "ubuntu")
    ssh_authorized_keys  = var.ssh_authorized_keys
    dns_servers          = var.dns_servers
    ntp_servers          = var.ntp_servers
    kube_minor_channel   = var.kube_minor_channel
    kube_version         = var.kube_version
    kubeadm_join_command = local.effective_kubeadm_join_command
    kubelet_extra_args   = local.kubelet_extra_args
  })
}

provider "proxmox" {
  endpoint  = var.proxmox_ve_endpoint != "" ? var.proxmox_ve_endpoint : null
  api_token = var.proxmox_ve_api_token != "" ? var.proxmox_ve_api_token : null
  insecure  = var.proxmox_ve_insecure

  ssh {
    agent       = var.proxmox_ve_ssh_agent
    username    = var.proxmox_ve_ssh_user
    private_key = var.proxmox_ve_ssh_private_key_path != "" ? file(var.proxmox_ve_ssh_private_key_path) : null
  }
}

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

  ubuntu_image_url       = var.ubuntu_image_url
  ubuntu_image_file_name = var.ubuntu_image_file_name
  download_ubuntu_image  = var.download_ubuntu_image
  enable_drain_on_destroy = var.enable_drain_on_destroy
  drain_command           = var.drain_command
  tags                   = ["kubernetes", "worker", var.cluster_name]
}
