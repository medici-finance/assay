"""Offline release-producer tests; fake Docker, real artifact bytes and manifests."""
import copy
import fnmatch
import hashlib
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
        'from tools.release.images_contract import RUNNER_PLATFORM, validate',
        'runner_platforms=(RUNNER_PLATFORM,)',
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
        check = self.root / 'images/verify-runner/check.py'
        check.parent.mkdir(parents=True)
        check.write_text('# fixture capability check')
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
        (self.inputs / 'statusgen-linux-arm64').write_bytes(b'published arm64 statusgen bytes')
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
import hashlib, json, os, sys
args = sys.argv[1:]
with open(os.environ['CALLS'], 'a') as f: f.write(json.dumps(args)+'\\n')
mode = os.environ.get('MODE', '')
def digest(ref): return 'sha256:' + hashlib.sha256(ref.encode()).hexdigest()
ARCHES = {'linux/amd64': '-linux-amd64', 'linux/arm64': '-linux-arm64'}
if args[:2] == ['manifest', 'inspect'] and '@' in args[-1]:
    repo, dgst = args[-1].split('@')
    if dgst != 'sha256:' + 'c'*64: print('manifest unknown', file=sys.stderr); sys.exit(1)
    entries = [{'digest': digest(repo + ':v1.2.3' + suffix),
                'platform': {'os': p.split('/')[0], 'architecture': p.split('/')[1]}}
               for p, suffix in ARCHES.items()]
    if mode == 'index-extra': entries.append({'digest': digest('x'), 'platform': {'os': 'linux', 'architecture': 's390x'}})
    if mode == 'index-dup': entries.append(dict(entries[0]))
    if mode == 'index-wrong-digest': entries[1]['digest'] = digest('unpushed')
    media = 'application/vnd.oci.image.manifest.v1+json' if mode == 'index-not-list' else 'application/vnd.oci.image.index.v1+json'
    print(json.dumps({'schemaVersion': 2, 'mediaType': media, 'manifests': entries})); sys.exit(0)
if args[:2] == ['manifest', 'inspect']:
    if mode == 'exists' or (mode == 'arch-tag-exists' and args[-1].endswith('-linux-arm64')):
        print('{}'); sys.exit(0)
    print('unauthorized' if mode == 'lookup-fail' else 'manifest unknown', file=sys.stderr)
    sys.exit(1)
if args[:2] == ['manifest', 'create']: sys.exit(0)
if args[:2] == ['manifest', 'push']:
    if mode != 'index-no-digest': print('sha256:' + 'c'*64)
    sys.exit(0)
if args[:2] == ['image', 'inspect'] and args[3] == '{{.Os}}/{{.Architecture}}':
    platform = next(p for p, suffix in ARCHES.items() if args[-1].endswith(suffix))
    print('linux/amd64' if mode == 'wrong-arch' else platform); sys.exit(0)
if args[0] == 'run' and 'python3' in args:
    if mode in ('missing-yaml', 'missing-jsonschema'):
        print('ModuleNotFoundError: ' + mode, file=sys.stderr); sys.exit(1)
    print('python capability checked'); sys.exit(0)
if args[0] == 'run':
    print('v0.0.0' if mode == 'stale' and 'desk-base:' in args[-2] else 'v1.2.3')
if args[0] == 'push' and mode == 'push-fail': sys.exit(1)
if args[:2] == ['image', 'inspect']:
    if mode == 'missing-digest': sys.exit(0)
    repo = args[-1].rsplit(':', 1)[0]
    if mode == 'wrong-digest-repo': repo = 'ghcr.io/unrelated/image'
    print(repo + '@' + digest(args[-1]))
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

    # regression: #3469
    def test_python_before_publish(self):
        result = self.invoke(True)
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        probes = [c for c in calls if c[0] == 'run' and 'python3' in c]
        # regression: #4323 — the probe runs on every runner platform.
        self.assertEqual([(p[p.index('--platform') + 1], p[p.index('python3') + 1]) for p in probes],
                         [('linux/amd64', 'ghcr.io/example/verify-runner:v1.2.3-linux-amd64'),
                          ('linux/arm64', 'ghcr.io/example/verify-runner:v1.2.3-linux-arm64')])
        for probe in probes:
            self.assertEqual(probe[-3:], ['-B', '-c',
                             (self.root / 'images/verify-runner/check.py').read_text()])
            for flag, value in [('--pull', 'never'), ('--network', 'none'),
                                ('--user', '1000:1000'), ('--cap-drop', 'ALL'),
                                ('--security-opt', 'no-new-privileges'),
                                ('--pids-limit', '64'), ('--memory', '256m'),
                                ('--cpus', '1'),
                                ('--tmpfs', '/tmp:rw,noexec,nosuid,size=16m')]:
                self.assertEqual(probe[probe.index(flag) + 1], value)
            self.assertIn('--read-only', probe)
            self.assertNotIn('--volume', probe)
            self.assertNotIn('--mount', probe)
            self.assertNotIn('seccomp=unconfined', probe)
            self.assertLess(calls.index(probe), next(i for i, c in enumerate(calls) if c[0] == 'push'))

    # regression: #3469
    def test_missing_python_stops(self):
        for mode in ('missing-yaml', 'missing-jsonschema'):
            with self.subTest(mode=mode):
                self.setUp()
                result = self.invoke(True, mode)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertIn(mode, result.stderr)
                self.assertFalse(any('"push"' in s for s in self.recorded()))
                self.assertFalse((self.out / 'release-images.json').exists())

    def test_dry_build_never_pushes_or_writes_pinnable_refs(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.out / 'release-images.json').exists())
        calls = self.recorded()
        self.assertFalse(any('"push"' in s or '"manifest"' in s for s in calls))
        self.assertEqual(sum(s.startswith('scan ') for s in calls), 6)
        # Eight amd64 images and two runner platforms report a version; both
        # runner platforms run the capability probe.
        self.assertEqual(sum('"run"' in s for s in calls), 12)
        self.assertIn('DESK_TOOLS_IMAGE=ghcr.io/example/assay/desk-tools:v1.2.3', '\n'.join(calls))
        self.assertEqual((self.root / '.release-images/harness/statusgen').read_bytes(),
                         (self.inputs / 'statusgen-linux-amd64').read_bytes())
        for arch in ('amd64', 'arm64'):
            self.assertEqual((self.root / f'.release-images/verify-runner/statusgen-linux-{arch}').read_bytes(),
                             (self.inputs / f'statusgen-linux-{arch}').read_bytes())
        self.assertFalse((self.root / '.release-images/harness/statusgen-linux-arm64').exists())

    # regression: #4323
    def test_runner_builds_each_platform_and_never_the_bare_tag(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        builds = [c for c in calls if c[0] == 'build' and c[-2].endswith('images/verify-runner/Dockerfile')]
        self.assertEqual([(c[c.index('--platform') + 1], c[c.index('-t') + 1]) for c in builds],
                         [('linux/amd64', 'ghcr.io/example/verify-runner:v1.2.3-linux-amd64'),
                          ('linux/arm64', 'ghcr.io/example/verify-runner:v1.2.3-linux-arm64')])
        versions = [c for c in calls if c[0] == 'run' and 'statusgen' in c and 'verify-runner' in c[-2]]
        self.assertEqual([(c[c.index('--platform') + 1], c[-2]) for c in versions],
                         [('linux/amd64', 'ghcr.io/example/verify-runner:v1.2.3-linux-amd64'),
                          ('linux/arm64', 'ghcr.io/example/verify-runner:v1.2.3-linux-arm64')])
        self.assertFalse(any('ghcr.io/example/verify-runner:v1.2.3' in c for c in calls))

    # regression: #4323
    def test_runner_platform_builds_push_a_single_manifest(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        builds = [c for c in calls if c[0] == 'build' and c[-2].endswith('images/verify-runner/Dockerfile')]
        self.assertEqual(len(builds), 2)
        for build in builds:
            # An attestation-carrying image pushes as an index, which
            # `docker manifest create` refuses after the tags are spent.
            self.assertIn('--provenance=false', build)
            self.assertIn('--sbom=false', build)

    # regression: #4323
    def test_runner_platforms_match_the_contract(self):
        spec = importlib.util.spec_from_file_location('images', HERE / 'images.py')
        images = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(images)
        self.assertEqual(images.RUNNER_PLATFORMS, contract.RUNNER_PLATFORMS)
        self.assertEqual(images.RUNNER_PLATFORM, contract.RUNNER_PLATFORM)
        self.assertEqual(contract.RUNNER_PLATFORM, 'linux/amd64,linux/arm64')

    # regression: #4323
    def test_missing_arm64_statusgen_stops_before_any_build(self):
        (self.inputs / 'statusgen-linux-arm64').unlink()
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertFalse(any('"build"' in s for s in self.recorded()))

    def test_publish_produces_complete_consistent_manifest_after_all_checks(self):
        result = self.invoke(True)
        self.assertEqual(result.returncode, 0, result.stderr)
        manifest = json.loads((self.out / 'release-images.json').read_text())
        strict = (contract.RUNNER_PLATFORM,)
        refs = contract.validate(manifest, VERSION, SOURCE,
                                 (self.out / 'verify-runner-image.txt').read_text(), 'example/assay',
                                 runner_platforms=strict)
        self.assertEqual(len(refs), 9)
        # regression: #4323 — the runner reference pins the index digest the
        # registry returned, and its record names both platforms.
        runner = next(i for i in manifest['images'] if i['name'] == 'verify-runner')
        self.assertEqual(runner, {'name': 'verify-runner', 'platform': 'linux/amd64,linux/arm64',
                                  'ref': 'ghcr.io/example/verify-runner:v1.2.3@sha256:' + 'c' * 64})
        self.assertEqual({i['platform'] for i in manifest['images'] if i['name'] != 'verify-runner'},
                         {'linux/amd64'})
        calls = self.recorded()
        parsed = [json.loads(s) if s.startswith('[') else [] for s in calls]
        pushes = [i for i, c in enumerate(parsed) if c[:1] == ['push']]
        self.assertEqual(len(pushes), 10)
        self.assertEqual([parsed[i][1] for i in pushes][-2:],
                         ['ghcr.io/example/verify-runner:v1.2.3-linux-amd64',
                          'ghcr.io/example/verify-runner:v1.2.3-linux-arm64'])
        create = next(i for i, c in enumerate(parsed) if c[:2] == ['manifest', 'create'])
        self.assertEqual(parsed[create][2:], [
            'ghcr.io/example/verify-runner:v1.2.3',
            'ghcr.io/example/verify-runner@' + 'sha256:' + hashlib.sha256(
                b'ghcr.io/example/verify-runner:v1.2.3-linux-amd64').hexdigest(),
            'ghcr.io/example/verify-runner@' + 'sha256:' + hashlib.sha256(
                b'ghcr.io/example/verify-runner:v1.2.3-linux-arm64').hexdigest()])
        self.assertLess(max(pushes), create)
        index_push = next(i for i, c in enumerate(parsed) if c[:2] == ['manifest', 'push'])
        read_back = next(i for i, c in enumerate(parsed) if c[:2] == ['manifest', 'inspect'] and '@' in c[-1])
        self.assertLess(create, index_push)
        self.assertLess(index_push, read_back)
        self.assertLess(max(i for i, s in enumerate(calls) if s.startswith('scan ')), min(pushes))
        self.assertLess(max(i for i, s in enumerate(calls) if '"run"' in s), min(pushes))
        # Absence is established for the bare tag and both platform tags.
        absent = {c[-1] for c in parsed if c[:2] == ['manifest', 'inspect'] and '@' not in c[-1]}
        self.assertLessEqual({'ghcr.io/example/verify-runner:v1.2.3',
                              'ghcr.io/example/verify-runner:v1.2.3-linux-amd64',
                              'ghcr.io/example/verify-runner:v1.2.3-linux-arm64'}, absent)
        self.assertEqual(len(absent), 11)
        # An amd64-only runner record is a legacy consumer record: the default
        # contract still admits it, the producer's strict call does not.
        legacy = copy.deepcopy(manifest)
        next(i for i in legacy['images'] if i['name'] == 'verify-runner')['platform'] = 'linux/amd64'
        runner_txt = (self.out / 'verify-runner-image.txt').read_text()
        contract.validate(legacy, VERSION, SOURCE, runner_txt, 'example/assay')
        with self.assertRaises(ValueError):
            contract.validate(legacy, VERSION, SOURCE, runner_txt, 'example/assay', runner_platforms=strict)
        with self.assertRaises(ValueError):
            contract.validate(manifest, VERSION, SOURCE, runner_txt, 'example/assay',
                              runner_platforms=contract.RUNNER_PLATFORM)
        for mutate in ['missing', 'duplicate', 'tag', 'digest', 'source', 'runner',
                       'runner-platform', 'desk-multiarch']:
            with self.subTest(mutate=mutate):
                broken = copy.deepcopy(manifest)
                runner = (self.out / 'verify-runner-image.txt').read_text()
                if mutate == 'missing': broken['images'].pop()
                if mutate == 'duplicate': broken['images'].append(broken['images'][0])
                if mutate == 'tag': broken['images'][0]['ref'] = broken['images'][0]['ref'].replace(VERSION, 'v0.0.0')
                if mutate == 'digest': broken['images'][0]['ref'] = broken['images'][0]['ref'].split('@')[0]
                if mutate == 'source': broken['source_revision'] = 'c' * 40
                if mutate == 'runner': runner = runner.rstrip('\n')
                if mutate == 'runner-platform': broken['images'][-1]['platform'] = 'linux/arm64'
                if mutate == 'desk-multiarch': broken['images'][0]['platform'] = contract.RUNNER_PLATFORM
                with self.assertRaises(ValueError):
                    contract.validate(broken, VERSION, SOURCE, runner, 'example/assay', runner_platforms=strict)

    def test_refusals_never_publish_metadata(self):
        for mode in ['exists', 'lookup-fail', 'stale', 'scan-fail', 'push-fail',
                     'missing-digest', 'wrong-digest-repo', 'arch-tag-exists', 'wrong-arch',
                     'index-no-digest', 'index-extra', 'index-dup', 'index-wrong-digest',
                     'index-not-list']:
            with self.subTest(mode=mode):
                stage = self.root / '.release-images'
                if stage.exists(): shutil.rmtree(stage)
                self.calls.unlink(missing_ok=True)
                result = self.invoke(True, mode)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertFalse((self.out / 'release-images.json').exists())
                if mode in ['exists', 'lookup-fail', 'stale', 'scan-fail', 'arch-tag-exists', 'wrong-arch']:
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
        # linux-arm64 carries only the runner image's statusgen (#4323).
        partial = {'linux-arm64': ['statusgen-linux-arm64']}
        common = ['cursor-bundle.tar.gz', 'checksums.txt', 'sbom.spdx.json',
                  'tool-validation-v1.2.3.json', 'tool-validation-v1.2.3.md',
                  'image-bugs-gc-linux-amd64', 'prepared-source.tar.gz',
                  'build-inputs.sha256', 'release-highlights.md']
        for path in ['.github/workflows/release.yml', 'ci/staged-workflows/release.yml']:
            jobs = workflow_jobs((root / path).read_text())
            uploads = re.findall(
                r'(?ms)^      - name: Upload prepared release artifacts[^\n]*\n'
                r'(.*?)(?=^      - |\Z)', jobs['artifacts'])
            self.assertEqual(len(uploads), 7)
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
                    for platform, names in partial.items():
                        expected['release-build-' + platform] = names
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

    # regression: #4323 — the arm64 statusgen the runner image carries is a
    # published release asset: built, described, checksummed and attested.
    def test_runner_statusgen_assets_flow_through_the_release(self):
        root = HERE.parents[1]
        for path in ['.github/workflows/release.yml', 'ci/staged-workflows/release.yml']:
            jobs = workflow_jobs((root / path).read_text())
            steps = workflow_steps(jobs['artifacts'])
            published = workflow_steps(jobs['release'])
            for asset in ['statusgen-linux-amd64', 'statusgen-linux-arm64']:
                goarch = asset.rsplit('-', 1)[1]
                with self.subTest(path=path, asset=asset):
                    self.assertRegex(steps['Build binaries'],
                                     r'GOOS=linux +GOARCH=' + goarch + r' go build [^\n]*-o \.\./' + asset + r' ')
                    sbom = steps['Generate the SBOM'].split('<<', 1)[0]
                    self.assertRegex(sbom, r'(?<![\w-])' + asset + r'(?![\w-])')
                    checksums = steps['Generate checksums'].split('> checksums.txt', 1)[0]
                    self.assertRegex(checksums, r'(?m)^ +' + asset + r' \\$')
                    attested = published['List the release assets to attest'].split('; do', 1)[0]
                    self.assertRegex(attested, r'(?<![\w-])' + asset + r'(?![\w-])')

    # regression: #4784 — the release runner had no arm64 handler, so the
    # linux/arm64 verify-runner build failed with "exec format error". The job
    # registers emulation itself, before any credential exists, in dry runs too.
    def test_arm64_emulation_is_registered_before_credentials_and_images(self):
        root = HERE.parents[1]
        step_name = 'Register and prove arm64 emulation'
        for path in ['.github/workflows/release.yml', 'ci/staged-workflows/release.yml']:
            with self.subTest(path=path):
                publisher = workflow_jobs((root / path).read_text())['release']
                step = workflow_steps(publisher)[step_name]
                at = publisher.find('      - name: ' + step_name + '\n')
                self.assertGreater(at, publisher.find('      - name: Restore the prepared source'))
                self.assertLess(at, publisher.find('      - name: Build, verify and publish the release images\n'))
                login = publisher.find('| docker login ghcr.io')
                self.assertGreater(login, 0)
                self.assertLess(at, login)
                # Per line (re.M): an `if:` anywhere in the step skips it.
                self.assertNotRegex(step, r'(?m)^        if:', 'dry runs build arm64 too')
                # The whole step, run script included: no expression may put a
                # token or secret into the privileged step.
                self.assertNotIn('${{', step, 'the privileged step must carry no token or secret')
                env = step.split('        run: |\n', 1)[0]
                self.assertRegex(env, r'BINFMT_IMAGE: [a-z0-9./-]+:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}\n')
                self.assertIn('docker run --privileged --rm --network none "$BINFMT_IMAGE"'
                              ' --uninstall qemu-aarch64 --install arm64 2>&1', step)
                self.assertIn("grep -qx 'installing: arm64 OK'", step)

    def test_arm64_emulation_step_fails_closed(self):
        main = (HERE.parents[1] / '.github/workflows/release.yml').read_text()
        step = workflow_steps(workflow_jobs(main)['release'])['Register and prove arm64 emulation']
        script = '\n'.join(line[10:] for line in step.split('        run: |\n', 1)[1].splitlines())
        work = self.root / 'emulation'
        (work / 'images/verify-runner').mkdir(parents=True)
        (work / 'bin').mkdir()
        fake = work / 'bin/docker'
        # Models the pinned installer (cmd/binfmt/main.go at e29e7d72): it logs
        # one line per uninstall and install to stderr, prints its status JSON
        # to stdout, and exits 0 even when a registration fails. Only a failed
        # container start exits non-zero. Also model an interrupted installer
        # after its success line, to prove that partial output cannot mask a
        # non-zero exit. The probe answers per FAKE_PROBE_*.
        fake.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$CALLS"\n'
                        'case "$*" in *--install*)\n'
                        '  printf "%s" "$FAKE_LOG" >&2\n'
                        '  echo \'{"supported":["linux/amd64"],"emulators":[]}\'\n'
                        '  exit "$FAKE_START";;\nesac\n'
                        '[ "$FAKE_PROBE_EXIT" = 0 ] || exit "$FAKE_PROBE_EXIT"\n'
                        'echo "$FAKE_ARCH"\n')
        fake.chmod(0o755)
        base = 'golang:1.2.3-bookworm@sha256:' + 'a' * 64
        fresh = 'uninstalling: qemu-aarch64 not found\ninstalling: arm64 OK\n'
        replaced = 'uninstalling: qemu-aarch64 OK\ninstalling: arm64 OK\n'
        stale = 'uninstalling: qemu-aarch64 not found\ninstalling: arm64 qemu-aarch64 already registered\n'
        refused = 'uninstalling: qemu-aarch64 not found\ninstalling: arm64 EPERM opening /proc/sys/fs/binfmt_misc/register check permissions?\n'
        # Exact registration failure from release dry run 40777382211. This
        # tool still exits zero; the filesystem diagnostic must stop the job.
        read_only = ('uninstalling: qemu-aarch64 not found\n'
                     'installing: arm64 Cannot open /proc/sys/fs/binfmt_misc/register: '
                     'open /proc/sys/fs/binfmt_misc/register: read-only file system\n')
        no_install = 'error: cannot mount binfmt_misc filesystem at /proc/sys/fs/binfmt_misc: operation not permitted\n'
        not_installed = "no 'installing: arm64 OK' line"
        not_working = '::error::arm64 emulation is not working'
        # name: (container start exit, installer log, probe exit, probe arch,
        #        RUNNER_BASE, expected exit, expected message, probe expected)
        cases = {
            'fresh-node': ('0', fresh, '0', 'aarch64', base, 0, 'arm64 emulation ready', True),
            'replaces-existing-handler': ('0', replaced, '0', 'aarch64', base, 0, 'arm64 emulation ready', True),
            # The real tool exits 0 here; a working pre-existing handler must
            # not be accepted in place of the pinned one.
            'stale-handler-survives': ('0', stale, '0', 'aarch64', base, 1, 'unregister it on the runner host', False),
            'register-refused': ('0', refused, '0', 'aarch64', base, 1, not_installed, False),
            'register-read-only': ('0', read_only, '0', 'aarch64', base, 1, 'runner registration filesystem is read-only', False),
            'read-only-despite-success-line': ('0', read_only + fresh, '0', 'aarch64', base, 1, 'runner registration filesystem is read-only', False),
            'binfmt-not-mountable': ('0', no_install, '0', 'aarch64', base, 1, not_installed, False),
            'ok-for-another-arch-only': ('0', 'installing: riscv64 OK\n', '0', 'aarch64', base, 1, not_installed, False),
            'container-fails-to-start': ('125', '', '0', 'aarch64', base, 1, 'exit 125', False),
            'success-line-with-nonzero-exit': ('137', fresh, '0', 'aarch64', base, 1, 'exit 137', False),
            'wrong-arch': ('0', fresh, '0', 'x86_64', base, 1, not_working, True),
            'probe-fails': ('0', fresh, '125', '', base, 1, not_working, True),
            'unpinned-base': ('0', fresh, '0', 'aarch64', 'golang:1.2.3-bookworm', 1, 'no single digest-pinned RUNNER_BASE', False),
        }
        installer = 'run --privileged --rm --network none binfmt:x@sha256:' + 'b' * 64 + ' --uninstall qemu-aarch64 --install arm64'
        prober = 'run --rm --network none --platform linux/arm64 ' + base + ' uname -m'
        for name, (start, log, probe_exit, arch, ref, code, message, probed) in cases.items():
            with self.subTest(case=name):
                (work / 'images/verify-runner/Dockerfile').write_text('ARG RUNNER_BASE=' + ref + '\nFROM ${RUNNER_BASE}\n')
                calls = work / ('calls-' + name)
                result = subprocess.run(
                    ['bash', '-c', script], cwd=work, text=True, capture_output=True,
                    env=dict(os.environ, PATH=str(work / 'bin') + os.pathsep + os.environ['PATH'],
                             CALLS=str(calls), FAKE_START=start, FAKE_LOG=log, FAKE_PROBE_EXIT=probe_exit,
                             FAKE_ARCH=arch, BINFMT_IMAGE='binfmt:x@sha256:' + 'b' * 64))
                self.assertEqual(result.returncode, code, result.stdout + result.stderr)
                self.assertIn(message, result.stdout)
                if name.startswith('register-read-only') or name == 'read-only-despite-success-line':
                    self.assertIn('runner owner', result.stdout)
                    self.assertIn('security decision', result.stdout)
                if ref == base:
                    expected = [installer] + ([prober] if probed else [])
                    self.assertEqual(calls.read_text().splitlines(), expected)
                    # The installer's own lines are echoed into the job log.
                    for line in log.splitlines():
                        self.assertIn(line, result.stdout)

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
            'missing-ref-check': main.replace('from tools.release.images_contract import RUNNER_PLATFORM, validate', 'removed', 1),
            'lenient-runner-platform': main.replace('runner_platforms=(RUNNER_PLATFORM,)', '', 1),
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
