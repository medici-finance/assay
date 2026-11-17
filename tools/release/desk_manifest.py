"""Write and check the MANIFEST.sha256 a desk-tools tarball carries (#6604).

The manifest describes exactly one artifact: the contents of the
`desk-tools-<platform>.tar.gz` it sits in. It is written into the staging
directory from the bytes that are then tarred, so the bytes it names are the
bytes that ship. It is NOT a manifest of the release: the standalone
`statusgen-<platform>` and `qualgen-<platform>` assets, and the tarballs
themselves, are covered by the release's `checksums.txt`.

The format is what `sha256sum -c` and `shasum -a 256 -c` read: one line per
regular file, `<64 lowercase hex>  <path relative to the tarball root>`, sorted
by path, no comments. The manifest does not list itself.

`check` is stricter than `sha256sum -c`, which only reads the lines present: it
also refuses a file in the directory that the manifest does not list, so a
manifest that leaves out a binary (as the stale one in #6604 left out two)
fails instead of passing on the entries it happens to carry.

Called from the release workflow's package step:

    python3 -B -m tools.release.desk_manifest write <stage-dir>
    python3 -B -m tools.release.desk_manifest check <stage-dir>

Exit 0 on success, 1 when `check` finds a problem, 2 when it cannot run.
"""
import hashlib
import os
from pathlib import Path
import re
import sys

MANIFEST_NAME = 'MANIFEST.sha256'
LINE = re.compile(r'([0-9a-f]{64})  (.+)')


class ManifestError(Exception):
    """The directory and its manifest disagree, or the manifest is unreadable."""


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, 'rb') as handle:
        for block in iter(lambda: handle.read(1 << 20), b''):
            digest.update(block)
    return digest.hexdigest()


def _files(root):
    """Every regular file under root as a sorted list of '/'-joined relative
    paths, the manifest itself excluded. A symlink or other non-regular entry is
    refused: the tarball must not carry anything the manifest cannot hash."""
    root = Path(root)
    if not root.is_dir():
        raise ManifestError(f'{root}: not a directory')
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        for name in sorted(dirnames + filenames):
            full = Path(dirpath) / name
            rel = full.relative_to(root).as_posix()
            if full.is_symlink():
                raise ManifestError(f'{rel}: symlink; the manifest covers regular files only')
            if full.is_dir():
                continue
            if not full.is_file():
                raise ManifestError(f'{rel}: not a regular file')
            if '\n' in rel or '\\' in rel:
                raise ManifestError(f'{rel!r}: a path the manifest format cannot carry')
            if rel != MANIFEST_NAME:
                found.append(rel)
    return sorted(found)


def write(root):
    """Write root/MANIFEST.sha256 over every regular file under root. Refuses an
    empty directory: a release must not ship a manifest that covers nothing."""
    root = Path(root)
    files = _files(root)
    if not files:
        raise ManifestError(f'{root}: no files to describe')
    text = ''.join(f'{sha256_file(root / rel)}  {rel}\n' for rel in files)
    target = root / MANIFEST_NAME
    tmp = root / (MANIFEST_NAME + '.tmp')
    tmp.write_text(text, encoding='utf-8')
    os.replace(tmp, target)
    return target


def check(root):
    """Return the list of problems between root and root/MANIFEST.sha256; an
    empty list is a clean check. Raises ManifestError when it cannot check."""
    root = Path(root)
    manifest = root / MANIFEST_NAME
    try:
        text = manifest.read_text(encoding='utf-8')
    except OSError as err:
        raise ManifestError(f'{manifest}: {err.strerror or err}') from err
    problems = []
    listed = {}
    for number, line in enumerate(text.splitlines(), 1):
        match = LINE.fullmatch(line)
        if not match:
            problems.append(f'line {number}: not "<sha256>  <path>"')
            continue
        digest, rel = match.groups()
        if rel in listed:
            problems.append(f'{rel}: listed twice')
        listed[rel] = digest
    present = set(_files(root))
    for rel, digest in sorted(listed.items()):
        if rel not in present:
            problems.append(f'{rel}: listed but not present')
        elif sha256_file(root / rel) != digest:
            problems.append(f'{rel}: sha256 does not match the manifest')
    for rel in sorted(present - set(listed)):
        problems.append(f'{rel}: present but not listed')
    return problems


def main(argv):
    if len(argv) != 2 or argv[0] not in ('write', 'check'):
        print('usage: desk_manifest write|check <dir>', file=sys.stderr)
        return 2
    verb, root = argv
    try:
        if verb == 'write':
            target = write(root)
            print(f'desk_manifest: wrote {target} ({len(_files(root))} files)')
            return 0
        problems = check(root)
    except ManifestError as err:
        print(f'desk_manifest: could not {verb}: {err}', file=sys.stderr)
        return 2
    for problem in problems:
        print(f'desk_manifest: {problem}', file=sys.stderr)
    if problems:
        return 1
    print(f'desk_manifest: {root}: every file matches {MANIFEST_NAME}')
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
