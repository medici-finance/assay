---
brief: quality/09
title: "`check <paths>` mode — brittleness screen for a named file set"
why: >-
  The `pr` feed (brief 08) is for a PR that already exists. Authors and CI need the same
  brittleness signal BEFORE the PR — when picking up a file set to change. `qualgen check
  <paths>` screens a named set against the same features and emits advisory NOTICEs: touch
  this hotspot with a stronger execution tier, add coverage over its defect history, check its
  coupling partner. Advisory posture only — it informs the author, it does not gate the work.
wave: 2
depends: ["quality/02", "quality/03", "quality/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §9.2 — authoring/CI screen (`check <paths>`; advisory NOTICE; stronger tier / add coverage / coupling-partner check)"
  - "docs/streams/quality/spec.md §4.3–4.5 — hotspot, ownership/SPOF, change coupling (M1 features consumed)"
  - "docs/streams/quality/spec.md §4.6 — instruction-layer brittleness (consumed from brief 04)"
  - "docs/streams/quality/spec.md §3.2 — three-state; §9.2 advisory-first (hard gating is a later, separate decision)"
---

# Brief 09 — `check <paths>` mode — brittleness screen for a named file set

## Context

files:
- NEW `qualgen/check.go` (planned) (+ `qualgen/check_test.go` (planned)) — the `check <paths>` mode: take a named
  file set (globs or explicit paths), assemble each file's features, and emit advisory flags.
- REUSES `qualgen/features.go` (planned) (brief 08 factored it) if present; this brief and brief 08
  share one feature assembly, so if `features.go` does not yet exist on the branch, this brief adds it
  and brief 08 reuses it — either order, one shared assembly, no fork.
- CONSUMES M1 hotspot/ownership/coupling from briefs 02–03 and instruction-layer
  reference-validity from brief 04 (a screened instruction/config/brief file with dead
  references is a brittleness flag too, spec §4.6).
- OUTPUT is advisory text/JSON to stdout (a screen result, not a committed artifact); the
  mode is designed to run at authoring time OR in CI as a non-blocking step.

facts:
- advisory flags (spec §9.2): for a file above the brittleness signal, emit up to three
  NOTICE-level advisories — (a) use a STRONGER execution tier on this change; (b) ADD COVERAGE
  over the hotspot's defect history; (c) explicit COUPLING-PARTNER check (name the historical
  partners not in the screened set).
- ADVISORY posture (spec §9.2): the mode's exit code is a screen result, NOT a gate. Default
  is NOTICE-only — it exits 0 even when it flags. Hard gating is "a later, separate decision"
  (spec §9.2) and is explicitly out of scope here.
- GENERIC target (spec §9.2): the screened set is arbitrary — any path list — and the mode is
  runnable authoring-time or in CI. It carries no house-specific wiring.
- three-state (spec §3.2): a screened file with no measurable history emits an explicit
  `could-not-screen` note, not a clean bill of health.
- brittleness signal, not threshold: like brief 08 this mode ships no numeric cutoff of its
  own — it flags on the relative signal and names the numbers; any hard threshold is consumer
  config (spec §9.1/§9.2). Screen advises; it does not decide.

single-point-of-failure: none — an advisory screen sits beside the work, not between a fault
and damage; its NOTICE posture is deliberate, and the three-state `could-not-screen` note is
the layer that stops an unmeasurable file from reading as "safe."

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- READ-ONLY, writes NO artifact — prints a screen result to stdout.
- ADVISORY only: the mode exits 0 in its default posture even when it flags. Do NOT add a
  failing/blocking exit path — hard gating is a separate later decision (spec §9.2). If a
  design pull toward "fail CI on a flag" appears, report NEEDS_CONTEXT.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `qualgen/check.go` (planned): implement `qualgen check <paths>` — expand the path/glob set, assemble
   each file's features via the shared `FileFeatures` assembly (reuse brief 08's `features.go`;
   add it here if absent), and for each file decide which of the three advisories apply.
2. Advisory rules (spec §9.2): file above the hotspot signal → emit the **stronger-tier**
   advisory; file with traced defect-density history → emit the **add-coverage** advisory
   naming the defect count + trace-rate; file with historical coupling partners not in the
   screened set → emit the **coupling-partner** advisory naming the missing partners. An
   instruction/config/brief file with brief-04 reference-validity decay → emit a
   reference-rot advisory (spec §4.6).
3. Emit at NOTICE level with a default exit 0 (advisory posture). Structure the output so a
   consumer could later choose to gate on it, but this mode never does.
4. Three-state per screened file: a file with no measurable history emits `could-not-screen`
   with the reason, never an implied all-clear.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run 'Check'` | exit 0; check-mode tests pass |
| 3 (DEREFERENCING — hotspot file draws the stronger-tier advisory) | `cd qualgen && go test ./... -run TestCheck_HotspotFileFlaggedStrongerTier -v` | exit 0. The test builds a fixture repo with one heavily-churned complex file and one quiet file, runs `check` over both, and asserts the churned file's screen result contains the stronger-execution-tier advisory and the quiet file does NOT. |
| 4 (DEREFERENCING — coupling partner named) | `cd qualgen && go test ./... -run TestCheck_CouplingPartnerNamed -v` | exit 0. Fixture: files A and B are historically coupled; `check A` (B not in the set) asserts the result's coupling-partner advisory names B specifically. |
| 5 (DEREFERENCING — advisory posture, exit 0 on a flag) | `cd qualgen && go test ./... -run TestCheck_AdvisoryPostureExitsZeroOnFlag -v` | exit 0. The test runs `check` over a fixture hotspot file and asserts the output contains the stronger-tier advisory AND that the mode returned exit 0 — it flags but does not fail (advisory NOTICE posture, spec §9.2). |
| 6 (three-state — unmeasurable file) | `cd qualgen && go test ./... -run TestCheck_NoHistoryIsCouldNotScreen -v` | exit 0. `check` on a brand-new file asserts a `could-not-screen` note, not an all-clear. |
| 7 (no hard gate leaked) | `cd qualgen && grep -icE -e 'os.Exit\(1\)' -e 'os.Exit\(2\)' -e 'fail.*ci' -e block qualgen/check.go` | prints `0` (advisory posture — no failing/blocking exit path in this mode). |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — a read-only, write-nothing, advisory-only screen over
any named file set; emits NOTICEs and always exits 0 in its default posture). Reviewer
confirms the advisory posture is real (no blocking exit path), the three advisories fire on
the right signals, and an unmeasurable file screens as `could-not-screen` rather than clean.
Records verdict + date in the stream README table.
