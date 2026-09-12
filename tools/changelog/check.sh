#!/usr/bin/env bash
# changelog-check — the PR-gate decision, extracted so it is unit-testable
# offline (tools/changelog/check_test.sh) and so the staged workflow stays thin.
#
# THE CONVENTION IT ENFORCES. A notable change records itself by ADDING one
# fragment file under changelog/ (changelog/<slug>.md); a genuinely non-notable
# PR carries the changelog:skip label instead. The retired path — hand-editing
# CHANGELOG.md's `## Unreleased` section — is now REFUSED: that shared section
# was a standing merge-conflict generator, and CHANGELOG.md is written only by
# the release workflow (which aggregates the fragments).
#
# THREE OUTCOMES, decided cheaply from the diff (no semantic classifier):
#   REFUSE  — the PR adds a bullet under `## Unreleased` in CHANGELOG.md
#             (deprecation guard; absolute — checked first, a skip label does
#             not excuse it).
#   PASS    — the changelog:skip label is present, OR the PR adds/updates a
#             fragment file under changelog/ (other than README.md), OR — the
#             PROXY path, for a PR whose branch maintainers cannot commit to —
#             a fragment named changelog/pr-<PR_NUMBER>-<slug>.md already sits
#             on the BASE branch carrying a real highlight bullet.
#   FAIL    — none of the above: no fragment, no proxy, no skip.
#
# INPUTS (env):
#   SKIP        "true" when the PR carries the changelog:skip label, else "false"
#   BASE_SHA    the PR base commit
#   HEAD_SHA    the PR head commit
#   HEAD_REF    the PR head branch name (optional; used only to suggest the exact
#               changelog/<slug>.md path in the failure message; degrades to the
#               literal <slug> when unset, so the script half can merge before the
#               workflow half that supplies it)
#   PR_NUMBER   the PR's number (optional; enables the PROXY path below). Any
#               value that is not a positive integer — including the literal
#               unexpanded expression a workflow that has not been updated yet
#               would pass — is treated as UNSET, so the script half may merge
#               before the workflow half that supplies it and every existing
#               outcome stays byte-identical until it does.
#   CHANGELOG_AGG  path to aggregate.py (default: alongside this script)
#
# THE PROXY PATH (fork PRs). A fragment must be added by the PR itself — except
# when it CANNOT be: a pull request from a fork whose branch the maintainers
# cannot commit to. Labelling such a PR changelog:skip would silently drop a
# notable change from the release notes, so instead a maintainer lands the
# fragment on the BASE branch under the reserved name
# changelog/pr-<N>-<slug>.md, and this check accepts it on that PR's behalf.
# The name is what binds the proxy to exactly one PR: <N> is matched against
# PR_NUMBER, so a proxy for one PR can never green another. The proxy is read
# from the BASE commit (never from the untrusted head), and the base SHA the
# gate is handed is re-resolved on every PR event — which is why re-running the
# check after landing the proxy is just "add or remove a label" (this leg
# re-runs on labeled/unlabeled).
#
# It reads git history only; it contacts no network and needs no toolchain
# beyond git + python3.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGG="${CHANGELOG_AGG:-$here/aggregate.py}"
SKIP="${SKIP:-false}"
: "${BASE_SHA:?BASE_SHA is required}"
: "${HEAD_SHA:?HEAD_SHA is required}"

# The suggested fragment path in the failure message. When HEAD_REF is supplied
# (by the workflow) the slug is its basename, so the message names the EXACT file
# to create; unset, it degrades to the literal <slug> placeholder.
slug="<slug>"
[ -n "${HEAD_REF:-}" ] && slug="$(basename "$HEAD_REF")"

# The Unreleased bullet ENTRIES at a given commit's CHANGELOG.md, or nothing when
# the file is absent there. Piped through aggregate.py so the parse matches the
# release-time aggregator exactly.
unreleased_at() {
  local sha="$1" tmp
  tmp="$(mktemp "${TMPDIR:-/tmp}/cl-XXXXXX")"
  if git show "${sha}:CHANGELOG.md" >"$tmp" 2>/dev/null; then
    python3 "$AGG" unreleased-bullets "$tmp" || true
  fi
  rm -f "$tmp"
}

# 1) DEPRECATION GUARD (absolute, first). Compare the Unreleased bullet sets at
#    base and head; any bullet present at head but not at base is a NEW direct
#    edit of the retired section. Set difference means a pre-existing residual
#    bullet (untouched by this PR) never trips the guard — only an addition does.
base_bul="$(unreleased_at "$BASE_SHA" || true)"
head_bul="$(unreleased_at "$HEAD_SHA" || true)"
added="$(comm -13 <(printf '%s\n' "$base_bul" | sort -u) <(printf '%s\n' "$head_bul" | sort -u) | sed '/^$/d' || true)"
if [ -n "$added" ]; then
  echo "::error title=Unreleased edit refused::This PR adds highlight line(s) under '## Unreleased' in CHANGELOG.md. That section is RETIRED — it was a standing merge-conflict source. Record the change as a fragment file instead: changelog/<slug>.md (one-to-few highlight lines; see changelog/README.md). CHANGELOG.md is written only by the release workflow, which aggregates the fragments at cut time."
  echo "Refused — added under '## Unreleased':"
  printf '%s\n' "$added"
  exit 1
fi

# 2) SKIP label — the escape hatch for a genuinely non-notable PR. Always printed
#    so a skipped changelog is visible in the check record, never silent.
if [ "$SKIP" = "true" ]; then
  echo "::notice title=changelog:skip::PR carries the 'changelog:skip' label — a changelog fragment is not required for this PR."
  echo "SKIP: changelog:skip label present — fragment requirement waived for this PR."
  exit 0
fi

# 3) FRAGMENT added/updated? Any added-or-modified changelog/*.md other than
#    README.md satisfies the gate. --diff-filter=AM: an added fragment (the norm)
#    or a modified one; a deletion never satisfies it.
frag="$(git diff --name-status --diff-filter=AM "${BASE_SHA}...${HEAD_SHA}" -- 'changelog/' \
          | awk '{print $2}' \
          | grep -E '^changelog/.+\.md$' \
          | grep -v '^changelog/README\.md$' || true)"
if [ -n "$frag" ]; then
  # FILENAME is not enough — a 0-byte, whitespace-only, or bullet-less fragment
  # satisfies the name but records NOTHING (aggregate.py lifts only bullet lines,
  # so a bullet-less fragment contributes zero to the release). Reject it, or the
  # gate is satisfiable by `touch changelog/x.md`. Require at least one added/
  # modified fragment to carry a real highlight bullet (`- …`), read from HEAD.
  have_bullet=0
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    if git show "${HEAD_SHA}:${f}" 2>/dev/null | grep -qE '^[[:space:]]*-[[:space:]]+[^[:space:]]'; then
      have_bullet=1
      break
    fi
  done <<< "$frag"
  if [ "$have_bullet" = 1 ]; then
    echo "PASS: this PR adds/updates changelog fragment(s) with content:"
    printf '%s\n' "$frag"
    exit 0
  fi
  echo "::error title=empty changelog fragment::This PR adds/updates changelog fragment file(s) but none carries a highlight bullet. An empty, whitespace-only, or bullet-less fragment records nothing and is rejected — the gate is not satisfiable by 'touch changelog/x.md'. Put at least one '- …' highlight line in the fragment (see changelog/README.md), or label the PR 'changelog:skip' if the change is genuinely non-notable."
  printf 'fragment file(s) with no highlight bullet:\n%s\n' "$frag"
  exit 1
fi

# 4) PROXY FRAGMENT on the BASE branch — the fork-PR path. Consulted only when
#    the ordinary in-PR fragment rule above did not fire, deliberately: the
#    ordinary path stays the first and cheapest decision, the proxy is the
#    exception. Ordering it here rather than ahead of rule 3 also keeps a
#    bullet-less fragment that the PR ITSELF added a loud failure (its author
#    can fix that one) instead of letting a proxy mask it. Either position
#    leaves the PR_NUMBER-unset behaviour byte-identical — the block is a no-op
#    without it — so the tie is broken on which failure stays visible.
if [ -n "${PR_NUMBER:-}" ] && printf '%s' "$PR_NUMBER" | grep -qE '^[1-9][0-9]*$'; then
  # Read the BASE tree, not the diff and not the head: the proxy is a file a
  # maintainer already merged to the base branch, so it appears in NEITHER the
  # PR diff nor (for a fork PR) anything the contributor controls.
  proxy="$(git ls-tree -r --name-only "$BASE_SHA" -- 'changelog/' 2>/dev/null \
             | grep -E "^changelog/pr-${PR_NUMBER}-[A-Za-z0-9._-]+\.md$" || true)"
  if [ -n "$proxy" ]; then
    # Same content bar as rule 3: the NAME is not enough, at least one proxy
    # must carry a real highlight bullet, read from the base commit.
    proxy_bullet=0
    while IFS= read -r f; do
      [ -n "$f" ] || continue
      if git show "${BASE_SHA}:${f}" 2>/dev/null | grep -qE '^[[:space:]]*-[[:space:]]+[^[:space:]]'; then
        proxy_bullet=1
        break
      fi
    done <<< "$proxy"
    if [ "$proxy_bullet" = 1 ]; then
      echo "PASS: proxy fragment(s) for PR #${PR_NUMBER} found on the base branch:"
      printf '%s\n' "$proxy"
      exit 0
    fi
    echo "::error title=empty changelog fragment::The proxy fragment(s) for this PR on the base branch carry no highlight bullet. An empty, whitespace-only, or bullet-less fragment records nothing and is rejected — the gate is not satisfiable by 'touch changelog/x.md'. Put at least one '- …' highlight line in the proxy fragment (see changelog/README.md), or label the PR 'changelog:skip' if the change is genuinely non-notable."
    printf 'proxy fragment(s) with no highlight bullet:\n%s\n' "$proxy"
    exit 1
  fi
fi

echo "::error title=missing changelog fragment::This PR adds no changelog fragment and carries no 'changelog:skip' label. Fix: create changelog/${slug}.md with at least one highlight bullet, e.g. printf '### Fixed\n- <one line>\n' > changelog/${slug}.md then commit and push. The ### Added / ### Fixed / ### Changed bucket heading is optional; a bullet is not — an empty, whitespace-only, or bullet-less fragment is rejected. NEVER edit CHANGELOG.md (it is written only by the release workflow, which aggregates the fragments). The 'changelog:skip' waiver is a maintainer act, not one you self-apply. For a PR whose branch maintainers cannot commit to (a fork), a maintainer may instead land changelog/pr-<N>-<slug>.md on the base branch, then re-run this check by adding or removing a label — see changelog/README.md."
echo "MISSING: no changelog/<slug>.md fragment between base ${BASE_SHA} and head ${HEAD_SHA} — suggested path: changelog/${slug}.md"
exit 1
