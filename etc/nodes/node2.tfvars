proxmox = {
  node_name             = "proxmox-node2"
  vm_id                 = 3102
  vm_name               = "k8s-worker-2"
  bridge                = "vmbr0"
  datastore_id          = "local-lvm"
  image_datastore_id    = "local"
  snippets_datastore_id = "local"
}

vm = {
  cpu_cores    = 4
  memory_mb    = 8192
  disk_size_gb = 60
  ssh_username = "ubuntu"
}

network = {
  use_dhcp     = false
  ipv4_address = "192.168.8.122/24"
  ipv4_gateway = "192.168.8.1"
}

ssh_authorized_keys = [
  "ssh-ed25519 REPLACE_WITH_YOUR_PUBLIC_KEY homelab-node2",
]

kubeadm_join_command = ""

kubelet_node_labels = {
  "node-role.kubernetes.io/worker" = "true"
  "topology.kubernetes.io/zone"    = "lab-b"
}

kubelet_register_taints = []

