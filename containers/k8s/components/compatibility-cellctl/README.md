# Trusted compatibility cellctl component

This opt-in Kustomize root reuses the historical five-role base while setting every
workload to zero replicas. It adds exact-name role-custodian reads/scale/attach only.
The five candidate image pins are published; operator preparation and custody values
remain unresolved. Replace the remaining `REQUIRES_*` values in an adopter-owned
reviewed copy; do not apply this template. The existing base and its v1.0.52 release
record stay unchanged. The inherited node selectors restrict all five workloads to
`linux/amd64`; these role images do not provide ARM64 support.

`candidate.json` binds the five role manifest references from the byte-identical
`candidate-release-images.json` published in [v1.0.65](https://github.com/medici-finance/assay/releases/tag/v1.0.65).
The [successful producer run](https://github.com/medici-finance/assay/actions/runs/40884178309)
built source `fc0da35c571c9c01e23b1499b594f275fa2c2c61`.
The release record SHA-256 is
`6f0a71fd6a9887ff2810e1f047b3df2fc3031400f76393bd20f7eeb3b2914b4e`;
its published `checksums.txt` SHA-256 is
`36ae76a69952c5c8fe69f3694be840c560ac6c8a4e3ef0f092caf38ae7d82caf`.
Both artifacts' attestations bind the release workflow and that source. The source
packages the bounded launcher through desk-base into each role image, preserving
its credential entrypoint. These are published manifest references, not local
Docker config IDs, source hashes or synthetic fixture pins. The multi-platform
verify-runner entry in the unchanged release record is a separate image.

The image must contain `/usr/local/bin/desk-session` and its image-owned Python
launcher. The existing credential preflight runs first. Preinstall the approved
pinned Claude-compatible package and work inputs on each retained volume. Use the
role instructions baked into `/opt/assay/plugin/skills/<role>/SKILL.md`, or explicitly
select approved custom instructions. Then prepare cold history through the bounded
helper described below before first boot. Normal boot never installs packages or creates missing history.

The operator's profile and mounted launch descriptor must have the same canonical
launch digest. `cellctl check` is offline and makes no API call. The checked profile
selects only a named API context with static credentials and exact TLS identity.
Operator identity, all five role-group memberships, effective network and terminal,
log, node and downstream-log custody require separate human qualification. The RBAC
subjects here are unresolved examples, not new credential grants. Node or collector
log access is not constrained by this Role or by NetworkPolicy.

The agent owns its ordinary STOP/claim/work controls. The launcher does not grant
work authority. Down requires explicit custodian confirmation of idle and reconciled
claims/effects, stops the whole private process namespace, and waits for exact native
container termination plus the clean lower-layer acknowledgement. Dirty, absent or
corrupt history, same-Pod restart, unmeasured old work and lost acknowledgement stay
held. There is no automatic failover, force deletion, history reset or claim release.

Attach is presentation only. Disconnect/reconnect/resize starts no agent. Retained
files, conversation continuity and process continuity are separate observations;
PVC retention supplies only the filesystem part.

## Remaining preparation and qualification

Publication supplies the image pins, not the operator's launch facts. Each role
still needs its approved pinned agent/version/model and work inputs on the retained
volume, baked or explicitly selected custom role instructions, a genuine
cold-preparation identity, and a matching launch descriptor. The launcher's existing `desk-session --prepare` command creates the
initial history only when none exists; it does not reset history. Its prepared
state records `launch_id` (the SHA-256 of the canonical launch descriptor) and the
descriptor's `initialization_id`. The descriptor has no `launch_id` member and
`--prepare` does not print that hash; read it from `.assay-compat/state.json`.
Use those same values in the operator profile and the workload's
`ASSAY_COMPAT_LAUNCH_ID` and `ASSAY_COMPAT_INIT_ID`. Never substitute fixture hashes.

Follow the [custodian preparation guide](../../../../docs/kubernetes-compatibility-preparation.md)
for the approved-input inventory, bounded Claude preparation helper and per-role
profile/workload mapping. The helper generates both binding identities and reads
them back through the existing image-owned launcher; no manual hashes are needed. The guide starts inside an already
authorized preparation context; it supplies no cluster or credential grant.

A custodian must also supply each existing role-custodian group in `rbac.yaml` and
qualify the explicit operator/TLS endpoint, static credential, effective access,
network and transcript custody. Choosing group names or possessing image pins
never grants that custody. The example profile keeps these unresolved values
visible and is not deployable.

`sh containers/k8s/check.sh --compatibility-cellctl --self-test` first checks
synthetic hostile fixtures, then checks the actual rendered component. With the
remaining launch/init placeholders it returns `checked-failed: prepared launch
identity missing`; group placeholders are another unresolved actual-profile gate.
Fixture success is not the `checked-clean: compatibility cellctl profile` result.
Do not weaken either check or replace preparation values merely to make it pass.

This pin checkpoint follows source checkpoint [#4770](https://github.com/medici-finance/assay/pull/4770).
The remaining operator acts are tracked in [#4772](https://github.com/medici-finance/assay/issues/4772).
Brief 31 stays blocked until its actual profile and required acceptance are complete.
Neither this checkpoint nor the release source supplies the final delivery merge
and first parent required by the independent qualification handoff. Deployment,
credential grants, activation, live model usefulness and adoption remain held.
