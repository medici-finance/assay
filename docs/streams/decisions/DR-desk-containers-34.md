---
id: DR-desk-containers-34
date: "2026-11-10"
title: "Project secrets reach worker and verifier windows as a launcher-held, owner-only file per project, role and service, with per-role read isolation; reviewers accept the verifier's signed outcome"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/6076#issuecomment-6778126739"
alternatives:
  - "Operating-system keychain lookup by project scope (brief option 2). Ruled out: it protects the value at rest better on one desktop, but Linux containers and Kubernetes have no equivalent, so the container path would still need the launcher-held file."
  - "A local-only verify lane is the sole holder (brief option 3). Ruled out: it exposes the least, but workers cannot run a secret-bound check while implementing, so failures surface only after merge."
  - "Hold (brief option 4): open no channel, and leave such rows in the Environment-blocked queue for an operator. Ruled out by the driver's choice of option 1; it stays the behaviour of any cell that cannot provide per-role read isolation."
  - "Treat owner-only mode as the read boundary on a cell where every window runs as one OS user, backed only by a harness read-deny and the never-build-an-untrusted-head rule. Ruled out in the brief after security review: owner-only mode separates OS users, not the role windows of one user, and a harness read-deny is client-side and advisory."
accepted:
  - "A secret is scoped to (project repository, role, service name). The service name is the name in a Verify row's service:<name> Env cell. Only worker and verifier roles may hold one. Reviewer, intake and coordinator roles never do."
  - "The operator keeps each secret as an owner-only file outside every checkout. The launcher checks the file and exports only its path, and only into worker and verifier windows. The value is never passed in argv, in the environment, in a commit, in an image layer or in a pipeline-readable variable."
  - "The dispatcher strips the path variables from any child that runs an unblessed head and from every review-kit child. Reviewers witness a secret-bound row through the verifier's signed outcome for that row, never through a run report."
  - "A non-holding role must be unable to open the file, not only unaware of its path: a per-role container mount, or a separate OS identity per role on a host. A cell that declares neither binds no role secret, so its secret-bound rows stay in the Environment-blocked queue. A declared surface counts only once the launcher's check has verified it: on a host, every non-holding identity is distinct from every holder and the file owner, is not root and cannot elevate without a prompt, and each window starts under its role's identity; on a container cell, no non-holding window runs on the host as the file owner. The write-path rules, approved by the driver's later comment on #6076, add to read isolation: no non-holding identity may write anything a holding window executes or reads as configuration (a checkout's shared git directory and hooks, the cell config and identity map, the holder's PATH and home, or a writable mount shared with a holding container), and a secret file whose access-control list grants a non-holder read is refused."
  - "Deleting or replacing the local file stops future launches only. Revoking a leaked secret means rotating it at the issuing service. The launcher's receipt records role, project, service, path and times, never the value or a hash of it."
---

**Status:** routed — approved by the driver in their own login on 2026-11-10, in two comments
on #6076, and cited by brief 34's `sources:`. `ruling:` links the later one. Both name this
record and sit on an issue whose body carries the decision-gate marker for it, so the
registers-v1 §7.5 check corroborates the ruling.
**Routes-to:** docs/streams/desk-containers/

Brief 34 cites this record in `design:` and `sources:`.

## Basis

The decision text above comes from
[desk-containers/34](../desk-containers/brief-34-project-secrets-reach-dispatched-windows.md)
(`## Human decision`, `## Task`) and from three comments by the driver.

The first, on #6065 (https://github.com/medici-finance/assay/issues/6065#issuecomment-6773391431),
chose the option. It is kept here as history; it did not name this record, so it could not
corroborate it. Relayed as given, it reads:

> desk-containers/34: A — launcher-held owner-only file per project, role and service; path exported only to worker and verifier windows, stripped from unblessed heads; reviewers accept the verifier's signed outcome.

The comment labels its choice "A". The brief numbers its options, and the comment's text
restates option 1.

Three parts of this record were added to the brief by security review. The first round was
posted before that comment and the second after it, so the comment saw none of them:

- per-role read isolation, and the refusal to bind a secret on a cell without it (brief Task
  2a, Verify rows 8 and 9);
- the requirement that the launcher's check verifies a declared surface rather than
  trusting the declaration, and starts each window under its role's identity (Task 2a's last
  paragraph, Task 3, Verify rows 5, 8 and 9), added by the second security round;
- the note that revocation of a leaked copy is rotation at the issuing service (Task 5).

The second comment, on #6076
(https://github.com/medici-finance/assay/issues/6076#issuecomment-6775066942), approved this
record by name and confirms all three. It is kept here as history. Relayed as given, it reads:

> DR-desk-containers-34: approved — option 1 as ruled on #6065, plus per-role read isolation (single-user cells fail closed and bind no role secret), cellctl check verification of any declared isolation surface with windows launched under their role identity, and revocation by rotation at the issuing service.

A third security round, after that approval, found that a non-holder able to write what a
holder runs could make the holder read the secret out for it. It added write-path rules to
read isolation, and a refusal of a secret file whose access-control list grants another
identity read (brief Tasks 1, 2 and 2a, Verify rows 8 and 9). The second comment did not
cover them.

The third comment, on #6076, is the ruling `ruling:` links. It approves the write-path rules
and the access-control-list refusal by name. Relayed as given, it reads:

> DR-desk-containers-34: approved, adding the write-path rules to read isolation: no non-holding identity may write anything a holding window executes or reads as configuration (a checkout's shared git directory and hooks, the cell config and identity map, the holder's PATH and home, or a writable mount shared with a holding container), and a secret file whose ACL grants a non-holder read is refused.

## What this record does not decide

It does not stand in for the `gate: human` acceptance at done. It does not choose the
container runtime or the way a host provisions an OS identity per role; those stay with the
operator. It does not enact the signed-verdict lane that reviewers rely on. That lane keeps
its own ruling, and until it is enacted, the interim Environment-blocked rule stands. It
loosens no existing custody control: owner-only checks, read-only mounts, the
no-secret-in-a-layer scan, the leak sweep and the never-build-an-untrusted-head rule all
stand.
