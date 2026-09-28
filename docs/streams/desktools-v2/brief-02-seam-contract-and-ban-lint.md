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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -x tools/desk/scripts/forge-ban.sh; echo rc=$?` | pass exit=0 | sha256:93ff7811a209 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `sh tools/desk/scripts/forge-ban.sh; echo rc=$?` | pass exit=0 | sha256:8b61c43e2c87 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh -c 'for p in forge_github.go forge_gitlab.go; do grep -qF -- "$p" tools/desk/scripts/forge-ban.sh; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $p"; exit 1; fi; done; echo both-exempt'` | pass exit=0 | sha256:ad42e1c04e97 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'forge-ban' .github/workflows/forge-surface-control.yml` | pass exit=0 | sha256:53c234e5e847 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb.txt` | pass exit=0 | sha256:8443c9eb04b0 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `test -f docs/streams/desktools-v2/seam-contract.md && grep -cE -e 'origin' -e 'pullRequest' -e 'api.github.com' docs/streams/desktools-v2/seam-contract.md` | pass exit=0 | sha256:917df3320d77 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `sh tools/desk/scripts/forge-ban.sh --baseline && grep -cE '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | pass exit=0 | sha256:16607f48fa96 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb2s.txt 2>&1; grep -oE 'statusgen sites: [0-9]+' /tmp/dv2-fb2s.txt` | pass exit=0 | sha256:b00167580a83 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |

Verifier detail (desktools-v2/02 — NON-implementer, merged main 9585b4b6cc2e, 2026-09-27):

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | `test -x tools/desk/scripts/forge-ban.sh; echo rc=$?` | rc=0 | exit 0; printed `rc=0` | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |
| 2 | `sh tools/desk/scripts/forge-ban.sh; echo rc=$?` | prints `forge reach-around sites: <N>`; rc=0 | exit 0; printed `forge reach-around sites: 61 (desk: 29, statusgen: 32)` then per-class lines (a: desk=18 statusgen=29; b: desk=2 statusgen=1; c: desk=1 statusgen=0; d: desk=8 statusgen=2) and `rc=0` | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |
| 3 | `sh -c 'for p in forge_github.go forge_gitlab.go; do grep -qF -- "$p" tools/desk/scripts/forge-ban.sh; ...; done; echo both-exempt'` | exit 0; `both-exempt` | exit 0; printed `both-exempt` (each backend name found separately) | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |
| 4 | `grep -c 'forge-ban' .github/workflows/forge-surface-control.yml` | exit 0; count >= 1 | exit 0; printed `2` (advisory step "Forge reach-around counter (forge-ban, advisory)" runs the script with no failing gate) | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb.txt` | exit 0; `reach-around sites: <N>`, N an integer | exit 0; printed `reach-around sites: 61` | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |
| 6 | `test -f docs/streams/desktools-v2/seam-contract.md && grep -cE -e 'origin' -e 'pullRequest' -e 'api.github.com' docs/streams/desktools-v2/seam-contract.md` | exit 0; count >= 1 | exit 0; printed `10` (the contract's class table names all of classes b, c, d) | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |
| 7 | `sh tools/desk/scripts/forge-ban.sh --baseline && grep -cE '^desktools-v2/02 [0-9]+$' docs/streams/desktools-v2/forge-ban-baseline.txt` | exit 0; count = 1 | exit 0; printed `baseline written: desktools-v2/02 61` then `1` (upsert keeps one line; the verifier's worktree copy of the baseline file was restored to its committed value afterwards) | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |
| 8 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb2s.txt 2>&1; grep -oE 'statusgen sites: [0-9]+' /tmp/dv2-fb2s.txt` | exit 0; `statusgen sites: <N>`, N an integer | exit 0; printed `statusgen sites: 32` | 2026-09-27 assay-verifier-app[bot] (claude-opus-5-5) @ 9585b4b6cc2e |

Execution witness: `statusgen verifyrun --dry-run` (v1.0.27) on the same head: rows 1-8 all `pass exit=0` (output hashes 93ff7811a209, 8b61c43e2c87, ad42e1c04e97, 53c234e5e847, 8443c9eb04b0, 917df3320d77, 16607f48fa96, b00167580a83).

Risk-bearing values (enumerated over the implementing change, #1322: the counter script, the workflow step, the contract doc, the baseline file):
- `BACKEND_EXCLUDE = 'forge_github|forge_gitlab'` @ tools/desk/scripts/forge-ban.sh:59
- `FORGE_GO_EXCLUDE = '/deskkit/forge\.go:'` @ tools/desk/scripts/forge-ban.sh:61
- `GH_SUBCMDS = 'issue|pr|api|auth|repo|release|workflow|run|label|search|browse'` @ tools/desk/scripts/forge-ban.sh:84
- `exit 0` (advisory mode, unconditional) @ tools/desk/scripts/forge-ban.sh:177
- `desktools-v2/02 53` @ docs/streams/desktools-v2/forge-ban-baseline.txt:1
All reversible by an edit and a redeploy (a CI counter shipped advisory; nothing fails a build and nothing touches a token or a funds path).

RISK-VALUE: DERIVED — BACKEND_EXCLUDE = 'forge_github|forge_gitlab' @ tools/desk/scripts/forge-ban.sh:59 — the brief's facts set the exemption as the two backends plus their `_test.go` siblings; prefix matching (no `.go` suffix) is what covers the many `forge_github*_test.go` / `forge_gitlab*_test.go` siblings. Caveat: the pattern is applied to the whole `path:line:content` grep line, so it also exempts a non-backend line that merely mentions a backend file name. That is a small over-exemption, noted but not a Verify failure.
RISK-VALUE: DERIVED — exit 0 @ tools/desk/scripts/forge-ban.sh:177 — the brief puts flipping the gate to failing out of scope (advisory first; desktools-v2/08 flips the statusgen half later).

Observation (not a Verify failure): the live count is 61 (desk 29, statusgen 32). The committed baseline is 53 (desk 24, statusgen 29). Reach-around sites have grown by 8 since the ban-lint landed, and the advisory counter does not stop that. Also, row 7 rewrites the committed baseline file when it runs, so running the Verify table on a checkout that will be committed would silently re-baseline it.

VERIFY: PASS

## Review
Gate: model (all four risk answers no — a CI counter shipped ADVISORY, a portable grep, and a
one-page contract doc; adds a lint per the stream's gate rule, binds no identity/token, edits
no security control). Reviewer records verdict + date in the stream README table. Row 5 is the
dereferencing row (the counter emits a real integer, not a placeholder); row 3 pins the
two-backend exemption the whole ban depends on.
