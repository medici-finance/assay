---
brief: desk-hardening/03
title: Source↔render drift + stale-normative-doc detection
why: >
  A document is corrected; the rendered artifact — the PDF/HTML actually sent to a funder — is
  not regenerated, so the artifact still carries the retracted claim. Reviewers diff sources; a
  stale binary render produces no diff, so every instance passed review, including one in a
  grant-submission PDF. The inverse also happens (render ahead of source), and a third variant
  is worse: a normative audit that was correct on Sunday kept instructing a deletion on Monday
  after the world moved, and cost us our strongest evidence. The fix is a check that source and
  render AGREE (both directions), plus dating the world a normative doc audited.
wave: 1
depends: ["desk-hardening/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [46, 39, 50]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#46 (source↔render drift — 3 instances in one day, invisible to diff review)"
  - "assay-toolkit#39 (near-duplicate of #46; corrected source, shipped-false artifact — resolve as dup of #46)"
  - "assay-toolkit#50 (normative docs go stale silently — a CUT verdict expired and deleted true evidence)"
  - "inherits desk-hardening/01 three-state rule (a clean pdftotext grep on unflattened output is NOT evidence of absence)"
exec-tier: strong
exec-tier-why: "correctness depends on tool behaviour a subtle error survives (pdftotext line-wrap false-negatives) and a both-directions comparison, not a habit of re-rendering."
---

# Brief 03 — Source↔render drift + stale-normative-doc detection

## Context
files:
- `[toolkit]` a CI workflow `.github/workflows/render-drift.yml` (planned) (or a check folded into an existing workflow)
- `[toolkit]` `docs/brief-rules.md` (the "assert on the ARTIFACT, not the source" Verify rule)
- `[toolkit]`/`[repos]` a normative-doc convention doc (audited-SHA header + CUT-verdict expiry)
out-of-repo files: none
facts:
- byte-comparison of PDFs fails (non-deterministic output) — compare **extracted text** (`pdftotext -layout` then flatten whitespace) of a fresh re-render vs the committed artifact; fail on difference (#46)
- **pdftotext line-wraps** — a grep returning zero on unflattened output is NOT evidence of absence and has already produced a false all-clear; flatten whitespace first, always (#39, and desk-hardening/01)
- drift goes BOTH ways (#46 instance 2: render ahead of source) — so "always re-render" is not a complete fix; the deliverable is a check that they AGREE
- #50 is the inverse-and-worse case: a *deleted true claim* produces no error, no contradiction, no failing check; a normative doc (audit issuing CUT/CORRECT/KEEP verdicts) must carry the SHA of the world it audited, so it can be flagged when the audited repo moves past it; a CUT verdict asserts a negative ("no such artifact exists") and expires like any dated negative
- cross-artifact divergence (#50): an application cited a circuit as closing evidence while its own attached deck never mentioned it — nothing checked the two agreed because each was reviewed alone

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Render-drift CI check.** For every `X.md`/`X.html` with a committed `X.pdf`: re-render to a
   temp file, `pdftotext` both, flatten whitespace, fail if the extracted text differs. Catches
   both directions; needs no human to remember. (Candidate scope: gate only submission
   directories first if a repo-wide render is too heavy — record the choice.)
2. **Verify-table rule (`docs/brief-rules.md`):** any brief producing a rendered artifact must
   assert on the ARTIFACT (`pdftotext <pdf> | tr -s '[:space:]' ' ' | grep -v '<retracted>'`),
   never on the source. Flag existing source-asserting rows as satisfiable-while-broken.
3. **Normative-doc convention.** A document that issues verdicts other agents act on carries an
   `audited: <repo>@<sha>` header; a mechanical check flags it when the audited repo advances
   past that SHA. A CUT/negative verdict gets a date + a re-check trigger.
4. **Cross-artifact consistency (candidate approaches):** (a) a pre-submission checklist row —
   "does any sibling artifact in this package cite evidence this one omits?"; (b) a mechanical
   claim-reconciliation over a submission directory; recommend (a) now, (b) as follow-up.
5. Resolve **#39 as a duplicate of #46** (same class, same evidence week); scope its unique
   pdftotext-flatten caveat into deliverable 1/2.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | run the render-drift check against a repo with a deliberately-stale committed PDF | check FAILS naming the drifted file (positive control — proves it sees drift) |
| 2 | run the same check when source and render agree | exit 0 |
| 3 | `grep -ci 'pdftotext\|flatten\|tr -s' <the check + docs/brief-rules.md>` | exit 0; ≥ 1 (whitespace-flatten caveat present) |
| 4 | `grep -ci 'audited:\|@<sha>\|world it audited\|expire' <normative-doc convention doc>` | exit 0; ≥ 1 |
| 5 | `cd status* && go run . --root .. --lint; echo $?` | exit 0 |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date. MUST confirm Verify row 1 actually fails on a
stale render (the instrument sees the defect) — a drift check that never goes red is this
stream's own dominant defect.
