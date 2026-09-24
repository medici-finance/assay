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
  Adds new App identities (a lander with a branch-protection bypass and, under Option 1, a
  validator that posts a required status), restructures the rulesets on the default branch, and
  adds three workflows, two of which hold a write credential: an identity/auth change on the
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
  (a) the landing race, re-validation, wait window and fallback shape are design not fully
  pre-specified; (b) correctness is the cross-artifact argument that the ruleset, the validator,
  the lander workflow and the post-land audit are independent layers; (c) auth/branch-protection
  plumbing where a subtle scope bug survives a happy-path test.
domain: complicated
sources:
  - "#1588 — the request this brief carries: validator workflow, lander App, verifier App with no main write, fallback to the batch PR path; its four fail-first Done rows are Verify rows 12-15 here."
  - "#1568 — batching Evidence landings into one PR per window; it names this direct lane as the longer-term follow-on. Batching stays the path until this brief is live."
  - "#1565, #586 — the evidence-automerge fixed-cost and self-heal defects that make the per-PR Evidence path expensive."
  - "[DR-server-controls](../decisions/DR-server-controls.md) — the required-status-check pattern and its four conditions (non-author identity, base-repo execution context, protected source, custodied evidence). Under Option 1 the `evidence-scope` status is built to all four: posted only by a dedicated validator App, pinned in the ruleset by that App's integration id; posted only from a `workflow_run` run of the default-branch definition; its inputs re-read from the forge and the default-branch checkout, never from the triggering run or the staging tree."
  - ".github/workflows/evidence-automerge.yml — the existing Evidence-PR lane (the fallback), and the precedent for a workflow that reads a change as data and never executes its tree."
  - ".github/workflows/assay-statusgen.yml — the board-writer App precedent: an App on the ruleset bypass lists whose only writes on main are generated board surfaces (`STATUS.md` and stream README status cells)."
  - "docs/protected-paths.md and tools/desk/cmd/deskpathguard/verifysection.go — the existing diff-hunk section detector (a heuristic, per its own header), reused as a starting point but not as the validator."
  - "live read 2026-09-23: `gh api repos/medici-finance/assay/rules/branches/main` — see facts."
  - "freshness-checked 2026-09-23 @ 2a5c230ef"
consumers:
  - "tools/desk/cmd/evidencegate/ (validator scope check + lander decision, with fixtures and mutation script): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation adds it)"
  - "statusgen evidence-audit subcommand (post-land audit, statusgen/evidenceaudit.go + test): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation adds it)"
  - ".github/workflows/evidence-scope.yml (new event-facing validator workflow, no credential; also runs the post-land audit): follow-up desk-supervision/23 (this brief; lands through the workflow-only PR path, flips to fixed-here then)"
  - ".github/workflows/evidence-scope-post.yml (new status poster, Option 1 only): follow-up desk-supervision/23 (this brief; lands through the workflow-only PR path, flips to fixed-here then)"
  - ".github/workflows/evidence-lander.yml (new lander workflow): follow-up desk-supervision/23 (this brief; lands through the workflow-only PR path, flips to fixed-here then)"
  - "plugins/assay/skills/verify-desk/SKILL.md (the PR-required-main Evidence section gains the direct lane and its fallback): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation edits it)"
  - "docs/adopting-assay.md (branch-protection section: the optional lander App next to the board-writer bypass): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation edits it)"
  - ".github/workflows/evidence-automerge.yml: out-of-scope (the fallback reuses the existing Evidence-PR lane unchanged; Verify row 6 pins that it is untouched)"
  - "the withheld-content status poster, operated outside this repository: out-of-scope (an operator act; until it posts its status on evidence-landing/** heads the lander waits and then falls back, and the ruleset refuses every landing, so the lane fails closed, not open)"
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
- **add** `.github/workflows/evidence-scope.yml` (planned) (event-facing validator half, holds no
  credential; also runs the post-land audit), `.github/workflows/evidence-scope-post.yml` (planned)
  (Option 1: the status poster) and `.github/workflows/evidence-lander.yml` (planned) (lander).
  These are workflow files: no implementer App can push them. They land ONLY through the
  workflow-only PR path of desk-supervision/11, which is why this brief depends on it. Never as a
  staged copy for hand-landing.
- **edit** `plugins/assay/skills/verify-desk/SKILL.md` (§ "Public repo (PR-required main)") and
  `docs/adopting-assay.md` (branch-protection section).
- **add** `changelog/<branch>.md` — the fragment this repo enforces.
- **not files, human acts** (Task step 1): the lander App, (Option 1) the validator App, their
  environment secrets, the ruleset restructure, and the change to the withheld-content status
  poster.

single-point-of-failure: the lander App's bypass of the PR-required rule is the ONE grant that lets a commit reach main without review — layers behind it: (1) the required `leak-sweep` and (Option 1) `evidence-scope` statuses sit in a ruleset the lander does NOT bypass, each pinned to its own dedicated App's integration id; `evidence-scope` is posted only by the validator App from a default-branch run, and reads `success` only as a staging-ref `admit` or as the not-a-landing pass on a PR head approved at that exact head, so the server refuses any lander push whose SHA is neither validated Evidence nor an approved PR head; (2) the lander workflow, from a default-branch definition with a key only a default-branch run can read, acts only on the current head of an `evidence-landing/**` ref, re-runs the `scope` core itself instead of trusting the status, and lands only a fast-forward on its own `admit` (this layer is independent of a forged or misapplied status, not of a bug in the shared `scope` core; layer 4 is the one independent of that code); (3) the lander App's permission ceiling (contents + metadata on this one repository; no workflows, no administration); every claim this brief makes about the lander key is about main, since a contents permission also reaches tags and releases; (4) the post-land `statusgen evidence-audit` selects by the PUSH, never by commit author (every push to main by the lander, and every push carrying a commit with no associated PR), re-derives Evidence-only-ness per commit with a different algorithm, and halts the lane on a mismatch.

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
- Commit statuses are keyed by SHA and context, not by ref (GitHub REST docs, read 2026-09-23). A
  status on a SHA that is also a PR head satisfies a required check for ANY push of that SHA, so the
  lander must bind each landing to a staging ref's current head itself.
- Which definition a run executes (GitHub Actions docs, read 2026-09-23): `push`, `pull_request`
  and `pull_request_review` runs execute the triggering ref's own workflow file; `workflow_run`,
  `status` and `schedule` runs execute the default-branch file. A status posted with a workflow's
  `GITHUB_TOKEN` carries the generic GitHub Actions integration, which every workflow in the
  repository shares; a status posted by an App carries that App's integration. A status post fires
  the `status` event (unless posted with `GITHUB_TOKEN`) and never fires `workflow_run`.
- A fast-forward push adds no commit. The landed commits keep the author and committer of whoever
  wrote them to the staging ref: the verifier App, since `deskevidence` commits through the
  Contents API with the App's installation token (`tools/desk/cmd/deskevidence/main.go` header).
  Who pushed is recorded on the push, not the commit: the `push` event's `sender`, and the
  repository activity API (`repos/{owner}/{repo}/activity`, one entry per ref update: `actor`,
  `activity_type`, `before`, `after`).
- What an Evidence landing writes today (batch Evidence PR, 2026-09-23): additions inside the
  `## Evidence` section of `docs/streams/<stream>/brief-*.md` files, plus appended lines in
  `docs/streams/verify-outcomes.jsonl`. It deletes nothing and edits no other line. The v1
  allow-list is exactly those two path shapes. A landing that also needs a stream README cell
  change keeps the PR path.
- `deskevidence` commits through the Contents API, refuses `main` unless `VERIFIER_MAIN_OK=1`,
  and writes only under `docs/streams/`. That refusal is client-side and advisory. The server-side
  control is the ruleset (the verifier App is on no bypass list today).
- The withheld-content (`leak-sweep`) status is posted from outside this repository against PR
  heads, asynchronously, on its own polling interval. A staging ref with no PR gets no status until
  the operator extends the poster, and the ruleset then refuses the landing. Fail-closed, never
  open.
- The design-approval gate: a `gate: human` brief cannot move to `in-progress` without an approved
  design record cited in `design:`. The ruling on the decision below is recorded as that record
  before pickup.

layering: two decision cores, each a pure function over data (per-commit changed-file lists,
pre-image and post-image text, statuses with their creators, the staging ref's current head,
wait age, halt state) and tested with no forge: `scope` (admit / reject / could-not-check) and
`lander` (land / remerge / wait / fallback / refuse / skip). The workflows are thin adapters that
gather the data and act. The event-facing workflow holds no credential; only default-branch runs
hold a key. The audit is a separate implementation in a separate module (statusgen's brief parser,
comparing section hashes). A bug in the validator's hunk-to-section mapping therefore does not also
blind the audit. Rows 1-4 test the cores. Rows 9-15 and 17-18 test the live boundary.

## Human decision
<!-- decision-trigger: creation — options enumerable now; filed as the brief lands. Self-contained. -->
On this repository every Evidence landing (a verifier adding result rows to a work item's Evidence
section, plus one line in an outcomes log) is its own pull request. Each needs CI, a reviewer
approval and an auto-merge, and these PRs fill a saturated review queue and runner pool. The
proposal: Evidence-only changes land on the default branch directly, with no PR, through a
dedicated "lander" identity. The lander is the only identity allowed to skip the PR requirement.
Before landing, the lander re-checks for itself that the change is Evidence-only, and an audit
after every landing re-checks it again with a different method and stops the lane on a mismatch.
The verifier identity keeps no write to the default branch. A rejected landing files a finding and
falls back to today's batched PR path.

This needs a human because it creates new Apps, gives one a branch-protection bypass, restructures
the branch rulesets and adds workflows that hold a write credential. A wrong scope would let
unreviewed content reach a public default branch.

What is being decided:
1. Whether to adopt the direct Evidence lane at all.
2. Where the "Evidence-only" rule is enforced. (A) Also by the server: a validator's verdict
   becomes a required status in a ruleset the lander does not bypass. The status is posted only by
   a second dedicated App, from a run that neither the lander nor the verifier can influence, and
   the server accepts only that App's report. On an ordinary PR the same status reads as a pass
   only once the PR is approved at its current head. A stolen lander key can then put on the
   default branch only validated Evidence or an already-approved PR head. The cost is a second new
   App and one more required check on every PR, which turns green one short run after approval.
   (B) By the lander workflow only: the server enforces identity and the withheld-content sweep,
   and content scope is checked by the lander's own code plus the after-the-fact audit. This is
   cheaper, but content scope is no longer server-enforced.
3. A dedicated lander App, or reuse of the existing board-writer App, which already bypasses both
   rulesets to write the generated board.

The lander's key, like any App key with write access to repository contents, also reaches tags and
releases; this proposal narrows only what reaches the default branch.

Options:
1. **Adopt, server-enforced scope (A), dedicated lander App and dedicated validator App.** Split
   the PR rule from the required statuses. The lander bypasses only the PR rule. Recommended: every
   layer fails for a different reason in a different component.
2. **Adopt, workflow-enforced scope (B), dedicated lander App.** No validator App and no extra
   required check on ordinary PRs. Content scope rests on the lander's code and the audit.
3. **Adopt, reuse the board-writer App as the lander.** No new lander App, but one bypass identity
   carries two lanes, and revoking one revokes both.
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
   App: contents read/write and metadata read only, installed on this one repository. Under
   Option 1 also create the validator App: commit statuses read/write, contents and pull requests
   read, metadata read, installed on this one repository. Store each key as a secret of its own
   environment (`evidence-lander`, `evidence-scope`), each with a deployment policy that admits
   only the default branch. Restructure the rulesets so the PR rule and the required statuses live
   in different rulesets. The lander bypasses only the PR rule. `leak-sweep` and (Option 1)
   `evidence-scope` are required in a ruleset the lander does not bypass, each pinned to its own
   App's integration id (`evidence-scope` to the validator App, never the generic GitHub Actions
   integration). Neither new App and not the verifier App is on that ruleset's bypass list. The
   board-writer App keeps its existing bypass. The operator extends the withheld-content poster to
   `evidence-landing/**` heads.
2. **Validator core** (`evidencegate scope`). Input: the exact main tip the landing would
   fast-forward from, the staging SHA, and every commit in `tip..staging` with its parents. Admit
   only when the staging SHA descends from that tip and EVERY commit in the range is itself in
   scope: a single-parent commit checked against its parent; a two-parent commit only as a lander
   `remerge` whose other parent is on main's first-parent history, checked against that main-side
   parent. A commit is in scope when every changed path is on the allow-list, each brief-file
   change is pure additions whose every added line falls inside the `## Evidence` section, and each
   `verify-outcomes.jsonl` change is a pure append of lines that parse as JSON objects. Compute the
   section boundaries byte-exactly from each commit's pre-image file, not from hunk context: the
   existing `deskpathguard` detector is a heuristic by its own statement. Because every admitted
   commit is additions-only, no later commit can remove what an earlier one added, so the net tree
   carries every line any commit in the range added (the withheld-content sweep of that tree sees
   them all) and the per-commit and net verdicts agree. The audit in step 5 uses the same unit.
   Anything else is `reject`, naming the offending commit, path and line. An unreadable input is
   `could-not-check`, which is never admit.
3. **Validator workflows** (`evidence-scope.yml`; under Option 1 also `evidence-scope-post.yml`).
   The run that sees an event is not the run that may hold a credential, so there are two halves.
   - `evidence-scope.yml` is the event-facing half. It runs on pushes to `evidence-landing/**` and
     (Option 1) on `pull_request` and `pull_request_review`. Those runs execute the triggering
     ref's own definition, so the jobs for those events hold no credential beyond a read-only
     `GITHUB_TOKEN`, post no status, and nothing downstream reads their outputs or artifacts. The
     workflow never uses `pull_request_target`. Its push-to-main job (step 5) is the one job that
     writes, and only `issues: write` for the audit finding.
   - `evidence-scope-post.yml` is the poster. It triggers on `workflow_run` of `evidence-scope.yml`,
     so its definition always comes from the default branch, and runs in environment
     `evidence-scope` with the validator App's key. It re-reads its inputs itself: the SHA and the
     ref or PR it belongs to from the forge API, never from the triggering run's event fields,
     outputs or artifacts. It checks the validator source out from the DEFAULT branch only and reads
     the staging change as data. It never executes the staging tree: it runs `statusgen --lint`
     over the staging tree with a `statusgen` built from the default-branch checkout (or the
     pinned release), never one built from the staging tree. It posts `evidence-scope` as the
     validator App. On the current head of an `evidence-landing/**` ref it posts the `scope`
     verdict (`success` only on `admit`). On a PR head it posts the not-a-landing pass `success`
     only when that PR's review decision reads APPROVED and its head is exactly that SHA, and
     `pending` otherwise; a new push to the PR resets it. Any other SHA gets no status.
   - Under Option 2 there is no poster and no required `evidence-scope`; the lander's own
     re-derivation in step 4 is the scope check.
4. **Lander core + workflow** (`evidencegate lander`, `evidence-lander.yml`). Triggers:
   `workflow_run` of the poster (of `evidence-scope.yml` under Option 2); `status` events for the
   `leak-sweep` and `evidence-scope` contexts, because `leak-sweep` arrives asynchronously from
   outside this repository and a status post never fires `workflow_run`; and a `schedule` sweep of
   open `evidence-landing/**` refs. All three run the default-branch definition, in environment
   `evidence-lander`. Event fields reach a shell only through `env:` assignments, never as `${{ }}`
   inside `run:`; `if:` conditions are written without `${{ }}`; the workflow downloads no artifact
   from any triggering run.
   The lander never trusts the event. For each candidate it re-reads from the forge the current
   head of the `evidence-landing/**` ref it names, the current main tip, and the SHA's statuses
   with each status's creator. It re-runs the pure `scope` core itself, from default-branch
   source, over that tip and SHA; a posted `evidence-scope` is never its only witness. The expected
   status creators and the halt label come from the default-branch checkout, never from the staging
   tree. Outcomes:
   - `skip` — the SHA is not the current head of any `evidence-landing/**` ref (a PR head, a stale
     staging SHA): no push, no finding.
   - `refuse` — any open finding carries the lane's halt label: every input, no push.
   - `land` — its own `scope` verdict is `admit`, AND `leak-sweep` and (Option 1) `evidence-scope`
     are green, each posted by its expected App: push only a fast-forward of that SHA to main.
     A green posted by any other creator counts as missing.
   - `remerge` — main moved: merge current main into the staging ref (two parents, never rebase)
     and let the cycle re-validate, up to a fixed attempt cap.
   - `wait` — a required status is pending or missing (normal while `leak-sweep` has not posted
     yet): no push, no finding, the ref kept. Bounded: once the SHA has waited longer than a fixed
     window (the implementation names the constant, and the schedule sweep runs more often than
     the window), it becomes `fallback`.
   - `fallback` — its own `scope` verdict is `reject` or `could-not-check`, a required status is
     `failure`, the remerge cap is reached, a remerge conflicts, or the wait window expired: no
     push; file one finding naming the staging ref (deduped per ref); keep the ref for the verify
     desk's batch Evidence PR.
5. **Post-land audit** (`statusgen evidence-audit`, run by `evidence-scope.yml` on pushes to
   main; a push to main runs main's own definition). Select by the PUSH, never by a commit's
   author: a fast-forward landing adds no lander-authored commit. The audit inspects every push to
   main whose `sender` is the lander App, AND every push to main whose `before..after` range
   carries a commit with no associated pull request (`commits/{sha}/pulls` empty). The one
   exclusion is a push by the board-writer App whose range touches only its generated board
   surfaces (`STATUS.md` and stream `README.md` files). An unreadable sender or pull association
   selects the push (fail toward auditing). For each selected push, check every commit in
   `before..after` with the same per-commit unit as step 2: compare the normalized hash of each
   touched brief's non-Evidence content and each prior outcomes line, before and after that
   commit, and flag any path off the allow-list. On a mismatch, file a finding carrying the halt
   label. This is detection plus halt. It never reverts on its own. The push selection is data the
   adapter gathers; the audit core stays pure.
6. **Skill + adopter docs.** The verify-desk PR-required-main section gains the direct lane
   (write Evidence to a staging ref; the lander lands it; fallback = today's batch Evidence PR from
   that same ref). `docs/adopting-assay.md` documents the optional lander App (and, under
   Option 1, the validator App) next to the board-writer bypass.
7. **Fail-first.** Before claiming rows 1-4, show each red on the unfixed code (the mutation
   script for row 1; a pre-implementation run for rows 3-4) in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/evidencegate/ -run 'TestScope' -count=1 -timeout 180s` | exit 0. Planted-mutation fixtures, each `reject`: a Verify-table cell edit, a frontmatter edit, a prose edit outside `## Evidence`, a deleted Evidence row, an off-list path, a rewritten `verify-outcomes.jsonl` line, a non-descendant staging SHA, an empty diff, and a two-commit range whose first commit edits a Verify cell and whose second reverts it (net diff clean, per-commit unit rejects). An appended Evidence row, an appended outcome line, and a `remerge` commit checked against its main-side parent are `admit`. An unreadable diff is `could-not-check` | check:ci +mutation |
| 2 | `bash tools/desk/cmd/evidencegate/testdata/mutate.sh` | exit 0 and prints `MUTANT KILLED`. The script makes the section check treat every hunk as in-section, re-runs row 1, and requires it to FAIL: the fail-first proof that row 1 bites | check:ci +mutation |
| 3 | `cd statusgen && go test . -run 'TestEvidenceAudit' -count=1 -timeout 180s` | exit 0. LOWER LAYER WITH THE UPPER BYPASSED, fixture pushes to main carrying NO validator verdict: (i) a fast-forward pushed by the lander App whose commit is authored by the verifier App, NOT the lander, and edits a Verify table: selected, `checked-failed`, halt finding emitted; (ii) a push by another identity whose commit has no associated PR and edits frontmatter: selected, `checked-failed`; (iii) a lander push whose first commit edits prose outside `## Evidence` and whose second is Evidence-only: `checked-failed`. An Evidence-only lander push is `checked-clean`. A non-lander push whose every commit has an associated PR is not selected; a board-writer push touching only generated board surfaces is not selected. An unreadable commit, sender or pull association selects the push and reports `could-not-check`, never clean | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/evidencegate/ -run 'TestLanderDecision' -count=1 -timeout 180s` | exit 0. `land` only when the lander's own `scope` re-derivation is `admit` and both required statuses are green from their expected creators. No land in each of: a SHA carrying the PR-head not-a-landing pass plus green `leak-sweep` that is no staging ref's current head gives `skip`; a green `evidence-scope` on a staging head whose content the lander's own `scope` rejects gives `fallback`; a green posted by an unexpected creator counts as missing; a missing or pending `leak-sweep` gives `wait`, and past the fixed window gives `fallback`. `reject` and `could-not-check` both give `fallback` (no push, a finding naming the staging ref, the ref kept). An open halt finding gives `refuse` for every input. A moved main tip gives `remerge` up to the cap, then `fallback`; a conflicting remerge gives `fallback` | check:ci +mutation |
| 5 | `test -s .github/workflows/evidence-scope.yml && grep -qE '^[[:space:]]+workflow_run:' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+status:' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+schedule:' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+environment:[[:space:]]*evidence-lander' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+workflow_run:' .github/workflows/evidence-scope-post.yml && grep -qE '^[[:space:]]+environment:[[:space:]]*evidence-scope' .github/workflows/evidence-scope-post.yml && grep -qF 'github.event.repository.default_branch' .github/workflows/evidence-scope-post.yml && ! grep -qE 'statuses:[[:space:]]*write' .github/workflows/evidence-scope.yml && ! grep -qF 'pull_request_target' .github/workflows/evidence-scope.yml .github/workflows/evidence-scope-post.yml .github/workflows/evidence-lander.yml` | exit 0 (lander runs from the default-branch definition on all three triggers in its environment; the poster runs from the default-branch definition in its own environment with validator source from the default branch; the event-facing half cannot post a status; no `pull_request_target`). Under Option 2 the poster clauses are dropped and recorded as not-applicable in Evidence | check |
| 6 | `git diff $(git merge-base refs/remotes/origin/main HEAD)..HEAD -- .github/workflows/evidence-automerge.yml .github/workflows/leaksweep-control.yml .github/workflows/leaksweep-pattern.yml` | empty (the fallback lane and the leak workflows are untouched). The base is spelled `refs/remotes/origin/main` in full so that a stray local `origin/main` branch cannot become the comparison base | check +neighbour |
| 7 | `statusgen --consumers --root . --brief desk-supervision/23` | exit 0 (consumers routing corroborated against the diff) | check:ci |
| 8 | `statusgen --root . --lint` | exit 0 | check:ci |
| 9 | `gh api repos/medici-finance/assay/rules/branches/main --jq '[.[] \| select(.type=="required_status_checks") \| .parameters.required_status_checks[]]'` | contains `leak-sweep` and (Option 1) `evidence-scope`, each carrying an `integration_id`. The `evidence-scope` id equals the validator App's id (`gh api apps/<validator-app-slug> --jq .id`), NOT the generic GitHub Actions integration; the `leak-sweep` id equals the withheld-content poster App's id | gate:human +dereference |
| 10 | `for id in $(gh api repos/medici-finance/assay/rulesets --jq '.[].id'); do gh api repos/medici-finance/assay/rulesets/$id --jq '{name, rules: [.rules[].type], bypass_actors}'; done` (admin credential) | The lander App is a bypass actor ONLY on the ruleset holding the `pull_request` rule, and that ruleset holds no `required_status_checks`. The verifier App and the validator App are on no bypass list. The ruleset holding the required statuses lists only the pre-existing board-writer App | gate:human +dereference |
| 11 | `for e in evidence-lander evidence-scope; do gh api repos/medici-finance/assay/environments/$e --jq '{name, policy: .deployment_branch_policy}'; done` | each admits the default branch only (`protected_branches: true`, or a custom policy naming only it): a run triggered from a staging ref or a PR ref cannot read either key. Under Option 2 only `evidence-lander` exists | gate:human +dereference |
| 12 | DONE (a), planted mutation: the verify desk pushes a commit editing ONE Verify-table cell of this brief to `evidence-landing/example-canary`, then `gh api "repos/medici-finance/assay/commits/$(git rev-parse refs/remotes/origin/evidence-landing/example-canary)/status" --jq '.statuses[] \| select(.context=="evidence-scope") \| .state'` | `failure`. `git ls-remote origin refs/heads/main` is unchanged from before the push, and a finding naming `evidence-landing/example-canary` is filed (fallback, nothing dropped) | gate:human +mutation |
| 13 | DONE (b): under the implementer App credential, `git push origin refs/remotes/origin/evidence-landing/example-canary:refs/heads/main` | exit non-zero with `GH013` in stderr. The main tip is unchanged | gate:human |
| 14 | DONE (d), LOWER LAYER WITH THE UPPER BYPASSED: under the verifier App, with one real outcome line appended locally, `VERIFIER_MAIN_OK=1 deskevidence medici-finance/assay main --evidence-file docs/streams/verify-outcomes.jsonl`. The client-side refusal is deliberately switched off and the content is admissible Evidence, so only the server's identity scope can refuse | exit non-zero, and the forge refuses the write (ruleset violation). The main tip is unchanged. If it lands, the row FAILS: the line is a real outcome row, so the harm is bounded and the failure is itself the finding | gate:human +mutation |
| 15 | DONE (c) + FLOW: after a real Evidence-only landing through a staging ref, with `LANDED_SHA` exported as the new main tip: `gh api "repos/medici-finance/assay/commits/$LANDED_SHA/pulls" --jq 'length'` and `gh api "repos/medici-finance/assay/activity?ref=refs/heads/main&per_page=20" --jq '.[] \| select(.after == env.LANDED_SHA) \| {actor: .actor.login, activity_type}'` | `0` (no PR), and the ref update that set main to `LANDED_SHA` names the lander App's bot login as `actor` with `activity_type` `push`. The commit author is deliberately NOT asserted: a fast-forward keeps the staging commit's author (the verifier App). The post-land audit run on that push reports `checked-clean`. Fail-first: the same push attempted before Task step 1 was refused `GH013`, recorded in Evidence | gate:human +flow |
| 16 | `grep -qF 'evidence-landing/' plugins/assay/skills/verify-desk/SKILL.md && grep -qiF 'lander' docs/adopting-assay.md` | exit 0 (the skill and the adopter doc name the lane) | check |
| 17 | `test -s .github/workflows/evidence-scope-post.yml && test -s .github/workflows/evidence-lander.yml && ! grep -hE '[$][{][{][^}]*github[.]event[.]' .github/workflows/evidence-scope-post.yml .github/workflows/evidence-lander.yml \| grep -qvE '^[[:space:]]+[A-Z][A-Z0-9_]*:[[:space:]]*[$][{][{]' && ! grep -qF 'download-artifact' .github/workflows/evidence-scope-post.yml .github/workflows/evidence-lander.yml` | exit 0: in the two credential-holding workflows every `github.event.*` interpolation sits on an upper-case `env:` assignment line, never inside `run:` or `if:`, and neither downloads an artifact from a triggering run. Fail-first: a planted `run: echo ${{ github.event.workflow_run.head_branch }}` line makes the row exit 1. Under Option 2 the poster file is dropped from both greps | check +mutation |
| 18 | LOWER LAYER WITH THE UPPER BYPASSED, live (Option 1): open a draft canary PR from `example-canary-pr`, a fast-forward of main that edits ONE Verify-table cell of this brief; approve it at its head so the poster posts the not-a-landing pass; let `leak-sweep` post. With `CANARY_SHA` exported as that head: `gh api "repos/medici-finance/assay/commits/$CANARY_SHA/status" --jq '[.statuses[] \| select(.context=="evidence-scope" or .context=="leak-sweep") \| .state]'` then `git ls-remote origin refs/heads/main` | both statuses `success` (the server layer alone would accept a push of this SHA), yet main is NOT `CANARY_SHA`, and the lander runs those statuses triggered each logged `skip` for `CANARY_SHA` (not a staging ref's head). The canary PR is then closed unmerged | gate:human +mutation |

Pre-mortem (failure mode → row):

| Failure mode | Caught by |
|---|---|
| Validator admits a Verify-table or frontmatter edit | rows 1-2 (and row 3 after the fact) |
| An intermediate commit carries content the net diff hides | row 1 (per-commit unit); row 3 (iii) |
| Validator skipped or buggy, and bad content lands anyway | row 3 (audit + halt); row 9 (server-required status, Option 1); row 4 (lander's own re-derivation) |
| Audit selects by commit author and misses a fast-forward landing | row 3 (i) (verifier-authored commit pushed by the lander is still audited); row 15 (pusher, not author) |
| Lander lands an approved or unapproved PR head carrying the not-a-landing pass | row 4 (`skip`); row 18 (live) |
| `evidence-scope` forged by another workflow or identity | row 9 (integration pinned to the validator App); row 5 (event-facing half cannot post); row 11 (poster key default-branch only); row 4 (unexpected creator counts as missing) |
| Lander steered by attacker-named event fields | rows 5, 17 |
| Verifier App gains a direct main write | rows 10, 14 |
| Lander key readable from a staging-ref run | rows 5, 11 |
| Landing bypasses the withheld-content sweep | rows 9-10 (lander not on that ruleset); row 4 (no land without `leak-sweep`) |
| `leak-sweep` arrives after the validator, and the landing is stranded | row 4 (`wait`, then `fallback` past the window); row 5 (`status` and `schedule` triggers) |
| Rejected landing silently dropped | rows 4, 12 |
| Rebase or force on race | row 4 (`remerge` is a two-parent merge); `non_fast_forward` stays on both rulesets (row 10) |
| Ruleset later widened by hand | no row. Review-only: re-audit is DR-server-controls' standing commitment |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 9-15 and 18 need Task
     step 1 done; until then they are could-not-check, never pass. -->

## Review
Gate: human (from frontmatter: sensitive-data and irreversible are yes). The human gate is MANDATORY.
The reviewer answers BOTH, in the verdict:
1. What is the single control standing between the fault and the damage, and is that acceptable?
   (The SPOF line above names the lander's PR-rule bypass. Confirm the layers behind it are
   present at the chosen option.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed? (Rows
   3, 14 and 18 are designed to. Confirm they bypass the upper layer rather than walking the happy
   path through every layer at once.)

Reviewer records verdict + date in the stream README table.
