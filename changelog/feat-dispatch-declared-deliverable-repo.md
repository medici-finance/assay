### Changed
- `deskdispatch` accepts `--repo` at a repo the brief itself declares as a deliverable: an
  `[alias]` tag on an entry of the brief's `files:` list, resolved through the alias registry,
  admits that repo for a brief whose home alias names a different tracking repo. The claim keeps
  the tracking key and, like every claim, lands in `--repo`; `--root` must still be a checkout of
  `--repo`. An undeclared `--repo`, an unknown or unpublished tag, a tag outside the `files:`
  list (including one in a fenced example block), or a brief with an explicit
  `deliverable_repo`/`homed-in` keeps the existing HARD FAIL.
