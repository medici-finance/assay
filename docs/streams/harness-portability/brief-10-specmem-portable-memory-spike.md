---
brief: harness-portability/10
title: SpecMem portable-memory spike — one stream's registers across Claude Code and a second harness
why: >-
  The desks' memory and specs are welded to Claude Code's formats (CLAUDE.md, the per-session
  memory dir, the in-git registers). That lock-in is precisely what makes switching harnesses
  expensive — the ceiling this stream removes. SpecMem claims a portable, MCP-exposed memory layer
  usable across agents. If it can hold ONE stream's briefs/registers portably across Claude Code
  AND a second harness, harness-switching gets cheap; if it can't, we learn the real cost before
  betting a migration on it. This spike INFORMS — it does not gate — the HP/03 harness-target
  ruling (a proposal of the authoring session, not a dependency Ian set).
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-16 by intake-desk authoring session
sources:
  - "authoring dispatch (Ian, 2026-08-16): also evaluate SpecMem — two spikes, one per question; this is spike (b)"
  - "SpecMem: SuperagenticAI/specmem (Apache-2.0, built at Kiroween 2025) — github.com/SuperagenticAI/specmem. Claims: unified, embeddable, MCP-exposed cognitive-memory layer over Spec-Driven-Development metadata. Note: upstream is Kiro-native (it indexes `.kiro/specs/` as the agent memory) and asserts portability of that spec store ACROSS agents — whether it serves faithfully to a NON-Kiro harness is exactly the claim this spike tests, not an assumed freedom from `.kiro`/CLAUDE.md/.cursorrules"
  - "freshness-checked 2026-08-16: no docs/research/specmem-* file exists"
exec-tier: strong
exec-tier-why: >-
  (b) correctness depends on cross-harness reasoning — whether the SAME specs/registers serve
  faithfully to two different agents is exactly the portability claim under test, not a demo.
---

# Brief 10 — SpecMem portable-memory spike

## Context
files:
- **create** `docs/research/specmem-portability-spike.md` (planned) — the measured findings + go/no-go
- **amend** `freshness.yaml` (planned) — register the new file (empirical facts rot)
out-of-repo files: none (SpecMem runs as an MCP server against a chosen stream's docs; no desk memory is migrated in this spike)
facts:
- the SECOND harness is whichever is available to pair with Claude Code — Codex (this stream's primary target) or jcode (HP/09). The claim under test is harness-INDEPENDENCE, so the pairing is the point, not the specific partner.
- pick a LOW-STAKES stream's briefs/registers for the trial — never a load-bearing register (the desk registers stay authoritative in-git during the spike).
- the test is faithfulness, not presence: does querying the SAME SpecMem store from two harnesses return the same specs/impact/context, or does one harness silently get a degraded view?

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- Do NOT move any load-bearing desk register into SpecMem; this is a read-mostly portability trial on a copy of one low-stakes stream.

## Task
1. Stand up SpecMem (Apache-2.0) as an MCP server over a COPY of one low-stakes stream's briefs/registers.
2. Query it from Claude Code AND from a second harness (Codex or jcode) via its MCP surface — specs lookup, impact analysis, optimized-context retrieval.
3. Fill the planned findings file: what served IDENTICALLY across both harnesses vs what still needed each harness's native format; and where SpecMem's spec model does / does not fit Assay's brief-v1 + register shapes.
4. Write the go/no-go read: does SpecMem meaningfully de-couple memory from the harness (making a Claude-Code↔other switch cheap), and at what adoption cost. Register the file in `freshness.yaml`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | exit 0 — the findings doc exists (planned deliverable) |
| 2 | `grep -qiE -e identical -e degraded -e portable -e native-only docs/research/specmem-portability-spike.md` | exit 0 — faithfulness verdicts present (separate `-e` patterns; `\|` in `-E` is a literal pipe) |
| 3 | (dereferencing) the doc records the SAME query run from BOTH harnesses with their actual returned output quoted — proving portability was exercised, not asserted | two harnesses' outputs for one query are both quoted and compared |
| 4 | `grep -q 'specmem-portability-spike' freshness.yaml` | exit 0 — the empirical file is registered |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer at implementation time -->
SpecMem portable-memory spike. Documentary/architectural spike (no code, no gating config — "informs, does not gate" HP/03). Verified in-tree at the source repo. SpecMem referenced as documentary upstream only (no local clone).

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `test -f docs/research/specmem-portability-spike.md` | 0 | present | 2026-08-24 | opus-4.8[1m]-verifier |
| 2 | grep -qiE identical/degraded/portable/native-only in the doc | 0 | all four faithfulness terms present (§4 table + §5) | 2026-08-24 | opus-4.8[1m]-verifier |
| 3 | SAME query from BOTH harnesses, outputs compared | — | COULD-NOT-CHECK / BLOCKED by design — no live specmem-mcp server + no 2nd MCP harness offline; doc declares BLOCKED §7, gives verbatim reproduction protocol §6, does NOT fabricate output | 2026-08-24 | opus-4.8[1m]-verifier |
| 4 | `grep -q specmem-portability-spike freshness.yaml` | 0 | registered (last-reviewed 2026-08-24, max-age-days 45, upstreams []) | 2026-08-24 | opus-4.8[1m]-verifier |

**RISK-VALUE: DERIVED** — top value = `go/no-go verdict = NO-GO (watch-list)`, DERIVED in §5 from the §4 register-mapping (SpecMem ships no adapter for brief-v1/Verify/freshness/STATUS; only CLAUDE.md natively ingested; adoption would re-lock registers to Kiro's SDD triad) — internally sound, follows from documented upstream facts, not a bare assertion. `max-age-days=45`, `last-reviewed="2026-08-24"`/`upstreams:[]` — NAMED, house-consistent, reversible (empty-upstreams justified inline: SpecMem not a locally-tracked clone). No irreversible operational literal.

**VERIFY: PASS** — all 3 mechanically-runnable rows (1,2,4) pass; row 3 is a genuinely unrunnable live cross-harness comparison, honestly declared BLOCKED §7 with a positive-control reproduction protocol §6 and no fabricated output (matching the brief-01 house pattern). The go/no-go read does not hinge on the blocked row — it turns on the statically-assessable register-mapping. gate:model + all-risk-no → flipped `implemented → verified`.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the harness-portability README table.
