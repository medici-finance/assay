---
brief: assay:assay:desk-containers:07
title: Optional Linux operator terminal layout over the shared desk attach path
wave: 11
depends: ["desk-containers/28"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [64]
schema: brief-v2
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#64 — the request (tmux and equivalents as a cheap way to control all the desks; win32 as well as mac; the aim is to fire up the docker/k8s pods)"
  - "docs/streams/desk-containers/spec.md — multi-desk control layer section + open question 8"
  - "docs/streams/desk-containers/brief-28-package-in-cluster-cellctl-and-bind-the-interactive-desk-profile.md — single attach path"
  - "docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md — Linux operator cohort"
  - "freshness-checked 2026-10-20 @ 65f16f4a6b94 (fetched main; layout remains planned)"
why: A single Linux operator terminal should expose five independent desk sessions without requiring native Windows or adding another launcher.
version: 6
id: cbf80c98-0c5e-4309-836b-89bced9ffae5
exec-tier: strong
exec-tier-why: Terminal presentation must preserve remote session and lifecycle ownership.
consumers:
  - "containers/control/: follow-up desk-containers/07"
  - "docs/docker.md: follow-up desk-containers/07"
outcome: none

---

# Brief 07 — Optional Linux operator terminal layout

## Context

files: `containers/control/EVALUATION.md` (planned), `containers/control/desks.yaml` (planned), `containers/control/check.sh` (planned), `docs/docker.md`, `changelog/linux-operator-layout.md` (planned).

facts: The 20 October 2026 direction permits cellctl inside Kubernetes and removes Windows
from this target. /28 is the single operator path. A multiplexer is optional presentation;
its absence cannot block start/attach/reconnect or require five models to stay active.
The original macOS/Windows evaluation is historical context, not a current delivery gate.

layering: A terminal layout calls /28's existing attach interface; no container flags,
credential reads, lifecycle logic or second launcher in layout scripts.

design-fit:
  owner: operator terminal presentation; execution remains loopadmin
  contract: existing cell-operation/attach boundary
  retires: [required cross-platform multiplexer evaluation for this Linux cohort]
  weight: verbs +0, flags +0 runtime, refusals +0, layout files +3
  why-add: A convenience layout can expose the five sessions without granting new authority; direct cellctl attachment remains sufficient.

## Read first

[Current target](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md)
and [operator profile](brief-28-package-in-cluster-cellctl-and-bind-the-interactive-desk-profile.md).

## Ground rules

Branch/draft PR only; no live API, model call, deployment or workflow. No credentials in
layout or output. Independent verification owns Evidence.

## Desk-instance scope — 25 October 2026

Consume the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) through desk-containers/28. Layout entries enumerate configured instance identities, including paused/draining peers; a role is a grouping, not one terminal. Reconnect is presentation and starts no model or replacement. Existing singleton layouts remain supported.

## Task

1. Evaluate the smallest supported Linux terminal layout inside the operator image. Record
   actual tool/version and observed attach/detach behavior; direct one-role cellctl attachment
   remains the baseline. Do not implement native Windows, Docker orchestration or a service.
   `check.sh --version-evidence` (planned) invokes the actual selected tool's version and
   required local capabilities with literal argv, compares them to the recorded version/
   capability manifest, and fails on a missing tool, mismatch or unsupported capability.
   Include a mutation changing the recorded version and require the comparison to fail.
2. Expose five named role attachments through /28, using configured role aliases. Closing a
   pane/session never terminates its desk or starts a replacement; reconnect is idempotent.
   The layout composes no Pod specs and knows no Secret or credential values.
3. Add an offline fixture that replays actual layout calls against /28's fake stream backend,
   counting launches and owners. Update docs/docker.md with direct and optional layout use,
   checked versions and the distinction between disconnect and explicit lifecycle stop.

- **Instance amendment:** enumerate exact instance/generation attach bindings and label role groups; ambiguous role shortcuts refuse rather than select the first peer. Preserve detached or paused peers and reuse the shared runner attach path.

## Verify

For the consumer row, the independent verifier obtains the actual implementation delivery
from the merged PR, records its full immutable SHA in `ASSAY_DELIVERY_SHA`, and records that
commit's actual first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither value defaults to
HEAD, origin/main or this authoring PR. Both commits must be available in the verification
checkout; absent/unknown provenance fails closed. Retain exact consumer result counts and
resolve unchecked required consumers separately; zero disproofs alone is not complete flow
coverage. These inputs are future implementation-verification evidence, not claims made here.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +dereference | `sh containers/control/check.sh --offline` | exit 0; five role panes call the single attach path; closing/reopening the layout produces zero new model invocations or duplicate role owners |
| 2 | check +mutation | `sh containers/control/check.sh --offline --self-test` | exit 0; planted start-on-attach, stop-on-pane-close and credential read all fail the fixture; no cluster contact |
| 3 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && statusgen --root . --consumers --brief desk-containers/07 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers remain a review obligation |
| 4 | check +dereference +mutation | `sh containers/control/check.sh --version-evidence --self-test` | exit 0; real selected tool version and local capabilities match EVALUATION.md; changed recorded version/capability fails; missing tool is could-not-check |
| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDeskInstanceLayout$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceLayout ' "$o")` | Two worker and three reviewer bindings remain distinct; paused/draining views and reconnect start zero model calls. Mutation: attach first role match makes the named test fail. |

## Pre-mortem

Terminal becomes launch/lifetime owner: rows 1–2. A credentials-owning parallel launcher:
row 2. Tool/version claims require a reviewer to dereference actual recorded tool output;
real stream behavior is independently qualified in /29, not inferred from layout existence.

## Evidence

<!-- Independent verifier records command, version and observed behavior. -->

## Review

Gate: model; presentation adds no privilege. Native Windows is optional separate work.
