---
stream: measured-status
repo: medici-finance/assay
serves: assay
status: parked
priority: P1
track: platform
issues: []
board: generated
---

# measured-status Stream

**Status:** proposed — this scoping doc is authored for review; the stream is `parked` (its
briefs are recorded but not dispatchable) until a human ratifies it and flips `status:` to
`active`. Parked is how a not-yet-approved stream is represented without reddening the
stream-source lint (`active` is the only status that requires a priority and admits its briefs
to the Next-up board; `parked` reserves the namespace and holds the work off the board, the
same shape a `PROPOSED` design-decision record uses to reserve a gate it does not yet satisfy).

## The seam

**Status is attested, not measured.** The board and the desk tools trust a set of
risk-bearing facts that an agent or a human *typed into a cell* rather than *derived from a
computable source*, and in one load-bearing place they do not enforce that a verifier is
independent of the implementer. This is the house's own #1 known weakness — the
self-attestation error class: everything a session writes about its own work is, at the last
mile, prose it authored, and a gate derived from those self-declared answers inherits their
unchecked-ness (`docs/archive/mistake-proofing/README.md`, "the self-attestation error
class"; `docs/iso9001-mapping.md`).

The design principle this stream drives toward is two-sided:

- **derive-not-assert** — a risk-bearing value is *computed from its source and checked*,
  never hand-typed and left with no derivation on record. A literal with no derivation is a
  fact wearing a computed value's clothes.
- **enforce independence** — the author≠verifier property is a *hard gate the code makes
  true*, not a substring a session writes about itself and not a NOTICE a reader may skim
  past.

The stream is a natural continuation of `derived-board` (which made the lifecycle *cells* —
`todo`/`in-progress`/`implemented`/`verified`/`done` — derived rather than hand-asserted) but
is deliberately its own stream: `derived-board` is done/in-flight and scoped to the generated
Briefs table and the `reconcile` engine, whereas this stream touches a different set of source
files (`attribution.go`, `learned.go`, `exitcodes.go`, `modelstamp.go`, `loop.go`, the
`--lint` stale-FAIL nudge) and, in its enforcement half, *narrows a trust gate* rather than
generating a table. Extending `derived-board` with a verifier-independence hard-reject would
be a bad fit; this stream cites `derived-board` as its parent principle instead.

## The evidence

Each brief below closes an open issue where a risk-bearing value is asserted-not-derived, or
where independence is noticed-not-enforced:

- **#1216** — `deskkit.ExitRefused = 5` (and the whole `0/3/4/5/6` table) has no derivation
  on record.
- **#1171** — `MinCorpus = 40` is not derived against the learned model's 15-feature vector's
  events-per-variable guidance.
- **#1065** — `risk = false` is hardcoded in the commsloop router, so the
  `risk:yes`→`tier:human` backstop can never fire.
- **#862** — a stale `VERIFY: FAIL` nudge is computed from the Evidence text alone, not from
  whether the fix commits postdate it, so it routes a worker to hand-file a sign-off that
  cannot take effect.
- **#1116** — the committer-identity cross-check only ever `NOTICE`s a same-identity
  author/verifier pair; it never hard-rejects, so the independence property the check exists
  to establish does not actually gate `verified`/`done`.
- **#336** — the model-floor keys dispatch authority off the label *actor's login* rather
  than a derived, verifiable stamp, so a legitimately-dispatched PR whose label was applied
  by a trusted login (not a trusted dispatcher slug) is refused a verdict.

Related, mentioned but not pulled in: #1176 (reconcile witness-matching after the brief-v2
flag-day) and #249 (a `Wave:` trailer for docs-only Evidence landings) are adjacent
board-integrity items; they are noted here and decided per-brief to be out of this stream's
scope (they are not derive-not-assert / independence-enforcement changes).

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Derive the deskkit exit-code table — record the convention ExitOK/Disabled/RateLimited/Refused/Unverifiable follow, and pin it with a test](brief-01-exit-code-derivation.md) | 0 | S | todo | — | — |
| 02 | [Derive MinCorpus for the learned riskscore model against its 15-feature events-per-variable floor, or record the rationale — and pin it with a test](brief-02-mincorpus-derivation.md) | 0 | M | todo | — | — |
| 03 | [Derive the commsloop router's risk field from the envelope instead of hardcoding false, or record why false is sound — restore the risk:yes->tier:human backstop](brief-03-commsloop-risk-derivation.md) | 0 | M | todo | — | — |
| 04 | [statusgen --lint: derive stale-FAIL vs missing-card from commit dates, and route each state to the verify desk instead of nudging a worker to hand-file a sign-off](brief-04-stale-fail-lint-derivation.md) | 0 | M | todo | — | — |
| 05 | [attribution.go: a same-identity author/verifier pair in a multi-identity repo becomes a hard PROBLEM, not a NOTICE — the independence gate the check exists to establish](brief-05-attribution-hard-reject.md) | 1 | M | todo | — | — |
| 06 | [model-floor: derive dispatch authority from the dispatch stamp, and give a first-class re-stamp path — stop refusing verdicts by guessing at a label actor's login](brief-06-modelfloor-derived-stamp.md) | 1 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

The enforcement half is the value head, and its real blocker is a human ruling, not code:
**`DR-independence-gate`** (currently `PROPOSED` — no ruling recorded yet) **→
`measured-status/05`** (the same-identity author/verifier pair becomes a hard PROBLEM) is the
chain that, if it slips, slips the whole "enforce independence" side of the seam; `05` cannot
start until the DR is ratified. `measured-status/06` (model-floor authority from a derived
stamp) rides the same DR and is blocked the same way. The three derivation briefs (`01`, `02`,
`03`) and the lint fix (`04`) are unblocked today and run in parallel — they are `gate: model`,
cheap, and establish the derive-not-assert pattern the enforcement half then hardens.

## Dependency waves

- **Wave 0** — `01` (exit-code table derivation, #1216), `02` (MinCorpus derivation, #1171),
  `03` (commsloop risk derivation, #1065), `04` (lint distinguishes stale-FAIL from
  missing-card, #862). Independent, `gate: model`.
- **Wave 1** — `05` (attribution same-identity hard-reject, #1116) and `06` (model-floor
  derived stamp, #336). Both `gate: human`, both behind `DR-independence-gate` (currently
  `PROPOSED`; neither may start `in-progress` until it is ratified), because each narrows a
  trust/enforcement gate.
