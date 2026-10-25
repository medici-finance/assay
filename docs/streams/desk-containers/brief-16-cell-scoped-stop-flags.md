---
brief: assay:assay:desk-containers:16
title: "Interim mechanism for the runner-lifecycle `pause` transition — cell-scoped stop flags `STOP.cell.<cell>[.<loop>]`, honoured by the Guard for any session carrying the cell marker"
why: >-
  Pausing one cell's desks today means touching `STOP` or `STOP.<loop>` in the operator's
  config home. A house cell's home links to that same directory, so the flag also stops every
  other cell on the machine that runs a loop with that name. A `drain` or `pause` verb cannot
  be built on a switch that cannot be aimed. In the target architecture, pause is a
  transition of the one runner lifecycle and ownership contract, not a flag file. Until that
  contract ships, this brief gives the transition an interim first implementation scoped to
  one cell, enforced in the same two places the existing flags are: the loop's own check and
  the tool Guard.
wave: 7
depends: []
unblocks: ["desk-containers/18", "desk-containers/20"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
decision-trigger: creation
decision-issue: 4401
ruling: "https://github.com/medici-finance/assay/issues/4401#issuecomment-6399335268"
design: DR-cell-stop-flags
issues: []
schema: brief-v2
version: 2
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-14: a `drain <cell>` that drains the loops first and then shuts down; the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge): drain/pause/resume/restart/handoff are runner-lifecycle states and transitions, with today's STOP-flag/cadence mechanism labelled interim"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §What happens to today's tools — 'loopengine, drainloop, cellctl cadence, loopadmin, graph controller → One runner lifecycle and ownership contract; reuse implementations against it'"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) — defines the lifecycle states and transitions and maps `pause`/`resume` to this interim mechanism"
  - "tools/desk/README.md §Runtime state — `STOP` halts every loop at its next iteration boundary, `STOP.<loop-name>` halts one loop, precedence `DISABLED` > `STOP` > `STOP.<name>`, flags are not self-clearing, names resolved through `tools/desk/internal/deskkit/loopnames.go`"
  - "plugins/assay/skills/{pr-review-desk,verify-desk,worker-desk}/SKILL.md — each loop's iteration-boundary check reads `$HOME/.config/assay/STOP` and `STOP.$DESK_LOOP`; `deskkit.Guard()` enforces the same flags independently before every outward verb"
  - "docs/cellctl.md — a house cell's `home/.config/assay` is a symlink to the operator's real config home, so today's flags are machine-wide for every house cell"
  - "#2880 (PR #2884) — sets the cell marker `ASSAY_CELL` (plus `ASSAY_CELL_DIR`, `ASSAY_CELL_ROLE`) on every process cellctl launches, whether or not comms is configured; `DESK_CELL`/`DESK_ROLE` stay comms-only. The marker name is the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge)"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no cell-scoped stop flag exists; `deskkit.Guard()` knows `DISABLED`, `STOP`, `STOP.<name>`, `STOP.run.<key>` and `HEARTBEAT`"
gate-why: >-
  This changes `deskkit.Guard()`, the tool-layer kill switch every desk verb consults. The risk
  answers are all no, but the path is the suite's safety plumbing, and a stop flag that
  silently matches nothing reads as "stopped" while desks keep writing. What the human
  confirms: cell flags only ever ADD stops (never clear or outrank DISABLED, STOP or
  STOP.<loop>); cell membership comes only from the launcher-set marker; a cell flag
  whose cell name is malformed or whose loop part names no known loop is reported inert, and one whose
  cell has no registration under the reader's cells root is reported with that root named, not honoured silently; and the Guard enforces it for every loop, with the iteration check of the three loops that have one (pr-review-desk, verify-desk, worker-desk) as a second layer; intake-desk and the-desk have no iteration check yet, so for them the Guard is the only layer.
exec-tier: strong
exec-tier-why: (b) the flag is a shared value read by the Guard, three skill bodies and cellctl; (c) safety plumbing, where a silent no-op stop is the failure that unit tests miss
domain: complicated
consumers:
  - "the runner lifecycle and ownership contract (target owner of pause/resume): out-of-scope (graph-execution runtime-coordination/runner packages R2/R3; this brief is the interim adapter and is retired when that contract ships, per docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
  - "tools/desk/internal/deskkit (Guard and the stop-flag reader): follow-up desk-containers/16 (this brief)"
  - "plugins/assay/skills/{pr-review-desk,verify-desk,worker-desk}/SKILL.md iteration-boundary check (the three with an existing STOP check): follow-up desk-containers/16 (this brief; one added line each, beside the existing two)"
  - "plugins/assay/skills/{intake-desk,the-desk}/SKILL.md: out-of-scope (they have no iteration-boundary STOP check today; adding one is new loop behaviour this brief does not take on — for these two the Guard is the only layer, as it already is for STOP and STOP.<loop>)"
  - ".cursor/skills/** (generated mirror of the skill bodies): follow-up desk-containers/16 (this brief; regenerated with `deskinstall --harness cursor`, never hand-edited)"
  - "tools/desk/cmd/deskboard (STOP display, inert-name marking): follow-up desk-containers/16 (this brief; shows cell-scoped flags and marks a malformed cell name or unknown loop inert, and a cell unregistered under its cells root with that root named)"
  - "tools/desk/cmd/cellctl launch environment (`ASSAY_CELL`, `ASSAY_CELL_DIR`, `ASSAY_CELL_ROLE`): out-of-scope (#2880 / PR #2884 sets them; this brief only reads `ASSAY_CELL` and `ASSAY_CELL_ROLE`)"
  - "tools/desk/README.md §Runtime state: follow-up desk-containers/16 (this brief)"
---

# Brief 16 — Cell-scoped stop flags

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context
files:
- `tools/desk/internal/deskkit/` — the stop-flag reader behind `Guard()`, plus tests
- `plugins/assay/skills/{pr-review-desk,verify-desk,worker-desk}/SKILL.md` — the iteration-boundary check (and the regenerated `.cursor/skills/` mirror)
- `tools/desk/internal/deskkit/skillstop_test.go` (planned) — runs the three skills' extracted iteration-boundary snippet with the Guard absent
- `tools/desk/cmd/deskboard/` — the stop display
- `tools/desk/README.md` — §Runtime state
- `changelog/desk-containers-16-cell-stop-flags.md` (planned)

facts:
- **Interim, not the design.** This is the first implementation of the lifecycle `pause` / `resume` transitions defined in amendment 21. The runner lifecycle contract replaces it, and the replacement must keep this brief's two guarantees: a pause aimed at one cell never stops another cell, and a stop only ever adds.
- New flags, in the same directory as today's: `STOP.cell.<cell>` stops every loop of that cell; `STOP.cell.<cell>.<loop>` stops one loop of that cell. `<loop>` resolves through `loopnames.go` exactly as `STOP.<loop>` does.
- A session belongs to a cell only through `ASSAY_CELL` (its role through `ASSAY_CELL_ROLE`), which #2880 sets on every launched process. `ASSAY_CELL*` are process-environment markers (constants in `tools/desk/internal/cellstop/marked.go`), never `roster.env` keys, so the fail-closed roster key schema does not apply to them. `DESK_CELL` is comms-only and is not read. A session with no marker is unaffected by cell flags. Cell flags only ever add stops; they never clear or override `DISABLED`, `STOP` or `STOP.<loop>`.
- Precedence: `DISABLED` > `STOP` > `STOP.cell.<cell>` > `STOP.<loop>` = `STOP.cell.<cell>.<loop>`. Any one present is a stop.
- **Baseline (checked 2026-10-14 @ 7bb318197):** `grep -c 'config/assay/STOP'` is 2 in pr-review-desk, verify-desk and worker-desk, and 0 in intake-desk and the-desk. Only those three loops have an iteration-boundary STOP check today, and only they gain the cell line. intake-desk and the-desk keep relying on the Guard alone, as they already do for `STOP` and `STOP.<loop>`.
- **Threat model (same OS account).** Any desk on the machine can write `STOP.cell.<other-cell>` and pause another cell, and a session can drop its own marker. Neither is an escalation over today's machine-wide `STOP`, which any desk can already write.
- Layer 1 is the loop's own iteration-boundary check in the three skill bodies above. Layer 2 is `Guard()` before every outward verb. They fail for different reasons in different components: a skill that skips its check is still refused by the Guard; a binary that predates this brief still stops at the skill check.
- A cell flag whose cell name is malformed, or whose loop part names no known loop, is reported **inert** by `deskboard` (it halts nothing), the same treatment an unknown loop name gets today. A cell flag naming a cell with no registration under the cells root **as `deskboard` resolves it** is also tagged inert, but its reason names that root and says the flag may still be live: the Guard keys on the session's marker alone, so a cell registered under another root is still stopped. It is never reported as halting nothing.
- Cell names are validated (`[a-z0-9-]`, the existing cell-name rule) before any path is built from them.

## Human decision
Should desks be stoppable one cell at a time, by adding a cell-scoped stop flag to the safety check every desk tool runs?

Today a stop switch either stops every desk on the machine or every desk with a given role, across all cells, because host cells share one settings folder. This adds a switch aimed at one cell (and one for one role in one cell). It can only add stops; it never lifts an existing stop or the master kill switch. A desk belongs to a cell only through a marker the launcher sets. A switch naming a cell with no registration under the cells root the board resolves is tagged INERT on the board, its reason naming that root and saying the cell may be registered elsewhere, rather than silently ignored.

Options:
1. **Adopt** — add both cell-scoped flags to the safety check and to the loop check of the three desks that have one. Drain and pause (brief 18) build on it.
2. **Adopt the loop check only** — desks honour the cell flag in their own loop, but the tool-level safety check is unchanged. Weaker: a desk that skips its own check keeps writing.
3. **Reject** — drain and pause use the machine-wide flags, stopping every cell's matching desks.

Default if no answer: none — blocks until answered.

**Ruled 2026-10-25: option 1, adopt**, per the driver's ruling on decision issue
medici-finance/assay#4401:
https://github.com/medici-finance/assay/issues/4401#issuecomment-6399335268
The ruling keeps the implementation already on main, delivered by medici-finance/assay#2971
(merged 2026-10-16, trailer `Brief: desk-containers/16`). The adopted design is recorded in
[DR-cell-stop-flags](../decisions/DR-cell-stop-flags.md).

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- Do not change the semantics of any existing flag.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner\'s delta; unchanged obligations remain.

## Task
1. Extend the deskkit stop reader and `Guard()` with the two cell-scoped flags and the precedence above.
2. Add the cell-flag line to the three skill bodies that have an iteration-boundary check; regenerate the `.cursor/skills/` mirror.
3. Show cell flags in `deskboard`, with the inert marking.
4. Document the flags in `tools/desk/README.md` §Runtime state; write the changelog fragment.

- **Instance amendment:** Preserve existing machine/cell/role STOP meanings and add the instance-targeted stop mapping through the common lifecycle owner. An instance pause holds only its new assignments; role/cell stops hold every matching instance. Resume removes only the stop it owns, and up never removes a pause to satisfy population. Desired paused and runtime persistent stops are both respected.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./internal/deskkit/ -run 'TestGuardCellStop' -count=1 -v` | `--- PASS` — with the marker `cell=a`: `STOP.cell.a` stops; `STOP.cell.a.worker-desk` stops worker-desk only; `STOP.cell.b` does not stop it; with no marker, no cell flag stops it |
| 2 | check | `go -C tools/desk test ./internal/deskkit/ -run 'TestGuardCellStopNeverClears' -count=1 -v` | `--- PASS` — `STOP` present plus a cell marker for a different cell is still a stop |
| 3 | check | `go -C tools/desk test ./internal/deskkit/ -run 'TestCellNameRejectsTraversal' -count=1 -v` | `--- PASS` — a marker value of `../x` or `a/b` is refused exit 6 and builds no path |
| 4 | check +dereference | `grep -c 'STOP.cell.' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md` | every file ≥ 1 |
| 4b | check | `go -C tools/desk test ./internal/deskkit/ -run 'TestSkillStopSnippetGuardBypassed' -count=1 -v` | `--- PASS` — the iteration-boundary snippet extracted from each of the three SKILL.md files, run in a temp HOME with the marker `cell=a`, `STOP.cell.a` present and no Guard on PATH, exits the loop and names the flag (the skill layer holds with the Guard layer absent) |
| 4c | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestStampCellMarkerReplacesInherited\|TestCockpitLaunchCarriesNoMarker' -count=1 -v` | `--- PASS` for both — #2880's (PR #2884) tests: every launched process carries `ASSAY_CELL`, `ASSAY_CELL_DIR` and `ASSAY_CELL_ROLE` (an inherited marker is replaced), and a cockpit launch carries no marker. Both tests are on main since #2884 merged (2026-10-14) |
| 5 | gate:model +flow | In a temp HOME: set the marker `cell=a`, touch `STOP.cell.a`, then run an outward verb such as `deskfile check` with `DESK_LOOP=worker-desk` | the Guard refuses with the stop exit code, naming `STOP.cell.a`. Remove the flag and the same command proceeds. With the skill-body check bypassed, the Guard alone still refuses. |
| 6 | check | `statusgen --root . --lint` | exit 0 |
| 6a | check | `statusgen --root . --consumers --brief desk-containers/16` | exit 0 — the `consumers:` routing holds against the branch diff |
| 6b | check +mutation | `go -C tools/desk run ./cmd/muhar -spec cmd/deskboard/cellstopbanner-mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, `Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate`. The three mutants are planted in `tools/desk/cmd/deskboard/main.go`'s stop banner and each reddens `TestBannerShowsCellStopFlags` (`go -C tools/desk test ./cmd/deskboard/ -run '^TestBannerShowsCellStopFlags$' -count=1 -v` is `--- PASS` unmutated): the INERT tag dropped (a flag that halts nothing reads as armed), the INERT tag inverted (a live `STOP.cell.<cell>[.<loop>]` flag reads as inert), and the banner's read of active flags switched off (a cell-scoped drain is invisible on every board read) |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestDeskInstanceStops$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceStops ' "$o")` | Named test executes (zero matching tests is not a pass); Pause one of two workers, verify the peer can work, then role-stop both; resume cannot lift a foreign stop and repeated up preserves the paused member. Mutation: use only the role stop path for an instance makes the test fail. |
## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (see gate-why). All four risk answers are no; the gate is for the safety-plumbing path. Rows 1, 2 and 5 exercise the Guard; row 4b exercises the skill-layer check with the Guard absent. The reviewer checks that the two layers are proven independently.
