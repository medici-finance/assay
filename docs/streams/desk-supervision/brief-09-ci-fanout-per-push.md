---
brief: desk-supervision/09
title: Per-push CI fan-out — trigger selection so a docs-only push stops paying for a Go build
why: >-
  The desk caps how many workers run at once, but nothing caps what each of their pushes
  costs downstream. Every push to a pull-request branch fans out into about a dozen jobs on
  a runner pool two runners wide, and half of them cannot change their verdict for the
  change that triggered them — a board-regeneration commit that touches one markdown file
  still pays for three Go builds. Measured over one 20-minute window: 100 runs created
  against 67 completed, 36 still queued at the sample, and 46 cancelled as already
  superseded. Selecting triggers more precisely removes about a fifth of the pool's job
  count without removing a single check from a single pull request.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-05 by desk-supervision authoring session
exec-tier: strong
exec-tier-why: >-
  (b) — correctness is a cross-artifact argument: which of eleven workflows fires for which
  diff shape, read against the branch rulesets' required-check set and against the board
  tooling that reads the resulting check rollup.
sources:
  - "Driver ruling, 2026-09-05 — raise the supply side (pool width) and author the demand side (this brief) as a pair; only the demand half is in this repo."
  - "`.github/workflows/` (all sixteen files) — read at authoring for triggers, `paths:`/`paths-ignore:`, `concurrency`, and `runs-on` labels."
  - "`gh api repos/medici-finance/assay/rulesets` — the two active branch rulesets on the default branch. `leak-sweep` is the ONLY required status check; no workflow in this repo is a required check."
  - "`gh run list --limit 120` sampled 2026-09-05 11:14–11:34Z — 120 runs, the per-workflow/per-event counts in `facts:` below, and 46/120 cancelled."
  - "`tools/desk/cmd/deskboard/zeroci.go` `wouldFire` — the board's own model of GitHub's `paths` / `paths-ignore` semantics, and the consumer that would misread a filtered workflow if it did not model them."
  - "`tools/desk/cmd/deskflip/flip.go` ~line 228 (`ciEmpty` on a CI-required repo) — an absent rollup is `Unverifiable`, never green; the reason this brief must prove the rollup stays non-empty."
  - "freshness-checked 2026-09-05 @ 256dce8"
consumers:
  - "tools/ci-load/activation/*.yml: fixed-here (the five post-change workflow files are the deliverable; a human copies them into .github/workflows/)"
  - "tools/desk/cmd/deskboard/zeroci.go (reads every pull_request workflow's filters to decide `no-checks` vs `CI-NEVER-RAN`): out-of-scope (no change needed — `wouldFire` already models `paths-ignore` as \"skipped only when EVERY changed file is ignored\", which is exactly the semantics this brief relies on; Verify row 6 proves it on the new filter set rather than assuming it)"
  - "tools/desk/cmd/deskflip/flip.go condition `checks-green`: out-of-scope (no change needed — an EMPTY rollup is `Unverifiable` on a CI-required repo, and the smallest post-change fan-out is four check runs, never zero; Verify row 7 proves it)"
  - "the `leak-sweep` required status check: out-of-scope (posted by a gate that is not one of these workflows; unfiltered and unreachable from this diff, before and after)"
---

# Brief 09 — Per-push CI fan-out

## Context

files:
- `tools/ci-load/activation/ci.yml`, `plugin-drift.yml`, `assay-statusgen.yml`,
  `assay-qualgen.yml`, `evidence-automerge.yml` — the five post-change workflow files.
- `tools/ci-load/activation/ci-load.diff` — the unified diff of those five against the
  versions in `.github/workflows/`, as a reading aid.
- `tools/ci-load/activation/README.md` — copy command, verification, rollback, and the rule
  the edits were held to.
- `tools/ci-load/pathsemantics.py` — the offline negative control for Verify rows 3 / 3b: it
  reads the `paths-ignore` list out of the staged workflows and asserts GitHub's rule on
  five diff shapes, three of which must NOT skip.
- `docs/streams/desk-supervision/README.md` — the Briefs row and the wave block.
- `changelog/ci-per-push-fanout.md` — the fragment.

prescribes (applied by a human copy, NOT written by this brief's diff):
`.github/workflows/ci.yml`, `plugin-drift.yml`, `assay-statusgen.yml`, `assay-qualgen.yml`,
`evidence-automerge.yml`. The implementing identity holds no `workflows` permission and
cannot write that directory at all, which is why the deliverable is a staged copy set —
the same shape `tools/pairedversions/activation/` already uses. The risk×files cross-read
is not being dodged by that split: the four risk answers are stated for the PRESCRIBED
change, not just for the staged files, and the reasoning a reviewer should check is in
`single-point-of-failure` below.

single-point-of-failure: none of these edits stands between a fault and any damage — they
select WHICH workflows are asked a question, and change nothing about how any workflow
answers one. The control that stands between withheld content and a public merge is the
`leak-sweep` required status check, and it is not one of these workflows: it is posted by a
separate gate, it is the only required status check either branch ruleset names, and it
runs on every pull request unconditionally before and after this change. Behind it sit two
in-repo leak legs (`leaksweep-control.yml`, `leaksweep-pattern.yml`) which keep
`paths: "**"` on both legs, gain no filter of any kind, keep their runner, keep their
existing `concurrency` groups byte-for-byte, and are not in the staged set at all — a
docs-only diff is precisely the change a narrower trigger would let through in silence. The
three layers fail for different reasons in different components: a private control-based
token sweep, an in-tree pattern matcher, and an in-tree Go test of the disclosure controls.

facts (all measured 2026-09-05, sample window 11:14–11:34Z, `gh run list --limit 120` = 120
runs; re-establish any of them with the command named):

- **Required checks.** `gh api repos/medici-finance/assay/rulesets` returns two active
  branch rulesets on the default branch. `leak-sweep` is the only `required_status_checks`
  context in either. No workflow in `.github/workflows/` is a required check; the other
  ruleset rules are `deletion`, `non_fast_forward` and `pull_request` (one approving review,
  last-push approval, extra approval for unattributed changes).
- **`ci.yml` fires twice per pull-request push.** `on: push:` carries no `branches:`, so it
  fires on every branch and every tag as well as `main`. In the window: 5 `ci`/`push` runs on
  non-`main` branches and 5 `ci`/`pull_request` runs — a 1:1 duplicate. `ci.yml` has three
  jobs (`build-test`, `plugin-shell-suites`, `skillslint`), so that duplicate is 3
  self-hosted jobs per push.
- **`ci.yml` has no `concurrency` block**, so its superseded runs are never cancelled. Only
  two workflows in the repo lack one (`for f in .github/workflows/*.yml; do grep -q
  '^concurrency:' "$f" || echo "$f"; done`): `ci.yml` and `assay-drainloop.yml`. Drainloop's
  trigger is `paths:`-scoped to `drainloop/**` and it produced **0** runs in the window, so
  only `ci.yml` is staged; the drainloop gap is recorded here and left alone rather than
  changed on no measured load. Meanwhile 46 of the 120 runs in the window were `cancelled`
  — the groups that DO exist are doing real work.
- **Every job but one runs on `medici-builder-public`.** The single exception is
  `inbound-triage.yml`'s job, which requests `ubuntu-latest` — and it was `skipped` at its
  own `if:` gate in 60 of its last 60 runs, so its runner was never allocated. **There is no
  observed evidence that a GitHub-hosted job has ever executed in this repository.** Hosted
  availability here is could-not-check, not "available".
- **The documentary share of `main` pushes.** Of the last 50 commits on `main`, 19 changed
  nothing outside `docs/`, `changelog/`, `CHANGELOG.md` and `STATUS.md`. In the sample
  window itself it was 2 of 10 — the window was code-heavy, which is why the aggregate below
  is quoted against the window's own mix and not against the 50-commit share.
- **`plugin-drift.yml` argues its own case for no filter, and that argument is about the
  `push: main` leg only.** Its header: the pins "can rot with no file in this repo changing
  at all", which is why that leg must look again with no diff. The `pull_request` leg's own
  stated purpose is narrower — "catches the bump BEFORE it lands" — and a pull request that
  changes nothing outside the four documentary paths cannot be a bump.
- **`evidence-automerge.yml` starts a job on every pull request and every review.** 11 runs
  in the window (5 `pull_request`, 6 `pull_request_review`), none on an Evidence pull
  request. Its `enable` job carries no `if:`, so the whole outcome for every non-verifier
  pull request is the guard step printing "not an Evidence PR" and exiting 0.
- **Two writers must stay uncancellable.** `assay-statusgen.yml`'s `regen` job is STATUS.md's
  single writer and `assay-qualgen.yml`'s `regen` job is QUALITY.md's; both currently carry
  `cancel-in-progress: false`, and both files say why (a cancelled regen throws the push's
  board or view away rather than delaying it). Both also carry a `pull_request`-only job
  (`lint` / `render`) that writes nothing.
- **GitHub filter semantics, as the board tool models them** (`zeroci.go` `wouldFire`):
  `paths` runs the workflow iff at least one changed file is included; `paths-ignore` skips
  it only when EVERY changed file is ignored. A mixed diff therefore always runs in full —
  a filter cannot half-skip.

### The three pull-request classes, measured

Self-hosted (`medici-builder-public`) **jobs** started by one push to the pull-request
branch. `inbound-triage` is excluded throughout: it is the one `ubuntu-latest` job and costs
the pool nothing.

| Class | Changed paths | Today | After | Δ |
|---|---|---|---|---|
| **A** docs-only Evidence / board | `docs/streams/**` (+ `changelog/<slug>.md`) | 12 | 5 | −7 (−58%) |
| **B** Go code | `tools/desk/**` or `statusgen/**` | 12 | 8 | −4 (−33%) |
| **C** plugin / skill | `plugins/**` | 11 | 7 | −4 (−36%) |
| — documentary push to `main` | `STATUS.md` only | 7 | 4 | −3 (−43%) |

Class A, itemised, because it is the class the Evidence lane lives in:

| Workflow / event | Today | After | Why |
|---|---|---|---|
| `ci` / `pull_request` | 3 | 0 | `paths-ignore` — no job in it reads `docs/`, `changelog/`, `CHANGELOG.md` or `STATUS.md` |
| `ci` / `push` | 3 | 0 | `push` scoped to `branches: [main]`; it was a 1:1 duplicate of the line above |
| `changelog-check` / `pull_request` | 1 | 1 | unchanged — the fragment rule is exactly what a docs-only PR must satisfy |
| `leaksweep-control` / `pull_request` | 1 | 1 | unchanged, deliberately — see `single-point-of-failure` |
| `leaksweep-pattern` / `pull_request` | 1 | 1 | unchanged, deliberately — including its existing `concurrency` group, which is not retuned |
| `statusgen-board` / `pull_request` (`lint`) | 1 | 1 | unchanged — this is the load-bearing check for class A |
| `plugin-drift` / `pull_request` | 1 | 0 | `paths-ignore` on the PR leg only; the `push: main` rot-detection leg keeps no filter |
| `evidence-automerge` / `pull_request` | 1 | 1 | unchanged for a verifier-authored Evidence PR (0 for any other author, via the new `if:`) |
| `forge-surface-control` | 0 | 0 | already filtered to `tools/desk/**` + `docs/streams/**/inventory.md` |
| **total** | **12** | **5** | |

**Expected effect, with its derivation.** Applying the five edits to the sample window's own
120 runs removes:

```
  5 ci/push runs on non-main branches   × 3 jobs = 15   (push scoped to branches: [main])
  2 ci/push runs on documentary main    × 3 jobs =  6   (paths-ignore)
  0 ci/pull_request runs                × 3 jobs =  0   (all 5 in-window PRs were class B/C)
  0 plugin-drift/pull_request runs      × 1 job  =  0   (same reason)
 11 evidence-automerge runs             × 1 job  = 11   (job-level if:, none was an Evidence PR)
                                                 ─────
                                                    32
```

against a window total of ≈145 self-hosted jobs (`ci` 15 runs × 3, plus 10 `changelog-check`,
15 `plugin-drift`, 15 `leaksweep-pattern`, 15 `leaksweep-control`, 10 `verify-gate-open`, 12
`forge-surface-control`, 11 `evidence-automerge`, ≤9 `statusgen-board`, 1 branch-local
workflow — the `statusgen-board` term is the only estimate, since its `main` jobs are
skip-guarded).

**≈32 of ≈145 = ≈22%**, in a window that contained **zero** class-A pull requests. The two
zero rows are not a rounding-down: they are what that particular window happened to hold. On
a window with the 50-commit documentary share (19/50 rather than 2/10) the `ci` paths-ignore
term alone rises from 6 to ≈11.

Not counted, deliberately: the new `ci` concurrency group. Its saving is real — `ci` is the
only PR-triggered workflow whose superseded runs currently run to completion, and 46/120 runs
in the window were cancelled by the groups that do exist — but no `ci` supersession occurred
inside the 20-minute sample, so the magnitude is **could-not-check** and is left out of the
number rather than estimated into it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **No security or leak workflow gains a `paths:` or `paths-ignore:` filter, and no required
  check acquires a condition.** If a reduction can only be had by weakening a control or its
  CI assertion, it is not in scope for this brief: stop and escalate.

## Task

1. Create `tools/ci-load/activation/` holding a post-change copy of each of the five
   workflows below, each carrying its rationale in a header comment, plus `ci-load.diff`
   (the unified diff against `.github/workflows/`) and a `README.md` with the copy command,
   the verification steps, the rollback, and the statement that required checks were not
   filtered. Same shape as `tools/pairedversions/activation/`.

2. `ci.yml` — `on: push:` gains `branches: [main]`; both the `push` and `pull_request` legs
   gain `paths-ignore: ["docs/**", "changelog/**", "CHANGELOG.md", "STATUS.md"]`; add
   `concurrency: {group: ci-${{ github.ref }}, cancel-in-progress: ${{ github.event_name ==
   'pull_request' }}}`. The cancel is conditioned so a `main` run — the record that a merged
   commit builds — is never cancelled by the next merge.

3. `plugin-drift.yml` — the same `paths-ignore` on the **`pull_request` leg only**. Leave
   `push: branches: [main]` unfiltered and extend the file's own `WHY NO paths: FILTER` note
   to say which leg that argument governs, so the next reader does not undo one and read the
   other as licence.

4. `assay-statusgen.yml` and `assay-qualgen.yml` — change `cancel-in-progress: false` to
   `${{ github.event_name == 'pull_request' }}` in each, and rewrite the comment above it to
   state the asymmetry: the `main` half is a single writer and must never be cancelled; the
   pull-request half (`lint` / `render`) writes nothing and its superseded runs are answering
   about a head nobody will read.

5. `evidence-automerge.yml` — add a job-level
   `if: github.event.pull_request.user.login == 'assay-verifier-app[bot]'` to `enable`, with
   a comment stating that it is a pre-filter and not a guard: the in-job guard step is
   unchanged, still writes `eligible=false` first, and still re-reads the author, the draft
   flag and the full paginated file list. Drift in the `if:` is fail-safe in both directions
   — narrowed, the lane goes quiet and Evidence PRs wait for a human merge (the pre-lane
   state); widened, the extra PR is declined by the unchanged in-job check.

6. Touch nothing else in `.github/workflows/`. In particular `leaksweep-control.yml` and
   `leaksweep-pattern.yml` are not copied, not filtered, and not given a concurrency group.

7. Add `changelog/ci-per-push-fanout.md`, and add the row + wave entry for this brief to
   `docs/streams/desk-supervision/README.md`.

### Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| A `paths-ignore` swallows a diff that DOES need the job (a mixed docs+Go pull request silently skips `ci`) | rows 3 + 3b — the semantics reproduction asserts the mixed, Go-only and plugin-only cases all still run, and the mutation proves that assertion can go red |
| A leak/security workflow acquires a filter or a cancel, quietly | row 4 — a byte-diff of both leak workflows across the landing commit, which no coincidence satisfies |
| A writer job on `main` becomes cancellable, so a merge burst throws a board away | row 5 — every `cancel-in-progress` in the tree is `false` or event-conditioned; no `push: main` writer acquires a bare `true` |
| The staged copies drift from what actually lands in `.github/workflows/` | row 8 — byte-diff of the five staged files against the landed ones |
| Filtering leaves a pull request with an EMPTY check rollup, which `deskflip` reads as unverifiable and refuses to flip | row 7 (the flow row) — the rollup on a landed class-A pull request is ≥ 4; and row 6, which proves the board's own `wouldFire` still models the new filters |
| The claimed reduction is asserted rather than measured | row 1 — the workflow/event set actually produced by one class-A push, run against a pre-landing and a post-landing branch |
| A required check stops running on a docs-only pull request | row 2 — `leak-sweep` present in the live status list at a class-A head, dereferenced against the live ruleset |
| A `consumers:` routing claim is asserted and never corroborated | row 11 — `statusgen --consumers` on this brief |
| The `evidence-automerge` `if:` and the in-job `EVIDENCE_AUTHOR` drift apart | **no row.** Review-only, and bounded by design: the two can only disagree into the fail-safe direction (§Task 5), so the cost of drift is a quiet lane, not a widened one. A lint that binds two constants across a job gate and a step env is not worth building for one call site. |

## Verify (executable — no prose-only DoD items)

Rows 3, 3b, 6, 9, 10 and 11 run against the staged files and need no landing. Rows 1, 2, 4,
7 and 8 run AFTER a human has copied the staged files into `.github/workflows/` and that
commit is on `main` — every one of them derives its own inputs, so there is nothing for a
verifier to substitute by hand.

The class-A pull request the post-landing rows measure is derived, not named: the newest
pull request authored by the verify desk's App is an Evidence pull request, which is
docs-only by construction (its own lane refuses anything outside `docs/streams/`). Rows 1,
2 and 7 open with the same lookup, written out in row 1 and abbreviated to `$PR` after.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `PR=$(gh pr list --repo medici-finance/assay --state all --author 'assay-verifier-app[bot]' --limit 1 --json number --jq '.[0].number'); B=$(gh pr view "$PR" --repo medici-finance/assay --json headRefName --jq .headRefName); gh run list --repo medici-finance/assay --branch "$B" --json workflowName,event --jq '[.[]\|"\(.workflowName)/\(.event)"]\|sort\|unique'` | exit 0; the list contains `leaksweep-control/pull_request`, `leaksweep-pattern/pull_request`, `changelog-check/pull_request` and `statusgen-board/pull_request`, and contains NEITHER `ci/push` NOR `ci/pull_request` NOR `plugin-drift/pull_request`. **This is the before/after measurement**: the same command against a pre-landing Evidence branch returns all three of those |
| 2 | check +dereference | `H=$(gh pr view "$PR" --repo medici-finance/assay --json headRefOid --jq .headRefOid); gh api "repos/medici-finance/assay/commits/$H/status" --jq '[.statuses[].context]'; gh api repos/medici-finance/assay/rulesets --jq '[.[].name]'` | exit 0; the first output contains `leak-sweep`, and the second names the ruleset that requires it. **DEREFERENCE, and the fail-first row**: the claim "the required check still runs on a docs-only PR" is resolved against the live ruleset and the live status list rather than restated. Also run `--jq '[.statuses[].context]\|map(select(.=="no-such-check"))\|length'` and record `0`, which shows the assertion discriminates instead of matching anything present |
| 3 | check:ci | `python3 tools/ci-load/pathsemantics.py` | exit 0; last line `PASS`; `mixed: skipped=False`, `go-only: skipped=False`, `plugin-only: skipped=False` — **negative control, inverts**: three of the five cases must NOT skip. The filter list is read out of the staged `ci.yml` / `plugin-drift.yml`, so this fails if the shipped filters and the asserted ones diverge |
| 3b | check:ci +mutation | Change `skipped()` in `tools/ci-load/pathsemantics.py` from `all(...)` to `any(...)` — GitHub's rule read backwards — then re-run row 3 and restore the file | exit **1** on the mutant, with `FAIL mixed: skipped=True (want False)` for both workflows; exit 0 again after restoring — **fail-first**: row 3 is sensitive to the one semantic the whole filter argument rests on |
| 4 | check | `L=$(git log -1 --format=%H -- .github/workflows/ci.yml); git diff --stat "$L^" HEAD -- .github/workflows/leaksweep-control.yml .github/workflows/leaksweep-pattern.yml` | exit 0, **no output** — across the landing commit (derived as the last commit to touch `ci.yml`) the two leak workflows are byte-identical. This is the whole security claim, asserted as a diff rather than as a grep a coincidentally equal count could satisfy |
| 5 | check | `grep -n 'cancel-in-progress' .github/workflows/*.yml` | exit 0; exactly two settings read `${{ github.event_name == 'pull_request' }}` — `assay-statusgen.yml` and `assay-qualgen.yml`, the two that were `false` before. Every other setting is unchanged: `false` on `docker-publish`, `evidence-automerge`, `verify-gate-close`, `verify-gate-open`, `release`; bare `true` on `changelog-check`, `forge-surface-control`, `inbound-triage`, `leaksweep-control`, `leaksweep-pattern`, `plugin-drift`. **No workflow whose `push` leg runs a job that COMMITS acquires a bare `true`** |
| 6 | check:ci | `cd tools/desk && go test ./cmd/deskboard/ -run TestWouldFire -count=1 && go test ./cmd/deskboard/ -run TestZeroCI -count=1` | exit 0 twice — the board's model of `paths` / `paths-ignore` still matches GitHub's, so a filtered workflow reads as "a checked zero", never as `CI-NEVER-RAN`. Two single-pattern runs, not one alternation: `go test -run` compiles RE2 and a table cell cannot carry an unambiguous `\|` |
| 7 | check +flow | `gh pr view "$PR" --repo medici-finance/assay --json statusCheckRollup --jq '.statusCheckRollup\|length'` | exit 0; a number **≥ 4** — **FLOW**: the cross-component path this change could break runs *filter selects fewer workflows → GitHub produces a smaller check rollup → `deskboard` classifies it → `deskflip` reads `checks-green`*. `deskflip` treats an EMPTY rollup on a CI-required repo as `Unverifiable`, so a class-A pull request that reached zero checks could never be flipped ready. Four is the smallest post-change fan-out, so the empty case is unreachable — proven on a real pull request, not at the changed site alone |
| 8 | check | `for f in ci plugin-drift assay-statusgen assay-qualgen evidence-automerge; do diff -q "tools/ci-load/activation/$f.yml" ".github/workflows/$f.yml" \|\| exit 1; done` | exit 0, no output — the staged copies and the landed files are byte-identical, so the reviewed diff is the running diff |
| 9 | check:ci | `for f in tools/ci-load/activation/*.yml; do python3 -c "import yaml,sys; d=yaml.safe_load(open(sys.argv[1])); print(sys.argv[1], d.get(True) or d.get('on'), d.get('concurrency')); sys.exit(0 if (d.get(True) or d.get('on')) and d.get('jobs') else 1)" "$f" \|\| exit 1; done` | exit 0; five lines. `ci.yml` shows `push` with `branches: ['main']` and a four-entry `paths-ignore`, and `cancel-in-progress` conditioned on `github.event_name == 'pull_request'`; `plugin-drift.yml` shows `paths-ignore` on `pull_request` and a bare `branches: ['main']` on `push` |
| 10 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | `0`, and no `PROBLEM:` line naming `desk-supervision/09` or any file under `tools/ci-load/` |
| 11 | check:ci +dereference | `cd statusgen && go run . --root .. --consumers 2>&1 \| grep -A6 'desk-supervision/09'` | exit 0; the block lists all four `consumers:` entries and marks `tools/ci-load/activation/*.yml` **CORROBORATED**. The three `out-of-scope` entries render UNCHECKED by design — that routing is a judgement no diff settles, and each one's stated reason is discharged by a row above (row 6 for `zeroci.go`, row 7 for `flip.go`, rows 2 and 4 for `leak-sweep`) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review

Gate: model (from frontmatter; all four risk answers are `no`). Reviewer records verdict +
date in the stream README table.

**The risk×files cross-read fires on this brief, and the disposition is recorded here rather
than engineered around.** `statusgen --lint` NOTICEs that a brief answering all four risk
questions `no` names `.github/workflows/` — that is correct and expected, and it is the
input the check exists to put in front of a human. The answer offered, for the reviewer to
accept or reject: the paths are named as the destination of a human copy, not as something
this diff writes (the authoring identity holds no `workflows` permission); the change is
trigger selection with no assertion, guard or step altered anywhere; the two leak workflows
and the one required status check are provably untouched (Verify rows 2 and 4); and a revert
is a single `git revert` with no state to unwind. If a reviewer disagrees with any of those
four, the correct outcome is `gate: human` on this brief, not a quieter notice.

Two further questions this brief asks its reviewer to answer explicitly, because they are
where a CI-selection change goes wrong:

1. **Did any control lose coverage?** Read the diff for a `paths`, `paths-ignore`, `if:` or
   `concurrency` added to a workflow that ASSERTS something, as opposed to one that selects.
   The claim under review is that all five edits are selection-only: `leaksweep-control.yml`
   and `leaksweep-pattern.yml` are absent from the diff, the `leak-sweep` required status is
   not a workflow in this repo, and the one `if:` added
   (`evidence-automerge.yml`) sits in FRONT of an unchanged default-deny guard rather than
   replacing any part of it.
2. **Can any pull request now reach zero checks?** A filtered workflow produces no check run
   at all, not a `skipped` one, and an empty rollup is `Unverifiable` to `deskflip` on a
   CI-required repo — a PR that can never flip. The claim is that the smallest post-change
   fan-out is four (`changelog-check`, both leak legs, `statusgen-board lint` for anything
   under `docs/**`); check that claim against a diff shape the table above does not list.
