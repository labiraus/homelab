locals {
  effective_kubelet_node_labels  = merge(var.kubelet_node_labels, { "node.mesh" = "enabled" })

  kubelet_extra_args_parts = concat(
    length(local.effective_kubelet_node_labels) > 0 ? [
      "--node-labels=${join(",", [for key, value in local.effective_kubelet_node_labels : "${key}=${value}"])}"
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
    kubelet_extra_args   = local.kubelet_extra_args
  })
}
