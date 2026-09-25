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
version: 1
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
  - "tools/desk/internal/deskkit/reviewscope.go ScopeBases(): follow-up build-less-brittle/06 (this brief)"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md reviewscope block + new clause: follow-up build-less-brittle/06 (this brief)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md tiering + finding-class register: follow-up build-less-brittle/06 (this brief)"
  - "docs/contracts.md rule register (R-design-fit-basis row): follow-up build-less-brittle/06 (this brief)"
  - "the review-scope case corpus scored against ScopeBases(), if present: follow-up build-less-brittle/06 (this brief)"
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
pin. Rows 7–8 are net ≤ 0 weight rows.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ ./cmd/deskdispatch/ -count=1` | `ok` for both |
| 2 | `grep -c 'BasisDesignFit ScopeBasis = "design-fit"' tools/desk/internal/deskkit/reviewscope.go` | `1` |
| 3 | `cd tools/desk && f=cmd/deskdispatch/references/review-prompt.md && cp "$f" /tmp/bl06-kit.bak && awk '!/^[\|] design-fit [\|]/' /tmp/bl06-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestReviewScopeKitMatchesModel -count=1 > /tmp/bl06-mut.out 2>&1; rc=$?; cp /tmp/bl06-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | `RED-ON-DRIFT` (dropping the kit row while the model keeps the basis fails the pin) |
| 4 | `grep -cE '^## [0-9]+\. Design fit first' tools/desk/cmd/deskdispatch/references/review-prompt.md` | `1` |
| 5 | `grep -cE '^[\|] design-fit [\|] advisory [\|]' plugins/assay/skills/pr-review-desk/SKILL.md` | `1` (D-A: the class lands `advisory`; the promotion to `blocking` is a later recorded decision, a one-cell edit) |
| 6 | `grep -cE '^[\|] *R-design-fit-basis .*S-review-verdict' docs/contracts.md` | `1` (the new rule is registered and serves an existing S- row) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/06$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 9 | `grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/review-prompt.md && test -f tools/desk/internal/weight/weight_test.go && echo COUNTER-EXISTS` | a count ≥ `1`, then `COUNTER-EXISTS` |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/06; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |
| 11 | `s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'Brittle marks'; echo "$s" \| grep -c 'internal/arch'` | two counts, each ≥ `1` (a marked module and a red arch test both trigger the stage) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer answers this stage's own three questions about this
PR. It adds an enum value, a clause and a register row. Is the review kit the right owner, and
did the offsets remove narrative rather than rules?
