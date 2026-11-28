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
  answers it exactly one of two ways — accept (the finding class resolves as
  `resolved-by-author-disposition`, recorded with the disposition's words as the resolution
  evidence; flip gates untouched — a same-head APPROVE is still not posted, the standing CR
  clears at the next push or by the human) or escalate (clause 20's reviewer-named arbiter
  hand-off, quoted disposition, ONE, filed by the desk — never reviewer-asserted).
  A re-statement round is neither.
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
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md — the finding-class register, round caps, class continuity, and the derived arbiter packet (review-prompt.md:366 on main, cited as ~341 the round cap files ONE arbiter packet and holds the class). All of it governs findings and rounds; no clause governs an author's explicit disposition on a finding. The prompt-audit clause's 'pending disposition' (review-prompt.md:491 on main) is a different concept — a fleet-wide audit baseline's own accept/decline — and must not be conflated."
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
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here (the disposition-finality clause outside the lane markers: an explicit author disposition on a finding class ends the reviewer's argument in accept-or-escalate; a re-statement round is neither; the disposition's words are resolution evidence when accepted; escalate = clause 20's reviewer-named arbiter hand-off)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here (one neutral paragraph in the reviewer's bar: same rule, pointing at the prompt clause)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: out-of-scope (the worker's reply-with-evidence duty already exists; this brief changes nothing on the author side)"
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md: out-of-scope (wrong home — emitted to every dispatched class; the rule binds reviewers only)"
  - "tools/desk/cmd/deskflip: out-of-scope (route (b) chosen for review F3: an accept at an unchanged head records in the finding block; the standing CR clears at the next push or by the human — no new deskflip exemption)"
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
   (with its head), not a code read; `resolved-by-author-disposition` is a legitimate
   resolution, recorded in the verdict's typed finding block so a later reader sees why the
   class closed. **Flip-gate interaction (unchanged by this brief):** the common case — a
   "won't fix" with no new commit — leaves the reviewer's own standing CHANGES_REQUESTED at
   an unchanged head, and the same-head CR-to-APPROVE rule (clause 12) plus deskflip's
   standing-CR refusal still stand. The accepted disposition ends the reviewer's ARGUMENT,
   not the pipeline's gate: the CR clears at the author's next push (a re-review at the new
   head then posts the APPROVE), or the human clears the standing CR. Nothing here asks
   deskflip for a new exemption.
2. **Escalate** — the reviewer names the class in its verdict as an arbiter hand-off for
   the desk to file (review-prompt clause 20's existing reviewer-named hand-off form, the
   one at review-prompt.md:589 on main), quoting the disposition and the finding, when the
   reviewer believes the disposition is wrong in a way that matters (safety-consequence, a
   control weakened, a spec violated). ONE hand-off, never a fresh finding round. The
   hand-off changes nothing about blocking: Escalate files the human packet WITHOUT
   touching the finding's blocking status — on the security lane the verdict stays `fail`
   while the hand-off is pending; Escalate adds the packet, it never replaces the block.
   The derivation and filing stay the desk's, never the reviewer's own packet.

A re-statement round is neither. A disposition is not re-opened by a later reviewer round
absent NEW evidence that the disposed concern materially regressed — a code change alone is
not new evidence about the disposition, though it can be new evidence about the code.

**Boundaries (so the worker does not over-broaden the rule):**

- **Applies to an EXPLICIT disposition.** Silence, a bare "done" with no answer to the
  finding, or a fix attempt that does not address the finding is not a disposition — the
  ordinary rules (inadequate-fix reply, round cap) govern those.
- **Never for safety-consequence findings, and never for the security lane.** A
  disposition on a safety-consequence finding escalates, full stop; it is never accepted by
  the reviewer alone. And the security lane, or any finding in the `security` or
  `public-surface / leak` class, never takes the Accept move at all: a finding that would
  fail stays a fail at every round, whatever the disposition — mirroring the existing rule
  that the cap never changes a security verdict. Only a recorded human ruling clears it.
  Without this boundary the reviewed party — routinely the worker role App — could clear
  the independent security lane's own finding in words.
- **The disposition is the author's (or a trusted human maintainer's), not a third
  party's.** A comment from an untrusted or unblessed login blesses nothing (trust gate);
  a disposition carries authority from the change's author identity — where the author is
  a role App, a trusted human maintainer's disposition on the same finding carries the same
  finality — and only when that identity passes the same trust gate as any other actionable
  comment: this repo is public, PRs arrive from untrusted outside authors, and an untrusted
  author's disposition is not a disposition at all; the ordinary rules govern.
- **Nothing here changes the flip gates.** A resolved class still needs the checks, the
  security lane, and the merge-time re-check; disposition finality governs the reviewer's
  argument, not the pipeline's gates. An accept at an unchanged head does NOT post a
  same-head APPROVE (clause 12 holds); it is recorded in the finding block and the standing
  CR clears at the next push or by the human — the Accept move's flip-gate interaction is
  stated in full above, so the reviewer never has to guess.

files:
- `tools/desk/cmd/deskdispatch/references/review-prompt.md` — Task 1 (the clause, outside the lane markers)
- `plugins/assay/skills/pr-review-desk/SKILL.md` — Task 2
- `changelog/desk-tools-33.md` (planned) — Task 3

## Task

**Task 1 — the clause.** Add the disposition-finality clause to
`tools/desk/cmd/deskdispatch/references/review-prompt.md` OUTSIDE the lane markers (the
next numbered clause), so BOTH lane cuts carry it. NOT `common-clauses.md` — that kit is
emitted to every dispatched class (workers and verifiers too), and the rule's two legal
moves are the reviewer's alone. One home, no second copy, no pointer needed.
The clause states the rule, the two legal moves, the resolution-evidence shape
(`resolved-by-author-disposition` + the disposition's words + its head), the
safety-consequence carve-out, the security-lane boundary (the clause carries the sentence
`never takes the Accept move` for the security lane and the security / public-surface /
leak classes, only a recorded human ruling clearing a fail) and the hand-off-changes-nothing
rule, the explicit-only and trust-gated disposition boundary, and the no-reopen rule with
its new-evidence exception.

**Task 2 — the skill paragraph.** One neutral paragraph in the pr-review-desk skill's
reviewer's bar stating the rule and pointing at the clause's home.

**Task 3 — changelog fragment.** changelog/desk-tools-33.md (planned), one bullet.

## Verify (executable — no prose-only DoD items)

Rows 1–3 are discriminating: each FAILS against today's tree (at 69ca08c69) and passes only
once the Task lands. Row 4 passes today and must still pass. Run from the repo root.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -rq 'resolved-by-author-disposition' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | prints `ok`: the clause exists in its home with its named resolution form (`disposition` alone cannot discriminate — the prompt-audit baseline already uses the word). FAILS today: the string appears 0 times in the reference. |
| 2 | check | `grep -rq 're-statement round is neither' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | prints `ok`: the clause's own sentence for the illegal third move is present in its home (a phrase the clause introduces — `safety-consequence` and `restate` both already exist in the reference today, so neither can discriminate; alternations are written as repeated `-e`, never a `\|` BRE inside a cell). FAILS today: the phrase appears 0 times. |
| 3 | check | `grep -q 'disposition' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | prints `ok`: the skill's reviewer's bar carries the rule's paragraph. FAILS today: the word appears 0 times in the skill. |
| 4 | check +neighbour | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt33-sg" . && "${TMPDIR:-/tmp}/dt33-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | prints `ok`: tree lints clean, skillslint passes. The changelog fragment is deliberately NOT tested — the release workflow clears `changelog/` at aggregation. Passes today and must still pass. |
| 5 | check | `grep -rq 'never takes the Accept move' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | prints `ok`: the security-lane boundary sentence the clause must carry (S1's fix) is present in its home. FAILS today: the phrase appears 0 times in the reference. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -rq 'resolved-by-author-disposition' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-11-25 | assay-verifier-app[bot] @ 9c284587ff37 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -rq 're-statement round is neither' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-11-25 | assay-verifier-app[bot] @ 9c284587ff37 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -q 'disposition' plugins/assay/skills/pr-review-desk/SKILL.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-11-25 | assay-verifier-app[bot] @ 9c284587ff37 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt33-sg" . && "${TMPDIR:-/tmp}/dt33-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && cd ../tools/skillslint && go run . --root ../.. > /dev/null 2>&1 && echo ok` | fail exit=1 | sha256:e3b0c44298fc | 2026-11-25 | assay-verifier-app[bot] @ 9c284587ff37 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -rq 'never takes the Accept move' tools/desk/cmd/deskdispatch/references/review-prompt.md && echo ok` | pass exit=0 | sha256:dc51b8c96c2d | 2026-11-25 | assay-verifier-app[bot] @ 9c284587ff37 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7784 run=98968309a37e390c65f42fbd431db3077da702864934331ef4af02a1c2f11046 source=9c284587ff373eda0e606739b863474e6ccd3bff brief=docs/streams/desk-tools/brief-33-author-disposition-finality.md model=claude-opus-5-5 tier=strong

**VERIFY: FAIL** — row 4 exit 1 (the repo-wide offline lint reports 3 PROBLEMs, none on this brief); rows 1, 2, 3 and 5 pass.

Post-merge verification on 2026-11-25 by a non-implementer verifier (model claude-opus-5-5,
tier strong) at merged main 9c284587ff37, desk tools v1.0.94, darwin arm64 host, declared
verifier Git environment, admitted by the attestation line above. The delivery is the merge
f7621fc3966b (delivering commits 864570413110 and 488211ef78dc), an ancestor of the source.
Every row is classed check (none check:ci), so the rows ran on the host and no container
bracket applies. The witness table above was written by the installed statusgen verifyrun;
it is the second of two host attempts in this dispatch (the first ran the same five
commands from a script over the same tree, with the same per-row results).

- Rows 1, 2 and 5: each prints ok. Clause 22 of the review kit carries the
  resolved-by-author-disposition resolution form, the sentence "A re-statement round is
  neither" and the security-lane sentence "The security lane never takes the Accept move".
- Row 3: prints ok. The pr-review-desk skill body carries the disposition paragraph (6
  matching lines).
- Row 4: exit 1, no ok, empty output; the Expect is ok, so the row is graded fail under
  rule 37. A supplemental diagnostic outside both attempts (no acceptance credit) showed the
  row's own go build succeeds, its built lint exits 1 with LINT: FAIL 3 problem(s), and the
  skillslint stage exits 0. The three PROBLEMs are desk-tools/37, desktools-v2/65 and
  forge-neutral/54, each "protected table: acceptance transition: could-not-check human
  authority: --acceptance-forge is required". The pinned statusgen v1.0.94 offline lint over
  the same tree gives the same three PROBLEMs and LINT: FAIL. No PROBLEM line names
  desk-tools/33. The row's compound command is outside the rule-51 command class, so no
  rule-51 stamp applies and none was attempted. Open issue for this cause:
  medici-finance/assay#4823.

Attempts and credit: two host attempts, five rows each. Credit is settled by the desk at
landing; nothing here states a credit result.

Risk-bearing values: the brief's risk fields are all no and not irreversible, and the diff
touches no risk-classed path. Enumeration over the delivery diff (review kit clause 22, the
skill paragraph, the mutation and test entries, the changelog fragment) found two prose
bounds, both reversible by an edit.

RISK-VALUE: DERIVED — Escalate hand-off bound = ONE @ tools/desk/cmd/deskdispatch/references/review-prompt.md:776 — the brief's Context fixes Escalate as one reviewer-named hand-off, never a fresh finding round.
RISK-VALUE: DERIVED — legal moves after a disposition = exactly one of two @ tools/desk/cmd/deskdispatch/references/review-prompt.md:756 — the brief's rule names exactly Accept and Escalate, and a re-statement round is neither.


## Review
Gate: model (from frontmatter). The reviewer answers, in the verdict: (1) is the rule
scoped to EXPLICIT dispositions only, with silence and bare "done" left to the ordinary
rules? (2) does the safety-consequence carve-out escalate always, accept never — and does
  the security lane (and every security / public-surface / leak class) never take the Accept
  move, with Escalate changing nothing about blocking? (3) does
the clause sit in exactly ONE home with the other file pointing at it, not duplicated?
(4) is the new-evidence exception stated narrowly enough that a code change alone cannot
reopen a disposed class? (5) does nothing here touch the flip gates or the worker-side
reply-with-evidence duty?
