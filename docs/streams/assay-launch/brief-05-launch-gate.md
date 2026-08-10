---
brief: assay-launch/05
title: Launch readiness + go-live gate (human, irreversible)
why: >-
  Publishing assay.guide is an outward, irreversible act — the site and its claims get cached
  and indexed the moment it goes live, and it publicly asserts the Assay mark. Someone has to
  own the single moment where "all the moving parts are green" becomes "we launched." This brief
  is that gate: it converges the readiness check into one decision human:<name> makes, so the launch
  happens on evidence, not vibes, and no half-built page ships by accident.
wave: 3
depends: ["assay-launch/01", "assay-launch/02", "assay-launch/03", "assay-launch/04", "assay-launch/06", "assay-launch/07", "methodology/09", "methodology/10", "methodology/11"]
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus session (human:<name>'s publish-the-methodology direction)
sources: ["human:<name> 2026-07-17: 'we need a website up and running' (the go-live he performs)", "assay-launch/06 (the assay-site repo + landing) + 01,02,03,04,07 (all deliverables must be green first)", "methodology/09,10,11 publication gate (articles publish only on lived data + human:<name>'s explicit go)", "docs/streams/intake/2026-07-11-outward-claims-gate.md (outward public claims need the human gate)", "human:<name> 2026-07-17: 'leave the SaaS/Enterprise versions out of the website for now' (final-cut checks the exclusion held)", "F-08/F-12 honest-framing (final review-cut before anything is public)"]
gate-why: >-
  irreversible + customer-facing: this is the actual publish of assay.guide. The site, its
  copy, the metrics numbers, and the PDF all become publicly cached and indexed and cannot be
  un-said; the site publicly claims the Assay mark ahead of/alongside assay-product/04's
  deferred formal trademark clearance. human:<name> is confirming: (1) every deliverable is green and
  honest-framing holds on every surface; (2) the hosting home + DNS are his choice; (3) the
  three articles' publication (methodology/09–11) is his explicit go; (4) he accepts the mark
  is asserted publicly. The implementer prepares and STOPS at `implemented` / BLOCKED-ON-IAN —
  making the repo public, hosting, DNS, and the publish are human:<name>'s acts alone.
exec-tier: strong
exec-tier-why: (a)/(b) launch-readiness adjudication across every deliverable + the outward-claims judgment compounds if wrong.
---

# Brief 05 — Launch readiness + go-live gate

**Coordination terminal.** This brief holds the single human go-live gate for the whole
stream. Cross-repo: the publish touches `../assay-site` (Cloudflare Pages project + first
deploy) + `assay.guide` DNS; no code lands here beyond the stream-row + the readiness record.

## Context
files: docs/streams/assay-launch/README.md (readiness record + status row); the
`statusgen --launch` output (assay-launch/04) is the machine readiness check
facts:
- **Everything must be green first.** `assay-launch/06` (assay-site repo — migrated site +
  landing + hardening), `/01` (pages), `/02` (PDF), `/03` (metrics page), `/04` (--launch view),
  `/07` (auto-publish pipeline) all `done`; `methodology/09,10,11` at whatever publication state
  human:<name>'s go requires.
  `statusgen --launch` prints `READY` only when the transitive dep closure is done.
- **The publish is human:<name>'s, and it is the risky act.** Connecting the `assay-site` repo to a
  Cloudflare Pages project (06 stands the repo + `predeploy-check.sh` up; hosting trade-offs are
  06's deliverable), pointing `assay.guide` DNS, and the first live deploy are all human:<name>'s. This
  brief does NOT do any of them.
- **Outward-claims final cut (intake `2026-07-11-outward-claims-gate`):** a last pass across the
  landing, the page suite, the metrics page, and the PDF before anything is public — honest
  framing holds, the SaaS/Enterprise exclusion held (no Desk Console / pricing copy leaked in),
  and `predeploy-check.sh` passes on every shipped artifact. Any violation blocks the gate.
- **Trademark:** the site asserts the Assay mark; assay-product/04 deferred FORMAL clearance
  (2026-07-14). human:<name> is accepting that posture at launch — surface it, do not decide it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. NEVER make a repo public, host, or
  change DNS. Prepare the readiness record and STOP at `implemented`, reporting BLOCKED-ON-IAN.
- The Reviewed cell at `done` MUST be `human:ian` (gate:human — statusgen enforces this).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Run `go run ./tools/statusgen --root . --launch` and record the readiness verdict + the
   dependency table in the README (a "Launch readiness" section). If BLOCKED, list what is not
   done and STOP — the gate cannot open.
2. When READY: do the outward-claims final pass across all four surfaces (landing, pages,
   metrics, PDF); record the pass in the readiness section.
3. Assemble the go-live checklist for human:<name>: hosting options (from 06's table), the Cloudflare
   Pages connect step, the `assay.guide` DNS step, the article-publication go, and the trademark
   posture note. Report BLOCKED-ON-IAN with the checklist.
4. STOP at `implemented`. human:<name> performs the launch; `verified`/`done` (with `human:ian`) follow
   the live publish.

## Verify (executable — readiness gate; the publish itself is human:<name>'s, verified post-launch)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --launch 2>&1 \| grep -ciE -e READY -e BLOCKED` | ≥1 (readiness verdict recorded) |
| 2 | `grep -ci "launch readiness" docs/streams/assay-launch/README.md` | ≥1 (readiness section present) |
| 3 | `grep -riE -e "is ground truth" -e "measured ground truth[^.]" -e "30:1" ../assay-site/*.html ../assay-site/assets/*.pdf 2>/dev/null \| wc -l` | 0 (outward-claims final cut clean) |
| 4 | `grep -riE -e "desk console" -e "\benterprise\b" -e "\bsaas\b" -e "pricing" ../assay-site/*.html \| wc -l` | 0 (SaaS/Enterprise exclusion held) |
| 5 | `cd ../assay-site && ./predeploy-check.sh; echo $?` | 0 (leak guard clean on shipped artifacts) |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- one row per Verify item, filled by a non-implementer. The go-live confirmation
     (site reachable at assay.guide) is recorded here by human:<name> post-launch. -->

## Review
Gate: **human** (mandatory — irreversible + customer-facing). human:<name> records the go-live decision;
the `done` Reviewed cell is `human:ian`. A model sign-off cannot close this brief.
