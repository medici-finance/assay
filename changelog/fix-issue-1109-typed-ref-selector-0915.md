### Fixed
- `deskclose` accepts **typed item references** on both the item it acts on and the target of
  `--of` / `--by`: `!N` names a merge request or pull request, `#N` and a bare `N` state no kind
  and leave the resolution to the forge, and a web URL states the kind in its own path — with
  `--kind` / `--of-kind` / `--by-kind` as the equivalent flags. On a project that numbers issues
  and merge requests in separate sequences, a bare number could name two different objects, so
  the read failed closed and told the caller to "use the typed operation for the kind you mean"
  — an operation `deskclose` did not expose, which left the whole supersession lane unreachable
  in both the proposing and the confirming role. Bare numbers keep that fail-closed behaviour,
  and the refusal now names the forms that exist (#1109).
- `deskclose` routes every read and write of a lane at the kind of object it actually read: the
  item read, the pre-close comment, the proposal-thread read, the back-reference and the close.
  The untyped close addressed only the issue sequence, so on a project carrying both an issue
  and a merge request at one number it would have closed the object the caller never named, and
  the untyped thread read returned another object's notes — or, on a single-sequence forge, an
  empty thread for any issue, which reads as "no proposal stands".

### Added
- A shared typed-reference parser (`deskkit.ParseItemRef`) and two typed forge operations
  (`ListCommentsTyped`, `CloseIssueTyped`) alongside the existing `GetIssueTyped` /
  `PostCommentTyped`, so a reference that resolves for one desk verb resolves for all of them
  rather than each growing its own spelling.
