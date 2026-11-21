"""Qualify target selection and declared custody against offline fixtures."""
import argparse
from copy import deepcopy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
TEST = 'TestKubeCompatCustodyTarget'
NATIVE_REFUSALS = {'endpoint', 'tls-name', 'ca-pin', 'context', 'namespace', 'profile'}
FIXTURE_REFUSALS = {'opt-in', 'valid-other-profile', 'other-run', 'missing-receipt'}
TARGET_CASES = {'admitted'} | NATIVE_REFUSALS | FIXTURE_REFUSALS
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('k8s_check', HERE.parent / 'check.py')
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


def workloads(docs):
    return [doc for doc in docs if doc['kind'] == 'StatefulSet']


def pod(workload):
    return workload['spec']['template']['spec']


def secret_refs(workload):
    config = pod(workload)
    return [config['volumes'][0]['secret']['secretName'],
            config['containers'][0]['envFrom'][0]['secretRef']['name']]


def prepared_fixture():
    docs = check.compatibility_render()
    pins = check.candidate_images()
    custody = json.loads((HERE.parent / 'components/compatibility-cellctl/custody.json').read_bytes())
    markers = {}
    for workload in workloads(docs):
        role = workload['metadata']['name']
        for env in pod(workload)['containers'][0]['env']:
            if env['name'] in ('ASSAY_COMPAT_LAUNCH_ID', 'ASSAY_COMPAT_INIT_ID'):
                env['value'] = hashlib.sha256(f'fixture-{role}-{env["name"]}'.encode()).hexdigest()
        for ref in secret_refs(workload):
            markers[ref] = 'offline-marker-' + ref
    for doc in docs:
        if doc['kind'] == 'RoleBinding':
            doc['subjects'][0]['name'] = 'fixture-' + doc['metadata']['name']
    return docs, pins, custody, markers


def validate(docs, pins, custody, markers):
    check.compatibility_structured(docs, pins, custody)
    # Two distinct Secret refs can still contain the same material. Here the
    # only values are inert fixture markers; the suite has no Secret reader.
    observed = set()
    for workload in workloads(docs):
        for ref in secret_refs(workload):
            marker = markers.get(ref)
            check.need(marker and marker not in observed, 'missing or shared fixture credential material')
            observed.add(marker)


def check_mutations(docs, pins, custody, markers):
    failures = {}
    for name, reason in [
        ('broader-terminal-grant', 'broad/foreign-role/log/debug credential scope forbidden'),
        ('broader-log-grant', 'broad/foreign-role/log/debug credential scope forbidden'),
        ('shared-secret-reference', 'shared per-role credential Secret forbidden'),
        ('shared-material', 'missing or shared fixture credential material'),
        ('transcript-sidecar', 'exactly one interactive role container per Pod'),
        ('downstream-collector', 'terminal/log/node/network custody declaration missing'),
        ('egress', 'default ingress/egress deny required'),
        ('privilege', 'container privilege isolation required'),
    ]:
        changed, declaration, material = deepcopy(docs), custody.copy(), markers.copy()
        first, second = workloads(changed)[:2]
        if name in ('broader-terminal-grant', 'broader-log-grant'):
            rule = next(doc for doc in changed if doc['kind'] == 'Role')['rules'][-1]
            if name == 'broader-terminal-grant':
                rule['resourceNames'].append('other-fixture-0')
            else:
                rule['resources'].append('pods/log')
        elif name == 'shared-secret-reference':
            pod(first)['volumes'][0]['secret']['secretName'] = secret_refs(second)[0]
        elif name == 'shared-material':
            material[secret_refs(first)[0]] = material[secret_refs(second)[0]]
        elif name == 'transcript-sidecar':
            pod(first)['containers'].append({'name': 'fixture-transcript-collector'})
        elif name == 'downstream-collector':
            declaration['downstream_logs'] = 'shared-fixture-collector'
        elif name == 'egress':
            deny = next(doc for doc in changed if doc['metadata']['name'] == 'desk-default-deny')
            deny['spec']['egress'] = [{}]
        elif name == 'privilege':
            pod(first)['containers'][0]['securityContext']['allowPrivilegeEscalation'] = True
        try:
            validate(changed, pins, declaration, material)
        except check.Invalid as err:
            check.need(str(err) == reason, f'{name} refused for unexpected reason: {err}')
            failures[name] = 'rejected'
        else:
            raise check.Invalid(f'{name} accepted')
    return failures


def check_targets():
    run = subprocess.run(['go', 'test', '-json', '-count=1', '-timeout=5m',
                          '-run', '^' + TEST + '$', './cmd/cellctl'],
                         cwd=ROOT / 'tools/desk', env=check.offline_env(), capture_output=True, text=True)
    required = {TEST} | {f'{TEST}/{case}' for case in TARGET_CASES}
    passed, failed = set(), set()
    for line in run.stdout.splitlines():
        event = json.loads(line)
        name = event.get('Test')
        if event.get('Action') == 'fail' and name in required:
            failed.add(name)
        if event.get('Action') == 'pass':
            passed.add(name)
        if run.returncode and event.get('Output'):
            sys.stderr.write(event['Output'])
    if run.stderr:
        sys.stderr.write(run.stderr)
    if failed:
        raise check.Invalid('named custody-target tests failed: ' + ', '.join(sorted(failed)))
    if run.returncode:
        raise check.Unavailable('Go did not report a result for the selected custody-target tests')
    missing = required - passed
    if missing:
        raise check.Unavailable('missing named test passes: ' + ', '.join(sorted(missing)))


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument('--offline', required=True, action='store_true')
    parser.add_argument('--self-test', action='store_true')
    args = parser.parse_args()
    docs, pins, custody, markers = prepared_fixture()
    validate(docs, pins, custody, markers)
    rejections = check_mutations(docs, pins, custody, markers) if args.self_test else {}
    check_targets()
    print(json.dumps({'suite': 'custody-target', 'environment': 'offline-fixture',
                      'result': 'pass', 'target_cases': sorted(TARGET_CASES),
                      'compiled_cli_refusals': sorted(NATIVE_REFUSALS),
                      'fixture_only_admission_refusals': sorted(FIXTURE_REFUSALS),
                      'hosted_target_admission': 'undelivered',
                      'policy_mutations': rejections, 'roles': check.roles(),
                      'effective_cluster_custody': 'unmeasured'}, sort_keys=True))


def run_suite():
    try:
        main()
        return 0
    except check.Invalid as error:
        print(f'checked-failed: {error}', file=sys.stderr)
        return 1
    except (OSError, ValueError, check.Unavailable) as error:
        print(f'could-not-check: {error}', file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(run_suite())
