---
brief: assay:assay:statusgen:41
title: 'Objective projection over the board view: critical path and ranked moves'
why: "The trace-path-to-live skill answers \"what stands between us and this objective, and who moves next\" by calling several single-root readers and stitching their output in prose. The stitch, the critical path and the move ranking live in a skill body where no test can hold them. With the board view of statusgen/39 in place, the same answer becomes one pure function over that view and one command, so it is deterministic and testable and the skill is left as a thin renderer."
wave: 3
depends: ["statusgen/39"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [4107]
schema: brief-v2
version: 1
authored: 2026-11-15 by statusgen newbrief
sources:
  - "statusgen/39 version 1 (22 October 2026): this brief carries its objective resolution, critical path, action ranking and skill swap forward unchanged in intent; statusgen/39 version 2 now builds only the multi-root board view those steps read"
  - "#4107: the issue. It asks for an objective skill (v1, since merged as plugins/assay/skills/trace-path-to-live) and names a first-class multi-root objective reader that emits {nodes, edges, critical_path, actions} as JSON as v2"
  - "docs/dependency-graph-design.md §3.3, §3.4 and §3.7: the reference grammar, the gates/feathers edge types, and the three-state eligibility verdict reused unchanged"
  - "statusgen/eligibility.go, statusgen/evaluation/eligibility.go, statusgen/drivefrontier.go, statusgen/awaiting_bucket.go: the verdicts and states this projection reads and never recomputes"
  - freshness-checked 2026-11-14 @ 876ca4bca (origin/main)
exec-tier: strong
exec-tier-why: "The projection joins eligibility, frontier and queue facts across roots and must keep each one's could-not-check state intact through the join."
domain: complicated
outcome: none
consumers:
  - "statusgen/board objective projection, the statusgen --objective command adapter, schemas/objective-view-v1.json, statusgen/README.md, docs/contracts.md: follow-up statusgen/41"
  - "plugins/assay/skills/trace-path-to-live and its generated .cursor copy: follow-up statusgen/41 (Task step 5 swaps the skill's stitching onto this projection)"
  - "goal-text to objective resolution: out-of-scope (a plain-language goal stays the skill's job; the projection takes a brief id or a drive slug only)"
---

# Brief 41 — Objective projection over the board view: critical path and ranked moves

## Context

files: `statusgen/board/objective.go` (planned), `statusgen/board/objective_test.go` (planned), `statusgen/objective_adapter.go` (planned), `statusgen/main.go`, `statusgen/testdata/board/` (the two fixture roots of statusgen/39, extended), `schemas/objective-view-v1.json` (planned), `statusgen/README.md`, `docs/contracts.md`, `plugins/assay/skills/trace-path-to-live/SKILL.md`, `.cursor/skills/trace-path-to-live/SKILL.md` (generated copy), `changelog/statusgen-objective-projection.md` (planned).

facts (876ca4bca, 14 November 2026):
- statusgen/39 delivers `statusgen/board` (the view, canonical identifiers
  `<repo-alias>:<stream>/<NN>`, cross-root `depends:`/`gates:`/`feathers:` edges with a
  three-state value) and `statusgen/board/tree` (the loader). This brief adds no loading.
- The v1 objective skill has merged as `plugins/assay/skills/trace-path-to-live/SKILL.md`. It
  calls the single-root readers (`--launch`, `--graph`, `--eligibility`, `--gate-scores`) and
  stitches them in prose.
- `EvaluateEligibility` (`statusgen/evaluation/eligibility.go:179`) gives each brief `eligible`,
  `held` or `eligible-with-notice`, each hold and notice three-state. It never reads the forge,
  so an `#<issue>` edge is always could-not-check.
- The drive frontier (`drivefrontier.go`: `briefFrontierState` at line 81, `driveFrontier` at
  line 164) classifies each drive item as done, in-flight, ready or blocked-on (item, review,
  operator-act, needs-brief).
- `awaiting_bucket.go` (`bucketAwaiting` at line 259, `queueFor` at line 853) maps an awaiting
  brief to a queue owner and a next act.
- `grep -rn -e "critical_path" -e "--objective" statusgen/` finds nothing: no reader computes a
  critical path or takes an objective.
- Effort weights already used for throughput are S=1, M=3, L=8.

layering: the closure, the critical path and the ranking are pure functions in `statusgen/board`
over a view plus per-node facts (eligibility verdict, frontier state, queue owner and next act)
passed in as an argument. The command package supplies those facts from its existing helpers and
prints JSON. No forge call, no network, no write. When statusgen/42 ships, the queue facts come
from its library call instead; the projection's signature does not change.

design-fit:
  owner: statusgen
  contract: objective-view-v1 (planned; register its semantic owner in docs/contracts.md)
  retires: ["the stitching, critical-path and ranking prose in plugins/assay/skills/trace-path-to-live, which step 5 replaces with one call"]
  weight: "verbs 0, flags +1 (--objective), refusals 0, rule-text lines 0, and the skill body shrinks"
  why-add: "No existing reader computes a critical path or ranks moves. Extending --launch was considered and rejected: it would change an established single-root text output."

## Read first

- #4107, the whole issue body, and `plugins/assay/skills/trace-path-to-live/SKILL.md`. The
  skill's move list defines the five role buckets this projection must reproduce.
- `docs/streams/statusgen/brief-39-multi-root-objective-reader.md`: the view this reads.
- `docs/dependency-graph-design.md` §3.3, §3.4 and §3.7.
- `docs/three-state-instrument-rule.md`.

## Ground rules

- Isolated branch and draft PR. No push to main, no merge, no release, no workflow trigger.
- Read-only and offline. No forge process, no network call, no write to any root or to STATUS.md.
- Reuse, do not fork: eligibility, frontier and queue ownership are read from their existing
  owners and never recomputed here.
- Never collapse could-not-check. A could-not-check node stays on the path it belongs to, and the
  answer is marked incomplete.
- Stop at `implemented`. A different, non-implementing identity verifies and moves the brief on.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Objective resolution.** Add `statusgen --objective <ref>`, where `<ref>` is a brief
   reference (`<stream>/<NN>` or `<repo-alias>:<stream>/<NN>`) or `drive:<slug>` (resolved
   through the drive manifest's items). Roots are every `--root` plus every `--sibling-root` and
   `DESK_ROOTS` entry, loaded once through statusgen/39's loader. A reference that resolves
   nowhere exits 2 and names the reference. A plain-language goal is not accepted.
2. **Closure.** From the objective, take the transitive closure over `depends:`, `gates:` and
   `feathers:` edges in the view, across roots. Each node carries its stream status, gate,
   eligibility verdict with holds and notices, frontier state, and its root.
3. **Critical path.** Over the unfinished subgraph (status not `done` or `verified`), compute
   the longest effort-weighted chain that ends at the objective (S=1, M=3, L=8). Break ties on
   total node count, then on lexical canonical identifier. If any node on a candidate chain is
   could-not-check, include it and set `critical_path.complete: false`.
4. **Per-role action ranking and one document.** For each unfinished node, emit one action:
   an eligible `todo` brief goes to `worker`; an open PR or awaiting review goes to `review`
   (owner and next act from the queue facts); `implemented` goes to `verify`; a human gate, a
   `needs-decision`, or a frontier `blocked-on: operator-act` goes to `driver`; a frontier
   `needs-brief` or a could-not-check authoring gap goes to `desk`. Rank by critical-path
   membership, then by the number of unfinished nodes the action unblocks, then by canonical
   identifier. Emit `{schema: "objective-view-v1", objective, roots, nodes, edges,
   critical_path, actions}` as compact JSON, described by `schemas/objective-view-v1.json` and
   registered in `docs/contracts.md`. Exit codes: 0 complete; 3 partial; 2 unresolvable
   objective or unreadable home root. Document the mode in `statusgen/README.md`.
5. **Thin renderer.** Replace the stitching, critical-path and ranking steps of
   `plugins/assay/skills/trace-path-to-live/SKILL.md` with one `statusgen --objective` call. The
   skill keeps plain-language goal resolution and rendering. Regenerate the `.cursor` copy with
   the installer's check mode so the generated-copy gate stays green. Add the changelog fragment.

## Interface contract

```
statusgen --root <home> [--root <path> ...] [--sibling-root <owner>/<repo>=<path> ...] \
          --objective <stream>/<NN> | <repo-alias>:<stream>/<NN> | drive:<slug>
```

Output (`objective-view-v1`, abridged and wrapped here for reading; the command prints it compact):

```json
{"schema":"objective-view-v1",
 "objective":{"ref":"…","kind":"brief|drive"},
 "roots":[{"repo":"…","path":"…","readable":true}],
 "nodes":[{"id":"<alias>:<stream>/<NN>","status":"…","gate":"…","eligibility":"…","frontier":"…","root":"…","reason":"…"}],
 "edges":[{"from":"…","to":"…","kind":"depends|gates|feathers","type":"build-dep|ordering-gate|…","state":"satisfied|unsatisfied|could-not-check"}],
 "critical_path":{"nodes":["…"],"weight":0,"complete":true},
 "actions":[{"rank":1,"role":"worker|review|verify|driver|desk","node":"…","act":"…","on_critical_path":true}]}
```

## Verify

Commands run from this repository root. The named tests are planned deliverables. Each row greps
the exact PASS line, so a selector that matches nothing fails the row.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveClosure$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestObjectiveClosure " "$check_output")` | exit 0; PASS line printed. On the two fixture roots a node in the second root is reached from an objective in the first through a cross-root `gates:` edge and through a cross-root `feathers:` edge, and a brief outside the closure is absent |
| 2 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveCriticalPath$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestObjectiveCriticalPath " "$check_output")` | exit 0; PASS line printed. The effort-weighted longest unfinished chain is chosen over a longer-by-count lighter chain; a tie resolves by the documented order; a done node never appears on the path |
| 3 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveActionRanking$" ./board/... > "$check_output" && grep -F -- "--- PASS: TestObjectiveActionRanking " "$check_output")` | exit 0; PASS line printed. Each of the five roles (worker, review, verify, driver, desk) gets its action from a fixture node in that state, and critical-path actions rank first |
| 4 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveCouldNotCheck$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveCouldNotCheck " "$check_output")` | exit 0; PASS line printed. An `#<issue>` edge, an unpublished alias and a missing sibling root each produce a could-not-check node with a reason, `critical_path.complete` is false, and the command exits 3 |
| 5 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveDriveResolve$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveDriveResolve " "$check_output")` | exit 0; PASS line printed. `drive:<slug>` resolves to the manifest's items and the node frontier states equal `driveFrontier`'s for the same fixture; an unknown slug exits 2 |
| 6 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveSchemaGolden$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveSchemaGolden " "$check_output")` | exit 0; PASS line printed. The command output on the two-root fixture validates against `schemas/objective-view-v1.json`, equals the checked-in golden, and is byte-identical across two runs |
| 7 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveOffline$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveOffline " "$check_output")` | exit 0; PASS line printed. Run with `--forge` set and a forge seam that fails the test on any call: no forge or network call is made, and no file under either root changes |
| 8 | check:ci +neighbour | `(cd statusgen && GOWORK=off go test -count=1 ./...)` | exit 0; every existing statusgen test still passes, including the `TestBoard*` tests of statusgen/39 and the `TestLaunch*`, `TestGraph*`, `TestEligibility*`, `TestDriveFrontier*` and `TestDriveSnapshot*` tests, so the readers this projection consumes are unchanged |
| 9 | check +flow +dereference | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-41" .) && { "${TMPDIR:-/tmp}/sg-41" --root . --objective statusgen/41 > "${TMPDIR:-/tmp}/objective-41.json"; rc=$?; } ; test "$rc" -ne 2 && test "$rc" -ne 1 && grep -qF '"schema":"objective-view-v1"' "${TMPDIR:-/tmp}/objective-41.json" && grep -F -o '"id":"assay:statusgen/39"' "${TMPDIR:-/tmp}/objective-41.json"` | exit 0; the command runs on this repository's own tree with this brief as the objective, and the document carries the schema tag and a node for statusgen/39, this brief's declared dependency (flow row; dereferences this brief's own frontmatter). Record the exit code |
| 10 | check:ci | `grep -F -- "statusgen --objective" plugins/assay/skills/trace-path-to-live/SKILL.md` | exit 0; the matched line shows the skill calling the projection (Task step 5) |
| 11 | gate:model +dereference | `grep -n -F -e "--objective" statusgen/README.md && grep -n -F "objective-view-v1" docs/contracts.md && test -f schemas/objective-view-v1.json` | exit 0; the reviewer then reads the matched sections: the README documents `--objective`, its roots, output and exit codes; `docs/contracts.md` names statusgen as owner of `objective-view-v1`; the schema and the README agree on every top-level key |
| 12 | check | `"${TMPDIR:-/tmp}/sg-41" --root . --lint` | exit 0; output contains "LINT: PASS" (the binary built in row 9) |
| 13 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && "${TMPDIR:-/tmp}/sg-41" --root . --consumers --brief statusgen/41 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0, post-merge: `ASSAY_DELIVERY_SHA` is the merged delivery commit and `ASSAY_DELIVERY_BASE_SHA` its first parent. The report corroborates each `consumers:` line above with no unchecked obligation; record its exact counts |

| Failure mode of the work | Caught by |
|---|---|
| The projection recomputes eligibility or queue ownership and disagrees with their owners | rows 3 and 5 compare against the owners' output on the same fixture |
| A could-not-check node is dropped, so the path looks complete | row 4 |
| The critical path counts nodes instead of effort | row 2 |
| The skill still stitches in prose | row 10 |
| The projection reaches the forge | row 7 |
| Whether the five-role mapping matches how desks actually divide work | review-only: it is copied from the merged skill's move list, which the reviewer compares |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Not a core-system surface: the
projection reads a loaded view and writes nothing. Reviewer records verdict + date in the stream
README table.
