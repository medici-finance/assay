---
id: DR-handoff-shared
date: "2026-10-25"
title: "Adopt the delivered shared selection and advisory handoff contract"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4216#issuecomment-6399186726"
alternatives:
  - "Retain per-caller policy copies — rejected because callers can assign different meanings to the same blocker."
  - "Add a helper process — rejected because pure selection and assessment need no process boundary."
  - "Treat labels or local logs as assignment authority — rejected because they do not establish source and actor binding."
accepted:
  - "deskcore/issueflow owns pure population selection and advisory handoff assessment; callers retain trust, provider reads, custody, claims, effects and independent acceptance."
  - "Legacy or incomplete observations remain unknown; observed routing grants neither readiness nor acceptance."
  - "A separately versioned additive handoff observation and source-module dependency are retained; the five existing intake exits remain unchanged."
  - "Operational writer activation, release, consumer qualification and final acceptance remain separate."
---

This record adopts the bounded delivered design described in PR #4306 and
[`verification-handoff-v1`](../../../spec/verification-handoff-v1.md).
It records the driver's explicit design confirmation, rather than treating the
earlier brief-only approval as approval of an unnamed record.

The driver approved `verification-handoff/01` and `DR-handoff-shared` as described in PR #4306: https://github.com/medici-finance/assay/issues/4216#issuecomment-6399186726

Delivery #4225 and independent PASS #4255 predate this design-record transcription.
Their Evidence remains unchanged. This record supplies the missing design
reference; it does not manufacture a new execution witness or final human sign-off.
