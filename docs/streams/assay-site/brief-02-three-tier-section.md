---
brief: assay-site/02
title: Three-tier open-core section — free / premium / on-prem, each pitch + who-it's-for + link
why: >-
  The open-core three-tier story is the page's lede (human:<name>, 2026-07-16) and the newest thing in
  the product, so it needs its own decided copy rather than being improvised at build time. A
  visitor must, in one scan, see the three postures (free toolkit + Desk Solo / hosted Desk
  Console / on-prem licensed Console), who each is for, and where to go next — with the
  boundary honest: the free tier is complete for one present person, the paid tiers own the
  hosted-control-surface scope.
wave: 1
depends: ["assay-site/01"]
unblocks: ["assay-site/04"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus session (assay-site authoring pass)
sources: ["docs/site-messaging.md (assay-site/01 — the three-tier frame this fills)", "docs/market-analysis.md §0 (open-core: free = documents-plus-a-linter; premium = Desk Console)", "docs/desk-console-design.md §2.0 (where the Console sits)", "docs/desk-solo-design.md §1,§4,§5 (FREE tier = toolkit + Desk Solo; what it deliberately does NOT get)", "docs/desk-console-saas.md §3 + §3.3 (SaaS delivery + the hosted-on-prem third posture, ruled 2026-07-16)", "docs/saas-platform-design.md §10 (which components ship on-prem)"]
---

# Brief 02 — Three-tier open-core section

## Context
files: docs/site-three-tier.md (NEW)
facts:
- CONTENT SPEC (not HTML): the copy + structure for the page's headline section; brief 04
  renders it. Slots into the IA position brief 01 assigned (hero → THIS → explainers → CTAs).
- THE THREE TIERS, exactly (name · one-line pitch · who-it's-for · where-it-links):
  1. **FREE — the toolkit + Desk Solo.** Pitch: the open-source (Apache-2.0) methodology —
     statusgen, briefs, registers, the lifecycle, the loop skills — plus **Desk Solo**, a
     documented Supacode cockpit so one person runs several loops on one machine with no
     service. Who: the solo agent-fleet operator (one accountable human, one repo). Link:
     the repo Quickstart + the Desk Solo setup page (desk-solo/03, WHEN it ships — until then
     link the repo/design, never a dead link).
  2. **PREMIUM — the hosted Desk Console (SaaS).** Pitch: a derived-read / human-write
     oversight service — deskd, its clients (web, menubar, Ask Assay), optional pods
     substrate — hosted by us; rolls the loops' escalations into one inbox so a blocked loop
     stops being invisible. Who: a team whose loops have become load-bearing and who won't
     babysit them. Link: a "talk to us" / waitlist CTA (design-stage; no signup flow claimed).
  3. **ON-PREM — the Console, licensed into your cluster.** Pitch: the same commercial
     Console binary deployed in the customer's own infrastructure under the commercial
     licence — their cluster, their credentials, their App installs; "us as adversary"
     vanishes. Who: buyers who won't accept SaaS trust surfaces (regulated / data-resident).
     Link: same commercial contact CTA.
- THE HONEST BOUNDARY (state it, don't hide it): the FREE tier is *complete for one person who
  is present* (no stored index, no push, no team gates — desk-solo §4); the paid tiers own the
  hosted-control-surface scope (market-analysis §0). The upgrade is "a login, not a migration"
  — the free tier already wrote the records the Console derives from (desk-console-saas §5).
  DO NOT claim shipped Console/Solo features — both are design-stage; describe, link honestly.
- OPEN-CORE licensing line (accurate): free tier = Apache-2.0 in this repo; the Console + deskd
  are commercial, non-open-source, in a private repo (F-02; desk-console-design §2.0).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write docs/site-three-tier.md: the three tiers as three parallel blocks, each with the four
   fields (name / one-line pitch / who-it's-for / link-target). Lead with FREE.
2. Include the honest-boundary line (free = complete-for-one-present-person; paid = hosted
   surface) and the open-core licensing line.
3. Every "link" is a target description (repo Quickstart, setup page WHEN it ships, commercial
   contact) — no fabricated URLs or claimed signup flows.
4. Add the page to the README docs/ index.

## Verify (executable — no prose-only DoD items)
Prose deliverable: PRESENCE gate (the three tiers + required fields exist); prose quality is
the human review gate's call.
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/site-three-tier.md; echo $?` | `0` |
| 2 | `grep -Eic -e "desk solo" -e "desk console" -e "on-prem" docs/site-three-tier.md` | ≥ `3` (all three tiers present) |
| 3 | `grep -ic "who" docs/site-three-tier.md` | ≥ `3` (a who-it's-for per tier) |
| 4 | `grep -Eic -e "apache" -e "commercial" docs/site-three-tier.md` | ≥ `2` (open-core licensing stated: free = Apache, Console = commercial) |
| 5 | `grep -Eic -e "login, not a migration" -e "complete for one" docs/site-three-tier.md` | ≥ `1` (the honest boundary / upgrade line present) |
| 6 | `grep -q "site-three-tier.md" README.md; echo $?` | `0` (linked in the docs/ index) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
The human review gate owns claim-quality; the grep rows only prove the elements exist.
