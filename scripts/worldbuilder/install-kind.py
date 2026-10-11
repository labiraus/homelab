#!/usr/bin/env python3
"""Install the official checksum-pinned kind binary for disposable acceptance."""
import hashlib
import json
from pathlib import Path
import platform
import sys
import urllib.request

VERSION = '0.31.0'
NODE = 'kindest/node:v1.35.0@sha256:452d707d4862f52530247495d180205e029056831160e22870e37e3f6c1ac31f'
CHECKSUMS = {
    'amd64': 'eb244cbafcc157dff60cf68693c14c9a75c4e6e6fedaf9cd71c58117cb93e3fa',
    'arm64': '8e1014e87c34901cc422a1445866835d1e666f2a61301c27e722bdeab5a1f7e4',
}


def install(directory):
    arch = {'x86_64': 'amd64', 'aarch64': 'arm64'}.get(platform.machine())
    if platform.system() != 'Linux' or arch not in CHECKSUMS:
        raise ValueError('Disposable kind acceptance requires Linux amd64/arm64')
    directory = Path(directory); directory.mkdir(parents=True, exist_ok=True)
    target = directory / 'kind'
    url = f'https://github.com/kubernetes-sigs/kind/releases/download/v{VERSION}/kind-linux-{arch}'
    with urllib.request.urlopen(url, timeout=60) as response:
        data = response.read(64 * 1024**2 + 1)
    if len(data) > 64 * 1024**2 or hashlib.sha256(data).hexdigest() != CHECKSUMS[arch]:
        raise ValueError('Official kind binary checksum mismatch')
    target.write_bytes(data); target.chmod(0o755)
    (directory / 'kind.json').write_text(json.dumps(dict(version=VERSION, architecture=arch,
        url=url, sha256=CHECKSUMS[arch], node=NODE), indent=2) + '\n')
    return target


if __name__ == '__main__':
    install(sys.argv[1])
