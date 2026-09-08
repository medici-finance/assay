### Added
- `desktoken` supports an optional **role→App binding** (`<ROLE>_APP=<app-name>` in
  `apps.env` or the environment), so a deployment can run fewer Apps than desk roles — the
  recommended two-App tier is one key that reads and one that writes — without symlinking or
  copying keys. The App-name is the stem for the role's PEM file, App ID and install ID keys;
  absent, it defaults to `<role>-app`, byte-identical to the previous layout. `desktoken
  --version` now prints the effective `bindings=` line, `role=app-name` per role.
