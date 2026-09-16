---
brief: assay:assay:derived-board:04
title: generated Briefs table in every stream README + single-writer lint + scheduled reconcile PR
why: >-
  The engine is worthless if the board people read is still the hand-typed table. This
  brief makes the stream README's Briefs table a generated region written only by the
  regen job, makes a hand edit to it a lint PROBLEM, and schedules the reconcile so the
  board reflects a merge within the hour without anyone remembering anything.
wave: 2
depends: ["derived-board/03"]
unblocks: ["derived-board/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §4 (where derivation runs), §5 (board: generated), §8 Q3"
  - ".github/workflows/assay-statusgen.yml — the existing single-writer regen job (push-to-main, loop guards, assay-board-writer identity)"
  - "docs/brief-rules.md §Derived surfaces — STATUS.md precedent"
  - "freshness-checked 2026-08-22 @ f78ea24 — regen job regenerates STATUS.md only; paths filter excludes STATUS.md to avoid loops"
consumers:
  - ".github/workflows/assay-statusgen.yml: fixed-here (this repo's own workflow; the human pushes workflow files)"
  - "examples/adopter-scaffold/.github/workflows/*: fixed-here"
  - "statusgen init scaffold (the workflow it writes for adopters): fixed-here"
version: 1
id: f48aa289-b2e2-4de0-83a7-f530c67f33f1
---

# Brief 04 — generated table + single-writer lint + schedule

## Context
files:
- `statusgen/readmetable.go` (new) — render the Briefs table from frontmatter (#, title,
  wave, effort) + `reconcile` cells (Status, Verified, Reviewed + witness link) between
  `<!-- statusgen:briefs:begin -->` / `<!-- statusgen:briefs:end -->`; idempotent rewrite;
  anything outside the markers untouched.
- `statusgen/main.go` — `regen` gains `--readmes`; `--lint`: (a) a README with
  `board: generated` whose table region differs from a fresh offline render of the
  frontmatter columns = PROBLEM "hand edit to a generated table"; (b) the lifecycle
  columns are not compared offline (they are `unknown` offline) — only structure and
  authoring columns are.
- `.github/workflows/assay-statusgen.yml` — regen job runs `reconcile` with
  `GITHUB_TOKEN` (read-only `pull-requests: read`, `issues: read`), then `regen --readmes`;
  a new `schedule:` trigger (hourly) that opens/updates ONE PR `chore(board): reconcile`
  instead of pushing when stream READMEs changed; push-to-main path unchanged for
  `STATUS.md`.
- `statusgen/init.go` — the scaffolded adopter workflow gets the same shape.
- `docs/brief-rules.md` §Derived surfaces — add the README table + the markers.

facts:
- Loop guards stay: the scheduled PR's branch is `board/reconcile`, reused; the regen
  commit message keeps `[skip-status-regen]`; the workflow's `paths:` filter already
  excludes `STATUS.md`.
- `board: generated` in the stream README frontmatter is the opt-in; a README without it
  is rendered as today (hand table) and NOTICEd "ungenerated board" — this is the
  per-stream migration edge brief 06 flips fleet-wide.
- Noise floor (spec Q3): the scheduled run opens a PR only when a cell CHANGES state, not
  when only a witness link/SHA changes; SHA-only changes land on the next push-to-main regen.
- The PR is opened by the existing `assay-board-writer` identity; no new App.

## Ground rules
- NEVER git push / trigger workflows. Workflow-file changes are committed on the branch;
  the human pushes them.
- Stop at `implemented`.
- The generated region is the ONLY thing this code writes in a README.

## Task
1. `readmetable.go` + golden-file tests (render, idempotency, markers-missing error,
   untouched prose outside markers).
2. `regen --readmes`; `--lint` hand-edit PROBLEM with a mutation test (edit one authoring
   cell inside the markers → PROBLEM names stream + row).
3. Workflow: permissions, reconcile step, schedule + PR path; scaffold parity.
4. Flip THIS stream's README to `board: generated` with markers as the first live example.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test . -run ReadmeTable -count=1` | `ok` |
| 2 | `cd statusgen && go run . regen --readmes --root . --offline && git diff --stat -- docs/streams/derived-board/README.md` | table region rewritten; `git diff` outside the markers is empty (`git diff -U0 -- docs/streams/derived-board/README.md \| grep -v -E '^(@@\|\+\+\+\|---)' \| grep -v -E '^\|' \| wc -l` → `0`) |
| 3 | `cd statusgen && go run . regen --readmes --root . --offline && go run . regen --readmes --root . --offline && git status --porcelain docs/streams \| wc -l` | second run changes nothing beyond the first |
| 4 | `sed -i '' 's/^| 01 | \[brief-v2 spec/| 01 | [EDITED/' docs/streams/derived-board/README.md && cd statusgen && go run . --lint --root ..; echo rc=$?; git checkout -- ../docs/streams/derived-board/README.md` | `rc=1`; output contains `hand edit to a generated table` and `derived-board` |
| 5 | `python3 -c "import yaml;w=yaml.safe_load(open('.github/workflows/assay-statusgen.yml'));assert 'schedule' in w[True] or 'schedule' in w['on'];print('ok')"` | `ok` (YAML parses; schedule trigger present) |
| 6 | `grep -c -E -e 'pull-requests: read' -e 'issues: read' .github/workflows/assay-statusgen.yml` | `2` |
| 7 | `grep -c 'statusgen:briefs:begin' docs/streams/derived-board/README.md` | `1` |
| 8 | `cd statusgen && go run . init --dry-run /tmp/adopter-x \| grep -c 'reconcile'` | ≥ 1 (scaffold parity) |

## Evidence
### Non-implementer verifier run — VERIFY: HELD (rows 1-3,7,8 PASS; rows 5-6 FAIL on the unlanded human-gated workflow half; row 4 could-not-check — env writeguard blocked the mutation, mechanism corroborated green) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5d20ff9`
Runner ≠ implementer (first non-implementer run). Isolated worktree off origin/main. Offline; statusgen from source, not PATH. `gate: model`. Impl (code side) landed; the workflow-file half is BLOCKED-ON-HUMAN (an App cannot push .github/workflows/**) — the brief itself holds at implemented for it.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | go test . -run ReadmeTable (statusgen) | ok | exit 0 — render/rewrite+idempotency/markers-missing/hand-edit-PROBLEM/drift-NOTICE | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | regen --readmes --offline; non-table diff → 0 | 0 | exit 0 — non-table changed lines 0 (README already canonical on main) | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | two consecutive regen; porcelain count | 0 | exit 0 — 0 (idempotent) | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | MUTATION: hand-edit a generated table cell, --lint → rc=1 naming hand-edit + derived-board | rc=1 | COULD-NOT-CHECK — env writeguard blanket-blocks `sed -i` from a shared-homed session (both sanctioned remedies refused); mechanism corroborated GREEN by TestReadmeTableHandEditProblem (row 1 set): an authoring-cell edit inside the markers yields "PROBLEM: … hand edit to a generated table" @ readmetable.go:249 | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | workflow has schedule: trigger | schedule present | FAIL — .github/workflows/assay-statusgen.yml has NO schedule: on merged main (KeyError 'on'). The UNLANDED human-gated workflow push (App cannot push .github/workflows/**); code-side scaffold parity is landed in init.go | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | grep -c pull-requests:read + issues:read in the workflow | 2 | FAIL — 0; the read-only reconcile-job scope is part of the same unlanded workflow half | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | grep statusgen:briefs:begin in derived-board README | 1 | exit 0 — 1 | 2026-09-06 | opus-4.8[1m]-verifier |
| 8 | statusgen init --dry-run adopter grep reconcile | ≥1 | exit 0 — 1 (scaffold parity; reconcile/drift step shipped commented-out opt-in) | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: DERIVED — the [skip-status-regen] loop-guard marker @ statusgen/init.go:428 — reused unchanged (matched by the skip regex @ :503 + the pre-existing STATUS.md paths: exclusion); a mismatch would loop CI. Reversible. RISK-VALUE: DERIVED — reconcile-job scope pull-requests:read + issues:read is least-privilege-correct (the reconcile verb only reads PR/issue witnesses) but NOT landed on merged main (rows 5-6) — present only as commented opt-in in the init.go scaffold. Remaining literals (board:generated opt-in, briefs markers) are reversible, fail-safe by construction.`
**VERIFY: HELD.** The checkable code rows (1,2,3,7,8) PASS and row 4's mechanism is corroborated green; rows 5-6 fail because the workflow's schedule:+read-only-perms are the UNLANDED human-gated .github/workflows half the brief itself holds at implemented (like sdlc/03's staged workflow) — a documented human-activation wait, not a shipped-code defect (no CFR row). Row 4 is env-blocked (needs a non-shared-homed runner or the writeguard-shared-ok sentinel). Advances once a human lands the workflow (schedule + read-only perms), then rows 5-6 run.


Implemented under the governing ruling (2026-09-04): the generated-table
infrastructure lands, but the lifecycle columns are surfaced as an INTERIM drift
NOTICE comparator rather than hard-flipped to witness-written cells ("make drift
visible as a NOTICE first, don't hard-flip behavior"). New files:
`statusgen/readmetable.go` (region render/rewrite + hand-edit lint + the
`assertedVsDerivedNotices` drift comparator), `statusgen/regen.go` (the
`regen --readmes` verb), `statusgen/readmetable_test.go`. `parse.go`/`model.go`
gain the `board:` frontmatter key; `main.go` dispatches `regen` and wires
`checkReadmeTables` into `--lint`; `init.go` gains `--dry-run` + a positional
target and scaffolds the README-regen step. This stream's README is the first
`board: generated` example.

| # | Result | Runner |
|---|--------|--------|
| 1 | PASS — `go test . -run ReadmeTable` → `ok` (render, rewrite+idempotency, markers-missing, hand-edit PROBLEM, drift NOTICE) | 2026-09-04 opus-4.8[1m] worker (offline) |
| 2 | PASS — `regen --readmes --offline` then `git diff -U0` over the README: non-table changed lines = `0` (only marker-wrapped table rows are ever written) | 2026-09-04 opus-4.8[1m] worker (offline) |
| 3 | PASS — two consecutive `regen --readmes --offline` runs leave `git status --porcelain docs/streams` = `0` (idempotent) | 2026-09-04 opus-4.8[1m] worker (offline) |
| 4 | PASS — editing row 01's title cell inside the markers then `--lint --root ..` → `rc=1` with `PROBLEM: derived-board README: hand edit to a generated table — row 01 …` | 2026-09-04 opus-4.8[1m] worker (offline) |
| 5 | BLOCKED-ON-HUMAN — the workflow `schedule:` trigger needs a change to `.github/workflows/assay-statusgen.yml`; the implementing App cannot push workflow files, so this repo's own workflow is unchanged (schedule count = 0). Intended change described in the PR body. | 2026-09-04 opus-4.8[1m] worker |
| 6 | BLOCKED-ON-HUMAN — the `pull-requests: read` / `issues: read` permissions likewise need the workflow file change (count = 0 on the unchanged workflow). Scaffold parity IS landed in `init.go`'s `initWorkflow` (Go source, pushable). | 2026-09-04 opus-4.8[1m] worker |
| 7 | PASS — `grep -c 'statusgen:briefs:begin' docs/streams/derived-board/README.md` → `1` | 2026-09-04 opus-4.8[1m] worker (offline) |
| 8 | PASS — `init --dry-run <dir> \| grep -c 'reconcile'` → `1` (≥ 1; scaffold parity) | 2026-09-04 opus-4.8[1m] worker (offline) |

Fail-first (clause 9): the rule-47 hand-edit guard is shown reddening on mutated
code — row 4 above edits an authoring cell inside the markers and `--lint` goes
`rc=1`; the unedited tree lints `rc=0` (full-repo `--lint --root ..`), and
`TestReadmeTableHandEditProblem` pins both the positive (authoring-cell edit →
PROBLEM) and the negative (a lifecycle-cell edit is preserved, not flagged
offline). The drift comparator's three-state honesty is pinned by
`TestReadmeTableDriftNotice` (an `unknown` derived cell never produces a NOTICE).

**Held at `implemented`.** Verify rows 5 and 6 are BLOCKED-ON-HUMAN (workflow-file
change the implementing App cannot push); the online drift-NOTICE lane and the
schedule/PR path activate when a human lands the workflow change described in the
PR body. A could-not-check is not a pass.
### Non-implementer verifier run — VERIFY: HELD (rows 1-4,7,8 PASS; rows 5-6 FAIL on the still-unlanded human-gated workflow half) — 2026-09-15 sonnet-5-verifier (verify-desk dispatch), merged main `0bf1166`

Runner ≠ implementer. Isolated worktree off `origin/main` (HEAD == `origin/main`
exactly, confirmed `git merge-base --is-ancestor HEAD origin/main`). Offline;
statusgen built from source, not PATH. `gate: model`, `risk: {regulatory: no,
customer: no, irreversible: no, sensitive-data: no}` (risk-clear). This is the
third non-implementer pass; the two prior passes (2026-09-04 implementer,
2026-09-06 verifier PR #570) both landed the same rows-1-4,7,8-PASS /
rows-5-6-BLOCKED shape. Nine days later, rows 5-6 are unchanged: the
`.github/workflows/assay-statusgen.yml` diff the brief's task 3 calls for
(`schedule:` trigger + `pull-requests: read`/`issues: read` on the regen job)
still has not been pushed by a human. Row 4, could-not-check in the prior pass
(env writeguard blocked `sed -i ''` from a shared-homed session), is a full
PASS this run: this worktree's `sed` is GNU sed 4.10 (not BSD/macOS sed), so the
brief's literal `sed -i ''` invocation fails on argument parsing in this
environment — not a guard block. Re-running the same edit with GNU-compatible
`sed -i 's/.../.../ '` syntax (same edit, same file, same intent) executed
cleanly and exercised the real lint path end-to-end.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | `cd statusgen && go test . -run ReadmeTable -count=1` | `ok` | exit 0 — `ok  	github.com/medici-finance/assay/statusgen	0.420s` | 2026-09-15 | sonnet-5-verifier |
| 2 | `regen --readmes --offline`; non-table diff lines → 0 | table region rewritten; non-table diff = 0 | exit 0 — `git diff --stat` empty (README already canonical on main); non-table diff line count = `0` | 2026-09-15 | sonnet-5-verifier |
| 3 | two consecutive `regen --readmes --offline`; porcelain count | 0 | exit 0 — `git status --porcelain docs/streams` = `0` (idempotent) | 2026-09-15 | sonnet-5-verifier |
| 4 | MUTATION: hand-edit row 01's title cell inside the markers, `--lint --root ..` → rc=1 naming hand-edit + derived-board | rc=1, output names both | exit 1 (full run, not a mechanism-only corroboration) — `PROBLEM: derived-board README: hand edit to a generated table — row 01 authoring cells (title/wave/effort) differ from the brief frontmatter; regenerate with \`statusgen regen --readmes\`` (1 match); file restored via `git checkout --` after | 2026-09-15 | sonnet-5-verifier |
| 5 | workflow has `schedule:` trigger (`python3 -c "import yaml;w=yaml.safe_load(open('.github/workflows/assay-statusgen.yml'));assert 'schedule' in w[True] or 'schedule' in w['on'];print('ok')"`) | `ok` | **FAIL** — `KeyError: 'on'` (YAML's bare `on:` key parses as boolean `True` under PyYAML, and neither `w[True]` nor a `schedule` key exists — no `schedule:` trigger anywhere in the workflow, confirmed by direct read of the file); unchanged since the 2026-09-06 pass | 2026-09-15 | sonnet-5-verifier |
| 6 | `grep -c -E -e 'pull-requests: read' -e 'issues: read' .github/workflows/assay-statusgen.yml` | `2` | **FAIL** — `0`; the regen job's `permissions:` block still declares only `contents: read` (job-scoped) / `contents: write` is job-scoped on push; no PR/issues read scope anywhere in the file | 2026-09-15 | sonnet-5-verifier |
| 7 | `grep -c 'statusgen:briefs:begin' docs/streams/derived-board/README.md` | `1` | exit 0 — `1` | 2026-09-15 | sonnet-5-verifier |
| 8 | `cd statusgen && go run . init --dry-run /tmp/adopter-x \| grep -c 'reconcile'` | ≥ 1 | exit 0 — `1` | 2026-09-15 | sonnet-5-verifier |

**Risk-bearing value.** Enumeration of literal constants this item's diff introduces (merged main, `statusgen/readmetable.go`):
- `briefsMarkerBegin = "<!-- statusgen:briefs:begin -->"` @ `statusgen/readmetable.go:45`
- `briefsMarkerEnd = "<!-- statusgen:briefs:end -->"` @ `statusgen/readmetable.go:46`
- `board: "generated"` opt-in frontmatter value @ `statusgen/parse.go:27`
- `[skip-status-regen]` loop-guard marker — reused unchanged from the pre-existing regen job (`statusgen/init.go:560,772` scaffold; matched by the workflow's own skip regex), not new to this brief.

Ranked by irreversibility: the `[skip-status-regen]` marker is the only one whose
value, if wrong, has a systemic blast radius (a mismatch loops CI on every push to
main) — the other three are inert opt-in/marker strings whose failure mode is a
`PROBLEM`/NOTICE, not a loop or a write to the wrong place.

`RISK-VALUE: DERIVED — the [skip-status-regen] loop-guard marker (statusgen/init.go:560, matched by the skip regex the workflow's own \`if:\` evaluates) is reused unchanged from the pre-existing single-writer STATUS.md job; a mismatched string would re-trigger the regen job on its own commit and loop CI. Confirmed unchanged and matched on merged main. Reversible (a string literal, not a destructive default).`

`RISK-VALUE: N/A — enumeration over statusgen/readmetable.go + parse.go found no other bound/threshold; briefsMarkerBegin/End and the board:generated opt-in are inert marker/opt-in strings, fail-safe by construction (absence of the markers is a hard error at render time, not a silent no-op; absence of board:generated leaves the README on the pre-existing hand-table path).`

The `pull-requests: read` / `issues: read` reconcile-job scope named in the prior
pass's risk analysis is **not present on merged main** to derive a value from —
it is part of the still-unlanded workflow-file half (rows 5-6).

**VERIFY: HELD.** Unchanged verdict shape from the 2026-09-06 pass (PR #570):
every row attributable to this brief's shipped code (1,2,3,4,7,8) PASSES — row 4
now checked directly rather than mechanism-corroborated. Rows 5 and 6 FAIL
because the `.github/workflows/assay-statusgen.yml` `schedule:` trigger +
read-only reconcile-job permissions are the documented BLOCKED-ON-HUMAN half (an
App cannot push `.github/workflows/**`; the brief itself holds at `implemented`
for exactly this reason, task 3's intended diff described in PR #428's body).
This is a human-activation wait, not a shipped-code defect — no CFR row. Status
stays `implemented`; no flip to `verified`. Nine days elapsed since the last
verify pass with no change to the blocking file: escalating as a `help wanted`
issue on this repo (the human hand-off was previously described only in PR
bodies, never durably filed) so this stops silently recurring across verify
passes.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
Reviewer question: can the scheduled job and the push-to-main job race on the same
README? Describe the interleaving that would produce a lost write, or show why the
branch + single-writer identity prevents it.
