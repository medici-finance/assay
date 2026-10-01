---
brief: assay:assay:graph-execution:10
title: Typed advice and separate deterministic policy records
why: Today bounded advice has no durable description of probability, calibration or applicability. Consumers need to distinguish an uncertain suggestion from an authorized policy result without breaking existing Decide callers.
wave: 0
depends: []
unblocks:
- graph-execution/11
- graph-execution/13
effort: M
gate: model
risk:
  regulatory: no
  customer: no
  irreversible: no
  sensitive-data: no
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'tools/desk/internal/deskkit/decide.go: fixed-here'
- 'tools/desk/internal/runnertable/decider.go: out-of-scope (tier separation remains unchanged)'
- 'tools/desk/internal/deskkit/admission.go: follow-up graph-execution/13'
version: 1
id: 2a0db879-6a28-4530-8f8a-c1b5c54ca540
---

# Brief 10 — Typed advice and separate deterministic policy records

## Context

files: `spec/decision-assessment-v1.md` (planned), `schemas/decision-assessment-v1.json` (planned), `tools/desk/internal/deskkit/decide.go`, `tools/desk/internal/deskkit/decide_test.go`, `tools/desk/internal/deskkit/decisionassessment.go` (planned), `tools/desk/internal/deskkit/decisionassessment_test.go` (planned), `tools/desk/internal/deskkit/decide.md`, `changelog/graph-execution-10-decision-contract.md` (planned)

facts: Advice has Answer and Justification; Decide already has conservative defaults, kill switch and non-default journal enforcement. The decider is not a worker tier.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Task

1. Define request, prediction and separate policy-result envelopes per GEA-04,09–11. Include subject, input/schema digests, label distribution, explicit abstention, provider/calibrator versions, actual backend, budget and evidence references. Preserve Advice callers through an explicit projection.
2. Reject unknown labels, NaN/infinity, invalid normalization with declared rounding tolerance, mismatched subject, stale input and inapplicable calibration. Uncalibrated labels may be shadow-only; do not synthesize confidence.
3. Keep no-advisor, disabled, timeout, malformed, out-of-budget and journal-failure paths conservative. Evidence explanations are untrusted; no tools or write credentials enter the provider contract.

## Interface contract

Provider envelope → Decide → journal/default behaves correctly with malformed and unjournalled advice.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessment" ./...` | exit 0; output includes PASS for TestDecisionAssessment, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentMalformedDefaults" ./...` | exit 0; output includes PASS for TestDecisionAssessmentMalformedDefaults, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentDecideJournal" ./...` | exit 0; output includes PASS for TestDecisionAssessmentDecideJournal, with no [no tests to run] for its owning package |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessment" ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentMalformedDefaults" ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentDecideJournal" ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier notes — VERIFY: BLOCKED — 0/3 hermetic pass, 3 could-not-check (environment), 0 fail — 2026-09-27 claude-opus-5-5-verifier

Runner is not the implementer (implementing change: PR #1499, merge commit 01548d87ad3b). Isolated detached worktree cut off origin/main at the merged head (HEAD == origin/main == 9585b4b6cc2e), offline envelope (KUBECONFIG=/dev/null), read-only. gate: model; all four risk answers no; not irreversible.

All three Verify rows are check:ci, and the hermetic witness above could not run them on this darwin host (the network-off sandbox needs Linux `unshare --net`). Each row was also run directly (non-hermetic, supporting only — it does not replace the owed Linux witness):

- Row 1 — direct run exit 0; the prefix regex selects all three of this brief's tests, each `--- PASS`; the owning package line is `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit` (no `[no tests to run]` for that package; other packages report it, as expected for a name filter).
- Row 2 — direct run exit 0; one `--- PASS` for the malformed-defaults test, 13 rejection subtests (unknown label, unknown shadow label, NaN, Inf, out-of-range, normalization beyond tolerance, mismatched subject, stale input digest, mismatched schema digest, calibrated-and-shadow overlap, abstained-with-probabilities, budget overrun, inapplicable calibration); owning package `ok`, not `[no tests to run]`.
- Row 3 — direct run exit 0; one `--- PASS` for the decide-journal flow test; it drives the production Question.Decide through PredictionAdvisor (not a hand-built record) for well-formed (advised, journalled), malformed (default, default-error), abstained (default, default-invalid) and journal-failure (advice discarded, default) paths; owning package `ok`.

Mutation probes (verifier-run, each a one-line edit to the production validator, restored byte-identical afterwards): normalization check disabled → row 2 fails on its normalization subtest; NaN/Inf check disabled → row 2 fails on its NaN subtest (NaN also slips the range and sum checks, so this guard is load-bearing); shadow/calibrated overlap check disabled → row 2 fails on its overlap subtest; calibrator-version check disabled → row 2 fails on its inapplicable-calibration subtest. Lower-boundary independence: with PredictionAdvisor's ValidatePrediction call bypassed, row 3's malformed subtest still gets the pre-declared default from Decide — it fails only on the outcome label (`default-invalid`, want `default-error`), proving Decide's own vocabulary check independently rejects the unknown label with the upper validator removed.

Deliverables present on main: spec/decision-assessment-v1.md, schemas/decision-assessment-v1.json, decisionassessment.go and its test, decide.md, and a comment-only reconciliation in decide.go (no behaviour change, consumer routed fixed-here). The changelog fragment was consumed into the v1.0.26 aggregate in CHANGELOG.md (the deskkit decision-assessment entry). runnertable/decider.go untouched (out-of-scope as routed); deskkit/admission.go does not yet exist on main (follow-up graph-execution/13, as routed).

Risk-bearing value enumeration (literals the diff introduces):

1. PredictionNormalizationTolerance = 1e-6 @ tools/desk/internal/deskkit/decisionassessment.go:68 (restated in spec/decision-assessment-v1.md:87)
2. probability range bounds 0 and 1 (`prob < 0 || prob > 1`) @ decisionassessment.go:165
3. normalization target 1 (`math.Abs(sum-1)`) @ decisionassessment.go:195
4. budget-overrun sentinel `p.Budget.Limit > 0` (0 = no self-declared limit, check skipped) @ decisionassessment.go:209
5. justification cap 280 (`truncate(just, 280)`) @ decisionassessment.go:258 — mirrors the existing journal cap at decide.go:450
6. schema budget.limit / budget.used `minimum: 0` @ schemas/decision-assessment-v1.json:39-40
7. authority binding: an abstained or shadow-only prediction projects to a zero Advice with a nil error (decisionassessment.go:299), which Decide resolves to OutcomeInvalid and the pre-declared default

Ranking: none is irreversible — every entry is fixed by an edit and a redeploy, the envelope is advisory only, and Decide's fail-closed default stands behind it. Highest consequence: 1 and 7 (they decide whether advice can steer a Decide answer), then 2/3 (probability definition), then 4/6 (budget, a reversible operational knob backed by the independent caller-side Consult.Budget), then 5 (cosmetic).

RISK-VALUE: DERIVED — PredictionNormalizationTolerance = 1e-6 @ tools/desk/internal/deskkit/decisionassessment.go:68 — float64 summation of k values in [0,1] accumulates at most about k·1.1e-16 rounding error, so for any realistic vocabulary 1e-6 sits at least six orders of magnitude above float error and never false-rejects an unrounded, correctly normalized distribution, while still refusing any real normalization defect (the test's 0.7 sum). Caveat, reversible and fail-closed: a provider that serializes probabilities rounded to a few decimal places can miss 1 by more than 1e-6 and would be refused to the conservative default, never admitted.

RISK-VALUE: DERIVED — probability bounds 0 and 1 @ tools/desk/internal/deskkit/decisionassessment.go:165, normalization target 1 @ decisionassessment.go:195 — the definition of a probability distribution; spec rules 4 and 7 state the same.

RISK-VALUE: DERIVED — abstention projection (zero Advice, nil error) @ tools/desk/internal/deskkit/decisionassessment.go:299 — the empty answer is never a vocabulary member (the verifier's lower-boundary probe above shows Decide's own vocabulary check alone returns the default), so an abstention can only ever resolve to the pre-declared default, never to a synthesized label; this matches the brief's "do not synthesize confidence".

Budget sentinel (4), schema minimums (6) and the 280 cap (5) are reversible operational knobs, ranked last, no derivation required.

Observations (non-blocking): (a) the JSON schema types labelProbabilities values as bare `number` without `minimum: 0` / `maximum: 1`, although that per-value range is expressible in JSON Schema; the Go validator and spec rule 4 enforce it. (b) Stale input is detected by digest mismatch only; GeneratedAt is not checked against any freshness window, and a request with an empty input or schema digest skips that comparison. Both match the spec as written. (c) The statusgen lint risk-files cross-read NOTICE names this brief: all four risk answers are no, yet a declared path sits under the security-path trigger for tools/desk/internal/deskkit/. The implementing diff there adds a read-only advisory envelope plus a comment in decide.go and does not change Decide's default, kill switch, budget or journal code; whether the risk answers stand is the gate owner's call, not this verifier's. (d) Lint also notes no Verify row runs the consumers check and graph-execution/13 does not yet reference this brief back; both are brief-authoring observations, not implementation defects.

VERIFY: BLOCKED — every row passes on a direct run and the mutation and lower-boundary probes bite, but all three check:ci rows still need the hermetic witness on a Linux runner with `unshare --net`; no implementation defect found.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessment" ./...` | pass exit=0 | sha256:c404d724d215 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentMalformedDefaults" ./...` | pass exit=0 | sha256:84a0e5a87684 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentDecideJournal" ./...` | pass exit=0 | sha256:c402bc22a8c5 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | Verify row 1: cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessment" ./... | exit 0; PASS for TestDecisionAssessment; owning package not [no tests to run] | exit 0; --- PASS for TestDecisionAssessment, TestDecisionAssessmentMalformedDefaults, TestDecisionAssessmentDecideJournal (17 subtests PASS); owning package line "ok github.com/medici-finance/assay/tools/desk/internal/deskkit"; [no tests to run] only on other packages (e.g. deskkit/untrustcorpus), as expected for a name filter | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | Verify row 2: cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentMalformedDefaults" ./... | exit 0; PASS for TestDecisionAssessmentMalformedDefaults; owning package not [no tests to run] | exit 0; --- PASS TestDecisionAssessmentMalformedDefaults with 13 rejection subtests PASS; owning package deskkit "ok". Mutation probes (below) bite | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | Verify row 3: cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestDecisionAssessmentDecideJournal" ./... | exit 0; PASS for TestDecisionAssessmentDecideJournal; owning package not [no tests to run] | exit 0; --- PASS TestDecisionAssessmentDecideJournal with 4 subtests PASS (well-formed advised+journalled, malformed to default-error, abstained to default-invalid, journal failure to default); owning package deskkit "ok". Test drives production Question.Decide through PredictionAdvisor | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — PredictionNormalizationTolerance = 1e-6 @ tools/desk/internal/deskkit/decisionassessment.go:68 (restated spec/decision-assessment-v1.md:87) — float64 summation of k probabilities in [0,1] accumulates rounding error of order k x 1.1e-16, so for any realistic label vocabulary 1e-6 is many orders of magnitude above float error (no false reject of a correctly normalized distribution) while any real normalization defect (the test's 0.7 sum) is refused; a provider that rounds to few decimals and misses by more than 1e-6 is refused to the conservative default, never admitted (fail-closed, reversible).
RISK-VALUE: DERIVED — probability bounds 0 and 1 (prob < 0 or prob > 1 refused) @ tools/desk/internal/deskkit/decisionassessment.go:165, normalization target 1 (math.Abs(sum-1)) @ decisionassessment.go:195 — the definition of a probability distribution; NaN/Inf is refused first at :161 because NaN compares false against both bounds and the sum check.
RISK-VALUE: DERIVED — abstention projection returns zero Advice with nil error @ tools/desk/internal/deskkit/decisionassessment.go:299 — the empty answer is never a vocabulary member, so Decide's own vocabulary check resolves it to OutcomeInvalid and the pre-declared default; no confidence is synthesized (brief task 2). The bypass probe below confirms Decide's check holds with the upper validator removed.

Notes:
- PASS. Rows 1-3 pass by hand on darwin (owning package reports ok, never no-tests-to-run) and in the Linux witness (dry and non-dry both exit 0). `statusgen brief --check-verified` with a hypothetical flip exits 0 with this witness appended and `--lint` prints LINT: PASS; without it the gate refuses on the 2026-09-27 could-not-run rows, so this table is what clears it. Five validator checks were disabled one at a time as mutation probes and each made a test fail. No RISK-VALUE is NAMED, NOT DERIVED.
- Ground first: before reading tests or the diff, the expected deliverables were written down from the brief (<scratch>/wv-ge-10/expectation.txt): typed request/prediction/policy-result envelopes with an explicit Advice projection; a validator refusing unknown labels, NaN/Inf, normalization beyond a declared tolerance, mismatched subject, stale input, inapplicable calibration; shadow-only uncalibrated labels; conservative Decide paths; spec, schema, decide.md and changelog. All present on main: spec/decision-assessment-v1.md, schemas/decision-assessment-v1.json, decisionassessment.go and its test, decide.md; the changelog fragment was consumed into the CHANGELOG.md aggregate (deskkit decision-assessment entry, line 606).
- Consumers: decide.go (fixed-here) gains a comment-only reconciliation, 7 added comment lines, no behaviour change; runnertable/decider.go untouched by the implementing change (out-of-scope as routed); deskkit/admission.go does not yet exist on main (follow-up graph-execution/13, as routed).
- Mutation probes (+mutation class, verifier-run on the throwaway clone, each a one-expression edit to the production validator, file restored byte-identical and confirmed by cmp): normalization check disabled, row 2 fails on its normalization subtest; NaN/Inf check disabled, fails on NaN subtest; subject check disabled, fails on mismatched-subject subtest; calibrator-version check disabled, fails on inapplicable-calibration subtest; unknown-label check disabled, row 2 fails on unknown-label AND row 3 fails only on the outcome label (default-invalid, want default-error) with the answer still the default.
- Lower-boundary independence (+flow): with PredictionAdvisor's ValidatePrediction call bypassed, row 3's malformed subtest still receives the pre-declared default from production Decide and fails only on the outcome label, proving Decide's vocabulary check independently rejects the unknown label with the upper validator removed.
- The known unshare-sandbox loopback issue did not arise: these tests start no local servers.
- Non-blocking lint NOTICEs naming this brief (brief-authoring observations, lint PASS): no Verify row runs statusgen --consumers; graph-execution/13 does not yet reference this brief back; a +dereference Verify-row obligation is owed but not declared; risk-files cross-read notes decide.go sits under the security-path trigger while all four risk answers are no. The decide.go change is comment-only, so the implementation does not touch Decide's default, kill switch, budget or journal code; whether the risk answers stand remains the gate owner's call.
- Carried observations (non-blocking, match the spec as written): the JSON schema types labelProbabilities values as bare number without minimum 0 / maximum 1 (the Go validator and spec rule 4 enforce the range); stale input is detected by digest mismatch only, and a request with an empty input or schema digest does not compare that digest.
- The previous (2026-09-27) verdict was BLOCKED solely for want of this Linux network-off witness; it is now supplied and passes on all three rows.

VERIFY: PASS

## Review

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
