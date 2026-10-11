---
brief: assay:assay:desk-tools:31
title: "The correctness lane carries a coverage contract — a named bug-class register every class of which is accounted in the verdict body (finding, n/a with a reason, or could-not-check)"
why: >-
  A correctness review today is only as exhaustive as the reviewer's attention that round.
  Nothing in the reviewer's prompt or the verdict-body schema requires a review to ACCOUNT
  for the bug classes it never considered: a verdict that never once thought about
  concurrency, mid-write failure, or caller blast-radius is textually indistinguishable from
  one that examined them and found nothing. Selective review is invisible, and the review
  gate cannot see the difference. The house's deterministic layers already cover their own
  axes (leak-sweep, branch protection, the deskpost body check), and the security lane
  carries trust/authz/injection/secrets — but the correctness lane's model judgment has no
  completeness accounting at all. An external public artifact demonstrates the missing
  mechanic: permanu/superskill's review/architect pack (Apache-2.0) walks 18 named
  axes under the rule "silence is not n/a" — every skipped axis needs an explicit
  `n/a: <one reason>` line, so selective review becomes visible as a gap in the accounting.
  The mechanic is adoptable as a neutral body: a fixed class register plus per-class
  accounting lines in the verdict body, with the accounting reusing this repo's existing
  third state (could-not-check) rather than inventing a new one.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-10-11 by a house authoring session at the driver's direction (external mechanic surveyed from permanu/superskill, public)
sources:
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md — the reviewer's current first-pass duties: finding-class register, round caps, class continuity. All of it governs classes the reviewer RAISED; nothing requires accounting for classes never considered. Line ~202: correctness and security verdicts are separate artifacts — the register below is correctness-only by design."
  - "plugins/assay/skills/pr-review-desk/references/verdict-format.md — the body schema deskpost enforces (H2 heading, bare verdict line, 16 KiB cap, one verdict kind). The Coverage block must ride INSIDE that schema as plain body lines; no schema change, no new enforced structure."
  - "PR #2474 (merged 2026-10) — review rounds: the dispatcher states each round's scope (DELTA or FULL PASS with the reason) in the reviewer's assignment. The coverage accounting keys on that stated scope: a delta round accounts for the classes the scope's paths can touch; a full pass accounts for every class."
  - "permanu/superskill review/architect pack (public, Apache-2.0, surveyed 2026-10-11, https://github.com/permanu/superskill/blob/main/catalog/review/architect.md) — the external evidence for the mechanic: 18 axes, 'silence is not n/a', one n/a-with-reason line per skipped axis, escalation when an axis cannot be honestly n/a'd."
  - "freshness-checked 2026-10-11 @ a4bc0c0cf (origin/main) — review-prompt.md carries no coverage-accounting language; verdict-format.md documents no Coverage block; brief-31 is the next free number in this stream."
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument: one register stated consistently across the
  reviewer prompt's correctness cut, the verdict-body reference, and the skill body — plus
  the layer-2 boundary (the project layer appends classes, never removes the base register)
  that keeps the public body neutral.
consumers:
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here (the correctness-lane class register, the three accounting states, the delta/full-pass accounting rule, and the refusal rule when a class can be neither honestly n/a'd nor checked)"
  - "plugins/assay/skills/pr-review-desk/references/verdict-format.md: fixed-here (the Coverage block: where it sits in the body, the line shape, the 16 KiB budget note, the statement that deskpost neither requires nor refuses the block today)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here (one neutral paragraph in the reviewer's bar: the coverage contract and the project-layer extension rule)"
  - "tools/desk/cmd/deskpost: out-of-scope (deskpost does NOT grow a bodycheck rule for the Coverage block in this brief; whether the block becomes enforced is a follow-up named in ## Context, ruled separately — this brief adds doctrine, not a gate)"
version: 1
id: 3f2b9c52-8e1d-4f7a-9c46-2d15a8e7b6d0
---

# Brief 31 — Correctness-lane coverage contract

## Context

The correctness lane's reviewer prompt is rich in machinery for findings ALREADY raised —
class ids, round caps, continuity, delta scoping — and silent about coverage: no list of
classes a review owes an answer on, no requirement to say which classes it did not examine.
The failure shape is not hypothetical: a reviewer that reads a diff as "a small refactor" may
never ask what happens to the caller two packages over, whether a write is atomic mid-failure,
or whether a loop tolerates duplicate input — and its APPROVE reads exactly like one that did.

The mechanic this brief adopts (surveyed from the public superskill repo, cited above) is one
sentence: **every class in a fixed register is accounted for, and silence is not accounting.**
Adapted to this repo's vocabulary, each class ends the review in exactly one of three states —
a finding (carrying its class id under the existing finding-class machinery), an
`n/a: <class> — <one-sentence reason>` line, or a `could-not-check: <class> — <what was
missing>` line. The third state already exists in this methodology; the register merely
forces it to be per-class instead of per-review.

**Design boundaries this brief holds, stated so the worker does not re-litigate them:**

1. **Correctness-only.** The register contains no security classes (trust/authz, secrets/PII,
   injection). Those are the security lane's, and the two lanes' independence is the design
   (#2474, review-prompt §"SEPARATE artifacts"). The security lane's coverage accounting is a
   possible follow-up, not this brief.
2. **Delta rounds account for the stated scope.** The assignment's `Scope: DELTA` names paths;
   the review accounts for the classes those paths can touch. A `Scope: FULL PASS` accounts
   for the whole register. A delta that grows a path into a class the scope did not name is
   the existing clause-19 wrong-stated-scope case: full pass, say so.
3. **No enforcement in deskpost.** The block rides the verdict body as plain lines. Whether
   deskpost should REFUSE a correctness body with no Coverage block is a real question with a
   real cost (five schema refusals open the 15-minute breaker), and it is a control-narrowing
   decision: ruled separately, named here as follow-up `desk-tools/32+`.
4. **Project layer appends, never removes.** The base register lives in the public reviewer
   prompt (neutral body). A house may append classes for its stack; it may never strike a
   base class. This is the same layer-2 boundary the desk-shell reference uses.

## Task

**Task 1 — the register in the reviewer prompt (correctness cut).**
`tools/desk/cmd/deskdispatch/references/review-prompt.md` gains a clause (numbered in
sequence) carrying the correctness class register. The register, stated once, lower-case
kebab ids:

| class | owes an answer on |
|---|---|
| `spec-intent` | the diff matches the change's stated invariant; extra behavior and dead code are findings |
| `callers-blast-radius` | every caller of a changed symbol, including verbs, tests, jobs and workflow callers |
| `algorithm-edges` | empty, one, duplicate, overflow, unicode, unordered input, idempotent replay |
| `concurrency` | shared mutable state, missing synchronization, double-check, lost updates, duplicate jobs |
| `error-paths-rollback` | mid-write failure, atomic replace, swallowed errors, reversibility |
| `api-contract` | breaking change to a CLI flag, verb schema, JSON field or HTTP shape; versioning |
| `persistence-migrations` | schema expand-then-contract, backfill, dual-write drift, source of truth |
| `resource-lifecycle` | handles, files, servers, timers, processes closed on failure; no leaks |
| `time-ordering` | wall vs monotonic clocks, timezone, TTL/expiry, replay semantics |
| `observability-signals` | a failure in this change can be reconstructed from what it emits; no secrets in logs |
| `tests-evidence` | a check that fails if the new branch is wrong; no generated theater |
| `operability-ship` | feature flag, config, rollback path; who acts when it fails |
| `ui-a11y` | keyboard, labels, contrast, XSS — ONLY when the diff touches UI surfaces; `n/a` otherwise is the norm |
| `structure-ownership` | the owning module is named; a new pattern with one implementation is a finding |

The clause states: every class ends the round as a finding, an `n/a:` line with a one-sentence
reason, or a `could-not-check:` line naming what was missing; silence is not accounting; the
accounting covers the stated scope (delta: the classes the scope's paths can touch; full pass:
the whole register); the project layer may append classes and may never remove a base class;
and the refusal rule — **a class that cannot be honestly n/a'd and was not checked is
`could-not-check`, never a silent APPROVE** — which is the whole point of the contract.

**Task 2 — the verdict body reference.**
`plugins/assay/skills/pr-review-desk/references/verdict-format.md` documents the Coverage
block: an `## Coverage` section at the end of the correctness verdict body, one line per class
(`class — finding <id>` / `class — n/a: <reason>` / `class — could-not-check: <what>`), the
16 KiB budget note (the block for all 14 classes runs well under 1 KiB), and the statement
that deskpost neither requires nor refuses the block today (design boundary 3).

**Task 3 — the skill body (neutral paragraph).**
`plugins/assay/skills/pr-review-desk/SKILL.md` §"The reviewer's bar" gains one paragraph
stating the coverage contract and pointing at the register's home (the review-prompt
reference), with the project-layer append-only rule. Neutral body: no house-specific class
names, no house identifiers.

**Task 4 — changelog fragment.** changelog/desk-tools-31.md (planned), one bullet.

Out of scope: deskpost enforcement (follow-up, ruled separately), the security lane's register
(follow-up), any change to deskdispatch Go code (the register is prompt doctrine; the
dispatcher already hands the prompt through unchanged).

files:
- `tools/desk/cmd/deskdispatch/references/review-prompt.md` — Task 1
- `plugins/assay/skills/pr-review-desk/references/verdict-format.md` — Task 2
- `plugins/assay/skills/pr-review-desk/SKILL.md` — Task 3
- changelog/desk-tools-31.md (planned) — Task 4

## Verify (executable — no prose-only DoD items)

Rows 1–5 are discriminating: each FAILS against today's tree (at a4bc0c0cf) and passes only
once the Task lands. Row 6 passes today and must still pass. Run every row from the repo root.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | prints `ok`: the register and the accounting rule are in the reviewer prompt. FAILS today: none of these strings appear. |
| 2 | check | `grep -c -E '^\| `[a-z-]+` \|' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok` | prints `ok`: at least the 14 register rows are present as table rows in the prompt. FAILS today: 0. |
| 3 | check | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | prints `ok`: the Coverage block, the third state, the budget note and the no-enforcement statement are all in the verdict-body reference. FAILS today: '## Coverage' appears 0 times in that file. |
| 4 | check | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'review-prompt' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'append' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | prints `ok`: the skill states the contract, points at the register's home, and carries the append-only rule. House-neutrality is Review-gate question 4, not a grep — a withheld-name negative grep would itself publish the names. FAILS today: 'coverage contract' appears 0 times in the skill. |
| 5 | check +flow | READ FROM THE FORGE after the first dispatched correctness review post-landing: its verdict body carries an `## Coverage` section with one line per register class in the stated scope's set, each line a finding reference, an `n/a:` with a reason, or a `could-not-check:` with what was missing; and deskpost accepted the body (the review exists as the reviewer App's review at that head). | A real correctness verdict on a real PR demonstrates the contract end to end. Could-not-check at authoring: no review has run under the contract yet. |
| 6 | check +neighbour | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 | grep -q 'LINT: PASS' && test -f ../changelog/desk-tools-31.md && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | prints `ok`: the tree lints clean (statusgen brief/board lint — the new brief file must parse and the README table stays generated), the changelog fragment exists, and skillslint passes. Passes today and must still pass. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer answers, in the verdict: (1) is the register
correctness-only — no class that belongs to the security lane or to a deterministic control?
(2) does every register row owe an answer a competent reviewer could actually give, or is any
row unfalsifiable filler? (3) does the accounting reuse the existing three states with no new
vocabulary? (4) does the skill paragraph stay neutral-body (project-layer extension stated,
no house values inlined)? (5) is deskpost genuinely untouched?
