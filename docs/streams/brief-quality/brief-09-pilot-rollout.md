---
brief: "assay:assay:brief-quality:09"
title: "Evaluate authoring quality and decide whether to expand"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 6
depends: ["brief-quality/08"]
unblocks: []
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
issues: []
schema: "brief-v2"
id: "20af60e5-c4b8-412f-b7d1-63892e13ce51"
version: 1
authored: "2026-09-26 by the authoring session recorded in commit 6a7d90b97"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
gate-why: "Human authority selects review obligations and data/budget policy; a model cannot ratify these choices."
decision-trigger: "spec"
---

# Brief 09 — Evaluate authoring quality and decide whether to expand

## Context

files:
- `docs/streams/brief-quality/pilot-report.md (NEW)`
- `docs/streams/brief-quality/rollout.md (NEW)`

facts:
- Read-first: `docs/streams/brief-quality/spec.md`; this is a proposed design, not active fleet policy.
- Current source was inspected at `7aa3835d7` on 2026-09-26; recheck before implementation.
- Existing brief-v2 IDs/version and qualgen telemetry/attribution are the integration seams.
- No live infrastructure, automatic publication, or raw session collection is needed.

layering: Extend existing tools; pure assessment/reduction rules in qualgen/briefquality, file/identity/dispatch effects at adapters. Test both separately and one full offline join.

## Ground rules
- Preserve human gates and capability minima. Stop at implemented; a non-implementer verifies.
- If facts conflict with current source, record NEEDS_CONTEXT rather than guessing.
- Follow the ruled pilot policy; unknown evidence is never a passing result.

## Task
1. Run the ruled offline pilot within its declared budget/retention envelope and preserve all eligible outcomes.
2. Compare authored/approved/completed assessments and adjudicated gaps; report measured costs and missing evidence.
3. Have a fresh non-author judge review claims, confounding and observation horizon; propose bounded template corrections.
4. Obtain the human promote/revise/stop ruling; document version pins and rollback before any default gate is enabled.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `python3 -c "from pathlib import Path; s=Path('docs/streams/brief-quality/pilot-report.md').read_text(); assert all(x in s for x in ['cohort','coverage','cost','censor','ruling'])"` | exit 0; report structure exists; rerun the report command recorded in report against retained inputs and compare digest |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: human. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
