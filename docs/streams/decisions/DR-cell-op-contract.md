---
id: DR-cell-op-contract
date: "2026-10-15"
title: "The cell-operation contract ships now with apply only as an interactive-terminal confirmation; approval-based, non-interactive and API apply are deferred until R7/R3 satisfy the five approval properties of amendment 21"
consequence: major  # the transcriber's classification, not part of a ruling
decided-by: "human:<name>"
alternatives:
  - "Build approval-based apply now: an operator-signed plan approval verified against a key registry. Ruled out in the brief's sources: every approval design reviewed so far was reachable by a desk running under the operator's own OS account (presence is not intent; a desk-authored registry; an unpinned enrollment root; a desk-writable consumed set — security reviews 5521033873 and 5521336354, correctness reviews 5521049854 and 5521344748)."
  - "Allow a non-interactive apply (a yes or assume-yes flag, a skip-verify flag, or apply over an API). Ruled out: any same-account process could then apply without even the mistake guard. `spec/cell-op-v1.md` §8 forbids such a surface and Verify row 9 of the brief holds it."
  - "Let each verb keep its own output and plan format. Ruled out by the brief's `why`: the future Cell API would either copy it or diverge, and every client would screen-scrape."
accepted:
  - "single-point-of-failure: the operator's own OS account — NONE behind it. A state change a desk starts has no second authorization layer today; a second layer is infeasible until desks are isolated from the operator's account, which is R3's job (amendment 21, §Approvals: deferred)."
  - "The typed confirm code and the desk-session refusal guard against mistakes and races. They are not authorization controls: a process under the operator's account can drive a terminal (`spec/cell-op-v1.md` §7)."
  - "A same-account process can delete a consumed marker, which allows one more interactive apply of the same plan before it expires, still subject to the re-measure (`spec/cell-op-v1.md` §7)."
  - "No approval mechanism, no non-interactive apply and no HTTP server ship with brief desk-containers/15. Enabling approval-based apply gates on the five properties recorded in amendment 21."
---

**This record awaits its human ruling. It is not approved.** It states the design that brief
desk-containers/15 (`docs/streams/desk-containers/brief-15-cell-op-machine-seam.md`) was built
to, from the brief's `gate-why` and from amendment 21
(`docs/proposals/non-ci-architecture/21-cell-operator-surface.md` §Approvals: deferred). The
brief cites it through `design:` because the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4) requires a cited record before a risk-gated brief leaves `todo`.

Until the driver posts a ruling comment, under the driver's own login, on the brief's decision
issue (its body carrying `<!-- decision-gate: assay:assay:desk-containers:15 -->`) and that
comment names `DR-cell-op-contract`, `statusgen --corroborate` reports this record
`placeholder-unratified`. That is the expected state of the pull request that adds it. The
ruling's link is then added as this record's `ruling:` and nothing else here changes unless the
ruling asks for a change.

## The decision

The cell-operation result and plan documents (`cell-op-v1`: `spec/cell-op-v1.md`,
`schemas/cell-op-v1.json`) ship now, with the plan store, the atomic consume and the
interactive apply order: terminal and desk-session refusals, typed confirm, re-measure, consume,
then run. Applying a plan stays an interactive confirmation at the operator's own terminal.

## Not decided here

- Which verbs adopt the contract, and how: each verb brief (desk-containers 17 to 24) with its
  own rows.
- The approval design itself (operator key, registry, enrollment, signed payload) and when the
  five properties are met: owned by R7/R3.
- The Cell API: stream `cell-api`, parked.
