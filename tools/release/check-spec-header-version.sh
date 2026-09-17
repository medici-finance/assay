#!/usr/bin/env bash
# check-spec-header-version.sh — refuse to cut a release whose spec/brief-v1.md
# header names a stale statusgen version (assay#1192).
#
# WHY. spec/brief-v1.md's `Describes reference implementation:` line names a
# `statusgen` version by hand. #302 (2026-09-02) did a one-time freshen
# (v0.8.0 -> v0.22.0, per brief-13's own drift-class `facts:`) but added no
# mechanical floor, so the header drifted silently again — it still read
# v0.22.0 while the released umbrella tag had moved dozens of releases ahead
# (v1.0.9 when #1192 was filed, v1.0.12 by the time this script landed). This
# is the still-missing second half of brief-13's Task 4
# (docs/streams/desk-tools/brief-13-schema-first-conformance.md, house repo
# medici-finance/assay-toolkit): "a release-time check ... that the spec
# header matches the version being cut."
#
# CONTRACT. Reads the `Describes reference implementation:` line's `statusgen`
# version token out of the spec file and compares it, byte for byte (leading
# `v` included), to the tag being cut. THREE-STATE: a header line that is
# missing or carries no parseable vX.Y.Z token is a could-not-check FAILURE
# (exit 2), never a quiet pass — matching the umbrella scheme's own
# fail-closed convention for an unreadable version (statusgen/versiongate.go,
# plugins/assay/scripts/stamp-plugin-version.sh).
#
# USAGE
#   check-spec-header-version.sh <vX.Y.Z> [spec-file]
#
# spec-file defaults to spec/brief-v1.md, resolved against the CURRENT
# working directory — the release workflow runs this from the repo root right
# after checkout, same convention as tools/changelog/check.sh (no --root flag).
#
# EXIT CODES   0 match · 1 mismatch (the release-blocking case) · 2 usage or
# could-not-check (unreadable file, unparseable line, malformed tag).
#
# Unit-tested offline by check-spec-header-version.test.sh.
set -euo pipefail

version_re='^v[0-9]+\.[0-9]+\.[0-9]+$'

usage() {
  echo "usage: $0 <vX.Y.Z> [spec-file]" >&2
  exit 2
}

[[ $# -ge 1 && $# -le 2 ]] || usage
tag="$1"
file="${2:-spec/brief-v1.md}"

if [[ ! "$tag" =~ $version_re ]]; then
  echo "FAIL usage: tag must be vMAJOR.MINOR.PATCH (got '$tag')" >&2
  exit 2
fi

if [[ ! -r "$file" ]]; then
  echo "FAIL could-not-check: cannot read $file" >&2
  exit 2
fi

line=$(grep -m1 'Describes reference implementation:' "$file" || true)
if [[ -z "$line" ]]; then
  echo "FAIL could-not-check: $file has no 'Describes reference implementation:' line" >&2
  exit 2
fi

have=$(grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' <<<"$line" | head -1 || true)
if [[ -z "$have" ]]; then
  echo "FAIL could-not-check: could not parse a vX.Y.Z token out of: $line" >&2
  exit 2
fi

if [[ "$have" != "$tag" ]]; then
  echo "FAIL $file names statusgen $have, but this release is cutting $tag — update the 'Describes reference implementation:' line in $file to $tag before releasing (brief-13 task 4, assay#1192)" >&2
  exit 1
fi

echo "ok   $file names statusgen $have, matches $tag"
