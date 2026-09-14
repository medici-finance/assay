### Added

- The `forge-gitlab` stream board now states an explicit **finish line** for "the review desk works
  on GitLab" — which verbs, on which tier, under which credentials, and with which two instruments
  (a live per-verb conformance table carrying at least one refusal row, plus the offline fixture
  goldens) — so the claim can be checked rather than felt.
- A **sequencing note** recording why this stream is planned rather than fanned out: its fixes
  uncover the next latent defect in sequence, so discovering that sequence one field report at a
  time costs an adopter round trip per link. It carries two standing rules — a GitLab review-desk
  issue routes to the plan's map before it is dispatched, and a code read is never a field verdict.
- An **issue → brief map** placing every open GitLab review-desk, adopter-path and stream issue
  against exactly one brief, or out of scope with a reason.
- Four briefs: **13** (board reads degrade per row, never per sweep), **14** (the public-repo gate
  reads the forge that serves the repo, at every site), **15** (the three keys the GitLab runbook
  never names — forge binding, board-push credential, source-pin lane), and **16** (the human-gated
  live review-tick conformance walk and adopter-backlog close-out).

### Changed

- The critical path is re-cut into three tracks, with the open one first. **Its verified head is
  brief 14**: two of the seven verbs in a review tick still send the public-repo gate's live
  visibility read to a hardcoded GitHub client instead of the backend they already resolved, so
  on a GitLab project the desk's workpad and its Evidence landing have no working path.
  Two tempting-but-wrong heads are recorded with what was checked to rule each out.
- The head moved while the plan was being written, and the plan says so. It started at brief 13 —
  on GitLab a review approval carries no commit sha, the board's benign-merge arm asked for a diff
  against that empty sha, and the per-change refusal came back as a whole-sweep error, blanking
  the queue. That arm was fixed mid-pass and the board is visible again, so the blocker moved one
  step down the ceremony within the hour. Brief 13 stays on the plan re-baselined onto what
  landed: the instance is closed, the class of four further unguarded whole-sweep returns is not.
- Brief 10's board row corrected from `todo` to `implemented`: its work merged on 2026-09-11 and
  the hand-maintained row never moved.
