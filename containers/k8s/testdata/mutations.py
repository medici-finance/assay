"""Committed hostile-manifest mutations; every control must be rejected."""
import base64
import copy
import hashlib
import json


# regression: desk-containers/06
def cases(clean):
    def set_value(path, value):
        def change(docs):
            obj = next(d for d in docs if d['kind'] == 'StatefulSet')
            for key in path[:-1]:
                obj = obj[key]
            obj[path[-1]] = value
        return change

    pod = ['spec', 'template', 'spec']
    container = pod + ['containers', 0]
    fixtures = {
        'missing-role': lambda ds: ds.remove(next(d for d in ds if d['kind'] == 'StatefulSet')),
        'duplicate-role': lambda ds: ds.append(copy.deepcopy(next(d for d in ds if d['kind'] == 'StatefulSet'))),
        'no-tty': set_value(container + ['tty'], False),
        'no-stdin': set_value(container + ['stdin'], False),
        'stdin-closes': set_value(container + ['stdinOnce'], True),
        'no-pvc': set_value(['spec', 'volumeClaimTemplates'], []),
        'mutable-image': set_value(container + ['image'], 'ghcr.io/medici-finance/assay/intake-desk:latest'),
        'privileged': set_value(container + ['securityContext', 'privileged'], True),
        'root-container': set_value(container + ['securityContext', 'runAsUser'], 0),
        'escalation': set_value(container + ['securityContext', 'allowPrivilegeEscalation'], True),
        'added-capability': set_value(container + ['securityContext', 'capabilities'], {'drop': ['ALL'], 'add': ['SYS_ADMIN']}),
        'host-network': set_value(pod + ['hostNetwork'], True),
        'host-path': set_value(pod + ['volumes'], [{'name': 'host', 'hostPath': {'path': '/'}}]),
        'docker-socket': set_value(pod + ['volumes'], [{'name': 'socket', 'hostPath': {'path': '/var/run/docker.sock'}}]),
        'agent-sa-token': set_value(pod + ['automountServiceAccountToken'], True),
        'projected-sa-token': set_value(pod + ['volumes'], [{'name': 'token', 'projected': {'sources': [{'serviceAccountToken': {'path': 'token'}}]}}]),
        'extra-container': set_value(pod + ['initContainers'], [{'name': 'other-role', 'image': 'example:latest'}]),
        'no-resources': set_value(container + ['resources'], {}),
        'writable-pem': set_value(container + ['volumeMounts', 1, 'readOnly'], False),
        'wrong-container': set_value(container + ['name'], 'other'),
        'inline-model-credential': set_value(container + ['env'], [{'name': 'ANTHROPIC_API_KEY', 'value': 'unexpected'}]),
        'example-secret-rendered': lambda ds: ds.append({'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': 'example', 'namespace': 'assay-desks'}}),
        'terminal-service': lambda ds: ds.append({'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': 'terminal', 'namespace': 'assay-desks'}}),
        'terminal-ingress': lambda ds: ds.append({'apiVersion': 'networking.k8s.io/v1', 'kind': 'Ingress', 'metadata': {'name': 'terminal', 'namespace': 'assay-desks'}}),
        'absent-ingress-deny': lambda ds: ds.remove(next(d for d in ds if d['metadata']['name'] == 'desk-default-deny')),
        'allow-ingress': lambda ds: next(d for d in ds if d['metadata']['name'] == 'desk-default-deny')['spec'].update({'ingress': [{}]}),
        'open-egress': lambda ds: next(d for d in ds if d['metadata']['name'] == 'desk-dns')['spec'].update({'egress': [{}]}),
    }
    def combine(ds):
        workloads = [d for d in ds if d['kind'] == 'StatefulSet']
        workloads[0]['spec']['template']['spec']['containers'] = [
            copy.deepcopy(w['spec']['template']['spec']['containers'][0]) for w in workloads]
        ds[:] = [d for d in ds if d['kind'] != 'StatefulSet'] + workloads[:1]
    fixtures['one-workload-all-roles'] = combine
    for name, path in [('missing-tty', container + ['tty']),
                       ('missing-stdin', container + ['stdin']),
                       ('missing-token-isolation', pod + ['automountServiceAccountToken']),
                       ('missing-work-mount', container + ['volumeMounts']),
                       ('missing-namespace', ['metadata', 'namespace'])]:
        def remove(ds, path=path):
            obj = next(d for d in ds if d['kind'] == 'StatefulSet')
            for key in path[:-1]:
                obj = obj[key]
            del obj[path[-1]]
        fixtures[name] = remove
    for field in ['model', 'pem']:
        def share(ds, field=field):
            pods = [d['spec']['template']['spec'] for d in ds if d['kind'] == 'StatefulSet']
            if field == 'model':
                pods[1]['containers'][0]['envFrom'] = copy.deepcopy(pods[0]['containers'][0]['envFrom'])
            else:
                pods[1]['volumes'] = copy.deepcopy(pods[0]['volumes'])
        fixtures['shared-' + field + '-secret'] = share
    # regression: F-k8s-uid-mismatch — plant each identity field at every role,
    # including sites beyond the originally reported worker workload.
    for workload in (d for d in clean if d['kind'] == 'StatefulSet'):
        role = workload['metadata']['name']
        for field in ['runAsUser', 'runAsGroup', 'fsGroup']:
            def wrong_identity(ds, role=role, field=field):
                target = next(d for d in ds if d['kind'] == 'StatefulSet'
                              and d['metadata']['name'] == role)
                target['spec']['template']['spec']['securityContext'][field] = 1000
            fixtures['image-identity-' + role + '-' + field] = wrong_identity
    # regression: #3857 — every role reference must be the one its release
    # build recorded. Each plant is a well-formed immutable reference that the
    # shape check alone admits: a digest no release recorded, another role's
    # recorded digest, and the recorded digest under a different release tag.
    for workload in (d for d in clean if d['kind'] == 'StatefulSet'):
        role = workload['metadata']['name']
        others = [d for d in clean if d['kind'] == 'StatefulSet' and d['metadata']['name'] != role]
        foreign = others[0]['spec']['template']['spec']['containers'][0]['image'].split('@')[1]
        def unrecorded(ds, role=role, kind='digest', foreign=foreign):
            target = next(d for d in ds if d['kind'] == 'StatefulSet'
                          and d['metadata']['name'] == role)
            box = target['spec']['template']['spec']['containers'][0]
            name, digest = box['image'].split('@')
            if kind == 'digest':
                digest = 'sha256:' + hashlib.sha256(b'unrecorded ' + role.encode()).hexdigest()
            elif kind == 'swapped':
                digest = foreign
            else:
                name = name.rsplit(':', 1)[0] + ':v0.0.1'
            box['image'] = name + '@' + digest
        for kind in ['digest', 'swapped', 'tag']:
            fixtures['image-provenance-' + role + '-' + kind] = (
                lambda ds, f=unrecorded, kind=kind: f(ds, kind=kind))
    for name, change in fixtures.items():
        mutant = copy.deepcopy(clean)
        change(mutant)
        yield name, mutant


# regression: #3857 — a coordinated edit: the same fabricated digest written
# into the record and into one role's manifest, at every role. The record stays
# a valid release-images-v1 document and the manifest equals it, so only the
# pin on the record's own bytes can reject the pair.
def record_cases(clean, record):
    for workload in (d for d in clean if d['kind'] == 'StatefulSet'):
        role = workload['metadata']['name']
        ref = workload['spec']['template']['spec']['containers'][0]['image']
        fabricated = ref.split('@')[0] + '@sha256:' + hashlib.sha256(b'fabricated ' + role.encode()).hexdigest()
        if record.count(ref.encode()) != 1:
            raise ValueError('record does not hold exactly one reference for ' + role)
        mutant = copy.deepcopy(clean)
        target = next(d for d in mutant if d['kind'] == 'StatefulSet'
                      and d['metadata']['name'] == role)
        target['spec']['template']['spec']['containers'][0]['image'] = fabricated
        yield ('image-provenance-' + role + '-fabricated-pair', mutant,
               record.replace(ref.encode(), fabricated.encode()))


# regression: desk-containers/06
def secret_control(encoded=False):
    # Construct only at test time; no secret-shaped literal or scanner exemption.
    token = 'gh' + 's_' + hashlib.sha256(b'public synthetic positive control').hexdigest()[:36]
    data = {'data': {'token': base64.b64encode(token.encode()).decode()}} if encoded else {'stringData': {'token': token}}
    return json.dumps({'apiVersion': 'v1', 'kind': 'Secret',
                       'metadata': {'name': 'excluded-example'}, **data}).encode()
