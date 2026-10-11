#!/usr/bin/env python3
"""Refuse incomplete, unsafe or modified snapshots before fixture extraction."""
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import unittest

s = importlib.util.spec_from_file_location('snapshot', Path(__file__).with_name('tsdb-snapshot.py'))
m = importlib.util.module_from_spec(s); s.loader.exec_module(m)
BLOCK = '01M4NS8HN7S11WMTFXZ2XM5T4R'
META = {'ulid': BLOCK, 'version': 1, 'minTime': 1, 'maxTime': 2, 'stats': {'numSamples': 3}}


def archive(changes=None, extra=()):
    files = {BLOCK + '/meta.json': json.dumps(META).encode(), BLOCK + '/index': b'index',
             BLOCK + '/chunks/000001': b'chunk'}
    files.update(changes or {})
    out = io.BytesIO()
    with tarfile.open(fileobj=out, mode='w:gz') as tar:
        for name, data in files.items():
            if data is None: continue
            info = tarfile.TarInfo(name); info.size = len(data); tar.addfile(info, io.BytesIO(data))
        for info in extra: tar.addfile(info)
    return out.getvalue()


class SnapshotRefusal(unittest.TestCase):
    def test_changed_chunk_refused(self):
        good = archive(); expected = m.inspect_snapshot(good)
        with self.assertRaisesRegex(ValueError, 'checksum'):
            m.verify_snapshot(archive({BLOCK + '/chunks/000001': b'changed'}), expected)

    def test_missing_index_chunk_or_metadata(self):
        for suffix in ('index', 'chunks/000001', 'meta.json'):
            with self.subTest(suffix=suffix), self.assertRaises(ValueError):
                m.inspect_snapshot(archive({BLOCK + '/' + suffix: None}))

    def test_invalid_metadata(self):
        for changes in ({'ulid': 'wrong'}, {'version': 2}, {'maxTime': 0}, {'stats': {'numSamples': 0}},
                        {'stats': {'numSamples': '3'}}, {'stats': {'numSamples': True}}):
            meta = {**META, **changes}
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                m.inspect_snapshot(archive({BLOCK + '/meta.json': json.dumps(meta).encode()}))

    def test_path_escape_and_unexpected_live_wal(self):
        for path in ('../escape', '/escape', BLOCK + '/../escape', 'wal/00000000', BLOCK + '/metaXjson'):
            with self.subTest(path=path), self.assertRaises(ValueError):
                m.inspect_snapshot(archive({path: b'data'}))

    def test_links_special_files_and_duplicates(self):
        for kind in (tarfile.SYMTYPE, tarfile.LNKTYPE, tarfile.FIFOTYPE, tarfile.REGTYPE):
            info = tarfile.TarInfo(BLOCK + ('/index' if kind == tarfile.REGTYPE else '/tombstones')); info.type = kind; info.linkname = '/outside'
            with self.subTest(kind=kind), self.assertRaises(ValueError): m.inspect_snapshot(archive(extra=[info]))

    def test_bounds_and_truncation(self):
        good = archive()
        with self.assertRaises(ValueError): m.inspect_snapshot(good[:15])
        previous = m.MAX_EXPANDED
        try:
            m.MAX_EXPANDED = 2
            with self.assertRaises(ValueError): m.inspect_snapshot(good)
        finally: m.MAX_EXPANDED = previous


if __name__ == '__main__': unittest.main()
