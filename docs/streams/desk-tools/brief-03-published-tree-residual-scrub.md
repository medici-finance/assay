---
brief: desk-tools/03
title: Published-tree residual-identity scrub — drive the cold-read to an independent CLEAN
why: >-
  When a tree is published from a private origin, a tokens-only leak-sweep passing over it does
  not make it identity-clean: doc-comments, write-indicator rules, and test fixtures can still
  read as the origin's product long after the risk-path config is genericized. checked-clean is
  necessary, never sufficient. This brief owns the mechanical genericization of the residual
  origin-identity in the published `tools/desk` tree, ratchets the residue class into the
  leak-sweep token set so it cannot regress silently, and gates "done" on a FRESH independent
  cold-read of the re-staged tree returning CLEAN — the mechanical sweep is not self-certifying,
  and that is the whole lesson.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-12 by Fable session; re-homed to the desk-tools board 2026-08-26
sources:
  - "A multi-pass independent cold-read of the staged desk-tools tree returned HAS-BLOCKERS: the tree still read as its private origin's product in compiled-source doc-comments, a write-indicator rule, and test fixtures that the prose and config passes had not touched. A tokens-only leak-sweep passed over that same tree, re-proving that checked-clean is not identity-clean."
  - "The leak-sweep tool and its three-state interface (checked-clean / checked-failed / could-not-check), and the token-file floor rules that bind an ADD-only ratchet."
exec-tier: strong
exec-tier-why: >-
  The fixture half is safety plumbing where a subtle error survives green tests: the classifier
  fixtures encode token-SHAPE cases (case segmentation, path-segment shape, digit placement); a
  replacement that changes the shape quietly stops exercising the branch it pinned while the
  suite stays green.
gate-why: >-
  The human gate carries the anti-self-certification lesson: they confirm (1) the FRESH cold-read
  was run by an INDEPENDENT session — not the implementer, not this brief's author — against the
  RE-STAGED tree, and returned CLEAN; and (2) fixture replacements preserved classifier-relevant
  token SHAPES so the detection tests still exercise the branches they pinned (a weakened fixture
  is a silent detection regression no green suite reports). `sensitive-data: no` here: this
  re-homed brief carries the mechanism only — the enumerated residue set was the withheld value
  and is not restated on a public surface.
---

# Brief 03 — Published-tree residual-identity scrub — drive the cold-read to an independent CLEAN

The published `tools/desk` tree is a fresh, history-free copy of source developed from a private
origin. This brief mutates SOURCE content (comments, examples, fixtures) so the tree stops
reading as the origin's product. It certifies nothing and publishes nothing on its own: the
mechanical scrub feeds the standing security-review gate and the human public-copy gate, and
replaces neither.

> **Withheld value.** The concrete residue set — the exact origin-specific tokens, the file:line
> locations, and the product vocabulary the cold-read found — is the withheld value of this scrub
> and is NOT restated here. The mechanism is public; the enumerated targeting aid stays out of
> any public surface. An implementer works the residue from the private cold-read record, never
> from a public restatement.

## Dependencies
The risk-path / taxonomy / grouping briefs this originally depended on (which genericize the
compiled origin-map before the cold-read can return CLEAN) have landed outside this stream, so no
typed `depends:` edge remains. The mechanical scrub may be prepared in parallel, but the terminal
Verify rows run against a tree with all of those landed.

## Context
files: the residual-scrub edit surface across `tools/desk` compiled-source doc-comments and test
fixtures (worked from the private cold-read record — the greps are the contract, not restated
here), plus the leak-sweep token set (ADD-only ratchet).

facts:
- `leaksweep run --tree DIR` is three-state and TOKENS-ONLY, case-sensitive — which is exactly
  why the terminal gate is a fresh independent cold-read, not the sweep. Checked-clean is
  necessary, never sufficient.
- The staging step materialises the tree for the copy disposition; it must report STAGED (exit 0).
  INCOMPLETE/REFUSED is a STOP routed to the publication owner, never worked around here.
- token-SHAPE preservation rule: a fixture replacement must preserve the classifier-relevant
  shape of what it replaces — length class, case pattern, digit placement, separator/segment
  structure. Same-shape is what keeps the detection tests exercising the same branches.
- the module path and repo self-references for the public home are ACCEPTED (a kept
  self-reference); the residue greps are scoped so they do not flag it.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity
  does).
- Never touch the risk-path compiled base or the live write-indicator rule — those belong to the
  briefs that own them, behind their own ordering guards; scrubbing them here would silently
  weaken a guard. This brief owns only the PROSE and fixture residue those briefs leave behind.
- The token set is ADD-only here: no entry removed, no `match:`/`control:` weakened, no
  directory-prefix `allow:` — a widening ratchet only.
- Replacements preserve classifier-relevant token SHAPE — never swap a fixture for a token of a
  different shape class.
- Stream grep conventions apply: no `\b`, no `\|` alternation inside `-E`, every negative grep
  carries a positive control, capture to a file and test `$?`.
- This brief certifies nothing and publishes nothing; the copy is the human's act behind the
  security-review gate and the human copy gate.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Re-grep every residue item at pickup from the private cold-read record (line numbers drift;
   the greps are the contract). Report any item already fixed or newly moved as an annotation,
   not silence.
2. Apply the residual-genericization mechanically, shape-preserving. Confirm-and-keep any
   methodology-native vocabulary (the register directory names, etc.), recording the
   confirmation rather than editing it.
3. ADD the ratchet token entries for the residue class (all `ci: true`, each with a valid
   `match:` + `control:`), under the token-file floors — case-folding included, so a
   camel-cased survivor of a rename is still caught.
4. Build/test/format green (Verify rows 1–2); residue greps empty with positive controls
   (rows 3–5); fixture-coverage neighbour check (row 6) shows no coverage thinned.
5. Re-stage and sweep: staging STAGED, leaksweep certificate with zero could-not-check (rows
   7–8), ratchet mutation red (row 9), token-file floors hold (row 10).
6. Dispatch the FRESH INDEPENDENT COLD-READ of the re-staged `tools/desk` tree to a session that
   neither implemented nor authored this brief; its verbatim verdict goes in Evidence (row 11).
   CLEAN is the terminal condition; HAS-BLOCKERS re-opens the residue set.
7. Do not run the copy.

## Verify (executable — no prose-only DoD items)
Rows 7–11 require the dependency briefs landed — run them against a tree with all in; recording
them earlier is BLOCKED, never PASS. Row 11 is a PRESENCE gate on the independent verdict
existing and naming its runner; its quality/independence is the human review gate's item.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 |
| 2 | `gofmt -l tools/desk > /tmp/dt03fmt.txt; test ! -s /tmp/dt03fmt.txt; echo $?` | `0` — no unformatted file |
| 3 | residue grep (origin-product surface — patterns from the private record): capture to a file, `test ! -s`, `echo $?` | `0` — no origin-product residue anywhere in `tools/desk` |
| 3a | positive control for row 3: the same grep at the branch merge-base, `test -s`, `echo $?` | `0` — same grep shape hits at the base, so row 3's empty means absent, not broken |
| 4 | residue grep (origin names/paths — patterns from the private record; separate `-e`, never `-E` alternation), `test ! -s`, `echo $?` | `0` — empty. The accepted self-ref is outside this pattern set: the public module home stays |
| 5 | residue grep (operating record — real SHAs, dated provenance IDs), `test ! -s`, `echo $?`; positive control at the base is non-empty | `0` — the operating record is gone |
| 6 | NEIGHBOUR row (fixture coverage did not thin): diff the deskkit test/subtest name set at HEAD vs base; `test ! -s /tmp/dt03lost.txt; echo $?` | `0` — no deskkit test or subtest present at base is missing at HEAD (a rename must be justified 1:1 in the PR body) |
| 7 | staging: run the staging step to a temp out dir; `echo $?` | `0` — STAGED. INCOMPLETE/REFUSED is a STOP routed to the publication owner |
| 8 | leaksweep certificate (three-state) over the staged tree: exit 0, zero `could-not-check`, positive `checked-clean` count | `0`,`0`,`0` — NECESSARY, NOT SUFFICIENT; row 11 is the gate |
| 9 | ratchet mutation: drop a probe file carrying a residue token into the staged tree, re-sweep | **non-zero**, the sweep reports `checked-failed` naming the token and the probe file; re-run row 8 → clean again |
| 10 | token-file floors: run the token-file floor checks verbatim against this branch | `0`,`0`,`0` — no directory-prefix `allow:`, base token set ⊆ head, base tuple set ⊆ head (ADD-only) |
| 11 | FRESH INDEPENDENT COLD-READ (terminal gate): an independent session — NOT the implementer, NOT this brief's author — cold-reads the staged `tools/desk` end-to-end for origin/product identity; its verbatim verdict + runner identity are pasted into Evidence | verdict `CLEAN`. PRESENCE gate only; HAS-BLOCKERS re-opens the residue set, and the sweep's green (row 8) does not overrule it |
| 12 | `cd statusgen && go run . --root .. --lint; echo $?` | `0` |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Row 11's cell carries the independent cold-read verdict VERBATIM plus the runner identity;
     the terminal rows (7 through 11) are recorded BLOCKED (never PASS) until the dependency
     briefs have landed. The
     "verified" status requires this section filled by someone who did NOT implement. -->

## Review
Gate: human. The reviewer confirms: (1) row 11's cold-read was run by a genuinely independent
session against the RE-STAGED tree and returned CLEAN — a green row 8 with row 11 unevidenced is
indistinguishable from the failure mode this brief exists to close; (2) fixture replacements are
shape-preserving and row 6 shows no coverage thinned; (3) the token-file diff is ADD-only under
the floors; (4) the risk-path base and the live write-indicator were not touched; and (5) the
standing gates — security-review and the human public-copy gate — are still in front of the copy:
this brief's CLEAN feeds them and replaces neither. The enumerated residue set is never restated
on a public surface.
