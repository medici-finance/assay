### Fixed
- `verifyloop plan` now fails safe on risk: any brief with `gate: human` or any risk answer
  `yes` (irreversible included) is bucketed under `awaiting-human / ROUTE-HUMAN`, never printed
  as a DISPATCH candidate. Previously an `irreversible: yes` brief whose gate was `model` was
  routed to a dispatchable tier and read as dispatchable — a gate that failed open on its most
  serious input. The decision is now made in two independent places (the tier policy routes
  irreversible to the human first; the queue classifier reads the brief's own gate/risk
  frontmatter), and the gate-value comparison is normalized so a re-cased or qualified `human`
  value (`Human`, `human — <qualifier>`) still counts as the human gate.
- The `ROUTE-HUMAN` bucket now states the Evidence-only lane explicitly: each risk-flagged
  member line carries its risk reason plus `Evidence-only (never flip-eligible)`, so a reader
  can see that a model may gather Evidence for the brief but never flips it.
