### Fixed
- The `assay-inbox` regression suite no longer flakes on its screen-relay-only check: a `printf | grep -q` behind `pipefail` reported a matched line as a miss whenever the producer's output spanned more than one pipe write. The same early-reader-behind-a-pipe shape is removed from every plugin shell script and suite, and a new `early-reader-lint` suite fails CI if one returns.
