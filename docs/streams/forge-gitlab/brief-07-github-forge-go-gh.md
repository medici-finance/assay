---
brief: forge-gitlab/07
title: GitHub forge backend on go-gh — retire the exec-`gh` shell path
why: >-
  The GitLab backend (brief 02) talks REST v4 over a library, but the GitHub backend still
  reaches the forge by shelling the full `gh` CLI — which exposes the entire `gh <anything>`
  subcommand-and-flag surface plus a shell-exec surface, requires a version-matched `gh`
  installed on every runner, and routes auth through gh's ambient keyring rather than the
  desk identity. Re-seating GitHub on the official `go-gh` library (REST + GraphQL clients +
  auth resolution) behind brief 01's interface makes both backends API-behind-one-interface,
  drops a runtime dependency and a shell surface, and binds GitHub auth to the minted desk
  token — the precondition for closing the surface in brief 08.
wave: 2
depends: ["forge-gitlab/01"]
unblocks: ["forge-gitlab/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-26 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §6 (interface scope, concept mapping, freeze rule)"
  - "docs/streams/forge-gitlab/brief-01-forge-interface-extraction.md — the Forge seam + golden corpus this re-implements against"
  - "docs/streams/forge-gitlab/brief-02-gitlab-forge-impl.md — the library-backed peer this brings GitHub to parity with"
  - "docs/streams/forge-gitlab/inventory.md — the frozen 14-op method set; op 8/12/13 name the current `gh pr/issue` shell paths retired here"
  - "freshness-checked 2026-08-26 @ e89cf5a — no `github.com/cli/go-gh` import anywhere in tools/; 20 `exec.Command(\"gh\", …)` call sites live under tools/desk/cmd/**, and deskpr injects auth as GH_TOKEN into the shelled gh (tools/desk/cmd/deskpr/exec.go)"
exec-tier: strong
exec-tier-why: "swaps the transport under a security-relevant, golden-pinned backend: a subtle divergence (pagination, error mapping, auth source, draft semantics) survives happy-path tests but breaks parity with the goldens (question b and c)."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit: fixed-here (the github backend + its tests)"
  - "tools/desk/cmd/deskpr, deskfile, deskclose: fixed-here (forge-op call sites move off shelled gh onto the go-gh-backed backend)"
---

# Brief 07 — GitHub forge backend on go-gh

## Context
files:
- `tools/desk/internal/deskkit/forge_github.go` — re-seat the GitHub `Forge` implementation
  on `github.com/cli/go-gh/v2` (its `pkg/api` REST + GraphQL clients and `pkg/auth`
  resolution) in place of hand-rolled `net/http` and any shelled-`gh` path left by brief 01.
- `tools/desk/internal/deskkit/forge_github_test.go` (planned) — the golden corpus from brief 01,
  re-run unchanged against the go-gh-backed backend (same scenario names, same expectations).
- `tools/desk/cmd/deskpr/exec.go`, `tools/desk/cmd/deskfile/exec.go`,
  `tools/desk/cmd/deskclose/exec.go` — the forge operations these tools reach via
  `exec.Command("gh", …)` (inventory ops 8, 12, 13: `gh pr create --draft`,
  `gh issue create`, `gh issue close`) move onto the backend; the shelled-gh forge path is
  deleted, not left dormant.
- `go.mod` / `go.sum` — add the `go-gh` dependency (vendored/pinned per repo convention).

single-point-of-failure: the golden corpus is again the one control proving the transport
swap changed no observable behavior — the SAME scenarios brief 01 pinned for the extracted
backend must pass unmodified against the go-gh-backed one, backed independently by each
tool's existing per-tool tests staying green (two layers, different components).

facts:
- The interface is FROZEN (spec §6): this brief changes the GitHub backend's TRANSPORT and
  its call sites, never the `Forge` method set. No method is added or removed here — that is
  brief 08's surface work under the same freeze rule (a consuming tool per added op).
- Auth binds to the desk token, not gh's ambient config: the backend authenticates from the
  minted token value the tool already holds (deskpr mints the worker token today and injects
  it as `GH_TOKEN` into the shelled gh — `tools/desk/cmd/deskpr/exec.go`). go-gh resolves a
  token from the environment/host config; the backend MUST be given the minted token
  explicitly and MUST NOT fall back to an ambient gh-CLI keyring identity — preserving
  deskpr's existing refuse-if-unminted guard (mirrors #562/#563) rather than weakening it.
- Behavior to preserve exactly, pinned by goldens: pagination (GitHub `Link` rel-next across
  reviews/files/statuses), error mapping (401 credential vs 403 permission vs 404 visibility,
  surfaced as could-not-check, never clean), draft-PR create + GraphQL
  `markReadyForReview` flip, and head-SHA pinning on review/check reads.
- Budgets, rate limiting, breakers, and body/secret checks WRAP the interface (spec §6) and
  stay put — this brief must not pull any of them into the backend.
- Removes a runtime dependency (the `gh` binary need no longer be installed/version-matched
  on runners for these ops) and a shell-exec surface. The fleet-wide assertion that NO
  exec-gh remains anywhere is brief 08; this brief retires only the forge-operation call
  sites it re-seats.
- Coordination (README): the desktools-go-git stream refactors the same tools' git-binary
  seam; this brief touches only the forge-API seam of the named tools. Do not run
  concurrently with an in-flight desktools-go-git migration brief of the same tool; rebase
  across whichever lands first.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add `github.com/cli/go-gh/v2` to the module (pinned per repo convention) and build a thin
   backend client wrapper that takes an explicit minted token and yields go-gh REST + GraphQL
   clients — no ambient-keyring fallback.
2. Re-implement each `Forge` operation in `forge_github.go` on that client, deleting the
   hand-rolled `net/http` construction and the shelled-gh forge path. Keep method signatures
   and semantics identical to brief 01.
3. Move the forge-op call sites in `deskpr`/`deskfile`/`deskclose` off `exec.Command("gh", …)`
   onto the backend; delete the now-dead shell helpers for those ops. Preserve deskpr's
   refuse-if-unminted auth guard.
4. Re-run brief 01's golden corpus unchanged against the go-gh-backed backend; keep every
   affected tool's existing tests green unmodified.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./... && go test ./tools/...` | exit 0 |
| 2 | `grep -rl "github.com/cli/go-gh" tools/desk/internal/deskkit --include='*.go' \| grep -v _test.go \| wc -l` | `≥ 1` — the GitHub backend is built on the official library |
| 3 | `grep -rnE 'exec\.Command(Context)?\([^)]*"gh"' tools/desk/cmd/deskpr tools/desk/cmd/deskfile tools/desk/cmd/deskclose --include='*.go' \| grep -v _test.go \| wc -l` | `0` — the re-seated tools no longer shell `gh` for forge operations |
| 4 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGithubGolden -v` | exit 0; output contains `PASS` and lists ≥ 10 golden-pinned operations — the brief-01 corpus passes unchanged against the go-gh backend |
| 5 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGithubAuth -v \&\& go test ./tools/desk/internal/deskkit/ -run TestForgeGithubTierErrors -v` | exit 0 on both; a test proves the backend authenticates from the injected token and refuses (not silently falls back to ambient gh identity) when unset, and that a 403 fixture surfaces `could-not-check` distinct from empty |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers the defense-in-depth questions: the golden corpus is the upper layer proving
transport-swap equivalence — do the unmodified per-tool tests independently catch a go-gh
divergence (pagination, error mapping, auth source) with a golden deleted? And is the ambient
gh-keyring fallback provably closed, not merely unused on the happy path?
