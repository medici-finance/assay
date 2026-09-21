### Fixed
- The GitLab forge adapter now preserves GitLab's own structured error body (`message` /
  `error`) on every write and read verb: `mapErr` carries the forge's message onto the
  `ForgeAPIError` and into the rendered refusal, so a rejection that used to reach the caller
  as a bare `HTTP 400` now names the actual cause (a permission message, a validation
  message, a missing branch). Redaction rules are unchanged; the body is only control-stripped
  like every other forge-origin string this tree renders.
- `CreateDraftChange` now rides out GitLab's transient post-push "source branch does not
  exist" rejection with a bounded, same-identity retry (3 attempts, 2s apart) for that one
  specifically identified condition — the race where a just-pushed branch is readable through
  Git and the branches API but the merge-request create briefly still 400s. Every other error,
  including every other 400, is surfaced on the first response and never retried; exhausting
  the bounded attempts is reported as could-not-check, never rounded up to a pass.
