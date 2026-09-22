### Fixed
- `deskclose superseded` on a PULL REQUEST no longer reports a close it did not perform. The
  close is now **read back** after the call — the item is re-fetched and its state confirmed
  `closed` before success is printed — so a state-change request that returns without error but
  leaves the item open (an issue-shaped `state_reason` PATCH on a pull request's number returns
  `422`, which was swallowed after the confirmation comment posted) is caught. When the comment
  posted but the close did not take, the run reports `partial: comment posted, close refused: …`
  with the forge's own error body and exits `6` (could-not-check), never success. The read-back
  lives in the shared close path, so every permanent-close lane (`superseded`, `duplicate`,
  `review-request`, `triage`, `self-withdraw`, manifest rows) gets it; the deliberately transient
  `verify-gate-refire` close, which the repository's verify-gate-close workflow reopens, opts out.
- `deskclose` flag parsing now accepts the single-dash spelling of a long value flag (e.g.
  `-by`, which Go's `flag` package treats identically to `--by`) in every argument order. The
  positional splitter previously recognised only the double-dash spelling, so `superseded <pr>
  -R … -by …` tripped `flag needs an argument: -by` — it dropped the value that was present and
  mis-read it as a second item number — a failure that looked "environmental" because it
  depended only on the dash spelling typed.
