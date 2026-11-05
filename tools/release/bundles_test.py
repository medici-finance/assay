"""Offline fixtures for the release provenance-bundle lane (#3094).

They cover tools/release/bundles.py: the release job stages one Sigstore bundle
as <asset>.sigstore.json beside every published asset, then refuses before
anything is published if an asset has no bundle, if a bundle does not attest
the asset's exact bytes, or if the bundle's shape is one deskinstall's verifier
refuses. They also pin the release.yml wiring (permissions, pin, order, dry-run
guards) with planted-defect mutants, the same way images_test.py pins the
approval boundary.
"""
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('bundles', HERE / 'bundles.py')
bundles = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bundles)

ASSETS = ['statusgen-linux-amd64', 'desk-tools-linux-amd64.tar.gz',
          'cursor-bundle.tar.gz', 'tool-validation-v1.2.3.json', 'checksums.txt']
CHECKSUMMED = ['statusgen-linux-amd64', 'desk-tools-linux-amd64.tar.gz', 'cursor-bundle.tar.gz']
ATTEST_PIN = 'actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2'


def b64(raw):
    return base64.b64encode(raw).decode()


def make_bundle(subjects, **over):
    """A bundle with the structure the public-good attestation action emits."""
    statement = {'_type': over.pop('statement_type', 'https://in-toto.io/Statement/v1'),
                 'predicateType': over.pop('predicate_type', 'https://slsa.dev/provenance/v1'),
                 'subject': [{'name': n, 'digest': {'sha256': d}} for n, d in subjects.items()],
                 'predicate': {'buildDefinition': {}, 'runDetails': {}}}
    entry = {'logIndex': '1', 'logId': {'keyId': b64(b'log')},
             'kindVersion': {'kind': 'dsse', 'version': '0.0.1'},
             'integratedTime': '1700000000',
             'inclusionPromise': {'signedEntryTimestamp': b64(b'set')},
             'canonicalizedBody': b64(b'{}')}
    entry.update(over.pop('entry', {}))
    for key in over.pop('drop_entry', []):
        entry.pop(key)
    bundle = {'mediaType': 'application/vnd.dev.sigstore.bundle.v0.3+json',
              'verificationMaterial': {'certificate': {'rawBytes': b64(b'leaf')},
                                       'tlogEntries': [entry]},
              'dsseEnvelope': {'payload': b64(json.dumps(statement).encode()),
                               'payloadType': 'application/vnd.in-toto+json',
                               'signatures': [{'sig': b64(b'sig')}]}}
    for key, value in over.items():
        if value is None:
            bundle.pop(key)
        else:
            bundle[key] = value
    return json.dumps(bundle).encode()


class Release:
    """A release directory: asset bytes, checksums.txt, the asset list."""

    def __init__(self, root):
        self.root = root
        for name in ASSETS:
            if name != 'checksums.txt':
                (root / name).write_bytes(('bytes of ' + name).encode())
        (root / 'checksums.txt').write_text(''.join(
            f'{self.digest(n)}  {n}\n' for n in CHECKSUMMED))
        (root / 'release-assets.txt').write_text(''.join(n + '\n' for n in ASSETS))

    def digest(self, name):
        return hashlib.sha256((self.root / name).read_bytes()).hexdigest()

    def digests(self):
        return {n: self.digest(n) for n in ASSETS}

    def write_bundle(self, raw):
        path = self.root / 'attestation.json'
        path.write_bytes(raw)
        return path

    def stage(self, raw):
        bundles.stage(self.root, self.write_bundle(raw), self.root / 'release-assets.txt',
                      self.root / 'checksums.txt')

    def bundle_files(self):
        return sorted(p.name for p in self.root.glob('*' + bundles.BUNDLE_EXT))


class BundleLaneTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.rel = Release(Path(temp.name))

    # regression: #3094
    def test_every_published_asset_gets_its_bundle(self):
        self.rel.stage(make_bundle(self.rel.digests()))
        self.assertEqual(self.rel.bundle_files(), sorted(n + '.sigstore.json' for n in ASSETS))
        self.assertEqual(bundles.check(self.rel.root, self.rel.root / 'release-assets.txt'), [])

    # regression: #3094
    def test_missing_bundle_fails_the_release(self):
        self.rel.stage(make_bundle(self.rel.digests()))
        (self.rel.root / ('cursor-bundle.tar.gz' + bundles.BUNDLE_EXT)).unlink()
        problems = bundles.check(self.rel.root, self.rel.root / 'release-assets.txt')
        self.assertEqual(len(problems), 1)
        self.assertIn('cursor-bundle.tar.gz has no cursor-bundle.tar.gz.sigstore.json', problems[0])

    # regression: #3094
    def test_bundle_for_other_bytes_fails_the_release(self):
        self.rel.stage(make_bundle(self.rel.digests()))
        (self.rel.root / 'statusgen-linux-amd64').write_bytes(b'swapped after attestation')
        problems = bundles.check(self.rel.root, self.rel.root / 'release-assets.txt')
        self.assertTrue(any('statusgen-linux-amd64' in p and 'no subject' in p for p in problems), problems)

    def test_stage_refusals_write_no_bundle(self):
        digests = self.rel.digests()
        missing_subject = dict(digests)
        missing_subject.pop('desk-tools-linux-amd64.tar.gz')
        cases = {
            'subject missing an asset': (make_bundle(missing_subject), 'no subject'),
            'subject with other bytes': (make_bundle({**digests, 'cursor-bundle.tar.gz': '0' * 64}), 'no subject'),
            'not a sigstore bundle': (make_bundle(digests, mediaType='application/json'), 'media type'),
            'bare public key': (make_bundle(digests, verificationMaterial={'publicKey': {'hint': 'k'}, 'tlogEntries': []}), 'certificate'),
            'no envelope': (make_bundle(digests, dsseEnvelope=None), 'DSSE envelope'),
            'two signatures': (make_bundle(digests, dsseEnvelope={'payload': b64(b'{}'), 'payloadType': 'application/vnd.in-toto+json', 'signatures': [{'sig': 'YQ=='}, {'sig': 'Yg=='}]}), 'exactly 1'),
            'rekor v2 entry': (make_bundle(digests, entry={'kindVersion': {'kind': 'hashedrekord', 'version': '0.0.2'}}), 'dsse/0.0.1'),
            'no signed entry timestamp': (make_bundle(digests, drop_entry=['inclusionPromise']), 'signed entry timestamp'),
            'wrong predicate': (make_bundle(digests, predicate_type='https://spdx.dev/Document'), 'predicateType'),
            'wrong statement type': (make_bundle(digests, statement_type='https://in-toto.io/Statement/v0.1'), '_type'),
            'not json': (b'{', 'does not parse'),
            'over the size bound': (b' ' * (bundles.MAX_BUNDLE_BYTES + 1), 'byte bound'),
        }
        for name, (raw, want) in cases.items():
            with self.subTest(name=name):
                with self.assertRaises(bundles.BundleError) as caught:
                    self.rel.stage(raw)
                self.assertIn(want, str(caught.exception))
                self.assertEqual(self.rel.bundle_files(), [])

    def test_stage_refuses_bytes_that_disagree_with_checksums(self):
        (self.rel.root / 'desk-tools-linux-amd64.tar.gz').write_bytes(b'rebuilt')
        with self.assertRaises(bundles.BundleError) as caught:
            self.rel.stage(make_bundle(self.rel.digests()))
        self.assertIn('checksums.txt', str(caught.exception))
        self.assertEqual(self.rel.bundle_files(), [])

    def test_stage_refuses_a_checksummed_file_left_unpublished(self):
        (self.rel.root / 'release-assets.txt').write_text(
            ''.join(n + '\n' for n in ASSETS if n != 'cursor-bundle.tar.gz'))
        with self.assertRaises(bundles.BundleError) as caught:
            self.rel.stage(make_bundle(self.rel.digests()))
        self.assertIn('cursor-bundle.tar.gz', str(caught.exception))
        self.assertEqual(self.rel.bundle_files(), [])

    def test_stage_never_trusts_a_bundle_already_on_disk(self):
        planted = self.rel.root / ('statusgen-linux-amd64' + bundles.BUNDLE_EXT)
        planted.write_bytes(make_bundle(self.rel.digests()))
        with self.assertRaises(bundles.BundleError) as caught:
            self.rel.stage(make_bundle(self.rel.digests()))
        self.assertIn('already exists', str(caught.exception))

    def test_asset_list_refusals(self):
        cases = {
            'empty': '',
            'duplicate': 'checksums.txt\nchecksums.txt\n',
            'path': 'dir/checksums.txt\n',
            'bundle as asset': 'checksums.txt.sigstore.json\n',
            'blank line': 'checksums.txt\n\n',
        }
        for name, text in cases.items():
            with self.subTest(name=name):
                (self.rel.root / 'release-assets.txt').write_text(text)
                with self.assertRaises(bundles.BundleError):
                    bundles.read_assets(self.rel.root / 'release-assets.txt')

    # regression: #3094
    def test_published_release_must_carry_every_bundle(self):
        self.rel.stage(make_bundle(self.rel.digests()))
        listing = [{'name': p.name, 'size': p.stat().st_size}
                   for n in ASSETS for p in [self.rel.root / n, self.rel.root / (n + bundles.BUNDLE_EXT)]]
        assets = self.rel.root / 'release-assets.txt'
        self.assertEqual(bundles.published_problems(self.rel.root, assets, listing), [])
        dropped = [a for a in listing if a['name'] != 'checksums.txt.sigstore.json']
        self.assertTrue(any('checksums.txt.sigstore.json' in p
                            for p in bundles.published_problems(self.rel.root, assets, dropped)))
        truncated = [dict(a, size=1) if a['name'] == 'cursor-bundle.tar.gz.sigstore.json' else a
                     for a in listing]
        self.assertTrue(bundles.published_problems(self.rel.root, assets, truncated))
        self.assertTrue(bundles.published_problems(self.rel.root, assets, listing + [listing[0]]))
        self.assertTrue(bundles.published_problems(self.rel.root, assets, 'not a list'))


def workflow_jobs(text):
    return dict(re.findall(r'(?ms)^  ([a-zA-Z0-9_-]+):\n(.*?)(?=^  [a-zA-Z0-9_-]+:\n|\Z)',
                           text.split('\njobs:\n', 1)[1]))


def workflow_steps(block):
    return re.findall(r'(?ms)^      - (?:name: ([^\n]+)|uses: ([^\n]+))\n(.*?)(?=^      - |\Z)', block)


DRY = "${{ !(github.event_name == 'workflow_dispatch' && inputs.dry_run) }}"


def wiring_problems(text):
    """release.yml must attest every asset and refuse a release missing a bundle."""
    problems = []
    jobs = workflow_jobs(text)
    publisher = jobs.get('release', '')
    for permission in ('id-token', 'attestations'):
        if not re.search(r'^      ' + permission + r': write$', publisher, re.M):
            problems.append(permission + ' write belongs to the release publisher')
        if any(re.search(r'^\s+' + permission + r': write$', block, re.M)
               for name, block in jobs.items() if name != 'release'):
            problems.append(permission + ' write escaped the release publisher')
    if re.search(r'^\s+(id-token|attestations): write$', text.split('\njobs:\n', 1)[0], re.M):
        problems.append('workflow defaults must not grant attestation writes')
    steps = workflow_steps(publisher)
    order = [name or uses for name, uses, _ in steps]
    body = {name or uses: block for name, uses, block in steps}
    wanted = ['List the release assets to attest',
              ATTEST_PIN,
              'Stage and check a Sigstore bundle beside every release asset',
              'Create and push the dispatched tag',
              'Create GitHub release and upload assets']
    if not all(w in order for w in wanted):
        problems.append('missing attestation step: ' + ', '.join(w for w in wanted if w not in order))
    elif [order.index(w) for w in wanted] != sorted(order.index(w) for w in wanted):
        problems.append('assets must be attested and bundled before the tag and the release')
    if re.search(r'actions/attest(-build-provenance)?@(?![0-9a-f]{40} # v)', text):
        problems.append('the attestation action must be pinned by full commit SHA')
    for name in wanted[:3] + wanted[4:]:
        if name in body and not re.search(r'^        if: ' + re.escape(DRY) + '$', body[name], re.M):
            problems.append('missing dry-run guard on ' + name)
    attest = body.get(ATTEST_PIN, '')
    if 'subject-path: ${{ steps.assets.outputs.paths }}' not in attest:
        problems.append('the attestation must cover exactly the listed release assets')
    if re.search(r'^\s+continue-on-error: true$', publisher, re.M):
        problems.append('a failed attestation must stop publication')
    required = {
        'List the release assets to attest': ['> release-assets.txt', 'paths<<'],
        'Stage and check a Sigstore bundle beside every release asset': [
            'BUNDLE: ${{ steps.provenance.outputs.bundle-path }}',
            'from tools.release.bundles import check, stage',
            "stage(Path.cwd(), Path(os.environ['BUNDLE']), Path('release-assets.txt'), Path('checksums.txt'))"],
        'Create GitHub release and upload assets': [
            'from tools.release.bundles import check',
            'for name in "$f" "$f.sigstore.json"; do',
            'done < release-assets.txt',
            'from tools.release.bundles import published_problems'],
    }
    for name, values in required.items():
        for value in values:
            if value not in body.get(name, ''):
                problems.append('missing bundle guard in ' + name + ': ' + value)
    upload = body.get('Create GitHub release and upload assets', '')
    if upload.find('published_problems') > upload.find('"draft":false') >= 0:
        problems.append('the published-bundle check must run before the draft is published')
    return problems


class WorkflowWiringTest(unittest.TestCase):
    PATHS = ['.github/workflows/release.yml', 'ci/staged-workflows/release.yml']

    # regression: #3094
    def test_release_attests_and_bundles_every_asset(self):
        root = HERE.parents[1]
        for path in self.PATHS:
            with self.subTest(path=path):
                self.assertEqual(wiring_problems((root / path).read_text()), [])

    def test_wiring_regressions_are_rejected(self):
        main = (HERE.parents[1] / '.github/workflows/release.yml').read_text()
        self.assertEqual(wiring_problems(main), [])
        stage = 'Stage and check a Sigstore bundle beside every release asset'
        mutants = {
            'no-id-token': main.replace('      id-token: write\n', '', 1),
            'no-attestations': main.replace('      attestations: write\n', '', 1),
            'id-token-elsewhere': main.replace('  changelog-roll:\n', '  changelog-roll:\n    permissions:\n      id-token: write\n', 1),
            'default-attestations': main.replace('\npermissions:\n  contents: read\n', '\npermissions:\n  contents: read\n  attestations: write\n', 1),
            'floating-pin': main.replace(ATTEST_PIN, 'actions/attest-build-provenance@v4', 1),
            'no-attest-step': main.replace('      - uses: ' + ATTEST_PIN, '      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1', 1),
            'unguarded-attest': main.replace('      - uses: ' + ATTEST_PIN + '\n        id: provenance\n        if: ' + DRY,
                                             '      - uses: ' + ATTEST_PIN + '\n        id: provenance\n        if: true', 1),
            'other-subjects': main.replace('subject-path: ${{ steps.assets.outputs.paths }}', 'subject-path: statusgen-*', 1),
            'no-stage': main.replace('from tools.release.bundles import check, stage', 'removed', 1),
            'stage-after-tag': main.replace('- name: ' + stage, '- name: TEMP', 1)
                                   .replace('- name: Create and push the dispatched tag', '- name: ' + stage, 1)
                                   .replace('- name: TEMP', '- name: Create and push the dispatched tag', 1),
            'bundle-not-uploaded': main.replace('for name in "$f" "$f.sigstore.json"; do', 'for name in "$f"; do', 1),
            'no-local-check': main.replace('          from tools.release.bundles import check\n', '', 1),
            'no-published-check': main.replace('from tools.release.bundles import published_problems', 'removed', 1),
            'ignored-failure': main.replace('- name: ' + stage + '\n', '- name: ' + stage + '\n        continue-on-error: true\n', 1),
        }
        for name, mutant in mutants.items():
            with self.subTest(name=name):
                self.assertTrue(mutant != main, name + ' did not apply')
                self.assertTrue(wiring_problems(mutant), name)


if __name__ == '__main__':
    unittest.main()
