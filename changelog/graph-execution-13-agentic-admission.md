### Added
- Agentic admission draft (`spec/agentic-admission-v1.md`, `schemas/agentic-assessment-v1.json`, `deskkit` `EvaluateAgenticAdmission`). It is a pure, deterministic policy that maps hard facts and recorded advice to one of five dispositions: bounded-agent-work, supervised-agent, human-led, discovery-only or blocked.
  - Advice can only restrict.
  - Unknown readiness holds implementation.
  - discovery-only stands only on a discovery grant whose read scope covers the subject; otherwise the result is blocked.
  - Malformed input fails closed: empty bindings, invalid or oversized subjects and inputs, and out-of-vocabulary values. Reason codes never echo an assessed string.
  - Mandatory graph gates are the union of the brief's risk verdict and the disposition's.
  - Nothing activates it yet; dispatch wiring is a separate gated change.
