"""The CI step that runs the release image contract, checked from outside it.

images_test.py cannot prove that CI runs it: if the step stopped running that
suite, its own checks would never execute. This module reads ci.yml directly,
through the strict reader in workflow_keys.py, and requires the step to run
images_test (and this module), to fail its shard when the tests fail, and to
stop at a fixed deadline. A line at top level or in the job that the reader
does not accept is refused, so another spelling of a setting cannot stand in
for the one checked. Offline; reads only workflow text.

Two runners execute it. The step itself runs it on the rest shard. The Go
regression TestReleaseGoCache (tools/desk/internal/regression) discovers every
tools/release/*_test.py, so the desk shards and the release workflow's
desk-tools leg run it too. That second runner is the backstop for a step that
stops running both suites, so its whole file is pinned here as well. Another
file in that Go package (a TestMain, say) can still stop it from running; the
pin does not read those.
"""
import importlib.util
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
CI = ROOT / '.github/workflows/ci.yml'
JOB = 'build-test-shard'
STEP = 'Test release image contract'
SUITES = ('tools.release.images_test', 'tools.release.ci_step_test')
MAX_MINUTES = 15
KEYS = importlib.util.spec_from_file_location('workflow_keys', Path(__file__).with_name('workflow_keys.py'))
workflow_keys = importlib.util.module_from_spec(KEYS)
KEYS.loader.exec_module(workflow_keys)
BACKSTOP = ROOT / 'tools/desk/internal/regression/releasecache_test.go'
BACKSTOP_RUN = ('\tcmd := exec.CommandContext(ctx, "python3", "-B", "-m", "unittest", "discover", '
                '"-s", "tools/release", "-p", "*_test.py", "-v")\n')
# The whole backstop file. A skip, an early return, an ignored failure or a
# rename changes the test's text. Wrapping it in a block comment or a string,
# or any other change that leaves the text but stops Go from compiling it as
# a test, changes the file around it. So the file is pinned whole, not by
# one line or by the function alone.
BACKSTOP_HEAD = (
    'package regression\n'
    '\n'
    'import (\n'
    '\t"context"\n'
    '\t"os/exec"\n'
    '\t"path/filepath"\n'
    '\t"runtime"\n'
    '\t"testing"\n'
    '\t"time"\n'
    ')\n'
    '\n'
    '// regression: #138\n'
    '// Includes the release-owned image producer/consumer contract.\n')
BACKSTOP_TEST = (
    'func TestReleaseGoCache(t *testing.T) {\n'
    '\tif runtime.GOOS == "windows" {\n'
    '\t\tt.Skip("release workflow executes on Linux; fixtures run in build-test")\n'
    '\t}\n'
    '\troot, err := filepath.Abs("../../../..")\n'
    '\tif err != nil {\n'
    '\t\tt.Fatal(err)\n'
    '\t}\n'
    '\tctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)\n'
    '\tdefer cancel()\n'
    + BACKSTOP_RUN +
    '\tcmd.Dir = root\n'
    '\tcmd.Env = FixtureEnv("KUBECONFIG=/dev/null", "PYTHONDONTWRITEBYTECODE=1")\n'
    '\tcmd.WaitDelay = time.Second\n'
    '\tout, err := cmd.CombinedOutput()\n'
    '\tif err != nil {\n'
    '\t\tt.Fatalf("release cache fixtures: %v\\n%s", err, out)\n'
    '\t}\n'
    '\tt.Log(string(out))\n'
    '}\n')
STEP_KEYS = ('name', 'if', 'timeout-minutes', 'run')


def job_block(text, name):
    match = re.search(r'(?ms)^  ' + re.escape(name) + r':\n(.*?)(?=^  [A-Za-z0-9_-]+:\n|\Z)', text)
    return match.group(1) if match else ''


def step_block(job, name):
    match = re.search(r'(?ms)^      - name: ' + re.escape(name) + r'\n(.*?)(?=^      - |^\S|\Z)', job)
    return match.group(1) if match else ''


def step_keys(entries):
    """The keys of the release contract step, or None unless it is named once."""
    steps, current = [], None
    for entry in entries:
        if entry.path == ('jobs', JOB, 'steps') and entry.item:
            current = []
            steps.append(current)
        if current is not None and entry.under('jobs', JOB, 'steps'):
            current.append(entry)
    named = [step for step in steps
             if any(e.path == ('jobs', JOB, 'steps') and e.key == 'name' and e.value == STEP
                    for e in step)]
    return named[0] if len(named) == 1 else None


def step_problems(text):
    """Why the release contract step would not run, fail closed, or finish.

    Settings are read with workflow_keys: a line at top level or in the job
    written in a form that reader does not accept is refused, so no other
    spelling of a setting can stand in for the one checked here.
    """
    entries, unread = workflow_keys.read(text)
    step = step_keys(entries)
    if step is None:
        return ['ci.yml has no ' + JOB + ' step named ' + STEP]
    problems = workflow_keys.unread_problems(unread, JOB)
    job = [entry for entry in entries if entry.under('jobs', JOB)]
    own = {}
    for entry in step:
        if entry.path == ('jobs', JOB, 'steps'):
            own.setdefault(entry.key, []).append(entry.value)
    shards = [workflow_keys.flow_items(entry.value) or [] for entry in job
              if entry.path == ('jobs', JOB, 'strategy', 'matrix') and entry.key == 'shard']
    if len(shards) != 1 or 'rest' not in shards[0]:
        problems.append('the rest shard must stay in the ' + JOB + ' matrix')
    if own.get('if') != ["matrix.shard == 'rest'"] or any(
            entry.key == 'if' and entry.path == ('jobs', JOB) for entry in job):
        problems.append("the step must run on exactly the rest shard")
    if any(entry.key == 'continue-on-error' for entry in step) or any(
            entry.key == 'continue-on-error' and entry.path == ('jobs', JOB) for entry in job):
        problems.append('a release contract failure must fail the shard')
    if (any(entry.key == 'shell' for entry in job)
            or any(entry.key == 'defaults' and entry.path in ((), ('jobs', JOB)) for entry in entries)):
        problems.append('the step must run in the default shell')
    if any(entry.key == 'exclude' for entry in job):
        problems.append('the ' + JOB + ' matrix must not exclude a shard')
    if sorted(own) != sorted(STEP_KEYS) or any(entry.path != ('jobs', JOB, 'steps') for entry in step):
        problems.append('the step must hold only ' + ', '.join(STEP_KEYS))
    if [(entry.key, entry.value) for entry in entries if not entry.path and entry.key == 'env'] \
            != [('env', '')] or [entry.key for entry in entries if entry.path == ('env',)] != ['go_version']:
        problems.append('the workflow environment must hold only GO_VERSION')
    if any(entry.path == ('jobs', JOB) and entry.key == 'env' for entry in job):
        problems.append('the ' + JOB + ' job must not set its own environment')
    deadlines = own.get('timeout-minutes', [])
    if len(deadlines) != 1 or not re.fullmatch(r'[0-9]+', deadlines[0]) \
            or not 0 < int(deadlines[0]) <= MAX_MINUTES:
        problems.append('the step needs one fixed deadline of at most %d minutes' % MAX_MINUTES)
    run = own.get('run', [])
    match = re.fullmatch(r'python3 -B -m unittest ((?:tools\.release\.[a-z_]+ )+)-v', run[0]) \
        if len(run) == 1 else None
    modules = match.group(1).split() if match else []
    for suite in SUITES:
        if suite not in modules:
            problems.append('the step must run ' + suite)
    return problems


def backstop_problems(text):
    """Why TestReleaseGoCache would no longer run, and fail on, every release suite."""
    lines = [line for line in text.splitlines(True) if 'exec.Command' in line]
    if lines != [BACKSTOP_RUN]:
        return ['TestReleaseGoCache must discover every tools/release suite']
    problems = []
    if not text.startswith('package regression\n') or re.search(r'(?m)^//\s*(?:go:build|\+build)', text):
        problems.append('TestReleaseGoCache must build on every platform')
    if text != BACKSTOP_HEAD + BACKSTOP_TEST:
        problems.append('TestReleaseGoCache must run every suite and fail when one fails')
    return problems


class ReleaseContractStepTest(unittest.TestCase):
    # regression: #5091
    def test_ci_step_runs_bounded(self):
        self.assertEqual(step_problems(CI.read_text()), [])

    # regression: #5091
    def test_ci_step_controls(self):
        text = CI.read_text()
        self.assertEqual(step_problems(text), [], 'controls need a healthy step to mutate')
        step = step_block(job_block(text, JOB), STEP)
        run = re.search(r'(?m)^        run: .*$', step).group(0)
        deadline = re.search(r'(?m)^        timeout-minutes: .*$', step).group(0)
        images = 'the step must run tools.release.images_test'
        rest = 'the step must run on exactly the rest shard'
        deadlines = 'the step needs one fixed deadline of at most %d minutes' % MAX_MINUTES
        shell = 'the step must run in the default shell'
        header = '    runs-on: medici-builder-public\n'
        mutants = {
            'images-suite-dropped': (run, run.replace(' tools.release.images_test', ''), images),
            'suite-renamed': (run, run.replace('tools.release.images_test', 'tools.release.go_cache_test'), images),
            'step-suite-dropped': (run, run.replace(' tools.release.ci_step_test', ''),
                                   'the step must run tools.release.ci_step_test'),
            'failure-swallowed': (run, run + ' || true', images),
            'other-shard': ("        if: matrix.shard == 'rest'\n", "        if: matrix.shard == 'desk-1'\n", rest),
            'never-runs': ("        if: matrix.shard == 'rest'\n", "        if: false\n", rest),
            'rest-not-scheduled': ('shard: [rest, ', 'shard: [',
                                   'the rest shard must stay in the ' + JOB + ' matrix'),
            'rest-excluded': ('        shard: [rest, desk-1, desk-2, desk-3]\n',
                              '        shard: [rest, desk-1, desk-2, desk-3]\n'
                              '        exclude:\n          - shard: rest\n',
                              'the ' + JOB + ' matrix must not exclude a shard'),
            'continue-on-error': (deadline, deadline + '\n        continue-on-error: true',
                                  'a release contract failure must fail the shard'),
            'shell-discards-result': (deadline, deadline + "\n        shell: bash -c 'exit 0' {0}", shell),
            'job-default-shell': (header, header + '    defaults:\n      run:\n'
                                  "        shell: bash -c 'exit 0' {0}\n", shell),
            'deadline-removed': (deadline + '\n', '', deadlines),
            'deadline-zero': (deadline, '        timeout-minutes: 0', deadlines),
            'deadline-16': (deadline, '        timeout-minutes: 16', deadlines),
            'deadline-expression': (deadline, '        timeout-minutes: ${{ inputs.minutes }}', deadlines),
            'step-renamed': ('      - name: ' + STEP + '\n', '      - name: Test release images\n',
                             'ci.yml has no ' + JOB + ' step named ' + STEP),
        }
        self.assertEqual(text.count(step), 1, 'step block is not unique')
        for name, (old, new, problem) in mutants.items():
            with self.subTest(name=name):
                # Mutate inside the step only; the matrix line lives in the job header.
                scope = step if old in step else job_block(text, JOB)
                self.assertEqual(scope.count(old), 1, 'control target is not unique')
                self.assertEqual(text.count(scope), 1, 'control scope is not unique')
                mutant = text.replace(scope, scope.replace(old, new, 1), 1)
                self.assertIn(problem, step_problems(mutant))
        workflow = text.replace('\njobs:\n', "\ndefaults:\n  run:\n    shell: bash -c 'exit 0' {0}\njobs:\n", 1)
        self.assertNotEqual(workflow, text)
        self.assertIn(shell, step_problems(workflow))

    # regression: #5091
    def test_ci_step_other_spellings(self):
        text = CI.read_text()
        self.assertEqual(step_problems(text), [])
        job = job_block(text, JOB)
        step = step_block(job, STEP)
        deadline = re.search(r'(?m)^        timeout-minutes: .*$', step).group(0) + '\n'
        header = '    runs-on: medici-builder-public\n'
        unread = 'is not in the plain form this check reads: '
        tolerate = 'a release contract failure must fail the shard'
        shell = 'the step must run in the default shell'
        rest = 'the step must run on exactly the rest shard'
        plants = {
            'step-tolerates-quoted-key': (step, deadline, deadline + '        "continue-on-error": true\n',
                                          unread + '"continue-on-error": true'),
            'step-tolerates-spaced-key': (step, deadline, deadline + '        continue-on-error : true\n',
                                          unread + 'continue-on-error : true'),
            'step-tolerates-other-case': (step, deadline, deadline + '        Continue-On-Error: true\n',
                                          tolerate),
            'job-tolerates-quoted-key': (job, header, header + '    "continue-on-error": true\n',
                                         unread + '"continue-on-error": true'),
            'job-tolerates-other-case': (job, header, header + '    CONTINUE-ON-ERROR: true\n', tolerate),
            'step-shell-spaced-key': (step, deadline, deadline + "        shell : bash -c 'exit 0' {0}\n",
                                      unread + "shell : bash -c 'exit 0' {0}"),
            'step-shell-other-case': (step, deadline, deadline + "        Shell: bash -c 'exit 0' {0}\n", shell),
            'step-if-quoted-second': (step, "        if: matrix.shard == 'rest'\n",
                                      "        if: matrix.shard == 'rest'\n        \"if\": false\n",
                                      unread + '"if": false'),
            'job-skipped': (job, header, header + '    if: false\n', rest),
            'shard-excluded-quoted': (job, '        shard: [rest, desk-1, desk-2, desk-3]\n',
                                      '        shard: [rest, desk-1, desk-2, desk-3]\n'
                                      '        "exclude":\n          - shard: rest\n',
                                      unread + '"exclude":'),
        }
        for name, (scope, old, new, problem) in plants.items():
            with self.subTest(name=name):
                self.assertEqual(scope.count(old), 1, 'control target is not unique')
                self.assertEqual(text.count(scope), 1, 'control scope is not unique')
                problems = step_problems(text.replace(scope, scope.replace(old, new, 1), 1))
                self.assertTrue(any(problem in found for found in problems), problems)
        # A workflow-level default placed after the jobs still reaches every step.
        self.assertIn(shell, step_problems(text + "defaults:\n  run:\n    shell: bash -c 'exit 0' {0}\n"))

    # regression: #5091 — settings the step or job could inherit or change.
    def test_ci_step_settings_closed(self):
        text = CI.read_text()
        self.assertEqual(step_problems(text), [])
        job = job_block(text, JOB)
        step = step_block(job, STEP)
        deadline = re.search(r'(?m)^        timeout-minutes: .*$', step).group(0) + '\n'
        header = '    runs-on: medici-builder-public\n'
        top = '  GO_VERSION: "1.27.3"\n\njobs:\n'
        keys = 'the step must hold only ' + ', '.join(STEP_KEYS)
        workflow = 'the workflow environment must hold only GO_VERSION'
        plants = {
            'step-directory': (step, deadline, deadline + '        working-directory: tools/desk\n', keys),
            'step-env': (step, deadline, deadline + '        env:\n          PYTHONPATH: ci/extra\n', keys),
            'step-id': (step, deadline, deadline + '        id: contract\n', keys),
            'job-env': (job, header, header + '    env:\n      BASH_ENV: ci/prelude.sh\n',
                        'the ' + JOB + ' job must not set its own environment'),
            'workflow-env': (text, top, '  BASH_ENV: ci/prelude.sh\n' + top, workflow),
        }
        for name, (scope, old, new, problem) in plants.items():
            with self.subTest(name=name):
                self.assertEqual(scope.count(old), 1, 'control target is not unique')
                self.assertEqual(text.count(scope), 1, 'control scope is not unique')
                self.assertIn(problem, step_problems(text.replace(scope, scope.replace(old, new, 1), 1)))

    # regression: #5091
    def test_reader_reads_one_form(self):
        read = workflow_keys.read
        entries, unread = read('a:\n  b: c # d\n  h: [i, j]\nk:\n  - l\n  - M: n\n'
                               'steps:\n  - run: |\n      f: g\n    with:\n      p: >-\n        q: r\n')
        self.assertEqual([(e.path, e.key, e.value, e.item) for e in entries],
                         [((), 'a', '', False), (('a',), 'b', 'c', False),
                          (('a',), 'h', '[i, j]', False), ((), 'k', '', False), (('k',), 'm', 'n', True),
                          ((), 'steps', '', False), (('steps',), 'run', '|', True),
                          (('steps',), 'with', '', False), (('steps', 'with'), 'p', '>-', False)])
        self.assertEqual(unread, [])
        # A block scalar anywhere else is refused, and its body is still not read.
        entries, unread = read('a:\n  e: |\n    - name: f\n      g: h\n')
        self.assertEqual([(e.path, e.key) for e in entries], [((), 'a')])
        self.assertEqual([(path, number) for path, number, line in unread], [(('a',), 2)])
        for name, line in {
            'quoted-key': '  "b": c', 'spaced-colon': '  b : c', 'flow-mapping': '  b: {c: d}',
            'anchor': '  b: &x c', 'alias': '  b: *x', 'merge-key': '  <<: *x', 'tag': '  b: !!str c',
            'open-flow': '  b: [c,', 'continuation': '  b: c\n    d', 'duplicate': '  b: c\n  B: d',
            'under-scalar': '  b: c\n    d: e', 'tab': '\tb: c', 'explicit-key': '  ? b',
            'open-quote': '  b: "c', 'document': '---', 'scalar-indicator': '  b: |-2x',
            'item-under-scalar': '  b: c\n    - d',
        }.items():
            with self.subTest(name=name):
                self.assertTrue(read('a:\n' + line + '\n')[1], name)

    # regression: #5091
    def test_backstop_discovers_all(self):
        text = BACKSTOP.read_text()
        self.assertEqual(backstop_problems(text), [])
        self.assertEqual(text.count(BACKSTOP_RUN), 1, 'control target is not unique')
        for name, new in {
            'one-suite': BACKSTOP_RUN.replace('*_test.py', 'images_test.py'),
            'other-root': BACKSTOP_RUN.replace('"tools/release"', '"tools/desk"'),
            'dropped': '',
        }.items():
            with self.subTest(name=name):
                self.assertIn('TestReleaseGoCache must discover every tools/release suite',
                              backstop_problems(text.replace(BACKSTOP_RUN, new, 1)))
        whole = 'TestReleaseGoCache must run every suite and fail when one fails'
        for name, (old, new, problem) in {
            'always-skipped': ('if runtime.GOOS == "windows" {', 'if true {', whole),
            'returns-early': ('\tdefer cancel()\n', '\tdefer cancel()\n\tif true {\n\t\treturn\n\t}\n', whole),
            'failure-ignored': ('\t\tt.Fatalf("release cache', '\t\tt.Logf("release cache', whole),
            'renamed': ('func TestReleaseGoCache(', 'func testReleaseGoCache(', whole),
            'build-excluded': ('package regression\n', '//go:build ignore\n\npackage regression\n',
                               'TestReleaseGoCache must build on every platform'),
        }.items():
            with self.subTest(name=name):
                self.assertEqual(text.count(old), 1, 'control target is not unique')
                self.assertIn(problem, backstop_problems(text.replace(old, new, 1)))
        # The test's text unchanged, but no longer a Go test: imports kept in use
        # so the package still compiles, and go test finds nothing to run.
        used = ('\nvar (\n\t_ = context.Background\n\t_ exec.Cmd\n\t_ = filepath.Abs\n'
                '\t_ = runtime.GOOS\n\t_ testing.TB\n\t_ = time.Second\n)\n')
        for name, (start, end) in {'commented-out': ('/*\n', '*/\n'),
                                   'in-a-string': ('var _ = `\n', '`\n')}.items():
            with self.subTest(name=name):
                inert = text.replace(BACKSTOP_TEST, start + BACKSTOP_TEST + end + used, 1)
                self.assertEqual(inert.count(BACKSTOP_TEST), 1)
                self.assertIn(whole, backstop_problems(inert))


if __name__ == '__main__':
    unittest.main()
