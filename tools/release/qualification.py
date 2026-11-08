"""Fixture-only native transport qualification, called by the protected workflow.

No release/publication operations. This module has no CLI or implicit execution.
The existing transport remains the sole bundle and credential implementation.
"""
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import tempfile
import time
import urllib.parse

from tools.release import transport as t

FIXTURE_BYTES = 20 << 20
DENIED = re.compile(r'An error occurred \((AccessDenied|Forbidden|403)\)')


def admission(env):
    required = {'GITHUB_REPOSITORY':'medici-finance/assay', 'GITHUB_EVENT_NAME':'workflow_dispatch',
                'GITHUB_REF':'refs/heads/main', 'RUNNER_OS':'Linux', 'RUNNER_ARCH':'X64',
                'QUALIFICATION_CONFIRM':'ONE_PRODUCER_ONE_PUBLISHER'}
    if any(env.get(k)!=v for k,v in required.items()):
        raise ValueError('qualification admission refused')
    if env.get('QUALIFICATION_SOURCE') != env.get('GITHUB_SHA'):
        raise ValueError('qualification source differs from dispatch')
    if any(k.startswith('AWS_') for k in env):
        raise ValueError('ambient AWS configuration is forbidden')
    if platform.system()!='Linux' or platform.machine() not in ('x86_64','AMD64'):
        raise ValueError('native Linux AMD64 runner is required')
    ca = env.get('RELEASE_OBJECT_CA_BUNDLE')
    if ca!='/etc/release-object-ca/ca.pem':
        raise ValueError('the reviewed regular CA file is required')
    return t.metadata(env['GITHUB_REPOSITORY'],env['GITHUB_RUN_ID'],env['GITHUB_RUN_ATTEMPT'],
                      env['GITHUB_SHA'],env['GITHUB_SHA'],'v0.0.0')


def refusal(fn):
    try: fn()
    except (ValueError,OSError): return True
    raise ValueError('a required transport refusal was accepted')


def denied(store,*args,authentication=False):
    result=subprocess.run([str(store.aws),*args,'--endpoint-url',store.endpoint,
                           '--cli-connect-timeout','5','--cli-read-timeout','15'],
                          env=store.env,text=True,capture_output=True,timeout=25)
    pattern = re.compile(r'An error occurred \((InvalidAccessKeyId|SignatureDoesNotMatch|AccessDenied|Forbidden|403)\)') if authentication else DENIED
    if result.returncode and pattern.search(result.stderr): return True
    # Neither command output nor private error bodies enter the public receipt.
    raise ValueError('expected an explicit S3 authorization denial')


def tls_controls(store):
    endpoint=urllib.parse.urlsplit(store.endpoint)
    # Wrong hostname uses the real endpoint address, retains CA validation and
    # differs only in SNI/Host. No skip-verification mode exists here.
    args=['curl','--silent','--show-error','--max-time','8','--cacert',store.env['AWS_CA_BUNDLE'],
          '--connect-to','wrong.invalid:'+str(endpoint.port or 443)+':'+endpoint.hostname+':'+str(endpoint.port or 443),
          'https://wrong.invalid:'+str(endpoint.port or 443)+'/', '--output',os.devnull]
    wrong=subprocess.run(args,capture_output=True,timeout=12)
    if wrong.returncode!=60: raise ValueError('wrong-SAN TLS refusal was not observed')
    with tempfile.NamedTemporaryFile() as ca:
        ca.write(b'not a trusted CA\n');ca.flush()
        bad=subprocess.run(['curl','--silent','--show-error','--max-time','8','--cacert',ca.name,
                            store.endpoint,'--output',os.devnull],capture_output=True,timeout=12)
    if bad.returncode!=77: raise ValueError('unusable trust file refusal was not observed')
    with tempfile.TemporaryDirectory(prefix='qualification-ca-') as scratch:
        cert=Path(scratch)/'untrusted-ca.pem';key=Path(scratch)/'ephemeral-key.pem'
        made=subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','1',
                '-subj','/CN=qualification-untrusted-ca','-keyout',str(key),'-out',str(cert)],
                capture_output=True,timeout=20)
        if made.returncode: raise ValueError('could not construct the local TLS refusal control')
        untrusted=subprocess.run(['curl','--silent','--show-error','--max-time','8','--cacert',str(cert),
                    store.endpoint,'--output',os.devnull],capture_output=True,timeout=12)
        if untrusted.returncode!=60: raise ValueError('untrusted CA refusal was not observed')
    return {'wrongSAN':True,'unusableCAFile':True,'untrustedCA':True}


def network_controls():
    checks={}
    for name,address in [('metadata','http://169.254.169.254/'),('serviceAPI','https://10.96.0.1:443/')]:
        out=subprocess.run(['curl','--silent','--show-error','--connect-timeout','3','--max-time','5',
                            '--noproxy','*',address,'--output',os.devnull],capture_output=True,timeout=8)
        if out.returncode not in (7,28):
            raise ValueError('a required forbidden network destination was reachable or unverifiable')
        checks[name]={'blocked':True,'curlExit':out.returncode}
    # These are observed connection refusals, not a claim of packet attribution.
    return checks


def make_fixture(root,meta):
    root.mkdir()
    for name in t.members(meta):
        (root/name).write_bytes(('qualification fixture only: '+name+'\n').encode())
    # Incompressible bytes force a >8MiB multi-part transfer with the pinned
    # AWS client's unchanged defaults. None of these bytes are release assets.
    (root/'statusgen-linux-amd64').write_bytes(os.urandom(FIXTURE_BYTES))


def summary(role,meta,checks,extra=None):
    result={'schema':'release-object-qualification-v1','role':role,'source':meta['source'],
            'repository':meta['repository'],'run':meta['run'],'attempt':meta['attempt'],
            'fixtureOnly':True,'releasePerformed':False,'checks':checks}
    result.update(extra or {})
    return result


def write_receipt(path,result):
    with Path(path).open('x') as out: json.dump(result,out,sort_keys=True,indent=2);out.write('\n')


def produce(env,aws,receipt):
    started=time.monotonic()
    meta=admission(env)
    store=t.Store(aws,env)
    temp=Path(env['RUNNER_TEMP'])
    checks={}
    with tempfile.TemporaryDirectory(prefix='object-qualification-',dir=temp) as scratch:
        scratch=Path(scratch);root=scratch/'inputs';bundle=scratch/'fixture.tar.gz'
        make_fixture(root,meta)
        result=t.prepare(root,bundle,meta)
        if result['bundle_bytes']<=16<<20: raise ValueError('multipart fixture is too small')
        key=t.object_key(meta,result['bundle_sha256'])
        store.upload(bundle,key)
        head=t._json(store.call('s3api','head-object','--bucket',store.bucket,'--key',key,'--output','json'))
        etag=head.get('ETag','').strip('"')
        if not re.fullmatch(r'[0-9a-fA-F]{32}-[2-9][0-9]*',etag):
            raise ValueError('stored fixture was not confirmed as multipart')
        checks['multipartUpload']=True
        downloaded=scratch/'roundtrip.tar.gz';store.download(key,downloaded,result['bundle_bytes'])
        t.restore(downloaded,scratch/'roundtrip',meta,result['bundle_sha256'],result['manifest_sha256'],result['bundle_bytes'])
        checks['producerRoundtrip']=True
        checks['listBucketsDenied']=denied(store,'s3api','list-buckets')
        checks['listObjectsDenied']=denied(store,'s3api','list-objects-v2','--bucket',store.bucket)
        checks['deleteDenied']=denied(store,'s3api','delete-object','--bucket',store.bucket,'--key',key)
        checks['outsidePrefixDenied']=denied(store,'s3api','put-object','--bucket',store.bucket,
                 '--key','qualification-denied/'+meta['run'],'--body',str(root/'checksums.txt'))
        checks['tls']=tls_controls(store);checks['network']=network_controls()
        missing=dict(env);missing.pop('RELEASE_OBJECT_SECRET_ACCESS_KEY',None)
        checks['missingCredentialRefused']=refusal(lambda:t.Store(aws,missing))
        invalid=dict(env);invalid['RELEASE_OBJECT_ACCESS_KEY_ID']='qualification-invalid-key'
        checks['invalidCredentialDenied']=denied(t.Store(aws,invalid),'s3api','head-object',
                  '--bucket',store.bucket,'--key',key,authentication=True)
        # Simulated store outage uses only an isolated loopback port, never an
        # alternate store or another production endpoint.
        failed=dict(env);failed['RELEASE_OBJECT_ENDPOINT']='https://127.0.0.1:1'
        checks['outageRefused']=refusal(lambda:t.Store(aws,failed).call('s3api','head-object','--bucket',store.bucket,'--key',key))
        write_receipt(receipt,summary('producer',meta,checks,{'bundleBytes':result['bundle_bytes'],'qualificationElapsedSeconds':time.monotonic()-started}))
        with open(env['GITHUB_OUTPUT'],'a') as out:
            for k,v in result.items():out.write(k+'='+str(v)+'\n')


def received_metadata(env):
    meta=admission(env)
    # Partial publisher reruns retain the producer's original trusted attempt.
    if not re.fullmatch(r'[1-9][0-9]*',env.get('PRODUCER_ATTEMPT','')):
        raise ValueError('trusted producer attempt is required')
    meta['attempt']=env['PRODUCER_ATTEMPT']
    return meta


def receive(env,aws,receipt):
    started=time.monotonic()
    meta=received_metadata(env)
    store=t.Store(aws,env)
    digest=env.get('EXPECTED_BUNDLE_SHA256','');manifest=env.get('EXPECTED_MANIFEST_SHA256','')
    size=int(env.get('EXPECTED_BUNDLE_BYTES','0'))
    key=t.object_key(meta,digest)
    checks={}
    with tempfile.TemporaryDirectory(prefix='object-qualification-',dir=env['RUNNER_TEMP']) as scratch:
        scratch=Path(scratch);bundle=scratch/'received.tar.gz'
        store.download(key,bundle,size)
        t.restore(bundle,scratch/'restored',meta,digest,manifest,size)
        checks['trustedOutputHandoff']=True
        checks['wrongHashRefused']=refusal(lambda:t.restore(bundle,scratch/'wrong-hash',meta,'0'*64,manifest,size))
        wrong=dict(meta);wrong['run']=str(int(meta['run'])+1)
        checks['wrongRunRefused']=refusal(lambda:t.restore(bundle,scratch/'wrong-run',wrong,digest,manifest,size))
        checks['wrongManifestRefused']=refusal(lambda:t.restore(bundle,scratch/'wrong-manifest',meta,digest,'0'*64,size))
        checks['putDenied']=denied(store,'s3api','put-object','--bucket',store.bucket,
                                   '--key',key+'.publisher-denied','--body',str(bundle))
        checks['deleteDenied']=denied(store,'s3api','delete-object','--bucket',store.bucket,'--key',key)
        checks['listObjectsDenied']=denied(store,'s3api','list-objects-v2','--bucket',store.bucket)
        checks['outsidePrefixDenied']=denied(store,'s3api','get-object','--bucket',store.bucket,
                 '--key','qualification-denied/'+meta['run'],str(scratch/'forbidden'))
        checks['tls']=tls_controls(store);checks['network']=network_controls()
        write_receipt(receipt,summary('publisher',meta,checks,{'bundleBytes':size,'executionAttempt':env['GITHUB_RUN_ATTEMPT'],'qualificationElapsedSeconds':time.monotonic()-started}))
