<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.
     Regenerate: run statusgen against this repo root — installed release binary `statusgen --root .`, or `go run . --root ..` from statusgen/ inside the assay repo. Channels + pin spec: docs/distribution.md, section: The .assay-versions pin file -->

# Project Status

_Repo: `medici-finance/assay` — this board covers the streams in this repo only; sibling repos have their own._

## Roll-up

### Platform

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [derived-board](docs/streams/derived-board/README.md) | P1 | active | 3/7 | 2026-09-04 |  |
| [desk-containers](docs/streams/desk-containers/README.md) | P2 | active | 0/7 | 2026-09-05 |  |
| [desk-supervision](docs/streams/desk-supervision/README.md) | P2 | active | 2/8 | 2026-09-05 |  |
| [desk-tools](docs/streams/desk-tools/README.md) | P2 | active | 3/16 | 2026-09-05 |  |
| [desktools-go-git](docs/streams/desktools-go-git/README.md) | P2 | active | 1/8 | 2026-09-05 |  |
| [forge-gitlab](docs/streams/forge-gitlab/README.md) | P2 | active | 3/8 | 2026-09-05 |  |
| [forge-neutral](docs/streams/forge-neutral/README.md) | P1 | active | 0/11 | 2026-09-05 |  |
| [harness-portability](docs/streams/harness-portability/README.md) | P2 | active | 0/12 | 2026-09-05 |  |
| [iso-9001](docs/streams/iso-9001/README.md) | P2 | active | 1/6 | 2026-09-05 |  |
| [mistake-proofing](docs/streams/mistake-proofing/README.md) | P2 | active | 4/6 | 2026-09-05 |  |
| [quality](docs/streams/quality/README.md) | P2 | active | 13/16 | 2026-09-05 |  |
| [spec-routing](docs/streams/spec-routing/README.md) | P2 | active | 1/1 | 2026-09-05 |  |
| [statusgen](docs/streams/statusgen/README.md) | P2 | active | 7/13 | 2026-09-05 |  |
| [windows-port](docs/streams/windows-port/README.md) | P2 | active | 1/6 | 2026-09-05 |  |

## Next up

_Held by per-stream caps: 3 brief(s) across 1 stream(s) — top: desk-tools. By stream: desk-tools (3). A stream at its dispatch cap (perStreamCap 4, a declared max-concurrent, or in-flight claims) offers nothing more until a claiming branch or PR clears — this backlog is capped here, not drained._

| Stream | Brief | Wave | Score |
|---|---|---|---|
| iso-9001 | 01 — Emit the tool-validation evidence pack as a release asset (7.1.5) [exec:strong] | 0 | 2500 |
| desk-supervision | 02 — Per-run stop signal — `STOP.run.<key>` flag + desk-window stop on observer signal [exec:strong] | 1 | 1500 |
| desk-supervision | 04 — Lifecycle hooks — after-create / before-run / after-run / before-remove from config home [exec:strong] | 1 | 1000 |
| desk-supervision | 07 — Runtime snapshot — `desksupervise status` for operators and the console | 1 | 1000 |
| desk-tools | 08 — `deskgit push` / `deskgit fetch --as <role>` — authenticated transport from the role's token file [exec:strong] | 1 | 1000 |
| desk-tools | 10 — `deskclaim stale` + branch-liveness on `acquire` — reclaim a dead session's claim through the tool, not by hand [exec:strong] | 1 | 1000 |
| desk-tools | 12 — `statusgen brief <stream/NN>` — resolve an item key to its file, frontmatter and board row, as JSON | 1 | 1000 |
| desk-tools | 13 — `pr-monitor.sh` — a paced, per-repo head-sha / draft-state PR monitor shipped in the plugin tree | 1 | 1000 |
| mistake-proofing | 05 — `newbrief` — the scaffolder as the authoring front door (B1) [exec:strong] | 2 | 1000 |
| mistake-proofing | 06 — D1 promoted to a lint obligation — a new check must carry its mutation row | 2 | 1000 |
| quality | 15 — learned riskscore graduation — JIT defect-prediction model [exec:strong] | 3 | 1000 |

## Intake queue

_0 untriaged entries — the front door is clear._

## Awaiting verification / review (35 desk-actionable of 38 total — 38 at implemented, 0 verified awaiting review)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner: the desk-actionable headline counts only the queue the desk can actually drain._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._


### Desk-actionable (35)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| forge-neutral | 01 [exec:strong] | implemented | 7000 | 10 | — | — | — |
| harness-portability | 01 [exec:strong] | implemented | 5000 | 8 | — | — | — |
| desktools-go-git | 02 | implemented | 4000 | 6 | — | — | — |
| harness-portability | 02 [exec:strong] | implemented | 4000 | 6 | — | — | — |
| derived-board | 03 [exec:strong] | implemented | 3500 | 3 | — | — | — |
| desk-containers | 01 | implemented | 3500 | 5 | — | — | — |
| desk-containers | 02 | implemented | 3500 | 5 | — | — | — |
| harness-portability | 04 [exec:strong] | implemented | 3500 | 5 | — | — | — |
| derived-board | 04 | implemented | 3000 | 2 | — | — | — |
| desk-containers | 03 | implemented | 3000 | 4 | — | — | — |
| harness-portability | 05 [exec:strong] | implemented | 3000 | 4 | — | — | — |
| windows-port | 00 | implemented | 3000 | 4 | — | — | — |
| harness-portability | 06 [exec:strong] | implemented | 2500 | 3 | — | — | — |
| desk-supervision | 06 | implemented | 1500 | 1 | — | — | — |
| forge-gitlab | 05 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| forge-gitlab | 07 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| harness-portability | 12 [exec:strong] | implemented | 1500 | 1 | — | — | — |
| iso-9001 | 05 | implemented | 1500 | 1 | — | — | — |
| desk-tools | 01 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 02 | implemented | 1000 | 0 | — | — | — |
| desk-tools | 03 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 07 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| desk-tools | 09 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| forge-gitlab | 08 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 07 | implemented | 1000 | 0 | — | — | — |
| harness-portability | 09 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 10 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| harness-portability | 11 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| quality | 14 | implemented | 1000 | 0 | — | — | — |
| quality | 16 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 03 | implemented | 1000 | 0 | — | — | — |
| statusgen | 05 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 06 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 08 | implemented | 1000 | 0 | — | — | — |
| statusgen | 09 | implemented | 1000 | 0 | — | — | — |

### Awaiting human gate (1)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| harness-portability | 03 | implemented | 4500 | 7 | — | — | — |

### Awaiting implementer rework (2)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| forge-gitlab | 04 | implemented | 2000 | 2 | — | — | — |
| statusgen | 11 [exec:strong] | implemented | 1000 | 0 | — | — | — |

## Age at the human gate

_Per stream: how long the longest-waiting `gate: human` brief has sat in its CURRENT awaiting status (implemented/verified), from the historian (`.history.jsonl`). Oldest stream first. Render-only — never a Next-up or gate-score input. `—` means the historian has no recorded transition into that status (a brief older than the log, or a fresh checkout): the age is UNKNOWN, not zero._

_Deliberately WIDER than `--signoff-digest`: this counts every `gate: human` brief sitting at implemented/verified, whereas the digest lists only those the per-brief sign-off surface has judged actionable (a recorded model verify pass behind them). A stream appearing here with no digest row is a brief waiting on its VERIFIER, not on the human — a different queue, and worth seeing separately._

| Stream | Oldest at gate | Brief |
|---|---|---|
| desk-containers | — | — |
| desk-tools | — | — |
| desktools-go-git | — | — |
| forge-gitlab | — | — |
| forge-neutral | — | — |
| harness-portability | — | — |
| statusgen | — | — |

## Unresolved findings

_None._

## Incomplete briefs

### derived-board (4 open)

- 03 `statusgen reconcile` — derive lifecycle state from PRs, witnesses, approvals and rulings; brief-v2 parser — implemented (wave 1)
- 04 generated Briefs table in every stream README + single-writer lint + scheduled reconcile PR — implemented (wave 2)
- 06 v1.0.0 — deskmigrate statusgen-regen op, the v0.13.0→v1.0.0 migration, paired-versions bump, same-tag pin lint, brief-reading tools refuse v2 below v1 — todo (wave 3)
- 07 per-repo rollout — upgrade-assay to v1.0.0, reconcile step in each regen workflow, historical backfill as a drift-report PR; private re-stage of spec + skills — todo (wave 4)

### desk-containers (7 open)

- 01 base image — toolchains, desk-tools, skills, volume — implemented (wave 1)
- 02 runtime credential contract + layer-secret scan — implemented (wave 1)
- 03 per-desk images + build matrix + publish wiring — implemented (wave 2)
- 04 interactive launch script `desk-run.sh` — todo (wave 3)
- 05 docker-compose definition — todo (wave 3)
- 06 Kubernetes manifests — todo (wave 3)
- 07 multi-desk control layer — tmux/equivalents, macOS + win32 — todo (wave 4)

### desk-supervision (6 open)

- 02 Per-run stop signal — `STOP.run.<key>` flag + desk-window stop on observer signal — todo (wave 1)
- 03 Eligibility reconciliation — stop a run whose item became ineligible — todo (wave 2)
- 04 Lifecycle hooks — after-create / before-run / after-run / before-remove from config home — todo (wave 1)
- 06 Workpad — one upserted progress comment per PR — implemented (wave 0)
- 07 Runtime snapshot — `desksupervise status` for operators and the console — todo (wave 1)
- 08 Objectives over transitions — measure an objective-style worker kit with skillbench — todo (wave 1)

### desk-tools (13 open)

- 01 Binary channel — publish the `.assay-versions` contract, validate it, stamp desk-tools with its release tag — implemented (wave 1)
- 02 Generalize — batch-fanout as the second drain-engine consumer — implemented (wave 1)
- 03 Published-tree residual-identity scrub — drive the cold-read to an independent CLEAN — implemented (wave 1)
- 07 `clusterguard` — exec-boundary shim for cluster CLIs, operator opt-in — implemented (wave 1)
- 08 `deskgit push` / `deskgit fetch --as <role>` — authenticated transport from the role's token file — todo (wave 1)
- 09 `desktoken coverage <role>` — list the repositories a role's App installations can see — implemented (wave 1)
- 10 `deskclaim stale` + branch-liveness on `acquire` — reclaim a dead session's claim through the tool, not by hand — todo (wave 1)
- 11 `deskwt add` — a worktree whose directory is gone does not hold its branch — blocked (wave 1)
- 12 `statusgen brief <stream/NN>` — resolve an item key to its file, frontmatter and board row, as JSON — todo (wave 1)
- 13 `pr-monitor.sh` — a paced, per-repo head-sha / draft-state PR monitor shipped in the plugin tree — todo (wave 1)
- 14 bodycheck — three measured false-positive classes into the negative corpus, plus `--explain` — todo (wave 1)
- 15 `deskdispatch --dry-run --worktree <path>` — render the prompt against an operator-supplied home — todo (wave 1)
- 16 `deskevidence` — an Evidence block equivalent to one already standing is a no-op, not a second block — todo (wave 1)

### desktools-go-git (7 open)

- 02 `gitcore` transport + in-process auth (BasicAuth) + go-git pin — implemented (wave 2)
- 03 migrate read/plumbing verbs (read-heavy tools) — todo (wave 3)
- 04 migrate `deskpushguard` detection reads (parity + mutation test) — todo (wave 3)
- 05 migrate fetch + retire bespoke hardening (`deskgit` / `deskadvisory`) — todo (wave 4)
- 06 migrate push + retire ambient-credential machinery + preflight probe — todo (wave 4)
- 07 `deskmerge` exception — fence the trial merge, migrate the rest — todo (wave 3)
- 08 flip the drop-the-binary CI gate + CVE floor + file the follow-on — todo (wave 5)

### forge-gitlab (5 open)

- 04 Fleet provisioning + adopter doc + ci-config-project runbook — implemented (wave 3)
- 05 Live pilot — one brief round-tripped on a real GitLab group; parity table walked — implemented (wave 4)
- 06 Ultimate refinements — custom reviewer role + external-status-check verdict lane — todo (wave 5)
- 07 GitHub forge backend on `go-gh` — retire the exec-`gh` shell path — implemented (wave 2)
- 08 Close the forge surface — enumerated operations, no passthrough, shell-exec ban — implemented (wave 3)

### forge-neutral (11 open)

- 01 Forge resolution contract — the forge comes from repo config, and refusal is the only fallback — implemented (wave 1)
- 02 Forge-qualified identity — roster entries, bot renderings, review corroboration — todo (wave 2)
- 03 Write verbs A — deskpost, deskreply, deskflip onto the resolver — todo (wave 2)
- 04 Write verbs B — deskpr, deskfile, deskclose, deskevidence onto the resolver — todo (wave 2)
- 05 Claim layer — the GitLab shape of `refs/dispatch/*` and its release — todo (wave 2)
- 06 Read verbs — deskboard, issueboard, scanloop, reviewloop on the seam — todo (wave 3)
- 07 statusgen acting identity — Evidence-actor and `verifyrun` name the forge identity that acted — todo (wave 3)
- 08 statusgen forge-aware — `init` CI scaffold, auto-flip corroboration, honest claim decay — todo (wave 4)
- 09 Substrate — leak-gate verdict on merge requests, `cellctl` forge-aware `new`/`up` — todo (wave 3)
- 10 Conformance — one round trip driven entirely by desk verbs, and the writes they refuse — todo (wave 5)
- 11 Install without `gh` — binary acquisition, forge-neutral prerequisites, per-forge primitives — todo (wave 5)

### harness-portability (12 open)

- 01 Codex capability ground-truth — measured matrix, not inherited prior art — implemented (wave 0)
- 02 Kill the drift debt — re-sync the bundle, flip the canonical home — implemented (wave 0)
- 03 Ruling: target harnesses, delivery channel, degradation matrix — implemented (wave 1)
- 04 Neutral-core skill bodies + per-harness binding files + neutrality lint — implemented (wave 2)
- 05 Resident rules: one source, per-harness delivery generated — implemented (wave 2)
- 06 Codex packaging — generated manifest, coverage rule, install path — implemented (wave 3)
- 07 Adoption docs, freshness registration, live Codex smoke protocol + first run — implemented (wave 4)
- 09 jcode desk-harness spike — measured parity + fleet-density for driving desks — implemented (wave 0)
- 10 SpecMem portable-memory spike — one stream's registers across two harnesses — implemented (wave 0)
- 11 Durable-monitor capability + residual harness-token prose-audit — implemented (wave 3)
- 12 Cursor — the third harness column (ground-truth + binding + generator verb + public column) — implemented (wave 5)
- 13 Cursor live-desk-smoke protocol + first run — todo (wave 6)

### iso-9001 (5 open)

- 01 Emit the tool-validation evidence pack as a release asset (7.1.5) — todo (wave 0)
- 03 A finding closes on a fired control — the effectiveness record (10.2) — todo (wave 1)
- 04 Record the authorizing human in the release itself (8.6) — todo (wave 1)
- 05 Records control and retention, stated once (7.5.3) — implemented (wave 1)
- 06 The auditor one-pager — what Assay is and is not — todo (wave 2)

### mistake-proofing (2 open)

- 05 `newbrief` — the scaffolder as the authoring front door (B1) — todo (wave 2)
- 06 D1 promoted to a lint obligation — a new check must carry its mutation row — todo (wave 2)

### quality (3 open)

- 14 auto-filed refactor work + quality error-budgets + RETRO output feed — implemented (wave 5)
- 15 learned riskscore graduation — JIT defect-prediction model — todo (wave 3)
- 16 code-slop forensic sweep lane — deterministic suspects → agent verification → evidenced report — implemented (wave 1)

### statusgen (6 open)

- 03 self-improvement metric (self-healed vs human-touched) — implemented (wave 2)
- 05 drives phase 3 — anti-starvation floors + critical tier — implemented (wave 1)
- 06 findings register — corroborated state machine — implemented (wave 1)
- 08 composite AssayScore computation — implemented (wave 2)
- 09 opt-in telemetry — anonymized fleet-drift corpus (off by default) — implemented (wave 1)
- 11 DORA/insights hybrid — DevLake commodity split — implemented (wave 1)

### windows-port (5 open)

- 00 Build-tag split for the unix-only syscall sites — implemented (wave 0)
- 01 Release build matrix — windows/amd64 + windows/arm64 + sha256s — todo (wave 1)
- 03 Windows install path — PowerShell-vs-Go-installer fork, then build — todo (wave 2)
- 04 Windows CI leg — statusgen --lint + a desk-verb smoke on Windows — todo (wave 2)
- 05 Adoption-doc delta — the Windows adopter walkthrough — todo (wave 3)

## Done briefs

_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

### derived-board (3 done)

- 01 brief-v2 spec — derived lifecycle cells, generated table, reserved graph keys; public re-stage of brief-rules + template — done (wave 0)
- 02 `Brief:` trailer — the PR→brief link, required by deskpr create, linted on main — done (wave 0)
- 05 desk skills — reference the brief, never flip the cell (author-brief, worker-desk, pr-review-desk, verify-desk; public copies) — done (wave 1)

### desk-supervision (2 done)

- 01 Observable probes + the `desksupervise` observer — liveness that bites — done (wave 0)
- 05 Per-class concurrency reservation — fresh / resume / rework caps in the planner — done (wave 0)

### desk-tools (3 done)

- 04 Deterministic runner — execute rows, batch, sign, file verdict issues — done (wave 1)
- 05 Escape-valve `Decide()` primitive in deskkit — done (wave 1)
- 06 Roster from deployment — resolve trust / role-binding config from the cell registry + mounted secrets, not a machine-local `roster.env` (design direction) — done (wave 1)

### desktools-go-git (1 done)

- 01 inventory freeze + `gitexec` single-seam contract + golden harness + counting CI gate — done (wave 1)

### forge-gitlab (3 done)

- 01 `Forge` interface extraction in deskkit — `github` impl pinned by goldens — done (wave 1)
- 02 `gitlab` forge implementation (MRs, notes, approvals, statuses) — done (wave 2)
- 03 GitLab token custody — rotate-on-mint + expiry backstop in desktoken — done (wave 2)

### iso-9001 (1 done)

- 02 Align three shipped disclosures with the code they describe (B9) — done (wave 0)

### mistake-proofing (4 done)

- 01 Cross-read a brief's declared paths against the risk classifier (B3) — done (wave 0)
- 02 Dereference named identifiers, not just backticked paths (B4) — done (wave 0)
- 03 Typed Verify-row obligation classes, derived from the diff shape (B2, D7) — done (wave 1)
- 04 Derive the authoring guidance's enforcement-status claims from the lint (B9) — done (wave 1)

### quality (13 done)

- 01 miner skeleton — go-git extraction, incremental runs, three-state plumbing — done (wave 0)
- 02 M1 line-operation taxonomy + churn/rework rate — done (wave 1)
- 03 M1 hotspots + knowledge distribution (SPOF) + change coupling — done (wave 1)
- 04 M1 instruction-layer brittleness (reference-validity + doc↔code drift) — done (wave 1)
- 05 `QUALITY.md` single-writer trend view + metrics artifacts — done (wave 2)
- 06 M2 fix identification — pluggable linkage adapter + evidence tiers — done (wave 1)
- 07 M2 B-SZZ inducing trace + derived defect metrics — done (wave 2)
- 08 `pr <n>` mode — per-file risk features (generic riskscore feed) — done (wave 3)
- 09 `check <files>` mode — brittleness screen for a named file set — done (wave 2)
- 10 M3 stage attribution — dossier + ledger, pluggable provenance-linkage adapter — done (wave 3)
- 11 DORA join — quality denominator + traced-CFR, pluggable delivery-metrics source — done (wave 3)
- 12 M4 gate-yield accounting + ritual-effectiveness joins — done (wave 4)
- 13 M4 session forensics — pluggable telemetry-source interface + reference adapters — done (wave 3)

### spec-routing (1 done)

- 01 Enforce the §8 lifecycle — the linter and the authoring-owed emitter — done (wave 0)

### statusgen (7 done)

- 01 30-day lint-firing audit — retire cold rules — done (wave 1)
- 02 issue metrics (`--issues`) — done (wave 1)
- 04 ladder-position indicator (`--ladder`) — done (wave 1)
- 07 new brief-flow metrics — done (wave 1)
- 10 graph export (`--graph` DOT + JSONL) — done (wave 1)
- 12 `homed-in: <owner/repo>` — exclude a re-homed brief from THIS board's Next-up, keep its tracking row, carry the target repo — done (wave 1)
- 13 cadenced roadmap artifacts (`--cadence weekly/monthly`) — done (wave 1)

### windows-port (1 done)

- 02 Portability audit — enumerate + triage the shell-assuming surfaces — done (wave 0)

## Totals

**14** streams (**14** active, **0** paused) · **39/125** briefs done · completed initiatives: see `docs/archive/`
