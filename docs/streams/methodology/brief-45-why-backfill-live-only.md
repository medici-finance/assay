---
brief: methodology/45
title: 'why: backfill (phase 2) — add why: to the 32 LIVE briefs missing it; grandfather the 75 closed'
why: >-
  brief-27 made why: required for new briefs but left a NOTICE-level backfill for the ~107 legacy
  briefs that predate it. Of those, 75 are done/verified (backfilling motivation onto a closed brief is
  archaeology) and only 32 are live (todo/in-progress/implemented) — where a missing why: means active
  work no non-engineer can justify. This closes the loop brief-27 opened, mirroring the proven
  gate-why 24→25 sequence, but scoped to the work that's still moving.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus desk session (human:<name> directive)
sources:
  - "methodology/27 (brief-27-why-field.md) Task item 4: 'follow-up brief(s) per the gate-why pattern — active streams first; done/archived briefs exempt' (decision recorded, not executed)"
  - "methodology/24 + /25 — the proven backfill→hard-flip precedent this copies"
  - "statusgen --lint 2026-07-17: 107 'has no why:' NOTICEs; triage fanout split them 75 closed / 32 live"
  - "freshness-checked 2026-07-17 @ origin/main: `go run ./tools/statusgen --root . --lint 2>&1 | grep -c 'has no why:'` reports the live total; brieffile.go:537-546 comment confirms the intended active-first/exempt-closed backfill"

---

# Brief 45 — why: backfill (phase 2), live briefs only

## Context
files: the `why:` frontmatter line of the **32 live briefs** listed below (add one line after
`sources:`); no status/risk/Evidence/body/README changes.

facts:
- **Scope is LIVE only.** brief-27 already decided "done/archived exempt" — do NOT mirror gate-why's
  blanket backfill (that backfilled done briefs too, only because the set was 26). Here the set is 107
  with 75 closed; backfilling closed briefs is low-value and the exemption is deliberate.
- **The 32 live briefs, by stream** (todo/in-progress/implemented):
  - midnight-poc: 01, 02, 03, 04, 05, 06, 09, 10
  - daml-hardening: 03, 04, 05, 06, 07, 08, 09
  - reconciler-spinout: 13, 14, 15, 16, 21
  - assay-product: 02, 04, 05, 06
  - ledger-hardening: 11, 13, 15, 18
  - agentic-first: 04, 11
  - methodology: 09, 11
  (Re-derive the exact set at execution time: `go run ./tools/statusgen --root . --lint 2>&1 | grep 'has no why:'` intersected with each brief's README status — main moves.)
- **The why: must be DERIVED per brief, not templated.** Each `why:` is a compression of what that
  brief's own `title` + `sources:` + Context/Task already say it's for — one to three lines a
  non-engineer could justify the work from. A plausible-but-wrong rationale is a real defect (the
  gate-why backfill, brief-24, enforced the same bar). Review per-stream batch before merge.
- **gate-why is already fully done** (24→25 landed; 0 current missing-gate-why) — this brief is why:
  only.

## Task
1. For each of the 32 live briefs, read the brief and add one `why:` line derived from its own
   title/sources/task. Edit only that line. Group the work by stream for fan-out (midnight-poc 8,
   daml-hardening 7, reconciler-spinout 5, assay-product 4, ledger-hardening 4, agentic-first 2,
   methodology 2).
2. Leave the 75 done/verified briefs untouched (grandfathered).

## Follow-up (NOT this brief — author after this lands)
**Phase-3 hard lint, status-scoped.** Because closed briefs are exempt, the hard flip CANNOT be
blanket (brief-25's approach) or main reddens on 75 grandfathered briefs. A follow-up brief changes the
`why:` `notice(...)` in `../assay-toolkit/statusgen/brieffile.go` (~L545) to `add(...)` **only when
`row.Status ∉ {done, verified, archived}`** — catching all new/live work (every new brief enters as
`todo`) while never failing CI over closed archaeology. Gate it on THIS brief driving the live-status
missing-why count to 0 first. (statusgen already has `row.Status` in the lint, so this is feasible.)

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `for f in $(statusgen --root . --lint 2>&1 \| grep 'has no why:' \| grep -oE 'docs/streams/[^ :]+\.md'); do grep -L '^why:' "$f"; done \| wc -l` (live set only) | `0` — no LIVE brief still missing why: |
| 2 | `git diff origin/main -- docs/streams/ \| grep -E '^\+' \| grep -vE '^\+why:' \| grep -vE '^\+\+\+'` | empty — only `why:` lines added |
| 3 | `statusgen --root . --lint` | exit 0; the 75 closed-brief why: NOTICEs remain (grandfathered), live ones gone |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
