# Gate-effectiveness metric definitions

Deliverable of `orchestra-review/01`. Data, not narrative — implemented verbatim by
`statusgen --gate-telemetry` (`statusgen/gatetelemetry.go`). Extends the #290 flow-report
spec with the two families it lacks (trust, risk); does not implement #290 itself and does
not close it (coupling rule: whichever of #290 / this brief lands second folds its families
into the first's tool rather than emitting a parallel report — see brief-01 `sources:`).

## Three-state semantics (every row below)

Per `docs/three-state-instrument-rule.md`, every metric reports one of three states, never
a bare pass/fail:

| State | Meaning here |
|---|---|
| `checked-clean` equivalent | the metric was computed from a source that was read AND understood; printed as a plain numerator/denominator line |
| `checked-failed` equivalent | the source itself is malformed (bad JSON) — a hard error, exit 1, distinct from "nothing to report" |
| `could-not-check` | the metric's source surface could not be read or could not be understood for this window — printed literally as `could-not-check` with the reason, never rendered as `0`. Triggers process exit `3`. |

**A zero fire count is this tool's ALARM condition**, so any path that produces a zero without
having understood its source affirmatively accuses a working gate of being ceremonial. Four
distinct shapes could produce that zero, and only the first is a measurement:

| Shape | State | Why |
|---|---|---|
| source read, records understood, count genuinely 0 | measured `0` | small fleet, nothing fired — legitimately zero |
| source file absent | `could-not-check (… missing)` | nothing was examined |
| source file present, zero records | `could-not-check (… present but empty)` | an append-only log with no lines is a collection failure (rotation, a collector that created the file then died, a truncating write), not evidence of a quiet window. A well-formed empty JSON array `[]` is NOT this case — `[]` is an affirmative "zero rows" someone wrote, which is a defensible measured zero |
| source records present, none in a recognized shape | `could-not-check (… schema mismatch)` | producer/reader drift. Matching nothing is indistinguishable from "nothing happened", so it must never be reported as the latter |

This is the "absence of evidence ≠ evidence of absence" sub-rule
(`docs/three-state-instrument-rule.md`) applied to gate telemetry: a gate with 0 fires because
nobody reported must never render the same as a gate with 0 fires because the reader could not
understand the log.

## Trust family — override rate

Override rate measures gates that pass work an App verdict called clean, which a human (or a
downstream defect-discovery surface) later reverses. Bound by incident #280 (a forged
`human:` token silently suppressing a gate) — a gate that is being routinely overridden is
either too strict (ceremonial friction) or too weak (rubber-stamping), and #280 shows the
override channel itself can be the attack surface.

Every leg is scoped to **App-`APPROVED`** work. A PR the App gate never called clean is not
evidence about the App gate, and counting it dilutes the rate — always in the direction of
making the gate look better than it is.

| Metric | Numerator | Denominator | Source surface | Three-state note |
|---|---|---|---|---|
| `override-rate` (a) app-approved-then-human-reversed | count of PRs where the App verdict was `APPROVED` and the human outcome was `human-rejected` / `reworked` / `closed-unmerged` | count of App-`APPROVED` PRs in the window | `pr-verdicts.json` (PR review threads: App verdicts + terminal human outcome) | absent file → `could-not-check`; `[]` (no PRs this window) → `0/0 n=0`, not an alarm; rows present but unrecognized → `could-not-check (schema mismatch)` |
| `override-rate` (b) merged-PR-named-by-defect-finding | count of **App-`APPROVED`** merged PRs subsequently named by a FINDINGS-register entry or `bug`-labeled issue (escaped-defect rate against an APPROVED verdict) | count of **App-`APPROVED`** PRs that merged in the window | `defect-findings.json` (FINDINGS register + `bug`-labeled issues), denominator from `pr-verdicts.json` | absent file → `could-not-check`; `[]` → `0/0 n=0`. Findings naming PRs absent from `pr-verdicts.json` are counted and reported on a `note:` line, so an incomplete verdict list cannot shrink the numerator silently |
| `override-rate` (c) human-gate-flip-reversal | count of **distinct PRs** whose ready-flip was reversed at a human gate | count of **distinct PRs** ready-flipped in the window | `audit.jsonl` (`deskkit.Entry`: `tool: "deskpost"`, `verb: "ready"` / `"ready-reversal"`) | counting distinct PRs rather than log rows makes the metric independent of whether a reversal is appended alongside the original flip or logged in its place. **No desk tool emits a `ready-reversal` record today**, so a window with zero reversals reports `could-not-check (numerator unobservable)`, never `0` — the tool cannot distinguish "no reversals happened" from "reversals are invisible on this surface" |

## Risk family — catch rate + ceremonial-gate detection

Catch rate measures whether a gate class actually blocks defects when it fires, and whether
it ever fires at all. Bound by incident #229 (a detector that was unwirable with the suite
green — a gate that structurally cannot fire is worse than no gate, because it reads as
coverage that does not exist).

| Metric | Numerator | Denominator | Source surface | Three-state note |
|---|---|---|---|---|
| `catch-rate` per gate class | fires where the gate blocked a real defect (proxy: the PR was subsequently amended before merge, or the finding was acknowledged) | total fires for that class in the window | `gates.json` (App review verdict, security review, statusgen lint PROBLEM, corroborate, CI red) for non-audit-sourced classes; `audit.jsonl` for `auditSourced` classes (e.g. deskpost refusals) | `auditSourced` + `audit.jsonl` absent/empty/unrecognized → `could-not-check`, even though `gates.json` itself is readable. An `auditSourced` class with **no selector defined** in `gtAuditGateSelectors` also reports `could-not-check` — an unknown selector matches nothing, and matching nothing must never render as "never fired". **The desk audit log records that a gate fired, never whether the fire blocked a defect**, so audit-sourced classes report a measured fire count and `catch-rate=could-not-check` |
| ceremonial-gate detection — never fires | fire count for the window (0 or not) | — (a flag, not a ratio) | `gates.json` fire list / `audit.jsonl`, paired with the `mutationTested` field | 0 fires AND `mutationTested: false` → alarm, printed as `ceremonial-or-untested`. 0 fires AND `mutationTested: true` → NOT an alarm, printed as `proven-able-to-fire`. This is the "windows where a numerator is legitimately zero are expected" rule: only the never-proven-able case alarms |
| ceremonial-gate detection — fires without catching | fires > 0 AND blocked = 0 | — (a flag, not a ratio) | same | printed as `fires-without-catching`. A gate that fires 40 times and blocks nothing is the second, arguably more expensive shape of ceremony — it spends the fleet's attention every time |

### `mutationTested` is an operator assertion, not a corroborated fact

`mutationTested` is read **straight out of `gates.json`**. Nothing resolves it against a real
mutation-test artifact: setting one boolean to `true` moves a gate out of the ceremonial alarm
with no evidence. For a detector whose whole purpose is to catch controls that cannot fire, an
unverified opt-out is the same shape as the problem it is built to find.

The report therefore prints the provenance on the line itself —
`proven-able-to-fire (operator-asserted in gates.json; NOT corroborated against a mutation-test
artifact)` — so no reader mistakes it for proof. Resolving the flag against an actual
mutation-test surface is open follow-up work, not something this tool does.

### Small and zero denominators

The report prints raw fractions and never percentages, so N is always visible. Because any
consumer that divides them reintroduces the "0% over zero samples" failure, a denominator of
`0` is annotated `n=0 (no data — not a rate)` and a denominator below 5 is annotated
`small-n (n=N)`.

## Output shape

`statusgen --gate-telemetry --root <dir>` emits one dated-artifact-shaped report to stdout for
the window described by `<dir>` (same cadence/home as the #290 flow-report once one of the two
tools absorbs the other): the `override-rate` lines above, followed by one `gate-class` line per
entry in `gates.json`, sorted by class name for determinism (no wall-clock, no map-iteration
order leaks into the text — `diff` across two runs over the same fixture is byte-identical).

## Exit codes

Diagnostic report, not a boolean gate (same family as `--dora`/`--trend`/`--bottleneck`): the
*content* of the report (an override, a ceremonial gate) never fails the process — that is the
review desk's job to act on, not this tool's job to gate on. The exit code reports the
INSTRUMENT's own health:

| Exit | Meaning |
|---|---|
| `0` | ran to completion; every source was read AND understood (report may still contain alarms — that's data, not instrument failure) |
| `1` | a source file exists but is malformed (bad JSON) — the instrument itself hit a defect, distinct from "nothing to report" |
| `2` | usage error (already claimed by the `flag` package and statusgen's own arg-parsing errors) |
| `3` | `could-not-check` — at least one metric could not be computed because its source surface was absent, empty, unrecognized, or does not record the thing the metric needs (distinct from clean-0 and malformed-input) |

Exit `3` is expected on live windows today, not an anomaly: metric (c)'s reversal channel has
no producer, and the audit log does not record blocked-defect outcomes, so both report
`could-not-check` against real data. That is the instrument declining to invent numbers, and it
is the correct reading until the collectors above land.

## Fixtures (`statusgen/testdata/gatetelemetry/`)

| Case | What it proves |
|---|---|
| `override-one` | `override-rate` (a) computes **1/2** naming PR `#101` — the window holds two App-`APPROVED` PRs (`#101` human-rejected, `#102` merged), so the numerator is 1 and the denominator is 2. `override-rate` (c) computes 1/2 from a real ready-flip + `ready-reversal` pair |
| `zero-fire-untested` | a gate class with 0 fires and no mutation-test marker prints `ceremonial-or-untested` (positive control) |
| `zero-fire-tested` | a gate class with 0 fires but a mutation-test marker prints `proven-able-to-fire`, never `ceremonial-or-untested` |
| `missing-audit` | `audit.jsonl` absent → the audit-sourced metrics (override-rate (c), any `auditSourced` gate class) print `could-not-check`, never `0`; process exits `3` |

The remaining three-state paths — a present-but-empty log, an unrecognized log schema, an
unrecognized JSON row shape, an `auditSourced` class with no selector, and both log shapes for
a ready-flip reversal — are pinned in `statusgen/gatetelemetry_test.go` against temp windows
rather than shipped fixtures.

## Collection status (brief-01 Task 2)

| Surface | Producer | Status |
|---|---|---|
| `audit.jsonl` | `tools/desk/internal/deskkit.Log` (`Entry` schema) | **read in the producer's native schema.** Field names and the verb/result vocabulary belong to `deskkit`; a line that is not a `deskkit.Entry` is `could-not-check`, not an absence of events |
| `pr-verdicts.json` | none yet | hand-authored window. Needs a read-only GitHub collector over PR review threads (App verdicts + terminal outcome) |
| `defect-findings.json` | none yet | hand-authored window. Needs a collector over the FINDINGS register + `bug`-labeled issues |
| `gates.json` | none yet | hand-authored window. Non-audit gate classes and the `mutationTested` marker are operator-supplied |

The GitHub-sourced collector and the dated-artifact emitter are **not** in this brief's
delivered scope — see brief-01 Task 2's scope note.
