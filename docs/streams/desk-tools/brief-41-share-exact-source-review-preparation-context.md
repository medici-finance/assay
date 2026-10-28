---
brief: assay:assay:desk-tools:41
title: Share exact-source review preparation context
why: "Review round and packet builders repeat reads and can read a different head or the allocated worktree instead of the assigned change."
wave: 3
depends: ["desk-tools/38", "desk-tools/39"]
unblocks: ["desk-tools/43", "desk-tools/44"]
effort: M
gate: human
decision-issue: 4523
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Share exact-source review preparation context'
id: 8e177504-b12b-4d23-a48b-81e7764a5558
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
consumers:
  - "tools/desk/cmd/deskdispatch/: fixed-here"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here"
  - ".cursor/skills/pr-review-desk/SKILL.md: fixed-here"
  - ".cursor/assay-managed.json: fixed-here"
---

# Brief 41 — Share exact-source review preparation context

## Context

files: `tools/desk/cmd/deskdispatch/reviewroundread.go` (existing), `tools/desk/cmd/deskdispatch/packet_review.go` (existing), `tools/desk/cmd/deskdispatch/main.go` (existing), `tools/desk/internal/reviewflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `tools/desk/internal/gitcore/` (existing), `changelog/desk-tools-41.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: deskdispatch preparation adapters over shared reviewflow context and gitcore exact objects. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: deskdispatch preparation adapters over shared reviewflow context and gitcore exact objects
  contract: approved spec PrepareReview/ReviewContext and A4; existing claim/effect preconditions
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +1 observation input, refusals +0 authority, rule-text +0 duplicate policy
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Task

1. Add --observation FILE to existing deskdispatch --kit review; advisory review observations supply
   applicable facts without authorizing effects. Construct one typed ReviewContext and pass it to
   round and packet builders. Reuse immutable source facts within preparation before considering
   persistent caching.
2. Key source reuse by provider-qualified identity, full head SHA, path and content/adapter version;
   diff/merge-base keys include both immutable endpoints and relevant algorithm/version. A branch
   name, mtime or allocated worktree HEAD is not a key. Use existing gitcore exact-object reads; no
   shared object-store project.
3. Body/review/check state, permissions, claims and ready/publication inputs remain mutable with
   their existing freshness boundaries. Recheck before consequential operations; retain
   dispatch/model admission, independent review lanes, refusal/hold history and effect
   intent/receipt/unknown behavior. Respect existing file/history budgets and report partial packet
   coverage.
4. Measure complete dispatch plus agent preparation to detect work shifted downstream. Coordinate
   active #3302 parser port. Future desk-review-prepare integration remains the role executor
   owner's cutover; this bridge grants no new custody or writer activation.
5. Update affected command/operator contracts and the declared changelog; regenerate applicable
   documentation with the existing statusgen README generator. No generated global board is branch-
   committed. All planned tests must be wired to actual production paths, not a parallel test
   implementation.

## Verify

Run from the public repository root; each named test is a planned deliverable and its PASS
line is mandatory (zero selected tests is a failure). No authoring-time execution Evidence
is asserted. Each test covers production flow plus the stated wrong-but-plausible mutation.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPrepareFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewPrepareFlow ' "$o")` | exit 0; named PASS. Real dispatch preparation uses one exact-source context for round and packet; an allocated worktree at another head still yields assigned-head bytes, and duplicate immutable reads are counted once. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPrepareInvalidation$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewPrepareInvalidation ' "$o")` | exit 0; named PASS. Head/base/body/review/check/access changes invalidate applicable inputs; a saved observation cannot bypass claim, attestation, authorization or immediate consequential freshness rechecks. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/41` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Architecture pickup

Continue against main `d92df1b1ab898db5f537fab33dae3087114c9fb0` after #4248 merged.
The controlling spec and change map, library-first direction, brief rules, telemetry
contract and semantic-owner index were read at that revision. Spec SHA-256:
`c2aa79b9423289b3484eb81973ae92f5a538dfde015c770938eb250b8790e1ed`;
library-first SHA-256: `9272c94d8ff0fa2f02ddf6ee173e12e267527515c2e9ba88eb92fcb9bf3103af`.
The existing round, gate, packet, CLI and gitcore APIs are inspected; #3248 and
#3302 are merged, so this extends the delivered CLI parser rather than another port.
Reuse `reviewflow` for the pure context/key values, deskdispatch for credentialed
composition, gitcore for exact objects and deskkit for checked provider transport.
The process boundary remains at the existing CLI/custody owner. No SDK gains credentials.

Adapt the stale prompt consumer locator to the shipped embedded reference under
`tools/desk/cmd/deskdispatch/references/`; no parallel prompt owner is created.
Observation v1 contains resolved advisory classification, not raw reviews/checks or
an authenticated approved-policy receipt. Therefore preparation takes source
attribution from it and reads mutable raw inputs live; it cannot remove those reads
by trusting the file. Existing terminal checks are strengthened, not replaced.
The API/CLI/packet consumers are updated here; actual prose/discrete flag adoption
remains desk-tools/43 and full attributed downstream cost remains /44. Existing
readonly packet guidance is retained. No lifecycle/design approval or independent
Evidence is inferred from this implementation checkpoint.

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
