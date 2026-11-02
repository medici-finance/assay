"""Offline installed-source and witness-call-site capability probe.

Only local synthetic Git objects and fake row processes are used. No container,
network, model, forge admission or acceptance is exercised by this probe.
"""
import os
from pathlib import Path
import subprocess
import tempfile


def probe(helper, witness):
    with tempfile.TemporaryDirectory(prefix='proof-probe-') as tmp:
        base = Path(tmp)
        root = base / 'work'
        root.mkdir()
        env = dict(os.environ, GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM='1',
                   GIT_CONFIG_COUNT='0', GIT_CONFIG_PARAMETERS='')
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
        sha = git('rev-parse', 'HEAD')
        helper = Path(helper).resolve()
        # Install-style fixed path in production; replace that literal only in the local
        # scratch copy when probing a source checkout rather than the installed image.
        script = base / 'witness.sh'
        script.write_text(Path(witness).read_text().replace(
            '/usr/local/lib/verify-runner/source_proof.py', str(helper)))
        bins = base / 'bin'
        bins.mkdir()
        marker = base / 'rows'
        unshare = bins / 'unshare'
        unshare.write_text('#!/bin/sh\nshift 3\nexec "$@"\n')
        statusgen = bins / 'statusgen'
        statusgen.write_text('''#!/bin/sh
if [ "$1" = __verifyrun-netns ]; then shift 2; exec "$@"; fi
printf 'check\ncheck:ci\n' > "$VR_TEST_ROWS"
printf '%s\n%s\n' "${ASSAY_DELIVERY_SHA-unset}" "${ASSAY_DELIVERY_BASE_SHA-unset}" > "$VR_TEST_DELIVERY"
exit 0
''')
        readlink = bins / 'readlink'
        readlink.write_text('#!/bin/sh\n[ "$1" = /proc/self/ns/net ] || exit 1\nprintf \'net:[123]\\n\'\n')
        for path in (unshare, statusgen, readlink):
            path.chmod(0o755)
        admit = base / 'admit'
        admit.mkdir()
        (admit / 'receipt.json').write_text('{}')
        (admit / 'image-digest').write_text('sha256:' + 'a' * 64)
        runenv = dict(env, PATH=str(bins) + ':/usr/bin:/bin', VR_WORK_DIR=str(root),
                      VR_ADMISSION_DIR=str(admit), VR_SCRATCH_DIR=str(base),
                      VR_TEST_ROWS=str(marker), VR_TEST_DELIVERY=str(base / 'delivery'),
                      ASSAY_DELIVERY_SHA='ambient-poison', ASSAY_DELIVERY_BASE_SHA='ambient-poison')
        def run(flags=(), good=True):
            marker.unlink(missing_ok=True)
            result = subprocess.run(['/bin/bash', str(script), 'proof/01', '--source-proof',
                                     '--sha=' + sha, *flags], env=runenv, capture_output=True)
            if good:
                assert result.returncode == 0, result.stderr.decode()
                assert marker.read_text() == 'check\ncheck:ci\n', 'both row sentinels must execute'
            else:
                assert result.returncode != 0 and not marker.exists(), 'refused source executed rows'
            return result
        run()
        assert (base / 'delivery').read_text() == 'unset\nunset\n'
        head, parent = 'b' * 40, 'c' * 40
        for flags, expected in [(['--delivery-sha=' + head], head + '\nunset\n'),
                                (['--delivery-base-sha=' + parent], 'unset\n' + parent + '\n'),
                                (['--delivery-sha=' + head, '--delivery-base-sha=' + parent], head + '\n' + parent + '\n')]:
            run(flags)
            assert (base / 'delivery').read_text() == expected
        for flags in (['--sha=' + sha], ['--source-proof'], ['--unknown'], ['--delivery-sha=x']):
            run(flags, good=False)
        pristine = brief.read_bytes()
        brief.write_bytes(pristine + b'changed\n')
        run(good=False)
        brief.write_bytes(pristine)
        extra = root / 'extra'
        extra.write_text('untracked')
        run(good=False)
        extra.unlink()
        (root / 'payload.txt').unlink()
        run(good=False)
        (root / 'payload.txt').write_text('fixture payload')
        run() # restored source really reaches the row seam again
        # Source proof supplies no admission; the existing receipt gate remains required.
        (admit / 'receipt.json').unlink()
        run(good=False)
    return 'SOURCE-PROOF PASS'


if __name__ == '__main__':
    import sys
    if len(sys.argv) != 3:
        raise SystemExit('usage: source_proof_check.py HELPER WITNESS')
    print(probe(*sys.argv[1:]))
