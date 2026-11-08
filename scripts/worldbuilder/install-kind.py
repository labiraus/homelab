#!/usr/bin/env python3
"""Install the official checksum-pinned kind binary for disposable acceptance."""
import hashlib
import json
from pathlib import Path
import platform
import sys
import urllib.request

VERSION = '0.33.0'
NODE = 'kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5'
CHECKSUMS = {
    'amd64': 'aee6151561422756b764a4ae28e7f44cda5af5a9eead3cc9985112b1de8d8e0d',
    'arm64': '20022bee6cfcd5086cb7234d218e3454e6090022f2a8f55d1fa7fcf42c3867a2',
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
