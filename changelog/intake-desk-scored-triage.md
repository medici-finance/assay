### Added
- The `intake-desk` skill gains a scored-triage convention: every triage exit records an `impact`/`risk`/`effort` label triple with a one-line per-axis rationale (judgment recorded, never computed in CI); human-facing surfaces order SLA-ESCALATE items first, then impact-desc / risk-desc / effort-asc, then the existing urgency-then-age — unlabelled items sort exactly as today (#294).
