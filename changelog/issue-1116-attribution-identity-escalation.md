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
  design, since identity cannot discriminate there. **Known limitation, flagged for
  human review on the PR:** the underlying identity signal is "most recent commit
  touching the brief file," which an unrelated later commit (e.g. a repo-wide
  migration) can reset past a genuinely independent verification commit, producing
  false escalations — see the PR's `NEEDS_CONTEXT` section for a concrete
  reproduction against this repo's own history.
