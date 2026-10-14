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
depends: ["desk-containers/19"]
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
  - "desk-containers/19 — the rolling restart"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no cell-level upgrade verb"
exec-tier: strong
exec-tier-why: (b) crosses the installer, the version marker, the plugin cache and the restart; (c) a partially applied upgrade (new binary, old plugin) is a state unit tests do not model
domain: complicated
consumers:
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
- Target: the pin the cell already declares (the repo's pin file the cell was booted against), or `--to vX.Y.Z` (an umbrella version only, the `upgrade-assay` rule). `upgrade` never resolves "latest" on its own.
- Steps:
  1. **plan**: show the current and target versions, the assets with their sha256 values, and the roles to restart;
  2. install through `deskinstall` (sha256-verified; any mismatch stops before anything is replaced);
  3. update the plugin through the harness's own plugin update command;
  4. verify that `cellctl version` and each desk tool's `--version` report the target;
  5. `restart` (rolling).
  A failure at 2–4 stops before 5, with the old binaries still in place or restored, and says which.
- The installed binaries are machine-wide: every cell on the machine uses them. The plan lists the other registered cells, and the confirmation names them. Upgrading one cell restarts only that cell; other cells pick up the binaries at their next restart.
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
| 4 | gate:model +flow +dereference | On a disposable house cell pinned one release back: `cellctl upgrade <cell>` on a TTY, answer `y` | every desk tool reports the target version; all roles return up through a rolling restart; record the before and after `cellctl version` |
| 5 | check | `statusgen --root . --lint` | exit 0 |
| 5a | check | `statusgen --root . --consumers --brief desk-containers/23` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: installs are pinned and hash-verified, and the previous release can be re-pinned and reinstalled. The reviewer checks rows 2 and 3, and that the plan names the other cells sharing the binaries.
