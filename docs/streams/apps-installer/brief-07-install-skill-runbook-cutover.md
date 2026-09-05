---
brief: apps-installer/07
title: Install skill + adoption runbook cutover — `deskapps` replaces the hand runbook, tiers documented
why: >-
  The tool is worth nothing if the adoption path still tells adopters to click through eight steps
  per App by hand. The install skill's human hand-off and the runbook's App primitive both need to
  say: run `deskapps init`, click Create and Install when the page asks, and read the proof — and
  the tiers need one honest page that says what each one automates and what stays the adopter's.
wave: 4
depends: ["apps-installer/01", "apps-installer/03", "apps-installer/04", "apps-installer/06"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./README.md — the three-tier table (what runs without the adopter, what stays theirs); ./design.md §1, §6."
  - "`plugins/assay/skills/install/SKILL.md` — the 'hand the human their half' section (create + install the Apps and store their PEMs at the config-home, choose the roster values …) and the NEVER-autonomous list (Reviewer GitHub App creation / installation)."
  - "`docs/adopting-assay.md` — `PRIMITIVE: setup-reviewer-app — HUMAN-GATED`, the required duty set table, the roster section (`ASSAY_TRUSTED_BOT_SLUGS=[role=]slug:<bot-user-id>`), the Human post-install checklist."
  - "apps-installer/01 — the `<ROLE>_APP` binding the runbook must now describe; apps-installer/02 — the `deskapps init` flow and the `apps.env` records the runbook describes; apps-installer/03 — the roster block deskapps writes."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — the skill and the runbook describe hand creation only; neither mentions tiers or a manifest flow."
exec-tier: strong
exec-tier-why: >-
  Question (b) — the skill, the runbook and the tool must agree on which acts stay human (Create,
  Install, avatar drop) and which the tool performs; a runbook that says "the tool creates the App"
  overclaims, one that keeps the hand steps under-claims, and only a cross-read of all three catches
  either.
consumers:
  - "plugins/assay/skills/install/SKILL.md: fixed-here"
  - "docs/adopting-assay.md: fixed-here (App primitive, roster section, post-install checklist)"
  - "plugins/assay/skills/adopt/SKILL.md: fixed-here (one pointer line, if it names the App runbook)"
  - "tools/desk/README.md: fixed-here (Operator reference § App credentials mentions deskapps)"
  - "docs/how-assay-works.md: out-of-scope (methodology, not install mechanics)"
---

# Brief 07 — Install skill and runbook cutover

## Context
files:
- `plugins/assay/skills/install/SKILL.md`, `plugins/assay/skills/adopt/SKILL.md`.
- `docs/adopting-assay.md` — § PRIMITIVE setup-reviewer-app (renamed `setup-apps`), the roster
  section, the Human post-install checklist; new § "Choosing a tier".
- `tools/desk/README.md` § Operator reference → App credentials.

facts:
- What stays human, verbatim in all three places: clicking **Create GitHub App** on GitHub's page,
  clicking **Install** and choosing repositories, dropping the avatar. Everything else — naming,
  permissions, key storage, records, roster block, proof — is `deskapps`.
- The skill's NEVER-autonomous list keeps "App creation / installation" and gains the sentence: the
  skill may RUN `deskapps init --tier <t> --org <o> --no-browser` and hand the person the URL; it
  never clicks for them and never claims the run is done before `deskapps status` exits 0.
- Tier copy: the README's table, reproduced (not paraphrased) in the runbook's new section, with
  the sentence *the tiers change who does the work, never the method*.
- The runbook's roster section now shows the block `deskapps` writes and says when it writes it
  (only when `ASSAY_TRUSTED_BOT_SLUGS` is absent) and when it prints it instead.
- The runbook's duty-set table is unchanged; the sentence "provision all three for the reviewer
  App, and for any other role App you create" gains "— `deskapps` manifests carry exactly this set".
- Honest claims: no page may say the tool "creates" or "installs" an App. It *prepares*,
  *converts*, *records*, *proves*.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never remove the hand walkthrough; move it to an appendix titled "Without deskapps" for GHES or
  air-gapped adopters.

## Task
1. Runbook: new § "Choosing a tier" (table + who-does-what); `setup-apps` primitive built around
   `deskapps init` with the three human clicks called out; roster section updated; hand walkthrough
   to the appendix; post-install checklist items for Apps collapse to "run deskapps, click when
   asked, `deskapps status` exits 0".
2. Skill: the hand-off section and the NEVER list per facts; the skill runs `deskapps` in
   `--no-browser` mode and prints the URL and the console lines; the final report quotes
   `deskapps status`.
3. `adopt` skill: one pointer line if it references the App runbook.
4. README operator reference: one paragraph.
5. Cross-read: a test script `tools/desk/scripts/claims-check.sh` (planned) greps the three documents for the
   forbidden overclaims (`deskapps creates`, `deskapps installs`, `tool creates the App`) and exits
   1 on a hit; wire nothing into CI here (that is a lint decision), but make it runnable.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -cE -e '^### PRIMITIVE: setup-apps' -e '^## Choosing a tier' -e 'Without deskapps' docs/adopting-assay.md` | 3 |
| 2 | `grep -cE -e 'deskapps init' plugins/assay/skills/install/SKILL.md docs/adopting-assay.md` | ≥ 2 (one per file) |
| 3 | `grep -cE -e 'click.*Create GitHub App' -e 'click.*Install' -e 'avatar' plugins/assay/skills/install/SKILL.md` | ≥ 3 |
| 4 | `grep -cE -e 'the tiers change who does the work, never the method' docs/adopting-assay.md` | 1 |
| 5 | `sh tools/desk/scripts/claims-check.sh; echo $?` | last line `0` |
| 6 | `printf 'deskapps creates the App\n' > /tmp/bad.md && sh tools/desk/scripts/claims-check.sh /tmp/bad.md; echo $?` | last line `1` (the check fires on a planted overclaim) |
| 7 | `grep -cE -e 'ASSAY_TRUSTED_BOT_SLUGS' -e 'only when' docs/adopting-assay.md` | ≥ 2 |
| 8 | `grep -cE -e 'deskapps' tools/desk/README.md` | ≥ 1 |
| 9 | `statusgen --root . --lint` | exit 0 |
| 10 | `statusgen --root . --consumers --brief apps-installer/07` | exit 0 (routing claims corroborated against the diff) |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Reviewer reads the skill,
the runbook and `deskapps --help` side by side and confirms the human-act list is identical in all
three.
