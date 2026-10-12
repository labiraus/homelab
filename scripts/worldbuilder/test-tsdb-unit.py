#!/usr/bin/env python3
"""Refuse incomplete, unsafe or modified snapshots before fixture extraction."""
import importlib.util
import io
import json
from pathlib import Path
import tarfile
from types import SimpleNamespace
import unittest

s = importlib.util.spec_from_file_location('snapshot', Path(__file__).with_name('tsdb-snapshot.py'))
m = importlib.util.module_from_spec(s); s.loader.exec_module(m)
s = importlib.util.spec_from_file_location('recovery', Path(__file__).with_name('test-tsdb.py'))
recovery = importlib.util.module_from_spec(s); s.loader.exec_module(recovery)
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


class AdminRefusal(unittest.TestCase):
    def probe(self, code, body):
        fixture = SimpleNamespace(execute=lambda *args, **kwargs: (code, body, b'fixture transport error'))
        return recovery.Recovery(fixture).admin_disabled('http://127.0.0.1:1/api/v1/admin/tsdb/snapshot')

    def test_initial_connection_refusal_is_not_disabled_evidence(self):
        self.assertFalse(self.probe(7, b''))

    def test_explicit_disabled_response_is_required(self):
        self.assertTrue(self.probe(0, b'{"status":"error","error":"admin APIs disabled"}'))
        for body in (b'{"status":"success","data":{"name":"unexpected"}}',
                     b'{"status":"error","error":"unauthorized"}', b'not JSON'):
            with self.subTest(body=body), self.assertRaises((AssertionError, ValueError)):
                self.probe(0, body)

    def test_other_transport_errors_fail_immediately(self):
        for code in (6, 28, 35, 60):
            with self.subTest(code=code), self.assertRaisesRegex(AssertionError, 'probe failed'):
                self.probe(code, b'')


if __name__ == '__main__': unittest.main()
