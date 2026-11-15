---
brief: assay:assay:statusgen:42
title: 'Queue projections from one board load: next-up, awaiting and brief info'
why: "The dispatch queue, the verification queue and the per-brief lookup are three separate command modes. Each loads the tree on its own, each accepts one root, and each can only be reached by running the binary, so a desk tool that needs two queues over four roots starts eight processes and loads every tree twice. This brief makes the three answers library calls over one already-loaded board view, with the existing flags kept as adapters whose output does not change by a byte."
wave: 3
depends: ["statusgen/39"]
unblocks: ["statusgen/43", "cell-api/10"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-11-15 by statusgen newbrief
sources:
  - "statusgen/39: the board view and loader these projections read"
  - "docs/library-first.md lines 17 to 22 and 118 to 121: commands are adapters over one implementation, and command flags, exit codes and envelopes are preserved"
  - "docs/shared-policy-consolidation-plan.md lines 86 to 88: old commands and wire shapes stay until every consumer has migrated, and a wrapper may forward but may not keep a second implementation"
  - "docs/streams/decisions/DR-desk-tools-45.md: the --gate-scores JSON is the supported cross-module seam for queue classification; that seam is kept and unchanged here"
  - "statusgen/dispatchqueue.go, statusgen/nextup.go, statusgen/awaiting_bucket.go, statusgen/briefinfo.go, statusgen/verifyqueue/queue.go: the three modes and their wire types"
  - freshness-checked 2026-11-14 @ 876ca4bca (origin/main)
exec-tier: strong
exec-tier-why: "The ranking and classification code reads process-wide settings and caches; making that state explicit without changing one row of output needs design decisions the facts do not fully specify."
domain: complicated
outcome: none
consumers:
  - "statusgen/board/queue (planned), the --next-up, --gate-scores and brief --json command adapters, statusgen/README.md, docs/contracts.md, docs/library-first.md: follow-up statusgen/42"
  - "tools/desk callers that run these modes as subprocesses (deskboard, verifyloop): follow-up statusgen/43"
  - "the --next-up, --gate-scores and brief --json wire shapes read by shell and non-Go consumers: out-of-scope (unchanged; Verify rows 1 to 3 hold them byte-identical)"
  - "--eligibility, --graph, --launch, --bottleneck, --intake-debt and the metric modes: out-of-scope (not queue projections; each keeps its own mode and loader call)"
---

# Brief 42 — Queue projections from one board load: next-up, awaiting and brief info

Effort is L, at the ceiling, and is kept whole because the two queues share one options value
and one facts port; splitting them would design that seam twice. The re-cut rule in Ground rules
applies if the inventory grows.

## Context

files: `statusgen/board/queue/` (planned: `nextup.go`, `awaiting.go`, `briefinfo.go`, `facts.go` and tests), `statusgen/queue_adapter.go` (planned), `statusgen/nextup.go`, `statusgen/dispatchqueue.go`, `statusgen/awaiting_bucket.go`, `statusgen/briefinfo.go`, `statusgen/main.go`, `statusgen/verifyqueue/queue.go`, `statusgen/testdata/queue/` (planned goldens), `statusgen/README.md`, `docs/contracts.md`, `docs/library-first.md`, `changelog/statusgen-queue-projections.md` (planned).

facts (876ca4bca, 14 November 2026):
- `--next-up` prints `dispatchView` and `dispatchRow` (`dispatchqueue.go:34-78`), built by the
  pure `buildDispatchView` (`dispatchqueue.go:161`) after `runNextUp` (`dispatchqueue.go:88`)
  loads the tree. It has structural tests (`TestDispatchView*`) and no golden file.
- `--gate-scores` prints `verifyqueue.Row` values (`verifyqueue/queue.go:18`), built at
  `main.go:1209-1231` from `bucketAwaiting` and `queueFor` (`awaiting_bucket.go:259`, `:853`).
  It has a golden, `testdata/flowqueue/gate-scores.golden.json`, shared by `TestFlowQueue` and by
  two tests in `tools/desk`.
- `brief <key> --json` prints `briefInfo` and `briefInfoRow` (`briefinfo.go:61-87`). It has
  structural tests (`TestBriefInfo*`) and no golden file.
- Each of `--next-up` and `--gate-scores` accepts exactly one `--root` (`main.go:2054-2072`).
- Process-wide state on these paths: `spanOfControl` and `overflowThreshold` (`nextup.go:70-71`,
  set from flags), `activeDriveSet` (`nextup.go:81`) and `activeFindings` (`nextup.go:90`), both
  assigned by `runNextUp` (`dispatchqueue.go:98`, `:125`), `requireClaims` (`claims.go:21`), and
  `awaitOutcomeCache` (`awaiting_bucket.go:725`), which is filled once per process and never
  invalidated.
- `--next-up` also reads facts the board view does not hold: per-stream last touch from git,
  remote branches and claim decay (`claims.go:168-210`), delivery history, and drive manifests
  (`dispatchqueue.go:100-137`).
- Eligibility is computed inside `nextUp` (`nextup.go:915`) and again per drive
  (`drivefrontier.go:172`).
- Timings on this repository's tree with the released v1.0.84 binary: `--gate-scores` 10.8 s,
  `--next-up` 18.4 s, `brief --json` 0.02 s. statusgen/44 addresses the hydration share of those.

layering: a pure package `statusgen/board/queue` holds the ranking and classification, with
every setting passed in an options value and every outside fact (last touch, claims, delivery
history, drive manifests, verify-outcome records) read through a facts port, as
`statusgen/evaluation` reads history through its `Repo` port. The command package implements
the port with its existing git and forge adapters and keeps the three flags as adapters. The
library launches no process and reads no environment.

single point of failure: unchanged output rests on the goldens. A second, independent layer is
the structural rule that the classification exists once: the moved functions carry their
semantic-owner marker in one place and the command package holds no second body.

design-fit:
  owner: statusgen
  contract: none — the three wire shapes are existing contracts and do not change; the queue classification keeps its one owner and moves with it
  retires: ["the process-wide Next-up settings and drive and findings variables, replaced by an options value", "the once-per-process verify-outcome cache, replaced by a loader-scoped one", "the second eligibility evaluation per drive, which reads the first result"]
  weight: "verbs 0, flags 0, refusals 0, rule-text lines 0"
  why-add: n/a

## Read first

- `docs/streams/statusgen/brief-39-multi-root-objective-reader.md`: the view and loader.
- `docs/streams/decisions/DR-desk-tools-45.md`, whole file.
- `docs/library-first.md` and `docs/shared-policy-consolidation-plan.md` lines 40 to 140.
- `statusgen/evaluation/coverage_repo.go` lines 36 to 74: the port pattern to follow.
- `tools/desk/internal/arch/arch.go` lines 11 to 27: the one-implementation rule and its markers.

## Ground rules

- Isolated branch and draft PR. No push to main, no merge, no release, no workflow trigger.
- Characterise first. Step 1 lands the missing goldens against the current code before any
  function moves, in its own commit, so every later commit is compared to a fixed baseline.
- The three flags, their exit codes and their output do not change. The one-root rule on the
  flags stays; only the library accepts several roots.
- One implementation. Move the ranking and classification; do not copy them.
- Unknown is not clean. When the facts port cannot read claims or history, the projection says
  so in the same fields the modes use today (`claimsKnown`, `claimsReason`, `queueValid`).
- Re-cut rule: if the facts port needs more than the five fact families Context lists, or if
  moving `nextUp` requires moving lint rule bodies, STOP and report NEEDS_CONTEXT.
- Stop at `implemented`. A different, non-implementing identity verifies and moves the brief on.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Characterise.** Add golden files for `--next-up` and `brief <key> --json` on a fixture
   tree, and a test for each that compares the command output byte for byte. Commit this before
   any other step.
2. **Options and facts.** Define the options value (span of control, overflow threshold, role
   claims, require-claims) and the facts port (last touch, claims and their known state,
   delivery history, drive manifests, verify-outcome records). Implement the port in the command
   package with the existing adapters.
3. **Move the projections.** Move the Next-up ranking, the awaiting classification and the
   brief-info mapping into `statusgen/board/queue` as functions of a board view, an options
   value and a facts port. They return the existing wire types. Evaluate eligibility once per
   call and reuse it for the drive frontier.
4. **Adapters.** `--next-up`, `--gate-scores` and `brief --json` load through statusgen/39's
   loader and call the library. Remove the old bodies and the process-wide variables they used.
5. **Several roots in one call.** The library accepts a view over several roots and returns
   rows carrying their repo, so a caller needs one load and one call. The flags keep their
   one-root refusal.
6. **Documents.** Document the three calls in `statusgen/README.md` and `docs/library-first.md`.
   Record in `docs/contracts.md` that the queue classification owner path moved. Add the
   changelog fragment.

## Interface contract

```
import "github.com/medici-finance/assay/statusgen/board/queue"

next, err := queue.NextUp(view, options, facts)       // the --next-up document, any number of roots
rows, err := queue.Awaiting(view, options, facts)     // []verifyqueue.Row, the --gate-scores rows
info, err := queue.BriefInfo(view, "<stream>/<NN>")   // the brief --json document
```

The JSON printed by the three flags is the existing shape, unchanged.

## Verify

Commands run from this repository root. The named tests are planned deliverables. Each row greps
the exact PASS line, so a selector that matches nothing fails the row.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestNextUpGolden$" . > "$check_output" && grep -F -- "--- PASS: TestNextUpGolden " "$check_output")` | exit 0; PASS line printed. `--next-up` output on the fixture equals the golden committed in Task step 1 byte for byte |
| 2 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestFlowQueue$" . > "$check_output" && grep -F -- "--- PASS: TestFlowQueue " "$check_output")` | exit 0; PASS line printed. `--gate-scores` output still equals the existing shared golden `testdata/flowqueue/gate-scores.golden.json` |
| 3 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestBriefInfoGolden$" . > "$check_output" && grep -F -- "--- PASS: TestBriefInfoGolden " "$check_output")` | exit 0; PASS line printed. `brief <key> --json` output on the fixture equals the golden committed in Task step 1 byte for byte |
| 4 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestQueueLibraryParity$" . > "$check_output" && grep -F -- "--- PASS: TestQueueLibraryParity " "$check_output")` | exit 0; PASS line printed. For each of the three projections the library result, encoded, equals the command output on the same fixture |
| 5 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -race -count=1 -v -run "^TestQueueTwoRoots$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestQueueTwoRoots " "$check_output")` | exit 0; PASS line printed. One call over a two-root view returns the union of the two single-root results with each row's repo set; two calls with different options run at once under the race detector and neither sees the other's settings |
| 6 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestQueueFactsUnknown$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestQueueFactsUnknown " "$check_output")` | exit 0; PASS line printed. With a facts port that cannot read claims, the Next-up document sets `claimsKnown` false with a reason and returns the unfiltered rows; with unreadable verify-outcome records an awaiting row is marked not valid, never dropped |
| 7 | check:ci +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestQueuePurity$" . > "$check_output" && grep -F -- "--- PASS: TestQueuePurity " "$check_output")` | exit 0; PASS line printed. The structural check of statusgen/39, run over `statusgen/board/queue`, finds no package-level mutable variable, environment read or process launch, and the command package holds no second body of the moved functions; run over statusgen/39's planted fixture it still reports the plant |
| 8 | check:ci +neighbour | `(cd statusgen && GOWORK=off go test -count=1 ./...) && (cd tools/desk && GOWORK=off go test -count=1 ./cmd/deskboard/... ./cmd/verifyloop/...)` | exit 0; every existing statusgen test passes, and the two `tools/desk` tests that read the shared `--gate-scores` golden still pass |
| 9 | check +flow | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-42" .) && "${TMPDIR:-/tmp}/sg-42" --root . --gate-scores > "${TMPDIR:-/tmp}/gate-42.json" && grep -F -o '"queueVersion":1' "${TMPDIR:-/tmp}/gate-42.json" && "${TMPDIR:-/tmp}/sg-42" brief statusgen/42 --json --root . > "${TMPDIR:-/tmp}/brief-42.json" && grep -F -o '"key":"statusgen/42"' "${TMPDIR:-/tmp}/brief-42.json"` | exit 0; on this repository's own tree the awaiting queue prints versioned rows and the brief lookup resolves this brief (flow row through loader, library and adapter) |
| 10 | gate:model +dereference | `grep -n -F -e "board/queue" statusgen/README.md docs/library-first.md docs/contracts.md` | exit 0; at least one line from each of the three files. The reviewer then reads the matched sections: the README documents the three calls; `docs/contracts.md` names the moved owner path; neither claims a changed wire shape |
| 11 | check | `"${TMPDIR:-/tmp}/sg-42" --root . --lint` | exit 0; output contains "LINT: PASS" (the binary built in row 9) |
| 12 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && "${TMPDIR:-/tmp}/sg-42" --root . --consumers --brief statusgen/42 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0, post-merge: `ASSAY_DELIVERY_SHA` is the merged delivery commit and `ASSAY_DELIVERY_BASE_SHA` its first parent. The report corroborates each `consumers:` line above with no unchecked obligation; record its exact counts |

| Failure mode of the work | Caught by |
|---|---|
| A row's order, score or field changes when the ranking moves | rows 1 to 3: goldens committed before the move |
| The library and the command diverge later | row 4 (parity) and row 7 (no second body) |
| A setting from one caller leaks into another's result | row 5 (concurrent calls) and row 7 (no package-level state) |
| Unreadable claims turn into an empty, healthy-looking queue | row 6 |
| The shared golden's other readers break | row 8 |
| Whether five fact families is the right port boundary | review-only: the re-cut rule stops the work if it is not |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Not a core-system surface: the
projections rank and classify planning rows and write nothing. Reviewer records verdict + date
in the stream README table.
