### Fixed
- **`deskclaim-ref` no longer silently dials `gitlab.com` on a self-hosted GitLab.** When go-git
  could not read the origin remote — the `worktreeConfig` extension it does not support, which git
  itself enables under the linked-worktree model the desks require — the tool fell back to the
  canonical SaaS host, so a self-hosted PAT was presented to `gitlab.com`, denied, and every claim
  verb exited `unverifiable` (6). That stalled `deskdispatch --kit review` at `claim-acquire`, so a
  review desk could not fill a reviewer slot. Three independent fixes:
  - `deskkit.ForgeKindFromSlugAndHost` never defaults the host to the SaaS instance when the caller
    supplies none — a roster entry names the forge **software**, not the **instance** — and returns
    could-not-check instead of a guess.
  - `deskclaim-ref` reads the origin remote through a `worktreeConfig`-aware path: it falls back to
    native `git remote get-url origin`, then to a direct parse of the common `.git/config` (which
    resolves a linked worktree's `commondir`), covering both the shared checkout and the role
    worktree.
  - a fail-closed transport error now carries the host it dialed and the underlying cause —
    `could not create the claim refs/dispatch/<id>: <host>: <error>` — so the message attributes
    the failure instead of costing an operator a debug cycle on the wrong suspects.
