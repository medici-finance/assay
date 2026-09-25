---
brief: assay:assay:build-less-brittle:08
title: "Hotspot metric (churn × complexity, temporal coupling) from git history, and the brittle mark"
why: >-
  The weight counter (03) says how much machinery there is; it does not say where the next fix
  will land. Forty years of defect-prediction work agrees on that answer: change history beats
  static complexity, and a small share of files takes most of the changes and defects. In this
  tree, 10 of 550 non-test Go files took 28% of 630 commits in 90 days, and three forge files
  co-changed in 15–17 commits. A cheap, reproducible hotspot report from git history, combined
  with the class defect history from 04, nominates the modules to mark `brittle`, so the
  investigation (09) and the review stage (06) spend strong-tier attention where it pays.
wave: 1
depends: ["build-less-brittle/01"]
unblocks: ["build-less-brittle/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; SOTA amendment)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 row 10, §4.8, §5.1 M5, §11"
  - "docs/streams/build-less-brittle/spec.md §11 (the SOTA table: Tornhill hotspots; Nagappan & Ball relative churn; Graves et al. change history; Rahman & Devanbu process > product metrics)"
  - "tools/desk/internal/forgeban/allowlist.go (the test-package precedent) and build-less-brittle/03 (the -root/-rev test-flag pattern)"
  - "statusgen/alarms.go, statusgen/emit.go (findings entries render on the board as Unresolved findings; doracli.go counts them in the change-fail proxy)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no hotspot, churn or coupling report exists in tools/desk or statusgen; git history is the only input and is read-only"
exec-tier: strong
exec-tier-why: "(b) the metric definitions are the shared measuring stick for the brittle mark, the investigation trigger (09), the review-stage trigger (06) and a project's baseline; they must reproduce at old SHAs and windows."
domain: clear
consumers:
  - "docs/contracts.md (a `## Brittle marks` section beside the semantic index): follow-up build-less-brittle/08 (this brief)"
  - "docs/streams/findings/ entries of the form F-brittle-<module>: follow-up build-less-brittle/08 (this brief; the board renders them with no statusgen change)"
  - "build-less-brittle/04 intake procedure (module: line on class instances): follow-up build-less-brittle/04 (amended by this stream; see its Task step 2)"
  - "build-less-brittle/06 review stage (brittle-module trigger): follow-up build-less-brittle/06 (amended; see its facts)"
---

# Brief 08 — Hotspot metric and the brittle mark

## Context

files:
- `tools/desk/internal/hotspot/hotspot.go` (planned): NEW. Pure functions over a parsed `git log` stream and a file tree: churn, fix share, indentation complexity, score, temporal coupling.
- `tools/desk/internal/hotspot/hotspot_test.go` (planned): NEW. `TestHotspotFixture` (planned), `TestCouplingFixture` (planned), `TestPrintHotspots` (planned) (test-only flags `-root`, `-since`, `-until`, `-top`), `TestShallowIsCouldNotCheck` (planned).
- `tools/desk/internal/hotspot/testdata/log.txt` (planned): NEW. A captured `git log --name-only` fixture with hand-known counts.
- `docs/contracts.md`: add `## Brittle marks` after the semantic-owner index (brief 01).
- `docs/streams/findings/<date>-brittle-<module>.md`: one entry per mark, `id: F-brittle-<module>`, `resolved: false` until cleared.
- `changelog/build-less-brittle-08.md` (planned)

facts:
- **What SOTA measures** (spec §11). Hotspot = change frequency × complexity, with complexity
  taken as indentation depth rather than cyclomatic complexity because it is language-neutral
  and correlates about as well (Tornhill, *Your Code as a Crime Scene*; *Software Design
  X-Rays*). Relative churn predicts defect density better than size or static complexity
  (Nagappan & Ball, ICSE 2005; Graves et al., TSE 2000; Rahman & Devanbu, ICSE 2013). Temporal
  coupling (files that change together without a static dependency) flags a hidden seam; it is
  an **architecture** signal, not a defect predictor (Graves et al. found co-change a poor
  fault predictor), so it feeds the investigation's reading, never the mark's key 1 alone.
  Hotspots are a **ranking**, not an absolute threshold: a few per cent of files carry most of
  the change. A ranking nobody acts on changes nothing (Lewis et al., ICSE 2013: Google's bug
  predictor produced "no identifiable change in developer behavior"), which is why every mark
  here is bound to a next step (09). This brief implements the ranking; the mark adds the
  defect history.
- **Definitions** (all over non-`_test.go` `.go` files under `tools/desk`, window `[since,
  until]`, default the trailing 90 days, UTC):
  - `churn`: commits on the first-parent line of `origin/main` that touch the file
    (`git log --first-parent --since --until --name-only --format=%H`). Merge-squash trees
    count once.
  - `fixes`: those commits whose subject matches `(?i)\bfix(es|ed)?\b|revert` (a proxy, stated
    as one; the class record is the real defect history).
  - `complexity`: the sum of leading tab counts over the file's lines at `until`
    (indentation sum). Reported beside `lines`.
  - `score`: `churn × complexity`, ranked descending. `rank-pct` is the file's percentile.
  - `coupling`: for commits touching ≤ 8 files, the pair count; a pair is reported when
    `count ≥ 5` and `count / min(churn_a, churn_b) ≥ 0.3` (Tornhill's filters, stated in the
    output header so a reader can recompute).
- **A sample at f7bde6bfa, trailing 90 days** (authoring-time shell proxies over ALL commits;
  the counter's first-parent figures run slightly lower, e.g. `forge_gitlab.go` 40 first-parent
  vs 41 all-commit, and the counter's figures are the ones the baseline records): 630 commits
  touched `tools/desk`; the top 10 files by churn (1.8% of 550)
  appear in 178 of them (28%). Top churn: `tools/desk/internal/deskkit/forge_gitlab.go` 41 commits / 18
  fix-commits / indentation sum 4,640; `tools/desk/cmd/deskdispatch/dispatch.go` 36 / 23 / 2,265;
  `tools/desk/internal/deskkit/forge_github.go` 34 / 12 / 3,540; `tools/desk/cmd/deskflip/flip.go` 30 / 16 / 2,356;
  `tools/desk/cmd/deskboard/board.go` 24 / 11 / 3,895. Top coupling: `forge.go`–`forge_github.go` 17,
  `forge.go`–`forge_gitlab.go` 16, `forge_github.go`–`forge_gitlab.go` 15. A project's baseline
  (spec §5.4) records the counter's own figures at the pin and sets the cut.
- **The mark is a two-key decision, never the metric alone.** A module (an `S-` owner path or
  a `cmd/<verb>` directory) is marked `brittle` when BOTH hold at the monthly pass:
  1. **nominated by the metric**: at least one of its files is in the top `N%` by score
     (project value; default 2%) with `fixes ≥ 2` in the window. (A module that only holds
     the other end of a nominated file's coupling pair is `watch`, and the pair is recorded on
     the nominated module's row for the investigation to read.) And
  2. **confirmed by history**: an `error-class` issue (04) records ≥ 2 counted instances in
     it, OR a chain arrow at `introduced-by-commit` lands in it (the baseline's re-graded chains).
  A module that meets 1 only is `watch`; 2 only is a class-issue matter and no mark. The
  pass is the monthly rule diet's (07) sibling, run by the same role on the same cadence.
- **Where the mark lands, using what exists.** (a) `docs/contracts.md` `## Brittle marks`:
  `module | S- row | since | churn | fixes | complexity | coupling partner(s) | class issue |
  investigation | cleared`. (b) The board: a findings entry `F-brittle-<module>` with
  `affects:` naming the streams whose briefs touch the module. Findings entries already render
  under "Unresolved findings" and already feed the change-fail proxy in `doracli.go`, so no
  statusgen change is needed. This repository has no `docs/streams/findings/` directory at
  the pin: the loader reads `<root>/docs/streams/findings/` when it exists, and the first
  mark's entry creates it (this brief marks nothing, so it adds no entry). Row 8 proves an
  entry there lints; row 8a proves the loader reads it. (c) The class issue gets the label `brittle` (a label, not a
  status token). The mark is **cleared** when 09's recommendation has merged and the next pass
  no longer nominates the module; the findings entry flips `resolved: true` in that PR.
- **Never a CI gate.** The report needs history; GitHub-hosted checkouts are shallow, and a
  ranking is not a pass/fail. `TestPrintHotspots` (planned) is a report; on a shallow clone (`git
  rev-parse --is-shallow-repository` = true) it logs `could-not-check (shallow)` and skips.
  The ratchets live in 03 and 10.
- **layering:** flat tool. One package with pure functions over a parsed log; the only `git`
  reads are in the test file's `-root` path, read-only.

design-fit:
  owner: tools/desk/internal/hotspot (new; no existing owner reads change history)
  contract: none — new instrument; its mark table lives beside S- rows in docs/contracts.md
  retires: []
  weight: verbs 0, flags 0 (test-only flags live in _test.go), refusals 0, rule-text lines 0
  why-add: n/a (no ratcheted growth); ~250 lines of pure counting reported under golines. The alternative, hand-listing brittle areas from memory, is what the review's chains show does not happen.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No new verb and no new flag on any shipped binary. This is a test package plus a docs section.
- Read history only; never rewrite it. `git log` and `git show` are the only git calls.
- This brief marks nothing. The first marks are made at the first monthly pass after a project's go-live.

## Task

1. Write `hotspot.go`: `Parse(r io.Reader) ([]Commit, error)` over `git log --name-only
   --format=%H%x00%ci%x00%s`; `Score(commits, tree fs.FS, opts) []FileScore`;
   `Coupling(commits, opts) []Pair`. Complexity reads the tree, never the log.
2. Fixture: `testdata/log.txt` with 12 commits over 6 files and a tree under `testdata/tree/`
   with hand-known indentation sums. Include decoys: a `_test.go` path, a merge commit touching
   40 files (excluded from coupling), a subject with "prefix" (must NOT match `fix`).
   `TestHotspotFixture` (planned) asserts churn, fixes, complexity and rank for every file;
   `TestCouplingFixture` (planned) asserts exactly the pairs that pass both filters.
3. `TestPrintHotspots` (planned) logs, for `-root <dir>` (default the repo, via `git -C`), `-since`,
   `-until` and `-top N` (default 10), a header line stating the window and filters, one line
   per file `hotspot: rank=<n> score=<n> churn=<n> fixes=<n> complexity=<n> lines=<n>
   pct=<p> <path>`, and one line per pair `coupling: <a> <b> count=<n> ratio=<r>`.
4. `TestShallowIsCouldNotCheck` (planned): on a shallow clone the test skips with
   `could-not-check (shallow)`; prove it against a `git clone --depth 1` of the repo into a
   temp dir.
5. `docs/contracts.md`: add `## Brittle marks` with the two-key rule, the table (empty rows
   are fine), the `watch` state, and the clearing rule. State that the module named is an
   `S-` owner path or a `cmd/<verb>` directory, never a bare file.
6. Changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Row 4 dereferences the counter against an
independent shell count. Row 5 is the negative control for the `fix` proxy. Row 6 proves the
three-state behaviour. Row 8 is the neighbour row: the findings loader still lints a
`F-brittle-` entry shape; row 8a is its negative control.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./internal/hotspot/ -count=1` | `ok` |
| 2 | `cd tools/desk && go test ./internal/hotspot/ -run TestHotspotFixture -count=1 -v > /tmp/bl08-r2.out && go test ./internal/hotspot/ -run TestCouplingFixture -count=1 -v >> /tmp/bl08-r2.out && grep -c -e '^--- PASS: TestHotspotFixture ' -e '^--- PASS: TestCouplingFixture ' /tmp/bl08-r2.out` | `2` (two single-pattern runs, chained; each top-level test passes once) |
| 3 | `cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -since=2026-06-26 -until=2026-09-24 -top=5 \| grep -cE '^ *hotspot_test.go:[0-9]+: hotspot: rank=[1-5] score=[0-9]+ churn=[0-9]+ fixes=[0-9]+ complexity=[0-9]+ lines=[0-9]+ pct=[0-9.]+ tools/desk/'` | `5` |
| 4 | `top=$(cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -since=2026-06-26 -until=2026-09-24 -top=1 \| grep -oE 'churn=[0-9]+ .* (tools/desk/[^ ]+)$' \| sed -E 's/churn=([0-9]+) .* (tools\/desk\/[^ ]+)$/\1 \2/'); c=${top%% *}; f=${top##* }; s=$(git log --first-parent --since=2026-06-26 --until=2026-09-24 --format=%H refs/remotes/origin/main -- "$f" \| wc -l \| tr -d ' '); test "$c" = "$s" && echo MATCH \|\| echo "counter=$c shell=$s"` | `MATCH` (the top file's churn equals an independent first-parent count) |
| 5 | `cd tools/desk && go test ./internal/hotspot/ -run TestHotspotFixture -count=1 -v \| grep -c 'prefix-decoy fixes=0'` | `1` (a subject containing "prefix" does not count as a fix) |
| 6 | `d=$(mktemp -d) && git clone -q --depth 1 file://"$PWD" "$d/shallow" && cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -root="$d/shallow" \| grep -c 'could-not-check (shallow)'` | `1` (a shallow clone is could-not-check, never an empty ranking) |
| 7 | `grep -c '^## Brittle marks$' docs/contracts.md && sed -n '/^## Brittle marks/,$p' docs/contracts.md \| grep -c -e 'nominated' -e 'confirmed' -e 'watch' -e 'cleared'` | `1`, then ≥ `4` |
| 8 | `cd statusgen && go build -o /tmp/bl08-statusgen . && cd .. && rm -rf /tmp/bl08 && mkdir -p /tmp/bl08 && git archive HEAD \| tar -x -C /tmp/bl08 && mkdir -p /tmp/bl08/docs/streams/findings && printf -- '---\nid: F-brittle-example\ndate: "2026-09-24"\ntitle: "brittle: example module"\naffects: []\nack: ""\nresolved: false\n---\nbody\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && /tmp/bl08-statusgen --root /tmp/bl08 --lint; echo "exit=$?"` | `exit=0` (an `F-brittle-<module>` entry is a valid findings entry as the board loader stands; no statusgen change. The fixture is a full copy of the tree at `HEAD`, because the lint resolves every backticked path against its root, and it creates `docs/streams/findings/`, which this repository does not have yet) |
| 8a | `test -x /tmp/bl08-statusgen && test -d /tmp/bl08/docs/streams/findings && printf 'no frontmatter\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && /tmp/bl08-statusgen --root /tmp/bl08 --lint 2>&1 \| grep -c 'parsing findings/2026-09-24-brittle-example.md'` | `1` (negative control, run after row 8: the loader does read that directory, so a malformed entry there is reported and row 8's `exit=0` is not vacuous) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/08$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git diff --name-only "$base" "$tip" -- tools/desk/cmd \| grep -v _test.go \| wc -l \| tr -d ' ')" = 0 && echo NO-CMD-CHANGE` | `NO-CMD-CHANGE` (no shipped verb or flag changed; the package is test-only. The base is derived, never `HEAD~1`: on merged main it is the parent of the squash commit carrying `Brief: build-less-brittle/08`, before merge it is the merge-base) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer checks the decoys bite: a counter that counts
`_test.go` files, includes the 40-file merge in coupling, or matches "prefix" as a fix passes
row 1 and fails only rows 2 and 5. The reviewer also confirms the docs section says the metric
nominates and history confirms, in that order, and that nothing in this brief marks a module.
