"""Offline installed-source and witness-call-site capability probe.

Only local synthetic Git objects and fake row processes are used. No container,
network, model, forge admission or acceptance is exercised by this probe.
"""
import os
from pathlib import Path
import subprocess
import tempfile


class ProbeFailure(AssertionError):
    """A named behaviour mismatch; setup, syntax and execution errors are not kills."""
    def __init__(self, code):
        self.code = code
        super().__init__(code)


def require(condition, code):
    if not condition:
        raise ProbeFailure(code)


def probe(helper, witness):
    with tempfile.TemporaryDirectory(prefix='proof-probe-') as tmp:
        base = Path(tmp)
        root = base / 'work'
        root.mkdir()
        env = {'PATH': '/usr/bin:/bin', 'HOME': str(base / 'empty-home'),
               'GIT_CONFIG_GLOBAL': os.devnull, 'GIT_CONFIG_NOSYSTEM': '1',
               'GIT_CONFIG_COUNT': '0', 'GIT_CONFIG_PARAMETERS': ''}
        def git(*args):
            return subprocess.check_output(['/usr/bin/git', '-C', str(root), *args], env=env).decode().strip()
        git('init', '-q', '--template=')
        brief = root / 'docs/streams/proof/brief-01-probe.md'
        brief.parent.mkdir(parents=True)
        brief.write_text('## Verify\n\n| # | Class | Command | Expect |\n|---|---|---|---|\n'
                         '| 1 | check | `true` | exit 0 |\n| 2 | check:ci | `true` | exit 0 |\n')
        (root / 'payload.txt').write_text('fixture payload')
        git('add', '.')
        git('-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid',
            '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture')
        older = git('rev-parse', 'HEAD')
        (root / 'payload.txt').write_text('current fixture payload')
        git('add', '.')
        git('-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid',
            '-c', 'commit.gpgsign=false', 'commit', '-qm', 'second real commit')
        sha = git('rev-parse', 'HEAD')
        helper = Path(helper).resolve()
        # Install-style fixed path in production; replace that literal only in the local
        # scratch copy when probing a source checkout rather than the installed image.
        script = base / 'witness.sh'
        script.write_text(Path(witness).read_text().replace(
            '/usr/local/lib/verify-runner/source_proof.py', str(helper)))
        marker = base / 'rows'
        # Bash reads this file; nothing under the scratch mount is exec'd. This
        # preserves the release's read-only root and noexec /tmp fence.
        seams = base / 'seams.sh'
        seams.write_text('''unshare() { shift 3; "$@"; }
statusgen() {
  if [ "$1" = __verifyrun-netns ]; then shift 2; "$@"; return $?; fi
  printf 'check\ncheck:ci\n' > "$VR_TEST_ROWS"
  printf '%s\n%s\n' "${ASSAY_DELIVERY_SHA-unset}" "${ASSAY_DELIVERY_BASE_SHA-unset}" > "$VR_TEST_DELIVERY"
}
readlink() { [ "$1" = /proc/self/ns/net ] || return 1; printf 'net:[123]\n'; }
''')
        # Parse all candidates first. Syntax/import failures cannot accidentally
        # satisfy a delivery or source refusal assertion.
        compile(helper.read_bytes(), str(helper), 'exec')
        for path in (script, seams):
            parsed = subprocess.run(['/bin/bash', '-n', str(path)], capture_output=True)
            if parsed.returncode:
                raise RuntimeError('invalid probe candidate: ' + parsed.stderr.decode())
        admit = base / 'admit'
        admit.mkdir()
        (admit / 'receipt.json').write_text('{}')
        (admit / 'image-digest').write_text('sha256:' + 'a' * 64)
        runenv = dict(env, PATH='/usr/bin:/bin', BASH_ENV=str(seams), VR_CACHE_DIR=str(base / 'cache'), VR_WORK_DIR=str(root),
                      VR_ADMISSION_DIR=str(admit), VR_SCRATCH_DIR=str(base),
                      VR_TEST_ROWS=str(marker), VR_TEST_DELIVERY=str(base / 'delivery'),
                      ASSAY_DELIVERY_SHA='ambient-poison', ASSAY_DELIVERY_BASE_SHA='ambient-poison')
        def run(flags=(), good=True, case='source', source=None, proof=True, usage=False, key='proof/01'):
            marker.unlink(missing_ok=True)
            args = [key]
            if proof:
                args += ['--source-proof']
            if source is not False:
                args += ['--sha=' + (sha if source is None else source)]
            result = subprocess.run(['/bin/bash', str(script), *args, *flags], env=runenv, capture_output=True)
            if good:
                if result.returncode:
                    raise RuntimeError('unexpected witness failure: ' + result.stderr.decode())
                require(marker.is_file() and marker.read_text() == 'check\ncheck:ci\n', 'rows:' + case)
            else:
                require(result.returncode == 2 and not marker.exists(), 'refusal:' + case)
                if usage:
                    require(result.stderr.startswith(b'usage: '), 'usage:' + case)
            return result
        run()
        require((base / 'delivery').read_text() == 'unset\nunset\n', 'delivery:ambient')
        head, parent = 'b' * 40, 'c' * 40
        for flags, expected in [(['--delivery-sha=' + head], head + '\nunset\n'),
                                (['--delivery-base-sha=' + parent], 'unset\n' + parent + '\n'),
                                (['--delivery-sha=' + head, '--delivery-base-sha=' + parent], head + '\n' + parent + '\n')]:
            run(flags)
            require((base / 'delivery').read_text() == expected, 'delivery:' + ('head' if flags[0].startswith('--delivery-sha') else 'base'))
        for flags in (['--sha=' + sha], ['--source-proof'], ['--unknown'], ['--delivery-sha=x']):
            run(flags, good=False, case='argv', usage=True)
        run(source=older, good=False, case='requested-sha')
        run(source=sha[:39], good=False, case='malformed-sha', usage=True)
        for key in ('seed', 'seed-probe', 'probe'):
            run(key=key, good=False, case='seed-flags', usage=True)
        run(source=False, good=False, case='no-sha', usage=True)
        run(['--delivery-sha=' + head], source=False, proof=False,
            good=False, case='delivery-without-proof', usage=True)
        pristine = brief.read_bytes()
        brief.write_bytes(pristine + b'changed\n')
        run(good=False, case='bytes')
        brief.write_bytes(pristine)
        extra = root / 'extra'
        extra.write_text('untracked')
        run(good=False, case='extra')
        extra.unlink()
        (root / 'payload.txt').unlink()
        run(good=False, case='missing')
        (root / 'payload.txt').write_text('current fixture payload')
        run() # restored source really reaches the row seam again
        # Source proof supplies no admission; the existing receipt gate remains required.
        (admit / 'receipt.json').unlink()
        run(good=False, case='receipt')
    return 'SOURCE-PROOF PASS'


if __name__ == '__main__':
    import sys
    if len(sys.argv) != 3:
        raise SystemExit('usage: source_proof_check.py HELPER WITNESS')
    print(probe(*sys.argv[1:]))
