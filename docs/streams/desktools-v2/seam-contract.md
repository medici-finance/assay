# The v2 seam contract

One page. The seam is `tools/desk/internal/deskkit/forge.go` — one `Forge` interface, two
complete backends (`forge_github.go`, `forge_gitlab.go`). Every desk tool that talks to a
forge does it through a `Forge` value obtained from `ResolveForge` (`forgeresolve.go:451`,
the single construction site pinned by `TestForgeSingleConstructionSite`). A GitHub- or
GitLab-specific fact appearing anywhere else — outside the two backend files and their
`_test.go` siblings — is a **reach-around**: the seam exists, but the call site went past it.

## The four fact classes (the ban)

Each class may appear **only** inside `forge_github.go` / `forge_gitlab.go` and their tests.
One exception is noted under (d).

| Class | Shape | Where it's allowed |
|---|---|---|
| (a) | a `gh` subprocess — `exec.Command("gh", …)`, a shim, or a shell script that shells `gh` | `forge_github.go` (it does not shell `gh` today — it runs on `go-gh`'s REST/GraphQL client — but a future `gh`-CLI fallback would belong here, not elsewhere) and its tests |
| (b) | the remote name `"origin"` hardcoded to decide **which forge or which repo** a `Forge` operation targets | `forgeresolve.go`'s `originRemoteHost` (a documented fallback of last resort, consulted only when the roster has no entry) |
| (c) | a `pullRequest` / `mergeRequest` GraphQL query block built outside a backend | `forge_github.go`'s own GraphQL documents |
| (d) | the GitHub REST/GraphQL host literal `api.github.com` | `forge.go`'s `GitHubAPIBase` constant — **the one carve-out**: the literal's single canonical home is the seam's own interface file, not one of the two backends. Every other desk command sources the host from `GitHubAPIBase` / `GitHubBaseURLOrDefault`, never by restating the string. |

## "Construction only inside a backend"

A `Forge` implementation (`GitHubForge{}`, `GitLabForge{}`) may be built in exactly one place,
`forgeresolve.go`. This is a distinct control from the four classes above — the ban says WHERE
a GitHub/GitLab *fact* may appear; the single-construction-site check
(`TestForgeSingleConstructionSite`) says where a backend may be *built*. Together with the
no-passthrough shape check (`TestForgeNoPassthrough` — no generic method, no caller-supplied
endpoint, neither backend exports a method outside the frozen surface), these three controls
are what `.github/workflows/forge-surface-control.yml` already runs. This brief's counter
(`tools/desk/scripts/forge-ban.sh`) is a fourth, independent layer over the same seam: those
three are Go-level (AST-resolved, `go test`), this one is a portable grep across every
extension the four classes can appear in (Go, shell, skill scripts) — including `statusgen/**`,
which the Go-level controls do not reach because `forgeban` (`internal/forgeban`) is scoped to
`tools/desk/**` only.

## Scope includes statusgen — enforcement, not migration

statusgen is a separate Go module that does not import `deskkit`, by design
(`statusgen/forgeread.go` header): it reaches the seam by *running* the `deskread` verb, never
by linking `deskkit`. Its `gh`-shelling sites (26 at `desktools-v2/01`'s baseline) are migrated
by the sibling brief `forge-neutral/18`, not by this stream. This counter brings
`statusgen/**` under the same rule so that migration's progress is visible as a falling count
and `desktools-v2/08` can hold the zero once it lands — it does not propose a client, a
library, or a port for statusgen (out of scope, same as `desktools-v2/01`).

## The counter's per-class scope (why class (b) is narrower than a blanket grep)

`desktools-v2/01`'s inventory found ~30 more `"origin"` literals across `tools/desk/**`
(`deskwt`, `deskpr`, `deskmerge`, `deskflip`, `scanloop`, `verifyloop`, …) and excluded every
one of them from its own table **by design, not oversight**: those are ordinary git-transport
plumbing — "push/fetch this worktree's own configured `origin` remote" — not a decision about
*which forge or which repo* a `Forge` operation targets, and folding them in "would dilute the
count the ban-lint needs to be meaningful" (the inventory's own words, naming this brief).
That class of `"origin"` usage belongs to `desktools-go-git` (spec.md §4), not this stream.

Rather than grep every `"origin"` literal in `tools/desk/**` — which would immediately fold in
that same ~30-site git-transport noise and make the count useless as a signal for a *new*
forge-identity bypass — the counter scopes class (b) to files whose name contains `forge` or
`resolve` (`*forge*.go`, `*resolve*.go`, both trees), which is where a forge/repo-identity
decision actually lives (`forgeresolve.go`, statusgen's `forge.go`). This is a **known,
documented narrowing**, not a silent exclusion: a new "which forge" hardcode landing in a file
without `forge`/`resolve` in its name would not be caught by class (b) today. Classes (a),
(c) and (d) are NOT narrowed this way — a `gh` subprocess literal, a GraphQL `pullRequest(`/
`mergeRequest(` block, and the `api.github.com` host string are precise enough patterns that a
whole-tree grep does not dilute (calibrated against the tree at this brief's baseline: class
(d) surfaces real sites outside the seam, e.g. `tools/desk/internal/deskkit/compositionsource.go`'s
hardcoded release-URL construction, that a narrower scope would have hidden).

## Known limitations of a grep-based counter (v1, advisory)

- Comments and doc strings match the same patterns as code (e.g. `api.github.com` mentioned in
  a comment counts the same as a literal in a URL construction). The count is therefore an
  upper bound, not an exact reach-around tally — consistent with `desktools-go-git`'s
  `count-git-exec.sh`, which counts every `git` spawn the same way, advisory first.
- `tools/desk/internal/deskkit/trustfetch.go`'s `PRTrustQuery`/`IssueTrustQuery` constants are a single
  shared definition (like `GitHubAPIBase`) consumed by both a backend and
  `tools/desk/cmd/deskpost/github.go`'s duplicate client; they are **not** exempted the way
  `GitHubAPIBase` is, so they count under class (c). A future brief may want to extend the
  (d)-style carve-out to this pair once the query constants have exactly one canonical home
  the way the host literal does.
- `tools/desk/cmd/deskpost/github.go` (17 sites, `desktools-v2/01` inventory group E) is the largest single
  finding: a second, hand-rolled GitHub REST+GraphQL client, entirely outside both backends.
  No v2 brief currently names it — it is counted, not hidden, and its migration is unrouted.
