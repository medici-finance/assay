---
brief: forge-gitlab/08
title: Close the forge surface — enumerated operations, no passthrough, shell-exec ban
why: >-
  A constrained typed surface is stronger than an ambient full-CLI one: the spec's governing
  requirement is that the forge profile be at least as secure as the existing GitHub controls,
  and shelling `gh`/`glab` (or exposing a generic `Do(endpoint)` escape hatch) re-opens the
  entire CLI/API surface the interface was meant to fence. This brief makes the fence
  enforceable — the Forge exposes ONLY the enumerated operations the assay workflows need, no
  arbitrary-endpoint method survives on either backend, and a lint/test asserts zero
  `exec.Command("gh"|"glab", …)` anywhere in tools/desk — so "limit the tool to our workflows,
  not a wide-open CLI" is a checked property, not a convention.
wave: 3
depends: ["forge-gitlab/02", "forge-gitlab/07"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-26 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §3 (parity table — a constrained surface is the stronger side), §6 (interface FROZEN at consumed operations; additions require a consuming tool)"
  - "docs/streams/forge-gitlab/brief-07-github-forge-go-gh.md — the go-gh GitHub backend the newly-enumerated ops land on"
  - "docs/streams/forge-gitlab/brief-02-gitlab-forge-impl.md — the GitLab backend the ban applies to symmetrically (no glab shell, no arbitrary-endpoint method)"
  - "docs/streams/forge-gitlab/inventory.md — the frozen op set the closed surface is measured against"
  - "freshness-checked 2026-08-26 @ e89cf5a — 20 `exec.Command(\"gh\", …)` call sites remain across tools/desk/cmd/** (deskflip, deskboard, deskmerge, deskdispatch, deskroster, deskadvisory, deskpushguard, issueboard, scanloop, repohardenguard, deskdisposition), and fanoutloop reaches an arbitrary endpoint via `gh api -X DELETE repos/…/git/refs/…` (tools/desk/cmd/fanoutloop/land.go) — a passthrough the enumerated surface must replace with a typed op"
exec-tier: strong
exec-tier-why: "a security-control brief where the deliverable IS the control: a gap in the ban's detection (a missed callsite form, a passthrough method left reachable) ships a false-clean, which is exactly the failure the parity requirement forbids (question c)."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit: fixed-here (the ban lint/test + any newly-enumerated ops)"
  - "tools/desk/cmd/*: fixed-here (residual gh call sites route through the interface or are removed)"
---

# Brief 08 — Close the forge surface

## Context
files:
- `tools/desk/internal/deskkit/forge.go` — the frozen interface: confirm NO generic
  `Do`/`Raw`/`APIRequest`/arbitrary-endpoint method exists; add typed ops for the residual
  shell callsites that are genuine forge operations (e.g. a `DeleteRef(repo, ref)` to retire
  fanoutloop's `gh api -X DELETE`, a `ListPullRequests`/`SetLabels`/`ReadyForReview` op where
  a tool shells one) — each addition carries its consuming tool in this change (spec §6).
- `tools/desk/internal/deskkit/forge_github.go`, `forge_gitlab.go` — implement each newly
  enumerated op on both backends (GitHub via go-gh per brief 07; GitLab via REST v4 per brief
  02); neither backend exposes an arbitrary-endpoint escape hatch.
- `tools/desk/internal/deskkit/forge_surface_test.go` (new) — the two enforcement tests
  (shell-exec ban + no-passthrough), runnable in CI.
- `tools/desk/cmd/**` — the residual `exec.Command("gh"|"glab", …)` forge callsites
  (deskflip, deskboard, deskmerge, deskdispatch, deskroster, deskadvisory, deskpushguard,
  issueboard, scanloop, repohardenguard, deskdisposition, fanoutloop) move onto the interface
  or are removed; the `askassay` gh-probe entry that only exists to check the `gh` binary is
  installed is dropped, since no tool requires the binary any more.

single-point-of-failure: the ban's detection logic is the one control — if it misses a
callsite spelling or a reachable passthrough, it ships a false-clean. It is backed by a
second, independent layer: the interface's method set is asserted by reflection to equal the
committed inventory (no method takes an arbitrary endpoint/path), so a passthrough that slips
the string-level ban is still caught by the shape check (two layers, different mechanisms).

facts:
- Spec §6 freeze rule is the authority for every op added here: the interface stays frozen at
  the operations a SHIPPING tool consumes; each newly-enumerated op MUST land with its
  consuming callsite converted in the same change — no speculative ops.
- The ban targets forge-CLI INVOCATIONS, not string mentions: `exec.Command`/
  `exec.CommandContext` (and the tools' `runCmd`/`run` wrappers) invoking `gh` or `glab`.
  `askassay`'s read-only gh GUARD validates an agent's ad-hoc gh argv — it does not invoke
  gh — and is out of scope except for dropping the now-unnecessary binary-present probe entry.
  Legitimate non-invocation references (test fixtures, the ban's own pattern literal) must not
  trip the check — detect callsites structurally / exclude `_test.go`, do not naive-grep.
- No arbitrary-API method on EITHER backend: no `Do(endpoint)`, no `api(path)` passthrough,
  no exported raw-request method. fanoutloop's `gh api -X DELETE repos/…/git/refs/…` is the
  live example — it becomes a typed `DeleteRef` op, not a generic call.
- Symmetric on GitLab (brief 02): the same tests assert zero `glab` shell (currently trivially
  true — preventive) and no arbitrary-endpoint method on `forge_gitlab.go`, so the closed
  surface is a property of the interface, not of one backend.
- This is the parity deliverable named in spec §3: a constrained typed surface is the STRONGER
  side of the GitHub-controls comparison, not merely equal — an ambient full-CLI/keyring
  surface is what it replaces.
- Coordination (README): touches `tools/desk/**` broadly; land after brief 07 (GitHub on
  go-gh) and brief 02 (GitLab backend) so every residual op has a typed home, and rebase
  across any in-flight desktools-go-git migration of a shared tool rather than running
  concurrently.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Inventory the residual `gh`/`glab` shell callsites under `tools/desk` and classify each:
   forge operation (→ enumerate + route through the interface) vs. non-forge (out of scope,
   record why). Extend `inventory.md` with the added ops and their consuming tools.
2. Add the typed ops the forge callsites need to `forge.go`; implement on both backends
   (go-gh + REST v4); convert each callsite; delete the dead shell helpers. Drop askassay's
   gh-binary probe entry.
3. Write the shell-exec ban test (fails on any `gh`/`glab` invocation in tools/desk non-test
   code) and the no-passthrough test (reflects the interface method set against the committed
   inventory; fails on any arbitrary-endpoint method on either backend).
4. Keep every affected tool's existing tests green unmodified.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./... && go test ./tools/...` | exit 0 |
| 2 | `go test ./tools/desk/... -run TestNoForgeCLIShellout -v` | exit 0; output contains `PASS` — the ban test passes because no `gh`/`glab` invocation remains |
| 3 | `grep -rnE -e 'exec\.Command(Context)?\([^)]*"gh"' -e 'exec\.Command(Context)?\([^)]*"glab"' tools/desk --include='*.go' \| grep -v _test.go \| wc -l` | `0` — independent cross-check of the ban across the whole desk tree |
| 4 | `go test ./tools/desk/internal/deskkit/ -run TestForgeNoPassthrough -v` | exit 0; the test reflects `deskkit.Forge`'s method set against `inventory.md` and FAILS on any generic/arbitrary-endpoint method (`Do`/`Raw`/`api`) on the interface or either backend |
| 5 | `go doc ./tools/desk/internal/deskkit Forge \| grep -cE -e 'Do\(' -e 'Raw\(' -e 'APIRequest\(' -e 'Call\('` | `0` — no arbitrary-request method surfaces in the interface's godoc |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers the defense-in-depth questions: the string-level ban is the upper layer —
does the reflection/shape check independently catch a passthrough method that evades the
callsite grep (e.g. a raw request reachable through an exported helper)? And is the ban's
detection provably resistant to a re-introduced callsite in a spelling the test does not
match — i.e. does it enumerate `exec` forms structurally rather than by one literal pattern?
