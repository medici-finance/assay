"""Image traversal and release verdicts; credential detection belongs to Gitleaks.

Never extract image paths onto the host. Read each layer independently and give
Gitleaks raw numbered files. Its normal binary handling applies. A synthetic
canary proves the stock detector ran; image-controlled ignore files never apply.
"""
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import secrets
import signal
import string
import subprocess
import sys
import tarfile
import tempfile

SCANNER_IMAGE = 'zricethezav/gitleaks:v8.30.1@sha256:b109bc5f8f76a38196a3e413704fc5b9e3c32360bce4e4b603bd6f45b3721dbb'
BASELINE = Path(__file__).resolve().with_name('image-secret-baseline.json')


class Incomplete(Exception):
    pass


def command(args):
    p = subprocess.run(args, capture_output=True, timeout=600)
    if p.returncode:
        # Tool errors can contain credentials. Never relay stdout/stderr.
        raise Incomplete(f'{args[0]} {args[1]} failed (exit {p.returncode})')
    return p.stdout


def image_archive(image, dest):
    command(['docker', 'image', 'inspect', SCANNER_IMAGE])
    command(['docker', 'save', image, '-o', str(dest)])


def member_bytes(archive, name):
    try:
        member = archive.getmember(name)
        if not member.isfile():
            raise Incomplete('referenced image member is not a regular file')
        data = archive.extractfile(member).read()
    except (KeyError, OSError, tarfile.TarError) as e:
        raise Incomplete('cannot read referenced image member') from e
    if name.startswith('blobs/sha256/'):
        if hashlib.sha256(data).hexdigest() != name.removeprefix('blobs/sha256/'):
            raise Incomplete('image blob digest mismatch')
    return data


def surfaces(path):
    """Yield metadata and every regular file, without following image links."""
    with tarfile.open(path, 'r:*') as image:
        names = [m.name for m in image.getmembers()]
        if len(names) != len(set(names)):
            raise Incomplete('duplicate saved-image member')
        manifest = json.loads(member_bytes(image, 'manifest.json'))
        if not isinstance(manifest, list) or not manifest:
            raise Incomplete('empty image manifest')
        seen = set()
        for entry in manifest:
            data = member_bytes(image, entry['Config'])
            if not isinstance(json.loads(data), dict):
                raise Incomplete('invalid image config')
            yield 'image config/history', data
            layers = entry['Layers']
            if not isinstance(layers, list) or not layers:
                raise Incomplete('manifest lists no layer')
            for layer in layers:
                if layer in seen:
                    continue
                seen.add(layer)
                data = member_bytes(image, layer)
                with tarfile.open(fileobj=io.BytesIO(data), mode='r:*') as fs:
                    for member in fs:
                        if member.isfile():
                            content = fs.extractfile(member).read()
                            if len(content) != member.size:
                                raise Incomplete('truncated layer member')
                            label = f'layer {layer}: /{member.name.removeprefix("./")}'
                            yield label, content
                        elif not (member.isdir() or member.issym() or member.islnk()
                                  or member.isdev() or member.isfifo()):
                            raise Incomplete('unsupported layer member')


def project(items, directory):
    """Isolate image content from scanner configuration; preserve raw bytes."""
    index = {}
    canary = ('ghp' + '_' + ''.join(secrets.choice(string.ascii_letters + string.digits)
                                  for _ in range(36))).encode()
    (directory / 'canary.txt').write_bytes(canary + b'\n')
    for label, data in items:
        # Bind stock baseline locations to the logical path AND every file
        # byte. Altered files cannot inherit a same-line baseline clearance.
        logical_path = label.split(': /', 1)[-1]
        identity = hashlib.sha256(logical_path.encode())
        identity.update(b'\0')
        identity.update(data)
        name = identity.hexdigest() + '.txt'
        (directory / name).write_bytes(data)
        index[name] = label
    if not index:
        raise Incomplete('no scan inputs')
    return index


def gitleaks(directory):
    # No network, credentials, host config/cache, or image-controlled ignores.
    policy = directory.parent / 'gitleaks.toml'
    policy.write_text('[extend]\nuseDefault = true\n')
    args = ['docker', 'run', '--rm', '--platform', 'linux/amd64', '--pull', 'never', '--network', 'none',
            '--read-only', '--tmpfs', '/tmp',
            '--mount', f'type=bind,src={directory.parent},dst=/work,readonly',
            '--mount', f'type=bind,src={BASELINE},dst=/baseline.json,readonly',
            '--entrypoint', 'gitleaks', SCANNER_IMAGE, 'dir', '/work/inputs',
            '--config', '/work/gitleaks.toml', '--gitleaks-ignore-path', '/nonexistent',
            '--ignore-gitleaks-allow', '--no-banner', '--redact', '--log-level', 'error', '--exit-code', '9',
            '--baseline-path', '/baseline.json', '--report-format', 'json', '--report-path', '-']
    # stdout report avoids storing credential-bearing diagnostics. Gitleaks
    # can write the redacted JSON report directly to stdout.
    p = subprocess.run(args, capture_output=True, timeout=600)
    if p.returncode not in (0, 9):
        raise Incomplete(f'scanner failed (exit {p.returncode})')
    if p.stderr.strip():
        raise Incomplete('scanner reported an error')
    report = json.loads(p.stdout)
    if not isinstance(report, list) or bool(report) != (p.returncode == 9):
        raise Incomplete('scanner exit/report disagree')
    return report


def verdict(report, index):
    canary = False
    findings = set()
    for finding in report:
        name = PurePosixPath(finding['File']).name
        if name == 'canary.txt' and finding['RuleID'] == 'github-pat':
            canary = True
        elif name in index:
            findings.add((index[name], finding['RuleID']))
        else:
            raise Incomplete('scanner reported an unknown input')
    if not canary:
        raise Incomplete('scanner did not detect the synthetic canary')
    return sorted(findings)


def prepare():
    # This explicit verb is the ONLY network-reaching operation. Scans never
    # pull or update tools. The release producer calls it before building.
    print('Scanner download target: ' + SCANNER_IMAGE, flush=True)
    command(['docker', 'pull', '--platform', 'linux/amd64', SCANNER_IMAGE])


def scan_archive(archive, work):
    inputs = work / 'inputs'
    inputs.mkdir()
    index = project(surfaces(archive), inputs)
    return verdict(gitleaks(inputs), index)


def main():
    if len(sys.argv) != 2:
        print('usage: layer-secret-scan.sh <local-image> | --prepare', file=sys.stderr)
        return 2
    def interrupted(*_):
        raise Incomplete('interrupted')
    for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
        signal.signal(sig, interrupted)
    try:
        if sys.argv[1] == '--prepare':
            prepare()
            return 0
        with tempfile.TemporaryDirectory(prefix='image-secret-scan-') as temp:
            work = Path(temp)
            archive = work / 'image.tar'
            image_archive(sys.argv[1], archive)
            findings = scan_archive(archive, work)
            for surface, rule in findings:
                print('hit: ' + json.dumps({'surface': surface, 'rule': rule}))
            if findings:
                print(f'FAIL: credentials detected ({len(findings)} surface/rule pairs)')
                return 1
            print('clean: image text, config and history scanned by Gitleaks (normal binary handling)')
            return 0
    except (Incomplete, OSError, ValueError, KeyError, TypeError, EOFError,
            subprocess.TimeoutExpired, tarfile.TarError) as e:
        # Exceptions may embed archive/scanner content. Emit class only.
        reason = str(e) if isinstance(e, Incomplete) else type(e).__name__
        print('image-secret-scan: cannot scan (exit 2): ' + reason, file=sys.stderr)
        return 2
