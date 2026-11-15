---
brief: assay:assay:statusgen:39
title: 'Board reader library: importable multi-root loader and the whole-board view'
why: "Other tools can only reach statusgen's parsed board by running the binary once per root and decoding its output, or by re-implementing its parsers; both exist today, and both are slow or drift. The loader lives in the command's own package, reads process-wide state and cannot be called twice safely, so no service or desk tool can link it. This brief moves the loader into an importable, re-entrant library and adds one whole-board view across every configured root, so a caller makes one function call and gets every stream, every brief row and every edge, with unreadable parts named instead of dropped."
wave: 2
depends: ["statusgen/44"]
unblocks: ["statusgen/41", "statusgen/42", "statusgen/43", "cell-api/10"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-22 by statusgen newbrief (worker-desk authoring, #4107 part v2)
sources:
  - "Version 2, 14 November 2026, before first dispatch: re-scoped from one objective's closure to the whole board, and from a command mode to a library with a command adapter. The objective closure, critical path and ranked moves of version 1 moved unchanged in intent to statusgen/41."
  - "#4107: the issue that asked for a first-class multi-root reader so the objective skill becomes a thin renderer. The multi-root stitch it needs is built here; its objective output is statusgen/41."
  - "docs/library-first.md lines 17 to 22, 118 to 124, 192 to 194 and 328 to 332: shared behaviour has one implementation exposed as a library, commands are adapters, statusgen's `package main` is a named blocker, and the effectful filesystem adapter sits beside a pure core"
  - "statusgen/15 (verified): the extraction of the eligibility and coverage evaluator, whose method this brief repeats for the loader"
  - "docs/dependency-graph-design.md §3.3, §3.4 and §3.7: the reference grammar, the gates/feathers edge types, and the three-state verdict reused unchanged"
  - "docs/proposals/non-ci-architecture/29-extensible-json-contracts.md and desktools-v2/69: the rules a new JSON contract follows"
  - "docs/shared-policy-consolidation-plan.md lines 83 to 88: no import of the desk hub from statusgen, and old commands and wire shapes stay until consumers migrate"
  - freshness-checked 2026-11-14 @ 876ca4bca (origin/main)
exec-tier: strong
exec-tier-why: "Moving a loader out of a command package without changing one verdict needs design decisions the facts do not fully specify, and correctness depends on every existing reader still seeing the same model."
domain: complicated
outcome: none
id: 260bbb3c-5477-4967-8f07-6867b71195af
consumers:
  - "statusgen/board, statusgen/board/tree, the `statusgen board` command adapter, schemas/board-view-v1.json, statusgen/README.md, docs/contracts.md, docs/library-first.md: follow-up statusgen/39"
  - "statusgen --next-up, --gate-scores, --eligibility, --graph, --launch and brief --json wire shapes: out-of-scope (unchanged here; Verify row 9 holds them byte-identical, and their library projections are statusgen/42)"
  - "objective closure, critical path and ranked moves, and the trace-path-to-live skill: follow-up statusgen/41"
  - "tools/desk read-side callers (deskboard, verifyloop, issueboard): follow-up statusgen/43"
  - "deskd roadmap and portfolio reads (deskd/internal/mcp/query.go, deskd/internal/serve/v1.go): follow-up cell-api/10 (this brief ships the call, the filesystem-root form and the fields those reads name; that brief links them into the service)"
  - "statusgen/25 portfolio projections and milestone-support/11 stream views: out-of-scope (each composes its own records and neither is edited here; both may read the board view once it ships)"
  - "parsers that edit text in place or read a git revision instead of a working tree (tools/desk/internal/deskkit/briefstable.go, tools/desk/cmd/deskevidence, tools/desk/cmd/deskpathguard, tools/desk/cmd/deskpushguard, tools/desk/internal/loopengine/reconcile.go, qualgen/regressionlink.go): out-of-scope (they need a shared parser function on bytes, which is the brief-document unit of the shared-policy consolidation plan, not a loaded view)"
---

# Brief 39 — Board reader library: importable multi-root loader and the whole-board view

Version 2. Version 1 of this brief specified a command mode for one objective. Nothing was
dispatched or built against it. The file name keeps its original slug.

## Context

files: `statusgen/board/` (planned: `doc.go`, `view.go`, `ids.go`, `stitch.go` and tests), `statusgen/board/tree/` (planned: `load.go` and tests), `statusgen/board_adapter.go` (planned), `statusgen/model.go`, `statusgen/parse.go`, `statusgen/load.go`, `statusgen/brieffile.go`, `statusgen/eligibility.go`, `statusgen/multiroot.go`, `statusgen/main.go`, `statusgen/testdata/board/` (planned: two fixture roots and an alias registry), `statusgen/testdata/board-plant/` (planned), `schemas/board-view-v1.json` (planned), `statusgen/README.md`, `docs/contracts.md`, `docs/library-first.md`, `changelog/statusgen-board-library.md` (planned).

facts (876ca4bca, 14 November 2026):
- Every statusgen source file outside its subpackages is `package main`, so nothing can import
  the loader. `docs/library-first.md:192` names this as a first blocker.
- The load path: `loadStreams` (`load.go:195`), `parseStreamREADME` (`parse.go:210`),
  `parseBriefTable` (`parse.go:153`), and `loadHydratedStreams` (`load.go:366`), which fills the
  frontmatter-derived fields only as a side effect of `checkBriefFiles` (`brieffile.go:1613`,
  writes at lines 1856 to 1900). The model types are `Stream`, `Brief` and `Finding`
  (`model.go:12`, `:116`, `:296`).
- The load path reads package-level mutable state: `stagedReadmes` (`reconcilehold.go:42`),
  `allowEmptyRoot` (`load.go:38`), `milestoneReader` (`milestones.go:193`), the read and parse
  memos (`readmemo.go:102`, `brieffile.go:578`), and the performance stack (`perfEnter`). The
  memos are unbounded and are reset only by tests. `run()` is not re-entrant: it sets
  `activeGitReads`, `activeDriveSet`, `activeFindings` and others for the process.
- The load path itself runs no process, reads no environment variable and calls no `os.Exit`.
- A multi-root run loads every root twice: once in `crossRootProblems` (`multiroot.go:149`) and
  once in the per-root `run()` loop (`main.go:1147-1166`), serially.
- `siblingLookup` (`eligibility.go:98-112`) reloads a whole sibling root for each cross-repo
  edge, with no memo.
- `--graph`, `--gate-scores`, `--next-up` and `--eligibility` each accept exactly one `--root`
  (`main.go:2054-2072`), and `--graph` emits no `gates:` or `feathers:` edge.
- The model holds no Verify row identifiers; that parsing lives in `verifyrun.go:1456-1482`.
- The extraction precedent is `statusgen/evaluation`: its `doc.go` states it "opens no file,
  runs no process, reads no environment variable, clock, token or network, and keeps no global
  state". The root package re-exports its types as aliases (`eligibility.go:35-50`) and adapts
  to its `Repo` port. `statusgen/readercaps/tree` is the precedent for an effectful filesystem
  adapter kept apart from a pure core.
- `tools/desk` already requires and replaces the statusgen module (`tools/desk/go.mod:129`) and
  imports five of its packages. `statusgen/evaluation_contract_test.go:853-861` fails if
  statusgen's manifest names anything under `tools/desk`.
- `docs/streams/graph-repos.yaml` is the alias registry; an alias with `repo: null` resolves no
  further from this tree and is a could-not-check, never a silent answer.

layering: a pure core `statusgen/board` holds the view types, canonical identifiers, the
multi-root stitch and edge states, under the same no-file, no-process, no-environment,
no-global-state rule as `statusgen/evaluation`. An explicitly effectful `statusgen/board/tree`
reads roots through the standard `fs.FS` interface and nothing else: no git, no forge, no
network. A checkout is one such filesystem; a caller with no checkout can hand in another. The
two board packages import nothing outside the statusgen module except the one sibling the model
already imports (`deskcore/issueflow`, `model.go:4`), so a module that links them resolves as
little as possible. The command package keeps adapters and type aliases. Facts that need git or a forge (last touch, claims,
delivery history) are not in the view; they stay behind the command package's adapters and are
statusgen/42's concern. Task steps 2 to 4 build this; Verify rows 1, 5 and 6 check it.

single point of failure: the design depends on the library and the command giving the same
answer. Two independent layers hold that: a parity test over fixtures (row 1), and the removal of
the old bodies so no second implementation exists to diverge (row 5, a structural check that
fails for a different reason than a fixture mismatch).

design-fit:
  owner: statusgen
  contract: board-view-v1 (planned; register its semantic owner in docs/contracts.md)
  retires: ["the loader and hydration bodies inside the command package, kept only as forwarding adapters", "the per-edge reload of a sibling root in siblingLookup", "the second load of every root in crossRootProblems"]
  weight: "verbs +1 (statusgen board), flags 0, refusals 0, rule-text lines 0"
  why-add: "No existing reader is importable or crosses roots. Extending --graph was considered and rejected: it would change an established single-root wire shape, and it carries no brief rows and no gates or feathers edges."

## Read first

- `docs/library-first.md`, whole file.
- `docs/streams/statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md`:
  the method and its re-cut rule.
- `statusgen/evaluation/doc.go`, `statusgen/eligibility.go`, `statusgen/readercaps/tree/tree.go`.
- `docs/dependency-graph-design.md` §3.3, §3.4 and §3.7.
- `docs/three-state-instrument-rule.md`: could-not-check is never collapsed into satisfied or empty.
- `docs/proposals/non-ci-architecture/29-extensible-json-contracts.md`.
- `statusgen/streamview/contract.go`: the availability vocabulary reused here.

## Ground rules

- Isolated branch and draft PR. No push to main, no merge, no release, no workflow trigger.
- Read-only and offline: the library and its command adapter open files under the given roots and
  nothing else. No process, no network, no write to any root or to STATUS.md.
- One implementation. Move the loader; do not copy it. A thin forwarding wrapper is allowed in
  the command package; an independent recalculation is not.
- Existing commands, flags, exit codes and wire shapes do not change. This brief adds a library
  and one command; it retires no flag.
- Never collapse could-not-check. An unreadable root, an unpublished alias or a forge-only
  target stays named in the view, and the answer is marked partial.
- Re-cut rule, as in statusgen/15: if moving the loader requires moving lint rule bodies, the
  Next-up ranking, or any git-backed check, STOP and report NEEDS_CONTEXT with the dependency
  inventory. Do not widen the brief.
- Stop at `implemented`. A different, non-implementing identity verifies and moves the brief on.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Inventory.** List, in the PR description, every package-level variable the load and
   hydration path reads or writes, starting from the Context list, and how each is replaced: an
   argument, a field on a loader value, or left in the command package because the library does
   not need it.
2. **Pure core, `statusgen/board`.** Define the view types and the canonical identifier
   `<repo-alias>:<stream>/<NN>`. A brief row carries: identifier, number, title, repo-relative
   file, schema, wave, effort, status, verified, reviewed, gate, the four risk answers,
   exec tier, value, blocked marker, blocked-until, implementer, verify lane, in-repair marker,
   depends, unblocks, gates, feathers, issues, the Verify row identifiers, and whether Evidence is
   empty. A stream carries its name, repo, root, status, priority, track, spec, issues and its
   `tiering` guidance (absent stays absent, distinct from present and empty). The
   view also carries findings, intake entries and placeholder rows with their flat frontmatter
   (repo, status, blocked, id, date, title, disposition). Every section carries an availability
   state from the `statusgen/streamview` vocabulary.
3. **Effectful loader, `statusgen/board/tree`.** Provide `Load(ctx, roots, options)` returning a
   view. A root is a repository identity plus an `fs.FS`; a helper builds one from a directory
   path, confined to that directory so that a path or symbolic link leaving it is refused. A
   loader value owns every memo; memos are bounded and are not shared between loader values.
   The two board packages import no sibling module other than `deskcore`. No package-level
   mutable state, no environment read (roots and options are
   arguments; parsing `DESK_ROOTS` stays in the command package), no write to standard output or
   error, no process exit. Two loads may run at once, and a second load after a file changes
   sees the change.
4. **The command package becomes an adapter.** `loadStreams`, `loadHydratedStreams`,
   `parseStreamREADME`, `parseBriefTable` and the hydration half of `checkBriefFiles` call the
   library; the model types become aliases, as `eligibility.go:35-50` does for the evaluator.
   Remove the old bodies. The lint half of `checkBriefFiles` stays where it is.
5. **Multi-root stitch.** One node set across every root. Resolve cross-root `depends:`,
   `gates:` and `feathers:` edges through `docs/streams/graph-repos.yaml` and the root map; when
   the map has no entry, fall back to the sibling-by-basename rule. That fallback is a directory
   operation (`eligibility.go:98-110` stats the parent directory), so it applies only to a root
   built from a directory path; for a root handed in as any other filesystem, a sibling that is
   not in the root map is could-not-check. A target that still cannot be
   resolved (an unpublished alias, an absent checkout, an `#<issue>` reference) becomes a node
   with `status: "could-not-check"` and its reason, never a dropped edge. Each root is read
   exactly once per load, and a sibling is taken from the roots already loaded.
6. **One JSON document.** Emit `{schema: "board-view-v1", roots, streams, briefs, edges,
   findings, intake}`, described by `schemas/board-view-v1.json` and registered in
   `docs/contracts.md`. Follow the extensible-contract rules: additive evolution, unknown fields
   ignored by readers. Output is compact, deterministically ordered and byte-identical across
   runs. `roots[]` lists each root with `readable: true|false`.
7. **Command adapter.** Add the positional command
   `statusgen board [--root <path>]... [--sibling-root <owner>/<repo>=<path>]...`, which prints
   the document. It is positional, so the meaning of a bare `--json` elsewhere does not move.
   Exit codes: 0 complete; 3 partial (any could-not-check node or an unreadable non-home root);
   2 when the home root is unreadable.
8. **Performance.** Loading this repository's tree through the library stays within the budget
   statusgen/44 set, using that brief's harness. A two-root load reads each root once.
9. **Documents.** Document the library and the command in `statusgen/README.md`; add the
   component to `docs/library-first.md`; add the changelog fragment.

## Interface contract

```
import "github.com/medici-finance/assay/statusgen/board"
import "github.com/medici-finance/assay/statusgen/board/tree"

view, err := tree.NewLoader(options).Load(ctx, roots)   // one call, every root
```

```
statusgen board --root <home> [--root <path> ...] [--sibling-root <owner>/<repo>=<path> ...]
```

Output (`board-view-v1`, abridged and wrapped here for reading; the command prints it compact):

```json
{"schema":"board-view-v1",
 "roots":[{"repo":"…","path":"…","readable":true}],
 "streams":[{"id":"<alias>:<stream>","repo":"…","root":"…","status":"…","priority":"…","tiering":"…","availability":"available"}],
 "briefs":[{"id":"<alias>:<stream>/<NN>","num":"…","title":"…","file":"…","status":"…","verified":"…","reviewed":"…",
            "wave":0,"effort":"…","gate":"…","risk":{},"exec_tier":"…","verify_rows":["1","2"],"evidence_empty":true}],
 "edges":[{"from":"…","to":"…","kind":"depends|gates|feathers","type":"…","state":"satisfied|unsatisfied|could-not-check"}],
 "findings":[{"id":"…","status":"…","affects":["…"]}],
 "intake":[{"id":"…","status":"…","disposition":"…"}]}
```

## Verify

Commands run from this repository root. The named tests are planned deliverables. Each row greps
the exact PASS line, so a selector that matches nothing fails the row.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardLoadParity$" . > "$check_output" && grep -F -- "--- PASS: TestBoardLoadParity " "$check_output")` | exit 0; PASS line printed. On the fixture roots the `statusgen board` output equals the library's encoded view byte for byte, and the streams the command package's `loadHydratedStreams` returns equal the view's rows field for field |
| 2 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardStitchRoots$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestBoardStitchRoots " "$check_output")` | exit 0; PASS line printed. Two fixture roots: a cross-root `depends:`, a cross-root `gates:` and a cross-root `feathers:` edge each appear with canonical identifiers, and every brief of both roots is present |
| 3 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardCouldNotCheck$" . > "$check_output" && grep -F -- "--- PASS: TestBoardCouldNotCheck " "$check_output")` | exit 0; PASS line printed. An `#<issue>` edge, an unpublished alias and an unreadable second root each produce a could-not-check node or an unreadable root entry with a reason, and the command exits 3; an unreadable home root exits 2 |
| 4 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardRowFields$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestBoardRowFields " "$check_output")` | exit 0; PASS line printed. A fixture brief's row carries every field Task step 2 lists, including its Verify row identifiers and its Evidence-empty flag, each equal to the value in the fixture file; a fixture stream's row carries its `tiering` value, and a stream with no `tiering` key has the field absent; a root handed in as an in-memory filesystem with a sibling missing from the root map yields a could-not-check node, not a directory lookup |
| 5 | check:ci +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardPurity$" . > "$check_output" && grep -F -- "--- PASS: TestBoardPurity " "$check_output")` | exit 0; PASS line printed. A structural check over `statusgen/board` and `statusgen/board/tree` finds no package-level mutable variable, no environment read, no process launch and no exit call; the pure core imports no filesystem package; and the command package holds no second body of the moved functions |
| 6 | check:ci +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardPurityFlagsPlant$" . > "$check_output" && grep -F -- "--- PASS: TestBoardPurityFlagsPlant " "$check_output")` | exit 0; PASS line printed. Run over `testdata/board-plant/` (a package with one planted package-level variable written by a load), the same structural check reports that variable |
| 7 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -race -count=1 -v -run "^TestBoardReentrant$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestBoardReentrant " "$check_output")` | exit 0; PASS line printed. Two loads of different roots run at once under the race detector and each equals its serial result; a second load after a brief file is edited reports the edited value; a two-root load opens each README once |
| 8 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardExternalConsumer$" . > "$check_output" && grep -F -- "--- PASS: TestBoardExternalConsumer " "$check_output")` | exit 0; PASS line printed. A module outside this repository's module graph imports `statusgen/board/tree` through a local proxy, builds, and loads a fixture root handed in as an in-memory `fs.FS` with no directory behind it, as the evaluator's external-consumer test does; the test also asserts the two board packages import no sibling module other than `deskcore` |
| 9 | check:ci +neighbour | `(cd statusgen && GOWORK=off go test -count=1 ./...)` | exit 0; every existing statusgen test still passes, including `TestFlowQueue`, `TestEvaluationLibraryCLIParity`, `TestGraphJSONLDeterministic`, `TestDispatchView*`, `TestBriefInfo*`, `TestLaunch*` and `TestMultiRoot*`, so `--gate-scores`, `--eligibility`, `--graph`, `--next-up`, `brief --json` and `--launch` output is unchanged |
| 10 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardSchemaGolden$" . > "$check_output" && grep -F -- "--- PASS: TestBoardSchemaGolden " "$check_output")` | exit 0; PASS line printed. The command output on the two-root fixture validates against `schemas/board-view-v1.json`, equals the checked-in golden, and is byte-identical across two runs |
| 11 | check +flow +dereference | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-39" .) && { "${TMPDIR:-/tmp}/sg-39" board --root . > "${TMPDIR:-/tmp}/board-39.json"; rc=$?; } ; test "$rc" -ne 2 && test "$rc" -ne 1 && grep -qF '"schema":"board-view-v1"' "${TMPDIR:-/tmp}/board-39.json" && grep -F -o '"id":"assay:statusgen/39"' "${TMPDIR:-/tmp}/board-39.json"` | exit 0; the command runs on this repository's own tree and exits 0, or 3 (partial) when the tree holds an edge to a forge-only or unpublished target the offline reader cannot resolve; the document carries the schema tag and this brief's own row (flow row; dereferences this brief's own file). Record the exit code and the count of could-not-check nodes |
| 12 | check +flow | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && ASSAY_HYDRATE_TREE="$(cd .. && pwd)" GOWORK=off go test -count=1 -v -run "^TestBoardLoadBudget$" . > "$check_output" && grep -F -- "--- PASS: TestBoardLoadBudget " "$check_output")` | exit 0; PASS line printed (a skipped test prints no PASS line and fails the row). A library load of this repository's tree stays within the budget statusgen/44 set |
| 13 | gate:model +dereference | `grep -n -F -e "statusgen board" statusgen/README.md && grep -n -F "board-view-v1" docs/contracts.md && grep -n -F "statusgen/board" docs/library-first.md && test -f schemas/board-view-v1.json` | exit 0; the reviewer then reads the matched sections: the README documents the library call, the command, its roots, output and exit codes; `docs/contracts.md` names statusgen as owner of `board-view-v1`; the schema and the README agree on every top-level key |
| 14 | check | `"${TMPDIR:-/tmp}/sg-39" --root . --lint` | exit 0; output contains "LINT: PASS" (the binary built in row 11). If the run reports a problem on a brief this delivery did not touch, the row is could-not-check for that reason, never passed: record the problem lines and re-run where the tree lints clean |
| 15 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && "${TMPDIR:-/tmp}/sg-39" --root . --consumers --brief statusgen/39 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0, post-merge: `ASSAY_DELIVERY_SHA` is the merged delivery commit and `ASSAY_DELIVERY_BASE_SHA` its first parent. The report corroborates each `consumers:` line above with no unchecked obligation; record its exact counts |

| Failure mode of the work | Caught by |
|---|---|
| The loader is copied instead of moved, so two implementations drift | row 5: no second body in the command package; row 1: field-for-field parity |
| The library still reads a process-wide variable, so a long-lived caller gets stale or mixed results | rows 5 and 6 (structural, with a planted control) and row 7 (concurrent and repeated loads) |
| An unreadable root or unresolved alias silently disappears from the view | row 3 |
| An existing flag's output shifts because its loader moved | row 9 |
| The view lacks a field a later consumer needs (Verify row identifiers, Evidence-empty) | row 4 |
| The library works only inside this repository's module graph | row 8 |
| Moving the code makes the load slower than statusgen/44 left it | row 12 |
| The schema is well-formed but describes a different shape than the command prints | rows 10 and 13 |
| Whether the row field list is the right list for every future consumer | review-only: statusgen/42 and statusgen/43 extend the view additively if a field is missing |

## Evidence

<!-- Appended at verification time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). This authoring PR runs
     no implementation check. -->

## Review

Gate: model (from frontmatter; all four risk answers no). Not a core-system surface: the library
reads planning documents from a checkout and writes nothing, so no settlement, identity or
ledger path is touched. The reviewer records verdict and date in the stream README table.
