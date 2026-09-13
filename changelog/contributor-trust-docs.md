### Added
- **Pull-request template and a changelog note for fork contributors** (contributor-trust/06,
  partial). `.github/PULL_REQUEST_TEMPLATE.md` asks two short questions on every pull request:
  which claims the description makes and how each was checked, and whether the change was
  produced with the help of an AI coding tool or agent — neither is checkable by any tool, so
  the value is that an honest answer is cheap and a false one is a specific statement a
  reviewer can point at. `CONTRIBUTING.md` gains a plain explanation of the fork
  changelog-fragment proxy: a fork pull request missing a fragment is not something the
  contributor has to fix, because a maintainer lands it on the base branch on their behalf.
  The rest of this brief — the trust-tier model, the provenance-comment disclosure, and
  `docs/contributor-trust.md` — is deferred pending the open decision on how much of the trust
  model to publish (`medici-finance/assay#963`).
