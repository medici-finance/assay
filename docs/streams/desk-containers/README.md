---
stream: desk-containers
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: [63, 64, 1193]
board: generated
---

# desk-containers Stream

Package each assay desk (`intake-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`,
`the-desk`) as its own versioned container image built from one shared base image
(Go + Python + desk-tools + the assay plugin skills + a persistent volume), with a shell
script to fire any desk up interactively from a desktop, docker-compose + Kubernetes as
secondary launch targets, and a cross-platform (macOS + win32) tmux-or-equivalent
control layer to fire up and hold the whole desk fleet at once. Requested in
medici-finance/assay#63 (images, script, compose/k8s) and medici-finance/assay#64
(the control layer).

Hard constraint carried through every brief: **no credential — bot App PEM, token, or
model API key — ever appears in an image layer**; everything is injected at runtime via
mounted secrets and runtime env. See [spec.md](spec.md) for goals, non-goals, the
component list, build/run topology, and open questions.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [base image — toolchains, desk-tools, assay skills, persistent-volume layout](brief-01-base-image.md) | 1 | M | done | 2026-09-15 sonnet-5-verifier | 2026-09-16 assay-reviewer-app[bot] (approved PR #1177 @ a7eb237eb00bce036bd1f1bed9ca6091ea868e4d) |
| 02 | [runtime credential contract (PEM + model env) + image layer-secret scan](brief-02-secret-injection-contract.md) | 1 | M | implemented | — | — |
| 03 | [per-desk images (named by desk) + build matrix + publish wiring](brief-03-desk-images-build-matrix.md) | 2 | M | done | 2026-09-16 verify-desk (desk-containers/03 dispatched verifier; row 5 now PASS given assay#1025, rows 1-4/7 could-not-check offline-docker-lane, risk-value NAMED-NOT-DERIVED filed assay#1218) | 2026-09-17 assay-reviewer-app[bot] (approved PR #1220 @ c391ed99ccedc80dc426893c265dd8100c8f398f) |
| 04 | [interactive desktop launch script (desk-run.sh)](brief-04-launch-script.md) | 3 | M | todo | — | — |
| 05 | [docker-compose definition for the five desks](brief-05-docker-compose.md) | 3 | S | todo | — | — |
| 06 | [Kubernetes manifests for the five desks](brief-06-k8s-manifests.md) | 3 | M | implemented | — | — |
| 07 | [Optional Linux operator terminal layout over the shared desk attach path](brief-07-desk-control-layer.md) | 11 | M | todo | — | — |
| 08 | [A tick contract: one bounded pass when the harness says `--tick`, so a loop pod can finish](brief-08-tick-contract-for-desk-skills.md) | 3 | M | implemented | — | — |
| 09 | [cellctl: host-local harness cell — scrubbed per-cell environment, `smoke`, `status`, session lock, stricter `check`](brief-09-scrubbed-host-cell.md) | 0 | M | implemented | — | — |
| 10 | [cellctl in Go: `tools/desk/cmd/cellctl`, bash kept as the oracle until parity](brief-10-cellctl-go-port.md) | 1 | L | implemented | — | — |
| 11 | [retire the out-of-tree bridge: migrate `CELL_KIND=local` registrations, remove the shell shim, one `cellctl` on PATH](brief-11-retire-bridge.md) | 2 | S | todo | — | — |
| 12 | [Unattended mode for the cluster manifests — a Kustomize component that runs each desk as a tick-mode CronJob](brief-12-unattended-tick-mode-cronjob-component.md) | 4 | M | todo | — | — |
| 13 | [Argo CD install example — an adopter overlay plus an Application and AppProject that install the desks from a pinned release](brief-13-argo-cd-install-example.md) | 5 | M | todo | — | — |
| 14 | [Flux install example — a GitRepository and Kustomization that install the desks from a pinned release, with an adopter overlay](brief-14-flux-install-example.md) | 6 | M | todo | — | — |
| 15 | [The cell-operation contract — one versioned result and plan schema shared by the future Cell API and the CLI; apply only as an interactive-terminal confirmation, with approval-based apply deferred](brief-15-cell-op-machine-seam.md) | 7 | M | implemented | — | — |
| 16 | [Interim mechanism for the runner-lifecycle `pause` transition — cell-scoped stop flags `STOP.cell.<cell>[.<loop>]`, honoured by the Guard for any session carrying the cell marker](brief-16-cell-scoped-stop-flags.md) | 7 | M | todo | — | — |
| 17 | [`cellctl up <cell>` converges the cell — already-up roles are skipped, stale ones reconciled, launches serialized, one summary table](brief-17-up-converges-the-cell.md) | 8 | M | implemented | — | — |
| 18 | [`cellctl drain\|pause\|resume <cell> [role...]` — CLI adapter onto the runner-lifecycle drain/pause/resume transitions and the handoff record, implemented on today's stop flags and cadence as the interim mechanism](brief-18-drain-pause-resume-handoff.md) | 10 | M | todo | — | — |
| 19 | [`cellctl restart <cell> [role...]` — CLI adapter onto the runner-lifecycle restart sequence (drain → stop → start per role, rolling), so the pipeline never fully stops](brief-19-restart-rolling.md) | 11 | M | todo | — | — |
| 20 | [`cellctl status <cell>` as a cell overview — per-role liveness, model, heartbeat, claims, stop state, plus comms, cache, locks and tool version](brief-20-status-overview.md) | 10 | M | todo | — | — |
| 21 | [`cellctl doctor <cell>` — diagnose every known way a cell gets stuck, name the cause, and offer the fix as one confirmed plan](brief-21-doctor.md) | 13 | M | todo | — | — |
| 22 | [`cellctl logs <cell> [role] [--lines N]` — read a desk window's recent output from whichever cockpit holds it](brief-22-logs.md) | 11 | S | todo | — | — |
| 23 | [`cellctl upgrade <cell>` — install the pinned desk-tools release and plugin, verify them, then rolling-restart the cell onto them](brief-23-upgrade.md) | 12 | M | todo | — | — |
| 24 | [`cellctl gc <cell>` — one housekeeping verb, plan-first: cache, scratch, worktrees, stale claims, orphaned processes, expired plans](brief-24-gc.md) | 14 | M | todo | — | — |
| 25 | [`cellctl desk --force` — diagnose a failed desk boot, repair what it can, ask before each destructive step, `-y` answers yes](brief-25-desk-force-repair.md) | 2 | M | todo | — | — |
| 26 | [Local PostgreSQL in Docker for a multi-cell host — pinned image, one database and restricted role per cell, runtime credentials, backup and restore drill, and the `cell.env` keys a cell reads its DSN from](brief-26-local-docker-postgres.md) | 3 | M | todo | — | — |
| 27 | [PostgreSQL on Kubernetes with CloudNativePG — a Kustomize component with one Cluster, a database and role per cell, out-of-band Secrets, object-store backups, and the explicit local-to-CNPG migration procedure](brief-27-cnpg-kubernetes-postgres.md) | 4 | L | todo | — | — |
| 28 | [Package in-cluster cellctl and bind the interactive desk profile](brief-28-package-in-cluster-cellctl-and-bind-the-interactive-desk-profile.md) | 10 | M | todo | — | — |
| 29 | [Qualify the Kubernetes desk cell and operator recovery end to end](brief-29-qualify-the-kubernetes-desk-cell-and-operator-recovery-end-to-end.md) | 11 | M | todo | — | — |
| 30 | [Role instructions for Codex windows and dispatched agents](brief-30-role-instructions-for-codex-windows-and-dispatched-agents.md) | 0 | L | todo | — | — |
| 31 | [Automatic desk instances and desired population through the shared runtime](brief-31-automatic-desk-instances-and-desired-population-through-the-shared-runtime.md) | 9 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

`desk-containers/02` (credential contract — **human gate**: App-PEM custody design) →
`desk-containers/03` (per-desk images) → `desk-containers/04` (interactive launch
script — the primary aim of #63) → `desk-containers/07` (fleet control layer — #64,
which drives the script and so cannot precede it).

Wave 1 holds two independent heads: 01 (base image) and 02 (credential contract). Both
unblock 03, and 01 is mechanically the larger piece — but the **real pacing item is
02's human security review**: 03/04/05/06 all build against the mount-path/env-var
contract and the layer-scan check that 02 defines, and that review cannot be
model-cleared. The head was verified before authoring: the existing published
`desk-tools` image + root `Dockerfile` + `docker-publish.yml` are live at the base SHA,
so 01 has no hidden upstream blocker — the base image can reuse the published binaries
directly. Smallest unblocking move: land 01 and put 02 in front of its human reviewer
in the same wave.

Tempting-but-wrong first step: writing the five desk Dockerfiles first. They are thin
and easy, but without 01's base and 02's contract they would hard-code credential paths
that the human gate may reject — churn, not progress.

**`desk-containers/08` sits beside that path rather than on it, and it is the one brief
here that blocks a SHIPPED surface.** The five desk skills are standing loops by design:
every body says the loop is armed before the first sweep and kept ticking for the life of
the window, and none has a second mode. A scheduled pod invokes a role skill as a one-shot
call under an outer `timeout`, so a standing loop there has exactly one possible ending —
the kill — and the one-shot output mode prints nothing when that happens. Measured
2026-09-14: the review loop's first successful boot on the fixed image ran the full tick
deadline and was killed with an empty log; no review tick has ever completed. Brief 08 adds
the missing mode as a contract — one bounded pass, a fixed-grammar summary line, no durable
wake — stated once in a shared reference and derived into all five bodies. It depends on no
other brief in this stream (its deliverables are plugin content every image already bakes),
and the dependency runs the other way: the image cannot usefully pass a flag to a mode that
does not exist.

**GitOps install path (added 2026-09-23): `desk-containers/06` → `desk-containers/12` → `desk-containers/13` → `desk-containers/14`.**
06's Kustomize base is the one package; both Argo CD and Flux consume Kustomize natively, so there is no second packaging format.
12 adds the opt-in unattended component, CronJobs running brief 08's tick mode, which is what a GitOps install actually runs.
13 is the Argo CD example and ships `examples/gitops/check.sh` (planned). 14 is the Flux example and reuses that script and the same overlay shape, so it follows 13 to keep the two from drifting.
The real head is 06 (todo; its dependencies 02 and 03 are implemented and done). Nothing in this chain can be validated before 06's base exists.
Rows that dereference a pinned release tag stay could-not-check until a release ships `containers/k8s/`.

## Dependency waves

- **Wave 1** — `desk-containers/01`, `desk-containers/02` (independent;
  parallelizable; 02 carries the human gate).
- **Wave 2** — `desk-containers/03` (depends on 01 + 02).
- **Wave 3** — `desk-containers/04`, `desk-containers/05`, `desk-containers/06`
  (all depend on 02 + 03; parallelizable), and `desk-containers/08` (depends on
  NOTHING — it changes skill content, not image content, so it is parallelizable with
  every other brief in the stream; it is placed here because that is when the launch
  surfaces make it operationally load-bearing, not because anything blocks it).
- **Wave 4** — `desk-containers/07` (depends on 04; the tmux/equivalent fleet-control
  layer over the launch script).
- **Wave 4** — `desk-containers/12` (depends on 06 + 08; the unattended CronJob component).
- **Wave 5** — `desk-containers/13` (depends on 06 + 12; the Argo CD install example).
- **Wave 6** — `desk-containers/14` (depends on 06 + 12 + 13; the Flux install example).

Path: `02 → 03 → 04 → 07`, with `01 → 03` joining at wave 2 and `05`/`06` fanning out
beside `04`. `08` hangs off no edge at all.

### The PostgreSQL path — `26 → 27` (added 2026-10-14)

The non-CI target keeps each cell's operational state in PostgreSQL, run first in local
Docker and then on CloudNativePG (`docs/proposals/non-ci-architecture/14-storage-forge-graph.md`).
These two briefs ship the server side only. The consumer, graph-execution/19, owns the store's
schema, migrations and reconstruction, and its own tests use a separate disposable server, so
neither brief blocks it.

- **Wave 3** — `desk-containers/26`: the local server, its own compose project beside brief
  05's planned desk compose file, one database and restricted role per cell, dump backups
  with a restore drill, and the `cell.env` keys a cell reads its database from. It depends
  on nothing in this stream.
- **Wave 4** — `desk-containers/27` (depends on `desk-containers/06` and `desk-containers/26`): <!-- graph: encoded desk-containers/26→desk-containers/27 -->
  the CloudNativePG component beside 06's base, object-store backups, and the explicit
  local-to-CNPG migration procedure.

### The `cellctl` chain — `09 → 10 → 11` (#1193)

A second, independent chain, numbered from wave 0 because it shares no edge with the image
path above: it lives in the host launcher (the Go program `tools/desk/cmd/cellctl`, its shell
test oracle `tools/cellctl/testdata/cellctl-shell-oracle.sh`, and `docs/cellctl.md`), not in
`containers/`. #1193 records why: a host-local cell that must NOT inherit the operator's
shell had no kind to register under, so an out-of-tree bridge grew beside the launcher, and
the launcher itself already runs as three divergent shell copies on one machine.

- **Wave 0** — `desk-containers/09`: a `scrubbed` cell kind (the harness runs on the host
  inside an environment `cellctl` composes from nothing), plus `smoke`, `status`, an
  exclusive session lock and a stricter `check`. Its dry-run **plan grammar** is the seam
  brief 10 diffs against, so it is the head.
- **Wave 1** — `desk-containers/10`: the Go port, `tools/desk/cmd/cellctl`, on `deskkit`;
  the bash stays in the tree as the **oracle** and a parity harness diffs the two
  implementations' dry-run plans across every kind × harness × cockpit. **Human gate**: the
  port is ruled on #1193; the cutover of the tarball's `cellctl` entry is what the human
  signs.
- **Wave 2** — `desk-containers/11`: `CELL_KIND=local` (the bridge's registration) refused
  with the migration line to run; one-command migration into the brief-09 kind; docs on why
  there is exactly one `cellctl` on PATH — the pin, never a shell shim.

Path: `09 → 10 → 11`. The head was checked before authoring: `load_cell` on `main` refuses
every kind but `k8s|house|container` before any verb runs, so nothing in 10 or 11 can start
until 09 gives the launcher the kind the bridge was standing in for; 10 is the pacing item
(effort L, human-signed cutover), and 11 is deliberately small so the bridge's last trace
goes the release after the port lands, not with it. Native-Windows `cellctl` is a named
consequence of 10 for the `windows-port` stream, delivered there, not here.

### The cell-operator chain — `15/16 → 17/20 → 18 → 19/22 → 23 → 21 → 24` (2026-10-14)

A third chain, also in the host launcher, and independent of both chains above. It makes
`cellctl` usable at the level an operator thinks in, the CELL, rather than one desk at a time.
The trigger was a routine restart on 2026-10-14. Five hand-closed desk windows needed eleven
per-desk recovery commands: five `unfinished cadence pass` refusals, a failed comms
checkpoint, ten orphaned background loops, a 70.5 GB go cache pinned by dead sessions'
records, and four of five desks failing at launch on a lock race (#2709) whose fix had merged
but not shipped. #2879 (cache), #2880 (`up` reconcile preflight) and #2881 (idle deskd pane)
are the issue-sized pieces already in flight. This chain is the verb surface around them.

**Aligned with the future architecture.** Amendment
[21-cell-operator-surface.md](../../proposals/non-ci-architecture/21-cell-operator-surface.md)
classifies every verb here:

- **Interim bridges on today's tools, each naming its target owner:** `up`, `status`,
  `doctor`, `logs`, `gc`, `upgrade`.
- **Adapters onto target-owned capabilities:** drain, pause, resume, restart and the handoff
  record are runner-lifecycle transitions (graph-execution), and today's stop-flag and cadence
  mechanism is labelled interim. The shared result and plan contract (15) is consumed by both
  the CLI and the future Cell API; approval-based apply is deferred until R7/R3 meet the five
  approval properties in amendment 21, and apply is an interactive convenience confirm meanwhile.

Amendment 21 is proposed in medici-finance/assay#2890; ratified when that PR is merged by the maintainer.

- **Wave 7** — `desk-containers/15` (the shared cell-op contract: result and plan, shipping
  now for read, plan, status and `--json`. Apply is an interactive-terminal confirm only,
  honestly described as a convenience, not an authorization control. Approvals are deferred.
  **Human gate**: the honest SPOF and the deferral) and `desk-containers/16` (the interim
  mechanism for the lifecycle `pause` transition, a cell-scoped stop flag; **human gate**,
  because it changes the tool Guard). They are independent, and both are heads.
- **Wave 8** — `desk-containers/17` (`up` converges; reconcile through the contract) and
  `desk-containers/20` (`status` and the shared `cellop.Inventory`; reads 16's stop state).
- **Wave 9** — `desk-containers/18` (drain/pause/resume adapters onto the lifecycle transitions,
  the handoff record, and the per-role interim stop `down <cell> <role...>`. The stop closes a
  window only after re-verifying its live identity against the receipt, and uses a per-mode
  idle signal. `resume` lifts only its own stops. Needs 16 and 17, because it records the
  per-role surface receipts at launch in 17's `up`).
- **Wave 10** — `desk-containers/19` (`restart`, the lifecycle drain → stop → start sequence per
  role; `--now` is one batch plan over per-role plans, consumed before its first role) and
  `desk-containers/22` (`logs`, through 18's identity-bound receipt).
- **Wave 11** — `desk-containers/23` (`upgrade`: pins from a ref fetched from the forge in this
  run, sha256 plus signature/attestation where published, pinned plugin, restore on failure,
  then 19; refuses under another cell's live or unknown desks).
- **Wave 12** — `desk-containers/21` (`doctor`, over 20's inventory; its fixes point at 18's
  `resume` and 23's `upgrade`).
- **Wave 13** — `desk-containers/24` (`gc`, reusing 21's classification).

Critical path: `15 → 17 → 18 → 19 → 23 → 21 → 24` (16 joins at 18 and 20). The head was checked:
nothing in the chain can expose a result or a confirmation to a client before 15 fixes their
shape, and 18 cannot aim a pause before 16, because today's `STOP` flags live in the operator's
config home, which every house cell shares through a symlink. #2880 (its PR is #2884, still a
draft) is a dependency of 16 (its cell marker), 17, 21 and 24 as an issue, not a brief: if its
PR changes its marker or function names, those briefs report NEEDS_CONTEXT rather than
re-implement it. #2881's fix (#2882) has merged.

Numbering: briefs 15–24 keep their numbers. Other open drafts that propose desk-containers
15–17 renumber on their side.

**`send` is not a brief here.** Status for external parties is an authenticated Cell API
query, and operator talk-to-desk is a scoped Cell API request (R7, owned by the new `cell-api` stream, parked at creation).
The requirements (sender classes from authenticated identity, a deterministic status
projection, contained-reader free text with an answer-only lock and outbound scrub, mTLS plus
signed tokens pending ratification, rate limits, audit) are recorded in amendment 21. No comms
listener is added for external callers: comms stays desk-to-desk plus the contained reader.

Tempting-but-wrong first step: building `restart` straight on `down`/`up`. Without 16's
aimed stop and 18's wait for in-flight work, a restart is today's hand-close with extra
steps, and it leaves the same unfinished checkpoints behind.

### Role instructions beyond Claude windows — `30` (added 2026-10-24)

A standalone brief in the host launcher, outside every chain above. `CELL_ROLE_CONTEXT`
(issue #2438) gives each Claude Code role window its own instruction file. It refuses a Codex
role, and it never reaches the agents a window dispatches. `desk-containers/30` closes both
gaps: an argv-only Codex binding for `instructions`, and a `dispatch_instructions` key that
deskdispatch quotes verbatim, digest-checked, into every prompt of the named kit. It depends
on nothing here; a project that moves role rules out of its shared instruction file needs it
first.

- **Wave 0** — `desk-containers/30` (no dependencies; exec-tier strong). The Codex half
  probes the installed Codex at pickup; the dispatch half needs no probe.

The head was checked: on `main` the role-context binding exists for Claude Code only, and
deskdispatch reads no role-context value, so nothing else has to land first.
Harness facts for the Codex half come from the `harness-portability` stream's capability
survey; that stream's resident-rules fragment is not reused, because it writes into a
session-writable worktree file.

## Shared conventions

- All container assets live under `containers/`; docs land in `docs/docker.md`
  (extended) — briefs name their exact paths.
- Every image-producing brief carries the explicit DoD line: *no secret in any image
  layer* (with an executable scan row), and every launch-surface brief injects
  credentials only per the brief-02 contract.
- Feature branch + draft PR per brief; never commit `STATUS.md` on a branch; never
  self-flip a leak-sweep.
- Cell-operator verbs (briefs 15–24) share one contract, `cell-op-v1` (brief 15): the CLI's
  `--json` emits its result document, and `--plan` writes its plan, anywhere. Applying a plan
  is an interactive-terminal confirm only (typed confirm code, then re-measure, then atomic
  consume before any effect). It is a convenience, not an authorization control.
  Approval-based apply is deferred (amendment 21). No verb defines a blanket `--yes`.

## Current Kubernetes-first path — 20 October 2026

Read [amendment 28](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md).
It supersedes the historical desktop-first critical path above for this cohort. One desk
per interactive Pod/PVC; cellctl runs in a separate Linux operator Pod. Native Windows,
desktop Docker, CronJobs, GitOps examples and a workflow controller are not required.

- Packaging head: /06, with existing /02 credential acceptance still owed.
- Managed runtime: graph-execution/26 + /22 -> /27, plus agent-execution/01–03 -> new /09.
- Operator: /06 + /15 + agent-execution/09 -> new /28.
- Qualification: /28 + /27 (CNPG) + /18 (lifecycle) + /20 (inventory) -> new /29.
- Optional Linux layout: /28 -> revised /07; it is not a qualification gate.

The new branch has waves /09=9, /28=10, /29=11; /07=11. These are dependency depths,
not promises that external admission/security prerequisites have passed. All reusable
specs, briefs, code, profiles and generic migration/qualification live in Assay. Preserve
historical facts and old gates; the managed path cannot claim the legacy PEM profile as
candidate isolation. No cluster was contacted or profile activated by this amendment.

## Desk-instance amendment — 25 October 2026

The [requested instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) amends unfinished owner briefs without changing lifecycle status or completed Evidence. Shared R2/R3 instance ownership/capacity → standing clients (graph-execution/27) → desired-population adapter (desk-containers/31) → lifecycle/Kubernetes consumers. API contract work can proceed against the declared versioned shape; runtime activation still requires its actual dependencies and independent qualification. Paused instances and draining predecessors remain part of pickup/retirement inventory.

The instance-specific critical path is graph-execution/16 → /26 → /22 and /27 → desk-containers/31 (wave 9) → /18 (10) → /19 (11) → /23 (12). Status /20 is wave 10; logs /22 is 11, doctor /21 is 13 and GC /24 is 14. Managed rendering /28 remains wave 10 and independent journey /29 remains 11. These are delivery prerequisites, not activation evidence. The actual first blocker remains the existing shared runtime ownership/supervisor qualification; merely lifting cellctl's role lease is not an unblocking implementation.
