# Agentic admission v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `tools/desk/internal/deskkit/admission.go`
(graph-execution/13)
**Machine-readable contract:** `schemas/agentic-assessment-v1.json`

## 1. Scope

This document specifies agentic admission: a pure, deterministic policy that decides how
much of an owner-admitted category of work an agent may do on one subject. It reads hard
facts and recorded probabilistic advice, and it adds gate nodes to the work's graph. It
implements GEA-09, GEA-10 and GEA-11 of the amendment in
`docs/streams/graph-execution/admission-assurance-spec.md`. Advice reaches it only as a
recorded `decision-assessment-v1` `Prediction` (`spec/decision-assessment-v1.md`).

Out of scope: activation. This document defines no dispatch wiring. Binding an admission
result at the dispatch boundary is a separate, separately gated change (graph-execution/14).
A conforming evaluator that nothing consults changes no behaviour.

The policy VALUES (category ceilings, fail ceilings, freshness bounds, applicability) are
an owner's ruling. This document fixes their shape and the rules any value set MUST obey.
The reference fixtures carry example values only.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## 2. Dispositions

The admission result is exactly one of five dispositions, from least to most restrictive:

| Disposition | Meaning | Permitted agent operations |
|---|---|---|
| `bounded-agent-work` | an agent works inside the category's permitted operations | the requested operations, plus `read` |
| `supervised-agent` | as above, with the more heavily gated graph | the requested operations, plus `read` |
| `human-led` | a human leads the work; an agent may read | `read` |
| `discovery-only` | no implementation by anyone; reads bounded to a separate grant's read scope | `read`, within `discoveryScope` |
| `blocked` | nothing proceeds | none |

These are admission dispositions. They do not replace graph risk classes, desk roles or
`AssayScore`.

**`discovery-only` stands only on a grant.** A result of `discovery-only` MUST carry the
grant's read-scope entries that cover the subject (`discoveryScope`), and discovery reads are
bounded to them. A path that would reach `discovery-only` without a grant covering the
subject — unknown readiness, a readiness fail ceiling of `discovery-only`, or an advice label
— is `blocked` instead (§4 step 12). No other disposition carries a `discoveryScope`.

The result MUST NOT carry a numeric safety, suitability or compliance score. It carries a
disposition, the ordered reason codes that produced it, the permitted operations, the
human floors and the risk-input verdict (§6).

**Human floors.** `merge`, `release` and `deploy` are never a permitted agent operation, at
any disposition. A policy whose category lists one is invalid (§5). An assessment that
requests one is capped at `human-led`.

## 3. Inputs

### 3.1 Hard facts (deterministic)

An assessment carries one `HardFact` per check. Each fact has a `check`, a `state` of `pass`,
`fail` or `could-not-check`, optional evidence references, and `observedAt`.

| Check | Class | On `fail` | On could-not-check |
|---|---|---|---|
| `authority` | authority | `blocked` | `blocked` |
| `data-handling` | authority | `blocked` | `blocked` |
| `acceptance-availability` | readiness | the policy's fail ceiling | hold (§4 step 9) |
| `executor-capability` | readiness | the policy's fail ceiling | hold |
| `independent-verifier` | readiness | the policy's fail ceiling | hold |
| `budget` | readiness | the policy's fail ceiling | hold |
| `effect-recoverability` | readiness | the policy's fail ceiling | hold |

Category opt-in, the remaining GEA-09 check, is NOT a supplied fact. It is read from the
policy's own category record, so an assessment cannot assert its own admission.

A fact MUST be treated as could-not-check when any of these holds:

- it is absent;
- it appears more than once;
- the subject revision, schema digest or environment digest has changed since it was observed;
- its `observedAt` is zero, in the future, or older than the policy's `maxFactAge`;
- its state is outside the vocabulary.

could-not-check MUST NOT be rounded up to `pass` or down to a `fail` that was not observed.

### 3.1.1 Input trust

The evaluator trusts the facts it is given; it cannot detect a fabricated `pass`. A
deployment MUST take the assessment from a producer that is independent of the executor
whose work is being admitted, and MUST authenticate that producer where the result is bound
to dispatch (graph-execution/14). An executor that writes its own assessment admits itself.

### 3.2 Advice (probabilistic)

An assessment MAY carry recorded advice. Each record holds:

- a `dimension`: `ambiguity`, `verification-adequacy`, `task-model-fit` or `semantic-risk`;
- the `scope` it was produced for;
- the `AssessmentRequest` it answered, whose vocabulary is the disposition set;
- the `Prediction`;
- `observedAt`.

The evaluator MUST NOT call a provider.

Only the top calibrated label of each record is read; the rest of the distribution never
moves the result.

Advice MUST be ignored, with an `advice-ignored:<dimension>:<why>` reason, when any of these
holds. An unknown dimension is reported as `advice-ignored:unknown-dimension`: a value
outside the vocabulary is never echoed into a reason (§7).

| Condition | `<why>` |
|---|---|
| the advice kill switch is set | `disabled` |
| the dimension is unknown | `unknown-dimension` |
| the subject, schema or environment binding changed | `binding-changed` |
| the scope or request subject is not the assessed subject | `out-of-scope` |
| the vocabulary names a non-disposition | `vocabulary` |
| `ValidatePrediction` refuses the prediction | `malformed` |
| `observedAt` is zero, in the future, or older than `maxAdviceAge` | `stale` |
| the prediction yields no calibrated answer: abstained, shadow-only, or no applicable calibration | `uncalibrated` |

## 4. Evaluation

Evaluation MUST be total, deterministic and fail-closed. The order is normative.

1. An invalid policy (§5) → `blocked` (`policy-invalid`).
2. A set stop flag → `blocked` (`stop-flag`).
3. **Input shape.** Each of these is `blocked`:
   - an empty subject (`subject-missing`);
   - a subject over 256 characters or containing whitespace or a control character
     (`subject-invalid`);
   - more than 16 operations, 32 facts or 16 advice records (`input-oversized`);
   - a zero clock (`clock-missing`);
   - an empty subject revision, schema digest or environment digest on either side
     (`binding-missing:<binding>`). Two empty values are not an unchanged binding.
4. **Applicability.** A subject outside the policy's applicability prefixes is `blocked`
   (`not-applicable`), unless an owner-named exception covers it.
   - A prefix covers a subject only at a boundary: the subject equals it, the prefix ends
     in `#` or `/`, or the subject continues with `#` or `/`. `example-org/widgets` covers
     `example-org/widgets#1`, never `example-org/widgets-evil#1`.
   - A live exception adds `exception-applied:<owner>`.
   - An expired exception is `blocked` (`exception-expired:<owner>`).
   - An exception extends applicability ONLY. It MUST NOT waive a hard check, lift a
     category ceiling or remove a gate.
5. **Category.** An absent or revoked category → `blocked`. Otherwise the result is capped
   at the category's owner-declared ceiling.
6. **Binding.** A changed subject revision, schema digest or environment digest is
   recorded as a reason. It makes every fact could-not-check and every piece of advice
   ignored.
7. **Authority checks.** Any non-`pass` → `blocked`. Without authority or data-handling
   permission, not even discovery may run.
8. **Failed readiness checks.** A failed readiness check caps at the policy's declared
   fail ceiling for that check.
9. **Unknown readiness.** Any could-not-check readiness fact holds implementation.
   - The result is `discovery-only` when a separately authorized discovery grant is
     supplied whose read scope covers the subject (`discovery-scope:<owner>`). Coverage
     uses the same boundary rule as applicability.
   - Otherwise it is `blocked`: `discovery-not-authorized` with no grant,
     `discovery-grant-invalid` when the owner is not a reason-code token or the read scope
     is empty or has a blank entry, `discovery-out-of-scope` when no entry covers the
     subject.
   - Readiness that is not known never reaches an agent lane.
10. **Operations.**
    - No requested operation → `blocked` (`operations-undeclared`).
    - A human-floor operation → `human-led` (`human-floor:<op>`).
    - An operation outside both the agent and human-floor vocabularies → `human-led`
      (`operation-unknown`, a fixed code: the requested string is never echoed).
    - An operation the category does not permit → `human-led`
      (`operation-not-permitted:<op>`).
11. **Advice.** Each applicable advice record's top calibrated label MAY restrict the
    result to that label (`advice-restricted:<dimension>:<label>`). It MUST NOT widen it.
    The fold is `result = moreRestrictive(result, label)`.
12. **Discovery grant.** If the result is now `discovery-only`, by whichever step, it is
    re-checked against the grant as in step 9. With no grant covering the subject it
    becomes `blocked` with that step's reason. Otherwise it carries `discovery-scope:<owner>`
    and the covering entries as `discoveryScope`.

Steps 7–10 are why **all-hard-pass is necessary, not sufficient**: a subject whose every
fact passes is still bounded by its category's ceiling and permitted operations.

### 4.1 Advice can only restrict

For every policy, assessment and context, and for any advice set A:

    rank(evaluate(with A)) >= rank(evaluate(with no advice))

and the permitted operations with A are a subset of those without it. A label of any
confidence, on any dimension, can only move the result toward `blocked`.

The facts-only result IS the default. Absent, disabled, stale, malformed or uncalibrated
advice leaves it standing. Advice is never a precondition that its own presence then lifts.

## 5. Policy validity

An `AdmissionPolicy` MUST be refused (and evaluation MUST return `blocked`) when any of the
following holds:

- **Identity and applicability.**
  - Its version, owner or applicability list is empty.
  - An applicability entry is empty; an empty prefix matches every subject.
- **Categories.**
  - It admits no category.
  - A category has no owner, or a ceiling outside the vocabulary.
  - A category permits a human-floor operation or an unknown operation.
- **Fail ceilings.**
  - A readiness check has no fail ceiling.
  - A readiness check's fail ceiling is an agent lane (`bounded-agent-work` or
    `supervised-agent`). A failed hard check MUST cap at `human-led` or more restrictive.
  - A fail ceiling names a non-readiness check. Authority checks always fail to `blocked`
    and are not configurable.
- **Freshness.** `maxFactAge` or `maxAdviceAge` is not positive.
- **Exceptions.** An exception lacks a subject, an owner or an expiry, or its owner is
  not a reason-code token (§7).

## 6. Risk-input mapping and mandatory gates

Each disposition maps EXPLICITLY to a workflow-pattern risk-input verdict
(`spec/workflow-pattern-v1.md` §5):

| Disposition | Risk-input verdict |
|---|---|
| `bounded-agent-work` | `standard` |
| `supervised-agent` | `elevated` |
| `human-led` | `human` |
| `discovery-only` | `human` |
| `blocked` | `human` |

The work's mandatory gates are the **union** of two sets:

- the gates the brief's own risk verdict names in the pattern's `risk-input`;
- the gates the disposition's mapped verdict names.

Some notes on the union:

- It MUST NOT subtract a gate. A high score cannot delete a required node.
- It is a union, not "take the stricter verdict". Where two verdicts' gate lists are not
  nested, both lists' gates are mandatory.
- An unmapped verdict on either side, the brief's or the disposition's, is a refusal
  (hold), never an empty gate set.
- Gates are returned for every disposition, `blocked` included. The caller MUST check the
  disposition first: a `blocked` result proceeds to no gate at all.
- A workflow-pattern file over 1 MiB is refused before it is parsed.
- A pattern whose `risk-input` omits a verdict, or names a gate that is not a node, MUST be
  refused at this seam, independently of the pattern lint.

## 7. Projection to `PolicyResult`

An admission result projects onto the `decision-assessment-v1` `PolicyResult` record:

- `decision` is the disposition;
- `reason` is the ordered reason codes, joined by `; `;
- `policyVersion` is the policy's version;
- `subject` is the assessed subject only when it passed the subject grammar (§4 step 3),
  and empty otherwise.

The projection carries no probability.

**Subject rule.** The admission result's `subject`, and the projection's, MUST be either a
subject that passed the subject grammar or empty. This holds on every return, including
the `policy-invalid` and `stop-flag` returns that precede the subject check, so a refused
subject is never echoed into the decision record.

**Reason-code grammar.** Every reason code matches
`^[a-z][a-z0-9-]*(:[A-Za-z0-9][A-Za-z0-9._@/-]{0,63})*$`: a fixed lowercase literal, then
`:`-separated tokens. A token is either drawn from a closed vocabulary (a check, a
dimension, a disposition, an agent operation) or is an owner that §5 or the grant check
has already held to the token grammar. A value outside a vocabulary is reported by a
fixed code and never echoed. So no assessed string can add a `; ` separator, a line, or a
code to `reason`.

## 8. Conformance

The reference implementation's tests (`tools/desk/internal/deskkit/admission_test.go`)
are the conformance suite.

- `TestAgenticAdmissionFixtures` covers the fixture set under `testdata/admission/`. It
  includes these negative paths:
  - high-confidence unsafe advice;
  - a missing verifier;
  - a revoked category;
  - a schema change and an environment change;
  - an expired exception;
  - no data authority;
  - all-hard-pass but policy-disallowed work;
  - a changed subject revision; a duplicated, future-dated or zero-time fact;
  - a subject outside applicability, including a raw-prefix neighbour;
  - no requested operation;
  - every path to `discovery-only` without a grant covering the subject.
- `TestAgenticAdmissionPolicyValidation` covers §5.
- `TestAgenticAdmissionMalformedInput` covers the step 3 refusals and the inputs the
  schema already refuses: empty bindings, malformed grants, out-of-vocabulary states,
  operations and dimensions, and oversized input.
- `TestAgenticAdmissionScopeBoundary` covers the boundary rule of step 4.
- `TestAgenticAdmissionGateSeamBounds` covers the §6 seam refusals.
- Every result in every test is checked against the reason-code grammar of §7 and the
  `discoveryScope` rule of §2.
- `TestAgenticAdmissionConfidenceCannotAuthorize` covers §4.1 across every fixture, every
  label, every confidence level and every dimension set.
- `TestAgenticAdmissionRiskInputUnion` covers §6 against the shipped workflow patterns.

The mutation map `admission-mutations.json` records the mutants the suite kills.
