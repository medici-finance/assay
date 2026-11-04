---
id: DR-forge-gitlab-18
date: "2026-11-03"
title: "Board-push credential rotation: a Go subcommand rotates a project access token under a person-held rotator credential and proves it by-id, self and push"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Extend the shell renewal script (choice 1B) — ruled out: the value would travel in a variable-update command's arguments or input, which is exactly the custody gap the brief closes."
  - "Stop storing a credential and push with the job's own short-lived token (choice 1C) — not chosen now: instance support and its protected-branch behaviour are not established. Recorded as a later investigation."
  - "A group access token (choice 2, group) — ruled out: every project in the group is the blast radius, and the verb as briefed handles a project token only."
  - "A dedicated rotator service account in automation custody (choice 3B) — deferred: a later decision once the person-held path (3A) has run cleanly. An existing automation role account (3C) is ruled out: no role gains project-variable access as a side effect."
  - "Fail the rotation when self-introspection is refused to a repository-only token (choice 6B) — ruled out: the rotator-side by-id read plus a live push probe is the proof, and the output says so."
accepted:
  - "Choice 1A: `deskfleet push-token rotate|check`, a Go subcommand of the existing provisioning tool; the value stays inside one process."
  - "Choice 2: a project access token, scopes exactly [write_repository], Maintainer (40)."
  - "Choice 3A: the rotator credential is a person's own Maintainer api-scoped credential in an owner-only file, and that person runs the command. The refusal on the desk loop's session marker is advisory only, because the marker is self-declared."
  - "Choice 4: 30-day tokens by default, at most 90; the proof needs at least 7 days left and an expiry no earlier than one day before the requested date; check mode alarms below 7 days."
  - "Choice 5: hidden through the create path where the instance supports it; otherwise masked and protected, the existing floor."
  - "Choice 6A: a 403 insufficient-scope answer from self-introspection is accepted beside the rotator-side by-id read and the live push probe; any other self-introspection failure still fails."
  - "The brief's recommended order of replacement, rotate-first (choice 7A), is the implemented order: at most one valid push credential at any moment, and any failure between the rotate and a proven install is reported with a re-run instruction."
---

**Ruling recorded: APPROVED as briefed — all six recommended defaults.** The driver
(`human:<name>`) answered the decision-gate issue #2912 of `forge-gitlab/18` (part B of #2910) in
the [ruling comment](https://github.com/medici-finance/assay/issues/2912#issuecomment-6640161913):
"Approve as briefed: all six recommended defaults". This record was written from that answer
and is authored in the same change as the implementation; it is not self-approved.

**The decision.** The board-push credential `STATUSGEN_PUSH_TOKEN` is rotated by one Go
process. It reads the project, its protected default branch, the project-level variable and the
one active project access token by name, and refuses before any write when one of them fails its
check. It then rotates the token, writes the new value into the variable with masked, protected
and scope `*` sent on every write, reads the variable back, and proves the new token by its
record by id, its own self-introspection, and a receive-pack advertisement on the API host. A
read-back that disagrees revokes the new token and confirms the revoke. `check` is the read-only
monitor between rotations, under a separate reader credential.

**The constraint behind it.** The value is a repository-write credential for the default
branch. It must never reach a shell, a transcript, a log, argv, the environment or a file. The
person holding the rotator credential runs the command, because the command cannot tell that
person from an automated session running as the same operating-system user.

**Choice 7.** The ruling's wording names six defaults; the brief listed seven choices. The
seventh, rotate-first, is the brief's recommended default and the implemented order, and the
implementing pull request declares it as its own decision for review.
