### Fixed
- Evidence-actor on GitLab: a verifier service account whose commit carries the account's DISPLAY name (not its username) can now back a `verified`/`done` row, unblocking `implemented → verified` on GitLab (#1477). Two accepting paths: `deskevidence` resolves the landed commit's account to its username ONLINE via the typed forge (`GET /users?search=`, never a forge CLI); `statusgen --lint` accepts the commit OFFLINE when the roster declares the account's display name in the new `ASSAY_GITLAB_DISPLAY_NAMES` map. A forge read that cannot resolve the account is could-not-check, never a pass and never a rejection.
- Evidence-actor now accepts a roster-known GitLab HUMAN verifier via GitLab's private commit noreply address (`<user-id>-<username>@users.noreply.<host>`, id-pinned) the way it already accepted the GitHub noreply form.

### Added
- `ASSAY_GITLAB_DISPLAY_NAMES` roster key (`<username>=<display name>`; entries separated by `;` or newline) — the offline fallback that lets statusgen's Evidence-actor gate accept a GitLab verifier's Evidence commit. statusgen consumes it; the desk tools recognise it and resolve the username online instead.

### Changed
- The Evidence-actor rejection for a GitLab service-account commit whose name matches neither the username nor a declared display name now names the display-name-vs-username gap and its remedy, instead of reading as a wrong-account tamper signal.
