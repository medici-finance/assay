---
brief: methodology/19
title: Verification-gate hardening — risk-keyed verifier floor + Verify-table structure lint
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by Fable session (assay-review-1)
sources: ["docs/assay-review-1/README.md (B-01, B-02)", "[F-16](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verifier-tier-practice-contradicts-the-tiering-default-and-t.md) (verifier-tier practice contradicts the tiering default and the verify-desk spec)", "PR #159 (merged 49a38d18, 2026-07-09: human gate at verified for ANY irreversible:yes brief; this brief covers the remaining risk band + the structural lints)", "PR #171 (in-flight: verify-gate issues — gate:human verified→done via human:<name>-closed GitHub issues; adjacent machinery, coordinate)", "issue #147 (a glm-verified fix missed a sibling read path — ledger-hardening/05, PR #139)", "[I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md) (challenge→verification-depth: the long-term derived model this floor bridges to)", "statusgen source inventory 2026-07-09: no Verify-section check exists anywhere in tools/statusgen"]
---

# Brief 19 — Verification-gate hardening: risk-keyed verifier floor + Verify-table structure lint

## Context
files: tools/statusgen/{brieffile.go,attribution.go,checks.go} + tests; docs/streams/methodology/README.md (conventions)
facts:
- statusgen parses `## Evidence` but **never parses `## Verify`** — a brief-v1 brief with no Verify
  table at all lints clean today, while the author-brief closing steps imply pre-flight covers it.
  The verify-desk's entire worklist is "run the Verify table"; a missing/empty table makes a brief
  unverifiable and nothing says so.
- **PR #159 (MERGED 2026-07-09, 49a38d18) already covers the sharpest slice** — broader than its
  draft: ANY `irreversible: yes` brief now requires a `human:` Reviewed entry to be marked
  `verified`/`done`. daml-hardening/01 was demoted under that gate and has since received its
  human sign-off (#166). PR #171 (in-flight) adds the verified→done issue-close machinery for
  `gate: human` briefs. Build on both; do not duplicate either.
- **The remaining band is uncovered**: a brief with `gate: human`-without-irreversible, or
  `customer: yes` / `sensitive-data: yes` alone, can still be marked `verified` on the cheapest
  tier — and NO risk class constrains the Verified-cell (verifier) tier itself; #159 constrains
  the Reviewed cell only. Specimen of why verifier tier matters: issue #147 — a glm-verified
  brief (ledger-hardening/05) missed a sibling read path of the exact class it claimed to close.
- Policy texts disagree ([F-16](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verifier-tier-practice-contradicts-the-tiering-default-and-t.md)): methodology/05 default says "strong-tier/human verifies"; the
  verify-desk skill says "opus+"; operating practice (human:<name>-directed, for cost) is glm-5.2 across
  the board. The reconciliation is a FLOOR, not a blanket: cheap verifiers are fine where all four
  risk answers are `no`; risk-flagged briefs get a strong-tier or human verifier.
- The Reviewed cell today has NO format requirement except `human:<name>` at done for gate:human —
  methodology/README.md:143-149 overclaims "runner-attribution on the Verified/Reviewed cell text";
  attribution runs on the Verified cell + Evidence rows only (attribution.go).
- Checker changes never weaken existing checks; new checks are opt-in via `schema: brief-v1`
  (stream convention). All lints below must pass on the current tree — verify before landing.

## Ground rules
- NEVER push to main / trigger workflows / run mutating kubectl. Feature-branch push + draft PR per
  the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) loop is the sanctioned flow; leave other commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
0. Rebase context first: #159 is merged (build on its irreversible-at-verified check — same code
   region, brieffile.go after the gate-human block). Check whether PR #171 (verify-gate issues)
   has merged; it touches adjacent statusgen surface (verified→done machinery) — coordinate so
   this brief's lints and #171's flip logic agree on cell formats.
1. **Verify-table structure lint (brief-v1):** a brief file must contain a `## Verify` section with
   ≥1 table row whose Command and Expect cells are non-empty. Structure/presence only — [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md) stands:
   this lint never claims quality, and the lint's error text must not imply it does.
2. **Risk-keyed verifier floor (brief-v1):** when a brief has `gate: human` OR any risk answer
   `yes`, and its README row is `verified` or `done`, the Verified cell must either contain
   `human:` or a runner NOT matching the cheap-tier token list (one place in code, case-insensitive:
   `glm|haiku|mini|flash|lite`; document how to extend it). Where #159's stricter human-at-verified
   rule already fires (regulatory+irreversible), that rule wins; this floor covers the rest of the
   band. Error text names the cell, the matched token, and the rule ("risk-flagged briefs verify at
   strong tier or human — methodology/19").
   > **Historical note (2026-08-09):** this task description is preserved as authored. The
   > "cheap-tier token list" it specifies (a bare-substring `glm|haiku|mini|flash|lite` match,
   > pricing-framed) was superseded by #298 (merged 3128cd4e, shipped `statusgen/v0.7.0`), which
   > re-founded the floor on **capability, not price**: `glm` was removed (it is cheap but
   > capable), `deepseek` and `sonnet` were added, and matching moved from bare substring to
   > name-segment. The single current source of truth is `belowFloorModels` in
   > `statusgen/attribution.go`; do not treat the list above as current.
3. **Reviewed-cell attribution at done (brief-v1):** at `done`, the Reviewed cell must match the
   same dated-runner shape as the Verified cell (`YYYY-MM-DD <runner>`); the existing `human:<name>`
   rule for gate:human is unchanged. This closes the README:143-149 overclaim by making it true.
4. Update the methodology README tiering convention to state the floor (replacing the blanket
   "strong-tier verifies" default), and note the verify-desk dispatch implication: risk-flagged
   briefs dispatch opus+/human verifiers, risk-clear briefs may use cheap tier. TDD throughout.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run 'TestVerifySection|TestVerifierFloor|TestReviewedCell' -v` | exit 0; ≥6 subtests PASS (present/missing/empty-table Verify; cheap-runner-on-risk-flagged rejected; human: accepted; non-cheap model accepted; dated Reviewed at done) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 (including #159's TestRiskGate cases if merged — no weakening) |
| 3 | `statusgen --root . --lint` | exit 0 on the current tree (no existing brief regresses) |
| 4 | `grep -ci "floor" docs/streams/methodology/README.md` | ≥1 (convention updated) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run 'VerifySection\|VerifierFloor\|ReviewedCell' -v` | 0 | 14 subtests PASS (≥6) | 2026-07-10 | opus-verifier |
| 2 | `go test && go vet ./tools/statusgen/` | 0 | ok; vet clean | 2026-07-10 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | 0 PROBLEM | 2026-07-10 | opus-verifier |
| 4 | `grep -ci floor README.md` | 0 | 5 (≥1) | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — verification-floor gate enforced (Verify-section + reviewer-cell checks).

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Note for the reviewer: per this brief's own rule, its verification may run at cheap tier (all risk
answers are no) — the floor it introduces applies to risk-flagged briefs, not to itself.
