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
volume. Run descriptor creation, cold preparation and readback as the workload's
UID **999** and primary GID **999**, not as root or a different preparer. The
descriptor and history are owner-only under umask 077; a different owner leaves
them unreadable at boot. Creating
or mounting a PVC, obtaining preparation access and provisioning the approved
package/toolchain use the deployment owner's existing procedure. This guide
supplies no cluster-access command or permission to create a preparation Pod.
If that context or the approved inputs do not exist, stop there.

Use the approved package at `/work/.npm-global` and approved work
repository/toolchain at `/work/repository`. The helper can use the image's baked
role instructions or separately reviewed `/work/role.md`; carry its actual paths
into the operator profile. If the approved layout differs, review those paths
together before preparation. Checksums measure the installed inputs; they do not
establish that those inputs were approved.

Use the bounded [preparation helper](../containers/k8s/compatibility/prepare.py)
in that context. Copy or mount this reviewed source file read-only at
`/opt/assay-preparation/prepare.py`; it is not baked into the existing v1.0.65
images, and those image pins do not need changing. It calls the image-owned
launcher at `/usr/local/lib/assay/desk-session.py`, rather than implementing a
second history or hash format. It starts no agent, installs no package and makes
no cluster or model call. The existing `assay-desks` namespace and five paused
StatefulSets remain the deployment layout. Start with Claude; this helper does
not claim a qualified Codex argv/credential profile.

The image's existing installer can provision the exact Claude package version
under `/work/.npm-global` in the already authorized preparation context. For
example, this selects version **2.1.228**, rather than its floating `latest`
default. It is an explicit network-reaching operator act; it is not part of the
helper or ordinary boot:

```sh
printf '%s\n' 'Installing @anthropic-ai/claude-code@2.1.228 from https://registry.npmjs.org'
NPM_CONFIG_REGISTRY=https://registry.npmjs.org \
  AGENT_CLI_PACKAGE=@anthropic-ai/claude-code AGENT_CLI_VERSION=2.1.228 install-agent-cli
```

The work repository/toolchain must still be preinstalled on the role's retained
volume. By default the helper uses the image's baked role SKILL.md as its pinned
instructions; `--instructions /work/role.md` selects separately reviewed appended
instructions instead. Package and model selections must match the approved
profile; measuring installed bytes does not approve them.

For a worker desk using Claude Sonnet 5.5, first inspect the read-only plan:

```sh
python3 /opt/assay-preparation/prepare.py --cell assay --role worker-desk \
  --agent-version 2.1.228 --model claude-sonnet-5-5
```

The explicit model ID is from the [model catalogue](https://platform.claude.com/docs/en/models/overview).
It is an example selection for the initial Claude setup, not evidence that a
model credential works or that a role's work has been accepted. Repeat for each
of the five roles using its approved cell/model and retained volume.

Only in that same authorized cold preparation context, run:

```sh
python3 /opt/assay-preparation/prepare.py --cell assay --role worker-desk \
  --agent-version 2.1.228 --model claude-sonnet-5-5 --prepare
```

The helper checks UID/GID 999, installed package/version, executable, instructions
and work directory, then exclusively writes `/work/compat-launch.json` and calls
the existing launcher's `--prepare`. It generates a fresh non-secret
initialization identity and reads the canonical launch ID back through the
image-owned validators. No manual hash or ID construction is needed.

Success is exit 0 and JSON containing `descriptor`, `launch_id` and
`workload_env` with the two actual binding values. Its retained state is
`prepared`, with empty `pod_uid` and zero `agent_pid`. The descriptor itself has
no `launch_id` member: that value is its canonical hash, not the checksum of a
pretty-printed file. The helper refuses existing descriptors, any existing or
unknown history, floating package/model aliases and concurrent preparation. A
failed or interrupted preparation remains held, with its files retained. Never
delete history, reset it or switch volumes to erase an unknown predecessor.

If preparation succeeded but its output was lost, run the same selection with
`--readback` instead of `--prepare`. It revalidates the installed inputs and
existing unused prepared state and prints the same bindings without writing,
installing or invoking a child. Changed inputs, foreign/corrupt state or a role
that has already booted remain held; readback never reinitializes a volume.

Retain the helper's non-secret JSON privately with volume identity, role,
preparer, time, approved-input references and exact image reference. It is input
for operator configuration, not independent execution or acceptance evidence.
Do not copy credentials or the retained volume into a handoff.

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

Use a Git checkout for the adopting source copy, not an extracted source archive.
The existing checker dereferences the candidate's source with `git show`; the
published source object `fc0da35c571c9c01e23b1499b594f275fa2c2c61` and its launcher
path must be available locally. A shallow checkout that lacks that object cannot
complete the check. Before checking, confirm it without a network fetch:

```sh
git cat-file -e 'fc0da35c571c9c01e23b1499b594f275fa2c2c61^{commit}'
git show fc0da35c571c9c01e23b1499b594f275fa2c2c61:containers/k8s/compatibility/desk-session.py > /dev/null
```

The preparation check also requires `kustomize` **v5.8.3**, Python 3, and the
existing Go toolchain/module cache for its YAML adapter. `check.sh` disables
network/module/toolchain downloads. Missing tools, cached dependencies or source
objects are **could-not-check**, not a clean profile; provision them through the
owner's approved tooling procedure before retrying.

From that checkout's repository root, run:

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
consumer check and /32 handoff. This preparation checkpoint is not that delivery.
Effective permissions, network/transcript custody, useful model work, deployment
and target acceptance remain the separately authorized qualification in /32.
