#!/usr/bin/env bash
# run.sh — the daily Gremlins sweep over the desk tools' muhar-spec packages.
#
# DISCOVERY, NEVER A GATE. Gremlins mutates every comparison and logical operator
# it finds and reports the mutants the tests did not catch. Those survivors are
# leads. Exit 0 means the sweep ran, whatever it found; exit 2 means the sweep
# itself broke (install, pin check, a Gremlins failure, a package whose tests are
# red in the mutant copy layout, or no evidence at all). See README.md here.
#
# RUN IT IN A DISPOSABLE CHECKOUT. Gremlins copies the Go module into
# $TMPDIR/gremlins-*/wd-* for every mutant. The desk module reaches files two
# levels up (its go.mod replaces ../../forgeread, and tests read ../../docs), so
# TMPDIR is set to the checkout root: then wd-*/../.. is the checkout root and
# those paths resolve. The copies are deleted on exit.
#
# usage: run.sh -o OUT_DIR [-b SECONDS] [-w WORKERS] [--day N] [--root DIR]
#               [--gremlins BIN] [PKG[:SECONDS] ...]
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The pin: gremlins.sum holds one go.sum line, `<module> <version> h1:<sum>`, with
# the module hash sum.golang.org records for that version. go install verifies the
# download against the checksum database; the build-info check below proves the
# binary that runs was built from exactly that module.
GREMLINS_MOD="" GREMLINS_VERSION="" GREMLINS_H1=""
read -r GREMLINS_MOD GREMLINS_VERSION GREMLINS_H1 _ <"$here/gremlins.sum" || true
case "$GREMLINS_MOD $GREMLINS_VERSION $GREMLINS_H1" in
github.com/go-gremlins/gremlins\ v*\ h1:*) ;;
*) echo "gremlins/run.sh: $here/gremlins.sum is not one go.sum line for github.com/go-gremlins/gremlins" >&2; exit 2 ;;
esac

# The muhar-spec packages, relative to tools/desk. A ":SECONDS" suffix overrides
# the budget: deskkit's suite takes minutes per run, so it gets more.
DEFAULT_PACKAGES=(
	internal/deskkit:1200
	cmd/deskmerge
	internal/loopengine
	cmd/deskclose
	cmd/reviewloop
	cmd/desksourceguard
)

# Guard-focused mutators: flipped and shifted comparisons and swapped && / ||.
# Arithmetic and increment mutants are off: in these packages they are mostly
# string concatenation and loop counters, and they would triple the run.
MUTATORS=(
	--conditionals-negation=true
	--conditionals-boundary=true
	--invert-logical=true
	--arithmetic-base=false
	--increment-decrement=false
	--invert-negatives=false
)

out="" budget=600 workers=2 day="" root="" gremlins=""
pkgs=()

die() {
	echo "gremlins/run.sh: $*" >&2
	exit 2
}

while [ $# -gt 0 ]; do
	case "$1" in
	-o | --out) out="${2:?}"; shift 2 ;;
	-b | --budget) budget="${2:?}"; shift 2 ;;
	-w | --workers) workers="${2:?}"; shift 2 ;;
	--day) day="${2:?}"; shift 2 ;;
	--root) root="${2:?}"; shift 2 ;;
	--gremlins) gremlins="${2:?}"; shift 2 ;;
	-h | --help) sed -n '2,17p' "$0"; exit 0 ;;
	-*) die "unknown flag $1" ;;
	*) pkgs+=("$1"); shift ;;
	esac
done
[ -n "$out" ] || die "-o OUT_DIR is required"
[ ${#pkgs[@]} -gt 0 ] || pkgs=("${DEFAULT_PACKAGES[@]}")
case "$budget$workers" in *[!0-9]*) die "budget and workers must be whole numbers" ;; esac
[ -n "$day" ] || day=$(($(date -u +%s) / 86400))
[ -n "$root" ] || root="$here/../.."
root="$(cd "$root" && pwd)"
module="$root/tools/desk"
[ -f "$module/go.mod" ] || die "no tools/desk/go.mod under $root"
command -v timeout >/dev/null || die "GNU timeout is required"
for left in "$root"/gremlins-*; do
	[ -e "$left" ] && die "$left exists; run in a fresh disposable checkout"
done

mkdir -p "$out/bin"
out="$(cd "$out" && pwd)"
gotmp="$(mktemp -d "${TMPDIR:-/tmp}/gremlins-gotmp.XXXXXX")"
# shellcheck disable=SC2329 # invoked by the EXIT trap
cleanup() { rm -rf "$root"/gremlins-* "$gotmp"; }
trap cleanup EXIT

# --- install and verify the pinned Gremlins --------------------------------
if [ -z "$gremlins" ]; then
	echo "installing ${GREMLINS_MOD}@${GREMLINS_VERSION}"
	# Checksum verification cannot be skipped or redirected:
	# - GOENV=off: go ignores the `go env -w` file, so an empty value below really
	#   is empty (cmd/go reads an empty variable as unset and falls back to that file).
	# - The GONOSUMDB/GOPRIVATE family is cleared; GOPROXY and GOSUMDB are pinned.
	# - GOMODCACHE is fresh: go never re-hashes a module already extracted in a
	#   shared cache, so a persistent runner's cache cannot stand in for the download.
	env GOENV=off GOBIN="$out/bin" GOTOOLCHAIN=local GOFLAGS="-mod=mod -modcacherw" \
		GOMODCACHE="$gotmp/modcache" \
		GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
		GONOSUMDB= GONOSUMCHECK= GOPRIVATE= GONOPROXY= GOINSECURE= \
		go install "${GREMLINS_MOD}/cmd/gremlins@${GREMLINS_VERSION}" || die "install failed"
	gremlins="$out/bin/gremlins"
fi
[ -x "$gremlins" ] || die "no gremlins binary at $gremlins"
buildinfo="$(go version -m "$gremlins")" || die "cannot read build info of $gremlins"
want="$(printf 'mod\t%s\t%s\t%s' "$GREMLINS_MOD" "$GREMLINS_VERSION" "$GREMLINS_H1")"
grep -qF "$want" <<<"$buildinfo" || die "$gremlins is not ${GREMLINS_MOD} ${GREMLINS_VERSION} with the pinned module sum"
echo "gremlins ${GREMLINS_VERSION} verified (${GREMLINS_H1})"

# The triage tool is built from this script's own checkout, never the target's.
(cd "$here/../desk" && go build -o "$out/bin/gremlintriage" ./internal/gremlintriage) || die "cannot build gremlintriage"

# --- one package at a time --------------------------------------------------
manifest="$out/manifest.tsv"
: >"$manifest"
record() { printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$@" >>"$manifest"; }

for spec in "${pkgs[@]}"; do
	pkg="${spec%%:*}"
	b="$budget"
	[ "$pkg" = "$spec" ] || b="${spec#*:}"
	case "$b" in '' | *[!0-9]*) die "bad budget in $spec" ;; esac
	slug="${pkg//\//_}"
	dir="$out/$slug"
	mkdir -p "$dir"
	start=$(date +%s)
	wall() { echo $(($(date +%s) - start)); }
	echo "== $pkg (budget ${b}s)"

	# 1. Dry run: which mutants exist, and what one uncached test run costs.
	rc=0
	(cd "$module" && GOFLAGS=-count=1 timeout --kill-after=30 "$b" \
		"$gremlins" unleash --dry-run "${MUTATORS[@]}" -o "$dir/dry.json" "$pkg") >"$dir/dry.log" 2>&1 || rc=$?
	if [ "$rc" -ne 0 ] || [ ! -s "$dir/dry.json" ]; then
		record "$pkg" dry-run-failed "$(wall)" "$rc" - "$dir/dry.log" -
		continue
	fi

	# 2. Plan: today's slice of files, so the run fits the budget.
	rc=0
	"$out/bin/gremlintriage" plan -dry-run "$dir/dry.json" -log "$dir/dry.log" \
		-budget "$b" -workers "$workers" -day "$day" >"$dir/plan.txt" 2>"$dir/plan.log" || rc=$?
	if [ "$rc" -ne 0 ]; then
		record "$pkg" dry-run-failed "$(wall)" "$rc" - "$dir/plan.log" -
		continue
	fi
	{ read -r planline; IFS= read -r exclude || exclude=""; } <"$dir/plan.txt"
	bin="$(awk '{print $2}' <<<"$planline")"
	echo "   plan: $planline"

	# 3. Copy probe, the positive control for the layout: the package's tests must
	# pass in a module copy two levels below the root, exactly where Gremlins puts
	# each mutant. Red here means every kill below would be false.
	probe="$root/gremlins-probe/wd-probe"
	mkdir -p "$(dirname "$probe")"
	cp -R "$module" "$probe"
	rc=0
	(cd "$probe" && TMPDIR="$root" GOTMPDIR="$gotmp" timeout --kill-after=30 "$b" \
		go test -count=1 "./$pkg") >"$dir/probe.log" 2>&1 || rc=$?
	rm -rf "$root/gremlins-probe"
	if [ "$rc" -ne 0 ]; then
		record "$pkg" probe-red "$(wall)" "$rc" - "$dir/probe.log" "$bin"
		continue
	fi

	# 4. The run, stopped with SIGINT at the budget. Gremlins writes no report when
	# interrupted, so a stopped run is read back from its log as partial.
	args=(unleash --workers "$workers" "${MUTATORS[@]}" -o "$dir/report.json")
	[ -z "$exclude" ] || args+=(--exclude-files "$exclude")
	rc=0
	(cd "$module" && TMPDIR="$root" GOTMPDIR="$gotmp" GOFLAGS=-count=1 \
		timeout --signal=INT --kill-after=60 "$b" "$gremlins" "${args[@]}" "$pkg") >"$dir/run.log" 2>&1 || rc=$?
	case "$rc" in
	0) if [ -s "$dir/report.json" ]; then state=ok; else state=failed; fi ;;
	124 | 137) state=partial ;;
	*) state=failed ;;
	esac
	record "$pkg" "$state" "$(wall)" "$rc" "$dir/report.json" "$dir/run.log" "$bin"
	echo "   $state in $(wall)s"
	rm -rf "$root"/gremlins-*
done

# --- triage -----------------------------------------------------------------
rc=0
"$out/bin/gremlintriage" triage -manifest "$manifest" -module-root "$module" \
	-md "$out/summary.md" -json "$out/triage.json" || rc=$?
echo "summary: $out/summary.md"
exit "$rc"
