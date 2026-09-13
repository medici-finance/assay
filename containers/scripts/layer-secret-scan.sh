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
# image config — a hit on either of those is never allowlisted).
allow_path() {
  case "$1" in
    # Go's own standard-library source tree, downloaded from go.dev/dl at the
    # pinned GO_VERSION. Every hit here to date is Go's own published test or
    # example fixture (crypto/tls/testdata/example-key.pem, the platform-verifier
    # test fixture crypto/x509/platform_root_key.pem, crypto/tls/example_test.go's
    # inline ExampleX509KeyPair cert+key) — verified by checking each file is
    # referenced only from a sibling *_test.go. Nothing this Dockerfile does
    # writes into /usr/local/go/src; it is the untouched upstream tree.
    "$WORK"/rootfs/usr/local/go/src/*) return 0 ;;
    # The npm CLI bundled with the pinned Node.js tarball (nodejs.org/dist),
    # not a project node_modules tree. Every hit here is npm's own published
    # documentation of its `ca`/`cert`/`key` config options, which illustrates
    # the PEM shape with the literal placeholder body `XXXX` (config.7,
    # config.md, docs/output/.../config.html, and the shared source of all
    # three, @npmcli/config/lib/definitions/definitions.js) — never a real key.
    "$WORK"/rootfs/usr/local/lib/node_modules/npm/*) return 0 ;;
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
    "$WORK"/rootfs/usr/bin/gpgv) return 0 ;;
    "$WORK"/rootfs/usr/lib/*/libssh2.so*) return 0 ;;
    "$WORK"/rootfs/usr/lib/*/libgnutls.so*) return 0 ;;
    # The `gh` CLI binary (installed by this Dockerfile's install-agent-cli
    # step), matched by the generic `sk-[A-Za-z0-9]{20,}` alternative against
    # one unrelated identifier string in its compiled string table
    # (`sk-fieldnamestringpprint` — verified: an internal Go struct-field/
    # pprint-label identifier baked in by the `gh` build, not a key; no
    # plausible secret body follows it). Exempting only this exact path
    # (never a glob) restores full `sk-` coverage for every other binary in
    # the image, including any other toolchain binary at any other path.
    "$WORK"/rootfs/usr/local/bin/gh) return 0 ;;
    *) return 1 ;;
  esac
}

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
# gzipped or not — tar auto-detects) into rootfs. A blob that IS a layer tar is
# recorded only via its extracted rootfs contents from here on — scanning the
# raw tar too would report the same file twice (#908: "each filesystem hit is
# reported twice, once as layer file: and once as image blob:"). Blobs that are
# NOT a tar (the config/manifest JSON, where ENV defaults live) are listed in
# nontar-blobs and scanned raw below, since they have no rootfs counterpart.
: > "$WORK/nontar-blobs"
find "$WORK/img" -type f 2>/dev/null | while IFS= read -r blob; do
  if tar -tf "$blob" >/dev/null 2>&1; then
    tar -xf "$blob" -C "$WORK/rootfs" 2>/dev/null || true
  else
    printf '%s\n' "$blob" >> "$WORK/nontar-blobs"
  fi
done

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
# history.txt/inspect.json/rootfs are searched recursively (-r); nontar-blobs
# has no directory to recurse (see the extraction step above), so each listed
# blob is checked individually.
: > "$WORK/hitfiles"
collect() { # <grep binary-handling flag: a|I> <pattern>
  bf="$1" pat="$2"
  grep "-${bf}Erl" "$pat" \
    "$WORK/history.txt" "$WORK/inspect.json" "$WORK/rootfs" \
    2>/dev/null >> "$WORK/hitfiles" || true
  if [ -s "$WORK/nontar-blobs" ]; then
    while IFS= read -r blob; do
      grep "-${bf}Eq" "$pat" "$blob" 2>/dev/null && printf '%s\n' "$blob" >> "$WORK/hitfiles"
    done < "$WORK/nontar-blobs"
  fi
}
collect a "$STRICT"
collect a "$LOOSE"
sort -u "$WORK/hitfiles" -o "$WORK/hitfiles" 2>/dev/null || true

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
  allow_path "$hf" && continue
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
