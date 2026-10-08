---
brief: assay:assay:forge-gitlab:13
title: Board reads degrade per row, never per sweep — the GitLab empty-field class
why: >-
  A review desk that cannot see its queue has no other symptom to report. On a GitLab-resolved
  project `deskboard actions` exited 6 with an empty board and one line, `compare needs both base
  and head`, whenever a SINGLE open change carried a review the forge could not pin to a commit —
  and the GitLab backend leaves that commit id empty BY DESIGN, because a GitLab approval carries
  no sha. That ONE arm was fixed on 2026-09-14 and its row now degrades. The CLASS did not move:
  five per-change reads in the classifier still propagate a per-change refusal as a whole-sweep
  error, while the read sitting beside them documents the opposite contract in as many words —
  one unreadable change degrades ITS row, closed, so one change cannot brick the desk. The next
  honestly-empty or unreadable GitLab field on any of those five reaches for the same blank
  board, and the adopter finds it again. This brief makes the degrade the class property of the
  sweep rather than one arm's local repair.
wave: 3
depends: ["forge-gitlab/02"]
unblocks: ["forge-gitlab/16"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1067]
schema: brief-v2
authored: 2026-09-14 by forge-gitlab wave-plan authoring session
sources:
  - "#1067 — the field report from a review desk on a self-managed GitLab adopter: `deskboard actions --delta --quiet` and `deskboard actions --repo <slug>` both exit 6 with empty stdout and the single line `compare needs both base and head`; two open merge requests had classified in the same window on an earlier build. The ask is a build that classifies GitLab changes, or a refusal that names the missing fields — not a blank board"
  - "#1071 — the sequencing notification this plan answers: the GitLab review-desk items are a chain whose fixes uncover the next latent defect, not independent one-offs"
  - "PR #1068 (MERGED 2026-09-14) — the single-site fix for the benign-merge arm, landed while this plan was being authored. It guards that ONE call with a both-shas-present test, degrades the row to the safe side, and prints a diagnostic naming which endpoint could not be established. It is the SHAPE this brief generalises and the arm this brief must NOT re-land. Issue #1067 is still open at the time of writing and needs a close, not work"
  - "tools/desk/cmd/deskboard/board.go — the classifier. After PR #1068 the benign-merge arm is guarded by a both-shas-present test before the compare is attempted; `classifyPR` still carries FIVE whole-sweep error returns, at lines 1942, 1970, 2018, 2022 and 2043 on 2c67b34f's successor, and every one of them is reached with a per-change input"
  - "tools/desk/cmd/deskboard/board.go — `fetchChangedFiles`' own header states the opposite, already-correct contract in as many words: an incomplete read is not an error, the caller degrades CLOSED per change, so one enormous change cannot brick the desk's sweep"
  - "tools/desk/internal/deskkit/forge_gitlab.go — `ReviewsAtHead` pins a commit id only where GitLab can establish one and leaves it EMPTY otherwise, deliberately and at length: a GitLab approval carries no sha and no timestamp, and whether it survives a push is a project setting, so stamping it would be an invention. The empty field is the CORRECT value and the backend is not the fix"
  - "docs/streams/forge-gitlab/spec.md §3 — parity per control even where the mechanism differs; a control that cannot read its input reports could-not-check, it does not fail open"
  - "freshness-checked 2026-09-14, after PR #1068 merged at 19:34Z — the refusal literal is still live in `changedFilesBetween`, now reached only past the new guard; `classifyPR` still carries five whole-sweep error returns; `ReviewsAtHead` on the GitLab backend still leaves the commit id empty by design. The instance is closed; the class is open"
exec-tier: strong
exec-tier-why: "correctness is a property of the whole read graph, not of any one arm (question b) — the audit must decide, per read, whether its input is repo-level (sweep-fatal is right) or change-level (degrade is right); and a degrade written on the permissive side fail-opens the board's UNREVIEWED alarm, which no happy-path test observes (question c)."
domain: complicated
tier: free
consumers:
  - "tools/desk/cmd/deskboard/board.go: fixed-here (each change-level read in `classifyPR` gains the documented per-row degrade; repo-level reads stay sweep-fatal and say so at the site)"
  - "tools/desk/cmd/deskboard/*_test.go: fixed-here (a single-unreadable-change regression test asserting the sweep still exits 0 and the other rows still render)"
  - "tools/desk/cmd/issueboard: out-of-scope (the issue lane's reads are a separate sweep with its own classifier; no issue-lane defect is on file, and widening this brief to a second sweep would make it L)"
  - "docs/streams/forge-gitlab/README.md: fixed-here (the status row)"
version: 1
id: 6b50f112-19d1-44fe-9326-54906ce5e1ed
---

# Brief 13 — Board reads degrade per row, never per sweep

## Context

The board's classifier reads several facts per open change. Some of those facts are
**repo-level** — the repo could not be resolved, the forge could not be constructed, the
credential is absent. A failure there is genuinely sweep-fatal: nothing about the repo can be
classified, and refusing is correct.

The rest are **change-level** — this change's changed files, this change's reviewed sha, this
change's brief resolution. A refusal there is a fact about ONE row. Five of them are returned as
if they were the first kind, so one unreadable change produces exit 6, an empty board, and one
line of diagnosis for a queue that may hold a dozen healthy changes.

One of those five was repaired on 2026-09-14: the benign-merge compare is now guarded by a
both-shas-present test, the row degrades to the safe side, and the diagnostic names which
endpoint could not be established. That repair is the SHAPE this brief generalises — it is not
the class, and the other four are unchanged.

GitLab is where this stops being theoretical. The GitLab backend leaves a review's commit id
empty wherever the forge cannot establish one, and its own comment explains why that is the
right value rather than a hole to plug: a GitLab approval carries no sha and no timestamp, and
whether it survives a push is a project setting, so stamping the current head onto it would
manufacture an at-head verdict nobody gave. The reviewed-sha reduction then folds two different
facts into one not-at-head answer — *the head genuinely advanced* and *one of the two shas was
never established* — which is also deliberate, so that an unread sha can never pass as an
at-head one. The benign-merge arm consumes that single boolean as if it always meant the first,
asks for the diff between an empty base and the head, and gets the refusal.

So there is no bug in the backend and no bug in the reduction. The defect is the **blast
radius** of a change-level refusal, and the fix is the contract the read sitting beside it
already documents for itself.

files:
- `tools/desk/cmd/deskboard/board.go` — the classifier. Each change-level read gains the
  per-row degrade; each repo-level read keeps the sweep-fatal return and carries a one-line
  comment saying which kind it is, so the next reader does not have to re-derive the
  distinction. The benign-merge arm is already repaired by PR #1068 — do NOT re-land it; treat
  its guard, its safe-side degrade and its which-endpoint diagnostic as the reference shape the
  other four sites are brought up to.
- `tools/desk/cmd/deskboard/*_test.go` — the regression test: a sweep over several open changes
  where exactly one carries an unreadable change-level field still exits 0, still renders every
  other row, and renders the affected row on the SAFE side.
- `docs/streams/forge-gitlab/README.md` — the status row.
- `changelog/forge-gitlab-13-board-reads-degrade-per-row.md` (planned).

single-point-of-failure: the direction of the degrade is the one control — a change whose
input could not be read must land on the side that asks for a human look (re-review /
needs-review), never on the side that reads as cleared. It is backed by two independent
layers that fail on different signals in different components: the classifier's own degrade
(a per-row decision made where the read failed) and the row's rendering, which carries the
could-not-check reason into the board text, so a row that degraded silently is visible as a
row that degraded — an operator reading the board is the second layer, and neither layer can
be satisfied by the other going wrong.

facts:
- The refusal literal is `compare needs both base and head`, raised where the compare helper is
  handed an empty base or head. It is now reached only past PR #1068's guard.
- `classifyPR` carries five whole-sweep error returns (lines 1942, 1970, 2018, 2022, 2043 as of
  2026-09-14). Classifying each as repo-level or change-level is the audit this brief delivers;
  the count is the checklist, not the fix.
- The reference shape is PR #1068's: a precondition test before the read, a degrade to the SAFE
  side, and a diagnostic that names WHICH field could not be established — because a degrade
  that does not say which field sends its reader to the wrong forge surface.
- The already-correct contract to copy is stated in the changed-files reader's own header: an
  incomplete read is not an error, the caller degrades closed per change, so one enormous
  change cannot brick the sweep.
- The GitLab empty commit id is CORRECT and stays. No change to `forge_gitlab.go` is in scope.
- `deskboard --version` in the field report printed a source sha; the defect is not
  version-specific and reproduces on `2c67b34f`.

## Edition
Minimum GitLab tier: **free**. Nothing here reads a tier-gated field; the empty commit id is a
property of the GitLab approval model on every tier, not of Community Edition. No new
degradation is disclosed — this brief removes a failure mode, it does not add a divergence.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity
  does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do NOT "fix" the empty commit id in the GitLab backend. It is the honest value and the
  backend says so; plugging it is how a verdict nobody gave becomes at-head.

## Task
1. Audit every error return in the classifier's per-change path and label it **repo-level**
   (stays sweep-fatal) or **change-level** (degrades). Record the labelling as a comment at
   each site — the audit is a deliverable, not a working note.
2. Give every change-level read the per-row degrade: the row is produced, it carries the
   could-not-check reason, and it lands on the side that asks for a human look. Reuse the
   existing changed-files degrade shape rather than inventing a second one.
3. Add the regression test: a multi-change sweep where exactly one change has an unreadable
   change-level field exits 0, renders every other row, and renders the affected row on the
   safe side. Include the empty-reviewed-sha case, which is the GitLab shape #1067 reports.
4. Add a negative-path test proving the degrade cannot be satisfied by the permissive side: a
   degraded row must not classify as the benign-merge outcome.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | exit 0; output contains `PASS` — a sweep over several open changes, exactly one carrying an empty reviewed sha, exits 0 and renders every other row (`TestSweepDegradesOneRowNotTheBoard` (planned)) | check +flow |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | exit 0; output contains `PASS` — the degraded row is NOT classified as the benign-merge outcome (`TestDegradedRowNeverReadsBenign` (planned)) | check |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | `5` or more — every audited error return carries its labelling at the site | check |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` run against the pre-fix tree (stash the fix, or check out the parent commit) | exit non-zero; the failure names the whole-sweep refusal, proving the test observes the defect it pins | check +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | exit 0; output contains `PASS` — the RENDERED row text carries the could-not-check reason that produced the degrade, so an operator can see which row degraded and why. This resolves the SPOF note's second-layer claim against the real output rather than counting that a row exists (`TestDegradedRowCarriesItsReason` (planned)) | check +dereference |
| 7 | `statusgen --root . --consumers` | exit 0 | check |
| 8 | `statusgen --root . --lint` | exit 0; output contains `LINT: PASS` | check |

### Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The degrade is applied to a repo-level read too, so a missing credential silently yields rows of could-not-check instead of a refusal | row 4's labelling + review of each site — adequacy of the repo/change split is review-only |
| The degraded row lands on the permissive side and the board reports a change as cleared that nobody read | row 3 |
| Only the already-repaired arm is covered and the other four still blank the sweep | row 2 exercises the sweep through a read OTHER than the benign-merge arm; row 4 pins the audit coverage |
| The test passes on the unfixed tree because the fixture never reaches the failing arm | row 5 (fail-first) |
| The fix "resolves" the empty commit id in the GitLab backend instead, manufacturing an at-head verdict | no row — Ground rules forbid it and the backend diff is review-only; a `forge_gitlab.go` hunk in this PR is a review bounce |

### Dispatch checklist
```
[x] 1. Rows discriminate — row 3 goes red on a permissive degrade; row 5 goes red on a test that never saw the defect.
[x] 2. Facts dated — every fact carries the 2026-09-14 @ 2c67b34f freshness check.
[x] 3. Self-contained — the refusal literal, the call site, the existing degrade contract and the GitLab rationale are all quoted here.
[x] 4. Risk answers match files: — cmd/deskboard only, a read-path blast-radius change; no credential, no write, no identity: all four stay no.
[x] 5. gate-why n/a (gate: model, all four no).
[x] 6. Effort honest — an audit of five sites plus two tests plus the arm: M, not S.
[x] 7. No shared value changes; consumers: names the one sweep out of scope and why.
[x] 8. Pre-mortem run; the one review-only item is named.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | pass exit=0 | sha256:6e71ed00a142 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | pass exit=0 | sha256:a3542fb74ec1 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | pass exit=0 | sha256:10159baf262b | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` | pass exit=0 | sha256:027f2411437b | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | pass exit=0 | sha256:283acfbd27c7 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --consumers` | pass exit=0 | sha256:c3bffb26510f | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint` | pass exit=0 | sha256:406c7e5f2433 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |

**Verifier notes (2026-09-28, re-witnessed at merged main after main advanced — the brief and cmd/deskboard are unchanged between the earlier and this head; assay-verifier-app[bot], non-implementer; merged main c50a38fc12518a4eec4db37e8dd847d49e79149a; implementing change PR #1084, commit 12087009339a50e166b76fddcc1642252387c950).**
Witness run with the pinned statusgen v1.0.27 binary (sha256 matches the pin) in a clean
environment with outbound network denied and only loopback allowed (the deskboard tests
serve fixtures on a loopback listener). No Verify cell mints a credential or calls a live forge.

- Row 1 — the witness recorded could-not-run: check:ci needs a Linux network-off runner
  (medici-finance/assay#1800). Run by hand in the loopback-only sandbox, the same command
  exited 0 (`ok .../cmd/deskboard 63.4s`). Held for the Linux runner.
- Row 2 — exit 0, `--- PASS: TestSweepDegrades…Board`. The fixture's sweep degrades one row
  through the reviews read, which is not the benign-merge arm.
- Row 3 — exit 0, subtests `own-files read fails` and `compare read fails` both PASS.
- Row 4 — output decodes to `7`, above the 5 floor. Six of the seven are real audit labels
  (one repo-level at the open-changes list read, five change-level in the classifier). The
  seventh is an unrelated worker-pool comment ("repo-level × PR-level").
- Row 5 — the witness `pass` is VACUOUS. The first code span runs on the fixed tree, and
  "against the pre-fix tree" is prose. Fail-first run by hand in a scratch copy: the fix
  commit's tree with the classifier file put back to its parent's version. Result: exit 1,
  `sweepActionsRepo returned cannot read reviews for example-org/tracker#52: simulated:
  cannot read reviews — one PR … must degrade its OWN row, never fail the whole sweep`. The
  same tree with the fixed classifier exits 0. The test observes the defect. Held because the
  witness cannot express this row.
- Row 6 — exit 0, PASS; the rendered Note carries `— DEGRADED: <reason>`.
- Row 7 — exit 0, but vacuous on merged main: `consumers: no brief files in the diff … —
  nothing to corroborate` (the medici-finance/assay#1281 class). Held.
- Row 8 — exit 0, `LINT: PASS`, 0 PROBLEM lines on the pre-Evidence tree. The only NOTICE
  naming this brief is `risk-files-crossread`: the brief answers all four risk questions
  "no" while its declared path sits under a security-path trigger.

**Observations for review (not Verify failures).**
- A sixth whole-sweep return is still in the classifier and has no label: the trust-gate
  blessing read for an untrusted author (`prBlessed` error, then `return prOutcome{}, berr`).
  It is a per-change read. At the pre-fix baseline the classifier had six
  `return prOutcome{}, …` returns, and the fix degraded five. Nothing in the task-1 audit
  says whether this one is repo-level or change-level. Its own header says the exit 6 is
  deliberate (Unverifiable), and a quarantine degrade would be the fail-closed alternative.
  This is the brief's Review question, and it is open.
- When the reviews read fails, `rs` degrades to its zero value (no verdict). For a trusted
  human author the row then lands HUMAN-OWNED, not NEEDS-REVIEW. HUMAN-OWNED is not a
  cleared state and the DEGRADED note is appended. But a standing bot CHANGES_REQUESTED at
  head that could not be read would show as HUMAN-OWNED instead of BLOCKED, and HUMAN-OWNED
  is outside the UNREVIEWED alarm. The rendered DEGRADED reason is the only layer that
  shows it.

**Risk-bearing value enumeration.** The trigger fires because the diff touches a
risk-classed path (the lint's crossread NOTICE). Scope: the classifier hunks of the
implementing commit. The fix adds no numeric constant. The literals are the degrade-direction
bindings and the rendered marker, all in tools/desk/cmd/deskboard/board.go at c50a38fc1251:
`in.ownFilesChanged = true` @ :2205 and :2213 (new; :2194 is the #1068 arm, reworded);
`complete = false` @ :2243; `var rs reviewState` zero value (ever=false) @ :2113; the
separator `" — DEGRADED: "` @ :2279. Outside the diff, the Verify timeouts (120s/300s) and
the row-4 floor `5` are check-definition knobs. Ranking: every entry can be undone by an
edit and a redeploy. The board is a read-only classifier and does no irreversible act.
The three direction bindings rank first because a wrong value there fails OPEN (a row reads
as cleared when nobody read it). The separator and the timeouts rank last.

- RISK-VALUE: DERIVED — in.ownFilesChanged = true @ tools/desk/cmd/deskboard/board.go:2205,2213 — MERGE-CURR asserts that the change's own files did not change since review. After a failed own-files or compare read that fact is not established, so `true` (RE-REVIEW) is the only value that does not assert something unobserved. It matches the existing truncation contract (an incomplete own-files set yields true) and spec §3 (could-not-check, never fail open).
- RISK-VALUE: DERIVED — complete = false @ tools/desk/cmd/deskboard/board.go:2243 — the risk gate is a union that only widens. A diff that could not be read may contain the trigger, and `case !complete` then sets riskClassed. This is the same fail-closed arm a truncated diff already takes, so a read error can never reach MERGE-NOW or FLIP.
- RISK-VALUE: DERIVED — rs = reviewState{} zero value (ever=false) @ tools/desk/cmd/deskboard/board.go:2113 — any other value would invent a verdict nobody gave. ever=false routes through the existing no-verdict arm (NEEDS-REVIEW, or HUMAN-OWNED for a trusted human author), and neither is a cleared state. See the HUMAN-OWNED caveat above; it is open for the human.

VERIFY: PASS — every Verify expectation was observed. The witness genuinely proves rows 2, 3, 4, 6 and 8. Row 1 passed by hand; its witness record is could-not-run pending a Linux runner (medici-finance/assay#1800). Row 5 was proven fail-first by hand. Row 7 exits 0 but is vacuous on merged main. Not witness-complete, so hold rows 1, 5 and 7.

### Non-implementer verifier re-run: 2026-10-02T22:09:24Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main cf31c32418ba49f93c679913813768542db1c072

Host: darwin/arm64, go1.27.1, statusgen v1.0.31. Every go command ran with a throwaway HOME and with the temp directory redirected into the verifier's own scratch directory (module and build caches left at their real location). No credential was minted and no live forge was called; the deskboard tests serve fixtures on loopback.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | exit 0 | By hand: exit 0, `ok  github.com/medici-finance/assay/tools/desk/cmd/deskboard 78.780s`; repeated with `-count=1` to defeat the test cache: exit 0, `ok … 55.605s`. The execution witness records could-not-run for this row on this host (check:ci needs a network-off sandbox, #1800). | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | exit 0; output contains `PASS` | exit 0, `--- PASS: TestSweepDegradesOneRowNotTheBoard (0.00s)` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | exit 0; output contains `PASS` | exit 0, `--- PASS: TestDegradedRowNeverReadsBenign (0.00s)` with subtests `own-files_read_fails` and `compare_read_fails` both PASS | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | `5` or more | exit 0, output `7` (one repo-level label, five change-level labels, one unrelated worker-pool comment) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` against the pre-fix tree | exit non-zero; the failure names the whole-sweep refusal | Mutation half applied to scratch copies, never to the tracked tree. (a) Tree of the implementing commit 12087009339a with the classifier file put back to its parent a7393d1dec0b: exit 1, `--- FAIL: TestSweepDegradesOneRowNotTheBoard`, `sweepActionsRepo returned cannot read reviews for example-org/tracker#52: simulated: cannot read reviews — one PR … must degrade its OWN row, never fail the whole sweep`. (b) Merged-main tree with the reviews-read degrade put back to a whole-sweep return: exit 1, same failure line. Code span as written on the unmodified merged-main tree: exit 0. | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | exit 0; output contains `PASS` | exit 0, `--- PASS: TestDegradedRowCarriesItsReason (0.00s)` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `statusgen --root . --consumers` | exit 0 | exit 0, `consumers: no brief files in the diff against cf31c32418ba… — nothing to corroborate` (exits 0 but corroborates nothing on merged main) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `statusgen --root . --lint` | exit 0; output contains `LINT: PASS` | exit 0, `LINT: PASS`, 0 lines beginning PROBLEM | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness** (`statusgen verifyrun --brief … --dry-run`, v1.0.31, nothing written to the brief): overall exit 2; 7 of 8 rows pass, 1 could-not-run.
- row 1: could-not-run (exit=-, sha256:e3b0c44298fc) — check:ci hermetic execution requires a network-off sandbox; the sandbox uses `unshare --net`, a Linux facility, and this host is darwin.
- row 2: pass (exit=0, sha256:0184a9359a86)
- row 3: pass (exit=0, sha256:36603f7749b2)
- row 4: pass (exit=0, sha256:10159baf262b) — exit-status only
- row 5: pass (exit=0, sha256:064eb0ffddd0) — exit-status only; the witness executes the code span on the fixed tree, so this pass does not show the fail-first half (shown by hand above)
- row 6: pass (exit=0, sha256:f3bb483e5da7)
- row 7: pass (exit=0, sha256:28cc4c1449d6)
- row 8: pass (exit=0, sha256:71a986d6d12a)

Disclosure: the witness dry-run executed the brief's go test rows itself under the ordinary HOME, not the throwaway one used for the by-hand runs. A read of the audit log afterwards showed no fixture-shaped rows in the witness window.

**What changed since the 2026-09-28 record (blocked, #1800).**
- Declared inputs that changed on main: the classifier and two of its test files, by #1851 (review-queue snapshot read) and #2040 (trusted humans' changes are reviewed; the HUMAN-OWNED class is retired). The brief, the sweep test file and the stream README are byte-identical to the recorded hashes.
- Row 1: unchanged. Still passes by hand; the witness still records could-not-run on darwin. #1800 is OPEN (read at this run), so the blocker stands.
- Row 5: unchanged in kind. The witness pass is still on the fixed tree; fail-first re-proven by hand on both the implementing commit and the current merged-main classifier, so the test still observes the defect after the two later changes.
- Row 7: unchanged. Exit 0, nothing to corroborate on merged main.
- Rows 2, 3, 4, 6, 8: pass by hand and in the witness, as before.

**Findings (not Verify failures).**
- The trust-gate blessing read for an untrusted author is still a whole-sweep return with no repo-level / change-level label (the only remaining `return prOutcome{}, <err>` in the per-change path, board.go:2123). The Review question about it is still open.
- The earlier observation that a failed reviews read on a trusted human's change landed HUMAN-OWNED is superseded: #2040 retired that class; the classifier now carries one comment mentioning the retired arm and no such outcome, so the no-verdict degrade routes to the classifier's ordinary no-verdict arm for every author. The resulting row class for a trusted human author was read from the code, not separately exercised in this pass.
- The lint carries NOTICE lines for this brief: `risk-files-crossread` (all four risk answers "no" while the declared path sits under a security-path trigger), `gotest-run-vacuous` on rows 2, 3, 5 and 6 (unanchored `-run` selectors with no `--- PASS:` assertion; the by-hand runs above confirm each named test exists and ran), and a one-sided depends edge to forge-gitlab/02.

**Risk-bearing value enumeration.** Trigger: the diff touches a risk-classed path. Scope: the classifier hunks of the implementing commit, read at their current lines on merged main. The fix adds no numeric constant; the literals are the degrade-direction bindings and the rendered marker, all in tools/desk/cmd/deskboard/board.go: `in.ownFilesChanged = true` @ :2242 and :2250 (new; :2231 is the #1068 arm); `complete = false` @ :2280; `var rs reviewState` zero value (ever=false) @ :2147; the separator `" — DEGRADED: "` @ :2316. Verify timeouts (120s / 300s) and the row-4 floor `5` are check-definition knobs. Ranking: all are undone by an edit and a redeploy — the board is a read-only classifier and performs no irreversible act. The three direction bindings rank first because a wrong value fails open; the separator and the knobs rank last.

- RISK-VALUE: DERIVED — in.ownFilesChanged = true @ tools/desk/cmd/deskboard/board.go:2242,2250 — the benign-merge outcome asserts the change's own files did not change since review; after a failed own-files or compare read that fact is not established, so `true` (re-review) is the only value that asserts nothing unobserved. It matches the existing truncation contract and spec §3 (could-not-check, never fail open).
- RISK-VALUE: DERIVED — complete = false @ tools/desk/cmd/deskboard/board.go:2280 — the risk gate only widens; a diff that could not be read may contain the trigger, and the incomplete arm sets risk-classed, the same fail-closed arm a truncated diff takes.
- RISK-VALUE: DERIVED — rs = reviewState{} zero value (ever=false) @ tools/desk/cmd/deskboard/board.go:2147 — any other value would invent a verdict nobody gave; ever=false routes through the existing no-verdict arm to NEEDS-REVIEW, which is not a cleared state.

VERIFY: BLOCKED — all eight Verify expectations were observed by hand (row 5's fail-first half on a scratch copy; row 7 exits 0 with nothing to corroborate), but the execution witness is 7 of 8: row 1 (check:ci) is could-not-run on this darwin host for want of a network-off sandbox, and #1800 is still open. Environment blocker; clears on a Linux runner.

### Non-implementer verifier re-run — VERIFY: BLOCKED (#1281) — 2026-10-04 claude-opus-5-5-verifier

Merged main fe12ee0c90b9e39e78afc291e5b17111844356cb. Runner assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), non-implementer. Frontmatter: `gate: model`; `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`.

**Why this re-run.** The last receipt (BLOCKED at cf31c3241, blocker medici-finance/assay#1800) went stale because three declared inputs changed on main: the classifier and two test files, through medici-finance/assay#2117, medici-finance/assay#2118 and medici-finance/assay#2159. Expectations were derived from the brief text first. Each change was then read against the per-row degrade. None of them weakens what a row proves:
- Classifier: #2118 rewrote the compare helper as a first-parent walk and added three new could-not-check returns. All three reach the existing per-row compare degrade in the classifier, which lands on RE-REVIEW. The empty-base/head guard and its refusal literal are unchanged. #2117 removed the approved-draft precondition around risk classification, so the changed-files read and its fail-closed degrade now run on every row. That widens the degrade's reach and does not narrow it. #2159 holds a risk-classed ready row off MERGE-NOW until a security pass lands at head.
- Sweep test: the only hunk adds a `Security-Review: pass` to the healthy control row (PR 50), so that row still reaches MERGE-NOW under #2159. The assertions on the two degraded rows are unchanged: the empty-sha row must read RE-REVIEW and the unreadable-reviews row must never read as cleared and must carry DEGRADED.
- Shared fake forge (deskboard_test.go): the compare fixture now returns a one-commit interval, and GetCommit serves that commit's files, which the new walk needs. The "compare read fails" subtest still forces the compare error, and its degrade line appears in the row 3 output.
- To check that the current test still detects the defect after these changes, the reviews-read degrade was put back to a whole-sweep return on the current classifier (see row 5b). The test fails.

**Execution witness — Linux, network-off.** The darwin host has no `unshare --net` (#1800), so the witness ran in the locally cached `golang:1.26-bookworm` image (go1.26.8 linux/arm64; no pull, `--pull never`). Container settings:
- `--network none`, confirmed inside by a failed DNS lookup. `GOPROXY=off`, `GOTOOLCHAIN=local`, and the host module cache mounted read-only.
- `--security-opt seccomp=unconfined`. **Disclosed:** this was allowed by the desk only for this throwaway network-off container, so statusgen's own `unshare --net --map-root-user` check:ci sandbox could start.
- The only host file mounted besides the module cache was the cell roster.env, read-only. No token, PEM or credential was mounted, and no credential was minted.
- The checkout was a local clone under the verifier worktree, detached at fe12ee0c9, with `refs/remotes/origin/main` pinned to fe12ee0c9. The origin URL was the canonical slug (never fetched) and the push URL was disabled.
- statusgen was built inside the container from that clone's own `statusgen/` source, the same way CI builds it (`--version` prints `dev`).
- `statusgen verifyrun --brief … --timeout 20m` exited 0. The table below is copied verbatim from that run. The witness was written only into the throwaway clone.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | pass exit=0 | sha256:6ff7c335eedc | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | pass exit=0 | sha256:40388acdf363 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | pass exit=0 | sha256:d36c9d55ad13 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | pass exit=0 | sha256:10159baf262b | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` | pass exit=0 | sha256:23d697845fc9 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | pass exit=0 | sha256:15e2dd1aad1e | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --consumers` | pass exit=0 | sha256:b5b14ee4da92 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint` | pass exit=0 | sha256:f9f9c9a71d46 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |

`statusgen verifyrun --check` (in the container, on that witness) exited 0 and printed: `docs/streams/forge-gitlab/brief-13-board-reads-degrade-per-row.md: 8 pass, 0 fail, 0 could-not-run/missing (of 8 Verify rows)`.

For comparison, the darwin host witness (pinned statusgen v1.0.31, written to the verifier worktree's brief and then restored) recorded row 1 as could-not-run for want of `unshare --net`. Its `--check` printed: `7 pass, 0 fail, 1 could-not-run/missing (of 8 Verify rows)`. Only the Linux table above is carried here.

**Per-row results by hand** (darwin/arm64, go1.27.1, verifier worktree at fe12ee0c9, `KUBECONFIG=/dev/null`, no credential, fixtures on loopback only):

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | exit 0 | exit 0, `ok  github.com/medici-finance/assay/tools/desk/cmd/deskboard 269.387s`. The run used most of its 300s budget under host load. The Linux network-off witness row also passed. | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | exit 0; `PASS`; one empty-reviewed-sha change degrades and every other row renders | exit 0, `--- PASS: TestSweepDegradesOneRowNotTheBoard (0.01s)`. Stderr shows PR #51 degraded (`could not establish the reviewed sha … degrades to RE-REVIEW rather than MERGE-CURR`) and PR #52 degraded (`could not read reviews … degrading to no verdict established rather than failing the sweep`). All 3 rows rendered. | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | exit 0; `PASS`; a degraded row is never the benign-merge outcome | exit 0, `--- PASS: TestDegradedRowNeverReadsBenign (0.24s)`, with subtests `own-files_read_fails` and `compare_read_fails` both PASS. Stderr shows `could not compare the reviewed sha to head (… simulated: cannot compare refs) — degrading to RE-REVIEW rather than MERGE-CURR`. | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | `5` or more | exit 0, output `7`: one repo-level label at the open-changes list read, five change-level labels in the classifier, and one unrelated worker-pool comment ("repo-level × PR-level"). | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` against the pre-fix tree | exit non-zero; the failure names the whole-sweep refusal | Fail-first by hand, on scratch copies only (never the tracked tree). (a) Tree of the implementing commit 12087009339a with the classifier file put back to its parent a7393d1dec0b: exit 1, `--- FAIL: TestSweepDegradesOneRowNotTheBoard`, `sweepActionsRepo returned cannot read reviews for example-org/tracker#52: simulated: cannot read reviews — one PR … must degrade its OWN row, never fail the whole sweep`. (b) The fe12ee0c9 tree with the reviews-read degrade replaced by a whole-sweep `return prOutcome{}, err`: exit 1, the same failure line. On the unmodified tree the code span exits 0, so the witness `pass` for this row records the fixed tree and does not show the fail-first half. | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | exit 0; `PASS`; the rendered row carries the could-not-check reason | exit 0, `--- PASS: TestDegradedRowCarriesItsReason (0.04s)`. The test asserts that the row Note contains `DEGRADED:` and `reviewed sha`. | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `statusgen --root . --consumers` | exit 0 | exit 0, `consumers: no brief files in the diff against fe12ee0c90b9… — nothing to corroborate`. Exit 0, but on merged main there is nothing for it to corroborate. | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `statusgen --root . --lint` | exit 0; `LINT: PASS` | exit 0, `LINT: PASS`, 0 lines beginning PROBLEM | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

**Findings (not Verify failures).**
- The trust-gate blessing read for an untrusted author is still a whole-sweep `return prOutcome{}, berr` with no repo-level / change-level label (board.go:2180). The brief's Review question about it is still open.
- The header comment of `TestRiskClassificationFailsClosedOnReadFailure` still says risk classification "only runs for a still-draft PR". Since #2117 that is no longer true. The test still passes and still forces the read failure, so this is a stale comment only.
- Lint NOTICE lines for this brief are unchanged: `risk-files-crossread`, `gotest-run-vacuous` on rows 2, 3, 5 and 6, and the one-sided depends edge to forge-gitlab/02. The by-hand runs above confirm that each named test exists and ran.

**Risk-bearing value enumeration.** `risk-files-crossread` fires the trigger: the declared path sits under the `tools/desk/cmd/deskboard/` security-path trigger. Scope: the implementing commit's classifier hunks, read at their current lines on fe12ee0c9, plus the post-receipt classifier hunks. The fix adds no numeric constant. The literals in tools/desk/cmd/deskboard/board.go are the degrade-direction bindings and the rendered marker:
- `in.ownFilesChanged = true` @ :2299 and :2307 (:2288 is the #1068 arm)
- `complete = false` @ :2336
- `var rs reviewState` zero value (ever=false) @ :2204
- the separator `" — DEGRADED: "` @ :2375

The post-receipt hunks add only message strings and a loop bound (`len(chain) >= len(byID)`, a walk-termination guard). The Verify timeouts (120s/300s) and the row-4 floor `5` are check-definition knobs.

Ranking: every entry can be undone by an edit and a redeploy, because the board is a read-only classifier with no irreversible act. The three direction bindings rank first because a wrong value fails open. The separator, the loop bound and the knobs rank last.
- RISK-VALUE: DERIVED — in.ownFilesChanged = true @ tools/desk/cmd/deskboard/board.go:2299,2307 — the benign-merge outcome asserts that the change's own files did not change since review. After a failed own-files or compare read (including #2118's three new incomplete-history returns) that fact is not established, so `true` (RE-REVIEW) is the only value that asserts nothing unobserved. This matches the truncation contract and spec §3 (could-not-check, never fail open).
- RISK-VALUE: DERIVED — complete = false @ tools/desk/cmd/deskboard/board.go:2336 — the risk gate is a union that only widens. A diff that could not be read may contain the trigger, so the read failure takes the same fail-closed arm as a truncated diff. Since #2117 this arm runs on every row.
- RISK-VALUE: DERIVED — rs = reviewState{} zero value (ever=false) @ tools/desk/cmd/deskboard/board.go:2204 — any other value would invent a verdict nobody gave. ever=false routes through the existing no-verdict arm (NEEDS-REVIEW), which is not a cleared state.

VERIFY: BLOCKED (#1281) — seven of eight Verify expectations were observed on substance. The Linux network-off execution witness records 8 of 8 and `--check` reports `8 pass, 0 fail, 0 could-not-run/missing`, but row 7's `pass exit=0` is vacuous: `statusgen --root . --consumers` printed "no brief files in the diff … nothing to corroborate", so 7 rows are counted as passing. Row 5's fail-first half was proven by hand on both the implementing commit and the current classifier. A one-row re-witness of row 7 at the implementing PR's own head (a63abe07bde7, medici-finance/assay#1084, statusgen v1.0.31, clean clone, origin/main at 4f6bb6d59a75) is vacuous too: the base is the merge-base d6d24c30db00, and that PR's diff touches only the changelog fragment and four files under `tools/desk/cmd/deskboard/`. The brief was authored earlier by medici-finance/assay#1075, so the verbatim row never sees this brief in any diff. Row 7 as written cannot corroborate this brief on any tree; it needs a fixed base or a different row shape, which is the medici-finance/assay#1281 class. The #1800 darwin gap is bypassed for this brief by the Linux container witness, and #1800 is no longer this brief's blocker. Status stays `implemented`.
### Non-implementer verifier re-run — VERIFY: BLOCKED (#1281) — 2026-10-07

Merged main dde4fbeaead9c173e8355869e06d86e8e9eee446. Runner assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), non-implementer. Host darwin/arm64, go1.27.1, pinned statusgen v1.0.32 called directly. KUBECONFIG=/dev/null; go runs used a throwaway HOME and a scratch TMPDIR; no credential minted, no live forge called; deskboard fixtures serve on loopback only.

**What changed since the 2026-10-04 record (fe12ee0c9).** Classifier file: only #2301 (adds a boardSweepLimit pool-width variable for a serial-reference test; no classifier-path hunk). Test files: #2301 (pooledverify_test.go, mutations.json) and #2308 (git maintenance vs TempDir). statusgen: #2184, #2203, #2206, #2238, #2281, #2239, #2304, #2312 — none touches the --consumers diff base. Brief: only the prior verify record (#2193). Stream README: unchanged. medici-finance/assay#1281 and medici-finance/assay#1800 both read OPEN on 2026-10-07.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | pass exit=0 | by hand: `ok  github.com/medici-finance/assay/tools/desk/cmd/deskboard 91.592s`; the darwin execution witness has no network-off sandbox for check:ci (assay#1800), so its witness row for this command is held | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | pass exit=0 | `--- PASS: TestSweepDegradesOneRowNotTheBoard (0.01s)`; stderr shows PR 51 degraded on the unestablished reviewed sha (RE-REVIEW rather than MERGE-CURR) and PR 52 degraded on the reviews read (no verdict established rather than failing the sweep) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | pass exit=0 | `--- PASS: TestDegradedRowNeverReadsBenign (0.03s)`, subtests own-files_read_fails and compare_read_fails both PASS | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | pass exit=0 | output 7 (floor 5): one repo-level label at the review-queue read (:2115), five change-level labels (:2206, :2246, :2301, :2310, :2336), one unrelated worker-pool comment (:2109) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` | pass exit=1 on the mutated classifier | fail-first on a scratch clone at dde4fbea with the reviews-read degrade replaced by a whole-sweep `return prOutcome{}, err`: exit 1, `--- FAIL: TestSweepDegradesOneRowNotTheBoard`, `sweepActionsRepo returned cannot read reviews for example-org/tracker#52: simulated: cannot read reviews — one PR whose verdict the forge could not pin to a head must degrade its OWN row, never fail the whole sweep`; same code span on the unmodified merged-main tree exits 0 | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | pass exit=0 | `--- PASS: TestDegradedRowCarriesItsReason (0.00s)` | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --consumers` | exit=0, vacuous | `consumers: no brief files in the diff against dde4fbeaead9c173e8355869e06d86e8e9eee446 — nothing to corroborate` — exits 0 but corroborates nothing on merged main (assay#1281 class, still open) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint` | pass exit=0 | `LINT: PASS`, 0 lines beginning PROBLEM; NOTICE lines for this brief unchanged (risk-files-crossread; gotest-run-vacuous on rows 2, 3, 5, 6; one-sided depends edge to forge-gitlab/02) | 2026-10-07 | assay-verifier-app[bot] @ dde4fbeaead9 (on-behalf-of human:ian) (forge-identity) |

**Execution witness** (non-dry `statusgen verifyrun --brief … --timeout 20m`, v1.0.32, darwin; written to the verifier worktree's brief, copied out, then the tree restored): exit 2. Row 1 could-not-run (check:ci needs `unshare --net`, assay#1800); rows 2–8 pass. `verifyrun --check`: 7 pass, 0 fail, 1 could-not-run/missing (of 8 Verify rows). No Linux container witness was taken this pass.

**Findings (not Verify failures).**
- The trust-gate blessing read for an untrusted author is still a whole-sweep `return prOutcome{}, berr` with no repo-level / change-level label (board.go:2187). The brief's Review question about it is still open.

**Risk-bearing value enumeration.** Trigger: risk-files-crossread (the declared classifier path sits under a security-path trigger). Scope: the implementing commit's classifier hunks at their current lines on dde4fbea, plus the post-receipt classifier delta (#2301: one variable `boardSweepLimit = sweepConcurrency`, an alias of an existing pool-width knob, no new literal). Literals in tools/desk/cmd/deskboard/board.go: `in.ownFilesChanged = true` @ :2306 and :2314 (:2295 is the #1068 arm); `complete = false` @ :2343; `var rs reviewState` zero value (ever=false) @ :2211; separator `" — DEGRADED: "` @ :2382. Verify timeouts (120s/300s) and the row-4 floor 5 are check-definition knobs. Ranking: all are undone by an edit and a redeploy (read-only classifier, no irreversible act); the three direction bindings rank first because a wrong value fails open; the separator, pool-width alias and knobs rank last.
- RISK-VALUE: DERIVED — in.ownFilesChanged = true @ tools/desk/cmd/deskboard/board.go:2306,2314 — the benign-merge outcome asserts the change's own files did not change since review; after a failed own-files or compare read that is not established, so true (RE-REVIEW) is the only value that asserts nothing unobserved; matches the truncation contract and spec §3 (could-not-check, never fail open).
- RISK-VALUE: DERIVED — complete = false @ tools/desk/cmd/deskboard/board.go:2343 — the risk gate is a union that only widens; an unread diff may contain the trigger, so the read failure takes the same fail-closed arm as a truncated diff.
- RISK-VALUE: DERIVED — rs = reviewState{} zero value (ever=false) @ tools/desk/cmd/deskboard/board.go:2211 — any other value would invent a verdict nobody gave; ever=false routes to the no-verdict arm (NEEDS-REVIEW), not a cleared state.

VERIFY: BLOCKED (#1281) — seven of eight Verify expectations observed on substance (row 1 by hand, row 5 fail-first on a scratch clone). Row 7 still exits 0 with "nothing to corroborate" on merged main: the medici-finance/assay#1281 check-definition blocker reproduces unchanged at dde4fbea and #1281 is OPEN. The darwin witness additionally records row 1 could-not-run (#1800, OPEN); the 2026-10-04 Linux network-off witness had cleared that for this brief and the classifier path has not changed since. Status stays `implemented`.

Attached to #1281: https://github.com/medici-finance/assay/issues/1281#issuecomment-6038171622

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers: for each site the audit labelled repo-level, is a whole-sweep refusal really
the right answer — or is it a change-level read wearing a repo-level name?
