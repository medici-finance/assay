---
brief: assay:assay:build-less-brittle:06
title: "Design-fit review stage: right layer, should it exist, what does it replace — before correctness; advisory at landing, blocking by a later recorded decision"
why: >-
  Review today asks whether a diff is correct, not whether it belongs where it was put or
  should exist at all. A correct patch in the wrong layer passes, and the next symptom gets the
  next patch. For any PR that adds weight (a verb, a flag, a refusal, rule text) or a new rule,
  the reviewer, at strong tier, first answers three design questions. A "wrong layer" answer
  is a `design-fit` finding: advisory at landing, blocking only after a later recorded
  promotion decision (D-A). The existing reversal-rate calibration then demotes the finding
  class if reviewers turn out to be imposing taste.
wave: 2
depends: ["build-less-brittle/02", "build-less-brittle/03", "build-less-brittle/07"]
unblocks: ["build-less-brittle/11"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §2 D2, §3 row 4, §4.4, §4.5, §4.7"
  - "tools/desk/internal/deskkit/reviewscope.go (ScopeBases: the four blocking bases, pinned to the kit block)"
  - "tools/desk/cmd/deskdispatch/reviewscope_test.go:240-268 (TestReviewScopeKitMatchesModel)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md (risk-keyed tiering; finding-class register; reversal-rate demotion; round cap)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no design-fit basis, clause or finding class; review tier is keyed to risk and author trust only, never to weight"
exec-tier: strong
exec-tier-why: "(b) a basis added to the model must agree with the kit block, the kit's clause and the skill's finding-class register, under two tests; (a) the three questions' blocking bar is judgement the reviewer must be told precisely."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/reviewscope.go ScopeBases(): fixed-here (BasisDesignFit and its ScopeBases row)"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md reviewscope block + new clause: fixed-here (block row + clause 3; clauses 3-15 renumbered 4-16 and every cross-reference updated)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md tiering + finding-class register: fixed-here (weight-keyed tier, design-fit | advisory row, growth-approval step)"
  - "docs/contracts.md rule register (R-design-fit-basis row): fixed-here"
  - "the review-scope case corpus scored against ScopeBases(), if present: fixed-here (docs/streams/desk-supervision/review-scope-cases.md names five bases)"
  - "installed deskdispatch binaries (kits are embedded): out-of-scope (reach consumers on the next desk-tools release and pin bump)"
---

# Brief 06 — The design-fit review stage

## Context

files:
- `tools/desk/internal/deskkit/reviewscope.go`: a fifth basis, `BasisDesignFit ScopeBasis = "design-fit"`, and its doc row in `ScopeBases()`.
- `tools/desk/cmd/deskdispatch/reviewscope_test.go` (the model/kit pin and the case tests; update any assertion of exactly four bases).
- `tools/desk/cmd/deskdispatch/references/review-prompt.md`: the `reviewscope` block row, and a clause "Design fit first".
- `plugins/assay/skills/pr-review-desk/SKILL.md`: weight-keyed tier; the finding-class register row; the growth-approval step.
- `docs/contracts.md`: rule row `R-design-fit-basis`.
- `changelog/build-less-brittle-06.md` (planned)

single-point-of-failure: the reviewer's check that a `# grow` line's URL resolves to a comment by
the driver's own login is the ONE control on ratchet growth. Behind it: the driver's own merge of
the growth PR, and the CI ceiling, which forces every bump into the diff where review sees it
(spec §4.7).

facts:
- `ScopeBases()` (reviewscope.go:83) is "the single source of truth the kit block and the case
  corpus are checked against". `TestReviewScopeKitMatchesModel` fails if the kit's
  `<!-- reviewscope:begin -->` block and the model differ. A basis added in one place only is
  red.
- **Trigger** (no diff classifier). The reviewer runs the weight counter (brief 03) at the
  merge-base and at the head:
  `cd tools/desk && go test ./internal/weight/ -run TestPrintWeight -count=1 -v -args -rev=<sha>`.
  The stage runs when any ratcheted dimension grows, or when the diff adds a `R-` row. The PR
  body's `## Weight` (brief 05) is a claim to check against this, not the trigger.
- **Questions:** (1) right layer: does the change live in the owner the semantic index names?
  (2) should it exist: could removal fix the symptom instead? (3) what does it replace: are
  `retires:`/`why-add:` true and sufficient? A "no" is a finding with basis `design-fit` and a
  concrete reason naming an `S-` row, a `R-` row or the counter delta. Once the class is
  `blocking` (see Calibration), that finding holds the PR and the reviewer stops before the
  correctness pass; while `advisory` it is recorded and the pass continues.
- **Ownership versus enforcement.** A second independent enforcement point at a different trust
  boundary is NOT a design-fit finding (spec §4.2 rule 1). Only a second owner of a meaning is.
- **Tier.** The pr-review-desk skill keys tier to risk today (strong for risk-flagged items).
  This adds: weight growth → strong tier for the correctness lane.
- **Calibration, and the landing state (D-A, ratified by the driver on #1660; spec §10).** The finding-class register
  table gains `design-fit | advisory`. While advisory, the reviewer records the finding with
  its basis and reason and proceeds to the correctness pass; the ready-flip is not held on it.
  The promotion to `blocking` is a later, recorded decision keyed to the project's baseline
  measurements, no earlier than one month after go-live, landed as the one table-cell edit
  the register already provides for. Once blocking, the existing rule demotes the class back
  to advisory at a reversal rate above 50% for two consecutive months. That is the
  self-correcting layer, and nothing new is built for it.
- **Growth approval** (spec §4.5). When the stage accepts a growth as justified, the reviewer
  attaches `PR #<N> head <sha>: <dimension> +<n> — <why-add>` to the project's standing
  weight-growth decision issue, using `deskfile attach`. The driver replies `grow <N>`. The
  reviewer checks that the `# grow` line's URL resolves to a comment by the driver's login
  before approving. The project layer names the issue and the login.
- Line counts at f7bde6bfa (for scale; the net ≤ 0 rows derive their own base): review-prompt.md 324, pr-review-desk 968. Both net ≤ 0.
- **Hotspot and fitness-function wiring (build-less-brittle/08, /10; SOTA amendment).** Two more
  triggers, one line each in the clause: (a) the diff touches a module carrying a `brittle`
  mark (`docs/contracts.md` §Brittle marks) — the stage runs even at zero weight delta, and
  the reviewer reads the module's investigation (09) if one exists, so question 1 is answered
  against a recorded intent rather than the reviewer's guess; (b) a red `internal/arch` test
  is a `design-fit` finding by construction, because each of its rules names the register row
  it enforces. Google's reviewer guide puts design first and asks whether the change belongs
  in the codebase at all; this stage is that question with a table to hold it to.
- **Why the risk answers stay `no`** although `reviewscope.go` sits on a security-path trigger:
  the change only ADDS a basis on which a reviewer may block. It alters no predicate that
  admits, authorises or writes (the file's own header says so of the whole model), and it
  weakens no existing basis or lane. The path trigger still routes the PR to the security lane.

design-fit:
  owner: tools/desk/internal/deskkit/reviewscope.go (the blocking-basis model)
  contract: S-review-verdict
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 (review-prompt.md and pr-review-desk)
  why-add: one enum value and one doc row in the model; no new refusal or verb

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The security lane, the round cap and the fail-first clause are unchanged.
- Kit and skill edits are net ≤ 0 lines.

## Task

1. Add `BasisDesignFit` and its doc row: "the change adds weight or a rule and fails a design-fit
   question: wrong owner, avoidable by removal, or an untrue/insufficient retires/why-add".
   Update tests that count bases.
2. In `review-prompt.md`: add the row to the `reviewscope` block, and a clause "Design fit
   first" (≤ 14 lines) after clause 2 (CI first), carrying the trigger, the three questions,
   the ownership-versus-enforcement boundary, and the landing state: advisory (record the
   finding and continue), with stop-on-block only once the class is promoted to `blocking`.
3. In `plugins/assay/skills/pr-review-desk/SKILL.md`: weight growth → strong tier (1–2 lines beside the risk-keyed
   rule); the register row `design-fit | advisory` (D-A; the promotion to `blocking` is a
   later one-cell edit); the growth-approval step (≤ 5 lines).
4. Add `R-design-fit-basis` to the rule register (serves `S-review-verdict`).
5. Offset lines, run the tests, and write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Row 3 is the mutation row for the kit/model
pin. Rows 7–8 are net ≤ 0 weight rows. Rows 3, 7, 9 and 10 were re-authored in version 2 (#1977)
so they can be decided on merged main, under any shell, without leaving the checkout: row 3 keeps
its scratch files in the checkout root (both gitignored), row 7 braces its revision variables, row 9
prints one count, and row 10 corroborates the `consumers:` claims against the delivering change
itself, pinned so it still has that diff to read after merge.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | `ok` for both |
| 2 | check | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' tools/desk/internal/deskkit/reviewscope.go` | `1` |
| 3 | check +mutation | `cd tools/desk && f=cmd/deskdispatch/references/review-prompt.md && cp "$f" ../../bl06-kit.bak && awk '!/^[\|] design-fit [\|]/' ../../bl06-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestReviewScopeKitMatchesModel -count=1 > ../../bl06-mut.out 2>&1; rc=$?; cp ../../bl06-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | `RED-ON-DRIFT` (dropping the kit row while the model keeps the basis fails the pin; the backup and the mutant's test log stay in the checkout root, where `*.bak` and `*.out` are gitignored, and the log is left there to read) |
| 4 | check | `grep -cE '^## [0-9]+\. Design fit first' tools/desk/cmd/deskdispatch/references/review-prompt.md` | `1` |
| 5 | check | `grep -cE '^[\|] design-fit [\|] advisory [\|]' plugins/assay/skills/pr-review-desk/SKILL.md` | `1` (D-A: the class lands `advisory`; the promotion to `blocking` is a later recorded decision, a one-cell edit) |
| 6 | check | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | `1` (the new rule is registered and serves an existing S- row) |
| 7 | check | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "${tip}:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "${base}:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | `NET-OK` (the revision variables are braced: unbraced, zsh reads `:t` as a path modifier, both reads fail, and 0 ≤ 0 would pass) |
| 8 | check | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 9 | check | `grep -q '^func TestPrintWeight' tools/desk/internal/weight/weight_test.go && grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/review-prompt.md` | ≥ `1` (one line: how often the kit's clause names the counter command, printed only after the counter test that command runs is found; without that test the row prints nothing and fails) |
| 10 | check | `d=$(mktemp -d "$PWD/.bl06-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach d1258928da48 && statusgen --consumers --root "$d" --brief build-less-brittle/06 --base d1258928da48~1; s=$?; rm -rf "$d"; exit $s` | exit 0; output is `summary: 4 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing` (the check runs in a throwaway shared clone, made inside the checkout and removed afterwards, checked out at d1258928da48, the squash that delivered this brief in #1879, with the base pinned to its parent, so the diff it reads is exactly the delivering change: never main's later commits, never the runner's own working tree. The four `fixed-here` path entries are corroborated by that diff. The two UNCHECKED entries stay the reviewer's call per brief-rule 9, never a pass: the case-corpus entry names its file in prose, and the installed-binaries entry is `out-of-scope`) |
| 11 | check | `s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'Brittle marks'; echo "$s" \| grep -c 'internal/arch'` | two counts, each ≥ `1` (a marked module and a red arch test both trigger the stage) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
### Verify pass — 2026-10-01, non-implementer verifier, merged main ca81ea0a9603 (contains the #1879 squash d1258928da48)

What moved since the last run: nothing to compare against. This is the first verify pass, and the brief's Evidence table was empty at ca81ea0a9603. All rows ran from the repository root of a clean detached worktree at refs/remotes/origin/main = ca81ea0a960387bb34f9dbc22b188802097d5251. The only commit after the implementation squash is a STATUS.md regeneration.

Grounded expectation, written before running anything: the squash d1258928da48 adds the basis, the kit row, clause 3, the register row and the R- row, so rows 1–9 and 11 should meet Expect. Row 10's Expect is written "at the PR head", so on merged main its diff against HEAD is empty, and the as-authored form is expected to come back could-not-check.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 (Verify row 1) | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | `ok` for both | exit 0. `ok …/tools/desk/internal/deskkit 66.374s` and `ok …/tools/desk/cmd/deskdispatch 12.743s` | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 1b (Verify row 1, skip audit) | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1 -v 2>&1 \| grep -e '^ok' -e '^FAIL' -e '--- FAIL' -e '--- SKIP' -e '--- PASS: TestReviewScope' -e '--- PASS: TestDesignFit'` | no FAIL, and no SKIP among the brief's own tests | Both packages `ok`, 0 `--- FAIL`. The brief's own tests all report `--- PASS`: the four TestReviewScope* tests (including the kit-model pin and all four RequiredAndUnrelated subtests) and TestDesignFitIsAScopeBasis. There are 18 `--- SKIP`, and each one is fixture- or env-gated outside this brief: a fixture absent from the published file set (`.github/`, `.claude/skills`, go.work, the leak-sweep token map), live census needing network, the released-binary smoke with no pinned binary path, and the withheld-token half of the kit leak guard with no token list. None of the skipped tests touches reviewscope or design-fit. | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 (Verify row 2) | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' tools/desk/internal/deskkit/reviewscope.go` | `1` | exit 0, output `1` (the line is reviewscope.go:74) | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 (Verify row 3, +mutation) | `cd tools/desk && f=cmd/deskdispatch/references/review-prompt.md && cp "$f" ../../bl06-kit.bak && awk '!/^[\|] design-fit [\|]/' ../../bl06-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run 'TestReviewScopeKit.*Model' -count=1 -v > ../../bl06-mut.out 2>&1; rc=$?; cp ../../bl06-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | `RED-ON-DRIFT` | exit 0, prints `RED-ON-DRIFT`, test_rc=1. The mutant run failed with `reviewscope_test.go:299: the model names basis "design-fit" but the review kit's block does not`. The awk dropped exactly 1 of the kit's 461 lines, and `git diff --quiet -- tools/` confirmed the kit was restored afterwards. Correction note: the as-authored command keeps its backup and output files in the system temp directory, outside the verifier's worktree. The isolation clause kept this hand run inside the worktree, so the two backup paths are moved to gitignored repo-root files and the -run name is a regex. The test, the mutation and the assertion are unchanged. The as-authored text itself was run by the execution witness below (row 3 pass, exit 0). | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 (Verify row 4) | `grep -cE '^## [0-9]+\. Design fit first' tools/desk/cmd/deskdispatch/references/review-prompt.md` | `1` | exit 0, output `1` (the heading is review-prompt.md:36, `## 3. Design fit first — before correctness, when a PR adds weight or a rule`) | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 (Verify row 5) | `grep -cE '^[\|] design-fit [\|] advisory [\|]' plugins/assay/skills/pr-review-desk/SKILL.md` | `1` | exit 0, output `1` (the row is SKILL.md:554, `\| design-fit \| advisory \|`) | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 (Verify row 6) | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | `1` | exit 0, output `1` (the row is contracts.md:246) | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 (Verify row 7) | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | `NET-OK` | exit 0, prints `NET-OK`. Run under bash (from a script file holding exactly this text) with impl=d1258928da48 and base=impl~1: review-prompt.md is 461 lines at the tip and 461 at the base, so net 0. Shell note: under zsh this exact text prints a FALSE `NET-OK`, because zsh reads `"$tip:t…"` as the `:t` history modifier. Both `git show` calls then fail, both counts read 0, and 0 ≤ 0 passes. That is why the bash run is the one recorded. | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 (Verify row 8) | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` | exit 0, prints `NET-OK`. Run under bash: SKILL.md is 1023 lines at the tip and 1023 at the base, so net 0. | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 (Verify row 9) | `grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/review-prompt.md && test -f tools/desk/internal/weight/weight_test.go && echo COUNTER-EXISTS` | a count ≥ `1`, then `COUNTER-EXISTS` | exit 0, outputs `1` and then `COUNTER-EXISTS` | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 (Verify row 10, as authored) | `statusgen --consumers --root "$(git rev-parse --show-toplevel)" --brief build-less-brittle/06; echo "exit=$?"` | `exit=0` at the PR head | COULD-NOT-CHECK: prints `exit=2`. statusgen v1.0.29 reports `--consumers: COULD-NOT-CHECK: assay:assay:build-less-brittle:06 is not in the diff against ca81ea0a9603…`. On merged main the diff against HEAD is empty, so the as-authored form carries no evidence. The command ran from a clean tree before the witness wrote back. The only change from the as-authored text is the root: the desk requires an absolute `--root`, so the worktree's absolute root was passed and is written here in the equivalent repo-relative form. | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10b (Verify row 10, corrected per statusgen's own re-run instruction) | `statusgen --consumers --root "$(git rev-parse --show-toplevel)" --brief build-less-brittle/06 --base d1258928da48~1; echo "exit=$?"` | `exit=0`, 0 disproved | exit 0. Base 2adf73c2a791 (the squash's parent). The tool reports `summary: 4 corroborated, 0 disproved, 2 unchecked`. The 4 CORROBORATED claims are reviewscope.go, review-prompt.md, pr-review-desk SKILL.md and contracts.md. The 2 UNCHECKED: the review-scope case corpus (a prose name, so it was hand-checked: review-scope-cases.md lines 17–18 name five bases ending `design-fit`) and the installed binaries (out-of-scope, and this branch made no claim about them). The result was the same when re-run after the witness modified the brief. | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 (Verify row 11) | `s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'Brittle marks'; echo "$s" \| grep -c 'internal/arch'` | two counts, each ≥ `1` | exit 0, outputs `1` and `1` | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Scope traceability: every hand row above names the Verify row it discharges. Two deliverables have no Verify row, so they were checked by reading the files. (a) Task 2's "≤ 14 lines" bound on the clause: clause 3 runs from review-prompt.md:36 to :49, 14 lines, so it is met. (b) Task 3's "≤ 5 lines" bound on the growth-approval step: SKILL.md:565–568, 4 lines, so it is met. Both are findings in the sense of addendum item 8, because each is a Deliverables bound with no executable row.

#### Execution witness (`statusgen verifyrun --root <worktree> --brief docs/streams/build-less-brittle/brief-06-design-fit-review-stage.md`, writing form, statusgen v1.0.29, clean tree, exit 2). Emitted verbatim:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | pass exit=0 | sha256:3721da8cb7dd | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' tools/desk/internal/deskkit/reviewscope.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && f=cmd/deskdispatch/references/review-prompt.md && cp "$f" /tmp/bl06-kit.bak && awk '!/^[\|] design-fit [\|]/' /tmp/bl06-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestReviewScopeKitMatchesModel -count=1 > /tmp/bl06-mut.out 2>&1; rc=$?; cp /tmp/bl06-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | pass exit=0 | sha256:cd183bfd8e84 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE '^## [0-9]+\. Design fit first' tools/desk/cmd/deskdispatch/references/review-prompt.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -cE '^[\|] design-fit [\|] advisory [\|]' plugins/assay/skills/pr-review-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/review-prompt.md && test -f tools/desk/internal/weight/weight_test.go && echo COUNTER-EXISTS` | could-not-run exit=0 — Expect requires a count ≥ 1 but the output carries no number to read | sha256:bd8115622356 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/06; echo "exit=$?"` | fail exit=0 | sha256:2cc011a2a512 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |
| 11 | `s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'Brittle marks'; echo "$s" \| grep -c 'internal/arch'` | pass exit=0 | sha256:ad0fadf63cc7 | 2026-10-01 | assay-verifier-app[bot] @ ca81ea0a9603 (on-behalf-of human:ian) (forge-identity) |

Reading the witness (the tool's own lines on stderr):
- Row 9 is `could-not-run — Expect requires a count ≥ 1 but the output carries no number to read`. The row prints two lines (`1`, then `COUNTER-EXISTS`), and the witness's Expect parser does not read a count from it. The hand run (row 9 above) shows the count is `1`.
- Row 10 is `fail — exit 0, expected 1`. The witness took the Expect phrase "Exit 1 names the disproved claim" as the expected exit code. The row's own `; echo "exit=$?"` always exits 0. On merged main the underlying statusgen call prints `exit=2`, which is COULD-NOT-CHECK, not a disproved claim (hand row 10). With the base the tool itself prescribes, the result is 0 disproved (hand row 10b).

Findings:
1. **Verify row 10 cannot produce evidence on merged main as authored.** The diff against HEAD is empty, so the call returns COULD-NOT-CHECK (exit 2), and `; echo` masks that from any exit-status reader. A post-merge form needs `--base <squash>~1`. This is a Verify-table authoring defect.
2. **Two Verify rows are not machine-witnessable.** In row 9 the count is followed by a second output line, so the witness reports could-not-run. In row 10 the Expect wording is parsed as "expected exit 1", so the witness reports fail. Until rows 9 and 10 are re-authored, the execution witness cannot come back green for this brief.
3. **Verify row 7 gives a false green under zsh.** `"$tip:tools/…"` triggers zsh's `:t` modifier, both line counts read 0, and the row prints `NET-OK` no matter what. It is correct under bash and sh, and the witness runs it in a subshell. `"${tip}:tools/…"` would make the text shell-portable.
4. **Verify row 3 as authored writes outside the checkout.** Its backup and output files go to the system temp directory. When the witness ran the as-authored text, it left those two files behind.
5. Scope traceability: Task 2's ≤ 14-line clause bound and Task 3's ≤ 5-line growth step map to no Verify row. Both were hand-checked and met (14 lines and 4 lines; see above).

Risk-bearing value. The trigger fires because the diff touches a security-path file (reviewscope.go), even though every risk answer is `no` and irreversible is `no`.

Step 1, enumeration. This covers every literal the squash d1258928da48 introduces or changes in its tools/, plugins/ and contracts.md surfaces, plus those named in the brief's Deliverables:
- `BasisDesignFit = "design-fit"` @ tools/desk/internal/deskkit/reviewscope.go:74
- kit block key `design-fit` @ tools/desk/cmd/deskdispatch/references/review-prompt.md:277
- register status `design-fit = advisory` @ plugins/assay/skills/pr-review-desk/SKILL.md:554
- tier binding "Weight growth … is strong tier too" @ plugins/assay/skills/pr-review-desk/SKILL.md:345
- clause-3 length bound `≤ 14` lines (Deliverable; observed 14, review-prompt.md:36–49)
- growth-step bound `≤ 5` lines (Deliverable; observed 4, SKILL.md:565–568)
- net weight bound `≤ 0` lines (kit 461→461, skill 1023→1023)
- demotion threshold `> 50%` for `two consecutive months` @ plugins/assay/skills/pr-review-desk/SKILL.md:556. This one is named in Deliverables but unchanged by the diff (it was at base line 558 and was only re-wrapped).
- promotion window `go-live + one month` @ docs/streams/build-less-brittle/spec.md:241

One candidate was dropped because it is not a literal in this repo: the growth-approval authority binding at SKILL.md:567–568 ("the `# grow` line's URL resolves to a comment by the driver's own login"). The login and the decision issue are held by the project layer by design ("The project layer names the issue and the login"), so no literal can be quoted here.

Step 3, ranking by irreversibility. Every entry above is reversible by an edit and a release. The kits are embedded in the deskdispatch binary, so a fix reaches consumers on the next desk-tools release and pin bump. The two entries that would do the most harm if wrong rank first: the register status (blocking-vs-advisory decides whether a PR is held) and the basis identifier (whether the kit, the model and the register agree). The line bounds, the tier binding, the demotion threshold and the promotion window are reversible operational knobs.

RISK-VALUE: DERIVED — design-fit register status = advisory @ plugins/assay/skills/pr-review-desk/SKILL.md:554 — D-A, ratified on #1660 and recorded at spec.md:241, lands a new finding class without power to hold a PR. Until a month of baseline reversal data exists, the only correction mechanism (the > 50% / two-month demotion rule at SKILL.md:556) has nothing to act on. A class that landed `blocking` could therefore hold PRs on reviewer taste with no self-correction. `advisory` is the fail-soft state, and promotion is the one-cell edit the register already provides.
RISK-VALUE: DERIVED — BasisDesignFit = "design-fit" @ tools/desk/internal/deskkit/reviewscope.go:74 — the value's only correctness criterion is being the same identifier on all three surfaces a reviewer is held to. Read directly, it is the same string as the kit block key (review-prompt.md:277), the register class name (SKILL.md:554), the contracts.md:246 R-row basis name, and the stage name in spec §4.4 and the brief.

rows_passed=10 rows_total=11. Verify rows 1–9 and 11 meet Expect as authored. Verify row 10 is could-not-check as authored on merged main, and its corrected form (10b) passes with 0 disproved. The execution witness reports 9 pass, 1 could-not-run (row 9) and 1 fail (row 10, an Expect misparse).

Verify-row re-authoring is filed as #1977.

VERIFY: BLOCKED. All implementation checks meet Expect, so the deliverable itself checks out. The block comes from Verify-table authoring: row 10 is could-not-check on merged main as authored, and rows 9 and 10 cannot be machine-witnessed. Re-authoring those two rows (findings 1–2) and re-running the witness clears it.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | pass exit=0 | sha256:b054b76ba812 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' tools/desk/internal/deskkit/reviewscope.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && f=cmd/deskdispatch/references/review-prompt.md && cp "$f" ../../bl06-kit.bak && awk '!/^[\|] design-fit [\|]/' ../../bl06-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestReviewScopeKitMatchesModel -count=1 > ../../bl06-mut.out 2>&1; rc=$?; cp ../../bl06-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | pass exit=0 | sha256:cd183bfd8e84 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE '^## [0-9]+\. Design fit first' tools/desk/cmd/deskdispatch/references/review-prompt.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -cE '^[\|] design-fit [\|] advisory [\|]' plugins/assay/skills/pr-review-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "${tip}:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "${base}:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -q '^func TestPrintWeight' tools/desk/internal/weight/weight_test.go && grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/review-prompt.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 10 | `d=$(mktemp -d "$PWD/.bl06-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach d1258928da48 && statusgen --consumers --root "$d" --brief build-less-brittle/06 --base d1258928da48~1; s=$?; rm -rf "$d"; exit $s` | pass exit=0 | sha256:85136008240b | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 11 | `s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'Brittle marks'; echo "$s" \| grep -c 'internal/arch'` | pass exit=0 | sha256:ad0fadf63cc7 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |

### Verify pass — 2026-10-02, non-implementer verifier, merged main 2a6460e85c78 (re-witness of the Verify table as amended by #2030)

- Target SHA: 2a6460e85c78 (detached worktree at the merged main head; contains the #1879 squash d1258928da48 that delivered this brief, and the #2030 amendment of Verify rows 3, 7, 9 and 10).
- Runner: assay-verifier-app[bot] (on-behalf-of human:ian). Not the implementer of this brief or its code.
- Host: darwin, statusgen v1.0.30. Every Verify row is class `check` (row 3 `check +mutation`); the table has no `check:ci` row, so no row needed a Linux runner or a network-off sandbox, and every row ran on this host.
- Brief frontmatter: gate model; risk regulatory no, customer no, irreversible no, sensitive-data no.

What moved since the 2026-10-01 pass at ca81ea0a9603: Verify rows 3, 7, 9 and 10 were re-authored (#1977, landed in #2030). Row 3 now keeps its scratch files in the checkout root, row 7 braces its revision variables, row 9 prints one count, row 10 pins the consumers check to the delivering squash. The four delivered files are unchanged since the squash except the pr-review-desk skill, which later commits grew by 4 lines (1023 at the squash, 1027 at this head); the design-fit register row moved from line 554 to 558.

#### Execution witness

`statusgen verifyrun --brief <this brief> --timeout 10m`, run from the worktree root, first with `--dry-run` and then in the writing form. Both runs: tool exit 0, 11 of 11 rows stamped `pass exit=0`, no could-not-run and no executing-identity refusal. The writing form appended a 14-line witness table to this brief's Evidence section in the worktree (left uncommitted). The tool states that rows 1–8 were decided on exit status only ("nothing else in the Expect cell is machine-decidable"); rows 9, 10 and 11 were decided against their Expect. The hand runs below are therefore the record for rows 1–8.

#### Hand runs (each row's as-authored text, extracted from the Verify table, run in a fresh `bash -c` at the worktree root)

| # | command (abbreviated) | exit | key observed output | result |
|---|---|---|---|---|
| 1 | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | 0 | `ok …/tools/desk/internal/deskkit 111.203s` and `ok …/tools/desk/cmd/deskdispatch 27.991s` | meets Expect (`ok` for both) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 2 | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' …/reviewscope.go` | 0 | `1` (line 74) | meets Expect (`1`) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 3 | mutation: drop the kit's design-fit block row, run `TestReviewScopeKitMatchesModel`, restore, `test $rc -ne 0 && echo RED-ON-DRIFT` | 0 | `RED-ON-DRIFT`. The mutant's log reads `--- FAIL: TestReviewScopeKitMatchesModel` / `reviewscope_test.go:299: the model names basis "design-fit" but the review kit's block does not`. The awk removed exactly one line (kit line 277); the restored kit is byte-identical to the backup and the tree shows no change under tools/. Backup and log stayed in the checkout root and are gitignored. | meets Expect; the mutation is real (the pin went red for the stated reason, not for a build error) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 4 | `grep -cE '^## [0-9]+\. Design fit first' …/review-prompt.md` | 0 | `1` (line 36, `## 3. Design fit first — before correctness, when a PR adds weight or a rule`) | meets Expect (`1`) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 5 | `grep -cE '^[\|] design-fit [\|] advisory [\|]' …/pr-review-desk/SKILL.md` | 0 | `1` (line 558) | meets Expect (`1`) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 6 | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | 0 | `1` (line 246) | meets Expect (`1`) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 7 | net-lines row for review-prompt.md, braced `${tip}` / `${base}` | 0 | `NET-OK`. impl resolves to d1258928da48 (the only first-parent commit carrying the trailer outside docs/streams and changelog), base is its parent, base ≠ tip; 461 lines at the tip, 461 at the base, net 0. Re-run under zsh: also `NET-OK`, now with real reads (the braced form no longer triggers the `:t` modifier). | meets Expect; not vacuous — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 8 | net-lines row for the pr-review-desk skill, unbraced `$tip:plugins/…` | 0 | `NET-OK`. Same impl and base; 1023 lines at the tip, 1023 at the base, net 0. Checked under zsh: `$tip:plugins` expands literally (`:p` is not applied to a parameter), so the unbraced text reads correctly there too. | meets Expect; not vacuous — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 9 | `grep -q '^func TestPrintWeight' …/weight_test.go && grep -c 'go test ./internal/weight/ -run TestPrintWeight' …/review-prompt.md` | 0 | `1` (one output line; the counter test is at weight_test.go line 301) | meets Expect (≥ 1) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 10 | shared clone at d1258928da48, `statusgen --consumers --root "$d" --brief build-less-brittle/06 --base d1258928da48~1; s=$?; rm -rf "$d"; exit $s` | 0 | `summary: 4 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. CORROBORATED: reviewscope.go, review-prompt.md, pr-review-desk SKILL.md, contracts.md. UNCHECKED: the review-scope case corpus (named in prose) and the installed binaries (out-of-scope). The throwaway clone was removed afterwards. | meets Expect verbatim. The row carries its real exit status (no trailing echo masks it) and reads the delivering diff, so it is not vacuous on merged main. The two UNCHECKED entries are not passes; they remain the reviewer's call — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 11 | clause-3 extract, `grep -c 'Brittle marks'`; `grep -c 'internal/arch'` | 0 | `1` and `1` | meets Expect (two counts, each ≥ 1) — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |

Rows that did not execute on this host: none.

#### Row defects and observations

No row was stamped pass while its output misses its Expect, no row is vacuous on merged main, and no row depends on a zsh path-modifier expansion. Three observations, none of which changes a result:

1. Rows 1–8 are decided by the witness on exit status alone. For rows 3, 7 and 8 that is sound: the token is printed by a trailing `&& echo`, so exit 0 holds only when the token was printed. For rows 2, 4, 5 and 6 the Expect is exactly `1`, while exit 0 from `grep -c` only proves a count ≥ 1; a duplicated line would still be stamped pass. The hand runs show `1` in each case.
2. Row 8 was not braced when row 7 was. It is correct under bash and zsh as written (verified), so this is a consistency note only.
3. Row 10's `consumers:` gate leaves two entries UNCHECKED by design. The case-corpus entry was not re-read by hand in this pass; the 2026-10-01 pass recorded reading it (five bases, ending design-fit). It is reported here as could-not-check by the tool, not as a pass.

Scope traceability, unchanged from the prior pass: Task 2's ≤ 14-line bound on the clause and Task 3's ≤ 5-line bound on the growth-approval step have no Verify row. Read directly at this head: clause 3 is review-prompt.md lines 36–49 (14 lines); the growth-approval step is 4 lines in the pr-review-desk skill. Both bounds are met.

#### Risk-bearing value

The trigger fires because the delivering diff touches a security-path file (reviewscope.go), although every risk answer is `no`.

Enumeration over the squash d1258928da48 (its tools/, plugins/ and contracts.md surfaces) plus the literals named in Deliverables, re-located at this head:
- `BasisDesignFit = "design-fit"` @ tools/desk/internal/deskkit/reviewscope.go:74
- kit block key `design-fit` @ tools/desk/cmd/deskdispatch/references/review-prompt.md:277
- register status `design-fit = advisory` @ plugins/assay/skills/pr-review-desk/SKILL.md:558
- tier binding "Weight growth … is strong tier too" @ plugins/assay/skills/pr-review-desk/SKILL.md:349
- clause length bound ≤ 14 lines (observed 14), growth-step bound ≤ 5 lines (observed 4), net weight bound ≤ 0 lines (461→461, 1023→1023)
- demotion threshold `> 50%` for `two consecutive months` @ plugins/assay/skills/pr-review-desk/SKILL.md:560 (named in Deliverables, not changed by the diff)
- promotion window `go-live + one month` @ docs/streams/build-less-brittle/spec.md:241

The growth-approval authority binding (the `# grow` URL must resolve to a comment by the driver's own login) has no literal in this repository: the project layer names the login and the issue.

Ranking: every entry is reversible by an edit and a release. The register status and the basis identifier rank first (the first decides whether a PR is held, the second whether model, kit and register agree); the bounds, tier binding, threshold and window are reversible operational knobs.

RISK-VALUE: DERIVED — design-fit register status = advisory @ plugins/assay/skills/pr-review-desk/SKILL.md:558 — D-A (ratified on #1660, recorded at spec.md:188 and :241) lands the class without power to hold a PR; the only correction mechanism, the > 50% two-month demotion rule, has no reversal data to act on until a month after go-live, so `advisory` is the fail-soft landing state and promotion is the one-cell edit the register already provides.
RISK-VALUE: DERIVED — BasisDesignFit = "design-fit" @ tools/desk/internal/deskkit/reviewscope.go:74 — the value's correctness criterion is identity across the surfaces a reviewer is held to; it is the same string as the kit block key (review-prompt.md:277), the register class (SKILL.md:558) and the R-design-fit-basis row (contracts.md:246), and row 3's mutation shows the pin goes red when the kit and model disagree.

rows_passed=11 rows_total=11.

VERIFY: PASS. All eleven rows of the amended table ran on merged main 2a6460e85c78 and each meets its Expect on the real output, by hand as well as by the witness (both witness runs exit 0, 11 pass). The four rows that held the 2026-10-01 pass at BLOCKED are cleared: row 10 now reads the delivering diff and returns exit 0 with the exact expected summary instead of could-not-check; row 9 prints a single count the witness can read; row 7 is shell-portable; row 3 stays inside the checkout. The mutation row fails for the stated reason. What this pass does not establish: the two UNCHECKED `consumers:` entries, which the gate does not judge. Gate is model with all risk answers `no`; this verdict records Evidence only and flips nothing.

### Non-implementer verifier run — VERIFY: PASS — 2026-10-04 claude-opus-5-5-verifier

- Target: merged main d6662bbc6b4f (detached, in the verifier's home worktree; `refs/remotes/origin/main` = HEAD = d6662bbc6b4fc64be615baf14a010da126aef16a). It contains the delivering squash d1258928da48 (medici-finance/assay#1879) and the Verify-row re-authoring 054f37274 (medici-finance/assay#2030).
- Runner: assay-verifier-app[bot], not the implementer of this brief or its code. Host darwin, statusgen v1.0.31, every hand row run in a fresh `bash` from the worktree root. Every row is class `check` (row 3 `check +mutation`); no row needs a Linux-only facility, so every row ran on this host.
- Frontmatter: gate `model`; risk regulatory no, customer no, irreversible no, sensitive-data no.
- What moved since the last pass (2026-10-02 at 2a6460e85c78, VERIFY: PASS): nothing in the deliverables. The four delivered files keep every row this brief added; the review kit grew 461→482 lines and the pr-review-desk skill 1023→1084 lines from later commits, which moved the kit's design-fit block row to line 298 and the register row to line 565.

Grounded expectation, written from the brief text before running anything: the merged work should carry a fifth scope basis `design-fit` in the deskkit model (constant plus `ScopeBases()` row), the same row in the review kit's `reviewscope` block, a kit clause "Design fit first" after the CI-first clause naming the weight-counter command, Brittle marks and `internal/arch`, a `design-fit | advisory` row in the pr-review-desk finding-class register with a weight-keyed strong tier and a growth-approval step, and an `R-design-fit-basis` row serving `S-review-verdict` in the rule register. All of that is present at this head.

| Verify row discharged | command | Expect | Observed | Result | Date | Runner |
|---|---|---|---|---|---|---|
| 1 | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | `ok` for both | exit 0. `ok …/tools/desk/internal/deskkit 89.071s`, `ok …/tools/desk/cmd/deskdispatch 32.167s`. Not vacuous: with the `{BasisDesignFit, …}` row removed from the model, TestDesignFitIsAScopeBasis fails (`ScopeBases() does not list "design-fit"`) and the kit pin fails (`the review kit names basis "design-fit" but the model does not carry it`); the model was restored byte-identical afterwards. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 2 | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' …/deskkit/reviewscope.go` | `1` | exit 0, output `1` (reviewscope.go line 74). On the squash's parent 2adf73c2a791 the same command prints `0`, exit 1. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 3 (+mutation) | drop the kit's `\| design-fit \|` block row, run `TestReviewScopeKitMatchesModel`, restore, `test $rc -ne 0 && echo RED-ON-DRIFT` (as authored) | `RED-ON-DRIFT` | exit 0, prints `RED-ON-DRIFT`. The mutant log reads `--- FAIL: TestReviewScopeKitMatchesModel` / `reviewscope_test.go:299: the model names basis "design-fit" but the review kit's block does not`. The awk removed exactly one line (kit line 298); the kit's sha256 matched before and after, and the tree showed no change under tools/. On the squash's parent the same command prints nothing and exits 1 (the test passes there, nothing to drop). | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 4 | `grep -cE '^## [0-9]+\. Design fit first' …/references/review-prompt.md` | `1` | exit 0, output `1` (line 36, `## 3. Design fit first — before correctness, when a PR adds weight or a rule`). Parent: `0`, exit 1. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 5 | `grep -cE '^[\|] design-fit [\|] advisory [\|]' …/pr-review-desk/SKILL.md` | `1` | exit 0, output `1` (line 565). Parent: `0`, exit 1. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 6 | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | `1` | exit 0, output `1` (line 246). Parent: `0`, exit 1. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 7 | net-lines row for the review kit, braced `${tip}` / `${base}` (as authored) | `NET-OK` | exit 0, `NET-OK`. impl resolves to d1258928da48 (the only first-parent commit with the trailer outside docs/streams and changelog); base is its parent; base ≠ tip; 461 lines at tip, 461 at base, net 0. Not vacuous: with no trailer commit, base and tip both fall back to HEAD and the `!=` guard stops the row before `NET-OK`. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 8 | net-lines row for the pr-review-desk skill (as authored) | `NET-OK` | exit 0, `NET-OK`. Same impl and base; 1023 lines at tip, 1023 at base, net 0. Run under bash. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 9 | `grep -q '^func TestPrintWeight' …/weight_test.go && grep -c 'go test ./internal/weight/ -run TestPrintWeight' …/review-prompt.md` | ≥ `1` | exit 0, one line `1` (the counter test is weight_test.go line 301; the clause names the command at review-prompt.md line 40). Parent: `0`, exit 1. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 10 | throwaway shared clone at d1258928da48, `statusgen --consumers --root "$d" --brief build-less-brittle/06 --base d1258928da48~1`, clone removed (as authored) | exit 0; `summary: 4 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing` | exit 0, `summary: 4 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. CORROBORATED: reviewscope.go, review-prompt.md, pr-review-desk SKILL.md, contracts.md. UNCHECKED: the review-scope case corpus (named in prose) and the installed binaries (out-of-scope). The clone directory was gone afterwards. The case-corpus entry was read by hand: review-scope-cases.md lines 17–18 list five bases ending `design-fit`. UNCHECKED entries are not passes. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 11 | clause extract, `grep -c 'Brittle marks'`; `grep -c 'internal/arch'` | two counts, each ≥ `1` | exit 0, `1` and `1`. Parent: `0` and `0`, exit 1. | PASS | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |

Rows that did not execute on this host: none. Rows found vacuous: none. Rows 2, 4, 5, 6, 9 and 11 were also run in a throwaway shared clone at the squash's parent 2adf73c2a791, where each prints a zero count and exits 1, so each one discriminates the deliverable. Row 1 at that parent exits 0 (`ok` for both packages, because TestDesignFitIsAScopeBasis does not exist there yet), so the parent run does not discriminate it; its discrimination rests on the model mutation in its Observed cell, which goes red. Row 1 needs an origin the forge resolves (the GitHub URL): with a local-path, `--shared` or git-archive origin, deskdispatch cannot resolve the forge and its stamp-identity tests fail, which is unrelated to this brief.

Deliverables with no Verify row, read directly at this head: the "Design fit first" clause runs review-prompt.md lines 36–49, 14 lines (bound ≤ 14, met); the growth-approval step is SKILL.md lines 576–579, 4 lines (bound ≤ 5, met); the weight-keyed tier line is SKILL.md line 356; the changelog fragment `changelog/build-less-brittle-06.md` landed with the squash and has since been folded into CHANGELOG.md (the design-fit stage entry is there).

Observations, none of which changes a result:
1. The witness decides rows 1–8 on exit status alone. For rows 2, 4, 5 and 6 the Expect is exactly `1`, while `grep -c` exits 0 for any count ≥ 1; the hand runs above show `1` in each.
2. Rows 7, 8 and 10 read the delivering squash, not the current head. That is their design (they decide the net-weight and consumers claims of the delivering change). Presence of the deliverables at this head is carried by rows 1–6, 9 and 11.
3. The witness stamped its Runner cells `d6662bbc6b4f+dirty`. At the moment it ran, the only entry `git status` reported was the verifier's own untracked scratch directory; no tracked file differed from the head (the row-3 backup and log are gitignored).

#### Execution witness

`statusgen verifyrun --brief docs/streams/build-less-brittle/brief-06-design-fit-review-stage.md`, run from the worktree root (exit 0; 11 of 11 rows `pass exit=0`), then `statusgen verifyrun --check --brief <same>` (exit 0). The check's summary line:

docs/streams/build-less-brittle/brief-06-design-fit-review-stage.md: 11 pass, 0 fail, 0 could-not-run/missing (of 11 Verify rows)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | pass exit=0 | sha256:fed43c277c92 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' tools/desk/internal/deskkit/reviewscope.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && f=cmd/deskdispatch/references/review-prompt.md && cp "$f" ../../bl06-kit.bak && awk '!/^[\|] design-fit [\|]/' ../../bl06-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestReviewScopeKitMatchesModel -count=1 > ../../bl06-mut.out 2>&1; rc=$?; cp ../../bl06-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | pass exit=0 | sha256:cd183bfd8e84 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE '^## [0-9]+\. Design fit first' tools/desk/cmd/deskdispatch/references/review-prompt.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -cE '^[\|] design-fit [\|] advisory [\|]' plugins/assay/skills/pr-review-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "${tip}:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "${base}:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -q '^func TestPrintWeight' tools/desk/internal/weight/weight_test.go && grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/review-prompt.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `d=$(mktemp -d "$PWD/.bl06-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach d1258928da48 && statusgen --consumers --root "$d" --brief build-less-brittle/06 --base d1258928da48~1; s=$?; rm -rf "$d"; exit $s` | pass exit=0 | sha256:85136008240b | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'Brittle marks'; echo "$s" \| grep -c 'internal/arch'` | pass exit=0 | sha256:ad0fadf63cc7 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |


#### Risk-bearing value

The trigger fires because the delivering diff touches a security-path file (reviewscope.go), although every risk answer is `no` and irreversible is `no`.

Step 1, enumeration, over the squash d1258928da48's tools/, plugins/ and contracts.md surfaces plus the literals named in the brief's Deliverables, located at this head:
- `BasisDesignFit = "design-fit"` @ tools/desk/internal/deskkit/reviewscope.go:74, and its `ScopeBases()` row @ reviewscope.go:93
- kit block key `design-fit` @ tools/desk/cmd/deskdispatch/references/review-prompt.md:298
- register status `design-fit = advisory` @ plugins/assay/skills/pr-review-desk/SKILL.md:565
- tier binding "Weight growth … is strong tier too" @ plugins/assay/skills/pr-review-desk/SKILL.md:356
- clause length bound `≤ 14` lines (observed 14, review-prompt.md:36–49); growth-step bound `≤ 5` lines (observed 4, SKILL.md:576–579); net weight bound `≤ 0` lines (kit 461→461, skill 1023→1023 across the squash)
- demotion threshold `> 50%` for `two consecutive months` @ plugins/assay/skills/pr-review-desk/SKILL.md:567 (named in Deliverables, not changed by the diff)
- promotion window `go-live + one month` @ docs/streams/build-less-brittle/spec.md:241

The growth-approval authority binding (a `# grow` line's URL must resolve to a comment by the driver's own login, SKILL.md:578–579) has no literal in this repository: the project layer names the login and the issue, by design.

Step 3, ranking: every entry is reversible by an edit and a release (the kits are embedded in the deskdispatch binary and reach consumers on the next desk-tools release and pin bump). The register status ranks first, because it decides whether a design-fit finding holds a PR. The basis identifier ranks second, because it decides whether model, kit and register agree. The line bounds, tier binding, threshold and window are reversible operational knobs.

RISK-VALUE: DERIVED — design-fit register status = advisory @ plugins/assay/skills/pr-review-desk/SKILL.md:565 — D-A (spec.md:188–192 and the §10 decision row at spec.md:241) lands the class without power to hold a PR. Its only self-correction, the > 50% / two-consecutive-month reversal demotion (SKILL.md:567), has no reversal data to act on until the class has run for a while, so a class landed `blocking` could hold PRs on reviewer taste with nothing to correct it. `advisory` is the fail-soft landing state; promotion is the one-cell edit the register already provides.
RISK-VALUE: DERIVED — BasisDesignFit = "design-fit" @ tools/desk/internal/deskkit/reviewscope.go:74 — the value's only correctness criterion is being one identifier on every surface a reviewer is held to. Read directly, it is the same string as the kit block key (review-prompt.md:298), the register class (SKILL.md:565), the basis named in the R-design-fit-basis row (contracts.md:246), the case corpus's fifth basis (review-scope-cases.md:18) and the stage name in spec §4.4. Row 3's mutation and this run's model mutation show the pin goes red whichever side drops it.

rows_passed=11 rows_total=11.

VERIFY: PASS. All eleven rows ran on merged main d6662bbc6b4f, and each meets its Expect on the real output, by hand and by the execution witness (11 pass, 0 fail, 0 could-not-run). Both mutations (kit row and model row) turn the pin red for the stated reason, and every grep row reads zero on the pre-delivery parent. Gate is `model` with every risk answer `no`; this run lands the Evidence, the `implemented → verified` board flip for row 06 and its verify-outcome record. The `verified → done` flip is CI's and is not made here.

## Review
Gate: model (from frontmatter). The reviewer answers this stage's own three questions about this
PR. It adds an enum value, a clause and a register row. Is the review kit the right owner, and
did the offsets remove narrative rather than rules?
