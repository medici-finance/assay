---
brief: methodology-metrics/17
title: Verify-desk lands results as verifiers return — continuous drain, not wave-end batches
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [282]
schema: brief-v1
authored: 2026-07-10 by Fable session (issue #282)
sources: ["issue #282 (19/24 PASSes in hand, 0 on main; board read as stalled)", "memory: verify-desk-operating-mode (human:<name> 2026-07-09: commit Evidence/status straight to main as it goes — the batching contradicts an existing direction)", "methodology-metrics/10 (queue-is-the-constraint alarm this latency inflates)", "methodology-metrics/08 (claim-aware pattern referenced, deliberately deferred here)", "freshness-checked 2026-07-10 @ 53012778"]
why: >-
  The verify-desk drains the Awaiting queue but batches every implemented→verified flip +
  Evidence write to wave-end: a 24-brief wave sat at 19 PASSes-in-hand with 0 landed, other
  sessions re-reported "stuck" briefs, and the verification-debt alarm read worse than
  reality. Merged→verified latency is the metric the desk exists to improve; batch-landing
  inflates it by pure reporting delay — and human:<name>'s 2026-07-09 operating direction was
  already land-as-you-go.
---

# Brief 17 — Verify-desk lands results as verifiers return

## Context
files: tools/statusgen/trend.go + the Awaiting-view emitter (merged→verified age column —
locate via `git grep -n "Awaiting verification" tools/statusgen`) + tests;
docs/streams/methodology-metrics/README.md (row)
out-of-repo files: ~/.claude/skills/verify-desk/SKILL.md (the landing-discipline amendment —
staged as a diff in the PR body, applied live as the LAST step before `implemented`,
committed in the ~/.claude stopgap repo; rule 7: max ONE out-of-repo brief in flight,
serialize against methodology/30 and /32)
facts:
- Skill amendment (the fix's core, from #282 + the standing verify-desk-operating-mode
  direction): the desk COMMITS each flip + Evidence as its verifier returns — per brief, or
  per stream-group at most; never accumulate a wave in a scratch buffer. Retry the
  race loop on push conflicts (existing skill rule); STATUS.md stays excluded. Cadence:
  a continuous drain loop, not episodic waves — the queue should never show items whose
  result is already in hand for longer than one landing cycle.
- statusgen half: the Awaiting-verification view gains a merged→verified AGE per row
  (merge time from the status historian mm/01 / git; "how long has this row been
  awaiting"), so latency is visible per brief, not just as queue depth. Rendering only —
  no score inputs ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) boundary).
- Deliberately OUT OF SCOPE: a "verify in progress" claim marker (issue #282's fix 3,
  mirroring mm/08). With land-as-you-go the in-flight window shrinks from wave-length to
  one verifier's runtime; add the marker only if latency stays visible after this lands —
  file a follow-up brief then, citing measured ages.
- The desk's landing commits go straight to main per human:<name>'s standing direction
  (verify-desk-operating-mode, 2026-07-09) — this brief does not change that; it removes
  the batching that contradicted it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Amend verify-desk SKILL.md per facts: land-per-verifier-return (per-stream-group max),
   continuous-drain cadence, and the explicit anti-pattern ("a PASS in hand and not on
   main within one landing cycle is a defect"). Stage as diff, apply last, stopgap commit.
2. statusgen: add the merged→verified age to the Awaiting view (TDD; fixture with a known
   historian timestamp → expected age rendering; missing historian data → "—", never a
   guess).
3. README row; lint green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci "as its verifier returns\|land-as-you-go" ~/.claude/skills/verify-desk/SKILL.md` | ≥1 (after the staged diff is applied) |
| 2 | `grep -ni -e "wave-end" -e "entire wave" ~/.claude/skills/verify-desk/SKILL.md \|\| true` | no hit in prescriptive text — every hit printed (with its line number, so the judgement is legible) sits inside the anti-pattern warning. Note: this row is exit-neutralised on purpose — "prescriptive vs anti-pattern" is not decidable by a count, so the row hands the reader the located hits instead of a number that cannot discriminate |
| 3 | `(rc=0; for t in Awaiting Age; do out=$(go test ./statusgen/ -count=1 -run "$t" -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf '%s' "$out" \| grep -q -- '--- PASS'; } \|\| { echo "MISSING-OR-FAIL $t"; rc=1; }; done; exit $rc)` | exit 0, prints nothing — both named test groups EXIST and pass; includes the known-timestamp and missing-data cases. Exit status is captured (`tr=$?`) and asserted BEFORE the `--- PASS` check, so a FAILING test in the group also goes red — the previous pipeline form discarded `go test`'s status and passed on a red suite |
| 4 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 5 | `statusgen --root . --lint; echo $?` | 0 |
| 6 | `git -C ~/.claude log --oneline -1 -- skills/verify-desk/SKILL.md` | one commit dated the implementation day |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

Verifier run (independent, non-implementer — opus-verifier, merged main `2a8cd673`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `grep -ci "as its verifier returns\|land-as-you-go" ~/.claude/skills/verify-desk/SKILL.md` | 0 | 2 (≥1) | 2026-07-10 | opus-verifier |
| 2 | `grep -ci "wave-end\|entire wave" ~/.claude/skills/verify-desk/SKILL.md` | 0 | 1 — the sole match is the bolded anti-pattern warning (SKILL.md:78-79) | 2026-07-10 | opus-verifier |
| 3 | `go test ./tools/statusgen/ -run 'Awaiting|Age' -v` | 0 | 5 age tests pass (Awaiting/RenderAge/EmitAwaitingAgeColumn + rate-flood + heading-counts); known-timestamp + missing-data→"—" cases present | 2026-07-10 | opus-verifier |
| 4 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | both exit 0 | 2026-07-10 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-10 | opus-verifier |
| 6 | `git -C ~/.claude log --oneline -1 -- skills/verify-desk/SKILL.md` | 0 | `db071d7 verify-desk: land each result as its verifier returns (mm/17, #282)`, dated 2026-07-10 | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — land-as-you-go rule added to the verify-desk skill with wave-end hoarding named as the anti-pattern; awaiting-age column tests green. (Row 3 brief text uses grep-BRE `\|` which Go RE2 reads literally; run with RE2 alternation the intended tests pass — cosmetic brief-text bug, substance met.)

## Review
Gate: model. Reviewer confirms: (a) the skill diff keeps the existing race-loop/STATUS.md
exclusions intact, (b) the age column is render-only (no [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) score input), (c) the claim
marker stayed out of scope with the follow-up condition stated.
