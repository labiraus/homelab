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
- `proxmox-node2`
- `proxmox-node3`
- `proxmox-node4`

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
  - IP: `192.168.8.XXX`
  - placeholder IP still present in repo tfvars
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
