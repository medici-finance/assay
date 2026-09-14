### Added
- **`deskread`** — a read-only desk verb that serves forge reads as versioned JSON, so a consumer
  outside the desk-tools module can reach the forge seam by running a process rather than shelling
  a forge CLI. It adds **no operation** to the frozen `Forge` interface: `deskread issues` is
  `ListOpenIssues`, which both backends already implement and golden-pin. `--repo` is repeatable
  and **one invocation serves the whole repo set**, read concurrently. A repo that could not be
  read lands in `partial` with its reason and is absent from `repos`, with the exit code still 0 —
  so a caller can always tell "no open issues" from "could not look"; only an all-unreadable set
  is could-not-check (exit 6).
- **`statusgen --forge`** — the opt-in for forge-backed checks. Without it statusgen is **offline**:
  it starts no forge process and makes no network call, and a forge-backed check reports
  could-not-check as itself rather than reading green. The default reader answers could-not-check
  for every repo and returns no data entry for it, so there is no shape in which "did not look" is
  indistinguishable from "looked and found nothing".

### Changed
- The stale-issue alarm on the `--lint` gate is now opt-in and reads **open issues only**, through
  the seam. It previously shelled one list-every-issue-ever call per configured repo, serially,
  under every `--lint` — measured at 13.9 s across ten repos on this repository — to print one
  advisory line whose inputs were open issues all along. It is also withheld entirely when any
  repo in the set could not be read: a debt count assembled from a subset understates the debt,
  and an understated alarm reads as "we looked and it is fine".

### Fixed
- `--lint` got **much faster, without checking less**. Two changes account for it: the attribution
  cross-check now builds the first/last author of every path in **one** `git log --name-only` walk
  instead of a `git log` pair per brief, and `parseBriefFile` is memoised on (path, mtime, size)
  so the thirty-odd checks that each walk the brief tree stop re-parsing the same file. Measured on
  this repository (24 streams, 165 briefs): git subprocesses per `--lint` **254 → 131**, offline
  wall **6.05 s → 3.90 s**, and with the forge reads gone the same lint that took **23.66 s** now
  takes **3.90 s**. A path the walk cannot account for falls back to the per-path read and a file
  that changes mid-run is re-parsed, so neither is allowed to change a check's answer — only its
  cost.
