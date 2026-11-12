---
brief: assay:assay:intake-class-routing:03
title: Give intake replay its own bound
why: >-
  One intake pass presents at most 25 retained issues per repository, and that bound is the
  poller's burst knob. Draining a backlog of several hundred owed judgments needs a larger
  batch, but raising the shared knob also loosens the bound that protects against a burst of
  fresh events. A separate knob lets an operator widen a drain and leave that protection alone.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
outcome: none
schema: brief-v2
version: 1
id: 74e00a19-5f99-41c6-8c69-f47c55c18f28
authored: "2026-11-12 by a strong-tier authoring session (read-only review)"
domain: clear
sources:
  - "docs/streams/intake-class-routing/README.md §1.1, §2 item 4, §8, §10"
  - "freshness-checked 2026-11-12 @ 0665bfb77 (origin/main): replayCap reads only INBOUND_MONITOR_BURST_CAP"
consumers:
  - "tools/desk/cmd/scanloop/observations.go: follow-up intake-class-routing/03 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskmonitor/inbound.go: out-of-scope (the poller keeps reading only its own burst knob; row 2, a test in the poller's package, proves it is unaffected)"
---

# Brief 03 — Give intake replay its own bound

## Context

files: tools/desk/cmd/scanloop/observations.go, tools/desk/cmd/scanloop/observations_test.go, tools/desk/cmd/scanloop/doc.go, tools/desk/cmd/deskmonitor/properties_test.go, changelog/intake-replay-bound.md

facts:
- Read at `0665bfb77` (2026-11-12).
- `replayCap()` in `tools/desk/cmd/scanloop/observations.go:65` returns `INBOUND_MONITOR_BURST_CAP` when it is a positive integer, else 25 (`grep -n -A6 'func replayCap' tools/desk/cmd/scanloop/observations.go`).
- The poller reads the same variable as its per-repository burst bound (`tools/desk/cmd/deskmonitor/inbound.go:152`). The poller is a separate binary that `scanloop` runs with its own environment (`tools/desk/cmd/scanloop/monitor.go:537`); no `scanloop` function computes the burst bound, so only a test in the poller's package can show it unmoved. `TestMonitorBurstCollapse` (`tools/desk/cmd/deskmonitor/properties_test.go:203`) already exercises the collapse.
- The shared knob is deliberate, not incidental. The comment on `replayCap` says retention reuses the poller's burst knob "so retention cannot route around the bound the poller already applies to a fresh delta" (`observations.go:58`). The burst bound limits how much of a sudden flood one pass is shown; the replay bound stops the retained remainder of that flood from arriving all at once on the next pass.
- Why a separate knob is still safe. It is opt-in: unset, replay keeps today's coupling exactly. An operator who sets it above the burst value chooses a larger judged batch per pass for a backlog that is already retained. The poller's bound on a fresh delta does not move, every replayed event still passes the current trust check when it is presented, and the pass still prints the effective bound in its deferred line.
- A pass already names what it held back: `<repo> <n> retained observation(s) deferred (replay cap <c> per repo; retained, oldest first)` (`observations.go:379`). That line stays.
- Observed on one installation on 2026-11-12: 147 owed judgments presented, with five of seven repositories at the bound.
- This is a new knob, not a defect fix, so no class-guard row applies.

design-fit:
  owner: tools/desk/cmd/scanloop (the replay bound)
  contract: none — an operator knob; no registered meaning changes
  retires: []
  weight: "verbs 0, flags 0, environment knobs +1, refusals 0, rule-text lines 0"
  why-add: "The bound cannot be widened today without also widening the poller's burst protection. Removal considered: dropping the replay bound entirely, which would present an unbounded batch to one pass."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Re-check the facts. If `replayCap` already reads a second variable, report NEEDS_CONTEXT.
2. Make `replayCap()` read `INTAKE_REPLAY_CAP` first. A positive integer there wins. Unset, empty, zero, negative or unparseable falls through to today's behaviour unchanged: `INBOUND_MONITOR_BURST_CAP`, then 25. Rewrite the comment on `replayCap` so it stays true: the default still reuses the burst knob for the reason it gives, and the new knob is an operator's explicit choice to present a larger retained batch.
3. Keep the deferred line's wording and make its printed cap the effective value.
4. Document the knob where `scanloop`'s other environment knobs are documented, in two or three lines: the fallback order, and that a value above the burst bound enlarges the judged batch per pass without changing what the poller admits.
5. Add `TestReplayCapOwnKnob` (planned): with `INTAKE_REPLAY_CAP=3` and the burst knob unset, a pass over eight retained events for one repository presents three and defers five, and the deferred line prints cap 3. With `INTAKE_REPLAY_CAP` unset and the burst knob at 4, it presents four.
6. Add `TestReplayCapLeavesBurst` (planned) in `tools/desk/cmd/deskmonitor`, beside `TestMonitorBurstCollapse` and using its fixture: with `INTAKE_REPLAY_CAP=3` and `INBOUND_MONITOR_BURST_CAP=7` in the poller's environment, a fresh delta of more than seven events for one repository collapses at 7, not 3. The same run with `INTAKE_REPLAY_CAP` unset gives the same output. The test goes red if the poller ever reads the replay knob.
7. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReplayCapOwnKnob$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestReplayCapOwnKnob ' "$o")` | exit 0 and the named PASS; the new knob sets the replay bound and its absence keeps today's fallback |
| 2 | check:ci +neighbour +flow | tree | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReplayCapLeavesBurst$' ./cmd/deskmonitor > "$o" && grep -q -- '--- PASS: TestReplayCapLeavesBurst ' "$o")` | exit 0 and the named PASS; run in the poller's own package, a fresh delta still collapses at the burst bound when the replay knob is set lower |
| 3 | check:ci | tree | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/scanloop ./cmd/deskmonitor)` | exit 0; output contains "ok" |
| 4 | check:ci | tree | `statusgen --consumers --brief assay:assay:intake-class-routing:03` | exit 0; output contains "0 disproved" |

Failure modes mapped: the new knob ignored → row 1; the fallback changed for installations that set nothing → row 1's second case; the poller starting to read the replay knob, which would loosen or tighten its burst bound → row 2; a regression in the existing replay or poller tests → row 3. Whether 25 remains the right default is not decided here.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
