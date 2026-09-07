#!/usr/bin/env bash
# check-paired-versions.sh — the guard that keeps the adopter front door honest.
#
# `assay:install` resolves the statusgen (and desk-tools) binaries an adopter gets FROM
# `plugins/assay/paired-versions.yaml`, keyed on the plugin version in
# `plugins/assay/.claude-plugin/plugin.json`. Those two files are edited by different hands at
# different times, and until this guard existed NOTHING compared them: the manifest sat on a
# plugin version and a release tag many releases behind the plugin.json the marketplace
# actually ships, so a cold install silently downloaded a stale tool. The drift was invisible
# because both files were individually well-formed.
#
# Three assertions, all offline and all on the tree as committed:
#
#   A  PAIRING       `plugin:` in paired-versions.yaml == `version` in plugin.json.
#                    This is the whole point of the manifest: the LEFT side of the pairing must
#                    name the plugin version being shipped, or the resolution keys on nothing.
#   B  SINGLE TAG    every pinned tag is the SAME tag — the `statusgen.tag`, the
#                    `desk-tools.tag`, and the tag field of every per-platform pin line. The
#                    binaries are cut from one release together; a per-platform line left on an
#                    older tag is the shape that ships a mismatched pair to one platform only.
#   C  HASH SHAPE    every sha256 is exactly 64 lowercase hex characters. Cannot prove a hash is
#                    the RIGHT one without the network — that is the release job's and the
#                    re-pin author's evidence — but it does catch a truncated, upper-cased or
#                    placeholder digest before an adopter's verification fails on it.
#
# Deliberately NOT checked: whether the pinned tag is the LATEST release. Pinning is the design
# (never a floating tag), so "behind latest" is a re-pin decision, not a defect this script can
# rule on. What it enforces is internal consistency, which is what silently rotted.
#
# THREE-STATE: could-not-check is a FAILURE, never a quiet pass. A missing or unreadable file,
# an absent key, or a section with no platform pins all exit non-zero — a guard that cannot see
# its subject must not report green.
#
# No network, no token, no jq, no Go. Run:
#   bash plugins/assay/scripts/check-paired-versions.sh
#   bash plugins/assay/scripts/check-paired-versions.sh --root <dir>   # check another tree
#
# CI: picked up automatically by ci.yml's `plugin-shell-suites` job via this script's
# companion suite `check-paired-versions.test.sh` (that job globs
# `plugins/assay/scripts/*.test.sh`), so no workflow edit was needed to arm it.
set -uo pipefail

ROOT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --root)
      ROOT="${2:-}"
      if [[ -z "$ROOT" ]]; then printf 'FAIL --root needs a directory\n' >&2; exit 2; fi
      shift 2
      ;;
    -h|--help)
      sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      printf 'FAIL unknown argument: %s\n' "$1" >&2
      exit 2
      ;;
  esac
done

if [[ -z "$ROOT" ]]; then
  # Default: the repo root two levels above this script (plugins/assay/scripts/ -> repo root).
  ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
fi

PAIRED="$ROOT/plugins/assay/paired-versions.yaml"
MANIFEST="$ROOT/plugins/assay/.claude-plugin/plugin.json"

fail=0
bad() { printf 'FAIL %s\n' "$1" >&2; fail=1; }
ok()  { printf 'ok   %s\n' "$1"; }

for f in "$PAIRED" "$MANIFEST"; do
  if [[ ! -r "$f" ]]; then
    bad "could not read ${f#"$ROOT"/} — could-not-check is a failure, not a pass"
  fi
done
if [[ "$fail" -ne 0 ]]; then exit 1; fi

# ---------------------------------------------------------------- A  PAIRING
# `plugin: "0.5.0"` — first top-level occurrence, quotes optional.
paired_plugin=$(sed -n 's/^plugin:[[:space:]]*"\{0,1\}\([^"[:space:]]*\)"\{0,1\}[[:space:]]*$/\1/p' "$PAIRED" | head -1)
# `"version": "0.5.0"` in the plugin manifest.
manifest_version=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$MANIFEST" | head -1)

if [[ -z "$paired_plugin" ]]; then
  bad "paired-versions.yaml has no top-level \`plugin:\` key"
elif [[ -z "$manifest_version" ]]; then
  bad "plugin.json has no \`version\` field"
elif [[ "$paired_plugin" != "$manifest_version" ]]; then
  bad "pairing drift: paired-versions.yaml plugin=${paired_plugin} but plugin.json version=${manifest_version}"
else
  ok "pairing: plugin ${paired_plugin} == plugin.json version ${manifest_version}"
fi

# ------------------------------------------------------------- B  SINGLE TAG
# Section tags: `  tag: v0.25.1` (any indent), plus the tag field of every platform pin line
# `    <platform>: <artifact> <tag> <sha256>`. Comment lines are excluded.
uncommented=$(sed 's/[[:space:]]*#.*$//' "$PAIRED")

section_tags=$(printf '%s\n' "$uncommented" | sed -n 's/^[[:space:]]*tag:[[:space:]]*\([^[:space:]]*\)[[:space:]]*$/\1/p')
pin_lines=$(printf '%s\n' "$uncommented" | grep -E '^[[:space:]]+[a-z0-9-]+:[[:space:]]+[A-Za-z0-9._-]+[[:space:]]+v[0-9]' || true)
pin_tags=$(printf '%s\n' "$pin_lines" | awk 'NF{print $3}')

if [[ -z "$section_tags" ]]; then
  bad "paired-versions.yaml declares no \`tag:\` — could-not-check"
fi
if [[ -z "$pin_lines" ]]; then
  bad "paired-versions.yaml declares no per-platform pin lines — could-not-check"
fi

if [[ "$fail" -eq 0 ]]; then
  distinct=$(printf '%s\n%s\n' "$section_tags" "$pin_tags" | grep -v '^$' | sort -u)
  count=$(printf '%s\n' "$distinct" | grep -c . )
  if [[ "$count" -ne 1 ]]; then
    bad "pins span $count tags, must be exactly one: $(printf '%s' "$distinct" | tr '\n' ' ')"
  else
    ok "single tag: every section and pin line names $distinct ($(printf '%s\n' "$pin_lines" | grep -c .) pin lines)"
  fi
fi

# ------------------------------------------------------------- C  HASH SHAPE
hashes=$(printf '%s\n' "$pin_lines" | awk 'NF{print $4}')
if [[ -z "$hashes" ]]; then
  bad "no sha256 field on any pin line — could-not-check"
else
  n=0
  while IFS= read -r h; do
    [[ -z "$h" ]] && continue
    n=$((n + 1))
    if [[ ! "$h" =~ ^[0-9a-f]{64}$ ]]; then
      bad "sha256 is not 64 lowercase hex: ${h}"
    fi
  done <<< "$hashes"
  # Every pin line must carry one, and nothing beyond the four fields.
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    fields=$(printf '%s\n' "$line" | awk '{print NF}')
    if [[ "$fields" -ne 4 ]]; then
      bad "pin line has $fields fields, expected 4 (<platform>: <artifact> <tag> <sha256>): ${line# }"
    fi
  done <<< "$pin_lines"
  [[ "$fail" -eq 0 ]] && ok "hash shape: $n sha256 values are 64 lowercase hex"
fi

if [[ "$fail" -ne 0 ]]; then
  printf 'check-paired-versions: FAILED\n' >&2
  exit 1
fi
printf 'check-paired-versions: OK\n'
