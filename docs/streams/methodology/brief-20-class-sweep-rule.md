---
brief: methodology/20
title: Fix-briefs sweep the defect class — authoring rule + reviewer questions
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by Fable session (assay-review-1)
sources: ["docs/assay-review-1/README.md (B-03)", "issue #147 (parse-error-returns-empty-success: a third sibling site missed by the brief that closed the class, a fourth named)", "issue #104 (DAML-Time parsing: C3 fixed in prices.go, sibling live in policy/resolver_canton.go)", "author-brief shared-value/consumers rule (the sibling discipline this extends)"]
---

# Brief 20 — Fix-briefs sweep the defect class

## Context
files: .claude/skills/author-brief/SKILL.md (project layer); docs/streams/methodology/README.md
(conventions); a recorded delta for the user-level pr-review-desk skill (see methodology/22 —
until that lands, user-level skill edits are a written delta the human applies)
facts:
- The 2026-07-08 review sweep's own meta-finding — "finds individual issues accurately but fails at
  sweeping a pattern to its other sites" — recursed into the remediation within a day: #147 (the
  H2/H3 fix landed while a third parse-error-as-success site stayed live, a fourth named) and #104
  (the C3 DAML-Time fix landed while the same parse bug sat in a file the sweep never opened).
- Both classes were grep-enumerable in minutes. The gap is procedural: nothing in brief authoring
  or PR review asks "is this an instance of a class, and where are the siblings?"
- The author-brief skill already has exactly this discipline for shared VALUES (enumerate
  consumers, route each, verify the flow). Defect CLASSES need the same rule.
- Reviewer half: the pr-review-desk dispatch template lists what every reviewer prompt must carry;
  it does not currently include class-sweep or real-path-test questions.

## Ground rules
- NEVER push to main / trigger workflows / run mutating kubectl. Feature-branch push + draft PR per
  the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) loop is the sanctioned flow; leave other commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add a **class-sweep rule** to the project author-brief skill, parallel to the shared-value rule:
   when a brief fixes an instance of a mechanical/greppable defect class (examples: a parse error
   returned as empty success; a timestamp-format mismatch; an unchecked type assertion), its
   Context MUST carry the class enumeration — the literal grep/search used, every matched site,
   each routed *fixed-in-this-brief / follow-up brief `stream/NN` / out-of-scope (why)* — and its
   Verify table MUST carry a class-enumeration row (the grep, with the expected routed count).
   An unlisted sibling is how #147/#104 happened; say so in the rule with both issue numbers.
2. Add the same rule (3-4 lines) to the methodology README conventions so non-brief fix work
   (issue-driven patches) inherits it.
3. Record the reviewer-side delta for the pr-review-desk dispatch template — two questions every
   fix-PR reviewer must answer: "did this fix sweep the pattern to every sibling site (what was the
   grep)?" and "does the new test exercise the real path, not a mock?" Write the delta into this
   brief's Evidence dir or the brief body; applying it to the user-level skill file is
   methodology/22's move (or the human's, whichever lands first).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "class-sweep\|defect class" .claude/skills/author-brief/SKILL.md` | ≥2 (rule present in the project skill) |
| 2 | `grep -c "147\|104" .claude/skills/author-brief/SKILL.md` | ≥1 (incident-anchored, per house style) |
| 3 | `grep -ci "class" docs/streams/methodology/README.md` | ≥1 more than before this brief (convention added) |
| 4 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `2a8cd673`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `grep -c "class-sweep\|defect class" .claude/skills/author-brief/SKILL.md` | 0 | 2 (≥2) — rule present in project skill | 2026-07-10 | opus-verifier |
| 2 | `grep -c "147\|104" .claude/skills/author-brief/SKILL.md` | 0 | 3 (≥1) — incident-anchored | 2026-07-10 | opus-verifier |
| 3 | `grep -ci "class" docs/streams/methodology/README.md` | 0 | 7 — new Class-sweep section (README §171-178) points at the skill rule | 2026-07-10 | opus-verifier |
| 4 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — class-sweep rule present in the project author-brief skill, incident-anchored, and surfaced in the stream README.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
