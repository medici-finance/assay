---
brief: assay:assay:desktools-v2:05
title: "migrate deskpushguard off the hardcoded remote name + expand insteadOf (#1201 / #884)"
why: >-
  deskpushguard hardcodes the remote name "origin" (#1201) and does not expand
  insteadOf/pushInsteadOf (#884). Both are forge-assumption reach-arounds AND guard-coverage
  holes: a push to a differently-named remote, or through a URL git rewrites via insteadOf,
  evades the guard. Deriving the remote from the push invocation and resolving the effective
  URL through git's own insteadOf rules closes the evasion and removes two hardcoded GitHub
  assumptions in one tool. This STRENGTHENS the guard; it never weakens it.
wave: 3
depends: ["desktools-v2/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §1 (#1201/#884 rows), §3 (boundary with desktools-go-git)"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the deskpushguard remote-name / insteadOf rows"
  - "tools/desk/cmd/deskpushguard/ — the guard; foreigncommit.go and its remote/URL handling"
  - "docs/streams/desktools-go-git/README.md — desktools-go-git owns git-TRANSPORT migration; this brief coordinates the transport half and owns the forge-assumption half (the hardcoded remote name + insteadOf expansion)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — deskpushguard's run() takes the remote name as an argument (deskpushguard_test.go passes \"origin\" positionally); the exact hardcode site and the insteadOf gap are pinned by the inventory (desktools-v2/01)"
consumers:
  - "tools/desk/cmd/deskpushguard: follow-up desktools-v2/05 (this brief; remote-name derivation + insteadOf resolution replace the hardcoded assumptions — flips to fixed-here when the implementation lands)"
  - "git transport / push mechanics: out-of-scope (owned by desktools-go-git; this brief coordinates with it and touches no git plumbing beyond reading the effective URL)"
exec-tier: strong
exec-tier-why: >-
  question (c) — a security guard where a subtle error (an insteadOf rule not fully resolved, a
  remote-name derivation that misses a case) leaves an evasion path that survives happy-path
  tests; the fix must be proven with a NEGATIVE-path test that pushes through a rewritten URL.
domain: complicated
version: 1
id: a8da9afb-a695-4ee1-bbb0-87064d9eae86
---

# Brief 05 — migrate deskpushguard off the hardcoded remote name + expand insteadOf

## Context

files:
- `tools/desk/cmd/deskpushguard/` (the guard; `foreigncommit.go` and the remote/URL handling
  the inventory pins) — remote-name derivation + insteadOf resolution replace the hardcoded
  `"origin"` and the un-expanded URL.
- NEW/extended `..._test.go` — including the negative-path evasion tests below.

single-point-of-failure: deskpushguard IS a security guard; the control that matters is that
it evaluates the ACTUAL push target, not an assumed one. This brief does not add a second
guard — it closes the existing guard's coverage so its single evaluation is correct. The
independent layer already present: the ban-lint (`desktools-v2/02`) catches a re-introduced
hardcoded `"origin"` in CI (a different component, a different signal) than the evasion test
that pins the runtime behavior. The guard's own assertion must be STRENGTHENED, never weakened.

facts:
- #1201: the guard hardcodes the remote name `"origin"`. It must derive the remote from the
  push invocation (a push names its remote), so a differently-named remote is still guarded.
- #884: the guard does not expand `insteadOf` / `pushInsteadOf`. It must resolve the EFFECTIVE
  push URL through git's own rewrite rules before evaluating, so a rewritten URL cannot evade.
- Boundary (spec §3): git-transport migration is `desktools-go-git`'s; this brief owns only
  the forge-assumption half (remote-name derivation + insteadOf resolution) and touches no git
  plumbing beyond reading the effective URL.
- The old hardcoded `"origin"` assumption is DELETED, not left as a default fallback.
- Out of scope: any credential change; migrating deskpushguard's git reads to go-git (that is
  desktools-go-git); weakening any existing guard assertion.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- If greening requires removing/weakening the guard or its CI assertion: STOP and escalate
  (needs-decision, gate:human) — a guard's coverage is never traded for a green check.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Derive the guarded remote from the push invocation instead of hardcoding `"origin"` (#1201).
2. Resolve the effective push URL through git's `insteadOf` / `pushInsteadOf` rules before
   evaluating the guard (#884).
3. DELETE the hardcoded `"origin"` assumption in the same change (no fallback default).
4. Add negative-path evasion tests, named so the Verify rows target them:
   `TestDeskpushguardGuardsNonOriginRemote` (a differently-named remote is still guarded) and
   `TestDeskpushguardResolvesInsteadOf` (a push through an `insteadOf`-rewritten URL does not
   evade the guard).
5. Record in the PR body that the guard's assertions were strengthened, not weakened, and the
   ban-lint count before/after.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskpushguard/` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskpushguard/` | exit 0; the full guard suite (existing assertions + new evasion tests) passes — proves no existing assertion was weakened |
| 3 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestDeskpushguardGuardsNonOriginRemote -v` | exit 0; the named test runs (`--- PASS`) — the negative-path row proving a non-`origin` remote is still guarded (#1201) |
| 4 | `cd tools/desk && go test ./cmd/deskpushguard/ -run TestDeskpushguardResolvesInsteadOf -v` | exit 0; the named test runs (`--- PASS`) — the negative-path row proving an `insteadOf`-rewritten URL does not evade the guard (#884) |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb5.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb5.txt` | exit 0; count STRICTLY LOWER than the pre-brief value recorded in the PR body (deskpushguard's hardcoded-remote reach-around is gone — the removal check; the ban-lint, not a bare grep, since tests legitimately reference the literal) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — the change STRENGTHENS a guard's coverage and removes
two hardcoded GitHub assumptions; it removes no capability and touches no credential). The
security-relevant nature is handled by the mandatory carve-out in Ground rules (no assertion
weakened) plus the two negative-path evasion rows, not by a human gate. Rows 3 and 4 are the
evasion tests (#1201 / #884); row 5 is the removal check. Reviewer records verdict + date in
the stream README table.
