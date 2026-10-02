---
brief: assay:assay:build-less-brittle:10
title: "Architectural fitness functions in CI: dependency direction, the hub's import allow-list, and one implementation per registered meaning"
why: >-
  The weight ratchet (03) holds how much there is. It says nothing about shape: whether a
  helper crept into the wrong direction, whether the hub package quietly grew a new dependency,
  or whether a meaning the semantic index (01) assigns one owner is now computed in an eighth
  place. Ford, Parsons and Kua's fitness functions and ArchUnit's frozen rules made such shape
  rules executable and ratcheted; Go already enforces the strongest of them (internal/
  boundaries, no import cycles) in the compiler. This brief adds the three rules that matter
  to this stream as one test package CI already runs: dependency direction, the hub's import
  allow-list, and a declared-implementation ratchet per S- row. New things should be
  compositions of shared building blocks, not unique logic that drifts.
wave: 2
depends: ["build-less-brittle/01", "build-less-brittle/07"]
unblocks: ["build-less-brittle/12"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; SOTA amendment)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 row 12, §4.10, §11"
  - "docs/streams/build-less-brittle/spec.md §11 (Ford/Parsons/Kua fitness functions; ArchUnit FreezingArchRule as a ratchet; Go internal/ boundary; depguard and go-arch-lint as the Go-native forms)"
  - "tools/desk/internal/forgeban/allowlist.go (the committed-ceiling precedent) and build-less-brittle/03 (the test-package pattern)"
  - "docs/contracts.md §Semantic owners (brief 01): the S- rows whose owner and duplicates columns this test reads"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no arch test, depguard or go-arch-lint config in tools/desk; `go list` shows 0 internal→cmd imports, 0 cmd→other-cmd imports (deskpost imports only its own cmd/deskpost/internal), and deskkit importing exactly internal/gitcore and internal/topology; 66 packages import deskkit"
exec-tier: strong
exec-tier-why: "(b) the marker convention must agree with the S- rows across tools/desk and statusgen, and the allow-list must reproduce `go list` exactly, or the ratchet lies."
domain: clear
consumers:
  - "docs/contracts.md §Semantic owners (a `markers` note under the how-to-cite line): follow-up build-less-brittle/10 (this brief)"
  - "docs/contracts.md §Rule register rows R-dep-direction, R-hub-allowlist, R-one-implementation: follow-up build-less-brittle/10 (this brief; the register exists from 07)"
  - "build-less-brittle/06 design-fit stage (a red arch test is a design-fit finding by construction): follow-up build-less-brittle/06 (amended; see its facts)"
  - "statusgen (S-eligibility's owner lives there): out-of-scope for the test's tree walk in this brief; the marker rule applies to it from the next statusgen brief that touches eligibility.go"
---

# Brief 10 — Architectural fitness functions in CI

## Context

files:
- `tools/desk/internal/arch/arch.go` (planned): NEW. Import-graph reader (`go/parser`, `ImportsOnly`) and the three rules as pure functions.
- `tools/desk/internal/arch/arch_test.go` (planned): NEW. `TestDependencyDirection` (planned), `TestHubAllowList` (planned), `TestOneImplementationPerMeaning` (planned), `TestRulesFixture` (planned), `TestMissingIndexIsCouldNotCheck` (planned).
- `tools/desk/internal/arch/hub-allow.txt` (planned): NEW. The internal packages `internal/deskkit` may import, one per line, with `# grow` lines as in 03.
- `tools/desk/internal/arch/markers.txt` (planned): NEW. `<S-id> <ceiling>`: the ceiling on declared implementations per meaning.
- `tools/desk/internal/arch/testdata/tree/**` (planned): NEW. A fixture module with one violation of each rule.
- `docs/contracts.md`: the marker convention (3 lines under "How a brief cites this") and three register rows.
- `changelog/build-less-brittle-10.md` (planned)

facts:
- **Rule 1, dependency direction** (hard, ceiling 0). No package under `tools/desk/internal/`
  imports any package under `tools/desk/cmd/`. No `cmd/<x>` package imports `cmd/<y>` for
  `y ≠ x`; `cmd/<x>/internal/...` is its own. `go list` at f7bde6bfa: 0 and 0 violations, so
  the rule starts green with nothing to grandfather. Go itself already forbids import cycles
  and cross-tree `internal/` imports; the test states that it relies on the compiler for
  those and does not re-check them.
- **Rule 2, the hub's allow-list** (FreezingArchRule style). `internal/deskkit` is imported by
  66 packages; anything it imports is a dependency of the whole tree. `hub-allow.txt` lists
  what it may import from `internal/` (at f7bde6bfa: `gitcore`, `topology`). The test is red
  when deskkit imports an unlisted package AND when a listed package is no longer imported
  (lock in the gain, the forgeban precedent). Adding a line needs a `# grow` line citing the
  driver's `grow <PR#>` reply on the standing weight-growth issue (03's mechanism; no second
  gate).
- **Rule 3, one implementation per registered meaning** (a declared ratchet). Convention: a
  function that computes a meaning the semantic index registers carries the line comment
  `// semantic: S-<slug>` directly above its declaration. The test reads `docs/contracts.md`'s
  S- rows and asserts, per row: (a) every marker sits in the owner path or in a path the
  row's `duplicates` column lists; (b) the marker count is ≤ `markers.txt`'s ceiling;
  (c) the owner path carries ≥ 1 marker (the owner is declared, not assumed). A marker in an
  unlisted path is red with the message "S-<id> implemented outside its owner at <file>:<line>;
  add it to the row's duplicates with a design-fit finding, or move it to the owner".
  **What it cannot see:** an undeclared duplicate. That is the design-fit stage's question 1
  (06) at review time; this test holds what has been declared. The brief says so in the
  register row's `catch source` (`declared only; undeclared duplicates are 06's`).
- **Reported, never ratcheted:** clone density, when a clone detector is on PATH (`dupl -t 60`
  over non-test Go, reported as `clones=<n>`), else `could-not-check (no dupl)`. It is context
  for 09's reading, and He et al. (MSR 2026) found no duplication effect from agent adoption
  where complexity rose 41.6%, so it is not a gate.
- **Seed markers.** This brief seeds markers on the S- rows whose owner is under `tools/desk`
  (the index's `S-claim`, `S-review-verdict`, `S-identity`, `S-publication-scan`, `S-exit-codes`
  at least; `S-eligibility`'s owner is in statusgen and gets its marker when statusgen is next
  touched). `markers.txt` ceilings equal the seeded counts, so the ratchet starts tight.
- **Three-state.** In a consumer checkout of `tools/desk` alone, `docs/contracts.md` is absent:
  `TestOneImplementationPerMeaning` (planned) skips with `could-not-check (no semantic index)`. Rules 1
  and 2 need only the Go tree and always run.
- **CI:** `.github/workflows/ci.yml` runs `go test ./...` in `tools/desk` (03's fact). No
  workflow edit.
- **layering:** flat tool; a single package of pure functions over `fs.FS` plus one
  `go/parser` import scan.

design-fit:
  owner: tools/desk/internal/arch (new; no existing owner checks shape)
  contract: S-semantic-index (rule 3 reads it; the three R- rows serve it)
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines 0; three register rows
  why-add: n/a (no ratcheted growth); ~200 lines of pure checking reported under golines. The alternative, a depguard/go-arch-lint config, needs a linter CI does not run and a workflow scope the worker App lacks.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No new verb and no new flag on any shipped binary. Marker comments are the only source edits outside the new package.
- Never weaken a rule to go green. If rule 1 finds a violation at your head, that is NEEDS_CONTEXT with the import named, not an allow-list entry.

## Task

1. `arch.go`: `Imports(fsys fs.FS) (map[pkg][]pkg, error)` via `go/parser` with `ImportsOnly`;
   `Direction(graph) []Violation`; `HubAllow(graph, allow []string) []Violation` (both
   directions); `Markers(fsys, index []SRow) ([]Marker, []Violation)`; `ParseIndex(r io.Reader)
   []SRow` over the `## Semantic owners` table (owner and duplicates columns).
2. Fixture tree with one violation per rule plus a clean control; `TestRulesFixture` (planned) asserts
   the exact violation set, with the decoys: a `_test.go` importing cmd (ignored), a marker in
   a listed duplicate (allowed), a `cmd/x/internal` import (allowed).
3. The three real-tree tests, each failing with a message that names the file, the rule and
   the register row. `hub-allow.txt` and `markers.txt` written from the head.
4. Seed markers on the owner functions the S- rows name under `tools/desk`, verified by
   reading each row's owner path.
5. `docs/contracts.md`: the marker convention under "How a brief cites this"; rows
   `R-dep-direction`, `R-hub-allowlist`, `R-one-implementation` (serving `S-semantic-index`,
   catch source: the test name).
6. Changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Rows 3–5 are the mutation rows, one per
rule. Row 6 dereferences the allow-list against `go list`. Row 7 is the three-state row. Row 8
checks the register rows landed and serve an existing S- row.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./internal/arch/ -count=1` | `ok` |
| 2 | `cd tools/desk && go test ./internal/arch/ -run TestRulesFixture -count=1 -v \| grep -c '^--- PASS: TestRulesFixture '` | `1` (the top-level test passes; subtests are not counted) |
| 3 | `cd tools/desk && printf 'package deskkit\nimport _ "github.com/medici-finance/assay/tools/desk/cmd/deskfile"\n' > internal/deskkit/zz_arch_mutation.go && go test ./internal/arch/ -run TestDependencyDirection -count=1 > /tmp/bl10-m1.out 2>&1; rc=$?; rm -f internal/deskkit/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'R-dep-direction' /tmp/bl10-m1.out` | `1` (an internal→cmd import is red and names the rule) |
| 4 | `cd tools/desk && printf 'package deskkit\nimport _ "github.com/medici-finance/assay/tools/desk/internal/forgeban"\n' > internal/deskkit/zz_arch_mutation.go && go test ./internal/arch/ -run TestHubAllowList -count=1 > /tmp/bl10-m2.out 2>&1; rc=$?; rm -f internal/deskkit/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'R-hub-allowlist' /tmp/bl10-m2.out` | `1` (an unlisted hub import is red) |
| 5 | `cd tools/desk && printf 'package forgeban\n// semantic: S-claim\nfunc zzMutation() {}\n' > internal/forgeban/zz_arch_mutation.go && go test ./internal/arch/ -run TestOneImplementationPerMeaning -count=1 > /tmp/bl10-m3.out 2>&1; rc=$?; rm -f internal/forgeban/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'implemented outside its owner' /tmp/bl10-m3.out` | `1` (a declared implementation outside the owner and its listed duplicates is red) |
| 6 | `cd tools/desk && a=$(grep -v '^#' internal/arch/hub-allow.txt \| sort); b=$(go list -f '{{join .Imports "\n"}}' ./internal/deskkit \| grep 'tools/desk/internal/' \| sed 's#.*/internal/##' \| sort); test "$a" = "$b" && echo MATCH \|\| { echo "allow: $a"; echo "golist: $b"; }` | `MATCH` (the committed allow-list equals the compiler's view) |
| 7 | `cd tools/desk && d=$(mktemp -d) && cp -R . "$d/desk" && cd "$d/desk" && go test ./internal/arch/ -run TestOneImplementationPerMeaning -count=1 -v \| grep -c 'could-not-check (no semantic index)'` | `1` (a checkout without docs/contracts.md is could-not-check, never a pass) |
| 8 | `for r in R-dep-direction R-hub-allowlist R-one-implementation; do grep -cE "^[\|] *$r .*S-semantic-index" docs/contracts.md; done \| grep -c '^1$'` | `3` |
| 9 | `n=$(git grep -c '^// semantic: S-' -- 'tools/desk/**/*.go' \| awk -F: '{s+=$2} END{print s+0}'); test "$n" -ge 5 && echo "markers=$n"` | `markers=` ≥ 5 (the owners are declared, so rule 3 cannot pass vacuously) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/10$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && n=$(git diff --numstat "$base" "$tip" -- tools/desk/cmd \| grep -v '_test.go$' \| awk '$1 > 1 \|\| $2 > 0' \| wc -l \| tr -d ' '); test "$n" = 0 && echo MARKERS-ONLY` | `MARKERS-ONLY` (each shipped `cmd` source file the brief touches gains at most one line, the marker comment, and loses none; base derived as in row 9 of brief 08, never `HEAD~1`) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
### Verification — 2026-10-01, non-implementer verifier @ b5e53a6a2dbc

What moved since the last run: this is the first verify pass (the brief had no earlier Evidence). The implementation landed in 921c22b0a (#1930). This pass ran against merged main b5e53a6a2dbc. The only commit after the snapshot that dispatch cut (b7ca79ab7) changes a windows-port brief and nothing under tools/desk. Toolchain: go1.27.1 darwin/arm64. Each row was run exactly as authored, from the repository root or with the `cd tools/desk` the row names. In the table, `\|` is the markdown escape for a literal shell pipe.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go test ./internal/arch/ -count=1` | `ok` | Verify row 1. exit 0. `ok github.com/medici-finance/assay/tools/desk/internal/arch 0.857s`. The `-v` form lists TestDependencyDirection, TestHubAllowList, the rule-3 test (markers=9 over 11 index rows), the missing-index test, TestReportCloneDensity and TestRulesFixture, and all of them report PASS. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./internal/arch/ -run TestRulesFixture -count=1 -v \| grep -c '^--- PASS: TestRulesFixture '` | `1` | Verify row 2. exit 0. Output `1`. The fixture runs 10 subtests: direction, hub, markers, ratchet-at-ceiling, ratchet-over-ceiling, ratchet-container-owner, grow-annotation, ratchet-owner-undeclared, marker-placement and index. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && printf 'package deskkit\nimport _ "github.com/medici-finance/assay/tools/desk/cmd/deskfile"\n' > internal/deskkit/zz_arch_mutation.go && go test ./internal/arch/ -run TestDependencyDirection -count=1 > /tmp/bl10-m1.out 2>&1; rc=$?; rm -f internal/deskkit/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'R-dep-direction' /tmp/bl10-m1.out` | `1` | Verify row 3. exit 0 (the inner go test exited 1). Output `1`. The failing line reads `R-dep-direction: tools/desk/internal/deskkit/zz_arch_mutation.go:2: internal/deskkit imports cmd/deskfile: a package under internal/ never imports a command`. The mutation file was removed and the worktree is clean afterwards. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && printf 'package deskkit\nimport _ "github.com/medici-finance/assay/tools/desk/internal/forgeban"\n' > internal/deskkit/zz_arch_mutation.go && go test ./internal/arch/ -run TestHubAllowList -count=1 > /tmp/bl10-m2.out 2>&1; rc=$?; rm -f internal/deskkit/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'R-hub-allowlist' /tmp/bl10-m2.out` | `1` | Verify row 4. exit 0 (the inner go test exited 1). Output `1`. The failing line reads `R-hub-allowlist: tools/desk/internal/deskkit/zz_arch_mutation.go:2: internal/deskkit imports internal/forgeban, which is not on hub-allow.txt`. The mutation file was removed. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && printf 'package forgeban\n// semantic: S-claim\nfunc zzMutation() {}\n' > internal/forgeban/zz_arch_mutation.go && go test ./internal/arch/ -run TestOneImplementationPerMeaning -count=1 > /tmp/bl10-m3.out 2>&1; rc=$?; rm -f internal/forgeban/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'implemented outside its owner' /tmp/bl10-m3.out` | `1` | Verify row 5. exit 0 (the inner go test exited 1). Output `1`. The failing line reads `R-one-implementation: ... S-claim implemented outside its owner at tools/desk/internal/forgeban/zz_arch_mutation.go:2`. The ceiling violation `S-claim has 2 declared implementations, over its markers.txt ceiling of 1` also fires. The mutation file was removed. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && a=$(grep -v '^#' internal/arch/hub-allow.txt \| sort); b=$(go list -f '{{join .Imports "\n"}}' ./internal/deskkit \| grep 'tools/desk/internal/' \| sed 's#.*/internal/##' \| sort); test "$a" = "$b" && echo MATCH \|\| { echo "allow: $a"; echo "golist: $b"; }` | `MATCH` | Verify row 6. exit 0. Output `MATCH`. The allow-list is gitcore and topology, which is the same set go list reports. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && d=$(mktemp -d) && cp -R . "$d/desk" && cd "$d/desk" && go test ./internal/arch/ -run TestOneImplementationPerMeaning -count=1 -v \| grep -c 'could-not-check (no semantic index)'` | `1` | Verify row 7. exit 0. Output `1`. In the copy, the rule-3 test takes its three-state could-not-check exit with the message `could-not-check (no semantic index): <tmp>/desk is not tools/desk inside a repository carrying docs/contracts.md`, and it does not report a green rule-3 result. This is the outcome the row expects. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `for r in R-dep-direction R-hub-allowlist R-one-implementation; do grep -cE "^[\|] *$r .*S-semantic-index" docs/contracts.md; done \| grep -c '^1$'` | `3` | Verify row 8. exit 0. Output `3`. The three register rows sit at docs/contracts.md lines 243 to 245, and each one serves S-semantic-index. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `n=$(git grep -c '^// semantic: S-' -- 'tools/desk/**/*.go' \| awk -F: '{s+=$2} END{print s+0}'); test "$n" -ge 5 && echo "markers=$n"` | `markers=` ≥ 5 | Verify row 9. exit 0. Output `markers=15`. Of these, 6 are fixture markers under internal/arch/testdata and 9 are real-tree markers. The real-tree count equals the test's own `markers=9` and still clears ≥ 5 (see finding F1). | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/10$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && n=$(git diff --numstat "$base" "$tip" -- tools/desk/cmd \| grep -v '_test.go$' \| awk '$1 > 1 \|\| $2 > 0' \| wc -l \| tr -d ' '); test "$n" = 0 && echo MARKERS-ONLY` | `MARKERS-ONLY` | Verify row 10. exit 0. Output `MARKERS-ONLY`. impl resolves to 921c22b0a072f14c0a60b02dfedbdae672ed26d7. Under tools/desk/cmd, the numstat shows exactly four files at `1 0`: deskclaim-ref/claim.go, deskflip/flip.go, deskpost/ready.go and deskwt/deskwt.go. | 2026-10-01 | assay-verifier-app[bot] @ b5e53a6a2dbc (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Scope traceability.** Each row above discharges the Verify row with the same number. Some verified work maps to no Verify row:
- The Review gate question (does each marker sit on the computing function?) was spot-checked by reading the declaration directly under each of the 9 real-tree markers. The functions are cmdAcquire (S-claim), flip, latestAppVerdict and ReduceAppVerdict (S-review-verdict), cmdAdd (S-worktree), scanSurface (S-publication-scan), ExitCodeOf (S-exit-codes), and splitBotEntry and CommitEmailSpec (S-identity). None of them is an obvious pass-through wrapper. The model reviewer still owns that judgment.
- The owner-presence check reports its owner half as could-not-check for five rows, and all five are legitimate. S-eligibility, S-decision-acceptance and S-status-derivation have their owners in statusgen, which the brief puts out of scope. S-delivery has no declared owner. S-semantic-index is owned by docs/contracts.md itself.
- Clone density is reported as could-not-check (no dupl) by TestReportCloneDensity. Under the brief it is reported and never ratcheted, and no Verify row covers it.

**F1 (minor, non-blocking).** Row 9's `git grep` also counts the 6 fixture markers under internal/arch/testdata. The row therefore does not isolate real-tree declarations, although the real-tree count (9) clears the bar independently. Excluding the testdata path would make the row strictly probative.

**Risk-bearing value.** The trigger does not fire. The risk metadata is present and every field is "no", the item is not irreversible, and gate is model. The enumeration was done anyway. The diff (921c22b0a, excluding testdata) introduces these literals:
- `S-claim 1`, `S-exit-codes 1`, `S-identity 2`, `S-publication-scan 1`, `S-review-verdict 3` and `S-worktree 1`, at tools/desk/internal/arch/markers.txt lines 8 to 13.
- hub-allow entries `gitcore` and `topology`, at tools/desk/internal/arch/hub-allow.txt lines 12 and 13.
- `hubLanding = {"gitcore","topology"}`, at tools/desk/internal/arch/arch_test.go line 38.
- The dupl threshold `-t 60`, at tools/desk/internal/arch/arch_test.go line 166.
- `Hub = "internal/deskkit"`, at tools/desk/internal/arch/arch.go line 67.

Rule 1's ceiling of 0 is structural, because any violation fails; it is not a literal. All of these values can be reversed with an edit and a redeploy of a test-only package. The top-ranked entries are the markers.txt ceilings and the hub allow-list, because a wrong value would let the ratchet either grandfather drift or go red spuriously.
- The ceilings are derived from the brief's own constraint ("ceilings equal the seeded counts"). Counting the real-tree markers per id gives 1, 1, 2, 1, 3 and 1, which is 9 in total. That matches every line.
- The allow-list is derived from the compiler's view (row 6 MATCH).

rows_passed=10 rows_total=10

RISK-VALUE: DERIVED — S-review-verdict = 3 @ tools/desk/internal/arch/markers.txt:12 (and siblings at lines 8 to 13) — each ceiling equals the count of real-tree `// semantic:` markers for that id at merged main (1/1/2/1/3/1), which is the brief's stated "ceilings equal the seeded counts" constraint, so the ratchet starts tight
RISK-VALUE: DERIVED — hub-allow = {gitcore, topology} @ tools/desk/internal/arch/hub-allow.txt:12-13 — equals `go list -f '{{join .Imports "\n"}}' ./internal/deskkit` filtered to tools/desk/internal at merged main (row 6 MATCH), the brief's stated source for the landing set

VERIFY: PASS

### Execution witness @ ca81ea0a9603 — 2026-09-30 assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5) (on-behalf-of human:ian)

What moved since the last run: nothing in the Verify table or the implementation (#1930, merged as 921c22b0a072); the hand-run block above recorded PASS on all ten rows but carried no execution-witness table, so the `verified` closure could not lint. This block adds that witness, produced by `statusgen verifyrun` (built from this tree at ca81ea0a9603) against merged main ca81ea0a960387bb34f9dbc22b188802097d5251. verifyrun exited 0. The table below is exactly as emitted.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/arch/ -count=1` | pass exit=0 | sha256:28d0c2a7412a | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/arch/ -run TestRulesFixture -count=1 -v \| grep -c '^--- PASS: TestRulesFixture '` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && printf 'package deskkit\nimport _ "github.com/medici-finance/assay/tools/desk/cmd/deskfile"\n' > internal/deskkit/zz_arch_mutation.go && go test ./internal/arch/ -run TestDependencyDirection -count=1 > /tmp/bl10-m1.out 2>&1; rc=$?; rm -f internal/deskkit/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'R-dep-direction' /tmp/bl10-m1.out` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && printf 'package deskkit\nimport _ "github.com/medici-finance/assay/tools/desk/internal/forgeban"\n' > internal/deskkit/zz_arch_mutation.go && go test ./internal/arch/ -run TestHubAllowList -count=1 > /tmp/bl10-m2.out 2>&1; rc=$?; rm -f internal/deskkit/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'R-hub-allowlist' /tmp/bl10-m2.out` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && printf 'package forgeban\n// semantic: S-claim\nfunc zzMutation() {}\n' > internal/forgeban/zz_arch_mutation.go && go test ./internal/arch/ -run TestOneImplementationPerMeaning -count=1 > /tmp/bl10-m3.out 2>&1; rc=$?; rm -f internal/forgeban/zz_arch_mutation.go; test $rc -ne 0 && grep -c 'implemented outside its owner' /tmp/bl10-m3.out` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && a=$(grep -v '^#' internal/arch/hub-allow.txt \| sort); b=$(go list -f '{{join .Imports "\n"}}' ./internal/deskkit \| grep 'tools/desk/internal/' \| sed 's#.*/internal/##' \| sort); test "$a" = "$b" && echo MATCH \|\| { echo "allow: $a"; echo "golist: $b"; }` | pass exit=0 | sha256:9160780d5c50 | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && d=$(mktemp -d) && cp -R . "$d/desk" && cd "$d/desk" && go test ./internal/arch/ -run TestOneImplementationPerMeaning -count=1 -v \| grep -c 'could-not-check (no semantic index)'` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 8 | `for r in R-dep-direction R-hub-allowlist R-one-implementation; do grep -cE "^[\|] *$r .*S-semantic-index" docs/contracts.md; done \| grep -c '^1$'` | pass exit=0 | sha256:1121cfccd591 | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 9 | `n=$(git grep -c '^// semantic: S-' -- 'tools/desk/**/*.go' \| awk -F: '{s+=$2} END{print s+0}'); test "$n" -ge 5 && echo "markers=$n"` | pass exit=0 | sha256:01747285dd0f | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/10$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && n=$(git diff --numstat "$base" "$tip" -- tools/desk/cmd \| grep -v '_test.go$' \| awk '$1 > 1 \|\| $2 > 0' \| wc -l \| tr -d ' '); test "$n" = 0 && echo MARKERS-ONLY` | pass exit=0 | sha256:38005de8dd2e | 2026-09-30 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |

Per Verify row (the witness row number is the Verify row it discharges; output hashes checked against the expected literal output):

- Verify row 1: pass, exit 0 (go test of the arch package, `ok`).
- Verify row 2: pass, exit 0, output `1` (sha256:4355a46b19d3 is the hash of `1`): the top-level fixture test passes.
- Verify row 3: pass, exit 0, output `1`: the internal-to-cmd mutation is red and names R-dep-direction.
- Verify row 4: pass, exit 0, output `1`: the unlisted hub import mutation is red and names R-hub-allowlist.
- Verify row 5: pass, exit 0, output `1`: the out-of-owner S-claim marker mutation is red with "implemented outside its owner".
- Verify row 6: pass, exit 0, output `MATCH` (sha256:9160780d5c50): hub-allow.txt (gitcore, topology) equals `go list`.
- Verify row 7: pass, exit 0, output `1`: a copy of tools/desk without the semantic index reports could-not-check.
- Verify row 8: pass, exit 0, output `3` (sha256:1121cfccd591): all three register rows serve S-semantic-index.
- Verify row 9: pass, exit 0, output `markers=15` (sha256:01747285dd0f). Finding: the count includes 6 fixture markers under the arch package's testdata tree; the real-tree count is 9 (S-claim 1, S-exit-codes 1, S-identity 2, S-publication-scan 1, S-review-verdict 3, S-worktree 1), still at least 5, so the row's intent holds.
- Verify row 10: pass, exit 0, output `MARKERS-ONLY` (sha256:38005de8dd2e); the implementing merge resolved to 921c22b0a072 (#1930), not the fallback base.

No row needed Linux or network-off; every row ran on this host (darwin/arm64, go1.27.1). The mutation rows removed their mutation files (tree clean after the run apart from the witness append). No verified work maps to no Verify row.

Risk-value enumeration (frontmatter risk: all four no, irreversible: no; the diff touches no risk-classed path). Literals introduced by the diff: S-claim 1, S-exit-codes 1, S-identity 2, S-publication-scan 1, S-review-verdict 3, S-worktree 1 in tools/desk/internal/arch/markers.txt lines 8 to 13; allow-list entries gitcore and topology in tools/desk/internal/arch/hub-allow.txt lines 12 and 13; the reported-only clone threshold `dupl -t 60` in tools/desk/internal/arch/arch_test.go line 166. All are reversible CI knobs (an edit and a re-run undo them). The ceilings are derived: the brief requires ceilings equal to the seeded counts, and `git grep` over the real tree (fixtures excluded) gives exactly 1/1/2/1/3/1. The allow-list is derived from the compiler's view (Verify row 6 MATCH).

rows_passed=10 rows_total=10

RISK-VALUE: DERIVED — S-review-verdict = 3 @ tools/desk/internal/arch/markers.txt:12 (and the five sibling ceilings at lines 8 to 13) — equals the real-tree marker count per meaning, as the brief's "ceilings equal the seeded counts" requires; the allow-list gitcore/topology @ tools/desk/internal/arch/hub-allow.txt:12-13 equals `go list` (row 6). All values are reversible CI knobs.

VERIFY: PASS

## Review
Gate: model (from frontmatter). The reviewer answers: does each seeded marker sit on the
function that actually computes the meaning, or on a wrapper? A marker on a wrapper declares
the wrong owner and the ratchet then protects the wrong thing. The reviewer also confirms rule
1 starts at zero violations without any grandfathering.
