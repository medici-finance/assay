"""The CI step that runs the release image contract, checked from outside it.

images_test.py cannot prove that CI runs it: if the step stopped running that
suite, its own checks would never execute. This module reads ci.yml directly,
with its own small parser, and requires the step to run images_test (and this
module), to fail its shard when the tests fail, and to stop at a fixed deadline.
Offline; reads only workflow text.

Two runners execute it. The step itself runs it on the rest shard. The Go
regression TestReleaseGoCache (tools/desk/internal/regression) discovers every
tools/release/*_test.py, so the desk shards and the release workflow's
desk-tools leg run it too. That second runner is the backstop for a step that
stops running both suites, so its command is pinned here as well.
"""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
CI = ROOT / '.github/workflows/ci.yml'
JOB = 'build-test-shard'
STEP = 'Test release image contract'
SUITES = ('tools.release.images_test', 'tools.release.ci_step_test')
MAX_MINUTES = 15
BACKSTOP = ROOT / 'tools/desk/internal/regression/releasecache_test.go'
BACKSTOP_RUN = ('\tcmd := exec.CommandContext(ctx, "python3", "-B", "-m", "unittest", "discover", '
                '"-s", "tools/release", "-p", "*_test.py", "-v")\n')


def job_block(text, name):
    match = re.search(r'(?ms)^  ' + re.escape(name) + r':\n(.*?)(?=^  [A-Za-z0-9_-]+:\n|\Z)', text)
    return match.group(1) if match else ''


def step_block(job, name):
    match = re.search(r'(?ms)^      - name: ' + re.escape(name) + r'\n(.*?)(?=^      - |^\S|\Z)', job)
    return match.group(1) if match else ''


def step_problems(text):
    """Why the release contract step would not run, fail closed, or finish."""
    job = job_block(text, JOB)
    step = step_block(job, STEP)
    if not step:
        return ['ci.yml has no ' + JOB + ' step named ' + STEP]
    problems = []
    shards = re.search(r'(?m)^        shard: \[([^\]\n]*)\]$', job)
    if not shards or 'rest' not in [s.strip() for s in shards.group(1).split(',')]:
        problems.append('the rest shard must stay in the ' + JOB + ' matrix')
    if not re.search(r"(?m)^        if: matrix\.shard == 'rest'$", step) or step.count('if:') != 1:
        problems.append("the step must run on exactly the rest shard")
    if 'continue-on-error' in step or re.search(r'(?m)^    continue-on-error:', job):
        problems.append('a release contract failure must fail the shard')
    if (re.search(r'(?m)^ *shell:', step) or re.search(r'(?m)^    defaults:', job)
            or re.search(r'(?m)^defaults:', text.split('\njobs:\n', 1)[0])):
        problems.append('the step must run in the default shell')
    if re.search(r'(?m)^ *exclude:', job):
        problems.append('the ' + JOB + ' matrix must not exclude a shard')
    deadlines = re.findall(r'(?m)^        timeout-minutes: (.*)$', step)
    if len(deadlines) != 1 or not re.fullmatch(r'[0-9]+', deadlines[0].strip()) \
            or not 0 < int(deadlines[0]) <= MAX_MINUTES:
        problems.append('the step needs one fixed deadline of at most %d minutes' % MAX_MINUTES)
    run = re.findall(r'(?m)^        run: (.*)$', step)
    match = re.fullmatch(r'python3 -B -m unittest ((?:tools\.release\.[a-z_]+ )+)-v', run[0]) \
        if len(run) == 1 else None
    modules = match.group(1).split() if match else []
    for suite in SUITES:
        if suite not in modules:
            problems.append('the step must run ' + suite)
    return problems


def backstop_problems(text):
    """Why TestReleaseGoCache would no longer run every release suite."""
    lines = [line for line in text.splitlines(True) if 'exec.Command' in line]
    if lines != [BACKSTOP_RUN]:
        return ['TestReleaseGoCache must discover every tools/release suite']
    return []


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


if __name__ == '__main__':
    unittest.main()
