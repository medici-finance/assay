---
brief: assay:assay:desktools-go-git:02
title: gitcore package + in-process transport/auth (BasicAuth) + go-git pin
wave: 2
depends: ["desktools-go-git/01"]
unblocks: ["desktools-go-git/03", "desktools-go-git/04", "desktools-go-git/05", "desktools-go-git/06", "desktools-go-git/07"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
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
version: 1
id: f8f94893-c6ec-4e20-916a-ff31eb579350
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
| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./internal/gitcore/` | 0 | build and vet both clean, no diagnostics | 2026-09-11 | opus-5[1m] (Claude Opus 5, 1M context) verifier |
| 2 | `cd tools/desk && go test ./internal/gitcore/` | 0 | `ok github.com/medici-finance/assay/tools/desk/internal/gitcore 2.125s` — 16 tests, all PASS; coverage spans the three families the brief requires: auth containment (TestTokenNeverLeaves), transport outcomes (TestFetchOutcomeMatchesServer, TestPushOutcomeMatchesLocal, TestListMatchesForEachRef, plus two push ref-update CAS/stale-old rejection cases and two fetch tag-payload round-trips), and read-helper goldens (TestGoldenResolveHead, TestGoldenDiffAfterChange, TestRefsMatchesForEachRef, TestFileAtMatchesCatFile, TestFilesMatchesLsTree, TestLogMatchesRevList, TestDiffNamesRenameMatchesGit, TestMergeBaseAndIsAncestor) | 2026-09-11 | opus-5[1m] (Claude Opus 5, 1M context) verifier |
| 3 | `cd tools/desk && go mod verify` | 0 | `all modules verified` | 2026-09-11 | opus-5[1m] (Claude Opus 5, 1M context) verifier |
| 4 | `grep -cE -e 'go-git/v5 v5\.1[3-9]' -e 'go-git/v5 v5\.[2-9][0-9]' tools/desk/go.mod` | 0 | count `1`. Read the actual number rather than trusting the pattern: the pin is `go-git/v5 v5.19.2`, above the v5.13 CVE floor. Resolved-version cross-check (a `>=` in go.mod does not by itself prove selection): `go list -m github.com/go-git/go-git/v5` reports `v5.19.2` and go.sum carries v5.19.2 only — the selected build is the pinned one | 2026-09-11 | opus-5[1m] (Claude Opus 5, 1M context) verifier |
| 5 | `grep -cE -e 'x-access-token' -e 'BasicAuth' tools/desk/internal/gitcore/auth.go` | 0 | count `14` (>= 2 required). The literal username is a named constant assigned the value `x-access-token`, used as the `Username` field of the returned go-git HTTP `BasicAuth` value | 2026-09-11 | opus-5[1m] (Claude Opus 5, 1M context) verifier |
| 6 | `cd tools/desk && go test ./internal/gitcore/ -run TokenNeverLeaves` | 0 | PASS. Read the test source rather than only its exit status: it makes six real containment assertions, not a smoke call. It asserts the fixture token is absent from four string renderings of the auth value (`.String()`, and `%v` / `%+v` / `%s` formatting), then drives two genuine transport failure paths against a nonexistent remote — Fetch and List — and asserts the token is absent from each returned error string. The fixture token is a throwaway literal deliberately not shaped like a real token prefix | 2026-09-11 | opus-5[1m] (Claude Opus 5, 1M context) verifier |

All six Verify rows pass on merged main at short SHA `86c7d62c`. Deliverables confirmed present:
`tools/desk/internal/gitcore/gitcore.go`, `tools/desk/internal/gitcore/auth.go`,
`tools/desk/internal/gitcore/gitcore_test.go` (plus `auth_test.go`), the `tools/desk/go.mod` and
`tools/desk/go.sum` pin, and the ticked op families in
`docs/streams/desktools-go-git/inventory.md`.

## Security substance checks

The four posture claims the brief's `gate-why` rests on were checked against the source, not
only against the Verify rows. All four PASS.

1. **No `insteadOf`, no credential helper, no askpass, no `GIT_*` environment — PASS.** Each of
   those four terms appears in the package *only inside comments asserting its own absence*;
   none appears in executable code. The package contains no `os.Setenv` / `os.Getenv` /
   `os.Environ` call at all, and no `exec.Command` outside the test files (the two test
   occurrences shell out to `git init` and fixture setup, never with a credential). The
   structural reason the `insteadOf` class cannot apply: each transport verb builds a *fresh,
   unstored* remote per call from a `RemoteConfig` whose URL is the literal string the caller
   passed, so no URL is ever resolved through repository config or a configured remote alias.
2. **Each of `Push` / `Fetch` / `List` takes its own explicit `Auth` — PASS.** `FetchOpts`,
   `PushOpts` and `ListOpts` each carry their own `Auth` field of the go-git transport
   auth-method type, passed per call into the corresponding go-git options value. There is no
   package-level credential state for it to leak through: the only package-level identifiers are
   constants (the two forge usernames, the transient remote name, an empty-blob hash, the claim
   tagger identity) and one diff-options value. A caller therefore cannot have auth from one
   repo-scoped call observed by another.
3. **Force-push is off by default, type-level — PASS.** `Force` is a plain boolean field on
   `FetchOpts` / `PushOpts`, so the Go zero value is `false`, and the refspec builder prepends
   the force marker only when it is explicitly true. Enabling force takes a deliberate act per
   call. Noted for completeness, consistent with the brief's own wording ("Force is off unless
   `Force`/`+` is set on the refspec"): a caller can also reach force by passing a refspec
   string that already begins with the force marker, which the builder passes through. Both
   paths are explicit and neither is the default.
4. **CVE floor is real at the resolved version, not just the stated minimum — PASS.** The
   actually selected version is v5.19.2 (`go list -m`, corroborated by go.sum), clearing both
   named advisories, which are fixed at the v5.13 line. An independent vulnerability scan of
   the package (Go's own vulnerability scanner, symbol-reachability mode) confirms this from the
   other direction: neither named go-git advisory is reported, and **no advisory is reported
   against go-git itself**.

Adversarial cross-check beyond the brief's rows: every error-formatting site in the package was
enumerated and read. Not one interpolates an auth value, a password, a token, or a request URL —
the interpolated values are directory, revision, path, object hash, refspec, ref name and
protocol status only. This is the failure mode row 6 exercises dynamically, confirmed here
statically across the whole package rather than on the two paths the test drives.

## Advisory note for the human reviewer (not a brief-02 defect)

The brief's `gate-why` puts the transitive-module audit in the human reviewer's remit, so this
belongs in front of that gate. The vulnerability scan above reported six findings, none in
go-git. Four are Go standard library items fixed in the next patch release of the toolchain — a
repo-wide toolchain bump, unrelated to this brief. The remaining two are denial-of-service
advisories in the `golang.org/x/crypto` module at the version the new tree resolves, reported as
reachable only through the SSH client path, and reached via `claimref.go` — a file added to this
package by later work, not one of brief 02's deliverables. The brief-02 surface under review
here (the read helpers plus the HTTP transport verbs and the auth builder) does not use that
path. Recording it rather than acting on it: enforcing and alerting on the dependency floor is
explicitly `desktools-go-git/08`'s scope, which owns the CI floor assertion and the dependency
alerting for this module. No brief-02 Verify row or security claim is affected.

## Verdict

**VERIFY: PASS** — six of six Verify rows pass on merged main, all four security substance
checks pass, and the token-containment claim holds under both the brief's own test and an
independent static sweep of the package.

HUMAN GATE: this brief is `gate: human` (`sensitive-data: yes`); the human security review of
the dependency tree and the auth path remains outstanding and is tracked on issue #903. This
verification fills Evidence and advances the row to `verified` only. The `Reviewed` column stays
empty for the human reviewer to record verdict, name and date, and the advisory note above is
addressed to that review.

## Review
Gate: human (sensitive-data yes — introduces the in-process credential path and a new
security-critical dependency tree into a previously single-dependency toolset). The
human reviewer audits the go-git dependency tree (transitive modules, the v5.13 CVE
floor) and confirms the auth path keeps the token out of disk, env, URL, and logs.
Reviewer records verdict + human name + date in the stream README table.
