---
brief: assay:assay:desk-containers:29
title: Qualify the Kubernetes desk cell and operator recovery end to end
why: Only a complete five-role and recovery proof can establish that the new Kubernetes profile preserves authority, useful work and the existing efficiency gains.
wave: 11
depends: ["desk-containers/28", "desk-containers/27", "desk-containers/18", "desk-containers/20"]
unblocks: ["cell-api/07", "cell-api/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
issues: []
schema: brief-v2
version: 5
authored: 2026-10-20 by Assay architecture authoring session
sources:
  - https://github.com/medici-finance/assay/issues/3669
  - docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md
  - freshness-checked 2026-10-20 @ d8687cab6 (fetched origin/main; code and open PRs inspected)

exec-tier: strong
exec-tier-why: Cross-process ownership, custody and recovery require design judgment.
domain: complicated
decision-trigger: spec
gate-why: Operator and workload authority cross a process boundary; the human confirms effective credential isolation, current-owner fencing and the independently witnessed profile before adoption.
id: c6a7b975-21db-462a-8c21-e4ecb28d3090
consumers:
  - "containers/k8s/: follow-up desk-containers/29"
  - "docs/cellctl.md: follow-up desk-containers/29"
  - "docs/docker.md: follow-up desk-containers/29"
  - "operator adoption: out-of-scope (separate authorized R8 cutover after qualification)"

outcome: none

---

# Brief 29 — Kubernetes desk cell, efficiency and recovery qualification

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `containers/k8s/qualify.sh` (planned), `containers/k8s/testdata/qualification/` (planned), `docs/kubernetes-desk-qualification.md` (planned), `docs/docker.md`, `docs/cellctl.md`, `changelog/kubernetes-desk-qualification.md` (planned).

facts: Inspected Assay main d8687cab6, 20 October 2026. /28 supplies the in-cluster
operator and /27 the CNPG database profile. Existing graph-execution/26–27 owns startup,
role policy, leases and zero-idle-model-call behavior. Existing desk-tools/37–44 owns
matched review-cost acceptance; desktools-v2/63–65 owns attribution. Successful manifest
rendering, fake streams or a running Pod do not prove effective custody or recovery.

layering: Qualification drives the actual operator, adapter, supervisor and existing domain
owners; it creates no runtime service or acceptance policy. Offline fixtures and the
operator-authorized disposable-environment witness are separate evidence classes.

design-fit:
  owner: existing runtime/profile owners; R8 owns adoption evidence and handover
  contract: existing runtime/profile contracts, not a new runtime authority
  retires: []
  weight: verbs +0, flags +0 shipped runtime, refusals +0 runtime policy, qualification-script options +2 (offline and explicit disposable target)
  why-add: Effective Kubernetes enforcement and restart behavior cannot be inferred from unit tests or manifests; extend existing conformance and R8 evidence rather than add a monitor or scheduler.

single-point-of-failure: test-only assumptions can misdescribe effective enforcement;
independent hostile-workload checks and separate role/executor fencing evidence must
exercise the lower boundary even with the upper operator CLI bypassed.

## Read first

- [Target, optimization and migration coverage](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md).
- [Operator profile](brief-28-package-in-cluster-cellctl-and-bind-the-interactive-desk-profile.md).
- [CNPG profile and restore limits](brief-27-cnpg-kubernetes-postgres.md).
- [Runtime recovery](../../proposals/non-ci-architecture/14-storage-forge-graph.md).
- [Complete workflow-cost contract](../../desk-workflow-efficiency-spec.md).

## Human decision

Before a real-profile run, select the disposable target, explicit credential configuration,
expected server identity, authorized identities, image/profile versions, matched workload,
capacity limits and recovery objectives. Production and ambient contexts are excluded.

Options:
1. **Disposable qualification (recommended)** — authorize the identified run-owned cluster
   and synthetic forge/model fixtures, then consider a separately approved scoped cohort
   only after complete independent evidence.
2. **Remain offline** — retain fixture evidence and report effective Kubernetes execution
   as unqualified; perform no cluster contact or adoption.

Default if no answer: none — blocks until answered

## Ground rules

Implementation and ordinary checks are offline. The real-profile row is gate:human and
requires explicit operator authorization and an identified disposable cluster owned by
that run; no ambient context is used. Never probe production or trigger workflows.
Independent verification owns Evidence; no process exit or script success flips work done.

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner\'s delta; unchanged obligations remain.

## Task

1. Deliver an offline matrix and operator-run qualification harness through the actual
   production entrypoints. Offline is the default. A network-reaching mode requires an
   explicit target and opt-in, prints that target, verifies the disposable run's ownership
   receipt, and reports unavailable/error evidence as could-not-check rather than clean.
   Require an explicit kubeconfig file and context; never use ambient KUBECONFIG/default
   files. Resolve the endpoint and TLS trust/server identity offline and compare them to
   the run-owned receipt before first network contact. Pin the expected identity on the
   authenticated connection; a substituted file, endpoint or same-named context refuses
   with zero operation calls. Fixtures must plant each mismatch.
   Missing evidence cannot satisfy the adoption gate. Pin tool/schema/image/profile versions.
2. Prove five separately interactive desk Pods/PVCs plus one operator Pod. Check terminal
   I/O/resize/detach/reconnect and operator-Pod recreation; concurrent up and changed Pod UID
   must not create a second owner or lose unpublished files. Model session continuation,
   filesystem retention, process continuation and acceptance are reported separately.
   Lifecycle /18 and inventory /20 are consumed at qualified versions; missing features hold
   rather than reimplementing their CLI logic here.
3. Verify effective credentials, service accounts, RBAC, admission and network from hostile
   workloads with upper callers bypassed. Agent Pods cannot reach operator/publisher keys,
   cluster API or privileged database identity; operator terminal capability cannot attach
   or exec into database, executor or foreign-cell workloads. For named desk Pods the
   operator is attach-only: all verbs on exec/port-forward/ephemeral-debug subresources
   (including get/create/patch/update) are denied. Attempt compatibility-desk attach as
   a negative case and reject it before exposing the terminal. Verify default-deny
   ingress/no Service or Ingress on terminals, and denial of managed-agent API, metadata,
   CNPG and direct-forge egress, with only profile-declared destinations allowed.
   Treat compatibility container logs as session transcripts. With synthetic markers,
   prove `pods/log` and attach/exec/ephemeral-debug are limited to the relevant
   key-custody principals; the managed operator and unrelated log readers must be denied.
   Trace node log files and shipping: prove namespace exclusion, or prove every collector,
   downstream store, export/backup and reader retains the same custody boundary and
   retention/access controls. Plant a broader log grant or shared-log destination and
   require rejection; unavailable collection evidence holds qualification. No real
   credential is used in a transcript. These are obligations of the existing human-gated
   real-profile row, not evidence supplied by offline render checks.
   Exercise separate publisher
   current-owner rejection. Use synthetic material, never production credentials.
4. Rehearse permission denial, API partition/lost response, supervisor and operator failure,
   Pod reschedule with possible surviving old execution, database unavailable and database
   restore. No replacement writer until confirmed execution-group stop/fencing and effect
   reconciliation. Restore starts paused under the existing out-of-database incarnation;
   stopping a terminal or receiving delete success cannot reopen admission.
5. Reuse existing startup/efficiency fixtures: cold/warm, 1x/10x retained history, five-role
   startup, idle/blocked queues and reconnect. Verify zero idle/reconnect model requests,
   no recursive history walk at admission/finish, one compatible shared preparation,
   separate role leases and unchanged required observations/independent verdicts. Report
   full launch/request/token/CPU/storage/retry/human cost and perf-v3 provenance; unavailable
   observations remain unknown. Optional collection or improvement services do not gate the
   deterministic contract; a claimed measured benefit needs their actual producer evidence.
6. Write candidate qualification artifacts and the operator runsheet for scoped R8 migration:
   inventory legacy writers/credentials, drain, reconcile, fence, switch, validate and rehearse
   rollback. Account for every consumer before credential revocation or retirement. No live
   rollout is performed by authoring or implementing this harness. Update docs with qualified
   capabilities and limits. Generic code, tests, profiles and runbooks live in Assay; source
   deployments retain private values and historical evidence only.

- **Instance amendment:** Extend independent journey qualification to two heterogeneous workers and three reviewers, shared capacity across two repositories, paused-peer preservation, divide-not-duplicate reviews and overlapping replacement with busy child/unknown-effect controls. Preserve all effective credential/RBAC/network, operator restart, rollback, attribution and zero-idle-launch checks. Production adapter fixture proof and separately authorized witnessed cluster proof remain distinct.

## Verify

The real-profile row is deliberately a separate gate. An offline-only result is useful but
cannot complete Kubernetes adoption. Test scripts are implementation deliverables.

For the consumer row, the independent verifier obtains the actual implementation delivery
from the merged PR, records its full immutable SHA in `ASSAY_DELIVERY_SHA`, and records that
commit's actual first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither value defaults to
HEAD, origin/main or this authoring PR. Both commits must be available in the verification
checkout; absent/unknown provenance fails closed. Retain exact consumer result counts and
resolve unchecked required consumers separately; zero disproofs alone is not complete flow
coverage. These inputs are future implementation-verification evidence, not claims made here.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `sh containers/k8s/qualify.sh --offline --suite lifecycle` | exit 0; actual operator/adapter fixture proves five roles, immutable execution identity and disconnect/restart recovery; planted duplicate-launch/old-UID acceptance fails |
| 2 | check +flow +mutation | `sh containers/k8s/qualify.sh --offline --suite authority-recovery` | exit 0; production-code fake-API paths hold missing/unknown authority and old execution; planted ambient-target fallback, stop-on-delete assumption or direct publisher bypass fails |
| 3 | check +flow +mutation | `sh containers/k8s/qualify.sh --offline --suite efficiency` | exit 0; matched role fixtures preserve policy/coverage, zero idle/reconnect model calls and bounded work; duplicate usage, recursive finish scan or required-read removal fails |
| 4 | gate:human +flow +mutation +dereference | `sh containers/k8s/qualify.sh --kubeconfig "$ASSAY_DISPOSABLE_KUBECONFIG" --context "$ASSAY_DISPOSABLE_CONTEXT" --allow-cluster-queries --suite real-profile` | exit 0 only with explicit authorization, kubeconfig/context, receipt-bound API/TLS identity and complete independent evidence; real terminal, transcript/API/node/downstream log custody (including denied readers and planted shared-log leakage), hostile containment, reschedule, partition, CNPG restore and fenced rollback satisfy operator-declared objectives. Missing target/evidence holds; no production or default context contact |
| 5 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/29 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers remain a review obligation |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDeskInstancesJourney$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstancesJourney ' "$o")` | Named test executes (zero matching tests is not a pass); Production-path disposable fixture traverses desired population → supervisor → fake Kubernetes → claims/capacity → instance/API status and replacement. Mutation: duplicate one review assignment, hide draining occupancy or resume a paused peer makes the journey fail. Actual activation additionally requires the existing independent effective-profile proof. |
## Pre-mortem

Fakes hide actual custody: row 4, never a claim from rows 1–3. Delete/restore creates two
writers: rows 2 and 4. Fast result skips coverage or double-counts usage: row 3. Operator
loss discards files or owns desk lifetime: rows 1 and 4. Recovery thresholds are an operator
decision before row 4, not invented acceptance numbers.

## Evidence

<!-- Independent verifier records profile/image/tool versions, target class, commands,
coverage, observations, hashes and could-not-check cases; no secrets or private targets. -->

## Review

Human gate. Runtime implementation and effective-profile evidence are both prerequisites
to adoption; private deployment values are not prerequisites to public implementation.
