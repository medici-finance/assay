### Fixed
- The release `changelog-roll` job's bounded push-retry loop could never re-sync
  against a moving default branch: its re-sync step ran
  `git fetch --no-tags -c http.extraheader=… origin main`, placing `-c` AFTER the
  `fetch` subcommand, which git rejects at parse time (`error: unknown switch 'c'`,
  exit 129) on every attempt. The corrected form places `-c` before `fetch`,
  matching the working tag-push and roll-push invocations elsewhere in the
  workflow. The fix is staged for maintainer apply under `tools/changelog/`
  (`release-roll-fetch-order.yml.patch`), since no App may push a
  `.github/workflows/*` file.
