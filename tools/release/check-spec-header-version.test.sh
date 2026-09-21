#!/usr/bin/env bash
# Offline, network-free unit test for check-spec-header-version.sh (assay#1192).
#
# Builds a throwaway spec/brief-v1.md fixture per case under a temp dir and
# runs the check against it — no git, no network, no toolchain beyond bash +
# grep. Committed fail-first evidence: case S1 (stale v0.22.0 header vs a
# v1.0.12 tag) is exactly the pre-fix state #1192 reported — the check must
# refuse it (exit 1). Case S2 is the post-fix state — the same tag, a freshened
# header — and must pass (exit 0).
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK="${CHECK_IMPL:-$here/check-spec-header-version.sh}"
case "$CHECK" in /*) ;; *) CHECK="$here/$CHECK" ;; esac

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

work="$(mktemp -d "${TMPDIR:-/tmp}/chsv-XXXXXX")"
trap 'rm -rf "$work"' EXIT

write_spec() {
  # $1 = file, $2 = header line body (e.g. "v0.22.0" or "" for missing/malformed)
  cat >"$1" <<EOF
# Brief v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT.
**Describes reference implementation:** \`statusgen\` $2

## 1. Scope
EOF
}

run_case() {
  # $1 = name, $2 = expected exit, remaining = args to the check
  local name="$1" want="$2"; shift 2
  local got out
  out=$(bash "$CHECK" "$@" 2>&1); got=$?
  if [ "$got" = "$want" ]; then
    ok "$name (exit $got)"
  else
    bad "$name (want exit $want, got $got) — output: $out"
  fi
}

# S1 — the committed fail-first row: the header still names the stale
# v0.22.0 statusgen version (#302's freshen target) but the release being cut
# is v1.0.12 — the exact drift #1192 reported (v0.22.0 vs a released tag
# dozens of versions ahead). Must REFUSE the release (exit 1).
stale="$work/stale-brief-v1.md"
write_spec "$stale" "v0.22.0"
run_case "S1 stale header (v0.22.0) vs tag v1.0.12 -> refused" 1 v1.0.12 "$stale"

# S2 — the post-fix state: header freshened to match the tag being cut. Must
# PASS (exit 0).
fresh="$work/fresh-brief-v1.md"
write_spec "$fresh" "v1.0.12"
run_case "S2 freshened header (v1.0.12) vs tag v1.0.12 -> passes" 0 v1.0.12 "$fresh"

# S3 — a header ahead of the tag being cut (e.g. re-cutting an older patch)
# is still a mismatch, not just "behind" — the check compares equality, not
# an ordering.
ahead="$work/ahead-brief-v1.md"
write_spec "$ahead" "v1.0.12"
run_case "S3 header ahead of the tag being cut -> refused" 1 v1.0.0 "$ahead"

# S4 — no 'Describes reference implementation:' line at all -> could-not-check
# (exit 2), never a quiet pass.
noline="$work/noline-brief-v1.md"
printf '# Brief v1.0-draft\n\nNo header line here.\n' >"$noline"
run_case "S4 missing header line -> could-not-check" 2 v1.0.12 "$noline"

# S5 — a header line present but with no parseable vX.Y.Z token -> could-not-check.
malformed="$work/malformed-brief-v1.md"
write_spec "$malformed" "TBD"
run_case "S5 unparseable version token -> could-not-check" 2 v1.0.12 "$malformed"

# S6 — an unreadable spec file -> could-not-check.
run_case "S6 missing spec file -> could-not-check" 2 v1.0.12 "$work/does-not-exist.md"

# S7 — a malformed tag argument is a usage error, independent of the file.
run_case "S7 malformed tag argument -> usage error" 2 "not-a-version" "$fresh"

# S8 — default file path argument (spec/brief-v1.md relative to cwd) is
# honoured when the caller supplies no explicit file.
mkdir -p "$work/defaultcwd/spec"
write_spec "$work/defaultcwd/spec/brief-v1.md" "v1.0.12"
out=$(cd "$work/defaultcwd" && bash "$CHECK" v1.0.12 2>&1); got=$?
if [ "$got" = 0 ]; then
  ok "S8 default spec/brief-v1.md path resolved (exit $got)"
else
  bad "S8 default spec/brief-v1.md path resolved (want exit 0, got $got) — output: $out"
fi

echo
echo "check-spec-header-version: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
