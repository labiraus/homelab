cluster_name = "homelab"

dns_servers = [
  "1.1.1.1",
  "8.8.8.8",
]

ntp_servers = [
  "time.cloudflare.com",
  "time.google.com",
]

kube_minor_channel                   = "v1.34"
kube_version                         = ""
kube_api_server_host                 = "192.168.8.132"
kube_api_server_port                 = 6443
kubeadm_discovery_token_ca_cert_hash = "69b8ddcb386dbaead6969961750e44f18ccbe8eee63f64e9357113b01fbb8e09"

ubuntu_image_url       = "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
ubuntu_image_file_name = "ubuntu-24.04-noble-server-cloudimg-amd64.img"
download_ubuntu_image  = false

enable_drain_on_destroy = true

proxmox_ve_insecure             = true
proxmox_ve_ssh_agent            = true
proxmox_ve_ssh_user             = "root"
proxmox_ve_ssh_private_key_path = ""
