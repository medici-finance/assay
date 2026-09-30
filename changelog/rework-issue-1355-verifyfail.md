### Fixed
- The cellctl bash oracle (`tools/cellctl/testdata/cellctl-shell-oracle.sh`) is `shellcheck`-clean again (#1355). Its seven `value_in "$x" $<LIST>_VALUES` calls word-split the value list on purpose, and the `policy_preflight` subshell keeps its policy variables local on purpose; each site now carries a `# shellcheck disable=` directive naming why, instead of an SC2086/SC2030/SC2031 finding. No behaviour change in the oracle or the Go program.

### Added
- `TestOracleShellcheckClean` in `tools/desk/cmd/cellctl` runs `shellcheck` over the whole oracle inside `go test`, so wherever `shellcheck` is installed a new finding at any site is a red test. CI does not install `shellcheck` yet, so there the test skips, and a non-verbose `go test` prints that skip as a plain `ok`; the install step is tracked in #1875. A planted-SC2086 positive control makes a broken or stubbed `shellcheck` fail the test rather than pass it; with no `shellcheck` on PATH the test skips as could-not-check.
