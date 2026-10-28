---
brief: assay:assay:desk-containers:32
title: Qualify the compatibility Kubernetes desk cell
why: Operators need evidence that an interactive desk remains useful when its terminal disconnects and can be stopped without starting a second agent or losing files. Packaging checks cannot establish those behaviours or the deployed profile's credential custody.
wave: 9
depends: ["desk-containers/31"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: The compatibility profile carries each role's signing and model credentials, so terminal and transcript access are key custody. The human confirms the exact disposable qualification target, authorized custodians and evidence before separately scoped adoption; offline PASS supplies no approval.
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
authored: 2026-10-28 by compatibility Kubernetes brief-authoring session
sources:
  - docs/streams/desk-containers/README.md#small-interactive-kubernetes-compatibility-milestone
  - docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md
  - docs/streams/desk-containers/brief-06-k8s-manifests.md
  - docs/streams/desk-containers/brief-02-secret-injection-contract.md
  - "desk-containers/31 version 1 @ bf056f8af42aaa84bc3da63deba29dcbb2ad28c9 (actual paired authoring baseline and compatibility interface)"
  - "freshness-checked 2026-10-28 @ 0a4c08b83b1c3a743d6bf1ce7b1f41b331d17125 (worker-App authenticated fetch; package, entrypoint, CLI launch/up paths and qualification owners inspected)"
exec-tier: strong
exec-tier-why: Terminal, lifecycle, storage and effective custody proof spans the real CLI, Kubernetes transport and independently observed execution.
domain: complicated
consumers:
  - "containers/k8s/compatibility/qualify.sh: follow-up desk-containers/32 (this brief; future qualification entrypoint)"
  - "containers/k8s/compatibility/testdata/qualification/: follow-up desk-containers/32 (this brief; disposable qualification fixtures)"
  - "tools/desk/cmd/cellctl/: follow-up desk-containers/32 (this brief; production-entrypoint qualification tests only)"
  - "docs/kubernetes-compatibility-qualification.md: follow-up desk-containers/32 (this brief; future qualification and operator runsheet)"
  - "containers/k8s/README.md: follow-up desk-containers/32 (this brief; small-profile evidence and limits)"
  - "docs/docker.md: follow-up desk-containers/32 (this brief; qualified compatibility workflow)"
  - "docs/cellctl.md: follow-up desk-containers/32 (this brief; qualified operations and reconnect semantics)"
  - "managed-profile qualification: out-of-scope (desk-containers/29 retains its own scope, dependencies and evidence)"
outcome: none
---

# Brief 32 — Qualify the compatibility Kubernetes desk cell

## Context
files: `containers/k8s/compatibility/qualify.sh` (planned), `containers/k8s/compatibility/testdata/qualification/` (planned), `tools/desk/cmd/cellctl/kube_compat_qualification_test.go` (planned), `docs/kubernetes-compatibility-qualification.md` (planned), `containers/k8s/README.md`, `docs/docker.md`, `docs/cellctl.md`, `changelog/kubernetes-compatibility-qualification.md` (planned).

facts:
- At source 0a4c08b83, the compatibility package has five separate role StatefulSets, retained PVCs, pinned role images, distinct credential references and interactive stdin/TTY. It supplies no operator workload or RBAC grant.
- The shared entrypoint checks runtime signing/model credentials, then executes its supplied command; its default shell does not start an agent. The smaller integration owns the actual pinned agent-launch command and compatibility backend; this brief qualifies it.
- Existing cellctl launch/up uses local process/cockpit mechanisms. The compatibility integration binds its Kubernetes API/stream path explicitly; this brief creates no second launcher.
- The integration's proposed `CELL_KUBERNETES_CONFIG` selects `cell-kube-compat-v1`; its receipt at `run/kubecompat/<role>.json` binds the config digest, StatefulSet UID, Pod UID, containerID and restartCount. These are implementation deliverables, not APIs already present at the inspected source.
- Compatibility terminal and session-log access confer transitive key custody. Existing package rules restrict terminal/log APIs, node files, collectors, downstream stores and readers to each role's authorized custodians. Managed operator isolation cannot be claimed for it.
- desk-containers/29 retains the broader managed supervision, CNPG and recovery proof. The smaller outcome claims controlled stop/start and holds replacement when the previous execution is unknown.

layering: One qualification harness drives the compiled cellctl commands and the same compatibility configuration, transport and lifecycle implementation as the operator. Injected offline API/stream fixtures and the explicitly authorized disposable target are separate evidence classes. The harness creates no runtime controller, queue, credential broker or acceptance authority; Tasks 1–5 and Verify 1–4 exercise the seam.

design-fit:
  owner: existing cellctl compatibility backend and profile; this brief owns qualification artifacts only
  contract: none — consumes the compatibility operator contract rather than defining another runtime meaning or semantic-owner row
  retires: []
  weight: verbs +0 shipped runtime, flags +6 qualification-only (offline, suite, self-test, kubeconfig, context, allow-cluster-queries), refusals +0 runtime policy, rule-text lines +0 runtime
  why-add: Disconnect behaviour, controlled stop/start and effective transcript custody cannot be proven by rendering; a bounded harness qualifies the operator path without importing the managed-runtime migration.

single-point-of-failure: a fake API/terminal fixture can misdescribe effective behaviour — behind it, an independent verifier exercises the actual disposable profile, and the cluster's existing authorization/custody controls are tested with the upper CLI bypassed. Neither evidence class substitutes for the other.

## Read first

- [Small compatibility milestone](README.md#small-interactive-kubernetes-compatibility-milestone), the controlling source and proposed criteria mapping.
- [Compatibility integration and its configuration, execution-identity and observed-stop contract](brief-31-operate-trusted-compatibility-kubernetes-desks-through-cellctl.md).
- [Compatibility package and limits](../../../containers/k8s/README.md).
- [Credential contract](../../../containers/secrets.md) and its current human acceptance in the owning brief.
- [Broader target](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md), for preserved compatibility/managed boundaries.
- [Library-first policy](../../library-first.md), [Go engineering](../../go-engineering.md), and [implementation pickup](../../../plugins/assay/references/implementation-pickup.md).

## Implementation pickup

Manual bootstrap applies: at pickup and after a material scope change, record this
brief/version, the controlling README scope revision, the actual integration implementation
and image/profile versions, credential-contract acceptance, relevant library/Go guidelines
and their source SHAs. Inspect the importable production seam from the desk module and
record affected consumers, coverage/unknowns and a reasoned continue/adapt/hold disposition.
At authoring, the existing shell-only package and local-launch path establish the need for
the integration; they establish no qualification result. Do not begin executable
qualification against a planned or incompatible integration API. Source/spec milestone
support remains proposed, so the current supported brief-v2 identifier is preserved.

## Human decision
At pickup, identify the run-owned disposable environment, exact server identity and
credential configuration, five desk images/agent versions, synthetic work inputs,
authorized role custodians and accepted retention. Terminal and transcript readers can
reach credentials inside these trusted compatibility desks. Preserve that existing
boundary; no untrusted-agent isolation claim is made. Production, ambient contexts and
automatic failover are excluded.

Options:
1. **Authorize the identified disposable qualification (recommended)** — run only that target and retain independent interaction, custody and controlled-stop evidence; later scoped adoption remains separate.
2. **Remain offline** — implement and check fixtures, leaving effective Kubernetes execution and adoption explicitly unqualified.

Default if no answer: none — blocks until answered

## Ground rules
- Branch and draft PR only. Authoring and ordinary implementation checks contact no cluster, production endpoint or real model. Offline fixture mode is the default; never inspect ambient kubeconfig.
- The real-profile row is a separate human-authorized act against an identified run-owned disposable target. An implementation worker never runs it merely because the harness exists.
- Preserve credential acceptance, per-role key/terminal/log custody, stop controls and image provenance. Missing/unknown evidence is could-not-check and holds qualification, never a zero or PASS.
- No untrusted-candidate isolation, automatic partition failover, database recovery, zero-downtime restart or conversation/process-persistence promise is introduced.
- Independent verification owns later Evidence and lifecycle. These authored prospective checks are not implementation evidence.

## Task
1. Deliver one profile-owned harness and fixtures through the actual compiled CLI and
   compatibility backend. Reuse the integration's configuration loader, UID validation,
   lifecycle result and API/stream implementation. Extend its injectable test seam;
   never reproduce lifecycle logic in the harness or attach a dummy shell beside the
   production path. Offline mode uses deterministic fake transport and synthetic
   agent/model adapters only. Record exact source SHA, binary hash, image/profile/config
   digests, role/harness/model binding, tool versions and invocation identities. Bind the
   actual integration candidate image and its agent entrypoint; the package's older shell
   images are provenance for packaging only, never evidence of the new launch path. Missing
   executables, named tests or tools must fail/report could-not-check, never silently skip.
   Keep fixtures and results separate from the future managed-profile qualification.
2. Qualify all five configured roles using their actual pinned agent-launch commands.
   Observe terminal I/O and resize, detach without EOF/signal, reconnect, repeated up,
   and loss/recreation of the operator client. Transport-only disconnect retains the
   same desk process and containerID/restartCount; attach/reconnect must produce zero additional agent starts or
   model requests. Run a bounded synthetic useful-work task through each role/harness
   binding and retain the expected file/result and exact request count. A Running Pod
   or interactive shell is insufficient. Plant duplicate-start and model-call-on-attach
   defects and require the interaction assertions to detect them. Exercise the integration's
   image-owned boot launcher directly with the cellctl caller bypassed: reused Pod UID/new
   container identity, an inherited dirty volume, unclean agent exit and corrupt or missing
   prior-required launch stamp all hold with zero agent/model launches. A missing stamp
   alone is not proof of fresh installation. Only the profile's explicit admitted first
   boot or a recorded clean controlled stop can make its next distinct execution eligible.
3. Qualify controlled stop and subsequent explicit start through the integration's exact
   execution-identity/observed-stop contract. Complete or reconcile the bounded work
   before stop; preserve existing claim and stop meanings. Read the retained test file
   after the new execution starts. Report file retention, same-process continuity,
   conversation support and work/effect acceptance separately: a PVC supplies only the
   file observation. Without a qualified harness conversation-resume contract, label
   that continuity unsupported. Exercise response loss, changed Pod UID, old/missing
   receipts, an unreachable previous execution, and an automatic container restart under
   the same Pod UID. The last case changes execution identity despite unchanged Pod/PVC
   names: old streams and old receipts must not reattach or admit an owner for it. Neither
   a single replica nor restartPolicy Always proves a single authorized agent launch.
   An unapproved restart or unknown execution lineage holds qualification. These hold replacement and retain
   diagnostic evidence. Stream closure, accepted scale/delete, lease expiry and absence
   from an incomplete observation never establish terminated execution. A human must
   reconcile an unknown old execution before a new explicit start; no force-delete or
   automatic failover is part of the profile. Plant a detached/reparented agent descendant
   that survives the immediate child: it cannot produce a clean-stop receipt or permit
   replacement until the old execution group is confirmed stopped.
4. Supply a disposable-target mode and exact operator runsheet. Require explicit
   kubeconfig/context, run-ownership receipt, expected API endpoint/TLS identity,
   namespace and compatibility config. Resolve/compare them locally before network
   contact, pin that identity on the connection and print the target before first
   contact. Wrong profile/target/context/receipt or missing opt-in makes zero operation
   calls. Installation and credential provisioning belong to the already-authorized
   operator. The harness cannot select arbitrary workloads, a more privileged service
   account or broader egress/RBAC. Real tests use only approved synthetic forge/model
   fixtures, with distinct synthetic credential material per role, not production
   credentials/data. Current credential-contract acceptance is required before the real
   run and final qualification; its absence does not prevent offline fixture development.
5. Verify the effective existing custody boundary with the upper CLI bypassed.
   Unauthorized identities cannot attach, exec, debug or read another role's logs;
   custodians can use only their admitted terminal scope. Test denial of database,
   executor and foreign-cell access and of Secret/RBAC/service-account mutation beyond
   the approved profile. Preserve disabled agent service-account-token automount, no
   terminal Service/Ingress, and only declared forge/model/DNS egress through the real
   CNI. Trace a synthetic transcript marker through container logs, node files and actual
   shipping. Prove namespace exclusion or equivalent custody/retention at every
   collector, downstream store, export/backup and reader. Plant broader log-reader or
   shared-log-destination defects and require rejection. Missing authorization/collection
   evidence holds qualification. Store qualification transcripts only within the same
   custody boundary; scrub credential values from retained artifacts.
6. Produce a pinned result manifest and short runsheet for this exact small outcome.
   Retain failed attempts, provenance and limits. Cover preflight/start/attach/detach,
   controlled stop/start, rollback to the earlier profile with all role replicas stopped,
   and custody-preserving cleanup retaining PVCs and evidence. No namespace/PVC deletion,
   key revocation, production cutover or background monitor is performed. Link the result
   from existing package/CLI guides. Passing this brief neither advances managed-profile
   qualification nor retires its obligations; operator-approved scoped adoption is separate.

## Interface and completion contract

The integration owns runtime config, effects and execution-identity meanings. This
harness selects fixtures/suites and records observations; it adds no journal or second
meaning of stopped execution. Results distinguish profile/image/source/tool versions,
per-role useful work and interactions, retained files, process continuity, conversation
support, work/effect acceptance, effective authorization/logging/network evidence,
attempts and unknowns. Never retain key material, model tokens or kubeconfig contents.

Offline PASS cannot complete effective qualification. The real-profile row needs complete
independent evidence and the human's exact target/profile authorization. Proposed milestone
membership stays in the controlling README until negotiated readers ship; no new operational
milestone YAML, brief `milestones:` field or acceptance authority is introduced.

## Verify

Scripts/tests below are implementation deliverables, not authoring-time execution claims.
Offline rows drive the actual compiled CLI with injected fake transport. The real-profile
row drives the same production backend. No ordinary fixture contacts a production model,
forge or Kubernetes endpoint.

For row 6 the independent verifier obtains the implementation's actual merged delivery
and first parent, sets `ASSAY_DELIVERY_SHA` and `ASSAY_DELIVERY_BASE_SHA`, and makes both
immutable commits available. They never default to HEAD, origin/main or this authoring
change. Retain exact corroborated/disproved/unchecked counts; an unchecked required
consumer remains held.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `sh containers/k8s/compatibility/qualify.sh --offline --suite interaction` | exit 0; five real CLI role paths perform useful synthetic work and I/O/resize; detach, reconnect, repeated up and operator-client loss preserve the current process with zero attach-triggered agent/model requests; expected files/results and exact invocation counts retained |
| 2 | check +flow +mutation | `sh containers/k8s/compatibility/qualify.sh --offline --suite controlled-restart --self-test` | exit 0 only when controlled stop/start retains files and reports new execution identity honestly; planted reconnect-launch, stop-on-stream-close, absent receipt, old UID, same-UID new container/restartCount, dirty/unclean/missing-stamp boot, live detached/reparented descendant, stale stream, wrong candidate image and unknown-stop replacement defects fail their specific assertions, including the image boot gate with cellctl bypassed; no unsupported conversation claim |
| 3 | check +flow +mutation | `sh containers/k8s/compatibility/qualify.sh --offline --suite custody-target --self-test` | exit 0; opt-in/endpoint/TLS/context/namespace/profile/receipt mismatches make zero API calls; planted broader terminal/log grant, shared credential material, transcript collector, egress and privilege mutations are rejected, including checks bypassing upper CLI validation |
| 4 | gate:human +flow +mutation +dereference | `CELL_KUBERNETES_CONFIG="$ASSAY_COMPATIBILITY_CONFIG" sh containers/k8s/compatibility/qualify.sh --kubeconfig "$ASSAY_DISPOSABLE_KUBECONFIG" --context "$ASSAY_DISPOSABLE_CONTEXT" --allow-cluster-queries --suite real-profile` | exit 0 only for the authorized run-owned target at accepted compatibility config/image versions, with independent five-role useful work/I/O/resize/reconnect, controlled stop/start, retained files, unknown-old-execution hold, denied unauthorized terminals/logs, actual node/downstream custody and effective network policy; missing acceptance/target/evidence holds; no ambient/production target |
| 5 | check +neighbour | `sh containers/k8s/check.sh --base --self-test && sh containers/k8s/check.sh --source-secrets --self-test` | exit 0; existing package and its hostile/secret controls remain effective; managed-profile assets and historical Evidence unchanged |
| 6 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/32 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the implementation's immutable delivery and first parent; record exact counts, 0 disproved, and separately resolve every unchecked required consumer |

## Definition of Done

Offline suites and planted controls are independently witnessed. The real-profile row has
explicit target authorization and complete independent evidence. The retained result
states the supported small profile and limits; the operator can follow its exact guide
without a second launcher or implicit credential grant. Current credential acceptance
and human sign-off remain binding. Offline-only delivery is implemented with effective
operation unqualified; successful rendering, container exit or a PVC file is insufficient.

## Pre-mortem

An attached shell is mistaken for an agent: row 1. Reconnect starts another agent or model
request: rows 1–2. PVC retention is mistaken for conversation/process recovery: row 2.
Delete acknowledgement is mistaken for terminated execution: rows 2 and 4. Mocks hide
effective transcript/custody/network leakage: row 4 with upper caller bypassed. A harness
contacts an ambient target: row 3's zero-contact negative cases.

## Evidence
<!-- An independent non-implementer records exact source/profile/image/tool versions,
commands, witnesses, result artifacts, attempts and unknowns. Prospective scripts and
authoring validation supply no implementation or operational evidence. -->

## Review
Human gate derives from credential/transcript custody. Review the small profile against
the controlling scope and unchanged credential/stop boundaries. Implementation and
independent evidence remain necessary; a model cannot authorize the real target, widen
custody or mark this done. Managed qualification retains its separate obligations.
