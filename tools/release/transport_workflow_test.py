"""Handoff wiring: hosted native checks and in-cluster checked-object delivery."""
import fnmatch
from pathlib import Path
import re
import unittest

from tools.release import transport
from tools.release.images_test import workflow_jobs, workflow_steps, publication_problems

ROOT = Path(__file__).resolve().parents[2]


def hybrid_problems(text):
    jobs = workflow_jobs(text)
    producer, publisher, native = (jobs.get(k, '') for k in ('artifacts', 'release', 'runner-arm64'))
    problems = []
    for block, pool in [(producer, 'medici-builder-release-objects'),
                        (publisher, 'medici-builder-public-release-objects')]:
        if '    runs-on: ' + pool + '\n' not in block:
            problems.append('object handoff must use its explicitly configured release pool')
    if '    environment: release-object-store-producer\n' not in producer:
        problems.append('producer needs its protected-ref credential environment')
    steps = workflow_steps(producer)
    store = steps.get('Store checked release inputs for the in-cluster publisher', '')
    fetch = workflow_steps(publisher).get('Fetch checked release inputs from the object store', '')
    for verb, step in [('upload', store), ('download', fetch)]:
        if 'python3 -B -m tools.release.transport ' + verb not in step:
            problems.append('missing explicit checked object operation: ' + verb)
        if 'continue-on-error:' in step or '||' in step:
            problems.append('object transfer must fail closed without a fallback')
        for key in ('ENDPOINT', 'BUCKET', 'REGION', 'ACCESS_KEY_ID', 'SECRET_ACCESS_KEY'):
            value = 'RELEASE_OBJECT_' + key + ': ${{ secrets.RELEASE_OBJECT_' + key + ' }}'
            if value not in step:
                problems.append('missing step-scoped protected object input: ' + key)
        for value in ('--repository "$REPOSITORY"', '--run "$RELEASE_RUN"',
                      '--attempt "$RELEASE_ATTEMPT"', '--base "$BASE_SHA"',
                      '--source "$SOURCE_SHA"', '--tag "$RELEASE_TAG"'):
            if value not in step:
                problems.append('missing immutable handoff coordinate: ' + value)
    for field in ('bundle_sha256', 'manifest_sha256', 'bundle_bytes'):
        if field + ': ${{ steps.object-store.outputs.' + field + ' }}' not in producer:
            problems.append('producer digest/size must leave through trusted job outputs')
        if '${{ needs.artifacts.outputs.' + field + ' }}' not in fetch:
            problems.append('consumer must bind the producing job output')
    for value in ('--expected-sha256 "$BUNDLE_SHA256"',
                  '--expected-manifest-sha256 "$MANIFEST_SHA256"', '--expected-bytes "$BUNDLE_BYTES"'):
        if value not in fetch:
            problems.append('consumer must require its trusted expected proof')
    if 'pattern: release-build-*' in publisher:
        problems.append('in-cluster publisher must not silently use GitHub artifact transport')
    if 'pattern: release-build-*' not in native or 'merge-multiple: true' not in native:
        problems.append('hosted native input transport must remain GitHub artifacts')
    if 'RELEASE_OBJECT_' in native or 'tools.release.transport' in native:
        problems.append('hosted native job must not receive object-store credentials or routing')
    if text.count('RELEASE_OBJECT_ACCESS_KEY_ID: ${{ secrets.RELEASE_OBJECT_ACCESS_KEY_ID }}') != 2:
        problems.append('object credentials must appear only in the two transport steps')
    if not (publisher.find('Fetch checked release inputs from the object store') <
            publisher.find('Restore the prepared source and verify release artifacts') <
            publisher.find('Build, verify and publish the release images')):
        problems.append('checked handoff must precede existing checksum and publication gates')
    return problems


class HybridWorkflowTest(unittest.TestCase):
    # regression: #4887
    def test_hybrid_wiring_preserves_all_existing_publication_gates(self):
        for rel in ('.github/workflows/release.yml', 'ci/staged-workflows/release.yml'):
            text=(ROOT/rel).read_text()
            self.assertEqual(hybrid_problems(text), [])
            self.assertEqual(publication_problems(text), [])

    # regression: #4887
    def test_every_github_upload_member_is_also_in_the_checked_object_handoff(self):
        text=(ROOT/'.github/workflows/release.yml').read_text()
        uploads=re.findall(r'(?ms)^      - name: Upload prepared release artifacts[^\n]*\n(.*?)(?=^      - |\Z)',workflow_jobs(text)['artifacts'])
        for source in ('a'*40, 'b'*40):
            meta=transport.metadata('example/project','42','2','a'*40,source,'v1.2.3')
            expected=transport.members(meta)
            selected=[]
            for block in uploads:
                patterns=re.findall(r'^            (.+)$',block,re.M)
                selected += [name for name in expected if any(fnmatch.fnmatchcase(name,p) for p in patterns)]
            self.assertCountEqual(selected,expected)

    # regression: #4887
    def test_expected_proof_credential_and_route_controls_cannot_disappear(self):
        text=(ROOT/'.github/workflows/release.yml').read_text()
        self.assertEqual(hybrid_problems(text), [])
        changes=[('--expected-sha256 "$BUNDLE_SHA256"',''),
                 ('--expected-manifest-sha256 "$MANIFEST_SHA256"',''),
                 ('--expected-bytes "$BUNDLE_BYTES"',''),
                 ('needs.artifacts.outputs.bundle_sha256','steps.untrusted.outputs.bundle_sha256'),
                 ('environment: release-object-store-producer','environment: unprotected'),
                 ('transport download','transport upload')]
        for old,new in changes:
            mutated=text.replace(old,new,1)
            self.assertNotEqual(mutated,text)
            self.assertTrue(hybrid_problems(mutated))

    # regression: #4887
    def test_client_is_installed_only_after_the_exact_archive_digest_check(self):
        text=(ROOT/'tools/release/install-object-client.sh').read_text()
        self.assertIn('readonly version=2.37.8',text)
        self.assertIn('readonly digest=6a2f98fee0901ff623ff9abb5c80f4d67c40e5c0ac5e2ad7afa2ba756a7d5088',text)
        self.assertLess(text.index('sha256sum -c -'),text.index('unzip -q'))
        self.assertLess(text.index('unzip -q'),text.index('"$1/unpacked/aws/install"'))
        self.assertNotIn('sudo',text)
        self.assertNotIn('latest',text)
