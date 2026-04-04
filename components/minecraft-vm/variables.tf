variable "cluster_name" {
  description = "Logical environment name used for shared tagging compatibility."
  type        = string
  default     = "homelab"
}

variable "dns_servers" {
  description = "DNS servers configured via cloud-init."
  type        = list(string)
  default     = []
}

variable "ntp_servers" {
  description = "NTP servers configured via cloud-init."
  type        = list(string)
  default     = []
}

variable "proxmox" {
  description = "Node-level Proxmox settings."
  type = object({
    node_name             = string
    vm_id                 = number
    vm_name               = string
    bridge                = string
    datastore_id          = string
    image_datastore_id    = optional(string)
    snippets_datastore_id = optional(string)
  })
}

variable "vm" {
  description = "VM sizing and guest settings."
  type = object({
    cpu_cores    = number
    memory_mb    = number
    disk_size_gb = number
    ssh_username = optional(string, "ubuntu")
    bios         = optional(string, "seabios")
    machine      = optional(string)
    efi_disk = optional(object({
      datastore_id      = optional(string)
      type              = optional(string, "4m")
      pre_enrolled_keys = optional(bool, false)
    }))
    hostpci_devices = optional(list(object({
      device   = string
      mapping  = optional(string)
      id       = optional(string)
      mdev     = optional(string)
      pcie     = optional(bool, true)
      rombar   = optional(bool, true)
      rom_file = optional(string)
      xvga     = optional(bool, false)
    })), [])
  })
}

variable "network" {
  description = "VM network config."
  type = object({
    use_dhcp     = bool
    ipv4_address = optional(string, "")
    ipv4_gateway = optional(string, "")
  })

  validation {
    condition     = var.network.use_dhcp || (try(var.network.ipv4_address, "") != "" && try(var.network.ipv4_gateway, "") != "")
    error_message = "When network.use_dhcp is false, network.ipv4_address and network.ipv4_gateway are required."
  }
}

variable "ssh_authorized_keys" {
  description = "SSH public keys for guest access."
  type        = list(string)

  validation {
    condition     = length(var.ssh_authorized_keys) > 0
    error_message = "At least one SSH public key is required."
  }
}

variable "ubuntu_image_url" {
  description = "Ubuntu cloud image URL."
  type        = string
  default     = "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
}

variable "ubuntu_image_file_name" {
  description = "Ubuntu cloud image filename in Proxmox datastore."
  type        = string
  default     = "ubuntu-24.04-noble-server-cloudimg-amd64.qcow2"
}

variable "download_ubuntu_image" {
  description = "Download Ubuntu cloud image to Proxmox. Set false to reuse an existing image file in datastore."
  type        = bool
  default     = true
}

variable "enable_drain_on_destroy" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = bool
  default     = false
}

variable "drain_command" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = string
  default     = ""
}

variable "kube_minor_channel" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = string
  default     = ""
}

variable "kube_version" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = string
  default     = ""
}

variable "kubeadm_join_command" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = string
  default     = ""
  sensitive   = true
}

variable "kubeadm_join_token" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = string
  default     = ""
  sensitive   = true
}

variable "kubeadm_discovery_token_ca_cert_hash" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = string
  default     = ""
}

variable "kube_api_server_host" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = string
  default     = ""
}

variable "kube_api_server_port" {
  description = "Compatibility input from shared env tfvars. Unused by minecraft-vm."
  type        = number
  default     = 6443
}

variable "proxmox_ve_endpoint" {
  description = "Proxmox API endpoint. Prefer env var PROXMOX_VE_ENDPOINT."
  type        = string
  default     = ""
}

variable "proxmox_ve_api_token" {
  description = "Proxmox API token. Prefer env var PROXMOX_VE_API_TOKEN."
  type        = string
  default     = ""
  sensitive   = true
}

variable "proxmox_ve_insecure" {
  description = "Disable TLS verification for Proxmox endpoint."
  type        = bool
  default     = false
}

variable "proxmox_ve_ssh_agent" {
  description = "Use SSH agent when provider uploads snippets to Proxmox."
  type        = bool
  default     = true
}

variable "proxmox_ve_ssh_user" {
  description = "SSH user for Proxmox file uploads."
  type        = string
  default     = "root"
}

variable "proxmox_ve_ssh_private_key_path" {
  description = "Optional private key path for snippet uploads when agent is not used."
  type        = string
  default     = ""
}
