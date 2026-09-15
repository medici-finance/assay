### Added
- **`forge-neutral` brief 19 — human-only surfaces made server-side (the closing brief of
  #992's five-brief series).** For each surface a human currently performs by hand — merge
  to a protected branch, a workflow-file push, ruleset/branch-protection edits, repo/CI
  variables, and App/OAuth installation — states plainly whether it is genuinely
  server-side-enforced today or held up by convention alone. Finds merge-to-protected-branch
  is the latter: this repo's role Apps hold `contents: write` + `pull_requests: write`,
  which is sufficient to merge outright, and the live ruleset read confirms
  `required_approving_review_count: 1` carries no restriction on which identity supplies the
  approval. Proposes a `human-approved` required status check (triggered on
  `pull_request_review`, checking the reviewing login against a trusted-human allow-list at
  the PR's head SHA) to close the gap server-side, and specifies fixture-only Verify rows
  that must never be pointed at this repo's own `main`. Doc only; no tool or workflow
  behaviour changes in this PR — the workflow file and the ruleset edit are named follow-on
  work for a human to land.
