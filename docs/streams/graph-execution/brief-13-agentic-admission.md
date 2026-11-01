---
brief: assay:assay:graph-execution:13
title: Deterministic admission over facts and bounded probabilistic advice
why: A task may look easy to a model while lacking a verifier, permission or safe recovery path. Admission must explain these differences and prevent confidence from bypassing a mandatory control.
wave: 1
depends:
- graph-execution/10
unblocks:
- graph-execution/14
effort: M
gate: human
risk:
  regulatory: yes
  customer: no
  irreversible: no
  sensitive-data: no
design: DR-graph-exec-13
gate-why: The owner confirms the proposed admission policy preserves mandatory controls and existing human authority.
decision-trigger: spec
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
- 'tools/desk/internal/deskkit: fixed-here'
- 'tools/desk/internal/loopengine: follow-up graph-execution/14'
- 'statusgen/assayscore.go: out-of-scope (formula and history unchanged)'
version: 1
id: 31e146c5-2294-437b-9314-55d1dbef8558
---

# Brief 13 — Deterministic admission over facts and bounded probabilistic advice

## Context

files: `spec/agentic-admission-v1.md` (planned), `schemas/agentic-assessment-v1.json` (planned), `tools/desk/internal/deskkit/admission.go` (planned), `tools/desk/internal/deskkit/admission_test.go` (planned), `tools/desk/internal/deskkit/testdata/admission/` (planned), `docs/enforcement-model.md`, `changelog/graph-execution-13-agentic-admission.md` (planned)

facts: Pattern risk-input already maps low/standard/elevated/human to mandatory gates. Suitability dispositions are a different dimension; AssayScore keeps its existing formula.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Human decision

Decision-trigger: spec. What is being decided is the admission policy values the owner adopts before anything consults the evaluator. The schema, the validator and the tests ship either way. Nothing is activated by this decision.

Three values are open:

- **Failed readiness checks.** Each one caps at human-led, except a missing acceptance definition, which caps at discovery-only. Validation already refuses any agent lane here.
- **Disposition to risk-input mapping.** Bounded maps to standard, supervised to elevated, and everything else to human. Gates are always unioned with the brief's own verdict.
- **Absent or uncalibrated advice.** It changes nothing: the facts-only result stands, so the owner's category ceiling is the only lane-opener.

Why it matters: these values decide how much an agent may do without a human, and only the owner can rule on them.

Options:

1. Adopt the three values above as the starting policy, with every category's ceiling set by its owner when that category is admitted. (Recommended.)
2. A stricter start: cap every category at supervised-agent until a bounded lane has been separately approved.
3. Rework: name the values to change, and they come back as a revised policy before any activation.

Default if no answer: none — blocks until answered.

Nothing activates. The evaluator stays unconsulted, and no admission policy is adopted.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Task

1. Implement pure policy evaluation with GEA-09/10 hard predicates, advisory dimensions, freshness and explicit reasons. Define allowed dispositions and an explicit mapping to risk-input preserving all mandatory gates. No model call is needed by the evaluator.
2. Unknown hard readiness holds implementation; discovery requires its own scope. Advice can only restrict an already owner-admitted category. Preserve human merge/release floors, stop flags, data policy and defaults when advice is absent or uncalibrated.
3. Add fixtures for high-confidence unsafe advice, missing verifier, revoked category, schema/environment change, expired exception, no data authority and all-hard-pass but policy-disallowed work. Document applicability and exception owner/expiry; never produce a single safety/compliance score.

## Interface contract

Facts + recorded advice → deterministic policy → mandatory graph gates; a high score cannot delete a required node.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmission" ./...` | exit 0; output includes PASS for TestAgenticAdmission, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionConfidenceCannotAuthorize" ./...` | exit 0; output includes PASS for TestAgenticAdmissionConfidenceCannotAuthorize, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionRiskInputUnion" ./...` | exit 0; output includes PASS for TestAgenticAdmissionRiskInputUnion, with no [no tests to run] for its owning package |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmission" ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 75fc02522bd9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionConfidenceCannotAuthorize" ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 75fc02522bd9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionRiskInputUnion" ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ 75fc02522bd9 (on-behalf-of human:ian) (forge-identity) |

### Verification — graph-execution/13 — first independent pass, 2026-10-02

**VERIFY: BLOCKED (witness owed) — rows 1 to 3 pass by hand at 75fc02522bd9 (contains the implementation, #2014), but all three are `check:ci` rows and no network-off witness has run, so this is not a pass verdict. Evidence only — the brief is `gate: human`, `regulatory: yes`; no status change is made here and no model signs it off.** Run by a non-implementer in an isolated worktree, with an isolated home directory for every Go command.

| # | Command | Exit | Observed | Verdict | Runner |
|---|---------|------|----------|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmission" ./...` | 0 | PASS for all 8 matched top-level tests (Fixtures, PolicyValidation, ConfidenceCannotAuthorize, RiskInputUnion, MalformedInput, ScopeBoundary, GateSeamBounds, SubjectNotEchoed); 69 subtest PASS lines, 0 FAIL; the owning package (internal/deskkit) prints `ok` with no no-tests marker | match | claude-sonnet-5-5-verifier @ 75fc02522bd9 (on-behalf-of human:ian) |
| 2 | same, `-run "^TestAgenticAdmissionConfidenceCannotAuthorize"` | 0 | `--- PASS: TestAgenticAdmissionConfidenceCannotAuthorize`; owning package `ok`, no no-tests marker | match | claude-sonnet-5-5-verifier @ 75fc02522bd9 (on-behalf-of human:ian) |
| 3 | same, `-run "^TestAgenticAdmissionRiskInputUnion"` | 0 | `--- PASS: TestAgenticAdmissionRiskInputUnion`; owning package `ok`, no no-tests marker | match | claude-sonnet-5-5-verifier @ 75fc02522bd9 (on-behalf-of human:ian) |

**Mutation check (row 2).** Two production mutations were applied in turn to the advice fold in tools/desk/internal/deskkit/admission.go and then reverted: (a) let an advice label replace the cap unconditionally; (b) let it replace the cap only when its probability is at least 0.95. Row 2 went red both times (exit 1, admission_test.go:349, "widened \"supervised-agent\" to \"bounded-agent-work\""). After the revert the tree was clean and row 2 was green again. Both mutants were caught at the first fixture that tripped them; the missing-verifier fixture alone was not isolated.

**Flow seam (row 3).** The test loads the shipped workflow-pattern tables through the production loader and calls the production gate mapping and evaluator for every brief verdict, disposition and fixture; for the shipped pattern the expected gate sets come from the shipped tables, not a hand-built expected document. The union case is the exception: it loads a test-only pattern with non-nested gate lists and compares the result against two literal gate lists written in the test (admission_test.go:467-477). Defective tables and an unmapped verdict are refused at the seam. No production caller of the evaluator or the gate mapping exists outside admission.go and its tests at this commit — wiring is graph-execution/14's scope, so the flow beyond loader and shipped table has no execution evidence yet.

**Risk-bearing values (regulatory: yes)**

| Value | Where | Basis |
|-------|-------|-------|
| disposition → risk input map (standard / elevated / human) | admission.go:126-132 | derived: the brief's Human decision text and the owner decision recorded in DR-graph-exec-13 |
| human-floor operations (merge, release, deploy) | admission.go:416 | derived: the brief's "preserve human merge/release floors" |
| readiness-fail ceiling may not sit below human-led | admission.go:474 | derived: the brief's "failed readiness checks cap at human-led" |
| input bounds 256 / 16 / 32 / 16 and a 1 MiB pattern cap | admission.go:332-336 | named, not derived: the spec and schema state the same numbers without a rationale; they fail closed |
| reason-token grammar, 64-character cap | admission.go:343 | named, not derived |
| fact and advice age of 24h | testdata/admission/policy.json:59-60 | fixture example only; the code requires only a positive value |

No confidence threshold exists in the code: no probability cutoff literal appears in the change.

**Execution witness.** The witness table above records all three rows as could-not-run: they are `check:ci` rows and the runner's network-off sandbox is a Linux facility, absent on this darwin host. A Linux witness is still owed before any stamp.

**Remaining for the human gate:** the sign-off itself; the real per-category policy values (ceilings, readiness-fail ceilings, fact and advice age) when a category is admitted; acceptance of, or a rationale for, the two named-not-derived bounds.

### Verification — graph-execution/13 — container-netns re-run of rows 1-3, 2026-10-30

**VERIFY: BLOCKED** — rows 1 to 3 pass network-off in a closed bracket at 0006683ef1c8, but no fail-first witness can be produced on the installed v1.0.67, and all three rows are risk-bearing under `regulatory: yes`. Evidence only: the brief is `gate: human`, no status changes here and no model signs it off. Blocker and test-strength finding: #5048. This supersedes the could-not-run table and the blocked marker above for the network-off witness only.

Envelope: runner image ghcr.io/medici-finance/verify-runner:v1.0.67 at the digest in the witness (amd64 under an arm64 host), `--network none`, read-only root, tmpfs /tmp, read-only source mount, `GOPROXY=off`, statusgen v1.0.67, go1.27.3 linux/amd64. Verifier: claude-opus-5-5, tier strong, a non-implementer.

Witness (closed bracket, `verdict=match`):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmission" ./...` | pass exit=0 sandbox=container-netns | sha256:4ebca3cc5c90 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionConfidenceCannotAuthorize" ./...` | pass exit=0 sandbox=container-netns | sha256:ec1910e8a605 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionRiskInputUnion" ./...` | pass exit=0 sandbox=container-netns | sha256:f05ed626d5eb | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:43331633ff79d183de9bb98544ed3f7f2af71a445a0b56b4bfb723059587daa8
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5040 run=c558c5a1c7063d103f3f034186b38f018f85d5f4beb397e75c663d888bcfd5ce source=0006683ef1c8dc7341d0c2e67a4cb6b566e7814e brief=docs/streams/graph-execution/brief-13-agentic-admission.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=c558c5a1c7063d103f3f034186b38f018f85d5f4beb397e75c663d888bcfd5ce source=0006683ef1c8dc7341d0c2e67a4cb6b566e7814e tree=sha256:43331633ff79d183de9bb98544ed3f7f2af71a445a0b56b4bfb723059587daa8 witness=sha256:e6d03e9b07c21b5995f11354dff14010e758a5b6df448f31bca10bc0e0d843a7 opened=2026-10-30T22:00:55Z closed=2026-10-30T22:13:21Z before=receipt after=check-verifier

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmission" ./...` | exit 0; PASS for TestAgenticAdmission tests; owning package has no no-tests marker | exit 0 in the witness above. Reading aid, supplemental copy of the same tree: 8 top-level PASS, 77 PASS lines in all, 0 FAIL, internal/deskkit prints `ok` | 2026-10-30 | claude-opus-5-5 verifier |
| 2 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionConfidenceCannotAuthorize" ./...` | exit 0; PASS for the named test; owning package has no no-tests marker | exit 0 in the witness above. Supplemental copy: named test PASS, internal/deskkit prints `ok` | 2026-10-30 | claude-opus-5-5 verifier |
| 3 | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestAgenticAdmissionRiskInputUnion" ./...` | exit 0; PASS for the named test; owning package has no no-tests marker | exit 0 in the witness above. Supplemental copy: named test PASS, internal/deskkit prints `ok` | 2026-10-30 | claude-opus-5-5 verifier |

The witness records exit codes and output hashes, not the PASS lines. The `Observed` details come from a separate supplemental run on a scratch copy of the source in the same envelope. They read the Expect column and carry no acceptance credit.

**Uncovered obligation: fail-first (rows 1, 2 and 3).** On v1.0.67 the host prints `flag provided but not defined: -history-pack`. The container, given the base 1f41825862a8 (parent of the delivering commit 1c437b26d45e), prints `statusgen verifyrun: --fail-first: --base "1f41825862a8551837e024a15902eabdbfa08e4e" names no commit in this clone`. That attempt wrote no witness and is not credited. v1.0.68 carries the flag. Needed: host tools and a qualified runner image at v1.0.68, then a fresh dispatch. Next actor: the desk (#5048).

**Supplemental probes, no acceptance credit.** All ran on scratch copies in the same envelope, with the selector narrowed to internal/deskkit.

| Probe | Selector | Result | Date | Runner |
|-------|----------|--------|------|--------|
| advice replaces the cap | row 2 | exit 1, admission_test.go:349 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| advice widens at p of 0.95 or more | row 2 | exit 1, admission_test.go:349 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| union drops brief gates | row 3 | exit 1, admission_test.go:418 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| union drops disposition gates | row 3 | exit 1, admission_test.go:419 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| readiness ceiling floor removed | row 1 | exit 1, admission_test.go:285 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| human floor loses deploy | row 1 | exit 1, admission_test.go:161 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| map: bounded to low | rows 1 and 3 | exit 1, admission_test.go:470 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| map: supervised to low | rows 1 and 3 | exit 1, admission_test.go:477 | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| map: blocked to low | row 3 | **exit 0, survives** | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| map: blocked to low | row 1 | exit 1, admission_test.go:696, a side effect of a partial fixture | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| map: human-led to low | rows 1 and 3 | **exit 0, survives** | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |
| map: discovery-only to low | row 1 | **exit 0, survives** | 2026-10-30 | claude-opus-5-5 verifier @ 0006683ef1c8 |

**Finding.** The human-led, discovery-only and blocked map values are not pinned as values. admission_test.go lines 194 and 419 compare against `DispositionRiskInput(d)`, the function under test. Production holds the ruled value, so this is a missing pin, not a wrong value, and the evaluator has no production caller yet. It is not recorded as a failed row: every row's Expect holds, and "non-discriminating" in docs/verify-row-strength.md means green at base and at head, which was not observed. The human gate should weigh it (#5048; it needs its own issue, because this session's filing budget ran out).

**Base run disclosure.** A hand run of the rows at the base exited 1 with `[setup failed]`: `open /go/pkg/mod/cache/download/github.com/go-git/go-git/v5/@v/v5.19.2.lock: read-only file system`. That is the envelope, not the change, so it is no evidence of discrimination. A v1.0.68 fail-first run could report the base `unproven` for the same reason unless the base's modules resolve offline.

**Risk-bearing values (`regulatory: yes`)**, ranked. All are in tools/desk/internal/deskkit/admission.go unless named otherwise, and none changed since 1c437b26d45e. All are reversible by an edit while the evaluator is inactive; the ranking is by what a wrong value would permit once it is consulted.

- RISK-VALUE: DERIVED — dispositionRiskInput[AdmitHumanLed] = "human" @ admission.go:129 — the brief's Human decision says everything other than bounded and supervised maps to human; DR-graph-exec-13 records the same. Not pinned by a literal test.
- RISK-VALUE: DERIVED — dispositionRiskInput[AdmitDiscoveryOnly] = "human" @ admission.go:130 — same derivation. Not pinned by a literal test.
- RISK-VALUE: DERIVED — dispositionRiskInput[AdmitBlocked] = "human" @ admission.go:131 — same derivation. Not pinned by a literal test.
- RISK-VALUE: DERIVED — DispositionRiskInput fallback = "human" @ admission.go:140 — an unknown disposition takes the most gated verdict, which fails closed and matches "everything else".
- RISK-VALUE: DERIVED — dispositionRiskInput[AdmitBoundedAgentWork] = "standard" @ admission.go:127 — the brief's Human decision and DR-graph-exec-13.
- RISK-VALUE: DERIVED — dispositionRiskInput[AdmitSupervisedAgent] = "elevated" @ admission.go:128 — the brief's Human decision and DR-graph-exec-13.
- RISK-VALUE: DERIVED — humanFloorOperations = {"merge","release","deploy"} @ admission.go:416 — the brief's "preserve human merge/release floors"; DR-graph-exec-13 states all three are never an agent operation.
- RISK-VALUE: DERIVED — readiness ceiling floor, ceil.rank() < AdmitHumanLed.rank() @ admission.go:474 — the brief's "each one caps at human-led", with validation refusing any agent lane.
- RISK-VALUE: DERIVED — riskVerdicts = {"low","standard","elevated","human"} @ admission.go:844 — the verdict vocabulary the shipped workflow patterns are keyed on.
- RISK-VALUE: NAMED, NOT DERIVED — admissionMaxSubject = 256 @ admission.go:332 — spec and schema state it with no rationale; it fails closed.
- RISK-VALUE: NAMED, NOT DERIVED — admissionMaxOperations = 16 @ admission.go:333 — as above.
- RISK-VALUE: NAMED, NOT DERIVED — admissionMaxFacts = 32 @ admission.go:334 — as above.
- RISK-VALUE: NAMED, NOT DERIVED — admissionMaxAdvice = 16 @ admission.go:335 — as above.
- RISK-VALUE: NAMED, NOT DERIVED — admissionMaxPatternLen = 1<<20 @ admission.go:336 — as above.
- RISK-VALUE: NAMED, NOT DERIVED — reasonTokenRE 64-character cap = {0,63} @ admission.go:343 — no stated rationale.
- RISK-VALUE: NAMED, NOT DERIVED — maxFactAge and maxAdviceAge = "24h0m0s" @ testdata/admission/policy.json:59 — a fixture example; the code requires only a positive value, and the owner has not adopted real values.

No confidence threshold literal exists in the evaluator.

**Remaining for the human gate:** the decision on #2034, which is still open; the owner's policy values; a view on the unpinned map entries and the named-not-derived bounds. The owed fail-first witness comes first.

## Review

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
