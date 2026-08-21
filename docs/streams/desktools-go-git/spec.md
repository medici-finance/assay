# desktools-go-git — spec

## Thesis

The desk tools authenticate to GitHub only through App installation tokens, yet every
git operation still shells out to the `git` binary. That binary is an escape-hatch
surface in its own right: `git` executes programs named by **config**
(`core.sshCommand`, `core.gitProxy`, `core.fsmonitor`, `remote.<n>.vcs`) and by
**environment** (`GIT_SSH_COMMAND`, `GIT_ASKPASS`, `GIT_CONFIG_*`), honours `insteadOf`
identity substitution, runs hooks, and trusts `PATH` for which `git` even runs. The
codebase already carries the scars of defending it vector-by-vector — one tool is a
~70-line threat-model essay whose sole job is to harden a single `fetch`, a second
re-implements the same hardening for third-party-fork fetches, and pushes inherit
whatever ambient credential machinery (credential helpers, the `:443` `insteadOf`
dodge, keychain state) the environment happens to carry, to the point that a preflight
has to *probe* what its own transport will do.

An in-process library executes **no external program, reads no credential helper,
honours no `insteadOf`, runs no hooks, and trusts no `PATH`**. The whole threat-model
class collapses structurally instead of being fenced one vector at a time. This stream
moves the desk tools' git operations in-process onto `go-git`, with the App
installation token handed **directly** to the transport as a Go value —
`BasicAuth{Username: "x-access-token", Password: <installation-token>}` — so the tools
carry no git-binary dependency and no credential-helper / `:443`-`insteadOf`
machinery. This reinforces the App-token-only posture rather than bolting more fences
onto the binary.

The precedent is already in-house: `desktoken`, `deskpost`, `deskevidence`, and
`deskrelease` already do their GitHub writes in-process over HTTPS with the token in a
header — no `git`, no `gh`. This stream extends that same pattern from the REST API to
the git pack transport.

## Decisions taken

1. **Library: `go-git` (pure Go), pinned `>= v5.13`.** Pure Go keeps the desk-tools
   posture of trivially cross-compiled, pinned, per-platform static release binaries
   (the release pipeline harvests per-platform sha256s of pure-Go builds). `v5.13` is
   the floor because CVE-2025-21613 (argument injection) and CVE-2025-21614
   (denial-of-service) are fixed there. `git2go`/`libgit2` — which *would* cover
   go-git's gaps (three-way merge, rebase, worktrees) — is rejected: it is
   archived/unmaintained, it is CGO (which breaks the static cross-compiled release
   flow), and a C parser surface processing attacker-influenceable repo data is a worse
   trade than the escape hatch it removes.

2. **One shared `internal/gitcore` package** holds the in-process helpers
   (open / resolve / refs / objects / diff / log) and the transport verbs
   (`Fetch` / `Push` / `List`) with in-process `BasicAuth`. Every migrated tool routes
   through it. The token flows `desktoken` -> memory -> `Authorization` header inside
   the tool's own process; it never touches disk, an askpass script, or the child
   environment.

3. **The migration is a seam-swap, not a rewrite.** Every tool already routes git
   through a single per-tool seam (`runGit` / `execCommand` / `gitOut` in each tool's
   `exec.go`). Swapping the seam keeps ~90 call sites across ~14 tools and 25 op
   families intact. The existing argv-asserting tests convert to behaviour goldens
   (same repo fixtures, assert *outcomes* not argv) so the swap is verifiable without
   rewriting intent.

4. **One audited `git`-binary fallback, counted down to a single caller.** The few
   genuinely unsupported verbs retreat behind one allowlisted seam. A CI gate counts
   `exec.Command("git"` sites, and by the end of the stream fails on any outside that
   allowlist.

5. **`deskmerge`'s trial merge stays on the `git` binary — a decided EXCEPTION, not a
   deferral.** `go-git`'s merge is fast-forward-only with no three-way merge and no
   conflict-stage enumeration, so it cannot express `deskmerge`'s
   `merge --no-ff --no-commit` trial merge, conflicted-file enumeration, parent
   verification, and "rolled back; nothing pushed" posture — the desk's most
   security-reasoned git write. `deskmerge merge` is human-gated, run only on the desk
   machine, never by agents. It therefore remains the **one sanctioned git-binary
   caller**, fenced and allowlisted in the fallback seam and documented as such. Its
   non-merge read/commit verbs still migrate to `gitcore`.

## Boundaries (in scope / out of scope)

**In scope** — the DESK TOOLS' git operations:

- The read-only / plumbing verbs across the read-heavy tools (`writeguard`,
  `desksourceguard`, `deskboard`, `deskscanbody`, `deskpushguard`, the `deskkit`
  preflight read paths, and the read/config paths of `deskwt`, `deskgit`, `deskpr`,
  `deskreply`).
- The transport verbs: `fetch` (`deskgit`, `deskadvisory` third-party-fork,
  `deskmerge` base/PR), `push` (`deskpr`, `deskreply`, `verifyloop`), and the
  preflight transport probe (replaced by an authenticated `List`).
- The `deskmerge` exception: fence its trial merge as the sole sanctioned git-binary
  caller; migrate its other verbs.
- The drop-the-binary CI gate and the `go-git >= v5.13` CVE floor.

**Out of scope** — a NAMED FOLLOW-ON stream, deliberately not attempted here:

- **Removing the `git` binary from the AGENT environment.** Agents themselves run
  `git add` / `commit` / `checkout` inside the linked worktrees `deskwt` creates, and
  `go-git` has **no linked-worktree support** (its "worktree" is one repo's checkout,
  not `git worktree add` — no linked-worktree creation, listing, locking, or pruning).
  Agent-facing worktrees must stay real linked worktrees while agents still run `git`.
  Fully dropping the binary needs a desk-tool verb family covering the agent's own
  commit loop plus a linked-worktree replacement — a dependent stream, not this one.
  The final brief files it.
- **`pull --rebase` in `verifyloop`'s durable-Evidence push race.** `go-git` supports
  neither `rebase` nor non-fast-forward `pull`. Redesigning that loop
  (regenerate-on-new-tip or the contents API the house already trusts) is a small
  design brief left to the follow-on; this stream migrates only the push half of that
  loop onto `gitcore.Push`.

## go-git coverage at a glance

- **Full / clean map:** fetch and push with exact refspecs and in-process `BasicAuth`;
  plain `add`; `commit` (with explicit `Author`/`Parents` — `--no-verify` is inherent
  since no hooks ever run); `checkout <sha>`; `init`; `status`; ref reads
  (`ResolveRevision`, `Head`, `References`); object reads (`CommitObject`, tree walks,
  file contents); `Log`; `merge-base` + `IsAncestor`; `diff` (name-only + unified, with
  rename detection); `remote get-url`; ref deletion.
- **Vectors that simply disappear:** the fetch-hardening flags (no upload-pack override
  to pin, no remote helpers, no submodule recursion, no hooks); `ls-remote --get-url`
  (no `insteadOf` layer exists to smuggle through — the effective URL *is*
  `remote.origin.url`); the preflight credential probe (auth is explicit, not ambient);
  force-push (only if `Force` is set — impossible by type, not by argv discipline).
- **Genuine gaps, handled by the boundaries above:** linked worktrees (follow-on),
  three-way trial merge (the `deskmerge` exception), `pull --rebase` (follow-on),
  per-worktree config (worktree-family, follow-on).

## Supply-chain note

`tools/desk/go.mod` today has exactly one dependency (`yaml.v3`) — deliberately
austere. `go-git` brings ~20+ transitive modules into a security-critical toolset.
That is a real posture change, which is why the transport/auth brief that introduces
the dependency is human-gated and carries its own dependency-tree security review, and
why the final brief pins the CVE floor and wires dependency update alerts.
