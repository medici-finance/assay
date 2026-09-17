---
brief: assay:assay:fresh-views:01
title: input-sha stamp + refuse-when-stale helper for derived reads
why: >-
  Every desk loop tool photographs main and then acts on the photograph after main has moved,
  and nothing tells it the photograph is stale — the class that produced #334, #339 and #228.
  A shared helper that stamps a derived read with the sha it was computed against, and refuses
  (not guesses) when the current head has advanced past the stamp, turns the house rule
  "refresh, don't remember" from a discipline agents violate into a fail-closed check the code
  enforces once, in one place, for every consumer.
wave: 0
depends: []
unblocks: ["fresh-views/02", "fresh-views/03"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by fresh-views scoping session
sources:
  - "docs/streams/fresh-views/spec.md §2 (the design principle: stamp the input, refuse when stale)"
  - "medici-finance/assay#334 (verifyloop plan staleness), #339 (ready-flip mergeability staleness) — the two consumers this helper unblocks"
  - "tools/desk/internal/deskkit (existing origin/main read + sha helpers this composes), tools/desk/internal/loopengine (the loop drivers that will call it)"
  - "plugins/assay/skills/the-desk/SKILL.md, plugins/assay/skills/intake-desk/SKILL.md, plugins/assay/skills/pr-review-desk/SKILL.md — \"Refresh, don't remember\" (the discipline this makes mechanical)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main): #334/#339 confirmed OPEN; no freshview helper exists under tools/desk/internal"
exec-tier: strong
exec-tier-why: (a) the stamp/refuse API is a design decision the facts do not fully pre-specify; (c) a concurrency helper whose refuse-check is subtly wrong ships a green lamp wired to nothing
domain: complicated
version: 1
id: 9abca675-e8d4-4fce-9175-961def29600e
# no consumers: this brief CREATES a new package (tools/desk/internal/freshview) with no
# pre-existing consumers to update; the fact that fresh-views/02 and /03 consume the helper
# is carried by unblocks: above, not by a shared-value edit.
---

# Brief 01 — input-sha stamp + refuse-when-stale helper for derived reads

## Context
files:
- `tools/desk/internal/freshview/freshview.go` (planned) — NEW package: the stamp type + the refuse-when-stale check.
- `tools/desk/internal/freshview/freshview_test.go` (planned) — NEW: unit + mutation tests.

facts:
- reads (not modified): reuse the existing `origin/main` fetch + rev-parse helpers under `tools/desk/internal/deskkit` rather than forking them; the helper takes head-reading as an injected `HeadFn`, so this brief adds no deskkit edit.
- seam: a derived view (plan, board snapshot, open-PR set, mergeability verdict) is computed against one sha of `main`; `main` advances; the tool acts on the stale view. Design principle (spec §2): stamp the input sha; refuse when the head advanced past it.
- stamp shape: `{InputSHA string; ComputedAt time.Time; Ref string}` — `Ref` names the input ("origin/main", or a PR-set query key) so a stamp is self-describing. Serializable to one header line for artifacts that persist (a plan file), holdable in memory for ephemeral views.
- refuse contract: `CheckFresh(stamp, currentHead) error` returns a typed `StaleError` (naming stamped sha + current head + ref) when `currentHead != stamp.InputSHA` AND `currentHead` is a descendant of `stamp.InputSHA` (i.e. main advanced); returns nil when equal. A non-ancestor/divergent head is also stale (fail-closed) with a distinct message.
- two modes, caller's choice (spec §5 Q2): `MustBeFresh` (return `StaleError`, caller re-derives — the DEFAULT) and `RefreshedInPlace` (helper re-reads and returns the new stamp, logging that it did). This brief SHIPS both; it wires neither consumer (02/03 do that).
- single point of failure (rule 10): the ONE control is the `CheckFresh` comparison. Second layer is by construction in the consumers, not here: each consumer calls `CheckFresh` at its ACT boundary (a different component, a different signal — the plan step in 02, the flip step in 03), so a bug in one call site does not blind the other. This brief's own layer is the pure, unit-tested predicate; making it a pure function of its inputs is what lets the mutation test below prove it in isolation.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add package `tools/desk/internal/freshview` with the `Stamp` type, `StaleError`, `CheckFresh`, and the two modes (`MustBeFresh`, `RefreshedInPlace`). Keep it PURE: `CheckFresh` takes the two shas + ref as arguments — it performs no I/O, so it is unit-testable without a repo. The I/O (reading current head) is the caller's, or a thin injected `HeadFn func(ref string) (string, error)`.
2. Provide `Stamp.String()` / `ParseStamp(line)` round-trip for the one-line artifact header form (`fresh-view: <ref>@<sha> computed <RFC3339>`), so a persisted plan can carry and re-check its own stamp.
3. Unit tests: equal head → nil; advanced head → `StaleError` naming both shas; divergent/unknown head → fail-closed `StaleError`; `RefreshedInPlace` returns the new stamp and reports refreshed=true; `String`/`ParseStamp` round-trip.
4. Do NOT modify verifyloop, deskpost, deskboard or scanloop in this brief — the wiring is 02/03. This brief delivers the helper + its tests only.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/freshview/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./internal/freshview/... -run TestCheckFreshStale -v` | exit 0; output contains `StaleError` and both the stamped and current sha | check:ci +dereference |
| 3 | In a scratch copy, invert the `CheckFresh` comparison (treat advanced head as fresh), then `cd tools/desk && go test ./internal/freshview/...` | exit non-zero — the mutation test reddens, proving the refuse-check is load-bearing | check +mutation |
| 4 | `cd tools/desk && go vet ./internal/freshview/...` | exit 0 | check:ci |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
Note on the `+flow` obligation: this brief ships the shared helper and no cross-component flow of its own; the end-to-end flow (a real tool refusing/refreshing on a stale input via `CheckFresh`) is discharged by the consuming briefs fresh-views/02 and fresh-views/03, each of which carries a flow row. The deskkit read is an injected `HeadFn`, not a modified path — hence all four risk answers are `no`.
