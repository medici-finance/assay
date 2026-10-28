---
brief: assay:assay:statusgen:39
title: 'Multi-root objective reader: prerequisite graph, critical path and ranked moves'
why: "The v1 objective skill of #4107 answers \"what stands between us and this objective, and who moves next\" by calling four separate readers, each scoped to one root, and stitching their output in prose. The stitch, the critical path and the move ranking then live in a skill body where no test can hold them. One read-only statusgen reader that emits {nodes, edges, critical_path, actions} as JSON across every configured root makes that answer deterministic and testable, and leaves the skill as a thin renderer."
wave: 1
depends: []
unblocks: []
feathers:
  - ref: "#4107"
    type: ordering-gate
    reason: "Task step 6 swaps the v1 objective skill onto this reader, so that skill must have merged first. Steps 1 to 5 do not need it, which is why this is a feather (a notice) and not a gate (a hold)."
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [4107]
schema: brief-v2
version: 1
authored: 2026-10-22 by statusgen newbrief (worker-desk authoring, #4107 part v2)
sources:
  - "#4107: the issue. It asks for an objective skill now (v1) that takes a brief id, a drive slug or a plain-language goal, walks depends and gates across every configured stream root including cross-repo gates, and outputs the prerequisite tree with its critical path, a ranked move list per role, and a rendered page. It names this reader as v2: a first-class multi-root objective mode in statusgen that emits {nodes, edges, critical_path, actions} as JSON so the skill becomes a thin renderer."
  - "docs/dependency-graph-design.md §3.3, §3.4 and §3.7: the reference grammar, the gates/feathers edge types, and the three-state eligibility verdict this reader reuses unchanged"
  - "statusgen/launch.go, statusgen/graph.go, statusgen/eligibility.go, statusgen/evaluation/eligibility.go, statusgen/drivefrontier.go, statusgen/drivesnapshot.go, statusgen/multiroot.go, statusgen/siblingmerge.go, statusgen/awaiting_bucket.go: the readers and helpers reused (see Context)"
  - freshness-checked 2026-10-22 @ 8f103c471 (origin/main)
exec-tier: strong
exec-tier-why: "The reader joins five existing readers across roots and must keep each one's three-state could-not-check semantics intact through the join; that needs design and consumer reasoning, not a mechanical port."
domain: complicated
outcome: none
id: 260bbb3c-5477-4967-8f07-6867b71195af
consumers:
  - "statusgen --objective CLI, schemas/objective-view-v1.json, statusgen/README.md, docs/contracts.md: follow-up statusgen/39"
  - "plugins/assay/skills/path-to-live: follow-up statusgen/39 (the v1 skill of #4107; Task step 6 swaps its stitching onto this reader)"
  - "goal-text to objective resolution: out-of-scope (a plain-language goal stays the skill's job; the reader takes a brief id or a drive slug only)"
---

# Brief 39 — Multi-root objective reader: prerequisite graph, critical path and ranked moves

**Wave:** 1 · **Depends on:** none in-repo; feather on #4107 (the v1 skill must merge before Task step 6) ·
**Unblocks:** the v1 skill's thin-renderer rewrite (Task step 6, done inside this brief) · **Effort:** L

## Context

files: `statusgen/objective.go` (planned), `statusgen/objective_test.go` (planned), `statusgen/testdata/objective/` (planned: two fixture roots and a repo-alias registry), `schemas/objective-view-v1.json` (planned), `statusgen/main.go`, `statusgen/README.md`, `docs/contracts.md`, `plugins/assay/skills/path-to-live/SKILL.md` (planned) (lands with #4107), `changelog/statusgen-objective-reader.md` (planned).

facts (8f103c471, 22 October 2026):
- `--launch` (`launch.go`, `launchTransitiveClosure`, `runLaunch(root, target)`) walks the transitive `depends:` closure of ONE target in ONE root and prints text. It does not follow `gates:` or `feathers:`, and has no JSON form.
- `--graph jsonl|dot` (`graph.go`, `buildGraph(root)`) exports one root's nodes and its contains/depends/unblocks/affects/sources/issues edges. It emits no `gates:` or `feathers:` edges and does not cross roots.
- `--eligibility [--json]` (`eligibility.go`, the pure `statusgen/evaluation.EvaluateEligibility`) gives every brief `eligible`, `held` or `eligible-with-notice`, each hold/notice three-state (`satisfied`/`unsatisfied`/`could-not-check`). It resolves a cross-repo reference only through `siblingLookup(root)`, which finds a sibling checkout by basename next to the root. It never reads the forge, so a `#<issue>` edge is always `could-not-check`.
- The drive frontier (`drivefrontier.go`, `briefFrontierState`, `driveFrontier`) already classifies each drive item as done, in-flight, ready or blocked-on (item, review, operator-act, needs-brief). `--drive-snapshot <slug>` (`drivesnapshot.go`) renders it as Markdown only.
- Repeatable `--root` (`multiroot.go`) runs the per-root pipeline once per root and refuses a stream name that two roots share (`crossRootProblems`). `--sibling-root` and `DESK_ROOTS` (`siblingmerge.go`, `parseSiblingRootSpecs`) parse `<owner>/<repo>=<path>` and today feed only the sibling-merge detector. Only repos named there are ever read.
- `awaiting_bucket.go` (`bucketAwaiting`, `queueFor`, `queueClassOf`) already maps an awaiting brief to a queue owner and a next act; `--gate-scores` emits them as `queueOwner` and `queueNextAct`.
- `grep -rn -e "critical_path" -e "--objective" statusgen/` finds nothing: no existing reader computes a critical path or takes an objective.

layering: the join, the critical path and the ranking are pure functions over the already-built per-root `[]*Stream`, the eligibility map and the frontier states. `objective.go` adds one CLI adapter that reads trees and prints JSON. No forge call, no network, no write: the reader is offline by construction, like `--eligibility`, and never touches STATUS.md.

design-fit:
  owner: statusgen
  contract: objective-view-v1 (planned; register its semantic owner in docs/contracts.md)
  retires: ["the stitching, critical-path and ranking logic in the v1 skill body (plugins/assay/skills/path-to-live), which step 6 replaces with one reader call"]
  weight: "one read-only CLI mode and one JSON schema; no new service, store, daemon or dispatch authority"
  why-add: "No existing reader crosses roots and follows gates and feathers together; extending --launch or --graph to do it would change their established single-root output contracts."

### Reused unchanged versus new

| Reused (called, not re-implemented) | New in this brief |
|---|---|
| Per-root stream load and the repeatable `--root` pipeline; `crossRootProblems` | **Multi-root stitch:** one node set across every root, with canonical fully qualified ids (`<repo-alias>:<stream>/<NN>`), and cross-root `depends:`/`gates:`/`feathers:` edges resolved through `docs/streams/graph-repos.yaml` plus the `--sibling-root`/`DESK_ROOTS` path map |
| `launchTransitiveClosure` for the in-root `depends:` walk | **Objective resolution:** a brief id or `drive:<slug>` becomes the closure root set; the closure also follows `gates:` and `feathers:`, across roots |
| `EvaluateEligibility` verdicts and their three-state holds/notices, per root | **Critical path:** the longest chain of unfinished prerequisites to the objective, effort-weighted (S=1, M=3, L=8, as throughput already weights them), with a fixed tie-break |
| `briefFrontierState` / `driveFrontier` states and the drive manifest | **Per-role action ranking:** each unfinished node becomes one move owned by worker, review, verify, driver or desk, ranked by critical-path membership, then by how many nodes it unblocks |
| `bucketAwaiting` / `queueFor` owner and next act for awaiting briefs | One JSON document (`objective-view-v1`) holding all four parts |
| `--graph` node/edge vocabulary (types and edge labels) | Exit codes for a partial (could-not-check) answer |

## Read first

- #4107: the whole issue body, and the v1 skill `plugins/assay/skills/path-to-live/SKILL.md` (planned) once it has merged. The skill's move list defines the five role buckets this reader must reproduce.
- `docs/dependency-graph-design.md` §3.3, §3.4 and §3.7: reference grammar, edge types, three-state verdicts.
- `docs/three-state-instrument-rule.md`: could-not-check is never collapsed into satisfied or empty.
- `statusgen/launch.go`, `statusgen/graph.go`, `statusgen/eligibility.go`, `statusgen/drivefrontier.go`, `statusgen/multiroot.go`, `statusgen/siblingmerge.go`, `statusgen/awaiting_bucket.go`.

## Ground rules

- Isolated branch and draft PR. No push to main, no merge, no release, no workflow trigger.
- Read-only and offline: no forge process, no network call, no write to any root or to STATUS.md. Never contact a cluster or production endpoint.
- Reuse, do not fork: call the existing functions in the left column above. Change one only where it must grow a return value, and keep its existing output byte-identical (Verify row 8).
- Never collapse could-not-check. An unreadable root, an unpublished alias or a forge-only target stays named in the output, and the answer is marked incomplete.
- Stop at `implemented`. A different, non-implementing identity verifies and moves the brief on.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT; do not guess.

## Task

1. **Objective resolution.** Add `statusgen --objective <ref>`, where `<ref>` is a §3.3 brief reference (`<stream>/<NN>` or `<repo-alias>:<stream>/<NN>`) or `drive:<slug>` (resolved through the drive manifest's items). Roots are every `--root` plus every `--sibling-root`/`DESK_ROOTS` entry. A reference that resolves nowhere exits 2 and names the reference. A plain-language goal is not accepted; it stays the skill's job.
2. **Multi-root stitch.** Starting from the objective, walk `depends:` (through `launchTransitiveClosure` per root), `gates:` and `feathers:` across roots. Resolve a repo alias through `docs/streams/graph-repos.yaml` and the sibling path map; when the map has no entry, fall back to `siblingLookup`'s basename rule. A target that still cannot be resolved (unpublished alias, absent checkout, `#<issue>`, cross-cell) becomes a node with `status: "could-not-check"` and its reason, never a dropped edge. Each node carries its stream status, gate, eligibility verdict with holds/notices, frontier state, and its root.
3. **Critical path.** Over the unfinished subgraph (status not `done`/`verified`), compute the longest effort-weighted chain that ends at the objective. Break ties on total node count, then on lexical canonical id, so the output is deterministic. If any node on a candidate chain is could-not-check, include it and set `critical_path.complete: false`.
4. **Per-role action ranking.** For each unfinished node, emit one action:
   - eligible `todo` brief → `worker` (dispatch);
   - open PR or awaiting review → `review` (owner and next act from `bucketAwaiting`/`queueFor`);
   - `implemented` → `verify`;
   - human gate, `needs-decision`, or a frontier `blocked-on: operator-act` → `driver`;
   - frontier `needs-brief` or a could-not-check authoring gap → `desk`.
   Rank actions within a role, and globally, by critical-path membership first, then by the number of unfinished nodes the action unblocks, then by canonical id.
5. **One JSON document.** Emit `{schema: "objective-view-v1", objective, roots, nodes, edges, critical_path, actions}`, described by `schemas/objective-view-v1.json` and registered in `docs/contracts.md`. `roots[]` lists each root with `readable: true|false`. Output is compact JSON (no indentation), one document per run. Exit codes: 0 complete; 3 when the answer is partial (any could-not-check node or unreadable root); 2 when the objective is unresolvable or the home root is unreadable. Document the mode in `statusgen/README.md`, and add a changelog fragment.
6. **Thin renderer.** Once the v1 skill of #4107 has merged (the `feathers:` edge above), replace its stitching, critical-path and ranking steps with one `statusgen --objective … --json` call. The skill keeps only plain-language goal resolution and rendering. If the skill has not merged when steps 1 to 5 are done, open the reader PR for them and leave this brief `in-progress` until a second PR lands step 6; Verify row 10 is run only after that.

## Interface contract

```
statusgen --root <home> [--root <path> ...] [--sibling-root <owner>/<repo>=<path> ...] \
          --objective <stream>/<NN> | <repo-alias>:<stream>/<NN> | drive:<slug>
```

Output (`objective-view-v1`, abridged and wrapped here for reading; the CLI prints it compact):

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

Commands run from this repository root. The named tests are planned deliverables. Each row greps the exact PASS line, so a selector that matches nothing fails the row.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveStitchRoots$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveStitchRoots " "$check_output")` | exit 0; PASS line printed. Two fixture roots: a cross-root `gates:` edge and a cross-root `feathers:` edge both appear as edges with canonical ids, and a node in the second root is reached from the objective in the first |
| 2 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveCriticalPath$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveCriticalPath " "$check_output")` | exit 0; PASS line printed. On the fixture the effort-weighted longest unfinished chain is chosen over a longer-by-count lighter chain; a tie resolves by the documented order; a done node never appears on the path |
| 3 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveActionRanking$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveActionRanking " "$check_output")` | exit 0; PASS line printed. Each of the five roles (worker, review, verify, driver, desk) gets its action from a fixture node in that state, and critical-path actions rank first |
| 4 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveCouldNotCheck$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveCouldNotCheck " "$check_output")` | exit 0; PASS line printed. A `#<issue>` edge, an unpublished alias and a missing sibling root each produce a could-not-check node with a reason, `critical_path.complete` is false, and the CLI exits 3 |
| 5 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveDriveResolve$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveDriveResolve " "$check_output")` | exit 0; PASS line printed. `drive:<slug>` resolves to the manifest's items and the node frontier states equal `driveFrontier`'s for the same fixture; an unknown slug exits 2 |
| 6 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveSchemaGolden$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveSchemaGolden " "$check_output")` | exit 0; PASS line printed. The CLI output on the two-root fixture validates against `schemas/objective-view-v1.json`, equals the checked-in golden, and is byte-identical across two runs |
| 7 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObjectiveOffline$" . > "$check_output" && grep -F -- "--- PASS: TestObjectiveOffline " "$check_output")` | exit 0; PASS line printed. Run with `--forge` set and a forge seam that fails the test on any call: no forge or network call is made, and no file under either root changes |
| 8 | check:ci | `(cd statusgen && GOWORK=off go test -count=1 ./...)` | exit 0; every existing statusgen test still passes, including the `TestLaunch*`, `TestGraph*`, `TestEligibility*`, `TestDriveFrontier*`, `TestDriveSnapshot*` and `TestMultiRoot*` tests of the reused readers, so `--launch`, `--graph`, `--eligibility` and `--drive-snapshot` output is unchanged (neighbour row) |
| 9 | check:ci +flow +dereference | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-39" .) && { "${TMPDIR:-/tmp}/sg-39" --root . --objective statusgen/39 > "${TMPDIR:-/tmp}/objective-39.json"; rc=$?; } ; test "$rc" -eq 3 && grep -qF '"schema":"objective-view-v1"' "${TMPDIR:-/tmp}/objective-39.json" && grep -F -o '"status":"could-not-check"' "${TMPDIR:-/tmp}/objective-39.json"` | exit 0; the reader runs on this repository's own tree with this brief as the objective, exits 3 (partial) because this brief's feather on #4107 is a forge-only target the offline reader cannot resolve, and the document carries the schema tag and a could-not-check node (flow row; dereferences this brief's own frontmatter) |
| 10 | check:ci | `grep -F -- "statusgen --objective" plugins/assay/skills/path-to-live/SKILL.md` | exit 0; the skill calls the reader (Task step 6) |
| 11 | gate:model +dereference | `grep -n -F -e "--objective" statusgen/README.md && grep -n -F "objective-view-v1" docs/contracts.md && test -f schemas/objective-view-v1.json` | exit 0; the reviewer then reads the matched sections: the README documents `--objective`, its roots, output and exit codes; `docs/contracts.md` names statusgen as owner of `objective-view-v1`; the schema and the README agree on every top-level key |
| 12 | check | `"${TMPDIR:-/tmp}/sg-39" --root . --lint` | exit 0 (the binary built in row 9) |
| 13 | check +flow | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && "${TMPDIR:-/tmp}/sg-39" --root . --consumers --brief statusgen/39 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0, post-merge: `ASSAY_DELIVERY_SHA` is the merged delivery commit and `ASSAY_DELIVERY_BASE_SHA` its first parent. The report corroborates each `consumers:` line above (reader CLI, schema, README, contracts entry, skill swap) with no unchecked obligation; record its exact counts |

## Evidence

<!-- Appended at verification time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). This authoring PR runs
     no implementation check. -->

## Review

Gate: model (from frontmatter; all four risk answers no). Not a core-system surface: no settlement, identity or ledger path is touched, so no single-point-of-failure line is owed. The reviewer records verdict and date in the stream README table.
