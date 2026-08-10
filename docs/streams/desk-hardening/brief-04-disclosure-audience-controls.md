---
brief: desk-hardening/04
title: Disclosure & audience controls — leak + candour scan for public artifacts
why: >
  We ship public artifacts, and our review gate checks whether a claim is TRUE — never whether a
  true statement should be PUBLIC. A leak is TRUE: source comments on a public site named an
  internal brief id, a private repo, the approver, and an internal approval rule; the truthfulness
  gate doesn't just miss it, it green-lights it. The adjacent axis is candour: outward-facing
  repos carry candid internal assessments of partners, a funder's ecosystem, and NAMED individuals
  who never consented. Neither is caught by checking claims against reality, because both are
  accurate. This adds the missing axes — disclosure and audience — as review + mechanical gates.
wave: 1
depends: ["desk-hardening/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [41, 42]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#41 (leak-detection: gate checks TRUE, never whether a true statement should be PUBLIC)"
  - "assay-toolkit#42 (candour about partners and NAMED individuals in outward-facing repos)"
  - "inherits desk-hardening/01 three-state rule (report checked-clean / checked-leaks / could-not-check)"
exec-tier: strong
exec-tier-why: "the KEEP/REMOVE distinction (scoping caveat vs internal candour) is a judgment a careless sweep inverts; deleting the wrong one removes exactly the caveats that keep us truthful."
---

# Brief 04 — Disclosure & audience controls

## Context
files:
- `[oit]` `../oit/.claude/skills/pr-review-desk/SKILL.md` (reviewer-prompt additions: leak-check + audience-check)
- `[toolkit]` a mechanical scanner `.github/workflows/disclosure-scan.yml` (planned) (or a script consumed by CI)
- `[toolkit]` optionally a publish-time comment-strip step for shipped HTML/CSS
out-of-repo files: none
facts:
- the source file IS the artifact — GitHub Pages serves it byte-for-byte, no build/minify step stands between a maintainer comment and the public reader (#41)
- leak checklist (ranked by what an outsider gains): people/approvers · private repos, docs, endpoints · internal machinery (brief/stream ids `[a-z-]+/[0-9]{2}`, `F-NN`/`I-NN`, statusgen vocabulary — leaks most freely because it feels like jargon) · undecided/unreleased plans · shipping placeholders (`TODO|TBD|FIXME`) · credentials (already covered by the secrets check — the floor, not the ceiling)
- **the KEEP/REMOVE line is load-bearing (#42):** KEEP honest caveats that SCOPE our own claim ("not yet built", "as of <date>", "we assert nothing about X") — these are why our claims are defensible; REMOVE internal candour that JUDGES a third party or a named individual. Test: "would I say this to the person it is about, in the room, with my name on it?"
- **highest-severity #42 instance:** a second-founder candidate assessment naming real people sits in the Night Sky *submission* directory — relocate out of `night-sky/` regardless of what else is decided
- pdftotext line-wraps (see desk-hardening/01/03): flatten before grepping PDF text; a text tool cannot see link targets — extract, don't `strings`
- three-state reporting is mandatory: checked-clean / checked-leaks (or checked-found) / could-not-check — never collapse the third

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Reviewer-prompt additions (do this first — costs nothing).** A **Leak check** and an
   **Audience check** section for any review of an outward-facing artifact (site, deck, PDF,
   public README, OG image, grant/partner submission). Both state explicitly: *a statement being
   TRUE is not a defence — this axis is orthogonal to the accuracy check*. The Audience check
   MUST carry the KEEP/REMOVE distinction verbatim so a reviewer never strips a scoping caveat.
   Both report three-state.
2. **Mechanical scanner (CI).** Grep shipped artifacts (HTML/CSS/JS/SVG comments, extracted PDF
   text with whitespace flattened, image metadata via `exiftool`) for: internal repo names,
   `docs/streams/` paths, brief-id pattern `[a-z-]+/[0-9]{2}`, `F-[0-9]+`/`I-[0-9]+`, known
   internal doc filenames, `TODO|TBD|FIXME`, a candour-tell list (`honest take|honest
   assessment|candidly|frankly|the real story`), plus a names list.
   **Decision to record (candidate approaches):** (a) leak-class hits (ids, repos, credentials)
   FAIL the build; candour-class hits WARN (the caveat/candour distinction needs a human/model,
   an auto-stripper would delete the wrong thing) — recommended; (b) all WARN; (c) all FAIL.
3. **(Optional, the only durable fix for #41) publish-time strip step** — minify comments out of
   shipped HTML/CSS so authors write freely and the notes never ship.
4. **Handle the names list carefully** — it references real people; keep it in the scanner
   config, not in a public artifact, and note this in the scanner docs.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | run the scanner against a fixture containing `reconciler-spinout/24` and a private-repo mention | FAILS (or WARNs per chosen policy) naming both hits — positive control |
| 2 | run the scanner against a clean fixture | exit 0 / no findings |
| 3 | `grep -ci 'TRUE is not a defence\|orthogonal' <pr-review-desk SKILL.md>` | exit 0; ≥ 1 (leak axis stated) |
| 4 | `grep -ci 'KEEP\|scope\|not yet built\|do not strip' <pr-review-desk SKILL.md>` | exit 0; ≥ 1 (KEEP/REMOVE distinction preserved) |
| 5 | run the scanner against a fixture with `<!-- honest assessment: ... -->` | candour tell reported (WARN or FAIL per policy) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date. MUST confirm the scanner does NOT flag a KEEP-class
scoping caveat ("as of 2026-07-14", "we assert nothing about X") as a leak — a scanner that eats
the caveats defeats the reason we ship them.
