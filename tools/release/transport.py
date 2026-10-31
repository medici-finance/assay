"""Checked release-file handoff through an explicitly configured S3 store.

GitHub job outputs, not bucket metadata, carry the expected digests. The AWS CLI
is checksum-pinned by the workflow; this adapter neither signs requests nor
resolves ambient credentials. Network activity requires upload/download commands.
"""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import urllib.parse

from tools.release.bundles import sha256_file

SCHEMA = 'release-handoff-v1'
MANIFEST = 'release-handoff.json'
MAX_BUNDLE = 16 << 30
MAX_CONTENT = 32 << 30
MAX_MANIFEST = 64 << 10
PLATFORMS = ('darwin-arm64', 'darwin-amd64', 'linux-amd64', 'windows-amd64', 'windows-arm64')


def metadata(repository, run, attempt, base, source, tag):
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('repository must be an explicit owner/name')
    if not all(re.fullmatch(r'[1-9][0-9]*', v) for v in (run, attempt)):
        raise ValueError('run and attempt must be positive decimal identities')
    if not all(re.fullmatch(r'[0-9a-f]{40}', v) for v in (base, source)):
        raise ValueError('base and prepared source must be full commit identities')
    if not re.fullmatch(r'v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)', tag) or len(tag) > 64:
        raise ValueError('a bounded bare umbrella version is required')
    return dict(schema=SCHEMA, repository=repository, run=run, attempt=attempt,
                base=base, source=source, tag=tag)


def members(meta):
    names = ['statusgen-linux-arm64', 'cursor-bundle.tar.gz', 'checksums.txt',
             'sbom.spdx.json', 'image-bugs-gc-linux-amd64', 'prepared-source.tar.gz',
             'build-inputs.sha256', 'release-highlights.md',
             'tool-validation-' + meta['tag'] + '.json', 'tool-validation-' + meta['tag'] + '.md']
    for platform in PLATFORMS:
        suffix = '.exe' if platform.startswith('windows-') else ''
        names += ['statusgen-' + platform + suffix, 'qualgen-' + platform + suffix,
                  'desk-tools-' + platform + '.tar.gz']
    if meta['source'] != meta['base']:
        names.append('release-source.bundle')
    return sorted(names)


def object_key(meta, digest):
    if not re.fullmatch(r'[0-9a-f]{64}', digest):
        raise ValueError('an expected bundle SHA256 is required')
    return '/'.join(('release-handoff', meta['repository'], meta['run'], meta['attempt'],
                     meta['source'], digest + '.tar.gz'))


def _regular(path):
    if path.is_symlink() or not path.is_file():
        raise ValueError('release inputs must be regular files: ' + path.name)


def _json(data):
    def unique(pairs):
        result = {}
        for k, v in pairs:
            if k in result:
                raise ValueError('duplicate handoff JSON key')
            result[k] = v
        return result
    return json.loads(data, object_pairs_hook=unique)


def prepare(root, bundle, meta):
    root, bundle = Path(root).resolve(), Path(bundle)
    expected = members(meta)
    for pattern in ('statusgen-*', 'qualgen-*', 'desk-tools-*.tar.gz', 'tool-validation-*', 'release-source.bundle'):
        found = {p.name for p in root.glob(pattern)}
        if not found <= set(expected):
            raise ValueError('unexpected prepared release file membership')
    records = []
    for name in expected:
        path = root / name
        _regular(path)
        size = path.stat().st_size
        records.append(dict(name=name, size=size, sha256=sha256_file(path)))
    if sum(r['size'] for r in records) > MAX_CONTENT:
        raise ValueError('prepared release exceeds the content bound')
    manifest = json.dumps(dict(meta, members=records), sort_keys=True, separators=(',', ':')).encode() + b'\n'
    if len(manifest) > MAX_MANIFEST:
        raise ValueError('release manifest exceeds its bound')
    with bundle.open('xb') as raw, gzip.GzipFile(fileobj=raw, mode='wb', compresslevel=1, mtime=0, filename='') as zipped:
        with tarfile.open(fileobj=zipped, mode='w', format=tarfile.USTAR_FORMAT) as archive:
            for name, size, stream in [(MANIFEST, len(manifest), io.BytesIO(manifest))] + [
                    (r['name'], r['size'], (root / r['name']).open('rb')) for r in records]:
                with stream:
                    info = tarfile.TarInfo(name)
                    info.size, info.mode, info.mtime = size, 0o644, 0
                    archive.addfile(info, stream)
    if bundle.stat().st_size > MAX_BUNDLE:
        raise ValueError('release bundle exceeds the transfer bound')
    return dict(bundle_sha256=sha256_file(bundle), manifest_sha256=hashlib.sha256(manifest).hexdigest(),
                bundle_bytes=bundle.stat().st_size, handoff_attempt=meta['attempt'])


def restore(bundle, destination, meta, expected_digest, expected_manifest, expected_size):
    bundle, destination = Path(bundle), Path(destination).resolve()
    object_key(meta, expected_digest)
    if not re.fullmatch(r'[0-9a-f]{64}', expected_manifest):
        raise ValueError('an expected manifest SHA256 is required')
    if type(expected_size) is not int or not 0 < expected_size <= MAX_BUNDLE:
        raise ValueError('an expected bounded bundle size is required')
    _regular(bundle)
    if bundle.stat().st_size != expected_size or sha256_file(bundle) != expected_digest:
        raise ValueError('release bundle differs from the trusted job output')
    destination.mkdir(parents=True, exist_ok=True)
    names = members(meta)
    if any((destination / name).exists() or (destination / name).is_symlink() for name in names):
        raise ValueError('refusing to replace existing release inputs')
    with tempfile.TemporaryDirectory(prefix='.release-handoff-', dir=destination) as scratch:
        scratch = Path(scratch)
        # Own the documented gzip reader instead of reading tarfile's private
        # streaming adapter past EOF (which can spin on Python 3.15).
        with gzip.open(bundle, 'rb') as plain, tarfile.open(fileobj=plain, mode='r|') as archive:
            first = archive.next()
            if first is None or first.name != MANIFEST or first.type not in (tarfile.REGTYPE, tarfile.AREGTYPE) or first.pax_headers or first.size > MAX_MANIFEST:
                raise ValueError('release bundle must start with its bounded manifest')
            manifest = archive.extractfile(first).read()
            if hashlib.sha256(manifest).hexdigest() != expected_manifest:
                raise ValueError('release manifest differs from the trusted job output')
            decoded = _json(manifest)
            if not isinstance(decoded, dict) or set(decoded) != set(meta) | {'members'} or any(decoded.get(k) != v for k, v in meta.items()):
                raise ValueError('release manifest belongs to a different run/source/version')
            records = decoded.get('members')
            if not isinstance(records, list) or len(records) != len(names) or not all(isinstance(r, dict) for r in records) or [r.get('name') for r in records] != names:
                raise ValueError('release manifest has incomplete or repeated membership')
            total = 0
            payload_end = 512 + ((len(manifest) + 511) // 512) * 512
            for record in records:
                if set(record) != {'name', 'size', 'sha256'} or type(record['size']) is not int or record['size'] < 0 or not isinstance(record['sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', record['sha256']):
                    raise ValueError('malformed release member record')
                total += record['size']
                if total > MAX_CONTENT:
                    raise ValueError('release content exceeds its bound')
                payload_end += 512 + ((record['size'] + 511) // 512) * 512
                entry = archive.next()
                if entry is None or entry.name != record['name'] or entry.type not in (tarfile.REGTYPE, tarfile.AREGTYPE) or entry.pax_headers or entry.size != record['size']:
                    raise ValueError('release archive membership or file type changed')
                h = hashlib.sha256()
                with archive.extractfile(entry) as source, (scratch / record['name']).open('xb') as target:
                    for chunk in iter(lambda: source.read(1 << 20), b''):
                        target.write(chunk)
                        h.update(chunk)
                if h.hexdigest() != record['sha256']:
                    raise ValueError('release member checksum mismatch')
            if archive.next() is not None:
                raise ValueError('release archive contains extra members')
            # Rewind the public gzip reader to the end derived from the verified
            # USTAR member sizes. Include the zero header already consumed by
            # archive.next(), preserving its previous 512 + 10240 byte bound.
            # The tar adapter's read-ahead buffer is intentionally not inspected.
            plain.seek(payload_end)
            padding = plain.read(512 + 10241)
            if len(padding) > 512 + 10240 or any(padding):
                raise ValueError('release archive has unexpected trailing data')
        # link is atomic and refuses an existing target; all payload checks finish
        # before any prepared file can enter the publisher workspace.
        for name in names:
            os.link(scratch / name, destination / name)


class Store:
    def __init__(self, aws, env):
        self.aws = Path(aws)
        if not self.aws.is_absolute() or not self.aws.is_file():
            raise ValueError('the explicit checksum-pinned AWS client is required')
        self.endpoint = env.get('RELEASE_OBJECT_ENDPOINT', '')
        parsed = urllib.parse.urlsplit(self.endpoint)
        if parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment or parsed.path not in ('', '/'):
            raise ValueError('an explicit HTTPS object endpoint is required')
        self.bucket = env.get('RELEASE_OBJECT_BUCKET', '')
        region = env.get('RELEASE_OBJECT_REGION', '')
        if not re.fullmatch(r'[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]', self.bucket) or not re.fullmatch(r'[a-z0-9-]+', region):
            raise ValueError('an explicit object bucket and region are required')
        key, secret = (env.get('RELEASE_OBJECT_' + k, '') for k in ('ACCESS_KEY_ID', 'SECRET_ACCESS_KEY'))
        if not key.strip() or not secret.strip():
            raise ValueError('dedicated transport-step credentials are required')
        self.env = {k: env[k] for k in ('PATH', 'LANG', 'LC_ALL', 'TMPDIR') if k in env}
        self.env.update(AWS_ACCESS_KEY_ID=key, AWS_SECRET_ACCESS_KEY=secret,
                        AWS_DEFAULT_REGION=region, AWS_REGION=region,
                        AWS_CONFIG_FILE='/dev/null', AWS_SHARED_CREDENTIALS_FILE='/dev/null',
                        AWS_EC2_METADATA_DISABLED='true', AWS_CLI_AUTO_PROMPT='off',
                        AWS_PAGER='', AWS_MAX_ATTEMPTS='1', AWS_RETRY_MODE='standard')
        if env.get('RELEASE_OBJECT_SESSION_TOKEN'):
            self.env['AWS_SESSION_TOKEN'] = env['RELEASE_OBJECT_SESSION_TOKEN']
        if env.get('RELEASE_OBJECT_CA_BUNDLE'):
            ca = Path(env['RELEASE_OBJECT_CA_BUNDLE'])
            try:
                valid = ca.is_absolute() and not ca.is_symlink() and ca.is_file()
            except (ValueError, OSError):
                valid = False
            if not valid:
                raise ValueError('the explicit CA bundle must be an absolute regular-file path')
            self.env['AWS_CA_BUNDLE'] = str(ca)

    def call(self, *args):
        result = subprocess.run([str(self.aws), *args, '--endpoint-url', self.endpoint,
                                 '--cli-connect-timeout', '20', '--cli-read-timeout', '120'],
                                env=self.env, text=True, capture_output=True)
        if result.returncode:
            # Private endpoint/bucket/credential detail must not enter public CI
            # logs. The workflow fails closed; never try another store or account.
            raise ValueError('configured object transfer failed; AWS client exit ' + str(result.returncode))
        return result.stdout

    def upload(self, bundle, key):
        print('target: release object ' + key, flush=True)
        self.call('s3', 'cp', str(bundle), 's3://' + self.bucket + '/' + key,
                  '--only-show-errors', '--no-progress')
        got = _json(self.call('s3api', 'head-object', '--bucket', self.bucket, '--key', key, '--output', 'json'))
        if not isinstance(got, dict) or type(got.get('ContentLength')) is not int or got['ContentLength'] != Path(bundle).stat().st_size:
            raise ValueError('stored release object length was not confirmed')

    def download(self, key, bundle, expected_size):
        print('target: release object ' + key, flush=True)
        got = _json(self.call('s3api', 'head-object', '--bucket', self.bucket, '--key', key, '--output', 'json'))
        if not isinstance(got, dict) or type(got.get('ContentLength')) is not int or got['ContentLength'] != expected_size:
            raise ValueError('stored release object differs from the trusted length')
        self.call('s3api', 'get-object', '--bucket', self.bucket, '--key', key,
                  '--range', 'bytes=0-' + str(expected_size - 1), str(bundle))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=('upload', 'download'))
    for name in ('repository', 'run', 'attempt', 'base', 'source', 'tag', 'root', 'bundle', 'aws'):
        parser.add_argument('--' + name, required=True)
    parser.add_argument('--expected-sha256')
    parser.add_argument('--expected-manifest-sha256')
    parser.add_argument('--expected-bytes', type=int)
    args = parser.parse_args()
    meta = metadata(args.repository, args.run, args.attempt, args.base, args.source, args.tag)
    if args.mode == 'upload':
        output = os.environ.get('GITHUB_OUTPUT')
        if not output:
            raise ValueError('trusted job output destination is required')
        store = Store(args.aws, os.environ)
        result = prepare(args.root, args.bundle, meta)
        store.upload(args.bundle, object_key(meta, result['bundle_sha256']))
        with open(output, 'a') as out:
            for key, value in result.items():
                out.write(key + '=' + str(value) + '\n')
    else:
        key = object_key(meta, args.expected_sha256 or '')
        if not re.fullmatch(r'[0-9a-f]{64}', args.expected_manifest_sha256 or '') or not args.expected_bytes or not 0 < args.expected_bytes <= MAX_BUNDLE:
            raise ValueError('trusted job output digests and size are required')
        store = Store(args.aws, os.environ)
        if Path(args.bundle).exists() or Path(args.bundle).is_symlink():
            raise ValueError('refusing to replace an existing bundle')
        store.download(key, args.bundle, args.expected_bytes)
        restore(args.bundle, args.root, meta, args.expected_sha256, args.expected_manifest_sha256, args.expected_bytes)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, tarfile.TarError, json.JSONDecodeError) as exc:
        raise SystemExit('release transport refused: ' + str(exc))
