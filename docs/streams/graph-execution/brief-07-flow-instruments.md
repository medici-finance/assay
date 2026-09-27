---
brief: assay:assay:graph-execution:07
title: Flow instruments — service/wait split, CI-slot saturation, gate catch/override
why: >-
  Today's bottleneck reading is WIP multiplied by how long an item has sat in its current
  stage. That number cannot tell "the verifier is busy" from "the item waited for a human",
  so every capacity decision made from it is a guess. Before anyone widens dispatch or adds
  an admission cap, the fleet needs the four durations that separate scheduling delay from
  service time, plus the two numbers that distinguish throughput from churn: how saturated
  the CI slots are, and whether gates are catching defects or being waived.
wave: 1
depends: ["graph-execution/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/spec.md §2.6 (the seven durations, CI-slot saturation, gate catch/override, environment identity on every comparison) and §3 (stage-age heuristic row)"
  - "graph-execution/01 — `statusgen --eligibility` and the eligible-at instant this brief measures FROM"
  - "statusgen/history.go (the historian: ts is the REGEN run that observed the change, not the event), statusgen/briefefficiency.go (--flow-efficiency: the existing touch/wait PROXY from historian dwell, gtSmallN thin-data rule), statusgen/gatetelemetry.go (--gate-telemetry: override-rate / catch-rate / ceremonial-gate families, exit-code contract 0/1/2/3), statusgen/doratiming.go (.dora-timing.jsonl: PR commit→merge lead time and restore episodes, single-writer), statusgen/bottleneck.go (WIP × stage-age, NOT replaced here)"
  - "Rajamohan, trajectory consistency vs aggregate task performance, https://youtu.be/BIBDhLDgMdE?t=434 (measure valid outcomes and constraints, not obedience to one sequence)"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `grep -rn 'ci_slot\\|ci-slot\\|slot saturation' statusgen/*.go docs/*.md` returns zero hits; `--flow-efficiency` and `--gate-telemetry` exist (statusgen/main.go) and are REUSED, not re-implemented"
exec-tier: strong
exec-tier-why: "(b) correctness depends on joining three independent logs (.history.jsonl, .dora-timing.jsonl, forge checks/reviews) on one brief identity without fabricating an interval; (a) which observed instant stands in for each unrecorded event is a design decision the facts do not pre-specify"
domain: complicated
consumers:
  - "docs/iso9001-mapping.md (the 9.1.1 monitoring-and-measurement row that names --bottleneck / --gate-telemetry as monitoring instruments): fixed-here (this brief's implementation adds the --flow instrument to that row)"
  - "statusgen/bottleneck.go (the stage-age score): out-of-scope (deliberately NOT replaced — this brief joins it with a second reading; replacing the score is a later decision from data this brief produces)"
version: 1
id: 76e53f95-97ce-4244-9a45-e88cb56244ec
---

# Brief 07 — Flow instruments

## Context
files: `statusgen/flow.go` (planned), `statusgen/flow_test.go` (planned), `statusgen/testdata/flow/` (planned) — fixture historian + timing + checks JSON, `example-org/*` names only), `statusgen/main.go` (the `--flow` flag and its `--json` / `--since` / `--until` reuse), `statusgen/gatetelemetry.go` (read-only; its report is CONSUMED for the catch/override families), `statusgen/briefefficiency.go` (read-only; its completed-dwell walker is REUSED), `docs/iso9001-mapping.md` (the monitoring-instrument row gains `--flow`), `docs/dependency-graph-design.md` (one paragraph: which instants the lifecycle graph now records), `changelog/graph-execution-07-flow-instruments.md` (planned)

facts:
- **What timestamps exist today (2026-09-16, this repo's main).** `.history.jsonl` records `ts` as the regen run that OBSERVED a status change, never the event itself (`statusgen/history.go` header). `.dora-timing.jsonl` records per-PR commit→merge lead time and restore episodes, single-writer from main's regen (`statusgen/doratiming.go`). There is NO recorded work-start event and NO recorded eligible-at instant (`statusgen/briefefficiency.go` lines 3–11 say so and ship a proxy). Brief 01 adds the eligible-at instant to the evaluator's output; this brief is the first consumer of it.
- **Four durations, and the instant each is measured between.** eligible-to-start delay = 01's eligible-at → the historian's first `in-progress` observation; active work time = `in-progress` dwell (the existing --flow-efficiency touch proxy, reused verbatim); external wait = `implemented` dwell that overlaps an open human-gate issue or a `blocked` row, else counted as verification wait; verification time = `implemented` → `verified` dwell. Every interval is counted only when COMPLETE (a later record closes it), the same no-fabricated-interval discipline `briefefficiency.go` and `doratiming.go` already keep.
- **Observation, not event: say so in the output.** Because `ts` is a regen instant, every duration carries a `resolution` field = the median regen cadence observed in the window; a duration shorter than one cadence is reported as `< resolution`, never as a number.
- **CI-slot saturation** = (merged PRs per day × median CI wall-clock per PR) ÷ available CI hours per day. The first two terms come from forge check-run timestamps, read ONLY when `--forge` is passed — the existing offline-by-default opt-in this repo already defines (`statusgen/main.go`'s `forgeMode` flag: without `--forge` statusgen starts no forge process and makes no network call, and every forge-backed check reports `could-not-check` as itself). The gate is the flag, never credential presence: `--flow` without `--forge` reports `could-not-check` for this term even when a token is set in the environment. When `--forge` IS set, the read goes through the sanctioned seam — `ghfetch.go`'s `ghClient` (the desk-tools `deskread` verb over `net/http`), never a `gh` subprocess — and `--flow --forge` prints the read target (`owner/repo`) before first contact. The divisor is a declared value in the report's input (`--ci-hours-per-day`, no default — absent ⇒ `could-not-check`, never an invented number).
- **Gate catch/override** = the `--gate-telemetry` override-rate and catch-rate families, CONSUMED from `gatetelemetry.go`'s report over the same window, re-emitted under `--flow` with their existing three-state semantics and exit codes (0 / 1 malformed / 3 could-not-check). Nothing about a gate is re-measured here.
- **Environment identity on every number:** `statusgen_version` (the build's own version string, as `docs/telemetry.md` already defines it), the window bounds, the historian's regen cadence, and `model_versions: could-not-check` unless a run-record source (a later brief) supplies them. A number without an environment stamp is a lint PROBLEM in the fixture test.
- **The stage-age score is not replaced.** `--bottleneck` keeps its current output; `--flow` is a second reading beside it. A reader that wants one number is told there is not one.
- Single-point-of-failure note: the ONE control is the fixture test that asserts every emitted duration is closed by a later record. Second, independent layer: the `resolution` field, computed in a different function from a different input (regen cadence), makes a sub-cadence number visibly unreliable even if the closing check regresses. Third, out-of-band: `--gate-telemetry`'s own three-state exit code propagates, so an unread source cannot read as zero.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: fixtures and tests use `example-org/*` names; no adopter's numbers appear anywhere.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `statusgen --flow [--forge] [--json] [--since --until] [--ci-hours-per-day N]`: per brief, the four durations from the facts, each with `status: measured | < resolution | could-not-check` and the instants it was measured between; a `resolution` field per window; fleet-level medians only over `measured` rows and only when the count ≥ `gtSmallN` (reuse the constant).
2. `ci_slot_saturation` and `gate_catch_override` blocks per the facts, three-state, with the source each read (the forge check-run read, via `ghfetch.go`'s `ghClient`, gated on `--forge` and never on credential presence alone; `gatetelemetry` report) or the reason it could not.
3. `environment` block on every report; the fixture test fails the report if it is absent or any field is empty rather than `could-not-check`.
4. Docs: the `docs/iso9001-mapping.md` instrument row; one paragraph in `docs/dependency-graph-design.md` naming the instants the lifecycle graph records after 01 + this brief, and the ones it still does not (a true work-start event).
5. Changelog fragment.

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | cd statusgen && go test -run 'TestFlow' ./... | exit 0 |
| 2 | check:ci +mutation | cd statusgen && go test -run 'TestFlowRefusesOpenInterval' ./... | exit 0; the fixture holds a brief whose current stage has no closing record and the report emits `could-not-check` for it, never a duration |
| 3 | check +flow +dereference | cd statusgen && go run . --root testdata/flow/window-a --flow --json | jq -e '.environment.statusgen_version != "" and .resolution.seconds > 0 and (.briefs[] | select(.id=="example-app/02") | .eligible_to_start.status=="measured")' | exit 0 — the numbers come from the fixture's records, not from the report's own defaults |
| 4 | check +neighbour | cd statusgen && go run . --root testdata/flow/window-a --bottleneck | grep -c 'constraint' | count >= 1 — the existing stage-age report still renders unchanged beside the new one |
| 5 | check | cd statusgen && env -u GH_TOKEN -u GITHUB_TOKEN go run . --root testdata/flow/window-a --flow --forge --json | jq -r '.ci_slot_saturation.status' | prints `could-not-check` (`--forge` set but no token ⇒ the forge-derived term is unread, never zero) |
| 6 | check:ci +mutation | cd statusgen && go test -run 'TestFlowForgeGatedOnFlagNotToken' ./... | exit 0; the test sets `GH_TOKEN`/`GITHUB_TOKEN` in the test environment, installs an `http.RoundTripper` stub that fails the test if `RoundTrip` is ever called, runs `--flow --json` WITHOUT `--forge`, and asserts both `ci_slot_saturation.status=="could-not-check"` AND the stub recorded zero calls — proves the read is gated on the flag, never on credential presence, so a plain `--flow` in an environment that happens to carry a token still starts no forge process and makes no network call |
| 7 | check | cd statusgen && go run . --root testdata/flow/window-a --flow --json; echo rc=$? | tail -1 | `rc=3` when any source is could-not-check, `rc=0` only when every source was read — the gate-telemetry exit contract carried through |
| 8 | check | statusgen --root . --consumers --brief assay:assay:graph-execution:07 --diff-base origin/main; echo rc=$? | tail -1 | `rc=0` on the implementing branch — the iso9001 row flipped to fixed-here is corroborated by the diff, the bottleneck out-of-scope entry is listed for the reviewer |
| 9 | check | statusgen --root . --lint; echo rc=$? | tail -1 | `rc=0` |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code, output line(s) or hash, date, runner). "verified" requires this section filled by someone who did NOT implement. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run 'TestFlow' ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run 'TestFlowRefusesOpenInterval' ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go run . --root testdata/flow/window-a --flow --json` | fail exit=1 | sha256:d70aacd5f8b2 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go run . --root testdata/flow/window-a --bottleneck` | pass exit=0 | sha256:bdc1fb933b90 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && env -u GH_TOKEN -u GITHUB_TOKEN go run . --root testdata/flow/window-a --flow --forge --json` | fail exit=1 | sha256:1f78facc948b | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go test -run 'TestFlowForgeGatedOnFlagNotToken' ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go run . --root testdata/flow/window-a --flow --json; echo rc=$?` | pass exit=0 | sha256:028c56a4b873 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --consumers --brief assay:assay:graph-execution:07 --diff-base origin/main; echo rc=$?` | pass exit=0 | sha256:049bf8de6bbe | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:b2175d91498e | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier run — 2026-09-27 claude-opus-5-5 (verify-desk dispatch), offline — **VERIFY: FAIL (check-definition: rows 3 and 7)**

Pin: `medici-finance/assay` main `9585b4b6cc2ea8d35d367fb912e7c8216a765ba3` (git rev-parse == gh api commits/main). Implementing commit `5ea3f4a8b` (PR #1347, squash, merged 2026-09-20). `gate: model`, `risk {all no}`, `irreversible: no`. `KUBECONFIG=/dev/null`; no forge call made by any row.

**Witness-table caveat.** The Command cells of rows 3, 4, 5, 7, 8 and 9 contain an unescaped pipe, so the witness above ran each of them TRUNCATED at the first pipe (the table cell split). Its pass/fail for those rows judges the truncated command, not the row: rows 7, 8 and 9 read `pass exit=0` only because the retained `; echo rc=$?` always exits 0, and rows 3 and 5 read `exit=1` because `go run` maps the program's exit 3 to 1. Rows 1, 2 and 6 are check:ci and the witness could not run them on a darwin host (no `unshare --net`). The per-row results below are from running each FULL row command directly at the pinned SHA.

**Per-row real output (full commands, run directly):**
1. `go test -count=1 -run 'TestFlow' ./...` → exit 0; `ok github.com/medici-finance/assay/statusgen`; four tests ran and passed: TestFlow, TestFlowRefusesOpenInterval, TestFlowWindowExcludesOutOfRange, TestFlowForgeGatedOnFlagNotToken. **PASS.**
2. `go test -count=1 -v -run 'TestFlowRefusesOpenInterval' ./...` → exit 0; `--- PASS: TestFlowRefusesOpenInterval`. **PASS.**
3. Full pipeline → jq printed `false`, exit 1. **FAIL as written.** The report's own reason for example-app/02: `eligible_to_start: could-not-check — in-progress instant falls outside the requested [since, until) window`. With no `--since/--until`, the window defaults to the 28 days preceding the run (defaultDoraWindowDays = 28), which on the run date starts Aug 30 2026. The fixture's in-progress instant for example-app/02 is Aug 7 2026, so it falls outside that window. With the window pinned to the fixture's own range (`--since` Aug 1 2026, `--until` Sep 20 2026), the same jq expression prints `true`, exit 0 (environment.statusgen_version `dev`, resolution.seconds 86400, example-app/02 eligible_to_start `measured`). The unit tests already pin their window. **Stale-shaped (check-definition):** the command is clock-sensitive. It stopped holding when the PR's window-scoping commit (9e445b118, merged with the PR on Sep 20 2026) began enforcing the default window. At merge time the window already started Aug 23 2026, so the row as written could not pass on merged main. The implementation is correct.
4. Full pipeline → prints `2`, exit 0 (count ≥ 1). **PASS.** Side effect: the non-JSON `--bottleneck` run writes a dated factory-floor report into the fixture tree (docs/reports/factory-floor/ under testdata/flow/window-a), which leaves the checkout dirty with an untracked file. The verifier moved the file out.
5. Full pipeline with GH_TOKEN/GITHUB_TOKEN unset → prints `could-not-check`. **PASS.**
6. `go test -count=1 -v -run 'TestFlowForgeGatedOnFlagNotToken' ./...` → exit 0; `--- PASS`. **PASS.** Mutation check: with the `!forgeMode` early return disabled, the test FAILS with `RoundTrip was called 1 time(s) with forgeMode=false`, so the stub really does detect a network call. The source was restored afterwards. Scope note: the test calls computeCISlotSaturation directly with forgeMode=false and does not drive the whole `--flow --json` CLI path. The rest of computeFlowReport reads only local files. The one network path is inside computeCISlotSaturation, behind the flag check.
7. `go run . --root testdata/flow/window-a --flow --json; echo rc=$?` → `rc=1`. **FAIL as written** (expected `rc=3`). The program itself exits 3: `go run` prints `exit status 3`, and a binary built from the same source returns `rc=3` directly. **Stale-shaped (check-definition):** `go run` never passes a child's non-zero exit code through; it always exits 1. So the row, as authored, can never observe `rc=3`. The exit contract is implemented correctly.
8. As written on merged main → `COULD-NOT-CHECK: ... is not in the diff against 9585b4b6...`, rc=2. That result is expected, because the Expect cell says "on the implementing branch". Re-run in a detached checkout of the implementing commit with `--base 983327a4` (its parent; `--diff-base` is a `--lint`-only flag and `--consumers` ignores it): rc=0, `CORROBORATED docs/iso9001-mapping.md ... fixed-here`, `UNCHECKED statusgen/bottleneck.go ... out-of-scope` (unchanged since the merge-base, as the brief intends: not replaced). **PASS on the implementing diff.** Authoring note: the row should say `--base`, not `--diff-base`.
9. `statusgen --root . --lint; echo rc=$?` → `rc=0`, zero PROBLEM lines. **PASS.**

**Review question (from ## Review).** Can `--flow` reach the forge without `--forge`? No. computeCISlotSaturation returns before it resolves a token or builds a client whenever forgeMode is false. The token is read only from GH_TOKEN/GITHUB_TOKEN, and only after the flag check. Row 6 proves that zero network calls are made, and the mutation above shows the proof has teeth.

**Deliverable gap (observation, not a row failure).** Task 2 asks for `ci_slot_saturation` computed from the forge check-run read. The implementation reads merged PRs through ghClient but has no check-run reader for the median-CI-wall-clock term. It therefore reports could-not-check even under `--forge` with a token and `--ci-hours-per-day`. The implementer disclosed this in the commit and in a code comment. Consequences: `ci_slot_saturation.status` is never `ok`, and `flowExitCode` can never return 0, so the "rc=0 only when every source was read" half of row 7 cannot be reached today. No Verify row covers a measured saturation. This needs a follow-up item.

**Risk-bearing value enumeration** (diff scope: flow.go and the main.go flag additions in 5ea3f4a8b, plus the constants they reuse):
- `ci-hours-per-day` flag default = `0` @ statusgen/main.go:1591, guard `ciHoursPerDay <= 0` → could-not-check @ statusgen/flow.go:470
- `gtExitCouldNotCheck = 3` @ statusgen/gatetelemetry.go:72, returned by flowExitCode @ statusgen/flow.go:631,634; `return 0` @ statusgen/flow.go:636; malformed-input `return 1` @ statusgen/flow.go:645,650
- `gtSmallN = 5` @ statusgen/gatetelemetry.go:76, reused by the fleet-median gate `len(secs) < gtSmallN` @ statusgen/flow.go:448
- minimum distinct regen instants for a resolution: `len(seen) < 2` @ statusgen/flow.go:423
- `defaultDoraWindowDays = 28` @ statusgen/roadmapdora.go:37 (pre-existing and unchanged; it sets the default `--flow` window and is the cause of row 3's clock dependence)

Ranking: this is a read-only reporting instrument, and every value is reversible with an edit and a redeploy. None is irreversible, so the trigger does not fire on irreversibility. The top-ranked entries below are the ones external consumers key on.
- `RISK-VALUE: DERIVED — gtExitCouldNotCheck = 3 @ statusgen/gatetelemetry.go:72 — the brief's facts require --flow to carry the --gate-telemetry exit contract (0 / 1 malformed / 3 could-not-check) through verbatim; flow.go returns the shared constant rather than a new literal, so the two instruments cannot drift apart.`
- `RISK-VALUE: DERIVED — ci-hours-per-day default = 0 @ statusgen/main.go:1591 — the brief specifies "no default — absent ⇒ could-not-check, never an invented number"; 0 is the absent sentinel, and the <= 0 guard at flow.go:470 turns it (and any negative value) into could-not-check before any division happens.`
- `RISK-VALUE: DERIVED — len(seen) < 2 @ statusgen/flow.go:423 — from first principles: the resolution is the median gap between distinct regen instants, and at least two instants are needed to form one gap.`
- `RISK-VALUE: NAMED, NOT DERIVED — gtSmallN = 5 @ statusgen/gatetelemetry.go:76 — the brief says to reuse this constant, and flow.go does, so the implementation matches the spec. Why 5 is the right thin-data floor belongs to the gate-telemetry brief that introduced it; that derivation is outside this item's scope and was not re-done here.`

**Verdict.** Rows 1, 2, 4, 5, 6, 8 and 9 pass. Rows 3 and 7 fail as written. Both failures are check-definition defects: row 3 depends on the clock through the default 28-day window, and row 7 captures the exit code through `go run`. The implementation behaves correctly under the corrected commands. The item should not advance until the Verify table is re-authored: pin `--since/--until` on row 3, build or `go build` the binary before capturing the exit code on row 7, use `--base` on row 8, and escape the pipes so the witness runs the full commands.

VERIFY: FAIL

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question this brief must answer: can `--flow` reach the forge without an explicit
`--forge` opt-in, under any combination of environment variables or default flag values — and
does Verify row 6 actually prove it cannot (not merely that the report reads `could-not-check`,
but that zero network calls were attempted)?
