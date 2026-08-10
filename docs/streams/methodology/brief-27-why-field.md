---
brief: methodology/27
title: Every brief carries a `why:` — human-justifiable motivation, not just the what
why: "A board of 94 briefs that state only WHAT to do is opaque to non-implementers — a stakeholder (or human:<name> in six months) cannot judge which work matters or why. Adding a why: field gives every brief a one-to-three-line justification that anyone can read, the same way gate-why made risk rationales visible."
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction, 2026-07-10)
sources: ["human:<name> 2026-07-10: 'frontend/05 is only the what — a human has no idea why it is being done, so we can't justify it'", "gate-why design docs/superpowers/specs/2026-07-09-gate-why-rationale-design.md (the pattern this generalizes)"]
---

# Brief 27 — Every brief carries a `why:` — human-justifiable motivation, not just the what

## Context
files: `../assay-toolkit/statusgen/brieffile.go` (frontmatter parse + `checkBriefFiles` lint),
`../oit/.claude/skills/author-brief/SKILL.md` (in-repo wrapper; user-level core mirrored
out-of-repo), the brief-v1 template embedded in both skills
facts:
- Exemplar of the gap (human:<name>): frontend/05 "code splitting" states WHAT (named vendor
  chunks, lazy routes) but nowhere WHY — a human reading the board cannot justify the
  work. The missing line is one sentence: "all 22 pages are eagerly imported into a
  single JS chunk, so every user pays a multi-MB first load; splitting cuts the initial
  payload ~5×."
- `gate-why` (merged, #184) solved this for RISK rationale on the gated subset. This
  brief generalizes the pattern: `why:` is the VALUE rationale, on every brief. The two
  are distinct fields answering different questions (why is it risky vs why do it at all);
  `sources:` is provenance (where it came from), not justification.
- Proven rollout pattern from gate-why: known-key parse → NOTICE-level lint → backfill →
  hard-error flip. Reuse it; do not invent a new sequence.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. statusgen: parse `why:` as a known frontmatter key (string, 1-3 lines); NOTICE-level
   lint when a brief lacks it (all briefs, not just risk-gated). Code comment marks the
   later hard-error flip (post-backfill, own follow-up brief mirroring methodology/25).
2. brief-v1 template + both author-brief skills: `why:` required for every NEW brief —
   one to three lines a non-engineer could read and justify the work from. Include the
   frontend/05 exemplar above as the quality bar in the skill text.
3. Surface it: render `why:` in the verify-gate issue card (under the gate-why block) and
   anywhere else the brief is summarized for a human, if a summary site exists cheaply
   (STATUS.md rows stay compact — do NOT bloat the table).
4. Backfill strategy decision recorded (not executed here): follow-up brief(s) per the
   gate-why pattern — active streams first; done/archived briefs exempt.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes tests: brief without `why:` → NOTICE (not error); with it → no notice |
| 2 | `statusgen --root . --lint; echo $?` | 0 (existing briefs produce NOTICEs only) |
| 3 | `grep -c 'why:' <author-brief template section>` | template carries the field + exemplar |
| 4 | verify-gate card render test: card body contains the why text when the brief has one | test passes |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `4a0ea1f9`). All 4 rows RUN, none UNRUN.

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok github.com/medici/statusgen`. Named tests present + passing: `TestWhyNotice/without_why`, `TestWhyNotice/with_why`, `TestRenderVerifyWhy` (3 subtests) | 2026-07-13 | opus-verifier |
| 2 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0, NOTICEs only; `105` × `brief <id> has no why:` NOTICE lines. No PROBLEM from the why rule | 2026-07-13 | opus-verifier |
| 3 | `grep -n 'why:' <author-brief SKILL.md>` | 0 | user-level template (canonical, :72): `why: <prose>  # REQUIRED for every NEW brief` + the frontend/05 exemplar verbatim. In-repo wrapper (:109-113) states the rule + NOTICE semantics + card rendering | 2026-07-13 | opus-verifier |
| 4 | `go test ./tools/statusgen/... -run 'TestRenderVerifyWhy' -v` | 0 | PASS — present → blockquote above Gate reason; absent → none; both → gate-why then why. Code path `../assay-toolkit/statusgen/verifyissues.go:175` | 2026-07-13 | opus-verifier |

**Falsification attempt — the rule demonstrably fires** (mutations made in a throwaway worktree, file restored, tree clean before removal):

| Probe | Mutation | Observed |
|---|---|---|
| A | `why:` line deleted | `NOTICE: … brief methodology/27 has no why: …` — exit 0 |
| B | `why: "   "` (whitespace only) | same NOTICE fires (`strings.TrimSpace`, `brieffile.go:531`) — exit 0 |
| C | `why:` as a YAML list (wrong type) | `PROBLEM: … why must be a string` — exit **1** |

This is not a rule that passes its own tests and rejects nothing. Exit 0 on A/B is the **specified** behaviour, not a miss — Task item 1 says NOTICE-level and Verify row 2 explicitly expects exit 0 ("existing briefs produce NOTICEs only"); hard-error is deliberately deferred (`brieffile.go:523-530`, PHASE 3).

**Grandfathering:** applied prospectively. **105 briefs currently lack a `why:`**; 72 carry one. No backfill was executed — Task item 4 scopes it out ("decision recorded, not executed here"), and the recorded strategy (`brieffile.go:528-530`) is "active-stream briefs first; done/archived exempt; follow-up brief(s) mirror the methodology/24→25 sequence."

**VERIFY: PASS**

*Carried forward, not blocking:* the follow-up briefs that strategy promises **do not exist on the board yet**. methodology/24 (gate-why backfill) and 25 (hard lint) are both `done`; there is no equivalent `why:`-backfill or `why:`-hard-lint brief anywhere in `docs/streams/`. Until they are authored the NOTICE stays advisory and 105 briefs stay un-backfilled — the pattern is set up but the loop is not closed.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
