### Fixed
- `tools/changelog/check.sh`'s `PR_NUMBER` validation now anchors the whole
  parameter (`[[ =~ ]]`) instead of anchoring per-line via a piped `grep -E`,
  closing an embedded-newline bypass that could launder one PR's number into
  matching another PR's proxy changelog fragment; the downstream proxy-lookup
  `grep -E` match gained its own newline guard as defense in depth. Not
  exploitable in production (`github.event.pull_request.number` cannot carry a
  newline) — a hardening fix plus a new regression test (`check_test.sh` P13).
