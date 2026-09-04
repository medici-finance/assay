### Changed
- The per-PR changelog fragment requirement is now stated on every implementer-facing surface, not just enforced by the CI gate: the deskdispatch worker kit gained a changelog-fragment clause (detection-based — inert where a repo carries no `changelog/README.md`), and the worker-desk, pr-shepherd, and author-brief skills each name it.

### Fixed
- The missing-fragment CI failure now names the exact fix — it derives the suggested `changelog/<slug>.md` path from the PR head branch and prints a copy-pasteable `printf … > changelog/<slug>.md` command plus a `MISSING:` line with the base/head SHAs — instead of only saying a fragment is absent.
