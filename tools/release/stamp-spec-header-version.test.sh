#!/usr/bin/env bash
# Offline, network-free unit test for stamp-spec-header-version.sh (assay#1192).
#
# Builds a throwaway spec/brief-v1.md fixture per case under a temp dir, stamps
# it, and proves the result with the real check-spec-header-version.sh — no
# git, no network, no toolchain beyond bash + grep + awk + cmp.
#
# STAMP_IMPL overrides the script under test (the mutation hook: point it at a
# broken copy and this suite must go red).
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STAMP="${STAMP_IMPL:-$here/stamp-spec-header-version.sh}"
case "$STAMP" in /*) ;; *) STAMP="$here/$STAMP" ;; esac
CHECK="$here/check-spec-header-version.sh"

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

work="$(mktemp -d "${TMPDIR:-/tmp}/sshv-XXXXXX")"
trap 'rm -rf "$work"' EXIT

write_spec() {
  # $1 = file, $2 = header version token (e.g. "v0.22.0", or "TBD" for malformed)
  cat >"$1" <<EOF
# Brief v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT.
**Describes reference implementation:** \`statusgen\` $2

## 1. Scope

Body text that mentions v0.22.0 and a second
**Describes reference implementation:** \`statusgen\` v0.1.0 line stays untouched.
EOF
}

expect_exit() {
  # $1 = name, $2 = expected exit, $3 = got, $4 = output
  if [ "$3" = "$2" ]; then ok "$1 (exit $3)"; else bad "$1 (want exit $2, got $3) — output: $4"; fi
}

# T1 — a stale header is stamped to the tag, and the check then passes.
f="$work/t1.md"; write_spec "$f" "v0.22.0"
out=$(bash "$STAMP" v1.0.39 "$f" 2>&1); got=$?
expect_exit "T1 stamp stale header (v0.22.0) to v1.0.39" 0 "$got" "$out"
out=$(bash "$CHECK" v1.0.39 "$f" 2>&1); got=$?
expect_exit "T1 check after stamp passes" 0 "$got" "$out"

# T2 — only the header token changed: the result is byte-identical to the same
# fixture written with the new version (the body's v0.22.0 mention and the
# second marker line are untouched).
want="$work/t2-want.md"; write_spec "$want" "v1.0.39"
if cmp -s "$f" "$want"; then ok "T2 only the first header token rewritten"
else bad "T2 only the first header token rewritten — diff: $(diff "$want" "$f" | head -5)"; fi

# T3 — idempotence: stamping the same tag again exits 0 and leaves the file
# byte-identical.
cp "$f" "$work/t3-before.md"
out=$(bash "$STAMP" v1.0.39 "$f" 2>&1); got=$?
expect_exit "T3 re-stamp same tag" 0 "$got" "$out"
if cmp -s "$f" "$work/t3-before.md"; then ok "T3 re-stamp leaves the file byte-identical"
else bad "T3 re-stamp leaves the file byte-identical — diff: $(diff "$work/t3-before.md" "$f" | head -5)"; fi

# T4 — stamping a DIFFERENT version, then checking against the tag being cut,
# fails: the check is a real proof of which stamp ran, not a rubber stamp.
f="$work/t4.md"; write_spec "$f" "v0.22.0"
out=$(bash "$STAMP" v1.0.38 "$f" 2>&1); got=$?
expect_exit "T4 stamp v1.0.38" 0 "$got" "$out"
out=$(bash "$CHECK" v1.0.39 "$f" 2>&1); got=$?
expect_exit "T4 check against v1.0.39 after stamping v1.0.38 -> refused" 1 "$got" "$out"

# T5 — no header line at all -> could-not-check (exit 2), file untouched.
f="$work/t5.md"; printf '# Brief v1.0-draft\n\nNo header line here, only v0.22.0.\n' >"$f"
cp "$f" "$work/t5-before.md"
out=$(bash "$STAMP" v1.0.39 "$f" 2>&1); got=$?
expect_exit "T5 missing header line -> could-not-check" 2 "$got" "$out"
if cmp -s "$f" "$work/t5-before.md"; then ok "T5 file untouched"; else bad "T5 file untouched"; fi

# T6 — header line present but with no vX.Y.Z token -> could-not-check, untouched.
# (The fixture's SECOND marker line carries a token; the stamp must not fall
# through to it.)
f="$work/t6.md"; write_spec "$f" "TBD"
cp "$f" "$work/t6-before.md"
out=$(bash "$STAMP" v1.0.39 "$f" 2>&1); got=$?
expect_exit "T6 malformed header token -> could-not-check" 2 "$got" "$out"
if cmp -s "$f" "$work/t6-before.md"; then ok "T6 file untouched"; else bad "T6 file untouched"; fi

# T7 — an unreadable / missing spec file -> could-not-check.
out=$(bash "$STAMP" v1.0.39 "$work/does-not-exist.md" 2>&1); got=$?
expect_exit "T7 missing spec file -> could-not-check" 2 "$got" "$out"

# T8 — a malformed tag argument is a usage error, and nothing is written.
f="$work/t8.md"; write_spec "$f" "v0.22.0"
cp "$f" "$work/t8-before.md"
out=$(bash "$STAMP" "1.0.39" "$f" 2>&1); got=$?
expect_exit "T8 malformed tag argument -> usage error" 2 "$got" "$out"
if cmp -s "$f" "$work/t8-before.md"; then ok "T8 file untouched"; else bad "T8 file untouched"; fi

# T9 — default file path (spec/brief-v1.md relative to cwd) is honoured.
mkdir -p "$work/defaultcwd/spec"
write_spec "$work/defaultcwd/spec/brief-v1.md" "v0.22.0"
out=$(cd "$work/defaultcwd" && bash "$STAMP" v1.0.39 2>&1); got=$?
expect_exit "T9 default spec/brief-v1.md path stamped" 0 "$got" "$out"
out=$(cd "$work/defaultcwd" && bash "$CHECK" v1.0.39 2>&1); got=$?
expect_exit "T9 default-path check after stamp passes" 0 "$got" "$out"

echo
echo "stamp-spec-header-version: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
