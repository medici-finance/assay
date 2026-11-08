import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from tools.release import qualification as q

class QualificationTest(unittest.TestCase):
 def env(self):
  return dict(GITHUB_REPOSITORY='medici-finance/assay',GITHUB_EVENT_NAME='workflow_dispatch',GITHUB_REF='refs/heads/main',
      RUNNER_OS='Linux',RUNNER_ARCH='X64',QUALIFICATION_CONFIRM='ONE_PRODUCER_ONE_PUBLISHER',
      GITHUB_SHA='a'*40,QUALIFICATION_SOURCE='a'*40,GITHUB_RUN_ID='123',GITHUB_RUN_ATTEMPT='1',
      RELEASE_OBJECT_CA_BUNDLE='/etc/release-object-ca/ca.pem')
 def test_admission_and_independent_refusals(self):
  with patch.object(q.platform,'system',return_value='Linux'),patch.object(q.platform,'machine',return_value='x86_64'):
   self.assertEqual(q.admission(self.env())['source'],'a'*40)
   for key,value in [('GITHUB_EVENT_NAME','pull_request'),('GITHUB_REF','refs/tags/v1.0.0'),('GITHUB_REPOSITORY','other/repo'),('QUALIFICATION_SOURCE','b'*40),('AWS_PROFILE','default'),('RELEASE_OBJECT_CA_BUNDLE','/tmp/unreviewed'),('RUNNER_ARCH','ARM64')]:
    env=self.env();env[key]=value
    with self.subTest(key=key),self.assertRaises(ValueError):q.admission(env)
 def test_partial_rerun_uses_producer_attempt(self):
  env=self.env();env['GITHUB_RUN_ATTEMPT']='2';env['PRODUCER_ATTEMPT']='1'
  with patch.object(q.platform,'system',return_value='Linux'),patch.object(q.platform,'machine',return_value='x86_64'):
   self.assertEqual(q.received_metadata(env)['attempt'],'1')
   del env['PRODUCER_ATTEMPT']
   with self.assertRaises(ValueError):q.received_metadata(env)
 def test_denial_is_not_any_failure(self):
  class Store:aws=Path('/fake/aws');endpoint='https://fixture.invalid';env={}
  for code,stderr,ok in [(1,'network timeout',False),(0,'',False),(254,'An error occurred (AccessDenied) when calling operation',True)]:
   with patch.object(q.subprocess,'run',return_value=subprocess.CompletedProcess([],code,'',stderr)):
    if ok:self.assertTrue(q.denied(Store(),'s3api','list-buckets'))
    else:
     with self.assertRaises(ValueError):q.denied(Store(),'s3api','list-buckets')
 def test_positive_control_cannot_count_as_refusal(self):
  with self.assertRaises(ValueError):q.refusal(lambda:None)
  self.assertTrue(q.refusal(lambda:(_ for _ in ()).throw(ValueError('refused'))))
 def test_fixture_uses_original_membership_and_real_bytes(self):
  with tempfile.TemporaryDirectory() as d:
   root=Path(d)/'inputs';meta=q.t.metadata('medici-finance/assay','1','1','a'*40,'a'*40,'v0.0.0')
   q.make_fixture(root,meta)
   names=sorted(x.name for x in root.iterdir())
   self.assertEqual(names,q.t.members(meta))
   self.assertIn('statusgen-linux-arm64',names)
   self.assertIn('build-inputs.sha256',names)
   self.assertGreater((root/'statusgen-linux-amd64').stat().st_size,16<<20)
   result=q.t.prepare(root,Path(d)/'fixture.tar.gz',meta)
   self.assertGreater(result['bundle_bytes'],16<<20)
 def test_workflow_contract(self):
  raw=Path('.github/workflows/release-object-qualification.yml').read_text()
  self.assertIn('runs-on: assay-release-object-builder',raw);self.assertIn('runs-on: assay-release-object-publisher',raw)
  self.assertIn('environment: release-object-store-producer',raw);self.assertIn('environment: release\n',raw)
  self.assertEqual(raw.count("github.ref == 'refs/heads/main'"),2)
  self.assertEqual(raw.count('persist-credentials: false'),2)
  self.assertIn('needs.producer.outputs.bundle_sha256',raw)
  self.assertIn('PRODUCER_ATTEMPT: ${{ needs.producer.outputs.handoff_attempt }}',raw)
  for unsafe in ['contents: write','packages: write','pull_request:','--no-verify-ssl','gh release','git push']:
   self.assertNotIn(unsafe,raw)
 def test_corrupt_client_archive_stops_before_extraction(self):
  import sys
  with tempfile.TemporaryDirectory() as directory:
   root=Path(directory);bin=root/'bin';bin.mkdir();marker=root/'extracted'
   curl=bin/'curl';curl.write_text('#!'+sys.executable+'\nfrom pathlib import Path\nimport sys\nPath(sys.argv[-1]).write_bytes(b"corrupt archive")\n');curl.chmod(0o755)
   unzip=bin/'unzip';unzip.write_text('#!/bin/sh\ntouch "'+str(marker)+'"\nexit 1\n');unzip.chmod(0o755)
   env=dict(os.environ,RUNNER_TEMP=str(root),PATH=str(bin)+os.pathsep+os.environ['PATH'])
   result=subprocess.run(['bash','tools/release/install-object-client.sh',str(root/'release-aws')],env=env,capture_output=True)
   self.assertNotEqual(result.returncode,0);self.assertFalse(marker.exists())
 def test_success_report_is_not_release_authority(self):
  meta=q.t.metadata('medici-finance/assay','1','1','a'*40,'a'*40,'v0.0.0')
  out=q.summary('producer',meta,{})
  self.assertTrue(out['fixtureOnly']);self.assertFalse(out['releasePerformed'])

if __name__=='__main__':unittest.main()
