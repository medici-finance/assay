---
id: DR-tool-refactoring-01
date: "2026-11-21"
title: "Approve the bounded operation-subject refactor once its independent class and caller controls pass"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/7087#issuecomment-7066509833"
alternatives:
  - "Option 2 — keep the existing behavior and request a narrower migration. Not chosen by the driver's ruling (option 1). It would leave each consumer deriving its own tracking, decision, review and Evidence coordinate, which is the repository-substitution class the brief exists to close."
accepted:
  - "One immutable operation subject in the existing pure owner (`statusgen/domain`, composed with `statusgen/workref`) binds the declaring brief revision, tracking source, delivery candidate, review target, decision target and Evidence destination separately; deskkit stays its observation adapter."
  - "A missing coordinate is could-not-check in the projection that needs it, and a supplied coordinate that disagrees with the selected brief is refused; no repository silently stands in for another."
  - "Production consumers are not routed in this brief. Each consumer named in the census moves onto the subject only in the follow-up brief that edits it (tool-refactoring/02, /09 and /10), so independent current-state, custody and permission checks at each effect boundary are unchanged here."
---

**Ruling recorded (2026-11-21): option 1.** The brief's `## Human decision` asked whether to
approve the bounded refactor after its independent class and caller controls pass, or to keep
the existing behavior and request a narrower migration. The driver (`human:<name>`) ruled on
the brief's decision-gate [issue 7087](https://github.com/medici-finance/assay/issues/7087) in a
comment posted under the driver's own login, never a role App relay. The
[ruling comment](https://github.com/medici-finance/assay/issues/7087#issuecomment-7065937224),
posted 2026-11-21T16:48:19Z and not edited since, chooses option 1 and says #7092 may proceed
once its independent class and caller controls pass.

**Record binding.** The ruling comment selects the option but does not name this record by its
id. The driver then posted a second comment on issue 7087 under the same login,
[naming this record](https://github.com/medici-finance/assay/issues/7087#issuecomment-7066509833)
(2026-11-21T17:28:35Z): `DR-tool-refactoring-01: approved as the design record for
tool-refactoring/01.` The `ruling:` link above points at that comment, so the record's
provenance names it as a whole token. This record transcribes the human acts; it makes no
independent verification claim.

**Why `consequence: major`.** A wrong tracking, delivery or Evidence coordinate can direct a
credentialed adapter at another repository. The route back exists: no production caller is
routed through the subject in this brief, so a wrong design is reversed by reverting the owner
and its adapter before the follow-up briefs land.

**Not decided here.** The consumer migrations themselves (tool-refactoring/02, /09 and /10),
and any change to a public schema, numeric exit contract, gate policy or custody boundary. Those
keep their own owners and approvals.
