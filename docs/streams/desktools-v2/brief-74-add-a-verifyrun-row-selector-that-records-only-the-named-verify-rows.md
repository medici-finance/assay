---
brief: assay:assay:desktools-v2:74
title: Add a verifyrun row selector that records only the named Verify rows
why: "A brief whose Verify table mixes container-credited rows with host-only rows needs two verifyrun runs, but each run records every row, so whichever table lands last masks the other run's passes and the brief can never close; a row selector lets each run record only the rows it can actually witness."
wave: 1
depends: []
unblocks: ["desktools-v2/71"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [4247]
schema: brief-v2
version: 1
authored: 2026-10-24 by statusgen newbrief
sources:
  - 'desktools-v2: Add a verifyrun row selector that records only the named Verify rows'
  - "driver ruling, 2026-10-23, option A on #4247 (comment URL in Context): add a verifyrun row selector (--rows) that records only the named checks; last-wins and rule 45 unchanged"
  - "analysis on #4247 (comment URL in Context): last-wins masking between a container run and a host run; no row selector; parseEvidenceRows ignores code fences"
  - docs/brief-rules.md (rule 45)
  - freshness-checked 2026-10-23 @ 3e24567bbc4e1c53c508e4f74d8ad04d3364d494 (origin/main)
exec-tier: strong
exec-tier-why: "(b) and (c): the selector must compose with the container admission bracket, the protected acceptance transition and the closure audit without weakening any of them, and a selector that silently runs or writes the wrong rows still produces a well-formed witness table."
domain: complicated
outcome: none
consumers:
  - "statusgen/verifyrun.go, statusgen/verifyincontainer.go and their tests: follow-up desktools-v2/74"
  - "docs/brief-rules.md rule 45 (a mixed-assembly paragraph only; the credit rule is unchanged): follow-up desktools-v2/74"
  - "plugins/assay/skills/verify-desk/SKILL.md and the generated Cursor mirror: follow-up desktools-v2/74"
  - "changelog/ (one top-level fragment): follow-up desktools-v2/74"
  - "closure audit last-wins (checkWitnesses) and rule 45 credit: out-of-scope (the #4247 ruling chose the row selector as the remedy and kept last-wins and rule 45 unchanged; the selector changes only which rows a run writes)"
  - "protected-record audit run selection and its single Verification-Table marker rule (statusgen/protectedclosure.go): out-of-scope (unchanged; the selector composes with it, it does not relax it)"
  - "parseEvidenceRows reading a table inside a code fence (statusgen/unrun.go): out-of-scope (a scope choice of this brief, not part of the #4247 ruling, which chose the row selector and kept last-wins and rule 45 unchanged; the fence-aware parser is owned by follow-up issue #4250)"
---

# Brief 74 — Add a verifyrun row selector that records only the named Verify rows

## Context

files: statusgen/verifyrun.go, statusgen/verifyincontainer.go and their tests (new tests in statusgen/verifyrunrows_test.go); statusgen/unrun.go, statusgen/protectedclosure.go, statusgen/containerbracket.go, statusgen/htmlcomment.go (read only); docs/brief-rules.md (rule 45); plugins/assay/skills/verify-desk/SKILL.md and its generated Cursor mirror; changelog/<slug>.md (new, top level)

ruling: https://github.com/medici-finance/assay/issues/4247#issuecomment-6377158517

analysis: https://github.com/medici-finance/assay/issues/4247#issuecomment-6377148170

facts:
- The closure audit keeps the LAST witness per row ID. `checkWitnesses` (`statusgen/verifyrun.go:1535`) loops every Evidence row for each Verify row and overwrites `latest`, "last witness for the row wins" (`:1543`). `verifyrun --check` reaches it through `closureWitnesses` over the whole Evidence section (`runVerifyrunCheck`, `verifyrun.go:2377`; `htmlcomment.go:122`). The closure verbs (`verifyflip.go:299`, `reconcilefold.go:179`) audit only the selected run, the rows before the latest strict PASS verdict (`closureRunWitnesses`, `protectedclosure.go:67`-`:79`).
- verifyrun has no row selector. Its flags are declared at `verifyrun.go:1901`-`:1921` (plus `--acceptance-forge`, `acceptancetransition.go:19`). Every run executes `briefVerifyRows(verify)` in full (`:2227`), writes one row per executed row through `witnessTable` (`:2309`, `:1444`) and, outside container mode, appends the table to the brief's `## Evidence` (`:2368`). Container mode writes it to `--witness <file>` instead, outside the home (`:2049`-`:2056`).
- The masking case. A brief mixing `check:ci` rows (credited from the rule 45 container-netns bracket, desktools-v2/71) with host-only rows that need the linked worktree's git directory (`statusgen --consumers`, `statusgen --lint`) needs two runs. The container run records the host-only rows as fail, since the git directory is not mounted. The host run records the `check:ci` rows as could-not-run or as an uncredited host result. Under last-wins, whichever table lands second masks the other run's passes, so `verifyrun --check` never reports every row pass. Live case: brief desktools-v2/71's Evidence on #4114.
- Row IDs. `briefVerifyRows` (`verifyrun.go:1258`) takes the ID from the `#` cell and falls back to the row ordinal only when the cell is empty or the table has no `#` column. `witnessHeader` (`verifyrun.go:151`) starts with a `#` column, so a verifyrun-written table keys its rows by the Verify row IDs.
- Hand-pasted tables are not a workaround. `parseEvidenceRows` (`statusgen/unrun.go:212`) reads every line starting with `|`, including lines inside a code fence. It resets its column map on any non-table line (`:218`-`:222`) and numbers rows by ordinal when no `#` header was seen (`:247`-`:253`). Header detection also ignores fences. So a host table pasted in a `text` fence is read as a table either way: with its header kept, its rows are keyed by their `#` cells and credited; without it, they are keyed by ordinal, so they take over rows 1..k's IDs, or are credited when the named rows are 1..k. Either way the fence is no barrier (see #4250).
- Protected records. `--acceptance-record` requires host execution, `--acceptance-forge` and same-repository admission (`verifyrun.go:2139`-`:2142`). Its run set is the protected plan's original rows plus the author-added rows (`:2227`-`:2238`), and the table is headed by the transition descriptor (`:2310`-`:2312`). The protected audit refuses more than one `Verification-Table` marker in the current run (`protectedclosure.go:118`).
- The container bracket. The close step digests the span from the nearest witness header above the `Admission-Bracket:` line to that line (`witnessSpanDigest`, `containerbracket.go:688`). `--bracket-digest` reports a `sandbox=container-netns` row outside every bracket's span as never credited (`containerRowsOutsideSpans`, called at `verifyrun.go:1990`). Nothing in the bracket requires the span to hold every Verify row.

design-fit:
  owner: statusgen verifyrun (the witness writer); the closure audit stays the owner of what a run proves
  contract: "S-evidence-coverage (docs/contracts.md): unchanged; coverage is still judged per Verify row from the witnesses present, and a selected run only supplies fewer witnesses"
  retires: ["hand-pasted host witness tables in code fences as the way to land a partial run", "the two-run ordering trap where the last table to land masks the other run's passes"]
  weight: "flags +1 (`verifyrun --rows`); refusals +1 family (unknown, empty, duplicate, ambiguous or unnumbered selection, and --rows with a mode that runs no rows); rule-text lines about +12 across brief-rules.md and verify-desk; no new verb, module or audit rule"
  why-add: "The ruling chose a selector over an audit change. A could-not-run-does-not-supersede audit rule was considered and rejected by the ruling, because it would let an older pass outlive a newer failing witness. The run is the only place that knows which rows it can witness."

single-point-of-failure: the closure audit (`verifyrun --check` and the closure verbs) reading the whole Verify table, which still reports every unselected, unwitnessed row as could-not-run. It is backed by the selector's own refusals, which stop a bad selection before any row runs.

## Ground rules

Normal worker admission, one isolated worktree and one draft PR. Tests use temporary git repositories, fixture briefs and the existing seams (the fake forge of `acceptancetransition_test.go`, the receipt and bracket fixtures of `containerbracket_test.go`), with no network. Do not change `checkWitnesses`, `closureRunWitnesses`, the protected audit or any bracket, admission or credit check (last-wins and rule 45 are ruled unchanged on #4247). Do not change `parseEvidenceRows` either: that is this brief's scope choice, not a ruling, and the fence-aware parser is owned by #4250. If a Verify row can only go green by changing one of them, stop and report NEEDS_CONTEXT. Never edit another brief's Evidence. Stop at implemented.

## Task

1. Add `--rows <ids>` to `statusgen verifyrun`: a comma-separated list of Verify row IDs as they appear in the brief's `#` column, surrounding spaces ignored. When given, verifyrun executes ONLY the named rows, in table order, and the witness table it writes holds only those rows, in the standard headed form (`witnessHeader`, or the fail-first header), so each row is keyed by its `#` ID. On stdout, every unselected row is reported as `row <id>: not selected`. Unselected rows never appear in the table. Without `--rows`, behaviour is byte-identical to today.
2. Refusals. Each exits with the usage-error code, runs no row and writes nothing (the brief and any `--witness` file are untouched), under `--dry-run` too:
   - an ID the Verify table does not carry;
   - an empty list, or an empty element (`--rows ''`, `--rows ,`, `--rows 2,`);
   - a duplicate ID in the list;
   - an ID that two Verify rows share;
   - a Verify table with no `#` column (an ordinal ID is not a selector);
   - `--rows` with `--check`, `--open-bracket`, `--close-bracket` or `--bracket-digest`, which run no rows. The audit always covers the whole table.
   The message names the offending ID or flag and says no row ran.
3. Composition:
   - `--dry-run`: prints the selected-rows table and writes nothing, as today.
   - `--fail-first`: the base run uses the same selected rows.
   - `--in-container`: `runInContainer` passes `--rows` through to the inner run, unchanged.
   - `--acceptance-record` with `--acceptance-forge`: the selection is made from the protected plan's run set (original rows plus author-added rows). An ID outside that set is refused like an unknown ID. A selected author-added row keeps its `author-added` state. The table is headed by the descriptor exactly once, as today.
   - `--sandbox=container-netns`: ALLOWED. The receipt, image-digest, `--witness`, tree-digest and admission checks run exactly as without `--rows`. The witness file holds only the selected rows plus the one `Tree-Digest:`, `Container-Image:` and attestation line, and the bracket closed over it covers only the rows run. No bracket, admission or credit check is weakened or skipped.
4. Docs. In `docs/brief-rules.md` rule 45 and in verify-desk (beside the container bracket procedure), say how to assemble a mixed container-and-host verification:
   - Run the container bracket first (open, container run, close) with `--rows` naming the rows the container can witness.
   - Then run on the host with `--rows` naming only the host-only rows.
   - The two selections are disjoint and together cover the Verify table, so no table carries a witness for the other run's rows, and the last-wins audit is satisfied in either landing order.
   - Both tables land in the same run, before its verdict line.
   - `statusgen verifyrun --check` on the landed brief must report every row pass, and `--bracket-digest` must still report `verdict=match` on the container block.
   - Never hand-paste a witness table, and never put one in a code fence.
   State that last-wins and the rule 45 credit rule are unchanged. Regenerate the Cursor mirror.
5. Add one top-level changelog fragment, `changelog/<slug>.md` (never under `changelog/feat/`), with an `### Added` bullet naming `verifyrun --rows`.
6. Usage text: the `verifyrun` usage (`verifyrunUsage`) documents `--rows`, its refusals and that `--check` always audits the whole table.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && rows_out=$(mktemp) && trap 'rm -f "$rows_out"' 0 && go test -run '^TestVerifyrunRowsSelectsOnlyNamed$' -count=1 -v . > "$rows_out" 2>&1 && grep -q -- '--- PASS: TestVerifyrunRowsSelectsOnlyNamed ' "$rows_out")` | exit 0 and named PASS. Fixture: a brief whose Verify table has rows `1`-`4` in a `#` column, each row's command touching its own marker file. `--rows 2,4` touches only the markers for 2 and 4. The appended table starts with the standard witness header and holds exactly two data rows, whose `#` cells are `2` and `4`. `parseEvidenceRows` on the result keys them `2` and `4`, never `1` and `2`. `verifyrun --check` then reports rows 2 and 4 pass and rows 1 and 3 could-not-run. Stdout names rows 1 and 3 `not selected`. Without `--rows`, the table is byte-identical to the pre-change output for the same fixture. The `--in-container` argument builder carries `--rows 2,4` to the inner run. Mutations, each making the named test fail: (a) ignore the selector at execution so all rows run; (b) write the selected rows without the header, so they key by ordinal; (c) drop `--rows` from the `--in-container` argument list |
| 2 | check:ci +mutation | `(cd statusgen && rows_out=$(mktemp) && trap 'rm -f "$rows_out"' 0 && go test -run '^TestVerifyrunRowsRefusals$' -count=1 -v . > "$rows_out" 2>&1 && grep -q -- '--- PASS: TestVerifyrunRowsRefusals ' "$rows_out")` | exit 0 and named PASS. Each case exits with the usage-error code, touches no marker file and leaves the brief byte-identical, with and without `--dry-run`: `--rows 9` on a four-row table; `--rows ''`; `--rows ,`; `--rows 2,`; `--rows 2,2`; an ID two Verify rows share; a Verify table with no `#` column; `--rows` with `--check`, `--open-bracket`, `--close-bracket` and `--bracket-digest`. The stderr message names the ID or flag. `verifyrun --help` names `--rows`. Mutation: filter unknown IDs out silently instead of refusing; the `--rows 9` case and the `--rows 2,9` case (which would run row 2 alone) then fail the named test |
| 3 | check:ci +flow +mutation +dereference | `(cd statusgen && rows_out=$(mktemp) && trap 'rm -f "$rows_out"' 0 && go test -run '^TestVerifyrunRowsMixedAssembly$' -count=1 -v . > "$rows_out" 2>&1 && grep -q -- '--- PASS: TestVerifyrunRowsMixedAssembly ' "$rows_out")` | exit 0 and named PASS. Fixture: a brief with `check:ci` rows C and host-only rows H, Evidence assembled from verifyrun-written tables only. (a) The assembly exactly as the Task 4 docs state it (container bracket over C with `--rows`, then a host `--rows` run of H), and again with the two tables landed in the reverse order: `verifyrun --check` reports every row pass and exits 0. (b) The container table alone: H could-not-run, non-zero exit. (c) The host table alone: C could-not-run, non-zero exit. (d) A host `--rows` table of H, then an unrestricted container table recording H as fail: H fail, non-zero exit, because last-wins is unchanged. (e) A host table of H hand-pasted, without its header, in a `text` code fence after an unrestricted container table that records H as fail, with the fixture pinned so C holds rows `1`-`3` and H holds rows `4`-`5`: the fenced rows key by ordinal as `1` and `2`, so rows 1 and 2 report could-not-run (the witness command differs from the row), H keeps the container fail, and `--check` exits non-zero. The layout is pinned on purpose: were H rows `1`-`k`, the ordinal keys would line up and the fenced rows would be credited, which is the parser defect #4250 owns, not behaviour this brief asserts. Mutation: change `checkWitnesses` to keep an earlier pass over a later fail or could-not-run witness. Case (d) then passes and the named test must fail, which shows the audit rule is untouched |
| 4 | check:ci | `(cd statusgen && rows_out=$(mktemp) && trap 'rm -f "$rows_out"' 0 && go test -run '^TestVerifyrunRowsContainerBracket$' -count=1 -v . > "$rows_out" 2>&1 && grep -q -- '--- PASS: TestVerifyrunRowsContainerBracket ' "$rows_out")` | exit 0 and named PASS. A `--sandbox=container-netns` run with a valid fixture receipt and `--rows` naming two of four rows writes to `--witness` only those two rows, plus exactly one `Tree-Digest:`, one `Container-Image:` and one attestation line. The close step appends an `Admission-Bracket:` line, and `--bracket-digest` on the closed file reports `verdict=match` and `span-lines=tree-digest:1,container-image:1,attestation:1`. With a host `--rows` table and its attestation line placed above the container block, and again below the bracket, `--bracket-digest` still reports `verdict=match` for the container bracket. With `--rows` present, a missing receipt, a missing image digest, a `--witness` inside the home, an existing `--witness` file and a mounted tree that differs from the receipt each refuse before any row runs, exactly as without `--rows` |
| 5 | check:ci | `(cd statusgen && rows_out=$(mktemp) && trap 'rm -f "$rows_out"' 0 && go test -run '^TestVerifyrunRowsAcceptanceTransition$' -count=1 -v . > "$rows_out" 2>&1 && grep -q -- '--- PASS: TestVerifyrunRowsAcceptanceTransition ' "$rows_out")` | exit 0 and named PASS. With `--acceptance-record` and `--acceptance-forge`, using the existing fake-forge seam: `--rows` naming one original row and one author-added row writes a table headed by the descriptor exactly once, holding only those two rows, with the author-added row in the `author-added` state. An ID outside the protected plan's run set is refused with nothing written. The same selection under `--dry-run` prints the table and writes nothing |
| 6 | check:ci | `cd statusgen && go test -count=1 .` | exit 0; output contains `ok`; the existing verifyrun, sandbox, container-bracket, protected-closure and acceptance-transition tests are unchanged and pass |
| 7 | check:ci | `grep -n -e '--rows' -e 'disjoint' docs/brief-rules.md plugins/assay/skills/verify-desk/SKILL.md` | both files match both patterns: each states the container-bracket-first, host `--rows`-second assembly with disjoint selections covering the table, and that last-wins and the rule 45 credit rule are unchanged |
| 8 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check` | exit 0; no Cursor-mirror drift reported |
| 9 | check | `statusgen --root . --consumers --brief desktools-v2/74 --base $(git merge-base origin/main HEAD)` | pre-merge, on the implementation branch: exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |
| 10 | check | `statusgen --root . --lint` | exit 0; no PROBLEM line names this brief |

Pre-mortem:
- A selector that filters the table but still executes every row (side effects, wall time, a host row run inside the container) is caught by row 1's marker files.
- A selector that writes a bare or ordinal-keyed table, which would credit the wrong rows, is caught by row 1's `parseEvidenceRows` keys.
- A selector that silently drops an unknown or mistyped ID, so a run meant to cover rows 2 and 9 covers only 2, is caught by row 2.
- An implementer who "fixes" the masking problem in the audit instead of the run is caught by row 3's mutation case (d).
- A selector that skips or reorders a container admission check is caught by row 4.
- A selection that escapes the protected plan or duplicates the descriptor is caught by row 5.
- Docs that describe an order-dependent assembly are caught by row 7's `disjoint` pattern together with row 3's either-order case (a).

Whether the docs' wording is clear enough for a verifier to follow without help is for review.

The implementation must create the exact named tests; an empty go-test match is not PASS.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

## Review

Gate: model (from frontmatter; all four risk answers no). The ruling fixed the design: a selector, with last-wins and rule 45 unchanged. Review checks that no admission, bracket, credit or protected-audit check is loosened.
