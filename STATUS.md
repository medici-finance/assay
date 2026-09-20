<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.
     Regenerate: run statusgen against this repo root — installed release binary `statusgen --root .`, or `go run . --root ..` from statusgen/ inside the assay repo. Channels + pin spec: docs/distribution.md, section: The .assay-versions pin file -->

# Project Status

_Repo: `medici-finance/assay` — this board covers the streams in this repo only; sibling repos have their own._

## Roll-up

### Platform

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [apps-installer](docs/streams/apps-installer/README.md) | P1 | active | 2/8 | 2026-09-20 |  |
| [composability](docs/streams/composability/README.md) | P2 | active | 2/6 | 2026-09-20 |  |
| [contributor-trust](docs/streams/contributor-trust/README.md) | P1 | active | 0/9 | 2026-09-20 |  |
| [derived-board](docs/streams/derived-board/README.md) | P1 | active | 3/7 | 2026-09-20 |  |
| [desk-containers](docs/streams/desk-containers/README.md) | P2 | active | 2/11 | 2026-09-20 |  |
| [desk-supervision](docs/streams/desk-supervision/README.md) | P2 | active | 6/22 | 2026-09-20 |  |
| [desk-tools](docs/streams/desk-tools/README.md) | P2 | active | 14/28 | 2026-09-20 |  |
| [desktools-go-git](docs/streams/desktools-go-git/README.md) | P2 | active | 4/8 | 2026-09-20 |  |
| [forge-gitlab](docs/streams/forge-gitlab/README.md) | P2 | active | 12/17 | 2026-09-20 |  |
| [forge-neutral](docs/streams/forge-neutral/README.md) | P1 | active | 5/30 | 2026-09-20 |  |
| [graph-execution](docs/streams/graph-execution/README.md) | P1 | active | 1/8 | 2026-09-20 |  |
| [harness-portability](docs/streams/harness-portability/README.md) | P2 | active | 4/14 | 2026-09-20 |  |
| [iso-9001](docs/streams/iso-9001/README.md) | P2 | active | 2/6 | 2026-09-20 |  |
| [quality](docs/streams/quality/README.md) | P2 | active | 16/16 | 2026-09-20 |  |
| [statusgen](docs/streams/statusgen/README.md) | P2 | active | 10/13 | 2026-09-20 |  |
| [windows-port](docs/streams/windows-port/README.md) | P2 | active | 5/10 | 2026-09-20 |  |

## Parked

_Shelved streams: excluded from Next-up and every dispatch view, briefs kept. Re-activate by flipping the README `status:` back to `active` (subject to the active-stream cap)._

| Stream | Priority | Briefs | Last touched |
|---|---|---|---|
| [auto-triage](docs/streams/auto-triage/README.md) | P2 | 0/4 | 2026-09-20 |
| [desktools-v2](docs/streams/desktools-v2/README.md) | P2 | 0/10 | 2026-09-20 |
| [fresh-views](docs/streams/fresh-views/README.md) | P2 | 0/6 | 2026-09-20 |
| [measured-status](docs/streams/measured-status/README.md) | P1 | 0/6 | 2026-09-20 |
| [server-controls](docs/streams/server-controls/README.md) | P2 | 0/5 | 2026-09-20 |

## Next up

> **COULD-NOT-CHECK — dead-claim decay did not run.** PR state through `gh pr list` could not be read: gh pr list: exec: "gh": executable file not found in $PATH
> Open branches whose PR/merge request has already **merged or closed** are still counted as claims, so they keep consuming their stream's dispatch cap. The rows below are a **subset**: briefs held behind those dead claims are missing from this board, not absent from the backlog.

| Stream | Brief | Wave | Score |
|---|---|---|---|
| forge-neutral | 18 — statusgen off gh — one desk-tools read verb on the seam, offline lint by default, one git walk [exec:strong] | 5 | 2500 |
| composability | 02 — Install ledger, paired inverses, and the `disable` verb | 1 | 2000 |
| derived-board | 07 — per-repo rollout — upgrade-assay to v1.0.0, reconcile step in each regen workflow, historical backfill as a drift-report PR; private re-stage of spec + skills | 4 | 2000 |
| desk-supervision | 16 — Verification wake conditions — stop repeating unchanged blocked checks [exec:strong] | 0 | 2000 |
| composability | 01 — Reactive activation — a missing extension key downs one component, not the fleet | 1 | 1500 |
| desk-supervision | 19 — Persist review findings and apply the existing round cap across sessions [exec:strong] | 0 | 1500 |
| forge-gitlab | 13 — Board reads degrade per row, never per sweep — the GitLab empty-field class [exec:strong] | 3 | 1500 |
| desk-supervision | 20 — Review scope and first-pass completeness [exec:strong] | 0 | 1000 |
| desk-tools | 27 — `deskboard`'s last serial repo loops onto the pool, `throughput` from one root resolution, `deskflip`'s cheapest gate first, and refusals that name the offline check [exec:strong] | 2 | 1000 |

## Intake queue

_0 untriaged entries — the front door is clear._

## Awaiting verification / review (39 desk-actionable of 59 total — 59 at implemented, 0 verified awaiting review)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner: the desk-actionable headline counts only the queue the desk can actually drain._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._


### Desk-actionable (39)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| contributor-trust | 02 [exec:strong] | implemented | 5000 | 6 | — | — | — |
| graph-execution | 02 [exec:strong] | implemented | 4500 | 5 | — | — | — |
| forge-neutral | 07 [exec:strong] | implemented | 4000 | 4 | — | — | — |
| forge-neutral | 04 [exec:strong] | implemented | 3500 | 3 | — | — | — |
| derived-board | 04 | implemented | 3000 | 2 | — | — | — |
| desk-containers | 02 | implemented | 3000 | 4 | — | — | — |
| contributor-trust | 01 [exec:strong] | implemented | 2500 | 1 | — | — | — |
| contributor-trust | 06 | implemented | 2500 | 1 | — | — | — |
| derived-board | 06 [exec:strong] | implemented | 2500 | 1 | — | — | — |
| desktools-go-git | 02 | implemented | 2500 | 3 | — | 2026-09-11 opus-5[1m]-verifier | — |
| forge-neutral | 09 [exec:strong] | implemented | 2500 | 1 | — | — | — |
| apps-installer | 08 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| contributor-trust | 04 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| desk-supervision | 04 [exec:strong] | implemented | 2000 | 2 | — | — | — |
| forge-neutral | 15 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| forge-neutral | 16 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| forge-neutral | 19 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| desk-tools | 21 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| forge-gitlab | 11 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| harness-portability | 14 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| iso-9001 | 04 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| desk-containers | 08 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-supervision | 09 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-supervision | 22 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 01 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 02 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 03 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 15 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 17 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 18 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 19 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 26 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 28 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| forge-gitlab | 05 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| forge-gitlab | 12 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 13 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 05 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 06 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 09 | implemented | 1000 | 0 | — | — | — |

### Awaiting human gate (5)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| forge-neutral | 01 [exec:strong] | implemented | 8500 | 13 | — | — | — |
| forge-neutral | 02 [exec:strong] | implemented | 5000 | 6 | — | — | — |
| harness-portability | 03 | implemented | 4500 | 7 | — | — | — |
| forge-neutral | 13 [exec:strong] | implemented | 2500 | 1 | — | — | — |
| windows-port | 03 [exec:strong] | implemented | 1500 | 1 | — | — | — |

### Awaiting implementer rework (14)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| harness-portability | 04 [exec:strong] | implemented | 4000 | 6 | — | — | — |
| derived-board | 03 [exec:strong] | implemented | 3500 | 3 | — | — | — |
| harness-portability | 06 [exec:strong] | implemented | 3500 | 5 | — | — | — |
| windows-port | 00 | implemented | 3000 | 4 | — | — | — |
| harness-portability | 12 [exec:strong] | implemented | 2500 | 3 | — | — | — |
| iso-9001 | 01 [exec:strong] | implemented | 2500 | 3 | — | — | — |
| desk-containers | 09 [exec:strong] | implemented | 2000 | 2 | — | — | — |
| windows-port | 01 | implemented | 2000 | 2 | — | — | — |
| desk-supervision | 08 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 07 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 07 | implemented | 1000 | 0 | — | — | — |
| harness-portability | 09 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 10 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 15 [exec:strong] | implemented | 1000 | 0 | — | — | — |

### Parked stream (1)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| desktools-v2 | 01 | implemented | 5500 | 9 | — | — | — |

## Age at the human gate

_Per stream: how long the longest-waiting `gate: human` brief has sat in its CURRENT awaiting status (implemented/verified), from the historian (`.history.jsonl`). Oldest stream first. Render-only — never a Next-up or gate-score input. `—` means the historian has no recorded transition into that status (a brief older than the log, or a fresh checkout): the age is UNKNOWN, not zero._

_Deliberately WIDER than `--signoff-digest`: this counts every `gate: human` brief sitting at implemented/verified, whereas the digest lists only those the per-brief sign-off surface has judged actionable (a recorded model verify pass behind them). A stream appearing here with no digest row is a brief waiting on its VERIFIER, not on the human — a different queue, and worth seeing separately._

| Stream | Oldest at gate | Brief |
|---|---|---|
| apps-installer | — | — |
| contributor-trust | — | — |
| derived-board | — | — |
| desk-containers | — | — |
| desk-supervision | — | — |
| desk-tools | — | — |
| desktools-go-git | — | — |
| forge-gitlab | — | — |
| forge-neutral | — | — |
| harness-portability | — | — |
| statusgen | — | — |
| windows-port | — | — |

## Unresolved findings

_None._

## Incomplete briefs

### apps-installer (6 open)

- 02 `deskapps init` — loopback page, tier manifests, the manifest→code→conversion flow, key and record writes — todo (wave 1)
- 03 `deskapps` install + prove — installation poll, fresh mint, scopes-vs-duties, roster write — todo (wave 2)
- 04 `deskapps resume` / `status` — per-App state machine, throttle pause, expired-code re-arm, page verbs — todo (wave 2)
- 06 Avatar step in the run board — generated PNG beside the settings page, confirmed not uploaded — todo (wave 3)
- 07 Install skill + adoption runbook cutover — `deskapps` replaces the hand runbook, tiers documented — todo (wave 4)
- 08 Solo identity mode — spec and decision for running the desk verbs on the operator's own token — implemented (wave 0)

### auto-triage (4 open)

- 01 Culprit identifier — parse a failing whole-tree gate into check + file:line + mechanical/judgement class — todo (wave 1)
- 02 Mechanical responder — file the bug and open a fixing draft PR, autonomously, for the mechanical class — todo (wave 2)
- 03 Judgement responder — file the bug and route it with a recommended default, for the judgement and opaque classes — todo (wave 2)
- 04 Never-invisible watchdog — escalate any red whole-tree gate that no responder acted on within N minutes — todo (wave 3)

### composability (4 open)

- 01 Reactive activation — a missing extension key downs one component, not the fleet — todo (wave 1)
- 02 Install ledger, paired inverses, and the `disable` verb — todo (wave 1)
- 03 Desired-state record and the reconcile engine behind deskmigrate / upgrade-assay — todo (wave 2)
- 05 Promote the draft to spec/component-v1.md + adopter doc delta — todo (wave 3)

### contributor-trust (9 open)

- 01 Contributor provenance probe — mechanical signals about an unknown author, rendered as a neutral card — implemented (wave 0)
- 02 Trust tiers + the contributor ledger — one vocabulary for how much automation an external identity gets — implemented (wave 0)
- 03 `deskbless` — a structured blessing act with a machine marker, a scope, a reason and an audit row — todo (wave 1)
- 04 Review depth by tier — an unknown author's pull request gets a claims-versus-diff fact check and a fail-first reproduction — implemented (wave 1)
- 05 Fork-safe continuous-integration posture — audited workflows, tier-keyed approve-and-run, and never-build-unblessed enforced by a check — todo (wave 2)
- 06 Contributor-facing documents — state the bar honestly, and ask for verification rather than assertion — implemented (wave 0)
- 07 Agent contributors — disclosure of automated authorship, and tiering the operating human rather than the tool — todo (wave 1)
- 08 External-contribution metrics — inbound pull requests by tier and outcome, on the board — todo (wave 1)
- 09 External-contributor credit in release notes — the aggregator names the author a fork change came from — todo (wave 1)

### derived-board (4 open)

- 03 `statusgen reconcile` — derive lifecycle state from PRs, witnesses, approvals and rulings; brief-v2 parser — implemented (wave 1)
- 04 generated Briefs table in every stream README + single-writer lint + scheduled reconcile PR — implemented (wave 2)
- 06 v1.0.0 — deskmigrate statusgen-regen op, the v0.28.0→v1.0.0 migration, paired-versions bump, same-tag pin lint, brief-reading tools refuse v2 below v1 — implemented (wave 3)
- 07 per-repo rollout — upgrade-assay to v1.0.0, reconcile step in each regen workflow, historical backfill as a drift-report PR; private re-stage of spec + skills — in-progress (wave 4)

### desk-containers (9 open)

- 02 runtime credential contract (PEM + model env) + image layer-secret scan — implemented (wave 1)
- 04 interactive desktop launch script (desk-run.sh) — todo (wave 3)
- 05 docker-compose definition for the five desks — todo (wave 3)
- 06 Kubernetes manifests for the five desks — todo (wave 3)
- 07 multi-desk control layer — tmux/tmuxinator evaluation + cross-platform config — todo (wave 4)
- 08 A tick contract: one bounded pass when the harness says `--tick`, so a loop pod can finish — implemented (wave 3)
- 09 cellctl: host-local harness cell — scrubbed per-cell environment, `smoke`, `status`, session lock, stricter `check` — implemented (wave 0)
- 10 cellctl in Go: `tools/desk/cmd/cellctl`, bash kept as the oracle until parity — todo (wave 1)
- 11 retire the out-of-tree bridge: migrate `CELL_KIND=local` registrations, remove the shell shim, one `cellctl` on PATH — todo (wave 2)

### desk-supervision (16 open)

- 04 Lifecycle hooks — after-create / before-run / after-run / before-remove from config home — implemented (wave 1)
- 08 Objectives over transitions — measure an objective-style worker kit with skillbench — implemented (wave 1)
- 09 Per-push CI fan-out — trigger selection so a docs-only push stops paying for a Go build — implemented (wave 0)
- 10 Confirm or repair the workflow App wiring — one identity holding workflows:write, installed and scope-proven — blocked (wave 0)
- 11 The single-workflow-only-PR contract, and the verb by which the workflow App writes and lands it — blocked (wave 1)
- 12 Retire the staged-copy hand-landing once the workflow App PR path is proven — blocked (wave 2)
- 13 Worker-operations vitals — the self-report resource block — todo (wave 2)
- 14 Budget-driven recycle — retire a healthy worker before it degrades — todo (wave 3)
- 15 Local supervisor host + multi-cell vitals aggregation — todo (wave 4)
- 16 Verification wake conditions — stop repeating unchanged blocked checks — todo (wave 0)
- 17 Verification failures create durable worker repair obligations — todo (wave 1)
- 18 Enforce repair reservations at worker dispatch — todo (wave 2)
- 19 Persist review findings and apply the existing round cap across sessions — todo (wave 0)
- 20 Review scope and first-pass completeness — todo (wave 0)
- 21 Reverify changed external prerequisites without a synthetic push — todo (wave 1)
- 22 Configure provider, model and effort per cell role — implemented (wave 0)

### desk-tools (14 open)

- 01 Binary channel sealed — publish the `.assay-versions` contract, validate it, stamp desk-tools with its release tag — implemented (wave 1)
- 02 Generalize — batch-fanout as the second drain-engine consumer (contract validation) — implemented (wave 1)
- 03 Published-tree residual-identity scrub — drive the cold-read to an independent CLEAN — implemented (wave 1)
- 07 `clusterguard` — exec-boundary shim for cluster CLIs, operator opt-in — implemented (wave 1)
- 11 `deskwt add` — a worktree whose directory is gone does not hold its branch — blocked (wave 1)
- 15 `deskdispatch --dry-run --worktree <path>` — render the prompt against an operator-supplied home — implemented (wave 1)
- 17 One trust bar for public-repo authors — `deskboard` classifies on the same predicate `deskpost` gates on — implemented (wave 1)
- 18 Public-repo write gate — an allowed-repos entry tagged `:public` authorizes outward writes, replacing the per-item `+1` reaction check — implemented (wave 1)
- 19 `verifyloop plan` fails safe on risk — any risk answer `yes` routes to ROUTE-HUMAN, and the Evidence-only lane says so — implemented (wave 1)
- 21 `DESK_TRACE` and cause-carrying errors — one subprocess runner, and a swallowed child's message reaches the operator on the first read — implemented (wave 1)
- 23 Opt-in local usage + timing telemetry — a per-invocation perf record with a 7-day history, and `deskperf` to read it — todo (wave 2)
- 26 `deskwt prune` — one origin/main walk per sweep, the merge gate before `Status()`, batched removal, a read-only `--dry-run`, and a prune singleton — implemented (wave 2)
- 27 `deskboard`'s last serial repo loops onto the pool, `throughput` from one root resolution, `deskflip`'s cheapest gate first, and refusals that name the offline check — in-progress (wave 2)
- 28 Role-keyed verdict signing (deskverdict --key) + the R-7 clause-4 cross-repo scan-delta verify path — implemented (wave 1)

### desktools-go-git (4 open)

- 02 gitcore package + in-process transport/auth (BasicAuth) + go-git pin — implemented (wave 2)
- 05 migrate fetch + retire bespoke hardening (deskgit / deskadvisory / deskmerge) — todo (wave 4)
- 06 migrate push + retire ambient-credential machinery + preflight transport probe — todo (wave 4)
- 08 flip the drop-the-binary CI gate to failing + assert CVE floor + file the follow-on — todo (wave 5)

### desktools-v2 (10 open)

- 01 audit & inventory — enumerate every gh shell-out + hardcoded-forge-assumption site (file:line) — implemented (wave 1)
- 02 the v2 seam contract + the ban-lint (advisory/counting first) — todo (wave 2)
- 03 native read-path installation-token client — retire gh shell-out in the read path (#1223 pilot) — todo (wave 3)
- 04 deskclose reads an authorizing comment by its stated kind — retire the kind-less default (#1019) — todo (wave 2)
- 05 the push guards judge the remote actually being pushed to — deskpushguard base ref (#1201) and insteadOf in the push-transport gate (#884) — todo (wave 2)
- 06 installation-token scoping — one token per repo, no ambient-credential hiding (#628 / #1145 / #1146) — todo (wave 4)
- 08 hold statusgen at zero — the gh ban fails on statusgen and the scan is proven with no gh present — todo (wave 6)
- 09 purpose-built access-pattern query operations (one tuned snapshot, not N per-item calls) — todo (wave 3)
- 10 one outbound-write check at the forge write seam, keyed on the target's visibility — todo (wave 2)
- 11 a house callout for the outbound-write check — deployment vocabulary stays out of the shipped tools — todo (wave 3)

### forge-gitlab (5 open)

- 05 Live pilot — one brief round-tripped on a real GitLab group, security-parity table walked — implemented (wave 4)
- 11 Guard-read custody — the last gh shell-outs onto the Forge seam — implemented (wave 4)
- 12 GitLab hardening reads — repohardenguard kinds on the GitLab backend — implemented (wave 5)
- 13 Board reads degrade per row, never per sweep — the GitLab empty-field class — todo (wave 3)
- 16 The review-tick conformance walk — one live GitLab review tick, every verb, zero hand-built calls — todo (wave 5)

### forge-neutral (25 open)

- 01 Forge resolution contract — the forge comes from repo config, and refusal is the only fallback — implemented (wave 1)
- 02 Forge-qualified identity — roster entries, bot renderings, review corroboration — implemented (wave 2)
- 04 Write verbs B — deskpr, deskfile, deskclose and deskevidence onto the resolver — implemented (wave 2)
- 07 statusgen acting identity — Evidence-actor and verifyrun name the forge identity that acted — implemented (wave 3)
- 09 Substrate — the leak gate's verdict on merge requests, and cellctl's forge-aware new/up — implemented (wave 3)
- 10 Conformance — one round trip driven entirely by desk verbs, and the writes they refuse — todo (wave 5)
- 11 Install without `gh` — binary acquisition, forge-neutral prerequisites, per-forge primitives — todo (wave 5)
- 13 Write verbs C — deskpr, deskfile and deskclose onto the resolver — implemented (wave 3)
- 14 Run and gate-approval verbs — RunWorkflow, ApproveGate and deskrun on the resolver — todo (wave 2)
- 15 desklabel — a role-keyed label verb — implemented (wave 2)
- 16 deskclose widened lanes — author-App self-withdraw, verifier reopen+close on verify-gate, and manifest as the documented human-ruled batch lane — implemented (wave 4)
- 17 deskrun log and retry — read-only run-log access broadly, retry roster-bound like dispatch — todo (wave 2)
- 18 statusgen off gh — one desk-tools read verb on the seam, offline lint by default, one git walk — in-progress (wave 5)
- 19 Human-only surfaces made server-side — merge, workflow-file pushes, rulesets, variables, App installs — implemented (wave 1)
- 20 Measurements — what the reviewer writes, who reads claims, and what the file store can know about where it runs — todo (wave 1)
- 21 Claim store seam — one interface in deskkit, resolved from cell configuration, refusing rather than falling back — todo (wave 2)
- 22 Claim readers onto the seam — the supervisor, the verdict stamp, the fan-out release and the roster read the resolved store — todo (wave 3)
- 23 File claim store — claims in a directory on the cell's host, with the single-host declaration and the container, filesystem and mixed-store guards — todo (wave 3)
- 24 Served claim store — the same directory store behind a small HTTP serve mode, member-initiated, holding no forge credential — todo (wave 4)
- 25 Store-aware duties — the reviewer needs repository read once the cell's store is set, and the boot check names the store — todo (wave 3)
- 28 Scaffold defaults — a fresh host cell gets the file store and the declaration, a container cell gets the served store; existing cells are left alone — todo (wave 5)
- 29 Adopter docs and store-neutral skills — supported topologies, the reviewer at repository read, and the removal window — todo (wave 6)
- 30 Release-N cutover — ship, prove the narrowed reviewer on a live cell, then the operator narrows the grant — todo (wave 7)
- 31 Remaining roles' write audit — what the desk, worker, verifier and loop roles actually write, measured after the reviewer change is live — todo (wave 8)
- 32 Release-N+1 deletion — the forge claim store is removed and an unset store key is refused — todo (wave 8)

### fresh-views (6 open)

- 01 input-sha stamp + refuse-when-stale helper for derived reads — todo (wave 0)
- 02 read-side loop views re-derive against live main: verifyloop plan + scanloop coalesce — todo (wave 1)
- 03 ready-flip gate re-verifies mergeability on main-advance — todo (wave 1)
- 04 shared append-only log discipline: verify-outcomes.jsonl merge=union + deskevidence post-write sha — todo (wave 0)
- 05 mirror-freshness gate: fail the release/CI when a mirrored stamped value drifts from its source — todo (wave 0)
- 06 reconcile ref-resolution across the brief-v2 id flag-day — todo (wave 0)

### graph-execution (7 open)

- 02 Workflow-pattern schema, node contract, and the implementation and research patterns — implemented (wave 0)
- 03 Evidence coverage rule and the observe evidence kind — todo (wave 1)
- 04 Recovery contract for effect-bearing nodes in drainloop — todo (wave 1)
- 05 Offline two-pattern experiment on frozen fixtures — todo (wave 2)
- 06 Run records and the replay/learning loop — todo (wave 3)
- 07 Flow instruments — service/wait split, CI-slot saturation, gate catch/override — todo (wave 1)
- 08 Signal-triggered pattern — incident and regression — todo (wave 1)

### harness-portability (10 open)

- 03 Ruling: target harnesses, delivery channel, degradation matrix — implemented (wave 1)
- 04 Neutral-core skill bodies + per-harness binding files + neutrality lint — implemented (wave 2)
- 06 Codex packaging — generated manifest, coverage rule, install path — implemented (wave 3)
- 07 Adoption docs, freshness registration, live Codex smoke protocol + first run — implemented (wave 4)
- 09 jcode desk-harness spike — measured parity + fleet-density for driving desks — implemented (wave 0)
- 10 SpecMem portable-memory spike — one stream's registers across Claude Code and a second harness — implemented (wave 0)
- 12 Cursor — the third harness column — implemented (wave 5)
- 13 Cursor live-desk-smoke protocol + first run — implemented (wave 6)
- 14 Code de-house — land the stream's tool and packaging deliverables in the public tree — implemented (wave 6)
- 15 Public CI wiring + harnesslint clean-up for the de-housed tools — implemented (wave 7)

### iso-9001 (4 open)

- 01 Emit the tool-validation evidence pack as a release asset — implemented (wave 0)
- 03 A finding closes on a fired control — the corrective-action effectiveness record — todo (wave 1)
- 04 Record the authorizing human in the release itself — implemented (wave 1)
- 06 The auditor one-pager — what Assay is and is not — todo (wave 2)

### measured-status (6 open)

- 01 Derive the deskkit exit-code table — record the convention ExitOK/Disabled/RateLimited/Refused/Unverifiable follow, and pin it with a test — todo (wave 0)
- 02 Derive MinCorpus for the learned riskscore model against its 15-feature events-per-variable floor, or record the rationale — and pin it with a test — todo (wave 0)
- 03 Derive the commsloop router's risk field from the envelope instead of hardcoding false, or record why false is sound — restore the risk:yes->tier:human backstop — todo (wave 0)
- 04 statusgen --lint: derive stale-FAIL vs missing-card from commit dates, and route each state to the verify desk instead of nudging a worker to hand-file a sign-off — todo (wave 0)
- 05 attribution.go: a same-identity author/verifier pair in a multi-identity repo becomes a hard PROBLEM, not a NOTICE — the independence gate the check exists to establish — todo (wave 1)
- 06 model-floor: derive dispatch authority from the dispatch stamp, and give a first-class re-stamp path — stop refusing verdicts by guessing at a label actor's login — todo (wave 1)

### server-controls (5 open)

- 01 Uniform-ruleset audit — define the target menu, read each repo's current ruleset state against it — todo (wave 0)
- 02 Standardize on rulesets and retire classic protection so controls are readable (#1020) — todo (wave 1)
- 03 The required-check enforcement pattern — a fine invariant as a required status check reported by a non-author-controllable runner — todo (wave 1)
- 04 Decision-dependency note — the credential/identity rulings that gate the credential-contract work — todo (wave 0)
- 05 Reference cross-operator / independent-approver check — the residual after require_last_push_approval, as a required status check — todo (wave 2)

### statusgen (3 open)

- 05 Drives phase 3 — anti-starvation floors + the hard critical tier (≤15/20 slots via a 2-pass fill, ≤6/8 workers, effectiveCap; a lexicographic never-buried tier fed by a stamped security label + a dependency-edge reciprocity lint so blockedCount is not gameable) — implemented (wave 1)
- 06 Findings register becomes a corroborated state machine — bounded shelving (parked) + transition guard on resolved/affects/parked — implemented (wave 1)
- 09 Opt-in statusgen telemetry — anonymized fleet-drift corpus (off by default) — implemented (wave 1)

### windows-port (5 open)

- 00 Build-tag split for the unix-only syscall sites in statusgen and desk-tools — implemented (wave 0)
- 01 Release build matrix — windows/amd64 + windows/arm64 + sha256s — implemented (wave 1)
- 03 Windows install path — PowerShell-vs-Go-installer fork, then build — implemented (wave 2)
- 08 Go-native GitLab fleet provisioning — retire the bash+curl+jq script's Windows dependency — todo (wave 1)
- 09 Three-command Windows install — widen the install skill's scope, collapse the walkthrough, correct the CI skew — todo (wave 4)

## Done briefs

_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

### apps-installer (2 done)

- 01 Role→App indirection — six desk roles on N GitHub Apps without symlinks — done (wave 0)
- 05 `deskavatar` — deterministic per-adopter App avatars with a 20 px legibility proof — done (wave 0)

### composability (2 done)

- 00 Component manifests, key catalogue, and the resolve/cycle lint — done (wave 0)
- 04 Harness as an exclusively-bound key — adapters as components — done (wave 1)

### derived-board (3 done)

- 01 brief-v2 spec — derived lifecycle cells, generated table, reserved graph keys; public re-stage of brief-rules + template — done (wave 0)
- 02 `Brief:` trailer — the PR→brief link, required by deskpr create, linted on main — done (wave 0)
- 05 desk skills — reference the brief, never flip the cell (author-brief, worker-desk, pr-review-desk, verify-desk; public copies) — done (wave 1)

### desk-containers (2 done)

- 01 base image — toolchains, desk-tools, assay skills, persistent-volume layout — done (wave 1)
- 03 per-desk images (named by desk) + build matrix + publish wiring — done (wave 2)

### desk-supervision (6 done)

- 01 Observable probes + the `desksupervise` observer — liveness that bites — done (wave 0)
- 02 Per-run stop signal — `STOP.run.<key>` flag + desk-window stop on observer signal — done (wave 1)
- 03 Eligibility reconciliation — stop a run whose item became ineligible — done (wave 2)
- 05 Per-class concurrency reservation — fresh / resume / rework caps in the planner — done (wave 0)
- 06 Workpad — one upserted progress comment per PR — done (wave 0)
- 07 Runtime snapshot — `desksupervise status` for operators and the console — done (wave 1)

### desk-tools (14 done)

- 04 Deterministic runner: execute rows, batch ~5 min, sign, file verdict issues — done (wave 1)
- 05 Escape-valve `Decide()` primitive in deskkit — enum-bounded agent consults for deterministic loops — done (wave 1)
- 06 Roster from deployment — resolve trust / role-binding config from the cell registry + mounted secrets, not a machine-local `roster.env` (design direction) — done (wave 1)
- 08 `deskgit push` / `deskgit fetch --as <role>` — authenticated transport from the role's token file — done (wave 1)
- 09 `desktoken coverage <role>` — list the repositories a role's App installations can see — done (wave 1)
- 10 `deskclaim stale` + branch-liveness on `acquire` — reclaim a dead session's claim through the tool, not by hand — done (wave 1)
- 12 `statusgen brief <stream/NN>` — resolve an item key to its file, frontmatter and board row, as JSON — done (wave 1)
- 13 `pr-monitor.sh` — a paced, per-repo head-sha / draft-state PR monitor shipped in the plugin tree — done (wave 1)
- 14 bodycheck — three measured false-positive classes into the negative corpus, plus `--explain` — done (wave 1)
- 16 `deskevidence` — an Evidence block equivalent to one already standing is a no-op, not a second block — done (wave 1)
- 20 Cross-repo triage/verify evidence binds to the remote — a sibling checkout must be cross-checked, not trusted as-is — done (wave 1)
- 22 Trust-gate account-liveness NOTICE — `deskroster liveness` reads what GitHub currently says about a trusted login, without touching `TrustedAuthor`'s verdict — done (wave 1)
- 24 Audit ledger — bounded tail read in `Guard`, no `desktoken` cache-reuse rows, daily rotation, and a `deskaudit tail` read verb — done (wave 2)
- 25 One token lookup per owner per process — a memo in front of the minter, and `desktoken` consulting its cache BEFORE it resolves the install id — done (wave 2)

### desktools-go-git (4 done)

- 01 inventory freeze + gitexec single-seam contract + golden harness + counting CI gate — done (wave 1)
- 03 migrate read/plumbing verbs (read-heavy tools) to gitcore — done (wave 3)
- 04 migrate deskpushguard detection reads to gitcore (parity + mutation test) — done (wave 3)
- 07 deskmerge exception — fence the trial merge as the sole git-binary caller, migrate the rest — done (wave 3)

### forge-gitlab (12 done)

- 01 Forge interface extraction in deskkit — github impl pinned by goldens — done (wave 1)
- 02 gitlab forge implementation — MRs, notes, approvals, statuses over REST v4 — done (wave 2)
- 03 GitLab token custody — rotate-on-mint + expiry backstop in desktoken — done (wave 2)
- 04 Fleet provisioning script + adopter doc + ci-config-project runbook — done (wave 3)
- 06 Ultimate refinements — custom reviewer role + external-status-check verdict lane — done (wave 5)
- 07 GitHub forge backend on go-gh — retire the exec-`gh` shell path — done (wave 2)
- 08 Close the forge surface — enumerated operations, no passthrough, shell-exec ban — done (wave 3)
- 09 GitLab reviewer write path — deskpost verdict/comment/ready + deskfile/desktoken PAT auth — done (wave 4)
- 10 GitLab trust-events + commit author-login for the deskpost trust read — done (wave 5)
- 14 The public-repo gate reads the forge that serves the repo — every verb, not one — done (wave 3)
- 15 The GitLab runbook's missing keys — forge binding, board-push credential, source-pin lane — done (wave 4)
- 17 An enforceable merge gate on GitLab Free — the unresolved review thread — done (wave 5)

### forge-neutral (5 done)

- 03 Write verbs A — deskpost, deskreply and deskflip onto the resolver — done (wave 2)
- 05 Claim layer — the GitLab shape of refs/dispatch/* and its release — done (wave 2)
- 06 Read verbs — deskboard, issueboard, scanloop and the loop planners on the seam — done (wave 3)
- 08 statusgen forge-aware — init's CI scaffold, auto-flip corroboration, honest claim decay — done (wave 4)
- 12 deskboard non-board reads onto the seam — done (wave 4)

### graph-execution (1 done)

- 01 Eligibility evaluator — gates and feathers become gating, with a reason — done (wave 0)

### harness-portability (4 done)

- 01 Codex capability ground-truth — measured matrix, not inherited prior art — done (wave 0)
- 02 Kill the drift debt — re-sync the bundle, flip the canonical home — done (wave 0)
- 05 Resident rules — one source, per-harness delivery generated — done (wave 2)
- 11 Durable-monitor capability + residual harness-token prose-audit — done (wave 3)

### iso-9001 (2 done)

- 02 Align three shipped disclosures with the code they describe — done (wave 0)
- 05 Records control and retention, stated once — done (wave 1)

### quality (16 done)

- 01 qualgen miner skeleton — go-git extraction, incremental mine, three-state plumbing — done (wave 0)
- 02 M1 line-operation taxonomy + churn / rework rate (GitClear-aligned) — done (wave 1)
- 03 M1 hotspots + knowledge distribution (SPOF) + change coupling — done (wave 1)
- 04 M1 instruction-layer brittleness — reference-validity + doc↔code co-change staleness — done (wave 1)
- 05 single-writer QUALITY.md trend view + metrics/defects/attribution artifact schemas — done (wave 2)
- 06 M2 fix identification — pluggable fix-linkage adapter + GitHub-labels reference adapter + evidence tiers — done (wave 1)
- 07 M2 B-SZZ inducing-commit trace + derived defect metrics — done (wave 2)
- 08 `pr <n>` mode — per-file risk features (generic riskscore feed) — done (wave 3)
- 09 `check <paths>` mode — brittleness screen for a named file set — done (wave 2)
- 10 M3 stage attribution — deterministic dossier + judgment stage-call + per-stage defect ledger — done (wave 3)
- 11 DORA join — quality denominator + traced-CFR refinement + pluggable delivery-metrics source — done (wave 3)
- 12 M4 gate-yield accounting + ritual-effectiveness natural-experiment joins — done (wave 4)
- 13 M4 session forensics — pluggable telemetry-source interface + file reference adapter — done (wave 3)
- 14 closing the loop — auto-filed refactor work + quality error-budgets + RETRO output feed — done (wave 5)
- 15 learned riskscore graduation — JIT defect-prediction model with heuristic fallback — done (wave 3)
- 16 code-slop forensic sweep lane — deterministic suspects → agent verification → evidenced report — done (wave 1)

### statusgen (10 done)

- 01 30-day statusgen check-firing audit — retire cold --lint rules — done (wave 1)
- 02 Issue metrics — statusgen --issues: standard counts + age/sitting-time + internal-vs-external + by-raising-desk — done (wave 1)
- 03 Self-improvement metric — loops that self-diagnose AND self-resolve (agent-raised + agent-fixed, no human touch) vs human-touched — done (wave 2)
- 04 Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck — done (wave 1)
- 07 New brief-flow metrics in statusgen — done (wave 1)
- 08 Composite AssayScore computation — done (wave 2)
- 10 statusgen graph export — derived-only DOT + JSONL from the existing parse tree, evaluated on real multi-hop questions — done (wave 1)
- 11 DORA/insights hybrid — Apache DevLake for commodity metrics, our methodology metrics retained — done (wave 1)
- 12 `homed-in: <owner/repo>` brief field — exclude a brief whose deliverable lives in another repo from THIS board's Next-up, keep its tracking row, carry the target repo — done (wave 1)
- 13 Cadenced roadmap artifacts — `--cadence weekly\|monthly` window computation reusing the roadmap renderer, a `theme:` render rule, config-driven priority order and brand — done (wave 1)

### windows-port (5 done)

- 02 Portability audit — enumerate + triage the shell-assuming surfaces — done (wave 0)
- 04 Windows CI leg — statusgen --lint + a desk-verb smoke on Windows — done (wave 2)
- 05 Adoption-doc delta — the Windows adopter walkthrough — done (wave 3)
- 06 Manifest-driven bootstrap — resolve tag + sha256 from the committed manifest, and write PATH — done (wave 3)
- 07 deskinstall --harness cursor — place the skills/references tree and write the AGENTS.md bindings — done (wave 3)

## Totals

**21** streams (**16** active, **0** paused, **5** parked) · **88/244** briefs done · completed initiatives: see `docs/archive/`
