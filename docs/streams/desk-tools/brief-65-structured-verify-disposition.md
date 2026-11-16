---
brief: assay:assay:desk-tools:65
title: "Read each Verify row's disposition from a field the verifier tooling writes, not from Evidence prose"
why: >-
  The flip from implemented to verified refuses when Evidence reads HELD or could-not-check on
  a row not marked deferred. It finds those words by scanning free prose, and six exclusion
  rules (with a seventh proposed in #6638) try to tell a real hold from the same word inside
  quoted command output, a risk definition, an exit-code mapping or a count such as
  "Could-not-check (3)". Each rule fixes one false positive and the next prose shape trips
  the gate again, so a verified brief stays stuck until someone rewrites a true record. The
  verifier tooling already knows each row's outcome when it writes the row. Recording that
  outcome as a typed field and reading it at the flip ends the prose guessing without
  letting a real hold through.
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  It changes what a verification control reads. Today any capitalised HELD or could-not-check
  in the credited run's prose refuses the flip. After this brief, inside a run that carries
  typed dispositions, only the typed field and the verdict lines can hold a row, so a hold
  written only as prose no longer refuses. The human confirms that trade, the fail-closed
  rules that bound it, and the dated end of the prose fallback.
decision-trigger: creation
issues: [5482]
schema: brief-v2
version: 1
authored: 2026-11-16 by a worker-desk authoring session (design proposal; no ruling recorded yet)
sources:
  - "medici-finance/assay#6638 (open: adds held-scan rule 7 for quoted output and names the unquoted prose-count residual this brief removes)"
  - "medici-finance/assay#5482 (the false-positive class #6638 answers)"
  - "medici-finance/assay#4721 (rule 6: structured signals only), #4294 (rule 4 and rule 3c), #4536 (rule 5)"
  - "the driver's ruling on the originating house issue (the flip gate should read a structured disposition, not Evidence prose)"
  - "docs/witness-readers.md (the census of witness-cell readers this brief adds one to)"
  - "freshness-checked 2026-11-16 @ 024095621 (origin/main)"
exec-tier: strong
exec-tier-why: "It changes a verify control's input, must fail closed on every missing or malformed field, carries a dated migration, and touches the witness writer, the landing tool and two flip readers that must agree."
domain: complicated
consumers:
  - "statusgen/verifyrun.go: follow-up desk-tools/65 (this brief; the witness Result cell gains the disposition token and verifyrun gains the hold and defer flags)"
  - "statusgen/verifyissues.go: follow-up desk-tools/65 (this brief; flipHeldRefusal reads the typed disposition when the credited run carries it)"
  - "statusgen/heldscope.go: follow-up desk-tools/65 (this brief; rules 1-6, and 7 if #6638 lands, are documented as the legacy fallback with its end date)"
  - "statusgen/verifyflip.go: follow-up desk-tools/65 (this brief; the refusal names the disposition, and a missing one exits could-not-check)"
  - "statusgen/reconcilefold.go: follow-up desk-tools/65 (this brief; the fold reaches the same reader through flipHeldRefusal)"
  - "tools/desk/cmd/deskevidence/verdictgate.go: follow-up desk-tools/65 (this brief; a landing keeps the token and refuses a run that mixes rows with and without it)"
  - "statusgen/evaluation/witness.go: follow-up desk-tools/65 (this brief; a [w2] record binds the disposition as part of the recorded outcome)"
  - "docs/witness-readers.md: follow-up desk-tools/65 (this brief; the census lists the new reader)"
  - "close-verify, the card emitter and autoflip (verifyPassHeldContradiction, closeVerifyHeldRefusal): out-of-scope (they keep the prose rules; this brief changes the flip read only, the same scope #6638 takes)"
id: 973762b5-72c4-4430-b5dd-0b84d7564282
---

# Brief 65 — Read each Verify row's disposition from a field the verifier tooling writes

## Dependencies
- No typed edge. The witness table, `verifyrun` and the flip readers all exist on main.
- **#6638 (open)** adds held-scan rule 7 (quoted output) to the same reader. This brief does not
  wait on it. If 7 lands first it becomes part of the legacy fallback, and its tests stay green
  unchanged. If it does not land, nothing here needs it.
- The head of the work is the writer. I verified that by reading `statusgen/verifyrun.go` on main:
  `witness.row()` renders the Result cell as `fmt.Sprintf("%s exit=%s", w.State, exitCell(w.Exit))`
  and appends only a sandbox token and, for a could-not-run row, a note. It showed that no run
  records a per-row disposition today, so no reader can use one until `verifyrun` writes it.
  Task steps 1 and 2 come first for that reason.

## Context

files:
- `statusgen/verifyrun.go` — `witness`, `witness.row()`, `witnessCells`, `witnessStateOf`, flag set in `runVerifyrun`
- `statusgen/verifyissues.go` — `flipHeldRefusal`, `heldRefusal`, `unroutedHeldLine`
- `statusgen/heldscope.go` — rules 1-6 and `heldScanScope`; doc block only
- `statusgen/verifyflip.go` — the flip's refusal and exit codes
- `statusgen/reconcilefold.go` — the fold's call to `flipHeldRefusal`
- `statusgen/evaluation/witness.go` — the `[w2]` record's recorded outcome
- `tools/desk/cmd/deskevidence/verdictgate.go` — the landing's verdict refusals
- `docs/witness-readers.md` — the reader census
- **add** — `statusgen/disposition.go` (planned), `statusgen/disposition_test.go` (planned), `statusgen/disposition-mutations.json` (planned)
- **add** — `changelog/<branch-slug>.md` — one fragment

facts:
- `verifyflip` and the reconcile fold both call `flipHeldRefusal`. It calls `unroutedHeldLine`
  with the latest-run scope (rule 3c), so only lines inside the credited PASS run are read. The
  heldscansite guard test pins `unroutedHeldLine` as the single read site.
- `unroutedHeldLine` matches `(?i)\b(HELD|could-not-check)\b` on every line in scope, then drops a
  line through rules 1-6. Rule 6 keeps a prose line only when the word is not all-lowercase,
  names a row number, or sits on a `VERIFY:` / `RISK-VALUE:` verdict line. So
  `… then Could-not-check (3); …` in prose refuses because it is capitalised. #6638's body names
  this residual and notes that a count exemption would also excuse a real summary such as
  "PASS (3), HELD (2)".
- The flip already refuses unless the credited run carries a passing execution witness for every
  Verify row (`closureRunWitnesses`). A witness row is recognised only by its shape: an `exit=`
  token in the Result cell and a `sha256:` fingerprint in the Output cell. Prose cannot pass for
  one.
- The Result cell already carries one appended token, `sandbox=`, placed so that `witnessStateRe`
  still lifts the leading state and `witnessResultRe` still finds `exit=`. A second token placed
  the same way leaves the six-column shape and every existing reader unchanged.
- verifyrun's states are `pass`, `fail` and `could-not-run`. It has no flag that records a
  verifier's hold or deferral of a row; today a verifier writes those as prose.
- The flip exits 0 (flipped), 1 (refused) or 2 (could-not-check).

design-fit:
  owner: statusgen/verifyrun.go writes the disposition; statusgen/disposition.go (planned) is the one reader the flip and the fold share
  contract: S-evidence-coverage
  retires: [the prose held-scan on the flip path for runs that carry dispositions; after 2027-01-31, for every run]
  weight: "verbs 0, flags 2 (verifyrun --hold, --defer), refusals 3 (missing, unparseable, mixed), one Result-cell token"
  why-add: "The token replaces a growing set of prose exclusions with one typed read. The legacy rules are kept only for older runs and end on a fixed date, so the ceiling falls once the window closes."

## Human decision

When a brief is checked after it merges, the verifier runs each Verify row and writes the result
into the brief's Evidence. A separate step then moves the brief from implemented to verified. That
step refuses if the Evidence says a row was held or could not be checked. It finds those words by
reading the Evidence text, so it also stops on the same words in a quoted error message or in a
sentence that counts rows. Each fix so far has taught it one more pattern to ignore, and the next
wording stops it again. This brief proposes that the verifier tool record each row's outcome as a
fixed field when it writes the row, and that the move read that field.

Options:
1. **Typed disposition, prose read kept for old runs until 2027-01-31** — the recommendation. A run
   written by the new tool is judged only by its typed fields and its verdict lines. A run without
   them is judged as today until 31 January 2027, and refused after that. The threat it opens: in a
   new run, a hold written only as a sentence no longer stops the move. The verifier must record it
   with the tool's hold flag or a `VERIFY:` verdict line.
2. **Keep reading prose, add more exclusions** — no new field. Count phrases and quoted output get
   their own rules, as #6638 starts. It opens no new threat, but each exclusion is a pattern a real
   hold could also match, as #6638 itself notes, and the false positives continue.
3. **No tool change; verifiers reword Evidence** — this rewrites true records to suit the gate. The
   source issue rules it out.

Default if no answer: none. The brief blocks until answered, because it changes a verification
control and the default must not do that. Option 1 is the recommendation.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Implement only the recorded answer. If the answer is option 2 or 3, this brief's Task does not
  apply; report NEEDS_CONTEXT so the desk re-authors.
- Fail closed. A missing, unparseable or contradictory disposition refuses. Could-not-check is
  never read as pass.
- The strict `**VERIFY: PASS**` marker check, the FAIL refusal (`closeVerifyFailRefusal`) and
  the witness check (`closureRunWitnesses`) are not touched.
- The existing held-scan tests in `statusgen/heldscope_test.go` are not edited. They pin the
  legacy fallback.

## Task
1. In `statusgen/verifyrun.go`, write a `disp=<value>` token into every witness Result cell,
   after `exit=` and any `sandbox=` token, ahead of a could-not-run note. Values: `pass`, `fail`,
   `held`, `could-not-check`, `deferred:#<N>`. An executed row takes its value from its state:
   `pass` → `pass`, `fail` → `fail`, `could-not-run` → `could-not-check`. An `author-added` row
   carries no token and earns no credit, as today.
2. Add `verifyrun --hold <row>=<reason>` and `--defer <row>=#<N>` (planned). Each sets the named
   row's disposition to `held` or `deferred:#<N>` and writes the reason after the token. Neither
   can raise a row: `--hold` on a failed row keeps `fail`, and `--defer` without an issue
   reference is refused as a usage error (exit 2).
3. Add `statusgen/disposition.go` (planned) with one reader, `runDispositions` (planned). Given
   the credited run's witness rows, it returns one of three results. Structured: every witness
   row carries a parseable token. Legacy: no witness row carries one. Malformed: some rows carry
   one and some do not, or a token's value is unknown, or `pass` sits on a row whose state is
   not `pass`.
4. In `flipHeldRefusal`, call `runDispositions` first:
   - Structured: refuse (exit 1) on any `held`, `fail` or `could-not-check`. Accept `deferred:#<N>`
     only with the issue reference, which matches today's routing rule. Then read only the
     `VERIFY:` and `RISK-VALUE:` verdict lines in the run for HELD or could-not-check. Do not read
     other prose.
   - Legacy: run `unroutedHeldLine` unchanged, but only while the run's recorded Date is on or
     before 2027-01-31. A legacy run dated later is refused as could-not-check (exit 2), naming
     the re-run it needs. The date is the run's own Date cell, never the wall clock, so the
     result is reproducible. The constant `legacyHeldScanUntil` (planned) holds the date.
   - Malformed: refuse as could-not-check (exit 2), naming the row.
   `closeVerifyHeldRefusal` and `verifyPassHeldContradiction` keep calling `unroutedHeldLine`
   directly; the heldscansite guard is updated to allow exactly the one new caller.
5. In `tools/desk/cmd/deskevidence/verdictgate.go`, keep the token byte-for-byte on landing and
   refuse a run that `runDispositions` classes Malformed.
6. In `statusgen/evaluation/witness.go`, include the token in the recorded outcome a `[w2]`
   record binds, so editing a landed disposition breaks the record match and the coverage gate
   refuses. A `[w1]` row is bound only by its landing, as its state is today.
7. Add the tests below to `statusgen/disposition_test.go` (planned) and the landing tests to
   `tools/desk/cmd/deskevidence`. Mark each with `// regression: #5482`. Each new flip test runs
   `verifyflip` end to end on a fixture brief.
   - `TestDispProseCountPasses` (planned): structured run, all `disp=pass`, prose line
     "then Could-not-check (3); retried" → exit 0. Red at base (today exit 1).
   - `TestDispQuotedOutputPasses` (planned): structured run, a fenced block of quoted output
     containing `HELD` and `could-not-check` → exit 0. Red at base.
   - `TestDispHeldRefuses` (planned): one row `disp=held` → exit 1.
   - `TestDispCouldNotCheckRefuses` (planned): one row `disp=could-not-check` → exit 1.
   - `TestDispMissingRefuses` (planned): one row without a token beside rows with one → exit 2.
   - `TestDispUnparseableRefuses` (planned): `disp=maybe` → exit 2.
   - `TestDispPassOnFailRefuses` (planned): `disp=pass` on a `fail` row → exit 2.
   - `TestDispDeferNeedsRef` (planned): `disp=deferred` with no reference → exit 2.
   - `TestDispVerdictLineHeld` (planned): structured run with a `VERIFY: HELD` verdict line → exit 1.
   - `TestDispLegacyFallback` (planned): legacy run dated 2027-01-31 with a prose `HELD row 2` → exit 1.
   - `TestDispLegacyRetired` (planned): legacy run dated 2027-02-01, otherwise clean → exit 2.
   - `TestDispFoldAgrees` (planned): the reconcile fold reaches the same verdict as the flip on
     the held, missing and prose-count fixtures.
   - `TestVerifyrunWritesDisp` (planned, in `statusgen/verifyrun_test.go`): a pass, a fail and a
     could-not-run row render `disp=pass`, `disp=fail`, `disp=could-not-check`, and
     `witnessStateOf` and `witnessCells` still read each row as before.
   - `TestVerifyrunHoldFlag` (planned): `--hold 2=reason` renders `disp=held`; `--defer 2=#7`
     renders `disp=deferred:#7`; `--defer 2=x` exits 2.
   - `TestLandKeepsDisp` and `TestLandMixedDispRefuses` (planned, deskevidence).
8. Add `statusgen/disposition-mutations.json` (planned) with a positive control and five
   mutants, each caught by a named test: a missing token read as pass (`TestDispMissingRefuses`),
   `held` dropped from the refusing set (`TestDispHeldRefuses`), the pass/state agreement check
   skipped (`TestDispPassOnFailRefuses`), the date check removed from the legacy path
   (`TestDispLegacyRetired`), and verdict lines no longer read in a structured run
   (`TestDispVerdictLineHeld`).
9. Update the heldscope.go doc block to say rules 1-6 (and 7, if present) are the legacy fallback
   and end on 2027-01-31, add `statusgen/disposition.go` (planned) to `docs/witness-readers.md` as
   `reads informational fields only`, and change every consumer entry to `fixed-here`.

## Why this does not weaken the gate
- The flip still needs a passing execution witness for every Verify row in the credited run, the
  strict PASS marker, and no unanswered FAIL. None of those checks change.
- A row's `pass` disposition comes from the same execution that wrote its passing witness.
  `pass` on a row whose state is not `pass` refuses. No new way to pass a row is added.
- Every hold that refuses today still refuses when the verifier records it: as `disp=held`, as
  `disp=could-not-check` from a run that could not execute, or on a `VERIFY:` / `RISK-VALUE:`
  verdict line. The verifier who would have typed the prose hold is the one who runs `--hold`.
- Absence fails closed. A run with some tokens missing, an unknown value, or a legacy run after
  2027-01-31 is could-not-check, never pass.
- Until the end date, a run without tokens is judged by exactly today's rules, so nothing that
  refuses today passes during the migration.
- What is given up is narrow: in a run that carries tokens, the words HELD or could-not-check in
  ordinary prose no longer refuse. That is the reading that produced the false positives, and the
  same verifier holds a typed way to say it.

## Verify (executable — no prose-only DoD items)
Every row is tagged risk-bearing in its Expect text, and rows 1, 2 and 4 red at the delivery base,
where the named tests and the mutation spec do not exist. The verifier records them with
`verifyrun --fail-first` against the parent of the delivering merge.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `test -f statusgen/disposition_test.go && cd statusgen && go test . -run '^TestDisp' -count=1 -v > "${TMPDIR:-/tmp}/dt65-flip.log" 2>&1; grep -F -e '--- PASS: TestDispProseCountPasses ' "${TMPDIR:-/tmp}/dt65-flip.log" && grep -F -e '--- PASS: TestDispQuotedOutputPasses ' "${TMPDIR:-/tmp}/dt65-flip.log" && grep -F -e '--- PASS: TestDispHeldRefuses ' "${TMPDIR:-/tmp}/dt65-flip.log" && grep -F -e '--- PASS: TestDispCouldNotCheckRefuses ' "${TMPDIR:-/tmp}/dt65-flip.log" && grep -F -e '--- PASS: TestDispMissingRefuses ' "${TMPDIR:-/tmp}/dt65-flip.log" && grep -F -e '--- PASS: TestDispLegacyRetired ' "${TMPDIR:-/tmp}/dt65-flip.log" && grep -F -e '--- PASS: TestDispFoldAgrees ' "${TMPDIR:-/tmp}/dt65-flip.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt65-flip.log"` | exit 0; risk-bearing; the prose-count and quoted-output fixtures flip, held and could-not-check dispositions refuse, a missing token and a retired legacy run exit could-not-check, and the fold agrees. Red at the delivery base (no such test file) |
| 2 | check:ci +flow | `cd statusgen && go test . -run '^TestVerifyrun(WritesDisp\|HoldFlag)$' -count=1 -v > "${TMPDIR:-/tmp}/dt65-run.log" 2>&1; grep -F -e '--- PASS: TestVerifyrunWritesDisp ' "${TMPDIR:-/tmp}/dt65-run.log" && grep -F -e '--- PASS: TestVerifyrunHoldFlag ' "${TMPDIR:-/tmp}/dt65-run.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt65-run.log"` | exit 0; risk-bearing; verifyrun writes the token for each state and the hold and defer flags, and the existing witness readers still read each row. Red at the delivery base (no such tests) |
| 3 | check:ci +neighbour | `cd statusgen && go test . -run 'Held\|Vflip\|FlipMergeBases\|Fold\|WitnessReaderCensus' -count=1 && cd ../tools/desk && go test ./cmd/deskevidence/ -count=1 -v > "${TMPDIR:-/tmp}/dt65-land.log" 2>&1; grep -F -e '--- PASS: TestLandMixedDispRefuses ' "${TMPDIR:-/tmp}/dt65-land.log" && ! grep -F -e '--- FAIL' "${TMPDIR:-/tmp}/dt65-land.log"` | exit 0; risk-bearing; the existing held-scan, flip, fold and reader-census suites pass unchanged beside the new reader, and a landing refuses a mixed run |
| 4 | check:ci +mutation | `T=$(mktemp -d) && (cd tools/desk && go build -o "$T/muhar" ./cmd/muhar) && cd statusgen && "$T/muhar" -spec disposition-mutations.json -j 4 > "$T/out" 2>&1; rc=$?; L=$(tail -1 "$T/out"); echo "$L"; [ $rc -eq 0 ] && grep -F -e 'positive control CAUGHT' "$T/out" && [ "$L" = 'Totals: 5 caught, 0 NOT CAUGHT, 0 could-not-mutate.' ]` | exit 0; risk-bearing; all five mutants and the positive control are caught, so each fail-closed rule has a test that fires. Red at the delivery base (no spec) |
| 5 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && git diff --quiet "$DELIVERY_BASE" "$DELIVERY_HEAD" -- statusgen/heldscope_test.go && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-65-structured-verify-disposition.md && o=$(mktemp) && statusgen --root . --consumers --brief desk-tools/65 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1; grep -F -e ' 0 disproved, 0 unchecked' "$o"` | exit 0; risk-bearing; the legacy held-scan tests are untouched and every consumer is in the delivering diff |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| A row with no disposition is read as pass | row 1 (`TestDispMissingRefuses`); row 4 (missing-as-pass mutant) |
| A held or could-not-check disposition no longer refuses | row 1 (`TestDispHeldRefuses`, `TestDispCouldNotCheckRefuses`); row 4 |
| `disp=pass` is written onto a row that did not pass | row 1 (`TestDispPassOnFailRefuses`); row 4 (agreement-skip mutant) |
| A deferral without a routing reference releases a row | row 1 (`TestDispDeferNeedsRef`); row 2 (`--defer 2=x` exits 2) |
| A verifier's `VERIFY: HELD` line is ignored in a structured run | row 1 (`TestDispVerdictLineHeld`); row 4 |
| The legacy prose fallback never ends | row 1 (`TestDispLegacyRetired`); row 4 (date-check mutant) |
| The legacy fallback changes during the window | row 3 (held-scan suite unchanged); row 5 (its test file untouched) |
| A token in the Result cell breaks an existing witness reader | row 2 (`TestVerifyrunWritesDisp`); row 3 (flip, fold and census suites) |
| A landed disposition is edited after the run | `[w2]`: the record no longer matches and coverage refuses (Task 6). `[w1]`: no row; the landing actor check is the binding control, as for the state today. Review-only |
| The flip and the fold disagree | row 1 (`TestDispFoldAgrees`) |
| A verifier records a real hold only as prose in a structured run | No row; it is the threat option 1 accepts. Review-only: the PR names it, and the `--hold` flag and verdict lines are the sanctioned forms |

**RISK-VALUE:** none of the four risk answers is yes; the brief is `gate: human` because it
changes a verification control's input.

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Review
Gate: human, decision at creation, not yet answered. The implementation PR needs a recorded
answer cited by `design:`, and a `Security-Review: pass` at its head before any ready flip,
because it changes a verify control. Every Verify row is tagged risk-bearing. Rows 1, 2 and 4 are
written to fail at the delivery base, and the verifier records them with `verifyrun --fail-first`
against the parent of the delivering merge.
