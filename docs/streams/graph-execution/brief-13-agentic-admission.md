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

Default if no answer: nothing activates. The evaluator stays unconsulted, and no admission policy is adopted.

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

## Review

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
