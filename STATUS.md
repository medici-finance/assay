<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.
     Regenerate: run statusgen against this repo root — installed release binary `statusgen --root .`, or `go run . --root ..` from statusgen/ inside the assay repo. Channels + pin spec: docs/distribution.md, section: The .assay-versions pin file -->

# Project Status

_Repo: `medici-finance/assay` — this board covers the streams in this repo only; sibling repos have their own._

## Roll-up

### Platform

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [apps-installer](docs/streams/apps-installer/README.md) | P1 | active | 0/8 | 2026-09-10 |  |
| [composability](docs/streams/composability/README.md) | P2 | active | 0/6 | 2026-09-10 |  |
| [derived-board](docs/streams/derived-board/README.md) | P1 | active | 3/7 | 2026-09-10 |  |
| [desk-containers](docs/streams/desk-containers/README.md) | P2 | active | 0/7 | 2026-09-10 |  |
| [desk-supervision](docs/streams/desk-supervision/README.md) | P2 | active | 4/9 | 2026-09-10 |  |
| [desk-tools](docs/streams/desk-tools/README.md) | P2 | active | 5/21 | 2026-09-10 |  |
| [desktools-go-git](docs/streams/desktools-go-git/README.md) | P2 | active | 1/8 | 2026-09-10 |  |
| [forge-gitlab](docs/streams/forge-gitlab/README.md) | P2 | active | 4/8 | 2026-09-10 |  |
| [forge-neutral](docs/streams/forge-neutral/README.md) | P1 | active | 0/13 | 2026-09-10 |  |
| [harness-portability](docs/streams/harness-portability/README.md) | P2 | active | 1/14 | 2026-09-10 |  |
| [iso-9001](docs/streams/iso-9001/README.md) | P2 | active | 2/6 | 2026-09-10 |  |
| [mistake-proofing](docs/streams/mistake-proofing/README.md) | P2 | active | 6/6 | 2026-09-10 |  |
| [quality](docs/streams/quality/README.md) | P2 | active | 15/16 | 2026-09-10 |  |
| [spec-routing](docs/streams/spec-routing/README.md) | P2 | active | 1/1 | 2026-09-10 |  |
| [statusgen](docs/streams/statusgen/README.md) | P2 | active | 8/13 | 2026-09-10 |  |
| [windows-port](docs/streams/windows-port/README.md) | P2 | active | 3/6 | 2026-09-10 |  |

## Next up

| Stream | Brief | Wave | Score |
|---|---|---|---|
| derived-board | 07 — per-repo rollout — upgrade-assay to v1.0.0, reconcile step in each regen workflow, historical backfill as a drift-report PR; private re-stage of spec + skills | 4 | 2000 |
| desk-tools | 17 — One trust bar for public-repo authors — `deskboard` classifies on the same predicate `deskpost` gates on [exec:strong] | 1 | 1000 |
| desk-tools | 18 — Public-repo write gate — an allowed-repos entry tagged `:public` authorizes outward writes, replacing the per-item `+1` reaction check [exec:strong] | 1 | 1000 |

## Intake queue

_0 untriaged entries — the front door is clear._

## Awaiting verification / review (51 desk-actionable of 61 total — 61 at implemented, 0 verified awaiting review)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner: the desk-actionable headline counts only the queue the desk can actually drain._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._


### Desk-actionable (51)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| harness-portability | 01 [exec:strong] | implemented | 5500 | 9 | — | — | — |
| apps-installer | 01 [exec:strong] | implemented | 4500 | 5 | — | — | — |
| harness-portability | 02 [exec:strong] | implemented | 4500 | 7 | — | — | — |
| desktools-go-git | 02 | implemented | 4000 | 6 | — | — | — |
| harness-portability | 04 [exec:strong] | implemented | 4000 | 6 | — | — | — |
| harness-portability | 05 [exec:strong] | implemented | 4000 | 6 | — | — | — |
| composability | 00 | implemented | 3500 | 5 | — | — | — |
| derived-board | 03 [exec:strong] | implemented | 3500 | 3 | — | — | — |
| desk-containers | 01 | implemented | 3500 | 5 | — | — | — |
| desk-containers | 02 | implemented | 3500 | 5 | — | — | — |
| harness-portability | 06 [exec:strong] | implemented | 3500 | 5 | — | — | — |
| apps-installer | 05 | implemented | 3000 | 2 | — | — | — |
| derived-board | 04 | implemented | 3000 | 2 | — | — | — |
| forge-neutral | 04 [exec:strong] | implemented | 3000 | 2 | — | — | — |
| forge-neutral | 06 [exec:strong] | implemented | 3000 | 2 | — | — | — |
| derived-board | 06 [exec:strong] | implemented | 2500 | 1 | — | — | — |
| forge-neutral | 05 [exec:strong] | implemented | 2500 | 1 | — | — | — |
| harness-portability | 12 [exec:strong] | implemented | 2500 | 3 | — | — | — |
| apps-installer | 08 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| forge-neutral | 12 [exec:strong] | implemented | 2000 | 0 | — | — | — |
| windows-port | 00 | implemented | 2000 | 2 | — | — | — |
| desk-supervision | 06 | implemented | 1500 | 1 | — | — | — |
| forge-gitlab | 05 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| forge-gitlab | 07 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| harness-portability | 14 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| windows-port | 01 | implemented | 1500 | 1 | — | — | — |
| desk-supervision | 03 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-supervision | 04 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-supervision | 09 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 01 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 02 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 03 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 08 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 10 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 12 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 13 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 15 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 16 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 19 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 21 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| forge-gitlab | 08 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 07 | implemented | 1000 | 0 | — | — | — |
| harness-portability | 09 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 10 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 13 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 15 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| quality | 15 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 03 | implemented | 1000 | 0 | — | — | — |
| statusgen | 05 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 06 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 09 | implemented | 1000 | 0 | — | — | — |

### Awaiting human gate (4)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| forge-neutral | 01 [exec:strong] | implemented | 8000 | 12 | — | — | — |
| harness-portability | 03 | implemented | 5000 | 8 | — | — | — |
| forge-neutral | 02 [exec:strong] | implemented | 4500 | 5 | — | — | — |
| windows-port | 03 [exec:strong] | implemented | 1000 | 0 | — | — | — |

### Awaiting implementer rework (6)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| forge-neutral | 03 [exec:strong] | implemented | 3500 | 3 | — | — | — |
| desk-containers | 03 | implemented | 3000 | 4 | — | — | — |
| iso-9001 | 01 [exec:strong] | implemented | 2500 | 3 | — | — | — |
| desk-tools | 07 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 09 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 11 [exec:strong] | implemented | 1000 | 0 | — | — | — |

## Age at the human gate

_Per stream: how long the longest-waiting `gate: human` brief has sat in its CURRENT awaiting status (implemented/verified), from the historian (`.history.jsonl`). Oldest stream first. Render-only — never a Next-up or gate-score input. `—` means the historian has no recorded transition into that status (a brief older than the log, or a fresh checkout): the age is UNKNOWN, not zero._

_Deliberately WIDER than `--signoff-digest`: this counts every `gate: human` brief sitting at implemented/verified, whereas the digest lists only those the per-brief sign-off surface has judged actionable (a recorded model verify pass behind them). A stream appearing here with no digest row is a brief waiting on its VERIFIER, not on the human — a different queue, and worth seeing separately._

| Stream | Oldest at gate | Brief |
|---|---|---|
| apps-installer | — | — |
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

### apps-installer (8 open)

- 01 Role→App indirection — six desk roles on N GitHub Apps without symlinks — implemented (wave 0)
- 02 `deskapps init` — loopback page, tier manifests, the manifest→code→conversion flow, key and record writes — todo (wave 1)
- 03 `deskapps` install + prove — installation poll, fresh mint, scopes-vs-duties, roster write — todo (wave 2)
- 04 `deskapps resume` / `status` — per-App state machine, throttle pause, expired-code re-arm, page verbs — todo (wave 2)
- 05 `deskavatar` — deterministic per-adopter App avatars with a 20 px legibility proof — implemented (wave 0)
- 06 Avatar step in the run board — generated PNG beside the settings page, confirmed not uploaded — todo (wave 3)
- 07 Install skill + adoption runbook cutover — `deskapps` replaces the hand runbook, tiers documented — todo (wave 4)
- 08 Solo identity mode — spec and decision for running the desk verbs on the operator's own token — implemented (wave 0)

### composability (6 open)

- 00 Component manifests, key catalogue, and the resolve/cycle lint — implemented (wave 0)
- 01 Reactive activation — a missing extension key downs one component, not the fleet — todo (wave 1)
- 02 Install ledger, paired inverses, and the `disable` verb — todo (wave 1)
- 03 Desired-state record and the reconcile engine behind deskmigrate / upgrade-assay — todo (wave 2)
- 04 Harness as an exclusively-bound key — adapters as components — todo (wave 1)
- 05 Promote the draft to spec/component-v1.md + adopter doc delta — todo (wave 3)

### derived-board (4 open)

- 03 `statusgen reconcile` — derive lifecycle state from PRs, witnesses, approvals and rulings; brief-v2 parser — implemented (wave 1)
- 04 generated Briefs table in every stream README + single-writer lint + scheduled reconcile PR — implemented (wave 2)
- 06 v1.0.0 — deskmigrate statusgen-regen op, the v0.28.0→v1.0.0 migration, paired-versions bump, same-tag pin lint, brief-reading tools refuse v2 below v1 — implemented (wave 3)
- 07 per-repo rollout — upgrade-assay to v1.0.0, reconcile step in each regen workflow, historical backfill as a drift-report PR; private re-stage of spec + skills — in-progress (wave 4)

### desk-containers (7 open)

- 01 base image — toolchains, desk-tools, assay skills, persistent-volume layout — implemented (wave 1)
- 02 runtime credential contract (PEM + model env) + image layer-secret scan — implemented (wave 1)
- 03 per-desk images (named by desk) + build matrix + publish wiring — implemented (wave 2)
- 04 interactive desktop launch script (desk-run.sh) — todo (wave 3)
- 05 docker-compose definition for the five desks — todo (wave 3)
- 06 Kubernetes manifests for the five desks — todo (wave 3)
- 07 multi-desk control layer — tmux/tmuxinator evaluation + cross-platform config — todo (wave 4)

### desk-supervision (5 open)

- 03 Eligibility reconciliation — stop a run whose item became ineligible — implemented (wave 2)
- 04 Lifecycle hooks — after-create / before-run / after-run / before-remove from config home — implemented (wave 1)
- 06 Workpad — one upserted progress comment per PR — implemented (wave 0)
- 08 Objectives over transitions — measure an objective-style worker kit with skillbench — todo (wave 1)
- 09 Per-push CI fan-out — trigger selection so a docs-only push stops paying for a Go build — implemented (wave 0)

### desk-tools (16 open)

- 01 Binary channel sealed — publish the `.assay-versions` contract, validate it, stamp desk-tools with its release tag — implemented (wave 1)
- 02 Generalize — batch-fanout as the second drain-engine consumer (contract validation) — implemented (wave 1)
- 03 Published-tree residual-identity scrub — drive the cold-read to an independent CLEAN — implemented (wave 1)
- 07 `clusterguard` — exec-boundary shim for cluster CLIs, operator opt-in — implemented (wave 1)
- 08 `deskgit push` / `deskgit fetch --as <role>` — authenticated transport from the role's token file — implemented (wave 1)
- 09 `desktoken coverage <role>` — list the repositories a role's App installations can see — implemented (wave 1)
- 10 `deskclaim stale` + branch-liveness on `acquire` — reclaim a dead session's claim through the tool, not by hand — implemented (wave 1)
- 11 `deskwt add` — a worktree whose directory is gone does not hold its branch — blocked (wave 1)
- 12 `statusgen brief <stream/NN>` — resolve an item key to its file, frontmatter and board row, as JSON — implemented (wave 1)
- 13 `pr-monitor.sh` — a paced, per-repo head-sha / draft-state PR monitor shipped in the plugin tree — implemented (wave 1)
- 15 `deskdispatch --dry-run --worktree <path>` — render the prompt against an operator-supplied home — implemented (wave 1)
- 16 `deskevidence` — an Evidence block equivalent to one already standing is a no-op, not a second block — implemented (wave 1)
- 17 One trust bar for public-repo authors — `deskboard` classifies on the same predicate `deskpost` gates on — todo (wave 1)
- 18 Public-repo write gate — an allowed-repos entry tagged `:public` authorizes outward writes, replacing the per-item `+1` reaction check — todo (wave 1)
- 19 `verifyloop plan` fails safe on risk — any risk answer `yes` routes to ROUTE-HUMAN, and the Evidence-only lane says so — implemented (wave 1)
- 21 `DESK_TRACE` and cause-carrying errors — one subprocess runner, and a swallowed child's message reaches the operator on the first read — implemented (wave 1)

### desktools-go-git (7 open)

- 02 gitcore package + in-process transport/auth (BasicAuth) + go-git pin — implemented (wave 2)
- 03 migrate read/plumbing verbs (read-heavy tools) to gitcore — todo (wave 3)
- 04 migrate deskpushguard detection reads to gitcore (parity + mutation test) — todo (wave 3)
- 05 migrate fetch + retire bespoke hardening (deskgit / deskadvisory / deskmerge) — todo (wave 4)
- 06 migrate push + retire ambient-credential machinery + preflight transport probe — todo (wave 4)
- 07 deskmerge exception — fence the trial merge as the sole git-binary caller, migrate the rest — todo (wave 3)
- 08 flip the drop-the-binary CI gate to failing + assert CVE floor + file the follow-on — todo (wave 5)

### forge-gitlab (4 open)

- 05 Live pilot — one brief round-tripped on a real GitLab group, security-parity table walked — implemented (wave 4)
- 06 Ultimate refinements — custom reviewer role + external-status-check verdict lane — todo (wave 5)
- 07 GitHub forge backend on go-gh — retire the exec-`gh` shell path — implemented (wave 2)
- 08 Close the forge surface — enumerated operations, no passthrough, shell-exec ban — implemented (wave 3)

### forge-neutral (13 open)

- 01 Forge resolution contract — the forge comes from repo config, and refusal is the only fallback — implemented (wave 1)
- 02 Forge-qualified identity — roster entries, bot renderings, review corroboration — implemented (wave 2)
- 03 Write verbs A — deskpost, deskreply and deskflip onto the resolver — implemented (wave 2)
- 04 Write verbs B — deskpr, deskfile, deskclose and deskevidence onto the resolver — implemented (wave 2)
- 05 Claim layer — the GitLab shape of refs/dispatch/* and its release — implemented (wave 2)
- 06 Read verbs — deskboard, issueboard, scanloop and the loop planners on the seam — implemented (wave 3)
- 07 statusgen acting identity — Evidence-actor and verifyrun name the forge identity that acted — todo (wave 3)
- 08 statusgen forge-aware — init's CI scaffold, auto-flip corroboration, honest claim decay — todo (wave 4)
- 09 Substrate — the leak gate's verdict on merge requests, and cellctl's forge-aware new/up — todo (wave 3)
- 10 Conformance — one round trip driven entirely by desk verbs, and the writes they refuse — todo (wave 5)
- 11 Install without `gh` — binary acquisition, forge-neutral prerequisites, per-forge primitives — todo (wave 5)
- 12 deskboard non-board reads onto the seam — implemented (wave 4)
- 13 Write verbs C — deskpr, deskfile and deskclose onto the resolver — todo (wave 3)

### harness-portability (13 open)

- 01 Codex capability ground-truth — measured matrix, not inherited prior art — implemented (wave 0)
- 02 Kill the drift debt — re-sync the bundle, flip the canonical home — implemented (wave 0)
- 03 Ruling: target harnesses, delivery channel, degradation matrix — implemented (wave 1)
- 04 Neutral-core skill bodies + per-harness binding files + neutrality lint — implemented (wave 2)
- 05 Resident rules — one source, per-harness delivery generated — implemented (wave 2)
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
- 04 Record the authorizing human in the release itself — todo (wave 1)
- 06 The auditor one-pager — what Assay is and is not — todo (wave 2)

### quality (1 open)

- 15 learned riskscore graduation — JIT defect-prediction model with heuristic fallback — implemented (wave 3)

### statusgen (5 open)

- 03 Self-improvement metric — loops that self-diagnose AND self-resolve (agent-raised + agent-fixed, no human touch) vs human-touched — implemented (wave 2)
- 05 Drives phase 3 — anti-starvation floors + the hard critical tier (≤15/20 slots via a 2-pass fill, ≤6/8 workers, effectiveCap; a lexicographic never-buried tier fed by a stamped security label + a dependency-edge reciprocity lint so blockedCount is not gameable) — implemented (wave 1)
- 06 Findings register becomes a corroborated state machine — bounded shelving (parked) + transition guard on resolved/affects/parked — implemented (wave 1)
- 09 Opt-in statusgen telemetry — anonymized fleet-drift corpus (off by default) — implemented (wave 1)
- 11 DORA/insights hybrid — Apache DevLake for commodity metrics, our methodology metrics retained — implemented (wave 1)

### windows-port (3 open)

- 00 Build-tag split for the unix-only syscall sites in statusgen and desk-tools — implemented (wave 0)
- 01 Release build matrix — windows/amd64 + windows/arm64 + sha256s — implemented (wave 1)
- 03 Windows install path — PowerShell-vs-Go-installer fork, then build — implemented (wave 2)

## Done briefs

_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

### derived-board (3 done)

- 01 brief-v2 spec — derived lifecycle cells, generated table, reserved graph keys; public re-stage of brief-rules + template — done (wave 0)
- 02 `Brief:` trailer — the PR→brief link, required by deskpr create, linted on main — done (wave 0)
- 05 desk skills — reference the brief, never flip the cell (author-brief, worker-desk, pr-review-desk, verify-desk; public copies) — done (wave 1)

### desk-supervision (4 done)

- 01 Observable probes + the `desksupervise` observer — liveness that bites — done (wave 0)
- 02 Per-run stop signal — `STOP.run.<key>` flag + desk-window stop on observer signal — done (wave 1)
- 05 Per-class concurrency reservation — fresh / resume / rework caps in the planner — done (wave 0)
- 07 Runtime snapshot — `desksupervise status` for operators and the console — done (wave 1)

### desk-tools (5 done)

- 04 Deterministic runner: execute rows, batch ~5 min, sign, file verdict issues — done (wave 1)
- 05 Escape-valve `Decide()` primitive in deskkit — enum-bounded agent consults for deterministic loops — done (wave 1)
- 06 Roster from deployment — resolve trust / role-binding config from the cell registry + mounted secrets, not a machine-local `roster.env` (design direction) — done (wave 1)
- 14 bodycheck — three measured false-positive classes into the negative corpus, plus `--explain` — done (wave 1)
- 20 Cross-repo triage/verify evidence binds to the remote — a sibling checkout must be cross-checked, not trusted as-is — done (wave 1)

### desktools-go-git (1 done)

- 01 inventory freeze + gitexec single-seam contract + golden harness + counting CI gate — done (wave 1)

### forge-gitlab (4 done)

- 01 Forge interface extraction in deskkit — github impl pinned by goldens — done (wave 1)
- 02 gitlab forge implementation — MRs, notes, approvals, statuses over REST v4 — done (wave 2)
- 03 GitLab token custody — rotate-on-mint + expiry backstop in desktoken — done (wave 2)
- 04 Fleet provisioning script + adopter doc + ci-config-project runbook — done (wave 3)

### harness-portability (1 done)

- 11 Durable-monitor capability + residual harness-token prose-audit — done (wave 3)

### iso-9001 (2 done)

- 02 Align three shipped disclosures with the code they describe — done (wave 0)
- 05 Records control and retention, stated once — done (wave 1)

### mistake-proofing (6 done)

- 01 Cross-read a brief's declared paths against the risk classifier — the one authoring mistake that downgrades a gate — done (wave 0)
- 02 Dereference named identifiers, not just backticked paths — test and function names must resolve — done (wave 0)
- 03 Typed Verify-row obligation classes — carry the prose MUSTs as data and derive their presence from the diff — done (wave 1)
- 04 Derive the authoring guidance's enforcement-status claims from the lint itself — done (wave 1)
- 05 newbrief — the scaffolder as the authoring front door, so derived fields stop being typed — done (wave 2)
- 06 D1 promoted to a lint obligation — a change that adds a check must carry its mutation row — done (wave 2)

### quality (15 done)

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
- 16 code-slop forensic sweep lane — deterministic suspects → agent verification → evidenced report — done (wave 1)

### spec-routing (1 done)

- 01 Enforce the §8 spec/scoping-doc lifecycle — the linter and the authoring-owed emitter — done (wave 0)

### statusgen (8 done)

- 01 30-day statusgen check-firing audit — retire cold --lint rules — done (wave 1)
- 02 Issue metrics — statusgen --issues: standard counts + age/sitting-time + internal-vs-external + by-raising-desk — done (wave 1)
- 04 Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck — done (wave 1)
- 07 New brief-flow metrics in statusgen — done (wave 1)
- 08 Composite AssayScore computation — done (wave 2)
- 10 statusgen graph export — derived-only DOT + JSONL from the existing parse tree, evaluated on real multi-hop questions — done (wave 1)
- 12 `homed-in: <owner/repo>` brief field — exclude a brief whose deliverable lives in another repo from THIS board's Next-up, keep its tracking row, carry the target repo — done (wave 1)
- 13 Cadenced roadmap artifacts — `--cadence weekly\|monthly` window computation reusing the roadmap renderer, a `theme:` render rule, config-driven priority order and brand — done (wave 1)

### windows-port (3 done)

- 02 Portability audit — enumerate + triage the shell-assuming surfaces — done (wave 0)
- 04 Windows CI leg — statusgen --lint + a desk-verb smoke on Windows — done (wave 2)
- 05 Adoption-doc delta — the Windows adopter walkthrough — done (wave 3)

## Totals

**16** streams (**16** active, **0** paused) · **53/149** briefs done · completed initiatives: see `docs/archive/`
