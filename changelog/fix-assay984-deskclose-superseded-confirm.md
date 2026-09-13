### Fixed
- `deskclose superseded`'s confirm/dispute now normalizes both the standing proposal's
  recorded target and the caller's `--by` value to `owner/repo#N` before comparing — a
  bare `--by 40` against a marker recorded as `owner/repo#40` used to read as a target
  DISAGREEMENT ("record and caller disagree") when it was only a format mismatch. A
  genuine disagreement now names both the recorded form and the expected form explicitly
  (#984).
- `deskdisposition read` no longer shells out to `gh pr view` at all: it mints the
  session-role App installation token and reads the record (labels + comment thread)
  through the resolved forge over REST, the same custody pattern `deskclose`/`deskfile`
  already use. Previously, `deskclose superseded`'s confirm path (a child process of an
  already-minted desk session with no usable ambient `gh` identity) got a bare
  `HTTP 401: Requires authentication` from the shelled `gh` call even though a valid
  App-token read was available (#984).
