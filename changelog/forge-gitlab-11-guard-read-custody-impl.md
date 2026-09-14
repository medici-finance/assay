### Added
- `RepoHardeningRead` (op 38) on the `Forge` seam: a closed hardening-read kind vocabulary
  (`repo`, `rulesets`, `actions-workflow-permissions`, `actions-fork-pr-approval`,
  `actions-private-fork-pr`, `vulnerability-reporting`), validated before any request exists.
  GitHub implements all six; GitLab refuses each by name until forge-gitlab/12.
- `desktoken`'s `auditor` role: a dedicated, read-only identity (no write permission of any
  kind) minted/read through the existing per-role custody paths.

### Fixed
- `repohardenguard` no longer shells out to `gh api <endpoint>` — it reads through the typed
  `RepoHardeningRead`/`ReadFile` ops under the `auditor` identity, closing forge-gitlab/08's
  Verify row 3 (the whole-tree forge-CLI grep) to 0. The checklist's `Read` cell grammar moves
  from `gh api <endpoint>` to `read <kind>` / `read file <path>`; the old form is refused by
  name at parse time.

### Changed
- `docs/adopting-assay.md` and `docs/adopting-assay-gitlab.md` document the new `auditor`
  identity (GitHub App permissions, GitLab role/token) per the #857 ruling's docs half, and
  also close a pre-existing gap where the `cell-issues` role was undocumented on both pages.
