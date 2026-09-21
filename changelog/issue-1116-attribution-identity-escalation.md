### Changed
- `statusgen`'s git-committer-identity cross-check (`attribution.go`) now escalates a
  same-identity author/verifier pair to a hard `PROBLEM` — not just a `NOTICE` — when
  (a) the repo's brief history carries more than one git identity (so identity is
  genuinely discriminating) and (b) the brief's Verified/Evidence tokens self-label as
  independent (the token layer alone would have passed it). This closes the
  security-hardening/27 Task 2/4(b) gap tracked as #1116: a same-identity pair that
  avoids the free-text "implementer" token previously only ever produced a `NOTICE`.
  A repo whose entire checked brief history shares one git identity (a solo-maintainer
  or single-App-identity workflow) is unaffected — that case stays a `NOTICE`, by
  design, since identity cannot discriminate there.
- `statusgen`'s committer-identity cross-check now uses a precise Evidence-section
  signal for its escalation candidates, replacing the whole-file "most recent commit
  touching the path" proxy the desk's PR-review round-1 decision (assay#1277) asked
  to be narrowed. `evidenceSectionTouchedByOtherIdentity` (`gitinfo.go`) asks whether
  an identity OTHER than the brief's author ever touched the `## Evidence` section
  specifically, anywhere in its history — not just whichever commit happens to be
  newest against the whole file. This closes two false-escalation shapes found live
  against this repo's own tree during review: (1) a later, unrelated, repo-wide
  mechanical commit (e.g. a brief-schema migration) that never touched Evidence at
  all resetting the whole-file signal past a genuine independent verification commit;
  (2) a later same-identity commit that appends a caveat/addendum *inside* the
  Evidence section after independent verification already landed (e.g. the
  implementer recording a security-review residual), which a narrower
  "most-recently-touched-Evidence" signal still misread as self-verification.
  `statusgen --lint` against this repo's own tree went from 37 false hard `PROBLEM`s
  to 0 after this refinement.
