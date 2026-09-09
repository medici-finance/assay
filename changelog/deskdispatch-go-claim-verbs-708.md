### Added
- `deskclaim-ref` — a pure-Go port of the consumer dispatch-claim script (verbs
  `acquire`/`progress`/`release`/`steal`/`show`/`list`). It speaks the same durable,
  cross-machine claim protocol (the `refs/dispatch/<id>` ref namespace, holder encoding, and
  0/5/6 exit codes) with no shebang and no bash — the only claim path that runs native on a
  Windows adopter. Every forge access is an **in-process git-smart-HTTP** call over go-git —
  no `gh`/`glab`/any CLI and no external `git` process. A claim is minted as an annotated tag
  and placed with an explicit-old (server-side compare-and-swap) receive-pack push, so a create
  loses cleanly against an existing claim and an advance/steal loses against a value that moved
  underneath it (closing the races the previous `PATCH force=true` advance and DELETE-then-POST
  steal left open). The forge (github/gitlab) resolves from `ASSAY_REPO_FORGES` or the origin
  host, and the credential from `--token-file` or `GH_TOKEN`/`GITHUB_TOKEN`/`GITLAB_TOKEN`; the
  tagger date is client-stamped (mutual exclusion rests on the server-side CAS, not the clock).

### Fixed
- `deskdispatch` now dispatches on a freshly adopted tree that carries **no**
  `tools/dispatch-claim.sh`: its claim-acquire step falls back to the pure-Go `deskclaim-ref`
  binary on PATH when the consumer script is absent, so a green-field (including
  native-Windows) adopter dispatches without the consumer script on disk and without a tribal
  `--claim-root`. A repo that still carries the script keeps using it unchanged, so a Go and a
  bash dispatcher collide on the same claim ref and never double-dispatch during the
  transition.
