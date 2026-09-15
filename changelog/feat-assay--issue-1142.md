### Fixed
- `desktoken --forge gitlab <role>` now SELF-CHECKS the rotated PAT with one live, read-only
  `GET /user` before printing the custody path. The rotation endpoint's 200 only says the forge
  issued the successor; in the field the caller's first read with it answered 401 while the
  on-disk token was valid seconds later (server-side propagation lag after self-rotation), and
  the mint path assumed that away. A self-check the forge does not answer 200 now exits 6
  naming the endpoint, the status the NEW token got and whether the PREVIOUS token was still
  accepted (lag: re-run the mint once) or rejected too (lockout: a group owner re-issues the
  PAT); the persisted path is not printed as good. One read per token, no retry loop, no
  sleep; rotate-on-mint custody is unchanged (#1142).
