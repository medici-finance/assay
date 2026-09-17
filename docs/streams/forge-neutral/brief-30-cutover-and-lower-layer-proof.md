---
brief: assay:assay:forge-neutral:30
title: Cutover — release, re-pin, drain and switch the store, prove on a fixture, and only then the operator narrows the grant
why: >-
  A grant narrowed before the tools accept it stops the review desk; a store switched while
  another dispatcher is live double-dispatches; and a grant never narrowed leaves the work
  without effect. The cutover needs one written order with the operator's grant change as its
  last step, and one proof — run by someone who did not build it — that a reviewer at
  repository read boots, dispatches, and cannot push.
wave: 6
depends: ["forge-neutral/22", "forge-neutral/23", "forge-neutral/25", "forge-neutral/29"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "the spec's §7 (drain, not merge) and §11 (order of operations)"
  - "tools/desk/internal/deskkit/preflight.go:835-838 — after a permission change the cached token carries the old grant; re-mint fresh"
  - "docs/UPGRADING.txt and docs/release-notes/ — where a release-bound operator step is announced"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): the proof spans the resolver, the readers, the boot check, the forge and the adopter guide at one pinned release"
gate-why: >-
  This brief is the point at which a role identity's grant actually changes on real
  installations. The grant change is a human act and is NOT performed by this brief or any
  tool. The human confirms the fixture proof is genuine (a credential that truly holds no
  repository write; a forge-side refusal), that release, re-pin and drain preceded it, and
  then decides when each installation is narrowed.
decision-trigger: start
domain: complicated
consumers:
  - "docs/UPGRADING.txt: follow-up forge-neutral/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/release-notes/ (the entry for the release carrying store-aware duties): follow-up forge-neutral/30 (this brief; flips to fixed-here when the implementation edits the path)"
  - "each adopter's reviewer App permission set or GitLab settings: out-of-scope (a human, admin-side act per installation; no tool in this tree performs it)"
version: 1
id: 183da2db-7fc2-4834-ac56-11d3c576a431
---

# Brief 30 — Cutover and the lower-layer proof

## Context
files:
- `docs/UPGRADING.txt`, `docs/release-notes/<release>.md` (planned) — the operator step.
- `docs/streams/forge-neutral/reviewer-write-boundary.md` — §11 gains the dated proof table.
- `changelog/<branch-slug>.md` (planned)

This brief writes no tool code and changes no grant.

single-point-of-failure: the written order (drain before switch; grant change last) — behind
it, the tools themselves: a store switched beside a live dispatcher is refused by the
mixed-store check (brief 23); a reviewer narrowed too early under `forge-ref` is refused at
boot naming both remedies (brief 25) and again at dispatch (brief 21). A wrong order produces a
loud stop, not a silent gap; restoring the grant or reversing the drain is the rollback.

facts:
- Order (spec §11): tools accept both states → release → re-pin → per cell: drain, set the
  store key on every dispatching process, resume → fixture proof → **the operator narrows the
  grant**. Releasing and re-pinning are existing human-gated processes; this brief waits for
  them.
- Narrowing: `file` / `service` cell → reduce the reviewer to repository read. `forge-ref` cell
  → apply the server-side bound (briefs 26 / 27); the grant itself stays.
- After any permission change, re-mint fresh before booting.
- The served store (brief 24) is not a precondition of narrowing a single-host cell.
- The proof is run by someone who did not implement briefs 21, 22, 23 or 25.

## Human decision
The tools have been changed so the reviewing identity no longer needs write access to the
repository when a cell keeps its dispatch claims off the code-hosting platform, and a release
containing that change is installed. What remains is the change to the identity's actual
access on each real installation. A person with administrator rights makes that change by
hand, and it is the last step. Before it, a test installation is used to show that a reviewing
identity with read-only access starts correctly, can dispatch reviews, and is refused by the
platform if it tries to push. The decision is whether that evidence is sufficient and when
each installation is narrowed.

Options:
1. **Narrow single-computer cells now**; cells that must keep claims on the platform get the
   platform-side rules instead.
2. **Wait until the claim service has shipped**, then handle every cell in one pass.
3. **Do not narrow yet** — the tools support both states.

Default if no answer: option 3 — nothing changes until a person acts.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- NEVER change a permission, ruleset or protection on any real installation. The proof runs on a throwaway fixture; the real change is the human's, after this brief is verified.
- Do not start until a release containing briefs 21, 22, 23 and 25 exists and the fixture is pinned to it; row 2 checks.

## Task
1. Write the operator step into `docs/UPGRADING.txt` and the release notes: preconditions, the
   drain, the per-store narrowing, the fresh re-mint, the rollback, and the sentence that no
   tool performs the grant change.
2. Build the fixture at the pinned release.
3. Run the proof rows; record command, outcome and date in the spec's §11 proof table.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check | On the fixture: `deskpreflight --version` and `deskclaim-ref --version` | both print a `releaseTag` that is a published release, not `dev` |
| 3 | check +flow +dereference | **FIXTURE ONLY.** Cell on `file`, reviewer App at repository read, fresh re-mint; boot the reviewer role | `app-scopes-vs-duties` checked-clean; detail carries `claim store: file` and the declared-not-verified notice |
| 4 | check +flow | **FIXTURE ONLY.** Same cell: dispatch a review of an open change; then `deskclaim-ref show fixture--issue-1` and `desksupervise status` | claim-acquire OK with `store file`; both readers show the holder; `git ls-remote` of the fixture shows no claim ref |
| 5 | check +flow | **FIXTURE ONLY.** Same reviewer credential, plain git: `git push origin HEAD:refs/heads/probe-cutover` | REFUSED by the forge for lack of write access, quoting the forge's text |
| 6 | check | **FIXTURE ONLY.** While a live forge claim exists for the repo, set the store key to `file` and dispatch | refused before any worktree, naming the live forge claim and the drain — the half-done migration is loud |
| 7 | check | **FIXTURE ONLY.** Cell on `forge-ref`, reviewer at repository read, boot | refused at boot naming both remedies — the wrong-order case is loud |
| 8 | check | **FIXTURE ONLY.** Restore the grant, re-mint fresh, boot and dispatch under `forge-ref` | clean; rollback needs no code change |
| 9 | check +flow | **FIXTURE ONLY.** Cell on `forge-ref` with brief 26's pair applied: branch push under the reviewer credential; then a claim acquire | push REFUSED by the forge; acquire exit 0 (spec V7 re-run at the pinned release). could-not-check if brief 26 has not shipped — say so, do not skip silently |
| 10 | check | `grep -n -i 'no tool performs' docs/UPGRADING.txt` | at least one hit inside the operator step |
| 11 | check | `(cd statusgen && go build -o /tmp/statusgen-fn30 .) && /tmp/statusgen-fn30 --root . --consumers --brief forge-neutral/30` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The proof runs on a dev build | row 2 |
| Row 5's refusal comes from a desk guard | row 5 requires plain git and the forge's text |
| A cell is switched while another dispatcher is live | row 6 |
| An operator narrows a forge-ref cell | row 7 shows a loud refusal; the runbook order; row 10 |
| Rollback needs an undocumented step | row 8 |
| This brief's implementer narrows a real installation "to finish the job" | Ground rules; review — the diff contains documents only |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; the cutover of a role identity's grant). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a mis-ordered cutover and a dark review desk or a double dispatch? (The written order; beneath it the mixed-store refusal and the boot and dispatch refusals that name the remedy.)
2. Does any row prove the lower layer with the upper bypassed? (Row 5: no desk tool in the path. Rows 6 and 7: the order deliberately violated.)
