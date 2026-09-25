---
brief: assay:assay:desk-tools:03
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
schema: brief-v2
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
version: 1
id: 81a0f56b-2959-4b38-8c31-13e58b440b28
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

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 893cd6114b03, claude-opus-5-5[1m], on-behalf-of human:ian) — VERIFY: FAIL

Execution witness (`statusgen verifyrun --brief`, statusgen built from this tree, verbatim):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — timed out after 10m0s — no verdict was produced | sha256:42dd641b0cfd | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 2 | `gofmt -l tools/desk > /tmp/dt03fmt.txt; test ! -s /tmp/dt03fmt.txt; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 3 | `test ! -s` | fail exit=1 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 3a | `test -s` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 4 | `-e` | fail exit=1 | sha256:fa916b3e66f6 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 5 | `test ! -s` | fail exit=1 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 6 | `test ! -s /tmp/dt03lost.txt; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 7 | `echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 8 | `could-not-check` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:273baddc86ad | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 9 | `ratchet mutation: drop a probe file carrying a residue token into the staged tree, re-sweep` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:2da8cf147470 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 10 | `token-file floors: run the token-file floor checks verbatim against this branch` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:051f6881cdbe | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 11 | `tools/desk` | could-not-run exit=126 — the shell could not execute the command (exit 126) | sha256:c6ef28401ddf | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 | sha256:9b9e71858974 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |

Hand-written rows for the same pass. The witness table above is the verbatim output of `statusgen verifyrun` built from this tree. For rows 3 to 11 the Verify table's Command cell is prose rather than a runnable command. The witness therefore ran whatever backticked fragment it found in each cell (`test ! -s`, `-e`, `echo $?`, `tools/desk`, and so on), and those witness verdicts measure the fragment, not the check. The witness passes for rows 3a, 6 and 7 are vacuous for that reason. The witness pass for row 2 is the exit status of the trailing `echo $?`, and the value that command printed was `1`. The rows below record what was actually measured. The row 3 to 5 grep patterns live in the private cold-read record and are not restated here, per this brief's withheld-value rule. Merged main is 893cd6114b0382a1f71e6ef763c6601a5e19d270, run on darwin/arm64 with go1.26.5, offline.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | tools/desk go build and go test, run directly with no per-row wall clock (the witness hit its 10m default) | exit 0 | exit 1 on the full run: go build ./... clean, 83 packages ok, 2 FAIL (cmd/commsloop, 1 test; internal/loopengine, 7 tests), every failure a 5s-deadline timing assertion (5.00-5.01s) while the host ran ~50 concurrent test binaries from other sessions. Isolated re-run of just those two packages (go test -count=1): exit 0, both ok (8.3s, 12.2s). Load-sensitive timing tests, outside this scrub's surface (loopengine is the repo's known load flake, #612); recorded as observed, not as a pass of the row as written | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | gofmt -l over tools/desk, then test ! -s on the capture, then echo $? | prints 0 | FAIL as written: printed 1, and 7 files were flagged. All 7 were last touched by other briefs between 2026-09-06 and 2026-09-23, so none is on this scrub's surface. The one diff inspected is comment-alignment whitespace; the module declares go 1.25.0 and the local toolchain is go1.26.5, so gofmt version skew is possible and was not ruled out | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | the private record's origin-product residue greps (a case-insensitive substring grep plus a word grep), run against origin/main -- tools/desk, then test ! -s | 0 | 0 and 0: both captures empty | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3a | positive control: the same row 3 grep at the pre-scrub baseline of the private origin, then test -s | 0 | 0: 203 hits at the baseline, so the empty row 3 result means the residue is absent and the grep still works | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | the private record's origin names/paths greps (separate -e patterns) plus its infra-layout grep (risk-path files excluded), against origin/main -- tools/desk | 0 | 0 and 0: both empty. Positive controls at the baseline were non-empty for both grep sets | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | the private record's operating-record grep (real SHA, provenance ids, dated ref) against origin/main -- tools/desk | 0, with the baseline control non-empty | 0: empty. The baseline control was non-empty | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | NEIGHBOUR row: diff of the deskkit top-level Test function name set, pre-scrub baseline vs origin/main. This is a static name diff, because the row's branch merge-base does not exist here: the implementation landed in the private origin and the home-handoff, not as a public branch | no base name missing at HEAD | could-not-check as written: no merge-base exists. The static diff gives 208 base names and 1076 at HEAD, with 7 base names missing. 1 is the scrub's own 1:1 rename to TestTrustGoldenRoster. 5 were removed by desk-tools/18 (#812, the per-item approval gate retirement). 1 is a risk-path test that is absent from the public tree since its first commit and belongs to the risk-path briefs. None of the 7 is a coverage loss from this scrub | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | staging step | STAGED, exit 0 | could-not-check: this repo has no staging step; the public tree is the end state. A git archive export of origin/main tools/desk (1688 files) stood in as the swept surface for rows 8 and 9 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | leaksweep certificate over the tools/desk export, gitleaks and trufflehog both present, private token set (80 entries) | exit 0, zero could-not-check, checked-clean over 0 | FAIL, NOT CERTIFIED: exit 2, entries=80 clean=78 failed=2 unchecked=0 files-swept=1688. (a) One identity-token entry fired on one test file. Every case-insensitive occurrence of its token in that file is the sanctioned public org self-reference, and the hit does not reproduce when the identical bytes are swept at another path, so this is likely a sweep-side false positive for the sweep owner to confirm. (b) trufflehog's own GitLab/URI detectors fired on 18 files of forge-client code and test fixtures; that is not this brief's residue class, and the files were not triaged one by one in this pass | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | ratchet mutation: a probe file carrying a residue-class token placed in the export, then a re-sweep, then the probe removed and the sweep re-run | non-zero, checked-failed naming the token and the probe file, then back to the row 8 baseline | PASS: exit 2, and the residue entry reports checked-failed naming PROBE.md. After removal the re-sweep is back to the row 8 baseline (failed=2, the same two entries), which is not a clean certificate because row 8 is red | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | token-file floors (no directory-prefix allow, token set ADD-only, (token, match, control) tuple set ADD-only) on the private token set at its main | 0, 0, 0 | 0, 0, 0 against the base of the last residue-class ratchet (77 tokens to 79, 19 allow paths, none ending in /). Against the pre-scrub baseline F6/F7 print 1: one token left the set. That removal is a separate, recorded human public-ruling de-registration, not this brief's ratchet | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 | FRESH INDEPENDENT COLD-READ verdict, present with its runner | verdict CLEAN | could-not-check: this brief's Evidence carries no independent CLEAN verdict, so the presence gate is unmet. The latest independent cold-read in the private record (2026-09-20, a non-implementer verifier) returned HAS-BLOCKERS with two findings. At 893cd6114b03, one of the two is no longer present in tools/desk and is now covered by an ADD-only ratchet; the other is still present and waits on the human security call. This pass ran targeted probes only, not an end-to-end read, so it issues no verdict | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 12 | cd statusgen, go run . --root .. --lint | 0 | PASS: 0, matching the witness row | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

RISK-VALUE (kit §4). No public diff exists for this brief, because the implementation landed in the private origin and the public home-handoff. The enumeration covers this brief's Deliverables: the fixture replacements in tools/desk, and the residue-class ratchet block (6 entries) in the private token set. Tokens are withheld; only fields and line numbers are cited.
- RISK-VALUE: DERIVED — goodCommit = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee00" @ tools/desk/cmd/desksourceguard/verify_test.go:169 — this is the top rank, because publishing a real internal SHA to public history cannot be undone. The classifier it feeds is fullSHA = ^[0-9a-f]{40}$ @ tools/desk/cmd/desksourceguard/verify.go:64. The synthetic value is 40 lowercase hex characters, which is exactly that shape class, so the test still exercises the full-SHA branch. The row 5 grep confirms the real value is gone.
- RISK-VALUE: NAMED, NOT DERIVED — match = word on two of the six ratchet entries @ private token set:1082 and :1094 — OPEN QUESTION: Task 3 requires case-folding "so a camel-cased survivor of a rename is still caught", but word matching does not fire on camel-case compounds. Measured with probes: `<token>Repo` and `foo<Token>()` produce no hit, while the bare, all-caps and dotted forms do. The private record names such a compound as a residue shape. The entry's own stated reason ("word-matched so it cannot bleed into a larger token") conflicts with Task 3, and choosing between them is an owner call that this verifier cannot derive. Today's greps (row 3) show no such compound in the tree, so this is a regression-control gap, not live residue.
- RISK-VALUE: DERIVED — ci = true on all six ratchet entries @ private token set:1083, :1095, :1105, :1115, :1129, :1146 — Task 3 of this brief requires case-folding for the residue class, and all six carry it.
- RISK-VALUE: DERIVED (low rank, reversible) — control = assay on all six entries — the control string must be present in the swept tree for a rule to count as live, and it is present across the public tree. The row 8 output reports every entry's control present.

VERIFY: FAIL. Row 1's full-suite run is red on two load-sensitive timing packages, which pass when re-run in isolation. Row 2 fails as written (7 unformatted files, none on this scrub's surface). Row 8 is NOT CERTIFIED (failed=2). The terminal row 11 has no independent CLEAN, and the last recorded cold-read was HAS-BLOCKERS with one finding still present. Rows 3 to 11 of this Verify table cannot be executed as written (check-definition), so the witness verdicts for those rows measure fragments. The mechanical residue greps (rows 3 to 5, with their controls firing) are clean, and the ratchet fires (row 9). gate: human. Status stays implemented. Evidence only, with no flip.

## Review
Gate: human. The reviewer confirms: (1) row 11's cold-read was run by a genuinely independent
session against the RE-STAGED tree and returned CLEAN — a green row 8 with row 11 unevidenced is
indistinguishable from the failure mode this brief exists to close; (2) fixture replacements are
shape-preserving and row 6 shows no coverage thinned; (3) the token-file diff is ADD-only under
the floors; (4) the risk-path base and the live write-indicator were not touched; and (5) the
standing gates — security-review and the human public-copy gate — are still in front of the copy:
this brief's CLEAN feeds them and replaces neither. The enumerated residue set is never restated
on a public surface.
