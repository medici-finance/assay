"""Offline release-producer tests; fake Docker, real artifact bytes and manifests."""
import copy
import fnmatch
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('images_contract', HERE / 'images_contract.py')
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)
VERSION = 'v1.2.3'
SOURCE = 'a' * 40


def workflow_jobs(text):
    return dict(re.findall(r'(?ms)^  ([a-zA-Z0-9_-]+):\n(.*?)(?=^  [a-zA-Z0-9_-]+:\n|\Z)',
                           text.split('\njobs:\n', 1)[1]))


def workflow_steps(block):
    return dict(re.findall(r'(?ms)^      - name: ([^\n]+)\n(.*?)(?=^      - |\Z)', block))


def publication_problems(text):
    """Check the approval boundary and the order of externally visible writes."""
    blocks = workflow_jobs(text)
    publisher = blocks.get('release', '')
    problems = []
    if '\npermissions:\n  contents: read\n' not in text.split('\njobs:\n', 1)[0]:
        problems.append('workflow defaults must not grant publication writes')
    approved = [name for name, block in blocks.items()
                if re.search(r'^    environment: release$', block, re.M)]
    if approved != ['release']:
        problems.append('exactly one release-approved job must own publication')
    match = re.search(r'^    needs: \[(.*?)\]', publisher, re.M)
    needs = {item.strip() for item in match.group(1).split(',')} if match else set()
    if not {'resolve', 'guard', 'test', 'artifacts'} <= needs:
        problems.append('publication must wait for all preparation gates')
    if 'runs-on: medici-builder-public-release' not in publisher:
        problems.append('publisher needs the dedicated Docker-capable release runner')
    for permission in ('contents', 'packages'):
        if not re.search(r'^      ' + permission + r': write$', publisher, re.M):
            problems.append(permission + ' write belongs to the approved publisher')
        if any(re.search(r'^      ' + permission + r': write$', block, re.M)
               for name, block in blocks.items() if name != 'release'):
            problems.append(permission + ' write escaped the approved publisher')
    names = ['Restore the prepared source and verify release artifacts',
             'Build, verify and publish the release images',
             'Validate and checksum the published image references',
             'Dry run — stop before tagging and publishing',
             'Create and push the dispatched tag',
             'Create GitHub release and upload assets']
    positions = [publisher.find('      - name: ' + name + '\n') for name in names]
    if min(positions) < 0 or positions != sorted(positions):
        problems.append('images and validated references must precede tag and release writes')
    if re.search(r'^\s*continue-on-error: true$', publisher, re.M):
        problems.append('a failed image or reference check must stop publication')
    steps = workflow_steps(publisher)
    for name, guard in {
        'Create and push the dispatched tag': "github.event_name == 'workflow_dispatch' && !inputs.dry_run",
        'Create GitHub release and upload assets': "${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
        'Validate and checksum the published image references': "${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
    }.items():
        if not re.search(r'^        if: ' + re.escape(guard) + '$', steps.get(name, ''), re.M):
            problems.append('missing dry-run guard on ' + name)
    required = [
        'git checkout --detach "$SOURCE_SHA"',
        'from tools.release.images import build_images',
        "build_images(Path.cwd(), os.environ['RELEASE_TAG'], os.environ['SOURCE_SHA'],",
        'from tools.release.images_contract import validate',
        'sha256sum release-images.json verify-runner-image.txt >> checksums.txt',
        'release-images.json verify-runner-image.txt checksums.txt; do',
        "publish=os.environ['DRY_RUN'] != 'true'",
        'persist-credentials: false',
        "trap 'rm -rf \"$DOCKER_CONFIG\" \"$SOURCE\"' EXIT",
    ]
    for value in required:
        if value not in publisher:
            problems.append('missing publication guard: ' + value)
    return problems


class ImagesTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.script = self.root / 'tools/release/images.py'
        self.script.parent.mkdir(parents=True)
        shutil.copyfile(HERE / 'images.py', self.script)
        plugin = self.root / 'plugins/assay/.claude-plugin'
        plugin.mkdir(parents=True)
        (plugin / 'plugin.json').write_text('{"version":"1.2.3"}')
        scan = self.root / 'containers/scripts/layer-secret-scan.sh'
        scan.parent.mkdir(parents=True)
        scan.write_text('''#!/bin/sh
printf 'scan %s\\n' "$1" >> "$CALLS"
[ "${MODE:-}" != scan-fail ]
''')
        self.inputs = self.root / 'inputs'
        self.inputs.mkdir()
        (self.inputs / 'statusgen-linux-amd64').write_bytes(b'published statusgen bytes')
        (self.inputs / 'image-bugs-gc-linux-amd64').write_bytes(b'bugs-gc bytes')
        with tarfile.open(self.inputs / 'desk-tools-linux-amd64.tar.gz', 'w:gz') as out:
            for name in ['deskboard', 'cellctl', 'deskpr', 'hooks/pre-push']:
                info = tarfile.TarInfo(name)
                info.mode = 0o755
                info.size = len(name)
                out.addfile(info, io.BytesIO(name.encode()))
        bindir = self.root / 'bin'
        bindir.mkdir()
        docker = bindir / 'docker'
        docker.write_text('''#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
with open(os.environ['CALLS'], 'a') as f: f.write(json.dumps(args)+'\\n')
mode = os.environ.get('MODE', '')
if args[:2] == ['manifest', 'inspect']:
    if mode == 'exists': print('{}'); sys.exit(0)
    print('unauthorized' if mode == 'lookup-fail' else 'manifest unknown', file=sys.stderr)
    sys.exit(1)
if args[0] == 'run':
    print('v0.0.0' if mode == 'stale' and 'desk-base:' in args[-2] else 'v1.2.3')
if args[0] == 'push' and mode == 'push-fail': sys.exit(1)
if args[:2] == ['image', 'inspect']:
    if mode == 'missing-digest': sys.exit(0)
    repo = args[-1].rsplit(':', 1)[0]
    if mode == 'wrong-digest-repo': repo = 'ghcr.io/unrelated/image'
    print(repo + '@sha256:' + 'b'*64)
''')
        docker.chmod(0o755)
        self.out = self.root / 'output'
        self.calls = self.root / 'calls'
        self.env = dict(os.environ, PATH=str(bindir) + os.pathsep + os.environ['PATH'],
                        CALLS=str(self.calls), PYTHONDONTWRITEBYTECODE='1')

    def invoke(self, publish=False, mode=''):
        code = ('from pathlib import Path; import sys; from images import build_images; '
                'build_images(Path(sys.argv[1]), *sys.argv[2:8], sys.argv[8] == "publish")')
        return subprocess.run(['python3', '-c', code, str(self.root), VERSION, SOURCE,
                               'ghcr.io/example/assay', 'ghcr.io/example/verify-runner',
                               str(self.inputs), str(self.out), 'publish' if publish else 'build'],
                              cwd=self.script.parent,
                              env=dict(self.env, MODE=mode), text=True, capture_output=True)

    def recorded(self):
        return self.calls.read_text().splitlines() if self.calls.exists() else []

    def test_dry_build_never_pushes_or_writes_pinnable_refs(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.out / 'release-images.json').exists())
        calls = self.recorded()
        self.assertFalse(any('"push"' in s or '"manifest"' in s for s in calls))
        self.assertEqual(sum(s.startswith('scan ') for s in calls), 7)
        self.assertIn('scan --prepare', calls)
        self.assertLess(calls.index('scan --prepare'), next(i for i, s in enumerate(calls) if '"build"' in s))
        self.assertEqual(sum('"run"' in s for s in calls), 9)
        self.assertIn('DESK_TOOLS_IMAGE=ghcr.io/example/assay/desk-tools:v1.2.3', '\n'.join(calls))
        self.assertEqual((self.root / '.release-images/harness/statusgen').read_bytes(),
                         (self.inputs / 'statusgen-linux-amd64').read_bytes())

    def test_publish_produces_complete_consistent_manifest_after_all_checks(self):
        result = self.invoke(True)
        self.assertEqual(result.returncode, 0, result.stderr)
        manifest = json.loads((self.out / 'release-images.json').read_text())
        refs = contract.validate(manifest, VERSION, SOURCE,
                                 (self.out / 'verify-runner-image.txt').read_text(), 'example/assay')
        self.assertEqual(len(refs), 9)
        calls = self.recorded()
        pushes = [i for i, s in enumerate(calls) if '"push"' in s]
        self.assertEqual(len(pushes), 9)
        self.assertLess(max(i for i, s in enumerate(calls) if s.startswith('scan ')), min(pushes))
        self.assertLess(max(i for i, s in enumerate(calls) if '"run"' in s), min(pushes))
        for mutate in ['missing', 'duplicate', 'tag', 'digest', 'source', 'runner']:
            with self.subTest(mutate=mutate):
                broken = copy.deepcopy(manifest)
                runner = (self.out / 'verify-runner-image.txt').read_text()
                if mutate == 'missing': broken['images'].pop()
                if mutate == 'duplicate': broken['images'].append(broken['images'][0])
                if mutate == 'tag': broken['images'][0]['ref'] = broken['images'][0]['ref'].replace(VERSION, 'v0.0.0')
                if mutate == 'digest': broken['images'][0]['ref'] = broken['images'][0]['ref'].split('@')[0]
                if mutate == 'source': broken['source_revision'] = 'c' * 40
                if mutate == 'runner': runner = runner.rstrip('\n')
                with self.assertRaises(ValueError):
                    contract.validate(broken, VERSION, SOURCE, runner, 'example/assay')

    def test_refusals_never_publish_metadata(self):
        for mode in ['exists', 'lookup-fail', 'stale', 'scan-fail', 'push-fail',
                     'missing-digest', 'wrong-digest-repo']:
            with self.subTest(mode=mode):
                stage = self.root / '.release-images'
                if stage.exists(): shutil.rmtree(stage)
                self.calls.unlink(missing_ok=True)
                result = self.invoke(True, mode)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertFalse((self.out / 'release-images.json').exists())
                if mode in ['exists', 'lookup-fail', 'stale', 'scan-fail']:
                    self.assertFalse(any('"push"' in s for s in self.recorded()))

    def test_release_has_one_approval_before_all_publication(self):
        root = HERE.parents[1]
        for path in ['.github/workflows/release.yml', 'ci/staged-workflows/release.yml']:
            with self.subTest(path=path):
                main = (root / path).read_text()
                self.assertEqual(publication_problems(main), [])
                blocks = workflow_jobs(main)
                self.assertNotIn('images', blocks)
                self.assertIn('git archive --format=tar.gz HEAD', blocks['artifacts'])
                self.assertIn('CGO_ENABLED: "0"', blocks['artifacts'])
                self.assertIn('git bundle create release-source.bundle', blocks['artifacts'])
                self.assertNotIn('publish_image:', main)
                self.assertNotIn('desk_image_tag:', main)
        self.assertEqual((root / '.github/workflows/release.yml').read_bytes(),
                         (root / 'ci/staged-workflows/release.yml').read_bytes())
        self.assertFalse((root / '.github/workflows/docker-publish.yml').exists())
        self.assertFalse((root / 'ci/staged-workflows/docker-publish.yml').exists())

    def test_platform_artifacts_reconstruct_the_complete_release(self):
        root = HERE.parents[1]
        platforms = ['darwin-arm64', 'darwin-amd64', 'linux-amd64',
                     'windows-amd64', 'windows-arm64']
        common = ['cursor-bundle.tar.gz', 'checksums.txt',
                  'tool-validation-v1.2.3.json', 'tool-validation-v1.2.3.md',
                  'image-bugs-gc-linux-amd64', 'prepared-source.tar.gz',
                  'build-inputs.sha256', 'release-highlights.md']
        for path in ['.github/workflows/release.yml', 'ci/staged-workflows/release.yml']:
            jobs = workflow_jobs((root / path).read_text())
            uploads = re.findall(
                r'(?ms)^      - name: Upload prepared release artifacts[^\n]*\n'
                r'(.*?)(?=^      - |\Z)', jobs['artifacts'])
            self.assertEqual(len(uploads), 6)
            for with_bundle in [False, True]:
                with self.subTest(path=path, with_bundle=with_bundle):
                    expected = {'release-build-common': common +
                                (['release-source.bundle'] if with_bundle else [])}
                    for platform in platforms:
                        suffix = '.exe' if platform.startswith('windows-') else ''
                        expected['release-build-' + platform] = [
                            'statusgen-' + platform + suffix,
                            'qualgen-' + platform + suffix,
                            'desk-tools-' + platform + '.tar.gz']
                    files = [name for names in expected.values() for name in names]
                    selected = []
                    for upload in uploads:
                        name = re.search(r'^          name: (.+)$', upload, re.M).group(1)
                        self.assertIn(name, expected)
                        self.assertIn('compression-level: 0', upload)
                        self.assertIn('if-no-files-found: error', upload)
                        patterns = re.findall(r'^            (.+)$', upload, re.M)
                        matched = [f for f in files if any(fnmatch.fnmatchcase(f, p) for p in patterns)]
                        self.assertCountEqual(matched, expected[name])
                        self.assertFalse(any(fnmatch.fnmatchcase('go-cache.tar', p) for p in patterns))
                        selected.extend(matched)
                    self.assertCountEqual(selected, files)
            release = jobs['release']
            self.assertIn('pattern: release-build-*', release)
            self.assertIn('merge-multiple: true', release)
            self.assertIn('sha256sum -c build-inputs.sha256', release)
            self.assertIn('sha256sum -c checksums.txt', release)

    def test_approval_and_publication_order_regressions_are_rejected(self):
        root = HERE.parents[1]
        main = (root / '.github/workflows/release.yml').read_text()
        self.assertEqual(publication_problems(main), [])
        mutants = {
            'default-write': main.replace('\npermissions:\n  contents: read\n',
                                           '\npermissions:\n  contents: write\n', 1),
            'missing-approval': main.replace('    environment: release\n', '', 1),
            'second-approval': main + '\n  second:\n    environment: release\n',
            'missing-tests': main.replace('needs: [resolve, guard, test, artifacts]',
                                          'needs: [resolve, guard, artifacts]', 1),
            'missing-image-step': main.replace('Build, verify and publish the release images', 'removed', 1),
            'wrong-order': main.replace('Build, verify and publish the release images', 'TEMP', 1)
                              .replace('Create and push the dispatched tag',
                                       'Build, verify and publish the release images', 1)
                              .replace('TEMP', 'Create and push the dispatched tag', 1),
            'ignored-image-failure': main.replace('Build, verify and publish the release images\n',
                                                  'Build, verify and publish the release images\n        continue-on-error: true\n', 1),
            'unguarded-writer': main + '\n  other:\n    permissions:\n      packages: write\n',
            'missing-ref-check': main.replace('from tools.release.images_contract import validate', 'removed', 1),
            'missing-checksum': main.replace('sha256sum release-images.json verify-runner-image.txt >> checksums.txt', 'removed', 1),
            'dry-run-tag': main.replace("if: github.event_name == 'workflow_dispatch' && !inputs.dry_run", 'if: true', 1),
            'dry-run-release': main.replace("- name: Create GitHub release and upload assets\n        if: ${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
                                             '- name: Create GitHub release and upload assets\n        if: true', 1),
            'dry-run-ref-check': main.replace("- name: Validate and checksum the published image references\n        if: ${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
                                               '- name: Validate and checksum the published image references\n        if: true', 1),
        }
        for name, mutant in mutants.items():
            with self.subTest(name=name):
                self.assertTrue(publication_problems(mutant), name)

    def test_wrong_seed_version_stops_before_registry_contact(self):
        (self.root / 'plugins/assay/.claude-plugin/plugin.json').write_text('{"version":"9.9.9"}')
        self.assertNotEqual(self.invoke(True).returncode, 0)
        self.assertEqual(self.recorded(), [])

    def test_normalized_versions_share_the_image_publish_lock(self):
        import re
        root = HERE.parents[1]
        main = (root / 'ci/staged-workflows/release.yml').read_text()
        # Execute the real resolver's dispatch branch, without forge contact.
        block = main.split('      - name: Resolve the release tag\n', 1)[1]
        lines = block.split('        run: |\n', 1)[1].splitlines()
        script = []
        for line in lines:
            if line.strip() and not line.startswith('          '):
                break
            script.append(line[10:])
        tags = []
        for version in ['1.2.3', 'v1.2.3']:
            output = self.root / ('resolved-' + version)
            result = subprocess.run(['bash', '-c', '\n'.join(script)],
                                    env=dict(os.environ, EVENT='workflow_dispatch', INPUT_VERSION=version,
                                             ACTOR='fixture', REF_NAME='main', GITHUB_OUTPUT=str(output)),
                                    text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            tags.append(dict(line.split('=', 1) for line in output.read_text().splitlines())['tag'])
        self.assertEqual(tags, [VERSION, VERSION])
        job = workflow_jobs(main)['release']
        lock = re.search(r'    concurrency:\n      group: (.+)\n      cancel-in-progress: false\n', job)
        self.assertIsNotNone(lock, 'image publication must serialize before tag-absence checks')
        self.assertIn('${{ needs.resolve.outputs.tag }}', lock.group(1))
        groups = [lock.group(1).replace('${{ needs.resolve.outputs.tag }}', tag) for tag in tags]
        self.assertEqual(len(set(groups)), 1)
        self.assertNotIn('${{', groups[0], 'lock must depend only on the normalized version')

    def test_archive_link_refuses_before_docker_build(self):
        with tarfile.open(self.inputs / 'desk-tools-linux-amd64.tar.gz', 'w:gz') as out:
            info = tarfile.TarInfo('deskboard')
            info.type = tarfile.SYMTYPE
            info.linkname = '/outside'
            out.addfile(info)
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertFalse(any('"build"' in s for s in self.recorded()))


if __name__ == '__main__':
    unittest.main()
