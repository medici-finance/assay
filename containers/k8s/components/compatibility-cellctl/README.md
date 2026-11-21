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

Use the [custodian preparation guide](../../../../docs/kubernetes-compatibility-preparation.md)
in each role’s authorized cold preparation context. Supply the pinned agent, model,
work directory and instructions approved for that role. Run the guide’s preparation
command in its default read-only mode to check the inputs, then add `--prepare`
to initialize the unused volume. Both calls use `cellctl compat-prepare` through
the guide’s companion binary and Python delegate.

If the preparation output was lost before the role first booted, repeat the same
selection with `--readback`. It validates the unused prepared state and returns
the same JSON without writing. Copy `launch_id` and `descriptor.initialization_id`
into the operator profile, and use the returned `workload_env` bindings for
`ASSAY_COMPAT_LAUNCH_ID` and `ASSAY_COMPAT_INIT_ID`. Changed inputs or a previously
used volume hold readback.

A custodian must also supply each existing role-custodian group in `rbac.yaml` and
qualify the explicit operator/TLS endpoint, static credential, effective access,
network and transcript custody. Choosing group names or possessing image pins
never grants that custody. The example profile keeps these unresolved values
visible and is not deployable.

`sh containers/k8s/check.sh --compatibility-cellctl --self-test` first checks
synthetic hostile fixtures, then checks the actual rendered component. With the
remaining launch/init placeholders it names the first affected role and environment
variable. The group placeholders also require custodian input.
Fixture success is not the `checked-clean: compatibility cellctl profile` result.
Do not weaken either check or replace preparation values merely to make it pass.

This pin checkpoint follows source checkpoint [#4770](https://github.com/medici-finance/assay/pull/4770).
The remaining operator acts are tracked in [#4772](https://github.com/medici-finance/assay/issues/4772).
Brief 31 stays blocked until its actual profile and required acceptance are complete.
Neither this checkpoint nor the release source supplies the final delivery merge
and first parent required by the independent qualification handoff. Deployment,
credential grants, activation, live model usefulness and adoption remain held.

### Offline interaction suite

From a full source checkout with the candidate's recorded commit available, set
`ASSAY_TEST_DOCKER_HOST` to an explicit local Unix socket for a disposable test
daemon. The daemon needs a cached `python:3.13-slim` or a Python 3 Linux image
selected by its immutable local `sha256:...` ID in `ASSAY_TEST_FIXTURE_IMAGE`.
The runner resolves the local image ID, disables image pulls and container
networking, and mounts only a temporary fixture directory. The Go toolchain and
the desk module dependencies must be available offline.

Run `sh containers/k8s/compatibility/qualify.sh --offline --suite interaction`.
The compiled CLI talks to a synthetic Kubernetes API. Its terminal reaches the
candidate source's actual PID-1 launcher, running a pinned synthetic Claude
adapter in the fixture runtime. Each of the five role records identifies the
published candidate image, executable and runtime digests, terminal execution,
request count and retained task result. This exercises the offline interaction
contract. Running the published images on a Kubernetes target belongs to the
separately authorized real-profile qualification.
