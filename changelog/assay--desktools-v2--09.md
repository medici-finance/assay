### Added
- New `ReviewQueueSnapshot` Forge operation. It returns every open change together with its reviews in one backend round-trip: a single GraphQL query on GitHub. On GitLab it is degraded: it marks each change incomplete, so reviews are still read per item.
- `deskboard`'s actions sweep now reads each repo's open PRs and their reviews with this operation, instead of one review read per PR. On GitHub that means 1 call per repo instead of 1 + N. Each PR's head and its reviews now come from one consistent snapshot, and the board output is unchanged.
