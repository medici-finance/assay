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
### Verification — 2026-09-30 (assay-verifier-app[bot] @ e03f4f5c7c41 (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main e03f4f5c7c412560a666d95383bee0444fb6d263, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | fail exit=1 | sha256:07e845bfa0d3 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | pass exit=0 | sha256:aa8198491619 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | pass exit=0 | sha256:b38f5b23f9ef | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | pass exit=0 | sha256:10159baf262b | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` | pass exit=0 | sha256:b9d4f611756a | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | pass exit=0 | sha256:13f69fd764b0 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 7 | `statusgen --root . --consumers` | pass exit=0 | sha256:6fb78bebd7af | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 8 | `statusgen --root . --lint` | pass exit=0 | sha256:fe7ade12c928 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskboard/... -timeout 300s` | exit 0 | darwin: exit 0, ok .../cmd/deskboard 31.361s. Linux container, network none with loopback up: exit 0, ok .../cmd/deskboard 7.895s (clean test cache). Linux under the check:ci wrapper (unshare --net --map-root-user, loopback down): exit 1, four fake-GitLab tests fail with dial tcp 127.0.0.1 connect: network is unreachable. Substance passes; the check:ci form fails | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -v -timeout 120s` | exit 0; PASS | exit 0; --- PASS: TestSweepDegradesOneRowNotTheBoard. Three-PR sweep: #50 MERGE-NOW, #51 (empty reviewed sha) RE-REVIEW, #52 (reviews read fails) degraded with DEGRADED note; stderr names both degrades | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowNeverReadsBenign -v -timeout 120s` | exit 0; PASS | exit 0; --- PASS: TestDegradedRowNeverReadsBenign, subtests own-files read fails and compare read fails both PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 4 | `cd tools/desk && grep -cE -e 'change-level' -e 'repo-level' cmd/deskboard/board.go` | 5 or more | exit 0; 7. Six are audit labels (one repo-level at the open-changes read, five change-level in the classifier); one is an unrelated worker-pool comment | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/ -run TestSweepDegradesOneRowNotTheBoard -count=1 -timeout 120s` against the pre-fix tree | exit non-zero; names the whole-sweep refusal | As written (code span on main): exit 0, which does not meet the Expect. Hand fail-first: fix commit tree with the classifier file restored to its parent: exit 1, sweepActionsRepo returned cannot read reviews for example-org/tracker#52 ... must degrade its OWN row, never fail the whole sweep. Control (same tree, fixed classifier): exit 0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/ -run TestDegradedRowCarriesItsReason -v -timeout 120s` | exit 0; PASS | exit 0; --- PASS: TestDegradedRowCarriesItsReason (Note carries DEGRADED: and names the reviewed sha) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 7 | `statusgen --root . --consumers` | exit 0 | exit 0; consumers: no brief files in the diff against e03f4f5c... nothing to corroborate (vacuous on merged main) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 8 | `statusgen --root . --lint` | exit 0; LINT: PASS | exit 0; LINT: PASS; 0 PROBLEM lines. NOTICEs naming this brief: risk-files-crossread, and gotest-run-vacuous on rows 2, 3, 5, 6 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — in.ownFilesChanged = true @ tools/desk/cmd/deskboard/board.go:2270,2278 (the #1068 arm at :2259) — MERGE-CURR asserts that the change's own files are unchanged since review; after a failed own-files or compare read that fact is not established, so true (RE-REVIEW) is the only value that asserts nothing unobserved. It matches the existing truncation contract and spec §3 (could-not-check, never fail open).
RISK-VALUE: DERIVED — complete = false @ tools/desk/cmd/deskboard/board.go:2308 — the risk gate is a union that only widens; a diff that could not be read may hold the trigger, and the !complete case sets riskClassed, the same fail-closed arm a truncated diff takes, so a read error cannot reach MERGE-NOW or FLIP (pinned by TestRiskClassificationFailsClosedOnReadFailure).
RISK-VALUE: DERIVED — var rs reviewState (zero value, ever=false) @ tools/desk/cmd/deskboard/board.go:2175 — any other value would invent a verdict nobody gave; ever=false routes through the existing no-verdict arm (NEEDS-REVIEW, or HUMAN-OWNED for an accountable human author), neither of which is a cleared state. See the HUMAN-OWNED caveat in Notes.

Notes:
- BLOCKED (check-definition), not a product failure. Row 1 fails only in the check:ci sandbox (loopback down; four fake-GitLab tests from another brief) and exits 0 by hand; row 5's command exits 0 on merged main where its Expect wants non-zero, and passes by hand at the fix commit with the classifier restored to its parent (exit 1) versus fixed (exit 0). `statusgen brief --check-verified` with a hypothetical flip exits 1 on row 1. Advancing needs row 5 re-authored to a pinned base and row 1 reclassed or the sandbox loopback raised, then a re-verify.
- Grounding written before reading the diff or tests (at <scratch>/expectation.txt): labelled sites, safe-side per-row degrade with a rendered reason, three named tests, fail-first, no GitLab backend change, README row and changelog fragment. The code at main matches it. The implementing commit touches no forge_gitlab.go hunk; it adds a changelog fragment (changelog/feat-assay-forge-gitlab-13.md rather than the planned name). README row 13 still reads implemented.
- Row 1 is a check-definition failure. The row is classed check:ci, so the witness runs it under unshare --net --map-root-user, where loopback is down. Four tests in gitlabrequestchanges_test.go (TestBoardGitLab..._1124 and TestBoardReadsGitLabSecurityFailAsChangesRequested_1124, which serve a fake GitLab over an httptest loopback listener; added by #1133 before the fix) then fail with connect: network is unreachable. The same command passes by hand on darwin and in a network-none container with loopback up. None of the four tests belongs to this brief. The fix belongs either in the row's class or in the witness facility (bring loopback up inside the namespace). Both are outside this item's code.
- Row 5 is a check-definition failure. Its code span, run as written on main, exits 0, and the Expect wants non-zero. "Against the pre-fix tree" is prose the witness cannot express. The witness records row 5 as pass exit=0 (exit-status only), which inverts the Expect. Fail-first was proven by hand: the fix commit's tree with the classifier restored to its parent exits 1 with the whole-sweep refusal. The fix does not reverse-apply cleanly at main because three later commits touched the classifier, so the hand procedure used the fix commit's own tree.
- Row 7 exits 0 but is vacuous on merged main (no brief files in the diff).
- Row 2: the fixture has three PRs, and two of them degrade. #51 is the empty-reviewed-sha case, which the #1068 guard handles. #52 is the reviews-read failure, the non-benign-merge site. Exactly one row carries an empty reviewed sha, as the Expect states.
- Open review observation, carried from the 2026-09-28 pass and still true at e03f4f5c: the trust-gate blessing read for an untrusted author (prBlessed error, return prOutcome{}, berr @ board.go:2150) is a per-change whole-sweep return with no repo-level or change-level label. The brief's Review question covers it.
- Open review observation: when the reviews read fails for an accountable human author, the row lands HUMAN-OWNED, not NEEDS-REVIEW. A standing CHANGES_REQUESTED that could not be read would then show outside the UNREVIEWED alarm. The DEGRADED note is the only layer that shows it.

VERIFY: BLOCKED

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers: for each site the audit labelled repo-level, is a whole-sweep refusal really
the right answer — or is it a change-level read wearing a repo-level name?
