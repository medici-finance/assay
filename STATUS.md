<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.
     Regenerate: run statusgen against this repo root — installed release binary `statusgen --root .`, or `go run . --root ..` from statusgen/ inside the assay repo. Channels + pin spec: docs/distribution.md, section: The .assay-versions pin file -->

# Project Status

_Repo: `medici-finance/assay` — this board covers the streams in this repo only; sibling repos have their own._

## Roll-up

### Platform

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [derived-board](docs/streams/derived-board/README.md) | P1 | active | 0/7 | 2026-08-25 |  |
| [desk-containers](docs/streams/desk-containers/README.md) | P2 | active | 0/7 | 2026-08-25 |  |
| [desktools-go-git](docs/streams/desktools-go-git/README.md) | P2 | active | 0/8 | 2026-08-25 |  |
| [forge-gitlab](docs/streams/forge-gitlab/README.md) | P2 | active | 0/6 | 2026-08-25 |  |
| [iso-9001](docs/streams/iso-9001/README.md) | P2 | active | 0/6 | 2026-08-25 |  |
| [mistake-proofing](docs/streams/mistake-proofing/README.md) | P2 | active | 0/6 | 2026-08-25 |  |
| [quality](docs/streams/quality/README.md) | P2 | active | 0/15 | 2026-08-25 |  |
| [statusgen](docs/streams/statusgen/README.md) | P2 | active | 0/6 | 2026-08-25 |  |

## Next up

| Stream | Brief | Wave | Score |
|---|---|---|---|
| forge-gitlab | 01 — `Forge` interface extraction in deskkit — `github` impl pinned by goldens [exec:strong] | 1 | 3500 |
| mistake-proofing | 01 — Cross-read a brief's declared paths against the risk classifier (B3) [exec:strong] | 0 | 3000 |
| iso-9001 | 01 — Emit the tool-validation evidence pack as a release asset (7.1.5) [exec:strong] | 0 | 2500 |
| iso-9001 | 02 — Align three shipped disclosures with the code they describe (B9) [exec:strong] | 0 | 2000 |
| mistake-proofing | 02 — Dereference named identifiers, not just backticked paths (B4) | 0 | 1500 |

## Intake queue

_0 untriaged entries — the front door is clear._

## Awaiting verification / review (12 desk-actionable of 12 total — 12 at implemented, 0 verified awaiting review)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner: the desk-actionable headline counts only the queue the desk can actually drain._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._


### Desk-actionable (12)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| quality | 01 [exec:strong] | implemented | 8000 | 14 | — | — | — |
| derived-board | 01 [exec:strong] | implemented | 4500 | 5 | — | — | — |
| derived-board | 02 | implemented | 4500 | 5 | — | — | — |
| desktools-go-git | 01 | implemented | 4500 | 7 | — | — | — |
| desk-containers | 01 | implemented | 3500 | 5 | — | — | — |
| desk-containers | 02 | implemented | 3500 | 5 | — | — | — |
| desk-containers | 03 | implemented | 3000 | 4 | — | — | — |
| statusgen | 02 | implemented | 1500 | 1 | — | — | — |
| statusgen | 01 | implemented | 1000 | 0 | — | — | — |
| statusgen | 04 | implemented | 1000 | 0 | — | — | — |
| statusgen | 05 [exec:strong] | implemented | 1000 | 0 | — | — | — |
| statusgen | 06 [exec:strong] | implemented | 1000 | 0 | — | — | — |

## Age at the human gate

_Per stream: how long the longest-waiting `gate: human` brief has sat in its CURRENT awaiting status (implemented/verified), from the historian (`.history.jsonl`). Oldest stream first. Render-only — never a Next-up or gate-score input. `—` means the historian has no recorded transition into that status (a brief older than the log, or a fresh checkout): the age is UNKNOWN, not zero._

_Deliberately WIDER than `--signoff-digest`: this counts every `gate: human` brief sitting at implemented/verified, whereas the digest lists only those the per-brief sign-off surface has judged actionable (a recorded model verify pass behind them). A stream appearing here with no digest row is a brief waiting on its VERIFIER, not on the human — a different queue, and worth seeing separately._

| Stream | Oldest at gate | Brief |
|---|---|---|
| desk-containers | — | — |
| statusgen | — | — |

## Unresolved findings

_None._

## Incomplete briefs

### derived-board (7 open)

- 01 brief-v2 spec: derived lifecycle, generated table, public re-stage of brief-rules/template — implemented (wave 0)
- 02 `Brief:` trailer — the PR→brief link, required by `deskpr`, linted on main — implemented (wave 0)
- 03 `statusgen reconcile` — derive lifecycle state from PRs + witnesses + approvals — todo (wave 1)
- 04 generated Briefs table + single-writer lint — todo (wave 2)
- 05 desk skills: reference the brief, never flip the cell (both copies) — todo (wave 1)
- 06 v1.0.0 cut: migration op + file, paired-versions, same-tag pin lint — todo (wave 3)
- 07 per-repo rollout + historical backfill as a drift-report PR — todo (wave 4)

### desk-containers (7 open)

- 01 base image — toolchains, desk-tools, skills, volume — implemented (wave 1)
- 02 runtime credential contract + layer-secret scan — implemented (wave 1)
- 03 per-desk images + build matrix + publish wiring — implemented (wave 2)
- 04 interactive launch script `desk-run.sh` — todo (wave 3)
- 05 docker-compose definition — todo (wave 3)
- 06 Kubernetes manifests — todo (wave 3)
- 07 multi-desk control layer — tmux/equivalents, macOS + win32 — todo (wave 4)

### desktools-go-git (8 open)

- 01 inventory freeze + `gitexec` single-seam contract + golden harness + counting CI gate — implemented (wave 1)
- 02 `gitcore` transport + in-process auth (BasicAuth) + go-git pin — todo (wave 2)
- 03 migrate read/plumbing verbs (read-heavy tools) — todo (wave 3)
- 04 migrate `deskpushguard` detection reads (parity + mutation test) — todo (wave 3)
- 05 migrate fetch + retire bespoke hardening (`deskgit` / `deskadvisory`) — todo (wave 4)
- 06 migrate push + retire ambient-credential machinery + preflight probe — todo (wave 4)
- 07 `deskmerge` exception — fence the trial merge, migrate the rest — todo (wave 3)
- 08 flip the drop-the-binary CI gate + CVE floor + file the follow-on — todo (wave 5)

### forge-gitlab (6 open)

- 01 `Forge` interface extraction in deskkit — `github` impl pinned by goldens — todo (wave 1)
- 02 `gitlab` forge implementation (MRs, notes, approvals, statuses) — todo (wave 2)
- 03 GitLab token custody — rotate-on-mint + expiry backstop in desktoken — todo (wave 2)
- 04 Fleet provisioning + adopter doc + ci-config-project runbook — todo (wave 3)
- 05 Live pilot — one brief round-tripped on a real GitLab group; parity table walked — todo (wave 4)
- 06 Ultimate refinements — custom reviewer role + external-status-check verdict lane — todo (wave 5)

### iso-9001 (6 open)

- 01 Emit the tool-validation evidence pack as a release asset (7.1.5) — todo (wave 0)
- 02 Align three shipped disclosures with the code they describe (B9) — todo (wave 0)
- 03 A finding closes on a fired control — the effectiveness record (10.2) — todo (wave 1)
- 04 Record the authorizing human in the release itself (8.6) — todo (wave 1)
- 05 Records control and retention, stated once (7.5.3) — todo (wave 1)
- 06 The auditor one-pager — what Assay is and is not — todo (wave 2)

### mistake-proofing (6 open)

- 01 Cross-read a brief's declared paths against the risk classifier (B3) — todo (wave 0)
- 02 Dereference named identifiers, not just backticked paths (B4) — todo (wave 0)
- 03 Typed Verify-row obligation classes, derived from the diff shape (B2, D7) — todo (wave 1)
- 04 Derive the authoring guidance's enforcement-status claims from the lint (B9) — todo (wave 1)
- 05 `newbrief` — the scaffolder as the authoring front door (B1) — todo (wave 2)
- 06 D1 promoted to a lint obligation — a new check must carry its mutation row — todo (wave 2)

### quality (15 open)

- 01 miner skeleton — go-git extraction, incremental runs, three-state plumbing — implemented (wave 0)
- 02 M1 line-operation taxonomy + churn/rework rate — todo (wave 1)
- 03 M1 hotspots + knowledge distribution (SPOF) + change coupling — todo (wave 1)
- 04 M1 instruction-layer brittleness (reference-validity + doc↔code drift) — todo (wave 1)
- 05 `QUALITY.md` single-writer trend view + metrics artifacts — todo (wave 2)
- 06 M2 fix identification — pluggable linkage adapter + evidence tiers — todo (wave 1)
- 07 M2 B-SZZ inducing trace + derived defect metrics — todo (wave 2)
- 08 `pr <n>` mode — per-file risk features (generic riskscore feed) — todo (wave 3)
- 09 `check <files>` mode — brittleness screen for a named file set — todo (wave 2)
- 10 M3 stage attribution — dossier + ledger, pluggable provenance-linkage adapter — todo (wave 3)
- 11 DORA join — quality denominator + traced-CFR, pluggable delivery-metrics source — todo (wave 3)
- 12 M4 gate-yield accounting + ritual-effectiveness joins — todo (wave 4)
- 13 M4 session forensics — pluggable telemetry-source interface + reference adapters — todo (wave 3)
- 14 auto-filed refactor work + quality error-budgets + RETRO output feed — todo (wave 5)
- 15 learned riskscore graduation — JIT defect-prediction model — todo (wave 3)

### statusgen (6 open)

- 01 30-day lint-firing audit — retire cold rules — implemented (wave 1)
- 02 issue metrics (`--issues`) — implemented (wave 1)
- 03 self-improvement metric (self-healed vs human-touched) — todo (wave 2)
- 04 ladder-position indicator (`--ladder`) — implemented (wave 1)
- 05 drives phase 3 — anti-starvation floors + critical tier — implemented (wave 1)
- 06 findings register — corroborated state machine — implemented (wave 1)

## Done briefs

_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

## Totals

**8** streams (**8** active, **0** paused) · **0/61** briefs done · completed initiatives: see `docs/archive/`
