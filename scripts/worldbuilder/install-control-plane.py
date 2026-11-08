#!/usr/bin/env python3
"""Install pinned, checksum-verified upstream envtest binaries into a test directory."""
import hashlib
import json
from pathlib import Path
import platform
import shutil
import sys
import tarfile
import tempfile
import urllib.request

VERSION = "1.37.0"
# Official kubernetes-sigs/controller-tools envtest release index, SHA-512.
CHECKSUMS = {
    "amd64": "1d1c453633b72c161a5d5a886cde7ac850be1a2ac796a9e1d4ffacacc64868295bdd2d57aa66cd0c158f5ce510f5dfe3fbc61ac21bd3dcfb875bd70658aa663a",
    "arm64": "ae6a670502988200b0131c943758cfd3d3a58cf4e6247ef7f0e6a6467f1cd9a333c802e86101f0446b1b92a0e327387224e1e44fe1a4693da0929c6e529cfe9a",
}


def install(destination):
    arch = {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine())
    if platform.system() != "Linux" or arch not in CHECKSUMS:
        raise ValueError("The disposable control-plane gate supports Linux amd64/arm64")
    destination = Path(destination)
    destination.mkdir(parents=True, exist_ok=True)
    name = f"envtest-v{VERSION}-linux-{arch}.tar.gz"
    url = f"https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v{VERSION}/{name}"
    hashes = {}
    with tempfile.TemporaryFile() as archive:
        checksum, size = hashlib.sha512(), 0
        with urllib.request.urlopen(url, timeout=60) as response:
            while block := response.read(1024**2):
                size += len(block)
                if size > 512 * 1024**2:
                    raise ValueError("Test binary archive exceeds download bound")
                checksum.update(block)
                archive.write(block)
        if checksum.hexdigest() != CHECKSUMS[arch]:
            raise ValueError("Official envtest archive checksum mismatch")
        archive.seek(0)
        with tarfile.open(fileobj=archive) as source:
            for binary in ("kube-apiserver", "etcd", "kubectl"):
                member = source.getmember("controller-tools/envtest/" + binary)
                if not member.isfile():
                    raise ValueError("Expected a regular test binary, not a link")
                target = destination / binary
                with source.extractfile(member) as content, target.open("wb") as output:
                    shutil.copyfileobj(content, output)
                target.chmod(0o755)
                with target.open("rb") as stream:
                    hashes[binary] = hashlib.file_digest(stream, "sha256").hexdigest()
    (destination / "assets.json").write_text(json.dumps(dict(version=VERSION, architecture=arch,
        url=url, sha512=CHECKSUMS[arch], binaries=hashes), indent=2) + "\n")
    print(f"PASS: envtest {VERSION} Linux {arch}, verified official SHA-512; {destination}")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("Usage: install-control-plane.py OUTPUT_DIRECTORY")
    install(sys.argv[1])
