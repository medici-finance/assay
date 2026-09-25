---
brief: assay:assay:build-less-brittle:11
title: "Regression tests are a ratchet: every fix ships a tagged bug-reproducing test, a tagged test leaves only with a Retires-test: trailer, and a report lists what left untagged for review to judge"
why: >-
  Every fix already has to show its test failing first (worker kit §9, review kit §3, the
  fail-first review lane), and every check needs a mutation row (brief-rules 16). Those prove
  the test can fail on the day it lands. Nothing holds the test in place afterwards. Between
  2026-09-17 and 2026-09-24 five test functions were deleted from the public tree and none of
  the four commits names the function it removed or says why; and when a migration renamed two
  tests, the Verify row that cited them kept passing with "no tests to run" (#1306). A tag that
  says what a test pins, a trailer that says why a tagged test leaves, and a report that lists
  every departure without one, make the regression suite ratchet instead of leak. The report
  never blocks: it is a rubric for the reviewer, not a ruling (the driver's ruling, 2026-09-24).
wave: 4
depends: ["build-less-brittle/05", "build-less-brittle/06", "build-less-brittle/07"]
unblocks: ["build-less-brittle/12"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; driver-ruled amendment)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 row 13, §4.11, §11"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md §9 (fail-first evidence, line 175 at f7bde6bfa) and §14 (defect class, line 315); worker-prompt-objective.md carries §9 verbatim (line 247)"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md §3 (line 36) and review-lanes.md §'The fail-first reproduction's two required records' (line 87): the fail-first lane every unknown or blessed-once author gets"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: the finding-class register (`test-evidence (fail-first / mutation) | blocking`, line 524) and the fail-first paragraph (line 735)"
  - "docs/brief-rules.md rule 16 (a brief that adds a check carries a mutation-test row); the committed mutation maps (`tools/desk/internal/deskkit/mutations.json` and 10 siblings)"
  - "#1581 (proposal: a TestRegression_ prefix, a count-can't-drop CI gate, a vacuous-selector gate) — this brief takes the tag and the report and declines both gates; #1580 (fix the class, regression-of:); #1306 (renamed tests leave a Verify row running zero tests); #1657 (the vacuous-row class); #1343 (a deleted Verify row is invisible to the integrity gate: the sibling class, not this brief's)"
  - "git evidence at origin/main 3f2c4505c: `git log --since=2026-09-17 -p -- '*_test.go' | grep '^-func Test'` lists 5 deletions in 4 commits (7af5d2b6c ×2, 96d26fa1c, c16e2dc55, 1cd46a954); no commit body names a deleted function; one (c16e2dc55) says a test was 'inverted'"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no `// regression:` tag convention, no Retires-test: trailer, no report of deleted or renamed test functions; 315 test files already cite an issue number in a comment, informally"
exec-tier: strong
exec-tier-why: "(b) one convention lands in two implementer kits, the review kit, a skill and the register, and the report must agree with git about what was deleted and what was renamed; (a) the tag-versus-prefix choice is a judgement the brief must settle with reasons."
domain: complicated
consumers:
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md §9: follow-up build-less-brittle/11 (this brief)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md §Fail-first evidence: follow-up build-less-brittle/11 (this brief)"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md §3: follow-up build-less-brittle/11 (this brief)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md fail-first paragraph: follow-up build-less-brittle/11 (this brief)"
  - "docs/contracts.md §Rule register row R-retires-test: follow-up build-less-brittle/11 (this brief; the register exists from 07)"
  - "build-less-brittle/12 (the oracle's failure-mode section reads the tags): follow-up build-less-brittle/12"
  - "the statusgen Verify-row test-name lint (a `go test -run <name>` that matches no test; authored in the project layer, not yet landed): out-of-scope (it catches the effect in Verify rows; this brief reports the cause in the test tree, and the rename trailer names the new name so the row is re-pointed in the same PR)"
  - "installed deskdispatch binaries (kits are embedded): out-of-scope (reach consumers on the next desk-tools release and pin bump)"
---

# Brief 11 — Regression tests are a ratchet

## Context

files:
- `tools/desk/cmd/deskdispatch/references/worker-prompt.md`: §9, the tag rule and the trailer (≤ 6 lines, offset).
- `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md`: its verbatim copy of §9.
- `tools/desk/cmd/deskdispatch/references/review-prompt.md`: §3, the three rubric questions for a reported line (≤ 4 lines, offset).
- `plugins/assay/skills/pr-review-desk/SKILL.md`: the fail-first paragraph, the report command and the rubric pointer (≤ 2 lines, offset).
- `tools/desk/internal/testledger/ledger.go` (planned): NEW. Pure functions: `Tests(fsys) []TestFunc` (name, package, tag, body hash) via `go/parser`; `Diff(base, head) Report` (deleted, renamed, added); `ParseTrailers(msgs) []Retirement`; `RowsNaming(fsys, name) []Row` over `docs/streams/**/brief-*.md`.
- `tools/desk/internal/testledger/ledger_test.go` (planned): NEW. `TestLedgerFixture`, `TestTrailerGrammar`, `TestReportTestLedger` (test-only flags `-base`, `-head`: a revision via `git archive`, or a directory), `TestUnresolvableBaseIsCouldNotCheck`.
- `tools/desk/internal/testledger/testdata/{base,head}/**` (planned): NEW. Two fixture trees and a `log.txt` of commit messages.
- `docs/contracts.md`: the tag convention (2 lines under "How a brief cites this") and one register row.
- Seeded tags: ≥ 5 existing tests whose comment already names the issue they pin.
- `changelog/build-less-brittle-11.md` (planned)

facts:
- **What already exists, and where.** Fail-first is the worker's obligation in kit §9 ("show
  the check failing before you claim it passes", quoted verbatim from review kit §3) and the
  class version in §14.3 ("show the class guard failing against a PLANTED SECOND instance").
  It is the reviewer's rule in review kit §3, a lane (`fail-first`) for every unknown or
  blessed-once author in `review-lanes.md`, and the `test-evidence` finding class in the
  pr-review-desk register, status `blocking`. Mutation rows are brief-rules rule 16, and 11
  committed mutation maps exist. This brief adds nothing to any of that. It adds what happens
  to the test **after** it lands.
- **The tag: a comment, not a name.** A regression test carries, directly above its
  declaration, `// regression: #<N>[, #<M>…]` (a public issue on the repository; a findings
  id `F-<slug>` or a class issue `class #<N>` are also legal). The alternative, the
  `TestRegression_<repo>_<issue>` prefix proposed in #1581, is declined for four reasons:
  1. Names describe behaviour (`TestClaimLivenessIsUnknownForAMalformedTrailer`). Moving to a
     prefix is a mass rename, and a rename is exactly the defect in #1306: the Verify rows
     that cite the old name keep passing with "no tests to run". 315 test files already cite
     issues in comments, so the tag formalises existing practice instead of replacing names.
  2. One test can pin several incidents; a comment lists them, a name holds one.
  3. `go/parser` reads a doc comment the same way brief 10 reads `// semantic:` markers: one
     marker shape, one parser, no regexes over source.
  4. The prefix's one advantage is a `-run 'TestRegression_'` selector for a count gate. The
     driver declined the gate (a rubric, not a ruling), so the advantage does not apply.
  The cost is stated: a comment is invisible in `go test -v` output. The report is where the
  tags are made visible, at review, which is where they are needed.
- **The trailer.** A commit that deletes or renames a test function carries one trailer per
  function: `Retires-test: <TestName> — <why>`; for a rename, `Retires-test: <Old> — renamed
  <New>; <why>`. `git interpret-trailers --parse` reads it as-is (checked). A rename's trailer
  names the new name so the same PR re-points every Verify row that named the old one; the
  report lists those rows.
- **The report, never a gate.** `TestReportTestLedger` logs, for the range `-base..-head`:
  `retired-untrailed: <pkg>.<Test> [regression #N] deleted in <commit>`,
  `renamed-untrailed: <pkg>.<Old> → <New> (body identical|same tag) in <commit>`, and
  `verify-rows-naming: <Old> → docs/streams/<s>/brief-<NN>.md:<row>`. It passes whatever it
  finds. It fails only on its own fixtures, and it skips with `could-not-check (<reason>)` when
  a revision cannot be resolved, never passing silently. CI runs it with the rest of
  `go test ./...` and shows nothing on a pass; the reviewer runs it with `-v` against the
  PR's merge-base and quotes it. A trailed retirement is not reported; a retirement of an
  untagged test is reported without the bracket, so the reviewer weighs a tagged one harder.
- **What the reviewer does with a line** (review kit §3, three questions): is the behaviour
  it pinned still pinned, and by which test; did the reason land as a trailer; if renamed,
  are the Verify rows re-pointed. A departure the reviewer judges unjustified is a
  `test-evidence` finding, the existing blocking class. No new basis, no new class, no new
  gate. A justified departure with a trailer is accepted as written.
- **Mutation specs guard the fix; the tag guards the test.** The mutation entry (kit §9 form
  2, rule 16) proves the test can go red when the fix is reverted, at the PR that lands it.
  The tag and trailer hold the test afterwards: a later PR that removes or renames it must say
  why, and the report puts that in front of the reviewer. One control per time-scale, and
  neither substitutes: a test with a mutation entry and no tag can vanish silently (the five
  deletions), and a tagged test with no mutation entry can be green for the wrong reason (the
  shapes review kit §3 lists). The oracle (12) consumes the tag: each failure mode maps to
  its regression test by `git grep 'regression: .*#<N>'`.
- **Self-referential base.** `git merge-base origin/main HEAD` is `HEAD` on a merged
  checkout (#1657), so the report takes an explicit `-base`, and every Verify row below pins
  one. There is no default range.
- **Three-state.** A `-base` that does not resolve, or a tree with no `_test.go`, is
  `could-not-check` with the reason; a tree with tests and nothing deleted reports `clean`.
- Line counts at f7bde6bfa: worker-prompt.md 358, worker-prompt-objective.md 509,
  review-prompt.md 324, pr-review-desk 968. Briefs 05 and 06 edit the same files earlier in
  the chain; this brief holds the same caps and offsets its own lines.

design-fit:
  owner: tools/desk/internal/testledger (new; no existing owner reads test functions across two trees)
  contract: S-review-verdict (the rubric lands in the review kit; the register row R-retires-test serves it)
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 in each of the four files; one register row
  why-add: n/a (no ratcheted growth); ~250 lines of pure parsing reported under golines. The alternative, #1581's count-can't-drop CI job, needs a workflow scope the worker App lacks and is the gate the driver declined.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No new verb and no new flag on any shipped binary. The report is a test package; its flags live in `_test.go`.
- The report NEVER fails on what it finds. A `t.Fatal` on a reported line is a finding against this brief.
- Kit wording that appears in both implementer kits changes in both in the same commit.
- Each edited file is net ≤ 0 lines. Tighten §9's two worked forms rather than dropping either.

## Task

1. `ledger.go`: `Tests` over an `fs.FS` (every `*_test.go`; `func Test*`, `Fuzz*`, `Benchmark*`
   with package path, doc-comment tag, and a hash of the normalised body); `Diff` (deleted =
   in base not head; renamed = a deleted whose body hash or tag matches an added in the same
   package; added); `ParseTrailers` over `git log --format='%H%n%(trailers:key=Retires-test,valueonly)%n--'`
   text; `RowsNaming` over `docs/streams/**/brief-*.md` Verify tables (a `-run` selector or a
   bare test name containing `<Old>`).
2. Fixture trees: base holds 6 tests (2 tagged); head deletes one tagged and one untagged,
   renames one (identical body) and one (same tag, changed body), keeps two, adds one; `log.txt`
   carries one `Retires-test:` trailer covering exactly one of the deletions. `TestLedgerFixture`
   asserts the exact report: 1 `retired-untrailed` with bracket, 0 for the trailed one, 2
   `renamed-untrailed`, and the decoy (a non-test `func Test…` helper in a `.go` file) ignored.
3. `TestReportTestLedger`: `-base`/`-head` as a revision (`git archive` into a temp dir, 03's
   pattern) or a directory; logs the lines above or `clean`; `t.Skip("could-not-check (…)")`
   on an unresolvable revision. `TestTrailerGrammar` over the grammar in facts, including the
   rename form.
4. Seed `// regression:` tags on ≥ 5 existing tests whose comment already names the issue
   (candidates: `statusgen/boardhonesty_test.go` `TestBoardHonestyDehousedRowScoped` → #1516;
   `statusgen/attribution_test.go` `TestVerifierFloorHumanTokenIsNotASuppressor`; the
   `claimliveness_test.go` and `modelfloor_test.go` tests added by #1498). Tag only, no other
   edit to those files.
5. Kit §9 (both kits, ≤ 6 lines, offset): the tag on every fail-first test; the trailer on
   every deletion or rename; re-point Verify rows named by the report in the same PR; paste a
   non-empty report under `## Tests retired` in the PR body. Review kit §3 (≤ 4 lines): run the
   report against the merge-base with `-v`; the three questions; an unjustified departure is
   `test-evidence`. Skill (≤ 2 lines): the command and the pointer.
6. `docs/contracts.md`: the tag convention beside 10's marker note; register row
   `R-retires-test` (serves `S-review-verdict`; catch source `TestLedgerFixture`; justifying
   issues #1306, #1581).
7. Changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Rows 3–4 are the mutation rows (a planted
deletion and a planted rename must appear in the report). Rows 5–6 dereference the report
against the real deletions this brief cites. Row 7 proves the report cannot block. Rows 11–14
are net ≤ 0 rows.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./internal/testledger/ -count=1` | `ok` |
| 2 | `cd tools/desk && { go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v; go test ./internal/testledger/ -run TestTrailerGrammar -count=1 -v; } \| grep -c -- '--- PASS'` | `2` |
| 3 | `d=$(mktemp -d) && cp -R . "$d/tree" && t=$(git grep -l '^// regression: #' -- '*_test.go' \| head -1) && n=$(grep -A1 '^// regression: #' "$t" \| grep -oE 'func (Test[A-Za-z0-9_]+)' \| head -1 \| cut -d' ' -f2) && rm "$d/tree/$t" && cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base="$(git rev-parse --show-toplevel)" -head="$d/tree" \| grep -c "retired-untrailed: .*$n \[regression"` | `1` (a planted deletion of a tagged test is reported with its tag; the file's other tests are reported too, without the bracket) |
| 4 | `d=$(mktemp -d) && cp -R . "$d/tree" && t=$(git grep -l '^// regression: #' -- '*_test.go' \| head -1) && n=$(grep -A1 '^// regression: #' "$t" \| grep -oE 'func (Test[A-Za-z0-9_]+)' \| head -1 \| cut -d' ' -f2) && sed "s/func $n(/func ${n}Renamed(/" "$t" > "$d/tree/$t" && cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base="$(git rev-parse --show-toplevel)" -head="$d/tree" \| grep -c "renamed-untrailed: .*$n → ${n}Renamed"` | `1` (a planted rename with an identical body is reported as a rename, not a deletion plus an addition) |
| 5 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=c16e2dc55~1 -head=c16e2dc55 \| grep -c 'retired-untrailed: .*TestRoleInitWiresWorktreeScopedCredentialHelper'` | `1` (dereference: a real untrailed deletion from the evidence window is reported) |
| 6 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=7af5d2b6c~1 -head=7af5d2b6c \| grep -c 'retired-untrailed: .*TestClaimLivenessIsUnknown'` | `2` (both deletions in #1498's commit) |
| 7 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -args -base=7af5d2b6c~1 -head=7af5d2b6c > /tmp/bl11-r7.out 2>&1; echo "exit=$?"` | `exit=0` (the report passes with two findings: it never blocks by itself) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=b7a82c025~1 -head=b7a82c025 \| grep -c 'verify-rows-naming: TestDedupeSearchOutageNamesTheAPIStatus → docs/streams/desk-tools/brief-21'` | `1` (dereference: the #1306 rename names the Verify row that went vacuous) |
| 9 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=deadbeefdeadbeef -head=HEAD \| grep -c 'could-not-check'` | `1` (an unresolvable base is could-not-check, never `clean`) |
| 10 | `n=$(git grep -c '^// regression: #' -- '*_test.go' \| awk -F: '{s+=$2} END{print s+0}'); test "$n" -ge 5 && echo "tags=$n"` | `tags=` ≥ 5 (seeded, so rows 3–4 cannot pass vacuously) |
| 11 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c 'regression: #' "$f"; done \| grep -c '^[1-9]'` | `2` (both kits carry the tag rule) |
| 12 | `grep -c 'Retires-test' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -c 'testledger' plugins/assay/skills/pr-review-desk/SKILL.md` | two counts, each ≥ `1` |
| 13 | `test "$(wc -l < tools/desk/cmd/deskdispatch/references/worker-prompt.md)" -le 358 && test "$(wc -l < tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md)" -le 509 && echo NET-OK` | `NET-OK` |
| 14 | `test "$(wc -l < tools/desk/cmd/deskdispatch/references/review-prompt.md)" -le 324 && test "$(wc -l < plugins/assay/skills/pr-review-desk/SKILL.md)" -le 968 && echo NET-OK` | `NET-OK` |
| 15 | `grep -cE '^[\|] *R-retires-test .*S-review-verdict' docs/contracts.md && grep -c 'TestLedgerFixture' docs/contracts.md` | `1`, then ≥ `1` (registered, serving an existing S- row, with a catch source that can fire) |
| 16 | `cd tools/desk && test "$(git diff --name-only HEAD~1 -- cmd \| grep -v _test.go \| grep -v '/references/' \| wc -l \| tr -d ' ')" = 0 && echo NO-CMD-CHANGE` | `NO-CMD-CHANGE` (no shipped verb or flag changed) |
| 17 | `statusgen --consumers --root . --brief build-less-brittle/11; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer confirms the report has no failing path other
than its fixtures (grep the package for `t.Fatal`/`t.Error` outside `TestLedgerFixture` and
`TestTrailerGrammar`): a report that can block is a gate the driver did not rule. The reviewer
also checks that the rename detector does not over-match (two different tests that happen to
share a tag are two lines, not one rename) and that each seeded tag sits on the test that
pins the cited issue, not on a neighbour in the same file.
