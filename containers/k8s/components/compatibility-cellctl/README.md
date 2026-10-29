# Trusted compatibility cellctl component

This opt-in Kustomize root reuses the historical five-role base while setting every
workload to zero replicas. It adds exact-name role-custodian reads/scale/attach only.
No subject value or candidate image is selected by this source template. Replace
all `REQUIRES_*` values in an adopter-owned reviewed copy; do not apply this template.
The existing base and its v1.0.52 release record stay unchanged.

`candidate.json` deliberately records `unbuilt`. No digest is invented. The actual
profile checker reports could-not-check until an approved build publishes the new
role-image manifest digests and their unchanged release record. A local image config
ID, source hash, fixture digest, old shell image or dry-run release cannot supply it.

The image must contain `/usr/local/bin/desk-session` and its image-owned Python
launcher. The existing credential preflight runs first. Preinstall the approved
pinned Claude-compatible package, role instructions and work inputs on each retained
volume, then explicitly prepare cold history with `desk-session --prepare` before
first boot. Normal boot never installs packages or creates missing history.

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
