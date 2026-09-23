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
unblocks: ["desktools-v2/03", "desktools-v2/05", "desktools-v2/06", "desktools-v2/08", "desktools-v2/09"]
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
  only after the migrations drive N down: `desktools-v2/08` flips the `statusgen/**` half, and
  the desk-tools half flips in a later brief that is not authored yet.
- **The counter's scope INCLUDES `statusgen/**`** (spec §2 Principle 2 / §3 commitment 2).
  statusgen is a separate Go module shelling `gh` directly and is NOT under `forgeban` today;
  the ban-lint is the control that brings it under the same rule. Its `gh` reads are migrated
  by the sibling brief `forge-neutral/18` (through the `deskread` verb — statusgen never
  imports `deskkit`); this counter is what makes that progress visible, and
  `desktools-v2/08` flips the statusgen half to failing once it reaches zero. The counter
  prints the statusgen and desk-tools counts SEPARATELY as well as the total, so a drop in one
  cannot hide a rise in the other.
- **The baseline is a file, not prose.**
  `tools/desk/scripts/forge-ban.sh --baseline` writes the count to
  `docs/streams/desktools-v2/forge-ban-baseline.txt` (planned; one integer per line, keyed
  `<brief-id> <count>`), so a later brief's "strictly lower than" row compares two machine-
  readable numbers instead of a sentence in a PR body.
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
| 3 | `sh -c 'for p in forge_github.go forge_gitlab.go; do grep -qF -- "$p" tools/desk/scripts/forge-ban.sh; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $p"; exit 1; fi; done; echo both-exempt'` | exit 0; prints `both-exempt` (each backend is checked SEPARATELY — one name on two lines cannot pass for both) |
| 4 | `grep -c 'forge-ban' .github/workflows/forge-surface-control.yml` | exit 0; count >= 1 (the counter is wired into the existing forge-surface job) |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb.txt` | exit 0; prints `reach-around sites: <N>` with N a real integer (the counter emits a number, not a placeholder — the dereferencing check against the inventory baseline) |
| 6 | `test -f docs/streams/desktools-v2/seam-contract.md && grep -cE -e 'origin' -e 'pullRequest' -e 'api.github.com' docs/streams/desktools-v2/seam-contract.md` | exit 0; count >= 1 (the contract names the banned fact classes) |
| 7 | `sh tools/desk/scripts/forge-ban.sh --baseline && grep -cE '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | exit 0; count = 1 (the baseline later briefs compare against is a machine-readable line, not a PR-body sentence) |
| 8 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb2s.txt 2>&1; grep -oE 'statusgen sites: [0-9]+' /tmp/dv2-fb2s.txt` | exit 0; prints `statusgen sites: <N>` with N a real integer (the statusgen half is counted and reported on its own line) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

| # | Command | Expected | Observed (exit + key output) | Date Runner |
|---|---------|----------|------------------------------|-------------|
| 1 | test -x tools/desk/scripts/forge-ban.sh; echo rc=$? | rc=0 (counter present and executable) | exit 0; rc=0 | 2026-09-23 opus-5.5-verifier |
| 2 | sh tools/desk/scripts/forge-ban.sh; echo rc=$? | prints "forge reach-around sites: N"; rc=0 (advisory) | exit 0; "forge reach-around sites: 55 (desk: 26, statusgen: 29)"; rc=0 | 2026-09-23 opus-5.5-verifier |
| 3 | sh -c 'for p in forge_github.go forge_gitlab.go; do grep -qF -- "$p" tools/desk/scripts/forge-ban.sh; ...; done; echo both-exempt' | exit 0; prints both-exempt (each backend checked separately) | exit 0; both-exempt | 2026-09-23 opus-5.5-verifier |
| 4 | grep -c 'forge-ban' .github/workflows/forge-surface-control.yml | exit 0; count >= 1 (counter wired into forge-surface job) | exit 0; 2 | 2026-09-23 opus-5.5-verifier |
| 5 | sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb.txt | exit 0; "reach-around sites: N" with N a real integer | exit 0; "reach-around sites: 55" | 2026-09-23 opus-5.5-verifier |
| 6 | test -f docs/streams/desktools-v2/seam-contract.md && grep -cE -e 'origin' -e 'pullRequest' -e 'api.github.com' docs/streams/desktools-v2/seam-contract.md | exit 0; count >= 1 (contract names the banned classes) | exit 0; 10 | 2026-09-23 opus-5.5-verifier |
| 7 | sh tools/desk/scripts/forge-ban.sh --baseline && grep -cE '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt | exit 0; count = 1 (baseline is a machine-readable line) | exit 0; 1 (line written: desktools-v2/02 55) | 2026-09-23 opus-5.5-verifier |
| 8 | sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb2s.txt 2>&1; grep -oE 'statusgen sites: [0-9]+' /tmp/dv2-fb2s.txt | exit 0; "statusgen sites: N" with N a real integer | exit 0; "statusgen sites: 29" | 2026-09-23 opus-5.5-verifier |

All 8 Verify rows pass. Independently corroborated by statusgen verifyrun (v1.0.26): rows 1-8 all pass (exit=0), witness rows appended to the brief's Evidence section in the verifier worktree.

RISK-VALUE lines (kit §4 — enumerate → rank → derive):

- Enumeration over the diff scope (tools/desk/scripts/forge-ban.sh, docs/streams/desktools-v2/seam-contract.md, the forge-surface workflow advisory step, docs/streams/desktools-v2/forge-ban-baseline.txt) yields exactly one data literal that this brief introduces and that a later brief consumes: the recorded baseline count, baseline = 53 @ docs/streams/desktools-v2/forge-ban-baseline.txt:1. Every other literal in the script is control-flow/pattern text: exit codes (2, 0) and the class regex/exclude patterns (BACKEND_EXCLUDE, GH_SUBCMDS, FORGE_GO_EXCLUDE) — patterns and exit statuses, not risk-bearing thresholds. The forgeban ceiling (allowedInvocationCeiling = 5) is referenced by the script but is NOT set or changed by this diff (out of scope, per the brief).
- RISK-VALUE: NAMED, NOT DERIVED — baseline = 53 @ docs/streams/desktools-v2/forge-ban-baseline.txt:1 — this is a reversible advisory metric (the counter always exits 0; no gate rides on it), so it ranks LAST by irreversibility and owes no first-principles derivation. It is a MEASURED value (whatever the counter emits on the frozen SHA), not a designed constant. It cannot be derived, only reproduced — and on this merged SHA the counter reproduces 55, not the recorded 53 (see Findings). A wrong/stale baseline is correctable by an edit + re-run, not an irreversible act.
- RISK-VALUE: N/A for irreversible values — enumeration found no irreversible or hard-pinned constraint value (no settlement/auth/token/money literal); the item introduces an advisory counter with no failing gate. The one enumerated literal is the reversible baseline metric above.

RISK-VALUE question filed: #1529 (baseline 53 no longer reproduces on merged main; the counter reads 55).


## Review
Gate: model (all four risk answers no — a CI counter shipped ADVISORY, a portable grep, and a
one-page contract doc; adds a lint per the stream's gate rule, binds no identity/token, edits
no security control). Reviewer records verdict + date in the stream README table. Row 5 is the
dereferencing row (the counter emits a real integer, not a placeholder); row 3 pins the
two-backend exemption the whole ban depends on.
