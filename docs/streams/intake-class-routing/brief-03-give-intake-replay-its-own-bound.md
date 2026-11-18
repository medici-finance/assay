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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReplayCapOwnKnob$' ./cmd/scanloop > "$o" && grep -q -- '--- PASS: TestReplayCapOwnKnob ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 2efcaa239c17 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp "${TMPDIR:-/tmp}/icr.XXXXXX") && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReplayCapLeavesBurst$' ./cmd/deskmonitor > "$o" && grep -q -- '--- PASS: TestReplayCapLeavesBurst ' "$o")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 2efcaa239c17 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:07b236730896134cdf256254a4df6c1777a64d1bf987fe76b02e2ecc18d60e26
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6827 run=b1a2b436597a099fbeebb2b577ee26da15e803903070903d92f15714df70db38 source=2efcaa239c17469e48da1727967a2df954e70d46 brief=docs/streams/intake-class-routing/brief-03-give-intake-replay-its-own-bound.md model=claude-opus-5-5 tier=any
Admission-Bracket: run=b1a2b436597a099fbeebb2b577ee26da15e803903070903d92f15714df70db38 source=2efcaa239c17469e48da1727967a2df954e70d46 tree=sha256:07b236730896134cdf256254a4df6c1777a64d1bf987fe76b02e2ecc18d60e26 witness=sha256:2ca38949f2a6ce955909bf6466bd5d0768534a390f8acf7d7f5f302c80e3133a opened=2026-11-18T02:01:32Z closed=2026-11-18T02:02:09Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `(cd tools/desk && GOWORK=off go test -count=1 ./cmd/scanloop ./cmd/deskmonitor)` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 2efcaa239c17 (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --consumers --brief assay:assay:intake-class-routing:03` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 2efcaa239c17 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6827 run=b1a2b436597a099fbeebb2b577ee26da15e803903070903d92f15714df70db38 source=2efcaa239c17469e48da1727967a2df954e70d46 brief=docs/streams/intake-class-routing/brief-03-give-intake-replay-its-own-bound.md model=claude-opus-5-5 tier=any

Rows 1 and 2 ran in the closed container-netns bracket above (verify-runner v1.0.87, arm64). Rows 3 and 4 are check:ci rows the darwin host cannot sandbox, and the container lane cannot carry them: both need the checkout's own git metadata, which a linked worktree does not bring into the container.

Supplemental observations, no acceptance credit, discharging no row:

- Container probe of row 3's command, same image and flags: the deskmonitor package passed; five scanloop tests failed (TestRun_CoalescesAcrossSessions, TestRun_ForeignScanPRIgnored, TestRun_ExplicitScanPRWins, TestRun_UnreadOpenPRsFailClosed, TestClassify_UnreadableLocalStateRoutesToJudgment). The first four fail because forge resolution falls back to the origin remote, which is unreadable without the git metadata ("cannot resolve which forge serves medici-finance/assay"). The fifth expects an unreadable file, and the container runs as root, which can still read it. None of these tests is in a file this brief changed. Date: 2026-11-18; Runner: assay-verifier-app[bot] (model verifier, claude-opus-5-5).
- Host probe of row 3's command, with no network sandbox, GOPROXY=off and the bracket's module cache: exit 0, "ok" for both packages. Date: 2026-11-18; Runner: assay-verifier-app[bot] (model verifier, claude-opus-5-5).
- Host probe of row 4's command as written: exit 2, COULD-NOT-CHECK. On merged main the diff is empty, and the delivery cannot be derived from history. With the delivery named explicitly (--delivery 7b24cdc21dc5d6b6fa99ac8c1ad559d7d8225d7f --base ebad098255fdd9bb443f3c2fe2d5df7503ed1740): exit 0, "summary: 1 corroborated, 0 disproved, 1 unchecked". The unchecked entry is the out-of-scope claim on the poller; no deskmonitor non-test source names INTAKE_REPLAY_CAP. Date: 2026-11-18; Runner: assay-verifier-app[bot] (model verifier, claude-opus-5-5).

Risk-bearing values, enumerated over the non-test Go changes in commits 8f394bc770e5 and 74646d214031 plus the brief's Deliverables: the knob names and their precedence, the positivity bound, and the default.

- RISK-VALUE: DERIVED — replayCap knob order = ["INTAKE_REPLAY_CAP", "INBOUND_MONITOR_BURST_CAP"] @ tools/desk/cmd/scanloop/observations.go:71 — Task 2 requires the new knob first and the burst knob as the unchanged fallback; the poller reads only INBOUND_MONITOR_BURST_CAP (tools/desk/cmd/deskmonitor/inbound.go:152), so its fresh-delta bound cannot move.
- RISK-VALUE: DERIVED — positivity bound v > 0 @ tools/desk/cmd/scanloop/observations.go:72 — Task 2 makes zero, negative and unparseable values fall through, so a bad value never yields a zero or negative replay bound.
- RISK-VALUE: NAMED, NOT DERIVED — replayCap default = 25 @ tools/desk/cmd/scanloop/observations.go:76 — unchanged by this diff and equal to the poller's own default (inbound.go:152); the brief states that whether 25 is the right default is not decided here.
- All three values are reversible operator knobs (an edit and a redeploy undo them); the brief marks irreversible: no.

**VERIFY: BLOCKED** — rows 1-2 pass (container witness, bracket verdict=match); rows 3-4 could-not-run: the darwin host has no network-off sandbox, and the container lane lacks the checkout's git metadata; no obligation was observed to fail.

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
