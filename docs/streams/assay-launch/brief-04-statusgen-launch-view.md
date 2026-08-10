---
brief: assay-launch/04
title: statusgen --launch — launch-readiness rollup view
why: >-
  human:<name> wants the moving-parts document "automated by statusgen." The stream README + cross-stream
  depends already make the board honest, but reading readiness means eyeballing several stream
  tables. A --launch view prints one panel — every dependency of the go-live gate and its live
  status — so anyone can answer "what is still blocking launch?" in one command, the same way
  --dora/--trend/--roadmap already give single-glance views of their slice.
wave: 0
depends: []
unblocks: ["assay-launch/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus session (human:<name>'s publish-the-methodology direction)
sources: ["human:<name> 2026-07-17: 'a document that shows all the moving parts… ideally automated by statusgen as we go through it'", "human:<name> 2026-07-17 (follow-up): 'the launch [view] sounds like a good idea longer term. we should probably do it. I don't think it's a major ask'", "tools/statusgen/main.go (flag.Bool mode-flag pattern — --dora/--trend/--roadmap are the models to mirror)", "tools/statusgen/nextup.go (buildRevDeps + eligibility over global stream/NN depends — the graph this view reads)", "tools/statusgen/brieffile.go:672 checkRef (cross-stream depends already resolve globally)", "freshness-checked 2026-07-17 @ cc7fd623: no --launch flag exists in tools/statusgen/main.go"]
exec-tier: strong
exec-tier-why: (b)/(c) Go against statusgen internals, reasoning over the cross-stream dependency graph — a subtle error (mis-resolved ref, wrong status roll-up) would print a false "ready".
---

# Brief 04 — statusgen --launch readiness rollup

**In-repo only** (`../assay-toolkit/statusgen/`). No cross-repo, no site changes.

## Context
files: `../assay-toolkit/statusgen/launch.go` (planned) — new emitter; `../assay-toolkit/statusgen/main.go` — flag +
dispatch; `../assay-toolkit/statusgen/launch_test.go` (planned) — new test; the statusgen flag docs
(wherever `--dora`/`--trend` are documented — grep for it)
facts:
- **Pattern to mirror:** `--dora`, `--trend`, `--roadmap` are `flag.Bool` sub-commands in
  `main.go` that dispatch to their own emitter file and exit; they do NOT read/write STATUS.md.
  `--launch` is the same shape.
- **The graph already resolves.** `brieffile.go:672 checkRef` resolves typed `<stream>/<NN>`
  `depends:` against a GLOBAL map of all streams; `nextup.go buildRevDeps` builds the reverse
  graph over global `stream/NN` keys. So the view can walk the `assay-launch/05` gate's
  transitive `depends:` across streams with the existing model — no new parsing.
- **What it prints:** for the launch gate (`assay-launch/05`, or a `--launch <stream>/<NN>`
  arg defaulting to it), the transitive `depends:` closure as a readiness table — each
  dependency's `stream/NN`, title, status, and a ✅/⏳/❌ readiness mark — plus a one-line
  verdict ("READY: all N deps done" / "BLOCKED: k of N deps not done: <list>"). Diagnostic
  output only; never a gate, never writes STATUS.md.
- **Honest scope:** it reports committed status from the brief tables — the same source Next-up
  trusts; it does not verify the deploy happened. Say "readiness per the board," not "live."

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- STATUS.md is single-writer (main CI) — `--launch` must not read or write it.

## Task
1. Add `launchMode := flag.Bool("launch", false, ...)` + a `--launch-target` string
   (default `assay-launch/05`) to `main.go`; dispatch to `emitLaunch` and exit.
2. Implement `launch.go`: resolve the target brief, walk its transitive `depends:` closure over
   the global stream graph, print the readiness table + verdict. Cycle-safe (reuse the visited-
   set approach in nextup.go).
3. `launch_test.go`: a fixture stream set under `testdata/` where the target has mixed done/not-
   done deps; assert the table lists every transitive dep and the verdict reports the blockers.
4. Document the flag alongside `--dora`/`--trend` in the statusgen flag docs.
5. Update the stream-README row.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run Launch; echo $?` | 0 (new test passes) |
| 2 | `statusgen --root . --launch 2>&1 \| grep -ciE -e READY -e BLOCKED` | ≥1 (verdict prints) |
| 3 | `statusgen --root . --launch 2>&1 \| grep -c "assay-product/05"` | ≥1 (transitive cross-stream dep appears) |
| 4 | `statusgen --root . --launch >/dev/null; test -f STATUS.md && git -C . diff --quiet -- STATUS.md; echo $?` | 0 (did NOT write STATUS.md) |
| 5 | `statusgen --root . --lint; echo $?` | 0 (flag addition doesn't break lint) |
| 6 | `go vet ./tools/statusgen/; echo $?` | 0 |

## Evidence
<!-- one row per Verify item, filled by a non-implementer. -->

Non-implementer re-run — 2026-07-20, against `60e3b637` (origin/main).

| # | Command | Exit | Output | Runner |
|---|---------|------|--------|--------|
| 1 | `go test ./tools/statusgen/ -run Launch; echo $?` | 0 | `ok  github.com/medici/statusgen  0.380s` | opus-verifier (non-implementer) |
| 2 | `go run ./tools/statusgen --root . --launch 2>&1 \| grep -ciE -e READY -e BLOCKED` | 0 | `1` (verdict line: `BLOCKED: 9 of 23 deps not done: ...`) | opus-verifier (non-implementer) |
| 3 | `go run ./tools/statusgen --root . --launch 2>&1 \| grep -c "assay-product/05"` | 0 | `2` | opus-verifier (non-implementer) |
| 4 | `go run ./tools/statusgen --root . --launch >/dev/null; test -f STATUS.md && git -C . diff --quiet -- STATUS.md; echo $?` | 0 | STATUS.md unchanged | opus-verifier (non-implementer) |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | advisory NOTICEs only | opus-verifier (non-implementer) |
| 6 | `go vet ./tools/statusgen/; echo $?` | 0 | clean | opus-verifier (non-implementer) |

**VERIFY: PASS (6/6 rows).**

## Review
Gate: model. Reviewer confirms the view reads the global dep graph correctly, is cycle-safe,
never writes STATUS.md, and its readiness verdict matches the fixture's expected blockers.
