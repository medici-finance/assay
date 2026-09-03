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
| 8 | check:ci | `gofmt -l statusgen > /tmp/sg-fmt.out; test ! -s /tmp/sg-fmt.out` | exit 0 |
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
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer confirms the subcommand is read-only and
that rows 3 and 5 are the rows that make a wrong-file or partial answer impossible to read as
success.
