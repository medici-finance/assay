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
version: 3
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
| 8 | `cd statusgen && go build -o /tmp/bl08-statusgen . && cd .. && rm -rf /tmp/bl08 && mkdir -p /tmp/bl08 && git archive HEAD \| tar -x -C /tmp/bl08 && mkdir -p /tmp/bl08/docs/streams/findings && printf -- '---\nid: F-brittle-example\ndate: "2026-09-24"\ntitle: "brittle: example module"\naffects: []\nack: ""\nresolved: false\n---\nbody\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && /tmp/bl08-statusgen --root /tmp/bl08 --lint; echo "exit=$?"` | output is `exit=0` (an `F-brittle-<module>` entry is a valid findings entry as the board loader stands; no statusgen change. The fixture is a full copy of the tree at `HEAD`, because the lint resolves every backticked path against its root, and it creates `docs/streams/findings/`, which this repository does not have yet). Expect re-written 2026-10-03 (#1862). |
| 8a | `test -x /tmp/bl08-statusgen && test -d /tmp/bl08/docs/streams/findings && printf 'no frontmatter\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && { out=$(/tmp/bl08-statusgen --root /tmp/bl08 --lint 2>&1); rc=$?; test "$rc" -eq 1 \|\| { echo "lint rc=$rc, want 1"; false; }; } && printf '%s\n' "$out" \| grep -c 'parsing findings/2026-09-24-brittle-example.md'` | output is `1` (negative control, run after row 8: the loader does read that directory, so a malformed entry there is reported and row 8's `exit=0` is not vacuous). The lint must also FAIL on the malformed entry: its status is captured and must be `1`, so a lint that prints the parse line but still passes fails this row, and a lint that crashes with another status fails it too. _Re-written 2026-09-30 per #1862: the former row piped the lint straight into `grep -c`. The lint returns `1` on purpose here, and the witness shell runs with `pipefail`, so the pipeline took the lint's status and the row failed on its own success path even though `grep -c` printed `1`. The lint now runs outside the pipeline, its status is asserted rather than discarded, and only the captured output is piped to `grep -c`._ |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/08$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git diff --name-only "$base" "$tip" -- tools/desk/cmd \| grep -v _test.go \| wc -l \| tr -d ' ')" = 0 && echo NO-CMD-CHANGE` | `NO-CMD-CHANGE` (no shipped verb or flag changed; the package is test-only. The base is derived, never `HEAD~1`: on merged main it is the parent of the squash commit carrying `Brief: build-less-brittle/08`, before merge it is the merge-base) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
### Non-implementer verifier run — VERIFY: PASS — 10/10 rows pass by hand, held (#1862) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Merged main 43420f7ecd743f5c930dc479f54f5ef5ca7b82ed (cross-checked against the forge's `commits/main`); implementing commit b6f8f3c4e (#1850). `gate: model`, all risk answers `no`. The first table is the `statusgen verifyrun --dry-run` execution witness, landed verbatim. It was run on Linux (linux/arm64 golang image, `--network none`, statusgen built from main's own source, the adopter roster mounted read-only): 9/10 pass. Row 8a records `fail exit=1` because of its own definition, not the product: the row pipes a lint that exits 1 on purpose into `grep -c`, and under verifyrun's `bash -o pipefail` the lint's exit wins even though grep prints `1`. Run without pipefail the same row prints `1` and exits 0. **Held at implemented on row 8a alone**; the row rewrite is routed on #1862.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/hotspot/ -count=1` | pass exit=0 | sha256:7349450258d6 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && go test ./internal/hotspot/ -run TestHotspotFixture -count=1 -v > /tmp/bl08-r2.out && go test ./internal/hotspot/ -run TestCouplingFixture -count=1 -v >> /tmp/bl08-r2.out && grep -c -e '^--- PASS: TestHotspotFixture ' -e '^--- PASS: TestCouplingFixture ' /tmp/bl08-r2.out` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -since=2026-06-26 -until=2026-09-24 -top=5 \| grep -cE '^ *hotspot_test.go:[0-9]+: hotspot: rank=[1-5] score=[0-9]+ churn=[0-9]+ fixes=[0-9]+ complexity=[0-9]+ lines=[0-9]+ pct=[0-9.]+ tools/desk/'` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 4 | `top=$(cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -since=2026-06-26 -until=2026-09-24 -top=1 \| grep -oE 'churn=[0-9]+ .* (tools/desk/[^ ]+)$' \| sed -E 's/churn=([0-9]+) .* (tools\/desk\/[^ ]+)$/\1 \2/'); c=${top%% *}; f=${top##* }; s=$(git log --first-parent --since=2026-06-26 --until=2026-09-24 --format=%H refs/remotes/origin/main -- "$f" \| wc -l \| tr -d ' '); test "$c" = "$s" && echo MATCH \|\| echo "counter=$c shell=$s"` | pass exit=0 | sha256:9160780d5c50 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && go test ./internal/hotspot/ -run TestHotspotFixture -count=1 -v \| grep -c 'prefix-decoy fixes=0'` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 6 | `d=$(mktemp -d) && git clone -q --depth 1 file://"$PWD" "$d/shallow" && cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -root="$d/shallow" \| grep -c 'could-not-check (shallow)'` | pass exit=0 | sha256:1561438aeff1 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 7 | `grep -c '^## Brittle marks$' docs/contracts.md && sed -n '/^## Brittle marks/,$p' docs/contracts.md \| grep -c -e 'nominated' -e 'confirmed' -e 'watch' -e 'cleared'` | pass exit=0 | sha256:8df72a86e2c3 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 8 | `cd statusgen && go build -o /tmp/bl08-statusgen . && cd .. && rm -rf /tmp/bl08 && mkdir -p /tmp/bl08 && git archive HEAD \| tar -x -C /tmp/bl08 && mkdir -p /tmp/bl08/docs/streams/findings && printf -- '---\nid: F-brittle-example\ndate: "2026-09-24"\ntitle: "brittle: example module"\naffects: []\nack: ""\nresolved: false\n---\nbody\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && /tmp/bl08-statusgen --root /tmp/bl08 --lint; echo "exit=$?"` | pass exit=0 | sha256:9caf8f729f8b | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 8a | `test -x /tmp/bl08-statusgen && test -d /tmp/bl08/docs/streams/findings && printf 'no frontmatter\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && /tmp/bl08-statusgen --root /tmp/bl08 --lint 2>&1 \| grep -c 'parsing findings/2026-09-24-brittle-example.md'` | fail exit=1 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/08$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git diff --name-only "$base" "$tip" -- tools/desk/cmd \| grep -v _test.go \| wc -l \| tr -d ' ')" = 0 && echo NO-CMD-CHANGE` | pass exit=0 | sha256:80a91942d970 | 2026-09-30 | assay-verifier-app[bot] @ 43420f7ecd74 (on-behalf-of human:ian) (git-config) |

Verifier detail (build-less-brittle/08 — NON-implementer, merged main 43420f7ecd74, 2026-09-30; host rows ran on darwin/arm64, offline):

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | row 1 hotspot package tests | ok | exit 0; `ok` for the hotspot package | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 2 | row 2 fixture tests, count both PASS lines | 2 | exit 0; printed `2` | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 3 | row 3 report over the fixed window, top 5 | 5 ranked lines | exit 0; printed `5`; header ref=refs/remotes/origin/main commits=1308 ranked=538; rank 1 is forge_gitlab.go at churn 38 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 4 | row 4 top file churn vs an independent first-parent git count | MATCH | exit 0; `MATCH` (38 and 38) | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 5 | row 5 prefix decoy is not a fix | 1 | exit 0; printed `1` | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 6 | row 6 shallow clone reads could-not-check | 1 | exit 0; printed `1`; temp clone removed | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 7 | row 7 contracts section plus its four states | 1 then >= 4 | exit 0; printed `1` then `10` | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 8 | row 8 findings loader accepts a brittle entry | LINT: PASS, exit=0 | exit 0; `LINT: PASS` / `exit=0` | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 8a | row 8a findings loader rejects a malformed entry | 1 | by hand exit 0, printed `1` (`parsing findings/2026-09-24-brittle-example.md: no frontmatter`); under pipefail exit 1 (row definition, #1862) | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |
| 9 | row 9 no shipped verb changed | NO-CMD-CHANGE | exit 0; `NO-CMD-CHANGE` (impl b6f8f3c4e touches nothing under tools/desk/cmd) | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 43420f7ecd74 |

Execution witness: `statusgen verifyrun --dry-run` (main-source build, Linux network-off) on 43420f7ecd74: rows 1-8 and 9 `pass exit=0`; row 8a `fail exit=1` (pipefail, see above).

Grounding before reading the diff: the fixture's expected values were re-derived by hand from testdata/log.txt (alpha/main.go churn 8 fixes 4; alpha/run.go 7/4; beta.go 6/3; store.go 5/2; gamma.go 3/2; prefix.go 2/0; only the alpha pair couples, at 6 commits, ratio 6/7; the beta/store pair shares 4 once the 40-file merge is excluded, below the minimum 5), and they match the tests. The contracts section states nominate before confirm, the watch state, the clearing rule and "never a bare file"; the mark table is empty and no brittle findings entry exists.

Risk-bearing values. Risk metadata is present and all-no; the diff is a new test-only package, one docs section and a changelog, so the fail-safe trigger does not fire. Enumerated anyway: the fix pattern, the prefix default, the changeset, count and ratio limits, the top and window defaults, and the nomination and confirmation keys. All are reversible report parameters; the report is never a CI gate and marks nothing.

RISK-VALUE: DERIVED — MinCount = 5 and MinRatio = 0.3 @ tools/desk/internal/hotspot/hotspot.go:78 and :81 — the brief's facts cite these coupling filters; the fixture rejects the beta pair at count 4 and a synthetic pair at ratio 0.2.
RISK-VALUE: DERIVED — MaxChangeset = 8 @ tools/desk/internal/hotspot/hotspot.go:75 — the brief's "commits touching ≤ 8 files"; the synthetic test counts 8-file commits and skips 9-file ones.
RISK-VALUE: DERIVED — FixPattern = `(?i)\bfix(es|ed)?\b|revert` @ tools/desk/internal/hotspot/hotspot.go:48 — identical to the brief's proxy definition; word boundaries keep "prefix" and "fixture" out (row 5).
RISK-VALUE: DERIVED — nomination cut "default 2%" with "fixes ≥ 2" @ docs/contracts.md:197 — the brief's two-key rule verbatim, marked as a project value the baseline sets.

Findings: (1) **held on #1862** — row 8a fails under pipefail by its own definition (the lint exits 1 on purpose); suggested rewrite on the issue. (2) Rows 3 and 4 pass bare-date bounds, which git reads as that date at the current time of day, so absolute churn figures drift with run time (38 here; 40 with full-day RFC 3339 bounds, the figure the brief records). Row 4 still compares like with like. A project baseline should record RFC 3339 bounds. (3) The brief's facts say there is no findings directory at the pin; one exists on main now. Row 8 is unaffected (it runs `mkdir -p` in a copied tree). (4) Rows 8 and 8a use fixed /tmp paths, so two concurrent verifiers on one host could collide.

VERIFY: PASS on all ten rows by hand; held at implemented until row 8a is rewritten (#1862).
### Verification — 2026-09-30 (assay-verifier-app[bot] @ 35496323b8fc (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer re-verify on merged main 35496323b8fc591651e44537baf51206cc22bbdd of #1850 (b6f8f3c4e). The row 8a rewrite landed as #1865 (dd9f8b630, for #1862). The prior pass held on row 8a alone. The pass grounded on the brief and re-derived the fixture ranking and coupling by hand before reading the tests; both match. First table: the `statusgen verifyrun` execution witness, landed verbatim. It ran on Linux (golang:1.25-bookworm, `--network none`, a full clone pinned to this SHA, statusgen built from main's own source) and passed 10/10. Second table: the hand run on the host.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/hotspot/ -count=1` | pass exit=0 | sha256:001504dd781d | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && go test ./internal/hotspot/ -run TestHotspotFixture -count=1 -v > /tmp/bl08-r2.out && go test ./internal/hotspot/ -run TestCouplingFixture -count=1 -v >> /tmp/bl08-r2.out && grep -c -e '^--- PASS: TestHotspotFixture ' -e '^--- PASS: TestCouplingFixture ' /tmp/bl08-r2.out` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -since=2026-06-26 -until=2026-09-24 -top=5 \| grep -cE '^ *hotspot_test.go:[0-9]+: hotspot: rank=[1-5] score=[0-9]+ churn=[0-9]+ fixes=[0-9]+ complexity=[0-9]+ lines=[0-9]+ pct=[0-9.]+ tools/desk/'` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 4 | `top=$(cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -since=2026-06-26 -until=2026-09-24 -top=1 \| grep -oE 'churn=[0-9]+ .* (tools/desk/[^ ]+)$' \| sed -E 's/churn=([0-9]+) .* (tools\/desk\/[^ ]+)$/\1 \2/'); c=${top%% *}; f=${top##* }; s=$(git log --first-parent --since=2026-06-26 --until=2026-09-24 --format=%H refs/remotes/origin/main -- "$f" \| wc -l \| tr -d ' '); test "$c" = "$s" && echo MATCH \|\| echo "counter=$c shell=$s"` | pass exit=0 | sha256:9160780d5c50 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && go test ./internal/hotspot/ -run TestHotspotFixture -count=1 -v \| grep -c 'prefix-decoy fixes=0'` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 6 | `d=$(mktemp -d) && git clone -q --depth 1 file://"$PWD" "$d/shallow" && cd tools/desk && go test ./internal/hotspot/ -run TestPrintHotspots -count=1 -v -args -root="$d/shallow" \| grep -c 'could-not-check (shallow)'` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 7 | `grep -c '^## Brittle marks$' docs/contracts.md && sed -n '/^## Brittle marks/,$p' docs/contracts.md \| grep -c -e 'nominated' -e 'confirmed' -e 'watch' -e 'cleared'` | pass exit=0 | sha256:8df72a86e2c3 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 8 | `cd statusgen && go build -o /tmp/bl08-statusgen . && cd .. && rm -rf /tmp/bl08 && mkdir -p /tmp/bl08 && git archive HEAD \| tar -x -C /tmp/bl08 && mkdir -p /tmp/bl08/docs/streams/findings && printf -- '---\nid: F-brittle-example\ndate: "2026-09-24"\ntitle: "brittle: example module"\naffects: []\nack: ""\nresolved: false\n---\nbody\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && /tmp/bl08-statusgen --root /tmp/bl08 --lint; echo "exit=$?"` | pass exit=0 | sha256:fb6032af40d7 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 8a | `test -x /tmp/bl08-statusgen && test -d /tmp/bl08/docs/streams/findings && printf 'no frontmatter\n' > /tmp/bl08/docs/streams/findings/2026-09-24-brittle-example.md && { out=$(/tmp/bl08-statusgen --root /tmp/bl08 --lint 2>&1); rc=$?; test "$rc" -eq 1 \|\| { echo "lint rc=$rc, want 1"; false; }; } && printf '%s\n' "$out" \| grep -c 'parsing findings/2026-09-24-brittle-example.md'` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/08$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git diff --name-only "$base" "$tip" -- tools/desk/cmd \| grep -v _test.go \| wc -l \| tr -d ' ')" = 0 && echo NO-CMD-CHANGE` | pass exit=0 | sha256:80a91942d970 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |

| # | Verify row | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | row 1 as written (exact command in the witness table above) | ok | exit 0; "ok github.com/medici-finance/assay/tools/desk/internal/hotspot 9.109s" | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 2 | row 2 as written (exact command in the witness table above) | 2 | exit 0; printed 2 ("--- PASS: TestHotspotFixture", "--- PASS: TestCouplingFixture") | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 3 | row 3 as written (exact command in the witness table above) | 5 | exit 0; printed 5. Header: ref=refs/remotes/origin/main tip=d227828fe3ae (last first-parent commit at until) commits=1308 ranked=538. Rank 1: score=173318 churn=38 fixes=15 complexity=4561 forge_gitlab.go | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 4 | row 4 as written (exact command in the witness table above) | MATCH | exit 0; c=38 f=tools/desk/internal/deskkit/forge_gitlab.go s=38; printed MATCH | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 5 | row 5 as written (exact command in the witness table above) | 1 | exit 0; printed 1 ("prefix-decoy fixes=0 churn=2 complexity=1 lines=6 score=2 rank=6 pct=100.0") | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 6 | row 6 as written (exact command in the witness table above) | 1 | exit 0; printed 1. Clone confirmed shallow (is-shallow-repository = true); the test logs "could-not-check (shallow): <clone path>" and passes without ranking; temp clone removed afterwards | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 7 | row 7 as written (exact command in the witness table above) | 1, then >= 4 | exit 0; printed 1 then 10 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 8 | row 8 as written (exact command in the witness table above) | exit=0 | exit 0; "LINT: PASS" then "exit=0" (NOTICE lines only, none about findings) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 8a | row 8a as written (exact command in the witness table above) | output 1, lint rc 1 | exit 0 under pipefail; printed 1. Lint line: "statusgen: parsing findings/2026-09-24-brittle-example.md: no frontmatter: first line must be ---" / "LINT: FAIL 1 problem(s)". Also exit 0 and 1 without pipefail. Mutation check with a stand-in lint that prints the parse line: rc 0 made the row exit 1 ("lint rc=0, want 1"), and rc 2 made it exit 1 ("lint rc=2, want 1"). The status assertion bites both ways | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 9 | row 9 as written (exact command in the witness table above) | NO-CMD-CHANGE | exit 0; impl=b6f8f3c4e86d base=impl~1 tip=impl; printed NO-CMD-CHANGE | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun` on linux, network-off, 10/10 pass at 35496323b8fc. Row 8a now passes. A mutation check with lint rc 0 and rc 2 shows its status assertion bites both ways.

RISK-VALUE: DERIVED — the fix pattern (hotspot.go line 48) is identical to the brief's proxy. A hand check against all 12 fixture subjects shows the word boundaries hold: prefix and fixture do not match.
RISK-VALUE: DERIVED — the coupling filters MinCount 5, MinRatio 0.3 and MaxChangeset 8 (hotspot.go lines 75-81) are the brief's stated filters and are printed in the report header. The live top-3 pairs, 17/16/15, match the brief's authoring sample.
RISK-VALUE: DERIVED — the nomination cut is 2% with fixes of at least 2 (docs/contracts.md line 313). It is the brief's two-key rule and matches the measured concentration.
RISK-VALUE: DERIVED — the confirmation key is at least two counted instances (docs/contracts.md line 318). It is the brief's key 2 verbatim.

Notes:
- The prior row 8a hold is cleared, and #1862's stated cause is resolved at this SHA.
- Rows 3 and 4 take bare-date bounds, so their absolute churn depends on the time of day. Row 4 compares like with like and is unaffected. A baseline should record RFC 3339 bounds.
- Rows 2, 8 and 8a use fixed temp paths, so concurrent verifiers on one host can collide.

VERIFY: PASS

## Review
Gate: model (from frontmatter). The reviewer checks the decoys bite: a counter that counts
`_test.go` files, includes the 40-file merge in coupling, or matches "prefix" as a fix passes
row 1 and fails only rows 2 and 5. The reviewer also confirms the docs section says the metric
nominates and history confirms, in that order, and that nothing in this brief marks a module.
