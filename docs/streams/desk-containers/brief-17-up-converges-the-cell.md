---
brief: assay:assay:desk-containers:17
title: "`cellctl up <cell>` converges the cell — already-up roles are skipped, stale ones reconciled, launches serialized, one summary table"
why: >-
  An operator who types `up <cell>` means "make every desk in this cell be running". Today it
  means "try to open every window once": a role that is already running fails with "role
  already owned", a role whose last run ended badly fails with "unfinished cadence pass", and
  roles started a moment apart can fail on each other's launch lock. On 2026-10-14 one restart
  of a five-role cell needed eleven hand-run recovery commands. Making `up` converge (start
  what is missing, leave what is running, repair what it safely can, and report all of it in
  one table) turns that into one command.
wave: 8
depends: ["desk-containers/15"]
unblocks: ["desk-containers/19"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) — classifies this verb as an interim bridge and names its target owner"
  - "Driver request 2026-10-14: `up <cell>` is the intent to bring up ALL the desks; some already up is fine; failing ones should be resolved interactively where possible"
  - "Incident 2026-10-14 (house cell, five roles hand-closed): five per-role `desk: unfinished cadence pass` refusals, one `comms last shutdown failed`, then four of five roles failing `interactive launch: deskkit: advisory lock is held by another process` when `up` opened them together"
  - "#2880 — the reconcile preflight (inventory, dead/alive/could-not-check by pid + start time, one TTY prompt, marker-only orphan stop). In flight as its own PR; this brief CALLS it per role and does not re-implement it"
  - "#2709 — launch sweep-lock race; fix on main as 6d576a565 (#2714, a busy lock waits and names its holder), not yet in a release"
  - "#2881 — `up` on a house cell opened an idle placeholder pane (PR #2882)"
  - "tools/desk/cmd/cellctl/desk.go:361-366 — `cellcadence.AcquireRole` failure dies `desk: role already owned or cannot lock`; :380 `state.Running` dies `unfinished cadence pass`"
  - "tools/desk/cmd/cellctl/up.go — `up` composes one window command per role and hands them to the cockpit (tmux / herdr / orca) in one pass; no per-role result is collected"
  - "freshness-checked 2026-10-14 @ 0cb900584 (origin/main) — `up` has no already-running detection, no per-role outcome, no `--role` selector"
exec-tier: strong
exec-tier-why: (b) correctness spans the cockpit layer, the cadence lease and #2880's reconcile across three cockpits; (c) a launch-ordering or lease-probe error is a race that unit tests do not surface
domain: complicated
consumers:
  - "target owner — cell operation (cell lifecycle) for the verb; the runner lifecycle and ownership contract for each role's `start` transition (graph-execution R2/R3): out-of-scope (this verb is an interim bridge on today's tools, per docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
  - "tools/desk/cmd/cellctl/up.go: follow-up desk-containers/17 (this brief)"
  - "tools/cellctl/testdata/cellctl-shell-oracle.sh: follow-up desk-containers/17 (this brief; the oracle's `up` plan mirrors the new per-role ordering so parity holds)"
  - "tools/desk/cmd/cellctl/testdata/cli-nonhelp-golden.json: follow-up desk-containers/17 (this brief; dry-run `up` output gains the per-role disposition lines, recorded as deliberate diffs)"
  - "docs/cellctl.md: follow-up desk-containers/17 (this brief)"
  - "the console cockpit (renders `up --json` results and `up --plan` plans only; never applies): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 17 — `cellctl up <cell>` converges the cell

## Context
files:
- `tools/desk/cmd/cellctl/up.go` — per-role disposition, serialized launch, summary table, `--role`
- `tools/desk/cmd/cellctl/up_converge_test.go` (planned)
- `tools/cellctl/testdata/cellctl-shell-oracle.sh` — parity for the dry-run plan only
- `tools/desk/cmd/cellctl/testdata/cli-nonhelp-golden.json` — deliberate-diff entries for the changed `up` plans
- `docs/cellctl.md` — §`up` rewritten around "converge"
- `changelog/desk-containers-17-up-converges.md` (planned)

facts:
- **Interim bridge.** `up` stays the operator's cell-lifecycle verb in the target (cellctl is 'operator surface for cell lifecycle and configuration'). Its per-role launch is today's process supervision, which the architecture moves behind the shared runner contract. When that ships, `up` keeps its converge semantics and calls the lifecycle `start` transition instead of launching windows itself.
- A role is **up** when its cadence role lease (`run/cadence/<role>/owner.lock`, `cellcadence.AcquireRole`) is held by a live process. Probe it non-blockingly; a held lease is "already up", never an error. Re-check 2026-10-14: `cellctl cadence <cell> status` prints `running=true` per held role.
- A role is **stale** when its checkpoint says `running=true` (or comms `outcome != ok`) and no live holder exists. Stale roles are reconciled with #2880's classification and actions for THAT role, but **the confirmation goes through the desk-containers/15 contract**: `up` wraps the reconcile actions in a `cell-op-plan` and applies it only through the desk-containers/15 interactive apply. Applying is an interactive-terminal confirmation only (desk-containers/15): the verb shows the plan and the operator types its confirm code. That is a convenience confirm, not an authorization control. #2880's own one-prompt path stays as it is for #2880 until this brief lands. After that there are two reconcile entry points, `up` and `doctor` (desk-containers/21), both going through a desk-containers/15 plan; neither clears state outside one.
- Launches are **serialized**: open role N+1 only after role N's launch has passed its lease acquisition and scratch sweep (or failed). This holds even with #2709's wait-for-lock fix, because the summary must attribute each failure to one role.
- `--role <name>` (repeatable) limits convergence to the named roles. With no `--role`, `ROLES` from cell.env is the target set.
- The final table has one row per targeted role: `up` (launched now) / `already-up` (pid, since) / `reconciled+up` / `failed` (the first error line, verbatim). Exit 0 when every targeted role ends `up`, `already-up` or `reconciled+up`; otherwise exit 1.
- `DRY_RUN=1` prints each role's disposition from the same probes and launches nothing.
- **Machine seam (desk-containers/15):** `--json` emits the summary as the contract's `cell-op-result` document, with one entry per role. When stale roles need reconciling, `up --plan` writes the reconcile plan and prints its id (this works anywhere). `up --apply <plan-id>` on a TTY shows it, takes the typed confirm code, re-measures every record, consumes the plan and runs it. Off a TTY, reconcile is plan-only: `up --apply` is refused `not-interactive`. **`up` without `--apply` is the lifecycle primitive allowed off a TTY** (desk-containers/15, amendment 21): it only starts roles that are not running, and reports stale ones with the `up --plan` command to plan their reconcile; it writes no plan itself. It never reconciles. `down` is the matching stop primitive: `down` is an abrupt stop: it hard-kills the role sessions, which is harsher than `drain --now`, and in-flight agent work dies; off a TTY, `down` then `up` approximates a restart, which is not a security boundary while approvals are deferred.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- Do not re-implement #2880's classification or orphan stop. If its PR has not merged when you start, build against its public function names as stated on that PR and report NEEDS_CONTEXT if they change.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Before composing windows, classify every targeted role as up / stale / absent using the lease probe and checkpoint read above.
2. Wrap stale roles' reconcile (using #2880's classification and actions) in one desk-containers/15 plan for the whole cell, applied only through the interactive confirm.
3. Launch the absent and reconciled roles one at a time in the resolved cockpit. For an existing cockpit session (tmux session, herdr workspace), add only the missing windows. Never re-create or close a window that holds an `already-up` role.
4. Print the summary table; set the exit code as above.
5. Add `--role`; keep `--no-the-desk` / `--with-the-desk` as aliases over the same target set.
6. Update the docs, goldens (as deliberate diffs) and the oracle plan; write the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConverge' -count=1 -v` | `--- PASS` — fixtures cover: all absent; two already up (lease held by a live fixture process); one stale (checkpoint `running=true`, no holder); `--role` subset |
| 2 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConvergeNeverTouchesLiveRole' -count=1 -v` | `--- PASS` — with a live fixture holding one role's lease, `up` neither launches a second instance nor closes that role's window, and its table row reads `already-up` |
| 3 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConvergeSerialLaunch' -count=1 -v` | `--- PASS` — a recording cockpit stub shows role N+1's launch starting only after role N's lease step returned; a mutant that launches all roles at once fails this test |
| 4 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestCLINonHelpParity\|TestCLICompatDoc' -count=1 -v` | `--- PASS` — the changed `up` plans are recorded deliberate diffs with a compat-doc section |
| 5 | check | `sh tools/cellctl/tests/parity.test.sh` with `PARITY_ONLY` set to each `up` cell | exit 0 for every claude cell; any pre-existing codex-cell differences are the same set as at base |
| 6j | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpEmitsCellOpV1' -count=1 -v` | `--- PASS` — `up --json` output validates against `schemas/cell-op-v1.json` (planned) and stdout carries nothing else; this brief owns its verb's adoption of the desk-containers/15 contract |
| 6 | check | `statusgen --root . --lint` | exit 0 |
| 6a | check | `statusgen --root . --consumers --brief desk-containers/17` | exit 0 — the `consumers:` routing holds against the branch diff |
| 6b | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpPlanApply' -count=1 -v` | `--- PASS` — `up --apply <id>` on a plan whose stale role came alive between plan and apply refuses with `plan-stale`, naming the role; `--json` output validates against the brief-15 schema |
| 7 | gate:model +flow +dereference | On a disposable house cell with stub harnesses: `cellctl up <cell>`, kill one role's harness, run `cellctl up <cell> --plan`, then `cellctl up <cell> --apply <plan-id>` on a TTY and type the plan's confirm code | the second run's table shows four `already-up` and one `reconciled+up`; exit 0. Record both tables. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: `up` only adds windows and only clears state through a desk-containers/15 plan applied by interactive confirm. The reviewer checks rows 2, 3 and 7.
