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
unblocks: ["desk-containers/18", "desk-containers/19"]
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
- **Machine seam (desk-containers/15):** `--json` emits the summary as the contract's `cell-op-result` document, with one entry per role. When stale roles need reconciling, `up --plan` writes the reconcile plan and prints its id (this works anywhere). `up --apply <plan-id>` on a TTY shows it, takes the typed confirm code, re-measures every record, consumes the plan and runs it. Off a TTY, reconcile is plan-only: `up --apply` is refused `not-interactive`. **`up` without `--apply` is the lifecycle primitive allowed off a TTY** (desk-containers/15, amendment 21): it only starts roles that are not running, and reports stale ones with the `up --plan` command to plan their reconcile; it writes no plan itself. It never reconciles. `down` is the matching stop primitive: `down` stops the cell: for each cadence-mode role it writes the role's STOP file, waits up to 20 s for the lease, and refuses if a pass is unfinished (`stopCadences`, `tools/desk/cmd/cellctl/cadence.go`); interactive-mode role sessions are not waited on but hard-killed with the cockpit session (`tools/desk/cmd/cellctl/down.go`), so their in-flight agent work dies, which is harsher than `drain --now`; off a TTY, `down` then `up` approximates a restart, which is not a security boundary while approvals are deferred. With role arguments, `down <cell> <role...>` is the per-role interim stop (desk-containers/18): it stops only the named roles' cadence and closes only their window after re-verifying the window's identity against the live cockpit (cell, session and cell-qualified `<cell>-<role>` name, plus the harness pid and start time), refuses on any mismatch or stale receipt, refuses unless each role is already paused or drained and re-measures idle per its mode (work-safety guards, not authorization), and so does not close a sibling's or another cell's window.

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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConverge' -count=1 -v` | pass exit=0 | sha256:37b5822bf96d | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 2 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConvergeNeverTouchesLiveRole' -count=1 -v` | pass exit=0 | sha256:4e74fd1ec106 | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 3 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConvergeSerialLaunch' -count=1 -v` | pass exit=0 | sha256:eb0c617509f0 | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 4 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestCLINonHelpParity\|TestCLICompatDoc' -count=1 -v` | pass exit=0 | sha256:7678bfa65962 | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 5 | `sh tools/cellctl/tests/parity.test.sh` | fail exit=2 | sha256:383d4ad18251 | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 6j | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpEmitsCellOpV1' -count=1 -v` | pass exit=0 | sha256:7db292077ec6 | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --lint` | pass exit=0 | sha256:7964c46b0dec | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 6a | `statusgen --root . --consumers --brief desk-containers/17` | fail exit=2 | sha256:c2f2282cb4af | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 6b | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpPlanApply' -count=1 -v` | pass exit=0 | sha256:c2b6f054e9c9 | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cellctl up <cell>` | could-not-run exit=- — unsubstituted placeholder(s) <cell> — the row cannot run as written | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3424 run=4aaed6b33a95f2080b2f2dbbecf11de152ae5274cf199baf891fac73db7aaa77 source=6de70c43819e91cc67e69de4ee0fabb31af3c239 brief=docs/streams/desk-containers/brief-17-up-converges-the-cell.md model=claude-opus-5-5 tier=strong

Verifier rows, run by hand at merged main 6de70c43819e (delivering change: ea4d3339c, medici-finance/assay#3377). Every row below was executed; outputs and scratch builds were kept outside the admitted home.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConverge' -count=1 -v` | `--- PASS`, fixtures cover all absent, two up, one stale, `--role` subset | exit 0, `ok .../tools/desk/cmd/cellctl 7.450s`. Subtests PASS: `all_absent`, `two_up,_one_stale,_two_absent`, `role_subset`, `a_launch_that_never_takes_its_lease_fails_its_row`, `a_cadenced_role_resting_until_its_next_pass_is_scheduled,_not_failed`, `dry_run_launches_nothing`; the filter also matched TestUpConvergeDryRunJSON, NeverTouchesLiveRole and SerialLaunch, all PASS, no FAIL or SKIP | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConvergeNeverTouchesLiveRole' -count=1 -v` | `--- PASS`, a live role is neither relaunched nor its window closed, row reads `already-up` | exit 0, `--- PASS: TestUpConvergeNeverTouchesLiveRole (1.05s)`. The test asserts every row is `already-up`, an empty launch timeline, no `new-session`/`new-window`/`kill-`/`respawn`/`send-keys` cockpit call, and every lease still held | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpConvergeSerialLaunch' -count=1 -v` | `--- PASS`; a launch-all-at-once mutant fails | exit 0, `--- PASS: TestUpConvergeSerialLaunch (3.19s)`. Mutation check in a scratch copy of the tree at this commit (the `gate(p.Role, launched)` wait in convergeRoles replaced by a no-op): exit 1, `--- FAIL: TestUpConvergeSerialLaunch`, `launches overlapped: [worker-desk started before the-desk was ready ... pr-review-desk started before intake-desk was ready]`. The mutant is killed | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestCLINonHelpParity\|TestCLICompatDoc' -count=1 -v` | `--- PASS`, changed `up` plans recorded with a compat-doc section | exit 0, `--- PASS: TestCLINonHelpParity (6.06s)`, `--- PASS: TestCLICompatDoc (0.00s)`. The compat doc carries the "up output" entry: absent-role dry-run lines byte for byte as before, plus a disposition line for an up or stale role. The golden JSON was left unchanged by the delivering PR, consistent with that entry | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `PARITY_ONLY=<kind>/claude/<cockpit>/up CELLCTL_A=<the shell oracle> CELLCTL_B=<cellctl built at this commit> bash tools/cellctl/tests/parity.test.sh`, once per claude `up` cell, plus `PARITY_ONLY=/up` at this commit and at base ea4d3339c^ | exit 0 for every claude cell; codex differences the same set as at base | exit 0, `parity: 1 cells, 0 divergent` for each of the 12 claude cells (k8s, house, container, scrubbed by tmux, herdr, orca). `PARITY_ONLY=/up` at head: `24 cells, 6 divergent`, divergent = k8s/codex and house/codex on tmux, herdr, orca; at base: the same 6 names, and the full output differs only in diff-header temp paths and timestamps. Spelling note: the row says `sh`; macOS `sh` (bash posix mode) stops at line 390, `syntax error near unexpected token '<'` (process substitution), exit 2, which is what the witness row above recorded. The script's own shebang is bash; run under GNU bash 5.3.20 | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6j | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpEmitsCellOpV1' -count=1 -v` | `--- PASS`, `up --json` validates against the cell-op-v1 schema, stdout carries nothing else | exit 0, `--- PASS: TestUpEmitsCellOpV1 (1.50s)`. The test decodes stdout as exactly one document, validates it with the schemacheck validator against schemas/cell-op-v1.json, decodes with unknown fields disallowed, and checks one entry per role | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `statusgen --root . --lint` | exit 0 | exit 0, `LINT: PASS` (statusgen v1.0.49; NOTICE lines only, none for desk-containers) | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6a | `statusgen --root . --consumers --brief desk-containers/17`, then at a scratch checkout of ea4d3339c with `--base 716a74574^` | exit 0, routing holds against the branch diff | As written on merged main: exit 2, `COULD-NOT-CHECK: assay:assay:desk-containers:17 is not in the diff against 6de70c43819e`. With `--base ea4d3339c^` at ea4d3339c: exit 2, same COULD-NOT-CHECK (the delivering squash commit does not touch the brief file). With `--base 716a74574^` (parent of the authoring commit) at ea4d3339c: exit 0, `summary: 4 corroborated, 0 disproved, 2 unchecked` (up.go, the shell oracle, the cli golden JSON and docs/cellctl.md corroborated; the two out-of-scope entries UNCHECKED, reasons stated). Caveat: in that span the oracle and the golden JSON were touched only by c893a16f2 (medici-finance/assay#3050), not by the delivering PR, so those two corroborations come from an unrelated commit; the delivering PR left both unchanged on purpose, since absent-role dry-run output is byte-identical (rows 4 and 5) | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6b | `go -C tools/desk test ./cmd/cellctl/ -run 'TestUpPlanApply' -count=1 -v` | `--- PASS`, `plan-stale` naming the role, schema-valid `--json` | exit 0, `--- PASS: TestUpPlanApply (1.60s)` | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | Disposable house cell `dc17` (five roles) in a scratch CELLS_ROOT, cellctl built at this commit, real tmux on a private TMUX_TMPDIR server, stub claude harness (sleeps), local bare origin, `env -i` with no host desk environment: `cellctl up dc17 --cockpit tmux --no-attach`; SIGKILL of verify-desk's supervisor (pid 93851) and harness (pid 93977) by the pids in its lease holder record; `cellctl up dc17 --plan`; `cellctl up dc17 --apply 921615645e12627a4a3c5a09` on a pty (expect), typing confirm code 2a491953 | second run's table: four `already-up`, one `reconciled+up`; exit 0 | First run: exit 0, table `the-desk up / worker-desk up / verify-desk up / intake-desk up / pr-review-desk up`. A repeat `up`: exit 0, five `already-up` with pids. After the kill, plain `up`: exit 1, `verify-desk failed stale (supervisor pid 93851 has exited); not reconciled by up: cellctl up dc17 --plan`, the other four `already-up`. `--plan`: exit 0, `plan 921615645e12627a4a3c5a09: up on dc17`, one action `reconcile-verify-desk`, requires state stale and class dead. `--apply` with stdin from /dev/null: exit 5, `refused (not-interactive)`. `--apply` on the pty: exit 0, `[reconcile] verify-desk: recovery recorded`, `[cell] dc17-cell already running — adding the windows of roles that are not up`, table `the-desk already-up pid 93482 / worker-desk already-up pid 93664 / verify-desk reconciled+up / intake-desk already-up pid 93990 / pr-review-desk already-up pid 94173`, `cellctl up dc17: ok`. The four live pids survived and their tmux window ids @0 @1 @3 @4 were unchanged; verify-desk got a new window @5 and a new holder pid 11251. Torn down with `cellctl down dc17` (exit 0) | 2026-10-18 | assay-verifier-app[bot] @ 6de70c43819e (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Risk-bearing value. Frontmatter risk is all `no` and the item is not irreversible, so the fail-safe trigger does not fire. Enumeration over the ea4d3339c diff (up.go, up_converge.go, cobra.go, cellop_cli.go) found only reversible operational knobs: `upDefaultLeaseWait = 2 * time.Minute` @ tools/desk/cmd/cellctl/up_converge.go:67, `upPoll = 100 * time.Millisecond` @ tools/desk/cmd/cellctl/up_converge.go:64, `launched := time.Now().UTC().Add(-time.Second)` (1 s heartbeat slack) @ tools/desk/cmd/cellctl/up_converge.go:260, `upStagger = 2 * time.Second` @ tools/desk/cmd/cellctl/up.go:670. All of them are wait or poll bounds, fixed by an edit and a rebuild, so they rank last and none needs a derivation. No top-ranked entry exists, so this pass records no RISK-VALUE verdict line. The act here is opening or reconciling local desk windows, which can be undone.

Note: the brief's facts list four row states. The implementation adds a fifth, `scheduled` (a cadenced role resting until its next pass; exit 0), documented in the up help text and docs/cellctl.md. This adds to the contract and does not contradict it.

**VERIFY: PASS** — 10 of 10 rows observed meeting their Expect. Row 5 needs bash (the row's `sh` spelling is a syntax error on macOS). Row 6a, run as written after merge, is COULD-NOT-CHECK and corroborates only over the authoring-to-delivery span. Both are instrument issues, not deliverable defects.


## Review
Gate: model. All four risk answers are no: `up` only adds windows and only clears state through a desk-containers/15 plan applied by interactive confirm. The reviewer checks rows 2, 3 and 7.
