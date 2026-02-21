variable "cluster_name" {
  description = "Logical cluster name used by cloud-init metadata."
  type        = string
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
  })
}

variable "network" {
  description = "Node network config."
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
  default     = "ubuntu-24.04-noble-server-cloudimg-amd64.img"
}

variable "kube_minor_channel" {
  description = "pkgs.k8s.io stable minor channel (for example v1.31)."
  type        = string
  default     = "v1.31"
}

variable "kube_version" {
  description = "Optional explicit package version for kubelet/kubeadm/kubectl. Empty string uses channel latest."
  type        = string
  default     = ""
}

variable "kubeadm_join_command" {
  description = "Optional kubeadm join command. Warning: if used in cloud-init it is stored in Terraform state."
  type        = string
  default     = ""
  sensitive   = true
}

variable "kubelet_node_labels" {
  description = "Optional node labels for kubelet registration."
  type        = map(string)
  default     = {}
}

variable "kubelet_register_taints" {
  description = "Optional kubelet taints on registration."
  type        = list(string)
  default     = []
}

variable "kubeconfig_path" {
  description = "Kubeconfig path used by kubernetes and helm providers."
  type        = string
  default     = "~/.kube/config"
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

variable "cilium" {
  description = "Optional Cilium Helm configuration."
  type = object({
    enabled                = bool
    chart_version          = string
    namespace              = string
    kube_proxy_replacement = string
    k8s_service_host       = string
    k8s_service_port       = number
    values                 = map(string)
  })
  default = {
    enabled                = false
    chart_version          = "1.19.1"
    namespace              = "kube-system"
    kube_proxy_replacement = ""
    k8s_service_host       = ""
    k8s_service_port       = 6443
    values                 = {}
  }
}
