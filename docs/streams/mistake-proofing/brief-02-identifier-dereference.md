---
brief: mistake-proofing/02
title: Dereference named identifiers, not just backticked paths — test and function names must resolve
why: >-
  The lint already refuses a brief that names a backticked FILE which does not exist on disk. It says
  nothing about a brief that names a backticked TEST or FUNCTION which exists nowhere — and that is
  the form the measured incident actually took: three briefs reached "implemented" in one pass citing
  three test names that were in no file. A presence check on a claim is judgment inspection; making
  the claim resolve against the tree it describes is source inspection on the claim itself. The
  machinery to do it shipped years of commits ago and needs one more matcher.
wave: 0
depends: []
unblocks: ["mistake-proofing/05"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the mistake-proofing board)
sources:
  - "`docs/mistake-proofing.md` §4 B4: 'A test name, function name, file path, or link named in a brief or its evidence MUST resolve against the tree it describes.' Also §5 adoption ladder step 2 — B4 is one of the two highest value-per-cost moves."
  - "A measured incident: three briefs in one pass reached `implemented` overstating their state, including three named tests that existed in no file. Every presence-check row passed on a factually wrong deliverable. Making the identifier resolve is the recommendation that came out of it, and it has been unbuilt since."
  - "The device inventory behind this stream (2026-08-25): 'the single cheapest open item with a named, dated incident behind it', cost S. Names the two shipped precedents to copy — the backticked-path link check, and the consumers check's three-state epistemics (CORROBORATED / DISPROVED / UNCHECKED)."
  - "The adjacent disclosed divergence: the existing backticked-path check exempts directory-shaped targets by construction, so its coverage depends on the target's file extension. Do not widen that exemption; do record where this brief's matcher inherits it."
  - "`docs/three-state-instrument-rule.md` — 'absence of evidence is not evidence of absence': an unsearchable tree is could-not-check, never a clean read."
  - "freshness-checked 2026-08-25 @ 657cab1 (origin/main) — the link checker's matcher is built from a file-EXTENSION list only; no identifier-shaped matcher exists in the lint tree."
---

# Brief 02 — Dereference named identifiers

## Context

files:
- `statusgen/` (implementation home) — the link-checking source file that already builds the
  backticked-path matcher, plus a new identifier matcher, plus tests.

facts:
- The existing mechanism is a regex built from an extension list, applied to backtick-delimited
  spans, with three escapes already designed in: a `(planned)` / `(new)` / `(future …)` suffix marks
  a deliverable that does not exist yet; a target beginning with a sibling-repo prefix is left
  unchecked because it names a file outside this tree; and a bare filename with no directory
  separator is skipped as too ambiguous to resolve. **All three escapes must survive this change** —
  they are what keeps the check usable, and the last one is precisely the case this brief has to
  reopen for identifiers.
- The scope is deliberately narrow: the backticked-path discipline is applied ONLY to the
  authored, convention-bound surfaces (the agent-instruction file plus briefs under the streams
  tree), because outbound narrative documents legitimately mention absent paths in prose. **Keep that
  scope.** Widening it to articles and specs is how this check becomes the thing people disable.
- The measured incident is about test names, so `<Something>_test.go` basenames and `Test<Name>`
  identifiers are the highest-yield shapes, followed by `func <Name>` references. A bare
  `helper_test.go` is skipped today by the no-directory-separator rule, which means the exact token
  from that incident sails through.
- The epistemics to copy are already worked out in the lint tree's consumer-routing check: three
  states, with the disproved state fatal, and a design note that the file "never asks 'is the field
  there?'". That is the right posture — this check asks whether the NAME RESOLVES, not whether a
  name is present.
- The spec's rule covers "a brief **or its evidence**". The Evidence section is where the incident's
  false test names actually lived, so the matcher must run over the whole brief body, Evidence
  included, not just the Task.
- Search cost matters: the brief corpus grows and the tree is large. Resolve identifiers with a
  single indexed pass over the source tree (one walk, one map from identifier to file), not a
  process spawn per token. An unreadable tree or a missing git dir is **could-not-check**, reported
  as such, never a silent clean read.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Add an identifier matcher beside the existing path matcher**, applied to the same backtick-
   delimited spans and the same narrow scope. Three shapes, and no more in this brief: a test-file
   basename ending `_test.go` with no directory separator; a `Test<Name>` identifier; and a
   `func <Name>` reference. Anything not matching one of the three is not this check's business —
   an over-eager matcher on prose is how a check earns a permanent exemption file.
2. **Resolve each match against the tree**, using one indexed pass. A test-file basename resolves if
   a file with that basename exists anywhere in the tree. A test or function identifier resolves if
   a declaration of that name exists. Resolution is by name only — this check does not verify that
   the test passes, or that the function does what the brief says. State that boundary in the
   failure message.
3. **Honour every existing escape, and add exactly one.** The `(planned)` family, the sibling-repo
   prefix, and the narrow scope all carry over unchanged. The one addition: a brief may mark an
   identifier as illustrative with the same `(planned)` suffix family, so a brief describing a name
   it is about to create is not blocked. Do not invent a second escape syntax.
4. **Three states.** Resolves → pass. Does not resolve → report. Cannot search (unreadable tree, no
   git dir, indexing failed) → could-not-check, printed, and the whole check declines rather than
   passing. Carry a stable rule-tag bracket token on every emitted line.
5. **Phase the severity, and commit to the flip.** Land as an advisory NOTICE with a corpus census
   recorded in the pull-request body, then flip to a fatal PROBLEM once the inherited hits are fixed
   or waived with the escape. Name the flip date or condition in the pull-request body. A permanent
   NOTICE is not an acceptable landing state for this check.
6. **Positive control.** A test that injects a brief naming a test that does not exist and asserts
   the check fires; and its inverse, a brief naming a test that does exist, asserting it does not.
   Add a third case reproducing the incident's shape — a bare `_test.go` basename with no
   directory separator — to prove the reopened case is genuinely covered.
7. **Record the inherited exemption.** The existing matcher's directory-shaped exemption is a
   disclosed divergence and this brief does not fix it. Note in the check's source header which
   shapes remain unchecked, so the coverage boundary lives beside the check rather than in a
   document nobody reads (spec §3 D6).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -c 'Test\[A-Z\]' -- statusgen/` | exit 1, no output — **DEREFERENCE, true at authoring (2026-08-25 @ `657cab1`)**: no identifier-shaped matcher exists in the lint tree. Inverts at implementation |
| 2 | `git grep -n 'func buildBacktickRe' -- statusgen/linkcheck.go` | exit 0; output contains `func buildBacktickRe` — **DEREFERENCE**: the matcher-construction seam this brief extends really exists |
| 3 | `git grep -n 'plannedRe' -- statusgen/linkcheck.go` | exit 0 — **DEREFERENCE**: the `(planned)` escape this brief must preserve and reuse ships today |
| 4 | `git grep -n 'too ambiguous to resolve' -- statusgen/linkcheck.go` | exit 0 — **DEREFERENCE**: the bare-filename skip is real, which is exactly why the incident's token passes today |
| 5 | `go test ./statusgen/ -run 'IdentifierDereference' -count=1` | exit 0 — the identifier check's own tests pass (fails today: no such test) |
| 6 | `go test ./statusgen/ -run 'IdentifierDereferenceFiresOnMissingTestName' -count=1` | exit 0 — **positive control**: an injected brief naming a nonexistent test fires the check; the same brief naming a real test does not |
| 7 | `go test ./statusgen/ -run 'IdentifierDereferenceCouldNotCheck' -count=1` | exit 0 — an unindexable tree reports could-not-check and declines, rather than reading clean |
| 8 | `go test ./statusgen/ -run 'LinkCheck' -count=1` | exit 0 — the existing path-checking behaviour is unchanged; the three inherited escapes still hold |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table. Reviewer questions specific to this brief: (1) do all three existing escapes
and the narrow scope survive unchanged? (2) does the matcher fire on the incident's exact shape (a
bare test-file basename), and is there a test proving it? (3) is resolution one indexed pass rather
than a process spawn per token?
