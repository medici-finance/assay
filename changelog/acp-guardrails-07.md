### Added
- `commsgw` now screens **every** outbound send through a quarantined prose gate on the
  gateway send path, after the deterministic pre-checks pass. Within-cell and cross-cell
  sends alike are consulted with no risk-trigger predicate — an independent second layer
  that fails on a different signal than the tokens-only, slug-blind body scanner. The gate
  is advise-only (it never rewrites content): its Decide-shaped verdict is one of
  `clean-send` / `hold-for-human` / `refuse`, and any non-clean verdict HOLDS the message
  (held mailbox plus a filed issue carrying the payload DIGEST, never the raw payload) — a
  hold is never a silent drop or an auto-retry.
- The gate's reader runs under the refuse-everything containment profile of the pinned
  decider runner entry: an empty filesystem root and callback policies that refuse every
  fs / terminal / tool request and file the attempt as a containment anomaly.

### Changed
- The outbound send path fails closed: a deterministic refusal is terminal and the gate is
  never consulted after it, while the gate's own default is `hold-for-human`, so a disabled
  valve (`DESK_DECIDE_DISABLED=1`), a spent budget, a timeout, an advisor error, or an
  injected/malformed answer all resolve to a hold rather than an ungated send.
