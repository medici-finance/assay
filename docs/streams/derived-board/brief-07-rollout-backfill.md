---
brief: assay:assay:derived-board:07
title: this repo's rollout — brief-v2 migration, scheduled reconcile, historical backfill as a drift-report PR; --backfill kept for the reconcile verb
why: >-
  Every repository that carries stream boards had phantom rows. The cut is worthless
  until a repo runs the migration, gets the reconcile step, and has its history derived
  once — with the disagreements between what the table said and what GitHub says put in
  front of a human, not silently overwritten. This brief delivers that for this repo;
  each consumer repo runs the same sequence under its own board.
wave: 4
depends: ["derived-board/05", "derived-board/06"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
delivery:
  - {in: "assay#735", covers: partial, note: "statusgen fixes that unblock this repo's brief-v2 migration only; the rollout to other consumers, overlay re-stage and fallback retirement (Task 2-4) remain owed"}
  - {in: "assay#736", covers: partial, note: "brief-v2 migration in this repo only; the rollout to other consumers, overlay re-stage and fallback retirement (Task 2-4) remain owed"}
  - {in: "assay#945", covers: partial, note: "backfill capability and this repo's drift report only; Task 2-4 remain owed"}
  - {in: "assay#1010", covers: partial, note: "the reconcile --apply write arm only; Task 2-4 remain owed"}
  - {in: "assay#1194", covers: partial, note: "this repo's rollout slice and trailer join repair only; its scope keeps the brief in progress, with Task 2-4 still owed"}
  - {in: "assay#5037", covers: partial, note: "retires the stale staged reconcile patch and records this repo's rollout evidence only; Task 2-4 remain owed"}
schema: brief-v2
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §7 (backfill as a drift-report PR)"
  - "migrations/0001-v0.13.0-to-v1.0.0-derived-board.md (brief 06) — the exact workflow YAML adopters add"
  - "freshness-checked 2026-08-22 @ f78ea24 — five stream-bearing repos, resolved via docs/streams/graph-repos.yaml"
exec-tier: any
domain: complicated
consumers:
  - "each consumer repo's .assay-versions, regen workflow and brief migration: out-of-scope (each consumer repo tracks its own rollout on its own board; re-scoped in version 2)"
  - "tools/desk/cmd/deskreconcile/reconcile.go: out-of-scope (calls `statusgen reconcile --backfill --apply`; Task 4 keeps the flag, so this caller is unchanged)"
version: 2
id: 3535fcb8-1d38-4a68-b59d-202985f32264
---

# Brief 07 — rollout + backfill

## Context
**Re-scoped (version 2).** This brief now covers this repo only. Version 1 also carried
the rollout to each consumer repo (old Task 2), a consumer-side overlay re-stage of the
spec docs and desk skills (old Task 3), and retiring the `--backfill` fallback once every
repo had merged (old Task 4). Their dispositions are recorded in the Task list below; the
original rollout description is kept here because it is the sequence each consumer repo
follows under its own board.

As originally written, the files below were touched per consumer repo, one PR each; this
repo first.

files:
- `.assay-versions` — re-pinned to v1.0.0 by `upgrade-assay` (sha256 from the published release's `checksums.txt`, harvested not typed).
- **the repo's statusgen regen workflow** — `reconcile` step + read permissions + schedule, exactly the YAML from the migration's release note.
- `docs/streams/*/README.md`, `docs/streams/*/brief-*.md` — migrated by the migration op.
- **private toolkit only:** `docs/brief-rules.md`, `docs/brief-template.md` — and the four skills: overlay re-stage from the public copies (public is now the newer text).
- **`statusgen reconcile --backfill --root . --repo <owner/name> --report`** — produces `docs/streams/board-drift-<date>.md`: one row per brief where `hand-said ≠ derived`, with the witness (or "no PR carries a trailer; last hand edit <sha>"). Committed in the same PR so the reviewer sees the drift before the generated table replaces the old one.

facts:
- Historical PRs have no trailer. `--backfill` uses a DECLARED, reviewable fallback for
  history only: a merged PR whose branch name or body contains the brief id in the
  `<stream>/<NN>` or `<stream>-<NN>` form. Every backfilled edge is listed in the report
  with its source. The scheduled reconcile job runs trailer-only (no `--backfill`); the
  fallback stays an explicit opt-in flag (see Task 4).
- Rows where hand-said `implemented`/`verified`/`done` and no PR is found are NOT demoted
  silently: they render `unknown (no witness — hand-asserted <state> at <sha>)` until a
  human either links the PR (adds the trailer to the merged PR's body via edit, which the
  next reconcile reads) or accepts the demotion in the drift PR.
- Workflow-file pushes in every repo are the human's; each rollout PR is opened by the
  worker with the workflow change included and the human pushes that file.
- Rate limits: one rollout PR at a time per repo; the reconcile step's single list call
  per run is the budget.

## Ground rules
- NEVER git push workflow files / trigger workflows; open the PR with the change and
  name the workflow file in the body for the human.
- Stop at `implemented` per repo; the board itself proves the rest.
- Never resolve a drift row by editing a table; link or accept, in the report.

## Task
1. **This repo — DONE.** Delivered by the merged PRs the `delivery:` list cites: the
   statusgen fixes that unblocked the migration (#735), the brief-v1 → brief-v2 migration
   (#736), `reconcile --backfill --report` and this repo's drift report (#945), the
   `reconcile --apply` write arm (#1010), the brief-v2/trailer join repair and this
   repo's rollout slice (#1194), and the retirement of the stale staged reconcile patch
   once the scheduled reconcile job was live in the board workflow (#5037). This repo is
   the source of the tooling, so it carries no root `.assay-versions` to re-pin; the
   `upgrade-assay --to v1.0.0` step of the original sequence does not apply here.
2. **Each consumer repo — MOVED OUT.** Each consumer repo's rollout (pin, regen-workflow
   reconcile step, migration, backfill report) belongs to that repo's own board, as its
   own brief and PR. This brief keeps this repo only.
3. **Overlay re-stage — RETIRED, not applicable.** The four desk skills and the brief spec
   docs have one canonical home, in this repo; the consumer-side overlay mirror they would
   have been re-staged into was retired, so there is nothing to re-stage.
4. **`--backfill` fallback — KEPT (recorded decision).** The brief offered two endings:
   remove the historical fallback flag, or leave it refusing. Neither: the flag stays as
   an explicit, declared opt-in, because the desk-side reconcile verb calls
   `statusgen reconcile --backfill --apply` (`tools/desk/cmd/deskreconcile/reconcile.go`,
   the `sgArgs` line in step 3). Removing or refusing the flag would break that caller. The
   default path stays trailer-only: the scheduled reconcile job in the board workflow runs
   without `--backfill`, and its job tests pin that.

## Verify (executable — no prose-only DoD items)
Every row runs from the repository root on `main`.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `ls docs/streams/board-drift-*.md \| wc -l` | a number ≥ 1 (this repo's committed drift reports) |
| 2 | check | `statusgen --root . --lint 2>&1 \| awk '/ungenerated board/{n++} END{print n+0}'` | `0` (no stream board in this repo is left ungenerated) |
| 3 | check:ci | `grep -c '^  schedule:' .github/workflows/assay-statusgen.yml` | `1` (the board workflow carries the scheduled reconcile trigger) |
| 4 | check:ci | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestReconcileJobStatusgenArgs$' . > "$out" && grep -q -- '--- PASS: TestReconcileJobStatusgenArgs ' "$out")` | exit 0; named PASS (the scheduled job's run blocks, extracted from the workflow, call `reconcile --apply` with no `--backfill`) |
| 5 | check:ci | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^(TestApplyReconcileWrites_TodoToImplemented\|TestApplyReconcileWrites_DoneRowImmutable\|TestApplyReconcileWrites_NeverTouchesHumanStamp)$' . > "$out" && grep -c -- '^--- PASS: TestApplyReconcileWrites_' "$out")` | `3` (top-level PASS lines only), the three named cases: (a) a fixture `todo` row with a merged-PR witness becomes `implemented` under `reconcile --backfill --apply`; (b) a `done`/`verified` row is left byte-for-byte untouched even when fed a fresh `implemented` witness; (c) a row already carrying a `human:<name>` stamp (in Reviewed) is left untouched — the write never touches anything but the Status cell |
| 6 | check:ci +flow | `grep -c 'fs.Bool("backfill"' statusgen/reconcile.go && grep -c '"reconcile", "--backfill", "--apply"' tools/desk/cmd/deskreconcile/reconcile.go` | `1` then `1` (Task 4, both sides of the flag contract: statusgen still declares `--backfill`, and the reconcile verb still passes it) |
| 7 | check:ci | `go -C tools/desk test ./cmd/deskreconcile/ -count=1 -timeout 300s` | `ok` (the reconcile verb that depends on the kept flag) |
| 8 | check:ci | `git ls-files '*reconcile.patch' \| wc -l` | `0` (the stale staged reconcile patch stays retired) |
| 9 | check +dereference | `for n in 735 736 945 1010 1194 5037; do gh pr view $n -R medici-finance/assay --json state --jq .state; done \| sort \| uniq -c` | `6 MERGED` (every PR that Task 1 and `delivery:` cite resolves to a merged PR) |
| 10 | check | `statusgen --consumers --brief derived-board/07` | exit 0 (after the merge it pins itself to the delivery the single `Brief:` trailer commit names); 0 disproved; the two out-of-scope entries stay UNCHECKED and reviewer-owned |

## Evidence
<!-- appended at implementation time -->

### Implementer run — `--apply` closes the report-only wiring gap — 2026-09-13 implementer, branch off main

**What this run covers.** `reconcile --backfill [--report]` was READ-ONLY: it could derive
a cell and report drift, but nothing wrote the derived value back into a stream README's
Status column — the house's own board-regen job had expected `--backfill` to actually flip
stale rows and found it never does (#945 tracks this brief; the gap itself surfaced from
that rollout work). This run adds `--apply` (`statusgen/reconcileapply.go`,
`statusgen/reconcile.go`): it requires `--backfill`, writes ONLY the witnessed
todo/in-progress → implemented transition (a real `Brief:` trailer or the declared
backfill branch/body match — never the backfill's no-PR hand-said `unknown` shape, never
verified/done), and never touches Verified/Reviewed or any `human:<name>` sign-off stamp.

**Fail-first.** `statusgen/reconcileapply_test.go` was written against the pre-change tree
first: with `reconcileapply.go` removed and `reconcile.go`'s `--apply` wiring reverted, the
suite fails to compile (`undefined: applyReconcileWrites`, `undefined: writeStatusCell`) —
the capability plainly does not exist. Restoring the change turns every case green,
including Verify row 9's three named assertions above.

**Environment.** Own worktree off `refs/remotes/origin/main`; offline, no network reads.
`go test ./... -run 'TestApplyReconcileWrites|TestWriteStatusCell'` (run inside the
`statusgen/` module), `gofmt -l`, and `go vet ./...` all clean. The pre-existing
`reconcile`/`backfill`/`readmetable` suites were re-run bounded and remain green (no
regression).

This is an IMPLEMENTER run, not a verdict — the frontmatter gate is model, and the flip is
the reviewer/verify-gate's per the Review section below.

### Implementer run — this repo's slice (Task 1) — 2026-10-30, branch off main @ 57285f1a9

**What this run covers.** Task 1 only, for this repo. Tasks 2-4 (each consumer repo,
the private toolkit's overlay re-stage, retiring the historical fallback after every repo
merges) are other checkouts and other PRs and remain owed. This repo is the SOURCE of the
tooling, so it carries no root `.assay-versions` to re-pin (the pin files here are adopter
fixtures); its workflow step and its drift reports already existed. What was wrong: the
stream README still described the reconcile step as "staged, not applied" and the staged
`assay-statusgen.reconcile.patch` was still committed, although the hourly `reconcile` job
(trailer-only, read-scoped token, `schedule`) was promoted into
`.github/workflows/assay-statusgen.yml` by a human push. Applying that patch now would add
a second `--backfill` reconcile to the push-time job. This run retires the patch and fixes
the README paragraph. Nothing in the generated Briefs table was hand-edited; this brief's
row is derived from the PRs carrying its `Brief:` trailer. This PR is declared a partial delivery in the `delivery:` list
above, so the row stays in-progress while Tasks 2-4 are owed.

**Verify rows, run offline against the head above.**

| # | Result |
|---|--------|
| 1 | could-not-check as written: no root `.assay-versions` in the source repo (not an adopter) |
| 2 | `statusgen --root . --lint` (v1.0.67): zero `ungenerated board` NOTICEs; one PROBLEM, `desktools-v2/65: ... --acceptance-forge is required` -- a could-not-check of human authority, because CI runs lint with `--acceptance-forge` and this offline run cannot reach the forge. Not a pass |
| 3 | 36 of 43 stream dirs carry `board: generated`; the other seven (`decisions`, `deploys`, `issue-flow`, `verify-outcomes`, `witnesses`, `findings`, `requirements`) hold no stream README or no briefs table, so the row's equality does not hold as written |
| 4 | `docs/streams/board-drift-2026-09-12.md` (70 table lines) and `board-drift-2026-09-16.md` (75) exist; a fresh report needs the forge, could-not-check offline |
| 5, 6 | could-not-check: need an authenticated forge read; not run |
| 7 | could-not-check: the other four stream-bearing repos are other checkouts, outside this run |
| 8 | not applicable: this is the public repo, not the private toolkit |
| 9 | `go test ./statusgen/ -run TestApplyReconcileWrites -v` (run in `statusgen/`): all five tests PASS, including the todo-to-implemented, done/verified-immutable and human-stamp-untouched cases |

**Fail-first.** Not applicable: this diff carries no test, only docs, a changelog fragment and
the removal of a stale patch file; the Verify-9 suite is unchanged and was re-run green.

**Owed to a human.** Rows 5-6 need the forge; rows 1, 7 and Tasks 2-4 need the other
checkouts. The brief therefore keeps its partial `delivery:` entries.

### Re-scope run (version 2) — 2026-11-17, branch off main @ 999315147

**What this run covers.** A re-scope, not new code. Task 1 is recorded done against the
six merged PRs in `delivery:` (each confirmed merged on the forge before this edit); the
consumer rollout moves to each consumer repo's own board; the overlay re-stage is retired
as not applicable; and the `--backfill` decision is recorded as KEEP (Task 4). The Verify
table was rewritten: the version-1 rows checked a `v1.0.0` pin this repo does not carry,
other checkouts, and a consumer-side copy, none of which this repo's scope delivers. The
`delivery:` entries are append-only history and are left as they were; their "Task 2-4
remain owed" notes describe version 1's scope. The two Evidence runs above use version 1's
row numbers (their row 9 is row 5 below).

**Verify rows, run against the head above** (statusgen v1.0.87, offline except where noted).

| # | Result |
|---|--------|
| 1 | `2` — `board-drift-2026-09-12.md` and `board-drift-2026-09-16.md` |
| 2 | `0` ungenerated-board notices (the full lint itself exits 1 on three pre-existing PROBLEMs in other streams, each `could-not-check human authority: --acceptance-forge is required`; none in this stream) |
| 3 | `1` |
| 4 | exit 0; `--- PASS: TestReconcileJobStatusgenArgs` |
| 5 | `3` (the full `TestApplyReconcileWrites_*` family, five tests, also passes) |
| 6 | `1`, `1` |
| 7 | `ok` |
| 8 | `0` |
| 9 | `6 MERGED` (authenticated forge read) |
| 10 | exit 0; `0 corroborated, 0 disproved, 2 unchecked` |

## Review
Gate: model. Reviewer records verdict + date via the generated board.
Reviewer question: open the drift report and pick three rows at random — does each
witness link resolve to the PR it claims?
