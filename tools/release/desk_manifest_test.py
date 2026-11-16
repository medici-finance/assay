"""Offline fixtures for tools/release/desk_manifest.py and the Makefile's
desk-manifest target (#6604). Run:

    python3 -B -m unittest discover -s tools/release -p desk_manifest_test.py -v
"""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import desk_manifest  # noqa: E402

REPO = Path(__file__).resolve().parents[2]


def stage(root, files):
    root = Path(root)
    for rel, body in files.items():
        path = root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(body)
        path.chmod(0o755)
    return root


SHIPPED = {
    'deskboard': b'deskboard bytes',
    'deskdecision': b'deskdecision bytes',
    'hooks/pre-push': b'#!/bin/sh\nexit 0\n',
}


class DeskManifestTest(unittest.TestCase):
    def setUp(self):
        self.dir = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)

    # regression: #6604
    def test_write_covers_every_file(self):
        root = stage(self.dir / 'stage', SHIPPED)
        desk_manifest.write(root)
        lines = (root / desk_manifest.MANIFEST_NAME).read_text().splitlines()
        self.assertEqual([line.split('  ', 1)[1] for line in lines],
                         ['deskboard', 'deskdecision', 'hooks/pre-push'])
        self.assertEqual(lines[0].split('  ')[0],
                         desk_manifest.sha256_file(root / 'deskboard'))
        self.assertEqual(desk_manifest.check(root), [])

    # regression: #6604
    def test_check_flags_unlisted_file(self):
        root = stage(self.dir / 'stage', SHIPPED)
        desk_manifest.write(root)
        (root / 'statusgen').write_bytes(b'placed beside the tarball')
        self.assertEqual(desk_manifest.check(root),
                         ['statusgen: present but not listed'])
        self.assertEqual(desk_manifest.main(['check', str(root)]), 1)

    # regression: #6604
    def test_check_flags_changed_bytes(self):
        root = stage(self.dir / 'stage', SHIPPED)
        desk_manifest.write(root)
        (root / 'deskboard').write_bytes(b'a different build')
        self.assertEqual(desk_manifest.check(root),
                         ['deskboard: sha256 does not match the manifest'])

    def test_check_flags_missing_file(self):
        root = stage(self.dir / 'stage', SHIPPED)
        desk_manifest.write(root)
        (root / 'deskdecision').unlink()
        self.assertEqual(desk_manifest.check(root),
                         ['deskdecision: listed but not present'])

    def test_check_flags_malformed_line(self):
        root = stage(self.dir / 'stage', SHIPPED)
        desk_manifest.write(root)
        with open(root / desk_manifest.MANIFEST_NAME, 'a') as handle:
            handle.write('# a comment\n')
        self.assertIn('line 4: not "<sha256>  <path>"', desk_manifest.check(root))

    def test_refuses_empty_or_symlink(self):
        empty = self.dir / 'empty'
        empty.mkdir()
        with self.assertRaises(desk_manifest.ManifestError):
            desk_manifest.write(empty)
        self.assertEqual(desk_manifest.main(['write', str(empty)]), 2)
        root = stage(self.dir / 'stage', SHIPPED)
        os.symlink('deskboard', root / 'alias')
        with self.assertRaises(desk_manifest.ManifestError):
            desk_manifest.write(root)

    def test_no_manifest_cannot_check(self):
        root = stage(self.dir / 'stage', SHIPPED)
        self.assertEqual(desk_manifest.main(['check', str(root)]), 2)

    def test_sha256sum_reads_it(self):
        tool = shutil.which('sha256sum')
        argv = [tool, '-c', '--strict', desk_manifest.MANIFEST_NAME] if tool else None
        if argv is None and shutil.which('shasum'):
            argv = ['shasum', '-a', '256', '-c', desk_manifest.MANIFEST_NAME]
        if argv is None:
            self.skipTest('could-not-check: neither sha256sum nor shasum on PATH')
        root = stage(self.dir / 'stage', SHIPPED)
        desk_manifest.write(root)
        run = subprocess.run(argv, cwd=root, capture_output=True, text=True)
        self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
        self.assertEqual(run.stdout.count(': OK'), len(SHIPPED))


class MakeDeskManifestTest(unittest.TestCase):
    """The from-source install path: `make desk-manifest` writes the manifest
    INTO the install dir and removes the old source-tree copy, so replacing the
    installed bytes with a release tarball's replaces the manifest too."""

    def setUp(self):
        if not shutil.which('make') or not shutil.which('shasum'):
            self.skipTest('could-not-check: make or shasum not on PATH')
        self.dir = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)

    def make(self, install, desk):
        return subprocess.run(
            ['make', '--no-print-directory', '-C', str(REPO), 'desk-manifest',
             f'INSTALL_DIR={install}', f'DESK_DIR={desk}'],
            capture_output=True, text=True)

    # regression: #6604
    def test_make_writes_beside_bytes(self):
        install = stage(self.dir / 'bin', {'deskboard': b'local build',
                                           'statusgen': b'statusgen'})
        desk = self.dir / 'src'
        desk.mkdir()
        legacy = desk / desk_manifest.MANIFEST_NAME
        legacy.write_text('0' * 64 + '  deskboard\n')
        run = self.make(install, desk)
        self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
        self.assertFalse(legacy.exists(), 'the source-tree manifest must be removed')
        self.assertEqual(desk_manifest.check(install), [])

        # Upgrade in place from a release tarball's contents: its own manifest
        # overwrites the local one, and the documented check still verifies.
        release = stage(self.dir / 'release', {'deskboard': b'release build',
                                               'deskdecision': b'new verb'})
        desk_manifest.write(release)
        for name in ('deskboard', 'deskdecision', desk_manifest.MANIFEST_NAME):
            shutil.copy2(release / name, install / name)
        # statusgen is not a tarball member (it ships as its own asset, covered
        # by checksums.txt), so a copy placed beside the tarball's files is
        # named rather than silently passed.
        self.assertEqual(desk_manifest.check(install),
                         ['statusgen: present but not listed'])

    def test_make_empty_install_dir(self):
        install = self.dir / 'bin'
        install.mkdir()
        run = self.make(install, self.dir)
        self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
        self.assertFalse((install / desk_manifest.MANIFEST_NAME).exists())


if __name__ == '__main__':
    unittest.main()
