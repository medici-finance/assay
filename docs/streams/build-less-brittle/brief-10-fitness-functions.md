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

## Review
Gate: model (from frontmatter). The reviewer answers: does each seeded marker sit on the
function that actually computes the meaning, or on a wrapper? A marker on a wrapper declares
the wrong owner and the ratchet then protects the wrong thing. The reviewer also confirms rule
1 starts at zero violations without any grandfathering.
