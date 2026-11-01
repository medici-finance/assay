"""Offline release-producer tests; fake Docker, real artifact bytes and manifests."""
from collections import Counter
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
keys_spec = importlib.util.spec_from_file_location('workflow_keys', HERE / 'workflow_keys.py')
workflow_keys = importlib.util.module_from_spec(keys_spec)
keys_spec.loader.exec_module(workflow_keys)
VERSION = 'v1.2.3'
SOURCE = 'a' * 40


NATIVE_CONFIG = b'{"architecture":"arm64","os":"linux"}'
NATIVE_ID = 'sha256:' + hashlib.sha256(NATIVE_CONFIG).hexdigest()


def native_archive(path, ref='ghcr.io/example/verify-runner:v1.2.3-linux-arm64', extra=False):
    config_name = NATIVE_ID.split(':')[1] + '.json'
    records = [{'Config': config_name, 'RepoTags': [ref], 'Layers': []}]
    if extra:
        records.append(dict(records[0], RepoTags=['ghcr.io/example/other:old']))
    files = {'manifest.json': json.dumps(records).encode(), config_name: NATIVE_CONFIG}
    with tarfile.open(path, 'w') as archive:
        for name, content in files.items():
            info = tarfile.TarInfo(name)
            info.size = len(content)
            archive.addfile(info, io.BytesIO(content))


def workflow_jobs(text):
    return dict(re.findall(r'(?ms)^  ([a-zA-Z0-9_-]+):\n(.*?)(?=^  [a-zA-Z0-9_-]+:\n|\Z)',
                           text.split('\njobs:\n', 1)[1]))


def workflow_steps(block):
    return dict(re.findall(r'(?ms)^      - name: ([^\n]+)\n(.*?)(?=^      - |\Z)', block))


# The release jobs that build and publish run on fixed, reviewed labels; the
# general CI pipeline (ci.yml) runs on GENERAL_CI_RUNNER and must never be one.
RELEASE_RUNNERS = {'artifacts': 'medici-builder-release',
                   'release': 'medici-builder-public-release'}
GENERAL_CI_RUNNER = 'medici-builder-public'


def job_runner(block):
    """The job's runner label, or None unless it is ONE literal job-level label."""
    bindings = re.findall(r'(?m)^( *)runs-on:(.*)$', block)
    if len(bindings) != 1 or bindings[0][0] != '    ':
        return None
    label = bindings[0][1].strip()
    return label if re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*', label) else None


def runner_binding_problems(text):
    """Each privileged release job names its own fixed label, never an expression.

    A line in either job that workflow_keys cannot read is refused, so a second
    binding in another spelling (a quoted key, say) cannot replace the label.
    """
    jobs = workflow_jobs(text)
    problems = workflow_keys.unread_problems(workflow_keys.read(text)[1], *RELEASE_RUNNERS)
    for name, label in RELEASE_RUNNERS.items():
        block = jobs.get(name, '')
        if re.search(r'(?m)^ *runs-on:.*\$\{\{', block):
            problems.append(name + ' must not select its runner through an expression')
        if job_runner(block) != label:
            problems.append(name + ' must bind exactly the fixed runner label ' + label)
    return problems


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
    if not {'resolve', 'guard', 'test', 'artifacts', 'runner-arm64'} <= needs:
        problems.append('publication must wait for all preparation gates')
    if job_runner(publisher) != RELEASE_RUNNERS['release']:
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
        'path: ${{ runner.temp }}/release-native-arm64-in',
        'NATIVE: ${{ runner.temp }}/release-native-arm64-in',
        "native=os.environ['NATIVE']",
        "trap 'rm -rf \"$DOCKER_CONFIG\" \"$SOURCE\"' EXIT",
    ]
    for value in required:
        if value not in publisher:
            problems.append('missing publication guard: ' + value)
    return problems


MUTATION_REPORTS = {
    'internal/deskkit/mutations.json': 'bodycheck.report',
    'cmd/desksourceguard/mutations.json': 'desksourceguard.report',
    'internal/loopengine/mutations.json': 'loopengine-dedupe.report',
    'internal/loopengine/mutations-retry.json': 'loopengine-retry.report',
    'cmd/reviewloop/mutations.json': 'reviewloop.report',
    'cmd/deskclose/mutations.json': 'deskclose.report',
    'cmd/deskmerge/mutations.json': 'deskmerge.${MUHAR_SHARD%%/*}.report',
}
DESKMERGE = 'cmd/deskmerge/mutations.json'
UPLOAD = 'Upload captured mutation reports (tool-validation pack)'
TEST_DEPENDENTS = ('artifacts', 'runner-arm64', 'release')
# Every release test leg, in order; muhar-light and the deskmerge shards are the
# mutation harnesses, the rest are the suites a release must also pass.
TEST_MATRIX = [('statusgen', ''), ('desk-tools', ''), ('drainloop', ''), ('qualgen', ''),
               ('muhar-light', ''), ('deskmerge', '0/3'), ('deskmerge', '1/3'),
               ('deskmerge', '2/3')]
# The whole live body of one mutation harness step. Anything added, dropped or
# reordered fails the match, so the checks below never rest on a substring.
HARNESS_STEP = re.compile(
    r"        if: matrix\.leg == '(?P<leg>[^']+)'\n"
    r"(?P<env>        env:\n          MUHAR_SHARD: \$\{\{ matrix\.shard \}\}\n)?"
    r"        run: \|\n"
    r"          set -euo pipefail\n"
    r"          cd tools/desk\n"
    r"          report=\$\(go run \./cmd/muhar -j 0 (?P<shard>-shard \"\$MUHAR_SHARD\" )?"
    r"-spec (?P<spec>[a-zA-Z0-9_./-]+\.json)\)\n"
    r"          echo \"\$report\"\n"
    r"          mkdir -p \"\$\{RUNNER_TEMP\}/tv-reports\"\n"
    r"          printf '%s\\n' \"\$report\" > \"\$\{RUNNER_TEMP\}/tv-reports/(?P<report>[^\"\n]+)\"\n"
    r"          if ! grep -qE 'Totals: \[0-9\]\+ caught, 0 NOT CAUGHT, 0 could-not-mutate' <<<\"\$report\"; then\n"
    r"            echo \"::error::[^\"$`\;&|\n]*\"\n"
    r"            exit (?P<exit>[0-9]+)\n"
    r"          fi\n*")
UPLOAD_LEGS = re.compile(r"matrix\.leg == '[^']+'(?: \|\| matrix\.leg == '[^']+')*")


def live_lines(block):
    """BLOCK without its comment lines: a comment never holds a setting."""
    return ''.join(line for line in block.splitlines(True) if not line.lstrip().startswith('#'))


def publish_wait_problems(entries):
    """Each publishing job still needs `test` and carries no job-level `if`.

    Any job-level condition is refused, not only one that names a status
    function: the three jobs have none, and a condition is the one setting
    that lets a job run after a dependency failed.
    """
    problems = []
    for name in TEST_DEPENDENTS:
        job = [entry for entry in entries if entry.path == ('jobs', name)]
        needs = [entry.value for entry in job if entry.key == 'needs']
        items = workflow_keys.flow_items(needs[0]) if len(needs) == 1 else None
        if not items or 'test' not in items:
            problems.append(name + ' must wait for the test job')
        if any(entry.key == 'if' for entry in job):
            problems.append(name + ' must not run past a failed test job')
    return problems


def matrix_legs(entries):
    """The test job's matrix as (leg, shard) pairs, or None if it is not one list."""
    where = ('jobs', 'test', 'strategy', 'matrix')
    if [entry.key for entry in entries if entry.path == where] != ['include']:
        return None
    matrix = []
    for entry in entries:
        if entry.path != where + ('include',):
            continue
        if entry.item:
            matrix.append({})
        if not matrix or entry.key not in ('leg', 'shard'):
            return None
        matrix[-1][entry.key] = entry.value
    return [(item.get('leg', ''), item.get('shard', '')) for item in matrix]


def step_text(text, entries, line):
    """The text of the test job step that holds LINE."""
    starts = [entry.line for entry in entries
              if entry.path == ('jobs', 'test', 'steps') and entry.item]
    start = max([number for number in starts if number <= line], default=line)
    end = min([number for number in starts if number > start], default=len(text) + 1)
    return '\n'.join(text.splitlines()[start - 1:end - 1])


def mutation_routing_problems(text):
    """Expand the release test matrix and bind each mutation harness to its report.

    muhar still owns baselines, controls and per-mutant verdicts. This checks
    that the test matrix is exactly TEST_MATRIX, that scheduling neither omits
    nor duplicates a full harness or a deskmerge shard, that every scheduled
    harness uploads its report, that each harness keeps its verdict gate and
    report name, and that no failure in the test job can be tolerated on the
    way to publication. Settings are read with workflow_keys, so a setting kept
    only in a comment counts as absent, and any line at top level or in the
    test or publishing jobs written in a form the reader does not accept is
    refused instead of being skipped.
    """
    entries, unread = workflow_keys.read(text)
    problems = workflow_keys.unread_problems(unread, 'test', *TEST_DEPENDENTS)
    problems += publish_wait_problems(entries)
    jobs = workflow_jobs(text)
    test = live_lines(jobs.get('test', ''))
    keys = [entry for entry in entries if entry.under('jobs', 'test')]
    matrix = matrix_legs(entries) or []
    if not matrix or len(matrix) != len(set(matrix)):
        problems.append('test matrix must contain unique leg/shard entries')
    if matrix != TEST_MATRIX:
        problems.append('the test matrix must be exactly TEST_MATRIX')
    if any(entry.key == 'exclude' for entry in keys):
        problems.append('the test matrix must not exclude a leg')
    if [(entry.path, entry.value) for entry in keys if entry.key == 'fail-fast'] != [
            (('jobs', 'test', 'strategy'), 'false')]:
        problems.append('all matrix verdicts must run without fail-fast cancellation')
    if (any(entry.key == 'shell' for entry in keys)
            or any(entry.key == 'defaults' and entry.path in ((), ('jobs', 'test'))
                   for entry in entries)):
        problems.append('the test job must run its steps in the default shell')
    for entry in keys:
        if entry.key != 'continue-on-error':
            continue
        if entry.path == ('jobs', 'test'):
            problems.append('the test job must not tolerate a failed leg')
            continue
        spec = re.search(r'-spec (\S+\.json)', step_text(text, entries, entry.line))
        problems.append('mutation harness step must not tolerate failure: ' + spec.group(1)
                        if spec else 'a test job step must not tolerate failure')
    steps = workflow_steps(test)
    upload = steps.get(UPLOAD, '')
    conditions = re.findall(r'(?m)^        if: (.*)$', upload)
    upload_legs = set()
    if len(conditions) == 1 and UPLOAD_LEGS.fullmatch(conditions[0]):
        upload_legs = set(re.findall(r"matrix\.leg == '([^']+)'", conditions[0]))
    else:
        problems.append('the report upload must run on exactly the legs it names')
    if re.findall(r'(?m)^ *if-no-files-found:.*$', upload) != ['          if-no-files-found: error']:
        problems.append('the report upload must fail when it finds no files')
    if (not re.search(r'(?m)^        uses: actions/upload-artifact@', upload)
            or not re.search(r'(?m)^          name: tv-reports-\$\{\{ matrix\.leg \}\}'
                             r'-\$\{\{ strategy\.job-index \}\}$', upload)):
        problems.append('each mutation leg must upload its reports without collision or omission')
    actual = Counter()
    for name, step in steps.items():
        specs = re.findall(r'-spec ([a-zA-Z0-9_./-]+\.json)', step)
        if not specs:
            continue
        if len(specs) != 1 or specs[0] not in MUTATION_REPORTS:
            problems.append('unknown or ambiguous mutation harness: ' + name)
            continue
        spec = specs[0]
        gate = HARNESS_STEP.fullmatch(step)
        if not gate or gate.group('spec') != spec:
            problems.append('mutation verdict gate changed: ' + spec)
            continue
        if gate.group('exit') != '1':
            problems.append('a surviving mutation must fail its harness step: ' + spec)
        if gate.group('report') != MUTATION_REPORTS[spec]:
            problems.append('mutation report identity changed: ' + spec)
        leg = gate.group('leg')
        if leg not in upload_legs:
            problems.append('mutation harness report is not uploaded: ' + spec)
        sharded = bool(gate.group('env')) and bool(gate.group('shard'))
        if spec != DESKMERGE and (gate.group('env') or gate.group('shard')):
            problems.append('full mutation harness unexpectedly sharded: ' + spec)
        for entry, shard in matrix:
            if entry != leg:
                continue
            if spec == DESKMERGE:
                if shard not in ('0/3', '1/3', '2/3') or not sharded:
                    problems.append('deskmerge must retain its exact three shards')
                    continue
                actual['deskmerge.' + shard[0] + '.report'] += 1
            else:
                if shard:
                    problems.append('full mutation harness unexpectedly sharded: ' + spec)
                actual[MUTATION_REPORTS[spec]] += 1
    expected = Counter(report for spec, report in MUTATION_REPORTS.items() if spec != DESKMERGE)
    expected.update('deskmerge.' + str(i) + '.report' for i in range(3))
    if actual != expected:
        problems.append('matrix must execute every full harness and deskmerge shard exactly once')
    return problems


def native_transport_problems(text):
    """runner-arm64 reads every build artifact and fails on an empty upload."""
    native = workflow_jobs(text).get('runner-arm64', '')
    problems = []
    downloads = [step for step in re.findall(r'(?ms)^      - (.*?)(?=^      - |\Z)', native)
                 if step.startswith('uses: actions/download-artifact@')]
    if not any(re.search(r'(?m)^          pattern: release-build-\*$', step)
               and re.search(r'(?m)^          merge-multiple: true$', step) for step in downloads):
        problems.append('runner-arm64 must download every release-build-* artifact, merged')
    upload = workflow_steps(native).get('Upload checked native arm64 image', '')
    if not re.search(r'(?m)^          if-no-files-found: error$', upload):
        problems.append('the native arm64 upload must fail when it finds no files')
    return problems


def ci_contract_step(text):
    return workflow_steps(workflow_jobs(text).get('build-test-shard', '')).get(
        'Test release image contract', '')


def mutate_job(text, job, old, new):
    """Replace OLD with NEW inside one job only; refuse an ambiguous target."""
    block = workflow_jobs(text)[job]
    if text.count(block) != 1 or block.count(old) != 1:
        raise AssertionError('control target is not unique: ' + job + ': ' + old)
    return text.replace(block, block.replace(old, new, 1), 1)


def plant(text, job, old, new):
    """Apply one plant: inside JOB, or appended to the workflow when JOB is None."""
    return text + new if job is None else mutate_job(text, job, old, new)


UNREAD = 'is not in the plain form this check reads: '
# Plants that write a weakening in a spelling YAML accepts but the round-two
# gate did not read. Each must still draw a refusal containing its problem.
SPELLING_PLANTS = {
    'release-if-block-scalar': (
        'release', '    needs: [resolve, guard, test, artifacts, runner-arm64]\n',
        '    needs: [resolve, guard, test, artifacts, runner-arm64]\n    if: >-\n      always()\n',
        'release must not run past a failed test job'),
    'artifacts-if-block-scalar': (
        'artifacts', '    needs: [resolve, guard, test]\n',
        '    needs: [resolve, guard, test]\n    if: >-\n      !cancelled()\n',
        'artifacts must not run past a failed test job'),
    'native-if-spaced-call': (
        'runner-arm64', '    needs: [resolve, guard, test, artifacts]\n',
        '    needs: [resolve, guard, test, artifacts]\n    if: always ()\n',
        'runner-arm64 must not run past a failed test job'),
    'release-if-quoted-key': (
        'release', '    needs: [resolve, guard, test, artifacts, runner-arm64]\n',
        '    needs: [resolve, guard, test, artifacts, runner-arm64]\n    "if": always()\n',
        UNREAD + '"if": always()'),
    'needs-test-in-comment': (
        'artifacts', '    needs: [resolve, guard, test]\n', '    needs: [resolve, guard] #,test]\n',
        'artifacts must wait for the test job'),
    'release-needs-in-comment': (
        'release', '    needs: [resolve, guard, test, artifacts, runner-arm64]\n',
        '    needs: [resolve, guard, artifacts, runner-arm64] # test\n',
        'release must wait for the test job'),
    'job-tolerates-quoted-key': (
        'test', '    runs-on: medici-builder-release\n',
        '    runs-on: medici-builder-release\n    "continue-on-error": true\n',
        UNREAD + '"continue-on-error": true'),
    'job-tolerates-spaced-key': (
        'test', '    runs-on: medici-builder-release\n',
        '    runs-on: medici-builder-release\n    continue-on-error : true\n',
        UNREAD + 'continue-on-error : true'),
    'job-tolerates-other-case': (
        'test', '    runs-on: medici-builder-release\n',
        '    runs-on: medici-builder-release\n    Continue-On-Error: true\n',
        'the test job must not tolerate a failed leg'),
    'job-tolerates-merge-key': (
        'test', '    runs-on: medici-builder-release\n',
        '    runs-on: medici-builder-release\n    <<: *tolerate\n', UNREAD + '<<: *tolerate'),
    'upload-tolerates-failure': (
        'test', "        if: matrix.leg == 'muhar-light' || matrix.leg == 'deskmerge'\n",
        "        if: matrix.leg == 'muhar-light' || matrix.leg == 'deskmerge'\n"
        '        continue-on-error: true\n',
        'a test job step must not tolerate failure'),
    'upload-tolerates-other-case': (
        'test', "        if: matrix.leg == 'muhar-light' || matrix.leg == 'deskmerge'\n",
        "        if: matrix.leg == 'muhar-light' || matrix.leg == 'deskmerge'\n"
        '        Continue-On-Error: true\n',
        'a test job step must not tolerate failure'),
    'harness-shell-quoted-key': (
        'test', "every bodycheck refusal and require the suite to catch it\n        if: matrix.leg == 'muhar-light'\n",
        "every bodycheck refusal and require the suite to catch it\n        if: matrix.leg == 'muhar-light'\n"
        "        'shell': bash -c 'exit 0' {0}\n",
        UNREAD + "'shell': bash -c 'exit 0' {0}"),
    'defaults-after-jobs': (
        None, '', "defaults:\n  run:\n    shell: bash -c 'exit 0' {0}\n",
        'the test job must run its steps in the default shell'),
    'statusgen-leg-dropped': (
        'test', '          - leg: statusgen\n', '', 'the test matrix must be exactly TEST_MATRIX'),
    'qualgen-leg-dropped': (
        'test', '          - leg: qualgen\n', '', 'the test matrix must be exactly TEST_MATRIX'),
    'desk-tools-leg-renamed': (
        'test', '          - leg: desk-tools\n', '          - leg: desk-tool\n',
        'the test matrix must be exactly TEST_MATRIX'),
    'matrix-dimension-added': (
        'test', '        include:\n', '        leg: [statusgen]\n        include:\n',
        'the test matrix must be exactly TEST_MATRIX'),
}


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
        self.native = self.root / 'native-in'
        self.native.mkdir()
        self.native_archive = self.native / 'verify-runner-linux-arm64.tar'
        native_archive(self.native_archive)
        self.native_receipt = self.native / 'verify-runner-linux-arm64.json'
        self.native_receipt.write_text(json.dumps({
            'schema': 'native-runner-v1', 'version': VERSION,
            'source_revision': SOURCE, 'platform': 'linux/arm64',
            'ref': 'ghcr.io/example/verify-runner:v1.2.3-linux-arm64',
            'image_id': NATIVE_ID,
            'archive_sha256': hashlib.sha256(self.native_archive.read_bytes()).hexdigest(),
        }))
        bindir = self.root / 'bin'
        bindir.mkdir()
        docker = bindir / 'docker'
        docker.write_text('''#!/usr/bin/env python3
import hashlib, io, json, os, sys, tarfile
args = sys.argv[1:]
with open(os.environ['CALLS'], 'a') as f: f.write(json.dumps(args)+'\\n')
mode = os.environ.get('MODE', '')
def digest(ref): return 'sha256:' + hashlib.sha256(ref.encode()).hexdigest()
ARCHES = {'linux/amd64': '-linux-amd64', 'linux/arm64': '-linux-arm64'}
if args[:1] == ['info']:
    print('linux/amd64' if mode == 'native-wrong-host' else 'linux/aarch64'); sys.exit(0)
if args[:1] == ['save']:
    from pathlib import Path
    config = b'{"architecture":"arm64","os":"linux"}'
    config_name = hashlib.sha256(config).hexdigest() + '.json'
    files = {'manifest.json': json.dumps([{'Config': config_name, 'RepoTags': [args[-1]], 'Layers': []}]).encode(), config_name: config}
    with tarfile.open(args[args.index('--output') + 1], 'w') as archive:
        for name, data in files.items():
            info = tarfile.TarInfo(name); info.size = len(data)
            archive.addfile(info, io.BytesIO(data))
    sys.exit(0)
if args[:2] == ['image', 'inspect'] and args[3] == '{{.Id}}':
    print('sha256:' + ('e'*64 if mode == 'native-id-mismatch' else hashlib.sha256(b'{"architecture":"arm64","os":"linux"}').hexdigest())); sys.exit(0)
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

    def invoke(self, publish=False, mode='', native=None):
        code = ('from pathlib import Path; import sys; from images import build_images; '
                'build_images(Path(sys.argv[1]), *sys.argv[2:8], sys.argv[8] == "publish", '
                'native=sys.argv[9])')
        return subprocess.run(['python3', '-c', code, str(self.root), VERSION, SOURCE,
                               'ghcr.io/example/assay', 'ghcr.io/example/verify-runner',
                               str(self.inputs), str(self.out), 'publish' if publish else 'build',
                               str(native or self.native)],
                              cwd=self.script.parent,
                              env=dict(self.env, MODE=mode), text=True, capture_output=True)

    def invoke_native(self, mode=''):
        code = ('from pathlib import Path; import sys; from images import build_native_runner; '
                'build_native_runner(Path(sys.argv[1]), *sys.argv[2:5], '
                'Path(sys.argv[5]), Path(sys.argv[6]))')
        return subprocess.run(['python3', '-c', code, str(self.root), VERSION, SOURCE,
                               'ghcr.io/example/verify-runner', str(self.inputs), str(self.out)],
                              cwd=self.script.parent, env=dict(self.env, MODE=mode),
                              text=True, capture_output=True)

    def recorded(self):
        return self.calls.read_text().splitlines() if self.calls.exists() else []

    # regression: #3469
    def test_python_before_publish(self):
        result = self.invoke(True)
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        probes = [c for c in calls if c[0] == 'run' and 'python3' in c]
        # The publisher probes amd64; native arm64 is checked before transfer.
        self.assertEqual([(p[p.index('--platform') + 1], p[p.index('python3') + 1]) for p in probes],
                         [('linux/amd64', 'ghcr.io/example/verify-runner:v1.2.3-linux-amd64')])
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
        # The native job checks arm64; this job checks eight amd64 images
        # and the amd64 runner, plus its capability probe.
        self.assertEqual(sum('"run"' in s for s in calls), 10)
        self.assertIn('DESK_TOOLS_IMAGE=ghcr.io/example/assay/desk-tools:v1.2.3', '\n'.join(calls))
        self.assertEqual((self.root / '.release-images/harness/statusgen').read_bytes(),
                         (self.inputs / 'statusgen-linux-amd64').read_bytes())
        self.assertEqual((self.root / '.release-images/verify-runner/statusgen-linux-amd64').read_bytes(),
                         (self.inputs / 'statusgen-linux-amd64').read_bytes())
        self.assertFalse((self.root / '.release-images/harness/statusgen-linux-arm64').exists())

    # regression: #4323
    def test_publisher_builds_only_amd64_and_imports_native_arm64(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        builds = [c for c in calls if c[0] == 'build' and c[-2].endswith('images/verify-runner/Dockerfile')]
        self.assertEqual([(c[c.index('--platform') + 1], c[c.index('-t') + 1]) for c in builds],
                         [('linux/amd64', 'ghcr.io/example/verify-runner:v1.2.3-linux-amd64')])
        versions = [c for c in calls if c[0] == 'run' and 'statusgen' in c and 'verify-runner' in c[-2]]
        self.assertEqual([(c[c.index('--platform') + 1], c[-2]) for c in versions],
                         [('linux/amd64', 'ghcr.io/example/verify-runner:v1.2.3-linux-amd64')])
        self.assertFalse(any('ghcr.io/example/verify-runner:v1.2.3' in c for c in calls))
        loads = [c for c in calls if c[:1] == ['load']]
        self.assertEqual(len(loads), 1)
        self.assertIn(str(self.native_archive.resolve()), loads[0])

    # regression: #4323
    def test_runner_platform_builds_push_a_single_manifest(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        builds = [c for c in calls if c[0] == 'build' and c[-2].endswith('images/verify-runner/Dockerfile')]
        self.assertEqual(len(builds), 1)
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
        self.assertNotEqual(self.invoke_native().returncode, 0)
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

    def test_native_arm64_build_checks_and_saves_without_publication(self):
        result = self.invoke_native()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        self.assertEqual(calls[0], ['info', '--format', '{{.OSType}}/{{.Architecture}}'])
        builds = [c for c in calls if c[0] == 'build']
        self.assertEqual(len(builds), 1)
        self.assertEqual(builds[0][builds[0].index('--platform') + 1], 'linux/arm64')
        self.assertIn('--provenance=false', builds[0])
        self.assertIn('--sbom=false', builds[0])
        runs = [c for c in calls if c[0] == 'run']
        self.assertEqual(len(runs), 2)
        self.assertTrue(all(c[c.index('--platform') + 1] == 'linux/arm64' for c in runs))
        self.assertTrue(any('statusgen' in c for c in runs))
        probe = next(c for c in runs if 'python3' in c)
        for flag in ('--read-only', '--cap-drop', '--security-opt', '--user', '--pids-limit',
                     '--memory', '--cpus', '--tmpfs'):
            self.assertIn(flag, probe)
        self.assertEqual(probe[probe.index('--network') + 1], 'none')
        self.assertEqual(probe[probe.index('--cap-drop') + 1], 'ALL')
        self.assertEqual(probe[probe.index('--security-opt') + 1], 'no-new-privileges')
        self.assertEqual(probe[probe.index('--user') + 1], '1000:1000')
        self.assertNotIn('--privileged', probe)
        self.assertNotIn('--mount', probe)
        self.assertNotIn('--volume', probe)
        save = next(i for i, c in enumerate(calls) if c[0] == 'save')
        self.assertLess(max(calls.index(c) for c in runs), save)
        self.assertFalse(any(c[0] in ('push', 'manifest', 'login') for c in calls))
        receipt = json.loads((self.out / 'verify-runner-linux-arm64.json').read_text())
        expected = json.loads(self.native_receipt.read_text())
        self.assertEqual(receipt, expected)
        self.assertEqual((self.out / 'verify-runner-linux-arm64.tar').read_bytes(),
                         self.native_archive.read_bytes())

    def test_native_arm64_failures_never_export_or_publish(self):
        for mode in ('native-wrong-host', 'wrong-arch', 'missing-yaml', 'missing-jsonschema'):
            with self.subTest(mode=mode):
                self.setUp()
                result = self.invoke_native(mode)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
                self.assertFalse(any(c[0] in ('save', 'push', 'manifest', 'login') for c in calls))
                self.assertFalse((self.out / 'verify-runner-linux-arm64.json').exists())
                if mode == 'native-wrong-host':
                    self.assertFalse(any(c[0] in ('build', 'run') for c in calls))

    def test_native_receipt_and_archive_must_match_before_import(self):
        for field in ('schema', 'version', 'source_revision', 'platform', 'ref',
                      'image_id', 'archive_sha256', 'missing-archive', 'tampered-archive'):
            with self.subTest(field=field):
                self.setUp()
                receipt = json.loads(self.native_receipt.read_text())
                if field == 'missing-archive':
                    self.native_archive.unlink()
                elif field == 'tampered-archive':
                    self.native_archive.write_bytes(b'changed native bytes')
                else:
                    receipt[field] = 'incorrect'
                    self.native_receipt.write_text(json.dumps(receipt))
                result = self.invoke(True)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
                self.assertFalse(any(c[0] in ('load', 'push') for c in calls))
                self.assertFalse((self.out / 'release-images.json').exists())

    def test_native_archive_cannot_replace_other_image_tags(self):
        for mode in ('extra-image', 'wrong-tag', 'wrong-config'):
            with self.subTest(mode=mode):
                self.setUp()
                native_archive(self.native_archive,
                               ref='ghcr.io/example/other:old' if mode == 'wrong-tag' else
                                   'ghcr.io/example/verify-runner:v1.2.3-linux-arm64',
                               extra=mode == 'extra-image')
                if mode == 'wrong-config':
                    with tarfile.open(self.native_archive, 'a') as archive:
                        info = tarfile.TarInfo(NATIVE_ID.split(':')[1] + '.json')
                        content = b'changed config'
                        info.size = len(content)
                        archive.addfile(info, io.BytesIO(content))
                receipt = json.loads(self.native_receipt.read_text())
                receipt['archive_sha256'] = hashlib.sha256(self.native_archive.read_bytes()).hexdigest()
                self.native_receipt.write_text(json.dumps(receipt))
                result = self.invoke(True)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
                self.assertFalse(any(c[0] in ('load', 'push') for c in calls))

    def test_each_native_import_refusal_fires_before_any_load(self):
        # Each case is well formed except for the one property under test, and
        # names the refusal that must fire, so another guard cannot satisfy it.
        config_name = NATIVE_ID.split(':')[1] + '.json'
        manifest = json.dumps([{'Config': config_name, 'Layers': [],
                                'RepoTags': ['ghcr.io/example/verify-runner:v1.2.3-linux-arm64']}]).encode()

        def save(members):
            with tarfile.open(self.native_archive, 'w') as archive:
                for name, content in members:
                    info = tarfile.TarInfo(name)
                    info.size = len(content)
                    archive.addfile(info, io.BytesIO(content))

        cases = {
            'config-differs': ([('manifest.json', manifest), (config_name, b'{"architecture":"amd64"}')],
                               None, 'config differs from the checked image'),
            'checksum-differs': ([('manifest.json', manifest), (config_name, NATIVE_CONFIG)],
                                 hashlib.sha256(b'other bytes').hexdigest(),
                                 'archive checksum mismatch'),
            'two-manifests': ([('manifest.json', manifest), ('manifest.json', manifest),
                               (config_name, NATIVE_CONFIG)],
                              None, 'no single Docker save manifest'),
        }
        for name, (members, digest, message) in cases.items():
            with self.subTest(case=name):
                self.setUp()
                save(members)
                receipt = json.loads(self.native_receipt.read_text())
                receipt['archive_sha256'] = digest or hashlib.sha256(self.native_archive.read_bytes()).hexdigest()
                self.native_receipt.write_text(json.dumps(receipt))
                result = self.invoke(True)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertIn(message, result.stderr)
                calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
                self.assertFalse(any(c[0] in ('load', 'push') for c in calls))

    def test_imported_native_image_identity_must_match_before_publication(self):
        result = self.invoke(True, 'native-id-mismatch')
        self.assertNotEqual(result.returncode, 0, result.stdout)
        calls = [json.loads(s) for s in self.recorded() if s.startswith('[')]
        self.assertTrue(any(c[0] == 'load' for c in calls))
        self.assertFalse(any(c[0] == 'push' for c in calls))
        self.assertFalse((self.out / 'release-images.json').exists())

    def test_native_transport_must_hold_exactly_the_archive_and_receipt(self):
        # S1: the transported files are admitted from a directory of their own.
        # Anything beside them, a missing file or a link refuses before any load.
        extra = self.native / 'checksums.txt'
        extra.write_text('replacement')
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertFalse(any(c[:1] == ['load'] for c in
                             (json.loads(s) for s in self.recorded() if s.startswith('['))))
        extra.unlink()
        moved = self.root / 'moved.json'
        self.native_receipt.rename(moved)
        self.assertNotEqual(self.invoke().returncode, 0)
        self.native_receipt.symlink_to(moved)
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('regular files', result.stderr)
        self.native_receipt.unlink()
        moved.rename(self.native_receipt)
        self.assertEqual(self.invoke().returncode, 0)

    def test_native_transport_shared_with_the_verified_inputs_is_refused(self):
        for name in ('verify-runner-linux-arm64.tar', 'verify-runner-linux-arm64.json'):
            (self.inputs / name).write_bytes((self.native / name).read_bytes())
        result = self.invoke(native=self.inputs)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('exactly the archive and its receipt', result.stderr)
        self.assertFalse(any(c[:1] == ['load'] for c in
                             (json.loads(s) for s in self.recorded() if s.startswith('['))))

    def test_native_arm64_workflow_precedes_the_approved_publisher(self):
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            with self.subTest(path=path):
                text = (HERE.parents[1] / path).read_text()
                jobs = workflow_jobs(text)
                native = jobs['runner-arm64']
                self.assertIn('runs-on: ubuntu-24.04-arm', native)
                self.assertIn('needs: [resolve, guard, test, artifacts]', native)
                self.assertNotIn('environment: release', native)
                self.assertNotRegex(native, r'(?m)^      (contents|packages|id-token): write$')
                step = workflow_steps(native)['Build and check native arm64 verify-runner']
                self.assertIn('build_native_runner', step)
                self.assertNotRegex(step, r'(?m)^        if:', 'dry runs must build native arm64 too')
                for name in ('verify-runner-linux-arm64.tar', 'verify-runner-linux-arm64.json'):
                    self.assertIn(name, native)
                self.assertIn('name: release-runner-arm64', native)
                publisher = jobs['release']
                self.assertIn('runner-arm64', publisher.split('    runs-on:', 1)[0])
                self.assertIn('name: release-runner-arm64', publisher)
                self.assertLess(publisher.index('name: release-runner-arm64'),
                                publisher.index('name: Build, verify and publish the release images'))
                download = workflow_steps(publisher)['Download checked native arm64 image']
                self.assertIn('path: ${{ runner.temp }}/release-native-arm64-in', download)
                self.assertNotRegex(download, r'(?m)^          path: (\.|\$\{\{ github\.workspace \}\})$',
                                    'the native image must not land in the verified inputs')
                self.assertNotIn('BINFMT_IMAGE', text)
                self.assertNotIn('--privileged', text)

    # regression: #5091
    def test_mutation_matrix_preserves_all_harnesses_and_reports(self):
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            with self.subTest(path=path):
                self.assertEqual(mutation_routing_problems((HERE.parents[1] / path).read_text()), [])

    # regression: #5091
    def test_mutation_routing_omission_and_duplication_controls(self):
        main = (HERE.parents[1] / '.github/workflows/release.yml').read_text()
        self.assertEqual(mutation_routing_problems(main), [])
        bodycheck = "every bodycheck refusal and require the suite to catch it\n        if: matrix.leg == 'muhar-light'\n"
        once = 'matrix must execute every full harness and deskmerge shard exactly once'
        gate = 'internal/deskkit/mutations.json'
        mutants = {
            'missing-light-leg': ('          - leg: muhar-light\n', '', once),
            'duplicate-light-leg': ('          - leg: muhar-light\n',
                                    '          - leg: muhar-light\n          - leg: muhar-light\n',
                                    'test matrix must contain unique leg/shard entries'),
            'missing-deskmerge-shard': ('          - leg: deskmerge\n            shard: 2/3\n', '', once),
            'changed-shard-count': ('            shard: 2/3\n', '            shard: 2/4\n',
                                    'deskmerge must retain its exact three shards'),
            'fail-fast': ('      fail-fast: false\n', '      fail-fast: true\n',
                          'all matrix verdicts must run without fail-fast cancellation'),
            'missing-light-upload': ("matrix.leg == 'muhar-light' || ", '',
                                     'mutation harness report is not uploaded: ' + gate),
            'upload-name-collision': ('tv-reports-${{ matrix.leg }}-${{ strategy.job-index }}',
                                      'tv-reports-${{ matrix.leg }}',
                                      'each mutation leg must upload its reports without collision or omission'),
            'misrouted-bodycheck': (bodycheck, bodycheck.replace("'muhar-light'", "'statusgen'"),
                                    'mutation harness report is not uploaded: ' + gate),
            'missing-caught-gate': ("grep -qE 'Totals: [0-9]+ caught, 0 NOT CAUGHT, 0 could-not-mutate' <<<\"$report\"; then\n"
                                    "            echo \"::error::a bodycheck",
                                    "grep -qE 'Totals:' <<<\"$report\"; then\n"
                                    "            echo \"::error::a bodycheck",
                                    'mutation verdict gate changed: ' + gate),
            'renamed-report': ('/tv-reports/bodycheck.report', '/tv-reports/other.report',
                               'mutation report identity changed: ' + gate),
        }
        for name, (old, new, problem) in mutants.items():
            with self.subTest(name=name):
                self.assertIn(problem, mutation_routing_problems(mutate_job(main, 'test', old, new)))

    # regression: #5091
    def test_routing_gate_weakening(self):
        # Each plant weakens the gate in one way only, often keeping the
        # correct text in a comment, and must draw its own named refusal.
        harness = 'internal/deskkit/mutations.json'
        light = "every bodycheck refusal and require the suite to catch it\n        if: matrix.leg == 'muhar-light'\n"
        grep = ("          if ! grep -qE 'Totals: [0-9]+ caught, 0 NOT CAUGHT, 0 could-not-mutate' "
                "<<<\"$report\"; then\n            echo \"::error::a bodycheck")
        upload_if = "        if: matrix.leg == 'muhar-light' || matrix.leg == 'deskmerge'\n"
        plants = {
            'job-tolerates-failure': ('test', '    runs-on: medici-builder-release\n',
                                      '    runs-on: medici-builder-release\n    continue-on-error: true\n',
                                      'the test job must not tolerate a failed leg'),
            'harness-tolerates-failure': ('test', light, light + '        continue-on-error: true\n',
                                          'mutation harness step must not tolerate failure: ' + harness),
            'shard-tolerates-failure': ('test', "        if: matrix.leg == 'deskmerge'\n",
                                        "        if: matrix.leg == 'deskmerge'\n"
                                        "        continue-on-error: ${{ matrix.shard == '2/3' }}\n",
                                        'mutation harness step must not tolerate failure: '
                                        'cmd/deskmerge/mutations.json'),
            'suite-tolerates-failure': ('test', "        if: matrix.leg == 'desk-tools'\n",
                                        "        if: matrix.leg == 'desk-tools'\n        continue-on-error: true\n",
                                        'a test job step must not tolerate failure'),
            'survivor-exits-zero': ('test', 'not load-bearing as written (docs/desk-tools-gate-bar.md)"\n'
                                            '            exit 1\n',
                                    'not load-bearing as written (docs/desk-tools-gate-bar.md)"\n'
                                    '            exit 0\n',
                                    'a surviving mutation must fail its harness step: ' + harness),
            'grep-weakened-shadowed': ('test', grep,
                                       grep.replace('          if', '          # if', 1)
                                       .replace('\n            echo', "\n          if ! grep -qE 'Totals:' "
                                                '<<<"$report"; then\n            echo', 1),
                                       'mutation verdict gate changed: ' + harness),
            'harness-shell-override': ('test', light, light + "        shell: bash -c 'exit 0' {0}\n",
                                       'the test job must run its steps in the default shell'),
            'fail-fast-shadowed': ('test', '      fail-fast: false\n',
                                   '      fail-fast: true\n      # fail-fast: false\n',
                                   'all matrix verdicts must run without fail-fast cancellation'),
            'leg-excluded': ('test', '      matrix:\n',
                             '      matrix:\n        exclude:\n          - leg: muhar-light\n',
                             'the test matrix must not exclude a leg'),
            'upload-warns-inline': ('test', '          if-no-files-found: error\n',
                                    '          if-no-files-found: warn # if-no-files-found: error\n',
                                    'the report upload must fail when it finds no files'),
            'upload-warns-shadowed': ('test', '          if-no-files-found: error\n',
                                      '          if-no-files-found: warn\n          # if-no-files-found: error\n',
                                      'the report upload must fail when it finds no files'),
            'upload-disabled': ('test', upload_if,
                                upload_if.replace('if: ', 'if: false && (', 1).replace('\n', ')\n', 1),
                                'the report upload must run on exactly the legs it names'),
            'artifacts-skips-test': ('artifacts', '    needs: [resolve, guard, test]\n',
                                     '    needs: [resolve, guard]\n', 'artifacts must wait for the test job'),
            'native-skips-test': ('runner-arm64', '    needs: [resolve, guard, test, artifacts]\n',
                                  '    needs: [resolve, guard, artifacts]\n',
                                  'runner-arm64 must wait for the test job'),
            'publisher-runs-anyway': ('release', '    needs: [resolve, guard, test, artifacts, runner-arm64]\n',
                                      '    needs: [resolve, guard, test, artifacts, runner-arm64]\n'
                                      '    if: ${{ always() }}\n',
                                      'release must not run past a failed test job'),
        }
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(mutation_routing_problems(text), [])
            for name, (job, old, new, problem) in plants.items():
                with self.subTest(path=path, plant=name):
                    self.assertIn(problem, mutation_routing_problems(mutate_job(text, job, old, new)))

    # regression: #5091
    def test_routing_other_spellings(self):
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(mutation_routing_problems(text), [])
            for name, (job, old, new, problem) in SPELLING_PLANTS.items():
                with self.subTest(path=path, plant=name):
                    problems = mutation_routing_problems(plant(text, job, old, new))
                    self.assertTrue(any(problem in found for found in problems), problems)

    # regression: #5091
    def test_release_runners_fixed(self):
        root = HERE.parents[1]
        ci = workflow_jobs((root / '.github/workflows/ci.yml').read_text())
        # The planted label is the one general CI really runs on.
        self.assertEqual(job_runner(ci['build-test-shard']), GENERAL_CI_RUNNER)
        self.assertNotIn(GENERAL_CI_RUNNER, RELEASE_RUNNERS.values())
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (root / path).read_text()
            with self.subTest(path=path):
                self.assertEqual(runner_binding_problems(text), [])
        text = (root / '.github/workflows/release.yml').read_text()
        for job, label in RELEASE_RUNNERS.items():
            line = '    runs-on: ' + label + '\n'
            for name, new in {
                'general-ci': '    runs-on: ' + GENERAL_CI_RUNNER + '\n',
                'expression': "    runs-on: ${{ vars.RELEASE_RUNNER || '" + label + "' }}\n",
                'label-list': '    runs-on: [' + label + ', ' + GENERAL_CI_RUNNER + ']\n',
                'shadowed': '    runs-on: ' + GENERAL_CI_RUNNER + '\n    # runs-on: ' + label + '\n',
                # regression: #5091 — a second binding under a quoted key.
                'quoted-rebind': line + '    "runs-on": ' + GENERAL_CI_RUNNER + '\n',
            }.items():
                with self.subTest(job=job, plant=name):
                    self.assertTrue(runner_binding_problems(mutate_job(text, job, line, new)))
        # A general-CI publisher also fails the publication gate on its own.
        self.assertTrue(publication_problems(mutate_job(
            text, 'release', '    runs-on: medici-builder-public-release\n',
            '    runs-on: ' + GENERAL_CI_RUNNER + '\n')))

    # regression: #5091
    def test_native_transport_held(self):
        root = HERE.parents[1]
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            with self.subTest(path=path):
                self.assertEqual(native_transport_problems((root / path).read_text()), [])
        text = (root / '.github/workflows/release.yml').read_text()
        for name, (old, new) in {
            'not-merged': ('          merge-multiple: true\n', ''),
            'merge-off': ('          merge-multiple: true\n', '          merge-multiple: false\n'),
            'one-platform': ('          pattern: release-build-*\n', '          pattern: release-build-common\n'),
            'empty-upload-passes': ('          if-no-files-found: error\n', '          if-no-files-found: warn\n'),
            'empty-upload-default': ('          if-no-files-found: error\n', ''),
        }.items():
            with self.subTest(name=name):
                self.assertTrue(native_transport_problems(mutate_job(text, 'runner-arm64', old, new)))

    # regression: #5091 — the CI step must also run the suite that checks the
    # step from outside this file (tools/release/ci_step_test.py).
    def test_ci_runs_step_suite(self):
        text = (HERE.parents[1] / '.github/workflows/ci.yml').read_text()
        step = ci_contract_step(text)
        self.assertRegex(step, r'(?m)^        run: python3 -B -m unittest [^\n]*'
                               r'(?<![\w.])tools\.release\.ci_step_test(?![\w.])')
        dropped = text.replace(' tools.release.ci_step_test', '', 1)
        self.assertNotEqual(dropped, text)
        self.assertNotRegex(ci_contract_step(dropped), r'tools\.release\.ci_step_test')

    def test_approval_and_publication_order_regressions_are_rejected(self):
        root = HERE.parents[1]
        main = (root / '.github/workflows/release.yml').read_text()
        self.assertEqual(publication_problems(main), [])
        mutants = {
            'default-write': main.replace('\npermissions:\n  contents: read\n',
                                           '\npermissions:\n  contents: write\n', 1),
            'missing-approval': main.replace('    environment: release\n', '', 1),
            'second-approval': main + '\n  second:\n    environment: release\n',
            'missing-tests': main.replace('needs: [resolve, guard, test, artifacts, runner-arm64]',
                                          'needs: [resolve, guard, artifacts, runner-arm64]', 1),
            'missing-native-dependency': main.replace('needs: [resolve, guard, test, artifacts, runner-arm64]',
                                                      'needs: [resolve, guard, test, artifacts]', 1),
            'missing-image-step': main.replace('Build, verify and publish the release images', 'removed', 1),
            'wrong-order': main.replace('Build, verify and publish the release images', 'TEMP', 1)
                              .replace('Create and push the dispatched tag',
                                       'Build, verify and publish the release images', 1)
                              .replace('TEMP', 'Create and push the dispatched tag', 1),
            'ignored-image-failure': main.replace('Build, verify and publish the release images\n',
                                                  'Build, verify and publish the release images\n        continue-on-error: true\n', 1),
            'native-into-workspace': main.replace('path: ${{ runner.temp }}/release-native-arm64-in', 'path: .', 1),
            'native-not-passed': main.replace("native=os.environ['NATIVE']", "native=os.environ['INPUT']", 1),
            'unguarded-writer': main + '\n  other:\n    permissions:\n      packages: write\n',
            'missing-ref-check': main.replace('from tools.release.images_contract import RUNNER_PLATFORM, validate', 'removed', 1),
            'lenient-runner-platform': main.replace('runner_platforms=(RUNNER_PLATFORM,)', '', 1),
            'missing-checksum': main.replace('sha256sum release-images.json verify-runner-image.txt >> checksums.txt', 'removed', 1),
            'dry-run-tag': main.replace("if: github.event_name == 'workflow_dispatch' && !inputs.dry_run", 'if: true', 1),
            'dry-run-release': main.replace("- name: Create GitHub release and upload assets\n        if: ${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
                                             '- name: Create GitHub release and upload assets\n        if: true', 1),
            'dry-run-ref-check': main.replace("- name: Validate and checksum the published image references\n        if: ${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
                                               '- name: Validate and checksum the published image references\n        if: true', 1),
            # regression: #5091 — the publisher runner is one exact job-level label.
            'publisher-general-ci': main.replace('    runs-on: medici-builder-public-release\n',
                                                 '    runs-on: medici-builder-public\n', 1),
            'publisher-label-suffix': main.replace('    runs-on: medici-builder-public-release\n',
                                                   '    runs-on: ' + RELEASE_RUNNERS['release'] + '-shared\n', 1),
            'publisher-label-shadowed': main.replace('    runs-on: medici-builder-public-release\n',
                                                     '    runs-on: medici-builder-public\n'
                                                     '    # runs-on: medici-builder-public-release\n', 1),
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
