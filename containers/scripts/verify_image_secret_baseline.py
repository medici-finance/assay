"""Offline audit of the reviewed upstream baseline against downloaded archives.

Usage: python3 verify_image_secret_baseline.py <directory-containing-archives>
This verifies an existing baseline; it never adds or suppresses findings.
"""
import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile

HERE = Path(__file__).resolve().parent


def contents(archive):
    data = archive.read_bytes()
    if archive.suffix == '.deb':
        if not data.startswith(b'!<arch>\n'):
            raise ValueError('invalid Debian package')
        offset = 8
        while offset < len(data):
            header = data[offset:offset + 60]
            size = int(header[48:58])
            name = header[:16].decode().strip().rstrip('/')
            if name.startswith('data.tar'):
                data = data[offset + 60:offset + 60 + size]
                break
            offset += 60 + size + size % 2
        else:
            raise ValueError('Debian package has no data archive')
    with tarfile.open(fileobj=io.BytesIO(data)) as archive:
        return {m.name.removeprefix('./'): archive.extractfile(m).read()
                for m in archive if m.isfile()}


def verify(directory):
    inventory = json.loads((HERE / 'image-secret-baseline-inventory.json').read_text())
    baseline = json.loads((HERE / 'image-secret-baseline.json').read_text())
    upstream = {}
    for name, record in inventory['distributions'].items():
        archive = directory / name
        with archive.open('rb') as f:
            if hashlib.file_digest(f, 'sha256').hexdigest() != record['sha256']:
                raise ValueError('distribution checksum mismatch')
        upstream[name] = contents(archive)
    expected = set()
    for file in inventory['files']:
        path = file['path'].lstrip('/')
        name = file['distribution']
        if name.startswith('go'):
            member = 'go/' + path.removeprefix('usr/local/go/')
        elif name.startswith('node'):
            member = name.removesuffix('.tar.xz') + '/' + path.removeprefix('usr/local/')
        else:
            member = path
        data = upstream[name][member]
        if b'\0' in data or hashlib.sha256(data).hexdigest() != file['sha256']:
            raise ValueError('source bytes differ from inventory')
        identity = hashlib.sha256(path.encode() + b'\0' + data).hexdigest() + '.txt'
        if identity != file['input']:
            raise ValueError('source identity differs from inventory')
        expected.update((identity, finding['rule'], finding['line']) for finding in file['findings'])
    observed = set()
    for row in baseline:
        if 'Secret' in row or 'Match' in row or 'Line' in row:
            raise ValueError('baseline must not carry matched material')
        observed.add((Path(row['File']).name, row['RuleID'], row['StartLine']))
    if observed != expected:
        raise ValueError('baseline and provenance inventory differ')
    print(f"Verified {len(inventory['files'])} upstream text files and {len(baseline)} baseline records")


if __name__ == '__main__':
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    verify(Path(sys.argv[1]))
