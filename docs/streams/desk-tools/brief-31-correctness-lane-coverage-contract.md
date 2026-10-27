---
brief: assay:assay:desk-tools:31
title: "The correctness lane carries a coverage contract — a named bug-class register every class of which is accounted in the verdict body (finding, checked-clean, n/a with a reason, or could-not-check)"
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
  accounting lines in the verdict body. Four line shapes, and the reason there are four:
  `checked-clean` and `could-not-check` reuse this repo's existing checked/unchecked
  vocabulary (common-clauses C4) verbatim; `n/a` is the one deliberately new line shape,
  because "this class does not apply to this diff" and "this class was examined and is
  sound" must never share a state — collapsing them is exactly what lets selective review
  hide inside `n/a` lines.
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
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md — the reviewer's current first-pass duties: finding-class register, round caps, class continuity. All of it governs classes the reviewer RAISED; nothing requires accounting for classes never considered. Line 221: correctness and security verdicts are separate artifacts — the register below is correctness-only by design."
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
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here (the correctness-lane class register, the four accounting line shapes (one grammar, class first), the delta/full-pass accounting rule, and the refusal rule when a class can be neither honestly n/a'd nor checked)"
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

The mechanic this brief adopts (surveyed from the public superskill repo, cited above, and
corrected by its round-1 review) is one sentence: **every class in a fixed register is
accounted for, and silence is not accounting.** Each class ends the review in exactly one of
four line shapes — a **finding** (carrying its class id under the existing finding-class
machinery), a **`<class> — checked-clean: <what was examined>`** line (the class applied,
was examined, and is sound — the methodology's existing checked-clean state, applied
per-class), an **`<class> — n/a: <one-sentence reason>`** line (the class genuinely does not
apply to this diff), or a **`<class> — could-not-check: <what was missing>`** line (the
methodology's existing unchecked state, per-class). The line grammar — class first, state
second, colon, detail — is THE one grammar, used identically by Task 1's clause, Task 2's
verdict-body reference, and every example in this brief. `n/a` is the one new line SHAPE;
`checked-clean` and `could-not-check` name the existing C4 states, applied per class; and
the reviewer must never write `n/a` for a class that was examined — examined-and-sound is
`checked-clean`, and conflating the two is precisely the leak this contract exists to close.

**Design boundaries this brief holds, stated so the worker does not re-litigate them:**

1. **Correctness-only.** The register contains no security classes (trust/authz, secrets/PII,
   injection). Those are the security lane's, and the two lanes' independence is the design
   (#2474, review-prompt §"SEPARATE artifacts"). The security lane's coverage accounting is a
   possible follow-up, not this brief.
2. **Delta rounds account for the stated scope.** The assignment's `Scope: DELTA` states the
   head range; the review accounts for the classes the paths that delta range touches can
   apply to (the assignment itself carries no path list — the paths are derived from the
   range). A `Scope: FULL PASS` accounts
   for the whole register. A delta that grows a path into a class the scope did not name is
   the existing clause-19 wrong-stated-scope case: full pass, say so.
3. **No enforcement in deskpost.** The block rides the verdict body as plain lines. Whether
   deskpost should REFUSE a correctness body with no Coverage block is a real question with a
   real cost (five schema refusals open the 15-minute breaker), and it is a control-narrowing
   decision: ruled separately, named here as a later desk-tools brief.
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
| `observability-signals` | a failure in this change can be reconstructed from what it emits (secret-free-logging itself is the security lane's, not this register's) |
| `tests-evidence` | a check that fails if the new branch is wrong; no generated theater |
| `operability-ship` | feature flag, config, rollback path; who acts when it fails |
| `ui-a11y` | keyboard, labels, contrast — ONLY when the diff touches UI surfaces; `n/a` otherwise is the norm (injection into the new surface, XSS included, is the security lane's) |
| `structure-ownership` | the owning module is named; a new pattern with one implementation is a finding |

The clause states the four line shapes in the Context's one grammar (class first, state
second): a finding reference, a checked-clean naming what was examined, an n/a with a
one-sentence reason, or a could-not-check naming what was missing; silence is not accounting; an examined-and-sound class is
`checked-clean`, never `n/a`; the accounting covers the stated scope (delta: the classes the
delta range's paths can touch; full pass: the whole register); the project layer may append
classes and may never remove a base class; and the refusal rule — **a class that cannot be
honestly n/a'd and was not checked is `could-not-check`, never a silent APPROVE** — which is
the whole point of the contract.

**Task 2 — the verdict body reference.**
`plugins/assay/skills/pr-review-desk/references/verdict-format.md` documents the Coverage
block: an `## Coverage` section at the end of the correctness verdict body, one line per class
(`class — finding <id>` / `class — checked-clean: <what was examined>` / `class — n/a:
<reason>` / `class — could-not-check: <what>`), the 16 KiB budget note (the block for all 14
classes is a small fraction of the 16 KiB cap), and the statement
that deskpost neither requires nor refuses the block today (design boundary 3).

**Task 3 — the skill body (neutral paragraph).**
`plugins/assay/skills/pr-review-desk/SKILL.md` §"The reviewer's bar" gains one paragraph
stating the coverage contract and pointing at the register's home (the review-prompt
reference), carrying the sentence "the project layer may append classes and never removes a
base one" verbatim (row 4 greps it). Neutral body: no house-specific class
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
| 2 | check | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | prints `ok`: at least the 14 register rows are present as table rows in the prompt (the cell is double-fenced because the ERE carries backticks; the pipe class is written `[\|]` — the sanctioned escaped form a GFM cell survives, which the lifter turns into a literal pipe class for the shell). FAILS today: 0 matches in review-prompt.md. |
| 3 | check | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | prints `ok`: the Coverage block, the checked-clean and could-not-check line shapes, the budget note and the no-enforcement statement are all in the verdict-body reference. FAILS today: '## Coverage' appears 0 times in that file. |
| 4 | check | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | prints `ok`: the skill states the contract and carries the append-only sentence Task 3 fixes verbatim (`never removes a base` appears nowhere in the skill today). House-neutrality is Review-gate question 4, not a grep — a withheld-name negative grep would itself publish the names. FAILS today: both strings appear 0 times in the skill. |
| 5 | check +flow | READ FROM THE FORGE after the first dispatched correctness review post-landing: its verdict body carries an `## Coverage` section with one line per register class in the stated scope's set, each line a finding reference, a `checked-clean:` naming what was examined, an `n/a:` with a reason, or a `could-not-check:` with what was missing — and no examined-and-sound class written as `n/a`; and deskpost accepted the body (the review exists as the reviewer App's review at that head). | A real correctness verdict on a real PR demonstrates the contract end to end. Could-not-check at authoring: no review has run under the contract yet. |
| 6 | check +neighbour | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | prints `ok`: the tree lints clean (statusgen brief/board lint — the new brief file must parse and the README table stays generated) and skillslint passes. The changelog fragment is deliberately NOT tested here — the release workflow clears `changelog/` when it aggregates, so a fragment-existence check cannot survive a release. Passes today and must still pass. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
**VERIFY: BLOCKED** — rows 1–4 and 6 pass; row 5 could-not-check because first post-landing dispatch chronology is unavailable. This is not an observed implementation failure.

Verification-Attestation: medici-finance/assay#2682 run=cb9d86ccf917639614d23bc30bf7237b1dc79313331e42c1f9ee9fc04910acf9 source=024b15e1c783d4f0694a215e4e32a8f94ce11e91 brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=gpt-6.1-sol tier=any

2026-10-12 UTC; runner assay-verifier-app[bot], fresh non-implementer, actual dispatched model gpt-6.1-sol. Execution platform Darwin. Admission exit 0 before execution. Source remains detached and untouched.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d (matches exactly ok + newline) | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | pass exit=0 | sha256:dc51b8c96c2d (matches exactly ok + newline) | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d (matches exactly ok + newline) | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d (matches exactly ok + newline) | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
| 5 | READ FROM THE FORGE after the first dispatched correctness review post-landing; actual read: gh api graphql repository pullRequests(first:100, orderBy UPDATED_AT DESC) reviews(first:100) | could-not-check | forge read exit=0; earliest observed post-delivery correctness review 5483396998, PR #2601, 2026-10-11T13:02:40Z, head 140e5519fe71efaebc312f84a9dee1ed6069f377 has no Coverage. Dispatch chronology unavailable, so not confirmed first qualifying dispatch. Later complete 14-class reviews are supplementary only. Blocker [#2684](https://github.com/medici-finance/assay/issues/2684). | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (gpt-6.1-sol, non-implementer) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | pass exit=0 | sha256:dc51b8c96c2d (matches exactly ok + newline) | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |

The raw verifyrun aggregate exited 0, but row 5 selected inline `## Coverage` and executed a shell comment, producing empty sha256:e3b0c44298fc. That mechanical result is rejected as evidence for the forge requirement. Raw witness preserved separately; filed command-lifting bug [#2684](https://github.com/medici-finance/assay/issues/2684). Ordinary row hashes match `ok\n` (full SHA256 dc51b8c96c2d745df3bd5590d990230a482fd247123599548e0632fdbf97fc22); row 2 output file contains 17 (14 register rows plus three kebab state rows). Row 6's ok entails successful build, LINT: PASS match, and skillslint exit 0 under verifyrun's pipefail Bash.

Requirements trace: PR #2598 merged at 7808e0f17, 2026-10-11T12:48:09Z, no wrote-to-the-test label. Task 1's 14 classes and rules match; Task 2's shapes, placement, cap and no-enforcement match; Task 3 is neutral/append-only; Task 4 has one bullet in delivery. Four added reviewlane_test.go data rows hold lane cuts; production dispatcher and deskpost code untouched. PR #2633's later ruled grammar/disclosure clarification keeps the shapes intact. Full grounding and diff consistency recorded separately.

Risk enumeration scope: the full delivering diff 7808e0f17^..7808e0f17 (six files) plus the two doctrine corrections in c82135abc and literals explicitly named by Deliverables. No irreversible runtime numerical knob or monetary/auth principal changed. Ranking below still covers the doctrine's authority and completeness values.

Literal inventory (source lines at admitted source): rank 1 = class, state, count, scope, owner and budget; wrong values can hide omitted review or assign it to the wrong lane. All are reversible doctrine changes; no irreversible transfer, spend or identity binding exists here. Rank 2 = navigation/test mapping, illustrative counts; reversible edits. All rank-1 entries are derived below.
- base-class[1] = "spec-intent" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:620.
- base-class[2] = "callers-blast-radius" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:621.
- base-class[3] = "algorithm-edges" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:622.
- base-class[4] = "concurrency" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:623.
- base-class[5] = "error-paths-rollback" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:624.
- base-class[6] = "api-contract" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:625.
- base-class[7] = "persistence-migrations" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:626.
- base-class[8] = "resource-lifecycle" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:627.
- base-class[9] = "time-ordering" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:628.
- base-class[10] = "observability-signals" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:629.
- base-class[11] = "tests-evidence" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:630.
- base-class[12] = "operability-ship" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:631.
- base-class[13] = "ui-a11y" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:632.
- base-class[14] = "structure-ownership" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:633.
- coverage-owner = "correctness" @ tools/desk/cmd/deskdispatch/reviewlane_test.go:29; newly used by rows 206–208 and literal lane marker "<!-- lane:correctness:begin -->" @ review-prompt.md:607.
- stand-in-owner = "security" @ tools/desk/cmd/deskdispatch/reviewlane_test.go:30; newly used at row 209 and literal lane marker "<!-- lane:security:begin -->" @ review-prompt.md:663.
- register-home = "review-prompt" @ plugins/assay/skills/pr-review-desk/SKILL.md:985.
- coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:60.
- accounting-state[1] = "finding" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:644.
- accounting-state[2] = "checked-clean" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:645.
- accounting-state[3] = "n/a" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:646.
- accounting-state[4] = "could-not-check" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:647.
- delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:649.
- full-scope = "Scope: FULL PASS" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:651.
- base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83.
- accounting-line-count = "one" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:60.
- state-count = "four" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:637.
- body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84.
- clause-number = 21 @ tools/desk/cmd/deskdispatch/references/review-prompt.md:605 and reviewlane_test.go:206–209; references = 4 @ review-prompt.md:640, 14 @ :644, 19 @ :649/:652, 5 @ :658/:667, 12 @ :661, 13 @ :668. Rank 2: wrong navigation/test-obligation mapping; reversible edit, no irreversible derivation required.
- examples = "one" @ review-prompt.md:622/:633; illustrative edge / structure count, not operational threshold; rank 2 reversible doctrine edit.

Top-ranked derivations (from requirements, never from green tests):
RISK-VALUE: DERIVED — base-class[1] = "spec-intent" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:620 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[2] = "callers-blast-radius" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:621 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[3] = "algorithm-edges" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:622 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[4] = "concurrency" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:623 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[5] = "error-paths-rollback" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:624 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[6] = "api-contract" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:625 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[7] = "persistence-migrations" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:626 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[8] = "resource-lifecycle" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:627 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[9] = "time-ordering" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:628 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[10] = "observability-signals" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:629 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[11] = "tests-evidence" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:630 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[12] = "operability-ship" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:631 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[13] = "ui-a11y" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:632 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — base-class[14] = "structure-ownership" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:633 — exact class specified by Brief 31 Task 1; all 14 are correctness concerns and the supplied obligation stays within that concern.

RISK-VALUE: DERIVED — coverage-owner = "correctness" @ tools/desk/cmd/deskdispatch/reviewlane_test.go:29 — Context boundary 1 makes the base register correctness-only; security retains its separate verdict.

RISK-VALUE: DERIVED — stand-in-owner = "security" @ tools/desk/cmd/deskdispatch/reviewlane_test.go:30 — the same boundary requires the security lane to disclose ownership instead of claiming checked-clean for the register.

RISK-VALUE: DERIVED — register-home = "review-prompt" @ plugins/assay/skills/pr-review-desk/SKILL.md:985 — Task 1 owns the one base register and Task 3 points there rather than duplicating it.

RISK-VALUE: DERIVED — coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:60 — Task 2 names this exact heading.

RISK-VALUE: DERIVED — accounting-state[1] = "finding" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:644 — Task 1 requires an existing finding-id reference.

RISK-VALUE: DERIVED — accounting-state[2] = "checked-clean" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:645 — existing C4 examined-and-sound state distinguishes genuine examination from non-applicability.

RISK-VALUE: DERIVED — accounting-state[3] = "n/a" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:646 — Task 1 reserves this for non-applicability with one reason.

RISK-VALUE: DERIVED — accounting-state[4] = "could-not-check" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:647 — C4 requires honest disclosure when it could not look.

RISK-VALUE: DERIVED — delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:649 — boundary 2 ties obligation to changed paths derived from assigned head range.

RISK-VALUE: DERIVED — full-scope = "Scope: FULL PASS" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:651 — boundary 2 owes the whole register on full pass.

RISK-VALUE: DERIVED — base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83 — Task 1 enumerates exactly fourteen distinct ids, counted from requirements.

RISK-VALUE: DERIVED — accounting-line-count = "one" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:60 — one terminal classification per class makes omitted/duplicated accounting visible.

RISK-VALUE: DERIVED — state-count = "four" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:637 — finding, examined sound, non-applicable, and unexamined are four distinct requirement states.

RISK-VALUE: DERIVED — body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84 — existing bodycheck MaxBodyBytes = 16 * 1024 @ tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go:28; 16*1024 = 16384 bytes. Task 2 reuses that cap without changing enforcement.


Rejected mechanical row 5 witness, transcribed verbatim for the command-lifting defect; this does not discharge the Verify row:

```text
| 5 | `## Coverage` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-12 | assay-verifier-app[bot] @ 024b15e1c783 (on-behalf-of human:ian) (forge-identity) |
```

### Re-verification 2026-10-15 (source 8efe5579d925)

Fresh non-implementer verifier, dispatched model claude-opus-5-5, platform Darwin, git 2.56.0, go 1.27.2. Admission (`deskdispatch --check-verifier`) exit 0 before any row. Window: started 2026-10-15T04:23:56Z, rows executed 04:24Z to 04:26Z, Evidence written 04:27Z. Every row ran from the repo root of a detached worktree at 8efe5579d925, each in its own isolated shell with HOME, TMPDIR, XDG config, GOCACHE pointed at a scratch dir outside the worktree, KUBECONFIG=/dev/null, GOPROXY=off, GOFLAGS=-mod=readonly. Table-escaped pipes were lifted to literal pipes before execution (row 2's bracket class `[|]`, row 6's pipeline). The worktree stayed clean (no tracked, untracked or ignored files) after every row.

Grounding before reading the diff: from the brief, merged main should carry (a) a numbered review-prompt clause with exactly the 14 kebab class ids of Task 1, the four line shapes in class-first grammar, "silence is not accounting", delta/full-pass scope, append-only project layer and the could-not-check refusal rule, all inside the correctness lane cut; (b) an `## Coverage` block documented in the verdict-format reference with the four shapes, a 16 KiB budget note and the deskpost no-enforcement statement; (c) one neutral skill paragraph carrying "never removes a base"; (d) no deskpost change.

Delivering change: PR #2598, squash-merged as 7808e0f17daa (2026-10-11T12:48:09Z), six files; later wording clarification PR #2633 (c82135abc745) scoped the grammar colon to the detailed shapes. Neither touches deskpost. Rows 1 to 4 are discriminating: at the pre-delivery parent 63bd67f01a01 the strings spec-intent, silence is not accounting, the Coverage heading and never removes a base each count 0.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | discharges Verify row 1; stdout `ok` (sha256:dc51b8c96c2d) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | pass exit=0 | discharges Verify row 2; stdout `ok` (sha256:dc51b8c96c2d); count file holds 17 = the 14 register rows (review-prompt lines 620 to 633) plus 3 state-table rows (finding, checked-clean, could-not-check); register rows byte-identical to the brief's Task 1 table (diff exit 0) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | pass exit=0 | discharges Verify row 3; stdout `ok` (sha256:dc51b8c96c2d) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | discharges Verify row 4; stdout `ok` (sha256:dc51b8c96c2d); paragraph names no house-specific class or identifier | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | READ FROM THE FORGE after the first dispatched correctness review post-landing (review list of medici-finance/assay pull requests after 2026-10-11T12:48:09Z) | could-not-check | Verify row 5 could not execute in this pass: the dispatch envelope is offline (no external endpoints), so no forge review was read. No local artifact substitutes for the forge read; the row stays open. Prior chronology blocker: #2684 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | pass exit=0 | discharges Verify row 6; stdout `ok` (sha256:dc51b8c96c2d); components re-run separately: lint printed LINT: PASS (exit 0, NOTICE lines only), skillslint exit 0 (GUARDRAILS, ENFORCEMENT-BLOCK, POSIX-TOKEN all PASS) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Row 5 re-run (needs forge read access, outside this offline envelope): list reviews on medici-finance/assay pull requests submitted after 2026-10-11T12:48:09Z by the reviewer App, identify the first correctness-lane review dispatched by a deskdispatch release whose embedded review prompt carries clause 21, and check its body for an `## Coverage` section with one class-first line per in-scope register class, e.g. `gh api 'repos/medici-finance/assay/pulls/<N>/reviews' --paginate` for each candidate PR. Dispatch chronology (which release dispatched which review) is the open question tracked in #2684.

Supplementary, not a Verify row: the deskdispatch lane-cut unit tests (`go test ./cmd/deskdispatch/ -run Lane -count=1` from tools/desk) exit 0, covering the four clause-21 obligation rows the delivery added to the lane obligation table.

Risk trigger: frontmatter risk fields are all present and `no`, gate model, the diff is prompt and reference doctrine plus test-table rows; no hard-pinned value is changed (the 16 KiB cap is referenced, not edited). Enumeration still done over 7808e0f17daa (six files) plus c82135abc745 and the Deliverables' named literals.

Enumerated literals at source 8efe5579d925:
- register ids = spec-intent, callers-blast-radius, algorithm-edges, concurrency, error-paths-rollback, api-contract, persistence-migrations, resource-lifecycle, time-ordering, observability-signals, tests-evidence, operability-ship, ui-a11y, structure-ownership @ tools/desk/cmd/deskdispatch/references/review-prompt.md:620 to :633 (one id per line, in that order).
- state tokens = "finding" :644, "checked-clean" :645, "n/a" :646, "could-not-check" :647 @ tools/desk/cmd/deskdispatch/references/review-prompt.md.
- state-count = "four" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:637.
- delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:649; full-scope = "Scope: FULL PASS" @ :651.
- clause-number = 21 @ tools/desk/cmd/deskdispatch/references/review-prompt.md:605 (and four test-table rows keyed 21 in reviewlane_test.go).
- coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:60.
- base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83.
- body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84.

Ranking: none is irreversible; every entry is doctrine text undone by an edit and a release. Rank 1 (a wrong value silently narrows what a review must account for, or lets a lane claim the wrong state): register ids, state tokens, base-count, body-budget, scope tokens. Rank 2 (navigation only): clause-number, heading text, state-count word.

**VERIFY: BLOCKED** — rows 1, 2, 3, 4 and 6 pass (5 of 6); row 5 is could-not-check (forge read outside the offline envelope; dispatch chronology open in #2684). No implementation failure observed.

RISK-VALUE: DERIVED — register ids = the 14 kebab ids @ tools/desk/cmd/deskdispatch/references/review-prompt.md:620 — byte-identical to the brief's Task 1 table (diff exit 0); none is a security class (trust/authz, secrets/PII, injection excluded per Context boundary 1; ui-a11y and observability-signals explicitly hand XSS and secret-free logging to the security lane).
RISK-VALUE: DERIVED — base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83 — counted independently from the brief's Task 1 table (14 rows) and from review-prompt lines 620 to 633 (14 rows); the two agree.
RISK-VALUE: DERIVED — state tokens = finding / checked-clean / n/a / could-not-check @ tools/desk/cmd/deskdispatch/references/review-prompt.md:644 — the brief requires exactly four mutually exclusive outcomes (raised, examined-sound, inapplicable, unexamined); checked-clean and could-not-check reuse the common-clause C4 vocabulary verbatim and n/a is the single new shape, as the brief's why states.
RISK-VALUE: DERIVED — body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84 — equals the enforced cap MaxBodyBytes = 16 * 1024 = 16384 bytes @ tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go:28 and the schema line at verdict-format.md:43; a realistic 14-line block measured 1521 bytes, about 9 percent of the cap, so "a small fraction" holds.
RISK-VALUE: DERIVED — delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:649 (full-scope "Scope: FULL PASS" @ :651) — exactly the scope tokens clause 19 already defines at review-prompt.md:562 to :566, so the accounting keys on the scope the dispatcher actually states (Context boundary 2).

Verification-Attestation: medici-finance/assay#3014 run=281c20851c632d53e7743a354e023cae3432c5649405423c38c9c145fcc8975b source=8efe5579d9253813835db650d45180eea76cf33e brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=claude-opus-5-5 tier=any

### Re-verification 2026-10-18 (source a724e99dcd89)

Fresh non-implementer verifier, dispatched model claude-opus-5-5, tier strong, platform Darwin, git 2.56.0, go 1.27.2. Admission (`deskdispatch --check-verifier`) exit 0 before any row. Rows executed 2026-10-18T20:23Z to 20:45Z from the repo root of a detached worktree at a724e99dcd89, TMPDIR and all output outside the worktree, KUBECONFIG=/dev/null, GOFLAGS=-mod=readonly; table-escaped pipes lifted to literal pipes (row 2's bracket class, row 6's pipeline). The worktree stayed clean (no tracked, untracked or ignored files) after every row. Row 5 is a read-only forge read under the verifier App token; nothing was posted.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | discharges Verify row 1; stdout `ok` (sha256:dc51b8c96c2d); each string present once in review-prompt.md (cross-checked with /usr/bin/grep) | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | pass exit=0 | discharges Verify row 2; stdout `ok` (sha256:dc51b8c96c2d); count file holds 17 = the 14 register rows (review-prompt lines 650 to 663) plus 3 state-table rows (lines 674, 675, 677); register rows byte-identical to the brief's Task 1 table (diff exit 0) | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | pass exit=0 | discharges Verify row 3; stdout `ok` (sha256:dc51b8c96c2d); Coverage section at verdict-format.md:58, budget note :83 to :84, no-enforcement statement :86 | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | discharges Verify row 4; stdout `ok` (sha256:dc51b8c96c2d); paragraph at SKILL.md:1005 to :1014 names no house-specific class or identifier | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | READ FROM THE FORGE after the first dispatched correctness review post-landing; actual read: gh api reviews by assay-reviewer-app[bot] on medici-finance/assay PRs 2590 to 2789 and on every house-reviewed sibling repo, 2026-10-11T12:48:09Z to 2026-10-12T16:24:06Z | pass exit=0 | discharges Verify row 5. The review kit is compiled into deskdispatch (go:embed at kits.go:31), so a review carries clause 21 only once its dispatcher build contains 7808e0f17. Forge compare: v1.0.39 (published 2026-10-11T12:33:33Z) is 4 commits behind 7808e0f17; v1.0.40 (published 2026-10-12T14:15:59Z) is the first release containing it; the house toolchain pin moved to v1.0.40 at 2026-10-12T15:18:41Z. First correctness review after that pin, across all house-reviewed repos: 2026-10-12T15:46:33Z on a private sibling repo (name withheld from this public record), APPROVED at its stated head, FULL PASS, Coverage 14 of 14 classes, one class-first line each: 10 checked-clean, 3 n/a each with a reason (concurrency, resource-lifecycle, ui-a11y on a docs-only diff), 1 in the fourth state naming the missing forward-test artifact; no examined-and-sound class written as n/a. First public witness: medici-finance/assay#2700 review 5493498845, 2026-10-12T16:24:06Z, commit 8b81400a4c9e = stated head, APPROVED, FULL PASS, 14 of 14 (13 checked-clean, ui-a11y n/a: no UI surface changes). Both exist as reviewer-App reviews at their stated heads, so deskpost accepted the bodies | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | pass exit=0 | discharges Verify row 6; stdout `ok` (sha256:dc51b8c96c2d); components re-run separately: lint exit 0 printing LINT: PASS (NOTICE lines only), skillslint exit 0 (GUARDRAILS, ENFORCEMENT-BLOCK, ACT-BLOCK, AMBIENT-AUTH, POSIX-TOKEN all PASS) | 2026-10-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Row 5 chronology bound. The earlier entries kept this row open because the first post-landing dispatch could not be identified. It is identified here from the release and pin history above. Every post-merge correctness review without a Coverage block was submitted before 2026-10-11T22:59:52Z, while the house pin was v1.0.39, which predates the delivery. From that time on, every correctness review in the scanned set carries the block: on medici-finance/assay, 131 of 131 from 2026-10-11T23:19Z across PRs 2590 to 2789; on the siblings, every one from 22:59:52Z to 16:24:06Z, including the review of the v1.0.40 pin change itself. So the first review dispatched under the contract carries the block under each candidate start: the first Coverage-bearing review, the v1.0.40 publish, or the v1.0.40 pin. One earlier review, medici-finance/assay#2633 (2026-10-11T15:41Z), also carries the block; its diff edits the Coverage doctrine itself. The verifyrun lifting defect named in the earlier entries (#2684) is closed.

Delivering change: PR #2598, squash-merged as 7808e0f17daa (2026-10-11T12:48:09Z), six files, none under tools/desk/cmd/deskpost. PR #2633 (c82135abc745) clarified the grammar colon. 6564d2da7 later edited review-prompt.md, but its hunks sit outside clause 21. Rows 1 to 4 discriminate: at the pre-delivery parent the strings spec-intent, silence is not accounting, the Coverage heading and never removes a base each count 0. Supplementary, not a Verify row: `go test ./cmd/deskdispatch/ -run Lane -count=1` from tools/desk exits 0.

Risk trigger: the frontmatter risk fields are present and all `no`, and the gate is model. The diff is prompt and reference doctrine plus test-table rows, and no hard-pinned value changes (the 16 KiB cap is referenced, not edited). Enumeration over 7808e0f17daa (six files), c82135abc745 and the Deliverables' named literals, at source a724e99dcd89:
- register ids = spec-intent, callers-blast-radius, algorithm-edges, concurrency, error-paths-rollback, api-contract, persistence-migrations, resource-lifecycle, time-ordering, observability-signals, tests-evidence, operability-ship, ui-a11y, structure-ownership @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 to :663.
- state tokens = the four state-table rows @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 to :677; state-count = "four" @ :667.
- delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679; full-scope = "Scope: FULL PASS" @ :681.
- clause-number = 21 @ tools/desk/cmd/deskdispatch/references/review-prompt.md:635.
- coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:58.
- base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83; body-budget = "16 KiB" @ :84.

Ranking: none of these is irreversible. Every entry is doctrine text that an edit and a release undo. Rank 1 (a wrong value silently narrows what a review must account for): register ids, state tokens, base-count, body-budget, scope tokens. Rank 2 (navigation only): clause-number, heading text, state-count word.

RISK-VALUE: DERIVED — register ids = the 14 kebab ids @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 — byte-identical to the brief's Task 1 table (diff exit 0); none is a security class, and ui-a11y and observability-signals hand XSS and secret-free logging to the security lane (Context boundary 1).
RISK-VALUE: DERIVED — base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83 — counted independently from the brief's Task 1 table (14 rows) and from review-prompt lines 650 to 663 (14 rows); both forge witnesses account exactly 14 classes.
RISK-VALUE: DERIVED — state tokens = the four state-table rows @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 — the brief requires four mutually exclusive outcomes (raised, examined-sound, inapplicable, unexamined); the examined and unexamined states reuse the common-clause C4 vocabulary verbatim, and n/a is the single new shape.
RISK-VALUE: DERIVED — body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84 — equals the enforced cap MaxBodyBytes = 16 * 1024 = 16384 bytes @ tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go:28; the real 14-line block in the first post-pin review sits inside an 8149-byte body, so "a small fraction" holds.
RISK-VALUE: DERIVED — delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679 (full-scope "Scope: FULL PASS" @ :681) — exactly the scope tokens clause 19 defines at review-prompt.md:592 to :596, so the accounting keys on the scope the dispatcher actually states (Context boundary 2).

**VERIFY: PASS** — 6 of 6 rows pass at a724e99dcd89; row 5 read from the forge.

Verification-Attestation: medici-finance/assay#3410 run=b4a8d48aa2699f37c20f9c563e99f988ac18acefc4ab2fe8d7c6969bf1e0fbfb source=a724e99dcd89a00f22de274f92ef0ca54fa918da brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=claude-opus-5-5 tier=strong

### Re-verification 2026-10-19 (source 50bef70d518d) — corrects the 2026-10-18 block's row 5

Fresh non-implementer verifier, dispatched model claude-opus-5-5, tier strong, platform Darwin. Admission (`deskdispatch --check-verifier`) exit 0 before any row and again before writing this block. Every row re-run at source 50bef70d518d from the repo root of a detached worktree; TMPDIR and all output outside the worktree, KUBECONFIG=/dev/null, GOFLAGS=-count=1. The only change in the home is this brief's Evidence section. Execution witness: `statusgen verifyrun` (v1.0.50, non-dry) ran 2026-10-19T05:11:41Z to 05:12:02Z, exit 2 because row 5 is could-not-run; its table follows (its `+dirty` marks this Evidence-only edit). By-hand rows follow the witness.

This block supersedes the "Re-verification 2026-10-18 (source a724e99dcd89)" block above on row 5 only. It withdraws three claims made there: the row 5 Evidence cell's "pass exit=0 — discharges Verify row 5"; the "Row 5 chronology bound" paragraph's premise that every post-merge correctness review without a Coverage block was dispatched while the house pin was v1.0.39 and so predates the delivery; and that block's PASS line, "6 of 6 rows pass, row 5 read from the forge". The pin does not identify which contract a review ran under (detail in row 5 below), so row 5 is could-not-check. Rows 1 to 4 and 6 of that block are not changed.

Execution witness:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `## Coverage` | could-not-run exit=- — prose-led-command: not executed; the first span ## Coverage is a shell comment (a command starting `#` is ignored by the shell: it exits 0 with no output and measures nothing), not a command. Mark the real command with a cmd: code span where one exists; otherwise keep the row a manual procedure with its evidence, never a command that merely exits 0 | sha256: | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3446 run=b3e7629772e514ed7fe0606f00ad3bdc87ba7172b6449f5dfc8b278000c6440c source=50bef70d518db5d73a6a2defd956f7bfcf346024 brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=claude-opus-5-5 tier=strong

By-hand rows (the witness's row 5 entry is the lifter's could-not-run for a prose row; the row 5 below records the forge read):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | stdout ok; each string present at source, absent (count 0) at the pre-delivery parent of 7808e0f17 | 2026-10-19 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | pass exit=0 | stdout ok; count 17 = 14 register rows (review-prompt lines 650 to 663) plus 3 state-table rows (674, 675, 677); the 14 register rows are byte-identical to the brief's Task 1 table (diff exit 0) | 2026-10-19 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | pass exit=0 | stdout ok; Coverage section at verdict-format.md:58, budget note :83 to :84, no-enforcement statement :86 | 2026-10-19 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | stdout ok; paragraph at SKILL.md:1005 to :1014, names no house-specific class or identifier | 2026-10-19 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | READ FROM THE FORGE after the first dispatched correctness review post-landing; actual read: gh api reviews on medici-finance/assay PRs 2601 and 2633, release list, and compare of 7808e0f17 against v1.0.39 and v1.0.40 | could-not-check | forge read. The first correctness review submitted after the 2026-10-11T12:48:09Z landing, review 5483396998 on PR #2601 at 2026-10-11T13:02:40Z (head 140e5519fe71), has no Coverage section. A correctness review at 2026-10-11T15:41:37Z (review 5483887086 on PR #2633, head e7034f9fef92) does carry one, although no release contained 7808e0f17 until v1.0.40 at 2026-10-12T14:15:59Z (v1.0.39 is behind it). The kit is embedded at build time (go:embed, kits.go:31), so at least one Coverage-bearing review ran on a build the release record does not name. Neither review body records its dispatcher build, and no per-review dispatch record was found. The first review dispatched under the contract therefore cannot be identified. Under the plain reading (the first review submitted after landing) the row would fail. Which review the row means is an open check-definition question, tracked in medici-finance/assay#1927 | 2026-10-19 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | pass exit=0 | stdout ok; components re-run separately: lint printed LINT: PASS, skillslint exit 0 (AMBIENT-AUTH and POSIX-TOKEN among its PASS lines) | 2026-10-19 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Supplementary, not Verify rows: `go test ./cmd/deskdispatch/ -run Lane -timeout 300s` from tools/desk exits 0. The delivering change (PR #2598, 7808e0f17daa, six files) touches nothing under tools/desk/cmd/deskpost. Coverage-bearing reviews appear from 2026-10-11T15:41Z on, including medici-finance/assay#2700 review 5493498845. They show the contract operating, but they cannot discharge row 5 while it is unknown which review was dispatched first under the contract.

Risk trigger: the frontmatter risk fields are present and all `no`, and the gate is model. Enumeration covered the delivering diff 7808e0f17daa (six files) and the Deliverables' named literals, at source 50bef70d518d:
- register ids = the 14 kebab ids spec-intent through structure-ownership @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 to :663.
- state tokens = finding, checked-clean, n/a, could-not-check @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 to :677; state-count = "four" @ :667.
- delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679; full-scope = "Scope: FULL PASS" @ :681.
- clause-number = 21 @ tools/desk/cmd/deskdispatch/references/review-prompt.md:635.
- coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:58.
- base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83; body-budget = "16 KiB" @ :84.

Ranking: none is irreversible. All are doctrine text that an edit and a release undo. Rank 1 (a wrong value narrows what a review must account for): register ids, state tokens, base-count, body-budget, scope tokens. Rank 2 (navigation): clause-number, heading, state-count word.

RISK-VALUE: DERIVED — register ids = the 14 kebab ids @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 — byte-identical to the brief's Task 1 table (diff exit 0); none is a security class (Context boundary 1).
RISK-VALUE: DERIVED — base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83 — counted independently from the brief's Task 1 table (14 rows) and from review-prompt lines 650 to 663 (14 rows).
RISK-VALUE: DERIVED — state tokens = finding / checked-clean / n/a / could-not-check @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 — the brief requires four mutually exclusive outcomes; checked-clean and could-not-check reuse the C4 vocabulary verbatim, and n/a is the one new shape.
RISK-VALUE: DERIVED — body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84 — equals the enforced cap MaxBodyBytes = 16 * 1024 = 16384 @ tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go:28 and the schema line at verdict-format.md:43.
RISK-VALUE: DERIVED — delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679 (full-scope @ :681) — the same scope tokens clause 19 defines at review-prompt.md:592 to :596 (Context boundary 2).

**VERIFY: BLOCKED** — 5 of 6 rows pass at 50bef70d518d (rows 1 to 4 and 6); row 5 could-not-check: the first contract-dispatched correctness review cannot be identified from the forge (open check-definition question, medici-finance/assay#1927). No implementation failure observed. This supersedes the 2026-10-18 block's row 5 result and its PASS line.

Verification-Attestation: medici-finance/assay#3446 run=b3e7629772e514ed7fe0606f00ad3bdc87ba7172b6449f5dfc8b278000c6440c source=50bef70d518db5d73a6a2defd956f7bfcf346024 brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=claude-opus-5-5 tier=strong

### Re-verification 2026-10-21 (source 637c9be9788a)

Fresh non-implementer verifier, dispatched model claude-opus-5-5, tier strong, platform Darwin, git 2.56.0, go 1.27.2. Admission (`deskdispatch --check-verifier`) exit 0 before any row, and again after this block was written. Rows executed 2026-10-21T17:53Z to 18:00Z from the repo root of a detached worktree at 637c9be9788a, with TMPDIR set to a scratch directory outside the worktree and KUBECONFIG=/dev/null. Table-escaped pipes were lifted to literal pipes (row 2's bracket class, row 6's pipeline). After every row the worktree was clean: no tracked, untracked or ignored files. Row 5 is a read-only forge read under the verifier App token. Nothing was posted.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | prints ok | pass exit=0; stdout ok; register at review-prompt.md lines 650 to 663, clause 21 heading at line 635 | 2026-10-21 | assay-verifier-app[bot] @ 637c9be9788a2f4d4ea1f59d3d24c9452968fd26 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | prints ok (at least 14 register rows) | pass exit=0; stdout ok; count file holds 17: the 14 register rows (lines 650 to 663) plus 3 state-table rows (lines 674, 675, 677). The 14 register rows match the brief's Task 1 table byte for byte (diff exit 0) | 2026-10-21 | assay-verifier-app[bot] @ 637c9be9788a2f4d4ea1f59d3d24c9452968fd26 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | prints ok | pass exit=0; stdout ok; Coverage section at verdict-format.md line 58, budget note at lines 83 to 84, no-enforcement statement at line 86 | 2026-10-21 | assay-verifier-app[bot] @ 637c9be9788a2f4d4ea1f59d3d24c9452968fd26 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | prints ok | pass exit=0; stdout ok; paragraph at SKILL.md lines 1005 to 1014 | 2026-10-21 | assay-verifier-app[bot] @ 637c9be9788a2f4d4ea1f59d3d24c9452968fd26 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | READ FROM THE FORGE after the first dispatched correctness review post-landing; actual read: gh api repos/medici-finance/assay/pulls/2601/reviews and pulls/2633/reviews (reviewer App only), commit 7808e0f17daa, issue medici-finance/assay#1927 state | Coverage section in the first contract-dispatched correctness review, accepted by deskpost | could-not-check: the first correctness review dispatched under the contract cannot be identified from the forge. Re-read this run: delivery 7808e0f17daa landed 2026-10-11T12:48:09Z. The next correctness review, 5483396998 on PR #2601 at 2026-10-11T13:02:40Z (head 140e5519fe71), has no Coverage heading, and neither does 5483451854 on the same PR at 13:19:34Z. Review 5483887086 on PR #2633 at 2026-10-11T15:41:37Z (head e7034f9fef92, APPROVED) does carry one. No review body records the dispatcher build it ran under. The check-definition question of which review the row means is still open (medici-finance/assay#1927, state OPEN), and the row text is unchanged at this source | 2026-10-21 | assay-verifier-app[bot] @ 637c9be9788a2f4d4ea1f59d3d24c9452968fd26 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | prints ok | pass exit=0; stdout ok. Components were re-run separately. The lint printed LINT: PASS, with NOTICE and could-not-check stale lines for other briefs only. skillslint exited 0 (ENFORCEMENT-BLOCK, ACT-BLOCK, AMBIENT-AUTH and POSIX-TOKEN all PASS) | 2026-10-21 | assay-verifier-app[bot] @ 637c9be9788a2f4d4ea1f59d3d24c9452968fd26 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Since the delivery (7808e0f17daa), review-prompt.md, verdict-format.md and SKILL.md have been edited by later commits. None of those edits moved the clause 21 register, the Coverage section or the skill paragraph: every line number above matches the 2026-10-19 block.

Risk trigger: all frontmatter risk fields are present and set to `no`, and the gate is model. Enumeration covered the delivering diff 7808e0f17daa and the Deliverables' named literals, re-read at source 637c9be9788a:
- register ids = the 14 kebab ids spec-intent through structure-ownership @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 to :663.
- state tokens = finding, checked-clean, n/a, could-not-check @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 to :677; state-count = "four" @ :667.
- delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679; full-scope = "Scope: FULL PASS" @ :681.
- clause-number = 21 @ tools/desk/cmd/deskdispatch/references/review-prompt.md:635.
- coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:58.
- base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83; body-budget = "16 KiB" @ :84.

Ranking: none of these is irreversible. Each is doctrine text, so an edit and a release undo it. Rank 1 (a wrong value narrows what a review must account for): register ids, state tokens, base-count, body-budget, scope tokens. Rank 2 (navigation only): clause-number, heading, state-count word.

RISK-VALUE: DERIVED — register ids = the 14 kebab ids @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 — they match the brief's Task 1 table byte for byte (diff exit 0, this run). None is a security class (Context boundary 1).
RISK-VALUE: DERIVED — base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83 — counted two ways that agree: the brief's Task 1 table has 14 rows, and review-prompt lines 650 to 663 hold 14 rows.
RISK-VALUE: DERIVED — state tokens = finding / checked-clean / n/a / could-not-check @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 — the brief requires four mutually exclusive outcomes. checked-clean and could-not-check reuse the C4 vocabulary verbatim, and n/a is the one new shape.
RISK-VALUE: DERIVED — body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84 — it equals the enforced cap MaxBodyBytes = 16 * 1024 @ tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go:28 and the schema line at verdict-format.md:43.
RISK-VALUE: DERIVED — delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679 (full-scope @ :681) — these are the same scope tokens clause 19 defines (Context boundary 2).

**VERIFY: BLOCKED** — 5 of 6 rows pass at 637c9be9788a (rows 1 to 4 and 6). Row 5 is could-not-check: the forge cannot identify the first correctness review dispatched under the contract, and the check-definition question is still open (medici-finance/assay#1927). No implementation failure was observed.

Verification-Attestation: medici-finance/assay#3889 run=baf34b0b459dc3631f7d10625e9c8d2cb0cc4f0485f0b43b61cb1a18c2978033 source=637c9be9788a2f4d4ea1f59d3d24c9452968fd26 brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=claude-opus-5-5 tier=strong

### Re-verification 2026-10-22 (source 05dc9fbc88f2)

Fresh non-implementer verifier, dispatched model claude-opus-5-5, tier strong, platform Darwin, git 2.56.0. The source 05dc9fbc88f2b1b23cd78bebc8634c564327eca4 equals forge main, read through the forge API before any row. Admission (`deskdispatch --check-verifier`) exited 0 before any row and again after this block was written, with the same binding. The table below is the write-mode `statusgen verifyrun` execution witness, run 2026-10-22T03:27Z from the repo root of the detached admitted worktree on a clean tree at the source, with TMPDIR and the Go caches set to a scratch directory outside the worktree and KUBECONFIG=/dev/null. Every Verify row has class `check`; none is check:ci, so no network-none container run was needed.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `## Coverage` | could-not-run exit=- — prose-led-command: not executed; the first span ## Coverage is a shell comment (a command starting `#` is ignored by the shell: it exits 0 with no output and measures nothing), not a command. Mark the real command with a cmd: code span where one exists; otherwise keep the row a manual procedure with its evidence, never a command that merely exits 0 | sha256: | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | pass exit=0 | sha256:f811bc73559e | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4048 run=829704b52b6466aacb5a46497b56c6224fdc36b75937537c93cef0af793ea8f8 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=claude-opus-5-5 tier=strong

Witness reading. Rows 1 to 4 exit 0 with output hash dc51b8c96c2d, which is the SHA-256 of `ok` plus a newline. Row 6 exits 0 with hash f811bc73559e. The Go module cache in the scratch directory was empty before the witness run and was filled during it, so row 6's captured output is consistent with module download lines from the first build alongside `ok`. Row 5 is could-not-check: the witness correctly refuses to execute the prose row (prose-led-command).

Hand re-runs (supplementary, prose only). Rows 1 to 4 and 6 were re-run verbatim, table-escaped pipes lifted to literal pipes, in a fresh scratch clone detached at the source, with the clone's own origin/main ref pinned to the source so the lint's merge-base matches forge main. All five printed `ok` and exited 0. Row 2's count file holds 17: the 14 register rows at review-prompt.md lines 650 to 663 plus 3 state-table rows (lines 674, 675, 677). The 14 register rows match the brief's Task 1 table byte for byte (diff exit 0). Row 3's Coverage heading is at verdict-format.md line 58, the budget note at lines 83 to 84 (the cap also at line 43), and the no-enforcement statement at line 86. Row 4's paragraph is at SKILL.md lines 1005 to 1014. Row 6 re-run with a warm module cache produced exactly `ok` plus a newline (hash dc51b8c96c2d), the lint printed LINT: PASS, and skillslint exited 0. A first clone attempt, before its origin/main was pinned, reported LINT: FAIL with 26 verify-obligation problems in other briefs; its origin/main was a stale branch of the checkout it was cloned from, not forge main, so that attempt was discarded as an environment fault. Since the 2026-10-21 block, review-prompt.md changed only at lines 43 to 47, outside clause 21; the register and the Coverage section are unchanged.

Row 5 (forge read, read-only, verifier App token, nothing posted): delivery 7808e0f17daa landed 2026-10-11T12:48:09Z. The reviewer App's correctness reviews on PR #2601 after that time (5483396998 at 13:02:40Z, 5483398121 at 13:03:06Z, 5483446254 at 13:18:28Z, 5483451854 at 13:19:34Z) carry no Coverage heading. Review 5483887086 on PR #2633 at 2026-10-11T15:41:37Z (head e7034f9fef92, APPROVED) does carry one. No review body records the dispatcher build it ran under, so the first review dispatched under the contract cannot be identified. Row 5 is could-not-check: the check-definition question of which review the row means is still open (medici-finance/assay#1927, state OPEN), and the row text is unchanged at this source.

Risk trigger: all frontmatter risk fields are present and set to `no`, and the gate is model. Enumeration covered the delivering diff 7808e0f17daa and the Deliverables' named literals, re-read at source 05dc9fbc88f2:
- register ids = the 14 kebab ids spec-intent through structure-ownership @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 to :663.
- state tokens = finding, checked-clean, n/a, could-not-check @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 to :677; state-count = "four" @ :667.
- delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679; full-scope = "Scope: FULL PASS" @ :681.
- clause-number = 21 @ tools/desk/cmd/deskdispatch/references/review-prompt.md:635.
- coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:58.
- base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83; body-budget = "16 KiB" @ :84.

Ranking: none of these is irreversible. Each is doctrine text, so an edit and a release undo it. Rank 1 (a wrong value narrows what a review must account for): register ids, state tokens, base-count, body-budget, scope tokens. Rank 2 (navigation only): clause-number, heading, state-count word.

RISK-VALUE: DERIVED — register ids = the 14 kebab ids @ tools/desk/cmd/deskdispatch/references/review-prompt.md:650 — they match the brief's Task 1 table byte for byte (diff exit 0, this run). None is a security class (Context boundary 1).
RISK-VALUE: DERIVED — base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83 — counted two ways that agree: the brief's Task 1 table has 14 rows, and review-prompt lines 650 to 663 hold 14 rows.
RISK-VALUE: DERIVED — state tokens = finding / checked-clean / n/a / could-not-check @ tools/desk/cmd/deskdispatch/references/review-prompt.md:674 — the brief requires four mutually exclusive outcomes. checked-clean and could-not-check reuse the C4 vocabulary verbatim, and n/a is the one new shape.
RISK-VALUE: DERIVED — body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84 — it equals the enforced cap MaxBodyBytes = 16 * 1024 @ tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go:28 and the schema line at verdict-format.md:43.
RISK-VALUE: DERIVED — delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:679 (full-scope @ :681) — these are the same scope tokens clause 19 defines (Context boundary 2).

**VERIFY: BLOCKED** — 5 of 6 rows pass at 05dc9fbc88f2 (rows 1 to 4 and 6, on the witness). Row 5 is could-not-check: the forge cannot identify the first correctness review dispatched under the contract, and the check-definition question is still open (medici-finance/assay#1927). No implementation failure was observed.
### Re-verification 2026-10-27 (source 0f2b208cebc3)

Fresh non-implementer verifier, dispatched model claude-opus-5-5, tier strong, platform Darwin, git 2.56.0, go 1.27.2, desk tools v1.0.63. The source 0f2b208cebc30af31269e7482b8b9159b602867d equals forge main, read through the forge API before any row. Admission (`deskdispatch --check-verifier`) exited 0 before any row and again after this block was written, with the same binding. The table below is the write-mode `statusgen verifyrun` execution witness, run 2026-10-27T22:08:40Z to 22:09:22Z from the repo root of the detached admitted worktree on a clean tree at the source, with TMPDIR outside the worktree, the Go build and module caches set explicitly, KUBECONFIG=/dev/null and no forge token in the row environment. Every Verify row is class check (row 5 check +flow, row 6 check +neighbour); none is check:ci, so no network-off bracket applies. The frontmatter risk fields are all present and all no, and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row is risk-bearing and no fail-first base run is owed; rows 1 to 4 are shown discriminating against the delivery's parent below.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -q 'spec-intent' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'callers-blast-radius' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'structure-ownership' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -q 'silence is not accounting' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 2 | ``grep -c -E '^[\|] `[a-z0-9-]+`[ ]*[\|]' tools/desk/cmd/deskdispatch/references/review-prompt.md > "${TMPDIR:-/tmp}/dt31-reg.txt"; [ "$(cat "${TMPDIR:-/tmp}/dt31-reg.txt")" -ge 14 ] && echo ok`` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -q -F '## Coverage' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'checked-clean' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'could-not-check' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q '16 KiB' plugins/assay/skills/pr-review-desk/references/verdict-format.md && grep -q 'neither requires nor refuses' plugins/assay/skills/pr-review-desk/references/verdict-format.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -q 'coverage contract' plugins/assay/skills/pr-review-desk/SKILL.md && grep -q 'never removes a base' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `## Coverage` | could-not-run exit=- — prose-led-command: not executed; the first span ## Coverage is a shell comment (a command starting `#` is ignored by the shell: it exits 0 with no output and measures nothing), not a command. Mark the real command with a cmd: code span where one exists; otherwise keep the row a manual procedure with its evidence, never a command that merely exits 0 | sha256: | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt31-sg" . && "${TMPDIR:-/tmp}/dt31-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-27 | assay-verifier-app[bot] @ 0f2b208cebc3 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4674 run=be2af95667950d1050a456fe176bcd2824dc0fc09b82baa01d087f40aee3dd8a source=0f2b208cebc30af31269e7482b8b9159b602867d brief=docs/streams/desk-tools/brief-31-correctness-lane-coverage-contract.md model=claude-opus-5-5 tier=strong

Witness reading. Rows 1 to 4 and 6 exit 0 with output hash dc51b8c96c2d, the SHA-256 of `ok` plus a newline. Row 2's count file holds 17: the 14 register rows at review-prompt.md lines 654 to 667 plus 3 state-table rows (lines 678, 679 and 681). Row 5 is could-not-run on the witness: verifyrun refuses the prose row (prose-led-command), which is the correct handling of a row with no command.

Grounding and delivery. The delivering change is PR #2598, merged as 7808e0f17daa at 2026-10-11T12:48:09Z (merge commit read from the forge this run). It changed six files, none under tools/desk/cmd/deskpost. PR #2633 (c82135abc745) later scoped the grammar colon, and 6564d2da7 later added one disclosure sentence to clause 21 while keeping the refusal rule. The 14 register rows at review-prompt.md lines 654 to 667 match the brief's Task 1 table byte for byte (diff exit 0, this run). Rows 1 to 4 discriminate: at the parent of 7808e0f17daa, spec-intent and silence is not accounting each count 0 in review-prompt.md, never removes a base counts 0 in the skill, and the Coverage heading counts 0 in verdict-format.md. A search of the deskpost non-test sources for "coverage" finds nothing at this source, so the no-enforcement statement (design boundary 3) still holds.

Row 5 (supplementary forge read, read-only, verifier App token, nothing posted; no row credit): the reviewer App's correctness reviews on PR #2601 after the landing, 5483396998 at 2026-10-11T13:02:40Z (head 140e5519fe71) and 5483451854 at 13:19:34Z (head abe260187e27), carry no Coverage heading. Correctness review 5483887086 on PR #2633 at 15:41:37Z (head e7034f9fef92, APPROVED) does carry one. No review body records the dispatcher build it ran under, so the first review dispatched under the contract cannot be identified from the forge. The row text is unchanged at this source, and the check-definition question of which review the row means is still open with no re-authoring or ruling recorded (medici-finance/assay#1927, state OPEN, read this run). Row 5 stays could-not-check; a further re-dispatch cannot change that until the row is re-authored or ruled.

Risk trigger: all frontmatter risk fields are present and set to no, and the gate is model. Enumeration covered the delivering diff 7808e0f17daa, the later clause edits in c82135abc745 and 6564d2da7, and the literals the Deliverables name, re-read at source 0f2b208cebc3:
- register ids = the 14 kebab ids spec-intent through structure-ownership @ tools/desk/cmd/deskdispatch/references/review-prompt.md:654 to :667.
- state tokens = finding, checked-clean, n/a and the unchecked state @ tools/desk/cmd/deskdispatch/references/review-prompt.md:678 to :681; state-count = "four" @ :671.
- delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:683; full-scope = "Scope: FULL PASS" @ :685.
- clause-number = 21 @ tools/desk/cmd/deskdispatch/references/review-prompt.md:639.
- coverage-heading = "## Coverage" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:58.
- base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83; body-budget = "16 KiB" @ :84.

Ranking: none of these is irreversible. Each is doctrine text, so an edit and a release undo it. Rank 1 (a wrong value narrows what a review must account for): register ids, state tokens, base-count, body-budget, scope tokens. Rank 2 (navigation only): clause-number, heading, state-count word.

RISK-VALUE: DERIVED — register ids = the 14 kebab ids @ tools/desk/cmd/deskdispatch/references/review-prompt.md:654 — they match the brief's Task 1 table byte for byte (diff exit 0, this run), and none is a security class (Context boundary 1).
RISK-VALUE: DERIVED — base-count = 14 @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:83 — counted two ways that agree: the brief's Task 1 table has 14 rows, and review-prompt.md lines 654 to 667 hold 14 rows.
RISK-VALUE: DERIVED — state tokens = finding / checked-clean / n/a / the unchecked state @ tools/desk/cmd/deskdispatch/references/review-prompt.md:678 — the brief requires four mutually exclusive outcomes; checked-clean and the unchecked state reuse the C4 vocabulary verbatim, and n/a is the one new shape.
RISK-VALUE: DERIVED — body-budget = "16 KiB" @ plugins/assay/skills/pr-review-desk/references/verdict-format.md:84 — it equals the enforced cap MaxBodyBytes = 16 * 1024 @ tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go:28 and the schema line at verdict-format.md:43 (16384 bytes).
RISK-VALUE: DERIVED — delta-scope = "Scope: DELTA" @ tools/desk/cmd/deskdispatch/references/review-prompt.md:683 (full-scope @ :685) — these are the scope tokens clause 19 defines (Context boundary 2).

**VERIFY: BLOCKED** — 5 of 6 rows pass at 0f2b208cebc3 (rows 1 to 4 and 6, on the witness). Row 5 is could-not-check: the forge cannot identify the first correctness review dispatched under the contract, and the check-definition question is still open (medici-finance/assay#1927). No implementation failure was observed.

## Review
Gate: model (from frontmatter). The reviewer answers, in the verdict: (1) is the register
correctness-only — no class that belongs to the security lane or to a deterministic control?
(2) does every register row owe an answer a competent reviewer could actually give, or is any
row unfalsifiable filler? (3) do the four line shapes keep `checked-clean` and
`could-not-check` in their existing C4 meanings, with `n/a` reserved strictly for a class
that does not apply? (4) does the skill paragraph stay neutral-body (project-layer extension stated,
no house values inlined)? (5) is deskpost genuinely untouched?
