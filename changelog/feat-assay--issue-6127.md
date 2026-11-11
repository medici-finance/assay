### Added
- The desk label set now carries a `to:<role>` addressee label for every role the roster binds, alongside `raised-by:<role>`. Both are built from the same role list `deskfile --to` / `--raised-by` accept, out of one shared table (`internal/forgelabels`) that `deskfleet` and `deskgitlab` both use. Addressed filings no longer degrade to `to=UNADDRESSED:label-missing` on a freshly installed repo.
- `cellctl new --kind house --labels-token-file FILE` ensures the full set on every `--roots` repo, idempotently. On GitLab a label inherited from the group counts as present. When no credential is given, or the credential cannot write labels, the step prints `NOT ENSURED` with the exact `deskfleet labels …` command that finishes it. A credential that was given and failed makes `new` exit 1.
- `cellctl check` reports the label set per house-cell repo (read with `CELL_LABELS_TOKEN_FILE`): `MISS` names each missing label, and labels that cannot be read are a new `?  could-not-check` row, never `ok`. Both label paths print their forge, API base and repos before the first request.
- `deskfleet labels` takes `--roles` (the roster's role names) and `--check` (names each missing label; exit 1 when any is missing, 6 when the labels cannot be read).

### Changed
- Label creation stays an operator act under the operator's own credential. No desk role App is granted label-write, and the open question of whether a desk role may create labels on its own is unchanged.
