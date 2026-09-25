# Decision-assessment v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `tools/desk/internal/deskkit/decisionassessment.go`
(graph-execution/10)

## 1. Scope

This document specifies the `decision-assessment-v1` envelope: a typed, calibrated
alternative to the bounded enum consult `deskkit.Decide` (`decide.md`) already implements,
plus the deterministic policy record that is kept strictly separate from it. It extends the
amendment in `docs/streams/graph-execution/admission-assurance-spec.md` §3 (GEA-04 through
GEA-08); it does not commission a runtime, a trained model, or a Laya adapter — those are
separately reviewed follow-ups (graph-execution/11, /12, /13).

This document does not change `deskkit.Decide`'s existing contract (`decide.md`). A
provider that answers in this richer shape plugs into that same bounded `Consult` flow
through an explicit projection; the enum-vocabulary contract, its fail-closed default, its
budget/timeout/journal machinery and its reserved-verb deny-list are unchanged.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## 2. Three products, never merged

A conforming implementation MUST keep these three values distinct — never averaging,
thresholding, or otherwise deriving one from another's numbers:

| Product | Nature | Purpose |
|---|---|---|
| `AssessmentRequest` | declared envelope | states the subject, its input/schema digests, the closed label vocabulary, and (optionally) the calibrator version required |
| `Prediction` | PROBABILISTIC advice | a calibrated label distribution, or an explicit abstention; never itself an authorization (GEA-07) |
| `PolicyResult` | DETERMINISTIC record | a policy's own disposition over a subject; never a probability, and never computed from a `Prediction`'s numbers inside the assessment layer itself |

This is the same separation GEA-11 states at the admission layer (`AssayScore` /
`AgenticAssessment` / `ControlAssurance`); this document is its typed-envelope precursor.

## 3. `AssessmentRequest`

| Field | Type | Requirement |
|---|---|---|
| `subject` | string | REQUIRED. Stable identity of what is being assessed. |
| `inputDigest` | string | OPTIONAL. A digest of the exact input content a `Prediction` must match. Empty = no input-digest check declared. |
| `schemaDigest` | string | OPTIONAL. A digest of the question/schema shape. Empty = no schema-digest check declared. |
| `vocabulary` | array of string | REQUIRED. The closed set of labels a `Prediction` may name, in either `labelProbabilities` or `shadowLabels`. |
| `requiredCalibratorVersion` | string | OPTIONAL. Empty = no calibration-version check declared. |

The request never carries the input's raw content, matching `Consult.Context`'s
digest-only journalling posture (`decide.md`).

## 4. `Prediction`

| Field | Type | Requirement |
|---|---|---|
| `subject` | string | REQUIRED. MUST equal the request's `subject`. |
| `inputDigest` | string | REQUIRED when the request declares one. MUST equal the request's `inputDigest`. |
| `schemaDigest` | string | REQUIRED when the request declares one. MUST equal the request's `schemaDigest`. |
| `labelProbabilities` | map<string, number> | Labels the provider's calibrator actually covers. Every key MUST be in `vocabulary`. Every value MUST be a finite number in `[0, 1]`. |
| `shadowLabels` | array of string | Labels the provider produced WITHOUT calibration support. Every entry MUST be in `vocabulary` and MUST NOT also be a key of `labelProbabilities` — an uncalibrated label MUST NEVER carry a synthesized probability. |
| `abstained` | boolean | An EXPLICIT "no usable prediction". MUST be paired with an empty `labelProbabilities`. |
| `providerVersion` | string | OPTIONAL. |
| `calibratorVersion` | string | OPTIONAL. Checked against the request's `requiredCalibratorVersion` when declared. |
| `requestedBackend` / `actualBackend` | string | OPTIONAL. The requested and the actually-used execution backend (GEA-06: an unapproved device fallback is a distinct fact from an approved one). |
| `budget` | `{limit, used}` | OPTIONAL. The provider's OWN report of budget consumed, independent of the caller-side `Consult.Budget`. |
| `evidenceRefs` | array of string | OPTIONAL. References to evidence, never raw evidence content. |
| `generatedAt` | timestamp | OPTIONAL. |

### 4.1 Normative rules (MUST)

1. `subject` MUST equal the request's `subject`; a mismatch is refused, never coerced.
2. When the request declares `inputDigest` or `schemaDigest`, the prediction's own value
   MUST equal it. A mismatch is treated as stale or wrong-subject input.
3. Every key of `labelProbabilities` and every entry of `shadowLabels` MUST be a member of
   the request's `vocabulary`. An unknown label is refused.
4. Every value of `labelProbabilities` MUST be a finite number in `[0, 1]` — NaN, ±Inf, and
   out-of-range values are refused.
5. A label MUST NOT appear in both `labelProbabilities` and `shadowLabels`. Uncalibrated
   labels MAY be reported shadow-only; they MUST NOT carry a synthesized probability.
6. When `abstained` is true, `labelProbabilities` MUST be empty.
7. When `abstained` is false and `labelProbabilities` is non-empty, its values MUST sum to
   `1` within the DECLARED rounding tolerance (the reference implementation's
   `PredictionNormalizationTolerance`, `1e-6`).
8. When the request declares `requiredCalibratorVersion` and `labelProbabilities` is
   non-empty, `calibratorVersion` MUST equal it — otherwise the calibration is inapplicable
   and the prediction is refused (GEA-08: a model/precision/schema/domain change
   invalidates calibration unless equivalence is proved).
9. When `budget.limit` is greater than zero, `budget.used` MUST NOT exceed it.

A `Prediction` failing any rule above is MALFORMED: a conforming implementation MUST refuse
it (report the violation) rather than accept, coerce, or silently repair it. `nil`/absence of
a violation is not itself a claim that the prediction's calibration is accurate — evaluating
calibration quality is a separate concern (graph-execution/12).

## 5. `PolicyResult`

| Field | Type | Requirement |
|---|---|---|
| `subject` | string | REQUIRED. |
| `inputDigest` | string | OPTIONAL. |
| `decision` | string | REQUIRED. A policy's own closed vocabulary; not `decision-assessment-v1`'s concern to enumerate (graph-execution/13 defines the admission disposition vocabulary this record carries). |
| `policyVersion` | string | REQUIRED. |
| `reason` | string | REQUIRED. |
| `generatedAt` | timestamp | OPTIONAL. |

A `PolicyResult` MUST NOT be computed by averaging, thresholding, or otherwise deriving a
value from a `Prediction`'s `labelProbabilities` inside the assessment layer. A policy that
consumes a `Prediction` as one of several inputs to its own separately-authored decision
logic is graph-execution/13's scope, not this document's.

## 6. Interaction with `deskkit.Decide`

A `Prediction`-based provider projects into the existing `Advice`/`Consult`/`Decide`
contract (`decide.md`) through `PredictionAdvisor`, never through a change to `Decide`
itself:

1. `PredictionAdvisor.Predict` returns a `Prediction`.
2. It is validated against the fixed `AssessmentRequest` (§4.1). A violation is returned as
   an error; `Decide`'s existing `OutcomeError` path takes over and the pre-declared
   default is used.
3. A validated `Prediction` is projected to an `Advice` (the highest-probability calibrated
   label, ties broken by label name for a deterministic answer) UNLESS it is an explicit
   abstention or carries no calibrated label at all, in which case no `Advice` is produced;
   `Decide`'s own vocabulary check resolves an empty/absent answer to `OutcomeInvalid` and
   falls back to the default.

Every existing `Decide` guarantee — the kill switch, the per-item/per-hour budget, the
timeout, the journal-or-discard rule for advised answers, and the reserved-verb deny-list —
applies to a `Prediction`-based provider identically to a plain `Advisor`, with no code
change to `decide.go`.

## 7. Known limitations

This version does not define: the mixed deterministic/probabilistic admission policy that
consumes `Prediction` and `PolicyResult` together (graph-execution/13, GEA-09/10); a
calibration evaluation or reliability-diagram procedure (graph-execution/12, GEA-08); or any
concrete provider (Laya or otherwise, graph-execution/11, GEA-05/06). It fixes only the
envelope shape and its validation rules.
