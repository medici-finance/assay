---
brief: assay:assay:build-less-brittle:03
title: Weight counter and CI ratchet (a Go test, not a verb)
why: >-
  Additions pass review easily and deletions are rare, so the desk tools only grow. Today no
  check notices growth: 63 verbs, hundreds of flags and about 845 refusal-constructor calls
  accumulated one reasonable PR at a time. A small counter, run by the test suite CI already
  executes, makes every increase visible and requires the driver's explicit approval to raise
  a ceiling. It also gives every PR a measured weight line instead of a hand-written one.
wave: 0
depends: []
unblocks: ["build-less-brittle/05", "build-less-brittle/06", "build-less-brittle/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 row 5, §4.5, §5.2"
  - "tools/desk/internal/forgeban/allowlist.go:80-84 (the existing ratchet precedent: allowedInvocationCeiling)"
  - ".github/workflows/ci.yml:59-66 (tools/desk runs `go test ./...` on push and pull_request)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no weight/ratchet package for verbs, flags, refusals or rule text exists; forgeban ratchets only forge-CLI invocations"
exec-tier: strong
exec-tier-why: "(b) the counter's dimension definitions are the shared measuring stick for the review stage (06), the PR Weight report (05) and an adopting project's baseline/close-out; they must be exact and reproducible at old SHAs."
domain: clear
---

# Brief 03 — Weight counter and CI ratchet

## Context

files:
- `tools/desk/internal/weight/weight.go` (planned): NEW. The counter. Pure functions over a directory tree.
- `tools/desk/internal/weight/weight_test.go` (planned): NEW. `TestCeiling` (planned), `TestPrintWeight` (planned),
  `TestCountsFixture` (planned), `TestCeilingRedOnGrowthFixture` (planned), and test-only flags `-root` and `-rev`.
- `tools/desk/internal/weight/ceiling.txt` (planned): NEW. One `<dimension> <N>` line per ratcheted
  dimension, plus `# grow …` annotation lines.
- `tools/desk/internal/weight/testdata/tree/**` (planned): NEW. A fixture tree with known counts.
- `changelog/build-less-brittle-03.md` (planned)

single-point-of-failure: the review stage's check (06) that a `# grow` line cites a comment by the
driver's own login is the ONE control on who may raise a ceiling; this test only checks that a
raised ceiling carries a `# grow` line. Behind it: the driver's own merge of the PR that raises
the ceiling, and the approved number written into the ceiling, so any later growth notices (or,
once promoted, fails) again (spec §4.5, §4.7).

facts:
- **CI runs it for free.** `.github/workflows/ci.yml` runs `go test ./...` in `tools/desk` on
  every push and pull_request (lines 59–66 at f7bde6bfa). No workflow edit is needed. A
  workflow edit would need a scope the worker App does not hold.
- **Precedent:** `forgeban`'s `allowedInvocationCeiling` (`allowlist.go:84`, currently 6) is a
  committed ceiling a test enforces, lowered in the PR that retires sites.
- **Dimensions** (all over non-`_test.go` files):
  - `verbs`: directories directly under `tools/desk/cmd/` containing a `package main` file.
    65 `cmd/` directories exist at f7bde6bfa. The count of main packages is the implementer's
    to record.
  - `flags`: syntactic flag registrations in `tools/desk/cmd/**`. A call whose selector is one
    of `String Bool Int Int64 Uint Uint64 Float64 Duration Func BoolFunc TextVar` with a
    string-literal first argument, or one of the `…Var` forms with a string-literal second
    argument. A grep proxy gave about 345 at f7bde6bfa.
  - `refusals`: calls to `Refused(`, `RefusedWithCause(`, `RefusedFinding(` (any qualifier)
    under `tools/desk/**`. These constructors are defined at
    `internal/deskkit/exitcodes.go:144,157,164`. A grep proxy gave about 845 at f7bde6bfa.
    This deliberately differs from the 5,485 raw occurrences of the word "refus", which are
    not sites.
  - `ruletext`: total lines of `plugins/assay/skills/{the-desk,intake-desk,worker-desk,
    pr-review-desk,verify-desk,author-brief,pr-shepherd}/SKILL.md` and
    `tools/desk/cmd/deskdispatch/references/*.md`.
  - `golines` (REPORTED, never ratcheted): non-blank lines of non-test `.go` under `tools/desk`.
- **Three-state.** When the plugin directory is absent (a consumer checkout of `tools/desk`
  alone), `ruletext` is `could-not-check` and `TestCeiling` (planned) skips with that reason. It never
  passes silently.
- **Growth approval** (spec §4.5). A ceiling may rise only with a line
  `# grow <dimension> +<n> <url-of-the-driver's-reply>` directly above it. The reply is the
  driver's `grow <PR#>` on the project's standing weight-growth decision issue. This test does
  **not** verify the URL's author (that is review stage 06). It checks only that every raised
  ceiling carries a `# grow` line. Raised means above the value in `ceiling.txt` at the
  `-base` revision when one is given; otherwise the check is skipped and says so.
- **Mode (D-A, ratified by the driver on #1660; spec §10).** `ceiling.txt` carries a first line `# mode: advisory`
  at landing. In advisory mode `TestCeiling` (planned) logs `GROWTH-NOTICE <dim>: <count> >
  ceiling <n> (+<d>)` and passes; in `# mode: blocking` it fails with the message in Task
  step 4. The promotion to `blocking` is a later, recorded decision keyed to the adopting
  project's baseline measurements (spec §5.4), no earlier than one month after
  go-live, landed as a one-line edit of that mode line citing the decision's URL. The
  test-only flag `-mode` overrides the file, so the mutation row proves the blocking path
  before it is switched on. The review stage (06) reads the notice either way.
- **layering:** flat tool. A single package with pure counting functions over an `fs.FS`, and
  the git read (`git archive <rev>` into a temp dir) confined to the test file's `-rev` path.

design-fit:
  owner: tools/desk/internal/weight (new; no existing owner measures weight)
  contract: none — new; build-less-brittle/07 registers the ceiling as rule R-weight-ceiling
  retires: []
  weight: verbs 0 (internal package), flags 0 (test-only flags live in _test.go), refusals 0, rule-text 0
  why-add: n/a (no ratcheted growth); the added Go is ~200 lines of pure counting, reported under golines

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No new verb and no new flag on any shipped binary. This is a test package.
- The initial `ceiling.txt` equals the counter's output at your branch head. Record those numbers
  and the head SHA in the PR body.

## Task

1. Write `weight.go`: `Count(fsys fs.FS) (Weight, error)` returning the five dimensions and a
   per-dimension could-not-check reason. Use `go/parser` for `flags` and `refusals`, never
   regexes over source text.
2. Write the fixture tree with hand-known counts. Suggested: 2 verbs, 3 flags (one of them a
   `…Var` form), 4 refusals (one of each constructor, one qualified), 10 rule-text lines, and
   one decoy per dimension: a `_test.go` refusal, a `package lib` dir under `cmd/`, a flag
   registration with a non-literal name. `TestCountsFixture` (planned) asserts the exact counts.
3. `TestCeilingRedOnGrowthFixture` (planned): a fixture ceiling one below the fixture count must produce
   a failure report naming the dimension and the delta. This is the negative control.
4. `TestCeiling` (planned) reads the real tree (the repository root, three levels up) against
   `ceiling.txt`. In blocking mode it fails on growth with the message "<dim>: <count> >
   ceiling <n> (+<d>). Reduce, or get the driver's `grow <PR#>` reply and raise the ceiling
   with a `# grow` line citing it." In advisory mode (the landing state) it logs the same
   text prefixed `GROWTH-NOTICE` and passes. It logs `slack <dim>=<n>` when below.
5. `TestPrintWeight` (planned) logs exactly one line,
   `weight: verbs=<n> flags=<n> refusals=<n> ruletext=<n|could-not-check> golines=<n>`, for
   `-root <dir>` (default: the repo) or `-rev <sha>` (a `git archive` of that revision into a
   temp dir). This is the line PR bodies (05), reviewers (06) and a project's baseline quote.
6. Write `ceiling.txt` from step 5's output at your head, and the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Row 3 is the mutation row required for a
brief that adds a check. It plants **50** refusals, more than any plausible slack between
monthly tightenings.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go test ./internal/weight/ -count=1` | `ok` |
| 2 | check | `cd tools/desk && go test ./internal/weight/ -run TestCountsFixture -count=1 -v` | `--- PASS: TestCountsFixture`; the log names `verbs=2 flags=3 refusals=4 ruletext=10` |
| 3 | check +mutation | `cd tools/desk && awk 'BEGIN{print "package main"; print "import \"github.com/medici-finance/assay/tools/desk/internal/deskkit\""; for(i=0;i<50;i++) print "var _ = deskkit.Refused(\"weight mutation\")"}' > cmd/deskfile/zz_weight_mutation.go && go test ./internal/weight/ -run TestCeiling -count=1 -args -mode=blocking > /tmp/bl03-mut.out 2>&1; rc=$?; rm -f cmd/deskfile/zz_weight_mutation.go; test $rc -ne 0 && grep -c 'refusals: .* > ceiling' /tmp/bl03-mut.out` | `1` (red on planted growth in blocking mode, and it names the dimension) |
| 3a | check | `cd tools/desk && awk 'BEGIN{print "package main"; print "import \"github.com/medici-finance/assay/tools/desk/internal/deskkit\""; for(i=0;i<50;i++) print "var _ = deskkit.Refused(\"weight mutation\")"}' > cmd/deskfile/zz_weight_mutation.go && go test ./internal/weight/ -run TestCeiling -count=1 -v > /tmp/bl03-adv.out 2>&1; rc=$?; rm -f cmd/deskfile/zz_weight_mutation.go; test $rc -eq 0 && grep -c 'GROWTH-NOTICE refusals:' /tmp/bl03-adv.out` | `1` (D-A: in the landing state the same growth is a notice and the test passes) |
| 3b | check | `head -1 tools/desk/internal/weight/ceiling.txt` | `# mode: advisory` (the landing state; the promotion is a later recorded decision) |
| 4 | check | `cd tools/desk && go test ./internal/weight/ -run TestPrintWeight -count=1 -v \| grep -cE 'weight: verbs=[0-9]+ flags=[0-9]+ refusals=[0-9]+ ruletext=[0-9]+ golines=[0-9]+$'` | `1` |
| 5 | check | `cd tools/desk && go test ./internal/weight/ -run TestPrintWeight -count=1 -v -args -rev=f7bde6bfa \| grep -cE 'weight: verbs=[0-9]+ flags=[0-9]+ refusals=[0-9]+ ruletext=[0-9]+ golines=[0-9]+$'` | `1` (the counter re-applies to a revision that predates it, which a project's baseline and close-out need) |
| 6 | check | `cd tools/desk && go test ./internal/weight/ -run TestPrintWeight -count=1 -v -args -root="$(mktemp -d)" \| grep -c 'ruletext=could-not-check'` | `1` (three-state: an absent plugin tree is could-not-check, never 0) |
| 7 | check | `v=$(for d in tools/desk/cmd/*/; do grep -lq '^package main' "$d"*.go 2>/dev/null && echo x; done \| wc -l \| tr -d ' '); cd tools/desk && go test ./internal/weight/ -run TestPrintWeight -count=1 -v \| grep -c "weight: verbs=$v "` | `1` (dereference: the counter's verbs figure equals an independent shell count over the same tree) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

Implemented on branch `feat/build-less-brittle-03` at head `d1c130ce5f0e627381d3793743bcf871ac226825`.
Deliverables: `tools/desk/internal/weight/weight.go` (new — `Count`, `Weight`, `Ceiling`/`ParseCeiling`,
`Evaluate`/`DimensionResult`, `GrowthMessage`/`SlackMessage`/`PrintWeight`, `GrowthAnnotationAbove`),
`tools/desk/internal/weight/weight_test.go` (new — `TestCountsFixture`, `TestCeilingRedOnGrowthFixture`,
`TestCeiling`, `TestPrintWeight`, plus `TestGrowthAnnotationAbove` covering the "# grow" presence
check the facts describe for a `-base` comparison; test-only flags `-root`/`-rev`/`-mode`/`-base`),
`tools/desk/internal/weight/ceiling.txt` (new, `# mode: advisory`, landing values below),
`tools/desk/internal/weight/testdata/tree/**` (new fixture: 2 verbs, 3 flags — one `…Var` form —,
4 refusals — one per constructor, one qualified —, 10 rule-text lines, plus the three named decoys),
`changelog/build-less-brittle-03.md` (new).

**Landing values** (the counter's own output at this head): `weight: verbs=66 flags=368 refusals=841
ruletext=6183 golines=156019` — written into `ceiling.txt` verbatim for the four ratcheted dimensions
(golines is reported only). Cross-checked against the brief's own f7bde6bfa facts: re-running
`TestPrintWeight -args -rev=f7bde6bfa` reports `verbs=65`, matching "65 `cmd/` directories exist at
f7bde6bfa" exactly.

**Fail-first.** This brief adds the check itself, so there is no prior red/green pair to restore; the
required fail-first evidence is Verify row 3 below: with `ceiling.txt` unmodified and 50 extra
`Refused(` calls planted, `TestCeiling -mode=blocking` exits non-zero and names `refusals: 891 >
ceiling 841 (+50)`. Row 3a is the same mutation in the landing (advisory) mode, which logs the
identical text prefixed `GROWTH-NOTICE` and exits zero — proving the D-A mode switch actually gates
the outcome rather than being cosmetic.

**Scope note.** The facts section's Growth-approval paragraph describes a `-base`-revision comparison
that requires a "# grow <dim> +<n> <url>" line above any dimension raised since that revision. Task
step 4 does not name a `-base` flag and no Verify row exercises it, so it is not part of this brief's
completeness bar — but the SPOF note asserts it as this test's behavior, so `TestCeiling` implements
it (skipped, and says so, when `-base` is omitted — true for every row below) and `weight.go` exposes
the pure presence-check (`GrowthAnnotationAbove`) with its own unit test rather than leaving the
SPOF note's claim unimplemented.

**Runner-cell re-render (review finding F2, 2026-09-25).** The table below replaces a
hand-typed one that named `assay-worker-app[bot]` with no tree SHA and no on-behalf-of
principal — `statusgen verifyrun --check` confirmed all 9 rows read as
could-not-run against it (no execution witness matched). It is superseded by a table
written through `statusgen verifyrun --brief`, the actual write path, immediately below.

## Review
Gate: model (from frontmatter). The reviewer checks the fixture decoys actually exercise each
exclusion. A counter that counts `_test.go` refusals would pass rows 1 and 4 and fail only
row 2.
