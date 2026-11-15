---
brief: assay:assay:statusgen:43
title: Desk read-side consumers call the board library in-process
why: "The desk board starts one statusgen process per configured root for each queue it shows, and the verify loop keeps its own copy of the stream-table and frontmatter parser because statusgen could not be imported. One recorded board read over four roots took 83 seconds, and the copied parser has to be kept in step with the original by hand. With the board library and its queue projections in place, these readers make one in-process call, and the copies can be deleted."
wave: 4
depends: ["statusgen/39", "statusgen/42"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-11-15 by statusgen newbrief
sources:
  - "statusgen/39 (the board view and loader) and statusgen/42 (the queue projections): the two library calls these consumers switch to"
  - "tools/desk/cmd/deskboard/roots.go lines 3 to 13: the recorded cost of one statusgen subprocess per root"
  - "tools/desk/cmd/verifyloop/briefscan.go lines 39 to 42: the comment recording that the desk mirrors statusgen's parser because the command package cannot be imported"
  - "docs/library-first.md lines 17 to 22 and 222 to 227: consumers link the one implementation, and a migrated command removes its old evaluator in the same change"
  - "docs/shared-policy-consolidation-plan.md: the brief-document unit, which owns the parsers this brief leaves alone"
  - freshness-checked 2026-11-14 @ 876ca4bca (origin/main)
exec-tier: strong
exec-tier-why: "Correctness depends on three consumers keeping their fail-closed behaviour when a subprocess boundary becomes a function call."
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/deskboard (awaiting, dispatch, throughput, stalled), tools/desk/cmd/verifyloop (plan and brief scan), tools/desk/cmd/issueboard (placeholder and intake rows): follow-up statusgen/43"
  - "tools/desk/cmd/deskevidence, deskpreflight, deskdispatch, deskreconcile, scanloop and deskinbox subprocess calls: out-of-scope (they run lint, gate, corroboration or metric modes, none of which is a board or queue read; each stays a command call until its own mode has a library form)"
  - "parsers that edit text in place or read a git revision (tools/desk/internal/deskkit/briefstable.go, tools/desk/cmd/deskevidence, tools/desk/cmd/deskpathguard, tools/desk/cmd/deskpushguard, tools/desk/internal/loopengine/reconcile.go): out-of-scope (they need a parser function on bytes, owned by the brief-document unit of the shared-policy consolidation plan)"
  - "plugins/assay/scripts and other shell readers of the statusgen command: out-of-scope (the command and its wire shapes are unchanged and remain the supported seam for non-Go readers)"
---

# Brief 43 — Desk read-side consumers call the board library in-process

## Context

files: `tools/desk/cmd/deskboard/nextup.go`, `tools/desk/cmd/deskboard/dispatch.go`, `tools/desk/cmd/deskboard/roots.go`, `tools/desk/cmd/deskboard/board.go`, `tools/desk/cmd/deskboard/stalled.go`, `tools/desk/cmd/deskboard/throughput.go`, `tools/desk/cmd/verifyloop/plan.go`, `tools/desk/cmd/verifyloop/briefscan.go`, `tools/desk/cmd/verifyloop/frontmatter.go`, `tools/desk/cmd/verifyloop/verifyrows.go`, `tools/desk/cmd/issueboard/board.go`, `tools/desk/internal/arch/` (allow-lists and a new exec guard test, planned), `tools/desk/testdata/board-exec-plant/` (planned), `changelog/desk-board-library-readers.md` (planned).

facts (876ca4bca, 14 November 2026):
- `tools/desk` already requires and replaces the statusgen module (`tools/desk/go.mod:129`) and
  imports its `briefdoc`, `readercaps`, `verifyqueue`, `workref` and `contexthandoff` packages.
- `deskboard` runs `statusgen --gate-scores --root <r>` (`nextup.go:202`) and
  `statusgen --next-up --root <r> --role-claims` (`dispatch.go:178`) once per configured root,
  four at a time (`roots.go:41`), plus `--version` once (`nextup.go:188`). `throughput` runs both
  sets. `roots.go:10-13` records 82.82 s for one `throughput` run over four roots.
- `verifyloop plan` runs `--gate-scores` once per root (`plan.go:160`, loop at `plan.go:314`).
- `tools/desk/cmd/verifyloop/briefscan.go` (832 lines) re-parses the stream README table (`parseStreamTable`,
  lines 347 to 395) and brief frontmatter (`frontmatter.go:13-75`: gate, risk, effort, exec tier,
  implementer, blocked-until, verify lane, in-repair), finds Verify row identifiers
  (`verifyrows.go:60`) and tests whether Evidence is empty. Its comment at lines 39 to 42 says it
  mirrors statusgen because statusgen "is package main and unimportable anyway".
- `tools/desk/cmd/issueboard/board.go:449-479` has its own flat frontmatter reader for placeholder and intake
  rows (repo, status, blocked, id, date, title, disposition).
- statusgen/39's brief row carries every field named above, and its view carries placeholder
  and intake rows with that flat frontmatter. statusgen/42 returns the `--gate-scores` rows and
  the `--next-up` document from one call over several roots.
- Today a failed or missing statusgen binary makes these verbs report could-not-check
  (`deskkit.Unverifiable`, exit 6), never an empty queue.
- `resolveRootsOnce` (`roots.go:77-115`) applies two fail-closed gates besides the binary lookup.
  The reader-capabilities admission (`roots.go:94-99`, `capstree.Check`) refuses a configured
  root that declares a requirement deskboard does not support (exit 5) and answers any other
  non-supported result could-not-check (exit 6). It is the only admission these verbs apply to a
  configured sibling root: statusgen exits for `--gate-scores` and `--next-up`
  (`statusgen/main.go:2415-2422`) before its own admission (`statusgen/main.go:2616`).
  `TestDeskCapsPreflight` (`tools/desk/cmd/deskboard/capabilities_test.go`) pins it. The pin check
  (`resolveStatusgenPin`, `nextup.go:252-303`) answers could-not-check (exit 6) when no configured
  root carries a statusgen pin, or the pin it finds is malformed or unusable.
- The `parseVerifyRows` and `parseVerifyRowsIn` parser in `verifyrows.go` is also read by the
  verdict runner (`verdictrun.go:456`), which executes Command cells, and by the pending-Evidence
  reader (`pending.go:492`), which parses a brief's bytes at a landed revision. Neither is a board
  read.
- `tools/desk/internal/arch` enforces dependency direction and the hub allow-list
  (`arch.go:11-27`); a new import from a command package must satisfy both.
- A shared golden, `statusgen/testdata/flowqueue/gate-scores.golden.json`, is read by
  `tools/desk/cmd/deskboard/flow_45_test.go` and `tools/desk/cmd/verifyloop/flow_45_test.go`.

layering: no new component. Three command packages replace a subprocess call or a copied parser
with a call into `statusgen/board/tree` and `statusgen/board/queue`. The statusgen command stays
the seam for shell readers and for every mode that is not a board or queue read.

single point of failure: the fail-closed behaviour used to be the process boundary (a missing
binary could not look like an empty queue). After this change it is the view's per-root
`readable` flag and availability states. Two independent layers hold it: the consumers map an
unreadable root to could-not-check (row 3), and a structural guard keeps the removed subprocess
calls from coming back (rows 5 and 6). The two gates `resolveRootsOnce` applies before any root
is read are not part of the subprocess boundary and do not move: the reader-capabilities
admission still runs ahead of the library load (rows 11 and 12), and a missing pin is still
could-not-check (row 13).

design-fit:
  owner: statusgen
  contract: none — no meaning changes; these consumers read the same rows through a call instead of a process
  retires: ["one statusgen subprocess per root in deskboard awaiting, dispatch and throughput, and in verifyloop plan", "the statusgen binary lookup in deskboard's root resolution (the reader-capabilities admission and the pin check stay)", "the stream-table and frontmatter parser copies in tools/desk/cmd/verifyloop, and the plan scan's own Verify-row parse", "the flat frontmatter reader in tools/desk/cmd/issueboard"]
  weight: "verbs 0, flags 0, refusals 0, rule-text lines 0; the copied parser bodies (about 130 lines) and the plan-scan file reads that feed them are removed"
  why-add: n/a

## Read first

- `docs/streams/statusgen/brief-39-multi-root-objective-reader.md` and
  `docs/streams/statusgen/brief-42-queue-projections-from-one-board-load-next-up-awaiting-and-brief-info.md`.
- `tools/desk/cmd/deskboard/roots.go`, whole file: the root resolution and the pin check.
- `tools/desk/cmd/verifyloop/briefscan.go` and `queueclass.go`.
- `docs/three-state-instrument-rule.md`.

## Ground rules

- Isolated branch and draft PR. No push to main, no merge, no release, no workflow trigger.
- Output of every changed verb is unchanged for the same tree. Existing tests and goldens are
  the baseline; do not edit a golden to make a row pass.
- Could-not-check stays could-not-check. An unreadable root, a partial view or an unknown claims
  state is reported exactly as a failed subprocess is reported today, with the same exit code.
- Delete what is replaced, in the same change. No copied parser is left behind as a fallback.
- Keep both fail-closed gates of `resolveRootsOnce`. The reader-capabilities admission stays where
  it is and runs before the library load, with the same refusal (exit 5) and could-not-check
  (exit 6) results; do not rely on the library's own admission in its place. The pin check stays a
  gate with the same could-not-check result. Do not edit `TestDeskCapsPreflight` to make a row
  pass. Removing or weakening either gate is outside this brief: STOP and report NEEDS_CONTEXT.
- The version line these verbs print keeps naming the statusgen version in use; it now reports
  the linked module version, and says so.
- Leave every out-of-scope caller in `consumers:` untouched.
- Stop at `implemented`. A different, non-implementing identity verifies and moves the brief on.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **deskboard.** `awaiting`, `dispatch`, `throughput` and `stalled` load every configured root
   once through `statusgen/board/tree` and call `statusgen/board/queue` once per queue. Remove
   the per-root subprocess calls and the worker pool that existed to run them. Keep the root
   resolution in `roots.go`, and in it, in the same order: root resolution, the
   reader-capabilities admission (`capstree.Check`, before the library load) and the pin check
   (`resolveStatusgenPin`). Replace only the binary lookup (`resolveStatusgen`) and
   `statusgenVersionOf`: the running version becomes the linked module version, still compared
   with the pin for the skew field. Rewrite the comment at `roots.go:92-93`, which places the
   admission "before any reader process is started", to say it runs before the library load.
2. **verifyloop.** `plan` reads the awaiting rows from the library. Replace `parseStreamTable`,
   the frontmatter extractor (`parseFrontmatter` and `parseRisk` in `frontmatter.go`) and the
   plan scan's calls to the Verify-row parser with fields of the board view's brief row. Delete
   `parseStreamTable`, `parseFrontmatter` and `parseRisk`. The Verify-row parser in
   `verifyrows.go` stays, because the verdict runner and the pending-Evidence reader still use it
   (Context facts); after this change no other file in the package calls it. If the view lacks a
   field the plan scan needs, extend the view additively, as statusgen/39 provides. The
   queue-class logic that is verifyloop's own stays.
3. **issueboard.** Read placeholder and intake rows from the board view and delete
   `parseFrontmatter`.
4. **Fail closed.** Map an unreadable root, a partial view and an unknown claims state to the
   same could-not-check results and exit codes the verbs produce today.
5. **Guard.** Add a structural test that lists every call in `tools/desk` that starts the
   statusgen binary and fails on any call outside a committed allow-list. The allow-list holds
   the out-of-scope callers named in `consumers:` and nothing in `deskboard`, `verifyloop` or
   `issueboard`. Commit a planted fixture the guard must flag.
6. **Measure.** Record in the PR body the wall time of `deskboard awaiting` and
   `deskboard throughput` over the configured roots before and after, and the number of
   statusgen processes started (the target is zero for these verbs). Add the changelog fragment.

## Verify

Commands run from this repository root. The named tests are planned deliverables. Each row greps
the exact PASS line, so a selector that matches nothing fails the row.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestFlowQueue$" ./cmd/deskboard/ > "$check_output" && grep -F -- "--- PASS: TestFlowQueue " "$check_output")` | exit 0; PASS line printed. deskboard's awaiting view over the shared fixture still equals the shared `--gate-scores` golden, now read through the library |
| 2 | check:ci | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPlanFromBoardView$" ./cmd/verifyloop/ > "$check_output" && grep -F -- "--- PASS: TestPlanFromBoardView " "$check_output")` | exit 0; PASS line printed. On a fixture tree `verifyloop plan` produces the same plan as the committed pre-change expectation, with gate, risk, exec tier, implementer, blocked-until, Verify row identifiers and Evidence-empty read from the board view |
| 3 | check:ci | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardUnreadableRoot$" ./cmd/deskboard/ > "$check_output" && grep -F -- "--- PASS: TestBoardUnreadableRoot " "$check_output")` | exit 0; PASS line printed. With one of two roots unreadable, `awaiting` and `dispatch` report could-not-check for that root with exit code 6 and never print it as an empty queue |
| 4 | check:ci | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestIssueRowsFromView$" ./cmd/issueboard/ > "$check_output" && grep -F -- "--- PASS: TestIssueRowsFromView " "$check_output")` | exit 0; PASS line printed. Placeholder and intake rows on a fixture tree equal the committed pre-change expectation |
| 5 | check:ci +mutation | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestNoBoardExec$" ./internal/arch/ > "$check_output" && grep -F -- "--- PASS: TestNoBoardExec " "$check_output")` | exit 0; PASS line printed. Every call in `tools/desk` that starts the statusgen binary is on the allow-list, and no allow-listed call is in `deskboard`, `verifyloop` or `issueboard` |
| 6 | check:ci +mutation | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestNoBoardExecFlagsPlant$" ./internal/arch/ > "$check_output" && grep -F -- "--- PASS: TestNoBoardExecFlagsPlant " "$check_output")` | exit 0; PASS line printed. Run over `testdata/board-exec-plant/` (one planted `--gate-scores` subprocess call outside the allow-list), the guard reports that call |
| 7 | check:ci | `test -z "$(grep -rl --include='*.go' -e 'func parseStreamTable' -e 'func parseFrontmatter' -e 'func parseRisk' tools/desk/cmd/verifyloop tools/desk/cmd/issueboard)" && test -z "$(grep -rl --include='*.go' --exclude='*_test.go' --exclude=verifyrows.go --exclude=verdictrun.go --exclude=pending.go -e 'parseVerifyRowsIn(' -e 'parseVerifyRows(' tools/desk/cmd/verifyloop)"` | exit 0 and no file name printed: no copied stream-table or frontmatter parser is defined anywhere in either package, and no file of verifyloop other than the parser itself, the verdict runner and the pending-Evidence reader calls the Verify-row parser, so the plan scan reads Verify row identifiers from the view |
| 8 | check:ci +neighbour | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/deskboard/... ./cmd/verifyloop/... ./cmd/issueboard/... ./internal/arch/...) && (cd statusgen && GOWORK=off go test -count=1 -run "^TestEvaluationExternalConsumer" .)` | exit 0; every existing test of the three consumers and the architecture rules passes, and statusgen's manifest still names nothing under `tools/desk` |
| 9 | check +flow +dereference | `(cd tools/desk && GOWORK=off go build -o "${TMPDIR:-/tmp}/deskboard-43" ./cmd/deskboard) && pin_root=$(mktemp -d) && mkdir -p "$pin_root/docs/streams" && printf 'statusgen v0.0.0-pin-43 %s\n' 0000000000000000000000000000000000000000000000000000000000000000 > "$pin_root/.assay-versions" && STATUSGEN_BIN="${TMPDIR:-/tmp}/no-such-statusgen-43" DESK_ROOTS="medici-finance/assay=$(pwd),example-org/pin-root=$pin_root" "${TMPDIR:-/tmp}/deskboard-43" awaiting --json > "${TMPDIR:-/tmp}/awaiting-43.json" && grep -F -o '"queueVersion":1' "${TMPDIR:-/tmp}/awaiting-43.json"` | exit 0; with this repository and a second root that carries only a statusgen pin configured, and `STATUSGEN_BIN` naming a file that does not exist, `deskboard awaiting` still prints versioned awaiting rows, which it could not do if it started the binary. The pin root is there because this repository carries no `.assay-versions` and the pin check stays a gate (row 13); the output reports skew, since the pinned tag is not the linked module version (flow row: root resolution, admission, pin check, loader, queue projection, deskboard rendering; it dereferences the claim that no subprocess is used) |
| 10 | check +flow | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-43" .) && test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && "${TMPDIR:-/tmp}/sg-43" --root . --consumers --brief statusgen/43 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0, post-merge: `ASSAY_DELIVERY_SHA` is the merged delivery commit and `ASSAY_DELIVERY_BASE_SHA` its first parent. The report corroborates each `consumers:` line above with no unchecked obligation; record its exact counts |
| 11 | check:ci | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestDeskCapsPreflight$" ./cmd/deskboard/ > "$check_output" && grep -F -- "--- PASS: TestDeskCapsPreflight " "$check_output")` | exit 0; PASS line printed. The existing test, unedited in its assertions: `dispatch` refuses with exit 5 and writes no output for a root that declares an unsupported requirement, and `resolveRootsOnce` still refuses a configured root that declares one |
| 12 | check:ci +mutation | `(cd tools/desk && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBoardCapsRefusedFirst$" ./cmd/deskboard/ > "$check_output" && grep -F -- "--- PASS: TestBoardCapsRefusedFirst " "$check_output")` | exit 0; PASS line printed. With two configured roots, one declaring a requirement deskboard does not support, `awaiting` and `dispatch` each exit 5, print no row, and never call the board loader (a test hook counts loads: zero); with that root's requirement record unreadable, each exits 6. With the admission call removed by the test's hook, the same assertions fail |
| 13 | check +flow | `nopin_root=$(mktemp -d) && mkdir -p "$nopin_root/docs/streams" && { STATUSGEN_BIN="${TMPDIR:-/tmp}/no-such-statusgen-43" DESK_ROOTS="example-org/no-pin=$nopin_root" "${TMPDIR:-/tmp}/deskboard-43" awaiting --json > /dev/null 2> "${TMPDIR:-/tmp}/awaiting-43-nopin.err"; rc=$?; } ; test "$rc" -eq 6 && grep -F -o 'carry a readable' "${TMPDIR:-/tmp}/awaiting-43-nopin.err"` | exit 0, using the binary built in row 9: with the one configured root carrying no `.assay-versions`, `deskboard awaiting` still answers could-not-check (exit 6) naming the missing pin file, as it does today, even though no statusgen binary is started |

| Failure mode of the work | Caught by |
|---|---|
| A verb's output changes when its source changes | rows 1, 2 and 4, against goldens and committed expectations |
| An unreadable root becomes an empty queue | row 3 |
| A copied parser is left in place as a fallback, in the cited file or a sibling file of the package | row 7 |
| The reader-capabilities admission is dropped with the subprocess boundary, so an unsupported tree is rendered as queue rows | rows 11 and 12 |
| The pin check is dropped with the binary lookup, so a missing pin stops being could-not-check | row 13 |
| A subprocess call creeps back into these verbs | rows 5 and 6 |
| The import breaks the module or architecture rules | row 8 |
| The whole chain fails on a real tree though fixtures pass | row 9 |
| Whether the measured speed-up is as large as expected | review-only: Task step 6 records before and after in the PR body; the budget itself is gated in statusgen/44 and statusgen/39 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). The declared paths under
`tools/desk/cmd/deskboard/` match a security-path trigger, so the four answers were read against
them: the changed code only reads planning rows from a checkout and prints them, the verbs that
post, flip or mint a credential are not touched, and Verify row 3 holds the fail-closed
behaviour. If the implementation finds it must change how a review or flip decision is derived,
STOP and report NEEDS_CONTEXT so the risk answers are taken again. Reviewer records verdict +
date in the stream README table.
