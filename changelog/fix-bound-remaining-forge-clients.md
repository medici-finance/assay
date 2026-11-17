### Fixed
- `deskinbox`'s comments read, `deskadvisory`'s GitHub API reads and the label transport `cellctl` uses now give up on a stalled forge after 30s per request instead of hanging indefinitely. A label transport handed a client that already sets its own timeout keeps it.
- A response body cut off after a successful status is now an error in all three (`deskinbox` reports it as could-not-check), never an empty comment or label list, and `deskadvisory` refuses an empty successful answer instead of reading it as one.
