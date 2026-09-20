#!/bin/sh
# forge-ban.sh — advisory counting gate for the desktools-v2 seam ban.
#
# Counts every reach-around the desktools-v2 seam contract (docs/streams/desktools-v2/
# seam-contract.md) bans: a GitHub/GitLab-specific fact appearing outside the two Forge
# backends (tools/desk/internal/deskkit/forge_github.go, forge_gitlab.go) and their
# forge_github*_test.go / forge_gitlab*_test.go siblings. Four fact classes, each its own
# column:
#
#   (a) a `gh` subprocess          — exec.Command("gh", …) in Go; a shell/skill script that
#                                     shells a `gh` subcommand.
#   (b) a hardcoded remote "origin" used to decide WHICH FORGE OR WHICH REPO a Forge op
#       targets — scoped to files named *forge*.go / *resolve*.go (see seam-contract.md
#       "The counter's per-class scope": a whole-tree "origin" grep folds in ~30 ordinary
#       git-transport sites desktools-v2/01's inventory explicitly excluded as belonging to
#       desktools-go-git, not this stream, and would dilute the count past usefulness).
#   (c) a `pullRequest(` / `mergeRequest(` GraphQL query block built outside a backend.
#   (d) the GitHub REST/GraphQL host literal `api.github.com`, outside forge.go's
#       GitHubAPIBase — the ONE carve-out beyond the two backends: forge.go is the seam's own
#       interface file and the literal's single documented canonical home.
#
# SCOPE. Two trees, counted and reported SEPARATELY so a drop in one cannot hide a rise in the
# other (desktools-v2/02 facts):
#   - desk:      tools/desk/** (Go + shell), tools/cellctl/** (shell), plugins/assay/**
#                (skill shell scripts) — this stream's own territory.
#   - statusgen: statusgen/** (Go) — NOT under `forgeban` today; brought under this ban so
#                a sibling migration's progress (example-stream/18) is visible as a falling
#                count (desktools-v2/08 holds the zero once it reaches it).
# .github/workflows/** is deliberately NOT scanned: its `gh` calls run as standalone CI steps
# under the workflow's own token, in a different execution context with no Go seam to reach
# (desktools-v2/01 inventory group H).
#
# COUNTING (advisory) MODE: prints the counts and exits 0, always — this is the recorded
# baseline, not a failure, exactly as desktools-go-git's count-git-exec.sh landed. It flips to
# FAILING only after a migration drives N down (desktools-v2/08 for the statusgen half; the
# desk-tools half in a later, not-yet-authored brief).
#
# --baseline: also writes (upserts) the `desktools-v2/02 <N>` line in
# docs/streams/desktools-v2/forge-ban-baseline.txt — the machine-readable baseline a later
# brief's "strictly lower than" row compares against, instead of a PR-body sentence.
#
# Portable: POSIX sh, grep -E, find, awk. No GNUisms (macOS + Linux). Follows the same shape
# as tools/desk/scripts/count-git-exec.sh.

set -u
cd "$(dirname "$0")/.." || exit 2   # now at tools/desk
ROOT="$(cd ../.. && pwd)" || exit 2  # repo root

DESK="$ROOT/tools/desk"
CELLCTL="$ROOT/tools/cellctl"
SKILLS="$ROOT/plugins/assay"
STATUSGEN="$ROOT/statusgen"

# The two backends and their test siblings — excluded from every class. Named explicitly
# (forge_github.go, forge_gitlab.go) so a reader can grep this file for either name and find
# the exemption; matched by prefix (no .go suffix) so every *_test.go sibling
# (forge_github_auth_test.go, forge_gitlab_typed_test.go, …) is excluded too, not just the two
# main files.
BACKEND_EXCLUDE='forge_github|forge_gitlab'
# The seam's own interface file — the sole carve-out, for class (d) only (see header).
FORGE_GO_EXCLUDE='/deskkit/forge\.go:'

count_lines() {
  # count_lines <grep -c output as "file:N" lines on stdin> -> sums the counts
  awk -F: '{s+=$NF} END{print s+0}'
}

# ---------- class (a): gh subprocess ----------

# Go: exec.Command("gh", …)
gh_go() {
  dir="$1"
  grep -rEn 'exec\.Command\("gh"' "$dir" --include='*.go' 2>/dev/null \
    | grep -v -E "$BACKEND_EXCLUDE" \
    | grep -v '_test\.go:' \
    | wc -l | tr -d ' '
}

# Shell: a script that shells a `gh` subcommand (issue/pr/api/auth/repo/release/workflow/run/
# label/search/browse), matched with a left boundary so "gh" is a whole word and a right
# boundary after the subcommand. forge-ban.sh itself is excluded (its own header/comments name
# these subcommands as prose, not an invocation) and *.test.sh fixtures are excluded the same
# way *_test.go is for Go.
GH_SUBCMDS='issue|pr|api|auth|repo|release|workflow|run|label|search|browse'
gh_sh() {
  dir="$1"
  grep -rEn "(^|[^A-Za-z0-9_.-])gh (${GH_SUBCMDS})([^A-Za-z0-9_-]|\$)" "$dir" --include='*.sh' 2>/dev/null \
    | grep -v -E '/forge-ban\.sh:' \
    | grep -v -E '\.test\.sh:' \
    | wc -l | tr -d ' '
}

GH_GO_DESK=$(gh_go "$DESK")
GH_SH_DESK=$(gh_sh "$DESK")
# cellctl's main script is extensionless (no --include filter), but its tests/*.test.sh
# fixtures are excluded the same way every other tree's .test.sh/_test.go is.
GH_SH_CELLCTL=$(grep -rEn "(^|[^A-Za-z0-9_.-])gh (${GH_SUBCMDS})([^A-Za-z0-9_-]|\$)" "$CELLCTL" 2>/dev/null \
  | grep -v -E '\.test\.sh:' \
  | wc -l | tr -d ' ')
GH_SH_SKILLS=$(gh_sh "$SKILLS")
CLASS_A_DESK=$((GH_GO_DESK + GH_SH_DESK + GH_SH_CELLCTL + GH_SH_SKILLS))
CLASS_A_STATUSGEN=$(gh_go "$STATUSGEN")

# ---------- class (b): hardcoded "origin" deciding which forge/repo (scoped) ----------

origin_scoped() {
  dir="$1"
  files=$(find "$dir" -type f -name '*.go' \( -name '*forge*' -o -name '*resolve*' \) 2>/dev/null \
    | grep -v -E "$BACKEND_EXCLUDE" \
    | grep -v '_test\.go$')
  [ -z "$files" ] && { echo 0; return; }
  printf '%s\n' "$files" | xargs grep -cE '"origin"' 2>/dev/null | count_lines
}

CLASS_B_DESK=$(origin_scoped "$DESK")
CLASS_B_STATUSGEN=$(origin_scoped "$STATUSGEN")

# ---------- class (c): pullRequest(/mergeRequest( GraphQL block outside a backend ----------

gql() {
  dir="$1"
  grep -rEn '(pullRequest|mergeRequest)\(' "$dir" --include='*.go' 2>/dev/null \
    | grep -v -E "$BACKEND_EXCLUDE" \
    | grep -v '_test\.go:' \
    | wc -l | tr -d ' '
}

CLASS_C_DESK=$(gql "$DESK")
CLASS_C_STATUSGEN=$(gql "$STATUSGEN")

# ---------- class (d): api.github.com host literal outside forge.go and the backends ----------

host_literal() {
  dir="$1"
  exclude_forge_go="$2"   # "yes" for desk tree (forge.go lives there), "no" for statusgen
  out=$(grep -rEn 'api\.github\.com' "$dir" --include='*.go' 2>/dev/null \
    | grep -v -E "$BACKEND_EXCLUDE" \
    | grep -v '_test\.go:')
  if [ "$exclude_forge_go" = "yes" ]; then
    out=$(printf '%s\n' "$out" | grep -v -E "$FORGE_GO_EXCLUDE")
  fi
  printf '%s\n' "$out" | grep -c . 2>/dev/null | tr -d ' '
}

CLASS_D_DESK=$(host_literal "$DESK" yes)
CLASS_D_STATUSGEN=$(host_literal "$STATUSGEN" no)

# ---------- totals ----------

DESK_TOTAL=$((CLASS_A_DESK + CLASS_B_DESK + CLASS_C_DESK + CLASS_D_DESK))
STATUSGEN_TOTAL=$((CLASS_A_STATUSGEN + CLASS_B_STATUSGEN + CLASS_C_STATUSGEN + CLASS_D_STATUSGEN))
TOTAL=$((DESK_TOTAL + STATUSGEN_TOTAL))

echo "forge reach-around sites: $TOTAL (desk: $DESK_TOTAL, statusgen: $STATUSGEN_TOTAL)"
echo "  class a (gh subprocess):                    desk=$CLASS_A_DESK statusgen=$CLASS_A_STATUSGEN"
echo "  class b (hardcoded origin, forge/resolve-scoped): desk=$CLASS_B_DESK statusgen=$CLASS_B_STATUSGEN"
echo "  class c (pullRequest/mergeRequest GraphQL): desk=$CLASS_C_DESK statusgen=$CLASS_C_STATUSGEN"
echo "  class d (api.github.com host literal):      desk=$CLASS_D_DESK statusgen=$CLASS_D_STATUSGEN"
echo "statusgen sites: $STATUSGEN_TOTAL"

if [ "${1:-}" = "--baseline" ]; then
  BASELINE_FILE="$ROOT/docs/streams/desktools-v2/forge-ban-baseline.txt"
  LINE="desktools-v2/02 $TOTAL"
  if [ -f "$BASELINE_FILE" ] && grep -qE '^desktools-v2/02 [0-9]+$' "$BASELINE_FILE"; then
    # upsert: replace the existing desktools-v2/02 line rather than appending a duplicate,
    # so a re-run stays a single machine-readable line for this brief's key.
    TMP=$(mktemp "${TMPDIR:-/tmp}/forge-ban-baseline.XXXXXX")
    awk -v line="$LINE" '{ if ($1 == "desktools-v2/02") print line; else print }' "$BASELINE_FILE" > "$TMP"
    mv "$TMP" "$BASELINE_FILE"
  else
    mkdir -p "$(dirname "$BASELINE_FILE")"
    printf '%s\n' "$LINE" >> "$BASELINE_FILE"
  fi
  echo "baseline written: $LINE ($BASELINE_FILE)"
fi

exit 0
