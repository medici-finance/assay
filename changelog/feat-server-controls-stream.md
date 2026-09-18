### Added
- New **`server-controls`** stream (parked, pending human scope approval): the server-side control
  posture, reframed. It replaces the impossible ask "provision fine-grained privileges" with the
  three primitives a forge actually offers — a uniform ruleset menu, one readable ruleset API
  (retiring classic protection, `#1020`), and required status checks reported by a runner the
  policed party cannot control. Ships a scoping doc, the design record `DR-server-controls`,
  and five briefs: a uniform-ruleset audit, readable standardization, the required-check enforcement
  pattern (with its self-attestation caveat), a credential/identity decision-dependency note
  (`#900`/`#903`/`#942`), and a reference cross-operator anti-collusion check for the `#997` residual
  that remains after `require_last_push_approval` already enforces author≠approver.
