
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
