proxmox = {
  node_name             = "proxmox-node3"
  vm_id                 = 3103
  vm_name               = "helheim"
  bridge                = "vmbr0"
  datastore_id          = "local-lvm"
  image_datastore_id    = "local"
  snippets_datastore_id = "local"
}

vm = {
  cpu_cores = 12
  # proxmox-node3 has about 15 GiB total RAM; keep host headroom for Proxmox,
  # QEMU, and passthrough overhead instead of sizing the guest to 100%.
  memory_mb = 12288
  # local-lvm currently lives on the 477 GiB NVMe. The host also has a 1 TB SATA
  # disk, but it is not yet configured as a Proxmox datastore, so the current
  # Terraform VM model can only consume the NVMe-backed storage pool.
  disk_size_gb = 320
  ssh_username = "ubuntu"
  bios         = "ovmf"
  machine      = "q35"
  efi_disk = {
    datastore_id = "local-lvm"
    type         = "4m"
  }
  # These mappings must exist in Proxmox before apply.
  hostpci_devices = [
    {
      device  = "hostpci0"
      mapping = "node3-intel-igpu"
      pcie    = true
      rombar  = true
      xvga    = false
    },
    {
      device  = "hostpci1"
      mapping = "node3-rtx2070-gpu"
      pcie    = true
      rombar  = true
      xvga    = false
    },
    {
      device  = "hostpci2"
      mapping = "node3-rtx2070-audio"
      pcie    = true
      rombar  = true
      xvga    = false
    },
    {
      device  = "hostpci3"
      mapping = "node3-rtx2070-usb"
      pcie    = true
      rombar  = true
      xvga    = false
    },
    {
      device  = "hostpci4"
      mapping = "node3-rtx2070-ucsi"
      pcie    = true
      rombar  = true
      xvga    = false
    },
  ]
}

network = {
  use_dhcp     = false
  ipv4_address = "192.168.8.123/24"
  ipv4_gateway = "192.168.8.1"
}

ssh_authorized_keys = [
  "cert-authority,principals=\"ubuntu\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAD/I8jzz4PjePbJRSS/Wh4ID1rfLUSgG7lkPN4H2E10 homelab-user-ca",
  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINkxqdUSQl+Nx1NBr1+YDXeXDnzzNQAD7CTrcTJX/PB9 homelab-login",
]

# Keep the RTX worker on an explicit NVIDIA driver family from first boot so
# rebuilt guests come up with matching kernel-module metapackages before the
# Ansible GPU runtime wiring runs.
cloud_init_packages = [
  "ubuntu-drivers-common",
  "nvidia-driver-580-open",
  "linux-modules-nvidia-580-open-generic",
]

kubeadm_join_command = ""

kubelet_node_labels = {
  "topology.kubernetes.io/zone" = "lab-c"
  "node-performance"            = "standard"
  "node-gpu"                    = "passthrough"
  "node-llm"                    = "gpu"
  "node-llm-class"              = "consumer-gpu"
  "node-llm-vram"               = "8gb"
}

kubelet_register_taints = []
