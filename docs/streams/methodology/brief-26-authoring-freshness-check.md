---
brief: methodology/26
title: Authoring freshness check — verify a deliverable isn't already satisfied on main before queueing it
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([F-21](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-privacy-hardening-02-was-authored-against-an-already-satisfi.md) remediation batch)
sources: ["FINDINGS [F-21](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-privacy-hardening-02-was-authored-against-an-already-satisfi.md)", "FINDINGS [F-14](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-vite-8-x-rolldown-breaks-object-form-manualchunks-frontend-0.md) (same class)", "issue #156 (claim-aware Next-up — the pick-time sibling)"]
---

# Brief 26 — Authoring freshness check — verify a deliverable isn't already satisfied on main before queueing it

## Context
files: `../oit/.claude/skills/author-brief/SKILL.md` (in-repo wrapper; the user-level core at
`~/.claude/skills/author-brief/SKILL.md` is out-of-repo — update it in the same session
and note it in the PR, it cannot ride the commit), `tools/statusgen` (optional lint hook,
see Task 3)
facts:
- [F-21](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-privacy-hardening-02-was-authored-against-an-already-satisfi.md): privacy-hardening/02 was authored asking for a caller-token injection that had
  ALREADY been on main for six days (`06f0f010`, hardened by #134). Only the implementer
  catching it kept the PR honest; a weaker implementer would have re-written the fix and
  claimed a vuln closed. [F-14](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-vite-8-x-rolldown-breaks-object-form-manualchunks-frontend-0.md) is the same class (frontend/05 DoD stale vs the as-built).
- The failure is at AUTHORING time: nothing requires the author of a fix/security brief
  to check the current state of the target site before writing "add X".
- Issue #156 / claim-aware Next-up solves the parallel problem at PICK time (in-flight
  work); this brief covers ALREADY-LANDED work at authoring time. They compose.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add a mandatory pre-flight rule to the author-brief in-repo wrapper (and mirror to the
   user-level core, noted as out-of-repo): before authoring any brief whose deliverable is
   a fix/change to an existing site, CHECK the site's current state on fresh `origin/main`
   (read the code, `git log` the file) and record the check in the brief's `sources:` as
   `freshness-checked <YYYY-MM-DD> @ <short-sha>`.
2. The rule's test: if the deliverable is already satisfied, the brief is NOT authored —
   the finding/issue gets resolved or re-scoped instead.
3. Optional (only if cheap): a statusgen NOTICE when a brief created after this rule's
   merge date lacks a `freshness-checked` token in sources AND its stream is a hardening
   stream. If not cheap, skip — the skill rule is the deliverable; record the decision.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "freshness-checked" .claude/skills/author-brief/SKILL.md` | ≥1 (the rule + token format documented) |
| 2 | `statusgen --root . --lint; echo $?` | 0 |
| 3 | next authored brief set (first use after merge): its briefs' `sources:` carry `freshness-checked` tokens | present |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `0174b912`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `grep -c "freshness-checked" .claude/skills/author-brief/SKILL.md` | 0 | 2 (≥1) — rule + token format documented (SKILL.md:260,273) | 2026-07-12 | opus-verifier |
| 2 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-12 | opus-verifier |
| 3 | next authored brief set carries `freshness-checked` in `sources:` | — | present — ~75 briefs authored post-merge carry the token (methodology 28-39, all methodology-metrics, midnight-poc, desk-tools, issue-loop, reconciler-spinout, agentic-first, assay-*) — real first-use-after-merge adoption | 2026-07-12 | opus-verifier |

**VERIFY: PASS** — the authoring freshness-check rule + token format are in the project author-brief skill and adopted across ~75 post-merge briefs.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
