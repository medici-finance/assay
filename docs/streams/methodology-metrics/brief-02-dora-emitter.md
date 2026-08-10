---
brief: methodology-metrics/02
title: DORA metrics emitter — statusgen --dora (5 metrics from historian + git/gh + verify-desk)
wave: 1
depends: ["methodology-metrics/01"]
unblocks: ["methodology/18"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (methodology-metrics scoping)
sources: ["https://dora.dev/ (DORA Core)", "methodology-metrics/01 (historian)", "methodology/brief-18 (retro consumer)", "verify-desk skill (change-failure/rework feed)"]
---

# Brief 02 — DORA metrics emitter (`statusgen --dora`)

## Context
files: `../assay-toolkit/statusgen/` (new `--dora` subcommand), reads `methodology-metrics/01`'s history log,
`git log`, `gh pr list --state merged`, `gh issue list --label bug`.
facts:
- DORA Core = 5 metrics in two families — throughput: **deployment frequency**, **change lead time**,
  **failed-deploy recovery time**; instability: **change failure rate**, **rework rate**.
- Mapping (see methodology/brief-18 for the table): deployment freq = merges/commits per period (git/gh);
  lead time = commit→merge AND `implemented→done` age (the latter needs the 01 historian); change failure
  = (verify-desk `VERIFY: FAIL` + new `bug` issues) ÷ merged; recovery = broken-main→green; rework =
  follow-up briefs/bugs from post-merge defects ÷ merged.
- Some inputs are computable now (git/gh); some need manual/verify-desk input (recovery, change-failure
  detail). The emitter computes what it can and **explicitly labels** the rest as needing input — it
  never fabricates a number (DORA: don't game the metric).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add `statusgen --dora [--since <date>]`: emit the 5 metrics over the window, as a **system** (all five
   together, throughput + instability grouped) — never one in isolation. Read the 01 history log for
   lead-time (implemented→done), git/gh for frequency + commit→merge lead-time.
2. For inputs not yet automatable (recovery time; change-failure needing verify-desk fail records), print
   the metric with an explicit `needs: verify-desk|manual` marker and a `0/unknown` placeholder — do NOT
   invent a value.
3. Machine-readable output option (`--dora --json`) so the retro (methodology/18) and a future `--trend`
   (methodology-metrics/03) can consume it.
4. Include the DORA anti-gaming note in `--dora`'s header/help: diagnostic, per-project, not a target.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run Dora` | exit 0 (metric-computation tests over a testdata history + fixture git/gh data) |
| 2 | `statusgen --dora --since 2026-07-01` | prints all 5 metrics grouped throughput/instability; un-automatable ones marked `needs:` with no fabricated number, exit 0 |
| 3 | `statusgen --dora --json` | valid JSON with the 5 metric keys |
| 4 | `go vet ./tools/statusgen/ && statusgen --lint` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run Dora` | 0 | 8 Dora tests PASS (incl. `TestDoraNoNumberFabricatedWhenGHUnavailable`, `TestDoraLeadTimeNoDataIsUnknownNotZero`) | 2026-07-10 | opus-verifier |
| 2 | `go run ./tools/statusgen --dora --since 2026-07-01` | 0 | 5 metrics grouped Throughput/Instability; recovery+rework marked `[needs: verify-desk\|manual]`, no fabricated number | 2026-07-10 | opus-verifier |
| 3 | `go run ./tools/statusgen --dora --json` | 0 | valid JSON, all 5 metric keys present; jq-parse ok | 2026-07-10 | opus-verifier |
| 4 | `go vet ./tools/statusgen/ && --lint` | 0 | clean | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — DORA emitter renders 5 metrics, never fabricates (no-data = Unknown), JSON valid.

## Review
Gate: model. Reviewer confirms no metric is fabricated when its input is missing (the `needs:` path), and
the five are presented as a system. Note the framing risk: `--dora` output is diagnostic, not a scoreboard.
