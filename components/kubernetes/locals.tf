locals {
  join_discovery_hash = trimsuffix(trimprefix(trimspace(var.kubeadm_discovery_token_ca_cert_hash), "sha256:"), "\n")
  computed_kubeadm_join_command = (
    trimspace(var.kube_api_server_host) != "" &&
    trimspace(var.kubeadm_join_token) != "" &&
    local.join_discovery_hash != ""
  ) ? "kubeadm join ${var.kube_api_server_host}:${var.kube_api_server_port} --token ${var.kubeadm_join_token} --discovery-token-ca-cert-hash sha256:${local.join_discovery_hash}" : ""
  effective_kubeadm_join_command = trimspace(var.kubeadm_join_command) != "" ? var.kubeadm_join_command : local.computed_kubeadm_join_command
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
    kubeadm_join_command = local.effective_kubeadm_join_command
    kubelet_extra_args   = local.kubelet_extra_args
  })
}
