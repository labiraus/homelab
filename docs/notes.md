# Platform Notes

- Kubernetes target: v1.34+
- GitOps controller: Argo CD App-of-Apps
- Deployment packaging: Helm charts (local charts under `charts/` + upstream charts)
- Upstream chart values source of truth: `values/` only
- Storage default class: Longhorn
- S3-compatible object storage: MinIO tenant in-cluster
- MinIO state strategy: Ansible playbooks under `ansible/` (no Crossplane/Terraform)
- Secret strategy: generate from Ansible outputs and apply to Kubernetes

## Optional / disabled by default

- Plex is optional and not included in Argo Application manifests by default.
- Harvester/KubeVirt resources are documentation-first because most deployments require dedicated hardware and node pools.
- Kafka is intentionally not installed. Add it after baseline stability using Strimzi operator or Redpanda operator.
