---
brief: desk-tools/12
title: "`statusgen brief <stream/NN>` — resolve an item key to its file, frontmatter and board row, as JSON"
why: >-
  `verifyloop plan` and the dispatch verbs emit item KEYS — `<stream>/<NN>` — and every
  consumer then re-derives the same three facts by hand: which file the key names (a glob),
  what its `gate:` / `risk:` / `exec-tier:` say (an awk over frontmatter), and what the stream
  README's board row says its status is (a grep). A 24-hour sweep of fifteen desk-role and
  worker session transcripts found one verify loop hand-rolling the glob about 17 times and
  the awk and the grep 7 times each, per brief per cycle. Every one of those is a chance to
  read the wrong file (two briefs sharing a numeric prefix) or the wrong row. `statusgen`
  already parses both the frontmatter and the board table to lint them; printing what it
  parsed for one key is the whole verb.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — `statusgen/main.go` § positional subcommands lists `init, verifyrun, mergecheck, shardcheck, conform, backfill, reconcile, enforcement-status, version`; no subcommand resolves a key to a brief; the only `--brief` flag scopes `--consumers`. `tools/desk/cmd/verifyloop/main.go` § cmdPlan prints item IDs and prompts, nothing about the brief's frontmatter."
  - "The parsers this reuses: `statusgen/brieffile.go` § parseBriefFile (frontmatter → `BriefFile`, incl. `Gate`, `Risk`, `ExecTier`, `ExecTierWhy`, `Effort`, `Wave`, `Depends`, `Unblocks`); `statusgen/parse.go` § parseBriefTable (README rows → `Brief{Status, Verified, Reviewed, …}`); `statusgen/brieffile.go` § briefFilePaths / expectedBriefID (key ↔ filename)."
  - "The subcommand registration pattern and the unknown-subcommand refusal: `statusgen/main.go` § verifyrun/shardcheck interception (before flag parsing, owns its own flags) and `unknownsubcommand_test.go`."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
---

# Brief 12 — `statusgen brief <stream/NN>`: resolve an item key to its file, frontmatter and board row

## Dependencies
None.

## Context

files:
- `statusgen/main.go` (register the positional subcommand; extend the known-subcommand list)
- `statusgen/briefinfo.go` (planned) — resolve, assemble, render
- `statusgen/briefinfo_test.go` (planned) — against `statusgen/testdata` fixture streams
- `statusgen/README.md` (usage)

facts:
- key grammar: `<stream>/<NN>`; the file is `docs/streams/<stream>/brief-<NN>-*.md` under
  `--root` (`expectedBriefID` is the existing key↔file rule). **Exactly one** file must match:
  zero → exit 2 with `no brief file for <key>`; more than one → exit 2 naming both (this is the
  numeric-prefix collision the hand-rolled glob could not detect).
- frontmatter via `parseBriefFile`; a legacy brief with no `schema:` is reported with
  `"schema": "legacy"` and empty frontmatter fields, exit 0 — resolving a legacy brief is not
  an error.
- board row via `parseBriefTable` on `docs/streams/<stream>/README.md`; the row whose `#`
  equals `<NN>`. A missing row is reported as `"row": null` with exit 0 — a brief file with no
  row is a lint finding elsewhere, not a resolution failure. (Three-state: the JSON says the
  row is absent; it never invents a status.)
- output (`--json`, the default; `--text` renders the same as `key: value` lines):
  `{"key","file","schema","title","wave","effort","gate","risk":{…},"exec_tier","exec_tier_why",
  "depends":[…],"unblocks":[…],"issues":[…],"row":{"status","verified","reviewed","wave","effort"}}`
  — file path RELATIVE to `--root`, so the output carries no machine path.
- multiple keys are accepted (`statusgen brief a/01 b/02`) and emit a JSON array in argument
  order; one unresolvable key fails the whole call (exit 2) after reporting every key's
  outcome, so a consumer never acts on a partial array read as complete.
- the subcommand reads files only: no STATUS.md write, no network, no `gh`.
- fixture: `statusgen/testdata` already carries stream fixtures; add one stream with a
  brief-v1 brief, a legacy brief, a brief with no README row, and two files sharing `brief-03-`.

## Ground rules
- Read-only: the subcommand must not touch STATUS.md or any generated file.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Register** `brief` as a positional subcommand intercepted before the parent flag parse
   (the `verifyrun` pattern), with its own `--root` (default `.`), `--json` / `--text`.
2. **Resolve + assemble** per the facts, reusing `parseBriefFile` and `parseBriefTable`.
3. **Tests**: happy path fields match the fixture; legacy brief resolves with `schema:
   legacy`; missing row → `row: null`, exit 0; duplicate prefix → exit 2 naming both files;
   unknown key → exit 2; multi-key array order; `--text` renders every key the JSON carries;
   the run leaves the fixture tree byte-identical.
4. **README** usage paragraph, and the one-line note in `tools/desk/cmd/verifyloop`'s usage
   text that the item keys `plan` prints resolve with `statusgen brief <key>`.
5. **Nothing else.** No lint change; no new frontmatter fields.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd statusgen && go test . -run '^TestBriefInfoResolvesFrontmatterAndRow$' -count=1` | exit 0 — gate, risk, exec-tier, effort and the row's status match the fixture |
| 3 | check:ci | `cd statusgen && go test . -run '^TestBriefInfoDuplicatePrefixIsAnError$' -count=1` | exit 0 — two `brief-03-*` files → exit 2 naming both, no JSON body |
| 4 | check:ci | `cd statusgen && go test . -run '^TestBriefInfoLegacyAndMissingRow$' -count=1` | exit 0 — legacy → `schema: legacy`; no row → `row: null`; both exit 0 |
| 5 | check:ci | `cd statusgen && go test . -run '^TestBriefInfoMultiKeyPartialFailure$' -count=1` | exit 0 — one bad key among three → exit 2, every key reported |
| 6 | check:ci | `cd statusgen && go run . brief desk-tools/12 --root .. --json > /tmp/bi.json; rc=$?; grep -q '"gate": *"model"' /tmp/bi.json; g=$?; grep -q '"status"' /tmp/bi.json; h=$?; [ "$rc" -eq 0 ] && [ "$g" -eq 0 ] && [ "$h" -eq 0 ]` | exit 0 — this brief resolves against the live tree with its own gate and a board row |
| 7 | check:ci | `cd statusgen && go test . -count=1` | exit 0 — the full statusgen suite, including the unknown-subcommand test with `brief` added to the known list |
| 8 | check:ci | `gofmt -l statusgen/briefinfo.go statusgen/briefinfo_test.go > /tmp/sg-fmt.out; test ! -s /tmp/sg-fmt.out` | exit 0 — the brief's touched files only; the unrelated pre-existing `statusgen` files flag only under a newer local gofmt, not the CI toolchain (#555's module-wide drift), and are out of scope. |
| 9 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The glob matches `brief-1*` for key `/1` and returns the wrong brief | row 3 (the duplicate fixture is the same class) + row 2 (exact key) |
| A missing row rendered as `status: todo` | row 4 |
| A partial array printed with exit 0 | row 5 |
| Output carries an absolute path | row 6 (`grep` for a leading `/` in `"file"` is added to the test in row 2) |
| Subcommand accidentally regenerates STATUS.md | row 7's byte-identical fixture assertion |

## Evidence
### Non-implementer verifier run — VERIFY: PASS on brief-attributable rows (1-7,9); HELD on row 8 (too-broad whole-dir gofmt, external pre-existing files + go1.26-vs-CI-go1.25 toolchain drift) — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5d20ff9`
Runner ≠ implementer (first non-implementer run; prior Evidence was implementer-only). Isolated worktree off origin/main. Offline; statusgen built from this worktree's source, not PATH. `gate: model`, all risk `no`.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | statusgen go build+vet | exit 0 | exit 0 clean | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | go test -run brief-info-resolves-frontmatter-and-row | exit 0 | exit 0, ok | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | go test -run brief-info-duplicate-prefix-is-an-error | exit 0 | exit 0 — two brief-03-* → exit 2 naming both, no JSON | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | go test -run brief-info-legacy-and-missing-row | exit 0 | exit 0 — legacy→schema:legacy; no row→row:null | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | go test -run brief-info-multi-key-partial-failure | exit 0 | exit 0 — 1 bad key of 3 → exit 2, every key reported | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | statusgen brief desk-tools/12 --json (+gate/status greps) | exit 0 | exit 0 — gate:model, status:implemented, file relative | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | go test . (whole statusgen suite) | exit 0 | exit 0, ok 24.7s (incl. leaves-fixture-byte-identical) | 2026-09-06 | opus-4.8[1m]-verifier |
| 8 | gofmt -l statusgen empty | exit 0 | COULD-NOT-CHECK (attribute-not-blame) — go1.26 gofmt flags 6 pre-existing files, ALL outside brief-12's diff (git-proven; land in other commits); brief-12's own briefinfo.go + briefinfo_test.go are gofmt-clean. CI-faithful re-run needs go1.25 gofmt (offline-unavailable); no CI workflow enforces gofmt. Too-broad row — re-baseline to the brief's files | 2026-09-06 | opus-4.8[1m]-verifier |
| 9 | statusgen --lint | exit 0 | exit 0 — LINT: PASS | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: DERIVED — briefInfoExitResolve = 2 @ statusgen/briefinfo.go:44 — matches the brief's exit contract (zero/duplicate/unresolvable → exit 2) and statusgen's standing usage/refusal=2 convention; proven by rows 3 and 5. Remaining literals (exit-0, default root ".", key grammar, "legacy" sentinel, 2-part split) are reversible operational constants, rank last.`
**VERIFY: PASS on all brief-attributable rows (1-7,9); HELD on row 8 only.** Row 8's whole-dir gofmt flags 6 external pre-existing files under a go1.26/go1.25 toolchain drift; brief-12's own files are gofmt-clean. Not a brief-12 defect (no CFR row); held pending a row-8 re-baseline (scope to the brief's diff) and/or the whole-dir gofmt drift being fixed. Side finding: the whole-dir gofmt drift is growing (4 files 2026-09-04 → 6 now) — worth a one-time `gofmt -w statusgen` under go1.25 or a pinned-gofmt CI check.

<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd statusgen && go build ./... && go vet ./...` | 0 | clean build + vet | 2026-09-04 | opus-4.8[1m] |
| 2 | `cd statusgen && go test . -run '^TestBriefInfoResolvesFrontmatterAndRow$' -count=1` | 0 | PASS — gate=model, risk all no, exec-tier=strong, effort=M, row.status=implemented; file path relative (no leading `/`) | 2026-09-04 | opus-4.8[1m] |
| 3 | `cd statusgen && go test . -run '^TestBriefInfoDuplicatePrefixIsAnError$' -count=1` | 0 | PASS — two `brief-03-*` files → exit 2 naming both, empty stdout | 2026-09-04 | opus-4.8[1m] |
| 4 | `cd statusgen && go test . -run '^TestBriefInfoLegacyAndMissingRow$' -count=1` | 0 | PASS — legacy → `schema: legacy`; no row → `"row": null`; both exit 0 | 2026-09-04 | opus-4.8[1m] |
| 5 | `cd statusgen && go test . -run '^TestBriefInfoMultiKeyPartialFailure$' -count=1` | 0 | PASS — one bad key among three → exit 2, every key reported, no partial array | 2026-09-04 | opus-4.8[1m] |
| 6 | `cd statusgen && go run . brief desk-tools/12 --root .. --json` | 0 | resolves live: `"gate": "model"`, `"status": "todo"` present, `"file"` relative | 2026-09-04 | opus-4.8[1m] |
| 7 | `cd statusgen && go test . -count=1` | 0 | full suite ok (incl. unknown-subcommand test with `brief` in the known list) | 2026-09-04 | opus-4.8[1m] |
| 8 | `gofmt -l statusgen/briefinfo.go statusgen/briefinfo_test.go` | 0 | empty — the brief's touched files (`briefinfo.go`, `briefinfo_test.go`) are gofmt-clean under both go1.25 (CI pin) and go1.26; the unrelated pre-existing `statusgen` files that flag only under a newer local gofmt (not the CI toolchain) are #555's module-wide drift, out of scope | 2026-09-04 / re-scoped 2026-09-06 | opus-4.8[1m] |
| 9 | `cd statusgen && go run . --root .. --lint` | 0 | `LINT: PASS` | 2026-09-04 | opus-4.8[1m] |

## Review

Gate: model (all four risk answers no). The reviewer confirms the subcommand is read-only and
that rows 3 and 5 are the rows that make a wrong-file or partial answer impossible to read as
success.
