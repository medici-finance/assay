---
brief: assay:assay:measured-status:02
title: "Derive MinCorpus for the learned riskscore model against its 15-feature events-per-variable floor, or record the rationale — and pin it with a test"
why: >-
  The learned defect-prediction model trusts itself over the heuristic fallback once the
  labeled corpus reaches MinCorpus=40, but standard events-per-variable guidance for a
  15-feature model would put that floor near 150. Either 40 is a deliberately conservative
  under-threshold (safe because the heuristic absorbs the gap) or it is an un-derived guess
  that graduates the model too early; either way the value must be computed from the feature
  count, not typed as a bare literal.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1171]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
sources:
  - "#1171 — MinCorpus=40 (learned.go) not derived against the 15-feature model's events-per-variable guidance"
  - "qualgen/riskscore/learned.go — Config.MinCorpus and DefaultConfig (MinCorpus:40); the under-corpus three-state threshold below which the score is heuristic-only"
  - "qualgen/riskscore/features.go — the 15-element feature vector MinCorpus must be derived against"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — MinCorpus:40 is a bare literal in DefaultConfig with no events-per-variable derivation in code or the quality/15 brief"
exec-tier: strong
exec-tier-why: derives a model-graduation threshold from the feature vector — an error graduates the learned model early or never, and it compounds through every score consumed downstream
domain: complicated
value: med
consumers:
  - "qualgen/riskscore/learned.go (Train under-corpus gate, len(examples) < cfg.MinCorpus): fixed-here"
version: 1
id: 81c2396f-a641-49ce-ae17-9683f990dfa2
---

# Brief 02 — Derive MinCorpus against the feature vector

## Context
files:
- `qualgen/riskscore/learned.go` — `Config.MinCorpus`, `DefaultConfig()`.
- `qualgen/riskscore/features.go` — the feature vector whose length is the derivation input.
- `qualgen/riskscore/learned_test.go` — add the derivation-pinning test.
facts:
- feature vector length today = 15 (`features.go`); MinCorpus today = 40 (`DefaultConfig`).
- events-per-variable (EPV) is the standard floor: `MinCorpus >= EPV * featureCount`; a
  conservative logistic-model EPV is ~10, giving ~150 for 15 features.
- below MinCorpus the model does not train and the score is emitted heuristic-only with a
  could-not-learn status (a three-state read), so an under-set floor over-trusts the learned
  model and an over-set one keeps the heuristic longer — the value is a real tradeoff, not cosmetic.
- `qualgen/riskscore` is buildable/testable from fixtures without a live corpus.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Derive MinCorpus from the feature count. Preferred: make MinCorpus a function of
   `len(features)` and a named `EventsPerVariable` constant (`MinCorpus = EventsPerVariable *
   featureCount`), so the floor tracks the vector automatically when a feature is added or
   removed. Choose and DOCUMENT the EPV value with its rationale.
2. If the team's decision is that 40 is a deliberately conservative under-threshold justified
   by the heuristic fallback (rather than the EPV floor), then instead record that rationale
   as a `// Derivation:` block on `MinCorpus` naming why the EPV floor is intentionally not
   applied and what condition would raise it — and add the same pinning test against the
   documented value. Do not leave a bare literal either way.
3. Add a test `TestMinCorpusDerivedFromFeatureCount` that recomputes the floor from
   `len(features)` and the documented EPV (or asserts the documented conservative value and
   its stated invariant), and fails if `MinCorpus` drifts from its derivation.
4. Add a FLOW test `TestMinCorpusGovernsLearnedSwitch` that trains against a corpus one below
   the derived floor (asserts heuristic-only / could-not-learn) and one at the floor (asserts
   the learned model trains), proving the derived value actually governs the heuristic↔learned
   switch end to end — MinCorpus is a shared default consumed by the Train under-corpus gate.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `go test ./qualgen/riskscore/ -run TestMinCorpusDerivedFromFeatureCount -count=1` | exit 0; output contains "ok" | check +dereference |
| 2 | `go test ./qualgen/riskscore/ -run TestMinCorpusGovernsLearnedSwitch -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (a corpus one below the derived floor stays heuristic-only/could-not-learn and one at the floor trains — the value actually governs the switch end to end) | check +flow |
| 3 | `go build ./qualgen/riskscore/` | exit 0 | check |
| 4 | `grep -q 'Derivation:' qualgen/riskscore/learned.go` | exit 0 (a written derivation exists next to the value) | check |
| 5 | `statusgen --root . --consumers --brief assay:assay:measured-status:02` | exit 0; output does not contain "DISPROVED" (the fixed-here consumer routing is corroborated, not contradicted) | check |

## Evidence
<!-- appended at implementation time by a non-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
