cluster_name = "homelab"

dns_servers = [
  "1.1.1.1",
  "8.8.8.8",
]

ntp_servers = [
  "time.cloudflare.com",
  "time.google.com",
]

kube_minor_channel = "v1.31"
kube_version       = ""
kubeconfig_path    = "~/.kube/config"

ubuntu_image_url       = "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
ubuntu_image_file_name = "ubuntu-24.04-noble-server-cloudimg-amd64.img"

proxmox_ve_insecure         = false
proxmox_ve_ssh_agent        = true
proxmox_ve_ssh_user         = "root"
proxmox_ve_ssh_private_key_path = ""

