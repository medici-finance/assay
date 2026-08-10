---
brief: assay-site/07
title: Domain wiring — assay.guide host choice + records (execution is human:<name>'s)
why: >-
  assay.guide is registered but not wired (Cloudflare, clean slate — naming-clearance §9.1), so
  the finished page has nowhere to serve from. This brief closes the last gap on roadmap 2.1/D5:
  it PICKS the static host and writes the exact records + any in-repo artifacts, so publication
  becomes a short, unambiguous set of clicks. It stops before executing — wiring DNS and serving
  the page (and thus the first public use of the mark) is human:<name>'s act, per naming-clearance §9.7.
wave: 5
depends: ["assay-site/06"]
unblocks: []
effort: S
gate: human
gate-why: >-
  Serving the page at assay.guide is the product's first public exposure AND the first public use
  of the mark — the mark is clear-with-constraints pending human:<name>'s go/no-go + counsel (naming-clearance
  §5, §8.1). Both DNS execution and publish are human:<name>'s.
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus session (assay-site authoring pass)
sources: ["docs/naming-clearance.md §9.1 (domain state: registered, clean slate), §9.2 (assay.guide wiring options A/B), §8 + §9.7 (BLOCKED-ON-IAN)", "docs/product-roadmap.md 2.1/D5 (domain wiring = the site's hard-dependency / publish gate)", "web/site/index.html (assay-site/04+05 — the artifact to serve)", "docs/streams/assay-site/honest-claims-review.md (assay-site/06 — copy must pass before publish)"]
---

# Brief 07 — Domain wiring

## Context
files: docs/site-domain-wiring.md (NEW), web/site/CNAME (NEW — only if host = GitHub Pages)
facts:
- GATE = HUMAN (risk: customer = yes). Serving the page at assay.guide is the product's first
  public exposure AND the first public use of the mark — the mark is `clear-with-constraints`
  pending human:<name>'s go/no-go + counsel (naming-clearance §5, §8.1). Both the DNS execution and the
  publish are human:<name>'s. This brief PREPARES and STOPS.
- DOMAIN STATE (naming-clearance §9.1, verified 2026-07-14): assay.guide is registered on the
  project's Cloudflare account, NS gemma/rene.ns.cloudflare.com, clean slate — no A/AAAA/CNAME,
  nothing served. Registrar + authoritative DNS are already Cloudflare, so records are created
  in the Cloudflare dashboard for the zone.
- HOST OPTIONS to choose from (naming-clearance §9.2, enumerated there — do not re-derive):
  **Option A — Cloudflare Pages** (recommended; lowest friction since registrar+NS are already
  Cloudflare, and where the Plumb site is headed): attach a Pages project, add custom domains
  assay.guide + www, Cloudflare auto-creates the flattened-apex CNAME, TLS automatic, add a
  www↔apex redirect. **Option B — GitHub Pages**: apex A → the four 185.199.10x.153 records,
  apex AAAA → the four 2606:50c0:800x::153 records, www CNAME → <org>.github.io, add a CNAME
  file to the served dir, records DNS-only (grey-cloud) so GitHub issues the cert, Enforce HTTPS.
- DELIVERABLE the model produces (stopping at implemented): (1) docs/site-domain-wiring.md — the
  chosen host + the exact record list / click-path for it (lifted from §9.2) + a BLOCKED-ON-IAN
  section naming every step that is human:<name>'s; (2) IF the choice is GitHub Pages, the in-repo
  web/site/CNAME file containing `assay.guide`. No DNS is created, no Pages project attached,
  nothing published.
- PRECONDITION recorded (not a Verify row — can't be checked from the repo): publish only after
  brief 06's honest-claims review is green AND human:<name>'s mark go/no-go is yes. State both in the
  BLOCKED-ON-IAN section.
- NO static-site email/MX (naming-clearance §9.2 "common to all"); no secrets; no defensive-
  domain registration (that is human:<name>'s separate call, §9.7).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- NEVER create DNS records, attach a Pages project, or publish anything — every execution step
  is human:<name>'s (naming-clearance §9.7). This brief writes the plan and the in-repo artifact only.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write docs/site-domain-wiring.md: pick the host (A Cloudflare Pages recommended, or B GitHub
   Pages), give the exact record list / click-path for that choice (from naming-clearance §9.2),
   and a BLOCKED-ON-IAN section listing every human:<name> step (create records / attach Pages / Enforce
   HTTPS; the mark go/no-go; the publish) and the precondition (brief 06 green + mark go/no-go).
2. If host = GitHub Pages, add web/site/CNAME containing exactly `assay.guide`.
3. Add the wiring doc to the README docs/ index.

## Verify (executable — no prose-only DoD items)
Prose/config deliverable: PRESENCE gate (the plan + BLOCKED-ON-IAN section exist; the chosen
host's records are enumerated). The actual DNS/publish is BLOCKED-ON-IAN and is NOT a Verify
row — it cannot be checked from the repo and must not be performed.
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/site-domain-wiring.md; echo $?` | `0` |
| 2 | `grep -Eic -e "cloudflare pages" -e "github pages" docs/site-domain-wiring.md` | ≥ `1` (a host is chosen) |
| 3 | `grep -ic "assay.guide" docs/site-domain-wiring.md` | ≥ `1` (the zone/domain is named) |
| 4 | `grep -Eic -e "blocked-on-ian" -e "ian" docs/site-domain-wiring.md` | ≥ `1` (the human-gated execution steps are recorded) |
| 5 | `grep -Eic -e "go/no-go" -e "mark" docs/site-domain-wiring.md` | ≥ `1` (the publish precondition — human:<name>'s mark go/no-go — is stated) |
| 6 | `grep -q "site-domain-wiring.md" README.md; echo $?` | `0` (linked in the docs/ index) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: human (from frontmatter — risk: customer = yes). A human (human:<name>, or a named reviewer on
his behalf) records the verdict in the stream README table; the human gate is MANDATORY here.
The actual publish (DNS + serve) is human:<name>'s separate act and is not part of reaching `done`.
