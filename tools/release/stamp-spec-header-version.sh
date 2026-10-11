#!/usr/bin/env bash
# stamp-spec-header-version.sh — write the release tag into spec/brief-v1.md's
# `Describes reference implementation:` header (assay#1192).
#
# WHY. The header names a `statusgen` version. Edited by hand it drifted twice
# (v0.8.0 while v0.19.0 was current; then v0.22.0 and v1.0.12 while releases
# moved on). The ruling on #1192 is option a, `stamp`: the release writes the
# tag being cut into the header, the same way it already stamps the plugin
# manifest (plugins/assay/scripts/stamp-plugin-version.sh), and
# check-spec-header-version.sh stays as the proof that the stamp ran. This
# script is the stamp half; the check script is the proof half.
#
# CONTRACT. Rewrites the first vX.Y.Z token on the first
# `Describes reference implementation:` line of the spec file to the given
# tag — the same line and the same token the check script reads, so a stamp
# followed by a check always agrees. Every other byte of the file is left
# alone. IDEMPOTENT: a header that already names the tag is not rewritten.
# FAIL-CLOSED: a missing or unreadable file, a missing header line, or a
# header line with no vX.Y.Z token to rewrite is a could-not-check failure
# (exit 2) and nothing is written — a stamp that silently skipped the header
# is exactly the drift #1192 reported. After writing, the result is read back
# through check-spec-header-version.sh; the stamp is only done when the check
# passes.
#
# USAGE
#   stamp-spec-header-version.sh <vX.Y.Z> [spec-file]
#
# spec-file defaults to spec/brief-v1.md, resolved against the CURRENT working
# directory — the release workflow runs this from the repo root, the same
# convention as check-spec-header-version.sh.
#
# EXIT CODES   0 stamped (or already stamped) · 2 usage or could-not-check
# (malformed tag, unreadable file, missing line, no token, read-back failed).
#
# No network, no git, no toolchain beyond bash + grep + awk. Unit-tested
# offline by stamp-spec-header-version.test.sh.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK="${CHECK_IMPL:-$here/check-spec-header-version.sh}"

version_re='^v[0-9]+\.[0-9]+\.[0-9]+$'
marker='Describes reference implementation:'

usage() {
  echo "usage: $0 <vX.Y.Z> [spec-file]" >&2
  exit 2
}
cnc() { echo "FAIL could-not-check: $1" >&2; exit 2; }

[[ $# -ge 1 && $# -le 2 ]] || usage
tag="$1"
file="${2:-spec/brief-v1.md}"

if [[ ! "$tag" =~ $version_re ]]; then
  echo "FAIL usage: tag must be vMAJOR.MINOR.PATCH (got '$tag')" >&2
  exit 2
fi

[[ -r "$file" && -f "$file" ]] || cnc "cannot read $file"

line=$(grep -m1 -F "$marker" "$file" || true)
[[ -n "$line" ]] || cnc "$file has no '$marker' line — nothing to stamp"

have=$(grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' <<<"$line" | head -1 || true)
[[ -n "$have" ]] || cnc "no vX.Y.Z token to rewrite in: $line"

if [[ "$have" == "$tag" ]]; then
  echo "ok   $file already names statusgen $tag — nothing to stamp"
  exit 0
fi

# Portable in-place edit: write a sibling temp file, then mv over the original
# (no `sed -i` dialect dependence between GNU and BSD sed). awk touches only the
# first marker line, and only its first vX.Y.Z token.
tmp="${file}.stamp.$$"
if ! awk -v marker="$marker" -v tag="$tag" '
    !seen && index($0, marker) {
      seen = 1
      if (match($0, /v[0-9]+\.[0-9]+\.[0-9]+/)) {
        $0 = substr($0, 1, RSTART - 1) tag substr($0, RSTART + RLENGTH)
        done = 1
      }
    }
    { print }
    END { if (!done) exit 3 }
  ' "$file" > "$tmp"; then
  rm -f "$tmp"
  cnc "rewriting the header line in $file"
fi
mv "$tmp" "$file"

# Read back — the stamp is only done when the proof would pass.
if ! bash "$CHECK" "$tag" "$file"; then
  echo "FAIL stamp-spec-header-version: wrote $file but the read-back check did not pass for $tag" >&2
  exit 2
fi
echo "stamped $file: statusgen $have -> $tag"
