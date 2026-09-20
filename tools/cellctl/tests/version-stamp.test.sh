#!/usr/bin/env bash
# version-stamp.test.sh — cellctl's `--version` contract and the release-workflow packaging
# stamp (#850: ship cellctl in desk-tools-<platform>.tar.gz with a --version stamp).
#
# What it proves (each an `assert` below):
#   source tree   `cellctl --version` and `cellctl version` both print "dev" — a checkout install
#                 (docs/cellctl.md) is honest about not being a pinned release
#   sole-arg only a version query combined with any other argument is NOT recognised (falls
#                 through to normal dispatch), mirroring statusgen's contract
#   packaging     re-running the EXACT sed/grep one-liner release.yml's "Build and package
#                 desk-tools binaries" step uses (kept byte-identical here — see the comment
#                 below) against the real tools/cellctl/cellctl stamps CELLCTL_VERSION to the
#                 given release tag, changes NOTHING else in the file, the result is executable,
#                 and it reports the stamped tag when run
#   fail-closed   the release workflow's own guard — grep the stamped copy for the exact stamped
#                 line, aborting the release if absent — actually trips when the source line's
#                 shape drifts (proves the guard is live, not decorative)
#
# No network, no tmux, no cluster. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time), and several
# vars are read only inside those eval'd strings — shellcheck's flow analysis misses both.
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# The binary under test. $CELLCTL lets the SAME suite run against either implementation
# (the bash oracle, the default, or the Go port) — desk-containers/10.
CELLCTL="${CELLCTL:-$HERE/../cellctl}"; [[ "$CELLCTL" == /* ]] || CELLCTL="$PWD/$CELLCTL"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-version.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

echo "[source tree]"
out="$("$CELLCTL" --version)"
assert '--version prints "dev" in a source checkout' '[[ "$out" == "dev" ]]'
out="$("$CELLCTL" version)"
assert 'version prints "dev" in a source checkout' '[[ "$out" == "dev" ]]'

echo "[sole-arg only]"
out="$("$CELLCTL" --version --lint 2>&1)" && rc=0 || rc=$?
assert '--version combined with another arg falls through (not treated as a bare version query)' '[[ "$rc" -ne 0 || "$out" != "dev" ]]'

echo "[packaging: the release.yml one-liner, byte-for-byte]"
# This sed and its follow-up grep are copy-pasted from the "Build and package desk-tools
# binaries" step in .github/workflows/release.yml (the ${RELEASE_TAG} shell var there is a
# plain string substitution — RELEASE_TAG=v9.9.9 below stands in for it). If that step's sed/grep
# text ever changes, update BOTH places together — TestCellctlPackagedInReleaseWorkflow
# (tools/desk/internal/deskkit/version_test.go) catches the marker-level drift; this test proves
# the mechanism actually works.
RELEASE_TAG="v9.9.9"
sed "s/^CELLCTL_VERSION=\"dev\"\$/CELLCTL_VERSION=\"${RELEASE_TAG}\"/" \
  "$CELLCTL" > "$T/cellctl-staged"
grep -q "^CELLCTL_VERSION=\"${RELEASE_TAG}\"\$" "$T/cellctl-staged"
staged_rc=$?
assert 'the sed stamps CELLCTL_VERSION to the release tag' '[[ $staged_rc -eq 0 ]]'
chmod 0755 "$T/cellctl-staged"
assert 'the staged copy is executable' '[[ -x "$T/cellctl-staged" ]]'
staged_out="$("$T/cellctl-staged" --version)"
assert 'the staged copy reports the stamped tag' '[[ "$staged_out" == "$RELEASE_TAG" ]]'

# Only the CELLCTL_VERSION line may differ from source — no other drift snuck in by the stamp.
diff_lines="$(diff "$CELLCTL" "$T/cellctl-staged" | grep -c '^[<>]' || true)"
assert 'exactly one line differs from source (the version stamp, both sides of the diff)' '[[ "$diff_lines" -eq 2 ]]'

echo "[fail-closed: the release guard actually trips on drift]"
# Simulate the source line's shape drifting (e.g. reformatted, quoting changed) — the same sed
# then matches nothing, and the workflow's grep guard (copied below) must catch that rather than
# silently ship an unstamped "dev" cellctl.
sed 's/^CELLCTL_VERSION="dev"$/CELLCTL_VERSION=dev/' "$CELLCTL" > "$T/cellctl-drifted-source"
sed "s/^CELLCTL_VERSION=\"dev\"\$/CELLCTL_VERSION=\"${RELEASE_TAG}\"/" \
  "$T/cellctl-drifted-source" > "$T/cellctl-drifted-staged"
if grep -q "^CELLCTL_VERSION=\"${RELEASE_TAG}\"\$" "$T/cellctl-drifted-staged"; then
  drift_caught=1
else
  drift_caught=0
fi
assert 'the grep guard trips (would abort the release) when the source line drifts unstamped' '[[ $drift_caught -eq 0 ]]'

echo
if [[ "$fails" -eq 0 ]]; then echo "version-stamp.test.sh: OK"; else echo "version-stamp.test.sh: $fails FAILED"; exit 1; fi
