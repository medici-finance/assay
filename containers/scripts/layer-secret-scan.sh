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
#      Each layer is extracted into its OWN directory and scanned there, never
#      merged into one shared tree (#2256): in a merged tree a later layer that
#      overwrites a path or replaces it with a directory hides the earlier
#      layer's file from the scan, although that earlier layer still ships in
#      the image with the key in it.
#
# Patterns (key material actually held by this project):
#   * PEM private-key blocks (App signing key, mounted GCP/WIF key files).
#   * GitHub token prefixes: personal (ghp_), app/installation (ghs_), fine-
#     grained (github_pat_).
#   * Model API-key shapes: Anthropic (sk-ant-) and the generic sk- family.
#
# Precision (#908, corrected — see review on PR #1011): the pattern set above
# matches key-SHAPED text wherever it appears, including toolchain material
# this project never wrote — Go's own stdlib test fixtures, npm's own bundled
# docs, and PEM format strings compiled into distro crypto binaries (gpgv,
# libssh2, libgnutls), plus one unrelated identifier string inside the `gh`
# CLI binary. The single mechanism that keeps those out without weakening
# detection of a REAL secret is PATH_ALLOWLIST (below): specific, verified-safe
# path prefixes/paths that are wholly populated by a pinned upstream download
# this Dockerfile performs (the Go SDK tree, the bundled npm CLI), by apt-get
# installing a well-known distro package (gpgv, libssh2, libgnutls), or — for
# the `gh` CLI false positive specifically — the exact installed path of that
# one binary. It applies only to the layer filesystem surface: a hit in build
# history or the image config is never allowlisted, since those are exactly
# where an accidental credential default would show up.
#
# An earlier version of this fix instead gated the whole generic `sk-`
# pattern class to text-shaped content (grep -I, which skips every binary
# file image-wide) to dodge the one `gh`-binary hit. Review on PR #1011 caught
# that this was a real regression, not a narrow exemption: it made a genuine
# `sk-`-shaped secret compiled into ANY OTHER binary in the image permanently
# invisible, not just the one proven false positive. That blanket gating has
# been reverted — the generic `sk-` pattern is checked against every surface,
# including binaries, exactly like the anchored patterns, with only the `gh`
# binary's specific path exempted via PATH_ALLOWLIST (see the entry below and
# the /opt/app/vendored-tool fixture in layer-secret-scan.test.sh's FALSEPOS
# image, which proves a real `sk-`-shaped secret in a binary at an ordinary,
# non-`gh` path is still caught).
# This is a precision fix, not recall weakening: nothing here narrows what a
# hit LOOKS like, only which already-verified-safe *paths* are exempt.
#
# Fail-closed: any hit exits 1; a surface that cannot be read (no image, no
# docker) exits 2 — "could not scan" is never reported as clean. That covers
# every step, not only the docker calls: a save that does not unpack, a layer
# list in manifest.json that cannot be read or lists no layer, a listed layer
# that is missing, does not match the digest it is named by, or does not
# extract completely, a compressed blob tar cannot open, a file or directory
# the scan cannot read, a path whose name the hit report cannot carry, a grep
# that errors, and a signal mid-scan all exit 2.
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

# The pattern set, as extended-regexp alternations. Notes on why these forms
# detect real secrets without themselves being secret-shaped:
#   * The PEM alternative matches the header body ("BEGIN ... PRIVATE KEY") that
#     every PEM variant carries; it deliberately omits the dashed fence so this
#     source file is not itself a private-key match (the fence + a real body is
#     exactly what the repo's own pattern-based leak-sweep looks for).
#   * The token/key alternatives require a run of body characters after the
#     prefix, so a bare mention of a prefix (like this comment) does not match,
#     but a real planted token does.
#
# Split into two classes (#908 defect 3): STRICT is anchored enough (a GitHub
# token/App-key prefix, or the Anthropic sk-ant- prefix) that it has never
# produced a binary false positive. The bare `sk-` shape (LOOSE) is looser —
# it matched one unrelated base64-ish identifier run inside the `gh` CLI
# binary's string table (proven false positive, see PATH_ALLOWLIST below).
# BOTH classes are checked against every surface, including compiled binaries
# (grep -a) — an earlier revision of this fix instead skipped binaries
# entirely for the whole LOOSE class (grep -I), which review on PR #1011
# correctly flagged as a real coverage regression (a genuine `sk-`-shaped
# secret compiled into any OTHER binary would have gone undetected, forever,
# image-wide). The fix for the one proven `gh` false positive is the same
# path-scoped allowlist used for the PEM-in-binary cases, not a pattern-wide
# exemption — see the `gh` entry in PATH_ALLOWLIST.
PEM='BEGIN[A-Z0-9 _-]*PRIVATE KEY'
GHTOK='gh[ps]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}'
MODELKEY_ANT='sk-ant-[A-Za-z0-9_-]{10,}'
MODELKEY_GENERIC='sk-[A-Za-z0-9]{20,}'
STRICT="${PEM}|${GHTOK}|${MODELKEY_ANT}"
LOOSE="${MODELKEY_GENERIC}"
PAT="${STRICT}|${LOOSE}"

# --- PATH_ALLOWLIST (#908) -----------------------------------------------------
# Narrow, explicit, commented exemptions for layer-filesystem paths that are
# wholly populated by a pinned upstream download or apt-get package this base
# Dockerfile performs — never a path this project's own build steps write a
# credential into. Each entry is verified against the real desk base image
# (see the PR for the reproduction); nothing here is a guess.
#
# Applies ONLY to the layer-filesystem surface (never build history or the
# image config — a hit on either of those is never allowlisted). Each entry is
# matched against the file's path INSIDE the image (see image_path below), the
# same in every layer: the per-layer extraction (#2256) changes where a file
# lands on the scanner's disk, never which image paths are exempt.
#
# image_path <scanner-path>: for a file under $WORK/layers/<n>/, print its path
# inside the image (/usr/bin/gpgv); any other scanner path (build history, the
# image config, a raw non-tar blob) is not a layer file, so return 1.
image_path() {
  case "$1" in
    "$WORK"/layers/*/*) ;;
    *) return 1 ;;
  esac
  _r="${1#"$WORK"/layers/}"
  printf '/%s\n' "${_r#*/}"
}
allow_path() {
  _p="$(image_path "$1")" || return 1
  case "$_p" in
    # Go's own standard-library source tree, downloaded from go.dev/dl at the
    # pinned GO_VERSION. Every hit here to date is Go's own published test or
    # example fixture (crypto/tls/testdata/example-key.pem, the platform-verifier
    # test fixture crypto/x509/platform_root_key.pem, crypto/tls/example_test.go's
    # inline ExampleX509KeyPair cert+key) — verified by checking each file is
    # referenced only from a sibling *_test.go. Nothing this Dockerfile does
    # writes into /usr/local/go/src; it is the untouched upstream tree.
    /usr/local/go/src/*) return 0 ;;
    # The npm CLI bundled with the pinned Node.js tarball (nodejs.org/dist),
    # not a project node_modules tree. Every hit here is npm's own published
    # documentation of its `ca`/`cert`/`key` config options, which illustrates
    # the PEM shape with the literal placeholder body `XXXX` (config.7,
    # config.md, docs/output/.../config.html, and the shared source of all
    # three, @npmcli/config/lib/definitions/definitions.js) — never a real key.
    /usr/local/lib/node_modules/npm/*) return 0 ;;
    # Distro-packaged (apt-get) crypto tooling whose compiled artifact embeds
    # the literal PEM armor text as a format string, not a key:
    #   * gpgv — GnuPG's own PGP-armor header/footer strings, used to recognize
    #     ASCII-armored blocks it parses; verified no body follows in the
    #     binary, only the bare header/footer pair back-to-back.
    #   * libssh2 — the OpenSSH private-key PEM fence pair used to recognize
    #     that key format when parsing a user-supplied key; verified no body
    #     between the BEGIN/END strings in the binary's rodata.
    #   * libgnutls — ships several fixed EC private keys with real base64
    #     bodies, compiled in as GnuTLS's own FIPS-140 power-on self-test
    #     vectors (known-answer tests run at library init), not project key
    #     material — a well-documented upstream behavior of this exact package.
    /usr/bin/gpgv) return 0 ;;
    /usr/lib/*/libssh2.so*) return 0 ;;
    /usr/lib/*/libgnutls.so*) return 0 ;;
    # The `gh` CLI binary (installed by this Dockerfile's install-agent-cli
    # step), matched by the generic `sk-[A-Za-z0-9]{20,}` alternative against
    # one unrelated identifier string in its compiled string table
    # (`sk-fieldnamestringpprint` — verified: an internal Go struct-field/
    # pprint-label identifier baked in by the `gh` build, not a key; no
    # plausible secret body follows it). Exempting only this exact path
    # (never a glob) restores full `sk-` coverage for every other binary in
    # the image, including any other toolchain binary at any other path.
    /usr/local/bin/gh) return 0 ;;
    *) return 1 ;;
  esac
}

# cannot_scan <reason>: the one fail-closed exit for a surface the scan could
# not read in full.
cannot_scan() {
  echo "layer-secret-scan: $1 — cannot scan '$IMG' (fail-closed)" >&2
  exit 2
}

# compressed <file>: true when the file's magic bytes are gzip, bzip2, xz or
# zstd. Such a blob is a compressed layer; if tar cannot list it, its content
# is unreadable here, and grepping the compressed bytes proves nothing.
compressed() {
  _mg="$(od -An -tx1 -N6 "$1" 2>/dev/null | tr -d ' \n')"
  case "$_mg" in
    1f8b*|425a68*|fd377a585a00|28b52ffd*) return 0 ;;
  esac
  return 1
}

WORK="$(mktemp -d 2>/dev/null || mktemp -d -t layerscan)"
# EXIT cleans up; a signal ENDS the scan with exit 2. A trap that only removed
# $WORK on INT/TERM would let the script carry on over an empty tree and
# report it clean.
trap 'chmod -R u+rwX "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT
trap 'cannot_scan "interrupted by a signal"' HUP INT TERM

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
mkdir -p "$WORK/img" "$WORK/layers"
if ! tar -xf "$WORK/img.tar" -C "$WORK/img" 2>"$WORK/err"; then
  cannot_scan "the saved image does not unpack: $(head -3 "$WORK/err")"
fi
# Extract every layer tar (gzipped or not — tar auto-detects) into a directory
# of its OWN, $WORK/layers/<n>/. A blob that IS a layer tar is recorded only
# via its extracted contents from here on — scanning the raw tar too would
# report the same file twice (#908: "each filesystem hit is reported twice,
# once as layer file: and once as image blob:"). Blobs that are NOT a tar (the
# config/manifest JSON, where ENV defaults live) are listed in nontar-blobs and
# scanned raw below, since they have no extracted counterpart.
#
# One directory per layer, never one merged tree (#2256). Merging the layers
# into a single tree lets whichever layer is extracted last win every path they
# share: a key an earlier layer wrote at /app/key.pem is overwritten by a later
# layer's file at the same path, or removed when a later layer puts a directory
# there, and is then never scanned — although the earlier layer still ships in
# the image with the key in it. Each layer extracted alone has nothing to
# collide with, so every file every layer carries is scanned. layer-index maps
# each <n> back to its blob, for the hit report.
#
# Fail-closed extraction. The layers are the ones the save's manifest.json
# lists, not whatever happens to be in the save: tar exits 0 on a save cut at
# a member boundary, so only the list shows a layer is missing. Every listed
# layer must be in the save, must match the sha256 it is named by (when it is
# named by one: a layer cut at a member boundary also extracts with exit 0),
# and must extract with exit 0, since a member tar refused is a file never
# scanned. A listed layer is never grepped raw. A blob the manifest does not
# list is still scanned: as a layer if tar reads it, or raw if it is not a tar.
# One that is compressed but tar cannot read exits 2: its raw bytes prove
# nothing.
# Each loop reads its list from a file, not a pipe, so its exit ends the script.
: > "$WORK/nontar-blobs"
: > "$WORK/layer-index"
n=0
# layer_list: print each layer path manifest.json lists, or fail.
layer_list() {
  [ -f "$WORK/img/manifest.json" ] || return 1
  awk '
    { s = s $0 }
    END {
      gsub(/[ \t\r]/, "", s)
      if (index(s, "\"Layers\":[") == 0) exit 3
      while ((i = index(s, "\"Layers\":[")) > 0) {
        s = substr(s, i + 10)
        j = index(s, "]")
        if (j == 0) exit 3
        a = substr(s, 1, j - 1)
        s = substr(s, j + 1)
        if (a == "") continue
        m = split(a, f, ",")
        for (k = 1; k <= m; k++) {
          if (f[k] !~ /^"[^"\\]+"$/) exit 3
          print substr(f[k], 2, length(f[k]) - 2)
        }
      }
    }
  ' "$WORK/img/manifest.json"
}
# listed <path in the save>: true when manifest.json lists it as a layer.
listed() {
  while IFS= read -r _p; do
    [ "$_p" = "$1" ] && return 0
  done < "$WORK/listed"
  return 1
}
# digest_ok <file> <path in the save>: a blob named blobs/sha256/<hex> must
# hash to <hex>. A path not named by a digest (the older save format) has
# nothing to check against; such a layer is still held to the other checks.
digest_ok() {
  case "$2" in
    blobs/sha256/*) ;;
    *) return 0 ;;
  esac
  if command -v sha256sum >/dev/null 2>&1; then
    _sum="$(sha256sum < "$1")" || return 1
  else
    _sum="$(shasum -a 256 < "$1")" || return 1
  fi
  [ "${_sum%% *}" = "${2#blobs/sha256/}" ]
}
# extract_layer <file> <path in the save>: extract one layer into its own dir.
extract_layer() {
  n=$((n + 1))
  mkdir -p "$WORK/layers/$n"
  if ! tar -xf "$1" -C "$WORK/layers/$n" 2>"$WORK/err"; then
    cannot_scan "layer $2 does not extract in full: $(head -3 "$WORK/err")"
  fi
  printf '%s\t%s\n' "$n" "$2" >> "$WORK/layer-index"
}
if ! layer_list > "$WORK/listed" 2>"$WORK/err"; then
  cannot_scan "cannot read the layer list in manifest.json: $(head -3 "$WORK/err")"
fi
while IFS= read -r p; do
  if [ ! -f "$WORK/img/$p" ]; then
    cannot_scan "layer $p, listed in manifest.json, is not in the saved image"
  fi
  if ! digest_ok "$WORK/img/$p" "$p"; then
    cannot_scan "layer $p does not match the digest it is named by"
  fi
  extract_layer "$WORK/img/$p" "$p"
done < "$WORK/listed"
if [ "$n" -eq 0 ]; then
  cannot_scan "manifest.json lists no layer"
fi
if ! find "$WORK/img" -type f > "$WORK/blobs" 2>"$WORK/err"; then
  cannot_scan "cannot list the saved image: $(head -3 "$WORK/err")"
fi
while IFS= read -r blob; do
  rel="${blob#"$WORK/img/"}"
  listed "$rel" && continue
  if tar -tf "$blob" >/dev/null 2>&1; then
    extract_layer "$blob" "$rel"
  elif compressed "$blob"; then
    cannot_scan "$rel is compressed but tar cannot read it"
  else
    printf '%s\n' "$blob" >> "$WORK/nontar-blobs"
  fi
done < "$WORK/blobs"

# Every extracted file must be readable, and every path must fit on one line
# of the hit list. chmod -R does not follow symlinks, so it only touches what
# the extraction wrote. A file or directory still unreadable after it, or a
# path with a newline in its name (the hit list is newline-separated, so such
# a name would split into fragments the report cannot attribute), exits 2.
if ! chmod -R u+rX "$WORK/layers" 2>"$WORK/err"; then
  cannot_scan "cannot make the extracted layers readable: $(head -3 "$WORK/err")"
fi
if ! find "$WORK/layers" ! -type l \( ! -perm -400 -o \( -type d ! -perm -100 \) \) \
     > "$WORK/unreadable" 2>"$WORK/err"; then
  cannot_scan "cannot walk the extracted layers: $(head -3 "$WORK/err")"
fi
if [ -s "$WORK/unreadable" ]; then
  cannot_scan "an extracted file or directory is not readable"
fi
NL='
'
if ! find "$WORK/img" "$WORK/layers" -name "*${NL}*" > "$WORK/nlpaths" 2>"$WORK/err"; then
  cannot_scan "cannot walk the extracted image: $(head -3 "$WORK/err")"
fi
if [ -s "$WORK/nlpaths" ]; then
  cannot_scan "an image path has a newline in its name"
fi

# --- scan all collected surfaces ----------------------------------------------
# -l: just the file names; the masked sample is pulled separately so a real
# secret value is never echoed to the scanner's own output.
#
# Both pattern classes (see STRICT/LOOSE above) run with -a (treat binary as
# text) — full coverage on every surface, including compiled binaries, for
# both the anchored patterns and the generic `sk-` shape. Precision for the
# one proven `gh`-binary false positive comes from PATH_ALLOWLIST, not from
# skipping binary scanning for a whole pattern class (see the corrected
# comments above and the PR #1011 review that caught the prior blanket -I
# gating as a coverage regression).
# history.txt/inspect.json/layers are searched recursively (-r); nontar-blobs
# has no directory to recurse (see the extraction step above), so each listed
# blob is checked individually.
#
# grep exits 0 on a match, 1 on none, and 2 on an error (a file it could not
# open or read). Exit 2 is could-not-scan, never "no match". -D skip: a FIFO or
# device carries no content, and reading one can block grep forever.
: > "$WORK/hitfiles"
collect() { # <grep binary-handling flag: a|I> <pattern>
  bf="$1" pat="$2" rc=0
  grep "-${bf}Erl" -D skip "$pat" \
    "$WORK/history.txt" "$WORK/inspect.json" "$WORK/layers" \
    2>"$WORK/grep-err" >> "$WORK/hitfiles" || rc=$?
  if [ "$rc" -gt 1 ]; then
    cannot_scan "grep could not read every file: $(head -3 "$WORK/grep-err")"
  fi
  if [ -s "$WORK/nontar-blobs" ]; then
    while IFS= read -r blob; do
      rc=0
      grep "-${bf}Eq" "$pat" "$blob" 2>"$WORK/grep-err" || rc=$?
      case "$rc" in
        0) printf '%s\n' "$blob" >> "$WORK/hitfiles" ;;
        1) ;;
        *) cannot_scan "grep could not read blob ${blob#"$WORK/img/"}" ;;
      esac
    done < "$WORK/nontar-blobs"
  fi
}
collect a "$STRICT"
collect a "$LOOSE"

mask() {
  # Report that a hit occurred and its length, without printing the value.
  _m="$1"
  printf '%.4s…(%s chars, redacted)' "$_m" "${#_m}"
}

label() {
  # Human-readable surface name for a hit path. A layer file names its path in
  # the image AND the layer blob that carries it, since one path may now be
  # reported once per layer that wrote key material there (#2256). The blob's
  # sha256 is cut to 12 hex digits — enough to find it in `docker save` output.
  case "$1" in
    "$WORK/history.txt") echo "build history" ;;
    "$WORK/inspect.json") echo "image config" ;;
    "$WORK"/layers/*/*)
      _r="${1#"$WORK"/layers/}"
      _blob="$(awk -F '\t' -v n="${_r%%/*}" '$1 == n { print $2; exit }' "$WORK/layer-index" \
        | sed 's/\([0-9a-f]\{12\}\)[0-9a-f]\{52\}/\1/')"
      echo "layer file: $(image_path "$1") (layer blob: ${_blob:-unknown})" ;;
    *) echo "image blob: ${1#"$WORK/img/"}" ;;
  esac
}

# The verdict is counted in the shell, not read back from a file: a write that
# failed (a full disk) must never turn a hit into "clean". A file both pattern
# classes matched is listed twice in hitfiles and reported once.
nhits=0
seen="$NL"
while IFS= read -r hf; do
  [ -n "$hf" ] || continue
  case "$seen" in *"$NL$hf$NL"*) continue ;; esac
  seen="$seen$hf$NL"
  allow_path "$hf" && continue
  sample="$(grep -aoE "$PAT" "$hf" 2>/dev/null | head -1)"
  [ "$nhits" -eq 0 ] && echo "FAIL: key-shaped material detected in image '$IMG'" >&2
  nhits=$((nhits + 1))
  printf '  hit: %s (%s)\n' "$(label "$hf")" "$(mask "$sample")" >&2
done < "$WORK/hitfiles"

if [ "$nhits" -gt 0 ]; then
  exit 1
fi

echo "clean: no key-shaped material found in image '$IMG'"
exit 0
