---
brief: desktools-go-git/01
title: inventory freeze + gitexec single-seam contract + golden harness + counting CI gate
wave: 1
depends: []
unblocks: ["desktools-go-git/02", "desktools-go-git/03", "desktools-go-git/04", "desktools-go-git/07"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-21 by desktools-go-git authoring session
sources:
  - "docs/streams/desktools-go-git/spec.md — thesis, decisions, boundaries"
  - "Feasibility study of migrating tools/desk off the git binary to in-process go-git (25 op families / ~90 call sites / 14 tools, all routed through per-tool exec.go seams)"
why: >-
  Every later brief swaps a per-tool exec seam for a go-git call. Before any swap, the
  stream needs (a) a frozen inventory of exactly which git operations each tool performs
  so no call site is missed, (b) ONE audited git-binary fallback package so the residue
  has a single home, and (c) behaviour goldens so a seam swap is provably outcome-
  preserving rather than argv-preserving. This brief lays those three foundations and
  stands up the CI grep gate in advisory (counting) mode so its baseline is recorded.
---

# Brief 01 — inventory freeze + gitexec seam contract + golden harness + counting CI gate

## Context

files:
- NEW `tools/desk/internal/gitexec/gitexec.go` (planned) (+ `gitexec_test.go`) — the single
  audited `git`-binary fallback seam: one exported runner plus a per-verb allowlist
  (initially the union of today's verbs; later briefs empty it toward one entry).
- NEW `tools/desk/internal/gittest/golden.go` (planned) — a shared behaviour-golden harness
  (build a fixture repo, run an operation, snapshot the OUTCOME — refs/objects/tree
  state/returned values — not the argv).
- NEW `docs/streams/desktools-go-git/inventory.md` (planned) — the frozen op-family table
  (op family -> tool -> seam site -> go-git mapping / gap), lifted from the spec and
  the feasibility study; the checklist every migration brief ticks against.
- NEW `tools/desk/scripts/count-git-exec.sh` (planned) — the CI grep that counts
  `exec.Command("git"` (and `runGit` / `gitOut` seam) sites outside `internal/gitexec`.
- The per-tool seams it inventories: `tools/desk/cmd/deskgit/exec.go`, `tools/desk/cmd/deskmerge/exec.go`,
  `tools/desk/cmd/deskwt/exec.go`, `tools/desk/cmd/deskscanbody/exec.go`, `tools/desk/cmd/deskpr/exec.go`,
  `tools/desk/cmd/deskreply/exec.go`, `tools/desk/cmd/deskadvisory/advisory.go`,
  `tools/desk/cmd/deskpushguard/foreigncommit.go`, `tools/desk/cmd/verifyloop/durable.go`,
  `tools/desk/internal/deskkit/preflight.go`, and the direct `exec.Command("git",...)` one-offs in
  `cmd/writeguard/`, `cmd/deskboard/`, `cmd/desksourceguard/`.

facts:
- Each tool already routes git through a single seam (`runGit` / `execCommand` /
  `gitOut`), which is what makes this migration a seam-swap. This brief does NOT swap any
  seam yet — it only inventories them, creates the fallback home, and stands up the
  harness and the counter.
- `tools/desk/go.mod` today declares exactly one dependency (`yaml.v3`). This brief adds
  NO new module dependency — `go-git` enters in brief 02. The golden harness uses only
  the stdlib plus the existing `git` binary to build fixtures.
- The CI grep starts in **counting** (advisory) mode: it prints the current count and
  exits 0. It flips to failing (non-zero outside the allowlist) only in brief 08, once
  the migrations have driven the count down. Recording the baseline count now is the
  point.
- `internal/gitexec`'s allowlist is a Go value (a set of permitted verb + tool pairs),
  documented as "counted downward to a single entry — `deskmerge`'s trial merge — by the
  end of the stream" (see brief 07).
- Out of scope: any behaviour change; introducing `go-git`; swapping any seam.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `docs/streams/desktools-go-git/inventory.md` (planned): the 25-op-family table keyed by
   op family, listing every seam site per tool and its go-git mapping or gap. This is the
   frozen checklist; later briefs reference rows by op-family number.
2. Create `internal/gitexec`: one exported runner that shells the `git` binary through a
   scrubbed environment (allowlisted env only) and a per-verb+tool allowlist it consults
   before running. Seed the allowlist with today's full verb set. No caller is rewired to
   it in this brief; it is the destination the later briefs point residue at.
3. Create `internal/gittest`: a golden harness that stands up a fixture repo from the
   stdlib + the local `git` binary, runs an operation, and snapshots the OUTCOME
   (resolved SHAs, ref set, tree/blob contents, returned error class) to a golden file —
   asserting outcomes, not argv. Add one worked golden per broad category
   (a read, a diff, a commit) as templates the migration briefs copy.
4. Add `tools/desk/scripts/count-git-exec.sh` (planned): portable (macOS + Linux) grep that counts
   `exec.Command("git"` plus the named seam helpers OUTSIDE `internal/gitexec`, prints
   `git-exec sites: N`, and exits 0. Record the baseline N in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/gitexec/ ./internal/gittest/` | exit 0 |
| 2 | `cd tools/desk && go test ./internal/gitexec/ ./internal/gittest/` | exit 0; golden-harness + gitexec allowlist tests pass |
| 3 | `test -f docs/streams/desktools-go-git/inventory.md && grep -cE -e 'fetch' -e 'push' -e 'merge-base' docs/streams/desktools-go-git/inventory.md` | exit 0; count >= 3 (the op-family table names the transport + plumbing verbs) |
| 4 | `sh tools/desk/scripts/count-git-exec.sh; echo rc=$?` | prints `git-exec sites: <N>`; rc=0 (advisory/counting mode — does not fail the build yet) |
| 5 | `grep -cE -e 'deskmerge' -e 'single sanctioned' -e 'allowlist' tools/desk/internal/gitexec/gitexec.go` | exit 0; count >= 1 (the allowlist documents its intended shrink to the deskmerge exception) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-08-26 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `b734dab`
Runner != implementer. Own isolated worktree off `origin/main`, OFFLINE (`KUBECONFIG=/dev/null`). gate: model, all risk no. Deliverables present: tools/desk/internal/gitexec/, .../gittest/, tools/desk/scripts/count-git-exec.sh, docs/streams/desktools-go-git/inventory.md. `tools/desk` is its own module.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/gitexec/ ./internal/gittest/` | exit 0 | rc=0 — clean build + vet | 2026-08-26 | opus-4.8[1m]-verifier |
| 2 | `cd tools/desk && go test ./internal/gitexec/ ./internal/gittest/` | exit 0; harness + allowlist tests pass | rc=0 — ok gitexec 0.27s, ok gittest 0.70s | 2026-08-26 | opus-4.8[1m]-verifier |
| 3 | `test -f inventory.md && grep -cE -e fetch -e push -e merge-base inventory.md` | exit 0; count >= 3 | rc=0 — count 14 | 2026-08-26 | opus-4.8[1m]-verifier |
| 4 | `sh tools/desk/scripts/count-git-exec.sh; echo rc=$?` | prints 'git-exec sites: <N>'; rc=0 | git-exec sites: 122 (10 direct spawns, 112 seam call sites); rc=0 — the advisory baseline this brief freezes | 2026-08-26 | opus-4.8[1m]-verifier |
| 5 | `grep -cE -e deskmerge -e 'single sanctioned' -e allowlist tools/desk/internal/gitexec/gitexec.go` | exit 0; count >= 1 | rc=0 — 29 | 2026-08-26 | opus-4.8[1m]-verifier |

**RISK-VALUE: DERIVED** — the process-env allowlist (deny-by-default; admits only vars with no execution surface, drops every git env-injection vector, injects GIT_TERMINAL_PROMPT=0) @ tools/desk/internal/gitexec/gitexec.go:117,140, and the sanctioned (tool,verb) allowlist seeded as the union of today's per-tool seam verbs with the deskmerge trial-merge terminal exception @ :47 — both correct by construction and outcome-neutral (no seam swapped in this brief; scaffolding only). No numeric threshold/timeout; no irreversible literal.

## Review
Gate: model (all four risk answers no — repo-internal Go scaffolding + docs; no
dependency added, no seam swapped, the CI grep is advisory-only). Reviewer records
verdict + date in the stream README table.
