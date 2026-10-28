# Land the write-policy CI and release qualification hunk

The worker App does not hold workflow-write permission. The extraction in
[#3772](https://github.com/medici-finance/assay/pull/3772) is merged; its branch
is retired. The human workflow owner landed the additions in
[#3793](https://github.com/medici-finance/assay/pull/3793). The recorded
[write-policy-ci.diff](write-policy-ci.diff) and procedure below describe that
landing; do not repeat it on a head where the patch is already applied. The
release workflow's staged twin must remain byte-identical to the live file.
Use the existing human-applied workflow leg described in
[go-analysis-ci-hunk.md](go-analysis-ci-hunk.md). The proposed workflow-App decision
record is still unapproved and supplies no alternative authority.

The diff adds the deskcore suite to the unfiltered source build-test loop and to
the existing desk-tools release-test leg. The desk suite already runs actual
built-hook parity, consumer and mutation fixtures. Existing jobs, conditions,
permissions and checks are preserved. A runbook or local pass is not CI activation.

From a human-owned isolated checkout, set `REVIEWED_HEAD` to the exact full
follow-up SHA reviewed on `codex/k8s-write-policy-ci-followup`, `PR_NUMBER` to
that open follow-up PR, and `AUTHORIZED_ACCOUNT` to the human's authorized
GitHub account. The original merged PR is never a landing target. These commands
must run under the human's own authorized Git identity. Run this entire block
together: its subshell keeps the guard, checkout and failure handling in one
session, and a failed check exits that subshell without closing your terminal.
Inspect the displayed diff and type `push` only when the two additions are correct:

```sh
(
set -eu
: "${REVIEWED_HEAD:?Set the full follow-up SHA you reviewed}"
: "${PR_NUMBER:?Set the open follow-up PR number}"
: "${AUTHORIZED_ACCOUNT:?Set your authorized GitHub account}"
check_current_pr() {
  current_pr="$(GH_TOKEN="$(gh auth token --hostname github.com --user "$AUTHORIZED_ACCOUNT")" &&
    test -n "$GH_TOKEN" &&
    GH_TOKEN="$GH_TOKEN" gh pr view "$PR_NUMBER" --repo medici-finance/assay \
      --json state,headRefName,headRefOid \
      --jq '.state + " " + .headRefName + " " + .headRefOid')" || return 1
  test "$current_pr" = "OPEN codex/k8s-write-policy-ci-followup $REVIEWED_HEAD"
}
check_current_pr
case "$REVIEWED_HEAD" in *[!0-9a-f]*|'') exit 1 ;; esac
test "${#REVIEWED_HEAD}" -eq 40
git fetch origin codex/k8s-write-policy-ci-followup
test "$(git rev-parse refs/remotes/origin/codex/k8s-write-policy-ci-followup)" = "$REVIEWED_HEAD"
CI_REVIEW_TREE=$(mktemp -d "${TMPDIR:-/tmp}/write-policy-workflow.XXXXXX")
git worktree add --detach "$CI_REVIEW_TREE" "$REVIEWED_HEAD"
cd "$CI_REVIEW_TREE"
git apply --check docs/runsheets/write-policy-ci.diff
git apply docs/runsheets/write-policy-ci.diff
git diff --check
git diff -- .github/workflows/ci.yml .github/workflows/release.yml
printf 'Review the two workflow additions above. Type push to commit and push: '
IFS= read -r CI_CONFIRM </dev/tty
test "$CI_CONFIRM" = push
test "$(git diff --name-only)" = "$(printf '%s\n' .github/workflows/ci.yml .github/workflows/release.yml)"
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m 'ci: qualify shared write policy in source and release tests'
check_current_pr
git push origin HEAD:refs/heads/codex/k8s-write-policy-ci-followup
)
```

Do not dispatch a workflow. The implementation worker must fetch and merge the
human's branch commit before another push. The independent review desk checks the
normal PR run at that exact head; merged-main verification and the normal release
lane remain separate. If the hunk stops applying or checks fail, preserve the
failure and reconcile source; do not copy a whole stale workflow over current main.

The source module replacement is build plumbing. The synthetic offline consumer
fixture proves packaging; a separate check resolved the immutable public source
pseudo-version `v0.0.0-20261020202831-a5c75d60d584` for
`github.com/medici-finance/assay/deskcore`, with these authentic sums:

```go.sum
github.com/medici-finance/assay/deskcore v0.0.0-20261020202831-a5c75d60d584 h1:EFIFHAvlGWz8l0/W19D/6AQq29MWxfcCKnF58qC6OZ0=
github.com/medici-finance/assay/deskcore v0.0.0-20261020202831-a5c75d60d584/go.mod h1:fT/P/8tLzSLzogIRNMLXVu8yWnqVOm3xwqf/tMjjPTg=
```

The [source handoff](../write-policy-consumers.md#module-build-and-workflow-handoff)
records the source commit, matching production blobs and independent import proof.
This available source version is not a semantic release tag. A named release stays
with the normal release owner. Actual managed consumers must independently resolve
an admitted exact pin with `GOWORK=off` and qualify allocation, write roots and
sandbox enforcement. Neither this workflow hunk nor importing `managed/v1`
supplies that qualification.
