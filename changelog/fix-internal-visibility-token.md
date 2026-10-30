### Fixed
- `ASSAY_ALLOWED_REPOS` accepts an `:internal` visibility token (`owner/name[:ci|:no-ci][:public|:private|:internal]`). The public-repo write gate now authorizes a live-`internal` repo only on an `:internal` entry and a live-public repo only on a `:public` entry, so an internal GitLab project or GitHub Enterprise repo no longer has to be mislabelled `:public`. An `:internal` repo stays under every public-repo requirement: risk-classing, the self-containment scan and the neutral on-behalf-of form, which needs `ASSAY_HUMAN_LOGIN_MAP`.

### Changed
- A live-`internal` repo tagged `:public` is now refused. Retag it `:internal`.
