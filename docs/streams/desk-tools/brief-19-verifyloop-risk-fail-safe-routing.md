---
brief: assay:assay:desk-tools:19
title: "`verifyloop plan` fails safe on risk — any risk answer `yes` routes to ROUTE-HUMAN, and the Evidence-only lane says so"
why: >-
  `verifyloop plan` is meant to keep risk-flagged briefs out of the dispatchable list, and it
  does — for three of the four risk answers. The fourth, `irreversible`, is routed by the tier
  policy to the LOCAL model tier with higher precedence than the risk branch, so a brief whose
  most serious risk answer is `yes` is printed under `=== DISPATCH … (tier=local) ===` while a
  sibling brief carrying `customer: yes` and nothing else is correctly held back. A reader of
  the plan cannot tell the two apart from the output, and the one that reads as dispatchable is
  the more dangerous of the two. A gate that fails OPEN on its most serious input is worse than
  no gate, because the plan output is what a desk trusts instead of re-reading the frontmatter.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-06 by an authoring session, from a maintainer ruling recorded 2026-09-06
sources:
  - "Maintainer ruling, 2026-09-06: FAIL SAFE — any risk field set to yes (irreversible first) routes to ROUTE-HUMAN. A model may still gather Evidence for such a brief, but the plan output must say so explicitly: Evidence-only, never flip-eligible."
  - "freshness-checked 2026-09-06 @ 0af8093 (origin/main) — `tools/desk/cmd/verifyloop/tier.go` § `TierPolicy` still returns `TierLocal` for `it.Risk.Irreversible` BEFORE the risk-flagged branch; `queueclass.go` § `classifyItem` catches only `tier == TierHuman` or a literal `gate: human`, so an irreversible brief whose gate is `model` reaches dispDispatch; `tools/desk/internal/loopengine/engine.go` § `Flagged` compares `gate == \"human\"` byte-exactly, so a re-cased or qualified gate value is not flagged either. Both leaks are live."
  - "The Evidence-only lane the ruling preserves: `tools/desk/cmd/verifyloop/land.go` § `Land` — irreversible + PASS writes Evidence with NO status flip and opens a checkpoint PR for a human."
exec-tier: strong
exec-tier-why: >-
  (c) a routing gate whose failure mode is fail-OPEN: every happy-path test on a risk-clear
  brief passes whether the fix is right or wrong, and the wrong version is indistinguishable
  from the right one except on inputs a table has to be written to produce.
consumers:
  - "tools/desk/internal/loopengine/engine.go (RiskFlags.Flagged): the shared predicate gains gate-value normalization and an Any() sibling. Sole other consumer verified at 0af8093 — the batch fan-out loop's tier policy does NOT call Flagged (it has no human branch by design), so no other loop's routing changes: out-of-scope (read and asserted, not changed)."
  - "tools/desk/cmd/verifyloop/land.go: out-of-scope (the irreversible Evidence-without-flip path is deliberately kept as the Evidence-only lane and is not modified)."
version: 1
id: 30e33995-765b-4d05-a3bf-200656eb4fc3
---

# Brief 19 — `verifyloop plan`: fail safe on risk

## Dependencies
None.

## Context

files:
- `tools/desk/cmd/verifyloop/tier.go` (`TierPolicy` — the irreversible arm)
- `tools/desk/cmd/verifyloop/queueclass.go` (`classifyItem`, `whyItWaits`, the bucket header)
- `tools/desk/cmd/verifyloop/main.go` (`printBuckets` — the ROUTE-HUMAN heading and the
  per-member Evidence-only line)
- `tools/desk/internal/loopengine/engine.go` (`RiskFlags.Flagged`, plus a new `Any()`)
- the packages' tests: `tools/desk/cmd/verifyloop/queueclass_test.go`,
  `tools/desk/internal/loopengine/engine_test.go`

facts (all read at `0af8093`, 2026-09-06):
- `TierPolicy` precedence today is: `Risk.Irreversible` → `TierLocal` (dispatched so the
  Evidence is real), THEN `Risk.Flagged(gate)` → `TierHuman`, THEN `TierLocal`. The first arm
  shadows the second for exactly the most serious risk answer.
- `classifyItem` buckets an item as awaiting-human when `tier == TierHuman` OR the brief's own
  `gate` is `human` (compared with `EqualFold` after trimming). An irreversible brief whose
  gate is `model` satisfies neither, so it falls through to `dispDispatch` and the plan prints
  it under `=== DISPATCH … (tier=local) ===`.
- `RiskFlags.Flagged(gate)` is `gate == "human" || regulatory || customer || irreversible ||
  sensitive-data` — a byte-exact gate comparison. `Human`, `human ` and a gate value carrying
  a trailing qualifier are all NOT flagged by it, which is a second, independent way an item
  can miss the human branch.
- The frontmatter reader is a small regex extractor: `gate:` is read by
  `scalar(body, "gate")`, which trims and strips surrounding quotes but keeps any trailing
  text on the line; each risk key is `yes` when `<key>\s*:\s*yes` matches case-insensitively
  anywhere in the frontmatter block (so both the inline flow map and per-line forms work).
- `F16ReversibleRiskToSession` is a deliberately dormant flag that routes risk-flagged but
  REVERSIBLE briefs to `TierSession`. It is off, and its meaning must be preserved exactly:
  it has never applied to irreversible work and must not begin to.
- `Land` already implements the Evidence-only lane: on `VerdictPass` with
  `Item.Risk.Irreversible` it writes Evidence with `flip=false` and opens a checkpoint PR. It
  also has a `VerdictRouteHuman` arm that calls `RouteHuman` and continues the drain. Neither
  is changed by this brief; the plan must stop CONTRADICTING them.
- `printBuckets` renders each non-dispatch disposition as `-- <slug> (<n>): <why>` followed by
  one line per member, with an optional per-member reason. The slug strings are stable and are
  asserted by tests.

single-point-of-failure: the ONE control keeping a risk-flagged brief out of the dispatchable
list is the plan's classification of it. The design therefore puts the decision in TWO places
that fail on different signals in different components: the tier policy (which computes a tier
from the risk flags) and the queue classifier (which reads the item's OWN frontmatter and
refuses to dispatch a risk-carrying item whatever tier it was handed). Either alone would be a
single point; together, a future edit to one is caught by the other's tests. Behind both sits a
third, independent layer already in place: the landing path writes NO status flip for an
irreversible brief even if one were somehow dispatched and passed — the flip is the human's
merge of the checkpoint PR. The three fail on a tier value, a frontmatter read, and a landing
decision respectively.

## Ground rules
- NEVER git push, trigger workflows, or contact the forge from a test or a Verify row.
- Stop at `implemented` — you do not set verified/done.
- Do NOT change `Land`'s irreversible arm, and do NOT remove the dormant reversible-risk flag
  or change what it means. Widening it is a separate decision that is not this brief's.
- The fix direction is one-way: this brief may only move items OUT of the dispatchable list,
  never into it. A change that makes any previously-bucketed item dispatchable is out of scope
  and is a defect.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Normalize the gate value** in `RiskFlags.Flagged`: trim, lowercase, and read the first
   whitespace-delimited token, so `human`, `Human` and `human — <qualifier>` all count as the
   human gate and anything else does not. Add `func (r RiskFlags) Any() bool` returning true
   when ANY of the four answers is yes, and express `Flagged` in terms of it.
2. **Tier policy fails safe.** Reorder `TierPolicy` so irreversible routes to `TierHuman`
   FIRST, before the reversible-risk branch that the dormant flag can divert:
   `Irreversible → TierHuman`; then `Flagged(gate) → TierSession` when the dormant flag is on,
   else `TierHuman`; then `TierLocal`. Rewrite the doc comment: it currently documents the old
   precedence as an irreducible, and that text is now wrong.
3. **Classifier fails safe independently.** In `classifyItem`, the awaiting-human arm becomes
   `tier == TierHuman || gate is human || it.Risk.Any()`. Keep `blocked-until` ahead of it
   (an item that cannot be attempted at all is still deferred first) and keep the remaining
   precedence unchanged. Return a per-item REASON naming which risk answers are yes, in the
   canonical key order, e.g. `risk: irreversible` or `risk: customer, irreversible`.
4. **Plan output says it explicitly.** The awaiting-human bucket prints as
   `-- awaiting-human / ROUTE-HUMAN (<n>): <why>` — the stable slug is kept for tests, the
   routing name is made visible. `whyItWaits` for that disposition becomes a line to the effect
   of: gated on human sign-off (gate:human or any risk answer yes) — a model MAY gather
   Evidence for it, and never flips it. Every member line for an item with a risk answer yes
   carries its reason plus the literal marker `Evidence-only (never flip-eligible)`, so the
   permission and its limit are on the same line. An item bucketed for `gate: human` alone
   keeps its existing member line.
5. **Tests.**
   - A TABLE-DRIVEN test over frontmatter shapes in `queueclass_test.go`: each of the four
     risk keys individually `yes` (plus the all-`no` row), crossed with each gate form
     (`model`, `human`, absent, `Human`, and a `human` value carrying a trailing qualifier).
     Expected disposition: `awaiting-human` whenever any risk answer is yes OR the gate reads
     human; `dispatch` only for the all-`no` + non-human-gate rows. Assert the tier the policy
     computed for the same input in the same table, so a divergence between the two layers is
     a test failure rather than a silent agreement.
   - A test on the PLAN OUTPUT: an irreversible, `gate: model` brief appears under the
     ROUTE-HUMAN heading with the `Evidence-only (never flip-eligible)` marker, and the string
     `=== DISPATCH` does not appear for that item's ID.
   - `engine_test.go`: `Flagged` on the gate-value variants, and `Any()` over the sixteen
     combinations of the four booleans.
   - A test that the dormant reversible-risk flag, when enabled, still does NOT divert an
     irreversible item away from `TierHuman`.
6. **Docs.** One line in `tools/desk/README.md`'s verifyloop section (and the `plan` usage
   text, if it describes the buckets) stating the fail-safe rule and the Evidence-only lane.
7. **Changelog fragment** under `changelog/` (`### Fixed`).
8. **Nothing else.**

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestClassifyItemRiskGateTable$' -count=1` | exit 0 — the table over risk-key × gate-form; every risk-yes row is awaiting-human, and the tier computed for the same row agrees |
| 3 | check:ci | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestIrreversibleBriefIsRouteHumanNotDispatch$' -count=1` | exit 0 — an irreversible `gate: model` brief prints under the ROUTE-HUMAN heading and its ID never appears after `=== DISPATCH` |
| 4 | check:ci | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRouteHumanLineCarriesEvidenceOnlyMarker$' -count=1` | exit 0 — the member line carries the risk reason and the literal `Evidence-only (never flip-eligible)` |
| 5 | check:ci | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRiskClearBriefStillDispatches$' -count=1` | exit 0 — the NEGATIVE control: an all-`no`, `gate: model` brief is still a DISPATCH candidate at tier local, and no previously-dispatchable shape was swept into the bucket |
| 6 | check:ci | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestDormantReversibleFlagNeverDivertsIrreversible$' -count=1` | exit 0 |
| 7 | check:ci | `cd tools/desk && go test ./internal/loopengine/ -count=1` | exit 0 — `Flagged` gate-value variants and `Any()` over all sixteen combinations |
| 8 | check:ci | `cd tools/desk && go test ./... -count=1` | exit 0 — the whole suite, including the untouched landing tests |
| 9 | check:ci | `gofmt -l tools/desk/cmd/verifyloop tools/desk/internal/loopengine > /tmp/b19-fmt.out; test ! -s /tmp/b19-fmt.out` | exit 0 |
| 10 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The tier policy is fixed but the classifier still admits a risk-yes item handed a non-human tier | row 2 — the table asserts the disposition AND the tier for the same input |
| The classifier is fixed and the tier policy is not, so the two layers stop agreeing and a later edit to the classifier alone re-opens the leak | row 2, same reason |
| The fix over-applies and sweeps risk-clear briefs out of the dispatchable list, quietly emptying the verify queue | row 5, the negative control |
| A re-cased or qualified `gate:` value still misses the human branch | row 2 (gate-form dimension) + row 7 |
| ROUTE-HUMAN is printed but a reader cannot tell that Evidence-gathering is still permitted, so the lane goes unused | row 4 |
| The dormant reversible-risk flag is enabled later and diverts irreversible work to a model tier | row 6 |
| The landing path's no-flip guarantee is refactored away while the routing is edited | row 8 (the landing tests are unchanged and must still pass) |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian))

VERIFY: BLOCKED — 0/10 witnessed pass, 10 could-not-check, 0 fail. Non-implementer run from a
worktree cut detached at merged main `893cd6114b0382a1f71e6ef763c6601a5e19d270`
(darwin/arm64, go1.26.5, offline, `KUBECONFIG=/dev/null`). No implementer Evidence was recorded in
this section before this run. All ten Verify rows are `check:ci`; the `statusgen verifyrun`
execution witness below (statusgen built from this tree) recorded every one as could-not-run,
because its network-off sandbox needs Linux `unshare --net` and this host is darwin. Each row was
then run directly, non-hermetic, and that run is recorded separately in the hand-written table
after the witness. Direct runs: nine rows ended exit 0 (row 7 on its second attempt). Row 8 ended
exit 1 on both attempts. The only failures were deadline-timing tests outside this brief's diff,
on a host at load average 45–124. These are the known open flakes #1232 and #612. Every test
this brief added or relies on passed. See the row notes. Evidence-only: the board row stays `implemented` and nothing is flipped here — see the
routing note after the tables.

Execution witness (tool-generated, verbatim):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestClassifyItemRiskGateTable$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestIrreversibleBriefIsRouteHumanNotDispatch$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRouteHumanLineCarriesEvidenceOnlyMarker$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRiskClearBriefStillDispatches$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestDormantReversibleFlagNeverDivertsIrreversible$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./internal/loopengine/ -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./... -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `gofmt -l tools/desk/cmd/verifyloop tools/desk/internal/loopengine > /tmp/b19-fmt.out; test ! -s /tmp/b19-fmt.out` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03+dirty (on-behalf-of human:ian) (forge-identity) |

Direct non-hermetic run (hand-written; separate from the witness above):

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (build and vet both silent across the module) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestClassifyItemRiskGateTable$' -count=1` | exit 0 — risk-key × gate-form table, disposition and tier agree | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (25 of 25 subtests PASS = 5 risk rows × 5 gate forms; the table asserts the TierPolicy tier and the classifyItem disposition for the same input) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestIrreversibleBriefIsRouteHumanNotDispatch$' -count=1` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (--- PASS: TestIrreversibleBriefIsRouteHumanNotDispatch) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRouteHumanLineCarriesEvidenceOnlyMarker$' -count=1` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (--- PASS: TestRouteHumanLineCarriesEvidenceOnlyMarker) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestRiskClearBriefStillDispatches$' -count=1` | exit 0 — negative control | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (--- PASS: TestRiskClearBriefStillDispatches) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd tools/desk && go test ./cmd/verifyloop/ -run '^TestDormantReversibleFlagNeverDivertsIrreversible$' -count=1` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0 (--- PASS: TestDormantReversibleFlagNeverDivertsIrreversible) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `cd tools/desk && go test ./internal/loopengine/ -count=1` | exit 0 — Flagged gate variants and Any() over sixteen combinations | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: attempts 1 and 2 exit 1 at host load average ~124, only in deadline-timing tests outside this brief's diff (TestDrain "engine did not stop within deadline" and "max concurrency observed 1", TestRun_PoolRefillAfterCompletion), the known open flakes #1232 and #612; attempt 3 at load ~34: exit 0 (ok loopengine 9.976s). The brief's own TestRiskFlags_Flagged_GateVariants and TestRiskFlags_Any_AllSixteen PASS on every attempt | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && go test ./... -count=1` | exit 0 — whole suite including the untouched landing tests | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 1 on both attempts (84 packages ok on attempt 2), failing only in deadline-timing tests outside this brief's diff at host load average 45–124: loopengine TestDrain and TestRun_PoolRefillAfterCompletion (both attempts), TestRun_IsDoneCallsOnIdleNeverExits and TestRun_LandFailureFilesAndContinues (attempt 1), and commsloop TestRunDoesNotBusySpinOnEmptyQueue (attempt 1). All are "did not stop within deadline" shapes, the known open flakes #1232 and #612. commsloop re-run alone: exit 0. loopengine re-run alone: exit 0. verifyloop, including the landing tests: ok on both attempts | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `gofmt -l tools/desk/cmd/verifyloop tools/desk/internal/loopengine > /tmp/b19-fmt.out; test ! -s /tmp/b19-fmt.out` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0, empty gofmt listing (output file redirected to the session scratch directory instead of the shared /tmp) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); direct run: exit 0, LINT: PASS, zero PROBLEM lines (NOTICEs only) | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Risk-bearing values** (enumeration over the diff of the implementing commit 635f8bde6 across
the verifyloop and loopengine packages, plus the values named in this brief's Task and facts):

1. evidenceOnlyMarker = "Evidence-only (never flip-eligible)" @ tools/desk/cmd/verifyloop/main.go:233. This is a display string, reversible.
2. bucket heading suffix " / ROUTE-HUMAN" @ tools/desk/cmd/verifyloop/main.go:241. This is a display string, reversible.
3. GateIsHuman first-token literal "human" (f[0] == "human") @ tools/desk/internal/loopengine/engine.go:71. This is an authority binding: which gate value routes to the human.
4. TierPolicy irreversible arm → loopengine.TierHuman @ tools/desk/cmd/verifyloop/tier.go:31. This is an authority binding. Before this brief it was TierLocal.
5. TierPolicy Flagged arm, flag off → loopengine.TierHuman @ tools/desk/cmd/verifyloop/tier.go:45. This value is unchanged; only its precedence moved.
6. riskReason key literals "regulatory" / "customer" / "irreversible" / "sensitive-data" and prefix "risk: " @ tools/desk/cmd/verifyloop/queueclass.go:255–269. These are display strings, reversible.
7. whyItWaits awaiting-human text @ tools/desk/cmd/verifyloop/queueclass.go:130. This is a display string, reversible.
8. parseRisk matcher (?i)<key>\s*:\s*yes over the whole frontmatter block @ tools/desk/cmd/verifyloop/frontmatter.go:61. This was not changed by this brief. It is named in this brief's facts, and it is the input both layers depend on.

The dormant reversible-risk flag (F16ReversibleRiskToSession) has no literal. It is a bool field
left at its Go zero value, and no non-test code assigns it. It is dropped from the list under
rule 2.

Ranking by irreversibility: 4, 3 and 8 come first. Each decides whether a risk-carrying brief
reaches a model-flippable lane. The flip itself is a README status edit that a revert can undo,
so even a wrong value here is recoverable. 5 is next. 1, 2, 6 and 7 are output text and rank last.

- RISK-VALUE: DERIVED — TierPolicy irreversible arm = loopengine.TierHuman @ tools/desk/cmd/verifyloop/tier.go:31 — the maintainer ruling of 2026-09-06 quoted in this brief's sources says FAIL SAFE, irreversible first, routes to ROUTE-HUMAN. Placing this arm ahead of the Flagged arm is what keeps the dormant flag (tier.go:42) from ever diverting irreversible work.
- RISK-VALUE: DERIVED — GateIsHuman first-token = "human" @ tools/desk/internal/loopengine/engine.go:71 — this brief's Task 1 specifies trim, lowercase, then the first whitespace-delimited token. The brief-v2 gate vocabulary is human or model, so "human" is the only value that names the human gate. The qualifier and re-cased forms are covered by row 2's gate-form dimension.
- RISK-VALUE: NAMED, NOT DERIVED — parseRisk matcher = (?i)<key>\s*:\s*yes @ tools/desk/cmd/verifyloop/frontmatter.go:61 — this matcher is demonstrably too broad. It matches a key-colon-yes sequence anywhere in the frontmatter, including prose. This brief is an example: its own why: text (frontmatter line 9) quotes a sibling brief's customer answer as yes. Direct probe of parseFrontmatter on this brief at 893cd6114b03: gate="model", risk Customer:true, any=true, reason "risk: customer". Declared risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}. A correct derivation needs the matcher anchored to the risk: key, either the inline flow map or the per-line entries under it. That is a change to a file outside this brief's scope, so it is not derived here. The error fails SAFE, over-routing to the human and never under-routing, so it does not reopen the leak this brief closed. It does misreport the routing reason.

**Routing note (gate mismatch).** The brief's frontmatter reads `gate: model` and
`risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`. verifyloop plan
buckets it awaiting-human / ROUTE-HUMAN with the reason `risk: customer`. The cause is the parseRisk
false positive named above: a prose match in the why: field, not a risk answer. This run takes the
fail-safe path: Evidence only, no status flip, and the board row stays `implemented`.

## Review

Gate: model (all four risk answers no — the brief tightens a routing gate and changes no
runtime authority). The reviewer confirms rows 2 and 5 are present and that row 5 can actually
fail: a table on which every case is risk-flagged proves the bucket works and says nothing
about whether the queue still has anything in it.
