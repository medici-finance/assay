---
brief: assay:assay:desk-containers:21
title: "`cellctl doctor <cell>` — diagnose every known way a cell gets stuck, name the cause, and offer the fix as one confirmed plan"
why: >-
  Each refusal on 2026-10-14 was correct and each named only itself: a busy lock without its
  holder, an unfinished checkpoint without saying whether its process lived, a full cache
  without saying which records held it, and an installed binary missing a fix already on main.
  Working out the cause took a process listing and the source code every time. `doctor` runs
  every known check at once, names the cause in operator terms, and offers each safe fix in
  one plan the operator confirms at the terminal.
wave: 11
depends: ["desk-containers/15", "desk-containers/18", "desk-containers/20", "desk-containers/23"]
unblocks: ["desk-containers/24"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) — classifies this verb as an interim bridge and names its target owner"
  - "Incident 2026-10-14: `interactive launch: deskkit: advisory lock is held by another process` with no holder named; five `unfinished cadence pass` refusals; `comms last shutdown failed`; go cache at 70.5 GB against a 64 GiB budget pinned by ten consumer records, five from dead sessions; an installed v1.0.41 missing the #2709 fix (6d576a565, merged two minutes after the tag)"
  - "#2880 — reconcile classification (dead/alive/could-not-check by pid + start time) and marker-only orphan stop; doctor calls it, never re-implements it"
  - "#2879 — stale go-cache consumer records; doctor reports them, and once #2879 lands per-record recovery doctor offers it"
  - "desk-containers/20 — `cellop.Inventory`, the single probe set doctor reads"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — no doctor verb; `cellctl check` audits launch preconditions, not runtime state"
exec-tier: strong
exec-tier-why: (b) every finding crosses a different store (cadence, comms, cache, locks, pins, tokens) and the fixes reuse three other verbs; (c) offering a fix on a misclassified record is the error tests built on clean fixtures miss
domain: complicated
consumers:
  - "target owner — projection and interaction (explanations, hold reasons, recovery actions) and cell operation (recovery): out-of-scope (this verb is an interim bridge on today's tools, per docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
  - "tools/desk/cmd/cellctl (new doctor verb): follow-up desk-containers/21 (this brief)"
  - "docs/cellctl.md: follow-up desk-containers/21 (this brief)"
  - "the console cockpit (a doctor panel rendering findings and showing fix plans (read/plan-only)): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 21 — `cellctl doctor <cell>`

## Context
files:
- `tools/desk/cmd/cellctl/doctor.go` (planned), `doctor_test.go` (planned)
- `tools/desk/cmd/cellctl/cobra.go` — register `doctor`
- `docs/cellctl.md` — §doctor with the finding catalogue
- `changelog/desk-containers-21-doctor.md` (planned)

facts:
- **Interim bridge.** doctor is an interim bridge over today's tools. In the target, its findings are the projection domain's 'hold reason and recovery action', exposed through the Cell API, and its fixes are owner actions (runtime coordination for reconcile, cell operation for recovery). It keeps no state that would have to be migrated.
- Findings, each with a stable code, the evidence, and either a fix action or "no automatic fix" plus the exact command:
  - `lock-held`: holder pid, command, start time, and whether the holder is this cell's live supervisor (expected) or a stale own-cell process. The fix is the existing opt-in stale-holder end, only for proven-stale own-cell holders.
  - `cadence-unfinished` / `comms-unfinished`: the fix is #2880's reconcile for that record.
  - `orphan-process`: marker-carrying, re-parented, harness gone (#2880's definition). The fix is #2880's stop.
  - `cache-over-budget` / `cache-stale-record`: bytes, budget, and records with their liveness. The fix is `cache clean`; per-record recover only once #2879 ships it, otherwise "no automatic fix" naming #2879.
  - `binary-behind-pin` / `binary-behind-known-fix`: installed `cellctl version` older than the cell's pin, or a known-fix list naming a later release. The fix is "no automatic fix" naming `upgrade` (desk-containers/23).
  - `token-expired`: a role's minted token past expiry. The fix is "no automatic fix"; the token re-mints on the desk's next verb, so this is informational.
  - `stop-flag-held`: a machine-wide, cell or role stop is set, which is why loops are idle. The fix is "no automatic fix" naming `resume`.
- **Bare `doctor` is read-only, on a TTY or off one.** It prints the findings and what each fix would do, writes no plan and changes nothing. The fix plan comes only from `doctor --plan`, which persists it under the cell's `run/plans/` (desk-containers/15) and prints its id. Fixes apply **only** through `doctor --apply <plan-id>`: on a TTY it shows the plan, the operator types its confirm code (a convenience confirm, not an authorization control), and every fix's precondition evidence is re-measured. Off a TTY, `doctor --apply` is refused `not-interactive` (desk-containers/15 row 8). `doctor --plan` works anywhere.
- `doctor` never kills a process or clears a record that the inventory classes `alive` or could-not-check.
- Exit 0 when there are no findings, 1 when findings remain after the run, 6 when a probe could not check.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- Every fix calls an existing verb or #2880's functions; doctor adds no new mutation of its own.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement the finding catalogue over `cellop.Inventory`.
2. Implement bare `doctor` as read-only, and the `doctor --apply` interactive path.
3. Docs (catalogue with codes and fixes); changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDoctorFindings' -count=1 -v` | `--- PASS` — one fixture per finding code produces that code and its evidence |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDoctorNeverTouchesAlive' -count=1 -v` | `--- PASS` — an alive or could-not-check record is reported but absent from the fix plan; a mutant that plans a fix for it fails |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDoctorApplyRestale' -count=1 -v` | `--- PASS` — a record that comes alive between `--plan` and `--apply` makes the apply refuse `plan-stale` |
| 4b | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestBareDoctorChangesNothing' -count=1 -v` | `--- PASS` — bare `doctor` on a pseudo-terminal, with every finding fixable, changes nothing and writes no plan (file-tree, `run/plans/` and process-table snapshots unchanged), and prints no plan id; `doctor --apply` off a TTY is refused `not-interactive` |
| 4 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDoctorNonTTYReadOnly' -count=1 -v` | `--- PASS` — off a TTY, plain `doctor` writes nothing (file-tree hash unchanged) |
| 5 | gate:model +flow +dereference | Reproduce the 2026-10-14 incident on a disposable house cell: start five stub desks, kill their windows, leave a stub background loop carrying the marker, and hold a cell lock with a stale own-cell process; then `cellctl doctor <cell>` (findings only), then `cellctl doctor <cell> --plan` for the plan id, then `cellctl doctor <cell> --apply <plan-id>` on a TTY, typing the plan's confirm code | findings name each injected fault; after the confirm, a second `doctor` reports none and `cellctl up <cell>` starts all roles with no prompt |
| 6j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestDoctorEmitsCellOpV1' -count=1 -v` | `--- PASS` — `doctor --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 6 | check | `statusgen --root . --lint` | exit 0 |
| 6a | check | `statusgen --root . --consumers --brief desk-containers/21` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: every fix is an existing, already-confirmed mutation, re-verified at apply. The reviewer checks rows 2, 3 and 5.
