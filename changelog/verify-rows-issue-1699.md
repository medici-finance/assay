### Fixed
- windows-port/11 Verify row 7 (no hard-coded `"/bin/bash"` under `tools/desk/cmd/scanloop/`) no longer fails on its own success path. The witness runs rows under `pipefail`, and `git grep` exits 1 when it matches nothing, so the old `git grep … | wc -l` row failed exactly when the property held. The row now tolerates only the no-match status, proves the directory exists at HEAD, and gates on `output is \`0\`` so a reintroduced literal fails the output check (#1699).

### Added
- statusgen Verify-row lint: advisory rule `grep-pipe-zero-count` flags a row whose final pipeline has a `grep`/`egrep`/`fgrep`/`git grep` feeding a later stage under a zero-count Expect — the shape that fails under the witness's `pipefail` when the property holds. Positive-control fixture under `statusgen/testdata/verifyrows/grep-pipe-zero-count/`; the author-brief enforcement block is regenerated.
