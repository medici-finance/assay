### Fixed
- **The public-repo write gate reads the forge that actually serves the repo, at every call
  site.** `deskreply` and `deskevidence` each resolved the correct forge backend for every
  other operation on a repo, then built a second, hardcoded GitHub-only client for the gate's
  live-visibility read — so on a GitLab-resolved project that read went to a host that had
  never heard of the project, and the gate failed closed for a reason unrelated to the repo's
  real authorization. Both sites now route the gate's read through the already-resolved
  backend (`deskkit.ForgeRepoInfoFetcher`), the same pattern the draft-change verb already
  used. The release verb's tag-cut gate is ruled to stay single-forge (it never resolves a
  forge backend at all) with the reasoning recorded at its call site. The superseded
  single-forge GitLab adapter this replaced is deleted, and a cross-command enumeration test
  now fails if any future call site builds a hardcoded fetcher outside the one ruled
  exception.
