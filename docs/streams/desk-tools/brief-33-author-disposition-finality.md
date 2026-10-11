---
brief: assay:assay:desk-tools:33
title: "An author's explicit disposition on a finding is final for the reviewer — accept it or escalate it once, never spend a round re-stating it"
why: >-
  The review lanes have machinery for findings the code answers (resolve at the current
  head, with current-head evidence) and machinery for findings nothing answers (the
  three-round cap and its derived arbiter packet). Between them sits a case with no rule:
  the AUTHOR ANSWERS — an explicit disposition on the finding class such as "won't fix",
  "fixed a different way", "tracked separately", or an evidence-backed dispute — and the
  code may or may not have changed. Today nothing binds the reviewer's next move: a
  reviewer can spend a fresh round re-stating the finding against the disposition, or hold
  it blocking purely because no code changed, reopening an argument the author already
  closed. That is a round wasted on both sides and a review that reads as not listening.
  The missing rule is small and has now been derived independently outside this
  methodology: langgraph-harness's public review workflow (MIT, cited below) states it as
  principle 8 — the author's disposition "is final. Do not reopen it, reply insisting
  otherwise, or treat it as still-blocking because no matching code change occurred." The
  same convergence that produced desk-tools/31's coverage contract: the failure class is
  real. This brief writes the rule into the reviewer's prompt and the review desk's skill
  body, in this methodology's own vocabulary: a disposition is an ANSWER, so the reviewer
  answers it exactly one of two ways — accept (the finding class resolves, recorded with
  the disposition's words as the resolution evidence) or escalate (ONE arbiter packet
  quoting the disposition, under the existing cap-and-arbiter machinery, which is already
  derived, never reviewer-asserted). A re-statement round is neither.
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-10-11 by a house authoring session at the driver's direction (external mechanic surveyed from vrajpal-jhala/langgraph-harness, public, MIT)
sources:
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md — the finding-class register, round caps, class continuity, and the derived arbiter packet (line ~341: the round cap files ONE arbiter packet and holds the class). All of it governs findings and rounds; no clause governs an author's explicit disposition on a finding. The prompt-audit clause's 'pending disposition' (line ~464) is a different concept — a fleet-wide audit baseline's own accept/decline — and must not be conflated."
  - "CLAUDE.md-level worker doctrine already carries the author's half ('fix every finding with follow-up commits, reply-with-evidence to disputes') — this brief is the reviewer's half: what a reply-with-evidence disposition obliges the reviewer to do."
  - "vrajpal-jhala/langgraph-harness backend/src/skills/gitlab-mcp/code-review.md principle 8 (public, MIT, surveyed 2026-10-11) — the independently derived statement of the rule: an author's disposition on a thread is final; a thread the author explicitly answered does not withhold approval regardless of a matching code change."
  - "freshness-checked 2026-10-11 @ 69ca08c69 (origin/main) — no clause in either lane's cut or the skill body states disposition finality; grep for 'disposition' in the review prompt matches only the prompt-audit baseline concept; brief-33 is the next free number after brief-32 (PR #2535)."
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument: one rule stated in the reviewer prompt's
  shared clauses (both lane cuts), mirrored in one skill paragraph, and reconciled with the
  two neighboring rules (resolve-with-evidence, round-cap arbiter) so the boundaries,
  not just the words, are right.
consumers:
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here (the disposition-finality clause: an explicit author disposition on a finding class ends the reviewer's argument in accept-or-escalate; a re-statement round is neither; the disposition's words are resolution evidence when accepted)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here (one neutral paragraph in the reviewer's bar: same rule, pointing at the prompt clause)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: out-of-scope (the worker's reply-with-evidence duty already exists; this brief changes nothing on the author side)"
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md: fixed-here only if the clause's natural home is the shared clauses both lane cuts read — the worker picks after reading both; if it lands there, the review-prompt entry is the pointer, not a second copy"
version: 1
id: 9a41e7d2-64c3-4f8a-b1e0-3c7d52a8f964
---

# Brief 33 — Author-disposition finality

## Context

Three rules govern a finding's life today: it resolves only at the current head with
current-head evidence; a fresh class starts a round counter; a class that reaches the
three-round cap in unchanged code goes advisory or to ONE derived arbiter packet. What
happens when the AUTHOR answers the finding — explicitly, in words, with or without a code
change — is unstated. The reviewer's legal moves today include the wasteful one: re-state
the finding next round as though the disposition were silence.

**The rule this brief adds, stated once:** an explicit author disposition on a finding
class (won't fix / fixed a different way / tracked separately / disputed with evidence) is
an ANSWER to that class. The reviewer's next move is exactly one of:

1. **Accept** — the class resolves. The resolution evidence is the disposition's own words
   (with its head), not a code read; "resolved-by-author-disposition" is a legitimate
   resolution. The verdict that accepts carries the disposition's substance, so a later
   reader sees why the class closed.
2. **Escalate** — ONE arbiter packet under the existing machinery, quoting the disposition
   and the finding, when the reviewer believes the disposition is wrong in a way that
   matters (safety-consequence, a control weakened, a spec violated). The packet is filed
   the way the round-cap packet is filed — derived by the existing path, never
   reviewer-asserted as a fresh finding round.

A re-statement round is neither. A disposition is not re-opened by a later reviewer round
absent NEW evidence that the disposed concern materially regressed — a code change alone is
not new evidence about the disposition, though it can be new evidence about the code.

**Boundaries (so the worker does not over-broaden the rule):**

- **Applies to an EXPLICIT disposition.** Silence, a bare "done" with no answer to the
  finding, or a fix attempt that does not address the finding is not a disposition — the
  ordinary rules (inadequate-fix reply, round cap) govern those.
- **Never for safety-consequence findings.** A disposition on a safety-consequence finding
  escalates, full stop; it is never accepted by the reviewer alone.
- **The disposition is the author's, not a third party's.** A comment from an untrusted or
  unblessed login blesses nothing (trust gate); a disposition carries authority only from
  the change's author identity.
- **Nothing here changes the flip gates.** A resolved class still needs the checks, the
  security lane, and the merge-time re-check; disposition finality governs the reviewer's
  argument, not the pipeline's gates.

## Task

**Task 1 — the clause.** Add the disposition-finality clause to the reviewer's shared
clauses so BOTH lane cuts carry it (if `common-clauses.md` is the shared home both cuts
read, it lands there and the review prompt points at it; otherwise it lands in the review
prompt directly — the worker reads both files and picks the home with no second copy).
The clause states the rule, the two legal moves, the resolution-evidence shape
(`resolved-by-author-disposition` + the disposition's words + its head), the
safety-consequence carve-out, the explicit-only boundary, and the no-reopen rule with its
new-evidence exception.

**Task 2 — the skill paragraph.** One neutral paragraph in the pr-review-desk skill's
reviewer's bar stating the rule and pointing at the clause's home.

**Task 3 — changelog fragment.** changelog/desk-tools-33.md (planned), one bullet.

files:
- `tools/desk/cmd/deskdispatch/references/common-clauses.md` or `tools/desk/cmd/deskdispatch/references/review-prompt.md` — Task 1 (ONE home, no second copy)
- `plugins/assay/skills/pr-review-desk/SKILL.md` — Task 2
- changelog/desk-tools-33.md (planned) — Task 3

## Verify (executable — no prose-only DoD items)

Rows 1–3 are discriminating: each FAILS against today's tree (at 69ca08c69) and passes only
once the Task lands. Row 4 passes today and must still pass. Run from the repo root.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -rq 'resolved-by-author-disposition' tools/desk/cmd/deskdispatch/references/ && grep -rq 'disposition' tools/desk/cmd/deskdispatch/references/common-clauses.md tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | prints `ok`: the clause exists with its named resolution form. FAILS today: the string exists in neither reference. |
| 2 | check | `grep -rq 'safety-consequence' tools/desk/cmd/deskdispatch/references/common-clauses.md tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -rqi 're-state\|restate' tools/desk/cmd/deskdispatch/references/common-clauses.md tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | prints `ok`: the carve-out and the no-restate rule are both present in the clause's home. FAILS today: neither formulation exists. |
| 3 | check | `grep -q 'disposition' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | prints `ok`: the skill's reviewer's bar carries the rule's paragraph. FAILS today: the word appears 0 times in the skill. |
| 4 | check +neighbour | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt33-sg" . && "${TMPDIR:-/tmp}/dt33-sg" --root .. --lint 2>&1 | grep -q 'LINT: PASS' && test -f ../changelog/desk-tools-33.md && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | prints `ok`: tree lints clean, fragment exists, skillslint passes. Passes today (modulo the fragment) and must still pass. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer answers, in the verdict: (1) is the rule
scoped to EXPLICIT dispositions only, with silence and bare "done" left to the ordinary
rules? (2) does the safety-consequence carve-out escalate always, accept never? (3) does
the clause sit in exactly ONE home with the other file pointing at it, not duplicated?
(4) is the new-evidence exception stated narrowly enough that a code change alone cannot
reopen a disposed class? (5) does nothing here touch the flip gates or the worker-side
reply-with-evidence duty?
