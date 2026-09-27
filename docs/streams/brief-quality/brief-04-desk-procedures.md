---
brief: "assay:assay:brief-quality:04"
title: "Rewrite authoring and review procedures around assessed contracts"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 3
depends: ["brief-quality/03"]
unblocks: ["brief-quality/06"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
consumers:
  - "plugins/assay/skills/author-brief/SKILL.md: follow-up brief-quality/04"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up brief-quality/04"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up brief-quality/04"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up brief-quality/04"
  - "plugins/assay/references/brief-quality.md: follow-up brief-quality/04"
issues: []
schema: "brief-v2"
id: "fbf1cff2-374b-491c-a08e-a9a1e702babc"
version: 1
authored: "2026-09-26 by the authoring session recorded in commit 6a7d90b97"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 04 — Rewrite authoring and review procedures around assessed contracts

## Context

files:
- `plugins/assay/skills/author-brief/SKILL.md`
- `plugins/assay/skills/pr-review-desk/SKILL.md`
- `plugins/assay/skills/worker-desk/SKILL.md`
- `plugins/assay/skills/verify-desk/SKILL.md`
- `plugins/assay/references/brief-quality.md (NEW)`

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
1. Teach the author to assess six dimensions with evidence and retain responsibility for initial DoD.
2. Teach reviewer the two-stage unanchored failure analysis, then criteria challenge and adjudication.
3. Define amendment handling, original/approved/completed snapshots and independent post-merge verification, using the ratified policy.
4. Use one neutral reference for shared vocabulary; no operator identity or provider assumptions; include a worked ambiguous brief and its corrected contract.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `git diff --check` | exit 0; patch whitespace clean |
| 2 | check | `python3 -c "from pathlib import Path; p=Path('plugins/assay/references/brief-quality.md'); assert p.is_file(); assert all('brief-quality' in Path('plugins/assay/skills',s,'SKILL.md').read_text() for s in ['author-brief','pr-review-desk','worker-desk','verify-desk'])"` | exit 0; shared reference wired; independent reviewer performs scenario walkthrough, not grep-quality certification |
| 90 | check | `statusgen --root . --consumers --brief brief-quality/04` | exit 0 on the implementation branch after dispositions are updated to match the actual diff; inherited/out-of-scope claims remain explicitly unchecked |
| 91 | check +dereference | `python3 -c "from pathlib import Path; assert Path('statusgen/briefv2.go').is_file(); assert 'parseBriefV2Keys' in Path('statusgen/briefv2.go').read_text(); assert Path('qualgen/telemetry/source.go').is_file()"` | exit 0; cited parser and telemetry seams resolve in this checkout; this does not prove document adequacy |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: model. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
