"""Offline release-cache trust and extraction controls (regression: #138)."""
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock
import subprocess
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import zipfile

spec = importlib.util.spec_from_file_location('go_cache', Path(__file__).with_name('go_cache.py'))
cache = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cache)


class CacheTests(unittest.TestCase):
    def setUp(self):
        self.run = dict(id=42, run_attempt=1, workflow_id=7, path=cache.WORKFLOW,
                        event='workflow_dispatch', head_branch='main', head_sha='a'*40,
                        status='completed', conclusion='success',
                        repository={'full_name': 'example/project'},
                        head_repository={'full_name': 'example/project'})
        self.job = dict(id=81, run_id=42, run_attempt=1, head_sha='a'*40,
                        name='release', status='completed', conclusion='success',
                        steps=[dict(name=n, status='completed', conclusion='success')
                               for n in cache.PRODUCER_STEPS])
        self.artifact = dict(id=91, name=cache.ARTIFACT, expired=False,
                             size_in_bytes=100, digest='sha256:'+'b'*64,
                             workflow_run=dict(id=42, head_sha='a'*40))
        self.log = '2026-10-13T00:00:00Z ASSAY_RELEASE_CACHE_V1 42 1 91 '+'b'*64+'\n'
        self.key = {'schema': 1, 'go': 'go1.25.0', 'host': 'linux-amd64'}

    def admit(self):
        return cache.select_artifact(self.run, [self.job], [self.artifact], self.log,
                                     'example/project', 7, 99)

    def test_producer(self):
        self.assertEqual(self.admit()['id'], 91)
        mutations = [('run', 'id', 99), ('run', 'event', 'pull_request'),
                     ('run', 'workflow_id', 8), ('run', 'path', 'trusted-build.yml'),
                     ('run', 'head_branch', 'feature'), ('run', 'conclusion', 'failure'),
                     ('run', 'head_repository', {'full_name': 'other/project'}),
                     ('job', 'run_id', 41), ('job', 'run_attempt', 2),
                     ('job', 'name', 'test (mutation)'), ('job', 'conclusion', 'failure'),
                     ('job', 'head_sha', 'c'*40), ('job', 'steps', []),
                     ('artifact', 'id', 92), ('artifact', 'expired', True),
                     ('artifact', 'digest', 'sha256:'+'c'*64),
                     ('artifact', 'workflow_run', {'id': 41, 'head_sha': 'a'*40})]
        for obj, field, value in mutations:
            with self.subTest(obj=obj, field=field):
                target=getattr(self,obj); old=copy.deepcopy(target[field]); target[field]=value
                with self.assertRaises(cache.Miss): self.admit()
                target[field]=old
        self.log += self.log
        with self.assertRaises(cache.Miss): self.admit()

    def test_absent_records(self):
        for jobs, artifacts, log in [([], [self.artifact], self.log),
                                      ([self.job], [], self.log),
                                      ([self.job], [self.artifact], ''),
                                      ([self.job, self.job], [self.artifact], self.log)]:
            with self.assertRaises(cache.Miss):
                cache.select_artifact(self.run, jobs, artifacts, log, 'example/project', 7, 99)

    def archive(self, path, members=None, key=None):
        data=io.BytesIO()
        with tarfile.open(fileobj=data, mode='w') as t:
            entries=[('manifest.json',json.dumps(key or self.key).encode(),tarfile.REGTYPE)]
            entries += members or [('build/ab/entry', b'compiled', tarfile.REGTYPE),
                                   ('mod/cache/download/example/@v/v1.zip', b'module', tarfile.REGTYPE)]
            for name, content, kind in entries:
                m=tarfile.TarInfo(name);m.type=kind;m.size=len(content) if kind==tarfile.REGTYPE else 0
                m.linkname='../../outside' if kind in (tarfile.SYMTYPE,tarfile.LNKTYPE) else ''
                t.addfile(m, io.BytesIO(content) if m.size else None)
        with zipfile.ZipFile(path,'w') as z: z.writestr('go-cache.tar',data.getvalue())
        return hashlib.sha256(path.read_bytes()).hexdigest()

    def test_restore_bytes(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d); arc=root/'cache.zip'; digest=self.archive(arc)
            cache.unpack(arc, root/'restored', digest, self.key)
            self.assertEqual((root/'restored/build/ab/entry').read_bytes(),b'compiled')
            self.assertEqual((root/'restored/mod/cache/download/example/@v/v1.zip').read_bytes(),b'module')

    def test_bad_archives(self):
        controls=[('parent',[('../escape',b'x',tarfile.REGTYPE)],None),
                  ('absolute',[('/escape',b'x',tarfile.REGTYPE)],None),
                  ('nested-parent',[('build/../../escape',b'x',tarfile.REGTYPE)],None),
                  ('symlink',[('build/link',b'',tarfile.SYMTYPE)],None),
                  ('hardlink',[('build/link',b'',tarfile.LNKTYPE)],None),
                  ('foreign',[('home/credentials',b'x',tarfile.REGTYPE)],None),
                  ('duplicate',[('build/x',b'x',tarfile.REGTYPE),('build/x',b'y',tarfile.REGTYPE)],None),
                  ('wrong-toolchain',None,dict(self.key,go='go1.24.0'))]
        for name,members,key in controls:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as d:
                root=Path(d);arc=root/'cache.zip';digest=self.archive(arc,members,key)
                with self.assertRaises(cache.Miss): cache.unpack(arc,root/'restored',digest,self.key)
                self.assertFalse((root/'restored').exists())
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);arc=root/'cache.zip';self.archive(arc)
            with self.assertRaises(cache.Miss): cache.unpack(arc,root/'restored','0'*64,self.key)
            arc.write_bytes(b'corrupt')
            with self.assertRaises(cache.Miss): cache.unpack(arc,root/'restored','0'*64,self.key)

    def test_pack_round_trip(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d); (root/'owned/build/ab').mkdir(parents=True);(root/'owned/mod').mkdir()
            (root/'owned/build/ab/a').write_bytes(b'cache object')
            cache.pack(root/'owned',root/'go-cache.tar',self.key)
            with zipfile.ZipFile(root/'cache.zip','w') as z: z.write(root/'go-cache.tar','go-cache.tar')
            digest=hashlib.sha256((root/'cache.zip').read_bytes()).hexdigest()
            cache.unpack(root/'cache.zip',root/'restored',digest,self.key)
            self.assertEqual((root/'restored/build/ab/a').read_bytes(),b'cache object')

    def test_source_contract(self):
        with mock.patch.object(cache.subprocess, 'run') as run, mock.patch.object(cache.subprocess, 'check_output') as show, mock.patch.object(cache.Path, 'read_bytes', return_value=b'approved'):
            run.return_value.returncode=0; show.return_value=b'approved'
            cache.same_producer('a'*40)
            run.return_value.returncode=1
            with self.assertRaises(cache.Miss): cache.same_producer('a'*40)
            run.return_value.returncode=0; show.return_value=b'different producer'
            with self.assertRaises(cache.Miss): cache.same_producer('a'*40)
            with self.assertRaises(cache.Miss): cache.same_producer('main')

    def test_http_restore(self):
        # Real loopback HTTP, fake token: exercises metadata/log/download flow.
        with tempfile.TemporaryDirectory() as d:
            root=Path(d); digest=self.archive(root/'cache.zip')
            self.artifact['digest']='sha256:'+digest
            self.log='2026-10-13T00:00:00Z ASSAY_RELEASE_CACHE_V1 42 1 91 '+digest+'\n'
            responses={
                '/actions/workflows/release.yml':dict(path=cache.WORKFLOW,state='active',id=7),
                '/actions/workflows/release.yml/runs?branch=main&event=workflow_dispatch&status=success&per_page=5':dict(workflow_runs=[self.run]),
                '/actions/runs/42/attempts/1/jobs?per_page=100':dict(total_count=1,jobs=[self.job]),
                '/actions/jobs/81/logs':self.log.encode(),
                '/actions/runs/42/artifacts?per_page=100':dict(total_count=1,artifacts=[self.artifact]),
                '/actions/artifacts/91/zip':(root/'cache.zip').read_bytes()}
            requested=[]
            class Handler(BaseHTTPRequestHandler):
                def do_GET(self):
                    requested.append(self.path)
                    value=responses.get(self.path)
                    if value is None: self.send_response(503);self.end_headers();return
                    self.send_response(200);self.end_headers()
                    self.wfile.write(value if isinstance(value,bytes) else json.dumps(value).encode())
                def log_message(self,*args): pass
            server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
            thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
            actual=cache.API
            def api(repo,token):
                obj=actual(repo,token);obj.base='http://127.0.0.1:'+str(server.server_port);return obj
            env=dict(GITHUB_REPOSITORY='example/project',GH_TOKEN='fixture',GITHUB_RUN_ID='99')
            try:
                with mock.patch.object(cache,'API',side_effect=api), mock.patch.object(cache,'same_producer'):
                    cache.restore(root/'ok',self.key,env)
                    self.assertEqual((root/'ok/build/ab/entry').read_bytes(),b'compiled')
                    self.assertIn('/actions/jobs/81/logs',requested)
                    requested.clear();self.artifact['id']=92
                    with self.assertRaises(cache.Miss): cache.restore(root/'replaced',self.key,env)
                    self.assertNotIn('/actions/artifacts/91/zip',requested)
                    self.assertFalse((root/'replaced').exists())
                    responses['/actions/workflows/release.yml']=None
                    with self.assertRaises(cache.Miss): cache.restore(root/'unavailable',self.key,env)
            finally:
                server.shutdown();server.server_close();thread.join()

    def test_cold_fallback(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);envfile=root/'env'
            with mock.patch.dict(os.environ, {'RUNNER_TEMP':d,'GITHUB_ENV':str(envfile)}), mock.patch.object(cache,'restore',side_effect=cache.Miss('unavailable')), mock.patch.object(cache.subprocess,'check_output',return_value='fixture'):
                cache.main('restore')
            lines=dict(line.split('=',1) for line in envfile.read_text().splitlines())
            self.assertEqual(set(lines),{'GOCACHE','GOMODCACHE'})
            self.assertTrue(Path(lines['GOCACHE']).is_dir())
            self.assertTrue(Path(lines['GOMODCACHE']).is_dir())
            self.assertEqual(list(Path(lines['GOCACHE']).iterdir()),[])

    def test_workflow_wiring(self):
        repo=Path(__file__).resolve().parents[2]
        for path in ['.github/workflows/release.yml','tools/release/release.yml.proposed','ci/staged-workflows/release.yml']:
            text=(repo/path).read_text()
            release=text.split('\n  release:\n',1)[1].split('\n  changelog-roll:',1)[0]
            self.assertIn('environment: release',release)
            self.assertIn('actions: read',release)
            self.assertNotIn('actions: write',release)
            self.assertNotRegex(text, r'uses:\s*actions/cache(?:/[^@\s]+)?@')
            self.assertNotIn('type=gha',text)
            self.assertIn('go_cache.main("restore")',release)
            self.assertIn('go_cache.main("pack")',release)
            self.assertIn('overwrite: false',release)
            self.assertIn('compression-level: 1',release)
            self.assertIn("github.ref == 'refs/heads/main' && !inputs.dry_run",release)
            for name in cache.PRODUCER_STEPS: self.assertIn('name: '+name,release)
            self.assertIn('-o "$stage/" ./cmd/...',release)
            self.assertIn('darwin-arm64 darwin-amd64 linux-amd64 windows-amd64 windows-arm64',release)
            self.assertNotIn('for cmd in cmd/*/',release)
