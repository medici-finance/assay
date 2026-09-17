---
brief: assay:assay:desk-supervision:10
title: Confirm or repair the workflow App wiring — one identity holding workflows:write, installed and scope-proven
why: >-
  The whole workflow-only-PR model rests on ONE narrowly-scoped identity — the workflow App —
  actually existing, being installed on the repo, and holding exactly the scope it should and no
  more. Today workflow files are hand-landed because the desk Apps deliberately lack
  workflows:write; if the intended single-holder App is missing, mis-scoped, or wired only for a
  straight-to-default-branch promote job rather than to author a PR, the model cannot be turned
  on. This brief establishes the ground truth before any verb is built on top of it, and turns a
  missing or mis-scoped App into an explicit provisioning ask rather than a silent blocker.
wave: 0
depends: []
unblocks: ["desk-supervision/11"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  The subject is a credential's permission scope: an identity holding workflows:write can rewrite
  what every CI check asserts, a supply-chain surface. The human is confirming that exactly ONE
  identity holds that grant, that its scope is contents:write + workflows:write + metadata:read and
  nothing wider, and — if provisioning is needed — authorising the App install/permission act, which
  GitHub reserves for a signed-in human.
design: DR-workflow-app-landing
decision-trigger: creation
decision-issue: 1245
issues: [1185, 1187]
schema: brief-v2
version: 1
id: 6c314a91-7a46-40d0-babb-64122a8792f7
authored: 2026-09-16 by desk-supervision authoring session
exec-tier: strong
exec-tier-why: >-
  (b) — correctness is a cross-artifact argument: the App's GRANTED permission set (read from the
  forge) against its DECLARED duties against the fleet-wide invariant that no OTHER App holds
  workflows:write.
sources:
  - "[DR-workflow-app-landing](../decisions/DR-workflow-app-landing.md) — the design record this brief's human gate dereferences; it proposes the one-identity model and defers merge-authority to this gate."
  - "ci/staged-workflows/README.md — states plainly that no bot or App in this project holds workflow-push permission and that GitHub hard-rejects an App push to .github/workflows/*; the constraint this brief measures against."
  - "tools/ci-load/activation/README.md — the second staging area, same constraint; #1187 tracks three of its five copies gone stale."
  - "docs/adopting-assay.md — the App setup runbook the workflow App must be recorded in alongside the desk-role Apps."
  - "docs/streams/apps-installer/README.md and design.md — the app-scopes-vs-duties preflight (granted-vs-declared) that is the dereferencing check here, and the tier model the workflow App sits beside as a capability, not a tier."
  - "freshness-checked 2026-09-16 @ e9fa19d3"
consumers:
  - "docs/adopting-assay.md: follow-up desk-supervision/10 (this brief; flips to fixed-here when the implementation records the workflow App in the App inventory)"
  - "the desk App-scopes-vs-duties preflight: out-of-scope (it reads the LIVE granted permission set from the forge, not this doc; Verify row 4 exercises it against the live App rather than this brief changing what it reads)"
---

# Brief 10 — Confirm or repair the workflow App wiring

## Context

files:
- **edit** `docs/adopting-assay.md` — record the **workflow App** in the App inventory as a
  capability that stands beside the desk-role Apps (it is not one of the desk roles and not a
  tier): the single identity holding `workflows: write`, scoped to `contents: write` +
  `workflows: write` + `pull_requests: write` + `metadata: read`, subscribing to zero webhook
  events, with `administration`, `actions`, `checks`/`statuses: write`, `members`, and
  secrets/variables withheld.
- **create** `docs/streams/desk-supervision/workflow-app-scope.md` (planned) — a short scope-and-duties
  note: the exact permission set (granted AND withheld), the duties it discharges (author a
  workflow-only PR; optionally land it — see the DR's deferred merge-authority sub-decision), and
  the invariant that NO other App holds `workflows: write`.

facts:
- workflows-write-holders-should-be: exactly one (the workflow App). Every desk-role App: none.
- required-scope: `contents: write` + `workflows: write` + `pull_requests: write` +
  `metadata: read`, zero events; `pull_requests: write` is required because the App's duty is to
  open (and, only under identity-merges, merge) the workflow-only PR — a forge action
  `contents`/`workflows` do not cover. Withheld, and checked as absent: `administration`,
  `actions`, `checks`/`statuses: write`, `members`, secrets/variables.
- sole-holder-invariant: binds `workflows: write` ALONE — the scope is a four-permission granted
  set plus a named withheld set, not "nothing else" read literally (a `pull_requests`-less App
  could not discharge the duty this record assigns it).
- constraint: GitHub hard-rejects any App push that creates or updates a `.github/workflows/*`
  file unless the App holds `workflows: write` — established in `ci/staged-workflows/README.md`.
- ground-truth-unknown-at-authoring: whether such an App is installed on this repo with the
  PR-authoring capability is NOT verifiable from the brief; it is Verify row 4's job, and a
  missing/mis-scoped result is a provisioning ask, not a defect in this brief.
- single-point-of-failure: the App's permission GRANT itself — one mis-scope (too wide, or held
  by more than one App) defeats the concentration. Second layer: the app-scopes-vs-duties
  preflight (row 4) reads the live grant and refuses on drift, and row 5 proves the negative —
  no other App holds the scope — so a widening is caught in a different component (the forge grant)
  from where it is declared (the doc).

## Human decision
<!-- decision-trigger: creation — options enumerable now; filed as the brief lands. Self-contained. -->
A workflow change today is hand-landed: an author who cannot write `.github/workflows/*` stages a
copy of the file, and a human copies it into place in a separate commit. That step stalls (a
change sat over a week waiting for it) and drifts (staged copies go stale and, landed verbatim,
silently revert intervening fixes). The proposed fix rests on ONE narrowly-scoped identity — a
workflow App — holding the only permission to write workflow files, so a workflow change can ride
its own reviewable pull request instead of a hand copy. Before anything is built on it, a human
must confirm that identity's ground truth and authorise any provisioning.

What is being decided:

1. Whether the workflow App exists, is installed on this repository, and holds exactly
   `contents: write` + `workflows: write` + `pull_requests: write` + `metadata: read` — the set
   its duty (open, and optionally merge, the workflow-only PR) actually requires — with
   `administration`, `actions`, `checks`/`statuses: write`, `members`, and secrets/variables all
   withheld, and no other identity holding `workflows: write`.
2. If it does not — whether to provision it now (create or adjust the App, set that exact scope,
   install it), which is an act only a signed-in human can perform.
3. The merge-authority posture to carry into the next brief: may the workflow App also merge its
   own workflow-only pull request, or does a human merge it as with every other pull request.

Options:
1. **Confirm as-is** — the App exists and is correctly scoped; record it and proceed. Next: the
   contract-and-verb brief builds on a confirmed identity.
2. **Provision, then confirm** — the App is missing or mis-scoped; a human creates/adjusts and
   installs it to the exact scope, then this brief's checks are re-run. Next: same as option 1
   once green.
3. **Hold** — the one-identity model is not accepted; the staged-copy hand-landing continues.
   Next: this stream's briefs 11 and 12 do not proceed.

Recommendation: **option 1 or 2** (whichever the ground truth requires) with merge-authority set
to **human-merges** for the first cutover, revisiting once the path is proven.

Default if no answer: none — blocks until answered. Mechanically, this brief's README row (and
briefs 11/12's) stays at status `blocked` (`lifecycle-v1.md` §2.0 — excluded from Next-up) while
this record is `proposed`; the design-approval gate alone does not hold a `todo` row out of
Next-up, so `blocked` is the control this brief and its siblings actually rely on until a ruling
flips them back to `todo`.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only the docs this
  brief specifies.
- Stop at `implemented` — you do not set verified/done.
- Do NOT create, install, or re-scope any App yourself: provisioning is a human act. If the App is
  missing or mis-scoped, report it as `NEEDS_CONTEXT` / a provisioning ask and stop.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add the workflow App to the App inventory in `docs/adopting-assay.md`: its exact scope, that it
   is the SINGLE holder of `workflows: write`, and that it is a capability beside the desk roles,
   not a desk role or a tier.
2. Write `docs/streams/desk-supervision/workflow-app-scope.md` (planned): the permission set, the duties, and
   the no-other-holder invariant, cross-linking the DR.
3. Record the ground-truth result of Verify rows 4 and 5 (confirmed / provisioning-required) in the
   Evidence section, and if provisioning is required, name it as the ask to the driver.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `test -f docs/streams/desk-supervision/workflow-app-scope.md` | exit 0 | check |
| 2a | `grep -q -E 'workflows: write' docs/streams/desk-supervision/workflow-app-scope.md` | exit 0 (one dedicated pattern per invocation — `grep -c` counts matching LINES not matching patterns, so a single correctly-phrased scope sentence false-fails a `≥3` line-count threshold; #1228 review) | check |
| 2b | `grep -q -E 'contents: write' docs/streams/desk-supervision/workflow-app-scope.md` | exit 0 | check |
| 2c | `grep -q -E 'pull_requests: write' docs/streams/desk-supervision/workflow-app-scope.md` | exit 0 | check |
| 2d | `grep -q -E 'metadata: read' docs/streams/desk-supervision/workflow-app-scope.md` | exit 0 | check |
| 3 | `grep -n -i 'workflow App' docs/adopting-assay.md` | ≥ 1 match | check |
| 4 | `statusgen --consumers --root . --brief desk-supervision/10` | exit 0 (consumers routing corroborated against the diff) | check:ci |
| 5 | (with the workflow App's own token) `gh api /installation/permissions` — read the granted permission set | exit 0; permissions are exactly `contents=write`, `workflows=write`, `pull_requests=write`, `metadata=read` — the set the duty requires — AND none of `administration`, `actions`, `checks`/`statuses=write`, `members`, secrets/variables is present | gate:human +dereference |
| 6 | for each desk-role App token, `gh api /installation/permissions` | exit 0; NONE reports `workflows=write` — the workflow App is the sole holder of that one permission (the other three in row 5 are not sole-holder-checked; only `workflows: write` is) | gate:human +dereference |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 4 and 5 record the
     ground-truth outcome: confirmed, or provisioning-required (a could-not-check that names the
     human provisioning ask). -->

## Review
Gate: human (from frontmatter — sensitive-data is yes). Human gate is MANDATORY: the reviewer
confirms the scope is exactly as stated and no wider, and that exactly one identity holds it.
Reviewer records verdict + date in the stream README table.
