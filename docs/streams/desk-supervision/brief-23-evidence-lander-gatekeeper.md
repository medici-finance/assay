---
brief: assay:assay:desk-supervision:23
title: Evidence lands on main behind a file-scoped gatekeeper — validator workflow + lander App
why: >-
  On a repository whose main requires a pull request, every Evidence landing is its own PR that pays
  the full fixed cost (CI, a reviewer approval, auto-merge), and those PRs crowd a saturated review
  queue and runner pool. Batching cuts the count but not the per-PR cost. Landing Evidence-only
  changes directly on main, behind a validator that admits nothing else and a dedicated lander
  identity that is the only one allowed to skip the PR, removes that cost for the one change class
  that carries no new logic.
wave: 2
depends: ["desk-supervision/11"]
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
gate-why: >-
  Adds a new App identity with a branch-protection bypass, restructures the rulesets on the default
  branch, and adds two workflows that hold a write credential: an identity/auth change on the
  supply-chain surface (sensitive-data). A wrong scope lets unreviewed content reach a PUBLIC main,
  where disclosure cannot be recalled and force-push is denied (irreversible). The human confirms
  the lane design (where content scope is enforced, dedicated vs reused App), performs the App,
  environment and ruleset changes, and reads them back.
decision-trigger: creation
issues: [1588]
schema: brief-v2
version: 1
id: db2a1264-d0bd-4540-aa1d-fab9194e7ed9
authored: 2026-09-23 by worker-desk authoring session
exec-tier: strong
exec-tier-why: >-
  (a) the landing race, re-validation and fallback shape are design not fully pre-specified;
  (b) correctness is the cross-artifact argument that the ruleset, the validator, the lander
  workflow and the post-land audit are independent layers; (c) auth/branch-protection plumbing
  where a subtle scope bug survives a happy-path test.
domain: complicated
sources:
  - "#1588 — the request this brief carries: validator workflow, lander App, verifier App with no main write, fallback to the batch PR path; its four fail-first Done rows are Verify rows 12-15 here."
  - "#1568 — batching Evidence landings into one PR per window; it names this direct lane as the longer-term follow-on. Batching stays the path until this brief is live."
  - "#1565, #586 — the evidence-automerge fixed-cost and self-heal defects that make the per-PR Evidence path expensive."
  - "[DR-server-controls](../decisions/DR-server-controls.md) — the required-status-check pattern and its four conditions (non-author identity, base-repo execution context, protected source, custody of read data); the validator is built to them."
  - ".github/workflows/evidence-automerge.yml — the existing Evidence-PR lane (the fallback), and the precedent for a workflow that reads a change as data and never executes its tree."
  - ".github/workflows/assay-statusgen.yml — the board-writer App precedent: an App on the ruleset bypass lists whose only write is one generated file on main."
  - "docs/protected-paths.md and tools/desk/cmd/deskpathguard/verifysection.go — the existing diff-hunk section detector (a heuristic, per its own header), reused as a starting point but not as the validator."
  - "live read 2026-09-23: `gh api repos/medici-finance/assay/rules/branches/main` — see facts."
  - "freshness-checked 2026-09-23 @ 2a5c230ef"
consumers:
  - "tools/desk/cmd/evidencegate/ (validator scope check + lander decision, with fixtures and mutation script): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation adds it)"
  - "statusgen evidence-audit subcommand (post-land audit, statusgen/evidenceaudit.go + test): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation adds it)"
  - ".github/workflows/evidence-scope.yml (new validator workflow): follow-up desk-supervision/23 (this brief; lands through the workflow-only PR path, flips to fixed-here then)"
  - ".github/workflows/evidence-lander.yml (new lander workflow): follow-up desk-supervision/23 (this brief; lands through the workflow-only PR path, flips to fixed-here then)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the PR-required-main Evidence section gains the direct lane and its fallback): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation edits it)"
  - "docs/adopting-assay.md (branch-protection section: the optional lander App next to the board-writer bypass): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation edits it)"
  - ".github/workflows/evidence-automerge.yml: out-of-scope (the fallback reuses the existing Evidence-PR lane unchanged; Verify row 6 pins that it is untouched)"
  - "the withheld-content status poster, operated outside this repository: out-of-scope (an operator act; until it posts its status on evidence-landing/** heads the ruleset refuses every landing, so the lane fails closed, not open)"
---

# Brief 23 — Evidence lands on main behind a file-scoped gatekeeper

## Context

files:
- **add** `tools/desk/cmd/evidencegate/` (planned) — `main.go` (deskkit contract: kill switch
  first, one audit line, exit 0 ok · 3 disabled · 5 refused · 6 unverifiable), `scope.go` (the
  validator's decision), `lander.go` (the lander's decision), `scope_test.go` (planned),
  `lander_test.go` (planned), fixtures under its `testdata/`, and
  `tools/desk/cmd/evidencegate/testdata/mutate.sh` (planned).
- **add** `statusgen/evidenceaudit.go` (planned) + `statusgen/evidenceaudit_test.go` (planned) —
  the `statusgen evidence-audit` subcommand (post-land audit).
- **add** `.github/workflows/evidence-scope.yml` (planned) (validator) and
  `.github/workflows/evidence-lander.yml` (planned) (lander). These are workflow files: no implementer App
  can push them. They land ONLY through the workflow-only PR path of desk-supervision/11, which is
  why this brief depends on it. Never as a staged copy for hand-landing.
- **edit** `plugins/assay/skills/verify-desk/SKILL.md` (§ "Public repo (PR-required main)") and
  `docs/adopting-assay.md` (branch-protection section).
- **add** `changelog/<branch>.md` — the fragment this repo enforces.
- **not files, human acts** (Task step 1): the lander App, its environment secret, the ruleset
  restructure, and the change to the withheld-content status poster.

single-point-of-failure: the lander App's bypass of the PR-required rule is the ONE grant that lets a commit reach main without review — layers behind it: (1) the required `leak-sweep` and `evidence-scope` statuses sit in a ruleset the lander does NOT bypass, so the server refuses any lander push whose SHA was not swept and validated on a staging ref; (2) the lander workflow lands only a fast-forward of a validated SHA, from a default-branch definition, with a key only a default-branch run can read; (3) the lander App's permission ceiling (contents on this one repo; no workflows, no administration); (4) the post-land `statusgen evidence-audit` on main re-derives Evidence-only-ness with a different algorithm and halts the lane on a mismatch.

facts:
- Live ruleset read (2026-09-23, `gh api repos/medici-finance/assay/rules/branches/main`): two
  branch rulesets on main. `protect-main` carries `deletion`, `non_fast_forward` and a
  `pull_request` rule (1 approval, last-push approval, dismiss stale). `leak-sweep` carries
  `deletion`, `non_fast_forward`, a `required_status_checks` rule (`leak-sweep`) **and its own
  `pull_request` rule (0 approvals)**. So a lander that does not bypass the `leak-sweep` ruleset is
  refused as "PR required" by that ruleset. The PR rule and the required statuses must be split
  into different rulesets before the lander can bypass one and stay bound by the other.
- A ruleset's required status checks also bind direct pushes: `assay-statusgen.yml` records the
  board-writer App needing bypass on both rulesets because a plain push is rejected `GH013`
  (PR-required + leak-sweep-required). A status must therefore already be green on the exact SHA,
  earned on another ref, before that SHA can be pushed to main. The staging ref
  `evidence-landing/<stream>-<NN>-<YYYYMMDD>` is that other ref.
- What an Evidence landing writes today (batch Evidence PR, 2026-09-23): additions inside the
  `## Evidence` section of `docs/streams/<stream>/brief-*.md` files, plus appended lines in
  `docs/streams/verify-outcomes.jsonl`. It deletes nothing and edits no other line. The v1
  allow-list is exactly those two path shapes. A landing that also needs a stream README cell
  change keeps the PR path.
- `deskevidence` commits through the Contents API, refuses `main` unless `VERIFIER_MAIN_OK=1`,
  and writes only under `docs/streams/`. That refusal is client-side and advisory. The server-side
  control is the ruleset (the verifier App is on no bypass list today).
- The withheld-content (`leak-sweep`) status is posted from outside this repository against PR
  heads. A staging ref with no PR gets no status until the operator extends the poster, and the
  ruleset then refuses the landing. Fail-closed, never open.
- The design-approval gate: a `gate: human` brief cannot move to `in-progress` without an approved
  design record cited in `design:`. The ruling on the decision below is recorded as that record
  before pickup.

layering: two decision cores, each a pure function over data (changed-file list, pre-image and
post-image text, statuses, halt state) and tested with no forge: `scope` (admit / reject /
could-not-check) and `lander` (land / remerge / fallback / refuse). The workflows are thin
adapters that gather the data and act. The audit is a separate implementation in a separate module
(statusgen's brief parser, comparing section hashes). A bug in the validator's hunk-to-section
mapping therefore does not also blind the audit. Rows 1-4 test the cores. Rows 9-15 test the live
boundary.

## Human decision
<!-- decision-trigger: creation — options enumerable now; filed as the brief lands. Self-contained. -->
On this repository every Evidence landing (a verifier adding result rows to a work item's Evidence
section, plus one line in an outcomes log) is its own pull request. Each needs CI, a reviewer
approval and an auto-merge, and these PRs fill a saturated review queue and runner pool. The
proposal: Evidence-only changes land on the default branch directly, with no PR, through a
dedicated "lander" identity. The lander is the only identity allowed to skip the PR requirement,
and a validator admits only Evidence-only changes. The verifier identity keeps no write to the
default branch. A rejected landing files a finding and falls back to today's batched PR path.

This needs a human because it creates a new App, gives it a branch-protection bypass, restructures
the branch rulesets and adds workflows that hold a write credential. A wrong scope would let
unreviewed content reach a public default branch.

What is being decided:
1. Whether to adopt the direct Evidence lane at all.
2. Where the "Evidence-only" rule is enforced. (A) By the server: the validator's verdict becomes a
   required status in a ruleset the lander does not bypass. A stolen lander key can then land only
   validated content. The cost is one more required check on every PR. (B) By the lander workflow
   only: the server enforces identity and the withheld-content sweep, and content scope is checked
   by the lander's own code plus an after-the-fact audit. This is cheaper, but content scope is no
   longer server-enforced.
3. A dedicated lander App, or reuse of the existing board-writer App, which already bypasses both
   rulesets to write the generated board.

Options:
1. **Adopt, server-enforced scope (A), dedicated lander App.** Split the PR rule from the required
   statuses. The lander bypasses only the PR rule. Recommended: every layer fails for a different
   reason in a different component.
2. **Adopt, workflow-enforced scope (B), dedicated lander App.** No extra required check on
   ordinary PRs. Content scope rests on the lander's code and the audit.
3. **Adopt, reuse the board-writer App.** No new App, but one bypass identity carries two lanes,
   and revoking one revokes both.
4. **Do not adopt.** Keep the batched Evidence PR as the only path.

Default if no answer: none. The brief blocks until answered. Batching stays the path meanwhile.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. The deliverable is a draft PR
  opened by the desk verbs. The workflow files ride the workflow-only PR path.
- Stop at `implemented`. You do not set verified/done.
- Never add any identity to a bypass list, never edit a ruleset, never create an App. Those are
  the human acts in Task step 1. If one is missing, the live rows are could-not-check, never faked.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Human provisioning (performed by a human, read back by Verify rows 9-11).** Create the lander
   App: contents read/write and metadata read only, installed on this one repository. Store its key
   as a secret of an environment `evidence-lander` whose deployment policy admits only the default
   branch. Restructure the rulesets so the PR rule and the required statuses live in different
   rulesets. The lander bypasses only the PR rule. `leak-sweep` and (Option 1) `evidence-scope` are
   required in a ruleset the lander does not bypass, each pinned to its expected reporting source.
   The board-writer App keeps its existing bypass. The operator extends the withheld-content poster
   to `evidence-landing/**` heads.
2. **Validator core** (`evidencegate scope`). Input: the exact main tip the landing would
   fast-forward from, and the staging SHA. Admit only when the staging SHA descends from that tip
   and every changed path is on the allow-list. Each brief-file change must be pure additions whose
   every added line falls inside the `## Evidence` section. Compute the section boundaries
   byte-exactly from the pre-image file, not from hunk context: the existing
   `deskpathguard` detector is a heuristic by its own statement. Each `verify-outcomes.jsonl`
   change must be a pure append of lines that parse as JSON objects. Anything else is `reject`,
   naming the offending path and line. An unreadable input is `could-not-check`, which is never
   admit.
3. **Validator workflow** (`evidence-scope.yml`). Run on pushes to `evidence-landing/**` (and,
   under Option 1, on pull requests, reporting "not a lander landing" as a pass for any head the PR
   rule governs). It checks the validator source out from the DEFAULT branch only and reads the
   staging change as data. It never executes the staging tree and never uses
   `pull_request_target`. It also runs `statusgen --lint` on the staging tree and posts
   `evidence-scope` under the pinned reporting identity.
4. **Lander core + workflow** (`evidencegate lander`, `evidence-lander.yml`). Trigger on
   `workflow_run` of the validator, so the definition always comes from the default branch, in
   environment `evidence-lander`. Land only a fast-forward of a SHA whose `evidence-scope` and
   `leak-sweep` are green. If main moved, merge current main into the staging ref (two parents,
   never rebase) and let the cycle re-validate, up to a fixed attempt cap. On `reject`,
   `could-not-check` or the cap: no push; file a finding naming the staging ref; keep the ref for
   the verify desk's batch Evidence PR. While any open finding carries the lane's halt label:
   `refuse` everything.
5. **Post-land audit** (`statusgen evidence-audit`, run by the validator workflow on pushes to
   main). For every lander-authored commit, compare the normalized hash of each touched brief's
   non-Evidence content and each prior outcomes line, before and after. On a mismatch, file a
   finding carrying the halt label. This is detection plus halt. It never reverts on its own.
6. **Skill + adopter docs.** The verify-desk PR-required-main section gains the direct lane
   (write Evidence to a staging ref; the lander lands it; fallback = today's batch Evidence PR from
   that same ref). `docs/adopting-assay.md` documents the optional lander App next to the
   board-writer bypass.
7. **Fail-first.** Before claiming rows 1-4, show each red on the unfixed code (the mutation
   script for row 1; a pre-implementation run for rows 3-4) in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/evidencegate/ -run 'TestScope' -count=1 -timeout 180s` | exit 0. Planted-mutation fixtures, each `reject`: a Verify-table cell edit, a frontmatter edit, a prose edit outside `## Evidence`, a deleted Evidence row, an off-list path, a rewritten `verify-outcomes.jsonl` line, a non-descendant staging SHA, an empty diff. An appended Evidence row and an appended outcome line are `admit`. An unreadable diff is `could-not-check` | check:ci +mutation |
| 2 | `bash tools/desk/cmd/evidencegate/testdata/mutate.sh` | exit 0 and prints `MUTANT KILLED`. The script makes the section check treat every hunk as in-section, re-runs row 1, and requires it to FAIL: the fail-first proof that row 1 bites | check:ci +mutation |
| 3 | `cd statusgen && go test . -run 'TestEvidenceAudit' -count=1 -timeout 180s` | exit 0. LOWER LAYER WITH THE UPPER BYPASSED: a fixture lander commit on main edits a Verify table and carries NO validator verdict, and the audit reports `checked-failed` and emits the halt finding. An Evidence-only lander commit is `checked-clean`. An unreadable commit is `could-not-check`, never clean | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/evidencegate/ -run 'TestLanderDecision' -count=1 -timeout 180s` | exit 0. `reject` and `could-not-check` both give `fallback` (no push, a finding naming the staging ref, the ref kept). An open halt finding gives `refuse` for every input. A moved main tip gives `remerge` up to the cap, then `fallback`. A green validator plus a missing `leak-sweep` gives no land | check:ci +mutation |
| 5 | `grep -qE '^[[:space:]]+workflow_run:' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+environment:[[:space:]]*evidence-lander' .github/workflows/evidence-lander.yml && grep -qF 'github.event.repository.default_branch' .github/workflows/evidence-scope.yml && ! grep -qF 'pull_request_target' .github/workflows/evidence-scope.yml .github/workflows/evidence-lander.yml` | exit 0 (lander runs from the default-branch definition in its environment; validator source from the default branch; no `pull_request_target`) | check |
| 6 | `git diff $(git merge-base refs/remotes/origin/main HEAD)..HEAD -- .github/workflows/evidence-automerge.yml .github/workflows/leaksweep-control.yml .github/workflows/leaksweep-pattern.yml` | empty (the fallback lane and the leak workflows are untouched). The base is spelled `refs/remotes/origin/main` in full so that a stray local `origin/main` branch cannot become the comparison base | check +neighbour |
| 7 | `statusgen --consumers --root . --brief desk-supervision/23` | exit 0 (consumers routing corroborated against the diff) | check:ci |
| 8 | `statusgen --root . --lint` | exit 0 | check:ci |
| 9 | `gh api repos/medici-finance/assay/rules/branches/main --jq '[.[] \| select(.type=="required_status_checks") \| .parameters.required_status_checks[]]'` | contains `leak-sweep` and (Option 1) `evidence-scope`, each carrying an `integration_id` that names its expected reporting source | gate:human +dereference |
| 10 | `for id in $(gh api repos/medici-finance/assay/rulesets --jq '.[].id'); do gh api repos/medici-finance/assay/rulesets/$id --jq '{name, rules: [.rules[].type], bypass_actors}'; done` (admin credential) | The lander App is a bypass actor ONLY on the ruleset holding the `pull_request` rule, and that ruleset holds no `required_status_checks`. The verifier App is on no bypass list. The ruleset holding the required statuses lists only the pre-existing board-writer App | gate:human +dereference |
| 11 | `gh api repos/medici-finance/assay/environments/evidence-lander --jq '.deployment_branch_policy'` | admits the default branch only (`protected_branches: true`, or a custom policy naming only it): a run triggered from a staging ref cannot read the lander key | gate:human +dereference |
| 12 | DONE (a), planted mutation: the verify desk pushes a commit editing ONE Verify-table cell of this brief to `evidence-landing/example-canary`, then `gh api "repos/medici-finance/assay/commits/$(git rev-parse refs/remotes/origin/evidence-landing/example-canary)/status" --jq '.statuses[] \| select(.context=="evidence-scope") \| .state'` | `failure`. `git ls-remote origin refs/heads/main` is unchanged from before the push, and a finding naming `evidence-landing/example-canary` is filed (fallback, nothing dropped) | gate:human +mutation |
| 13 | DONE (b): under the implementer App credential, `git push origin refs/remotes/origin/evidence-landing/example-canary:refs/heads/main` | exit non-zero with `GH013` in stderr. The main tip is unchanged | gate:human |
| 14 | DONE (d), LOWER LAYER WITH THE UPPER BYPASSED: under the verifier App, with one real outcome line appended locally, `VERIFIER_MAIN_OK=1 deskevidence medici-finance/assay main --evidence-file docs/streams/verify-outcomes.jsonl`. The client-side refusal is deliberately switched off and the content is admissible Evidence, so only the server's identity scope can refuse | exit non-zero, and the forge refuses the write (ruleset violation). The main tip is unchanged. If it lands, the row FAILS: the line is a real outcome row, so the harm is bounded and the failure is itself the finding | gate:human +mutation |
| 15 | DONE (c) + FLOW: after a real Evidence-only landing through a staging ref, with `LANDED_SHA` set to the new main tip: `gh api "repos/medici-finance/assay/commits/$LANDED_SHA/pulls" --jq 'length'` | `0` (no PR), and the commit's author is the lander App. Fail-first: the same push attempted before Task step 1 was refused `GH013`, recorded in Evidence | gate:human +flow |
| 16 | `grep -qF 'evidence-landing/' plugins/assay/skills/verify-desk/SKILL.md && grep -qiF 'lander' docs/adopting-assay.md` | exit 0 (the skill and the adopter doc name the lane) | check |

Pre-mortem (failure mode → row):

| Failure mode | Caught by |
|---|---|
| Validator admits a Verify-table or frontmatter edit | rows 1-2 (and row 3 after the fact) |
| Validator skipped or buggy, and bad content lands anyway | row 3 (audit + halt); row 9 (server-required status, Option 1) |
| Verifier App gains a direct main write | rows 10, 14 |
| Lander key readable from a staging-ref run | rows 5, 11 |
| Landing bypasses the withheld-content sweep | rows 9-10 (lander not on that ruleset); row 4 (no land without `leak-sweep`) |
| Rejected landing silently dropped | rows 4, 12 |
| Rebase or force on race | row 4 (`remerge` is a two-parent merge); `non_fast_forward` stays on both rulesets (row 10) |
| Ruleset later widened by hand | no row. Review-only: re-audit is DR-server-controls' standing commitment |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 9-15 need Task step 1 done;
     until then they are could-not-check, never pass. -->

## Review
Gate: human (from frontmatter: sensitive-data and irreversible are yes). The human gate is MANDATORY.
The reviewer answers BOTH, in the verdict:
1. What is the single control standing between the fault and the damage, and is that acceptable?
   (The SPOF line above names the lander's PR-rule bypass. Confirm the layers behind it are
   present at the chosen option.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed? (Rows
   3 and 14 are designed to. Confirm they bypass the upper layer rather than walking the happy path
   through every layer at once.)

Reviewer records verdict + date in the stream README table.
