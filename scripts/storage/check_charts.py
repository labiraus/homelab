#!/usr/bin/env python3
"""Ensure routine publications retain source storage; target overlays opt in."""
from pathlib import Path
import subprocess
import yaml

ROOT = Path(__file__).resolve().parents[2]
count = 0
for name in ("external", "orchestrator", "processor", "postgres"):
    chart = ROOT / "helm" / ("data" if name == "postgres" else "apps") / name
    for registry in (None, "ghcr", "ecr"):
        overlay = chart / f"values-{registry}.yaml"
        if registry and not overlay.exists():
            continue
        for target in (False, True):
            command = ["helm", "template", "homelab-" + name, str(chart),
                       "--set", "global.allowMissingGeneratedSecretRefs=true"]
            if registry:
                command += ["-f", str(overlay)]
            if target:
                command += ["-f", str(chart / "values-seaweedfs.yaml")]
            rendered = subprocess.run(command, text=True, capture_output=True, check=True).stdout
            resources = [r for r in yaml.safe_load_all(rendered) if r]
            port = 8333 if target else 9000
            if name == "postgres":
                cluster = next(r for r in resources if r["kind"] == "Cluster")
                backup = cluster["spec"]["backup"]["barmanObjectStore"]
                assert backup["endpointURL"] == f"http://svartalfheim:{port}"
                credential = "cnpg-seaweedfs-s3" if target else "cnpg-backup-s3"
                assert backup["s3Credentials"]["accessKeyId"]["name"] == credential
            else:
                secret = next(r for r in resources if r["kind"] == "Secret" and "MINIO_ENDPOINT" in r.get("stringData", {}))
                assert secret["stringData"]["MINIO_ENDPOINT"] == f"svartalfheim:{port}"
                credential = "documents-s3-credentials" if target else "documents-minio-credentials"
                for key in ("MINIO_ACCESS_KEY", "MINIO_SECRET_KEY"):
                    assert credential in secret["stringData"][key]
                policy = next(r for r in resources if r["kind"] == "NetworkPolicy" and r["metadata"]["name"] == "homelab-" + name)
                lan = [r for r in policy["spec"]["egress"] if any(t.get("ipBlock", {}).get("cidr") == "192.168.8.177/32" for t in r.get("to", []))]
                assert len(lan) == 1 and lan[0]["ports"] == [{"port": port, "protocol": "TCP"}]
            count += 1
for filename, port, secret in (("velero-values.yaml", 9000, "velero-minio-credentials"),
                               ("velero-seaweedfs-values.yaml", 8333, "velero-s3-credentials")):
    values = yaml.safe_load((ROOT / "values" / filename).read_text())
    assert values["configuration"]["backupStorageLocation"][0]["config"]["s3Url"] == f"http://svartalfheim:{port}"
    assert values["credentials"]["existingSecret"] == secret
print(f"PASS: {count} storage chart combinations and both Velero configurations")
