#!/usr/bin/env python3
"""Bounded snapshot verification for synthetic offline TSDB fixtures only."""
import gzip
import hashlib
import io
import json
from pathlib import PurePosixPath
import re
import tarfile

MAX_ARCHIVE = 32 * 1024 * 1024
MAX_EXPANDED = 64 * 1024 * 1024
BLOCK = r'[0-9A-HJKMNP-TV-Z]{26}'


def inspect_snapshot(data):
    if not data or len(data) > MAX_ARCHIVE:
        raise ValueError('Snapshot archive is empty or exceeds the fixture limit')
    files, directories, blocks, total = {}, set(), {}, 0
    try:
        with gzip.GzipFile(fileobj=io.BytesIO(data)) as compressed:
            expanded = compressed.read(MAX_EXPANDED + 1)
        if len(expanded) > MAX_EXPANDED: raise ValueError('Snapshot exceeds expanded fixture limit')
        with tarfile.open(fileobj=io.BytesIO(expanded), mode='r:') as archive:
            for count, entry in enumerate(archive, 1):
                if count > 2048: raise ValueError('Too many snapshot members')
                name = entry.name.removeprefix('./').rstrip('/')
                if name in ('', '.') and entry.isdir(): continue
                path = PurePosixPath(name)
                if path.is_absolute() or '..' in path.parts or str(path) != name:
                    raise ValueError('Unsafe snapshot path')
                if name in files or name in directories: raise ValueError('Duplicate snapshot member')
                if not re.fullmatch(BLOCK + r'(/(meta\.json|index|tombstones|chunks(/[0-9]{6})?))?', name):
                    raise ValueError('Unexpected snapshot layout')
                if entry.isdir():
                    if len(path.parts) > 2 or (len(path.parts) == 2 and path.parts[1] != 'chunks'):
                        raise ValueError('Unexpected snapshot directory')
                    directories.add(name); continue
                if entry.type not in (tarfile.REGTYPE, tarfile.AREGTYPE) or len(path.parts) < 2 or path.parts[-1] == 'chunks':
                    raise ValueError('Snapshot links and special entries are refused')
                total += entry.size
                if entry.size < 0 or total > MAX_EXPANDED: raise ValueError('Snapshot exceeds expanded fixture limit')
                content = archive.extractfile(entry).read(entry.size + 1)
                if len(content) != entry.size: raise ValueError('Truncated snapshot member')
                files[name] = {'bytes': len(content), 'sha256': hashlib.sha256(content).hexdigest()}
                if path.name == 'meta.json':
                    meta = json.loads(content)
                    if not isinstance(meta, dict) or not isinstance(meta.get('stats'), dict):
                        raise ValueError('Invalid snapshot metadata object')
                    if (type(meta.get('version')) is not int or meta['version'] != 1 or meta.get('ulid') != path.parts[0]
                        or type(meta.get('minTime')) is not int or type(meta.get('maxTime')) is not int
                        or meta['minTime'] >= meta['maxTime'] or type(meta['stats'].get('numSamples')) is not int
                        or meta['stats']['numSamples'] <= 0):
                        raise ValueError('Invalid or empty snapshot block metadata')
                    blocks[path.parts[0]] = meta
    except (tarfile.TarError, OSError, EOFError, json.JSONDecodeError, UnicodeError) as error:
        raise ValueError('Unreadable snapshot archive') from error
    roots = {name.split('/')[0] for name in [*files, *directories]}
    if not blocks or roots != set(blocks): raise ValueError('Missing snapshot block metadata')
    for block in blocks:
        if not files.get(block + '/index', {}).get('bytes'):
            raise ValueError('Missing or empty snapshot index')
        if not any(name.startswith(block + '/chunks/') and value['bytes'] for name, value in files.items()):
            raise ValueError('Missing snapshot chunks')
    return {'files': files, 'blocks': blocks}


def verify_snapshot(data, expected):
    actual = inspect_snapshot(data)
    if actual != expected: raise ValueError('Snapshot manifest/checksum mismatch')
    return actual
