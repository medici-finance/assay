---
brief: methodology/12
title: Model-tier gate for brief authoring — heavy model required, cheap models must escalate
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (process desk)
sources: ["process-desk observation 2026-07-08 (user: authoring needs heavy-duty models)", "arXiv:2505.20182 (task-dependent strong/weak tiering)", "spec §11 adopt-5", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)"]
---

# Brief 12 — Model-tier gate for brief authoring

## Context
files: ~/.claude/skills/author-brief/SKILL.md (user-level core); .claude/skills/author-brief/SKILL.md (project wrapper — pointer line only)
facts:
- Rationale: authoring is where errors COMPOUND (wrong gate, wrong deps, unverified critical-path head misdirect every downstream implementer + review cycle); implementation is where errors are CAUGHT (gates). Spend the strong model on the compounding end.
- A session knows its own model identity (system prompt); the gate is an instruction, not a programmatic check: "authoring/decomposition is design-tier work — if you are a fast/cheap-tier model, STOP, report which model you are, and tell the user to switch sessions or escalate; do not author anyway."
- Mirror rule for dispatchers: never delegate brief authoring/decomposition to a cheap-tier subagent (matches SDD model-selection guidance: architecture/design = most capable available).
- The gate must be near the TOP of the user-level skill (before the methodology), stated as a hard stop with the "do not author anyway" loophole closed — cheap models under pressure will otherwise rationalize proceeding.
- Skill edits follow superpowers:writing-skills discipline: cite the baseline failure mode (this brief's rationale is the documented baseline — no synthetic re-run needed for a one-rule addition), verify with one subagent test: a cheap-tier subagent given the updated skill + an authoring request must REFUSE and escalate rather than author.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add the model-tier gate section to the user-level skill (top, after frontmatter/intro): the stop-and-escalate rule + the dispatcher mirror rule + one-line rationale (errors compound at authoring).
2. Add one pointer line in the project wrapper so repo readers see it without opening the user-level file.
3. Verify per writing-skills: dispatch one haiku subagent with the updated skill text + "author a brief for X" — it must refuse/escalate, not author. Save the transcript as evidence (artifact required — Task-15 lesson).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci "design-tier\|do not author anyway" ~/.claude/skills/author-brief/SKILL.md` | ≥2 (amended 2026-07-08: `-i` — original was case-sensitive against capitalized prose; caught by verifier) |
| 2 | `grep -c "model-tier\|design-tier" .claude/skills/author-brief/SKILL.md` | ≥1 (wrapper pointer) |
| 3 | `test -s docs/streams/methodology/evidence/brief-12-verify-output.md && grep -ci "escalat\|refus\|stronger" docs/streams/methodology/evidence/brief-12-verify-output.md` | ≥1 (cheap model refused, artifact saved) |
| 4 | `statusgen --root . --check` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `grep -ci "design-tier\|do not author anyway" ~/.claude/skills/author-brief/SKILL.md` | 0 | `2` (gate section line 16ff: "design-tier work" + "**Do not author anyway**" loophole-closure) | 2026-07-08 | sonnet verifier (non-implementer) |
| 2 | `grep -c "model-tier\|design-tier" .claude/skills/author-brief/SKILL.md` | 0 | `1` (wrapper pointer restating stop-and-escalate + dispatcher rule, commit 5ef912ac) | 2026-07-08 | sonnet verifier (non-implementer) |
| 3 | `test -s docs/streams/methodology/evidence/brief-12-verify-output.md && grep -ci "escalat\|refus\|stronger" docs/streams/methodology/evidence/brief-12-verify-output.md` | 0 | `6` — haiku-tier spawn refused + escalated ("STOP … switch to a strong-tier session"), did not author | 2026-07-08 | sonnet verifier (non-implementer) |
| 4 | `go run ./tools/statusgen --root . --check` | 0 | no output (clean check) | 2026-07-08 | sonnet verifier (non-implementer) |
| 5 (suppl.) | read `docs/streams/methodology/evidence/brief-12-verify-output.md` § "Supplementary test 2" | — | dispatcher-rule refusal: via-deepseek wrapper declined to route authoring to its economy-tier backend, cited the gate, DeepSeek never invoked — both enforcement points (worker + dispatcher) evidenced | 2026-07-08 | sonnet verifier (non-implementer) |

Note (artifact validity, adjudicated by verifier): the test-1 transcript self-identifies as
"Claude Haiku 4.5" while the resumed agent's environment later reported differently. The refusal
occurred in the original haiku-tier spawn — the dispatch-time tier, not the model's self-reported
string, is dispositive — so the artifact stands as valid evidence. Verify row 1 was amended
2026-07-08 (`-i`, commit d504c382) after the first verification pass caught a case-sensitivity
mismatch; gate text was semantically complete throughout.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Human gate is MANDATORY when any risk answer is yes.
