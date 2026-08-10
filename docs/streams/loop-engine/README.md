---
stream: loop-engine
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# Loop-Engine Stream

Move the outer control loop of the drain-shaped desk roles (verify-desk, batch-fanout,
issue-loop's dispatch lane) out of the operator model's attention and into a deterministic
Go conductor built on the instruments `../assay-toolkit/tools/desk/` already provides (deskboard, issueboard,
deskpost/deskpr/deskreply, deskwt/desktoken/deskroster, `deskkit.Guard()`/audit/claims).
Design + archetype analysis + engine contract + open questions:
[docs/loop-engine-architecture.md](https://github.com/example-org/oit/blob/main/docs/loop-engine-architecture.md) (read it FIRST — the
drain-engine contract in §4 and the "what stays per-loop" list in §6 are binding on every
brief here). Origin: human:<name>'s "fix the verify loop, examine all the other loops" direction;
incidents #541 (inline-verify, zero artifacts), #79 (false idle), F-verify-self-attest.
Maintenance owner: the process desk, methodology track.

The one-line contract: **the model's only job per item is the judgment inside the dispatched
agent; every scheduling fact (pool, claims, refill, retry) lives in code.** The board-reactor
(pr-review-desk) is NOT a drain and gets its own driver; the-desk and intake triage stay prose.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Drain engine + verify-desk reference consumer](./brief-01-drain-engine-verify-desk.md) | 0 | L | implemented | — | — |
| 02 | [Generalize: batch-fanout as second engine consumer](./brief-02-generalize-batch-fanout.md) | 1 | M | todo | — | — |
| 03 | [issue-loop dispatch lane as third engine consumer](./brief-03-issue-loop-dispatch-lane.md) | 1 | M | todo | — | — |
| 04 | [Guardrail module — single home for the six duplicated rule blocks](./brief-04-guardrail-module.md) | 1 | M | todo | — | — |
| 05 | [pr-review-desk board-reactor — formalize, do NOT drain-ify](./brief-05-pr-review-board-reactor.md) | 1 | M | todo | — | — |
| 06 | [Companion article + generic cloneable drain-harness](./brief-06-companion-article.md) | 1 | M | todo | — | — |
| 07 | [Interim durable Monitor for verify-desk (bridge until 01)](./brief-07-verify-desk-interim-monitor.md) | 0 | S | todo | — | — |

## Dependency waves

```
Wave 0: [01, 07]
Wave 1: [02, 03, 04, 05, 06] ← 01
```

Brief **07** is an independent wave-0 interim: it gives verify-desk the durable-Monitor liveness
its sibling loops already have, so the drain self-sustains *today*. It is superseded and removed
when brief-01's Go conductor becomes verify-desk's driver.

Critical path: **01 → 02** (the second consumer is what proves the contract is a contract and
not a verify-desk implementation detail; 03/04/05 parallelize behind it).

## Shared conventions (inherited by every brief)

- Code home `../assay-toolkit/tools/desk/internal/loopengine/` (+ per-loop adapters under `../assay-toolkit/tools/desk/`);
  wired into the repo's existing Go workspace arrangement exactly as `tools/desk`/
  `tools/statusgen` are. deskkit stays the guard layer — the engine CALLS `Guard()`,
  audit, claims, writeguard; it never re-implements them.
- The drain-engine contract (architecture doc §4) is FROZEN SMALL: per-loop heterogeneity
  goes in `Land()`/`TierPolicy()` or stays per-loop prose. A brief that needs a new engine
  hook stops and files a design issue instead of adding one (§8 contract-erosion risk).
- Per-loop irreducibles (architecture doc §6) — tier floors, risk routing, irreversible
  carve-out, intake exits, App-approval gates — are NEVER absorbed into shared engine logic.
- Refusal/negative tests are deliverables (desk-tools C-9 inherited): claim-collision,
  author==runner refusal, stop-flag mid-drain, land-failure files-and-continues.
- Fail closed on any state the engine cannot positively verify (C-10); distinct exit codes
  per deskkit conventions.
- Each migration lands as its own branch + draft PR + review + verify — this stream dogfoods
  the pipeline it improves.
- Open Question 9.2 (F-16 tier rung) is human:<name>'s decision; every brief encodes the CURRENT
  collapse (risk-flagged ⇒ human) and keeps the rung flip a one-line `TierPolicy` change.
