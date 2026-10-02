---
brief: assay:assay:desk-supervision:11
title: The single-workflow-only-PR contract, and the verb by which the workflow App writes and lands it
why: >-
  This is the rule change itself: a workflow change stops being a staged copy a human hand-lands
  and becomes a single pull request whose diff touches only .github/workflows/** (plus its own
  changelog fragment), authored by the workflow App. Decoupling the workflow change from every
  other file is what removes the drift (nothing unrelated to go stale against) and the stall (no
  human-copy step); concentrating the write in the workflow App is what keeps CI a
  single-auditable-identity surface. Without a defined contract and a verb that enforces it, the
  team would improvise mixed PRs and the guarantee evaporates.
wave: 1
depends: ["desk-supervision/10"]
unblocks: ["desk-supervision/12", "desk-supervision/23"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This brief CHANGES the PR-discipline rule for CI files and grants a program (the verb) the act of
  authoring — and possibly landing — a change to the supply-chain surface. The human is confirming
  the workflow-only invariant is the rule of record, that the independent guard refusing a mixed PR
  is acceptable, and the merge-authority posture (App-merges vs human-merges) the DR left open.
design: DR-workflow-app-landing
decision-trigger: creation
decision-issue: 1246
issues: [1175, 1185]
schema: brief-v2
version: 1
id: dee4e24f-e570-40be-b76f-87ee7f516e58
authored: 2026-09-16 by desk-supervision authoring session
exec-tier: strong
exec-tier-why: >-
  (a) the verb's PR-construction and merge-authority behaviour is a design not fully pre-specified;
  (b) correctness is the cross-artifact argument that the two layers are independent; (c) it is
  supply-chain plumbing where a subtle bug (a mixed PR slipping through) survives a happy-path test.
sources:
  - "[DR-workflow-app-landing](../decisions/DR-workflow-app-landing.md) — the design record; this brief implements its rule and settles its deferred merge-authority sub-decision at this brief's gate."
  - "desk-supervision/10 — the confirmed/provisioned workflow App identity this verb authors as."
  - "ci/staged-workflows/README.md and tools/ci-load/activation/README.md — the two staging areas whose hand-copy step this contract replaces; #1175 (a workflow half sat 9+ days unlanded) and #1185 (help wanted, copy not landed) are the stalls it removes."
  - "docs/streams/apps-installer/design.md — the desk-verb + deskkit contract (kill switch first, one audit line, exit codes, fail closed) the new verb follows."
  - "freshness-checked 2026-09-16 @ e9fa19d3"
consumers:
  - "the PR-discipline rule surface (docs — the workflow-only-PR rule of record): follow-up desk-supervision/11 (this brief; flips to fixed-here when the implementation writes the rule doc)"
  - "tools/workflowpr/ (the new verb): follow-up desk-supervision/11 (this brief; flips to fixed-here when the implementation adds the verb)"
  - "the mixed-PR guard invoked by existing CI (not a new workflow file — see facts): follow-up desk-supervision/11 (this brief; flips to fixed-here when the implementation adds the guard)"
  - "ci/staged-workflows/ and tools/ci-load/activation/ (the staging areas): out-of-scope (their retirement is desk-supervision/12, not this brief — this brief adds the replacement path, 12 removes the old one)"
---

# Brief 11 — The single-workflow-only-PR contract and the verb

## Context

files:
- **create** `docs/streams/desk-supervision/workflow-only-pr-contract.md` (planned) — the rule of record:
  a workflow change travels as a single PR whose diff touches ONLY `.github/workflows/**` and its
  own `changelog/*` fragment; the workflow App is its author; no other-file coupling is permitted.
- **create** `tools/workflowpr/` (Go) — the verb: given a prepared workflow change, it opens a
  workflow-only branch, commits the `.github/workflows/**` change (and changelog fragment) as the
  workflow App, and opens the PR. Whether it also merges is set by the merge-authority decision
  below.
- **create** a mixed-PR **guard** invoked by the EXISTING CI (a `tools/workflowpr --check <base>..<head>`
  mode or a sibling guard tool run from `ci.yml`), NOT a brand-new workflow file — so installing
  the guard does not itself require `workflows: write` and cannot bootstrap-block on the very rule
  it enforces.
- **register** the guard's CI job as a REQUIRED status check in branch protection / a repository
  ruleset, with the workflow App absent from any bypass list — a repo-settings change, not a
  workflow file, and the placement that keeps the guard outside the one identity's own write
  surface (see the DR's branch-protection `accepted:` entry).
- **edit** `docs/adopting-assay.md` — reference the contract from the PR-discipline section.

facts:
- workflow-only-diff = paths under `.github/workflows/**` plus at most the change's own
  `changelog/*` fragment; ANY other path in the same diff violates the contract.
- author-of-record on a workflow-only PR = the workflow App (desk-supervision/10), never an
  ambient human credential.
- the guard runs in the EXISTING CI job, not a new `.github/workflows/*` file — avoids the
  bootstrap where the guard-installing PR would itself need the workflow App.
- guard placement: the CI job's own definition lives under `.github/workflows/**` — exactly the
  surface the workflow App can rewrite. Independence therefore does NOT come from where the guard
  runs; it comes from the guard's REQUIRED-ness being a branch-protection/ruleset setting the App
  cannot write (it holds no `administration` grant — DR `accepted:` #1) and from the App being
  absent from that ruleset's bypass list. A workflow-only PR that edits the guard's own CI step
  still cannot merge past the required check without a human, because required-ness is enforced
  from outside the diff entirely.
- credential boundary: the verb's authoring/merging mode runs only under a human-initiated
  invocation (an operator/desk run or `workflow_dispatch`); its `--check` classification mode is
  credential-free and offline, and is the only mode reachable from an ordinary `pull_request`-
  triggered CI run on this public repository (DR `accepted:` credential-boundary entry).
- single-point-of-failure: the workflow-only invariant — if a workflow change and a code change
  ride one PR, the coupling that causes drift is back. TWO INDEPENDENT LAYERS hold it:
  (1) the verb CONSTRUCTS a workflow-only diff by only ever staging the allowed paths — it cannot
  emit a mixed diff; (2) an INDEPENDENT guard, required at merge time from branch protection —
  not merely present in CI — FAILS any PR whose diff mixes `.github/workflows/**` with a
  disallowed path, regardless of who authored it or how, and regardless of whether the same PR
  also edited the guard's own CI step. They fail for different reasons (construction vs
  merge-time-required inspection) in different components (the verb vs the forge's branch
  protection), so a hand-made mixed PR that never touched the verb is still caught, AND a
  workflow-only PR that tries to neuter the guard from inside its own write surface still cannot
  merge past it.

## Human decision
<!-- decision-trigger: creation — options enumerable now; filed as the brief lands. Self-contained. -->
Today a CI-workflow change cannot ride the same pull request as the code change that needs it,
because the everyday automation identities are deliberately barred from writing workflow files. So
the change is staged as a copy and a human copies it into place later — a step that stalls for days
and drifts out of date. This brief makes a workflow change its own single pull request, touching
only the workflow files and its own changelog note, written by the one identity permitted to write
workflow files. A human must confirm this becomes the rule and choose who merges that pull request.

What is being decided:
1. Whether "a workflow change is a single workflow-only pull request, authored by the workflow
   identity" becomes the rule of record, replacing the staged-copy-then-hand-copy step.
2. Whether an automated guard that FAILS any pull request mixing workflow files with unrelated
   files is acceptable as a hard check.
3. Who merges the workflow-only pull request: the workflow identity itself (fully removing the
   human step), or a human as with every other pull request (keeping a human merge decision).

Options:
1. **Adopt, human-merges** — the contract and guard become the rule; the workflow identity writes
   the pull request; a human merges it. Removes the drift and most of the stall; keeps a human
   merge gate. (Recommended for the first cutover.)
2. **Adopt, identity-merges** — as option 1, but the workflow identity also merges its own
   workflow-only pull request after checks pass, removing the human step entirely. Wider than
   today's status quo (today's direct-to-default-branch promote route needs a maintainer
   credential): admissible only once Verify rows 11/12 (required-check membership and bypass-list
   absence) are green, AND paired with a rule that any diff touching the guard's own CI job still
   requires a human merge regardless of merge-authority mode.
3. **Do not adopt** — keep the staged-copy hand-landing. Briefs 11 and 12 do not proceed.

Recommendation: **option 1** — prove the path end to end with a human merge gate first; revisit
identity-merges once the guard has caught real mixed pull requests in practice.

Default if no answer: none — blocks until answered. This brief's README row is `blocked`
(`lifecycle-v1.md` §2.0) via its own `depends: [desk-supervision/10]` and via
desk-supervision/10 not yet being ruled — it stays `blocked` until 10 reaches `done`.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. The deliverable is a draft PR
  opened by the desk verbs.
- Stop at `implemented` — you do not set verified/done.
- The verb must follow the deskkit contract: kill switch first, one audit line per invocation, exit
  0 ok · 3 disabled · 5 refused · 6 unverifiable, fail closed.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `workflow-only-pr-contract.md` (planned): the diff-scope rule, the author-of-record rule, the
   no-coupling rule, cross-linking the DR and desk-supervision/10.
2. Build `tools/workflowpr/`: construct a workflow-only branch + PR as the workflow App; a
   `--check <base>..<head>` mode that classifies a diff as workflow-only or violating; merge only
   per the merge-authority decision.
3. Wire the `--check` guard into the existing CI job so every PR touching `.github/workflows/**` is
   classified, and a mixed diff reddens.
4. Register the guard's check as REQUIRED in branch protection / a ruleset, and confirm the
   workflow App carries no bypass entry.
5. Reference the contract from `docs/adopting-assay.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `test -f docs/streams/desk-supervision/workflow-only-pr-contract.md && grep -c 'workflow-only' docs/streams/desk-supervision/workflow-only-pr-contract.md` | exit 0; ≥ 1 | check |
| 2 | `test -f tools/workflowpr/main.go` | exit 0 | check |
| 3 | `cd tools/workflowpr && go test ./...` | exit 0 | check:ci |
| 4 | LAYER 1 — construct: `tools/workflowpr --dry-run` on a prepared workflow change prints the diff it would open | exit 0; every path is under `.github/workflows/` or `changelog/` and none other | check +dereference |
| 5 | LAYER 2 — guard, NEGATIVE path: run `tools/workflowpr --check` on a fixture diff mixing `.github/workflows/ci.yml` with `README.md` | exit non-zero; message names the disallowed path | check:ci +mutation |
| 6 | LAYER 2 — guard, POSITIVE path: run `tools/workflowpr --check` on a fixture diff touching only `.github/workflows/ci.yml` (plus its changelog fragment) | exit 0 | check:ci |
| 7 | INDEPENDENCE: `tools/workflowpr --check` on the row-5 fixture reddens even though no branch was made by the verb (proves the guard inspects the diff, not the verb's provenance) | exit non-zero | check +neighbour |
| 8 | FLOW (end to end): for a prepared change the workflow App opens a workflow-only PR, the CI guard passes, and the PR lands per the merge-authority decision — the change reaches `.github/workflows/` with no staged copy anywhere in the flow | the workflow is live via the PR path; no staging directory was touched | gate:human +flow |
| 9 | `statusgen --consumers --root . --brief desk-supervision/11` | exit 0 (consumers routing corroborated against the diff) | check:ci |
| 10 | `statusgen --root . --lint` | exit 0 (only pre-existing PROBLEMs) | check:ci |
| 11 | READ FROM THE FORGE, not the tool: the guard's check name appears in the repository's required-status-checks list (branch protection / ruleset API) | exit 0; the guard is REQUIRED, not merely present in CI | gate:human +dereference |
| 12 | READ FROM THE FORGE: the workflow App's identity does not appear in that ruleset's bypass-actors list | exit 0; no bypass entry names the workflow App | gate:human +dereference |
| 13 | CREDENTIAL BOUNDARY, negative: `tools/workflowpr --check <fixture>` run with no token in the environment | exit 0 as specified — the check mode makes no forge call and needs no credential | check +neighbour |
| 14 | CREDENTIAL BOUNDARY, refusal: the authoring/merge mode invoked without an explicit human-initiated dispatch (i.e. as if from a `pull_request`-triggered run) | exit non-zero; refuses rather than acting | check +mutation |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 5 and 7 are the negative /
     independence proofs — a table that only walks rows 4 and 6 has verified one layer, not two.
     Rows 11/12 prove the guard's required-check placement and the App's bypass-list absence; rows
     13/14 prove the credential boundary. All four are read from the forge or exercised directly —
     never inferred from the doc. -->

## Review
Gate: human (from frontmatter — sensitive-data is yes). Human gate is MANDATORY.
Reviewer questions (core-system / supply-chain): (1) what single control stands between a coupled
workflow+code change and the drift it causes, and is the two-layer answer acceptable? (2) does a
Verify row prove the guard (lower layer) catches a mixed PR with the verb (upper layer) bypassed?
Rows 5 and 7 are that proof. Reviewer records verdict + date in the stream README table.
