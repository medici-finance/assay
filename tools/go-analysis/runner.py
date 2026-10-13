#!/usr/bin/env python3
"""Thin adapter for pinned upstream JSON; no analyzer or new policy language."""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile
import urllib.request
import zipfile

OWNER = Path(__file__).resolve().parents[2]
BASELINE = 'tools/go-analysis/baseline.json'
POLICY = ['.golangci.yml', 'tools/go-analysis/pins.env']
SCHEMA = 'go-analysis/v1'
FINGERPRINT = 'source-window/v2'


def digest(data):
    return hashlib.sha256(data).hexdigest()


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args])


def pins(owner):
    return dict(line.split('=', 1) for line in (owner / POLICY[1]).read_text().splitlines()
                if line and not line.startswith('#'))


def modules(root):
    found = git(root, 'ls-files', '-z', '*go.mod').decode().split('\0')
    found = sorted({str(Path(x).parent) for x in found if x})
    if not found:
        raise ValueError('no tracked Go modules')
    for module in found:
        if not (root / module / 'go.mod').resolve().is_relative_to(root.resolve()):
            raise ValueError('module manifest escapes analyzed tree')
        if not (root / module / 'go.mod').is_file():
            raise ValueError('tracked manifest missing: ' + module)
    return found


def coverage(mods):
    result = [(m, 'linux', 'amd64', '') for m in mods]
    if 'tools/desk' in mods:
        result += [('tools/desk', 'windows', 'amd64', ''),
                   ('tools/desk', 'darwin', 'arm64', ''),
                   ('tools/desk', 'linux', 'amd64', 'parity')]
    return result


def download(url, target, expected):
    print('download ' + url, flush=True)
    if not re.fullmatch('[0-9a-f]{64}', expected):
        raise ValueError('invalid checksum pin')
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True)
        with urllib.request.urlopen(url, timeout=120) as r:
            data = r.read()
        if digest(data) != expected:
            raise ValueError('download checksum mismatch')
        target.write_bytes(data)
    if digest(target.read_bytes()) != expected:
        raise ValueError('cached archive checksum mismatch')


def environment(cache, toolchain, network=False):
    # Do not inherit workspace, proxy bypasses, auto-toolchain or injected compiler flags.
    temporary = cache / 'tmp'
    temporary.mkdir(parents=True, exist_ok=True)
    # GOPROXY=off still permits direct sumdb access; retain signature checks at a local-only URL.
    sumdb = 'sum.golang.org' if network else 'sum.golang.org ' + (cache / 'offline-sumdb').resolve().as_uri()
    env = dict(os.environ)
    for name in ['GOROOT', 'GOFLAGS', 'GOOS', 'GOARCH', 'GOPRIVATE', 'GONOPROXY',
                 'GONOSUMDB', 'GOINSECURE', 'GOTOOLDIR', 'GCCGO', 'CC', 'CXX']:
        env.pop(name, None)
    env.update(GOTOOLCHAIN='local', GOENV='off', GOWORK='off', CGO_ENABLED='0',
               GOPROXY='https://proxy.golang.org' if network else 'off',
               GOSUMDB=sumdb, GOVCS='*:off', GONOSUMDB='', GOPRIVATE='',
               TMPDIR=str(temporary), GOPATH=str(cache / 'gopath'), GOMODCACHE=str(cache / 'modules'),
               GOCACHE=str(cache / 'build'), GOLANGCI_LINT_CACHE=str(cache / 'lint-cache'),
               PATH=str(toolchain / 'bin') + os.pathsep + os.environ['PATH'])
    return env


def acquire(root, cache, owner):
    p = pins(owner)
    system = {'Darwin': 'darwin', 'Linux': 'linux'}.get(platform.system())
    arch = {'arm64': 'arm64', 'aarch64': 'arm64', 'x86_64': 'amd64'}.get(platform.machine())
    for kind in ['BUILD_GO', 'ANALYSIS_GO']:
        key = kind + '_' + str(system) + '_' + str(arch)
        if key not in p:
            raise ValueError('unsupported acquisition host')
        archive = cache / ('go' + p[kind] + '.tar.gz')
        download('https://go.dev/dl/go' + p[kind] + '.' + system + '-' + arch + '.tar.gz',
                 archive, p[key])
        dest = cache / kind.lower()
        if not (dest / 'go/bin/go').exists():
            dest.mkdir(parents=True, exist_ok=True)
            with tarfile.open(archive) as f:
                # Official archives are hash-admitted; still reject path escapes.
                for member in f.getmembers():
                    if not (dest / member.name).resolve().is_relative_to(dest.resolve()):
                        raise ValueError('archive path escape')
                f.extractall(dest, filter='data')
    build_go = cache / 'build_go/go'
    analysis_go = cache / 'analysis_go/go'
    archive = cache / 'linter.zip'
    url = 'https://proxy.golang.org/github.com/golangci/golangci-lint/v2/@v/v' + p['LINT_VERSION'] + '.zip'
    download(url, archive, p['LINT_SOURCE_SHA256'])
    source = cache / 'source' / ('github.com/golangci/golangci-lint/v2@v' + p['LINT_VERSION'])
    if not source.exists():
        with zipfile.ZipFile(archive) as z:
            for name in z.namelist():
                if not (cache / 'source' / name).resolve().is_relative_to((cache / 'source').resolve()):
                    raise ValueError('archive path escape')
            z.extractall(cache / 'source')
    for filename, key in [('go.mod', 'LINT_MOD_SHA256'), ('go.sum', 'LINT_SUM_SHA256')]:
        if digest((source / filename).read_bytes()) != p[key]:
            raise ValueError('upstream source lock mismatch: ' + filename)
    env = environment(cache, build_go, network=True)
    print('download https://proxy.golang.org; checksum verification https://sum.golang.org', flush=True)
    subprocess.run([str(build_go / 'bin/go'), 'mod', 'download'], cwd=source, env=env, check=True)
    subprocess.run([str(build_go / 'bin/go'), 'mod', 'verify'], cwd=source, env=env, check=True)
    binary = cache / 'golangci-lint'
    subprocess.run([str(build_go / 'bin/go'), 'build', '-mod=readonly', '-trimpath', '-buildvcs=false',
                    '-ldflags=-X main.version=' + p['LINT_VERSION'] + ' -X main.date=2026-09-24T11:27:54Z', '-o', str(binary),
                    './cmd/golangci-lint'], cwd=source, env=env, check=True)
    for filename, key in [('go.mod', 'LINT_MOD_SHA256'), ('go.sum', 'LINT_SUM_SHA256')]:
        if digest((source / filename).read_bytes()) != p[key]:
            raise ValueError('source build changed dependency graph')
    env = environment(cache, analysis_go, network=True)
    for module in modules(root):
        print('prepare module ' + module, flush=True)
        subprocess.run([str(analysis_go / 'bin/go'), 'mod', 'download'], cwd=root / module, env=env, check=True)
    receipt = {'pins_sha256': digest((owner / POLICY[1]).read_bytes()),
               'binary_sha256': digest(binary.read_bytes()), 'build_go': p['BUILD_GO'],
               'analysis_go': p['ANALYSIS_GO'], 'version': p['LINT_VERSION']}
    (cache / 'receipt.json').write_bytes(encoded(receipt))
    print('qualified inputs; binary sha256 ' + receipt['binary_sha256'])


def verify_tool(cache, owner):
    p = pins(owner)
    receipt = json.loads((cache / 'receipt.json').read_text())
    binary = cache / 'golangci-lint'
    if receipt['pins_sha256'] != digest((owner / POLICY[1]).read_bytes()):
        raise ValueError('tool pin changed; acquisition required')
    if receipt['binary_sha256'] != digest(binary.read_bytes()):
        raise ValueError('analyzer binary checksum mismatch')
    go = cache / 'analysis_go/go'
    env = environment(cache, go)
    actual = subprocess.check_output([str(go / 'bin/go'), 'version'], env=env).decode()
    if actual.split()[2] != 'go' + p['ANALYSIS_GO']:
        raise ValueError('analysis toolchain mismatch')
    actual = subprocess.check_output([str(binary), 'version'], env=env).decode()
    if not re.search(r'\bversion ' + re.escape(p['LINT_VERSION']) + r'\b', actual):
        raise ValueError('analyzer version mismatch')
    return binary, env, receipt


def contract(owner):
    return digest(b'\0'.join([(owner / f).read_bytes() for f in POLICY] + [FINGERPRINT.encode()]))


DECLARATION = re.compile(r'(?:func|var|const|type|import)\b')
RECEIVER = re.compile(r'func\s*\(\s*(?:\w+\s+)?\*?\s*([\w.]+)')
NAME = re.compile(r'func\s*(?:\([^)]*\)\s*)?(\w+)')


def declaration_scopes(lines):
    """Per source line: (index of its top-level declaration, stable declaration symbol)."""
    result = []
    start, symbol = -1, 'package'
    for number, text in enumerate(lines):
        if DECLARATION.match(text):
            start = number
            if text.startswith('func'):
                receiver, name = RECEIVER.match(text), NAME.match(text)
                symbol = 'func ' + (receiver.group(1) + '.' if receiver else '') + (name.group(1) if name else '')
            else:
                symbol = ' '.join(text.split('=', 1)[0].split('{', 1)[0].split())
        result.append((start, symbol))
    return result


def normalize(root, module, conf, issue):
    analyzer = issue['FromLinter']
    if analyzer not in ['errcheck', 'staticcheck', 'unused', 'govet', 'nolintlint']:
        raise ValueError('unknown analyzer result')
    pos = issue['Pos']
    path = Path(pos['Filename'])
    path = path if path.is_absolute() else root / module / path
    path = path.resolve()
    if not path.is_relative_to(root):
        raise ValueError('diagnostic path outside analyzed tree')
    rel = path.relative_to(root).as_posix()
    lines = path.read_text().splitlines()
    line = pos['Line']
    if not 1 <= line <= len(lines):
        raise ValueError('diagnostic source location invalid')
    # Stable source context is independent of line numbers and blank/comment shifts.
    # Identity is scoped to the enclosing top-level declaration, so a repair or addition
    # elsewhere in the file never re-identifies an untouched finding. Identical lines inside
    # one declaration need the smallest distinguishing neighborhood; a source edit that makes
    # an identity ambiguous fails for review, never count-matches.
    scopes = declaration_scopes(lines)
    significant = [(i, text.strip()) for i, text in enumerate(lines)
                   if text.strip() and (i == line - 1 or not text.lstrip().startswith('//'))]
    indexes = [n for n, (number, _) in enumerate(significant) if number == line - 1]
    if not indexes:
        raise ValueError('diagnostic has no source anchor')
    index = indexes[0]
    segments = {}
    for number, text in significant:
        segments.setdefault(scopes[number][0], []).append(text)
    own = scopes[line - 1][0]
    seg = segments[own]
    at = sum(1 for number, _ in significant[:index] if scopes[number][0] == own)
    # The preamble (package clause and the comments/directives above the first declaration) has
    # start -1, which is not a list index; it is its own 'package' scope.
    def symbol_of(start):
        return scopes[start][1] if start >= 0 else 'package'

    peers = [segments[start] for start in segments if symbol_of(start) == scopes[line - 1][1]]
    window = None
    for radius in range(0, 65):
        def around(rows, position):
            return (min(position, radius), rows[max(0, position - radius):position + radius + 1])
        candidate = around(seg, at)
        equals = sum(1 for rows in peers for i, text in enumerate(rows)
                     if text == seg[at] and around(rows, i) == candidate)
        if equals == 1:
            window = candidate
            break
    if window is None:
        raise ValueError('ambiguous source anchor')
    prefix = lines[line - 1][:max(0, pos.get('Column', 1) - 1)].strip()
    text = issue['Text']
    if not isinstance(text, str) or not text:
        raise ValueError('diagnostic missing message')
    rule = re.match(r'(SA\d+|S\d+|ST\d+|QF\d+):', text)
    rule = rule.group(1) if rule else analyzer
    identity = {'analyzer': analyzer, 'rule': rule, 'module': module,
                'configuration': conf, 'path': rel, 'anchor': digest(encoded([scopes[line - 1][1], window, prefix]))}
    # Message hash distinguishes changed diagnostic severity/content at the same location.
    return dict(identity, id=digest(encoded(identity)), diagnostic=digest(text.encode()),
                line=line, column=pos.get('Column', 0))


def records(value):
    if value.get('schema') != 'go-analysis-baseline/v1' or not isinstance(value.get('findings'), list):
        raise ValueError('malformed baseline')
    result = {}
    for record in value['findings']:
        required = ['id', 'diagnostic', 'analyzer', 'rule', 'module', 'configuration',
                    'path', 'anchor', 'first_seen', 'owner', 'work', 'reason']
        if any(not isinstance(record.get(k), str) or not record[k] for k in required):
            raise ValueError('malformed baseline record')
        if record['id'] in result:
            raise ValueError('duplicate baseline identity')
        identity = {k: record[k] for k in ['analyzer', 'rule', 'module', 'configuration', 'path', 'anchor']}
        if record['id'] != digest(encoded(identity)):
            raise ValueError('invalid baseline identity')
        result[record['id']] = record
    return result


def compare(findings, candidate, trusted, complete, policy):
    current, prior = records(candidate), records(trusted)
    if candidate.get('contract') != policy or trusted.get('contract') != policy:
        raise ValueError('tool/config migration requires reviewed baseline')
    if any(key not in prior or record != prior[key] for key, record in current.items()):
        raise ValueError('baseline expansion or rewrite prohibited')
    live = {}
    for item in findings:
        if item['id'] in live:
            raise ValueError('ambiguous duplicate finding identity: ' + item['path'])
        live[item['id']] = item
        old = current.get(item['id'])
        item['disposition'] = 'new' if old is None else ('existing' if old['diagnostic'] == item['diagnostic'] else 'worsened')
        item['exception'] = {k: old[k] for k in ['first_seen', 'owner', 'work', 'reason']} if old else None
    resolved = []
    if complete:
        stale = current.keys() - live.keys()
        if stale:
            raise ValueError('resolved findings retain baseline allowances: ' + str(len(stale)))
        resolved = [dict(v, disposition='resolved') for k, v in prior.items() if k not in live]
    return findings + resolved


def lint(root, cache, owner, base, trusted_root=None):
    raw = cache / 'raw'
    raw.mkdir(parents=True, exist_ok=True)
    now = dt.datetime.now(dt.timezone.utc).isoformat()
    report = {'schema': SCHEMA, 'repository': os.getenv('GITHUB_REPOSITORY', 'medici-finance/assay'),
              'source_sha': git(root, 'rev-parse', 'HEAD').decode().strip(), 'base_sha': base,
              'event_sha': os.getenv('GITHUB_SHA', ''), 'pr_head_sha': os.getenv('GO_ANALYSIS_PR_HEAD', ''), 'pr_base_sha': os.getenv('GO_ANALYSIS_PR_BASE', ''),
              'run_id': os.getenv('GITHUB_RUN_ID', 'local'), 'attempt': os.getenv('GITHUB_RUN_ATTEMPT', '1'),
              'scanned_at': now, 'family': 'lint', 'fingerprint': FINGERPRINT, 'contract': contract(owner),
              'coverage': [], 'findings': [], 'errors': [], 'state': 'could-not-check', 'gate': 'fail'}
    exit_code = 2
    try:
        mods = modules(root)
        report['module_count'] = len(mods)
        plan = coverage(mods)
        report['expected_scans'] = len(plan)
        for name in POLICY:
            if (root / name).read_bytes() != (owner / name).read_bytes():
                raise ValueError('analysis policy migration requires reviewed trusted runner')
        binary, env, receipt = verify_tool(cache, owner)
        report['tool'] = receipt
        findings = []
        for index, (module, goos, goarch, tags) in enumerate(plan):
            conf = goos + '/' + goarch + ';tags=' + tags + ';tests=true;cgo=0'
            outfile = raw / (str(index) + '.json')
            stderr = raw / (str(index) + '.stderr')
            stdout = raw / (str(index) + '.stdout')
            scan = dict(module=module, configuration=conf, state='could-not-check', findings=None)
            report['coverage'].append(scan)
            print('scan ' + module + ' ' + conf, flush=True)
            try:
                outfile.unlink(missing_ok=True)
                with stdout.open('wb') as out, stderr.open('wb') as err:
                    proc = subprocess.run([str(binary), 'run', '--config', str(owner / '.golangci.yml'),
                                           '--output.json.path', str(outfile), '--build-tags', tags, './...'],
                                          cwd=root / module, env=dict(env, GOOS=goos, GOARCH=goarch),
                                          stdout=out, stderr=err, timeout=660)
                scan['exit_code'] = proc.returncode
                scan['raw'] = {'path': 'raw/' + outfile.name, 'sha256': digest(outfile.read_bytes())}
                payload = json.loads(outfile.read_text())
                issues = payload['Issues']
                if issues is None:
                    issues = []
                if not isinstance(issues, list) or not isinstance(payload.get('Report'), dict):
                    raise ValueError('malformed analyzer output')
                if proc.returncode not in [0, 1] or (proc.returncode == 0 and issues) or (proc.returncode == 1 and not issues):
                    raise ValueError('analyzer execution failed or exit/result mismatch')
                if payload['Report'].get('Error') or payload['Report'].get('Warnings'):
                    raise ValueError('analyzer reports incomplete/warning state')
                normalized = [normalize(root, module, conf, issue) for issue in issues]
                findings.extend(normalized)
                scan.update(state='complete', findings=len(normalized))
            except (ValueError, KeyError, OSError, subprocess.SubprocessError) as exc:
                scan['error'] = type(exc).__name__ + ': analyzer output or execution could not be validated'
                print('scan could-not-check: ' + module + ': ' + str(exc), file=sys.stderr)
        report['findings'] = findings
        complete = all(s['state'] == 'complete' for s in report['coverage'])
        candidate_bytes = (root / BASELINE).read_bytes()
        if trusted_root:
            trusted_bytes = (trusted_root / BASELINE).read_bytes()
            trusted_sha = git(trusted_root, 'rev-parse', 'HEAD').decode().strip()
        else:
            if not base or not re.fullmatch('[0-9a-f]{40}', base):
                raise ValueError('explicit trusted base SHA required')
            trusted_bytes = git(root, 'show', base + ':' + BASELINE)
            trusted_sha = base
        report['baseline'] = {'candidate_digest': digest(candidate_bytes),
                              'trusted_digest': digest(trusted_bytes), 'revision': trusted_sha}
        report['findings'] = compare(findings, json.loads(candidate_bytes), json.loads(trusted_bytes), complete, contract(owner))
        report['state'] = 'complete' if complete else 'could-not-check'
        regressions = any(f['disposition'] in ['new', 'worsened'] for f in report['findings'])
        exit_code = 2 if not complete else (1 if regressions else 0)
        report['gate'] = 'pass' if exit_code == 0 else 'fail'
    except (ValueError, KeyError, OSError, subprocess.SubprocessError) as exc:
        report['errors'].append(type(exc).__name__ + ': acquisition, coverage or ratchet contract could not be validated')
        print('could-not-check: ' + str(exc), file=sys.stderr)
    report['exit_code'] = exit_code
    report['totals'] = {d: sum(f.get('disposition') == d for f in report['findings']) for d in ['existing', 'new', 'worsened', 'resolved']}
    target = cache / 'report.json'
    target.write_text(json.dumps(report, indent=2, sort_keys=True) + '\n')
    print('report ' + str(target.relative_to(root)) if target.is_relative_to(root) else 'report written')
    print(json.dumps({'state': report['state'], 'gate': report['gate'], 'totals': report['totals']}))
    return exit_code


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['acquire', 'lint'])
    parser.add_argument('--root', type=Path, default=Path.cwd())
    parser.add_argument('--allow-downloads', action='store_true')
    parser.add_argument('--base', default=os.getenv('GO_ANALYSIS_BASE', ''))
    parser.add_argument('--trusted-root', type=Path)
    parser.add_argument('--cache', type=Path)
    args = parser.parse_args()
    root = Path(git(args.root, 'rev-parse', '--show-toplevel').decode().strip()).resolve()
    cache = (args.cache or root / '.go-analysis').resolve()
    cache.mkdir(exist_ok=True)
    if args.mode == 'acquire':
        if not args.allow_downloads:
            parser.error('acquire requires explicit --allow-downloads')
        acquire(root, cache, OWNER)
        return 0
    if args.allow_downloads:
        parser.error('lint is offline; use acquire separately')
    return lint(root, cache, OWNER, args.base, args.trusted_root)


if __name__ == '__main__':
    sys.exit(main())
