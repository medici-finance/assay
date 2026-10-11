#!/usr/bin/env bash
# Offline tests for run.sh. Gremlins is a stub; `go version -m` is answered by a
# wrapper so the pin check runs for real against canned build info. Real Go builds
# gremlintriage and runs the fixture module's copy probe. No network.
# regression: #2647
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
real_go="$(command -v go)"
work="$(mktemp -d "${TMPDIR:-/tmp}/gremlins-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT
fails=0
fail() { echo "FAIL: $*" >&2; fails=$((fails + 1)); }
pass() { echo "ok: $*"; }

# --- fixture checkout: tools/desk is a tiny module with five packages --------
root="$work/root"
m="$root/tools/desk"
mkdir -p "$m"/pkg/{ok,slow,red,boom,big}
printf 'module example.com/fake\n\ngo 1.25\n' >"$m/go.mod"
for p in ok slow boom big; do
	printf 'package %s\n\nfunc F(n int) bool {\n\treturn n > 3\n}\n' "$p" >"$m/pkg/$p/f.go"
	printf 'package %s\n\nimport "testing"\n\nfunc TestF(t *testing.T) {\n\tif !F(4) {\n\t\tt.Fatal("F(4)")\n\t}\n}\n' "$p" >"$m/pkg/$p/f_test.go"
done
printf 'package big\n\nfunc G(n int) bool {\n\treturn n < 2\n}\n' >"$m/pkg/big/g.go"
# red passes in place and fails in any copy that is not under tools/desk: the
# copy probe must catch it.
printf 'package red\n\nfunc F() int { return 1 }\n' >"$m/pkg/red/f.go"
cat >"$m/pkg/red/f_test.go" <<'EOF'
package red

import (
	"os"
	"strings"
	"testing"
)

func TestLayout(t *testing.T) {
	wd, _ := os.Getwd()
	if !strings.Contains(wd, "/tools/desk/") {
		t.Fatal("not in tools/desk: " + wd)
	}
}
EOF
cat >"$m/pkg/ok/mutations.json" <<'EOF'
{"test": "go test ./pkg/ok/", "control": {"name": "c", "file": "pkg/big/g.go", "old": "return n < 2", "new": "return n <= 2"}, "mutations": []}
EOF

# --- stub gremlins ------------------------------------------------------------
stub="$work/stub"
mkdir -p "$stub"
cat >"$stub/gremlins" <<'EOF'
#!/usr/bin/env bash
dry=0 out="" excl=""
pkg="${@: -1}"
while [ $# -gt 0 ]; do
	case "$1" in
	--dry-run) dry=1 ;;
	-o) out="$2"; shift ;;
	--exclude-files) excl="$2"; shift ;;
	esac
	shift
done
cov=0.5s
case "$pkg" in *big*) cov=100s ;; esac
if [ "$dry" = 1 ]; then
	echo "Gathering coverage... done in $cov"
	g=""
	case "$pkg" in *big*) g=',{"file_name":"g.go","mutations":[{"type":"CONDITIONALS_BOUNDARY","status":"RUNNABLE","line":4,"column":11}]}' ;; esac
	printf '{"go_module":"example.com/fake","files":[{"file_name":"f.go","mutations":[{"type":"CONDITIONALS_BOUNDARY","status":"RUNNABLE","line":4,"column":11},{"type":"CONDITIONALS_NEGATION","status":"RUNNABLE","line":4,"column":11}]}%s]}\n' "$g" >"$out"
	exit 0
fi
printf '%s\t%s\t%s\n' "$pkg" "$TMPDIR" "$excl" >>"$STUB_CALLS"
mkdir -p "$TMPDIR/gremlins-123/wd-0"
echo "Gathering coverage... done in $cov"
case "$pkg" in
*boom*) echo "panic: engine broke"; exit 3 ;;
*slow*)
	printf '\033[32m      KILLED\033[0m CONDITIONALS_NEGATION at f.go:4:11\n'
	echo '       LIVED CONDITIONALS_BOUNDARY at f.go:4:11'
	trap 'kill $! 2>/dev/null; exit 1' INT
	sleep 30 &
	wait
	exit 0
	;;
esac
echo '      KILLED CONDITIONALS_NEGATION at f.go:4:11'
echo '       LIVED CONDITIONALS_BOUNDARY at f.go:4:11'
printf '{"go_module":"example.com/fake","files":[{"file_name":"f.go","mutations":[{"type":"CONDITIONALS_NEGATION","status":"KILLED","line":4,"column":11},{"type":"CONDITIONALS_BOUNDARY","status":"LIVED","line":4,"column":11}]}]}\n' >"$out"
EOF
chmod +x "$stub/gremlins"

# `go version -m` answers with canned build info; everything else is real Go.
goshim="$work/goshim"
mkdir -p "$goshim"
cat >"$goshim/go" <<EOF
#!/usr/bin/env bash
if [ "\$1" = version ] && [ "\$2" = -m ]; then cat "\$BUILDINFO"; exit 0; fi
exec "$real_go" "\$@"
EOF
chmod +x "$goshim/go"
good_info="$work/info-good"
read -r pin_mod pin_ver pin_h1 <"$here/gremlins.sum"
printf '%s: go1.25.0\n\tpath\t%s/cmd/gremlins\n\tmod\t%s\t%s\t%s\n' "$stub/gremlins" "$pin_mod" "$pin_mod" "$pin_ver" "$pin_h1" >"$good_info"
bad_info="$work/info-bad"
sed 's/h1:..../h1:XXXX/' "$good_info" >"$bad_info"

sweep() { # sweep NAME BUILDINFO ARGS... -> sets rc, writes $work/NAME.out
	local name="$1" info="$2"
	shift 2
	rc=0
	env PATH="$goshim:$PATH" BUILDINFO="$info" STUB_CALLS="$work/$name.calls" \
		bash "$here/run.sh" --root "$root" --gremlins "$stub/gremlins" -o "$work/$name" --day 0 "$@" \
		>"$work/$name.out" 2>&1 || rc=$?
}

# 1. survivors and a budget-stopped package are evidence, not a failure.
sweep a "$good_info" pkg/ok pkg/slow:3
if [ "$rc" = 0 ]; then pass "survivors exit 0"; else fail "survivors exit $rc"; cat "$work/a.out"; fi
grep -q $'^pkg/ok\tok\t' "$work/a/manifest.tsv" || fail "pkg/ok not recorded ok"
grep -q $'^pkg/slow\tpartial\t' "$work/a/manifest.tsv" || fail "pkg/slow not recorded partial"
# shellcheck disable=SC2016 # the backticks are literal Markdown code spans
grep -qF '`pkg/ok/f.go:4:11` `F` CONDITIONALS_BOUNDARY' "$work/a/summary.md" || fail "candidate row missing"
# shellcheck disable=SC2016
grep -qF '`pkg/slow/f.go:4:11`' "$work/a/summary.md" || fail "partial log survivor missing"
grep -q $'^pkg/ok\t'"$root"$'\t$' "$work/a.calls" || fail "gremlins did not run with TMPDIR at the checkout root"
ls -d "$root"/gremlins-* >/dev/null 2>&1 && fail "mutant copies left behind"
pass "manifest, summary, TMPDIR and cleanup"

# 2. a package whose tests are red in the copy layout breaks the sweep.
sweep b "$good_info" pkg/ok pkg/red
[ "$rc" = 2 ] || fail "probe-red exit $rc, want 2"
grep -q $'^pkg/red\tprobe-red\t' "$work/b/manifest.tsv" || fail "pkg/red not recorded probe-red"
grep -q '^pkg/red' "$work/b.calls" 2>/dev/null && fail "gremlins ran on a probe-red package"
pass "copy probe"

# 3. a Gremlins failure breaks the sweep.
sweep c "$good_info" pkg/ok pkg/boom
[ "$rc" = 2 ] || fail "gremlins failure exit $rc, want 2"
grep -q $'^pkg/boom\tfailed\t' "$work/c/manifest.tsv" || fail "pkg/boom not recorded failed"
pass "gremlins failure"

# 4. a binary without the pinned module sum never runs.
sweep d "$bad_info" pkg/ok
[ "$rc" = 2 ] || fail "wrong pin exit $rc, want 2"
grep -q 'pinned module sum' "$work/d.out" || fail "wrong pin not named"
[ -e "$work/d.calls" ] && fail "gremlins ran despite the wrong pin"
pass "pin check"

# 5. a slow suite gets today's slice of files only.
sweep e "$good_info" pkg/big:150
[ "$rc" = 0 ] || fail "plan sweep exit $rc"
grep -qF $'pkg/big\t'"$root"$'\t^(?:g\\.go)$' "$work/e.calls" || { fail "day 0 should exclude g.go"; cat "$work/e.calls"; }
pass "rotation exclusion"

# 6. leftovers from an earlier run refuse the sweep.
mkdir "$root/gremlins-old"
sweep f "$good_info" pkg/ok
[ "$rc" = 2 ] || fail "leftover exit $rc, want 2"
rmdir "$root/gremlins-old"
pass "disposable-checkout check"

# 7. a pin file that is not one go.sum line for Gremlins refuses before any install.
alt="$work/alt/tools"
mkdir -p "$alt/gremlins"
cp "$here/run.sh" "$alt/gremlins/run.sh"
ln -s "$here/../desk" "$alt/desk"
printf 'github.com/example/other v0.6.0 h1:AAAA=\n' >"$alt/gremlins/gremlins.sum"
rc=0
env PATH="$goshim:$PATH" BUILDINFO="$good_info" STUB_CALLS="$work/g.calls" \
	bash "$alt/gremlins/run.sh" --root "$root" --gremlins "$stub/gremlins" -o "$work/g" pkg/ok \
	>"$work/g.out" 2>&1 || rc=$?
[ "$rc" = 2 ] || fail "bad pin file exit $rc, want 2"
grep -q 'not one go.sum line' "$work/g.out" || fail "bad pin file not named"
[ -e "$work/g.calls" ] && fail "gremlins ran despite a bad pin file"
pass "pin file shape"

if [ "$fails" -gt 0 ]; then
	echo "$fails failure(s)" >&2
	exit 1
fi
echo "all run.sh tests passed"
