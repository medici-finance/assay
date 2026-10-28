"""Offline fixtures for the release SBOM lane (#4716).

They cover tools/release/sbom.py: the release's artifacts job writes one SPDX
2.3 JSON document, sbom.spdx.json, describing every Go-built asset from the Go
build information embedded in the binaries, and refuses (writing nothing) when
it cannot describe every asset. Parsing runs on `go version -m` text fixtures,
including replace directives; one fixture builds a real binary with the local
Go toolchain (skipped only when `go` is not on PATH). They also pin the
release.yml wiring with planted-defect mutants, the same way bundles_test.py
pins the provenance lane: the SBOM is generated in the artifacts job before
the checksums, and is checksummed, uploaded, attested and published.
"""
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('sbom', HERE / 'sbom.py')
sbom = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sbom)

PLATFORMS = ['darwin-arm64', 'darwin-amd64', 'linux-amd64', 'windows-amd64', 'windows-arm64']
BINARIES = [f'{tool}-{p}' + ('.exe' if p.startswith('windows-') else '')
            for tool in ('statusgen', 'qualgen') for p in PLATFORMS]
# statusgen alone also ships for linux-arm64: the arm64 verify-runner image's (#4323).
BINARIES.append('statusgen-linux-arm64')
TARBALLS = [f'desk-tools-{p}.tar.gz' for p in PLATFORMS]
GO_ASSETS = BINARIES + TARBALLS
REPO, TAG, EPOCH = 'example-org/assay', 'v1.2.3', 1761580800

STATUSGEN_INFO = (
    '{file}: go1.27.2\n'
    '\tpath\texample.com/assay/statusgen\n'
    '\tmod\texample.com/assay/statusgen\t(devel)\t\n'
    '\tdep\texample.com/assay/deskcore\tv0.0.0-00010101000000-000000000000\n'
    '\t=>\t../deskcore\t(devel)\t\n'
    '\t\n'
    '\tdep\tgithub.com/spf13/cobra\tv1.10.2\th1:DMTTonx5m65Ic0GOoRY2c16WCbHxOOw6xxezuLaBpcU=\n'
    '\tdep\tgolang.org/x/sys\tv0.46.0\th1:noSf2Fq6F8DBgS+LysIkx7rIExoNHJsxOAtPp4rthXw=\n'
    '\tdep\texample.com/old\tv1.0.0\th1:b2xkb2xkb2xkb2xkb2xkb2xkb2xkb2xkb2xkb2xkb2w=\n'
    '\t=>\texample.com/new\tv1.1.0+incompatible\th1:bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmU=\n'
    '\tbuild\t-buildmode=exe\n'
    '\tbuild\t-ldflags="-X main.statusgenVersion=v1.2.3"\n'
    '\tbuild\tCGO_ENABLED=0\n'
    '\tbuild\tGOARCH=amd64\n'
    '\tbuild\tGOOS=linux\n'
)
DESK_INFO = (
    '{file}: go1.27.2\n'
    '\tpath\texample.com/assay/tools/desk/cmd/{cmd}\n'
    '\tmod\texample.com/assay/tools/desk\t(devel)\t\n'
    '\tdep\tgithub.com/spf13/cobra\tv1.10.2\th1:DMTTonx5m65Ic0GOoRY2c16WCbHxOOw6xxezuLaBpcU=\n'
    '\tdep\tgithub.com/{cmd}/only\tv0.{n}.0\th1:b25seW9ubHlvbmx5b25seW9ubHlvbmx5b25seW9ubHk=\n'
    '\tbuild\tGOARCH=amd64\n'
    '\tbuild\tGOOS=linux\n'
)
NO_DEPS_INFO = (
    '{file}: go1.27.2\n'
    '\tpath\texample.com/tiny\n'
    '\tmod\texample.com/tiny\t(devel)\t\n'
    '\tbuild\tGOOS=linux\n'
)


class FakeGo:
    """Stands in for `go version -m`: build info keyed by the file's bytes."""

    def __init__(self):
        self.infos = {}

    def binary(self, label, info):
        data = b'\x7fELF fake binary ' + label.encode()
        self.infos[data] = info
        return data

    def __call__(self, path):
        data = Path(path).read_bytes()
        if data not in self.infos:
            raise sbom.SBOMError(f'{path}: could not read Go build info')
        return self.infos[data].replace('{file}', str(path))


def tarball(members):
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode='w:gz') as archive:
        for name, data in members:
            entry = tarfile.TarInfo(name)
            entry.size = len(data)
            entry.mode = 0o755
            archive.addfile(entry, io.BytesIO(data))
    return buffer.getvalue()


class Release:
    """A release root holding every Go asset, with a FakeGo that knows their build info."""

    def __init__(self, root):
        self.root = root
        self.go = FakeGo()
        for name in BINARIES:
            (root / name).write_bytes(self.go.binary(name, STATUSGEN_INFO))
        for name in TARBALLS:
            members = [('./hooks/pre-push', b'#!/bin/sh\nexit 0\n')]
            for n, cmd in enumerate(['deskwt', 'deskinstall']):
                members.append(('./' + cmd, self.go.binary(name + cmd, DESK_INFO.replace('{cmd}', cmd)
                                                           .replace('{n}', str(n)))))
            (root / name).write_bytes(tarball(members))
        self.out = root / 'sbom.spdx.json'

    def write(self, names=None, out=None, **over):
        kwargs = dict(repository=REPO, tag=TAG, created=EPOCH, read=self.go)
        kwargs.update(over)
        return sbom.write(self.root, GO_ASSETS if names is None else names,
                          out or self.out, **kwargs)

    def doc(self):
        return json.loads(self.out.read_text())


SPDX_ID = re.compile(r'SPDXRef-[A-Za-z0-9.-]+')
CHECKSUM_ALGORITHMS = {'SHA1', 'SHA224', 'SHA256', 'SHA384', 'SHA512', 'SHA3-256', 'SHA3-384',
                       'SHA3-512', 'BLAKE2b-256', 'BLAKE2b-384', 'BLAKE2b-512', 'BLAKE3',
                       'MD2', 'MD4', 'MD5', 'MD6', 'ADLER32'}


def spdx_problems(doc):
    """The SPDX 2.3 JSON schema's required fields, plus the identifiers they reference."""
    problems = []
    for key, want in (('spdxVersion', 'SPDX-2.3'), ('dataLicense', 'CC0-1.0'),
                      ('SPDXID', 'SPDXRef-DOCUMENT')):
        if doc.get(key) != want:
            problems.append(f'{key} is {doc.get(key)!r}, want {want!r}')
    if not doc.get('name'):
        problems.append('document has no name')
    if not re.fullmatch(r'https?://\S+', doc.get('documentNamespace', '')) or '#' in doc['documentNamespace']:
        problems.append('documentNamespace is not an absolute URI without a fragment')
    info = doc.get('creationInfo', {})
    if not re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ', info.get('created', '')):
        problems.append('creationInfo.created is not YYYY-MM-DDThh:mm:ssZ')
    if not info.get('creators') or not all(re.match(r'(Tool|Organization|Person): \S', c)
                                            for c in info['creators']):
        problems.append('creationInfo.creators is empty or malformed')
    ids = {'SPDXRef-DOCUMENT'}
    for package in doc.get('packages', []):
        for key in ('SPDXID', 'name', 'downloadLocation'):
            if not package.get(key):
                problems.append(f'package {package.get("name")!r} has no {key}')
        if not SPDX_ID.fullmatch(package.get('SPDXID', '')):
            problems.append(f'package SPDXID {package.get("SPDXID")!r} is malformed')
        if package.get('SPDXID') in ids:
            problems.append(f'SPDXID {package.get("SPDXID")} is used twice')
        ids.add(package.get('SPDXID'))
        if package.get('filesAnalyzed', True) and 'packageVerificationCode' not in package:
            problems.append(f'package {package.get("name")!r} analyses files it does not list')
        for checksum in package.get('checksums', []):
            if checksum.get('algorithm') not in CHECKSUM_ALGORITHMS:
                problems.append(f'{package["name"]}: checksum algorithm {checksum.get("algorithm")!r}')
            if checksum.get('algorithm') == 'SHA256' and not re.fullmatch(r'[0-9a-f]{64}', checksum.get('checksumValue', '')):
                problems.append(f'{package["name"]}: SHA256 value is not 64 hex')
        for ref in package.get('externalRefs', []):
            if not all(ref.get(k) for k in ('referenceCategory', 'referenceType', 'referenceLocator')):
                problems.append(f'{package["name"]}: incomplete externalRef')
    for rel in doc.get('relationships', []):
        if not all(rel.get(k) for k in ('spdxElementId', 'relationshipType', 'relatedSpdxElement')):
            problems.append(f'incomplete relationship {rel}')
        elif rel['spdxElementId'] not in ids or rel['relatedSpdxElement'] not in ids:
            problems.append(f'relationship names an unknown element: {rel}')
    if not any(r.get('relationshipType') == 'DESCRIBES' for r in doc.get('relationships', [])):
        problems.append('the document DESCRIBES nothing')
    return problems


def purl_of(package):
    return next(r['referenceLocator'] for r in package['externalRefs'] if r['referenceType'] == 'purl')


class ParseTest(unittest.TestCase):
    def test_parse_reads_every_line_kind(self):
        info = sbom.parse_build_info(STATUSGEN_INFO.replace('{file}', 'statusgen-linux-amd64'))
        self.assertEqual(info['go'], 'go1.27.2')
        self.assertEqual(info['path'], 'example.com/assay/statusgen')
        self.assertEqual(info['main']['version'], '(devel)')
        self.assertEqual([d['path'] for d in info['deps']],
                         ['example.com/assay/deskcore', 'github.com/spf13/cobra',
                          'golang.org/x/sys', 'example.com/old'])
        self.assertEqual(info['deps'][0]['replace']['path'], '../deskcore')
        self.assertEqual(info['deps'][3]['replace'],
                         {'path': 'example.com/new', 'version': 'v1.1.0+incompatible',
                          'sum': 'h1:bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmU=', 'replace': None})
        self.assertEqual(info['settings']['GOOS'], 'linux')
        self.assertEqual(info['settings']['-ldflags'], '"-X main.statusgenVersion=v1.2.3"')

    def test_replacement_is_what_is_recorded(self):
        info = sbom.parse_build_info(STATUSGEN_INFO.replace('{file}', 'x'))
        self.assertEqual(sbom.effective(info['deps'][3]),
                         ('example.com/new', 'v1.1.0+incompatible',
                          'h1:bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmV3bmU=',
                          'replaces example.com/old v1.0.0'))
        name, version, checksum, note = sbom.effective(info['deps'][0])
        self.assertEqual((name, version, checksum), ('example.com/assay/deskcore', None, None))
        self.assertIn('source tree', note)

    def test_parse_refusals(self):
        good = STATUSGEN_INFO.replace('{file}', 'x')
        cases = {
            'empty': '',
            'not go build info': 'x: could not read Go build info from x: not a Go executable\n',
            'devel toolchain': good.replace('go1.27.2', 'devel go1.28-abcdef', 1),
            'unknown line kind': good + '\tlicense\tMIT\n',
            'unindented line': good + 'dep\tx\tv1\n',
            'replacement with no module': 'x: go1.27.2\n\tpath\tp\n\t=>\ta\tv1\n\tmod\tp\t(devel)\n',
            'second replacement': good.replace('\t=>\t../deskcore\t(devel)\t\n',
                                               '\t=>\t../deskcore\t(devel)\t\n\t=>\t../other\t(devel)\n'),
            'replacement after build line': good + '\t=>\tx\tv1\n',
            'unreadable module hash': good.replace('h1:DMTT', 'sha256:DMTT', 1),
            'module line field count': good + '\tdep\tx\tv1\th1:AA=\textra\n',
            'two main modules': good + '\tmod\tother\tv1\n',
            'no main module': good.replace('\tmod\texample.com/assay/statusgen\t(devel)\t\n', ''),
            'no path': good.replace('\tpath\texample.com/assay/statusgen\n', ''),
        }
        for name, text in cases.items():
            with self.subTest(name=name):
                with self.assertRaises(sbom.SBOMError):
                    sbom.parse_build_info(text)


class DocumentTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.rel = Release(Path(temp.name))

    def test_every_go_asset_is_described_with_its_checksummed_bytes(self):
        self.rel.write()
        doc = self.rel.doc()
        self.assertEqual(spdx_problems(doc), [])
        by_id = {p['SPDXID']: p for p in doc['packages']}
        described = {by_id[r['relatedSpdxElement']]['name'] for r in doc['relationships']
                     if r['spdxElementId'] == 'SPDXRef-DOCUMENT' and r['relationshipType'] == 'DESCRIBES'}
        self.assertEqual(described, set(GO_ASSETS))
        for package in doc['packages']:
            if package['name'] in GO_ASSETS:
                digest = hashlib.sha256((self.rel.root / package['name']).read_bytes()).hexdigest()
                self.assertEqual(package['checksums'], [{'algorithm': 'SHA256', 'checksumValue': digest}])
                self.assertEqual(package['versionInfo'], TAG)
                self.assertEqual(package['primaryPackagePurpose'],
                                 'ARCHIVE' if package['name'].endswith('.tar.gz') else 'APPLICATION')

    def test_dependencies_carry_purls_and_relationships(self):
        self.rel.write()
        doc = self.rel.doc()
        by_id = {p['SPDXID']: p for p in doc['packages']}
        by_name = {p['name']: p for p in doc['packages']}

        def related(asset, kind):
            return {purl_of(by_id[r['relatedSpdxElement']]) for r in doc['relationships']
                    if r['spdxElementId'] == by_name[asset]['SPDXID'] and r['relationshipType'] == kind}

        self.assertEqual(related('statusgen-linux-amd64', 'DEPENDS_ON'), {
            'pkg:golang/stdlib@1.27.2',
            'pkg:golang/example.com/assay/deskcore',
            'pkg:golang/github.com/spf13/cobra@v1.10.2',
            'pkg:golang/golang.org/x/sys@v0.46.0',
            'pkg:golang/example.com/new@v1.1.0%2Bincompatible'})
        self.assertEqual(related('statusgen-linux-amd64', 'GENERATED_FROM'),
                         {'pkg:golang/example.com/assay/statusgen'})
        # A tarball depends on the union over the Go binaries inside it; the hook script is no binary.
        self.assertEqual(related('desk-tools-linux-amd64.tar.gz', 'DEPENDS_ON'), {
            'pkg:golang/stdlib@1.27.2', 'pkg:golang/github.com/spf13/cobra@v1.10.2',
            'pkg:golang/github.com/deskwt/only@v0.0.0', 'pkg:golang/github.com/deskinstall/only@v0.1.0'})
        self.assertIn('deskinstall, deskwt', by_name['desk-tools-linux-amd64.tar.gz']['comment'])
        self.assertNotIn('example.com/old', by_name)
        self.assertIn('replaces example.com/old v1.0.0', by_name['example.com/new']['comment'])
        self.assertEqual(by_name['stdlib']['versionInfo'], 'go1.27.2')

    def test_module_hash_is_never_mislabelled_as_a_checksum(self):
        self.rel.write()
        doc = self.rel.doc()
        cobra = next(p for p in doc['packages'] if p['name'] == 'github.com/spf13/cobra')
        self.assertNotIn('checksums', cobra)
        self.assertIn('go.sum hash: h1:DMTTonx5m65Ic0GOoRY2c16WCbHxOOw6xxezuLaBpcU=', cobra['comment'])
        for package in doc['packages']:
            for checksum in package.get('checksums', []):
                self.assertNotIn('h1:', checksum['checksumValue'])

    def test_output_is_deterministic(self):
        self.rel.write()
        first = self.rel.out.read_bytes()
        again = self.rel.root / 'again.json'
        self.rel.write(names=list(reversed(GO_ASSETS)), out=again)
        self.assertEqual(again.read_bytes(), first)
        doc = json.loads(first)
        self.assertEqual(doc['creationInfo'], {'created': '2025-10-27T16:00:00Z',
                                               'creators': ['Tool: assay-release-sbom']})
        self.assertEqual(doc['documentNamespace'],
                         'https://github.com/example-org/assay/releases/download/v1.2.3/sbom.spdx.json')
        self.assertEqual([p['SPDXID'] for p in doc['packages']], sorted(p['SPDXID'] for p in doc['packages']))
        keys = [(r['spdxElementId'], r['relationshipType'], r['relatedSpdxElement']) for r in doc['relationships']]
        self.assertEqual(keys, sorted(set(keys)))
        later = self.rel.root / 'later.json'
        self.rel.write(out=later, created=EPOCH + 1)
        self.assertNotEqual(later.read_bytes(), first)

    def test_refusals_write_nothing(self):
        root = self.rel.root
        cases = {}

        def case(name, setup, **over):
            cases[name] = (setup, over)

        case('missing asset', lambda: (root / 'qualgen-linux-amd64').unlink())
        case('asset is a symlink', lambda: ((root / 'qualgen-linux-amd64').unlink(),
                                            (root / 'qualgen-linux-amd64').symlink_to(root / 'statusgen-linux-amd64')))
        case('binary without build info', lambda: (root / 'statusgen-darwin-arm64').write_bytes(b'\x7fELF stripped'))
        case('tarball binary without build info', lambda: (root / 'desk-tools-darwin-amd64.tar.gz').write_bytes(
            tarball([('./deskwt', b'\xcf\xfa\xed\xfe not go'), ('./hooks/pre-push', b'#!/bin/sh\n')])))
        case('tarball with no binary', lambda: (root / 'desk-tools-windows-arm64.tar.gz').write_bytes(
            tarball([('./hooks/pre-push', b'#!/bin/sh\n')])))
        case('tarball unreadable', lambda: (root / 'desk-tools-linux-amd64.tar.gz').write_bytes(b'not gzip'))
        case('sbom already on disk', lambda: self.rel.out.write_text('{}'))
        case('bad tag', lambda: None, tag='v1.2.3"; rm')
        case('bad repository', lambda: None, repository='not a repo')
        case('negative time', lambda: None, created=-1)
        case('duplicate asset', lambda: None, names=GO_ASSETS + ['qualgen-linux-amd64'])
        case('no assets', lambda: None, names=[])
        case('path as asset name', lambda: None, names=['../statusgen-linux-amd64'])

        def no_deps():
            for name in BINARIES:
                (root / name).write_bytes(self.rel.go.binary('tiny' + name, NO_DEPS_INFO))
            for name in TARBALLS:
                (root / name).write_bytes(tarball([('./x', self.rel.go.binary('tiny' + name, NO_DEPS_INFO))]))
        case('no dependency at all', no_deps)

        def conflicting_hash():
            other = STATUSGEN_INFO.replace('h1:DMTT', 'h1:XXXX', 1)
            (root / 'qualgen-linux-amd64').write_bytes(self.rel.go.binary('conflict', other))
        case('one module two hashes', conflicting_hash)

        for name, (setup, over) in cases.items():
            with self.subTest(name=name):
                with tempfile.TemporaryDirectory() as temp:
                    self.rel = Release(Path(temp))
                    root = self.rel.root
                    setup()
                    already = self.rel.out.exists()
                    with self.assertRaises(sbom.SBOMError):
                        self.rel.write(**over)
                    if not already:
                        self.assertFalse(self.rel.out.exists())
                    self.assertFalse((root / 'sbom.spdx.json.partial').exists())


GO = shutil.which('go')


@unittest.skipUnless(GO, 'the go toolchain is not on PATH')
class RealBinaryTest(unittest.TestCase):
    """One real `go build` (linux/amd64, cached by the real GOCACHE) read by the real `go version -m`."""

    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        base = Path(cls.temp.name)
        src = base / 'src'
        (src / 'dep').mkdir(parents=True)
        (src / 'go.mod').write_text('module example.com/fixture\n\ngo 1.21\n\n'
                                    'require example.com/dep v1.0.0\n\n'
                                    'replace example.com/dep => ./dep\n')
        (src / 'main.go').write_text('package main\n\nimport (\n\t"os"\n\n\t"example.com/dep"\n)\n\n'
                                     'func main() { os.Stdout.WriteString(dep.Name()) }\n')
        (src / 'dep' / 'go.mod').write_text('module example.com/dep\n\ngo 1.21\n')
        (src / 'dep' / 'dep.go').write_text('package dep\n\nfunc Name() string { return "dep" }\n')
        cls.root = base / 'release'
        cls.root.mkdir()
        env = dict(os.environ, GOOS='linux', GOARCH='amd64', CGO_ENABLED='0', GOFLAGS='-mod=mod',
                   GOPROXY='off', GOTOOLCHAIN='local', GOWORK='off')
        subprocess.run([GO, 'build', '-buildvcs=false', '-o', str(cls.root / 'fixture-linux-amd64'), '.'],
                       cwd=src, env=env, check=True, capture_output=True, timeout=300)
        binary = (cls.root / 'fixture-linux-amd64').read_bytes()
        (cls.root / 'fixture-tools-linux-amd64.tar.gz').write_bytes(
            tarball([('./fixture', binary), ('./hooks/pre-push', b'#!/bin/sh\nexit 0\n')]))
        (cls.root / 'not-go').write_bytes(b'#!/bin/sh\necho not a go binary\n')

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def test_real_build_info_feeds_the_document(self):
        out = self.root / 'sbom.spdx.json'
        if out.exists():
            out.unlink()
        sbom.write(self.root, ['fixture-linux-amd64', 'fixture-tools-linux-amd64.tar.gz'], out,
                   REPO, TAG, EPOCH)
        doc = json.loads(out.read_text())
        self.assertEqual(spdx_problems(doc), [])
        go_version = sbom.parse_build_info(sbom.go_version_m(self.root / 'fixture-linux-amd64'))['go']
        purls = {purl_of(p) for p in doc['packages'] if p.get('externalRefs')}
        self.assertIn('pkg:golang/stdlib@' + go_version[2:], purls)
        self.assertIn('pkg:golang/example.com/dep', purls)  # replaced by a local directory
        self.assertIn('pkg:golang/example.com/fixture', purls)
        assets = {p['name'] for p in doc['packages'] if p.get('checksums')}
        self.assertEqual(assets, {'fixture-linux-amd64', 'fixture-tools-linux-amd64.tar.gz'})
        artifact = next(p for p in doc['packages'] if p['name'] == 'fixture-linux-amd64')
        self.assertIn('linux/amd64', artifact['comment'])
        out.unlink()

    def test_real_toolchain_refuses_a_file_without_build_info(self):
        with self.assertRaises(sbom.SBOMError):
            sbom.go_version_m(self.root / 'not-go')
        with self.assertRaises(sbom.SBOMError):
            sbom.go_version_m(self.root / 'does-not-exist')


def workflow_jobs(text):
    return dict(re.findall(r'(?ms)^  ([a-zA-Z0-9_-]+):\n(.*?)(?=^  [a-zA-Z0-9_-]+:\n|\Z)',
                           text.split('\njobs:\n', 1)[1]))


def workflow_steps(block):
    return re.findall(r'(?ms)^      - (?:name: ([^\n]+)|uses: ([^\n]+))\n(.*?)(?=^      - |\Z)', block)


STEP = 'Generate the SBOM'
CHECKSUMS = 'Generate checksums'
COMMON = 'Upload prepared release artifacts (common)'
ASSETS = 'List the release assets to attest'
PUBLISH = 'Create GitHub release and upload assets'
WRITE_CALL = ("write(Path.cwd(), sys.argv[1:], Path('sbom.spdx.json'), os.environ['REPOSITORY'],\n"
              "                os.environ['RELEASE_TAG'], int(os.environ['SOURCE_DATE_EPOCH']))")


def token(name, text):
    return re.search(r'(?<![\w.*-])' + re.escape(name) + r'(?![\w.*-])', text) is not None


def wiring_problems(text):
    """release.yml must build the SBOM before the checksums and publish it like every asset."""
    problems = []
    jobs = workflow_jobs(text)
    steps = workflow_steps(jobs.get('artifacts', ''))
    order = [name or uses for name, uses, _ in steps]
    body = {name or uses: block for name, uses, block in steps}
    wanted = ['Build and package desk-tools binaries', 'Build versioned Cursor source bundle', STEP, CHECKSUMS]
    if not all(w in order for w in wanted):
        problems.append('missing artifacts step: ' + ', '.join(w for w in wanted if w not in order))
    elif [order.index(w) for w in wanted] != sorted(order.index(w) for w in wanted):
        problems.append('the SBOM must follow every build and precede the checksums')
    if order.count(STEP) > 1:
        problems.append('the SBOM is generated twice')
    for name, block in jobs.items():
        if name != 'artifacts' and 'tools.release.sbom' in block:
            problems.append('the SBOM must be generated in the artifacts job, not ' + name)
    step = body.get(STEP, '')
    run = step.split('        run: |\n', 1)[1] if '        run: |\n' in step else ''
    if not run:
        problems.append('the SBOM step has no run block')
    if '${{' in run:
        problems.append('the SBOM run block splices an expression; values ride in via env:')
    for value in ('RELEASE_TAG: ${{ needs.resolve.outputs.tag }}', 'REPOSITORY: ${{ github.repository }}'):
        if value not in step.split('        run: |\n', 1)[0]:
            problems.append('missing SBOM env: ' + value)
    for value in ('SOURCE_DATE_EPOCH=$(git log -1 --format=%ct HEAD)', 'export SOURCE_DATE_EPOCH',
                  'from tools.release.sbom import write', WRITE_CALL):
        if value not in run:
            problems.append('missing SBOM call: ' + value)
    if re.search(r'^        if:', step, re.M):
        problems.append('the SBOM step must run on every path, the dry run included')
    if re.search(r'^        continue-on-error:', step, re.M):
        problems.append('a failed SBOM must stop the release')
    listed = run.split('<<', 1)[0]
    for name in GO_ASSETS:
        if not token(name, listed):
            problems.append('the SBOM does not describe ' + name)
    checksummed = body.get(CHECKSUMS, '').split('> checksums.txt', 1)[0]
    if not token('sbom.spdx.json', checksummed):
        problems.append('sbom.spdx.json is not in checksums.txt')
    if not re.search(r'^            sbom\.spdx\.json$', body.get(COMMON, ''), re.M):
        problems.append('sbom.spdx.json is not uploaded with the common release artifacts')
    publisher = {name or uses: block for name, uses, block in workflow_steps(jobs.get('release', ''))}
    attested = publisher.get(ASSETS, '').split('; do', 1)[0]
    if not token('sbom.spdx.json', attested):
        problems.append('sbom.spdx.json is not in the published, attested asset list')
    if 'Software bill of materials: sbom.spdx.json' not in publisher.get(PUBLISH, ''):
        problems.append('the release body does not name sbom.spdx.json')
    return problems


def in_step(text, name, old, new):
    start = text.index('      - name: ' + name + '\n')
    end = text.find('\n      - ', start + 1)
    end = len(text) if end < 0 else end
    block = text[start:end]
    return text[:start] + block.replace(old, new, 1) + text[end:]


class WorkflowWiringTest(unittest.TestCase):
    PATHS = ['.github/workflows/release.yml', 'ci/staged-workflows/release.yml']

    def test_release_builds_and_publishes_the_sbom(self):
        root = HERE.parents[1]
        for path in self.PATHS:
            with self.subTest(path=path):
                self.assertEqual(wiring_problems((root / path).read_text()), [])

    def test_wiring_regressions_are_rejected(self):
        main = (HERE.parents[1] / '.github/workflows/release.yml').read_text()
        self.assertEqual(wiring_problems(main), [])
        guard = "        if: ${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}\n"
        mutants = {
            'no-sbom-step': main.replace('- name: ' + STEP + '\n', '- name: Something else\n', 1),
            'sbom-after-checksums': main.replace('- name: ' + STEP + '\n', '- name: TEMP\n', 1)
                                        .replace('- name: ' + CHECKSUMS + '\n', '- name: ' + STEP + '\n', 1)
                                        .replace('- name: TEMP\n', '- name: ' + CHECKSUMS + '\n', 1),
            'sbom-skipped-on-dry-run': main.replace('- name: ' + STEP + '\n', '- name: ' + STEP + '\n' + guard, 1),
            'ignored-failure': main.replace('- name: ' + STEP + '\n',
                                            '- name: ' + STEP + '\n        continue-on-error: true\n', 1),
            'tag-spliced': in_step(main, STEP, "os.environ['RELEASE_TAG']", "'${{ needs.resolve.outputs.tag }}'"),
            'no-tag-env': in_step(main, STEP, 'RELEASE_TAG: ${{ needs.resolve.outputs.tag }}', 'TAG: v0.0.0'),
            'no-source-time': in_step(main, STEP, 'SOURCE_DATE_EPOCH=$(git log -1 --format=%ct HEAD)',
                                      'SOURCE_DATE_EPOCH=$(date +%s)'),
            'asset-dropped': in_step(main, STEP, 'qualgen-linux-amd64 \\', '\\'),
            'tarball-dropped': in_step(main, STEP, ' desk-tools-windows-arm64.tar.gz <<', ' <<'),
            'not-checksummed': in_step(main, CHECKSUMS, '            cursor-bundle.tar.gz \\\n            sbom.spdx.json > checksums.txt',
                                       '            cursor-bundle.tar.gz > checksums.txt'),
            'not-uploaded': in_step(main, COMMON, '            sbom.spdx.json\n', ''),
            'not-attested': in_step(main, ASSETS, ' sbom.spdx.json release-images.json', ' release-images.json'),
            'not-in-release-body': in_step(main, PUBLISH, 'Software bill of materials: sbom.spdx.json', 'SBOM'),
            'no-write-call': in_step(main, STEP, 'from tools.release.sbom import write', 'import json'),
        }
        for name, mutant in mutants.items():
            with self.subTest(name=name):
                self.assertTrue(mutant != main, name + ' did not apply')
                self.assertTrue(wiring_problems(mutant), name)


if __name__ == '__main__':
    unittest.main()
