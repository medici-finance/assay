# Evidence auto-merge

The scheduled poll discovers ready, verifier-authored Evidence PRs on main.
Drafts still require review and a ready flip. The existing author and changed-file
guards remain in force; no PR source is checked out or executed.

For each eligible PR the workflow asks GitHub to enable auto-merge, using a token
minted from the board-writer App. That is a request: GitHub merges only once every
branch requirement is satisfied, and the merge is attributed to the identity that
enabled it. `automerge-refusal.sh` decides which refusals of that request are
benign and which redden the run.

## Clean PRs wait for a human merge

GitHub refuses the auto-merge request when a PR is already in clean status,
because nothing is left to wait on. The classifier treats that refusal as benign
(exit 0, no step output) and the PR waits for a human merge.

An earlier version completed those merges directly with the workflow token. Main's
push/merge restriction now refuses that token. The recorded ruling on #997 chose
to drop the direct merge rather than give it a bypass-capable credential, so this
workflow holds no write permission on the workflow token and never merges a PR
itself.

## Activation

The staged copy in `ci/staged-workflows/` and the live workflow are identical.
A workflow-file change is promoted by a maintainer, because no App may push a
`.github/workflows/*` file.

## Offline checks

`python3 tools/evidence-automerge/automerge-poll_test.py` runs the discovery and
guard steps against a stub API and pins the workflow's jobs, steps, tokens and
permissions, including the absence of any direct-merge step.
`automerge-refusal_test.sh` and `automerge-step_test.sh` cover the refusal
classifier and the real request step. The live build-test regression entrypoint
runs all three.
