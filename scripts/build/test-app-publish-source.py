#!/usr/bin/env python3
"""Exercise publication freshness against owned local Git repositories."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

GUARD = Path(__file__).with_name('check-app-publish-source.sh').resolve()


class PublicationSourceTests(unittest.TestCase):
    def test_only_current_application_content_can_be_promoted(self):
        with tempfile.TemporaryDirectory(prefix='homelab-publish-source-') as directory:
            root = Path(directory)
            env = dict(os.environ, GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL='/dev/null', GITHUB_REF='refs/heads/main')

            def git(cwd, *args):
                return subprocess.run(['git', '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.test', *args],
                                      cwd=cwd, env=env, capture_output=True, text=True, check=True)

            git(root, 'init', '--bare', '--initial-branch=main', 'origin.git')
            git(root, 'clone', str(root / 'origin.git'), 'writer')
            writer = root / 'writer'

            def advance(path, body):
                target = writer / path
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text(body)
                git(writer, 'add', path)
                git(writer, 'commit', '-m', 'Fixture update')
                git(writer, 'push', 'origin', 'main')

            advance('apps/mcp/main.go', 'secured source')
            git(root, 'clone', str(root / 'origin.git'), 'queued')
            queued = root / 'queued'

            def guard(expected):
                result = subprocess.run(['bash', str(GUARD), 'mcp'], cwd=queued, env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode == 0, expected, result.stdout + result.stderr)

            guard(True)
            advance('docs/recovery.md', 'new notes')
            advance('apps/external/main.go', 'unrelated application')
            guard(True)
            for path in ['apps/mcp/main.go', 'apps/pkg/api/auth.go', 'helm/apps/mcp/values.yaml',
                         'helm/libraries/commonapi/templates/security.tpl', '.github/actions/docker/ghcr/action.yml',
                         'scripts/build/check-app-publish-source.sh', '.github/workflows/app-mcp.yml']:
                git(queued, 'fetch', 'origin', 'main')
                git(queued, 'reset', '--hard', 'FETCH_HEAD')
                advance(path, 'new security behavior: ' + path)
                guard(False)
                git(queued, 'reset', '--hard', 'FETCH_HEAD')
                guard(True)


if __name__ == '__main__':
    unittest.main()
