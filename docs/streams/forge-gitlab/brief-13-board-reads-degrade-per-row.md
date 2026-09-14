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

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers: for each site the audit labelled repo-level, is a whole-sweep refusal really
the right answer — or is it a change-level read wearing a repo-level name?
