variable "node_name" {
  description = "Proxmox node where the VM is created."
  type        = string
}

variable "vm_id" {
  description = "Unique VM ID in Proxmox."
  type        = number
}

variable "vm_name" {
  description = "VM name."
  type        = string
}

variable "bridge" {
  description = "Network bridge in Proxmox (for example vmbr0)."
  type        = string
}

variable "datastore_id" {
  description = "Primary datastore for VM disk."
  type        = string
}

variable "image_datastore_id" {
  description = "Datastore used for downloading the cloud image. Defaults to datastore_id."
  type        = string
  default     = null
}

variable "snippets_datastore_id" {
  description = "Datastore used for cloud-init snippets. Defaults to datastore_id."
  type        = string
  default     = null
}

variable "vm_cpu_cores" {
  description = "Number of vCPUs."
  type        = number
}

variable "vm_memory_mb" {
  description = "Memory in MB."
  type        = number
}

variable "vm_disk_size_gb" {
  description = "Disk size in GB."
  type        = number
}

variable "vm_bios" {
  description = "VM BIOS type."
  type        = string
  default     = "seabios"
}

variable "vm_machine" {
  description = "Optional QEMU machine type (for example q35 when using PCIe passthrough)."
  type        = string
  default     = null
}

variable "vm_efi_disk" {
  description = "Optional OVMF EFI disk settings. Required when vm_bios is ovmf."
  type = object({
    datastore_id      = optional(string)
    type              = optional(string, "4m")
    pre_enrolled_keys = optional(bool, false)
  })
  default = null
}

variable "hostpci_devices" {
  description = "Optional host PCI devices to pass through to the VM. Prefer cluster resource mappings when using the API token provider flow."
  type = list(object({
    device   = string
    mapping  = optional(string)
    id       = optional(string)
    mdev     = optional(string)
    pcie     = optional(bool, true)
    rombar   = optional(bool, true)
    rom_file = optional(string)
    xvga     = optional(bool, false)
  }))
  default = []
}

variable "ubuntu_image_url" {
  description = "Ubuntu cloud image URL."
  type        = string
}

variable "ubuntu_image_file_name" {
  description = "Filename used on Proxmox for the downloaded image."
  type        = string
}

variable "download_ubuntu_image" {
  description = "Download Ubuntu image into Proxmox when true. Set to false to reuse an existing image file in image_datastore_id."
  type        = bool
  default     = true
}

variable "enable_drain_on_destroy" {
  description = "Run a destroy-time drain command before deleting the VM."
  type        = bool
  default     = false
}

variable "drain_command" {
  description = "Optional drain command to run on destroy (defaults to kubectl drain <vm_name>)."
  type        = string
  default     = ""
}

variable "ssh_username" {
  description = "Username configured by cloud-init."
  type        = string
  default     = "ubuntu"
}

variable "ssh_authorized_keys" {
  description = "SSH public keys injected into cloud-init user account."
  type        = list(string)

  validation {
    condition     = length(var.ssh_authorized_keys) > 0
    error_message = "At least one SSH public key is required."
  }
}

variable "cloud_init_user_data" {
  description = "Rendered cloud-init user-data document."
  type        = string
}

variable "use_dhcp" {
  description = "Use DHCP for IPv4 if true."
  type        = bool
  default     = true

  validation {
    condition     = var.use_dhcp || (var.ipv4_address != "" && var.ipv4_gateway != "")
    error_message = "When use_dhcp is false, ipv4_address and ipv4_gateway must both be set."
  }
}

variable "ipv4_address" {
  description = "Static IPv4 CIDR (for example 192.168.8.121/24)."
  type        = string
  default     = ""
}

variable "ipv4_gateway" {
  description = "Static IPv4 gateway."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Optional Proxmox VM tags."
  type        = list(string)
  default     = []
}
