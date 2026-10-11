#!/usr/bin/env python3
"""Synthetic file fixture, sent over stdin to the owned test Pod's Python."""
import hashlib
import json
import os
from pathlib import Path
import stat
import sys

ROOT = Path('/data')


def seed():
    (ROOT / 'nested').mkdir()
    (ROOT / 'empty-directory').mkdir()
    (ROOT / 'readme.txt').write_text('Restored fixture Ω\nSecond line\n')
    (ROOT / 'nested' / 'Unicode Ω.txt').write_text('Private synthetic content\n')
    (ROOT / '.hidden').write_bytes(b'\x00\xff\n')
    (ROOT / 'empty-file').touch()
    with (ROOT / 'payload.bin').open('wb') as output:
        chunk = bytes(range(256)) * 256
        for _ in range(256): output.write(chunk)
    (ROOT / 'nested' / 'payload-link').symlink_to('../payload.bin')
    for path in sorted(ROOT.rglob('*')):
        if not path.is_symlink():
            path.chmod(0o750 if path.is_dir() else 0o640)
        os.chown(path, 11001, 11002, follow_symlinks=False)
        os.utime(path, (1700000000, 1700000000), follow_symlinks=False)


def manifest():
    result = {}
    for path in sorted(ROOT.rglob('*')):
        relative = path.relative_to(ROOT)
        # Reserved Velero restore-helper markers are not application data.
        if relative.parts[0] == '.velero': continue
        info = path.lstat()
        item = {'mode': stat.S_IMODE(info.st_mode), 'uid': info.st_uid,
                'gid': info.st_gid, 'mtime': int(info.st_mtime)}
        if path.is_symlink(): item.update(type='symlink', target=os.readlink(path))
        elif path.is_dir(): item['type'] = 'directory'
        elif path.is_file():
            digest = hashlib.sha256()
            with path.open('rb') as source:
                for chunk in iter(lambda: source.read(65536), b''): digest.update(chunk)
            item.update(type='file', bytes=info.st_size, sha256=digest.hexdigest())
        else: raise ValueError('Unexpected file type: ' + str(relative))
        result[str(relative)] = item
    return result


if __name__ == '__main__':
    if sys.argv[1:] == ['seed']: seed()
    elif sys.argv[1:] == ['manifest']: print(json.dumps(manifest(), sort_keys=True))
    else: raise SystemExit('Expected seed or manifest')
