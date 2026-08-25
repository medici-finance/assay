---
brief: forge-gitlab/01
title: Forge interface extraction in deskkit — github impl pinned by goldens
why: >-
  Every desk tool calls the GitHub API directly, so a second forge means a second fork of
  every tool. Extracting the ~12 forge operations behind one deskkit interface — with the
  current GitHub behavior captured as the first implementation and pinned by goldens —
  makes GitLab (and any later forge) an implementation, not a rewrite, and pays for
  itself as a seam even if no second forge ever ships.
wave: 1
depends: []
unblocks: ["forge-gitlab/02", "forge-gitlab/03"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §6 (interface scope, freeze rule)"
  - "freshness-checked 2026-08-24 @ 5c4a67d — no Forge interface exists in tools/desk/internal/deskkit; tools call the GitHub REST API per-tool"
exec-tier: strong
exec-tier-why: "cross-component refactor of security-relevant plumbing where a subtle behavior change survives per-tool tests (question b and c)."
domain: complicated
consumers:
  - "tools/desk/cmd/*: fixed-here"
  - "tools/desk/internal/deskkit: fixed-here"
---

# Brief 01 — Forge interface extraction in deskkit

## Context
files:
- `tools/desk/internal/deskkit/forge.go` (new) — the `Forge` interface + shared types.
- `tools/desk/internal/deskkit/forge_github.go` (new) — extraction of the existing
  GitHub REST behavior.
- `tools/desk/cmd/*` — call sites move from direct API calls to the interface.

single-point-of-failure: the golden corpus is the one control proving extraction changed
nothing — backed by the existing per-tool tests and by CI staying green on unmodified
tool behavior (two layers failing for different reasons in different components).

facts:
- Interface scope is FROZEN at the operations a shipping tool consumes (spec §6):
  create draft change, comment, approve/review, flip draft, read checks at head, read
  reviews at head, read reactions/awards, file issue, close issue, push-transport
  hints. Additions require a consuming tool in the same change.
- Budgets, rate limiting, breakers, and body checks are forge-agnostic: they WRAP the
  interface and MUST NOT move into either implementation.
- Coordination: the desktools-go-git stream migrates the same tools' git-binary seam.
  Do not run concurrently with an in-flight migration brief of the same tool; rebase
  across whichever lands first.
- Zero behavior change is the deliverable. Goldens capture request/response shapes per
  operation for the github impl before any call site moves.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per
  the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Inventory the forge calls per tool (op → tool → call site); commit the inventory as
   `docs/streams/forge-gitlab/inventory.md` (planned).
2. Define `Forge` in deskkit from the inventory (not from the spec's list alone —
   reconcile and record any delta in the inventory).
3. Implement `forge_github.go` by extraction; add golden tests pinning each operation's
   behavior (request shape, pagination, error mapping).
4. Move call sites tool-by-tool; each tool's existing tests stay green unmodified.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./... && go test ./tools/...` | exit 0 |
| 2 | `grep -rn "api.github.com" tools/desk/cmd --include='*.go' \| grep -v _test.go \| wc -l` | `0` — no direct API construction outside the forge implementation |
| 3 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGithubGolden -v` | exit 0; output contains `PASS` and lists ≥ 10 golden-pinned operations |
| 4 | `git diff --stat origin/main -- tools/desk/internal/deskkit/forge.go` then `go doc ./tools/desk/internal/deskkit Forge` | interface exists; method set matches the committed inventory (dereference: compare `go doc` output against inventory.md rows) |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Reviewer answers the defense-in-depth questions: the golden corpus is the upper
layer; do the unmodified per-tool tests independently catch an extraction fault with a
golden deleted?
