### Added
- `deskdispatch` worker prompts now name the worker's own loop identity
  (`export DESK_LOOP=worker-desk`) so the desk write verbs (`deskpr create`,
  `deskfile`, `deskreply`) stop refusing with `$DESK_LOOP is unset`, and — for an
  issue-only item — the exact `deskpr create` trailer (`Issue: #<N>`) and the
  sanctioned verb to comment on the issue it was dispatched from
  (`deskfile attach -R <owner/repo> --to <N>`), replacing hand-rolled `gh` writes.
