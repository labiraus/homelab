proxmox = {
  node_name             = "proxmox-node1"
  vm_id                 = 3101
  vm_name               = "jotunheim"
  bridge                = "vmbr0"
  datastore_id          = "local-lvm"
  image_datastore_id    = "local"
  snippets_datastore_id = "local"
}

vm = {
  cpu_cores    = 12
  memory_mb    = 30064
  disk_size_gb = 280
  ssh_username = "ubuntu"
  bios         = "ovmf"
  machine      = "q35"
  efi_disk = {
    datastore_id = "local-lvm"
    type         = "4m"
  }
  hostpci_devices = [
    {
      device  = "hostpci0"
      mapping = "intel-igpu"
      pcie    = true
      rombar  = true
      xvga    = false
    },
  ]
}

network = {
  use_dhcp     = false
  ipv4_address = "192.168.8.121/24"
  ipv4_gateway = "192.168.8.1"
}

ssh_authorized_keys = [
  "cert-authority,principals=\"ubuntu\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAD/I8jzz4PjePbJRSS/Wh4ID1rfLUSgG7lkPN4H2E10 homelab-user-ca",
  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINkxqdUSQl+Nx1NBr1+YDXeXDnzzNQAD7CTrcTJX/PB9 homelab-login",
]

kubeadm_join_command = ""

kubelet_node_labels = {
  "topology.kubernetes.io/zone" = "lab-a"
  "node-performance"            = "high"
  "node-gpu"                    = "passthrough"
  "node-llm"                    = "none"
  "node-llm-class"              = "igpu"
  "node-llm-vram"               = "shared"
}

kubelet_register_taints = []
