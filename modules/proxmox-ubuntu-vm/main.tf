locals {
  image_datastore_id    = coalesce(var.image_datastore_id, var.datastore_id)
  snippets_datastore_id = coalesce(var.snippets_datastore_id, var.datastore_id)
}

resource "proxmox_virtual_environment_download_file" "ubuntu_cloud_image" {
  node_name    = var.node_name
  datastore_id = local.image_datastore_id
  content_type = "import"
  url          = var.ubuntu_image_url
  file_name    = var.ubuntu_image_file_name
  overwrite    = false
}

resource "proxmox_virtual_environment_file" "cloud_init_user_data" {
  node_name    = var.node_name
  datastore_id = local.snippets_datastore_id
  content_type = "snippets"

  source_raw {
    data      = var.cloud_init_user_data
    file_name = "${var.vm_name}-user-data.yaml"
  }
}

resource "proxmox_virtual_environment_vm" "this" {
  name      = var.vm_name
  node_name = var.node_name
  vm_id     = var.vm_id
  tags      = var.tags

  started = true
  on_boot = true

  agent {
    enabled = true
  }

  cpu {
    cores = var.vm_cpu_cores
    type  = "x86-64-v2-AES"
  }

  memory {
    dedicated = var.vm_memory_mb
  }

  disk {
    datastore_id = var.datastore_id
    import_from  = proxmox_virtual_environment_download_file.ubuntu_cloud_image.id
    interface    = "scsi0"
    size         = var.vm_disk_size_gb
    iothread     = true
    discard      = "on"
  }

  network_device {
    bridge = var.bridge
    model  = "virtio"
  }

  initialization {
    datastore_id      = var.datastore_id
    user_data_file_id = proxmox_virtual_environment_file.cloud_init_user_data.id

    ip_config {
      ipv4 {
        address = var.use_dhcp ? "dhcp" : var.ipv4_address
        gateway = var.use_dhcp ? null : var.ipv4_gateway
      }
    }

    user_account {
      username = var.ssh_username
      keys     = var.ssh_authorized_keys
    }
  }
}
