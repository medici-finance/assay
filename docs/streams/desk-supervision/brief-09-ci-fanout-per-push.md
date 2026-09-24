---
brief: assay:assay:desk-supervision:09
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
schema: brief-v2
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
version: 1
id: 4b4dc7f7-13f1-42cc-8b97-6ff097f1e7c8
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
- `changelog/ci-per-push-fanout.md` (planned) — the fragment. Marked `(planned)` because a
  release roll AGGREGATES every fragment into the dated `CHANGELOG.md` section and CLEARS
  `changelog/` in the same commit, so a fragment path is transient by design: it exists only
  between the brief's own PR and the next release cut. This one was consumed by the v0.26.0
  roll.

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

7. Add `changelog/ci-per-push-fanout.md` (planned), and add the row + wave entry for this brief to
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

### Non-implementer verifier run — VERIFY: BLOCKED (6 offline rows PASS; 6 landing-dependent rows UNRUN pending human workflows-copy) — HELD at implemented — 2026-09-05 opus-4.8[1m]-verifier (verify-desk dispatch), medici-finance/assay merged main 55bb04c

Runner != implementer. Offline envelope (KUBECONFIG=/dev/null). gate: model; risk {regulatory:no, customer:no, irreversible:no, sensitive-data:no}. Two-phase deliverable: the five workflow files are STAGED under tools/ci-load/activation/ and differ from the landed .github/workflows/ — the implementing identity holds no workflows permission, so activation is a human copy. Rows that read the landed tree / a live class-A PR are unreachable until that copy lands.

| # | command | expected | observed (exit + key line) | date · runner |
|---|---------|----------|----------------------------|---------------|
| 1 | PR/branch run-list, ci absent (before/after on landed tree) | measurement | UNRUN — human workflows-copy not landed; no post-landing tree or class-A PR to measure | 2026-09-05 · opus-4.8[1m]-verifier |
| 2 | commit status + rulesets (leak-sweep present, ruleset named) | present | UNRUN — requires landed head + live GH API (offline) | 2026-09-05 · opus-4.8[1m]-verifier |
| 3 | python3 tools/ci-load/pathsemantics.py | exit 0 PASS; mixed/go-only/plugin-only skipped=False | exit 0 PASS; both workflows mixed=False go-only=False plugin-only=False docs-only=True status-regen=True | 2026-09-05 · opus-4.8[1m]-verifier |
| 3b | mutate all( to any(, re-run, restore | exit 1 on mutant, exit 0 after restore | exit 1 FAIL mixed skipped=True (want False) both; restored exit 0 PASS; git diff clean | 2026-09-05 · opus-4.8[1m]-verifier |
| 4 | byte-diff leak workflows across landing commit | exit 0, no output | UNRUN — derives base from last commit to touch ci.yml; no landing commit yet | 2026-09-05 · opus-4.8[1m]-verifier |
| 5 | grep cancel-in-progress the landed workflows | exactly two event-conditioned settings | UNRUN — reads landed tree; pre-landing count of event-conditioned settings = 0 | 2026-09-05 · opus-4.8[1m]-verifier |
| 6 | go test ./cmd/deskboard -run TestWouldFire then -run TestZeroCI | exit 0 twice | TestWouldFire 0 (0.54s); TestZeroCI 0 (23.2s); both ok | 2026-09-05 · opus-4.8[1m]-verifier |
| 7 | PR statusCheckRollup length >= 4 | >= 4 | UNRUN — requires landed tree + live class-A PR (offline) | 2026-09-05 · opus-4.8[1m]-verifier |
| 8 | staged vs landed byte-diff (5 files) | exit 0, no output | UNRUN — all five staged files DIFFER from landed (identity asserted only AFTER the human copy) | 2026-09-05 · opus-4.8[1m]-verifier |
| 9 | yaml structure of the staged files | 5 lines; specified shapes | exit 0; 5 lines exactly as specified | 2026-09-05 · opus-4.8[1m]-verifier |
| 10 | statusgen --root .. --lint | exit 0, no PROBLEM naming ds/09 or ci-load | exit 0; zero PROBLEM for brief/ci-load; only expected NOTICE risk-files-crossread + unrelated notices | 2026-09-05 · opus-4.8[1m]-verifier |
| 11 | statusgen --consumers block | 4 entries; activation/*.yml CORROBORATED; 3 out-of-scope UNCHECKED | exit 0 (base scoped to brief); CORROBORATED tools/ci-load/activation/*.yml; 3 UNCHECKED by design; 1 corroborated 0 disproved 3 unchecked | 2026-09-05 · opus-4.8[1m]-verifier |

**VERIFY: BLOCKED — HELD at implemented.** All 6 no-landing rows PASS (3, 3b, 6, 9, 10, 11); the 6 landing-dependent rows (1, 2, 4, 5, 7, 8) are UNRUN because the human workflows-permission copy of the five staged files into .github/workflows/ has not been performed and no class-A Evidence PR exists to measure. No row FAILs — blocked-on-human-activation, not FAIL. Advancing to verified requires the copy to land, then rows 1,2,4,5,7,8 re-run.

RISK-VALUE: DERIVED — evidence-automerge enable-if login = assay-verifier-app[bot] @ tools/ci-load/activation/evidence-automerge.yml:93 — must equal the verify-desk App that authors Evidence PRs (the same identity kept in sync with the EVIDENCE_AUTHOR literal); fail-safe — a pre-filter in front of an unchanged default-deny in-job guard, so drift narrows a lane or is declined, never widens automerge authority.
RISK-VALUE: DERIVED — ci/plugin-drift paths-ignore = the documentary paths (docs, changelog, CHANGELOG.md, STATUS.md) @ tools/ci-load/activation/ci.yml:57-66 — exactly the paths no build/test/plugin job reads; rows 3/3b prove mixed/go-only/plugin-only diffs still run and the leak/security workflows are unfiltered, so the required leak-sweep check is unaffected. Reversible knob.
### Non-implementer verifier re-run — VERIFY: BLOCKED (6 offline rows re-confirmed; 1 could-not-check on a tool guard; 6 landing-dependent rows still UNRUN — human workflows-copy still not landed) — HELD at implemented — 2026-09-15 sonnet-5-verifier (verify-desk dispatch), medici-finance/assay merged main 0bf1166a

Runner != implementer. Offline envelope (KUBECONFIG=/dev/null). gate: model; risk {regulatory:no, customer:no, irreversible:no, sensitive-data:no}. Re-verification of the 2026-09-05 BLOCKED verdict, ten days later: the state is UNCHANGED — the five staged files under `tools/ci-load/activation/` still have not been copied into `.github/workflows/` by a human (the implementing/verifying identities hold no `workflows` permission). `help wanted` issue filed: medici-finance/assay#1185.

| # | command | expected | observed (exit + key line) | date · runner |
|---|---------|----------|----------------------------|---------------|
| 1 | PR/branch run-list (newest verify-desk PR #1177, a different brief's Evidence PR, used only as the lookup target) | after-landing: ci/push, ci/pull_request, plugin-drift/pull_request absent | UNRUN as an after-measurement — landed tree not present; run list on PR #1177 branch still shows `ci/push`, `ci/pull_request` and `plugin-drift/pull_request` present, matching the documented BEFORE state | 2026-09-15 · sonnet-5-verifier |
| 2 | commit status + rulesets on PR #1177 head | leak-sweep present + ruleset named | rulesets list contains `leak-sweep`, `protect-main`, `protect-release-tags` (ruleset present); the `/commits/{sha}/status` statuses array came back empty on this particular head (unrelated to this brief — leak-sweep posts on its own ~15-min poll per house canon; not investigated further as out of this brief's scope) — COULD-NOT-CHECK the presence-on-this-commit sub-claim, ruleset-exists sub-claim confirmed | 2026-09-15 · sonnet-5-verifier |
| 3 | `python3 tools/ci-load/pathsemantics.py` | exit 0 PASS; mixed/go-only/plugin-only skipped=False | exit 0 PASS; both workflows mixed=False go-only=False plugin-only=False docs-only=True status-regen=True | 2026-09-15 · sonnet-5-verifier |
| 3b | mutate `all(` to `any(` in `skipped()`, re-run, restore | exit 1 on mutant, exit 0 after restore | COULD-NOT-CHECK — the mutation edit was refused by this session's tool-use guard ("Permission for this action was denied by the Claude Code auto mode classifier. Reason: [Modify Shared Resources]"); per no-evasion policy the edit was not re-attempted via another tool/command; file confirmed unmodified (`git diff --stat` clean) | 2026-09-15 · sonnet-5-verifier |
| 4 | byte-diff leak workflows across the `ci.yml` landing commit | exit 0, no output | UNRUN — `.github/workflows/ci.yml` has never been touched by a landing commit (still unfiltered, no `concurrency:` block); no landing commit exists to diff from | 2026-09-15 · sonnet-5-verifier |
| 5 | `grep -n cancel-in-progress .github/workflows/*.yml` | exactly two event-conditioned settings (assay-statusgen, assay-qualgen) | UNRUN as after-measurement — both still read bare `cancel-in-progress: false`, unconditioned; zero event-conditioned settings present anywhere in the live tree | 2026-09-15 · sonnet-5-verifier |
| 6 | `go test ./cmd/deskboard/ -run TestWouldFire -count=1` then `-run TestZeroCI -count=1` | exit 0 twice | exit 0 (0.29s) and exit 0 (0.67s); both `ok` | 2026-09-15 · sonnet-5-verifier |
| 7 | PR statusCheckRollup length >= 4 | >= 4 | measured on PR #1177 (not a class-A PR post-landing, since nothing has landed): 21 — informative only, not the row's designed after-measurement | 2026-09-15 · sonnet-5-verifier |
| 8 | staged vs landed byte-diff (5 files) | exit 0, no output | UNRUN — all five staged files still DIFFER from `.github/workflows/` (unchanged from 2026-09-05) | 2026-09-15 · sonnet-5-verifier |
| 9 | yaml structure of the staged files | 5 lines; specified shapes | exit 0; 5 lines, exactly as specified — `ci.yml` shows `push:branches:[main]` + 4-entry `paths-ignore` + conditioned `cancel-in-progress`; `plugin-drift.yml` shows `paths-ignore` on `pull_request` only and bare `branches:[main]` on `push` | 2026-09-15 · sonnet-5-verifier |
| 10 | `cd statusgen && go run . --root .. --lint` | exit 0, no PROBLEM naming ds/09 or ci-load | exit 0; zero `PROBLEM:` lines total; none name desk-supervision/09 or tools/ci-load/ | 2026-09-15 · sonnet-5-verifier |
| 11 | `statusgen --consumers` block for desk-supervision/09 | 4 entries; activation/*.yml CORROBORATED; 3 out-of-scope UNCHECKED | The literal `--consumers` (default base `origin/main`) command produces no output post-merge (HEAD already IS origin/main, no diff to corroborate). Ran `--consumers --brief desk-supervision/09 --base <pre-landing commit>` instead: base pinned 10 days upstream of HEAD picked up UNRELATED intervening commits and produced two false `DISPROVED` hits on `zeroci.go`/`flip.go`. Verified directly instead: `git diff --stat 162c07b7^..162c07b7` (the brief's own landing commit) touches neither `zeroci.go` nor `flip.go`, and does introduce all of `tools/ci-load/activation/{README.md,ci.yml,plugin-drift.yml,assay-statusgen.yml,assay-qualgen.yml,evidence-automerge.yml,ci-load.diff}` — confirms the CORROBORATED entry and both out-of-scope claims hold; the tool's stale-base run is a scoping artifact of re-running this check long after the brief's own commit, not a defect in the brief | 2026-09-15 · sonnet-5-verifier |

**VERIFY: BLOCKED — HELD at implemented (unchanged).** Re-verification ten days after the first BLOCKED pass finds the same state: the six offline rows (3, 6, 9, 10, 11) re-confirm PASS; row 3b is COULD-NOT-CHECK this session only, due to a tool-use guard blocking the required mutation edit (quoted verbatim above) — not re-attempted via another tool per no-evasion policy, and the underlying file is unchanged since its last confirmed PASS on 2026-09-05. The six landing-dependent rows (1, 2, 4, 5, 7, 8) remain UNRUN/not-yet-measurable: no human has copied the five staged workflow files into `.github/workflows/` on `main`. `help wanted` issue filed: medici-finance/assay#1185 (no prior open issue found for this blocker). No row FAILs. Advancing to verified still requires the human copy to land, then rows 1, 2, 4, 5, 7, 8 (and, given the guard above, a fresh attempt at row 3b under different tooling) to re-run.

RISK-VALUE: DERIVED (re-affirmed, literal unchanged since 2026-09-05) — `evidence-automerge` `enable`-job `if:` login = `assay-verifier-app[bot]` @ `tools/ci-load/activation/evidence-automerge.yml` (confirmed present in this session's row-9 YAML read) — must equal the verify-desk App that authors Evidence PRs; fail-safe by construction (pre-filter in front of an unchanged default-deny in-job guard), so drift narrows the lane or is declined, never widens automerge authority.
RISK-VALUE: DERIVED (re-affirmed, literal unchanged since 2026-09-05) — `ci`/`plugin-drift` `paths-ignore` = `["docs/**", "changelog/**", "CHANGELOG.md", "STATUS.md"]` @ `tools/ci-load/activation/ci.yml` (confirmed present in this session's row-9 YAML read) — exactly the documentary paths no build/test/plugin job reads; row 3 (re-run this session) proves mixed/go-only/plugin-only diffs still run, and the two leak/security workflows remain unfiltered (confirmed unchanged in the live tree this session, row 5's grep). Reversible knob (single `git revert` of the eventual landing commit).
### Non-implementer verifier re-run — VERIFY: BLOCKED, HELD at implemented (unchanged), infra loss mid-session — sonnet-5-verifier (verify-desk dispatch), @ merged main `951ca784d100a7d201a28a34033da6709ec2ec8f`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main, confirmed present/writable at task start (`git rev-parse --show-toplevel` matched). Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted.

**Session note**: after row 6 and before row 7, the home worktree vanished from disk entirely — no `git worktree list` registration anywhere, no destructive call made from this session (the one Edit attempt, row 3b, was blocked by the auto-mode classifier before touching disk). Rows 9-11 could not be run as a direct result and are recorded could-not-check, not rounded up to pass. Flagged separately as an infra finding (worktree-prune/active-dispatch race suspected), not restated here.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | PR/branch run-list for verifier's own recent Evidence PR (#1301) → workflow run list | after-landing: no `ci/push`, `ci/pull_request`, `plugin-drift/pull_request` | exit 0; all three still present — unchanged BEFORE state (documented, not a regression) | 2026-09-18 | sonnet-5-verifier |
| 2 | commit status + rulesets check on PR #1301 head | leak-sweep present + ruleset named + no-such-check=0 | exit 0; statuses=["leak-sweep"]; rulesets=["leak-sweep","protect-main","protect-release-tags"]; no-such-check=0 — checked-clean, full PASS | 2026-09-18 | sonnet-5-verifier |
| 3 | `python3 tools/ci-load/pathsemantics.py` | exit 0 PASS | exit 0; PASS; mixed=False go-only=False plugin-only=False docs-only=True status-regen=True | 2026-09-18 | sonnet-5-verifier |
| 3b | mutation on `skipped()` (`all(`→`any(`) | exit 1 on mutant, exit 0 after restore | **could-not-check** — Edit blocked verbatim by the auto-mode classifier ("Modify Shared Resources"); not re-attempted via another tool (no-evasion); `git status --porcelain` confirmed clean immediately after | 2026-09-18 | sonnet-5-verifier |
| 4 | byte-diff leak workflows across the landing commit | exit 0, no output | **UNRUN as after-measurement** — no landing commit exists yet; live `ci.yml` still has no concurrency/paths-ignore/branches block | 2026-09-18 | sonnet-5-verifier |
| 5 | `grep -n cancel-in-progress .github/workflows/*.yml` | exactly two event-conditioned settings | **UNRUN as after-measurement** — both files still read bare `cancel-in-progress: false`; unchanged BEFORE state | 2026-09-18 | sonnet-5-verifier |
| 6 | `go test ./cmd/deskboard/ -run TestWouldFire` then `-run TestZeroCI` | exit 0 twice | exit 0 (0.302s) ok; exit 0 (0.956s) ok — checked-clean | 2026-09-18 | sonnet-5-verifier |
| 7 | `statusCheckRollup` length on PR #1301 | ≥4 | 23 — informative only, not the row's designed after-measurement (nothing has landed yet) | 2026-09-18 | sonnet-5-verifier |
| 8 | byte-diff staged vs landed (5 files) | exit 0, no output | **UNRUN** — all 5 files under tools/ci-load/activation/ still differ from .github/workflows/*, unchanged since 2026-09-05/09-15 | 2026-09-18 | sonnet-5-verifier |
| 9 | yaml structure of staged files | 5 lines; specified shapes | **could-not-check** — home worktree vanished before this row ran | 2026-09-18 | sonnet-5-verifier |
| 10 | `statusgen --root .. --lint` | exit 0, no PROBLEM naming ds/09 or ci-load | **could-not-check** — same reason | 2026-09-18 | sonnet-5-verifier |
| 11 | `statusgen --consumers` block for desk-supervision/09 | 4 entries; activation CORROBORATED, 3 UNCHECKED | **could-not-check** — same reason | 2026-09-18 | sonnet-5-verifier |

Scope traceability: all 11 rows map 1:1 to the brief's own Verify table; no invented scope.

RISK-VALUE: DERIVED (re-affirmed via row 3's live output this session) — `ci`/`plugin-drift` `paths-ignore` = `["docs/**","changelog/**","CHANGELOG.md","STATUS.md"]` @ tools/ci-load/activation/ci.yml — row 3 (real output this session) proves mixed/go-only/plugin-only diffs still run; the two leak/security workflows remain unfiltered in the live tree (row 5, re-run this session). Reversible knob.
RISK-VALUE: DERIVED (carried forward from the 2026-09-05/09-15 confirmed reads — not independently re-read this session since the worktree vanished before that file's content could be re-cat'd) — `evidence-automerge` enable-job `if:` login = `assay-verifier-app[bot]` @ tools/ci-load/activation/evidence-automerge.yml:93 — flagging the provenance gap explicitly rather than claiming a fresh read.

**VERIFY: BLOCKED — HELD at implemented (unchanged).** Third consecutive pass (2026-09-05, 2026-09-15, 2026-09-18) finding the identical state: the human copy of the five staged workflow files into `.github/workflows/` has still not landed, 13 days after `implemented`. No row that ran contradicts the brief's claims. `help wanted` issue medici-finance/assay#1185 remains the tracked blocker — not re-filed. Rows 9-11 went could-not-check this session due to an infra loss (worktree vanished mid-session, not a code or brief defect) — flagged separately, not counted as a regression against the brief.
### Non-implementer verifier pass — VERIFY: BLOCKED (6 rows PASS, 5 landing-dependent rows UNRUN as designed, 0 rows FAIL; blocker unchanged since 2026-09-05: the human workflows-copy of the five staged files into .github/workflows/ has still not landed; tracked #1185, not re-filed) — verify-desk-dispatch-20260920T0246Z (verify-desk dispatch), @ merged main `e4109205`, 2026-09-20

Isolated worktree at origin/main, offline envelope, read-only, non-implementer.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | latest verifier-App Evidence PR head → gh run list --branch B unique-sorted workflow/event | after-landing: leaksweep-control/pull_request, leaksweep-pattern/pull_request, changelog-check/pull_request, statusgen-board/pull_request present; NO ci/push, ci/pull_request, plugin-drift/pull_request | UNRUN as the designed after-measurement (activation not landed). Widened window on PR #1334 (docs-only Evidence PR): all four survivors present, but ci/push (2 runs), ci/pull_request (1), plugin-drift/pull_request (1) ALL present = documented BEFORE state. (First literal run's ci/push absence was gh's default 20-run window, not an after-state; re-checked before recording) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 2 | PR #1334 head commit statuses; rulesets list | first contains leak-sweep; second names the requiring ruleset; no-such-check count 0 | exit 0 all three; statuses = [leak-sweep]; rulesets = [leak-sweep, protect-main, protect-release-tags]; ruleset leak-sweep (id 20872509, active) requires context leak-sweep; no-such-check count 0. CHECKED-CLEAN PASS (activation-independent) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 3 | python3 tools/ci-load/pathsemantics.py | exit 0; last line PASS; mixed/go-only/plugin-only skipped=False | exit 0; last line PASS; both staged workflows read paths-ignore = [docs/**, changelog/**, CHANGELOG.md, STATUS.md]; mixed=False go-only=False plugin-only=False docs-only=True status-regen=True | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 3b | MUTATION: all( → any( in skipped() in pathsemantics.py, re-run, restore | exit 1 on mutant with FAIL mixed: skipped=True; exit 0 after restore | mutant: exit 1, FAIL mixed: skipped=True (want False) for BOTH ci.yml and plugin-drift.yml; byte-clean restore. Newly re-confirmed after two could-not-check passes (the 2026-09-15/09-18 classifier block did not recur) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 4 | L=last commit touching .github/workflows/ci.yml; diff --stat L^ HEAD -- staged files | exit 0, no output, across the ACTIVATION landing commit | VACUOUS as written, recorded UNRUN-as-designed: derived last-ci.yml-commit ef99c1905 (2026-09-07) is a Go-test-tools merge, NOT the activation landing; no activation commit exists (the real landing commit 162c07b7 touches no .github/workflows/ file — see row 11) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 5 | grep -n cancel-in-progress .github/workflows/*.yml | exactly two event-conditioned settings (assay-statusgen.yml, assay-qualgen.yml) | UNRUN as after-measurement — BEFORE state: landed tree has ZERO event-conditioned settings (assay-statusgen.yml:52 and assay-qualgen.yml:55 both bare cancel-in-progress: false; others unchanged) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 6 | cd tools/desk && go test ./cmd/deskboard/ -run TestWouldFire -count=1 && go test ./cmd/deskboard/ -run TestZeroCI -count=1 | exit 0 twice | exit 0 twice — ok (0.276s), ok (1.034s), as two single-pattern runs per the row's RE2 note | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 7 | PR #1334 statusCheckRollup length | number ≥ 4, on a POST-landing class-A PR | length 27 — informative only, pre-landing; the designed after-measurement is unrunnable until activation | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 8 | for f in ci plugin-drift assay-statusgen assay-qualgen evidence-automerge; do diff -q tools/ci-load/activation/$f.yml .github/workflows/$f.yml; done | exit 0, no output (staged == landed byte-identical) | exit 1 — first line: ci.yml differs; all five staged files differ from landed. This exit 1 IS the blocker stated mechanically | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 9 | yaml structure check of tools/ci-load/activation/*.yml | exit 0; five lines, specified shapes | exit 0 — five lines exactly as specified (ci push branches:[main] + 4-entry paths-ignore, pull_request same, concurrency ci-<ref> with event-conditioned cancel; plugin-drift paths-ignore on pull_request only; both statusgen/qualgen staged files event-conditioned; evidence-automerge triggers unchanged) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 10 | cd statusgen && go run . --root .. --lint | exit 0; no PROBLEM naming desk-supervision/09 or tools/ci-load/ | exit 0 — LINT: PASS, zero PROBLEM lines | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 11 | --consumers grep desk-supervision/09 | block lists all four consumers entries, activation/*.yml CORROBORATED, 3 out-of-scope UNCHECKED | literal run: exit 0 but the known scoping artifact (HEAD IS the default base on merged main; diff empty). Completed by direct dereference from merged-main history: landing commit 162c07b75 (2026-09-05) introduces all of tools/ci-load/activation/ + brief + pathsemantics.py = CORROBORATED holds; touches NEITHER tools/desk/cmd/deskboard/zeroci.go NOR tools/desk/cmd/deskflip/flip.go and NO .github/workflows/ file = both out-of-scope routings and the leak-sweep claim hold AT LANDING. Scoped tool run deliberately NOT fabricated with a stale base (zeroci.go/flip.go have since changed substantially — the false-DISPROVED trap the 2026-09-15 pass documented) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |

RISK-VALUE: DERIVED — evidence-automerge enable-job if: login = assay-verifier-app[bot] @ tools/ci-load/activation/evidence-automerge.yml:96 — the automerge lane exists solely for Evidence PRs, authored exclusively by the verify-desk App (live-confirmed: gh pr list --author assay-verifier-app[bot] returns #1334); the same literal kept in sync at EVIDENCE_AUTHOR @ :153 in front of the default-deny guard @ :180 — drift narrows the lane or is declined, never widens automerge authority.
RISK-VALUE: DERIVED — ci/plugin-drift paths-ignore = [docs/**, changelog/**, CHANGELOG.md, STATUS.md] @ tools/ci-load/activation/ci.yml:56 (push) and :62 (pull_request) — exactly the documentary paths no build/test/plugin job reads (rationale in the ci.yml header @ :30-37); row 3 proves mixed/go-only/plugin-only diffs still run and docs-only/status-regen skip; the leak/security workflows carry no filter and the landing commit touches no .github/workflows/ file (row 2 dereferenced live).
RISK-VALUE: DERIVED — writer cancel conditioning = event_name == 'pull_request' @ tools/ci-load/activation/assay-statusgen.yml:61, assay-qualgen.yml:62 (and ci.yml:75) — STATUS.md and QUALITY.md each have exactly one writer (the main regen job); conditioning to pull_request means only the write-nothing PR jobs can be superseded and a main run is never cancelled by the next merge.

VERIFY: BLOCKED — 6 rows PASS (3, 3b, 6, 9, 10, 11-as-dereferenced), 5 landing-dependent rows (1, 4, 5, 7, 8) UNRUN as designed, 0 rows FAIL. Advancing requires the human workflows-permission copy of the five staged files to land on main, then rows 1, 4, 5, 7, 8 re-run as after-measurements. Blocker tracked at #1185 (help wanted, OPEN). Item stays implemented; no flip.

### Non-implementer verifier run — VERIFY: BLOCKED — 0/12 pass, 12 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Runner != implementer. Fresh classification pass on current merged main
(7e8e79ed43f30fcf43dc2fdbf735606cce2a8323). Offline envelope (KUBECONFIG=/dev/null); no PR,
no push, no status flip. gate: model; risk {regulatory:no, customer:no, irreversible:no,
sensitive-data:no}. Two-phase deliverable: the five workflow files are STAGED under
tools/ci-load/activation/ and the human workflows-permission copy into .github/workflows/ has
STILL not landed — so the six landing-dependent rows (1, 2, 4, 5, 7, 8) remain unrunnable as
after-measurements, unchanged since 2026-09-05. Blocker tracked at #1185 (help wanted, open).
Execution witness written by statusgen verifyrun into this brief's Evidence section in the
verifier worktree; note that verifyrun re-runs check:ci rows in a network-off unshare --net
sandbox that is a Linux facility unavailable on this darwin host, so the check:ci rows
3/3b/6/9/10 are COULD-NOT-CHECK on this darwin desk — the direct non-hermetic run is recorded
supporting-only, never as a pass (#1491 is the `--in-container` witness path). With the six
landing-dependent human-gated rows (1,4,5,7,8), the offline forge-API row 2, and the merged-tree
consumers row 11, every one of the 12 rows is could-not-check this pass.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | PR=$(gh pr list --repo medici-finance/assay --state all --author 'assay-verifier-app[bot]' --limit 1 --json number --jq '.[0].number'); B=$(gh pr view "$PR" --repo medici-finance/assay --json headRefName --jq .headRefName); gh run list --repo medici-finance/assay --branch "$B" --json workflowName,event --jq '[.[]\|"\(.workflowName)/\(.event)"]\|sort\|unique' | after-landing: leaksweep-control, leaksweep-pattern, changelog-check, statusgen-board pull_request present; NO ci/push, ci/pull_request, plugin-drift/pull_request | COULD-NOT-CHECK — the designed after-measurement cannot run: activation not landed, so no post-landing tree or class-A PR exists to measure; the live measurement is also outside this pass's offline envelope. Human-workflow copy not landed (#1185) | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | H=$(gh pr view "$PR" --repo medici-finance/assay --json headRefOid --jq .headRefOid); gh api "repos/medici-finance/assay/commits/$H/status" --jq '[.statuses[].context]'; gh api repos/medici-finance/assay/rulesets --jq '[.[].name]' | leak-sweep present; ruleset naming it; discriminator count 0 | COULD-NOT-CHECK — requires a live forge-API dereference, outside this pass's offline envelope; the ruleset/leak-sweep sub-claim is activation-independent and was confirmed live on the 2026-09-20 pass (statuses=[leak-sweep], ruleset leak-sweep active, no-such-check 0), not re-asserted here as this verifier's own observation | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | python3 tools/ci-load/pathsemantics.py | exit 0; last line PASS; mixed/go-only/plugin-only skipped=False | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; last line PASS; both staged workflows read paths-ignore [docs/**, changelog/**, CHANGELOG.md, STATUS.md]; mixed=False go-only=False plugin-only=False docs-only=True status-regen=True | 2026-09-23 | claude-opus-4-8-verifier |
| 3b | (manual row; no shell command) | exit 1 on mutant with FAIL mixed skipped=True; exit 0 after restore | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): manual mutation: skipped() outer all( changed to any( in tools/ci-load/pathsemantics.py, row 3 re-run gives exit 1 with FAIL mixed skipped=True (want False) for BOTH ci.yml and plugin-drift.yml; after restore exit 0 PASS; git status --porcelain byte-clean | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | L=$(git log -1 --format=%H -- .github/workflows/ci.yml); git diff --stat "$L^" HEAD -- .github/workflows/leaksweep-control.yml .github/workflows/leaksweep-pattern.yml | exit 0, no output, across the ACTIVATION landing commit | COULD-NOT-CHECK — no activation landing commit exists; the last commit to touch ci.yml is not the activation copy (activation still unlanded, live ci.yml unfiltered), so the after-measurement is vacuous until the human copy lands (#1185) | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | grep -n 'cancel-in-progress' .github/workflows/*.yml | exactly two event-conditioned settings (assay-statusgen.yml, assay-qualgen.yml) | COULD-NOT-CHECK — before-state: the landed tree has ZERO event-conditioned settings (assay-statusgen.yml and assay-qualgen.yml both bare cancel-in-progress: false); the two event-conditioned settings are the human-gated half not yet landed. No landed workflow whose push leg commits carries a bare true regression (#1185) | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | cd tools/desk && go test ./cmd/deskboard/ -run TestWouldFire -count=1 -v && go test ./cmd/deskboard/ -run TestZeroCI -count=1 -v | exit 0 twice; named tests actually run | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): TestWouldFire exit 0 (0.274s), 16 subtests RUN incl paths-ignore_one_file_survives and paths-ignore_all_files_ignored, --- PASS; TestZeroCI exit 0 (2.537s), 16 subtests --- PASS incl NoChecks_PathFiltered; run as two single-pattern invocations per the row's RE2 note | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | gh pr view "$PR" --repo medici-finance/assay --json statusCheckRollup --jq '.statusCheckRollup\|length' | number >= 4 on a POST-landing class-A PR | COULD-NOT-CHECK — no post-landing class-A PR exists (activation unlanded); the live rollup read is also outside this pass's offline envelope (#1185) | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | for f in ci plugin-drift assay-statusgen assay-qualgen evidence-automerge; do diff -q "tools/ci-load/activation/$f.yml" ".github/workflows/$f.yml" \|\| exit 1; done | exit 0, no output (staged == landed byte-identical) | COULD-NOT-CHECK — the human copy of the staged activation files into .github/workflows/ has not landed, so the staged/landed byte-identity this row measures cannot hold yet: diff -q reports ci.yml differs (all five differ from landed). The unlanded human-workflow action, not a shipped-code defect (#1185) | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | for f in tools/ci-load/activation/*.yml; do python3 -c "import yaml,sys; d=yaml.safe_load(open(sys.argv[1])); print(sys.argv[1], d.get(True) or d.get('on'), d.get('concurrency')); sys.exit(0 if (d.get(True) or d.get('on')) and d.get('jobs') else 1)" "$f" \|\| exit 1; done | exit 0; five lines; specified shapes | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; five lines. ci.yml: push branches [main] + four-entry paths-ignore, pull_request same paths-ignore, concurrency group ci-<ref> with cancel-in-progress conditioned on event_name == pull_request; plugin-drift.yml: paths-ignore on pull_request only, bare branches [main] on push; assay-statusgen/qualgen: cancel conditioned; evidence-automerge: triggers unchanged | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | cd statusgen && go run . --root .. --lint; echo $? | exit 0; no PROBLEM line naming desk-supervision/09 or any file under tools/ci-load/ | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run (supporting only): exit 0; LINT: PASS; zero lines beginning PROBLEM: in the whole output; none name desk-supervision/09 or tools/ci-load/; the expected risk-files-crossread NOTICE for desk-supervision:09 fires (brief answers four no's but declares .github/workflows/) — a NOTICE, not a PROBLEM, exactly as the brief's Review section anticipates | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | cd statusgen && go run . --root .. --consumers 2>&1 \| grep -A6 'desk-supervision/09' | block lists four consumers entries; activation/*.yml CORROBORATED; three out-of-scope UNCHECKED | COULD-NOT-CHECK — literal --consumers grep exit 1 (no output): HEAD is origin/main so there is no diff base to corroborate against (merged-tree scoping artifact). Completed by direct dereference: landing commit 162c07b751 introduces all of tools/ci-load/activation/*.yml + README.md + ci-load.diff + pathsemantics.py + the brief + changelog fragment (CORROBORATED holds); it touches none of tools/desk/cmd/deskboard/zeroci.go, tools/desk/cmd/deskflip/flip.go or any .github/workflows/ file (the three out-of-scope routings and the leak-sweep claim hold at landing). The gate did not decide it, so recorded as could-not-check | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE enumeration (fail-safe trigger fires: the diff touches a risk-classed path,
.github/workflows/, and the brief answers all four risk questions no). Every literal the diff
introduces/changes: (a) paths-ignore = [docs/**, changelog/**, CHANGELOG.md, STATUS.md] @
tools/ci-load/activation/ci.yml:56-66 and plugin-drift.yml:46; (b) evidence-automerge enable-if
login = assay-verifier-app[bot] @ tools/ci-load/activation/evidence-automerge.yml:96; (c) writer
cancel conditioning = github.event_name == pull_request @ tools/ci-load/activation/
assay-statusgen.yml:61, assay-qualgen.yml:62, ci.yml:75; (d) concurrency group = ci-<github.ref>
@ ci.yml:72; (e) branches: [main] on the ci.yml push leg @ ci.yml:55. RANK: none is
irreversible — every one reverts with a single git revert of the eventual landing commit; (d)
and (e) are pure reversible scoping/grouping knobs and need no derivation. Top three by
consequence-if-wrong derived below.

RISK-VALUE: DERIVED — evidence-automerge enable-if login = assay-verifier-app[bot] @ tools/ci-load/activation/evidence-automerge.yml:96 — must equal the verify-desk App that authors Evidence PRs; kept in sync with EVIDENCE_AUTHOR @ :153 and sits in FRONT of an unchanged default-deny in-job guard (writes eligible=false first @ :159, re-reads PR author against EVIDENCE_AUTHOR @ :180 plus draft and paginated file-list checks). Drift is fail-safe both directions: narrowed the automerge lane goes silent and Evidence PRs wait for a human merge (the pre-lane state); widened the extra PR is declined by the unchanged in-job check — never widens automerge authority.
RISK-VALUE: DERIVED — ci/plugin-drift paths-ignore = [docs/**, changelog/**, CHANGELOG.md, STATUS.md] @ tools/ci-load/activation/ci.yml:56-66 (push and pull_request legs) and plugin-drift.yml:46 (pull_request leg only) — exactly the documentary paths no build/test/plugin job reads; GitHub skips a workflow only when EVERY changed file is ignored, so a mixed diff always runs in full. Rows 3 and 3b prove mixed/go-only/plugin-only diffs all still run and the assertion can go red. The two leak/security workflows carry no filter (row 11 dereference: the landing commit touches no .github/workflows/ file), so the required leak-sweep check is unaffected. Reversible.
RISK-VALUE: DERIVED — writer cancel conditioning = github.event_name == 'pull_request' @ tools/ci-load/activation/assay-statusgen.yml:61 and assay-qualgen.yml:62 (and ci.yml:75) — STATUS.md and QUALITY.md each have exactly one writer, the main regen job; conditioning cancel-in-progress to pull_request means only the write-nothing PR jobs (lint/render) can be superseded, and a main run — the record a merged commit builds — is never cancelled by the next merge. Reversible.

The five staged workflow files still await the human workflows-scope copy into .github/workflows/
(#1185, help wanted); the check:ci rows (3,3b,6,9,10) await the Linux hermetic witness (#1491).
No row is a pass on this darwin desk.


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
