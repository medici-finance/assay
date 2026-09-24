---
id: DR-windows-port-08
date: "2026-09-23"
title: "Fleet-token custody at write time: create restricted, verify, WARN on an inconclusive read-back; a partial provisioning run stops and reports, never auto-revokes"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Option 1 — create restricted, verify, and REFUSE the whole run when the read-back cannot establish owner-only access (the brief's recommendation) — ruled out by the driver's ruling on issue 892 (option 2): an operator on a filesystem that cannot report its access list (a network share, a synchronised folder, some container mounts) would be unable to provision at all, with a refusal rather than a workaround."
  - "Option 3 — do not write credentials to files on Windows at all; hand them to the operating system's credential store and have every reading verb read from there — ruled out: a second, platform-specific storage mechanism beside the file path every other platform uses, a matching change to every reading verb, and a much larger piece of work than this provisioning; recorded as a reasonable future direction, not taken here."
  - "On a run that fails partway through, attempt to REVOKE the credentials already minted — ruled out by the driver's ruling on issue 1500 (report): revocation can itself fail halfway, leaving an inconsistent result that is harder to reason about than either extreme; reporting is always truthful and never fails."
accepted:
  - "A credential whose owner-only read-back is INCONCLUSIVE (the access list cannot be read, the owner or invoking user cannot be established, or an entry cannot be interpreted) is provisioned with a prominent WARNING naming the file, and the run continues. This is a second, weaker standard than the READ path applies: such a credential may be refused later at use time by the unchanged read-side custody check, after it is already minted and live. The operator meets that failure later rather than at provisioning time."
  - "Only the INCONCLUSIVE case is relaxed. A read-back that DEFINITELY shows the file is not owner-only (a foreign owner, a foreign write-capable or read-capable entry, a POSIX mode other than 0600) still stops the run, names the affected credential, and tells the operator it must be revoked — option 2 is identical to option 1 up to the inconclusive point, and no further."
  - "A partial run leaves manual cleanup: the tool stops, reports exactly which credentials were minted (by role, service-account username and token name/id — never a token value), exits non-zero, and performs no revocation. Revoking them is the operator's act."
  - "The read-side custody check every desk verb runs when it READS a token is unchanged by this decision; it stays the out-of-band layer that refuses a loosened file at use time."
---

**Ruling recorded (2026-09-11 and 2026-09-23): option 2, and report.** The brief's human
decision fork asked two questions together; each was ruled on its own decision issue by the
driver (`human:<name>`), each ruling posted under the driver's own login — never a role App
relay:

- **Write-time custody posture — option 2.** Ruled 2026-09-11T20:18:15Z on the brief's decision-gate
  [issue 892](https://github.com/medici-finance/assay/issues/892) — the
  [ruling comment](https://github.com/medici-finance/assay/issues/892#issuecomment-5640147554)
  reads `2`: create the credential file restricted, read the access list back, and WARN rather
  than refuse when that verification is inconclusive.
- **Partial-run behaviour — report.** Issue 892 closed with its second question still
  unanswered, so it was re-filed as the follow-up decision
  [issue 1500](https://github.com/medici-finance/assay/issues/1500) and ruled 2026-09-23T19:12:55Z — the
  [ruling comment](https://github.com/medici-finance/assay/issues/1500#issuecomment-5801240014)
  reads `report`: a run that fails partway through, having already minted some credentials,
  stops and reports exactly which credentials exist so an operator revokes them by hand; the
  tool never attempts revocation itself.

This record transcribes both rulings into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4); it does not mint a new decision — the human acts were the
driver's own-login comments on the two issues.

**The decision.** `docs/streams/windows-port/brief-08-go-native-gitlab-fleet-provisioning.md`
ports GitLab fleet provisioning to a Go desk verb that mints one personal access token per role
and writes each to `gitlab-<role>.token`. At the moment a token is WRITTEN: the file is created
restricted (owner-only at creation — the POSIX `0600` create, or an owner-only access list on
Windows — never widened then tightened); the existing owner-only custody evaluation is then run
on the file as written, before its path is reported as usable. A verified file is reported
usable. A definite failure stops the run. An inconclusive read-back warns, names the file, and
continues. When any step of the account/token loop fails after one or more tokens were minted,
the run stops, writes a report naming each minted token by role, account and token name/id,
and exits non-zero; it revokes nothing.

**The constraint behind it.** Windows has no equivalent of the POSIX permission bits — a file's
mode reads `0666` whether or not its access list is locked down — so "owner-only" must be
restated in access-list terms, and whether it holds cannot always be read back on every
filesystem. The ruling trades the strictest posture for the ability to finish provisioning on
such a filesystem, and accepts the cost named above: a warned credential meets the unchanged
read-side check later.

**What this record does not decide.** It does not change the read-side custody evaluation
itself (that is a different, security-gated change). It does not adopt the operating-system
credential store (option 3), which would need its own plan. It does not retire the bash
provisioning script, which stays the reference implementation. And, per the register's own
limits, it records that the alternatives were weighed and names a human approver; whether the
implementation matches the ruling is the review gate's judgement, and the live provisioning run
the brief's human-gated Verify row names is the change's own validation.
