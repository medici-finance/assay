"""Optional Go cache transport for the gated release job; never Actions Cache.

Only a completed main dispatch with this producer revision may seed a build.
A receipt in the *release job's* log binds the immutable artifact ID and digest;
a same-named artifact from a mutation job or another workflow is not a producer.
Every failure to obtain or validate a prior artifact (forge, transport,
decoding, archive) is a cold miss: restore still exports empty job-local
caches. A pre-existing cache directory on the runner is refused rather than
adopted unvalidated. GitHub Actions read permission is required.
"""
import hashlib
import contextlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile

WORKFLOW = '.github/workflows/release.yml'
ARTIFACT = 'release-go-cache-v1'
PRODUCER_STEPS = ('Save release Go cache', 'Record release cache receipt')
MAX_ARCHIVE = 2 * 1024**3
MAX_EXPANDED = 8 * 1024**3
MAX_ENTRIES = 200000


class Miss(Exception):
    """Unusable optional input, never an authorization to relax validation."""


def require(ok, reason):
    if not ok:
        raise Miss(reason)


def select_artifact(run, jobs, artifacts, log, repo, workflow_id, current_run):
    require(run['id'] != current_run and run['workflow_id'] == workflow_id and
            run['path'] == WORKFLOW and run['event'] == 'workflow_dispatch' and
            run['head_branch'] == 'main' and run['repository']['full_name'] == repo and
            run['head_repository']['full_name'] == repo and
            run['status'] == 'completed' and run['conclusion'] == 'success', 'producer run')
    producers = [j for j in jobs if j['name'] == 'release']
    require(len(producers) == 1, 'producer job count')
    job = producers[0]
    require(job['run_id'] == run['id'] and job['run_attempt'] == run['run_attempt'] and
            job['head_sha'] == run['head_sha'] and job['status'] == 'completed' and
            job['conclusion'] == 'success', 'producer job')
    for name in PRODUCER_STEPS:
        steps = [s for s in job['steps'] if s['name'] == name]
        require(len(steps) == 1 and steps[0]['status'] == 'completed' and
                steps[0]['conclusion'] == 'success', 'producer step')
    # Actual log output is timestamp-prefixed. Echoed shell source contains
    # variables, not decimal IDs, and cannot match this anchored receipt.
    receipts = re.findall(r'(?m)^\S+ ASSAY_RELEASE_CACHE_V1 ([1-9][0-9]*) '
                          r'([1-9][0-9]*) ([1-9][0-9]*) ([0-9a-f]{64})\r?$', log)
    require(len(receipts) == 1, 'producer receipt count')
    run_id, attempt, artifact_id, digest = receipts[0]
    require(int(run_id) == run['id'] and int(attempt) == run['run_attempt'], 'receipt run')
    matches = [a for a in artifacts if a['id'] == int(artifact_id)]
    require(len(matches) == 1, 'receipt artifact')
    artifact = matches[0]
    require(artifact['name'] == ARTIFACT and artifact['expired'] is False and
            0 < artifact['size_in_bytes'] <= MAX_ARCHIVE and
            artifact['digest'] == 'sha256:' + digest and
            artifact['workflow_run']['id'] == run['id'] and
            artifact['workflow_run']['head_sha'] == run['head_sha'], 'artifact metadata')
    return artifact


def sha256(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def unpack(archive, destination, digest, key):
    require(not destination.exists(), 'destination exists')
    require(archive.stat().st_size <= MAX_ARCHIVE and sha256(archive) == digest,
            'archive digest or size')
    # Unpack transactionally: rejected input never becomes GOCACHE/GOMODCACHE.
    with tempfile.TemporaryDirectory(prefix='go-cache-read-', dir=destination.parent) as tmp:
        tmp = Path(tmp)
        try:
            with zipfile.ZipFile(archive) as z:
                require(z.namelist() == ['go-cache.tar'], 'zip inventory')
                require(z.getinfo('go-cache.tar').file_size <= MAX_EXPANDED, 'zip size')
                with z.open('go-cache.tar') as source, (tmp/'cache.tar').open('wb') as out:
                    shutil.copyfileobj(source, out)
            with tarfile.open(tmp/'cache.tar', 'r:') as t:
                members = t.getmembers()
                require(len(members) <= MAX_ENTRIES, 'tar inventory size')
                seen = set(); total = 0
                for member in members:
                    name = member.name
                    parts = name.rstrip('/').split('/')
                    require(name not in seen and not name.startswith('/') and
                            all(p not in ('', '.', '..') for p in parts) and
                            '\\' not in name and len(name) < 4096 and
                            (member.isfile() or member.isdir()), 'tar member')
                    require(parts[0] in ('build', 'mod') or name == 'manifest.json', 'tar root')
                    require(name != 'manifest.json' or (member.isfile() and member.size < 16384),
                            'manifest member')
                    seen.add(name); total += member.size
                    require(total <= MAX_EXPANDED, 'expanded size')
                require('manifest.json' in seen, 'missing manifest')
                require(json.load(t.extractfile('manifest.json')) == key, 'cache inputs')
                stage = tmp/'restored'; stage.mkdir()
                for member in members:
                    if member.name == 'manifest.json':
                        continue
                    target = stage.joinpath(*PurePosixPath(member.name).parts)
                    if member.isdir():
                        target.mkdir(parents=True, exist_ok=True)
                    else:
                        target.parent.mkdir(parents=True, exist_ok=True)
                        with t.extractfile(member) as source, target.open('xb') as out:
                            shutil.copyfileobj(source, out)
                        # open('xb') ignores the member mode; Go execs cached
                        # `go run` binaries directly, so restore the exec bit.
                        if member.mode & 0o100:
                            target.chmod(0o755)
                (stage/'build').mkdir(exist_ok=True); (stage/'mod').mkdir(exist_ok=True)
                stage.rename(destination)
        except (zipfile.BadZipFile, tarfile.TarError, OSError, ValueError) as e:
            raise Miss('invalid archive') from e


def pack(root, output, key):
    total = 0; entries = 0
    with tarfile.open(output, 'w') as t:
        manifest = json.dumps(key, sort_keys=True).encode()
        info = tarfile.TarInfo('manifest.json'); info.size = len(manifest)
        t.addfile(info, io.BytesIO(manifest))
        for kind in ('build', 'mod'):
            base = root/kind
            require(base.is_dir() and not base.is_symlink(), 'cache root')
            for path in sorted(base.rglob('*')):
                require(not path.is_symlink() and (path.is_file() or path.is_dir()), 'cache member')
                entries += 1; total += path.stat().st_size if path.is_file() else 0
                require(entries <= MAX_ENTRIES and total <= MAX_EXPANDED, 'cache size')
                t.add(path, arcname=path.relative_to(root).as_posix(), recursive=False)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class API:
    def __init__(self, repo, token):
        require(re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo), 'repository')
        self.base = 'https://api.github.com/repos/' + repo
        self.token = token
        self.deadline = time.monotonic() + 60
        self.opener = urllib.request.build_opener(NoRedirect())

    def get(self, path, limit=4*1024**2, output=None):
        require(path.startswith('/') and '?' not in path.split('/')[0], 'API path')
        timeout = min(30, self.deadline - time.monotonic())
        require(timeout > 0, 'cache read deadline')
        req = urllib.request.Request(self.base + path, headers={
            'Authorization': 'Bearer ' + self.token, 'Accept': 'application/vnd.github+json',
            'X-GitHub-Api-Version': '2022-11-28'})
        try:
            response = self.opener.open(req, timeout=timeout)
        except urllib.error.HTTPError as e:
            if e.code != 302:
                e.close()
                raise Miss('API unavailable') from e
            location = e.headers['Location']
            e.close()
            url = urllib.parse.urlparse(location)
            require(url.scheme == 'https' and url.hostname and
                    (url.hostname.endswith('.blob.core.windows.net') or
                     url.hostname.endswith('.githubusercontent.com')), 'download redirect')
            # The API credential NEVER rides the signed storage redirect.
            response = self.opener.open(urllib.request.Request(url.geturl()), timeout=timeout)
        with response:
            data = bytearray() if output is None else None
            size = 0
            with output.open('xb') if output is not None else contextlib.nullcontext() as f:
                while True:
                    require(time.monotonic() < self.deadline, 'cache read deadline')
                    chunk = response.read(1024*1024)
                    if not chunk:
                        break
                    size += len(chunk); require(size <= limit, 'download size')
                    if f is not None: f.write(chunk)
                    else: data.extend(chunk)
            return bytes(data) if data is not None else None

    def json(self, path):
        return json.loads(self.get(path))


def same_producer(source):
    require(re.fullmatch('[0-9a-f]{40}', source), 'source SHA')
    require(subprocess.run(['git','merge-base','--is-ancestor',source,'HEAD'],
                           stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10).returncode == 0,
            'producer is not an ancestor')
    for path in (WORKFLOW, 'tools/release/go_cache.py'):
        data = subprocess.check_output(['git','show',source+':'+path],stderr=subprocess.DEVNULL,timeout=10)
        require(data == Path(path).read_bytes(), 'producer contract changed')


def restore(root, key, env):
    api = API(env['GITHUB_REPOSITORY'],env['GH_TOKEN'])
    print('release Go cache: reading GitHub artifacts for ' + env['GITHUB_REPOSITORY'])
    wf = api.json('/actions/workflows/release.yml')
    require(wf['path'] == WORKFLOW and wf['state'] == 'active', 'workflow identity')
    runs = api.json('/actions/workflows/release.yml/runs?branch=main&event=workflow_dispatch&status=success&per_page=5')['workflow_runs']
    for run in runs:
        try:
            # Source and forge metadata checks precede any archive download.
            same_producer(run['head_sha'])
            jobs = api.json('/actions/runs/%s/attempts/%s/jobs?per_page=100' %
                            (run['id'], run['run_attempt']))
            require(jobs['total_count'] == len(jobs['jobs']), 'jobs truncated')
            producers = [j for j in jobs['jobs'] if j['name'] == 'release']
            require(len(producers) == 1, 'producer job count')
            log = api.get('/actions/jobs/%s/logs' % producers[0]['id'],limit=16*1024**2).decode('utf-8')
            artifacts = api.json('/actions/runs/%s/artifacts?per_page=100' % run['id'])
            require(artifacts['total_count'] == len(artifacts['artifacts']), 'artifacts truncated')
            artifact = select_artifact(run,jobs['jobs'],artifacts['artifacts'],log,
                                       env['GITHUB_REPOSITORY'],wf['id'],int(env['GITHUB_RUN_ID']))
            with tempfile.TemporaryDirectory(prefix='go-cache-download-',dir=root.parent) as tmp:
                archive = Path(tmp)/'artifact.zip'
                api.get('/actions/artifacts/%s/zip' % artifact['id'],MAX_ARCHIVE,archive)
                unpack(archive,root,artifact['digest'].removeprefix('sha256:'),key)
            print('release Go cache: restored run=%s artifact=%s' % (run['id'],artifact['id']))
            return
        except Exception:
            # Any candidate failure (http.client.HTTPException included) is a
            # miss for that candidate; validation is never relaxed to recover.
            continue
    raise Miss('no admissible cache artifact')


def main(mode):
    require(mode in ('restore','pack'), 'expected restore or pack')
    root = Path(os.environ['RUNNER_TEMP'])/'assay-release-go'
    key = dict(schema=1,go=subprocess.check_output(['go','env','GOVERSION'],text=True).strip(),
               host=subprocess.check_output(['go','env','GOHOSTOS'],text=True).strip()+'-'+
                    subprocess.check_output(['go','env','GOHOSTARCH'],text=True).strip())
    if mode == 'pack':
        pack(root,root.parent/'go-cache.tar',key)
        return
    require(not root.exists(), 'release cache directory already exists')
    try:
        restore(root,key,os.environ)
    except Exception as e:
        # Do not print remote content, redirect URLs, tokens, or local paths.
        print('release Go cache: cold build (%s)' % type(e).__name__)
    (root/'build').mkdir(parents=True,exist_ok=True)
    (root/'mod').mkdir(parents=True,exist_ok=True)
    with open(os.environ['GITHUB_ENV'],'a',encoding='utf-8') as out:
        out.write('GOCACHE=%s\nGOMODCACHE=%s\n' % (root/'build',root/'mod'))
