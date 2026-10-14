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
| 06 | [Kubernetes manifests for the five desks](brief-06-k8s-manifests.md) | 3 | M | todo | — | — |
| 07 | [multi-desk control layer — tmux/tmuxinator evaluation + cross-platform config](brief-07-desk-control-layer.md) | 4 | M | todo | — | — |
| 08 | [A tick contract: one bounded pass when the harness says `--tick`, so a loop pod can finish](brief-08-tick-contract-for-desk-skills.md) | 3 | M | implemented | — | — |
| 09 | [cellctl: host-local harness cell — scrubbed per-cell environment, `smoke`, `status`, session lock, stricter `check`](brief-09-scrubbed-host-cell.md) | 0 | M | implemented | — | — |
| 10 | [cellctl in Go: `tools/desk/cmd/cellctl`, bash kept as the oracle until parity](brief-10-cellctl-go-port.md) | 1 | L | implemented | — | — |
| 11 | [retire the out-of-tree bridge: migrate `CELL_KIND=local` registrations, remove the shell shim, one `cellctl` on PATH](brief-11-retire-bridge.md) | 2 | S | todo | — | — |
| 12 | [Unattended mode for the cluster manifests — a Kustomize component that runs each desk as a tick-mode CronJob](brief-12-unattended-tick-mode-cronjob-component.md) | 4 | M | todo | — | — |
| 13 | [Argo CD install example — an adopter overlay plus an Application and AppProject that install the desks from a pinned release](brief-13-argo-cd-install-example.md) | 5 | M | todo | — | — |
| 14 | [Flux install example — a GitRepository and Kustomization that install the desks from a pinned release, with an adopter overlay](brief-14-flux-install-example.md) | 6 | M | todo | — | — |
| 15 | [The cell-operation machine seam — one `--json` result contract and a plan-id → apply confirmation that a program can drive and a human still clicks](brief-15-cell-op-machine-seam.md) | 7 | M | todo | — | — |
| 16 | [Cell-scoped stop flags — `STOP.cell.<cell>` and `STOP.cell.<cell>.<loop>`, honoured by the Guard for any session carrying the cell marker](brief-16-cell-scoped-stop-flags.md) | 7 | M | todo | — | — |
| 17 | [`cellctl up <cell>` converges the cell — already-up roles are skipped, stale ones reconciled, launches serialized, one summary table](brief-17-up-converges-the-cell.md) | 8 | M | todo | — | — |
| 18 | [`cellctl drain\|pause\|resume <cell> [role...]` — stop new work, let in-flight work finish, write a handoff note, then shut down](brief-18-drain-pause-resume-handoff.md) | 8 | M | todo | — | — |
| 19 | [`cellctl restart <cell> [role...]` — rolling drain → down → up, one role at a time, so the pipeline never fully stops](brief-19-restart-rolling.md) | 9 | S | todo | — | — |
| 20 | [`cellctl status <cell>` as a cell overview — per-role liveness, model, heartbeat, claims, stop state, plus comms, cache, locks and tool version](brief-20-status-overview.md) | 8 | M | todo | — | — |
| 21 | [`cellctl doctor <cell>` — diagnose every known way a cell gets stuck, name the cause, and offer the fix as one confirmed plan](brief-21-doctor.md) | 9 | M | todo | — | — |
| 22 | [`cellctl logs <cell> [role] [--lines N]` — read a desk window's recent output from whichever cockpit holds it](brief-22-logs.md) | 8 | S | todo | — | — |
| 23 | [`cellctl upgrade <cell>` — install the pinned desk-tools release and plugin, verify them, then rolling-restart the cell onto them](brief-23-upgrade.md) | 10 | M | todo | — | — |
| 24 | [`cellctl gc <cell>` — one housekeeping verb, plan-first: cache, scratch, worktrees, stale claims, orphaned processes, expired plans](brief-24-gc.md) | 10 | M | todo | — | — |
| 25 | [`cellctl send <cell> <role\|all>` for the local operator — an authenticated `operator` sender class on comms, and Tier A `--ask status` answered deterministically with no model reading the request](brief-25-send-operator-and-status-tier.md) | 9 | M | todo | — | — |
| 26 | [External ingress for `send` — an opt-in authenticated listener, the untrusted `external` sender class, per-sender rate limits, a request/reply audit log, and Tier A status with an external-safe projection](brief-26-send-external-ingress.md) | 10 | M | todo | — | — |
| 27 | [Tier B for `send` — free-text questions through the contained reader as labelled untrusted data, answer-only desk handling enforced at the tool layer, and every reply through the outbound filter](brief-27-send-free-text-tier-b.md) | 11 | M | todo | — | — |
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

### The cell-operator chain — `15/16 → 17/18/20/22 → 19/21/25 → 23/24/26 → 27` (2026-10-14)

A third chain, also in the host launcher, and independent of both chains above. It makes
`cellctl` usable at the level an operator thinks in, the CELL, rather than one desk at a time.
The trigger was a routine restart on 2026-10-14. Five hand-closed desk windows needed eleven
per-desk recovery commands: five `unfinished cadence pass` refusals, a failed comms
checkpoint, ten orphaned background loops, a 70.5 GB go cache pinned by dead sessions'
records, and four of five desks failing at launch on a lock race (#2709) whose fix had merged
but not shipped. #2879 (cache), #2880 (`up` reconcile preflight) and #2881 (idle deskd pane)
are the issue-sized pieces already in flight. This chain is the verb surface around them.

- **Wave 7** — `desk-containers/15` (the machine seam: one `--json` result contract and the
  plan-id → apply confirmation every later verb adopts, so the console cockpit can drive them
  and a human still clicks) and `desk-containers/16` (cell-scoped stop flags, so a pause can be
  aimed at one cell; **human gate**, because it changes the tool Guard). They are independent, and both are heads.
- **Wave 8** — `desk-containers/17` (`up` converges; uses #2880's reconcile),
  `desk-containers/18` (`drain`/`pause`/`resume` and the handoff note; needs 16),
  `desk-containers/20` (`status` overview and the shared `cellop.Inventory`),
  `desk-containers/22` (`logs`).
- **Wave 9** — `desk-containers/19` (rolling `restart` = 18 then 17, per role) and
  `desk-containers/21` (`doctor`, over 20's inventory).
- **Wave 10** — `desk-containers/23` (`upgrade` = pinned install then 19) and
  `desk-containers/24` (`gc`, reusing 21's classification).

Critical path: `15 → 18 → 19 → 23` (16 joins at 18). The head was checked: nothing in the
chain can expose a result or a confirmation to the console cockpit before 15 fixes their
shape, and 18 cannot aim a pause before 16, because today's `STOP` flags live in the operator's
config home, which every house cell shares through a symlink. #2880 is a dependency of 17 and
21 as an issue, not a brief: if its PR changes its function names, 17 and 21 report
NEEDS_CONTEXT rather than re-implement it.

**`send` — `20 → 25 → 26 → 27` (driver ruling 2026-10-14).** Messaging a desk is built on
the existing comms service (signed messages, local-only socket, a pinned contained reader
already doing inbound routing and outbound filtering, quarantine on a failed consultation),
with sender classes decided by authenticated identity only, never by message content.

- **Wave 9** — `desk-containers/25`: the local `operator` class and Tier A `--ask status`,
  answered by code from 20's status document with no model reading the request.
  **Human gate**: the first non-desk sender allowed to instruct a desk.
- **Wave 10** — `desk-containers/26`: the opt-in external listener (authentication is the
  human decision: mutual TLS reusing the gateway's existing mTLS front, or signed tokens),
  the untrusted `external` class limited to Tier A, an allow-listed status projection, an
  outbound scrub, per-caller rate limits and a request/reply audit log. **Human gate**,
  sensitive-data.
- **Wave 11** — `desk-containers/27`: Tier B free text. A contained reader turns untrusted text
  into a typed, capped, labelled summary; a Guard-enforced answer-only lock means a desk
  answering an external message cannot act, whatever the model decides; every reply passes the
  reader's outbound consultation and 26's scrub; anything that fails is quarantined. **Human
  gate**, sensitive-data.

Each layer has its own negative-path Verify row with the layer above it bypassed: an unsigned
or unknown sender refused at ingress, an injected instruction refused by the Guard with the
reader bypassed, a withheld token blocked by the scrub when the desk emits it, and the rate
limit tripping.

Tempting-but-wrong first step: building `restart` straight on `down`/`up`. Without 16's
aimed stop and 18's wait for in-flight work, a restart is today's hand-close with extra
steps, and it leaves the same unfinished checkpoints behind.

## Shared conventions

- All container assets live under `containers/`; docs land in `docs/docker.md`
  (extended) — briefs name their exact paths.
- Every image-producing brief carries the explicit DoD line: *no secret in any image
  layer* (with an executable scan row), and every launch-surface brief injects
  credentials only per the brief-02 contract.
- Feature branch + draft PR per brief; never commit `STATUS.md` on a branch; never
  self-flip a leak-sweep.
- Cell-operator verbs (briefs 15–27) share one output and confirmation contract, brief 15:
  `--json` emits a `cell-op-result-v1` document, and every confirmation has a TTY prompt plus
  a non-TTY `--plan` → `--apply <plan-id>` form, re-verified at apply. No verb defines a
  blanket `--yes`.
