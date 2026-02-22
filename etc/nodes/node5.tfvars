proxmox = {
  node_name             = "proxmox-node5"
  vm_id                 = 3105
  vm_name               = "musphelheim"
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
  ipv4_address = "192.168.8.125/24"
  ipv4_gateway = "192.168.8.1"
}

ssh_authorized_keys = [
  "cert-authority,principals=\"ubuntu\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAD/I8jzz4PjePbJRSS/Wh4ID1rfLUSgG7lkPN4H2E10 homelab-user-ca",
]

kubeadm_join_command = ""

kubelet_node_labels = {
  "node-role.kubernetes.io/worker" = "true"
  "topology.kubernetes.io/zone"    = "lab-e"
}

kubelet_register_taints = []

