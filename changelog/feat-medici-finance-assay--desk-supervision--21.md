### Added
- The desk's ready gate now recognises a **same-head external-prerequisite exemption**: a
  standing `CHANGES_REQUESTED` whose only blockers were external prerequisites (an upstream
  PR that had not merged, a decision that had not been made) can clear at an unchanged head —
  with no synthetic no-op push — once the reviewer declares it in a typed record and every
  named prerequisite is independently re-verified from fresh evidence at flip time.
- New reader and fail-closed decision in `deskkit` (`External-Prereq-Only:` on the CR,
  `Cleared-Prereq:` on the re-approve), the sibling of the existing check-only exemption.

### Changed
- `deskpost ready` clears the unchanged-head `CHANGES_REQUESTED` refusal only for a declared,
  fully re-verified external-prerequisite rejection; it still fails closed on a wrong
  revision, a prerequisite predating the rejection, a later revocation, an unrelated object,
  unreadable evidence, a standing `Security-Review: fail`, or any code/content finding.
- `deskboard` surfaces a declared external-prerequisite re-review as `EXTERNAL-PREREQ-REVIEW`
  rather than `SUSPECT-APPROVAL`, and `reviewloop` routes the new action; the board, planner
  and ready gate agree on what the row is. The three-round finding cap, the independent
  security review, the check-only exemption, and the human merge/ready authority are unchanged.
