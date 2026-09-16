---
brief: assay:assay:desktools-v2:01
title: audit & inventory — enumerate every gh shell-out + hardcoded-forge-assumption site (file:line)
why: >-
  Every later brief removes a reach-around: a place a GitHub fact (a subprocess name, a
  remote name, a query shape, a token scope) was written outside the two Forge backends.
  Before any removal, the stream needs one frozen, file:line-accurate inventory of exactly
  which sites those are, across Go, shell, and skills — so no reach-around is missed and no
  migration guesses. This brief produces that inventory and nothing else; it is the
  checklist every migration brief ticks against.
wave: 1
depends: []
unblocks: ["desktools-v2/02", "desktools-v2/03", "desktools-v2/04", "desktools-v2/05", "desktools-v2/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §1 — the reach-around table this inventory makes file:line-accurate"
  - "tools/desk/internal/deskkit/forge.go — the seam whose bypasses are being counted"
  - "tools/desk/internal/forgeban/allowlist.go — the existing permit-register (const allowedInvocationCeiling now 5) whose rows are a subset of this inventory"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — `forge_github.go` runs on go-gh with a minted token (not a shelled gh); `forgeban` ceiling is 5 (down from 24); `grep -rn 'exec.Command(\"gh\"' tools/desk` returns a small set of Go sites — the shell/skills reach-arounds are NOT yet counted anywhere, which is what this brief fixes"
exec-tier: any
domain: complicated
version: 1
id: 4041a2e5-62b9-47de-9d42-82e530c571b9
---

# Brief 01 — audit & inventory

## Context

files:
- NEW `docs/streams/desktools-v2/inventory.md` (planned) — the frozen file:line table. This
  is the ONLY deliverable; this brief changes no code.

facts:
- The seam is `tools/desk/internal/deskkit/forge.go` (one `Forge` interface, ~four-dozen
  operations). The two sanctioned backends are `forge_github.go` and `forge_gitlab.go`; a
  GitHub fact appearing OUTSIDE those two files (and their tests) is a reach-around and an
  inventory row.
- Four reach-around SHAPES to enumerate, each a column dimension: (a) a `gh` subprocess
  (`exec.Command("gh"`, a `gh`-shim, or a shell script / skill that shells `gh`);
  (b) a hardcoded remote name (`"origin"`); (c) a hardcoded query shape (a `pullRequest` /
  `mergeRequest` GraphQL block, or a REST path fragment) built outside a backend;
  (d) a token/identity assumption (an inherited `GH_TOKEN`, a `HOME` override, a token
  attached only to a child named `gh`).
- The existing `tools/desk/internal/forgeban/allowlist.go` permit-register already lists the
  surviving forge-CLI *Go* call sites (`const allowedInvocationCeiling` = 5 at
  `e9fa19d3`). Its rows are a SUBSET of this inventory — this brief adds the shell and
  skills sites it does not cover, and reconciles against it so no Go site is double-counted
  or dropped.
- Search surfaces: `tools/desk/**` (Go + `scripts/`), `tools/cellctl/**`, `plugins/assay/`
  and `.claude/` skill bodies (shell that shells `gh`), and `.github/workflows/**`.
- This brief asserts NO count in its own frontmatter or prose — the count is the
  deliverable's, produced by the run, not authored from memory.
- Out of scope: any code change; migrating any site; writing the ban-lint (that is
  `desktools-v2/02`).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch (single writer = main's CI).
- Public repo: use `example-*` placeholders for any org/repo name in prose; no absolute
  machine paths, no private slugs, no session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `docs/streams/desktools-v2/inventory.md` (planned): one table keyed by site, with
   columns `# | file:line | tool/skill | reach-around shape (a/b/c/d) | issue | seam op it
   should use (or GAP) | migrating brief`. One row per reach-around site.
2. Enumerate across all four shapes and all search surfaces named in `facts:`. Reconcile the
   Go rows against `tools/desk/internal/forgeban/allowlist.go` explicitly (a `Reconciled:` note under the table
   naming which allowlist rows map to which inventory rows, and any inventory row with no
   allowlist entry).
3. For each row, name the `Forge` op that should replace it, or write `GAP` where no
   enumerated op exists yet (a `GAP` is a signal the interface needs a method — recorded, not
   resolved here).
4. Map each row to the migrating brief (`desktools-v2/03..06`) or `unrouted` if it belongs to
   no wave-3 brief yet.
5. Record the total site count and the per-shape counts in the PR body (produced by the run,
   not asserted here).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/desktools-v2/inventory.md; echo rc=$?` | `rc=0` (the deliverable exists) |
| 2 | `grep -cE -e 'file:line' -e 'reach-around shape' docs/streams/desktools-v2/inventory.md` | exit 0; count >= 1 (the table declares its shape columns) |
| 3 | `grep -cE -e '#1145' -e '#1146' -e '#628' -e '#1019' -e '#1201' -e '#884' -e '#1223' docs/streams/desktools-v2/inventory.md` | exit 0; count >= 7 (every §1 issue is represented as at least one row) |
| 4 | `grep -c 'Reconciled:' docs/streams/desktools-v2/inventory.md` | exit 0; count >= 1 (the forgeban reconciliation note is present) |
| 5 | `bash -c 'for r in $(grep -oE "tools/desk/[A-Za-z0-9_./-]+\.go:[0-9]+" docs/streams/desktools-v2/inventory.md); do f=${r%%:*}; n=${r##*:}; if [ ! -f "$f" ]; then exit 1; fi; if [ "$(wc -l < "$f")" -lt "$n" ]; then exit 1; fi; done; echo ok'` | exit 0; prints `ok` (every cited Go file:line resolves to a real line in the tree — the dereferencing check; fails on an invented citation) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit, key output, date, runner). "verified" requires this filled by
     someone who did NOT implement. -->

## Review
Gate: model (all four risk answers no — a read-only inventory document; no code, no
dependency, no capability change). Reviewer records verdict + date in the stream README
table. Row 5 is the dereferencing check: it fails on a wrong-but-well-formed inventory that
cites a file:line the tree does not contain.
