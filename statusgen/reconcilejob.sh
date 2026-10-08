#!/usr/bin/env bash
# reconcilejob.sh — one tick of the scheduled board reconcile (derived-board/04).
#
# Called by the staged `reconcile` job in ci/staged-workflows/assay-statusgen.yml
# (from a checkout of the repo, with git identity and tokens already set up), and
# by statusgen/reconcilejob_test.go against a throwaway origin with stub
# `statusgen` and `gh` binaries. Keeping the logic here, not inline in YAML, is
# what lets the race and failure behaviour below be tested. Run a COPY that
# lives outside the checkout: the tick checks out main's tree, and bash reads a
# script file incrementally as it runs.
#
# Inputs (environment):
#   STATUSGEN    path to the statusgen binary (required)
#   REPO         owner/name the PRs are read from and the PR is opened on (required)
#   BRANCH       the carried branch (default board/reconcile)
#   BASE_BRANCH  the default branch (default main)
#   REMOTE       the git remote (default origin)
#
# What one tick does:
#   1. Fetch main. If the carried branch exists, refuse (exit 1, nothing pushed)
#      when it carries any commit this job or the desk-side writer did not make:
#      every commit on it must be a `chore(board): reconcile` commit (or a
#      `Merge` commit) whose change against one of its parents touches stream
#      READMEs only. A human commit on the branch is never overwritten.
#   2. Compute on main's tree, never on the branch's: reconcile --apply
#      (trailer-only), regen --readmes, then the full lint. The result is a pure
#      function of main plus the PR witnesses, so main moving under the branch
#      can never conflict — a stale branch is rebuilt, not patched.
#   3. Carry the result as one commit on the branch whose parents are the old
#      branch tip and (when the branch does not already contain it) main. That
#      push is a fast-forward of the branch; it is never forced. A push that
#      loses a race with another writer is rejected and fails the tick; the
#      next tick recomputes.
#   4. Open the draft PR if none is open.
#
# Every could-not-check fails the tick (nonzero exit): a reconcile that read
# no PRs, a remote that could not be listed, a PR list that could not be read.
set -euo pipefail

: "${STATUSGEN:?STATUSGEN (path to the statusgen binary) is required}"
: "${REPO:?REPO (owner/name) is required}"
branch="${BRANCH:-board/reconcile}"
base_branch="${BASE_BRANCH:-main}"
remote="${REMOTE:-origin}"
readme_re='^docs/streams/[^/]+/README\.md$'
subject_re='^chore\(board\): reconcile( |$)'

git fetch --no-tags "$remote" "$base_branch"
base="$(git rev-parse --verify 'FETCH_HEAD^{commit}')"

# A transport error must fail here, never read as "branch absent": no
# --exit-code and no silencing, so set -e sees git's own failure.
heads="$(git ls-remote --heads "$remote" "refs/heads/${branch}")"
tip=""
if [ -n "$heads" ]; then
  git fetch --no-tags "$remote" "+refs/heads/${branch}:refs/remotes/${remote}/${branch}"
  tip="$(git rev-parse --verify "refs/remotes/${remote}/${branch}^{commit}")"
  foreign=""
  for c in $(git rev-list "${base}..${tip}"); do
    subj="$(git log -1 --format=%s "$c")"
    parents="$(git rev-list --parents -n 1 "$c" | cut -d' ' -f2-)"
    nparents="$(echo "$parents" | wc -w | tr -d ' ')"
    if [ "$nparents" -ge 2 ]; then
      echo "$subj" | grep -q -E "${subject_re}|^Merge " || { foreign="${foreign}${c} ${subj}"$'\n'; continue; }
    else
      echo "$subj" | grep -q -E "$subject_re" || { foreign="${foreign}${c} ${subj}"$'\n'; continue; }
    fi
    readme_only=false
    for p in $parents; do
      if [ -z "$(git diff --name-only "$p" "$c" | grep -v -E "$readme_re" || true)" ]; then
        readme_only=true
        break
      fi
    done
    "$readme_only" || foreign="${foreign}${c} ${subj} (touches more than stream READMEs)"$'\n'
  done
  if [ -n "$foreign" ]; then
    echo "::error::${branch} carries commit(s) this job did not make — not overwriting them. Close its PR and delete the branch (or land those commits elsewhere), and the next tick re-cuts it:"
    printf '%s' "$foreign"
    exit 1
  fi
fi

# Compute on main's tree, with HEAD at main, exactly as a fresh cut would.
git checkout -q --detach "$base"
out="$(mktemp "${TMPDIR:-/tmp}/reconcile-out.XXXXXX")"
"$STATUSGEN" reconcile --apply --root . --repo "$REPO" > "$out"
"$STATUSGEN" regen --readmes --root .
"$STATUSGEN" --root . --lint
# This job may change stream READMEs and nothing else.
stray="$(git status --porcelain --untracked-files=all | grep -v -E '^ M docs/streams/[^ ]+/README\.md$' || true)"
if [ -n "$stray" ]; then
  echo "::error::reconcile changed paths other than stream READMEs — refusing to commit:"
  echo "$stray"
  exit 1
fi
git add -- 'docs/streams/*/README.md'
tree="$(git write-tree)"

if [ -z "$tip" ]; then
  if [ "$tree" = "$(git rev-parse "${base}^{tree}")" ]; then
    echo "no stream README changed state — nothing to carry"
    exit 0
  fi
  set -- -p "$base"
else
  if [ "$tree" = "$(git rev-parse "${tip}^{tree}")" ] && git merge-base --is-ancestor "$base" "$tip"; then
    echo "${branch} already carries this tick's result — nothing to push"
    exit 0
  fi
  set -- -p "$tip"
  git merge-base --is-ancestor "$base" "$tip" || set -- "$@" -p "$base"
fi

msg="$(mktemp "${TMPDIR:-/tmp}/reconcile-msg.XXXXXX")"
{
  echo "chore(board): reconcile $(date -u +%F)"
  echo
  sed -n '/^reconcile --apply:/,$p' "$out"
} > "$msg"
commit="$(git commit-tree "$tree" "$@" -F "$msg")"
git push "$remote" "${commit}:refs/heads/${branch}"

open="$(gh pr list --repo "$REPO" --head "$branch" --state open --json number --jq 'length')"
if [ "$open" != "0" ]; then
  echo "an open PR already carries ${branch} — the push above updated it"
  exit 0
fi
body="$(mktemp "${TMPDIR:-/tmp}/reconcile-body.XXXXXX")"
{
  echo "Scheduled board reconcile."
  echo
  echo "Flips a stream README Status cell from todo or in-progress to implemented when a merged PR carrying that brief's \`Brief:\` trailer witnesses it, and re-renders the generated Briefs tables from brief frontmatter. Only stream README files change; Verified and Reviewed cells and everything outside the generated markers are untouched. A row whose flip would add a lint problem is held and listed below, not written."
  echo
  echo '```'
  sed -n '/^reconcile --apply:/,$p' "$out"
  echo '```'
} > "$body"
gh pr create --repo "$REPO" --draft --base "$base_branch" --head "$branch" --title 'chore(board): reconcile' --body-file "$body"
