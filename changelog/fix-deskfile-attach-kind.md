### Fixed
- `deskfile attach` can now target a GitLab **issue** whose project also carries a merge request
  with the same number. GitLab numbers issues and merge requests in separate sequences, so `#4`
  and `!4` routinely both exist; the bare-number target read refused that case ("carries BOTH
  issue #N and merge request !N") and, with no way to state the kind, every such number was
  un-attachable (seen on a GitLab adopter cell: `--to 4` and `--to 5` both refused). The verb
  gains `--kind issue|mr` (default `issue` — attach is an observation on an issue; `pr` is an
  alias of `mr`), and the kind drives BOTH the target's state read and the note's endpoint
  (issue notes ↔ merge-request notes), so the check and the write address the same object.
  On GitHub (one number sequence) the kind is only validated against what the number is —
  asking for an issue at a pull request's number is could-not-check (exit 6), never a comment
  on the other kind. An unknown kind is refused (exit 5); the bare-number behaviour of other
  callers is unchanged.

### Added
- `Forge.GetIssueTyped(repo, number, kind)` and `Forge.PostCommentTyped(repo, number, kind, body)`
  — the typed read/write pair the bare `GetIssue` both-kinds refusal points at, implemented on
  both backends, with `TargetKind` (`issue` | `change`) and `ParseTargetKind` (accepts `issue`,
  `mr`, `pr`, `change`). Additive: the existing untyped methods keep their behaviour.
