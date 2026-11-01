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


def spans(text, starts, bounds):
    """{key: text} for each STARTS entry, from the line after it to the line
    before the next of BOUNDS (line numbers): where workflow_keys ends it."""
    lines = text.splitlines(True)
    found = {}
    for entry in starts:
        end = min([number for number in bounds if number > entry.line], default=len(lines) + 1)
        found[entry.value if entry.item else entry.key] = ''.join(lines[entry.line:end - 1])
    return found


def workflow_jobs(text):
    """Each job's text, cut where workflow_keys ends the job.

    A job ends at the next job or top-level key, or at a line in that position
    the reader cannot read, so text after the jobs never joins the last one.
    """
    entries, unread = workflow_keys.read(text)
    return spans(text, [entry for entry in entries if entry.path == ('jobs',)],
                 [entry.line for entry in entries if len(entry.path) <= 1]
                 + [number for path, number, line in unread if len(path) <= 1])


def workflow_steps(block):
    """Each named step of a job's BLOCK, cut where workflow_keys ends the step.

    A step ends at the next step or job key, or at a line in that position the
    reader cannot read. A name that two steps share maps to '', so no check
    reads either copy as the step it looks for.
    """
    entries, unread = workflow_keys.read(block)
    heads = [entry for entry in entries if entry.path == ('steps',) and entry.item]
    named = [entry for entry in heads if entry.key == 'name']
    steps = spans(block, named, [entry.line for entry in heads]
                  + [entry.line for entry in entries if not entry.path]
                  + [number for path, number, line in unread if not path])
    for name, count in Counter(entry.value for entry in named).items():
        if count > 1:
            steps[name] = ''
    return steps


def step_entries(entries, job):
    """JOB's steps as [name, entries] pairs, read from workflow_keys' structure."""
    where = ('jobs', job, 'steps')
    steps = []
    for entry in entries:
        if entry.path == where and entry.item:
            steps.append([None, []])
        if steps and entry.under(*where):
            steps[-1][1].append(entry)
            if entry.path == where and entry.key == 'name':
                steps[-1][0] = entry.value
    return steps


def step_setting(entries, job, step, key, under=()):
    """The values of KEY in JOB's one step named STEP; [] unless one step has that name."""
    named = [found for name, found in step_entries(entries, job) if name == step]
    if len(named) != 1:
        return []
    where = ('jobs', job, 'steps') + under
    return [entry.value for entry in named[0] if entry.path == where and entry.key == key]


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


def unquoted(value):
    """VALUE without the quotes of a single-line quoted scalar."""
    if len(value) > 1 and value[0] == value[-1] and value[0] in '"\'':
        return value[1:-1]
    return value


def own_setting(step, job, key, under=()):
    """The values of KEY directly in STEP (one step's entries), or under UNDER."""
    where = ('jobs', job, 'steps') + under
    return [entry.value for entry in step if entry.path == where and entry.key == key]


def uses_action(step, job, action):
    """Whether STEP runs ACTION (owner/name), at any ref."""
    return any(unquoted(value).lower().startswith(action + '@') for value in own_setting(step, job, 'uses'))


def run_script(text, entries, job, step):
    """The live lines of the run script of JOB's one step named STEP.

    The text comes from the step's own `run` entry: for a `|` or `>` header,
    the lines the reader skipped as its body; comment lines are dropped. ''
    unless exactly one step has that name and it has one `run`.
    """
    runs = [entry for name, found in step_entries(entries, job) if name == step
            for entry in found if entry.path == ('jobs', job, 'steps') and entry.key == 'run']
    names = [name for name, found in step_entries(entries, job)]
    if names.count(step) != 1 or len(runs) != 1:
        return ''
    if not workflow_keys.SCALAR.fullmatch(runs[0].value):
        return runs[0].value
    return live_lines(scalar_body(text, runs[0]))


def scalar_body(text, entry):
    """The body of ENTRY's `|` or `>` block scalar: the lines the reader skipped."""
    lines = text.splitlines(True)
    head = workflow_keys.ENTRY.fullmatch(lines[entry.line - 1].rstrip('\n'))
    depth = len(head.group(1)) + len(head.group(2) or '')
    body = []
    for line in lines[entry.line:]:
        if line.strip() and len(line) - len(line.lstrip(' ')) <= depth:
            break
        body.append(line)
    return ''.join(body)


def environment_name(entries, job):
    """JOB's deployment environment name in lower case, '' for none."""
    found = [entry.value for entry in entries if entry.path == ('jobs', job) and entry.key == 'environment']
    if not found:
        return ''
    if found[0]:
        return unquoted(found[0]).lower()
    names = [entry.value for entry in entries
             if entry.path == ('jobs', job, 'environment') and entry.key == 'name']
    return unquoted(names[0]).lower() if len(names) == 1 else '?'


def permission_problems(entries):
    """The workflow grants only `contents: read`, and only `release` writes.

    Read from workflow_keys entries: a grant kept in a comment counts as
    absent, and a trailing comment never hides a grant's value. A grant in a
    spelling the reader does not accept is not an entry, so publication_problems
    refuses each such line in every job through workflow_keys.unread_problems.
    """
    problems = []
    top = [(entry.key, unquoted(entry.value).lower()) for entry in entries if entry.path == ('permissions',)]
    if [entry.value for entry in entries if not entry.path and entry.key == 'permissions'] != [''] \
            or top != [('contents', 'read')]:
        problems.append('workflow defaults must not grant publication writes')
    for job in [entry.key for entry in entries if entry.path == ('jobs',)]:
        grants = [entry.value for entry in entries if entry.path == ('jobs', job) and entry.key == 'permissions']
        if grants not in ([], ['']):
            problems.append(job + ' must list each permission it grants, not ' + grants[0])
        writes = {entry.key for entry in entries if entry.path == ('jobs', job, 'permissions')
                  and unquoted(entry.value).lower() == 'write'}
        if job == 'release':
            problems += [name + ' write belongs to the approved publisher'
                         for name in ('contents', 'packages') if name not in writes]
        else:
            problems += [name + ' write escaped the approved publisher: ' + job for name in sorted(writes)]
    return problems


# The release job's steps that must run in this order, each named once.
RELEASE_ORDER = ('Restore the prepared source and verify release artifacts',
                 'Download checked native arm64 image',
                 'Build, verify and publish the release images',
                 'Validate and checksum the published image references',
                 'Dry run — stop before tagging and publishing',
                 'Create and push the dispatched tag',
                 'Create GitHub release and upload assets')
# Lines each named release step's run script must hold outside its comment lines.
RELEASE_SCRIPTS = {
    'Restore the prepared source and verify release artifacts': ('git checkout --detach "$SOURCE_SHA"',),
    'Build, verify and publish the release images': (
        'from tools.release.images import build_images',
        "build_images(Path.cwd(), os.environ['RELEASE_TAG'], os.environ['SOURCE_SHA'],",
        "publish=os.environ['DRY_RUN'] != 'true'",
        "native=os.environ['NATIVE']",
        "trap 'rm -rf \"$DOCKER_CONFIG\" \"$SOURCE\"' EXIT"),
    'Validate and checksum the published image references': (
        'from tools.release.images_contract import RUNNER_PLATFORM, validate',
        'runner_platforms=(RUNNER_PLATFORM,)',
        'sha256sum release-images.json verify-runner-image.txt >> checksums.txt'),
    'List the release assets to attest': ('release-images.json verify-runner-image.txt checksums.txt; do',),
}
NATIVE_IN = '${{ runner.temp }}/release-native-arm64-in'
# The jobs that check out the repository and must not leave the token in .git.
CHECKOUT_JOBS = ('test', 'artifacts', 'runner-arm64', 'release')


def publication_problems(text):
    """Check the approval boundary and the order of externally visible writes.

    Every setting here is read from workflow_keys entries, never from job text,
    so a line inside a run script or a `with` block scalar never counts as a
    step, a key or a value. The script checks look for fixed lines in one
    named step's run script, outside its comment lines. It also applies
    publisher_step_problems and transfer_problems, and refuses an unread line
    at top level, in a publishing job, among any job's own keys, or under any
    job's permissions or environment.
    """
    entries, unread = workflow_keys.read(text)
    problems = permission_problems(entries)
    jobs = [entry.key for entry in entries if entry.path == ('jobs',)]
    if any('${{' in environment_name(entries, job) for job in jobs):
        problems.append('every job must name its environment literally')
    if [job for job in jobs if environment_name(entries, job) == 'release'] != ['release']:
        problems.append('exactly one release-approved job must own publication')
    publisher = [entry for entry in entries if entry.path == ('jobs', 'release')]
    needs = [entry.value for entry in publisher if entry.key == 'needs']
    if not {'resolve', 'guard', 'test', 'artifacts', 'runner-arm64'} <= set(
            (workflow_keys.flow_items(needs[0]) or []) if len(needs) == 1 else []):
        problems.append('publication must wait for all preparation gates')
    if [entry.value for entry in publisher if entry.key == 'runs-on'] != [RELEASE_RUNNERS['release']]:
        problems.append('publisher needs the dedicated Docker-capable release runner')
    names = [name for name, step in step_entries(entries, 'release')]
    positions = [names.index(name) if names.count(name) == 1 else -1 for name in RELEASE_ORDER]
    if min(positions) < 0 or positions != sorted(positions):
        problems.append('images and validated references must precede tag and release writes')
    problems += workflow_keys.unread_problems(unread, *TEST_DEPENDENTS)
    problems += tolerance_problems(entries)
    problems += publisher_step_problems(entries)
    problems += [problem for problem in transfer_problems(text) if problem not in problems]
    for name, guard in {
        'Create and push the dispatched tag': "github.event_name == 'workflow_dispatch' && !inputs.dry_run",
        'Create GitHub release and upload assets': "${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
        'Validate and checksum the published image references': "${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}",
    }.items():
        if step_setting(entries, 'release', name, 'if') != [guard]:
            problems.append('missing dry-run guard on ' + name)
    for job in CHECKOUT_JOBS:
        checkouts = [step for name, step in step_entries(entries, job)
                     if uses_action(step, job, 'actions/checkout')]
        if not checkouts or any(own_setting(step, job, 'persist-credentials', ('with',)) != ['false']
                                for step in checkouts):
            problems.append(job + ' must check out with persist-credentials: false')
    if step_setting(entries, 'release', 'Download checked native arm64 image', 'path', ('with',)) != [NATIVE_IN]:
        problems.append('the native arm64 image must land in its own directory')
    native = [entry for name, step in step_entries(entries, 'release')
              if name == 'Build, verify and publish the release images'
              for entry in step if entry.path[3:] == ('env',) and entry.key == 'native']
    if len(native) != 1 or native[0].value != NATIVE_IN \
            or text.splitlines()[native[0].line - 1].strip() != 'NATIVE: ' + NATIVE_IN:
        problems.append('the image build must read the native arm64 directory')
    for step, lines in RELEASE_SCRIPTS.items():
        script = run_script(text, entries, 'release', step)
        for value in lines:
            if value not in script:
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
REPORT_INPUTS = ('if-no-files-found', 'name', 'path', 'retention-days')
REPORT_PATH = '${{ runner.temp }}/tv-reports/'
TEST_DEPENDENTS = ('artifacts', 'runner-arm64', 'release')
# The only publishing-path steps whose failure may be tolerated: the optional
# release Go cache, which readers accept only from a successful main dispatch.
# Each is pinned by its whole content, every key and value in order, so another
# step that takes one of these names is not exempt, and neither is a cache step
# whose command, action or settings changed.
OPTIONAL_STEPS = {
    ('artifacts', 'Package release Go cache'): (
        ('name', 'Package release Go cache'), ('id', 'release-cache-pack'), ('timeout-minutes', '2'),
        ('if', "github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main' && !inputs.dry_run"),
        ('continue-on-error', 'true'),
        ('run', 'python3 -c \'from tools.release import go_cache; go_cache.main("pack")\'')),
    ('artifacts', 'Save release Go cache'): (
        ('name', 'Save release Go cache'), ('id', 'release-cache-save'), ('timeout-minutes', '2'),
        ('if', "steps.release-cache-pack.outcome == 'success'"), ('continue-on-error', 'true'),
        ('uses', 'actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9'), ('with', ''),
        ('with.name', 'release-go-cache-v1'), ('with.path', '${{ runner.temp }}/go-cache.tar'),
        ('with.if-no-files-found', 'error'), ('with.retention-days', '7'),
        ('with.compression-level', '1'), ('with.overwrite', 'false')),
}


def step_content(step, job):
    """STEP's entries as (dotted key, value) pairs, in order."""
    return tuple(('.'.join(entry.path[3:] + (entry.key,)), entry.value) for entry in step
                 if entry.under('jobs', job, 'steps'))


def tolerance_problems(entries):
    """No publishing job, and no step in one, may tolerate its own failure.

    Any `continue-on-error`, whatever its value, is refused: true, an
    expression or a quoted value each let a failed build, check or upload pass
    on toward publication. The one exception is a step whose name is in
    OPTIONAL_STEPS, that no other step in the job shares, and whose content is
    exactly the pinned content.
    """
    problems = []
    for job in TEST_DEPENDENTS:
        if any(entry.path == ('jobs', job) and entry.key == 'continue-on-error' for entry in entries):
            problems.append(job + ' must not tolerate its own failure')
        steps = step_entries(entries, job)
        names = [name for name, step in steps]
        for name, step in steps:
            if names.count(name) == 1 and step_content(step, job) == OPTIONAL_STEPS.get((job, name)):
                continue
            if any(entry.path == ('jobs', job, 'steps') and entry.key == 'continue-on-error'
                   for entry in step):
                problems.append('a publishing step must not tolerate failure: %s: %s'
                                % (job, name or 'unnamed step'))
    return problems


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


def environment_problems(entries, job):
    """JOB sets no environment of its own, and the workflow sets only GO_VERSION.

    A variable such as BASH_ENV at either level reaches every step of the job
    without changing a line of the steps the other checks read.
    """
    problems = []
    if [(entry.key, entry.value) for entry in entries if not entry.path and entry.key == 'env'] \
            != [('env', '')] or [entry.key for entry in entries if entry.path == ('env',)] != ['go_version']:
        problems.append('the workflow environment must hold only GO_VERSION')
    if any(entry.path == ('jobs', job) and entry.key == 'env' for entry in entries):
        problems.append('the %s job must not set its own environment' % job)
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
    report name, and that neither the test job nor one of its steps is set to
    tolerate failure. It also refuses a job-level environment on the test job,
    any workflow-level variable but GO_VERSION, and a `container` on the test
    job or a publishing job. It does not read what an
    earlier step of the job writes to GITHUB_ENV or GITHUB_PATH, so a step
    that changes the environment the harness steps inherit is not refused
    here. Settings are read with workflow_keys, so a setting kept only in a
    comment counts as absent, and any line at top level, in the test or
    publishing jobs, among any job's own keys or under any job's permissions
    or environment, written in a form the reader does not accept, is refused
    instead of being skipped. The report upload holds exactly REPORT_INPUTS
    and sends REPORT_PATH.
    """
    entries, unread = workflow_keys.read(text)
    problems = workflow_keys.unread_problems(unread, 'test', *TEST_DEPENDENTS)
    problems += publish_wait_problems(entries)
    problems += environment_problems(entries, 'test')
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
    if any(entry.path == ('jobs', job) and entry.key == 'container'
           for job in ('test',) + TEST_DEPENDENTS for entry in entries):
        problems.append('the test and publishing jobs must run on their runner, not in a container')
    steps = workflow_steps(test)
    # The upload's settings come from workflow_keys entries, never from its text.
    conditions = step_setting(entries, 'test', UPLOAD, 'if')
    upload_legs = set()
    if len(conditions) == 1 and UPLOAD_LEGS.fullmatch(conditions[0]):
        upload_legs = set(re.findall(r"matrix\.leg == '([^']+)'", conditions[0]))
    else:
        problems.append('the report upload must run on exactly the legs it names')
    if step_setting(entries, 'test', UPLOAD, 'if-no-files-found', ('with',)) != ['error']:
        problems.append('the report upload must fail when it finds no files')
    uses = step_setting(entries, 'test', UPLOAD, 'uses')
    if (len(uses) != 1 or not uses[0].startswith('actions/upload-artifact@')
            or step_setting(entries, 'test', UPLOAD, 'name', ('with',))
            != ['tv-reports-${{ matrix.leg }}-${{ strategy.job-index }}']):
        problems.append('each mutation leg must upload its reports without collision or omission')
    reports = [step for name, step in step_entries(entries, 'test') if name == UPLOAD]
    if len(reports) != 1 or sorted(with_inputs(reports[0], 'test')) != list(REPORT_INPUTS) \
            or own_setting(reports[0], 'test', 'path', ('with',)) != [REPORT_PATH]:
        problems.append('the report upload must send exactly the reports directory')
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
    """runner-arm64 reads every build artifact and fails on an empty upload.

    Both are read from workflow_keys entries: a download is a step that uses
    actions/download-artifact, and its `with` settings are its own keys, so a
    line in a block scalar body or under another key never stands in for one.
    Every download step in the job must read all release-build-* artifacts,
    merged, into the job's input directory, with no other input, and there
    must be at least one. The native image's upload holds exactly
    UPLOAD_INPUTS, under its own artifact name and uncompressed.
    """
    entries, unread = workflow_keys.read(text)
    problems = workflow_keys.unread_problems(unread, 'runner-arm64')
    downloads = [step for name, step in step_entries(entries, 'runner-arm64')
                 if uses_action(step, 'runner-arm64', 'actions/download-artifact')]
    wanted = dict(BUILD_DOWNLOAD, path='${{ runner.temp }}/release-input')
    if not downloads or any(with_inputs(step, 'runner-arm64') != wanted for step in downloads):
        problems.insert(0, 'runner-arm64 must download every release-build-* artifact, merged')
    if step_setting(entries, 'runner-arm64', 'Upload checked native arm64 image',
                    'if-no-files-found', ('with',)) != ['error']:
        problems.append('the native arm64 upload must fail when it finds no files')
    native = [step for name, step in step_entries(entries, 'runner-arm64')
              if name == 'Upload checked native arm64 image']
    if len(native) != 1 or not upload_held(native[0], 'runner-arm64', 'release-runner-arm64', None):
        problems.append('the native arm64 upload must hold exactly its own artifact')
    return problems


# The artifacts job's uploads, one per platform, that the release job merges
# back into one directory; each holds exactly UPLOAD_INPUTS.
PLATFORM_UPLOADS = ('darwin-arm64', 'darwin-amd64', 'linux-amd64', 'linux-arm64',
                    'windows-amd64', 'windows-arm64', 'common')
PLATFORM_STEP = 'Upload prepared release artifacts (%s)'
UPLOAD_INPUTS = ('compression-level', 'if-no-files-found', 'name', 'path', 'retention-days')
BUILD_DOWNLOAD = {'pattern': 'release-build-*', 'merge-multiple': 'true'}
NATIVE_DOWNLOAD = 'Download checked native arm64 image'


def with_inputs(step, job):
    """STEP's `with` inputs as {key: value}, from its own entries."""
    return {entry.key: entry.value for entry in step if entry.path == ('jobs', job, 'steps', 'with')}


def upload_held(step, job, name, empty='error'):
    """Whether upload STEP sends artifact NAME uncompressed, keeps it seven days,
    holds no other input and, unless EMPTY is None, fails on no files."""
    inputs = with_inputs(step, job)
    return (uses_action(step, job, 'actions/upload-artifact') and sorted(inputs) == list(UPLOAD_INPUTS)
            and inputs['name'] == name and inputs['compression-level'] == '0'
            and inputs['if-no-files-found'] == (empty or inputs['if-no-files-found'])
            and inputs['retention-days'] == '7')


def upload_patterns(text, step, job):
    """The path patterns of upload STEP, from its one `path` input: the value,
    or each live line of its block scalar. [] unless it has one `path`."""
    paths = [entry for entry in step if entry.path == ('jobs', job, 'steps', 'with') and entry.key == 'path']
    if len(paths) != 1:
        return []
    if not workflow_keys.SCALAR.fullmatch(paths[0].value):
        return [paths[0].value]
    return [line.strip() for line in live_lines(scalar_body(text, paths[0])).splitlines() if line.strip()]


def platform_uploads(text):
    """{artifact name: path patterns} for each platform upload of the artifacts job."""
    entries = workflow_keys.read(text)[0]
    found = {}
    for name, step in step_entries(entries, 'artifacts'):
        for platform in PLATFORM_UPLOADS:
            if name == PLATFORM_STEP % platform:
                found[with_inputs(step, 'artifacts').get('name')] = upload_patterns(text, step, 'artifacts')
    return found


def transfer_problems(text):
    """The build artifacts reach the publisher whole, read as keys.

    Each platform in PLATFORM_UPLOADS has one step in `artifacts`, named for
    it, whose upload holds exactly UPLOAD_INPUTS under its own artifact name,
    uncompressed and failing when it finds no files. No other step in any job
    uploads under the release-build- prefix the publisher merges. Every
    download step in `release` but the native image's reads all
    release-build-* artifacts, merged, into the workspace, with no other
    input, and there is at least one; the native image's download reads only
    its own artifact into its own directory. Every value comes from the
    step's own `with` entries, so a comment or a block scalar body holding the
    expected text never stands in for one.
    """
    entries, unread = workflow_keys.read(text)
    problems = workflow_keys.unread_problems(unread, *TEST_DEPENDENTS)
    platform = {PLATFORM_STEP % name: 'release-build-' + name for name in PLATFORM_UPLOADS}
    steps = step_entries(entries, 'artifacts')
    for name, artifact in platform.items():
        found = [step for found, step in steps if found == name]
        if len(found) != 1 or not upload_held(found[0], 'artifacts', artifact):
            problems.append('the %s upload must send it whole, uncompressed, failing on no files' % artifact)
    for job in [entry.key for entry in entries if entry.path == ('jobs',)]:
        for name, step in step_entries(entries, job):
            if unquoted(with_inputs(step, job).get('name', '')).startswith('release-build-') \
                    and (job != 'artifacts' or name not in platform):
                problems.append('only the platform uploads may name a release-build- artifact: %s: %s'
                                % (job, name or 'unnamed step'))
    builds = 0
    for name, step in step_entries(entries, 'release'):
        if not uses_action(step, 'release', 'actions/download-artifact'):
            continue
        if name == NATIVE_DOWNLOAD:
            if with_inputs(step, 'release') != {'name': 'release-runner-arm64', 'path': NATIVE_IN}:
                problems.append('the native arm64 download must read only its own artifact')
            continue
        builds += 1
        if with_inputs(step, 'release') != dict(BUILD_DOWNLOAD, path='.'):
            problems.append('every release build download must read all release-build-* artifacts, '
                            'merged, and nothing else')
    if not builds:
        problems.append('the release must download the build artifacts')
    return problems


# The conditions on publishing-job steps, keyed by job and step name, or for an
# unnamed step by the action it uses. Every other step in a publishing job runs
# unconditionally, so `if: false` on a build or check step is refused.
PUBLISHING = "${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}"
STEP_CONDITIONS = {
    ('artifacts', 'Package release Go cache'):
        "github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main' && !inputs.dry_run",
    ('artifacts', 'Save release Go cache'): "steps.release-cache-pack.outcome == 'success'",
    ('artifacts', 'Record release cache receipt'): "steps.release-cache-save.outcome == 'success'",
    ('release', 'Validate and checksum the published image references'): PUBLISHING,
    ('release', 'actions/upload-artifact'): PUBLISHING,
    ('release', 'Dry run — stop before tagging and publishing'):
        "github.event_name == 'workflow_dispatch' && inputs.dry_run",
    ('release', 'List the release assets to attest'): PUBLISHING,
    ('release', 'actions/attest-build-provenance'): PUBLISHING,
    ('release', 'Stage and check a Sigstore bundle beside every release asset'): PUBLISHING,
    ('release', 'Create and push the dispatched tag'): "github.event_name == 'workflow_dispatch' && !inputs.dry_run",
    ('release', 'Create GitHub release and upload assets'): PUBLISHING,
}
# Each publishing job's own environment, as (lower-case key, value) pairs.
JOB_ENV = {'artifacts': [('cgo_enabled', '"0"')], 'runner-arm64': [], 'release': []}


def step_label(name, step, job):
    """STEP's name, or for an unnamed step the action it uses."""
    uses = own_setting(step, job, 'uses')
    if name or len(uses) != 1:
        return name or 'unnamed step'
    return unquoted(uses[0]).lower().split('@')[0]


def publisher_step_problems(entries):
    """No publishing job changes whether, how or where its steps run.

    Refused: `defaults` at the top level or on a publishing job, `shell` or
    `working-directory` on any step of one, a job environment other than
    JOB_ENV, and a step condition other than the one STEP_CONDITIONS pins
    (none, for a step it does not list).
    """
    problems = []
    if any(not entry.path and entry.key == 'defaults' for entry in entries):
        problems.append('the workflow must not set step defaults')
    for job in TEST_DEPENDENTS:
        where = ('jobs', job)
        if any(entry.path == where and entry.key == 'defaults' for entry in entries):
            problems.append(job + ' must not set step defaults')
        if any(entry.path == where + ('steps',) and entry.key in ('shell', 'working-directory')
               for entry in entries):
            problems.append(job + ' must run its steps in the default shell and directory')
        declared = [entry.value for entry in entries if entry.path == where and entry.key == 'env']
        env = [(entry.key, entry.value) for entry in entries if entry.path == where + ('env',)]
        if declared != ([''] if JOB_ENV[job] else []) or env != JOB_ENV[job]:
            problems.append(job + ' must set exactly its pinned job environment')
        for name, step in step_entries(entries, job):
            label = step_label(name, step, job)
            pinned = STEP_CONDITIONS.get((job, label))
            if own_setting(step, job, 'if') != ([pinned] if pinned else []):
                problems.append('a publishing step must keep its pinned condition: %s: %s' % (job, label))
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
        UNREAD + 'if: >-'),
    'artifacts-if-block-scalar': (
        'artifacts', '    needs: [resolve, guard, test]\n',
        '    needs: [resolve, guard, test]\n    if: >-\n      !cancelled()\n',
        UNREAD + 'if: >-'),
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


def move_step(text, job, name, before):
    """Move JOB's one step NAME to just before the line BEFORE in that job."""
    entries = workflow_keys.read(text)[0]
    heads = [entry.line for entry in entries if entry.path == ('jobs', job, 'steps') and entry.item]
    ends = heads + [entry.line for entry in entries if len(entry.path) <= 1]
    starts = [step[0].line for found, step in step_entries(entries, job) if found == name]
    if len(starts) != 1:
        raise AssertionError('control target is not unique: ' + job + ': ' + name)
    end = min([number for number in ends if number > starts[0]], default=len(text.splitlines()) + 1)
    lines = text.splitlines(True)
    step = ''.join(lines[starts[0] - 1:end - 1])
    return mutate_job(''.join(lines[:starts[0] - 1] + lines[end - 1:]), job, before, step + before)


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
            text = (root / path).read_text()
            jobs = workflow_jobs(text)
            # Names, inputs and patterns come from the reader's keys; the
            # transfer settings are held by transfer_problems.
            self.assertEqual(transfer_problems(text), [])
            uploads = platform_uploads(text)
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
                    for name, patterns in uploads.items():
                        self.assertIn(name, expected)
                        matched = [f for f in files if any(fnmatch.fnmatchcase(f, p) for p in patterns)]
                        self.assertCountEqual(matched, expected[name])
                        self.assertFalse(any(fnmatch.fnmatchcase('go-cache.tar', p) for p in patterns))
                        selected.extend(matched)
                    self.assertCountEqual(selected, files)
            release = live_lines(jobs['release'])
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
            'grep-weakened-behind-scalar': ('test', grep,
                                       grep.replace('          if', '          # if', 1)
                                       .replace('\n            echo', "\n          if ! grep -qE 'Totals:' "
                                                '<<<"$report"; then\n            echo', 1),
                                       'mutation verdict gate changed: ' + harness),
            'harness-shell-override': ('test', light, light + "        shell: bash -c 'exit 0' {0}\n",
                                       'the test job must run its steps in the default shell'),
            'fail-fast-behind-scalar': ('test', '      fail-fast: false\n',
                                   '      fail-fast: true\n      # fail-fast: false\n',
                                   'all matrix verdicts must run without fail-fast cancellation'),
            'leg-excluded': ('test', '      matrix:\n',
                             '      matrix:\n        exclude:\n          - leg: muhar-light\n',
                             'the test matrix must not exclude a leg'),
            'upload-warns-inline': ('test', '          if-no-files-found: error\n',
                                    '          if-no-files-found: warn # if-no-files-found: error\n',
                                    'the report upload must fail when it finds no files'),
            'upload-warns-behind-scalar': ('test', '          if-no-files-found: error\n',
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

    # regression: #5091
    def test_publisher_tolerance(self):
        build = '      - name: Build, verify and publish the release images\n'
        native = '      - name: Build and check native arm64 verify-runner\n'
        preflight = '      - name: Preflight — required tooling\n'
        step = 'a publishing step must not tolerate failure: '
        plants = {
            'publish-step-commented': ('release', build, build + '        continue-on-error: true # for now\n',
                                       step + 'release: Build, verify and publish the release images'),
            'publish-step-expression': ('release', build,
                                        build + '        continue-on-error: ${{ inputs.dry_run || true }}\n',
                                        step + 'release: Build, verify and publish the release images'),
            'release-job': ('release', '    runs-on: medici-builder-public-release\n',
                            '    runs-on: medici-builder-public-release\n    continue-on-error: true # for now\n',
                            'release must not tolerate its own failure'),
            'artifacts-job': ('artifacts', '    needs: [resolve, guard, test]\n',
                              '    needs: [resolve, guard, test]\n    continue-on-error: "true"\n',
                              'artifacts must not tolerate its own failure'),
            'native-job': ('runner-arm64', '    runs-on: ubuntu-24.04-arm\n',
                           '    runs-on: ubuntu-24.04-arm\n    Continue-On-Error: true\n',
                           'runner-arm64 must not tolerate its own failure'),
            'native-step': ('runner-arm64', native, native + '        continue-on-error: true\n',
                            step + 'runner-arm64: Build and check native arm64 verify-runner'),
            'artifacts-step': ('artifacts', preflight, preflight + '        continue-on-error: true\n',
                               step + 'artifacts: Preflight — required tooling'),
        }
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(publication_problems(text), [])
            for name, (job, old, new, problem) in plants.items():
                with self.subTest(path=path, plant=name):
                    self.assertIn(problem, publication_problems(mutate_job(text, job, old, new)))

    # regression: #5091 — text the reader skips, or text outside a job, never
    # stands in for a step the guards read.
    def test_skipped_text_refused(self):
        tag = "        if: github.event_name == 'workflow_dispatch' && !inputs.dry_run\n"
        weak_tag = "        if: github.event_name == 'workflow_dispatch'\n"
        tag_step = '      - name: Create and push the dispatched tag\n'
        upload = '          if-no-files-found: error\n'
        weak_upload = '          if-no-files-found: warn\n'
        native_upload = '      - name: Upload checked native arm64 image\n'
        empty = 'the native arm64 upload must fail when it finds no files'
        scalar = UNREAD + 'name: |'
        cover = '      - name: Report leg\n        run: echo done\n    name: |\n'
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            harness = re.search(r'(?ms)^      - name: Gate bar — mutate every desksourceguard.*?^          fi\n',
                                text).group(0)
            weak_harness = harness.replace('            exit 1\n', '            exit 0\n')
            self.assertNotEqual(weak_harness, harness)
            plants = {
                'harness-exit-behind-scalar': (
                    mutation_routing_problems, 'test', harness, weak_harness, cover + harness,
                    [scalar, 'a surviving mutation must fail its harness step: cmd/desksourceguard/mutations.json']),
                'tag-guard-behind-scalar': (
                    publication_problems, 'release', tag, weak_tag, cover + tag_step + tag,
                    [scalar, 'missing dry-run guard on Create and push the dispatched tag']),
                'native-upload-behind-scalar': (
                    native_transport_problems, 'runner-arm64', upload, weak_upload,
                    cover + native_upload + upload, [scalar, empty]),
            }
            for name, (guard, job, old, new, tail, problems) in plants.items():
                with self.subTest(path=path, plant=name):
                    self.assertEqual(guard(text), [])
                    mutant = mutate_job(text, job, old, new)
                    block = workflow_jobs(mutant)[job]
                    self.assertEqual(mutant.count(block), 1, 'control target is not unique')
                    found = guard(mutant.replace(block, block.rstrip('\n') + '\n' + tail + '\n', 1))
                    for problem in problems:
                        self.assertTrue(any(problem in line for line in found), found)
            with self.subTest(path=path, plant='text-after-jobs'):
                after = ('x-copy:\n  runner-arm64:\n    steps:\n'
                         '      - uses: actions/download-artifact@v8\n        with:\n'
                         '          pattern: release-build-*\n          merge-multiple: true\n'
                         '      - name: Upload checked native arm64 image\n        with:\n' + upload)
                mutant = mutate_job(text, 'runner-arm64', upload, weak_upload)
                self.assertEqual(native_transport_problems(mutant + after), [empty])
            with self.subTest(path=path, plant='tag-step-twice'):
                second = '      - name: Create GitHub release and upload assets\n'
                mutant = mutate_job(mutate_job(text, 'release', tag, weak_tag), 'release', second,
                                    tag_step + tag + '        run: echo tag\n' + second)
                self.assertIn('missing dry-run guard on Create and push the dispatched tag',
                              publication_problems(mutant))

    # regression: #5091
    def test_harness_environment(self):
        problem = 'the workflow environment must hold only GO_VERSION'
        top = '  GO_VERSION: "1.27.3"\n\njobs:\n'
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(text.count(top), 1, 'control target is not unique')
            with self.subTest(path=path, plant='test-job-env'):
                self.assertIn('the test job must not set its own environment', mutation_routing_problems(
                    mutate_job(text, 'test', '    runs-on: medici-builder-release\n',
                               '    runs-on: medici-builder-release\n    env:\n      BASH_ENV: ci/prelude.sh\n')))
            with self.subTest(path=path, plant='workflow-env'):
                self.assertIn(problem, mutation_routing_problems(
                    text.replace(top, '  BASH_ENV: ci/prelude.sh\n' + top, 1)))
            with self.subTest(path=path, plant='workflow-env-expression'):
                mutant = re.sub(r'(?ms)^env:\n.*?\n\njobs:\n',
                                'env: ${{ fromJSON(vars.RELEASE_ENV) }}\n\njobs:\n', text, count=1)
                self.assertNotEqual(mutant, text)
                self.assertIn(problem, mutation_routing_problems(mutant))

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
            'publisher-label-behind-scalar': main.replace('    runs-on: medici-builder-public-release\n',
                                                     '    runs-on: medici-builder-public\n'
                                                     '    # runs-on: medici-builder-public-release\n', 1),
        }
        for name, mutant in mutants.items():
            with self.subTest(name=name):
                self.assertTrue(publication_problems(mutant), name)

    # regression: #5091 — the cache exemption holds only for the cache steps.
    def test_cache_exemption_pinned(self):
        step = 'a publishing step must not tolerate failure: artifacts: '
        sbom = '      - name: Generate the SBOM\n'
        save = '      - name: Save release Go cache\n'
        pack = "        run: python3 -c 'from tools.release import go_cache; go_cache.main(\"pack\")'\n"
        plants = {
            'check-takes-save-name': [(sbom, save + '        continue-on-error: true\n')],
            'check-takes-pack-name': [('      - name: Build image-only bugs-gc binary\n',
                                       '      - name: Package release Go cache\n'
                                       '        continue-on-error: true\n')],
            'pack-runs-other-command': [(pack, '        run: sh ci/prepare-release.sh\n')],
            'pack-gains-a-key': [('        id: release-cache-pack\n',
                                  '        id: release-cache-pack\n        working-directory: tools\n')],
            'save-renamed-away': [(save, '      - name: Save release Go cache (v2)\n')],
            'save-name-moved': [(save, '      - name: Save release Go cache (v2)\n'),
                                (sbom, save + '        continue-on-error: true\n')],
        }
        problems = {'check-takes-save-name': 'Save release Go cache',
                    'check-takes-pack-name': 'Package release Go cache',
                    'pack-runs-other-command': 'Package release Go cache',
                    'pack-gains-a-key': 'Package release Go cache',
                    'save-renamed-away': 'Save release Go cache (v2)',
                    'save-name-moved': 'Save release Go cache'}
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(publication_problems(text), [])
            for name, edits in plants.items():
                with self.subTest(path=path, plant=name):
                    mutant = text
                    for old, new in edits:
                        mutant = mutate_job(mutant, 'artifacts', old, new)
                    self.assertIn(step + problems[name], publication_problems(mutant))

    # regression: #5091 — step order is read from the steps, not from text.
    def test_order_read_from_steps(self):
        order = 'images and validated references must precede tag and release writes'
        validate = 'Validate and checksum the published image references'
        dry = 'Dry run — stop before tagging and publishing'
        release = '      - name: Create GitHub release and upload assets\n'
        heredoc = ("          cat <<'ORDER'\n            - name: " + validate + '\n            - name: '
                   + dry + '\n          ORDER\n')
        refs = '          path: ${{ runner.temp }}/release-image-refs\n'
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(publication_problems(text), [])
            moved = move_step(move_step(text, 'release', validate, release), 'release', dry, release)
            self.assertIn(order, publication_problems(moved))
            plants = {
                'names-in-run-heredoc': ('      - name: ' + validate + '\n', heredoc + '      - name: '
                                         + validate + '\n'),
                'names-in-with-scalar': (refs, '          path: |\n            ${{ runner.temp }}/release-image-refs\n'
                                         '            - name: ' + validate + '\n            - name: ' + dry + '\n'),
            }
            for name, (old, new) in plants.items():
                with self.subTest(path=path, plant=name):
                    mutant = mutate_job(text, 'release', old, new)
                    self.assertEqual(publication_problems(mutant), [], 'the plant alone must stay healthy')
                    mutant = move_step(move_step(mutant, 'release', validate, release), 'release', dry, release)
                    self.assertIn(order, publication_problems(mutant))

    # regression: #5091 — publisher settings and scripts never read from comments.
    def test_publisher_settings_read(self):
        persist = '          persist-credentials: false\n'
        native = '${{ runner.temp }}/release-native-arm64-in'
        checkout = '          git checkout --detach "$SOURCE_SHA"\n'
        plants = {
            'persist-true-commented': (
                'release', persist + '          fetch-depth: 0\n',
                '          persist-credentials: true  # restore persist-credentials: false later\n'
                '          fetch-depth: 0\n',
                'release must check out with persist-credentials: false'),
            'persist-only-in-comment': (
                'release', persist + '          fetch-depth: 0\n',
                '          # persist-credentials: false\n          fetch-depth: 0\n',
                'release must check out with persist-credentials: false'),
            'artifacts-persists': (
                'artifacts', persist, '          persist-credentials: true # persist-credentials: false\n',
                'artifacts must check out with persist-credentials: false'),
            'native-path-in-comment': (
                'release', '          path: ' + native + '\n', '          path: .  # was path: ' + native + '\n',
                'the native arm64 image must land in its own directory'),
            'native-env-in-comment': (
                'release', '          NATIVE: ' + native + '\n',
                '          NATIVE: ${{ github.workspace }}  # NATIVE: ' + native + '\n',
                'the image build must read the native arm64 directory'),
            'checkout-in-comment': (
                'release', checkout, '          # ' + checkout.lstrip(),
                'missing publication guard: ' + checkout.strip()),
        }
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(publication_problems(text), [])
            for name, (job, old, new, problem) in plants.items():
                with self.subTest(path=path, plant=name):
                    self.assertIn(problem, publication_problems(mutate_job(text, job, old, new)))

    # regression: #5091 — write grants in every spelling the reader accepts.
    def test_permission_spellings(self):
        top = '\npermissions:\n  contents: read\n'
        defaults = 'workflow defaults must not grant publication writes'
        escaped = ' write escaped the approved publisher: '
        plants = {
            'workflow-adds-write': (None, top, top + '  id-token: write\n', defaults),
            'workflow-write-all': (None, top, '\npermissions: write-all\n', defaults),
            'job-write-all': ('guard', '    permissions:\n      contents: read\n', '    permissions: write-all\n',
                              'guard must list each permission it grants, not write-all'),
            'job-write-commented': ('runner-arm64', '      contents: read\n', '      contents: write # needed\n',
                                    'contents' + escaped + 'runner-arm64'),
            'job-write-quoted': ('artifacts', '      contents: read\n', '      contents: "write"\n',
                                 'contents' + escaped + 'artifacts'),
            'job-other-write': ('test', '      contents: read\n', '      contents: read\n      issues: write\n',
                                'issues' + escaped + 'test'),
            'environment-expression': ('changelog-roll', '    environment: board-writer\n',
                                       '    environment: ${{ inputs.target }}\n',
                                       'every job must name its environment literally'),
            'environment-mapping': ('guard', '    permissions:\n', '    environment:\n      name: Release\n'
                                    '    permissions:\n', 'exactly one release-approved job must own publication'),
        }
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(publication_problems(text), [])
            self.assertEqual(text.count(top), 1, 'control target is not unique')
            for name, (job, old, new, problem) in plants.items():
                with self.subTest(path=path, plant=name):
                    mutant = text.replace(old, new, 1) if job is None else mutate_job(text, job, old, new)
                    self.assertIn(problem, publication_problems(mutant))

    # regression: #5091 — upload, download and job settings read as structure.
    def test_routing_read_as_keys(self):
        name = '          name: tv-reports-${{ matrix.leg }}-${{ strategy.job-index }}\n'
        upload = (name + '          path: ${{ runner.temp }}/tv-reports/\n'
                  '          if-no-files-found: error\n          retention-days: 7\n')
        collide = 'each mutation leg must upload its reports without collision or omission'
        download = ('          pattern: release-build-*\n          merge-multiple: true\n'
                    '          path: ${{ runner.temp }}/release-input\n')
        merged = 'runner-arm64 must download every release-build-* artifact, merged'
        runner = '    runs-on: medici-builder-release\n'
        contained = 'the test and publishing jobs must run on their runner, not in a container'
        plants = {
            'upload-name-in-scalar': (
                mutation_routing_problems, 'test', upload,
                '         name: tv-reports\n         retention-days: 7\n         path: |\n'
                '          ${{ runner.temp }}/tv-reports/\n' + name + '          if-no-files-found: error\n',
                [collide, 'the report upload must fail when it finds no files']),
            'upload-name-in-env': (
                mutation_routing_problems, 'test', name,
                '          name: tv-reports-${{ matrix.leg }}\n        env:\n' + name, [collide]),
            'test-container': (
                mutation_routing_problems, 'test', runner, runner + '    container: ghcr.io/example/build:1\n',
                [contained]),
            'artifacts-container': (
                mutation_routing_problems, 'artifacts', '    needs: [resolve, guard, test]\n',
                '    needs: [resolve, guard, test]\n    container:\n      image: ghcr.io/example/build:1\n',
                [contained]),
            'download-in-scalar': (
                native_transport_problems, 'runner-arm64', download,
                '         pattern: release-build-linux-amd64\n         merge-multiple: false\n'
                '         path: ${{ runner.temp }}/release-input\n         repository: >\n'
                '          pattern: release-build-*\n          merge-multiple: true\n', [merged]),
            'download-in-env': (
                native_transport_problems, 'runner-arm64', download,
                '          pattern: release-build-common\n          path: ${{ runner.temp }}/release-input\n'
                '        env:\n          pattern: release-build-*\n          merge-multiple: true\n', [merged]),
        }
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            for name, (guard, job, old, new, problems) in plants.items():
                with self.subTest(path=path, plant=name):
                    self.assertEqual(guard(text), [])
                    found = guard(mutate_job(text, job, old, new))
                    for problem in problems:
                        self.assertIn(problem, found)

    def mutate_step(self, text, job, name, old, new):
        """Replace OLD with NEW inside JOB's one step NAME only."""
        step = workflow_steps(workflow_jobs(text)[job])[name]
        self.assertTrue(step, 'control target is not unique: ' + name)
        self.assertEqual(text.count(step), 1, 'control target is not unique: ' + name)
        self.assertEqual(step.count(old), 1, 'control target is not unique: ' + old)
        return text.replace(step, step.replace(old, new, 1), 1)

    # regression: #5091 — an unread line among any job's own keys, or under its
    # permissions or environment, is refused, in jobs no guard names too.
    def test_unread_grants_in_every_job(self):
        grant = '    permissions:\n      contents: read\n'
        plants = {
            'job-permissions-quoted-key': ('guard', grant, '    "permissions":\n      contents: write\n',
                                           '"permissions":'),
            'job-permissions-spaced-key': ('resolve', '    permissions:\n', '    permissions :\n      packages: write\n',
                                           'permissions :'),
            'job-permissions-flow-mapping': ('changelog-roll', '    permissions:\n',
                                             '    permissions: {contents: write}\n', 'permissions: {contents: write}'),
            'grant-quoted-key': ('guard', grant, '    permissions:\n      "contents": write\n', '"contents": write'),
            'grant-block-scalar': ('guard', grant, '    permissions:\n      contents: >-\n        write\n',
                                   'contents: >-'),
            'environment-quoted-key': ('guard', grant, "    'environment': release\n" + grant,
                                       "'environment': release"),
            'environment-flow-mapping': ('changelog-roll', '    environment: board-writer\n',
                                         '    environment: {name: release}\n', 'environment: {name: release}'),
            'environment-name-quoted-key': ('resolve', '    permissions:\n',
                                            '    environment:\n      "name": release\n    permissions:\n',
                                            '"name": release'),
            'new-job-quoted-grant': (None, '', '  extra:\n    runs-on: medici-builder-release\n'
                                     '    "permissions": write-all\n', '"permissions": write-all'),
        }
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(workflow_keys.read(text)[1], [], 'the workflow holds unread lines')
            self.assertEqual(publication_problems(text), [])
            for name, (job, old, new, line) in plants.items():
                with self.subTest(path=path, plant=name):
                    found = publication_problems(plant(text, job, old, new))
                    self.assertTrue(any(found_line.endswith(UNREAD + line) for found_line in found), found)

    # regression: #5091 — upload and download settings read as keys, never as
    # text: a comment holding the expected setting never stands in for it.
    def test_transfers_read_as_keys(self):
        darwin = 'Upload prepared release artifacts (darwin-arm64)'
        common = 'Upload prepared release artifacts (common)'
        whole = 'the release-build-darwin-arm64 upload must send it whole, uncompressed, failing on no files'
        merged = 'every release build download must read all release-build-* artifacts, merged, and nothing else'
        native_in = 'runner-arm64 must download every release-build-* artifact, merged'
        native_out = 'the native arm64 upload must hold exactly its own artifact'
        reports = 'the report upload must send exactly the reports directory'
        level = '          compression-level: 0\n'
        empty = '          if-no-files-found: error\n'
        pattern = '          pattern: release-build-*\n'
        merge = '          merge-multiple: true\n'
        extra = ('      - name: Upload more release files\n'
                 '        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2\n'
                 '        with:\n          name: release-build-extra\n          path: extra.bin\n')
        native = '      - name: Build and check native arm64 verify-runner\n'
        full = ('      - uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2\n'
                '        with:\n' + pattern + merge + '          path: ${{ runner.temp }}/release-input\n')
        steps = {
            'upload-level-trailing-comment': (
                'artifacts', darwin, level, '          compression-level: 6 # compression-level: 0\n', whole),
            'upload-level-next-line-comment': (
                'artifacts', darwin, level, '          compression-level: 9\n          # compression-level: 0\n', whole),
            'upload-empty-trailing-comment': (
                'artifacts', darwin, empty, '          if-no-files-found: warn # if-no-files-found: error\n', whole),
            'upload-empty-next-line-comment': (
                'artifacts', darwin, empty, '          if-no-files-found: ignore\n          # if-no-files-found: error\n',
                whole),
            'upload-extra-input': ('artifacts', darwin, level, level + '          overwrite: true\n', whole),
            'native-upload-extra-input': (
                'runner-arm64', 'Upload checked native arm64 image', level, level + '          overwrite: true\n',
                native_out),
            'native-download-extra-input': (
                'release', 'Download checked native arm64 image', '          path: ' + NATIVE_IN + '\n',
                '          path: ' + NATIVE_IN + '\n          pattern: release-*\n',
                'the native arm64 download must read only its own artifact'),
            'report-upload-path-widened': (
                'test', UPLOAD, '          path: ${{ runner.temp }}/tv-reports/\n', '          path: ${{ runner.temp }}/\n',
                reports),
            'report-upload-extra-input': (
                'test', UPLOAD, '          retention-days: 7\n', '          retention-days: 7\n          overwrite: true\n',
                reports),
        }
        jobs = {
            'download-merge-trailing-comment': ('release', merge, '          merge-multiple: false # merge-multiple: true\n',
                                                merged),
            'download-pattern-next-line-comment': (
                'release', pattern, '          pattern: release-build-common\n          # pattern: release-build-*\n',
                merged),
            'download-extra-input': ('release', pattern, pattern + '          name: release-build-common\n', merged),
            'native-download-sibling-input': ('runner-arm64', pattern, pattern + '          name: release-build-common\n',
                                              native_in),
            'native-download-stand-in': ('runner-arm64', pattern + merge,
                                         '          pattern: release-build-common\n' + merge, native_in),
            'second-release-build-upload': ('artifacts', '      - name: ' + common + '\n',
                                            extra + '      - name: ' + common + '\n',
                                            'only the platform uploads may name a release-build- artifact: '
                                            'artifacts: Upload more release files'),
        }
        guards = (publication_problems, native_transport_problems, mutation_routing_problems)
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            for guard in guards:
                self.assertEqual(guard(text), [])
            for name, (job, step, old, new, problem) in steps.items():
                with self.subTest(path=path, plant=name):
                    mutant = self.mutate_step(text, job, step, old, new)
                    self.assertIn(problem, [line for guard in guards for line in guard(mutant)])
            for name, (job, old, new, problem) in jobs.items():
                with self.subTest(path=path, plant=name):
                    mutant = mutate_job(text, job, old, new)
                    if name == 'native-download-stand-in':
                        mutant = mutate_job(mutant, job, native, full + native)
                    self.assertIn(problem, [line for guard in guards for line in guard(mutant)])

    # regression: #5091 — a publishing job never changes whether, how or where
    # its steps run.
    def test_publisher_steps_run_as_written(self):
        build = '      - name: Build, verify and publish the release images\n'
        restore = '      - name: Restore the prepared source and verify release artifacts\n'
        native = '      - name: Build and check native arm64 verify-runner\n'
        preflight = '      - name: Preflight — required tooling\n'
        receipt = "        if: steps.release-cache-save.outcome == 'success'\n"
        condition = 'a publishing step must keep its pinned condition: '
        plants = {
            'build-step-skipped': ('release', build, build + '        if: false\n',
                                   condition + 'release: Build, verify and publish the release images'),
            'verify-step-skipped': ('release', restore, restore + "        if: ${{ github.event_name == 'never' }}\n",
                                    condition + 'release: Restore the prepared source and verify release artifacts'),
            'native-step-skipped': ('runner-arm64', native, native + '        if: ${{ false }}\n',
                                    condition + 'runner-arm64: Build and check native arm64 verify-runner'),
            'receipt-condition-widened': ('artifacts', receipt, '        if: always()\n',
                                          condition + 'artifacts: Record release cache receipt'),
            'step-shell': ('artifacts', preflight, preflight + "        shell: bash -c 'exit 0' {0}\n",
                           'artifacts must run its steps in the default shell and directory'),
            'step-directory': ('release', build, build + '        working-directory: tools\n',
                               'release must run its steps in the default shell and directory'),
            'job-defaults': ('release', '    runs-on: medici-builder-public-release\n',
                             '    runs-on: medici-builder-public-release\n    defaults:\n      run:\n'
                             '        working-directory: tools\n', 'release must not set step defaults'),
            'job-env-added': ('runner-arm64', '    runs-on: ubuntu-24.04-arm\n',
                              '    runs-on: ubuntu-24.04-arm\n    env:\n      GOTOOLCHAIN: local\n',
                              'runner-arm64 must set exactly its pinned job environment'),
            'job-env-extended': ('artifacts', '      CGO_ENABLED: "0"\n', '      CGO_ENABLED: "0"\n      GOTOOLCHAIN: local\n',
                                 'artifacts must set exactly its pinned job environment'),
            'job-env-expression': ('release', '    runs-on: medici-builder-public-release\n',
                                   '    runs-on: medici-builder-public-release\n    env: ${{ fromJSON(vars.RELEASE_ENV) }}\n',
                                   'release must set exactly its pinned job environment'),
        }
        top = '\njobs:\n'
        for path in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text = (HERE.parents[1] / path).read_text()
            self.assertEqual(publication_problems(text), [])
            for name, (job, old, new, problem) in plants.items():
                with self.subTest(path=path, plant=name):
                    self.assertIn(problem, publication_problems(mutate_job(text, job, old, new)))
            with self.subTest(path=path, plant='workflow-defaults'):
                self.assertEqual(text.count(top), 1, 'control target is not unique')
                mutant = text.replace(top, '\ndefaults:\n  run:\n    working-directory: tools\n' + top, 1)
                self.assertIn('the workflow must not set step defaults', publication_problems(mutant))

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
