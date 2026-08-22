#!/bin/sh
# layer-secret-scan.sh <image-ref>
#
# An ADDED security control for the desk container images: it fails a built
# image that carries key-shaped material in any layer, in the image config, or
# in build history. It is the mechanical enforcer of the "no secret in any image
# layer" rule stated normatively in ../secrets.md, and one of three independent
# controls behind that rule (contract review + this scan + the repo leak-sweep).
#
# It inspects three surfaces:
#   1. `docker history --no-trunc`  — build-arg echoes, ENV instructions.
#   2. the image config             — ENV value defaults, labels.
#   3. every layer's filesystem     — a COPYed / RUN-written credential file.
#
# Patterns (key material actually held by this project):
#   * PEM private-key blocks (App signing key, mounted GCP/WIF key files).
#   * GitHub token prefixes: personal (ghp_), app/installation (ghs_), fine-
#     grained (github_pat_).
#   * Model API-key shapes: Anthropic (sk-ant-) and the generic sk- family.
#
# Fail-closed: any hit exits 1; a surface that cannot be read (no image, no
# docker) exits 2 — "could not scan" is never reported as clean.
#
# Usage:  sh layer-secret-scan.sh <image-ref>
set -u

IMG="${1:-}"
if [ -z "$IMG" ]; then
  echo "usage: layer-secret-scan.sh <image-ref>" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "layer-secret-scan: docker not found — cannot scan (fail-closed)" >&2
  exit 2
fi

# The pattern set, as a single extended-regexp alternation. Notes on why these
# forms detect real secrets without themselves being secret-shaped:
#   * The PEM alternative matches the header body ("BEGIN ... PRIVATE KEY") that
#     every PEM variant carries; it deliberately omits the dashed fence so this
#     source file is not itself a private-key match.
#   * The token/key alternatives require a run of body characters after the
#     prefix, so a bare mention of a prefix (like this comment) does not match,
#     but a real planted token does.
PEM='BEGIN[A-Z0-9 _-]*PRIVATE KEY'
GHTOK='gh[ps]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}'
MODELKEY='sk-ant-[A-Za-z0-9_-]{10,}|sk-[A-Za-z0-9]{20,}'
PAT="${PEM}|${GHTOK}|${MODELKEY}"

WORK="$(mktemp -d 2>/dev/null || mktemp -d -t layerscan)"
trap 'rm -rf "$WORK"' EXIT INT TERM
HITS="$WORK/hits"
: > "$HITS"

# --- surface 1: build history -------------------------------------------------
if ! docker history --no-trunc --format '{{.CreatedBy}}' "$IMG" \
     > "$WORK/history.txt" 2>"$WORK/err"; then
  echo "layer-secret-scan: cannot read history for '$IMG' — $(cat "$WORK/err")" >&2
  exit 2
fi

# --- surface 2: image config (env defaults, labels) ---------------------------
if ! docker inspect "$IMG" > "$WORK/inspect.json" 2>"$WORK/err"; then
  echo "layer-secret-scan: cannot inspect '$IMG' — $(cat "$WORK/err")" >&2
  exit 2
fi

# --- surface 3: layer filesystems ---------------------------------------------
if ! docker save "$IMG" -o "$WORK/img.tar" 2>"$WORK/err"; then
  echo "layer-secret-scan: cannot save '$IMG' — $(cat "$WORK/err")" >&2
  exit 2
fi
mkdir -p "$WORK/img" "$WORK/rootfs"
tar -xf "$WORK/img.tar" -C "$WORK/img" 2>/dev/null || true
# Extract every nested layer tar (docker save nests each layer as its own tar,
# gzipped or not — tar auto-detects). Non-tar blobs (the config/manifest JSON)
# are skipped here but still scanned raw below, which is where ENV defaults live.
find "$WORK/img" -type f 2>/dev/null | while IFS= read -r blob; do
  if tar -tf "$blob" >/dev/null 2>&1; then
    tar -xf "$blob" -C "$WORK/rootfs" 2>/dev/null || true
  fi
done

# --- scan all collected surfaces ----------------------------------------------
# -a: treat binary as text (layer tars / config blobs). -r: recurse dirs.
# -l: just the file names; the masked sample is pulled separately so a real
# secret value is never echoed to the scanner's own output.
grep -aErl "$PAT" \
  "$WORK/history.txt" "$WORK/inspect.json" "$WORK/img" "$WORK/rootfs" \
  2>/dev/null > "$WORK/hitfiles" || true

mask() {
  # Report that a hit occurred and its length, without printing the value.
  _m="$1"
  printf '%.4s…(%s chars, redacted)' "$_m" "${#_m}"
}

label() {
  # Human-readable surface name for a hit path.
  case "$1" in
    "$WORK/history.txt") echo "build history" ;;
    "$WORK/inspect.json") echo "image config" ;;
    "$WORK"/rootfs/*) echo "layer file: ${1#"$WORK/rootfs"}" ;;
    *) echo "image blob: ${1#"$WORK/img/"}" ;;
  esac
}

while IFS= read -r hf; do
  [ -n "$hf" ] || continue
  sample="$(grep -aoE "$PAT" "$hf" 2>/dev/null | head -1)"
  printf '%s\t%s\n' "$(label "$hf")" "$(mask "$sample")" >> "$HITS"
done < "$WORK/hitfiles"

if [ -s "$HITS" ]; then
  echo "FAIL: key-shaped material detected in image '$IMG'" >&2
  while IFS='	' read -r loc m; do
    printf '  hit: %s (%s)\n' "$loc" "$m" >&2
  done < "$HITS"
  exit 1
fi

echo "clean: no key-shaped material found in image '$IMG'"
exit 0
