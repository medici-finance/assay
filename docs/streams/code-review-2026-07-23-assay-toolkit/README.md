---
stream: code-review-2026-07-23-assay-toolkit
status: active
priority: P1
track: platform
serves: assay
tiering: implement=cheap verify=strong
---

# Code-review remediation — 2026-07-23 (assay-toolkit-resident fixes)

The 2026-07-23 codebase review (9-reviewer consolidation) of the Medici lending stack found two
defect clusters whose **CODE lives in THIS repo (`assay-toolkit`)** — the canonical `statusgen`
that runs as the pinned release binary. (The `oit` `tools/statusgen/`
is a FROZEN copy that does NOT run; fixing it changes nothing — that mismatch is finding T1 itself.)
Those two briefs live here where their fix lands.

**Everything else from that review is tracked in its home repo, not here:**
- The **`cheap`-tier remainder** (DAML, ledger-service, frontend, oit-native desk tooling, bots,
  medici-deploy) → a `code-review-2026-07-23` stream in **`oit`**
  (that repo has its own `docs/streams/` + batch-fanout board).
- The **`opus`/judgment/security/design findings** → issues in **a private review channel
  configured by the operator**, never on a public repo.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [statusgen anti-falsification & tripwire robustness (T1, T3, T7, T8, T9, T10)](./brief-01-statusgen-antifalsification.md) | 0 | M | implemented | — | — |
| 02 | [DAR-drift CI gate + Job-name-bump enforcement (M3)](./brief-02-darsync-ci-gate.md) | 0 | S | implemented | — | — |

## Critical path

```
(independent — no blocking chain)
   01 statusgen anti-falsification        02 DAR-drift CI gate
   (trustgate/corroborate/registers/       (wire darsync as a merge gate;
    idvalidate/scanissues fail-open)         + Job-name-bump enforcement)
```

Both are Wave 0 and independent. **Brief 01 is highest value** — it is fail-open in the
anti-falsification path (the code that decides whether a human actually approved), the worst failure
direction for a trust gate.

## Dependency waves

```
Wave 0: [01] [02]
```

## Gates

Both `gate: model` — revertible tooling changes, no risk-boolean surface. Brief 02 crosses a repo
boundary (the darsync check is here; the merge-gate wiring lands in the oit CI workflow) — it carries
the sibling-repo note in-brief.

## Definition of Done (stream conventions)

- The fix lands in **this repo's `statusgen/`** (brief 02 additionally needs a CI-workflow edit in
  the oit repo — noted in-brief). Every Verify row is runnable by a non-implementer.
- **Fail-closed, always**: every fix moves the ambiguous/tie/error case to FLAG, never to a silent
  pass. Do not weaken an existing check.
- Stop at `implemented`; a non-implementer runs Verify and fills Evidence before `verified`/`done`.
- `cd statusgen && go run . --root .. --lint` stays exit 0; never hand-edit STATUS.md.
