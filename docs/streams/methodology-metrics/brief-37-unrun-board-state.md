---
brief: methodology-metrics/37
title: Make UNRUN a first-class board state — block done on an unrun risk-bearing Verify row
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [885]
schema: brief-v1
authored: 2026-07-20 by Opus 4.8 authoring session (intake Tier-2, #885)
sources: ["[I-unrun-board-state](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-17-unrun-as-first-class-board-state.md)", "docs/streams/methodology/verify-desk-bottleneck-2026-07-17.md (R10)", "assay methodology review (verification-integrity)", "tools/statusgen/quality.go rowIsBacked/qualityToken/qualityNotices", "#885 (tracking)"]
why: >-
  The risk-bearing Verify row — the live/mutating end-to-end check — is systematically the row
  recorded UNRUN, and today a `done` brief with UNRUN rows renders identically to a fully
  verified one. 16 of 17 done ledger-hardening briefs (the unauth read/write security trio and
  the money-path idempotency brief among them) closed with their live rows never run. statusgen
  already flags UNBACKED Evidence (rowIsBacked) but has no analogous flag for UNRUN. Otherwise
  faster closing just means faster closing of the checks that matter least.
---

# Brief 37 — UNRUN as a first-class board state

## Context
files:
- `../assay-toolkit/statusgen/quality.go` — `rowIsBacked` / `briefEvidenceBacked` / `qualityToken` /
  `qualityNotices` (the existing point-quality machinery — the pattern to extend)
- `../assay-toolkit/statusgen/brieffile.go` — Evidence-row parsing + the irreversible lint precedent
  (brieffile.go:439-446)
- `../assay-toolkit/statusgen/quality_test.go` / `brieffile_test.go`
- `docs/streams/methodology-metrics/README.md` — one convention line

facts:
- `UNRUN` is a convention already used in Evidence tables (mm/12's card renders UNRUN rows
  prominently; the brief-23 live-verify pattern names them). This brief promotes it to a board
  state: statusgen scans a verified/done brief's Evidence for a row marked `UNRUN` (or
  `deferred`) and renders a distinct token — analogous to the `verified*`/`done*` unbacked
  marker `qualityToken` already emits, e.g. `done‡` with a legend "‡ = closed with an unrun
  risk-bearing Verify row".
- **The gate (the teeth):** a brief may NOT reach `done` with an UNRUN risk-bearing row UNLESS
  that row is either (a) closed live, or (b) routed to a named follow-up brief/issue recorded in
  the Evidence (the brief-23 live-verify pattern, now mandatory). "Risk-bearing" = a row whose
  Verify command is a live/mutating end-to-end check; detect via the row carrying a
  `risk-bearing`/`live` tag OR the brief's `risk.*` being any-yes (an irreversible/customer/
  regulatory brief's live row is risk-bearing by definition). An UNRUN risk-bearing row with no
  routed follow-up = a hard lint PROBLEM against the `done` transition.
- This pairs with the F-28 discipline ("a PASS must name the risk-bearing value") — together
  they keep the throughput-automation work (mm/38, mm/39) honest: closing faster must not mean
  closing the checks that matter least without a trace.
- Reads current state (Evidence text + brief risk frontmatter) — wave 0. No historian dep.
- Out of scope: retroactively re-opening the 16 already-`done` lh briefs (a verify-desk sweep,
  not this brief — though the new lint will surface them as PROBLEMs, which is the intended
  visibility); defining new live-verify tooling.

## Scope decision (recorded at implementation, 2026-08-02)

This brief was dispatched with an open question: it gates the **`done`** transition, while
F-impl-claims-unproven's
defect is one step earlier, at **`implemented`** ("nothing runs the Verify table before `implemented`
is asserted"). The finding's recommendation 3 — an alarm on `implemented` with empty Evidence past N
hours — is the same machinery at the earlier step. Extend 37, or leave that to its own brief?

**Decision: extend 37 to the `implemented` stage.** Reasoning:

1. **It is one derived quantity, not two.** The implementation computes exactly one thing —
   `unrunFindings(brief)`, the Verify rows with no completed Evidence row behind them. `implemented`
   reads "how many rows are corroborated at all?"; `done` reads "is a RISK-BEARING row uncorroborated
   and unrouted?". Same function, two thresholds, two severities. A separate brief would have to
   re-derive "what counts as run", and the moment two definitions of that exist they disagree — the
   near-duplicate alarm this dispatch warned against, with the added cost that the two would
   contradict each other on the same board.
2. **The finding's stage is the disease; 37's stage is where the symptom becomes permanent.** Gating
   only `done` puts the cure one step downstream of the defect. An uncorroborated `implemented` claim
   is already on the public record for the days or weeks before anything reaches `done`.
3. **Recommendation 3 is strictly cheaper as an extension**: one notice function and one threshold
   const, sharing the derivation, the parsing, and the fixture tree. As a standalone brief it would
   need all of that again.
4. **The finding is stronger when re-expressed through this derivation.** Recommendation 3 says "empty
   Evidence"; run-coverage is a superset — a brief with prose, links and an implementer's narrative in
   `## Evidence` but no completed row still reads as zero-coverage, where an emptiness test would call
   it filled.

**NOT absorbed** (deliberately left out of scope, so the residue stays visible rather than assumed
handled): the finding's recommendation 1 (fixing the three named oit briefs — a verify-desk sweep in
another repo), recommendation 2 (the convention that implementer-written evidence-claims belong only in
the Verify table's Expect column — a methodology-doc change, not statusgen), and recommendation 4 (the
optional pre-merge lint that greps the brief body for asserted test/artifact names and checks they
exist — a genuinely separate check, needing repo-wide artifact resolution this brief has no part of).
The finding register entry itself is not edited here; `docs/streams/findings/` is claimed by a
concurrent worker.

## Derivation, not declaration (the constraint this design is built around)

The pre-existing `UNRUN` convention was a string a verifier typed into an Evidence cell. As a board
state that is worthless: **omitting it costs nothing**, so a row that carries no `UNRUN` marker proves
nothing about whether anything ran. Any design where a brief author can write "UNRUN" or leave it out
at will reproduces the exact defect the finding describes.

So the state is computed from the two artifacts and runs the other way round:

> A Verify row is UNRUN **unless** the `## Evidence` section carries a row for it with a date and a
> runner, unmarked as unrun/deferred.

Silence is UNRUN. Clearing the state requires **adding** an artifact; no amount of not-writing produces
a clean board. The literal `UNRUN`/`deferred` marker is still honoured but **additively only** — it can
move a row *into* the unrun set (a verifier saying "I did not run this" is believed), never out of it.
There is no field, token or phrasing an implementer can use to assert a row was run.
`TestUnrunDerivedFromSilentOmission` pins this: fixture `ur/05` never writes the word as a marker
anywhere, its Evidence table simply stops before the live row, and it still flags. Under a mutant that
restores declared-only semantics that test fails.

The same non-controllability holds for the two supporting inputs: the `implemented` age comes from the
historian (main-CI-written, append-only) or git, never from the brief; and the grandfathering boundary
is the git merge-base with `origin/main`.

## Severity policy (deviation from the brief's literal "PROBLEM on the current backlog")

The Task text expected the new lint to fire hard PROBLEMs at the existing closed-with-UNRUN backlog
("the intended visibility"). Implemented instead as: **the gate is on the TRANSITION, not on history.**
A brief already `verified`/`done` at the merge-base with `origin/main` is grandfathered to a `--lint`
NOTICE; a brief this branch closes is a PROBLEM. Two reasons, one of them the brief's own:

- Firing a hard PROBLEM at a closed brief creates a live incentive to edit its Evidence table until the
  board goes green — the precise falsification this check exists to catch. `unfailableRowNotices`
  (#509) is NOTICE-only for exactly this reason, stated in `main.go`.
- A repo-wide red gate over a pre-existing backlog blocks every unrelated PR, and the brief's own scope
  note puts re-opening that backlog with a verify-desk sweep out of scope. Verify row 3 already says
  "exit non-fatal-per-policy; surfaces a PROBLEM/**NOTICE**".

Nothing is silenced: every offender is named in `--lint` and renders `‡` on the board. Unresolvable
base (no `.git` / no `origin/main`) grandfathers everything with a NOTICE naming the degradation — not
fail-open, but the check's subject (a transition) being unobservable.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first: a done brief with an UNRUN risk-bearing row and NO routed
   follow-up → lint PROBLEM + distinct board token; the same brief WITH a named follow-up
   brief/issue in Evidence → clean (token still marks it, but no PROBLEM); a non-risk-bearing
   UNRUN row → no PROBLEM; a fully-run brief → unchanged.
2. Implement: UNRUN detection over Evidence rows; risk-bearing determination (row tag OR any
   `risk.*: yes`); the distinct `qualityToken` variant + legend; the `done`-transition lint
   PROBLEM when UNRUN risk-bearing + no routed follow-up.
3. README: one line under conventions — UNRUN risk-bearing rows render distinctly and block
   `done` unless closed live or routed to a named follow-up (brief-23 live-verify pattern,
   mandatory).
4. (Scope extension, see "Scope decision") the `implemented` leg of the same derivation: a brief at
   `implemented` with ZERO corroborated Verify rows, aged past `staleImplementedThreshold` (72h from
   the historian, git LastTouch as fallback), emits a stale-implemented NOTICE —
   F-impl-claims-unproven recommendation 3.

> **Path note (amended at implementation):** the brief was authored against oit's layout
> (`tools/statusgen/`). In assay-toolkit statusgen is its own Go module rooted at `statusgen/`, so
> every Verify command below `cd`s into it — the shape this repo's own CI uses. Deliverable files:
> `statusgen/unrun.go`, `statusgen/unrun_test.go`, `statusgen/testdata/unrun/**`, plus the wiring in
> `statusgen/{main,quality,emit}.go`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test ./... -run Unrun -v` | exit 0; `TestUnrun*` covers the four Task-1 cases — `TestUnrunRiskBearingUnroutedBlocksDone` (→PROBLEM), `TestUnrunRoutedFollowUpIsCleanButStillMarked` (→no PROBLEM, still `‡`), `TestUnrunNonRiskBearingRowIsSilent`, `TestUnrunFullyRunBriefUnchanged` |
| 2 | `cd statusgen && go test ./... && go vet ./... && gofmt -l .` | exit 0; gofmt lists nothing |
| 3 | `cd statusgen && go run . --root .. --lint; echo $?` | `0` — the pre-existing backlog is grandfathered to NOTICEs (severity policy above), and at least one `methodology-metrics/37` NOTICE names a real closed brief with an UNRUN risk-bearing row |
| 4 | `cd statusgen && go run . --root .. && grep -c '‡' ../STATUS.md` | ≥ 1 — UNRUN-closed rows carry the `‡` token and both boards carry the legend line (STATUS regen local-only, **NOT committed**; `git checkout -- STATUS.md` after) |
| 5 | `cd statusgen && (rc=0; for t in UnrunDerivedFromSilentOmission UnrunStaleImplementedFiresOnRealBoardWhenAged; do out=$(go test ./... -count=1 -run "$t" -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf '%s' "$out" \| grep -q -- '--- PASS'; } \|\| { echo "MISSING-OR-FAIL $t"; rc=1; }; done; exit $rc)` | exit 0, prints nothing — UNRUN derives from a silent coverage gap (fixture writes no marker), and the `implemented` leg fires on THIS repo's real board once the clock is wound past the threshold (proves the state is not merely never-firing). The previous `-run 'A\|B'` form could not run: `\|` is a LITERAL pipe under RE2, so it matched no test and exited 0 with "no tests to run" (measured: 2 such lines, 0 `--- PASS`). Each name is now its own `-run`, existence is asserted from `--- PASS`, and `go test`'s own exit status is captured first so a FAILING test also goes red — #509, #580 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — repo-internal Go tooling; a board-integrity flag +
`done`-transition lint. NOTE: the briefs it POLICES include irreversible ones, but this tooling
brief is itself repo-internal Go with no funds/ledger/customer surface). Reviewer confirms the
UNRUN gate blocks `done` only when the row is risk-bearing AND unrouted, and records verdict +
date in the stream README table.
