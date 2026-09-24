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
  validator that posts a required status), restructures the rulesets on the default branch, adds
  a ruleset on the staging refs, and adds three workflows, two of which hold an App key with
  write access (the third's push-to-main audit job holds a `GITHUB_TOKEN` scoped to
  `issues: write` only): an identity/auth change on the
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
  - ".github/workflows/evidence-automerge.yml — the existing Evidence-PR lane (the fallback), and the precedent for a workflow that reads a change as data and never executes its tree. Its primary scope control is the author check (only the verifier App's PRs are eligible) plus a reviewer approval; the direct lane keeps that writer binding (Task steps 1, 2 and 5)."
  - ".github/workflows/assay-statusgen.yml, verify-gate-close.yml, assay-qualgen.yml and release.yml (changelog roll) — the board-writer App precedent: an App on the ruleset bypass lists that pushes generated files straight to main with no PR. Its direct-write surface is `STATUS.md`, stream board `README.md` files, `docs/quality/QUALITY.md`, and the release roll's `CHANGELOG.md`, `changelog/` fragment deletions and plugin version-stamp paths (the set `plugins/assay/scripts/stamp-plugin-version.sh paths` prints)."
  - "docs/protected-paths.md and tools/desk/cmd/deskpathguard/verifysection.go — the existing diff-hunk section detector (a heuristic, per its own header), reused as a starting point but not as the validator."
  - "live read 2026-09-23: `gh api repos/medici-finance/assay/rules/branches/main` — see facts."
  - "freshness-checked 2026-09-23 @ 2a5c230ef"
consumers:
  - "tools/desk/cmd/evidencegate/ (validator scope check + lander decision, with fixtures and mutation script): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation adds it)"
  - "statusgen evidence-audit subcommand (post-land audit, statusgen/evidenceaudit.go + test): follow-up desk-supervision/23 (this brief; flips to fixed-here when the implementation adds it)"
  - ".github/workflows/evidence-scope.yml (new event-facing validator workflow; its event-facing jobs hold only a read-only `GITHUB_TOKEN`, and its push-to-main post-land audit job holds a `GITHUB_TOKEN` scoped to `issues: write` only, never an App key): follow-up desk-supervision/23 (this brief; lands through the workflow-only PR path, flips to fixed-here then)"
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
- **add** `.github/workflows/evidence-scope.yml` (planned) (event-facing validator half: its
  event-facing jobs hold only a read-only `GITHUB_TOKEN`; it also runs the post-land audit, whose
  push-to-main job holds a `GITHUB_TOKEN` scoped to `issues: write` only and never an App key),
  `.github/workflows/evidence-scope-post.yml` (planned)
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

single-point-of-failure: the lander App's bypass of the PR-required rule is the ONE grant that lets a commit reach main without review — layers behind it: (1) the required `leak-sweep` and (Option 1) `evidence-scope` statuses sit in a ruleset the lander does NOT bypass, each pinned to its own dedicated App's integration id; `evidence-scope` is posted only by the validator App from a default-branch run, and reads `success` only as a staging-ref `admit` or as the not-a-landing pass on a PR head approved at that exact head, so the server refuses any lander push whose SHA is neither validated Evidence nor an approved PR head; a separate ruleset on `evidence-landing/**` lets only the verifier App and the lander App create, update or delete a staging ref; (2) the lander workflow, from a default-branch definition with a key only a default-branch run can read, acts only on the current head of an `evidence-landing/**` ref, re-runs the `scope` core itself instead of trusting the status (including the writer binding, on data a raw push cannot forge: a single-parent commit only when the forge attributes it to the verifier App and the verifier App moved the ref onto it; a two-parent `remerge` only when the lander App moved the ref onto it from its staging-side parent AND its tree is byte-identical to the clean three-way merge of its two parents recomputed from forge objects, so a remerge can carry no content of its own), and lands only a fast-forward of that exact SHA on its own `admit` (this layer is independent of a forged or misapplied status, not of a bug in the shared `scope` core; layer 4 is the one independent of that code); (3) the lander App's permission ceiling (contents + metadata on this one repository; no workflows, no administration); every claim this brief makes about the lander key is about main, since a contents permission also reaches tags and releases; (4) the post-land `statusgen evidence-audit` selects by the PUSH, never by commit author (every push to main by the lander, and every push carrying a commit that no merged pull request in that range accounts for), re-derives Evidence-only-ness and the writer per commit with a different algorithm (and re-checks every `remerge` tree against its own recompute of the merge, with git's merge rather than the `scope` core's code), and halts the lane on a mismatch or a `could-not-check` in a lander push (either in another identity's push files a non-halting finding for a human: halting the lane cannot contain a write the lane did not make). One assumption sits beneath layers 1, 2 and 4 alike: every job that holds a key or writes (the poster, the lander, and the audit job) executes only default-branch code, and that holds only if the job's MACHINE runs nothing else. A runner shared with PR-authored jobs that keeps state between jobs defeats all three through one channel, and layer 3 bounds nothing there, since main's contents are the damage. So every job in the three workflows runs on a GitHub-hosted runner, a fresh machine per job, never on the self-hosted pool that runs pull-request CI (Task step 3; rows 20 and 21).

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
- Where a job runs is a separate question from which definition it runs. This repository's CI runs
  on a self-hosted, non-ephemeral runner pool that also runs `pull_request` jobs;
  `.github/workflows/release.yml` (its "WHY A DEDICATED LABEL (H1)" comment) records that such a
  pool "can leave state behind ($HOME, the PATH-prepended Go dir, ~/.gitconfig)" for a later job to
  inherit. A default-branch-only environment policy guards the SECRET, not the machine: a
  PR-authored job that ran earlier on the same runner can plant state a later key-holding job
  executes. A `pull_request` run executes the PR's own definition, so it can name any runner label;
  a dedicated self-hosted label does not keep PR jobs off those runners unless the runner group
  itself is restricted to default-branch workflows. A GitHub-hosted runner gives each job a fresh
  virtual machine that is discarded when the job ends. Live read 2026-09-24 of this repository's
  jobs API (`actions/runs/{id}/jobs`): a job on a GitHub-hosted label reports `runner_group_id` 0
  and `runner_group_name` `GitHub Actions`; a job on the self-hosted CI label reports the group
  `Default`.
- A fast-forward push adds no commit. The landed commits keep the author and committer of whoever
  wrote them to the staging ref. The verify desk writes through `deskevidence`, which commits
  through the Contents API with the verifier App's installation token
  (`tools/desk/cmd/deskevidence/main.go` header), but that is the desk's behaviour, not a
  control: the design does not rely on it (Task steps 1, 2 and 5 bind the writer). Who pushed is
  recorded on the push, not the commit: the `push` event's `sender`, and the repository activity
  API (`repos/{owner}/{repo}/activity`, one entry per ref update, on any ref: `actor`,
  `activity_type`, `before`, `after`, `timestamp`).
- A commit's author and committer name and e-mail are whatever its creator wrote, so a raw
  `git push` can claim any identity, including a bot's noreply address, which the forge then maps
  to that bot's login. Two things a raw push cannot set: the activity API's `actor` on the ref
  update (the authenticated identity that moved the ref), and the forge's signature verdict on
  the commit (`verification.verified` and `verification.reason` in the commits API).
- The existing Evidence lane's primary scope control is WHO wrote the change:
  `evidence-automerge.yml` admits only a PR authored by the verifier App, and the PR still needs a
  reviewer approval. A direct lane that checked content shape alone would let any identity with
  contents write land Evidence on its own work with no non-implementer and no reviewer in the
  path. The writer binding in Task steps 1, 2 and 5 keeps that separation of duties.
- What an Evidence landing writes today (batch Evidence PR, 2026-09-23): additions inside the
  `## Evidence` section of `docs/streams/<stream>/brief-*.md` files, plus appended lines in
  `docs/streams/verify-outcomes.jsonl`. Its NET diff deletes nothing and edits no other line. The
  v1 allow-list is exactly those two path shapes. A landing that also needs a stream README cell
  change keeps the PR path.
- Per commit, that is not always true. A verifier sometimes corrects an outcome line it has just
  appended, in a follow-up commit that rewrites that line (one line removed, one added). Under the
  per-commit unit of Task step 2 such a range is `reject`, so the lander falls back to the batch
  Evidence PR. That is the designed fail-closed result for a routine correction, not a defect.
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
pre-image and post-image text, each commit's forge signature verdict and committer login, for each
two-parent commit its own tree id and the tree id of the clean three-way merge of its two parents
that the adapter recomputes from forge objects (or the fact that the recompute conflicted), the
staging ref's update history with each update's actor, statuses with their creators, the staging
ref's current head, wait age, halt state, the state of the audit run for the lander's previous
push to main) and tested with no forge: `scope` (admit / reject /
could-not-check) and `lander` (land / remerge / wait / fallback / refuse / skip). The workflows are thin adapters that
gather the data and act. The event-facing jobs hold only a read-only `GITHUB_TOKEN`. Only
default-branch runs hold anything that writes: an App key in the poster and the lander, and in the
push-to-main audit job a `GITHUB_TOKEN` scoped to `issues: write` (a push to main runs main's own
definition). The audit is a separate implementation in a separate module (statusgen's brief parser,
comparing section hashes). A bug in the validator's hunk-to-section mapping therefore does not also
blind the audit. Rows 1-4 test the cores. Rows 9-15 and 17-21 test the live boundary. The
server layer's identity pin on `evidence-scope` is proven by configuration read (row 9), not by
a live negative push: that push would need the lander key outside its default-branch-only
environment, which the design forbids. The lander's own unexpected-creator rule has its negative
fixture in row 4.

## Human decision
<!-- decision-trigger: creation — options enumerable now; filed as the brief lands. Self-contained. -->
On this repository every Evidence landing (a verifier adding result rows to a work item's Evidence
section, plus one line in an outcomes log) is its own pull request. Each needs CI, a reviewer
approval and an auto-merge, and these PRs fill a saturated review queue and runner pool. The
proposal: Evidence-only changes land on the default branch directly, with no PR, through a
dedicated "lander" identity. The lander is the only identity allowed to skip the PR requirement.
The lane admits only Evidence the verifier identity wrote: only the verifier (and the lander) may
write a staging branch, and the lander lands only commits the forge itself attributes to the
verifier, plus the lander's own merges of the default branch into a staging branch, each checked
to be exactly the merge of its two parents so it adds no content of its own. No other identity,
an implementer's above all, can land verification of its own work this way. Before landing, the lander re-checks for itself that the change is Evidence-only
and verifier-written, and an audit after every landing re-checks both with a different method
and stops the lane on a mismatch or on an input it cannot read. Only a human can restart a stopped lane. The verifier identity
keeps no write to the default branch. Every job that holds one of the new keys runs on a machine the
hosting platform provisions fresh for that one job, never on the shared machines that also run
pull-request checks, so no earlier job can leave anything behind for it to run. A rejected landing files a finding and falls back to
today's batched PR path.

One trade to rule on explicitly: today a reviewer reads each Evidence PR's text, which is often
raw command output, before it reaches the public default branch. The direct lane removes that
reader. What then stands between the Evidence text and public disclosure is the automated
withheld-content sweep, which matches a list of withheld terms rather than secrets or personal
data in general, plus the verifier's own client-side checks. Choosing any adopt option below
accepts that, unless the ruling also requires a further automated check (for example a
secret-scanning signal on the staging head) as a condition of landing.

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
   carries two lanes, and revoking one revokes both. The audit can no longer tell a landing from a
   board write by who pushed it, so a push by that App whose every path is a generated board
   surface is treated as a board write and stays outside the landing audit.
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
   environment (`evidence-lander`, `evidence-scope`), each with a custom deployment branch policy
   that names only the default branch. Never the `protected_branches` policy: it admits every
   protected branch, not only the default one. Restructure the rulesets so the PR rule and the
   required statuses live in different rulesets. The lander bypasses only the PR rule.
   `leak-sweep` and (Option 1) `evidence-scope` are required in a ruleset the lander does not bypass, each pinned to its own
   App's integration id (`evidence-scope` to the validator App, never the generic GitHub Actions
   integration). Neither new App and not the verifier App is on that ruleset's bypass list. The
   board-writer App keeps its existing bypass. Add a ruleset on `refs/heads/evidence-landing/**`
   that restricts creation, update and deletion, with only the verifier App (it writes the
   staging ref, and deletes it after a fallback's batch PR closes) and the lander App (`remerge`,
   and deleting the ref after `land`) on its bypass list; no other identity may write a staging
   ref. The operator extends the withheld-content poster to `evidence-landing/**` heads.
2. **Validator core** (`evidencegate scope`). Input: the exact main tip the landing would
   fast-forward from, the staging SHA, and every commit in `tip..staging` with its parents. Admit
   only when the staging SHA descends from that tip and EVERY commit in the range is itself in
   scope: a single-parent commit checked against its parent; a two-parent commit only as a lander
   `remerge` whose other parent is on main's first-parent history, checked against that main-side
   parent, AND whose tree is byte-identical to the clean three-way merge of its two parents,
   recomputed from the forge objects (`git --attr-source=<main-side parent> merge-tree
   --write-tree <parent1> <parent2>` over the fetched parents, never the commit's own tree; git
   2.43 or later: `--attr-source` arrived in 2.41, and 2.43 fixed a `merge-tree` crash with it).
   The merge attributes are pinned to the main-side parent's tree, which is the default branch's,
   so every recompute (this adapter, the audit's, row 19 (c)) resolves the merge the same way
   whatever checkout it runs in, and the staging side cannot change how it resolves. One file
   outranks the pinned source: the runner-local `$GIT_DIR/info/attributes`. No staging write
   reaches it, but it is runner state, so each adapter recomputes in a fresh clone and treats a
   non-empty `info/attributes` as `could-not-check` (row 19 (c) asserts it is empty). Global and
   system attributes rank below the pinned source but still apply to paths it leaves unset, and a
   merge DRIVER is config, not an attribute, so each adapter also runs the recompute with
   `GIT_ATTR_NOSYSTEM=1`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL=/dev/null` and
   `-c core.attributesFile=/dev/null`: no runner-global file can add a merge rule or a driver.
   A `git --version` below 2.43, or a `merge-tree` exit other than 0 (clean) or 1 (conflict), is
   `could-not-check`, never a verdict.
   The pin matters because
   the repository marks `docs/streams/verify-outcomes.jsonl` `merge=union`, and a bare or
   attribute-less recompute of two concurrent outcome appends conflicts where the pinned one is
   clean. A recompute that conflicts, or any difference
   between the recomputed tree and the commit's tree, is `reject`. A true forge merge passes this
   test; a two-parent commit written some other way (the Git Data API, a local `git commit-tree`)
   whose tree carries a line neither parent's side of the merge contributes cannot. So a `remerge`
   carries no content of its own: only what its main-side parent already has on main and what its
   staging-side parent carries, which is itself a commit in the range and checked on its own.
   A commit is in scope when every changed path is on the allow-list, each brief-file
   change is pure additions whose every added line falls inside the `## Evidence` section, and each
   `verify-outcomes.jsonl` change is a pure append of lines that parse as JSON objects. For a
   `remerge` checked against its main-side parent the outcome-log rule is the merge form instead:
   the main-side file's lines are all present, unchanged and in their order, and the only other
   lines are exactly the lines the staging side added since the merge base, each a JSON
   object, in any position. "Added", not "appended": on a second remerge of the same ref, the
   lines the first remerge placed above main's sit inside the file relative to the new merge
   base, so they are inserted there, not appended. A union merge of two concurrent appends places the staging side's
   lines above main's new ones, which is not a pure append against the main-side parent, and the
   usual reason main moves while a landing waits is another landing's append; without this form
   every such remerge would fall back. Compute the
   section boundaries byte-exactly from each commit's pre-image file, not from hunk context: the
   existing `deskpathguard` detector is a heuristic by its own statement. Because every admitted
   commit is additions-only, no later commit can remove what an earlier one added, so the net tree
   carries every line any commit in the range added (the withheld-content sweep of that tree sees
   them all) and the per-commit and net verdicts agree. The audit in step 5 uses the same unit.
   **Writer binding.** A commit in scope must ALSO have been written by the expected identity,
   bound on data a raw `git push` cannot forge. The forge reports the commit signature-verified
   (`verification.verified` true, `reason` `valid`) with the verifier App as committer, and every
   update of the staging ref from its creation to the staging SHA has the verifier App as its
   activity-API `actor`, except the updates that moved the ref onto a `remerge` commit. A `remerge`
   commit is bound on exactly three fields: the activity-API `actor` of the ref update that moved
   the ref onto it is the lander App; that update's `before` (the ref's previous head) is its
   staging-side parent, so a remerge cannot bring along a commit no verifier update ever put on the
   ref; and its tree equals the recomputed clean merge of its parents (above). Its committer and its signature verdict are NOT bound: with the tree bound to the
   merge, the commit adds no content, so its committer carries no weight, and the brief makes no
   claim about how the forge signs a merge-API commit. A two-parent commit moved onto the ref by
   any other actor is `reject`, however exact its tree. The author or committer NAME alone never
   counts. The expected identities come from the
   default-branch checkout. Admissible Evidence content written by any other identity is
   `reject`, naming the commit and the identity found. The implementation confirms, and row 19
   reads back live, that each writer's real mechanism yields what this binding expects: the
   verifier's write yields the verdict and committer (row 19 (b)); the lander's forge-side merge
   yields a two-parent commit whose tree equals the recomputed merge, moved onto the ref by the
   lander as actor (row 19 (c)). If one does not, it reports NEEDS_CONTEXT and never loosens the binding to
   fit.
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
     `evidence-scope` with the validator App's key. It re-reads its inputs itself: the SHA, the
     ref or PR it belongs to, each commit's signature verdict and committer, and the staging ref's
     update history, all from the forge API, never from the triggering run's event fields,
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
   - **Runner isolation (binds steps 3-5).** Every job in `evidence-scope.yml`,
     `evidence-scope-post.yml` and `evidence-lander.yml` runs on a GitHub-hosted runner: its
     `runs-on:` is a single `ubuntu-latest` or `ubuntu-<version>` label, a fresh machine per job
     that runs nothing before it and is discarded after it. Never a self-hosted label: the pool
     that runs pull-request CI keeps state between jobs, and even a dedicated self-hosted label
     would admit a PR-authored job that names it, unless its runner group were restricted to
     default-branch workflows, a runner-group setting this brief does not make (choosing that
     instead is a change to this brief, with its own read-back row). The event-facing jobs follow
     the same rule, so no label in these files is shared with a credential-less job. Every job
     also sets an explicit `timeout-minutes:`, and no job calls a reusable workflow (a job-level
     `uses:`), whose `runs-on:` row 20 could not read. Row 20 checks the text; row 21 reads back
     where the real runs executed.
4. **Lander core + workflow** (`evidencegate lander`, `evidence-lander.yml`). Triggers:
   `workflow_run` of the poster (of `evidence-scope.yml` under Option 2); `status` events for the
   `leak-sweep` and `evidence-scope` contexts, because `leak-sweep` arrives asynchronously from
   outside this repository and a status post never fires `workflow_run`; and a `schedule` sweep of
   open `evidence-landing/**` refs. All three run the default-branch definition, in environment
   `evidence-lander`. Event fields reach a shell only through `env:` assignments, never as `${{ }}`
   inside `run:`; `if:` conditions are written without `${{ }}`; the workflow downloads no artifact
   from any triggering run.
   The lander never trusts the event. For each candidate it re-reads from the forge the current
   head of the `evidence-landing/**` ref it names, that ref's update history, the current main
   tip, the SHA's statuses with each status's creator, and the halt finding's state. It re-runs the
   pure `scope` core itself, from default-branch source, over that tip and SHA, writer binding
   included; a posted `evidence-scope` is never its only witness. The expected status creators,
   the expected writers, the halt label and the allowlist of humans who may clear a halt come from
   the default-branch checkout, never from the staging tree. The outcomes are evaluated in this
   order, and the first that applies wins:
   1. `refuse` — the lane is halted: every input, no push. The halt is active from the moment a
      finding carrying the halt label is filed until a HUMAN on that allowlist closes it (the
      closing event's actor is a `User`, never a `Bot`, the same human-not-bot test
      `verify-gate-close.yml` applies). A halt finding closed, or unlabelled, by any App or bot
      still counts as active. An audit that never reports is treated as a halt too: before any
      other outcome the lander reads the step 5 audit run for its own most recent push to main
      (the `evidence-scope.yml` run for that push's `after` SHA, read with the job's own
      `GITHUB_TOKEN` scoped `actions: read`). That run concludes `success` only after it has
      recorded `checked-clean` or filed its finding. Queued, in progress, or not yet created, while
      that push is younger than the wait window of (5) (timed from the push's forge-recorded
      time) → a candidate that would otherwise `land` gets `wait` instead, and every other outcome
      is unchanged (the candidate's own window keeps running, so a slow audit can cost it a
      `fallback`, never a landing); concluded any other way (`failure`, `cancelled`, `timed_out`), still
      unfinished or not created past that window, or unreadable → the lander files the
      halt-labelled finding itself, naming that push, the run and its conclusion, and `refuse`
      holds until a human clears it. A push the lane made is never left unaudited.
   2. `skip` — the SHA is not the current head of any `evidence-landing/**` ref (a PR head, a stale
      staging SHA), or its ref has already fallen back, or main already contains the SHA (a
      landed ref not yet deleted, under the step 5 deletion contingency): no push, no finding. `fallback` is terminal
      per ref: once a fallback finding names a ref, the lander never remerges into it or lands from
      it again, so it cannot disturb the batch Evidence PR opened from that ref.
   3. `remerge` — the staging SHA does not descend from the current main tip: merge current main
      into the staging ref through the forge's merge API (two parents, never rebase; the forge,
      not the runner, writes the merge commit and records the lander as the ref update's actor)
      and let the cycle re-validate, up to a fixed attempt cap. The cap reached, or a conflicting
      merge, is `fallback`.
   4. `fallback` — its own `scope` verdict is `reject` or `could-not-check`, a required status is
      `failure`, or the wait window has expired: no push; file one finding naming the staging ref
      (deduped per ref); keep the ref for the verify desk's batch Evidence PR, which deletes the
      ref once that PR merges or closes.
   5. `wait` — a required status is pending or missing (normal while `leak-sweep` has not posted
      yet), or a green was posted by a creator other than the expected App (counted as missing):
      no push, no finding, the ref kept. Bounded: the window's clock starts at the forge-recorded
      time the staging ref was updated to that SHA (the activity API `timestamp`), never a commit
      date, which its writer sets. The implementation names the window constant, and the schedule
      sweep runs more often than the window. Past the window it is `fallback` (4).
   6. `land` — its own `scope` verdict is `admit`, AND `leak-sweep` and (Option 1) `evidence-scope`
      are green, each posted by its expected App: push that exact validated SHA by refspec
      (`<sha>:refs/heads/main`, a fast-forward; never the staging ref's name, so a commit added to
      the ref after the re-read cannot ride along), then delete the staging ref through the refs
      API. That SHA refspec is the lander's only `git push` and its only write to main (never a
      refs-API update of main): the remerge goes through the merges API and the staging-ref
      deletion through the refs API (row 17).
5. **Post-land audit** (`statusgen evidence-audit`, run by `evidence-scope.yml` on pushes to
   main; a push to main runs main's own definition). Select by the PUSH, never by a commit's
   author: a fast-forward landing adds no lander-authored commit. The audit inspects every push to
   main whose `sender` is the lander App, AND every push to main whose `before..after` range
   carries a commit that no MERGED pull request accounts for: a commit is accounted for only when
   `commits/{sha}/pulls` returns a merged PR whose merge commit (`merge_commit_sha`) lies in that
   same range. An open or closed-unmerged PR that happens to contain the commit does not account
   for it. One exclusion: a push by the board-writer App whose every changed path is on its
   generated-surface set, which is `STATUS.md`, stream board `docs/streams/<stream>/README.md`
   files, `docs/quality/QUALITY.md`, `CHANGELOG.md`, deletions (never additions or edits) of
   `changelog/*.md` fragments other than `changelog/README.md`, and the plugin version-stamp paths
   the default-branch `plugins/assay/scripts/stamp-plugin-version.sh paths` prints. The set is
   read from the default-branch checkout; a board-writer push with any path outside it IS
   selected. An unreadable sender, pull association or stamp-path list selects the push (fail
   toward auditing). For each selected push, check every commit in `before..after` with the same
   per-commit unit as step 2: compare the normalized hash of each touched brief's non-Evidence
   content and each prior outcomes line, before and after that commit, flag any path off the
   allow-list, and, in a lander push, flag any commit whose writer fails step 2's binding, read
   by the audit's own adapter, never taken from the validator's or the lander's result:
   - a single-parent commit on the forge's signature verdict and committer (the verifier App)
     AND on the activity-API `actor` of the `evidence-landing/**` ref update that put it there
     (the verifier App). The adapter finds the staging ref that an update set to the pushed
     tip and reads that ref's update history; a history it cannot read (a deleted ref whose updates the API no longer returns)
     is `could-not-check`, never clean;
   - a two-parent commit on the same three `remerge` fields as step 2: the lander App as the
     update's `actor`, that update's `before` as its staging-side parent, and its tree
     byte-identical to the clean three-way merge of its parents, which the audit job recomputes
     itself from the forge objects with the same pinned attribute source, neutralised global and
     system config and attributes, and version and exit-code rules as step 2, and its
     outcome-log change checked in step 2's merge form. That recompute is git's own merge, not the `scope` core's
     code, and it runs in a different job, so a bug in the `scope` core does not blind it; a
     conflicting recompute or any tree difference is a mismatch.

   On a mismatch, or on a `could-not-check` (an input the audit could not read, including a
   staging ref's update history), the audit files a finding that names its cause (`mismatch`
   or `could-not-check`) and, for a `could-not-check`, the input it could not read:
   - in a push whose `sender` is the lander App, or whose sender is unreadable, file a finding
     carrying the halt label (step 4's `refuse` then holds until a human clears it). A
     `could-not-check` fails toward halting, never toward silence. Because the lander deletes the
     staging ref at `land`, an unreadable history of a deleted ref would halt every landing; row
     15 surfaces that live. If it does, the implementation moves the deletion after the audit's
     read (the schedule sweep deletes a landed ref only once the audit of its landing push has
     reported `checked-clean`, never after a halting report, and only while the ref's head still
     equals the landed SHA, so a commit the verifier added to the ref since is never dropped; a
     ref that an active halt finding names is kept, so the update history a human needs to
     clear that halt survives; until deletion the lander `skip`s the ref, because main already
     contains its head) and never drops or weakens the check;
   - in a push by any other identity, file a finding for a human WITHOUT the halt label. That
     write did not come through the lane, so halting the lane would not contain it, and the
     identity's own bypass is what the human reviews. A board-writer surface added later that
     this set does not yet name therefore costs one non-halting finding, never a stopped lane.

   A read is `could-not-check` only after a bounded in-job retry with backoff, so a single
   transient forge error does not halt the lane, and the cause the finding names lets a human
   clear a transient halt quickly. The implementation names the attempt cap and the total retry
   budget (attempts times the longest backoff), and keeps that budget at most half the audit
   job's `timeout-minutes`, so retries cannot run the job out of time. A run that still ends
   without a verdict (a timeout, a cancellation, a lost runner) files nothing itself; step 4's
   `refuse` covers it, because the lander halts the lane when the audit of its own previous push
   did not conclude `success`.
   This is detection plus halt. It never reverts on its own. The push selection is data the
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
| 1 | `cd tools/desk && go test ./cmd/evidencegate/ -run 'TestScope' -count=1 -timeout 180s` | exit 0. Planted-mutation fixtures, each `reject`: a Verify-table cell edit, a frontmatter edit, a prose edit outside `## Evidence`, a deleted Evidence row, an off-list path, a rewritten `verify-outcomes.jsonl` line, a non-descendant staging SHA, an empty diff, a two-commit range whose first commit edits a Verify cell and whose second reverts it (net diff clean, per-commit unit rejects), and a rewrite of an outcome line appended by the previous commit. Writer-binding fixtures, each `reject` although the content is admissible Evidence: a commit whose committer is the implementer App; an unsigned commit whose committer name and e-mail claim the verifier App (a raw push); a signed verifier commit reached by a staging-ref update whose actor is another identity. Remerge fixtures, each `reject`: a two-parent commit moved onto the ref by the lander, parents (the previous staging head, the main tip), whose tree adds one Evidence row that neither parent carries (the recomputed merge differs from its tree), and the same with an appended outcome line instead; a two-parent commit whose recomputed merge conflicts; a two-parent commit whose tree equals the recomputed merge but that another actor (the verifier App included) moved onto the ref; a two-parent commit moved on by the lander whose staging-side parent is not the ref's previous head; a two-parent commit whose outcome log drops or reorders a main-side line (the tree check also catches this one, since a union merge never drops a main-side line, so it is not the fail-first proof of the merge form, whose only effect is to admit). A remerge after a concurrent landing, where both sides appended to `verify-outcomes.jsonl` and the recompute (attributes pinned to the main-side parent, so the `merge=union` rule applies) places the staging lines above main's, is `admit`; so is a second remerge of the same ref after a further concurrent landing, whose staging side already carries the lines the first remerge placed above main's. These two `admit` fixtures are the merge form's discriminating cases: without the merge form each is `reject`. An appended Evidence row and an appended outcome line, each forge-verified as the verifier App's and pushed by it, and a `remerge` commit moved on by the lander from the previous staging head whose tree equals the recomputed clean merge of its parents, are `admit`. An unreadable diff, signature verdict or update history is `could-not-check` | check:ci +mutation |
| 2 | `bash tools/desk/cmd/evidencegate/testdata/mutate.sh` | exit 0 and prints `MUTANT KILLED`. The script makes the section check treat every hunk as in-section, re-runs row 1, and requires it to FAIL: the fail-first proof that row 1 bites | check:ci +mutation |
| 3 | `cd statusgen && go test . -run 'TestEvidenceAudit' -count=1 -timeout 180s` | exit 0. LOWER LAYER WITH THE UPPER BYPASSED, fixture pushes to main carrying NO validator verdict: (i) a fast-forward pushed by the lander App whose commit is authored by the verifier App, NOT the lander, and edits a Verify table: selected, `checked-failed`, halt finding emitted; (ii) a push by another identity whose commit has no associated PR and edits frontmatter: selected, `checked-failed`; (iii) a lander push whose first commit edits prose outside `## Evidence` and whose second is Evidence-only: `checked-failed`; (iv) a lander push of admissible Evidence whose commit the forge attributes to the implementer App: `checked-failed`, halt finding; (v) a lander push carrying a two-parent commit, moved onto its staging ref by the lander, whose tree adds an Evidence row absent from both parents (the audit's own merge recompute differs from its tree): `checked-failed`, halt finding. An Evidence-only, verifier-written lander push is `checked-clean`, and so is one carrying a true `remerge` whose tree equals the audit's recompute. MERGE FORM: a lander push carrying a `remerge` made after a concurrent landing, where both sides appended to `verify-outcomes.jsonl` and the union merge (attributes pinned to the main-side parent) places the staging lines above main's, is `checked-clean` and files no finding; an audit that checks that outcome-log change as a pure append instead fails this fixture. A lander push whose staging ref's update history cannot be read reports `could-not-check`, never clean, and files the finding WITH the halt label; the finding names its cause (`could-not-check` and the unread input, or `mismatch`). A read that fails once and succeeds within the bounded retry is read, not `could-not-check`. Selection: a non-lander push whose every commit is accounted for by a merged PR with its merge commit in the range is not selected; a direct non-lander push of commits that belong only to an OPEN PR IS selected. A board-writer push is not selected when it touches only one surface of its set, one fixture per surface (`STATUS.md`, a stream `README.md`, `docs/quality/QUALITY.md`, `CHANGELOG.md`, a `changelog/` fragment deletion, a version-stamp path); a board-writer push that also adds a `changelog/` fragment, or touches any path outside the set, IS selected. Consequence: a mismatch in a non-lander push (fixture ii) files a finding WITHOUT the halt label; a mismatch in a lander push or a push with an unreadable sender files it WITH the halt label. An unreadable commit, sender, pull association or stamp-path list selects the push and reports `could-not-check`, never clean; in a lander push or a push with an unreadable sender that `could-not-check` files the finding WITH the halt label, and in another identity's push it files one WITHOUT | check:ci +mutation |
| 4 | `cd tools/desk && go test ./cmd/evidencegate/ -run 'TestLanderDecision' -count=1 -timeout 180s` | exit 0. `land` only when the lander's own `scope` re-derivation is `admit` and both required statuses are green from their expected creators. No land in each of: a SHA carrying the PR-head not-a-landing pass plus green `leak-sweep` that is no staging ref's current head gives `skip`; a green `evidence-scope` on a staging head whose content the lander's own `scope` rejects gives `fallback`; a green posted by an unexpected creator counts as missing; a missing or pending `leak-sweep` gives `wait`, and past the fixed window, timed from the forge-recorded ref update (a fixture whose commit date is far older than its ref update still gives `wait`), gives `fallback`; admissible Evidence content whose commit the implementer App wrote gives `fallback`, not `land`. `reject` and `could-not-check` both give `fallback` (no push, a finding naming the staging ref, the ref kept). An open halt finding gives `refuse` for every input, and so does a halt finding closed or unlabelled by a bot; one closed by an allowlisted human does not. A ref that already fell back gives `skip`, even when main has moved or its statuses turn green. A ref whose head main already contains (a landed ref not yet deleted) gives `skip`, not `fallback` or `remerge`, both while that head is main's tip and after main has moved on. A moved main tip gives `remerge` up to the cap, then `fallback`; a conflicting remerge gives `fallback`. Precedence: a fixture that is halted AND not a staging head gives `refuse`; one that is not a staging head AND out of scope gives `skip`. Audit silence: with the audit run of the lander's previous push to main queued or in progress and that push younger than the window, a candidate that would otherwise `land` gives `wait`; with that run concluded `cancelled`, `timed_out` or `failure`, not created or unfinished past the window, or unreadable, every input gives `refuse` and the lander's own halt finding names the push, the run and its conclusion; a run concluded `success` changes no outcome | check:ci +mutation |
| 5 | `test -s .github/workflows/evidence-scope.yml && grep -qE '^[[:space:]]+workflow_run:' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+status:' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+schedule:' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+environment:[[:space:]]*evidence-lander' .github/workflows/evidence-lander.yml && grep -qE '^[[:space:]]+workflow_run:' .github/workflows/evidence-scope-post.yml && grep -qE '^[[:space:]]+environment:[[:space:]]*evidence-scope' .github/workflows/evidence-scope-post.yml && grep -qF 'github.event.repository.default_branch' .github/workflows/evidence-scope-post.yml && ! grep -qE 'statuses:[[:space:]]*write' .github/workflows/evidence-scope.yml && ! grep -qF 'pull_request_target' .github/workflows/evidence-scope.yml .github/workflows/evidence-scope-post.yml .github/workflows/evidence-lander.yml` | exit 0 (lander runs from the default-branch definition on all three triggers in its environment; the poster runs from the default-branch definition in its own environment with validator source from the default branch; the event-facing half cannot post a status; no `pull_request_target`). Under Option 2 the poster clauses are dropped and recorded as not-applicable in Evidence | check |
| 6 | `git diff $(git merge-base refs/remotes/origin/main HEAD)..HEAD -- .github/workflows/evidence-automerge.yml .github/workflows/leaksweep-control.yml .github/workflows/leaksweep-pattern.yml` | empty (the fallback lane and the leak workflows are untouched). The base is spelled `refs/remotes/origin/main` in full so that a stray local `origin/main` branch cannot become the comparison base | check +neighbour |
| 7 | `statusgen --consumers --root . --brief desk-supervision/23` | exit 0 (consumers routing corroborated against the diff) | check:ci |
| 8 | `statusgen --root . --lint` | exit 0 | check:ci |
| 9 | `gh api repos/medici-finance/assay/rules/branches/main --jq '[.[] \| select(.type=="required_status_checks") \| .parameters.required_status_checks[]]'` | contains `leak-sweep` and (Option 1) `evidence-scope`, each carrying an `integration_id`. The `evidence-scope` id equals the validator App's id (`gh api apps/<validator-app-slug> --jq .id`), NOT the generic GitHub Actions integration; the `leak-sweep` id equals the withheld-content poster App's id | gate:human +dereference |
| 10 | `for id in $(gh api repos/medici-finance/assay/rulesets --jq '.[].id'); do gh api repos/medici-finance/assay/rulesets/$id --jq '{name, rules: [.rules[].type], bypass_actors}'; done` (admin credential) | The lander App is a bypass actor ONLY on the ruleset holding the `pull_request` rule, and that ruleset holds no `required_status_checks`. The verifier App and the validator App are on no bypass list. The ruleset holding the required statuses lists only the pre-existing board-writer App. A ruleset targeting `refs/heads/evidence-landing/**` restricts creation, update and deletion, and its bypass list is exactly the verifier App and the lander App | gate:human +dereference |
| 11 | `for e in evidence-lander evidence-scope; do gh api repos/medici-finance/assay/environments/$e --jq '{name, policy: .deployment_branch_policy}'; done` | each shows `custom_branch_policies: true` and `protected_branches: false`, and `gh api repos/medici-finance/assay/environments/$e/deployment-branch-policies --jq '[.branch_policies[].name]'` is exactly the default branch's name: a run triggered from a staging ref, a PR ref or any other protected branch cannot read either key. `protected_branches: true` FAILS the row, since it admits every protected branch. Under Option 2 only `evidence-lander` exists | gate:human +dereference |
| 12 | DONE (a), planted mutation: the verify desk pushes a commit editing ONE Verify-table cell of this brief to `evidence-landing/example-canary`, then `gh api "repos/medici-finance/assay/commits/$(git rev-parse refs/remotes/origin/evidence-landing/example-canary)/status" --jq '.statuses[] \| select(.context=="evidence-scope") \| .state'` | `failure`. `git ls-remote origin refs/heads/main` is unchanged from before the push, and a finding naming `evidence-landing/example-canary` is filed (fallback, nothing dropped) | gate:human +mutation |
| 13 | DONE (b): under the implementer App credential, `git push origin refs/remotes/origin/evidence-landing/example-canary:refs/heads/main` | exit non-zero with `GH013` in stderr. The main tip is unchanged | gate:human |
| 14 | DONE (d), LOWER LAYER WITH THE UPPER BYPASSED: under the verifier App, with one real outcome line appended locally, `VERIFIER_MAIN_OK=1 deskevidence medici-finance/assay main --evidence-file docs/streams/verify-outcomes.jsonl`. The client-side refusal is deliberately switched off and the content is admissible Evidence, so only the server's identity scope can refuse | exit non-zero, and the forge refuses the write (ruleset violation). The main tip is unchanged. If it lands, the row FAILS: the line is a real outcome row, so the harm is bounded and the failure is itself the finding | gate:human +mutation |
| 15 | DONE (c) + FLOW: after a real Evidence-only landing through a staging ref, with `LANDED_SHA` exported as the new main tip: `gh api "repos/medici-finance/assay/commits/$LANDED_SHA/pulls" --jq 'length'` and `gh api "repos/medici-finance/assay/activity?ref=refs/heads/main&per_page=20" --jq '.[] \| select(.after == env.LANDED_SHA) \| {actor: .actor.login, activity_type}'` | `0` (no PR), and the ref update that set main to `LANDED_SHA` names the lander App's bot login as `actor` with `activity_type` `push`. The commit author is deliberately NOT asserted: a fast-forward keeps the staging commit's author (the verifier App). The post-land audit run on that push reports `checked-clean`. Fail-first: the same push attempted before Task step 1 was refused `GH013`, recorded in Evidence | gate:human +flow |
| 16 | `grep -qF 'evidence-landing/' plugins/assay/skills/verify-desk/SKILL.md && grep -qiF 'lander' docs/adopting-assay.md` | exit 0 (the skill and the adopter doc name the lane) | check |
| 17 | `test -s .github/workflows/evidence-scope-post.yml && test -s .github/workflows/evidence-lander.yml && ! grep -hE '[$][{][{][^}]*github[.]event[.]' .github/workflows/evidence-scope-post.yml .github/workflows/evidence-lander.yml \| grep -qvE '^[[:space:]]+[A-Z][A-Z0-9_]*:[[:space:]]*[$][{][{]' && ! grep -qF 'download-artifact' .github/workflows/evidence-scope-post.yml .github/workflows/evidence-lander.yml && grep -qE '[$][{]?[A-Z_]*SHA[}]?:refs/heads/main' .github/workflows/evidence-lander.yml && ! grep -E ':refs/heads/main' .github/workflows/evidence-lander.yml \| grep -qvE '[$][{]?[A-Z_]*SHA[}]?:refs/heads/main' && ! grep -E 'git[[:space:]](.*[[:space:]])?push' .github/workflows/evidence-lander.yml \| grep -qvE '[$][{]?[A-Z_]*SHA[}]?:refs/heads/main' && ! grep -qF 'git/refs/heads/main' .github/workflows/evidence-lander.yml` | exit 0: in the two credential-holding workflows every `github.event.*` interpolation sits on an upper-case `env:` assignment line, never inside `run:` or `if:`, and neither downloads an artifact from a triggering run; the lander's every push to main is a refspec from a validated-SHA variable, never a ref name; every line of the lander workflow that invokes `git` with a `push` (whatever global options such as `-C` or `-c` come between; the match is deliberately loose, so a comment mentioning a push also fails the row, which fails closed) is that SHA-refspec form, so a shorthand destination (`:main`, `HEAD:main`) fails the row; and the workflow names no refs-API write of `heads/main`. This is a grep over the workflow text, so it bounds the forms it matches, not every way a program could write a ref; the server's required `evidence-scope` on the exact pushed SHA (Option 1) stays the layer behind it. Fail-first: a planted `run: echo ${{ github.event.workflow_run.head_branch }}` line makes the row exit 1, and so does each of a planted `git push origin "${STAGING_REF}:refs/heads/main"`, `git push origin "${STAGING_REF}":main`, `git push origin HEAD:main`, `git -C "$W" push origin HEAD:main`, `git -c http.extraheader=X push origin "$STAGING_REF":main`, a split `git push origin \` line, and a `gh api -X PATCH repos/medici-finance/assay/git/refs/heads/main` line. Under Option 2 the poster file is dropped from both greps | check +mutation |
| 18 | LOWER LAYER WITH THE UPPER BYPASSED, live (Option 1): open a draft canary PR from `example-canary-pr`, a fast-forward of main that edits ONE Verify-table cell of this brief; approve it at its head so the poster posts the not-a-landing pass; let `leak-sweep` post. With `CANARY_SHA` exported as that head: `gh api "repos/medici-finance/assay/commits/$CANARY_SHA/status" --jq '[.statuses[] \| select(.context=="evidence-scope" or .context=="leak-sweep") \| .state]'` then `git ls-remote origin refs/heads/main` | both statuses `success` (the server layer alone would accept a push of this SHA), yet main is NOT `CANARY_SHA`, and the lander runs those statuses triggered each logged `skip` for `CANARY_SHA` (not a staging ref's head). The canary PR is then closed unmerged | gate:human +mutation |
| 19 | Writer binding, live. (a) Under the implementer App credential: `git push origin refs/remotes/origin/main:refs/heads/evidence-landing/example-canary-writer`, then `git ls-remote origin refs/heads/evidence-landing/example-canary-writer`. (b) After row 15's real landing, with `LANDED_SHA` exported: `gh api "repos/medici-finance/assay/commits/$LANDED_SHA" --jq '{verified: .commit.verification.verified, reason: .commit.verification.reason, committer: .committer.login}'`. (c) After a real `remerge` made after a concurrent landing (a staging ref that appends to `verify-outcomes.jsonl` left waiting while ANOTHER landing, which also appends to `verify-outcomes.jsonl`, moves main, so the lander merges main into it and the forge's merge must apply `merge=union`), with `REMERGE_SHA` exported as that two-parent commit and `REMERGE_REF` as its full `refs/heads/evidence-landing/...` name, in a fresh clone on git 2.43 or later: `git fetch origin && test ! -s "$(git rev-parse --git-path info/attributes)" && test "$(git rev-parse "$REMERGE_SHA^{tree}")" = "$(GIT_ATTR_NOSYSTEM=1 GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null git -c core.attributesFile=/dev/null --attr-source="$REMERGE_SHA^2" merge-tree --write-tree "$REMERGE_SHA^1" "$REMERGE_SHA^2")"`, then `gh api "repos/medici-finance/assay/activity?ref=$REMERGE_REF&per_page=50" --jq '.[] \| select(.after == env.REMERGE_SHA) \| {actor: .actor.login, before}'` | (a) the push exits non-zero with `GH013` in stderr and the ls-remote prints nothing: no identity but the verifier App and the lander App can create a staging ref. (b) `verified` true, `reason` `valid`, `committer` the verifier App's bot login: the verify desk's real write mechanism yields exactly what Task step 2's binding expects. (c) the command exits 0 (no runner-local `info/attributes` outranks the pinned source, no global or system config or attributes file takes part in the recompute, and the forge's own merge commit of two concurrent outcome appends has exactly the tree the pinned recompute yields), and the activity entry names the lander App's bot login as `actor` with `before` equal to the commit's staging-side parent: the lander's real remerge mechanism yields exactly the three `remerge` fields step 2 binds. If (b) or (c) reads otherwise the row FAILS and the implementation's binding is re-examined, never loosened to fit (a forge merge whose tree differs from the recompute is already `reject` under step 2, so the lane falls back rather than landing). If the forge's merge of the two concurrent appends conflicts, so the lander falls back and no remerge commit exists to export, the row also FAILS: the forge does not apply `merge=union`, every concurrent-append remerge would fall back (fail-closed, a liveness cost), and that is filed as a finding | gate:human +mutation |
| 20 | `( for f in .github/workflows/evidence-scope.yml .github/workflows/evidence-scope-post.yml .github/workflows/evidence-lander.yml; do test -s "$f" && grep -qE '^[[:space:]]+runs-on:' "$f" && ! grep -E '^[[:space:]]+runs-on:' "$f" \| grep -qvE '^[[:space:]]+runs-on:[[:space:]]*ubuntu-[a-z0-9.]+[[:space:]]*$' && test "$(grep -cE '^[[:space:]]+runs-on:' "$f")" -le "$(grep -cE '^[[:space:]]+timeout-minutes:' "$f")" && ! grep -qE '^    uses:' "$f" \|\| exit 1; done )` | exit 0: in each of the three workflows every `runs-on:` line is a single GitHub-hosted `ubuntu-` label (no self-hosted label, no label list), there are at least as many `timeout-minutes:` lines as jobs, and no job calls a reusable workflow (a job-level `uses:` at the standard four-space indent). This is a grep over the workflow text, so it bounds the forms it matches (a `ubuntu-` name given to a self-hosted runner, or a non-standard indent, gets past it); row 21's live read of where the jobs actually ran is the layer behind it. Fail-first: in a scratch copy of one file, each of `runs-on: self-hosted`, `runs-on: [self-hosted, linux]`, a job naming the self-hosted CI label, one job's `timeout-minutes:` removed, and a job-level `uses: ./.github/workflows/x.yml` makes the row exit 1, and so does a missing file. Under Option 2 the poster file is dropped from the loop | check +mutation |
| 21 | RUNNER ISOLATION, live. After row 15's real landing, with `LANDER_RUN_ID` exported as the `evidence-lander.yml` run that pushed `LANDED_SHA`, `POSTER_RUN_ID` as the `evidence-scope-post.yml` run that posted `evidence-scope` on it, and `AUDIT_RUN_ID` as the `evidence-scope.yml` run that audited the push: `for r in "$LANDER_RUN_ID" "$POSTER_RUN_ID" "$AUDIT_RUN_ID"; do gh api "repos/medici-finance/assay/actions/runs/$r/jobs" --jq '.jobs[] \| {name, runner_group_id, runner_group_name, conclusion}'; done` | every job of all three runs, a called workflow's jobs included, prints `runner_group_id` 0 and `runner_group_name` `GitHub Actions` (the hosted pool, a fresh machine per job; read 2026-09-24 on this repository, a hosted job reads exactly that and a job on the self-hosted CI label reads group `Default`). Any other group, or a job with no group, FAILS the row: a key-holding job ran on a machine another job may have used. Under Option 2 `POSTER_RUN_ID` is dropped | gate:human +dereference |

Pre-mortem (failure mode → row):

| Failure mode | Caught by |
|---|---|
| Validator admits a Verify-table or frontmatter edit | rows 1-2 (and row 3 after the fact) |
| An intermediate commit carries content the net diff hides | row 1 (per-commit unit); row 3 (iii) |
| Validator skipped or buggy, and bad content lands anyway | row 3 (audit + halt); row 9 (server-required status, Option 1); row 4 (lander's own re-derivation) |
| Audit selects by commit author and misses a fast-forward landing | row 3 (i) (verifier-authored commit pushed by the lander is still audited); row 15 (pusher, not author) |
| An implementer (or any non-verifier identity) writes admissible Evidence on its own work to a staging ref and it lands unreviewed | row 19 (a) (staging-ref ruleset refuses the write); rows 1 and 4 (writer binding rejects it, content notwithstanding); row 3 (iv) (the audit flags it after the fact) |
| Writer binding trusts a forgeable name, or does not match the real write mechanism | row 1 (unsigned commit claiming the verifier's identity is `reject`); row 19 (b) (live signature verdict and committer); row 19 (c) (live remerge actor, previous head and tree) |
| A lander-key holder writes a two-parent commit (not through the merges API) whose tree injects Evidence neither parent carries | row 1 (tree differs from the recomputed merge: `reject`); row 3 (v) (the audit's own recompute: `checked-failed`, halt); row 19 (c) (the real remerge matches the recompute, so the check does not fail the lane's own merges) |
| A concurrent landing's outcome-log append makes every remerge fall back, or recomputes disagree on how the `merge=union` log resolves | row 1 (both sides appended, and a second remerge: `admit` under the pinned attribute source and the merge form); row 3 (the audit's own recompute and merge form read the same union remerge in a lander push as `checked-clean`, no finding, so an audit without the merge form fails the row instead of halting the lane live); row 19 (c) (the forge's merge of two concurrent appends equals the pinned recompute) |
| A runner-local `info/attributes`, a global or system attributes or config file (a merge rule or a merge driver for a path the pinned source leaves unset), or a git older than 2.43, changes or breaks the pinned recompute | row 19 (c) (fresh clone, empty `info/attributes`, the recompute run with global and system config and attributes neutralised, git 2.43 or later); rows 20-21 (hosted runners carry no state from an earlier job); in an adapter an old git or a `merge-tree` exit outside 0 and 1 is `could-not-check`, which falls back or halts, never admits |
| A key-holding job (poster, lander or audit) runs on a runner shared with PR-authored jobs and executes state an earlier job left there (`$HOME`, `~/.gitconfig`, `PATH`), reaching layers 1, 2 and 4 through one channel | row 20 (every job in the three workflows names a GitHub-hosted label); row 21 (live: every job of a real landing's lander, poster and audit runs ran in the hosted group) |
| An audit run ends without a verdict (timeout, cancellation, lost runner) and the lander push it was auditing goes unaudited | row 4 (audit silence: the lander `refuse`s and files the halt finding itself); keeping the retry budget at most half the job's `timeout-minutes` is review-only (the budget is a constant in the implementation and the timeout a workflow field, and row 20 checks only that the timeout is set) |
| An audit that cannot read an input in a lander push stays silent | row 3 (`could-not-check` files the halt-labelled finding); row 15 (a real landing's audit reads `checked-clean`) |
| A transient forge read error halts the lane | row 3 (a read that fails once and succeeds within the bounded retry is read; the halt finding names its cause, so a human clears a transient halt quickly) |
| A landed staging ref kept for the audit is re-landed, remerged or fallen back, or its sweep drops a later verifier commit, or the update history a human needs to clear a halt | row 4 (a ref whose head main already contains gives `skip`); step 5 (the sweep deletes only after a `checked-clean` report, only while the head equals the landed SHA, and never a ref an active halt finding names) is review-only, and applies only if row 15 triggers the contingency |
| A board-writer release or quality regen halts the lane | row 3 (one fixture per board-writer surface not selected; a non-lander mismatch files a finding without the halt label) |
| A bot clears the halt and resumes the lane | row 4 (a halt finding closed or unlabelled by a bot still gives `refuse`) |
| The lander remerges into, or lands from, a ref whose batch Evidence PR is open | row 4 (`fallback` is terminal per ref: `skip`) |
| A commit added to the staging ref between the re-read and the push rides along | row 17 (push by validated-SHA refspec only) |
| Lander lands an approved or unapproved PR head carrying the not-a-landing pass | row 4 (`skip`); row 18 (live) |
| `evidence-scope` forged by another workflow or identity | row 9 (integration pinned to the validator App); row 5 (event-facing half cannot post); row 11 (poster key default-branch only); row 4 (unexpected creator counts as missing) |
| Lander steered by attacker-named event fields | rows 5, 17 |
| Verifier App gains a direct main write | rows 10, 14 |
| Lander key readable from a staging-ref run | rows 5, 11 (custom policy naming only the default branch; `protected_branches` fails the row) |
| Landing bypasses the withheld-content sweep | rows 9-10 (lander not on that ruleset); row 4 (no land without `leak-sweep`) |
| `leak-sweep` arrives after the validator, and the landing is stranded | row 4 (`wait`, then `fallback` past the window); row 5 (`status` and `schedule` triggers) |
| Rejected landing silently dropped | rows 4, 12 |
| Rebase or force on race | row 4 (`remerge` is a two-parent merge); `non_fast_forward` stays on both rulesets (row 10) |
| Ruleset later widened by hand | no row. Review-only: re-audit is DR-server-controls' standing commitment |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 9-15, 18, 19 and 21 need Task
     step 1 done; until then they are could-not-check, never pass. -->

## Review
Gate: human (from frontmatter: sensitive-data and irreversible are yes). The human gate is MANDATORY.
The reviewer answers BOTH, in the verdict:
1. What is the single control standing between the fault and the damage, and is that acceptable?
   (The SPOF line above names the lander's PR-rule bypass. Confirm the layers behind it are
   present at the chosen option, and that the assumption beneath layers 1, 2 and 4, key-holding
   jobs on GitHub-hosted runners only, is bound by Task step 3 and read back by rows 20-21.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER layer bypassed? (Rows
   3, 14 and 18 are designed to; rows 3 (iv) and 3 (v) cover the writer-identity fault and the
   injected-remerge fault with no validator verdict in the path. Confirm they bypass the upper layer rather than walking the happy path
   through every layer at once.)

Reviewer records verdict + date in the stream README table.
