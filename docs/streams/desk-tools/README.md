---
stream: desk-tools
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# Desk-Tools Stream

Purpose-built, policy-in-code Go tools that make the methodology's standing loops
(pr-review-desk, verify-desk, batch-fanout, next-job) **zero-prompt for workflow verbs**
while keeping per-brief payload gating intact. Design + threat model: [scoping.md](./scoping.md)
(read it FIRST — its constraints C-1…C-10 are binding on every brief here; v2 folds in ~30
findings from a 3-critic adversarial filter run 2026-07-10). Origin: INTAKE I-23.
Maintenance owner: the process desk (Bob), methodology track.

The one-line contract (human:<name>, 2026-07-10): *"workflow no permissions, actual doing work might
have some"* — and *"if in doubt, ask permission, not assume"* (C-10: tools fail closed on any
state they cannot positively verify).

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [deskkit foundation — shared config/audit/kill-switch/rate-limit/version + install](./brief-01-deskkit-foundation.md) | 0 | M | done | 2026-07-10 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 02 | [deskboard v2 — read-only cross-repo board in tools/desk](./brief-02-deskboard.md) | 1 | M | done | 2026-07-10 opus-verifier | 2026-07-11 reviewer-app[bot] |
| 03 | [deskpost — review/comment/ready as the reviewer App, constraints in code](./brief-03-deskpost.md) | 1 | L | done | 2026-07-16 glm-5.2-verifier | 2026-07-11 reviewer-app[bot] |
| 04 | [deskpr — push feature branch + open draft PR, draft-only by construction](./brief-04-deskpr.md) | 1 | S | done | 2026-07-11 opus-verifier | 2026-07-11 reviewer-app[bot] |
| 05 | [deskwt — worktree add/remove under allowed prefixes only](./brief-05-deskwt.md) | 1 | S | done | 2026-07-12 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 06 | [Cutover — install, allowlist swap + local.json purge, skill wiring, drills](./brief-06-cutover.md) | 2 | M | todo | — | — |
| 07 | [deskreply — worker-identity PR replies (never the App voice)](./brief-07-deskreply.md) | 1 | S | done | 2026-07-11 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 08 | [Loop stop-flags — ALL/per-loop kill switch checked every iteration + heartbeat lease](./brief-08-loop-stop-flags.md) | 1 | S | done | 2026-07-24 glm-5.2-verifier | 2026-07-23 assay-reviewer-app[bot] |
| 09 | [deskroster — self-declared open-work → session roster (out-of-git)](./brief-09-deskroster.md) | 1 | M | verified | 2026-07-27 k3-verifier | — |
| 10 | [deskpushguard — git pre-push hook refuses a push to a MERGED/CLOSED PR branch](./brief-10-prepush-merged-guard.md) | 1 | S | done | 2026-07-25 glm-5.2-verifier | 2026-07-22 assay-reviewer-app[bot] |
| 11 | [deskboard orders actionable PRs by gate score (statusgen --gate-scores)](./brief-11-deskboard-gate-order.md) | 1 | S | verified | 2026-07-31 glm-5.2-verifier | — |
| 12 | repo scope widens to org-default for medici-finance, with a public-repo trust gate | 2 | M | todo | — | — |

## Dependency waves

```
Wave 0: [01]
Wave 1: [02, 03, 04, 05, 07, 08, 10] ← 01
Wave 2: [06 ← 02, 03, 04, 05, 07, 08], [12 ← 01]
```

Critical path: **01 → 03 → 06** (deskpost carries most of the safety constraints; the cutover
is `gate: human` — enabling zero-prompt outward verbs is human:<name>'s explicit trade, signed off with
the TM-1 accepted-residual record).

## Shared conventions (inherited by every brief)

- Code home `../assay-toolkit/tools/desk/` + shared internals `../assay-toolkit/tools/desk/internal/deskkit/`; module stays inside
  the repo's existing Go workspace arrangement (match how `tools/statusgen` is wired).
- Install target: `sudo make desk-install` → compiled binaries at `/opt/desk-tools/bin/<tool>`,
  root-owned 0755, `-ldflags "-X ...deskkit.SourceSHA=$(git rev-parse --short HEAD)
  -X ...deskkit.BuiltAt=<ts>"` (the sudo password IS the manual permission gate). Installing
  is a HUMAN act (C-1) — no brief automates it.
- Every tool: kill-switch check first (C-6), audit line always (C-5), fail closed on ambiguity
  with distinct exit codes (C-10): `3` = disabled, `4` = rate-limited, `5` = refused by
  constraint, `6` = precondition unverifiable. Exit 0 only on positive success or idempotent no-op.
- Refusal tests are deliverables (C-9) — a brief without its negative tests is not `implemented`.
