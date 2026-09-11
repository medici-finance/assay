### Added
- `deskpr create`, `deskpr update` and `deskwt add` now **refuse, fail-closed**, when the
  resolved PUSH url of `origin` is an SSH one (`ssh://…` or `git@host:path`) and the session
  presents a bot identity (`$DESK_LOOP` resolving to a role App). An SSH push authenticates
  with whatever key the machine's agent holds — a human's — so the forge recorded the HUMAN
  as the branch creator and the App's permission envelope was bypassed, however the commits
  were authored. The refusal names the config key, the url, the acting App, and the one-line
  `remote set-url --push` remedy with the equivalent https url computed for you.
- Fetch over SSH stays allowed — `remote.origin.pushurl` is what the gate reads whenever it
  is set — and `deskpr edit`, which pushes nothing, is not gated. With `$DESK_LOOP` unset the
  gate is inert: a human at a terminal pushes under their own key.
- An https push url with no App credential helper configured now prints a stderr **NOTICE**
  (never a refusal): the ambient-identity shape one layer along, reported as could-not-check
  rather than as a pass.
