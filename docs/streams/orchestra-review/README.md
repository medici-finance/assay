---
stream: orchestra-review
status: active
priority: P2
track: platform
serves: assay
tiering: implement=cheap verify=strong
---

# ORCHESTRA-review keepers — gate telemetry + docs cold-boot drill

The external-framework review of "Leading Human-Agent Teams: The ORCHESTRA Framework for
Accountable AI Work" (SSRN 6762245; issue #307) mapped the paper's nine elements against the
running methodology — full mapping, critique, and rejected ideas in
docs/research/orchestra-framework-for-assay.md.
Most of the framework we already run with stronger (machine-checked) enforcement. Two instruments
survived the cross-check against #290/#291 and the drift-issue class as genuinely additive; they
are this stream.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Gate-effectiveness telemetry: override rate + catch rate](./brief-01-gate-effectiveness-telemetry.md) | 0 | M | implemented | — | — |
| 02 | [Docs cold-boot reconstruction drill](./brief-02-docs-reconstruction-drill.md) | 0 | M | implemented | — | — |

## Critical path

```
(independent — no blocking chain)
   01 gate-effectiveness telemetry        02 docs cold-boot drill
   (are the gates catching or             (can a fresh agent reconstruct
    ceremonial? extends the #290           the pipeline from docs alone?
    flow-report spec)                      scheduled detection for the
                                           #258/#276/#279 drift class)
```

Both are Wave 0 and independent. **Brief 01 is higher value**: the review desks' verdict flow is
the system's trust anchor, and today nothing measures whether its gates catch defects or merely
process them. Note 01 is spec-coupled to open issue #290 (flow-report scoreboard) — if #290 gets
decided/implemented first, 01's metric families fold into that implementation rather than standing
alone; the brief says so in-file.

## Dependency waves

```
Wave 0: [01] [02]
```

## Gates

Both `gate: model` — additive measurement/drill tooling, no risk-boolean surface, nothing
irreversible. Neither brief changes a shared value; neither closes #290/#291 (they extend and
complement respectively — recorded in each brief's `sources:`).

## Definition of Done (stream conventions)

- Every Verify row runnable by a non-implementer; measurement/drill instruments report
  three-state (checked-clean / checked-failed / could-not-check), never two.
- Both briefs add instruments, so each carries a positive-control Verify row (brief-rules 16
  spirit): prove the instrument fires on seeded input before trusting its silence.
- Stop at `implemented`; a non-implementer runs Verify and fills Evidence before `verified`/`done`.
- `cd statusgen && go run . --root .. --lint` stays exit 0; never hand-edit STATUS.md.
