# Homelab Terraform: Proxmox + kubeadm Workers

This repository provisions Kubernetes worker VMs on Proxmox VE using Terraform, layered `tfvars`, and Terraform Cloud (HCP Terraform) for state.

## Repository Layout

```text
.
├── .devcontainer/
│   ├── Dockerfile
│   ├── devcontainer.json
│   ├── .bashrc
│   ├── .env
│   ├── hosts
│   ├── kubeconfig
│   └── ssh/
│       └── config
├── bin/
│   └── tf
├── components/
│   └── kubernetes/
├── modules/
│   └── proxmox-ubuntu-vm/
├── etc/
│   ├── env/
│   │   └── lab.tfvars
│   ├── nodes/
│   │   ├── node1.tfvars
│   │   ├── node2.tfvars
│   │   ├── node3.tfvars
│   │   └── node4.tfvars
├── Makefile
└── README.md
```

## Prerequisites

1. HCP Terraform organization exists and `TFC_ORG` is set.
2. Proxmox API and snippet upload access configured with environment variables:
   - `PROXMOX_VE_ENDPOINT` (example: `https://proxmox-node1:8006/`)
   - `PROXMOX_VE_API_TOKEN` (format is `full-tokenid=value`, for example `root@pam!homelab=<token-secret>`)
   - SSH agent forwarding enabled or provide `proxmox_ve_ssh_private_key_path` via tfvars.
3. Proxmox storage requirements:
   - Snippet target datastore must allow `snippets` content type.
   - Image datastore must allow image import/download (`iso`/import-capable content).

One-line Proxmox API token creation (run on a Proxmox host):
- `pveum user token add root@pam homelab --privsep 0`

## Devcontainer Setup And Customization

`devcontainer.json` mounts repo-managed files into the container so you can customize runtime behavior without rebuilding Terraform code.

Mounted files:
- `.devcontainer/.bashrc` -> `/home/vscode/.bashrc.devcontainer`
- `.devcontainer/.env` -> `/home/vscode/.env`
- `.devcontainer/hosts` -> `/etc/hosts`
- `.devcontainer/ssh/` -> `/home/vscode/.ssh`
- `.devcontainer/kubeconfig` -> `/home/vscode/.kube/config`

How to customize:
1. Edit `.devcontainer/.env` for non-secret environment defaults (for example `ENV=lab`, `TFC_ORG=<org>`).  
2. Edit `.devcontainer/.bashrc` for shell aliases and auto-loading `/home/vscode/.env`.
3. Edit `.devcontainer/hosts` to override container DNS resolution for homelab hostnames.
4. Edit `.devcontainer/ssh/config` to define SSH host aliases, usernames, and key behavior.

Notes:
- Reopen/rebuild container after mount/layout changes in `devcontainer.json`.
- Terraform is already installed in this devcontainer via features; no host install is required.
- Keep real secrets out of committed files. Prefer local overrides or Terraform Cloud variables for sensitive values.

## Terraform Cloud Account Setup

`bin/tf` creates a temporary `cloud.auto.tf` and targets workspaces named:

`<ENV>-<COMPONENT>-<PRIMARY_LAYER>`

Example workspace names:
- `lab-kubernetes-node1`
- `lab-kubernetes-node2`

Required one-time setup in Terraform Cloud:
1. Create your organization (and optional project).
2. Create workspaces or let Terraform create them on first init.
3. Set each workspace execution mode to `Local`.

Why `Local` mode is required:
- This repo uses local module sources such as `../../modules/proxmox-ubuntu-vm`.
- In `Remote` mode, Terraform Cloud uploads only the working directory (`components/kubernetes`), so parent `../../modules` paths are unavailable.

Authentication:
1. Run `terraform login` (or `terraform login <hostname>` if using `TFC_HOSTNAME`).
2. Token is stored in `~/.terraform.d/credentials.tfrc.json`.
3. `bin/tf` checks for credentials and prompts login if missing (interactive shells only).

Optional Terraform Cloud settings:
- `TFC_PROJECT` to include `project = "..."` in generated cloud config.
- `TFC_HOSTNAME` if you are using Terraform Enterprise instead of `app.terraform.io`.

## Wrapper Script

Command format:

```bash
bin/tf <action> <component> <primary_layer> [overlay_layer ...] [-- <extra terraform args>]
```

- `action`: `plan`, `apply`, `destroy`
- `component`: currently `kubernetes`
- `primary_layer`: first node layer (`node1`, `node2`, ...)
- overlay layers: from `etc/<layer>.tfvars` when needed
- extra Terraform flags can be passed after `--`

The script:
1. Resolves repo root from script location.
2. Builds var-file order:
   - `etc/env/${ENV:-lab}.tfvars`
   - `etc/nodes/<primary_layer>.tfvars`
   - each overlay as `etc/<overlay>.tfvars` or `etc/overlays/<overlay>.tfvars`
3. Verifies tfvars and Terraform Cloud credentials.
4. Exports `TF_IN_AUTOMATION=1`.
5. Uses `plugin-cache/` for plugin cache.
6. Uses `.tfdata/<ENV>/<component>/<workspace>/` for `TF_DATA_DIR`.
7. Generates `components/<component>/cloud.auto.tf` using `TFC_ORG` (plus optional `TFC_PROJECT` and `TFC_HOSTNAME`).
8. Runs:
   - `terraform init -upgrade`
   - `terraform <action> -input=false -lock-timeout=5m ...`
9. Removes generated `cloud.auto.tf` on exit.

## Example Commands

```bash
ENV=lab TFC_ORG=my-org bin/tf plan kubernetes node1
ENV=lab TFC_ORG=my-org bin/tf apply kubernetes node1
ENV=lab TFC_ORG=my-org bin/tf apply kubernetes node2
ENV=lab TFC_ORG=my-org bin/tf destroy kubernetes node2
```

Pass extra Terraform flags:

```bash
ENV=lab TFC_ORG=my-org bin/tf apply kubernetes node1 -- -auto-approve
```

Makefile shortcuts:

```bash
make plan COMPONENT=kubernetes LAYER=node1
make apply COMPONENT=kubernetes LAYER=node1
make destroy COMPONENT=kubernetes LAYER=node1
```

## Kubernetes Worker Bootstrap

Cloud-init user-data installs and configures:
- `containerd` with `SystemdCgroup=true`
- Kubernetes packages (`kubeadm`, `kubelet`, `kubectl`) using official `pkgs.k8s.io` repository
- swap disable (`swapoff -a` + `/etc/fstab` update)
- optional `kubeadm join` when `kubeadm_join_command` is non-empty

Idempotency:
- Join only runs if `/etc/kubernetes/kubelet.conf` does not exist.

## Helm Charts

This repository only provisions worker nodes and joins them to the cluster.
Install CNI and other Helm charts separately from your cluster operations workflow.

## Security Notes

- Do not commit real secrets.
- `kubeadm_join_command` is sensitive, but if embedded in cloud-init it will still appear in Terraform state.
- Keep secret overrides in local files such as `*.secret.tfvars` (already ignored in `.gitignore`).

## Node Layer Editing

Each `etc/nodes/nodeX.tfvars` defines:
- `proxmox.node_name`, `proxmox.vm_id`, `proxmox.vm_name`, `proxmox.bridge`, `proxmox.datastore_id`
- VM sizing (`vm.cpu_cores`, `vm.memory_mb`, `vm.disk_size_gb`)
- Network (`network.use_dhcp` or static `network.ipv4_address` + `network.ipv4_gateway`)
- `ssh_authorized_keys`
- `kubeadm_join_command` (blank for provision-only)

## Outputs

`components/kubernetes` outputs:
- `vm_name`
- `vm_id`
- `node_ip`
