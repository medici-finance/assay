#!/usr/bin/env bash
# End-to-end transcript for the local hotfix overlay, offline, in a throwaway HOME.
#
#   install overlay -> shim generation -> `running hotfix` (deskboot step) -> `HOTFIX:`
#   (deskboard header) -> pin moved -> `hotfix outlived its pin` (deskboot, exit 6) ->
#   regenerate shims -> `all pinned (v1.0.42)`
#
# Nothing here touches the real install dir, overlay dir, cells root or network: HOME, the
# cell, the pinned dir and the overlay dir are temp dirs, the source is a local bare clone, the
# forge is a local fake, and the verbs are built with the TEST-ONLY `hotfixseam` build tag
# (never passed by the release build) so the root-owner check, forge, clone URL and module
# proxy can point at those fixtures. The module proxy is the local module cache, so the
# dependencies must already be downloaded (`go mod download` once in tools/desk).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
desk="$(cd "$here/../../.." && pwd)"   # tools/desk
repo="$(cd "$desk/../.." && pwd)"      # repo root

goversion="$(go env GOVERSION)"
gocache="$(go env GOCACHE)"
gomodcache="$(go env GOMODCACHE)"
me="$(id -un)"
mykuid="$(id -u)"

tmp="$(cd "$(mktemp -d)" && pwd -P)"
forge_pid=""
cleanup() {
  [ -n "$forge_pid" ] && kill "$forge_pid" 2>/dev/null || true
  chmod -R u+w "$tmp" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT

step() { printf '\n== %s\n' "$*"; }
expect() { # expect <label> <file> <fixed string>
  if ! grep -qF -e "$3" "$2"; then
    printf 'FAIL: %s: expected %q in:\n' "$1" "$3" >&2
    sed 's/^/    /' "$2" >&2
    exit 1
  fi
}

mkdir -p "$tmp/home" "$tmp/pinned" "$tmp/overlay" "$tmp/cells/e2e" "$tmp/work"
export HOME="$tmp/home" GOCACHE="$gocache" GOMODCACHE="$gomodcache" GOFLAGS=-trimpath
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
export DESK_LOOP=worker-desk KUBECONFIG=/dev/null

pkg=github.com/medici-finance/assay/tools/desk/internal/deskkit
pin_stamp="-X $pkg.ReleaseTag=v1.0.41 -X $pkg.SourceSHA=aaaaaaaaaaaa -X $pkg.BuiltAt=2026-10-01T00:00:00Z"

step "build the pinned verbs (v1.0.41) with the test-only seam tag"
( cd "$desk" &&
  go build -tags hotfixseam -ldflags "$pin_stamp -X main.cellctlVersion=v1.0.41" -o "$tmp/pinned/cellctl" ./cmd/cellctl &&
  go build -tags hotfixseam -ldflags "$pin_stamp" -o "$tmp/pinned/deskboard" ./cmd/deskboard )
echo "built cellctl and pinned deskboard (v1.0.41)"

step "offline source: a bare clone of this tree, plus the release workflow's GO_VERSION"
mkdir -p "$tmp/work/src"
( cd "$repo" && git ls-files -co --exclude-standard -z -- tools/desk comms forgeread perfrec gitquiet deskcore statusgen |
  python3 -I -c '
import os, shutil, sys
dst = sys.argv[1]
for rel in sys.stdin.buffer.read().split(b"\0"):
    if not rel:
        continue
    src = rel.decode()
    if not os.path.isfile(src) or os.path.islink(src):
        continue
    out = os.path.join(dst, src)
    os.makedirs(os.path.dirname(out), exist_ok=True)
    shutil.copy2(src, out)
' "$tmp/work/src" )
mkdir -p "$tmp/work/src/.github/workflows"
printf 'env:\n  GO_VERSION: "%s"\n' "${goversion#go}" > "$tmp/work/src/.github/workflows/release.yml"
(
  cd "$tmp/work/src"
  git init -q -b main
  git add -A
  git -c user.name=e2e -c user.email=e2e@example.invalid commit -q -m "e2e source"
)
sha="$(git -C "$tmp/work/src" rev-parse HEAD)"
git clone -q --bare "$tmp/work/src" "$tmp/work/src.git"
echo "source $sha"

step "fake forge: main...<source> is behind (on main); the pin base is an ancestor"
cat > "$tmp/work/forge.py" <<'PY'
import http.server, json, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        spec = self.path.rsplit("/", 1)[-1].split("?")[0]
        base = spec.split("...")[0]
        body = json.dumps({"status": "behind" if base == "main" else "ahead", "files": []}).encode()
        self.send_response(200); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
    def log_message(self, *a): pass
s = http.server.HTTPServer(("127.0.0.1", 0), H)
open(sys.argv[1], "w").write(str(s.server_address[1]))
s.serve_forever()
PY
python3 -I "$tmp/work/forge.py" "$tmp/work/forge.port" &
forge_pid=$!
for _ in $(seq 1 50); do [ -s "$tmp/work/forge.port" ] && break; sleep 0.1; done
[ -s "$tmp/work/forge.port" ] || { echo "FAIL: fake forge did not start" >&2; exit 1; }

# The cell's worktree: pins desk-tools v1.0.41.
mkdir -p "$tmp/repo"
pins() { printf 'desk-tools %s 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n' "$1" > "$tmp/repo/.assay-versions"; }
pins v1.0.41

export ASSAY_HOTFIXSEAM_ROOT_UID="$mykuid"
export ASSAY_HOTFIXSEAM_FORGE="http://127.0.0.1:$(cat "$tmp/work/forge.port")"
export ASSAY_HOTFIXSEAM_CLONE="file://$tmp/work/src.git"
export ASSAY_HOTFIXSEAM_GOPROXY="file://$gomodcache/cache/download"
export SUDO_USER="$me"
export DESK_TOOLS_BIN="$tmp/pinned" DESK_TOOLS_HOTFIX="$tmp/overlay"
export HOTFIX_E2E_SHIMGEN="$tmp/cells/e2e" HOTFIX_E2E_REPO="$tmp/repo"

shimgen() { ( cd "$desk" && go test -tags hotfixseam -count=1 -run '^TestHotfixE2EShimGen$' ./cmd/cellctl >"$tmp/work/shimgen.out" 2>&1 ) || { cat "$tmp/work/shimgen.out" >&2; return 1; }; }
boot_step() { # runs boot step 10 against the cell; prints its lines
  ( cd "$desk" && ASSAY_CELL=e2e CELLS_ROOT="$tmp/cells" PATH="$tmp/cells/e2e/shim:$PATH" HOTFIX_E2E_BINARYPIN="$tmp/repo" \
      go test -tags hotfixseam -count=1 -v -run '^TestBinaryPinE2EDriver$' ./cmd/deskboot 2>&1 | grep '^deskboot:' ) || true
}

step "1. install the overlay (cellctl hotfix install deskboard)"
( cd "$tmp/repo" && "$tmp/pinned/cellctl" hotfix install deskboard --source "$sha" ) 2>&1 | tee "$tmp/work/install.out"
expect install "$tmp/work/install.out" "v1.0.41+hotfix-${sha:0:12}"
"$tmp/overlay/bin/deskboard" --version 2>&1 | tee "$tmp/work/overlay-version.out" | head -3
expect overlay-version "$tmp/work/overlay-version.out" "v1.0.41+hotfix-${sha:0:12}"

step "2. shim generation points the cell's deskboard at the overlay"
shimgen
grep -E '^(hotfix_bin|pinned_bin)=' "$tmp/cells/e2e/shim/deskboard" | tee "$tmp/work/shim.out"
expect shim "$tmp/work/shim.out" "hotfix_bin=\"$tmp/overlay/bin/deskboard\""

step "3. deskboot step binary-pin prints the marker"
boot_step | tee "$tmp/work/boot1.out"
expect boot-marker "$tmp/work/boot1.out" "running hotfix deskboard v1.0.41+hotfix-${sha:0:12} over pin v1.0.41"
expect boot-exit "$tmp/work/boot1.out" "exit=0"

step "4. deskboard's header names the hotfix"
( cd "$tmp/repo" && "$tmp/cells/e2e/shim/deskboard" throughput --json 2>/dev/null | grep '"staleDetail"' ) | tee "$tmp/work/board1.out"
expect board-header "$tmp/work/board1.out" "HOTFIX: desk-tools v1.0.41+hotfix-${sha:0:12} over pin v1.0.41"

step "5. the pin moves to v1.0.42; the shims are not regenerated"
pins v1.0.42
cat "$tmp/repo/.assay-versions" | cut -c1-30

step "6. deskboot refuses the hotfix that outlived its pin (exit 6)"
boot_step | tee "$tmp/work/boot2.out"
expect boot-outlived "$tmp/work/boot2.out" "hotfix outlived its pin"
expect boot-exit6 "$tmp/work/boot2.out" "exit=6"

step "7. regenerate the shims: the pinned deskboard is back, with a NOTICE"
shimgen
if grep -q '^hotfix_bin=' "$tmp/cells/e2e/shim/deskboard"; then echo "FAIL: the shim still names the overlay" >&2; exit 1; fi
grep -F "NOTICE" "$tmp/work/shimgen.out" || true
grep -F -e "\"$tmp/pinned/deskboard\"" "$tmp/cells/e2e/shim/deskboard" | head -1

step "8. deskboot is clean again"
boot_step | tee "$tmp/work/boot3.out"
expect boot-pinned "$tmp/work/boot3.out" "all pinned (v1.0.42)"
expect boot-exit0 "$tmp/work/boot3.out" "exit=0"

printf '\nhotfix-e2e: OK\n'
