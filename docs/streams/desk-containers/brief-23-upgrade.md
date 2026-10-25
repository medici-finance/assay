---
brief: assay:assay:desk-containers:23
title: "`cellctl upgrade <cell>` — install the pinned desk-tools release and plugin, verify them, then rolling-restart the cell onto them"
why: >-
  A fix that has merged does nothing for a running cell until three hand steps happen: install
  the new pinned binaries, update the plugin, and restart every desk. On 2026-10-14 the
  launch-lock fix sat unreleased while four desks failed on exactly that bug. `upgrade` does
  the install and the restart as one verified operation, so moving a cell onto a release is
  one command and a running pipeline never fully stops.
wave: 12
depends: ["desk-containers/19", "desk-containers/20"]
unblocks: ["desk-containers/21"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Incident 2026-10-14: installed v1.0.41 lacked the #2709 launch-lock fix (6d576a565, merged after the tag); four of five desks failed at launch on that bug"
  - "tools/desk/cmd/deskinstall — installs the pinned desk-tools tarball and verifies each asset's sha256 against the pin (`<asset> <tag> <sha256>`)"
  - "tools/desk/cmd/upgrade-assay and deskversion — the umbrella version marker and its dry-run-first upgrade; this verb drives them, never a second resolver"
  - "desk-containers/19 — the rolling restart, itself an adapter onto the runner-lifecycle restart sequence"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) — classifies `upgrade` as an interim bridge on today's installer; the target owner of upgrades is the cell-operation domain, with release/pin migration under R8"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §One semantic owner — Cell operation owns installation, resolved configuration, upgrades, recovery and revocation"
  - "The driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge): the plan names every other cell sharing the binaries, and apply refuses while any of them has live desks unless they are drained first or included explicitly (`--cells a,b`)"
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

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context
files:
- `tools/desk/cmd/cellctl/upgrade.go` (planned), `upgrade_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `upgrade`
- `plugins/assay/paired-versions.yaml` — add the plugin `digest` field (schema and reader; the release step populates it)
- `docs/cellctl.md`
- `changelog/desk-containers-23-upgrade.md` (planned)

facts:
- **Interim bridge.** `upgrade` drives today's installer (`deskinstall`, `upgrade-assay`). In the target, upgrades belong to the cell-operation domain, with release/pin migration qualified under R8. The verb stays an operator trigger over that owner and adds no upgrade logic of its own beyond sequencing and the shared-cells check.
- Target: the pin the cell declares, or `--to vX.Y.Z` (an umbrella version only, the `upgrade-assay` rule). `upgrade` never resolves "latest" on its own.
- **Provenance: where the trusted hashes come from.** Pins (tool assets and the plugin) are read from a ref **fetched from the forge in this run**: `git fetch <forge-url> <ref>` into a temporary ref, or a release tag object whose signature is verified, then read with `git show <that ref>:<path>`. Never from a working tree, and never from a local `origin/main`, which the same account can move. Within the interim same-account boundary (desk-containers/15) this narrows tampering but does not eliminate it. For `--to`, the pins come from that release's pin manifest at its fetched tag. Every asset is verified against the pinned sha256. Where the release publishes a signature or build attestation, it is verified too, and failure stops the run. Where none is published, the plan says so explicitly, and the interactive confirm shows it.
- Steps:
  1. **plan**: show the current and target versions, the assets with their sha256 values, and the roles to restart;
  2. install through `deskinstall` (sha256-verified against the pins read from the fetched ref; signature/attestation verified where published; any mismatch stops before anything is replaced), keeping the previous binaries for restore;
  3. install the **pinned plugin**: the plugin bundle at the pinned version, verified against its pinned digest before it replaces the installed plugin. If the harness's own update command cannot be pinned to that exact version and digest, `upgrade` does not run it: the plan marks the plugin `not upgraded` and refuses unless `--skip-plugin` is given, which the confirm text then shows. **Where the digest lives:** a `digest` field (sha256 of the plugin bundle) beside `plugin:` in `plugins/assay/paired-versions.yaml`, added by this brief, and populated at release by the step that already stamps the plugin version there. Until a release carries it, every `upgrade` refuses the plugin step without `--skip-plugin`, which is stated in the plan;
  4. verify that `cellctl version`, each desk tool's `--version` and the installed plugin's version/digest report the target;
  5. the runner-lifecycle restart sequence for the cell's roles, through `restart` (rolling). Upgrade never stops or starts a role itself.
  A failure at 2–4 stops before 5 and **restores** the previous binaries and plugin. The result names which component failed and confirms that the restore was verified (old versions reported).
- **Other cells share the binaries.** The installed binaries are machine-wide, so every cell on the machine runs them. The plan names every other registered cell and, for each, whether it has live desks (from desk-containers/20's inventory). **Apply refuses (`shared-cells-live`, exit 5) while any other cell has a live desk, or its liveness is could-not-check (unknown counts as live)**, unless that cell is either drained first (no live desks at apply time) or included explicitly with `--cells a,b`. An included cell is drained and rolling-restarted onto the new binaries in the same run, after the target cell. **Cross-cell scope:** the plan's `cells` lists the target and every included cell, and the interactive confirm shows every one of them; an action on a cell absent from `cells` is refused. The other cells' liveness is re-measured at apply (desk-containers/15), so a cell that came up after the plan was written also refuses.
- **Bare `upgrade` is read-only, on a TTY or off one:** it prints what it would do (versions, assets, roles, other cells), writes no plan and installs nothing. The plan comes only from `upgrade --plan`, which persists it under the cell's `run/plans/` (desk-containers/15) and prints its id. It applies **only** through `upgrade --apply <plan-id>`: on a TTY the operator types the plan's confirm code (a convenience confirm, not an authorization control). Off a TTY, `upgrade --apply` is refused `not-interactive` (desk-containers/15 row 8). `--dry-run` is an alias for the bare form.

## Ground rules
- CLI built on the desktools-v2 Cobra/Viper foundation (desktools-v2 cli-contract.md §7): every new command, verb or launcher this brief adds uses that shared foundation (added 2026-10-18).
- NEVER git push / trigger workflows / tag releases. Stop at `implemented`.
- Never install an unpinned or unverified asset.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner\'s delta; unchanged obligations remain.

## Task
1. Implement `upgrade` as above; register; docs; changelog fragment.

- **Instance amendment:** Apply upgrade to desired population through the qualified lifecycle owner, preserving intentionally paused peers and draining predecessors. Reuse /19 overlap only when the execution/profile and binary installation permit it; keep refusal of unsafe shared-binary swaps under live desks and existing migration/rollback gates. Track partial instance progress rather than one success per role.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradePlan' -count=1 -v` | `--- PASS` — the plan lists versions, asset hashes, the roles to restart, and the other cells sharing the binaries |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeShaMismatchStops' -count=1 -v` | `--- PASS` — a fixture asset with a wrong sha256 stops before any binary is replaced and before any restart |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeVersionVerifyGatesRestart' -count=1 -v` | `--- PASS` — when post-install `--version` reports the old tag, restart is never called |
| 3a | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeRefusesLiveSharedCells' -count=1 -v` | `--- PASS` — with a second fixture cell holding a live desk lease, `upgrade <cell>` apply refuses `shared-cells-live` naming that cell, and no binary is replaced; a mutant that only warns fails |
| 3b | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeSharedCellsDrainedOrIncluded' -count=1 -v` | `--- PASS` — the same apply proceeds when the other cell has no live desks, and with `--cells <other>` it drains and rolling-restarts the other cell after the target |
| 3c | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeSharedCellCameUpAfterPlan' -count=1 -v` | `--- PASS` — a cell with no live desks at plan time that has one at apply time makes the apply refuse (re-measured, not taken from the plan) |
| 3d | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeUnknownLivenessRefuses' -count=1 -v` | `--- PASS` — another cell whose liveness probe returns could-not-check makes apply refuse `shared-cells-live` |
| 3e | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeCellsScoped' -count=1 -v` | `--- PASS` — `--cells other` puts `other` in the plan's `cells` and in the confirm text; a plan edited to act on a cell absent from `cells` is refused, and nothing is installed |
| 3f | check +mutation | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradePinsFromFetchedRef' -count=1 -v` | `--- PASS` — a pin file edited in the working tree, and a local `origin/main` moved to a commit with different pins, both leave the hashes used unchanged (they come from the ref fetched in this run); a mutant reading either fails |
| 3g | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradePluginPinnedOrRefused' -count=1 -v` | `--- PASS` — a plugin bundle whose digest mismatches the pin is refused before replacing the installed plugin; a pin file without a plugin `digest` refuses the plugin step unless `--skip-plugin` is given |
| 3h | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeRestoreOnFailure' -count=1 -v` | `--- PASS` — a failure at the version-verify step restores the previous binaries and plugin; the result reports the restored versions and no restart ran |
| 4 | gate:model +flow +dereference | On a disposable house cell pinned one release back: `cellctl upgrade <cell> --plan` for the plan id, then `cellctl upgrade <cell> --apply <plan-id>` on a TTY, typing the plan's confirm code | every desk tool reports the target version; all roles return up through a rolling restart; record the before and after `cellctl version` |
| 3i | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeApplyRefusesOffTTY' -count=1 -v` | `--- PASS` — `upgrade --apply <plan-id>` with stdin or stdout not a TTY is refused `not-interactive`: nothing installed, no restart |
| 3j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestBareUpgradeChangesNothing' -count=1 -v` | `--- PASS` — bare `upgrade <cell>` on a pseudo-terminal and off one installs nothing, restarts nothing, writes no plan and prints no plan id (installed versions, `run/plans/` and file-tree snapshot unchanged) |
| 5j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpgradeEmitsCellOpV1' -count=1 -v` | `--- PASS` — `upgrade --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 5 | check | `statusgen --root . --lint` | exit 0 |
| 5a | check | `statusgen --root . --consumers --brief desk-containers/23` | exit 0 — the `consumers:` routing holds against the branch diff |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./cmd/cellctl/ -run '^TestUpgradeDeskInstances$' > "$o" 2>&1 && grep -F -- '--- PASS: TestUpgradeDeskInstances ' "$o")` | Named test executes (zero matching tests is not a pass); Upgrade preserves paused instance and partial replacement history and refuses a shared-binary swap beneath a draining predecessor. Mutation: restart every role member or ignore the draining old binary makes the test fail. |
## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: installs are pinned and hash-verified, the previous release can be re-pinned and reinstalled, and no other cell's live desks are ever swapped under. The reviewer checks rows 2, 3 and 3a–3c.
