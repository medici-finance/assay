<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.
     Regenerate: run statusgen against this repo root — installed release binary `statusgen --root .`, or `go run . --root ..` from statusgen/ inside the assay repo. Channels + pin spec: docs/distribution.md, section: The .assay-versions pin file -->

# Project Status

_Repo: `medici-finance/assay` — this board covers the streams in this repo only; sibling repos have their own._

## Roll-up

### Platform

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [derived-board](docs/streams/derived-board/README.md) | P1 | active | 0/7 | 2026-08-23 |  |
| [desk-containers](docs/streams/desk-containers/README.md) | P2 | active | 0/7 | 2026-08-23 |  |
| [desktools-go-git](docs/streams/desktools-go-git/README.md) | P2 | active | 0/8 | 2026-08-23 |  |
| [statusgen](docs/streams/statusgen/README.md) | P2 | active | 0/6 | 2026-08-23 |  |

## Next up

| Stream | Brief | Wave | Score |
|---|---|---|---|
| derived-board | 01 — brief-v2 spec: derived lifecycle, generated table, public re-stage of brief-rules/template [exec:strong] | 0 | 4500 |
| derived-board | 02 — `Brief:` trailer — the PR→brief link, required by `deskpr`, linted on main | 0 | 4500 |
| desktools-go-git | 01 — inventory freeze + `gitexec` single-seam contract + golden harness + counting CI gate | 1 | 4500 |
| desk-containers | 01 — base image — toolchains, desk-tools, skills, volume | 1 | 3500 |
| desk-containers | 02 — runtime credential contract + layer-secret scan | 1 | 3500 |
| statusgen | 02 — issue metrics (`--issues`) | 1 | 1500 |
| statusgen | 04 — ladder-position indicator (`--ladder`) | 1 | 1000 |
| statusgen | 06 — findings register — corroborated state machine [exec:strong] | 1 | 1000 |

## Intake queue

_0 untriaged entries — the front door is clear._

## Awaiting verification / review (2 desk-actionable of 2 total — 2 at implemented, 0 verified awaiting review)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner: the desk-actionable headline counts only the queue the desk can actually drain._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._


### Desk-actionable (2)

| Stream | Brief | Status | Score | _Blocked_ | Age | Verified | Reviewed |
|---|---|---|---|---|---|---|---|
| statusgen | 01 | implemented | 1000 | 0 | — | — | — |
| statusgen | 05 [exec:strong] | implemented | 1000 | 0 | — | — | — |

## Age at the human gate

_Per stream: how long the longest-waiting `gate: human` brief has sat in its CURRENT awaiting status (implemented/verified), from the historian (`.history.jsonl`). Oldest stream first. Render-only — never a Next-up or gate-score input. `—` means the historian has no recorded transition into that status (a brief older than the log, or a fresh checkout): the age is UNKNOWN, not zero._

_Deliberately WIDER than `--signoff-digest`: this counts every `gate: human` brief sitting at implemented/verified, whereas the digest lists only those the per-brief sign-off surface has judged actionable (a recorded model verify pass behind them). A stream appearing here with no digest row is a brief waiting on its VERIFIER, not on the human — a different queue, and worth seeing separately._

| Stream | Oldest at gate | Brief |
|---|---|---|
| statusgen | — | — |

## Unresolved findings

_None._

## Incomplete briefs

### derived-board (7 open)

- 01 brief-v2 spec: derived lifecycle, generated table, public re-stage of brief-rules/template — todo (wave 0)
- 02 `Brief:` trailer — the PR→brief link, required by `deskpr`, linted on main — todo (wave 0)
- 03 `statusgen reconcile` — derive lifecycle state from PRs + witnesses + approvals — todo (wave 1)
- 04 generated Briefs table + single-writer lint — todo (wave 2)
- 05 desk skills: reference the brief, never flip the cell (both copies) — todo (wave 1)
- 06 v1.0.0 cut: migration op + file, paired-versions, same-tag pin lint — todo (wave 3)
- 07 per-repo rollout + historical backfill as a drift-report PR — todo (wave 4)

### desk-containers (7 open)

- 01 base image — toolchains, desk-tools, skills, volume — todo (wave 1)
- 02 runtime credential contract + layer-secret scan — todo (wave 1)
- 03 per-desk images + build matrix + publish wiring — todo (wave 2)
- 04 interactive launch script `desk-run.sh` — todo (wave 3)
- 05 docker-compose definition — todo (wave 3)
- 06 Kubernetes manifests — todo (wave 3)
- 07 multi-desk control layer — tmux/equivalents, macOS + win32 — todo (wave 4)

### desktools-go-git (8 open)

- 01 inventory freeze + `gitexec` single-seam contract + golden harness + counting CI gate — todo (wave 1)
- 02 `gitcore` transport + in-process auth (BasicAuth) + go-git pin — todo (wave 2)
- 03 migrate read/plumbing verbs (read-heavy tools) — todo (wave 3)
- 04 migrate `deskpushguard` detection reads (parity + mutation test) — todo (wave 3)
- 05 migrate fetch + retire bespoke hardening (`deskgit` / `deskadvisory`) — todo (wave 4)
- 06 migrate push + retire ambient-credential machinery + preflight probe — todo (wave 4)
- 07 `deskmerge` exception — fence the trial merge, migrate the rest — todo (wave 3)
- 08 flip the drop-the-binary CI gate + CVE floor + file the follow-on — todo (wave 5)

### statusgen (6 open)

- 01 30-day lint-firing audit — retire cold rules — implemented (wave 1)
- 02 issue metrics (`--issues`) — todo (wave 1)
- 03 self-improvement metric (self-healed vs human-touched) — todo (wave 2)
- 04 ladder-position indicator (`--ladder`) — todo (wave 1)
- 05 drives phase 3 — anti-starvation floors + critical tier — implemented (wave 1)
- 06 findings register — corroborated state machine — todo (wave 1)

## Done briefs

_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

## Totals

**4** streams (**4** active, **0** paused) · **0/28** briefs done · completed initiatives: see `docs/archive/`
