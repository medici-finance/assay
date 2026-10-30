### Fixed
- GitLab board push: `pushcred_push` resets every `http.extraheader` and `http.<url>.extraheader` key by name for the one push that uses `STATUSGEN_PUSH_TOKEN`. Before this, a runner's job-token Authorization header outranked the board token's credential helper, and the push was refused with a 403. No config file is edited, and no header value is printed (#5004).
