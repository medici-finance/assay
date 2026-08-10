---
stream: qual
status: active
priority: P1
track: platform
---

# Quality Stream

Fixture stream for methodology-metrics brief-04 (point-quality rendering).
Every brief file here is legacy (no frontmatter) — none is brief-v1
schema-opted — so the hard checks (checkBriefFiles, attributionProblems)
are exempt on every row, and only the new soft quality-flag / `--lint`
NOTICE logic is exercised.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Backed done](./brief-01-backed.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 02 | [Unbacked done - empty evidence](./brief-02-unbacked-evidence.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 03 | [Unbacked done - bare verified cell](./brief-03-unbacked-verified.md) | 0 | S | done | not-dated | 2026-07-09 human:ian |
| 04 | [Legacy grandfathered done](./brief-04-legacy-grandfathered.md) | 0 | S | done | grandfathered | grandfathered |
| 05 | [Backed verified](./brief-05-backed-verified.md) | 0 | S | verified | 2026-07-08 sonnet-verifier | — |
| 06 | [Unbacked verified - empty evidence](./brief-06-unbacked-verified.md) | 0 | S | verified | 2026-07-08 sonnet-verifier | — |
| 07 | [Todo out of scope](./brief-07-todo.md) | 0 | S | todo | — | — |
| 08 | [Unbacked done - implementer-only evidence](./brief-08-implementer-evidence.md) | 0 | S | done | 2026-07-08 sonnet-verifier | 2026-07-09 human:ian |
| 09 | [Backed done - bare human reviewer](./brief-09-human-reviewed.md) | 0 | S | done | 2026-07-08 sonnet-verifier | human:ian |
