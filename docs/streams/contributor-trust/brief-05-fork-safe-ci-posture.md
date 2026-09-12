---
brief: assay:assay:contributor-trust:05
title: "Fork-safe continuous-integration posture — audited workflows, tier-keyed approve-and-run, and never-build-unblessed enforced by a check"
why: >-
  A fork's head is arbitrary code written by somebody the project does not know, and the only
  things standing between it and a runner holding credentials are a maintainer's click and a
  sentence of written guidance. A sentence has no failure signal: when it is broken, nothing
  reports it and the first indication is the consequence. This brief audits which workflows
  could expose a secret to a fork head, makes the approve-and-run policy a function of the
  author's tier and the workflow's class rather than of the maintainer's patience, and turns
  never-build-unblessed into something that reddens.
wave: 2
depends: ["contributor-trust/02", "contributor-trust/03"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
gate-why: >-
  This brief changes WHO can cause code to execute on this project's runners without a fresh
  human act, which is the highest-consequence widening in the whole stream: a secret exposed
  to a fork head is exfiltrated irreversibly the moment it executes, and no later control
  recovers it. The human is confirming three specific things, not the diff: that auto-approval
  is bounded to workflow classes shown unable to reach a secret or a credentialed runner
  (Verify rows 3 and 4), that a fork pull request touching continuous-integration
  configuration, lockfiles, install scripts or container definitions drops to the strictest
  posture regardless of tier (row 5), and that the desk-side refusal is described honestly as
  a second layer and never as a sandbox.
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §2 — the measured fork posture: manual approve-and-run for first-time fork contributors, and never-build-unblessed as a resident rule with no enforcement."
  - "docs/streams/decisions/DR-fork-ci-posture.md — the design record this brief is authored against (PROPOSED): two independent layers, posture as a tier-by-workflow-class matrix, risky paths overriding tier."
  - ".github/workflows/inbound-triage.yml — the one workflow here using pull_request_target; its safety argument is that it never checks out the pull request's code and reads event metadata only. The audit must assert that property rather than assume it."
  - "docs/streams/contributor-trust/brief-02-trust-tiers-and-ledger.md — the tier vocabulary; fork approve-and-run posture is one of the four per-tier capabilities defined there."
  - "docs/streams/contributor-trust/brief-03-bless-verb-and-audit.md — the blessing state the desk-side refusal reads; without the marked form there is no reliable signal for the check to key on."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — sixteen workflow files; one uses pull_request_target; no workflow audit, no tier-keyed approval policy and no never-build-unblessed check exist."
design: DR-fork-ci-posture
decision-trigger: creation
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument over every workflow's trigger, permissions and
  checkout behaviour read against the approval policy; and (c) a subtle error exposes a
  credential to untrusted code, and a happy-path run on a trusted author's branch passes
  identically either way.
consumers:
  - ".github/workflows/: follow-up contributor-trust/05 (this brief; the audited workflows and any trigger or permission correction the audit forces)"
  - "docs/fork-ci-posture.md: follow-up contributor-trust/05 (this brief; the published matrix of tier against workflow class)"
  - "tools/desk/internal/deskkit/forkbuildguard.go: follow-up contributor-trust/05 (this brief; the desk-side refusal)"
  - "plugins/assay/resident-rules.md: follow-up contributor-trust/05 (this brief; the never-build-unblessed rule gains a sentence noting the mechanical backstop, and the rule text itself stays)"
  - "docs/contributor-trust.md: follow-up contributor-trust/05 (this brief; the per-tier capability list gains the concrete posture)"
  - "Repository workflow-approval settings: out-of-scope (a forge setting a human applies; this brief documents the required setting and asserts the in-tree half, and never changes a repository setting itself)"
version: 1
---

# Brief 05 — Fork-safe continuous-integration posture

## Context

files:
- `tools/workflowaudit/` (new) — a checker over `.github/workflows/`: for each workflow, its
  triggers, its permissions block, whether it can be triggered by a fork, whether it checks
  out a pull-request head, and whether it can reach a secret or a self-hosted runner. Emits a
  classification per workflow and exits non-zero on an unsafe combination.
- `tools/workflowaudit/testdata/` — fixtures including a deliberately unsafe workflow
  (`pull_request_target` plus a head checkout plus a secret) that the audit must reject.
- `.github/workflows/` — any correction the audit forces, in the same change.
- `.github/workflows/fork-risky-paths.yml` (new) — labels a fork pull request whose diff
  touches continuous-integration configuration, dependency lockfiles, install scripts or
  container definitions. Metadata and diff-path reads only; never checks out the head.
- `tools/desk/internal/deskkit/forkbuildguard.go` (new) — the desk-side refusal: a build or
  test invoked against a fork head whose item is not blessed refuses, exit 5.
- `plugins/assay/resident-rules.md` — one sentence noting the mechanical backstop.
- `docs/fork-ci-posture.md` (planned) (new), `docs/contributor-trust.md` (planned), and
  `changelog/contributor-trust-fork-ci.md` (new).

single-point-of-failure: the maintainer's click on approve-and-run is the control that stands
alone today. This design puts two independent layers behind it, failing for different reasons
in different components: (1) at the forge, a workflow's own trigger and permissions decide
whether a fork head can execute at all and with what token — this holds even if every tool in
this repository is bypassed, because the platform enforces it; (2) on the operator's machine,
the desk-side guard refuses the local build path for an unblessed head — this holds when a
maintainer with approval rights approves a run they should not have, because it trips on a
different signal (blessing state) in a different place at a different time.

facts:
- Posture is a matrix of author tier against workflow class, not of tier alone. A workflow
  that can reach a secret or a self-hosted runner is NEVER auto-approved for any external
  tier, at any tier above `unknown`.
- A fork pull request whose diff touches continuous-integration configuration, dependency
  lockfiles, install scripts or container definitions drops to the strictest posture
  regardless of the author's tier, and is labelled so the queue shows why. Those paths change
  what the runner executes, so authorship trust is the wrong input for them.
- `pull_request_target` grants a write-capable token in the base repository's context. A
  workflow using it MUST NOT check out the pull request's head. The audit asserts this rather
  than trusting the comment that currently records it.
- The desk-side guard is a control over the desk's own behaviour. It can be evaded by a human
  running a command outside the tools. It raises the cost of the mistake and gives it a
  signal; it is NOT a sandbox and the documentation must not describe it as one.
- The audit is a three-state instrument: a workflow it cannot classify is reported as
  could-not-check and is never reported as safe.
- Nothing in this brief changes a repository setting. The required forge-side setting is
  documented for a human to apply.

## Human decision
<!-- gate: human — decision-trigger: creation. Lifted VERBATIM into the decision issue; self-contained. -->
When somebody outside the project opens a pull request from their own copy of the repository,
the code in it is arbitrary code written by a stranger. Today a maintainer must click
"approve and run" before the automated build and test jobs execute on it, and there is a
written rule saying nobody should build or test such a change locally until it has been
admitted. The written rule has no enforcement at all: if it is broken, nothing anywhere
reports it, and the first sign is the damage.

The proposal has three parts. First, audit every automated job in the project for whether a
stranger's code could cause it to run with access to a credential or to a machine the project
owns, and fix anything the audit finds. Second, replace the blanket "a maintainer clicks every
time" with a rule that depends on both how well known the author is AND what the job can
reach: a job that cannot touch any credential and runs on a disposable machine may start
automatically for a known contributor, while any job that can touch a credential always waits
for a human, no matter who the author is. Third, make the never-build-locally rule a real
check that refuses the command, instead of a sentence somebody has to remember.

There is one special case worth deciding explicitly: a change from a stranger that edits the
build configuration itself, the dependency lock files, the install scripts, or the container
definitions. Those files decide what the machine executes, so trusting the author tells you
nothing useful about them.

Options:
1. **Adopt all three parts, and make risky-path changes always take the strictest treatment
   regardless of how well known the author is (recommended)** — the automatic start is
   available only where a job provably cannot reach anything worth stealing, and the files
   that change what runs are always a human's call. Consequence accepted: a known contributor
   editing a lock file still waits for a click, which is slightly annoying and is the correct
   trade; and the local check can be bypassed by a person determined to bypass it, so it must
   be described as a second layer and not as a sandbox.
2. **Adopt all three parts, but let a well-known contributor's risky-path changes start
   automatically too** — fewer clicks for regular contributors. Consequence: the single
   highest-value target for a compromised or borrowed account becomes an automatic path, and
   account compromise is precisely the case where prior good behaviour predicts nothing.
3. **Audit and fix the jobs, but keep every external start behind a manual click** — the
   safety improvement without the convenience one. Consequence: the maintainer stays the
   bottleneck for every genuine contributor, which is close to today.
4. **Only enforce the never-build-locally rule; leave the jobs and the click policy alone** —
   the cheapest part. Consequence: the exposure the audit exists to find, if there is one,
   stays unfound.

Default if no answer: none — blocks until answered. This decides who can cause code to run on
the project's machines without a human present.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never check out, build, test or execute a fork pull-request head during this work, including
  in a fixture. Every fixture is a synthetic workflow file in this repository's test data.
- If greening a check would require removing or weakening a security control or its assertion,
  STOP and escalate for a human ruling. A weakened control is never the resolution.

## Task

1. `tools/workflowaudit`: classify every workflow under `.github/workflows/` on trigger,
   permissions, fork-triggerability, head checkout, secret reach and runner class. Three
   states; an unclassifiable workflow is could-not-check. Exit non-zero on an unsafe
   combination, naming the workflow and the combination.
2. Run it, and land any correction it forces in this change. Assert the existing
   `pull_request_target` workflow's never-checks-out property as a test, not as a comment.
3. `fork-risky-paths.yml`: label a fork pull request touching the four risky path classes.
   Metadata and diff-path reads only; no head checkout; least-privilege permissions.
4. `forkbuildguard.go`: refuse a build or test invoked against an unblessed fork head, exit 5,
   with one audit line. Kill switch first, fail closed.
5. `docs/fork-ci-posture.md` (planned): the matrix, the required forge-side setting for a human to
   apply, and an explicit statement that the desk-side guard is a second layer and not a
   sandbox. One sentence in the resident rules; the per-tier capability list updated; the
   changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/workflowaudit && GOWORK=off go test . -count=1` | exit 0; output contains `ok` | check |
| 2 | `cd tools/workflowaudit && GOWORK=off go run . --root ../..; echo rc=$?` | output contains `rc=0` (every workflow in this repository classifies safe after the audit's corrections land: the audit reads the real `.github/workflows/` tree, so this row exercises the tool and the workflows together rather than a fixture) | check:ci +flow |
| 3 | `cd tools/workflowaudit && GOWORK=off go run . --root testdata/unsafe-target-checkout; echo rc=$?` | output does not contain `rc=0`; output contains `pull_request_target` | check +mutation |
| 4 | `cd tools/workflowaudit && GOWORK=off go run . --root testdata/unclassifiable; echo rc=$?` | output contains `could-not-check`; output does not contain `rc=0` | check +mutation |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ForkRiskyPathOverridesTier' -count=1 -v` | exit 0; output contains `PASS` (a risky-path fork change takes the strictest posture at every tier) | check +mutation |
| 6 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ForkBuildGuardUnblessed' -count=1 -v` | exit 0; output contains `PASS` | check +mutation |
| 7 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ForkBuildGuardBlessedPasses' -count=1 -v` | exit 0; output contains `PASS` (positive control: a blessed head is not refused) | check |
| 8 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ForkPostureSecretReachNeverAuto' -count=1 -v` | exit 0; output contains `PASS` (no tier auto-approves a workflow that can reach a secret) | check +mutation |
| 9 | `grep -n 'not a sandbox' docs/fork-ci-posture.md` | exit 0; at least one matching line | check +dereference |
| 10 | `git -C . grep -n 'actions/checkout' -- .github/workflows/inbound-triage.yml` | exit 1; no matching line (the existing metadata-only workflow still checks out nothing) | check +neighbour |
| 11 | `statusgen --root . --consumers --brief contributor-trust/05` | exit 0; output does not contain `DISPROVED` | check |

Pre-mortem to detection map. "The audit passes because it only looks at triggers and never at
whether a job checks out the head" is caught by row 3, a fixture combining the event with a
head checkout. "A workflow the audit cannot parse is counted as safe" is caught by row 4. "A
known contributor's lock-file change is auto-approved because the posture keys on tier alone"
is caught by row 5. "The desk guard is added but passes on the unblessed path" is caught by
row 6, and "the guard refuses everything, so the desk stops working" by row 7. "Auto-approval
is extended to a secret-reaching workflow for a high tier" is caught by row 8. "The existing
metadata-only workflow later gains a checkout and nobody notices" is caught by row 10. "The
documentation oversells the desk guard as a sandbox" is caught by row 9. "A human never
applies the forge-side approval setting, so the in-tree half is the only half live" — no row;
a repository setting is outside every tree a check can read, and it is listed for the human
in the posture document.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
