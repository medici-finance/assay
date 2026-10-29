"""Write the release's software bill of materials, sbom.spdx.json (SPDX 2.3 JSON).

Every umbrella release publishes one SBOM covering every Go-built asset it
ships: the statusgen and qualgen binaries, and the desk-tools tarballs. It is
checksummed in checksums.txt and attested with every other asset, so the SBOM
a consumer reads is bound to the same provenance as the bytes it describes.

The dependency data comes from the Go build information the linker embeds in
every binary, read back with `go version -m <file>`; no other tool is
involved. The contract:

* one SPDX package per listed release asset, carrying the SHA256 of the asset
  file exactly as checksums.txt records it, and a DESCRIBES relationship from
  the document;
* a tarball's dependencies are the union over the Go binaries inside it (read
  from the tarball itself, so the SBOM describes the bytes published, not a
  build directory). Every member with an executable header (ELF, Mach-O, PE)
  must carry Go build information; other members (the bundled hook script)
  are not binaries and are skipped. A tarball with no Go binary is refused;
* every module a binary was linked from is a package with a
  `pkg:golang/<module>@<version>` purl and a DEPENDS_ON relationship from the
  asset; the Go toolchain is `pkg:golang/stdlib@<version>`; the main module is
  related by GENERATED_FROM. A replaced module is recorded as its replacement.
  A module replaced by a directory of the source tree has no module version,
  so its purl carries none. The go.sum `h1:` hash is a package comment (an
  SPDX checksum must name a real algorithm, and h1 is not a plain SHA256);
* the output is deterministic: sorted packages and relationships, SPDXIDs
  derived from names, `created` from the source commit time
  (SOURCE_DATE_EPOCH), and a documentNamespace built from the repository and
  the release tag. The same inputs give a byte-identical file;
* it fails closed: a listed asset missing, a binary without Go build
  information, an unparseable build-information line, or no module
  dependency found at all raises SBOMError and writes nothing.

It makes no network call: `go version -m` reads a local file, and runs with
GOTOOLCHAIN=local from an empty directory so no toolchain is ever fetched.

Imported by a fixed release workflow call; it has no command-line entrypoint.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import time
from urllib.parse import quote

SBOM_NAME = 'sbom.spdx.json'
TOOL = 'assay-release-sbom'
NAME = re.compile(r'[A-Za-z0-9][A-Za-z0-9._-]*')
REPOSITORY = re.compile(r'[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*')
TAG = re.compile(r'v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?')
GO_VERSION = re.compile(r'go([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:(?:rc|beta)[0-9]+)?)')
MODULE_HASH = re.compile(r'h[0-9]+:[A-Za-z0-9+/]+=*')
DEVEL = '(devel)'
# Executable headers: ELF, Mach-O (32/64-bit, both byte orders), universal Mach-O, PE.
EXECUTABLE_MAGIC = (b'\x7fELF', b'\xfe\xed\xfa\xce', b'\xfe\xed\xfa\xcf',
                    b'\xce\xfa\xed\xfe', b'\xcf\xfa\xed\xfe', b'\xca\xfe\xba\xbe', b'MZ')


class SBOMError(Exception):
    """The release must not publish: the SBOM cannot describe every asset."""


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, 'rb') as handle:
        for block in iter(lambda: handle.read(1 << 20), b''):
            digest.update(block)
    return digest.hexdigest()


def _module(fields, what):
    if len(fields) not in (2, 3) or not fields[0] or not fields[1]:
        raise SBOMError(f'{what}: unreadable module line {fields!r}')
    path, version = fields[0], fields[1]
    checksum = fields[2] if len(fields) == 3 else None
    if checksum is not None and not MODULE_HASH.fullmatch(checksum):
        raise SBOMError(f'{what}: {path} carries an unreadable module hash {checksum!r}')
    return {'path': path, 'version': version, 'sum': checksum, 'replace': None}


def parse_build_info(text, what='binary'):
    """Parse `go version -m <file>` output for one file.

    Returns {'go': 'go1.x.y', 'path': ..., 'main': module, 'deps': [module],
    'settings': {key: value}}, where a module is {'path', 'version', 'sum',
    'replace'} and 'replace' is the module the `=>` line names, or None.
    """
    lines = text.split('\n')
    while lines and not lines[-1].strip():
        lines.pop()
    if not lines:
        raise SBOMError(f'{what} has no Go build information')
    head = re.fullmatch(r'(.*): (\S+)', lines[0].rstrip('\r'))
    if not head or not GO_VERSION.fullmatch(head.group(2)):
        raise SBOMError(f'{what} has no Go build information ({lines[0]!r})')
    info = {'go': head.group(2), 'path': None, 'main': None, 'deps': [], 'settings': {}}
    last = None
    for line in lines[1:]:
        line = line.rstrip('\r')
        if not line.strip():
            continue
        fields = line.split('\t')
        if fields[0] != '' or len(fields) < 3:
            raise SBOMError(f'{what}: unreadable build information line {line!r}')
        fields = fields[1:]
        while fields and fields[-1] == '':
            fields.pop()
        kind, rest = fields[0], fields[1:]
        if kind == 'path':
            if info['path'] is not None or len(rest) != 1:
                raise SBOMError(f'{what}: unreadable path line {line!r}')
            info['path'] = rest[0]
            last = None
        elif kind == 'mod':
            if info['main'] is not None:
                raise SBOMError(f'{what}: two main modules')
            last = info['main'] = _module(rest, what)
        elif kind == 'dep':
            last = _module(rest, what)
            info['deps'].append(last)
        elif kind == '=>':
            if last is None or last['replace'] is not None or not rest:
                raise SBOMError(f'{what}: a replacement line follows no module line ({line!r})')
            if len(rest) == 1:  # a directory replacement on an older toolchain names no version
                rest = rest + [DEVEL]
            last['replace'] = _module(rest, what)
        elif kind == 'build':
            key, sep, value = '\t'.join(rest).partition('=')
            if not sep or not key:
                raise SBOMError(f'{what}: unreadable build setting {line!r}')
            info['settings'][key] = value
            last = None
        else:
            raise SBOMError(f'{what}: unknown build information line {line!r}')
    if info['path'] is None or info['main'] is None:
        raise SBOMError(f'{what} has no Go module information (no path or mod line)')
    return info


def effective(module):
    """(name, version or None, h1 or None, note) of the module actually linked in."""
    replace = module['replace']
    if replace is None:
        if module['version'] == DEVEL:
            return module['path'], None, None, 'built from the release source tree; no module version'
        return module['path'], module['version'], module['sum'], None
    if replace['version'] == DEVEL:
        return (module['path'], None, None,
                'replaced by a directory of the release source tree; no module version')
    return (replace['path'], replace['version'], replace['sum'],
            f'replaces {module["path"]} {module["version"]}')


def go_version_m(path, go='go'):
    """The `go version -m` text for one file; SBOMError if it has no Go build information."""
    env = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off', GOFLAGS='')
    with tempfile.TemporaryDirectory() as empty:
        try:
            done = subprocess.run([go, 'version', '-m', str(path)], cwd=empty, env=env,
                                  capture_output=True, text=True, timeout=120)
        except OSError as err:
            raise SBOMError(f'cannot run {go}: {err}') from None
    if done.returncode != 0:
        raise SBOMError(f'{path} has no Go build information: {done.stderr.strip() or done.stdout.strip()}')
    return done.stdout


def _is_executable(head):
    return any(head.startswith(magic) for magic in EXECUTABLE_MAGIC)


def tarball_infos(path, read):
    """[(member name, build info)] for every Go binary in the tarball at path."""
    found = []
    try:
        archive = tarfile.open(path, 'r:gz')
    except (OSError, tarfile.TarError) as err:
        raise SBOMError(f'{path.name} is not a readable tarball: {err}') from None
    with archive, tempfile.TemporaryDirectory() as scratch:
        for index, member in enumerate(archive.getmembers()):
            if not member.isreg():
                continue
            name = member.name[2:] if member.name.startswith('./') else member.name
            source = archive.extractfile(member)
            data = source.read() if source is not None else b''
            if not _is_executable(data[:4]):
                continue
            # Members are written to flat, numbered files: no member path is used on disk.
            target = Path(scratch) / str(index)
            target.write_bytes(data)
            what = f'{path.name}:{name}'
            try:
                found.append((name, parse_build_info(read(target), what)))
            except SBOMError as err:
                raise SBOMError(f'{what}: {err}') from None
            target.unlink()
    if not found:
        raise SBOMError(f'{path.name} contains no Go binary')
    return sorted(found, key=lambda item: item[0])


def _sanitize(text):
    return re.sub(r'[^A-Za-z0-9.-]+', '-', text).strip('-') or 'x'


def _purl(name, version):
    purl = 'pkg:golang/' + quote(name, safe='/-._~')
    return purl + ('@' + quote(version, safe='-._~') if version else '')


class _Packages:
    """Dependency packages keyed by (name, version), shared across assets."""

    def __init__(self):
        self.by_key = {}

    def add(self, name, version, checksum, note, purpose):
        key = (name, version)
        entry = self.by_key.get(key)
        if entry is None:
            digest = hashlib.sha256(f'{name}@{version or ""}'.encode()).hexdigest()[:12]
            spdx_id = f'SPDXRef-Go-{_sanitize(name)}-{_sanitize(version or "source")}-{digest}'
            entry = self.by_key[key] = {'id': spdx_id, 'name': name, 'version': version,
                                        'sum': checksum, 'notes': set(), 'purpose': purpose}
        elif entry['sum'] != checksum:
            raise SBOMError(f'{name} {version} is linked with two different module hashes '
                            f'({entry["sum"]} and {checksum})')
        if note:
            entry['notes'].add(note)
        return entry['id']

    def spdx(self):
        out = []
        for entry in self.by_key.values():
            purl_version = entry['version']
            if entry['name'] == 'stdlib':  # purl versions of the Go toolchain drop the "go" prefix
                purl_version = GO_VERSION.fullmatch(purl_version).group(1)
            package = {
                'SPDXID': entry['id'],
                'name': entry['name'],
                'downloadLocation': 'NOASSERTION',
                'filesAnalyzed': False,
                'licenseConcluded': 'NOASSERTION',
                'licenseDeclared': 'NOASSERTION',
                'copyrightText': 'NOASSERTION',
                'primaryPackagePurpose': entry['purpose'],
                'externalRefs': [{'referenceCategory': 'PACKAGE-MANAGER',
                                  'referenceType': 'purl',
                                  'referenceLocator': _purl(entry['name'], purl_version)}],
            }
            if entry['version']:
                package['versionInfo'] = entry['version']
            notes = sorted(entry['notes'])
            if entry['sum']:
                notes.insert(0, 'go.sum hash: ' + entry['sum'])
            if notes:
                package['comment'] = '; '.join(notes)
            out.append(package)
        return out


def document(assets, repository, tag, created):
    """The SPDX 2.3 document (a dict) for assets: [(name, sha256, [(binary, build info)])].

    An asset whose name ends in .tar.gz is an archive; any other is an application.
    """
    if not REPOSITORY.fullmatch(repository or ''):
        raise SBOMError(f'repository {repository!r} is not owner/name')
    if not TAG.fullmatch(tag or ''):
        raise SBOMError(f'release tag {tag!r} is not vX.Y.Z')
    if not isinstance(created, int) or isinstance(created, bool) or created < 0:
        raise SBOMError(f'creation time {created!r} is not a Unix epoch')
    if not assets:
        raise SBOMError('no release assets to describe')
    packages = _Packages()
    artifacts, relationships, ids = [], set(), {}
    module_deps = 0
    for name, digest, binaries in assets:
        spdx_id = 'SPDXRef-Artifact-' + _sanitize(name)
        if spdx_id in ids:
            raise SBOMError(f'{name} and {ids[spdx_id]} would share one SPDX identifier')
        ids[spdx_id] = name
        if not binaries:
            raise SBOMError(f'{name} contains no Go binary')
        archive = name.endswith('.tar.gz')
        relationships.add(('SPDXRef-DOCUMENT', 'DESCRIBES', spdx_id))
        for _, info in binaries:
            go = info['go']
            relationships.add((spdx_id, 'DEPENDS_ON', packages.add(
                'stdlib', go, None, 'the Go standard library and toolchain', 'LIBRARY')))
            main_name, main_version, main_sum, main_note = effective(info['main'])
            relationships.add((spdx_id, 'GENERATED_FROM', packages.add(
                main_name, main_version, main_sum, main_note, 'SOURCE')))
            for dep in info['deps']:
                module_deps += 1
                dep_name, dep_version, dep_sum, dep_note = effective(dep)
                relationships.add((spdx_id, 'DEPENDS_ON', packages.add(
                    dep_name, dep_version, dep_sum, dep_note, 'LIBRARY')))
        if archive:
            listed = ', '.join(sorted(binary for binary, _ in binaries))
            comment = f'desk-tools tarball; its {len(binaries)} Go binaries: {listed}'
        else:
            info = binaries[0][1]
            platform = '/'.join(info['settings'].get(k, '?') for k in ('GOOS', 'GOARCH'))
            comment = f'Go binary {info["path"]} for {platform}, built with {info["go"]}'
        artifacts.append({
            'SPDXID': spdx_id,
            'name': name,
            'versionInfo': tag,
            'downloadLocation': f'https://github.com/{repository}/releases/download/{tag}/{name}',
            'filesAnalyzed': False,
            'checksums': [{'algorithm': 'SHA256', 'checksumValue': digest}],
            'licenseConcluded': 'NOASSERTION',
            'licenseDeclared': 'NOASSERTION',
            'copyrightText': 'NOASSERTION',
            'primaryPackagePurpose': 'ARCHIVE' if archive else 'APPLICATION',
            'comment': comment,
        })
    if module_deps == 0:
        raise SBOMError('no module dependency found in any release asset; '
                        'a release SBOM with no dependencies describes nothing')
    return {
        'spdxVersion': 'SPDX-2.3',
        'dataLicense': 'CC0-1.0',
        'SPDXID': 'SPDXRef-DOCUMENT',
        'name': f'{repository.split("/")[1]}-{tag}',
        'documentNamespace': f'https://github.com/{repository}/releases/download/{tag}/{SBOM_NAME}',
        'creationInfo': {
            'created': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(created)),
            'creators': [f'Tool: {TOOL}'],
        },
        'packages': sorted(artifacts + packages.spdx(), key=lambda p: p['SPDXID']),
        'relationships': [{'spdxElementId': a, 'relationshipType': t, 'relatedSpdxElement': b}
                          for a, t, b in sorted(relationships)],
    }


def render(doc):
    return (json.dumps(doc, indent=2, sort_keys=True, ensure_ascii=True) + '\n').encode()


def write(root, names, out, repository, tag, created, read=go_version_m):
    """Describe every listed asset under root and write the SBOM to out.

    read(path) returns the `go version -m` text for one file (the default runs
    the Go toolchain on PATH). Every check runs before the file is written, so
    a refusal leaves no SBOM on disk.
    """
    root, out = Path(root), Path(out)
    names = list(names)
    if not names:
        raise SBOMError('no release assets listed')
    if len(set(names)) != len(names):
        raise SBOMError('a release asset is listed twice')
    if out.exists() or out.is_symlink():
        raise SBOMError(f'{out} already exists; an SBOM not produced by this run is never published')
    assets = []
    for name in sorted(names):
        if not NAME.fullmatch(name):
            raise SBOMError(f'{name!r} is not a release asset name')
        path = root / name
        if path.is_symlink() or not path.is_file():
            raise SBOMError(f'release asset {name} is missing or not a regular file')
        if name.endswith('.tar.gz'):
            binaries = tarball_infos(path, read)
        else:
            binaries = [(name, parse_build_info(read(path), name))]
        assets.append((name, sha256_file(path), binaries))
    raw = render(document(assets, repository, tag, created))
    partial = out.with_name(out.name + '.partial')
    partial.write_bytes(raw)
    os.replace(partial, out)
    return out
