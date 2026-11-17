### Fixed
- The pinned-release checker (`tools/pairedversions`) now refuses a release-home answer past its bound instead of using a cut-short one. Before, it read `checksums.txt` as its first 1 MiB, and it decoded the release and release-list answers with no bound at all.
- Correction to the over-cap entry for the `deskfleet`, label-transport and `deskapps` clients: the over-cap refusal is a could-not-check error, but the exit status is the caller's own and is not always 6. `deskfleet labels --check` and `deskfleet push-token` exit 6. `deskfleet provision` (project settings included) records the failure and exits 1. `cellctl check` reports the labels row as could-not-check, and `cellctl new` reports it NOT ENSURED. `deskapps` answers its callback with HTTP 502.

### Changed
- The forge-body source guard keys each permitted read by its kind as well as its file and function, so a second read added to a permitted function is still reported. It also reports a truncating reader (`io.LimitReader`, an `io.LimitedReader`, or a local alias of one) handed to a capped body reader, which would otherwise end the body before the cap could refuse it. It now scans `tools/pairedversions` too.
