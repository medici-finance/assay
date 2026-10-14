---
brief: assay:assay:desk-containers:16
title: "Cell-scoped stop flags — `STOP.cell.<cell>` and `STOP.cell.<cell>.<loop>`, honoured by the Guard for any session carrying the cell marker"
why: >-
  Pausing one cell's desks today means touching `STOP` or `STOP.<loop>` in the operator's
  config home. A house cell's home links to that same directory, so the flag also stops every
  other cell on the machine that runs a loop with that name. A `drain` or `pause` verb cannot
  be built on a switch that cannot be aimed. This brief adds a stop flag scoped to one cell,
  enforced in the same two places the existing flags are: the loop's own check and the tool
  Guard.
wave: 7
depends: []
unblocks: ["desk-containers/18"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-14: a `drain <cell>` that drains the loops first and then shuts down"
  - "tools/desk/README.md §Runtime state — `STOP` halts every loop at its next iteration boundary, `STOP.<loop-name>` halts one loop, precedence `DISABLED` > `STOP` > `STOP.<name>`, flags are not self-clearing, names resolved through `tools/desk/internal/deskkit/loopnames.go`"
  - "plugins/assay/skills/{pr-review-desk,verify-desk,worker-desk}/SKILL.md — each loop's iteration-boundary check reads `$HOME/.config/assay/STOP` and `STOP.$DESK_LOOP`; `deskkit.Guard()` enforces the same flags independently before every outward verb"
  - "docs/cellctl.md — a house cell's `home/.config/assay` is a symlink to the operator's real config home, so today's flags are machine-wide for every house cell"
  - "#2880 — introduces a cell marker in the environment of every child `cellctl` launches (proposed `ASSAY_CELL=<cell>` plus the role); this brief reads that marker and must use the name #2880 lands"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no cell-scoped stop flag exists; `deskkit.Guard()` knows `DISABLED`, `STOP`, `STOP.<name>`, `STOP.run.<key>` and `HEARTBEAT`"
exec-tier: strong
exec-tier-why: (b) the flag is a shared value read by the Guard, five skill bodies and cellctl; (c) safety plumbing, where a silent no-op stop is the failure that unit tests miss
domain: complicated
consumers:
  - "tools/desk/internal/deskkit (Guard and the stop-flag reader): follow-up desk-containers/16 (this brief)"
  - "plugins/assay/skills/{pr-review-desk,verify-desk,worker-desk,intake-desk,the-desk}/SKILL.md iteration-boundary check: follow-up desk-containers/16 (this brief; one added line each, in the same shape as the existing two)"
  - "tools/desk/cmd/deskboard (STOP display, inert-name marking): follow-up desk-containers/16 (this brief; shows cell-scoped flags and marks an unknown cell inert)"
  - "tools/desk/cmd/cellctl launch environment (the cell marker): out-of-scope (#2880 adds it; this brief only reads it)"
  - "tools/desk/README.md §Runtime state: follow-up desk-containers/16 (this brief)"
---

# Brief 16 — Cell-scoped stop flags

## Context
files:
- `tools/desk/internal/deskkit/` — the stop-flag reader behind `Guard()`, plus tests
- `plugins/assay/skills/{pr-review-desk,verify-desk,worker-desk,intake-desk,the-desk}/SKILL.md` — the iteration-boundary check
- `tools/desk/cmd/deskboard/` — the stop display
- `tools/desk/README.md` — §Runtime state
- `changelog/desk-containers-16-cell-stop-flags.md` (planned)

facts:
- New flags, in the same directory as today's: `STOP.cell.<cell>` stops every loop of that cell; `STOP.cell.<cell>.<loop>` stops one loop of that cell. `<loop>` resolves through `loopnames.go` exactly as `STOP.<loop>` does.
- A session belongs to a cell only through the cell marker env var that #2880 adds. A session with no marker is unaffected by cell flags. Cell flags only ever add stops; they never clear or override `DISABLED`, `STOP` or `STOP.<loop>`.
- Precedence: `DISABLED` > `STOP` > `STOP.cell.<cell>` > `STOP.<loop>` = `STOP.cell.<cell>.<loop>`. Any one present is a stop.
- Layer 1 is the loop's own iteration-boundary check in each skill body. Layer 2 is `Guard()` before every outward verb. They fail for different reasons in different components: a skill that skips its check is still refused by the Guard; a binary that predates this brief still stops at the skill check.
- A cell flag naming a cell that has no registration under the cells root is reported **inert** by `deskboard`, the same treatment an unknown loop name gets today.
- Cell names are validated (`[a-z0-9-]`, the existing cell-name rule) before any path is built from them.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- Do not change the semantics of any existing flag.
- If #2880's marker name differs from `ASSAY_CELL`, use #2880's name and say so in the PR body.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Extend the deskkit stop reader and `Guard()` with the two cell-scoped flags and the precedence above.
2. Add the cell-flag line to the five skill bodies' iteration-boundary checks.
3. Show cell flags in `deskboard`, with the inert marking.
4. Document the flags in `tools/desk/README.md` §Runtime state; write the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./internal/deskkit/ -run 'TestGuardCellStop' -count=1` | exit 0 — with the marker `cell=a`: `STOP.cell.a` stops; `STOP.cell.a.worker-desk` stops worker-desk only; `STOP.cell.b` does not stop it; with no marker, no cell flag stops it |
| 2 | check | `go -C tools/desk test ./internal/deskkit/ -run 'TestGuardCellStopNeverClears' -count=1` | exit 0 — `STOP` present plus a cell marker for a different cell is still a stop |
| 3 | check | `go -C tools/desk test ./internal/deskkit/ -run 'TestCellNameRejectsTraversal' -count=1` | exit 0 — a marker value of `../x` or `a/b` is refused exit 6 and builds no path |
| 4 | check +dereference | `grep -c 'STOP.cell.' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/intake-desk/SKILL.md plugins/assay/skills/the-desk/SKILL.md` | every file ≥ 1 |
| 5 | gate:model +flow | In a temp HOME: set the marker `cell=a`, touch `STOP.cell.a`, then run an outward verb such as `deskfile check` with `DESK_LOOP=worker-desk` | the Guard refuses with the stop exit code, naming `STOP.cell.a`. Remove the flag and the same command proceeds. With the skill-body check bypassed, the Guard alone still refuses. |
| 6 | check | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: the change only adds stops and is removed with `rm`. The reviewer checks that rows 1, 2 and 5 exercise the Guard and the skill check independently.
