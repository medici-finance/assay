---
brief: assay:assay:desk-containers:23
title: "`cellctl upgrade <cell>` — install the pinned desk-tools release and plugin, verify them, then rolling-restart the cell onto them"
why: >-
  A fix that has merged does nothing for a running cell until three hand steps happen: install
  the new pinned binaries, update the plugin, and restart every desk. On 2026-10-14 the
  launch-lock fix sat unreleased while four desks failed on exactly that bug. `upgrade` does
  the install and the restart as one verified operation, so moving a cell onto a release is
  one command and a running pipeline never fully stops.
wave: 10
depends: ["desk-containers/19", "desk-containers/20"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Incident 2026-10-14: installed v1.0.41 lacked the #2709 launch-lock fix (6d576a565, merged after the tag); four of five desks failed at launch on that bug"
  - "tools/desk/cmd/deskinstall — installs the pinned desk-tools tarball and verifies each asset's sha256 against the pin (`<asset> <tag> <sha256>`)"
  - "tools/desk/cmd/upgrade-assay and deskversion — the umbrella version marker and its dry-run-first upgrade; this verb drives them, never a second resolver"
  - "desk-containers/19 — the rolling restart, itself an adapter onto the runner-lifecycle restart sequence"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (proposed) (added by this PR) — classifies `upgrade` as an interim bridge on today's installer; the target owner of upgrades is the cell-operation domain, with release/pin migration under R8"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §One semantic owner — Cell operation owns installation, resolved configuration, upgrades, recovery and revocation"
  - "Driver answer 2026-10-14 (in session, relayed by the coordinator): the plan names every other cell sharing the binaries, and apply refuses while any of them has live desks unless they are drained first or included explicitly (`--cells a,b`)"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no cell-level upgrade verb"
exec-tier: strong
exec-tier-why: (b) crosses the installer, the version marker, the plugin cache and the restart; (c) a partially applied upgrade (new binary, old plugin) is a state unit tests do not model
domain: complicated
consumers:
  - "the cell-operation domain owner (target owner of upgrades; R8 release/pin migration): out-of-scope (this verb is an interim bridge on deskinstall; amendment 21 records the mapping)"
  - "tools/desk/cmd/cellctl (new upgrade verb): follow-up desk-containers/23 (this brief)"
  - "docs/cellctl.md: follow-up desk-containers/23 (this brief)"
  - "the console cockpit (an 'update available' affordance over `upgrade --plan`): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 23 — `cellctl upgrade <cell>`

## Context
files:
- `tools/desk/cmd/cellctl/upgrade.go` (planned), `upgrade_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `upgrade`
- `docs/cellctl.md`
- `changelog/desk-containers-23-upgrade.md` (planned)

facts:
- **Interim bridge.** `upgrade` drives today's installer (`deskinstall`, `upgrade-assay`). In the target, upgrades belong to the cell-operation domain, with release/pin migration qualified under R8. The verb stays an operator trigger over that owner and adds no upgrade logic of its own beyond sequencing and the shared-cells check.
- Target: the pin the cell already declares (the repo's pin file the cell was booted against), or `--to vX.Y.Z` (an umbrella version only, the `upgrade-assay` rule). `upgrade` never resolves "latest" on its own.
- Steps:
  1. **plan**: show the current and target versions, the assets with their sha256 values, and the roles to restart;
  2. install through `deskinstall` (sha256-verified; any mismatch stops before anything is replaced);
  3. update the plugin through the harness's own plugin update command;
  4. verify that `cellctl version` and each desk tool's `--version` report the target;
  5. the runner-lifecycle restart sequence for the cell's roles, through `restart` (rolling). Upgrade never stops or starts a role itself.
  A failure at 2–4 stops before 5, with the old binaries still in place or restored, and says which.
- **Other cells share the binaries.** The installed binaries are machine-wide, so every cell on the machine runs them. The plan names every other registered cell and, for each, whether it has live desks (from desk-containers/20's inventory). **Apply refuses (`shared-cells-live`, exit 5) while any other cell has a live desk**, unless that cell is either drained first (no live desks at apply time) or included explicitly with `--cells a,b`. An included cell is drained and rolling-restarted onto the new binaries in the same run, after the target cell. The other cells' liveness is re-measured at apply (desk-containers/15 layer 2), so a cell that came up after the plan was approved also refuses.
- TTY: one `[y/N]` on the plan. Off a TTY: `--plan` then `--apply <id>` (desk-containers/15). `--dry-run` is the plan only.

## Ground rules
- NEVER git push / trigger workflows / tag releases. Stop at `implemented`.
- Never install an unpinned or unverified asset.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `upgrade` as above; register; docs; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradePlan' -count=1` | exit 0 — the plan lists versions, asset hashes, the roles to restart, and the other cells sharing the binaries |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeShaMismatchStops' -count=1` | exit 0 — a fixture asset with a wrong sha256 stops before any binary is replaced and before any restart |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeVersionVerifyGatesRestart' -count=1` | exit 0 — when post-install `--version` reports the old tag, restart is never called |
| 3a | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeRefusesLiveSharedCells' -count=1` | exit 0 — with a second fixture cell holding a live desk lease, `upgrade <cell>` apply refuses `shared-cells-live` naming that cell, and no binary is replaced; a mutant that only warns fails |
| 3b | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeSharedCellsDrainedOrIncluded' -count=1` | exit 0 — the same apply proceeds when the other cell has no live desks, and with `--cells <other>` it drains and rolling-restarts the other cell after the target |
| 3c | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeSharedCellCameUpAfterPlan' -count=1` | exit 0 — a cell with no live desks at plan time that has one at apply time makes the apply refuse (re-measured, not taken from the plan) |
| 4 | gate:model +flow +dereference | On a disposable house cell pinned one release back: `cellctl upgrade <cell>` on a TTY, answer `y` | every desk tool reports the target version; all roles return up through a rolling restart; record the before and after `cellctl version` |
| 5 | check | `statusgen --root . --lint` | exit 0 |
| 5a | check | `statusgen --root . --consumers --brief desk-containers/23` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: installs are pinned and hash-verified, the previous release can be re-pinned and reinstalled, and no other cell's live desks are ever swapped under. The reviewer checks rows 2, 3 and 3a–3c.
