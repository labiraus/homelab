# Machine Names

This page is a quick reference for machine names currently used in the repo.

Some names appear in more than one section. For example, the Kubernetes workers and the Minecraft VM are Ubuntu machines, so they are listed under both `Ubuntu machines` and their VM grouping.

## Physical Hosts

### Raspberry Pi Machines

- `svartalfheim`
  - role: Raspberry Pi host for external MinIO
  
### Ubuntu Machines

- `yggdrasil`
  - control-plane / source host used for kubeconfig refresh in repo docs
- `midgard`
  - manual bare-metal Ubuntu Kubernetes worker used intermittently for GPU-heavy workloads

### Proxmox Machines

- `proxmox-node1`
  - IP: `192.168.8.229`
- `proxmox-node2`
  - IP: `192.168.8.133`
- `proxmox-node3`
  - IP: `192.168.8.191`
  - model: `Dell G7 7590`
  - CPU: `Intel Core i7-9750H` (`6` cores / `12` threads, up to `4.5 GHz`)
  - RAM: about `15 GiB`
  - storage: `476.9 GiB` NVMe plus `931.5 GiB` SATA
  - GPUs: Intel `UHD Graphics 630` plus NVIDIA `GeForce RTX 2070 Mobile`
- `proxmox-node4`
  - IP: `192.168.8.103 `

## Kubernetes Nodes

- `yggdrasil`
  - IP: `192.168.8.132`
- `jotunheim`
  - Proxmox host: `proxmox-node1`
  - IP: `192.168.8.121`
- `alfheim`
  - Proxmox host: `proxmox-node2`
  - IP: `192.168.8.122`
- `helheim`
  - Proxmox host: `proxmox-node3`
  - IP: `192.168.8.123`
  - planned as the consumer-GPU worker on the RTX-equipped Proxmox host
- `niflheim`
  - Proxmox host: `proxmox-node4`
  - IP: `192.168.8.124`
- `midgard`
  - IP: `192.168.8.205`

## Other VMs

- `nidavellir`
  - role: dedicated Minecraft VM
  - Proxmox host: `proxmox-node1`
  - IP: `192.168.8.126`
