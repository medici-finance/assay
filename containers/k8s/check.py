#!/usr/bin/env python3
"""Offline packaging checks, not cluster-policy or runtime-owner qualification."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
LABEL = 'app.kubernetes.io/name'
PART = 'app.kubernetes.io/part-of'
VERSIONS = {'kustomize': ('version', 'v5.8.3'),
            'kubeconform': ('-v', 'v0.8.0'), 'gitleaks': ('version', '8.30.1')}


class Invalid(Exception):
    pass


class Unavailable(Exception):
    pass


def need(condition, message):
    if not condition:
        raise Invalid(message)


def run(args, data=None, cwd=ROOT):
    try:
        return subprocess.run(args, input=data, stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE, cwd=cwd, timeout=90,
                              env={**os.environ, 'KUBECONFIG': '/dev/null',
                                   'GOPROXY': 'off', 'GOSUMDB': 'off',
                                   'GOTOOLCHAIN': 'local'})
    except (OSError, subprocess.TimeoutExpired) as err:
        raise Unavailable(f'{args[0]} could not complete') from err


def success(proc, what):
    if proc.returncode:
        raise Unavailable(f'{what} did not complete (exit {proc.returncode})')
    return proc.stdout


def version(tool):
    flag, expected = VERSIONS[tool]
    got = success(run([tool, flag]), f'{tool} version').decode().strip()
    if got != expected:
        raise Unavailable(f'{tool} version {got!r}; require {expected}')


def yaml_docs(raw):
    proc = run(['go', 'run', str(HERE / 'yaml-json.go')], raw,
               ROOT / 'tools/desk')
    if proc.returncode:
        # Compiler/cache failures are not evidence that a manifest is invalid.
        if b'invalid YAML:' in proc.stderr:
            raise Invalid('invalid YAML document')
        raise Unavailable('YAML adapter requires the existing Go toolchain/module cache')
    return json.loads(proc.stdout) or []


def roles():
    # Image Dockerfiles are the current role registry; no sixth copy of the list.
    found = []
    for dockerfile in sorted((ROOT / 'containers').glob('*/Dockerfile')):
        match = re.search(r'^ENV ASSAY_DESK=(\S+)\s*$', dockerfile.read_text(), re.M)
        if match:
            need(match[1] == dockerfile.parent.name, 'image role/path mismatch')
            found.append(match[1])
    need(len(found) == 5 and len(set(found)) == 5, 'expected five image roles')
    return found


def safe_file(path, home=HERE):
    need(path.is_file() and not path.is_symlink(), 'manifest must be a regular local file')
    need(path.resolve().is_relative_to(home.resolve()), 'manifest escapes the package')


RECORD = HERE / 'release-images.json'
# The sha256 the selected release's checksums.txt lists for release-images.json.
# README.md "Image provenance" states the same value; both move together with
# the record, so a record edit cannot pass without changing this line too.
RECORD_SHA256 = 'ff9237178e5bbf9523012a83e2366b99d2ff3cd0092c6116ad4c0dee08afdd64'
RECORD_PIN_MISMATCH = 'release image record is not the pinned release asset'


def recorded_images(raw=None):
    # The release build's own image record (release-images-v1), kept verbatim.
    # The release's checksums.txt lists the sha256 of these exact bytes, so the
    # file binds each role digest to the release and source revision that built
    # it. Its contract validator is reused rather than restated here.
    if raw is None:
        safe_file(RECORD)
        raw = RECORD.read_bytes()
    need(hashlib.sha256(raw).hexdigest() == RECORD_SHA256, RECORD_PIN_MISMATCH)
    readme = HERE / 'README.md'
    safe_file(readme)
    need(RECORD_SHA256 in readme.read_text(encoding='utf-8'),
         'README.md does not state the pinned release image record sha256')
    sys.dont_write_bytecode = True
    sys.path.insert(0, str(ROOT / 'tools/release'))
    try:
        from images_contract import validate
    finally:
        sys.path.pop(0)
    try:
        record = json.loads(raw)
        refs = {i['name']: i['ref'] for i in record['images']}
        return validate(record, record['version'], record['source_revision'],
                        refs['verify-runner'] + '\n', 'medici-finance/assay')
    except (ValueError, KeyError, TypeError) as err:
        raise Invalid('release image record is not a valid release-images-v1 record') from err


def render():
    # Admit a deliberately small Kustomize base BEFORE invoking its renderer.
    # Remote bases, generators, plugins and Helm cannot perform hidden I/O.
    wrapper = HERE / 'kustomization.yaml'
    safe_file(wrapper)
    need(yaml_docs(wrapper.read_bytes()) == [{'apiVersion': 'kustomize.config.k8s.io/v1beta1',
         'kind': 'Kustomization', 'resources': ['base']}], 'root must select only the local base')
    path = HERE / 'base/kustomization.yaml'
    safe_file(path)
    docs = yaml_docs(path.read_bytes())
    need(len(docs) == 1, 'one base kustomization required')
    conf = docs[0]
    need(set(conf) == {'apiVersion', 'kind', 'namespace', 'resources'},
         'base supports local resources only')
    expected = ['namespace.yaml', 'network.yaml'] + [r + '.yaml' for r in roles()]
    need(sorted(conf['resources']) == sorted(expected), 'base resource inventory differs')
    for name in conf['resources']:
        safe_file(HERE / 'base' / name)
    version('kustomize')
    return success(run(['kustomize', 'build', str(HERE),
                        '--load-restrictor=LoadRestrictionsRootOnly']), 'render')


def schema_check(docs):
    version('kubeconform')
    cache = HERE / 'schemas'
    checksums = json.loads((cache / 'checksums.json').read_text())
    need(set(checksums) == {'namespace-v1.json', 'serviceaccount-v1.json',
                          'statefulset-apps-v1.json', 'networkpolicy-networking-v1.json'},
         'schema cache inventory differs')
    for name, digest in checksums.items():
        path = cache / name
        safe_file(path)
        raw = path.read_bytes()
        need(hashlib.sha256(raw).hexdigest() == digest, 'schema checksum mismatch')
        def refs(node):
            if isinstance(node, dict):
                for key, value in node.items():
                    if key == '$ref':
                        need(isinstance(value, str) and value.startswith('#'),
                             'nonlocal schema reference')
                    refs(value)
            elif isinstance(node, list):
                for value in node:
                    refs(value)
        refs(json.loads(raw))
    payload = b'\n---\n'.join(json.dumps(d).encode() for d in docs)
    proc = run(['kubeconform', '-strict', '-summary', '-output', 'json',
                '-kubernetes-version', '1.34.1', '-schema-location',
                str(cache / '{{.ResourceKind}}{{.KindSuffix}}.json')], payload)
    try:
        report = json.loads(proc.stdout)
        summary = report['summary']
    except (ValueError, KeyError) as err:
        raise Unavailable('schema validator produced no complete report') from err
    need(proc.returncode == 0 and summary.get('valid') == len(docs)
         and all(summary.get(k) == 0 for k in ['invalid', 'errors', 'skipped']),
         'schema validation failed or skipped a resource')
    return len(docs)


def structured(docs, record=None):
    expected = set(roles())
    recorded = recorded_images(record)
    need(docs, 'empty render')
    allowed = {'Namespace', 'ServiceAccount', 'StatefulSet', 'NetworkPolicy'}
    seen, workloads, namespaces, accounts, policies = set(), {}, [], [], []
    for doc in docs:
        kind, meta = doc.get('kind'), doc.get('metadata', {})
        name = meta.get('name')
        need(kind in allowed, 'unexpected kind (Secrets and terminal Services/Ingress are excluded)')
        key = (kind, meta.get('namespace'), name)
        need(name and key not in seen, 'absent or duplicate resource identity')
        seen.add(key)
        if kind == 'Namespace':
            namespaces.append(doc)
        else:
            need(meta.get('namespace') == 'assay-desks', 'explicit namespace required')
        if kind == 'StatefulSet':
            need(name in expected, 'unknown role workload')
            workloads[name] = doc
        elif kind == 'ServiceAccount':
            accounts.append(doc)
        elif kind == 'NetworkPolicy':
            policies.append(doc)
    need(len(namespaces) == 1 and namespaces[0]['metadata']['name'] == 'assay-desks',
         'one explicit namespace required')
    labels = namespaces[0]['metadata'].get('labels', {})
    need(labels.get('pod-security.kubernetes.io/enforce') == 'restricted', 'restricted namespace required')
    need(len(accounts) == 1 and accounts[0]['metadata']['name'] == 'desk-compat'
         and accounts[0].get('automountServiceAccountToken') is False
         and not accounts[0].get('secrets'), 'credential-free service account required')
    need(set(workloads) == expected, 'absent role (five separate StatefulSets required)')
    all_secrets = set()
    for role, workload in workloads.items():
        spec = workload['spec']
        need(spec.get('replicas') == 1, 'one replica per role required')
        need(spec.get('serviceName') == role, 'explicit stable workload identity required')
        need(spec.get('persistentVolumeClaimRetentionPolicy') ==
             {'whenDeleted': 'Retain', 'whenScaled': 'Retain'}, 'retain per-role PVCs')
        template = spec['template']
        labels = template.get('metadata', {}).get('labels', {})
        need(spec.get('selector') == {'matchLabels': {LABEL: role}}
             and labels.get(LABEL) == role and labels.get(PART) == 'assay-desks'
             and labels.get('assay.dev/profile') == 'compatibility', 'explicit role/profile selectors required')
        pod = template['spec']
        need(pod.get('serviceAccountName') == 'desk-compat'
             and pod.get('automountServiceAccountToken') is False, 'agent token isolation required')
        need(not any(pod.get(k) for k in ['hostNetwork', 'hostPID', 'hostIPC',
                  'shareProcessNamespace', 'initContainers', 'ephemeralContainers']),
             'host namespaces and additional containers forbidden')
        sec = pod.get('securityContext', {})
        # containers/base/Dockerfile pins desk to 999:999 and asserts it at build
        # time; the v1.0.52 images predate that pin and were checked per README.
        need(sec.get('runAsNonRoot') is True
             and all(sec.get(k) == 999 for k in ['runAsUser', 'runAsGroup', 'fsGroup'])
             and sec.get('seccompProfile') == {'type': 'RuntimeDefault'},
             'non-root sandbox with image desk UID/GID/fsGroup 999 required')
        containers = pod.get('containers', [])
        need(len(containers) == 1, 'exactly one interactive role container per Pod')
        container = containers[0]
        need(container.get('name') == role, 'explicit container name must equal role')
        need(container.get('stdin') is True and container.get('tty') is True
             and container.get('stdinOnce') is False, 'retained interactive stdin/tty required')
        image = container.get('image', '')
        need(re.fullmatch(r'ghcr\.io/medici-finance/assay/' + re.escape(role)
                          + r':v[0-9.]+@sha256:[a-f0-9]{64}', image), 'immutable role image required')
        need(image == recorded.get(role), 'role image is not the reference its release build recorded')
        need(not container.get('command') and not container.get('args'), 'preserve role image boot')
        need(not container.get('ports'), 'terminal port exposure forbidden')
        csec = container.get('securityContext', {})
        need(csec.get('allowPrivilegeEscalation') is False and not csec.get('privileged')
             and csec.get('capabilities') == {'drop': ['ALL']}
             and not any(k in csec for k in ['runAsUser', 'runAsGroup', 'runAsNonRoot',
                         'seccompProfile', 'procMount']), 'container privilege isolation required')
        resources = container.get('resources', {})
        for group in ['requests', 'limits']:
            need(all(resources.get(group, {}).get(k) for k in ['cpu', 'memory']), 'declared resources required')
        need(container.get('env') == [{'name': 'ASSAY_DESK', 'value': role},
             {'name': 'ASSAY_APP_PEM_FILE', 'value': '/run/secrets/assay/app.pem'}],
             'only role identity and canonical PEM path may be inline env')
        env = container.get('envFrom', [])
        need(len(env) == 1 and set(env[0]) == {'secretRef'}, 'one role-specific model Secret required')
        model = env[0]['secretRef']
        need(model.get('name') and not model.get('optional') and not env[0].get('prefix'), 'required model Secret')
        volumes = pod.get('volumes', [])
        need(len(volumes) == 1 and set(volumes[0]) == {'name', 'secret'}
             and volumes[0]['name'] == 'app-pem', 'host/projected/socket/extra volumes forbidden')
        pem = volumes[0]['secret']
        need(pem.get('secretName') and not pem.get('optional')
             and pem.get('defaultMode') == 0o440
             and pem.get('items') == [{'key': 'app.pem', 'path': 'app.pem'}], 'readable, scoped PEM Secret required')
        need(container.get('volumeMounts') == [{'name': 'work', 'mountPath': '/work'},
             {'name': 'app-pem', 'mountPath': '/run/secrets/assay', 'readOnly': True}],
             'distinct workspace and read-only canonical PEM mount required')
        for secret in [model['name'], pem['secretName']]:
            need(secret not in all_secrets, 'shared per-role credential Secret forbidden')
            all_secrets.add(secret)
        claims = spec.get('volumeClaimTemplates', [])
        need(len(claims) == 1 and claims[0].get('metadata', {}).get('name') == 'work', 'one per-role work PVC required')
        claim = claims[0]['spec']
        need(claim.get('accessModes') == ['ReadWriteOnce']
             and claim.get('resources', {}).get('requests', {}).get('storage')
             and not any(k in claim for k in ['volumeName', 'dataSource', 'dataSourceRef', 'selector']),
             'independently provisioned work PVC required')
    # NetworkPolicies are additive: a deny object cannot neutralize another allow.
    need(len(policies) == 2, 'exact deny and DNS policy set required in base')
    by_name = {p['metadata']['name']: p['spec'] for p in policies}
    need(by_name.get('desk-default-deny') == {'podSelector': {},
         'policyTypes': ['Ingress', 'Egress'], 'ingress': [], 'egress': []}, 'default ingress/egress deny required')
    dns = {'podSelector': {'matchLabels': {PART: 'assay-desks'}},
           'policyTypes': ['Egress'], 'egress': [{'to': [{'namespaceSelector': {
               'matchLabels': {'kubernetes.io/metadata.name': 'kube-system'}},
               'podSelector': {'matchLabels': {'k8s-app': 'kube-dns'}}}],
               'ports': [{'protocol': 'UDP', 'port': 53}, {'protocol': 'TCP', 'port': 53}]}]}
    need(by_name.get('desk-dns') == dns, 'only selected DNS egress allowed in base')


def source_files(repo=ROOT, package=HERE):
    proc = run(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard',
                '--', 'containers/k8s'], cwd=repo)
    names = success(proc, 'source inventory').decode().split('\0')
    # Scan the whole package, not only rendered or extension-selected manifests.
    # This includes examples, comments and future nonstandard example suffixes.
    paths = [repo / n for n in names if n]
    need(paths, 'no manifest sources scanned')
    for path in paths:
        safe_file(path, package)
    return paths


def scan_sources(paths):
    version('gitleaks')
    with tempfile.TemporaryDirectory(prefix='assay-k8s-scan-') as scratch:
        target = Path(scratch)
        for i, path in enumerate(paths):
            raw = path.read_bytes()
            (target / f'{i}-{path.name}').write_bytes(raw)
            # Explicitly decode Secret.data; never allow base64 to hide a credential.
            documents = yaml_docs(raw) if path.suffix in {'.yaml', '.yml', '.json'} else []
            for j, doc in enumerate(documents):
                if doc.get('kind') == 'Secret':
                    for key, val in doc.get('data', {}).items():
                        try:
                            decoded = base64.b64decode(val, validate=True)
                        except (ValueError, TypeError) as err:
                            raise Invalid('invalid Secret.data encoding') from err
                        (target / f'{i}-{j}-{hashlib.sha256(key.encode()).hexdigest()[:8]}.txt').write_bytes(decoded)
        proc = run(['gitleaks', 'dir', str(target), '--config', str(ROOT / '.gitleaks.toml'),
                    '--no-banner', '--redact', '--ignore-gitleaks-allow', '--exit-code', '9'])
        if proc.returncode == 9:
            raise Invalid('credential detected in manifest source (details withheld)')
        success(proc, 'credential scanner')
    return len(paths)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--base', action='store_true')
    mode.add_argument('--source-secrets', action='store_true')
    parser.add_argument('--self-test', action='store_true')
    args = parser.parse_args()
    if args.base:
        docs = yaml_docs(render())
        structured(docs)
        count = schema_check(docs)
        print(f'checked-clean: {count} schemas; five distinct interactive role workloads/PVCs')
        if args.self_test:
            from testdata.mutations import cases
            rejected = 0
            for name, mutant in cases(docs):
                try:
                    structured(mutant)
                except Invalid:
                    print(f'rejected: {name}')
                    rejected += 1
                else:
                    raise Invalid(f'hostile fixture escaped: {name}')
            # A fabricated digest written into the record AND the manifest agrees
            # with itself; only the record pin may reject it.
            from testdata.mutations import record_cases
            for name, mutant, record in record_cases(docs, RECORD.read_bytes()):
                try:
                    structured(mutant, record)
                except Invalid as err:
                    need(str(err) == RECORD_PIN_MISMATCH, f'{name} rejected for another reason: {err}')
                    print(f'rejected: {name}')
                    rejected += 1
                else:
                    raise Invalid(f'hostile fixture escaped: {name}')
            # Prove the schema validator itself rejects malformed Kubernetes types.
            mutant = json.loads(json.dumps(docs))
            next(d for d in mutant if d['kind'] == 'StatefulSet')['spec']['minReadySeconds'] = 'invalid'
            try:
                schema_check(mutant)
            except Invalid:
                print('rejected: schema-wrong-type')
                rejected += 1
            else:
                raise Invalid('invalid schema fixture escaped')
            print(f'checked-clean: {rejected} hostile fixtures rejected')
    else:
        paths = source_files()
        count = scan_sources(paths)
        print(f'checked-clean: {count} package sources scanned, including unrendered examples')
        if args.self_test:
            from testdata.mutations import secret_control
            # Exercise the real git inventory on an independent, staged corpus.
            # Its excluded example is never listed in the render resources.
            for encoded in [False, True]:
                with tempfile.TemporaryDirectory(prefix='assay-k8s-control-') as scratch:
                    repo = Path(scratch)
                    package = repo / 'containers/k8s'
                    package.mkdir(parents=True)
                    (package / 'kustomization.yaml').write_text(
                        'apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: []\n')
                    (package / 'excluded-example.yaml').write_bytes(secret_control(encoded))
                    success(run(['git', 'init', '-q', str(repo)]), 'control repository')
                    success(run(['git', 'add', 'containers/k8s/kustomization.yaml',
                                 'containers/k8s/excluded-example.yaml'], cwd=repo), 'control index')
                    control_paths = source_files(repo, package)
                    need(len(control_paths) == 2, 'source inventory missed excluded example')
                    try:
                        scan_sources(control_paths)
                    except Invalid as err:
                        need('credential detected' in str(err), 'positive control failed for another reason')
                        print('rejected: excluded-example-' + ('base64' if encoded else 'credential'))
                    else:
                        raise Invalid('unrendered credential positive control escaped')



if __name__ == '__main__':
    try:
        main()
    except Invalid as err:
        print(f'checked-failed: {err}', file=sys.stderr)
        sys.exit(1)
    except (Unavailable, OSError, ValueError, KeyError, TypeError) as err:
        print(f'could-not-check: {err}', file=sys.stderr)
        sys.exit(2)
