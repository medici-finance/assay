### Added
- The review-tick conformance walk's OFFLINE half: the GitLab contract corpus
  (`TestForgeGitlabGolden`) now pins, per tick verb — board read, review dispatch, verdict,
  escalation filing, workpad edit, Evidence landing, ready-flip — at least one refusal the verb
  is built on, next to the success path it already pinned: a forbidden open-changes list, a
  project payload with no visibility field, an unnamed label, a forbidden dedupe search and
  issue create, a forbidden thread read, an absent Evidence target, an append-only shrink, a
  marker-only draft title, a merge request still draft after its marker is cleared, and an
  empty required-checks branch. Three mutation entries prove the new refusals are load-bearing.
- The stream's pilot report gains the per-verb conformance table (§7) with every LIVE cell
  left `could-not-check — live walk not yet run` and the offline column filled from the
  goldens; the issue → brief map covers the 2026-09-15 field reports.
