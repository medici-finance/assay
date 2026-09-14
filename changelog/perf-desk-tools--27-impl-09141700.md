### Changed
- `deskboard`'s last three serial repo loops run on the bounded worker pool `actions` and
  `health` already used: `prs`, `stalled` (repos AND, inside each, that repo's PRs) and the
  always-on policy-drift probe that rides inside `actions`. Measured back to back on one
  operating desk host with a ten-repo roster, structurally identical output: `stalled`
  84.82s → 12.63s, `prs` 19.82s → 6.63s, `actions` 33.59s → 24.04s.
- `dispatch` and `awaiting` resolve their stream roots through one shared resolver and read
  them concurrently, and `throughput` resolves them once for the whole run instead of twice:
  47.65s → 29.10s, identical output. A failed shared resolution blinds BOTH stages it fed,
  each naming it — never a counted zero.
- `deskflip` evaluates its conditions cheapest-first, as far as a recorded diagnostic rule
  allows: `mergeable` moves from seventh to fourth (it costs no forge read at all) and
  `model-floor` from fourth to seventh (it buys a paginated label-event timeline). A refusal
  on a CONFLICTING PR now costs one forge read instead of four. The check rollup at head,
  previously fetched twice by two conditions in the same run, is fetched once — each still
  reporting a failure under its own condition's name.

### Fixed
- The drift self-check resolved only the bare `desk-tools` pin name, so a consumer pinning
  the per-platform `desk-tools-<os>-<arch>` line reported a permanent could-not-check as
  STALE. It now tries the per-platform artifact name as a second EXACT lookup — the bare line
  still wins when both are present, and the trailing-space prefix match is untouched.
- A subcommand `--help` is no longer charged to the append-only audit ledger as a refusal.
  `deskpr`, `deskwt`, `deskfile`, `desktoken`, `deskpost` and `deskreply` print usage and exit
  0, writing no row; a genuinely bad flag in the same position still refuses and still audits.
- `deskpost`'s two largest refusal classes — a review body with no `## ` heading and one with
  no verdict line — now name the offline rehearsal (`--dry-run`) that would have caught them.
