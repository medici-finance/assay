---
brief: "assay:assay:brief-quality:02"
title: "Ratify acceptance-review and pilot policy"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 1
depends: ["brief-quality/01"]
unblocks: ["brief-quality/03"]
effort: "S"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
id: "2dc2d1ad-84e0-4bdf-b088-900f59faa220"
version: 1
authored: "2026-09-26 by the authoring session recorded in commit 6a7d90b97"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
gate-why: "Human authority selects review obligations and data/budget policy; a model cannot ratify these choices."
decision-trigger: "creation"
---

# Brief 02 — Ratify acceptance-review and pilot policy

## Context

files:
- `docs/streams/brief-quality/decisions.md (NEW)`
- `docs/streams/brief-quality/spec.md`

facts:
- Read-first: `docs/streams/brief-quality/spec.md`; this is a proposed design, not active fleet policy.
- Current source was inspected at `7aa3835d7` on 2026-09-26; recheck before implementation.
- Existing brief-v2 IDs/version and qualgen telemetry/attribution are the integration seams.
- No live infrastructure, automatic publication, or raw session collection is needed.

layering: Extend existing tools; pure assessment/reduction rules in qualgen/briefquality, file/identity/dispatch effects at adapters. Test both separately and one full offline join.

## Human decision
The proposed brief-quality pilot would add an independent review of acceptance criteria before implementation. The author would still write the initial definition of done. Review adds cost and queue time; its benefit has not been measured. We also need a policy for model diversity and retained cost records.

Options:
1. **Bounded pilot** — require a separate non-author review run for every new brief in explicitly selected pilot streams; prefer model diversity without making it mandatory. Keep structured records private and approve cohort, retention, spending ceiling and evaluation horizon before collection. Existing work keeps its current gates.
2. **Shadow evaluation** — collect independent challenges without adding an admission gate; this measures feasibility and gaps, but cannot establish the effect of blocking dispatch.
3. **Author-only baseline** — postpone the new review lane and capture the six dimensions plus outcomes first.

Default if no answer: none — policy implementation blocks until answered.

## Ground rules
- Preserve human gates and capability minima. Stop at implemented; a non-implementer verifies.
- If facts conflict with current source, record NEEDS_CONTEXT rather than guessing.
- Follow the ruled pilot policy; unknown evidence is never a passing result.

## Task
1. Present D2–D5 as a self-contained decision with costs and alternatives.
2. Record the human source, actor, time and selected options; never turn recommendations into rulings.
3. Specify pilot cohort, retention, spend ceiling, observation horizon and go/no-go criteria before results; update only the design sections affected.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `python3 -c "from pathlib import Path; s=Path('docs/streams/brief-quality/decisions.md').read_text(); assert all(x in s for x in ['D2','D3','D4','D5','source','actor','observation'])"` | exit 0; ruling record structure present; human independently confirms source and authority |
| 91 | check +dereference | `python3 -c "from pathlib import Path; assert Path('statusgen/briefv2.go').is_file(); assert 'parseBriefV2Keys' in Path('statusgen/briefv2.go').read_text(); assert Path('qualgen/telemetry/source.go').is_file()"` | exit 0; cited parser and telemetry seams resolve in this checkout; this does not prove document adequacy |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: human. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
