### Fixed
- The release `changelog-roll` retry loop placed `-c` after the `fetch` subcommand
  (`git fetch … -c http.extraheader=… origin main`), which git rejects at parse time
  and exits `129` on every retry, so the loop could never re-sync against a moving
  default branch. The re-sync step now orders the option before the subcommand
  (`git -c http.extraheader=… fetch … origin main`), matching the working tag-push
  and roll-push invocations elsewhere in the workflow (#312).
