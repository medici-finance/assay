"""The CI step that runs the release image contract, checked from outside it.

images_test.py cannot prove that CI runs it: if the step stopped running that
suite, its own checks would never execute. This module reads ci.yml directly,
with its own small parser, and requires the step to run images_test (and this
module), to fail its shard when the tests fail, and to stop at a fixed deadline.
Offline; reads only workflow text.
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
        mutants = {
            'images-suite-dropped': (run, run.replace(' tools.release.images_test', '')),
            'suite-renamed': (run, run.replace('tools.release.images_test', 'tools.release.go_cache_test')),
            'step-suite-dropped': (run, run.replace(' tools.release.ci_step_test', '')),
            'failure-swallowed': (run, run + ' || true'),
            'other-shard': ("        if: matrix.shard == 'rest'\n", "        if: matrix.shard == 'desk-1'\n"),
            'never-runs': ("        if: matrix.shard == 'rest'\n", "        if: false\n"),
            'rest-not-scheduled': ('shard: [rest, ', 'shard: ['),
            'continue-on-error': (deadline, deadline + '\n        continue-on-error: true'),
            'deadline-removed': (deadline + '\n', ''),
            'deadline-zero': (deadline, '        timeout-minutes: 0'),
            'deadline-16': (deadline, '        timeout-minutes: 16'),
            'deadline-expression': (deadline, '        timeout-minutes: ${{ inputs.minutes }}'),
            'step-renamed': ('      - name: ' + STEP + '\n', '      - name: Test release images\n'),
        }
        self.assertEqual(text.count(step), 1, 'step block is not unique')
        for name, (old, new) in mutants.items():
            with self.subTest(name=name):
                # Mutate inside the step only; the matrix line lives in the job header.
                scope = step if old in step else job_block(text, JOB)
                self.assertEqual(scope.count(old), 1, 'control target is not unique')
                self.assertEqual(text.count(scope), 1, 'control scope is not unique')
                mutant = text.replace(scope, scope.replace(old, new, 1), 1)
                self.assertTrue(step_problems(mutant), 'step control survived')


if __name__ == '__main__':
    unittest.main()
