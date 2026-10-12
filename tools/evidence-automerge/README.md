# Evidence merge completion

The scheduled poll discovers ready, verifier-authored Evidence PRs on main.
Drafts still require review and a ready flip. The existing author and changed-file
guards remain in force; no PR source is checked out or executed.

GitHub refuses an auto-merge request when the PR is already in clean status.
The refusal classifier marks that outcome `clean=true`. The proposed workflow
then calls `merge_clean.run()` with the normal workflow token, not the
board-writer App token that bypasses main's rulesets.

The helper re-reads the PR, all changed-file and review pages, and its check
rollup. It requires both reviewer-App lanes at the current head, GitHub's
APPROVED/CLEAN/MERGEABLE state, successful leak-sweep and completed non-failing
checks. It refuses drafts, another author/base/repository, generated views,
non-docs files and renamed source files. The REST merge specifies the inspected
head SHA and merge method `merge`. GitHub independently enforces branch
protection at the write; a changed head or a newly unsatisfied requirement is
not bypassed. Read failures and rejected writes fail the job.

## Activation

The change to `ci/staged-workflows/evidence-automerge.yml` is pending maintainer
promotion; the live workflow is unchanged in this PR. After review, a maintainer
copies the staged file to `.github/workflows/evidence-automerge.yml` and restores
its `declared-changes.json` mode to `identical` in the same commit. This grants
only the main-only enable job the workflow-token write permissions needed for
protected merges. It changes no App permissions or ruleset bypass.

Until promotion, a clean refusal only emits the output: the old live workflow
has no completion step. No direct merge is silently activated by the helper
landing alone.

## Offline checks

`python3 tools/evidence-automerge/automerge-poll_test.py` exercises scope,
current-head review lanes, paginated files, pending/failing checks, changed-head
and server-refusal cases. The existing live build-test regression entrypoint
runs this suite. The shell classifier and real-step suites run alongside it.
