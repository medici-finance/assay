"""Offline handoff tests: real bytes/archives and a fake pinned-client process."""
import copy
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

from tools.release import transport as t


class TransportTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.inputs = self.root / 'inputs'
        self.inputs.mkdir()
        self.meta = t.metadata('example/project', '42', '2', 'a'*40, 'b'*40, 'v1.2.3')
        for name in t.members(self.meta):
            (self.inputs / name).write_bytes((name + '\n').encode()*3)
        self.bundle = self.root / 'handoff.tar.gz'
        self.proof = t.prepare(self.inputs, self.bundle, self.meta)

    def restore(self, **overrides):
        args = dict(bundle=self.bundle, destination=self.root / 'out', meta=self.meta,
                    expected_digest=self.proof['bundle_sha256'],
                    expected_manifest=self.proof['manifest_sha256'], expected_size=self.proof['bundle_bytes'])
        args.update(overrides)
        return t.restore(**args)

    def repack(self, mutate_manifest=None, mutate_entries=None):
        with tarfile.open(self.bundle, 'r:gz') as src:
            entries = [(copy.copy(m), src.extractfile(m).read()) for m in src.getmembers()]
        if mutate_manifest:
            meta = json.loads(entries[0][1]); mutate_manifest(meta)
            data = json.dumps(meta, sort_keys=True, separators=(',', ':')).encode()+b'\n'
            entries[0][0].size = len(data); entries[0] = (entries[0][0], data)
            self.proof['manifest_sha256'] = hashlib.sha256(data).hexdigest()
        if mutate_entries:
            mutate_entries(entries)
        changed = self.root / 'changed.tar.gz'
        with tarfile.open(changed, 'w:gz', format=tarfile.USTAR_FORMAT) as out:
            for member, data in entries:
                out.addfile(member, io.BytesIO(data) if member.isfile() else None)
        self.bundle = changed
        self.proof['bundle_sha256'] = t.sha256_file(changed)
        self.proof['bundle_bytes'] = changed.stat().st_size

    # regression: #4887
    def test_roundtrip_is_exact_and_bound_to_job_outputs(self):
        self.restore()
        self.assertEqual(sorted(p.name for p in (self.root/'out').iterdir()), t.members(self.meta))
        for name in t.members(self.meta):
            self.assertEqual((self.root/'out'/name).read_bytes(), (self.inputs/name).read_bytes())
        other = self.root/'same.tar.gz'
        self.assertEqual(t.prepare(self.inputs, other, self.meta), self.proof)
        self.assertEqual(other.read_bytes(), self.bundle.read_bytes())

    # regression: #4887
    def test_expected_hashes_size_and_metadata_are_required(self):
        cases = [dict(expected_digest='c'*64), dict(expected_manifest='d'*64),
                 dict(expected_size=self.proof['bundle_bytes']+1), dict(expected_size=t.MAX_BUNDLE+1),
                 dict(meta=dict(self.meta, run='43')), dict(meta=dict(self.meta, attempt='3')),
                 dict(meta=dict(self.meta, source='c'*40)), dict(meta=dict(self.meta, tag='v1.2.4'))]
        for args in cases:
            with self.subTest(args=args), self.assertRaises(ValueError):
                self.restore(**args)
        self.assertFalse((self.root/'out'/'checksums.txt').exists())

    # regression: #4887
    def test_incomplete_duplicate_and_malformed_manifest_is_refused(self):
        mutations = [lambda m:m['members'].pop(), lambda m:m['members'].append(m['members'][0]),
                     lambda m:m['members'].append(None), lambda m:m['members'][0].update(size=True),
                     lambda m:m['members'][0].update(sha256='e'*64),
                     lambda m:m.update(run='99'), lambda m:m.update(extra='untrusted')]
        original, proof = self.bundle.read_bytes(), dict(self.proof)
        for mutate in mutations:
            with self.subTest(mutate=mutate):
                self.bundle=self.root/'handoff.tar.gz';self.bundle.write_bytes(original);self.proof=dict(proof)
                self.repack(mutate_manifest=mutate)
                with self.assertRaises(ValueError):self.restore()
                self.assertFalse((self.root/'out'/'checksums.txt').exists())

    # regression: #4887
    def test_links_paths_extra_members_and_overwrites_are_refused(self):
        def symlink(entries): entries[1][0].type=tarfile.SYMTYPE;entries[1][0].linkname='../escape'
        def traversal(entries): entries[1][0].name='../escape'
        def extra(entries): entries.append(copy.deepcopy(entries[-1]))
        original, proof = self.bundle.read_bytes(), dict(self.proof)
        for mutate in (symlink, traversal, extra):
            self.bundle=self.root/'handoff.tar.gz';self.bundle.write_bytes(original);self.proof=dict(proof)
            self.repack(mutate_entries=mutate)
            with self.assertRaises(ValueError):self.restore()
            self.assertFalse((self.root/'escape').exists())
        self.bundle=self.root/'handoff.tar.gz';self.bundle.write_bytes(original);self.proof=dict(proof)
        out=self.root/'out';out.mkdir(exist_ok=True);(out/'checksums.txt').write_bytes(b'existing')
        with self.assertRaises(ValueError):self.restore()
        self.assertEqual((out/'checksums.txt').read_bytes(),b'existing')

    # regression: #4887
    def test_prepare_rejects_missing_links_and_unexpected_release_files(self):
        (self.inputs/'release-source.bundle').unlink()
        with self.assertRaises(ValueError):t.prepare(self.inputs,self.root/'missing.gz',self.meta)
        (self.inputs/'release-source.bundle').symlink_to(self.inputs/'checksums.txt')
        with self.assertRaises(ValueError):t.prepare(self.inputs,self.root/'link.gz',self.meta)
        (self.inputs/'release-source.bundle').unlink();(self.inputs/'release-source.bundle').write_bytes(b'bundle')
        (self.inputs/'qualgen-unknown').write_bytes(b'unknown')
        with self.assertRaises(ValueError):t.prepare(self.inputs,self.root/'extra.gz',self.meta)
        self.assertNotIn('release-source.bundle', t.members(dict(self.meta,source=self.meta['base'])))

    # regression: #4887
    def test_store_requires_explicit_credentials_and_discards_ambient_identity(self):
        env=dict(PATH=os.environ['PATH'],RELEASE_OBJECT_ENDPOINT='https://store.example',
                 RELEASE_OBJECT_BUCKET='release-fixture',RELEASE_OBJECT_REGION='us-east-1',
                 RELEASE_OBJECT_ACCESS_KEY_ID='fixture-access',RELEASE_OBJECT_SECRET_ACCESS_KEY='fixture-secret',
                 AWS_PROFILE='ambient',AWS_ACCESS_KEY_ID='ambient',AWS_SECRET_ACCESS_KEY='ambient',
                 AWS_WEB_IDENTITY_TOKEN_FILE='/ambient',AWS_ENDPOINT_URL='https://ambient.example')
        aws=self.root/'aws';aws.write_text('#!/bin/sh\nexit 0\n');aws.chmod(0o755)
        store=t.Store(str(aws),env)
        self.assertNotIn('AWS_PROFILE',store.env);self.assertNotIn('AWS_WEB_IDENTITY_TOKEN_FILE',store.env)
        self.assertNotIn('AWS_ENDPOINT_URL',store.env)
        self.assertEqual(store.env['AWS_ACCESS_KEY_ID'],'fixture-access')
        self.assertEqual(store.env['AWS_CONFIG_FILE'],'/dev/null')
        self.assertEqual(store.env['AWS_SHARED_CREDENTIALS_FILE'],'/dev/null')
        self.assertEqual(store.env['AWS_EC2_METADATA_DISABLED'],'true')
        for field,value in [('RELEASE_OBJECT_ACCESS_KEY_ID',''),('RELEASE_OBJECT_SECRET_ACCESS_KEY',''),
                            ('RELEASE_OBJECT_ENDPOINT','http://store.example'),
                            ('RELEASE_OBJECT_ENDPOINT','https://user:password@store.example'),('RELEASE_OBJECT_BUCKET','')]:
            with self.subTest(field=field,value=value),self.assertRaises(ValueError):t.Store(str(aws),dict(env,**{field:value}))
        with self.assertRaises(ValueError):t.Store('aws',env)
        ca=self.root/'ca.pem';ca.write_text('fixture certificate file')
        self.assertEqual(t.Store(str(aws),dict(env,RELEASE_OBJECT_CA_BUNDLE=str(ca))).env['AWS_CA_BUNDLE'],str(ca))
        relative=os.path.relpath(ca,Path.cwd())
        inline='-----BEGIN CERTIFICATE-----\nfixture\n-----END CERTIFICATE-----'
        for invalid in (relative,inline,'/not-present/ca.pem'):
            with self.subTest(ca=invalid),self.assertRaisesRegex(ValueError,'absolute regular-file path'):
                t.Store(str(aws),dict(env,RELEASE_OBJECT_CA_BUNDLE=invalid))

    # regression: #4887
    def test_store_client_is_bounded_and_never_falls_back(self):
        env=dict(RELEASE_OBJECT_ENDPOINT='https://store.example',RELEASE_OBJECT_BUCKET='release-fixture',
                 RELEASE_OBJECT_REGION='us-east-1',RELEASE_OBJECT_ACCESS_KEY_ID='fixture-access',
                 RELEASE_OBJECT_SECRET_ACCESS_KEY='fixture-secret')
        aws=self.root/'aws';aws.write_bytes(b'fixture')
        store=t.Store(str(aws),env)
        with mock.patch.object(t.subprocess,'run',return_value=subprocess.CompletedProcess([],9,'','private endpoint detail')) as call:
            with self.assertRaisesRegex(ValueError,'AWS client exit 9'):store.call('s3','cp','one','two')
            self.assertEqual(call.call_count,1)
            args=call.call_args[0][0]
            self.assertNotIn('--no-verify-ssl',args)
            self.assertIn('--cli-read-timeout',args)
        with mock.patch.object(store,'call',return_value='{"ContentLength":1}') as call:
            with self.assertRaises(ValueError):store.download('key',self.root/'out.gz',99)
            self.assertEqual(call.call_count,1)
        with mock.patch.object(store,'call',side_effect=['{"ContentLength":99}','{}']) as call:
            store.download('key',self.root/'out.gz',99)
            self.assertIn('bytes=0-98',call.call_args[0])


if __name__ == '__main__':unittest.main()
