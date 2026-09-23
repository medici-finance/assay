#!/usr/bin/env bash
# assay-install.sh — the CLI-free, forge-neutral half of the `assay:install` flow.
#
# Every step here runs identically on a box with NO forge CLI on PATH (no `gh`, no `glab`):
# the release assets are fetched over plain HTTPS (curl, else wget) and verified against the
# sha256 in the target repo's `.assay-versions` pin file. A forge CLI was never load-bearing
# for acquisition; it was only the tool that happened to be there.
#
# THE CONTROL. The sha256 comparison against the PIN FILE is the one thing standing between an
# adopter and an unverified binary, so it is written to fail closed in every direction:
#
#   * the expected digest comes ONLY from the pin file (`.assay-versions`, whose lines are
#     written from the plugin's reviewed `paired-versions.yaml`) — never from anything fetched
#     from the release home, so a substituted asset cannot vouch for itself;
#   * a digest MISMATCH refuses (exit 5) and installs nothing;
#   * a digest that CANNOT BE READ — no pin line for the platform, a placeholder, a malformed
#     or truncated value, two competing lines — refuses (exit 5) and installs nothing. There is
#     no "verification unavailable, continuing" path;
#   * a fetch that fails, or a host with no sha256 tool, is COULD-NOT-CHECK (exit 6) and
#     installs nothing;
#   * the bytes are fetched into a private temp dir and reach --dest only AFTER the digest
#     matched.
#
# The second, independent layer is the post-install proof the flow already carries:
# `statusgen --version` must print the pinned tag (see `rehearse`). A binary that is wrong in a
# way the digest step missed still fails to name itself as the pinned tag.
#
# SUBCOMMANDS
#   classify --root <dir>
#       fresh | partial | adopted. `adopted` (a streams tree AND a real statusgen pin) is the
#       refuse-not-clobber outcome: it prints what it saw and exits 5 so nothing proceeds.
#   pin --manifest <paired-versions.yaml> --pins <.assay-versions> [--kind statusgen|desk-tools]
#       [--platform <os-arch>]
#       Write the platform's channel-E line from the manifest into the pin file. An identical
#       line is left untouched; a scaffold placeholder line is replaced; a DIFFERENT real line
#       refuses (a re-pin is a reviewed change, never an in-place edit).
#   acquire --pins <.assay-versions> --dest <bindir> [--kind statusgen|desk-tools]
#       [--platform <os-arch>] [--release-home <owner/repo>] [--base-url <url>]
#       Fetch the pinned asset, verify its sha256 against the pin file, install on match.
#   rehearse --target <repo> [--manifest <paired-versions.yaml>] [--forge github|gitlab]
#       [--base-url <url>] [--workdir <dir>]
#       The install flow end to end against a SCRATCH COPY of the target: classify → pin →
#       acquire + verify → `statusgen init` → prove (`--version` == pinned tag, `--lint` == 0).
#       Nothing is written to the real target, nothing is pushed, no PR is opened.
#
# --base-url defaults to https://github.com; the asset URL is
# <base-url>/<release-home>/releases/download/<tag>/<asset>. Only https:// and file:// are
# accepted (file:// is for offline rehearsal fixtures — the digest check is identical).
#
# EXIT CODES (the desk tools' contract): 0 ok · 2 usage · 5 refused · 6 could-not-check.
# Every non-zero exit leaves --dest untouched.
set -uo pipefail

DEFAULT_BASE_URL="https://github.com"
DEFAULT_RELEASE_HOME="medici-finance/assay"

say()          { printf 'assay-install: %s\n' "$*"; }
usage_err()    { printf 'assay-install: usage: %s\n' "$*" >&2; exit 2; }
refuse()       { printf 'assay-install: REFUSED — %s\n' "$*" >&2; exit 5; }
unverifiable() { printf 'assay-install: COULD-NOT-CHECK — %s\n' "$*" >&2; exit 6; }

detect_platform() {
  local os arch
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')
  printf '%s-%s' "$os" "$arch"
}

# asset_name <kind> <platform> — the release asset (and pin-line key) for a kind + platform.
asset_name() {
  case "$1" in
    statusgen)
      case "$2" in
        windows-*) printf 'statusgen-%s.exe' "$2" ;;
        *)         printf 'statusgen-%s' "$2" ;;
      esac ;;
    desk-tools) printf 'desk-tools-%s.tar.gz' "$2" ;;
    *) usage_err "--kind must be statusgen or desk-tools (got '$1')" ;;
  esac
}

is_sha256() { printf '%s' "$1" | grep -Eq '^[0-9a-f]{64}$'; }
is_placeholder() { case "$1" in REPLACE_WITH*|"") return 0 ;; esac; return 1; }

# pin_fields <pins-file> <asset> — prints "<tag> <sha256>" for the ONE pin line keyed on
# <asset>, or refuses. Every way the expected digest can fail to be read is a refusal.
pin_fields() {
  local pins="$1" asset="$2" lines n tag sha nf
  [ -r "$pins" ] || refuse "the pin file '$pins' is absent or unreadable — the expected digest for $asset could not be read, so nothing can be verified and nothing is installed"
  lines=$(awk -v a="$asset" '$0 !~ /^[[:space:]]*#/ && $1 == a' "$pins")
  n=$(printf '%s' "$lines" | grep -c . || true)
  if [ "$n" -eq 0 ]; then
    refuse "no pin line for $asset in $pins — the expected digest could not be read; refusing rather than guessing a platform or skipping verification"
  fi
  if [ "$n" -gt 1 ]; then
    refuse "$n competing pin lines for $asset in $pins — the expected digest is ambiguous; refusing"
  fi
  nf=$(printf '%s\n' "$lines" | awk '{print NF}')
  tag=$(printf '%s\n' "$lines" | awk '{print $2}')
  sha=$(printf '%s\n' "$lines" | awk '{print $3}')
  [ "$nf" -eq 3 ] || refuse "the pin line for $asset in $pins has $nf field(s), not '<asset> <tag> <sha256>' — the expected digest could not be read"
  if is_placeholder "$tag" || [ "$tag" = latest ]; then
    refuse "the pin line for $asset carries tag '$tag' — a placeholder or floating tag is not a pin"
  fi
  printf '%s' "$tag" | grep -Eq '^[A-Za-z0-9._/+-]+$' || refuse "the pin line for $asset carries an unusable tag '$tag'"
  case "$tag" in *..*) refuse "the pin line for $asset carries an unusable tag '$tag'" ;; esac
  is_sha256 "$sha" || refuse "the pin line for $asset carries '$sha', not a 64-lowercase-hex sha256 — the expected digest could not be read"
  printf '%s %s' "$tag" "$sha"
}

# sha256_of <file> — the file's sha256, or could-not-check when no tool can compute it.
sha256_of() {
  local out
  if command -v sha256sum >/dev/null 2>&1; then
    out=$(sha256sum "$1" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    out=$(shasum -a 256 "$1" | awk '{print $1}')
  elif command -v openssl >/dev/null 2>&1; then
    out=$(openssl dgst -sha256 -r "$1" | awk '{print $1}')
  else
    unverifiable "no sha256 tool (sha256sum, shasum, openssl) on PATH — the digest cannot be verified, so nothing is installed"
  fi
  is_sha256 "$out" || unverifiable "the sha256 tool produced no usable digest for $1"
  printf '%s' "$out"
}

# fetch <url> <out> — plain HTTPS (or file:// for offline fixtures). No forge CLI.
fetch() {
  local url="$1" out="$2"
  case "$url" in
    https://*|file://*) ;;
    *) refuse "refusing to fetch '$url' — only https:// (or file:// for an offline rehearsal) is accepted" ;;
  esac
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --proto '=https,file' --proto-redir '=https' -o "$out" "$url" \
      || unverifiable "fetch failed: $url — nothing installed"
  elif command -v wget >/dev/null 2>&1; then
    case "$url" in https://*) ;; *) unverifiable "wget cannot fetch '$url' (only https://) — install curl for a file:// rehearsal" ;; esac
    wget -q --https-only -O "$out" "$url" || unverifiable "fetch failed: $url — nothing installed"
  else
    unverifiable "no HTTPS fetcher (curl or wget) on PATH — nothing installed"
  fi
}

cmd_acquire() {
  local pins="" dest="" kind="statusgen" plat="" home="$DEFAULT_RELEASE_HOME" base="$DEFAULT_BASE_URL"
  while [ $# -gt 0 ]; do
    case "$1" in
      --pins) pins="${2:-}"; shift 2 ;;
      --dest) dest="${2:-}"; shift 2 ;;
      --kind) kind="${2:-}"; shift 2 ;;
      --platform) plat="${2:-}"; shift 2 ;;
      --release-home) home="${2:-}"; shift 2 ;;
      --base-url) base="${2:-}"; shift 2 ;;
      *) usage_err "acquire: unknown argument '$1'" ;;
    esac
  done
  [ -n "$pins" ] || usage_err "acquire --pins <.assay-versions> --dest <bindir> [--kind statusgen|desk-tools]"
  [ -n "$dest" ] || usage_err "acquire needs --dest <bindir>"
  [ -n "$plat" ] || plat=$(detect_platform)
  printf '%s' "$home" | grep -Eq '^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$' || usage_err "--release-home must be <owner>/<repo> (got '$home')"

  local asset fields tag want url tmp got
  asset=$(asset_name "$kind" "$plat") || exit $?
  fields=$(pin_fields "$pins" "$asset") || exit $?
  tag=${fields% *}
  want=${fields#* }
  url="${base%/}/$home/releases/download/$tag/$asset"
  say "acquire: $asset @ $tag from $url"
  say "acquire: expected sha256 (from $pins): $want"

  tmp=$(mktemp -d "${TMPDIR:-/tmp}/assay-install.XXXXXX") || unverifiable "cannot create a temp dir"
  # shellcheck disable=SC2064  # expand now: the trap must remove THIS temp dir
  trap "rm -rf '$tmp'" EXIT
  fetch "$url" "$tmp/$asset"
  got=$(sha256_of "$tmp/$asset") || exit $?
  if [ "$got" != "$want" ]; then
    refuse "sha256 MISMATCH for $asset @ $tag: pinned $want, fetched $got — nothing installed"
  fi
  say "verified: sha256 $got matches the pin — installing"

  mkdir -p "$dest" || unverifiable "cannot create --dest '$dest'"
  case "$kind" in
    statusgen)
      local name=statusgen
      case "$asset" in *.exe) name=statusgen.exe ;; esac
      install -m 0755 "$tmp/$asset" "$dest/$name" || unverifiable "install into $dest failed"
      say "installed: $dest/$name"
      ;;
    desk-tools)
      mkdir "$tmp/x" && tar -xzf "$tmp/$asset" -C "$tmp/x" || unverifiable "extracting $asset failed — nothing installed"
      local f count=0
      for f in "$tmp/x"/*; do
        [ -f "$f" ] || continue
        install -m 0755 "$f" "$dest/$(basename "$f")" || unverifiable "install into $dest failed"
        count=$((count + 1))
      done
      [ "$count" -gt 0 ] || unverifiable "$asset held no top-level files to install"
      say "installed: $count desk-tools binaries into $dest"
      ;;
  esac
}

# manifest_line <manifest> <section> <platform> — the channel-E pin line the manifest carries
# for that section + platform (the text after "<platform>:"), or empty.
manifest_line() {
  awk -v s="$2" -v p="$3" '
    /^[A-Za-z0-9_-]+:/ { sec = $1; sub(/:$/, "", sec); next }
    sec == s && $1 == p ":" { $1 = ""; sub(/^[[:space:]]+/, ""); print; exit }
  ' "$1"
}

manifest_home() {
  awk -v s="$2" '
    /^[A-Za-z0-9_-]+:/ { sec = $1; sub(/:$/, "", sec); next }
    sec == s && $1 == "release_home:" { print $2; exit }
  ' "$1"
}

cmd_pin() {
  local manifest="" pins="" kind="statusgen" plat=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --manifest) manifest="${2:-}"; shift 2 ;;
      --pins) pins="${2:-}"; shift 2 ;;
      --kind) kind="${2:-}"; shift 2 ;;
      --platform) plat="${2:-}"; shift 2 ;;
      *) usage_err "pin: unknown argument '$1'" ;;
    esac
  done
  [ -n "$manifest" ] && [ -n "$pins" ] || usage_err "pin --manifest <paired-versions.yaml> --pins <.assay-versions> [--kind statusgen|desk-tools]"
  [ -r "$manifest" ] || refuse "the pairing manifest '$manifest' is absent or unreadable"
  [ -n "$plat" ] || plat=$(detect_platform)

  local asset line mtag msha existing
  asset=$(asset_name "$kind" "$plat") || exit $?
  line=$(manifest_line "$manifest" "$kind" "$plat")
  [ -n "$line" ] || refuse "the manifest carries no $kind pin for platform $plat — refusing rather than guessing a platform"
  [ "$(printf '%s\n' "$line" | awk '{print $1}')" = "$asset" ] || refuse "the manifest's $kind line for $plat names '$(printf '%s\n' "$line" | awk '{print $1}')', expected $asset"
  mtag=$(printf '%s\n' "$line" | awk '{print $2}')
  msha=$(printf '%s\n' "$line" | awk '{print $3}')
  is_sha256 "$msha" || refuse "the manifest's $kind digest for $plat is not a 64-lowercase-hex sha256"
  line="$asset $mtag $msha"

  existing=""
  [ -f "$pins" ] && existing=$(awk -v a="$asset" '$0 !~ /^[[:space:]]*#/ && $1 == a' "$pins")
  if [ -z "$existing" ]; then
    printf '%s\n' "$line" >> "$pins" || unverifiable "cannot write $pins"
    say "pinned: $line"
    return 0
  fi
  if [ "$(printf '%s\n' "$existing" | awk '{print $1, $2, $3}')" = "$line" ]; then
    say "already pinned, left untouched: $line"
    return 0
  fi
  if is_placeholder "$(printf '%s\n' "$existing" | awk '{print $2}')"; then
    local tmpf
    tmpf=$(mktemp "${TMPDIR:-/tmp}/assay-pins.XXXXXX") || unverifiable "cannot create a temp file"
    awk -v a="$asset" -v l="$line" '$0 !~ /^[[:space:]]*#/ && $1 == a { print l; next } { print }' "$pins" > "$tmpf" \
      && cat "$tmpf" > "$pins" && rm -f "$tmpf" || unverifiable "cannot rewrite $pins"
    say "pinned (replaced the scaffold placeholder): $line"
    return 0
  fi
  refuse "$pins already pins $asset differently ('$(printf '%s' "$existing" | tr -s ' \t' ' ')'); a re-pin is a reviewed change, never an in-place edit — leaving it untouched"
}

cmd_classify() {
  local root=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --root) root="${2:-}"; shift 2 ;;
      *) usage_err "classify: unknown argument '$1'" ;;
    esac
  done
  [ -n "$root" ] || usage_err "classify --root <dir>"
  [ -d "$root" ] || refuse "classify: '$root' is not a directory"

  local streams="" pin="" ci="" present="" f
  for f in "$root"/docs/streams/*/README.md; do
    [ -f "$f" ] && streams="${streams:+$streams, }$(basename "$(dirname "$f")")"
  done
  if [ -f "$root/.assay-versions" ]; then
    pin=$(awk '$0 !~ /^[[:space:]]*#/ && $1 ~ /^statusgen(-[a-z0-9]+-[a-z0-9]+(\.exe)?)?$/ && $3 ~ /^[0-9a-f]+$/ && length($3) == 64 { print $2; exit }' "$root/.assay-versions")
  fi
  for f in "$root"/.github/workflows/*statusgen* "$root/.gitlab-ci.yml"; do
    [ -f "$f" ] && grep -q statusgen "$f" && ci="${ci:+$ci, }${f#"$root"/}"
  done

  if [ -n "$streams" ] && [ -n "$pin" ]; then
    say "adopted: streams [$streams]; statusgen pinned at $pin${ci:+; CI [$ci]}"
    refuse "this repo is already adopted; nothing to install — refusing rather than clobbering the live adoption"
  fi
  [ -d "$root/docs/streams" ] && present="${present:+$present, }docs/streams"
  [ -f "$root/.assay-versions" ] && present="${present:+$present, }.assay-versions${pin:+ ($pin)}"
  [ -n "$ci" ] && present="${present:+$present, }CI [$ci]"
  if [ -n "$present" ]; then
    say "partial: present = $present — only the unmet steps run; present artifacts are left untouched"
  else
    say "fresh: no streams tree, no pin, no statusgen CI"
  fi
}

cmd_rehearse() {
  local target="" manifest="" forge="" base="$DEFAULT_BASE_URL" work=""
  local here; here=$(cd "$(dirname "$0")" && pwd)
  while [ $# -gt 0 ]; do
    case "$1" in
      --target) target="${2:-}"; shift 2 ;;
      --manifest) manifest="${2:-}"; shift 2 ;;
      --forge) forge="${2:-}"; shift 2 ;;
      --base-url) base="${2:-}"; shift 2 ;;
      --workdir) work="${2:-}"; shift 2 ;;
      *) usage_err "rehearse: unknown argument '$1'" ;;
    esac
  done
  [ -n "$target" ] || usage_err "rehearse --target <repo> [--manifest <paired-versions.yaml>] [--forge github|gitlab] [--base-url <url>] [--workdir <dir>]"
  [ -d "$target" ] || refuse "rehearse: '$target' is not a directory"
  [ -n "$manifest" ] || manifest="$here/../paired-versions.yaml"
  case "$forge" in ""|github|gitlab) ;; *) usage_err "--forge must be github or gitlab" ;; esac

  say "rehearse: forge CLIs on PATH in this shell:"
  printf '  command -v gh   -> %s\n' "$(command -v gh || printf '(absent)')"
  printf '  command -v glab -> %s\n' "$(command -v glab || printf '(absent)')"

  [ -n "$work" ] || work=$(mktemp -d "${TMPDIR:-/tmp}/assay-rehearse.XXXXXX") || unverifiable "cannot create a workdir"
  mkdir -p "$work/bin" || unverifiable "cannot create $work/bin"
  local copy="$work/target"
  [ -e "$copy" ] && refuse "rehearse: $copy already exists — pass a fresh --workdir"
  cp -R "$target" "$copy" || unverifiable "cannot copy the target into $copy"
  say "rehearse: working on a scratch copy ($copy); the real target is never written"

  say "step 1/5 classify"
  bash "$0" classify --root "$copy" || exit $?

  local plat home
  plat=$(detect_platform)
  home=$(manifest_home "$manifest" statusgen)
  [ -n "$home" ] || refuse "the manifest names no statusgen release_home"
  say "step 2/5 pin (platform $plat, release home $home)"
  bash "$0" pin --manifest "$manifest" --pins "$copy/.assay-versions" --kind statusgen --platform "$plat" || exit $?

  say "step 3/5 acquire + verify"
  bash "$0" acquire --pins "$copy/.assay-versions" --dest "$work/bin" --kind statusgen \
    --platform "$plat" --release-home "$home" --base-url "$base" || exit $?

  local sg="$work/bin/statusgen" initargs
  say "step 4/5 scaffold — statusgen init (the scaffold is init's, never re-authored here)"
  initargs=(init --root "$copy")
  [ -n "$forge" ] && initargs+=(--forge "$forge")
  "$sg" "${initargs[@]}" || refuse "statusgen init failed — install NOT proven"
  [ -f "$copy/.gitlab-ci.yml" ] && say "scaffolded: .gitlab-ci.yml"
  [ -d "$copy/.github/workflows" ] && say "scaffolded: .github/workflows/"

  say "step 5/5 prove"
  local want got
  want=$(awk '$0 !~ /^[[:space:]]*#/ && $1 == "statusgen-'"$plat"'" { print $2; exit }' "$copy/.assay-versions")
  got=$("$sg" --version 2>/dev/null)
  printf '  statusgen --version -> %s (pinned: %s)\n' "$got" "$want"
  [ "$got" = "$want" ] || refuse "install NOT proven: the installed statusgen names itself '$got', not the pinned tag '$want'"
  "$sg" --root "$copy" --lint >"$work/lint.out" 2>&1
  local rc=$?
  printf '  statusgen --root <copy> --lint -> exit %s\n' "$rc"
  [ "$rc" -eq 0 ] || { cat "$work/lint.out" >&2; refuse "install NOT proven: --lint exited $rc"; }
  say "rehearsal PROVEN: acquired + sha256-verified, scaffolded by statusgen init, --version == pinned tag, --lint == 0"
  say "rehearsal: nothing pushed, no PR opened; artifacts left in $work"
}

main() {
  local sub="${1:-}"
  [ $# -gt 0 ] && shift
  case "$sub" in
    classify) cmd_classify "$@" ;;
    pin)      cmd_pin "$@" ;;
    acquire)  cmd_acquire "$@" ;;
    rehearse) cmd_rehearse "$@" ;;
    -h|--help|help) sed -n '2,51p' "$0" ;;
    *) usage_err "assay-install.sh classify|pin|acquire|rehearse … (see --help)" ;;
  esac
}

main "$@"
