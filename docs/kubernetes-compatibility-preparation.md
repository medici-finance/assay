# Prepare the trusted Kubernetes compatibility desks

This is the custodian preparation handoff for desk-containers/31, after the
published-image checkpoint [#4832](https://github.com/medici-finance/assay/pull/4832).
It uses the existing image-owned launcher. It neither installs a cluster nor
authorizes credentials, deployment, activation or qualification. The outstanding
acts remain tracked in [#4772](https://github.com/medici-finance/assay/issues/4772).

## Bind the real inputs first

Record the following in the adopting operator's private, reviewed configuration.
Do not put credentials, kubeconfig contents, private endpoints, instructions or
session history in a public issue or this repository.

| Input | Required source |
|---|---|
| Cell and namespace | Approved target; the supplied component uses namespace `assay-desks` and exact role workload names |
| Five role images | Exact references in the [published candidate record](../containers/k8s/components/compatibility-cellctl/candidate.json); all are Linux AMD64 |
| Agent and version, per role | Approved exact `@anthropic-ai/claude-code` version and preinstalled executable/package manifest; no floating version |
| Model, instructions and work, per role | Custodian-approved model identifier, instruction file, repository revision and toolchain/work inputs |
| Retained volume, per role | Identified cold volume and exclusive custodian preparation access; existing/unknown execution history holds |
| Five custodian groups | Existing authorized group identities for the five RoleBindings; naming a group grants no membership |
| Operator target | Explicit kubeconfig path/context, HTTPS endpoint, TLS server name, CA digest and existing static credential |
| Custody | Authorized terminal, Secret, log, node and downstream-log readers; effective network controls |

The roles are `the-desk`, `intake-desk`, `worker-desk`, `pr-review-desk` and
`verify-desk`. A choice for one role is not approval for another. Missing approved
values remain missing; the example's `REQUIRES_*`/`REPLACE_*` strings are not facts.

## Prepare one role's retained volume

Repeat this section in each role's **authorized preparation context**, using that
role's exact published image and intended retained volume mounted at `/work`.
The workload must remain paused, with no prior or concurrent execution on that
volume. The package runs as UID/GID 999; prepare files for that identity. Creating
or mounting a PVC, obtaining preparation access and provisioning the approved
package/toolchain use the deployment owner's existing procedure. This guide
supplies no cluster-access command or permission to create a preparation Pod.
If that context or the approved inputs do not exist, stop there.

Preinstall the approved package at `/work/.npm-global`, its approved role
instructions at `/work/role.md`, and the approved work repository/toolchain at
`/work/repository`. These are the paths in the operator example. If the approved
layout differs, review the descriptor/profile paths together before preparation.
No install or agent version command runs in the steps below. Checksums measure
the installed inputs; they do not establish that those inputs were approved.

In that preparation context, this snippet asks only for **already approved,
non-secret selections**. It verifies the installed version and creates a new
descriptor exclusively. It will not overwrite a descriptor or existing history.
An initialization identifier is generated once for this cold preparation; it is
a non-secret identity, not a credential or a substitute for preparation.

```sh
python3 - <<'PY'
import hashlib
import json
import os
from pathlib import Path
import re

work = Path('/work')
if os.path.lexists(work / '.assay-compat'):
    raise SystemExit('held: history exists; do not reset or reinitialize it')
with open('/dev/tty', 'r') as terminal_in, open('/dev/tty', 'w') as terminal_out:
    def approved(prompt):
        print(prompt, end='', file=terminal_out, flush=True)
        return terminal_in.readline().strip()
    cell = approved('Approved cell name: ')
    role = approved('Approved role: ')
    version = approved('Approved exact Claude package version: ')
    model = approved('Approved model identifier: ')
roles = {'the-desk', 'intake-desk', 'worker-desk', 'pr-review-desk', 'verify-desk'}
if not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,62}', cell) or role not in roles:
    raise SystemExit('held: invalid cell or role')
if not version or not model or version in {'latest', '*'}:
    raise SystemExit('held: approved exact version and model required')
agent = work / '.npm-global/bin/claude'
manifest = work / '.npm-global/lib/node_modules/@anthropic-ai/claude-code/package.json'
instructions = work / 'role.md'
repository = work / 'repository'
package = json.loads(manifest.read_text())
if package.get('name') != '@anthropic-ai/claude-code' or package.get('version') != version:
    raise SystemExit('held: installed package differs from approved version')
if not agent.is_file() or not os.access(agent, os.X_OK) or not repository.is_dir():
    raise SystemExit('held: installed executable or approved work directory missing')
descriptor = {
    'schema': 'cell-kube-compat-v1', 'cell': cell, 'role': role,
    'initialization_id': os.urandom(32).hex(),
    'agent': str(agent), 'agent_sha256': hashlib.sha256(agent.read_bytes()).hexdigest(),
    'agent_version': version, 'version_manifest': str(manifest), 'model': model,
    'instructions': str(instructions),
    'instructions_sha256': hashlib.sha256(instructions.read_bytes()).hexdigest(),
    'workdir': str(repository),
    'initial_prompt': 'Read /opt/assay/plugin/skills/' + role +
        '/SKILL.md and run that role under its existing STOP, claim and human gates.'
}
os.umask(0o077)
with (work / 'compat-launch.json').open('x') as target:
    json.dump(descriptor, target, ensure_ascii=False, indent=2)
    target.write('\n')
print('Descriptor written; cold history is not prepared yet.')
PY
```

Review the descriptor against the approved inputs before continuing. It has no
`launch_id` member: the launcher computes that value as the SHA-256 of its
canonical descriptor encoding. Adding a `launch_id` field makes the descriptor
invalid. Hashing the pretty-printed file with `sha256sum` gives the wrong value.

Run the existing launcher once, in the same preparation context:

```sh
/usr/local/bin/desk-session --prepare --profile /work/compat-launch.json --work /work
```

Expected: exit 0 and
`checked-clean: cold compatibility history prepared; no agent invoked`.
It writes `/work/.assay-compat/state.json` with state `prepared`, empty `pod_uid`
and `agent_pid` zero. It does not print the launch identity. An existing history,
interrupted write, missing input or any failure is a hold: retain the files and
diagnostic for the custodian. Never delete history, generate another identity or
retry with a fresh volume to erase an unknown predecessor.

Read back through the image-owned launcher's existing validators, without starting
an agent. Run this immediately after preparation, before any boot:

```sh
python3 - <<'PY'
import importlib.util
import json
from pathlib import Path

spec = importlib.util.spec_from_file_location('desk_session', '/usr/local/lib/assay/desk-session.py')
launcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(launcher)
history = Path('/work/.assay-compat')
if any((history / name).exists() for name in ['uncommitted', '.state-next']):
    raise SystemExit('held: interrupted history write')
descriptor, launch_id = launcher.profile(Path('/work/compat-launch.json'))
state = launcher.read(history / 'state.json')
launcher.state(state, descriptor, launch_id)
if state['state'] != 'prepared':
    raise SystemExit('held: this is not an unused cold preparation')
print(json.dumps({'descriptor': descriptor, 'launch_id': launch_id}, indent=2))
PY
```

Retain this non-secret handoff output privately with the volume identity, role,
preparer, time, approved-input references and exact image reference. It is input
for the operator configuration, not independent execution or acceptance evidence.
Do not copy signing/model credentials or the full retained volume into a handoff.

## Carry the prepared values into the adopting configuration

Use a private reviewed copy of the existing component and operator example,
preserving their relative base/patch layout. Keep the published record and five
image references unchanged. For each role, copy **all descriptor values**, not just
the two identities, into the corresponding operator-profile role object.

| Prepared source | Operator-profile destination | Workload destination |
|---|---|---|
| Handoff `launch_id` | `roles[role].launch_id` | `ASSAY_COMPAT_LAUNCH_ID` |
| Descriptor `initialization_id` | `roles[role].initialization_id` | `ASSAY_COMPAT_INIT_ID` |
| Descriptor agent/version/model/instructions/workdir/cell/prompt values | Same named fields under `roles[role]` | Read from the unchanged `/work/compat-launch.json` on that role's volume |
| Published role image reference | `roles[role].image` | Matching container `image` |

`schema` and `role` are descriptor members, not extra members of a profile role
object. Keep its existing `statefulset`, `pod`, `container` and image bindings.
Set the top-level profile `cell` to the same cell as every descriptor. Preserve
zero replicas, Linux AMD64 selectors, Pod-UID downward field, credential entrypoint,
readiness probe, separate Secrets, token-automount isolation and existing network
and custody controls. Do not put the two identities into the image or change the
launcher to accept a different hash. Changed descriptor inputs invalidate the
binding; this guide provides no history-reset or re-preparation procedure.

Replace each RoleBinding's one unresolved group subject with the corresponding
**existing authorized group**, preserving the exact-name Role/rules. This file
does not create group membership or authorize applying the grant. Complete the
top-level explicit operator/TLS and custody fields from the approved target.
Keep credential material only in the custodian-owned kubeconfig/Secret stores;
the profile contains their references, never their contents. No exec/auth-provider,
ambient context, proxy, insecure TLS or managed-isolation claim is introduced.

## Offline checks and handoff

In the adopting source copy, from its repository root, run:

```sh
sh containers/k8s/check.sh --compatibility-cellctl --self-test
```

The checker reads **that checkout's** component, not the operator JSON file or a
remote deployment. Required result: exit 0 and
`checked-clean: compatibility cellctl profile`, after all 11 hostile fixtures are
rejected. The unchanged public template still exits 1 at
`checked-failed: prepared launch identity missing`; that is expected until real
values are bound in the copy being checked. Fixture success alone clears nothing.

For an existing, appropriately configured operator cell, `cellctl check` validates
its explicitly selected profile offline. The custodian can supply its approved
cell name interactively, without an invented example target:

```sh
printf 'Approved existing operator cell name: '
read -r compat_cell
test -n "$compat_cell" && cellctl check "$compat_cell"
```

The cell must already select `CELL_KIND=k8s` and an absolute
`CELL_KUBERNETES_CONFIG` pointing to that reviewed profile. This check does not
qualify the credential or effective live permissions. Do not run up, attach,
status, down or apply manifests as part of this offline preparation handoff.

Return the actual prepared bindings and offline results through the appropriate
private custody channel. The worker can then reconcile the real component/profile
bindings and final delivery with /31's requirements. The independent verifier
owns Verify rows and lifecycle; /15's verification is still a separate dependency.
The final implementation merge and its first parent must be identified for the
consumer check and /32 handoff. This documentation change is not that delivery.
Effective permissions, network/transcript custody, useful model work, deployment
and target acceptance remain the separately authorized qualification in /32.
