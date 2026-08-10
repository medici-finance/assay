---
brief: assay-product/01
title: Product brief — what Assay is, for whom, honestly
wave: 0
depends: []
unblocks: ["assay-product/03", "assay-product/05", "assay-product/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable session (human:<name>'s assay-product direction)
sources: ["[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "[I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md) + docs/streams/methodology/assay-growth-2026-07-09.md", "../reconciler/docs/product-brief.md (the D1 shape to mirror)", "../assay-toolkit/README.md (current positioning)", "docs/streams/methodology/red-team-2026-07-09.md (A5 scope-honesty + [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) copy constraints)", "freshness-checked 2026-07-10 @ 78200803"]
---

# Brief 01 — Product brief: what Assay is, for whom, honestly

**CROSS-REPO:** the deliverable lands in `../assay-toolkit/docs/product-brief.md` (its own
git repo — commit there, record the SHA in Evidence). This repo gets only the stream-row
update.

## Context
files: ../assay-toolkit/docs/product-brief.md (new); ../assay-toolkit/README.md (one added
pointer line only)
facts:
- Mirror the Plumb D1 shape (`../reconciler/docs/product-brief.md`): problem, who it's for,
  jobs-to-be-done, what it is / is not, differentiators, honest limits, open decisions.
- Positioning raw material: toolkit README's framing (briefs / registers / lifecycle /
  statusgen; "drift, missing evidence, and register tampering made machine-visible");
  design-spec §11 differentiators (roll-ups/queues/findings-log/intake had NO published
  counterpart in the 2026-07-08 prior-art sweep); the Plumb pairing line ("Plumb, identity
  held true — Assay, work held true").
- **Binding copy constraints ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md), red-team):** never claim measured ground truth or
  self-reporting-proof; never publish the ≈30:1 leverage / tier-experiment numbers without
  computation. The honest frame IS the differentiator — lead with the falsification story.
- Scope-honesty starting fact (A5): today's machinery serves an agent fleet with ONE
  accountable human, one repo, one git identity. Multi-person growth is [I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md)'s stream —
  the product brief states the current regime and points forward, it does not promise SME
  features that don't exist.
- **Open decisions to SURFACE (not decide):** what is monetized (OSS toolkit + paid what?
  — service, support, hosted board, marketplace per [I-27](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-admin-pod-as-deliberate-chokepoint-evidence-receipts-mischie.md)); Apache-2.0 is already chosen
  for the toolkit (LICENSE on disk) — note the BSL tension history (methodology/07) as
  resolved-by-that-choice unless human:<name> reopens it; "Assay" vs "Assay by Medici" branding.
- Audience for the doc: human:<name> + future collaborators/investors — same register as Plumb D1.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only (assay-toolkit commits are local; pushing that repo is human:<name>'s).
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `../assay-toolkit/docs/product-brief.md` per the D1 shape and facts above: problem
   statement (multi-agent work's predictable failure modes), personas (agent-fleet operator
   today; SME team lead per [I-19](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-grow-assay-into-a-multi-person-sme-system-adoption-growth-de.md) as the growth vector), jobs-to-be-done, product surface
   (toolkit + conventions + generated board), differentiators (from §11 sweep), honest
   limits ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) framing verbatim), and an "Open decisions" section listing every human call.
2. Add one pointer line to `../assay-toolkit/README.md` linking the doc.
3. Update this brief's stream-README row to `implemented`.

## Verify (executable — presence gates; prose quality is owned by the review gate)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f ../assay-toolkit/docs/product-brief.md && wc -w < ../assay-toolkit/docs/product-brief.md` | ≥1500 |
| 2 | `grep -ci "open decision" ../assay-toolkit/docs/product-brief.md` | ≥1 |
| 3 | `grep -c "machine-visible" ../assay-toolkit/docs/product-brief.md` | ≥1 (honest framing present) |
| 4 | `grep -ciE "30:1|thirty.to.one" ../assay-toolkit/docs/product-brief.md` | 0 (F-12 forbidden numbers absent) |
| 5 | `grep -c "product-brief" ../assay-toolkit/README.md` | ≥1 |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). Include the
     assay-toolkit commit SHA. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `e0c56c8d`; deliverable lands cross-repo in `assay-toolkit` at commit `f904419`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `test -f .../product-brief.md && wc -w` | 0 | 2456 words (≥1500) | 2026-07-10 | opus-verifier |
| 2 | `grep -ci "open decision"` | 0 | 2 (≥1) | 2026-07-10 | opus-verifier |
| 3 | `grep -c "machine-visible"` | 0 | 2 (≥1) | 2026-07-10 | opus-verifier |
| 4 | `grep -ciE "30:1\|thirty.to.one"` | 1 | 0 matches — forbidden numbers absent (exit 1 = no-match = PASS) | 2026-07-10 | opus-verifier |
| 5 | `grep -c "product-brief" README.md` | 0 | 1 (≥1) | 2026-07-10 | opus-verifier |
| 6 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (informational NOTICEs only) | 2026-07-10 | opus-verifier |

assay-toolkit commit: `f904419` ("docs: product brief (assay-product/01)"), clean working tree.

**VERIFY: PASS** — all six executable rows pass; deliverable committed in `assay-toolkit` (`f904419`). The [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) prose-constraint + "genuinely open decisions" quality checks are the model review gate's job, not this executable table.

## Review
Gate: model. Reviewer checks the [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) copy constraints held and that every decision in
"Open decisions" is genuinely open (not silently decided by the draft).
