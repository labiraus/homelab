#!/usr/bin/env python3
"""Download the role-pinned official test server into a disposable directory."""
import hashlib
import io
from pathlib import Path
import platform
import sys
import tarfile
from urllib.request import urlopen
import yaml

values = yaml.safe_load((Path(__file__).resolve().parents[2] / "ansible/roles/seaweedfs/defaults/main.yml").read_text())
arch = {"x86_64": "amd64", "aarch64": "arm64"}[platform.machine()]
release = values["seaweedfs_version"]
with urlopen(f"https://github.com/seaweedfs/seaweedfs/releases/download/{release}/linux_{arch}.tar.gz", timeout=120) as response:
    data = response.read()
if hashlib.sha256(data).hexdigest() != values["seaweedfs_checksums"][arch]:
    raise SystemExit("SeaweedFS archive checksum mismatch")
target = Path(sys.argv[1])
target.mkdir(parents=True, exist_ok=True)
with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
    member = archive.getmember("weed")
    if not member.isfile():
        raise SystemExit("Expected regular weed binary")
    (target / "weed").write_bytes(archive.extractfile(member).read())
(target / "weed").chmod(0o755)
print(f"Verified SeaweedFS {release} ({arch})")
