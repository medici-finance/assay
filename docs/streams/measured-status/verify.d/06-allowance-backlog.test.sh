#!/usr/bin/env bash
# Offline test of 06-allowance-backlog.sh (brief-06 Verify row 5), run as row 6.
#
# A stub `gh` on PATH serves planted forge answers from a fixture directory, so
# nothing here reads the network. Each case asserts the check's exit code and
# its output: a planted user-standing label must exit 1 and name the PR, a
# cleared backlog must exit 0 with only `examined` lines, and a malformed or
# missing set, or a forge failure, must never exit 0.
set -euo pipefail
export KUBECONFIG=/dev/null

here=$(cd "$(dirname "$0")" && pwd)
check="$here/06-allowance-backlog.sh"
work=$(mktemp -d "${TMPDIR:-/tmp}/allowance-backlog-test.XXXXXX")
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"

# The stub maps each call to one fixture file; a missing fixture is a forge
# failure (exit 4), as a real `gh` reports one.
cat >"$work/bin/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
case "$1 $2" in
"repo list") f="repos_$3" ;;
"pr list") f="prs_${4//\//_}" ;;
"api --paginate") f=$(printf '%s' "$4" | tr / _) ;;
*) echo "stub gh: unexpected call: $*" >&2; exit 4 ;;
esac
[ -f "$FIX/$f" ] || { echo "stub gh: no fixture $f" >&2; exit 4; }
cat "$FIX/$f"
STUB
chmod +x "$work/bin/gh"

fails=0
# run <name> <want-rc> <want-output> <ASSAY_ALLOWED_REPOS value or UNSET>
run() {
    local name=$1 want_rc=$2 want_out=$3 set_to=$4 out rc
    rc=0
    if [ "$set_to" = UNSET ]; then
        out=$(env -u ASSAY_ALLOWED_REPOS PATH="$work/bin:$PATH" FIX="$FIX" bash "$check" 2>&1) || rc=$?
    else
        out=$(ASSAY_ALLOWED_REPOS="$set_to" PATH="$work/bin:$PATH" FIX="$FIX" bash "$check" 2>&1) || rc=$?
    fi
    if [ "$rc" -ne "$want_rc" ] || [ "$out" != "$want_out" ]; then
        printf 'FAIL %s: rc=%s want %s\n--- got\n%s\n--- want\n%s\n' "$name" "$rc" "$want_rc" "$out" "$want_out" >&2
        fails=$((fails + 1))
    else
        echo "pass $name (rc=$rc)"
    fi
}

ev() { # ev <event> <label> <actor-type>
    printf '{"event":"%s","label":{"name":"%s"},"actor":{"type":"%s"}}' "$1" "$2" "$3"
}

# --- fixtures: o/a has one PR whose model stamp stands under a User; o/b has
# one PR whose tier stamp was re-applied by a Bot (cleared); o/c has none.
FIX="$work/fix-backlog"
mkdir -p "$FIX"
echo '[{"number":9,"labels":[{"name":"dispatched-model:example"},{"name":"size:S"}]}]' >"$FIX/prs_o_a"
# Event fixtures are in `--paginate --slurp` form: an array of pages.
printf '[[%s,%s]]' "$(ev labeled dispatched-model:example User)" "$(ev labeled size:S User)" >"$FIX/repos_o_a_issues_9_events"
echo '[{"number":4,"labels":[{"name":"dispatched-tier:strong"}]},{"number":5,"labels":[]}]' >"$FIX/prs_o_b"
printf '[[%s],[%s,%s]]' "$(ev labeled dispatched-tier:strong User)" \
    "$(ev unlabeled dispatched-tier:strong User)" "$(ev labeled dispatched-tier:strong Bot)" >"$FIX/repos_o_b_issues_4_events"
echo '[]' >"$FIX/prs_o_c"
echo '[{"nameWithOwner":"o/b"},{"nameWithOwner":"o/c"}]' >"$FIX/repos_o"

# Red: a planted user-standing label in one repository of the set.
run planted-backlog 1 "PR o/a#9 dispatched-model:example
examined 1 PRs in o/a
examined 1 PRs in o/b
examined 0 PRs in o/c" "o/a:ci:private, o/b:no-ci:public;o/c"

# Green: the same set without that repository has no PR line.
run cleared 0 "examined 1 PRs in o/b
examined 0 PRs in o/c" "o/b,o/c"

# An owner/* pattern is enumerated from the forge and de-duplicated.
run pattern-entry 0 "examined 1 PRs in o/b
examined 0 PRs in o/c" "o/*,o/b"

# A case-variant stamp label still counts (the floor lower-cases names).
echo '[{"number":7,"labels":[{"name":"Dispatched-Model:Example"}]}]' >"$FIX/prs_o_d"
printf '[[%s]]' "$(ev labeled Dispatched-Model:Example User)" >"$FIX/repos_o_d_issues_7_events"
run case-variant 1 "PR o/d#7 dispatched-model:example
examined 1 PRs in o/d" "o/d"

# Forge failure part-way through: non-zero, never a pass.
run forge-failure 4 "examined 1 PRs in o/b
stub gh: no fixture prs_o_missing" "o/b,o/missing"

# Fail-closed guards, before any forge read.
run unset 2 "could-not-check: ASSAY_ALLOWED_REPOS is unset or empty" UNSET
run empty 2 "could-not-check: ASSAY_ALLOWED_REPOS is unset or empty" ""
run no-repo 2 "could-not-check: ASSAY_ALLOWED_REPOS names no repository" ",,;"
run bare-name 2 "could-not-check: ASSAY_ALLOWED_REPOS entry assay is not owner/name or owner/*" "o/b,assay"
run two-slash 2 "could-not-check: ASSAY_ALLOWED_REPOS entry o/b/c is not owner/name or owner/*" "o/b/c"
run star-owner 2 "could-not-check: ASSAY_ALLOWED_REPOS entry o*/* has a '*' in its owner" "o*/*"
run star-name 2 "could-not-check: ASSAY_ALLOWED_REPOS entry o/a* is not owner/name or owner/*" "o/a*"

if [ "$fails" -ne 0 ]; then
    echo "FAIL: $fails case(s)" >&2
    exit 1
fi
echo "ok: allowance-backlog check"
