### Changed
- **`deskpr`, `deskfile` and `deskclose` now reach the forge through the resolver under a
  minted App identity, so they create, file and close on GitLab CE with the same trailer and
  gate behaviour as GitHub.** The three verbs were the last of the fleet's outward-write
  commands still shelling `gh` under whatever ambient CLI credential happened to be active. They
  are re-seated onto `ForgeFor`, which mints the session-role App token and refuses rather than
  falling through to an ambient identity. To make the migration possible the frozen forge seam
  gained four enumerated operations — a branch→change lookup, a change body/title edit, a
  repo-scoped issue text-search, and a read-only list-labels — each on both backends with a
  golden contract case. `deskpr`'s `--as-app=false` ambient fallback is retired (a run with no
  minted token now REFUSES, it does not fall back), and `deskfile`'s interim GitLab
  named-refusal (`#691`) is superseded now the backend serves GitLab. The forge-CLI permit
  ratchet drops by three. Authorized by the `#781` ruling (token-custody decision).
