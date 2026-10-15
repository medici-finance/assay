---
brief: assay:assay:build-less-brittle:11
title: "Regression tests are a ratchet: every fix ships a tagged bug-reproducing test, a tagged test leaves only with a Retires-test: trailer, and a report lists what left untagged for review to judge"
why: >-
  Every fix already has to show its test failing first (worker kit §9, review kit §4, the
  fail-first review lane), and every check needs a mutation row (brief-rules 16). Those prove
  the test can fail on the day it lands. Nothing holds the test in place afterwards. Between
  2026-09-17 and 2026-09-24 five test functions were deleted from the public tree and none of
  the four commits names the function it removed or says why; and when a migration renamed two
  tests, the Verify row that cited them kept passing with "no tests to run" (#1306). A tag that
  says what a test pins, a trailer that says why a tagged test leaves, and a report that lists
  every departure without one, make the regression suite ratchet instead of leak. The report
  never blocks: it is a rubric for the reviewer, not a ruling (spec §4.11; proposed, ratified by the merge that lands this stream).
wave: 4
depends: ["build-less-brittle/05", "build-less-brittle/06", "build-less-brittle/07"]
unblocks: ["build-less-brittle/12"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 3
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; third-pass amendment)"
sources:
  - "medici-finance/assay#2789; scratch-isolation amendment, freshness-checked 2026-10-13 @ 53c536312"
  - "docs/streams/build-less-brittle/spec.md §3 row 13, §4.11, §11"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md §9 (fail-first evidence, line 175 at f7bde6bfa) and §14 (defect class, line 315); worker-prompt-objective.md carries §9 verbatim (line 247)"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md §4 (line 51) and review-lanes.md §'The fail-first reproduction's two required records' (line 87): the fail-first lane every unknown or blessed-once author gets"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: the finding-class register (`test-evidence (fail-first / mutation) | blocking`, line 524) and the fail-first paragraph (line 735)"
  - "docs/brief-rules.md rule 16 (a brief that adds a check carries a mutation-test row); the committed mutation maps (`tools/desk/internal/deskkit/mutations.json` and 10 siblings)"
  - "#1581 (proposal: a TestRegression_ prefix, a count-can't-drop CI gate, a vacuous-selector gate) — this brief takes the tag and the report and declines both gates; #1580 (fix the class, regression-of:); #1306 (renamed tests leave a Verify row running zero tests); #1657 (the vacuous-row class); #1343 (a deleted Verify row is invisible to the integrity gate: the sibling class, not this brief's)"
  - "git evidence at origin/main 3f2c4505c: `git log --since=2026-09-17 -p -- '*_test.go' | grep '^-func Test'` lists 5 deletions in 4 commits (7af5d2b6c ×2, 96d26fa1c, c16e2dc55, 1cd46a954); no commit body names a deleted function; one (c16e2dc55) says a test was 'inverted'"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no `// regression:` tag convention, no Retires-test: trailer, no report of deleted or renamed test functions; 315 test files already cite an issue number in a comment, informally"
exec-tier: strong
exec-tier-why: "(b) one convention lands in two implementer kits, the review kit, a skill and the register, and the report must agree with git about what was deleted and what was renamed; (a) the tag-versus-prefix choice is a judgement the brief must settle with reasons."
domain: complicated
consumers:
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md §9: fixed-here"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md §Fail-first evidence: fixed-here"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md §4: fixed-here"
  - "plugins/assay/skills/pr-review-desk/SKILL.md fail-first paragraph: fixed-here"
  - "docs/contracts.md §Rule register row R-retires-test: fixed-here (the register exists from 07)"
  - "build-less-brittle/12 (the oracle's failure-mode section reads the tags): follow-up build-less-brittle/12"
  - "the statusgen Verify-row test-name lint (a `go test -run <name>` that matches no test; authored in the project layer, not yet landed): out-of-scope (it catches the effect in Verify rows; this brief reports the cause in the test tree, and the rename trailer names the new name so the row is re-pointed in the same PR)"
  - "installed deskdispatch binaries (kits are embedded): out-of-scope (reach consumers on the next desk-tools release and pin bump)"
---

# Brief 11 — Regression tests are a ratchet

## Context

files:
- `tools/desk/cmd/deskdispatch/references/worker-prompt.md`: §9, the tag rule and the trailer (≤ 6 lines, offset).
- `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md`: its verbatim copy of §9.
- `tools/desk/cmd/deskdispatch/references/review-prompt.md`: §4, the three rubric questions for a reported line (≤ 4 lines, offset).
- `plugins/assay/skills/pr-review-desk/SKILL.md`: the fail-first paragraph, the report command and the rubric pointer (≤ 2 lines, offset).
- `tools/desk/internal/testledger/ledger.go` (planned): NEW. Pure functions: `Tests(fsys) []TestFunc` (name, package, tag, body hash) via `go/parser`; `Diff(base, head) Report` (deleted, renamed, added); `ParseTrailers(msgs) []Retirement`; `RowsNaming(fsys, name) []Row` over `docs/streams/**/brief-*.md`.
- `tools/desk/internal/testledger/ledger_test.go` (planned): NEW. `TestLedgerFixture` (planned), `TestTrailerGrammar` (planned), `TestReportTestLedger` (planned) (test-only flags `-base`, `-head`: a revision via `git archive`, or a directory), `TestUnresolvableBaseIsCouldNotCheck` (planned).
- `tools/desk/internal/testledger/testdata/{base,head}/**` (planned): NEW. Two fixture trees and a `log.txt` of commit messages.
- `docs/contracts.md`: the tag convention (2 lines under "How a brief cites this") and one register row.
- Seeded tags: ≥ 5 existing tests whose comment already names the issue they pin.
- `changelog/build-less-brittle-11.md` (planned)

facts:
- **What already exists, and where.** Fail-first is the worker's obligation in kit §9 ("show
  the check failing before you claim it passes", quoted verbatim from review kit §4) and the
  class version in §14.3 ("show the class guard failing against a PLANTED SECOND instance").
  It is the reviewer's rule in review kit §4, a lane (`fail-first`) for every unknown or
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
  4. The prefix's one advantage is a `-run 'TestRegression_'` selector for a count gate. This
     stream declines the gate (a rubric, not a ruling; spec §4.11), so the advantage does not apply.
  The cost is stated: a comment is invisible in `go test -v` output. The report is where the
  tags are made visible, at review, which is where they are needed.
- **The trailer.** A commit that deletes or renames a test function carries one trailer per
  function: `Retires-test: <TestName> — <why>`; for a rename, `Retires-test: <Old> — renamed
  <New>; <why>`. `git interpret-trailers --parse` reads it as-is (checked). A rename's trailer
  names the new name so the same PR re-points every Verify row that named the old one; the
  report lists those rows.
- **The report, never a gate.** `TestReportTestLedger` (planned) logs, for the range `-base..-head`:
  `retired-untrailed: <pkg>.<Test> [regression #N] deleted in <commit>`,
  `renamed-untrailed: <pkg>.<Old> → <New> (body identical|same tag) in <commit>`, and
  `verify-rows-naming: <Old> → docs/streams/<s>/brief-<NN>.md:<row>`. It passes whatever it
  finds. It fails only on its own fixtures, and it skips with `could-not-check (<reason>)` when
  a revision cannot be resolved, never passing silently. CI runs it with the rest of
  `go test ./...` and shows nothing on a pass; the reviewer runs it with `-v` against the
  PR's merge-base and quotes it. A trailed retirement is not reported; a retirement of an
  untagged test is reported without the bracket, so the reviewer weighs a tagged one harder.
- **What the reviewer does with a line** (review kit §4, three questions): is the behaviour
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
  shapes review kit §4 lists). The oracle (12) consumes the tag: each failure mode maps to
  its regression test by `git grep 'regression: .*#<N>'`.
- **Self-referential base.** `git merge-base origin/main HEAD` is `HEAD` on a merged
  checkout (#1657), so the report takes an explicit `-base`, and every Verify row below pins
  one. There is no default range.
- **Three-state.** A `-base` that does not resolve, or a tree with no `_test.go`, is
  `could-not-check` with the reason; a tree with tests and nothing deleted reports `clean`.
- Line counts at f7bde6bfa (for scale; the net ≤ 0 rows derive their own base): worker-prompt.md 358, worker-prompt-objective.md 509,
  review-prompt.md 324, pr-review-desk 968. Briefs 05 and 06 edit the same files earlier in
  the chain; each brief, this one included, offsets its own lines.

design-fit:
  owner: tools/desk/internal/testledger (new; no existing owner reads test functions across two trees)
  contract: S-review-verdict (the rubric lands in the review kit; the register row R-retires-test serves it)
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 in each of the four files; one register row
  why-add: n/a (no ratcheted growth); ~250 lines of pure parsing reported under golines. The alternative, #1581's count-can't-drop CI job, needs a workflow scope the worker App lacks and is the gate this stream declines (spec §4.11).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No new verb and no new flag on any shipped binary. The report is a test package; its flags live in `_test.go`.
- The report NEVER fails on what it finds. A `t.Fatal` on a reported line is a finding against this brief.
- Kit wording that appears in both implementer kits changes in both in the same commit.
- Each edited file is net ≤ 0 lines. Tighten §9's two worked forms rather than dropping either.

## Task

1. `ledger.go`: `Tests` (planned) over an `fs.FS` (every `*_test.go`; `func Test*`, `Fuzz*`, `Benchmark*`
   with package path, doc-comment tag, and a hash of the normalised body); `Diff` (deleted =
   in base not head; renamed = a deleted whose body hash or tag matches an added in the same
   package; added); `ParseTrailers` over `git log --format='%H%n%(trailers:key=Retires-test,valueonly)%n--'`
   text; `RowsNaming` over `docs/streams/**/brief-*.md` Verify tables (a `-run` selector or a
   bare test name containing `<Old>`), run for every untrailed departure, deleted as well as
   renamed.
2. Fixture trees: base holds 6 tests (2 tagged); head deletes one tagged and one untagged,
   renames one (identical body) and one (same tag, changed body), keeps two, adds one; `log.txt`
   carries one `Retires-test:` trailer covering exactly one of the deletions. `TestLedgerFixture` (planned)
   asserts the exact report: 1 `retired-untrailed` with bracket, 0 for the trailed one, 2
   `renamed-untrailed`, and the decoy (a non-test `func Test…` helper in a `.go` file) ignored.
3. `TestReportTestLedger` (planned): `-base`/`-head` as a revision (`git archive` into a temp dir, 03's
   pattern) or a directory; logs the lines above or `clean`; `t.Skip("could-not-check (…)")`
   on an unresolvable revision. `TestTrailerGrammar` (planned) over the grammar in facts, including the
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
   `R-retires-test` (serves `S-review-verdict`; catch source `TestLedgerFixture` (planned); justifying
   issues #1306, #1581).
7. Changelog fragment.

## Verify (executable — no prose-only DoD items)

Run consumer checks with `TMPDIR` pointing at owned scratch outside the admitted source checkout.
This scratch-location amendment (#2789) preserves the revision selection and assertions;
historical Evidence does not verify the revised row, which needs fresh verifier admission.

Rows run from the root of `medici-finance/assay`. Rows 3–4 are the mutation rows (a planted
deletion and a planted rename must appear in the report). Rows 5–6 dereference the report
against the real deletions this brief cites. Row 7 proves the report cannot block. Rows 11–14
are net ≤ 0 rows.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./internal/testledger/ -count=1` | `ok` | check:ci |
| 2 | `cd tools/desk && { go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v; go test ./internal/testledger/ -run TestTrailerGrammar -count=1 -v; } \| grep -c -e '^--- PASS: TestLedgerFixture ' -e '^--- PASS: TestTrailerGrammar '` | `2` (each top-level test passes once; subtests are not counted) | check:ci |
| 3 | `d=$(mktemp -d) && cp -R . "$d/tree" && t=$(git grep -l '^// regression: #' -- '*_test.go' \| head -1) && n=$(grep -A1 '^// regression: #' "$t" \| grep -oE 'func (Test[A-Za-z0-9_]+)' \| head -1 \| cut -d' ' -f2) && rm "$d/tree/$t" && cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base="$(git rev-parse --show-toplevel)" -head="$d/tree" \| grep -c "retired-untrailed: .*$n \[regression"` | `1` (a planted deletion of a tagged test is reported with its tag; the file's other tests are reported too, without the bracket) | check:ci +mutation |
| 4 | `d=$(mktemp -d) && cp -R . "$d/tree" && t=$(git grep -l '^// regression: #' -- '*_test.go' \| head -1) && n=$(grep -A1 '^// regression: #' "$t" \| grep -oE 'func (Test[A-Za-z0-9_]+)' \| head -1 \| cut -d' ' -f2) && sed "s/func $n(/func ${n}Renamed(/" "$t" > "$d/tree/$t" && cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base="$(git rev-parse --show-toplevel)" -head="$d/tree" \| grep -c "renamed-untrailed: .*$n → ${n}Renamed"` | `1` (a planted rename with an identical body is reported as a rename, not a deletion plus an addition) | check:ci +mutation |
| 5 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=c16e2dc55~1 -head=c16e2dc55 \| grep -c 'retired-untrailed: .*TestRoleInitWiresWorktreeScopedCredentialHelper'` | `1` (dereference: a real untrailed deletion from the evidence window is reported) | check |
| 6 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=7af5d2b6c~1 -head=7af5d2b6c \| grep -c 'retired-untrailed: .*TestClaimLivenessIsUnknown'` | `2` (both deletions in #1498's commit) | check |
| 7 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -args -base=7af5d2b6c~1 -head=7af5d2b6c > /tmp/bl11-r7.out 2>&1; echo "exit=$?"` | output is `exit=0` (the report passes with two findings: it never blocks by itself). Expect re-written 2026-10-03 (#1862). | check |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=b7a82c025~1 -head=b7a82c025 \| grep -c 'verify-rows-naming: TestDedupeSearchOutageNamesTheAPIStatus → docs/streams/desk-tools/brief-21'` | `1` (dereference: the #1306 rename names the Verify row that went vacuous. In b7a82c025 the old test's body also changed and no tags existed yet, so the ledger sees a deletion, not a rename: `RowsNaming` fires on every untrailed departure, deletions as well as renames, and this row depends on that) | check |
| 9 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=deadbeefdeadbeef -head=HEAD \| grep -c 'could-not-check'` | `1` (an unresolvable base is could-not-check, never `clean`) | check:ci |
| 10 | `n=$(git grep -c '^// regression: #' -- '*_test.go' \| awk -F: '{s+=$2} END{print s+0}'); test "$n" -ge 5 && echo "tags=$n"` | `tags=` ≥ 5 (seeded, so rows 3–4 cannot pass vacuously) | check:ci |
| 11 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c 'regression: #' "$f"; done \| grep -c '^[1-9]'` | `2` (both kits carry the tag rule) | check:ci |
| 12 | `grep -c 'Retires-test' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -c 'testledger' plugins/assay/skills/pr-review-desk/SKILL.md` | two counts, each ≥ `1` | check:ci |
| 13 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" && echo NET-OK` | `NET-OK` | check |
| 14 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` | check |
| 15 | `grep -cE '^[\|] *R-retires-test .*S-review-verdict' docs/contracts.md && grep -c 'TestLedgerFixture' docs/contracts.md` | `1`, then ≥ `1` (registered, serving an existing S- row, with a catch source that can fire) | check:ci |
| 16 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git diff --name-only "$base" "$tip" -- tools/desk/cmd \| grep -v _test.go \| grep -v '/references/' \| wc -l \| tr -d ' ')" = 0 && echo NO-CMD-CHANGE` | `NO-CMD-CHANGE` (no shipped verb or flag changed; kit text under `references/` is the one allowed edit; base derived, never `HEAD~1`) | check |
| 17 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "${TMPDIR:-/tmp}/bl11-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/11 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | exit 0 (no `consumers:` routing claim is disproved: the `consumers:` claims are judged against the delivering change itself, in a throwaway clone: after the merge, the first-parent commit on main carrying the `Brief: build-less-brittle/11` trailer (the net-weight rows resolve the same commit) diffed against its parent; before the merge, the PR head diffed against its merge-base; the implementer replaces each self-routed entry with `fixed-here` in the same change). A disproved claim fails the row and names the claim; an unresolvable delivering change fails the row rather than passing on an empty diff. Row re-authored 2026-10-06 (#1915): the unpinned form ran against merged main, where the brief is no longer in the diff, and reported COULD-NOT-CHECK; this form runs the same way before and after the merge (the #1908 pattern). Expect first re-written 2026-10-03 (#1862). | check |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/testledger/ -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && { go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v; go test ./internal/testledger/ -run TestTrailerGrammar -count=1 -v; } \| grep -c -e '^--- PASS: TestLedgerFixture ' -e '^--- PASS: TestTrailerGrammar '` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `d=$(mktemp -d) && cp -R . "$d/tree" && t=$(git grep -l '^// regression: #' -- '*_test.go' \| head -1) && n=$(grep -A1 '^// regression: #' "$t" \| grep -oE 'func (Test[A-Za-z0-9_]+)' \| head -1 \| cut -d' ' -f2) && rm "$d/tree/$t" && cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base="$(git rev-parse --show-toplevel)" -head="$d/tree" \| grep -c "retired-untrailed: .*$n \[regression"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `d=$(mktemp -d) && cp -R . "$d/tree" && t=$(git grep -l '^// regression: #' -- '*_test.go' \| head -1) && n=$(grep -A1 '^// regression: #' "$t" \| grep -oE 'func (Test[A-Za-z0-9_]+)' \| head -1 \| cut -d' ' -f2) && sed "s/func $n(/func ${n}Renamed(/" "$t" > "$d/tree/$t" && cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base="$(git rev-parse --show-toplevel)" -head="$d/tree" \| grep -c "renamed-untrailed: .*$n → ${n}Renamed"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=c16e2dc55~1 -head=c16e2dc55 \| grep -c 'retired-untrailed: .*TestRoleInitWiresWorktreeScopedCredentialHelper'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=7af5d2b6c~1 -head=7af5d2b6c \| grep -c 'retired-untrailed: .*TestClaimLivenessIsUnknown'` | pass exit=0 | sha256:53c234e5e847 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -args -base=7af5d2b6c~1 -head=7af5d2b6c > /tmp/bl11-r7.out 2>&1; echo "exit=$?"` | pass exit=0 | sha256:19eaf43821a7 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=b7a82c025~1 -head=b7a82c025 \| grep -c 'verify-rows-naming: TestDedupeSearchOutageNamesTheAPIStatus → docs/streams/desk-tools/brief-21'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./internal/testledger/ -run TestReportTestLedger -count=1 -v -args -base=deadbeefdeadbeef -head=HEAD \| grep -c 'could-not-check'` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 10 | `n=$(git grep -c '^// regression: #' -- '*_test.go' \| awk -F: '{s+=$2} END{print s+0}'); test "$n" -ge 5 && echo "tags=$n"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 11 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c 'regression: #' "$f"; done \| grep -c '^[1-9]'` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c 'Retires-test' tools/desk/cmd/deskdispatch/references/review-prompt.md && grep -c 'testledger' plugins/assay/skills/pr-review-desk/SKILL.md` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 13 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt.md" \| wc -l)" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 14 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 15 | `grep -cE '^[\|] *R-retires-test .*S-review-verdict' docs/contracts.md && grep -c 'TestLedgerFixture' docs/contracts.md` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 16 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git diff --name-only "$base" "$tip" -- tools/desk/cmd \| grep -v _test.go \| grep -v '/references/' \| wc -l \| tr -d ' ')" = 0 && echo NO-CMD-CHANGE` | pass exit=0 | sha256:80a91942d970 | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
| 17 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/11$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "$PWD/.bl11-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/11 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | pass exit=0 | sha256:27b7e89b918f | 2026-10-11 | assay-verifier-app[bot] @ a4bc0c0cfe06 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2526 run=40cbc80d0918789e628633ac5fb4c3f5844f00b8d7fb4766cd966cbfed727c66 source=a4bc0c0cfe0634bef4a4c52c1b7078f1b3f7fd3c brief=docs/streams/build-less-brittle/brief-11-regression-test-ratchet.md model=claude-opus-5-5 tier=any

### Verifier pass — 2026-10-11, assay-verifier-app[bot] (verify-desk, non-implementer), merged main a4bc0c0cf

Delivering change: 0333e8e14 (the first-parent commit on main carrying the `Brief: build-less-brittle/11` trailer). Host: darwin, go1.27.1, git 2.56.0, statusgen v1.0.37, GOFLAGS=-count=1.

The witness table above records rows 1, 2, 3, 4, 9, 10, 11, 12 and 15 (the check:ci rows) as could-not-run: statusgen's network-off sandbox is `unshare --net`, which this darwin host lacks, and the pinned harness image carries no Go toolchain, so `--in-container` cannot execute the Go rows either. Each of those nine rows was therefore also executed by hand from the repo root under macOS `sandbox-exec` with all IP network traffic denied and GOPROXY=off (a curl probe to github.com inside the same profile failed with exit 7, confirming the denial). The observed cells below are from those runs; rows 5–8, 13, 14, 16 and 17 are the witness's own pass rows, cross-checked by a hand run.

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | Verify row 1 | `ok` | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/testledger 2.911s` (network denied) | 2026-10-11 assay-verifier-app[bot] |
| 2 | Verify row 2 | `2` | exit 0; `2` (network denied) | 2026-10-11 assay-verifier-app[bot] |
| 3 | Verify row 3 | `1` | exit 0; `1` (network denied). Planted deletion of statusgen/boardhonesty_test.go, first tagged test TestBoardHonestyDehousedRowScoped (`// regression: #1516`) | 2026-10-11 assay-verifier-app[bot] |
| 4 | Verify row 4 | `1` | exit 0; `1` (network denied). Planted rename TestBoardHonestyDehousedRowScoped → TestBoardHonestyDehousedRowScopedRenamed | 2026-10-11 assay-verifier-app[bot] |
| 5 | Verify row 5 | `1` | exit 0; `1`; report line `retired-untrailed: tools/desk/cmd/deskwt.TestRoleInitWiresWorktreeScopedCredentialHelper deleted in c16e2dc55` | 2026-10-11 assay-verifier-app[bot] |
| 6 | Verify row 6 | `2` | exit 0; `2`; lines for deskpost.TestClaimLivenessIsUnknownForAMalformedTrailer and deskpost.TestClaimLivenessIsUnknownWithoutADerivableKey, both `deleted in 7af5d2b6c` | 2026-10-11 assay-verifier-app[bot] |
| 7 | Verify row 7 | `exit=0` | exit 0; prints `exit=0`; the captured output file holds `ok  github.com/medici-finance/assay/tools/desk/internal/testledger 0.716s` | 2026-10-11 assay-verifier-app[bot] |
| 8 | Verify row 8 | `1` | exit 0; `1`; line "verify-rows-naming: TestDedupeSearchOutageNamesTheAPIStatus → docs/streams/desk-tools/brief-21-desk-trace-and-cause-carrying-errors.md:11" | 2026-10-11 assay-verifier-app[bot] |
| 9 | Verify row 9 | `1` | exit 0; `1` (network denied); test logs `could-not-check ("deadbeefdeadbeef" is neither a directory nor a revision in <home>)` then `--- SKIP: TestReportTestLedger` | 2026-10-11 assay-verifier-app[bot] |
| 10 | Verify row 10 | `tags=` ≥ 5 | exit 0; `tags=7` (network denied): #1516, #1111, #1395, #1939, #616, #1428, #1490 | 2026-10-11 assay-verifier-app[bot] |
| 11 | Verify row 11 | `2` | exit 0; `2` (network denied) | 2026-10-11 assay-verifier-app[bot] |
| 12 | Verify row 12 | two counts ≥ 1 | exit 0; `1` then `1` (network denied) | 2026-10-11 assay-verifier-app[bot] |
| 13 | Verify row 13 | `NET-OK` | exit 0; `NET-OK` (worker-prompt.md 395 → 394 lines; worker-prompt-objective.md 548 → 547) | 2026-10-11 assay-verifier-app[bot] |
| 14 | Verify row 14 | `NET-OK` | exit 0; `NET-OK` (review-prompt.md 482 → 482; pr-review-desk SKILL.md 1168 → 1168) | 2026-10-11 assay-verifier-app[bot] |
| 15 | Verify row 15 | `1`, then ≥ `1` | exit 0; `1` then `1` (network denied) | 2026-10-11 assay-verifier-app[bot] |
| 16 | Verify row 16 | `NO-CMD-CHANGE` | exit 0; `NO-CMD-CHANGE` (tools/desk/cmd changes in 0333e8e14: three files under references/ plus one _test.go) | 2026-10-11 assay-verifier-app[bot] |
| 17 | Verify row 17 | exit 0 | exit 0; `summary: 5 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing` | 2026-10-11 assay-verifier-app[bot] |

Review-section check: TestReportTestLedger's body calls only t.Log and t.Skip/t.Skipf, with no t.Fatal or t.Error, so the report cannot fail on what it finds. Rename pairing in pair() requires exactly one deletion and one addition sharing a key per package, so two tests that share a tag are not collapsed into one rename. Spot-checked seeded tags (#1111, #1939, #1490): each sits directly above the test whose comment describes that issue's fix.

Risk-bearing value: the item carries risk metadata (all `no`, irreversible: no), touches no risk-classed path, and changes no hard-pinned value. Enumerated over the delivering diff (ledger.go literals; the kit and register text): TagPrefix = "// regression: " @ tools/desk/internal/testledger/ledger.go:64; body-hash truncation `[:16]` (16 hex chars = 64 bits) @ ledger.go:277; rename pairing `len(ds) == 1 && len(as) == 1` @ ledger.go:344; trailer scanner buffer `64*1024` initial / `1024*1024` max @ ledger.go:370; commit abbreviation `12` @ ledger.go:544. Ranked: all are reversible by an edit to a test-only package that never gates a merge. The top two are derived below; the buffer and abbreviation width are operational knobs.

- RISK-VALUE: DERIVED — TagPrefix = "// regression: " @ tools/desk/internal/testledger/ledger.go:64 — it is the tag convention exactly as the brief's facts specify (`// regression: #<N>[, #<M>…]`) and as docs/contracts.md registers it; the 7 seeded tags and row 10's git grep use the same prefix, so parser, convention and selector agree.
- RISK-VALUE: DERIVED — body-hash width = 16 hex chars (64 bits) @ tools/desk/internal/testledger/ledger.go:277 — a false rename needs two different test bodies in the same package with colliding 64-bit hashes; the birthday bound is about 2^32 tests per package, far beyond any real package, and pair() also requires a one-to-one match, so a collision could at worst mislabel one report line, never block.

**VERIFY: PASS** — 17/17 rows observed passing at a4bc0c0cf; the 9 check:ci rows carry could-not-run in the statusgen witness (no `unshare` on darwin) and were observed passing by hand with the network denied.


## Review
Gate: model (from frontmatter). The reviewer confirms the report has no failing path other
than its fixtures (grep the package for `t.Fatal`/`t.Error` outside `TestLedgerFixture` (planned) and
`TestTrailerGrammar` (planned)): a report that can block is a gate the driver did not rule. The reviewer
also checks that the rename detector does not over-match (two different tests that happen to
share a tag are two lines, not one rename) and that each seeded tag sits on the test that
pins the cited issue, not on a neighbour in the same file.
