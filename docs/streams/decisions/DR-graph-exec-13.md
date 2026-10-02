---
id: DR-graph-exec-13
date: "2026-10-02"
title: "Agentic admission is a deterministic, fail-closed evaluator: facts decide, advice only restricts, discovery needs a covering grant, gates are a union"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2034#issuecomment-5946145475"
alternatives:
  - "Let calibrated advice above a confidence threshold widen admission — ruled out: no confidence level may enlarge permission, so advice is restrict-only."
  - "Treat unknown readiness as `human-led` — ruled out: a human-led lane still implements, and unknown readiness must hold implementation."
  - "Take the stricter of the two risk verdicts instead of the union of their gates — ruled out: where two verdicts' gate lists are not nested, \"stricter\" drops gates."
  - "A single numeric admission score — ruled out: a disposition plus reason codes is auditable, and a score invites thresholds that widen."
accepted:
  - "The evaluator trusts its inputs. Producer independence and authentication are the dispatch binding's job (graph-execution/14), and the spec states this as a MUST."
  - "Policy values (ceilings, fail ceilings, freshness) are an owner's ruling. The fixtures carry example values only."
  - "Until graph-execution/14 lands, the evaluator changes no behaviour."
---

**Ruling recorded (2026-10-02): approved.** The driver (`human:<name>`) ruled on the brief's
decision-gate [issue #2034](https://github.com/medici-finance/assay/issues/2034) under the
driver's own login — the
[ruling comment](https://github.com/medici-finance/assay/issues/2034#issuecomment-5946145475)
(2026-10-02T05:28:47Z) reads `approve — DR-graph-exec-13`. This record transcribes the design
and record content that issue put for approval; it does not mint a new decision — the human
act is the driver's comment on #2034, not this file.

**The decision.** `docs/streams/graph-execution/brief-13-agentic-admission.md`: a pure,
deterministic admission evaluator. It decides how much of an owner-admitted category of work
an agent may do on one subject, and returns one of five dispositions:
`bounded-agent-work` < `supervised-agent` < `human-led` < `discovery-only` < `blocked`.
Spec: `spec/agentic-admission-v1.md`. Schema: `schemas/agentic-assessment-v1.json`.

- **Facts decide; advice only restricts.** A failed or unknown authority or data-handling
  fact is `blocked`. A failed readiness fact caps at an owner-set ceiling of `human-led` or
  stricter. Recorded model advice can move the result only toward `blocked`; a kill switch
  disables it.
- **Unknown readiness holds implementation.** It is `discovery-only` only under a separate
  discovery grant whose read scope covers the subject, and `blocked` otherwise. Every path
  to `discovery-only` is held to that grant.
- **Fail closed on malformed input.** Empty bindings, invalid or oversized subjects and
  inputs, out-of-vocabulary values and future or zero-time facts are never a pass. Reason
  codes never echo an assessed string, and a refused subject is never echoed into the result.
- **Gates are a union.** Each disposition maps to a workflow-pattern risk-input verdict, and
  the mandatory gates are the union with the brief's own verdict. Nothing subtracts a gate.
- **Human floors stand.** `merge`, `release` and `deploy` are never an agent operation.
- **Not active.** Nothing on the dispatch path consults it. Binding it at dispatch is
  graph-execution/14, behind its own human gate.

**Consequence: major.** If the design is wrong, a later dispatch binding could admit agent
work that should be human-led or held. The evaluator is inactive until graph-execution/14.
