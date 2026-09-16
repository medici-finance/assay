---
brief: assay:assay:desktools-v2:02
title: the v2 seam contract + the ban-lint (advisory/counting first)
why: >-
  A Forge seam exists but callers reach around it, and each reach-around is its own bug. The
  only durable fix is to make a reach-around a red build rather than a code-review catch. This
  brief writes the ban-lint — a CI check that flags a gh subprocess, a hardcoded remote name,
  and a pullRequest-shaped query outside the two backends — and lands it advisory (counting)
  so its baseline is recorded before it bites. It is the enforcement that makes every later
  migration stick: once the count is zero and the gate is failing, the class cannot return.
wave: 2
depends: ["desktools-v2/01"]
unblocks: ["desktools-v2/04", "desktools-v2/05", "desktools-v2/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §2 (commitments 1-2) — the seam contract and the ban"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the baseline set the counter freezes"
  - "tools/desk/internal/forgeban/allowlist.go — the existing permit-register ceiling (const allowedInvocationCeiling = 5) this generalizes into a positive ban whose target is zero"
  - ".github/workflows/forge-surface-control.yml — the existing shell-exec-ban / no-passthrough CI job the counter joins"
  - "docs/streams/forge-neutral/README.md — forge-neutral owns the resolver/custody (forge-neutral/01); this brief cites it and does NOT re-implement it"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — forge-surface-control.yml exists; forgeban ceiling is 5; #305/#834/#838 record the ban is a ratchet not a closure-to-zero"
consumers:
  - "tools/desk/scripts/forge-ban.sh (planned): follow-up desktools-v2/02 (this brief; flips to fixed-here when the implementation adds the counter script)"
  - ".github/workflows/forge-surface-control.yml: follow-up desktools-v2/02 (this brief; the counter joins the existing job — flips to fixed-here when the workflow edit lands)"
  - "tools/desk/internal/forgeban: out-of-scope (the Go permit-register is forge-neutral's ratchet surface; this brief's counter reconciles against it, does not edit it)"
exec-tier: strong
exec-tier-why: >-
  question (a) — the ban's pattern set is a design decision the facts do not fully pre-specify
  (how narrow, how to exempt the two backends and their tests), and a too-broad pattern reddens
  legitimate code while a too-narrow one passes a real reach-around; getting the exemption
  boundary exactly right is the correctness surface.
domain: complicated
version: 1
id: e2a63c73-00e1-4541-8131-3e9232bfa8fd
---

# Brief 02 — the v2 seam contract + the ban-lint

## Context

files:
- NEW `tools/desk/scripts/forge-ban.sh` (planned) — a portable (macOS + Linux) grep that
  counts reach-around sites and, in advisory mode, prints the count and exits 0.
- `.github/workflows/forge-surface-control.yml` — the existing forge-surface CI job the
  counter joins (advisory step added; not flipped to failing here).
- NEW `docs/streams/desktools-v2/seam-contract.md` (planned) — the one-page statement of the
  contract: the four fact classes (gh subprocess, remote name, query shape, host literal) may
  appear ONLY in `forge_github.go` / `forge_gitlab.go` and their tests.

single-point-of-failure: the ban-lint is the ONE control that makes a reach-around a red
build. It is NOT the only layer: it is backed by a second, independent control that fails for
a different reason in a different component — the existing `forge-surface-control.yml`
no-passthrough shape check + the `forgeban` permit-register ratchet, which already refuse a
new exported raw-request method and a new forge-CLI Go call site. A grep-based ban that finds
a `gh` string in a shell script and a Go-shape check that finds a new raw method catch two
different ways around the seam. This brief adds the first; the second already exists.

facts:
- The ban targets four fact classes, each exempted ONLY inside `forge_github.go` /
  `forge_gitlab.go` and their `_test.go` siblings: (a) a `gh` subprocess literal; (b) the
  remote name `"origin"` hardcoded in a desk tool; (c) a `pullRequest` / `mergeRequest`
  GraphQL query block; (d) the GitHub REST/GraphQL host literal (`api.github.com`), which
  `forge.go` already documents as living in exactly one place (`GitHubAPIBase`).
- The counter starts ADVISORY (prints `forge reach-around sites: N`, exits 0), exactly as
  `desktools-go-git`'s `count-git-exec.sh` did. It flips to failing (non-zero above zero)
  only after the migrations (`desktools-v2/04..06`) drive N down — that flip is a separate,
  later brief, not this one.
- This brief does NOT touch identity or token custody: WHICH forge and WHICH identity a write
  uses is `forge-neutral/01`'s resolver, which this brief cites and consumes. The ban only
  asserts that construction happens inside a backend; it does not decide which backend.
- The existing `tools/desk/internal/forgeban/allowlist.go` ceiling is 5 Go call sites at `e9fa19d3`. The counter
  reconciles against it (its Go rows are a subset) and does not lower or edit it.
- Out of scope: flipping the gate to failing; migrating any site; editing
  `tools/desk/internal/forgeban/allowlist.go`; anything touching a minted token.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If greening ever requires removing or weakening a security control or its CI assertion:
  STOP and escalate — do not weaken the existing forge-surface job to make room for the counter.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `docs/streams/desktools-v2/seam-contract.md` (planned): the four fact classes, the
   two-backend exemption, and the "construction only inside a backend" rule, in one page.
2. Write `tools/desk/scripts/forge-ban.sh` (planned): a portable grep that counts sites
   matching the four classes OUTSIDE `forge_github.go` / `forge_gitlab.go` (+ tests), prints
   `forge reach-around sites: N`, and exits 0 (advisory). Seed its exclusion list from the
   inventory's Reconciled note so a known-and-routed site is counted, not hidden.
3. Add an advisory step to `.github/workflows/forge-surface-control.yml` that runs the counter
   and echoes the count (exit 0). Do not add a failing gate.
4. Record the baseline N and the per-class breakdown in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -x tools/desk/scripts/forge-ban.sh; echo rc=$?` | `rc=0` (the counter is present and executable) |
| 2 | `sh tools/desk/scripts/forge-ban.sh; echo rc=$?` | prints `forge reach-around sites: <N>`; `rc=0` (advisory/counting mode — does not fail the build) |
| 3 | `grep -cE -e 'forge_github.go' -e 'forge_gitlab.go' tools/desk/scripts/forge-ban.sh` | exit 0; count >= 2 (the two backends are the exemption the counter excludes) |
| 4 | `grep -c 'forge-ban' .github/workflows/forge-surface-control.yml` | exit 0; count >= 1 (the counter is wired into the existing forge-surface job) |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb.txt` | exit 0; prints `reach-around sites: <N>` with N a real integer (the counter emits a number, not a placeholder — the dereferencing check against the inventory baseline) |
| 6 | `test -f docs/streams/desktools-v2/seam-contract.md && grep -cE -e 'origin' -e 'pullRequest' -e 'api.github.com' docs/streams/desktools-v2/seam-contract.md` | exit 0; count >= 1 (the contract names the banned fact classes) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a CI counter shipped ADVISORY, a portable grep, and a
one-page contract doc; adds a lint per the stream's gate rule, binds no identity/token, edits
no security control). Reviewer records verdict + date in the stream README table. Row 5 is the
dereferencing row (the counter emits a real integer, not a placeholder); row 3 pins the
two-backend exemption the whole ban depends on.
