---
brief: desktools-go-git/02
title: gitcore package + in-process transport/auth (BasicAuth) + go-git pin
wave: 2
depends: ["desktools-go-git/01"]
unblocks: ["desktools-go-git/03", "desktools-go-git/04", "desktools-go-git/05", "desktools-go-git/06", "desktools-go-git/07"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-08-21 by desktools-go-git authoring session
sources:
  - "docs/streams/desktools-go-git/spec.md — decisions 1-2 (go-git >= v5.13; one shared gitcore)"
  - "docs/streams/desktools-go-git/brief-01-inventory-and-seam-contract.md — the frozen inventory + golden harness this layer is built against"
  - "Feasibility study: in-process BasicAuth{Username: x-access-token}; go-git support matrix; CVE-2025-21613/21614 floor"
gate-why: >-
  This brief introduces the in-process credential path — the App installation token
  becomes a Go value handed straight to the git pack transport — and it brings a new,
  security-critical dependency tree (go-git + ~20 transitive modules) into a toolset
  whose go.mod deliberately carried a single dependency. Both are sensitive-data-
  handling posture changes a model must not self-certify: a human confirms the auth path
  keeps the token out of disk/env/URL and audits the dependency tree before any tool
  builds on it.
why: >-
  Every migration brief downstream calls gitcore. Standing up the shared package and its
  transport/auth layer once — token in a header inside the tool's own process, exactly as
  desktoken/deskpost/deskevidence/deskrelease already do for REST — is what lets waves 3-4
  be mechanical seam swaps. It also fixes the go-git version floor at the CVE fix line.
---

# Brief 02 — gitcore + in-process transport/auth + go-git pin

## Context

files:
- NEW `tools/desk/internal/gitcore/gitcore.go` (planned) (+ `gitcore_test.go`) — open / resolve /
  refs / objects / diff / log helpers over go-git, plus the transport verbs
  `Fetch` / `Push` / `List`.
- NEW `tools/desk/internal/gitcore/auth.go` (planned) — the `BasicAuth` builder that takes an
  in-memory installation token and returns
  `&githttp.BasicAuth{Username: "x-access-token", Password: token}`.
- `tools/desk/go.mod` + `tools/desk/go.sum` — add `github.com/go-git/go-git/v5` pinned
  `>= v5.13`; `go mod tidy`.
- `docs/streams/desktools-go-git/inventory.md` (planned) — tick the op families this layer now
  covers.

facts:
- Import path for transport auth: `githttp
  "github.com/go-git/go-git/v5/plumbing/transport/http"`; the auth value is
  `&githttp.BasicAuth{Username: "x-access-token", Password: installationToken}`.
  `"x-access-token"` is the literal GitHub-App username; the token is the installation
  token minted via the existing `desktoken` path, kept in memory only.
- go-git floor is `v5.13`: CVE-2025-21613 (argument injection) and CVE-2025-21614
  (denial-of-service) are fixed at that line. Do not pin below it.
- `gitcore.Push`/`Fetch`/`List` each take their OWN `Auth`, so a caller mints a
  repo-scoped token and is structurally unable to send it anywhere but that op's URL —
  which the caller builds from a roster-validated slug. There is no `insteadOf` layer,
  no credential helper, no askpass, no `GIT_*` env: the token never touches disk, the
  child environment, or the URL.
- Force is off unless `Force`/`+` is set on the refspec — "no force possible" becomes a
  type-level property, not argv discipline.
- This brief adds the LAYER and its tests only. It rewires NO tool's seam — those are
  briefs 03-07. Keeping this brief swap-free is what keeps its security review scoped to
  the dependency + auth path.
- Out of scope: any per-tool seam swap; linked worktrees, three-way merge, rebase
  (unsupported by go-git — see spec boundaries).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- The token is in-memory only: never write it to a file, an env var, a URL, or a log
  line. If a test needs a token, use a throwaway fixture value asserted never to leave
  the process.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add `go-git/v5` to `tools/desk/go.mod` pinned `>= v5.13`; `go mod tidy`; commit the
   updated `go.sum`.
2. Implement `internal/gitcore`: the read helpers (open/resolve/refs/objects/diff/log)
   the golden harness from brief 01 can exercise, and the transport verbs
   `Fetch(opts)` / `Push(opts)` / `List(opts)` each accepting an explicit `Auth`.
3. Implement `tools/desk/internal/gitcore/auth.go` (planned): the `BasicAuth` builder with the literal
   `"x-access-token"` username and the in-memory token; add a test asserting the token
   is never serialized into any returned URL/string/log.
4. Golden-verify the read/transport helpers against fixtures using the brief-01 harness
   (outcome snapshots, not argv).
5. Tick the covered op families in `inventory.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/gitcore/` | exit 0 |
| 2 | `cd tools/desk && go test ./internal/gitcore/` | exit 0; transport + auth + read-helper goldens pass |
| 3 | `cd tools/desk && go mod verify` | exit 0; `all modules verified` |
| 4 | `grep -cE -e 'go-git/v5 v5\.1[3-9]' -e 'go-git/v5 v5\.[2-9][0-9]' tools/desk/go.mod` | exit 0; count >= 1 (go-git pinned at or above the v5.13 CVE floor: v5.13-v5.19 or v5.20+) |
| 5 | `grep -cE -e 'x-access-token' -e 'BasicAuth' tools/desk/internal/gitcore/auth.go` | exit 0; count >= 2 (in-process App-token BasicAuth builder present) |
| 6 | `cd tools/desk && go test ./internal/gitcore/ -run TokenNeverLeaves` | exit 0; the token-containment test passes (token absent from returned URLs/strings/logs) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: human (sensitive-data yes — introduces the in-process credential path and a new
security-critical dependency tree into a previously single-dependency toolset). The
human reviewer audits the go-git dependency tree (transitive modules, the v5.13 CVE
floor) and confirms the auth path keeps the token out of disk, env, URL, and logs.
Reviewer records verdict + human name + date in the stream README table.
