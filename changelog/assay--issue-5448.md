### Fixed
- `deskgit fetch --pr N` on a GitLab-served repo now asks for `refs/merge-requests/N/head` instead of GitHub's `refs/pull/N/head`. The forge comes from the same resolution the `--as` credential arm already uses; GitHub fetches are unchanged.
