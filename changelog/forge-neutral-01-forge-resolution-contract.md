### Added
- `deskkit.ForgeFor(repo, role)` — the first resolver that can hand a desk tool a `Forge`
  backend at all. Two complete backends (`GitHubForge`, `GitLabForge`) have existed with no
  constructor, no config key, and no consumer; this is that missing answer, and the ONLY
  function in the tree allowed to construct either backend (enforced by
  `TestForgeSingleConstructionSite`'s AST walk plus an independent grep, and backed by the
  existing `forge-surface-control.yml` shell-exec/passthrough CI job). Resolution reads the
  repo's forge from a new roster key, `ASSAY_REPO_FORGES` (`owner/name=github` or
  `owner/name=gitlab`, full slug only — a bare basename is refused, unlike the display-only
  `ASSAY_REPO_ALIASES`), falls back to the origin remote's host when the mapping is
  unambiguous (`github.com`/`gitlab.com` only), and otherwise refuses could-not-check naming
  the repo and the configuration that would resolve it. There is no parameter, flag, or
  environment variable by which a caller supplies the forge itself
  (`TestForgeForRejectsCallerSuppliedForge`).
- Custody: `ForgeFor` obtains the resolved role's already-minted token from the existing
  per-forge path — GitHub via the `desktoken` mint-or-reuse path (`RoleTokenForRepo`),
  GitLab by reading the `gitlab-<role>.token` file a prior rotation produced — and never
  falls back to an ambient credential. A missing or insecurely-permissioned (non-0600)
  custody file is refused, naming the remedy. `SetGitHubCustodyMinter` is an installable
  seam a caller that already mints its own GitHub App tokens in-process can plug its
  existing, tested minter into, rather than this package growing a second implementation.
- `deskpost`'s `comment` verb is wired end-to-end through `ForgeFor` as the
  proof-of-reachability: the actual `POST .../comments` call now goes through the resolved
  `Forge.PostComment`, authenticated via `deskpost`'s own existing App-token mint installed
  as the custody minter above — every precondition read on the same command still runs on
  the pre-existing client, unchanged. `deskpost` carries no `forgeban` permit row, so this
  step moves that ratchet by zero; it only proves the resolver is reachable before any later
  brief's migration claim rests on it.

### Fixed
- `rosterconfig.go`'s known-key set, echo, and refusal message all recognise the new
  `ASSAY_REPO_FORGES` key, so a deployment that sets it does not fail the whole roster
  closed on the unregistered-`ASSAY_*`-key refusal.
