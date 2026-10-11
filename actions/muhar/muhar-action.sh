#!/usr/bin/env bash
# muhar-action.sh — the logic behind actions/muhar/action.yml.
#
#   muhar-action.sh install     build muhar from this checkout's tools/desk at the
#                               action's own ref; prints the binary path and, under
#                               Actions, writes it to $GITHUB_OUTPUT as muhar-bin
#   muhar-action.sh run         run every spec, write the verdicts to the job summary,
#                               set the outputs, and exit with the gate's verdict
#   muhar-action.sh parse FILE  print "caught=N not-caught=N could-not-mutate=N" from
#                               one muhar report; exit 3 when FILE holds no single
#                               Totals line this script can read
#
# Everything the action passes in arrives as an environment variable, never as text
# spliced into a shell line, so an input value cannot become a command.
#
#   install: MUHAR_VERSION (must be empty for now), MUHAR_ACTION_PATH, RUNNER_TEMP
#   run:     MUHAR_BIN, MUHAR_SPECS (newline-separated), MUHAR_WORKDIR, MUHAR_JOBS,
#            MUHAR_SHARD, MUHAR_FAIL_ON_NOT_CAUGHT, MUHAR_FAIL_ON_COULD_NOT_MUTATE,
#            GITHUB_STEP_SUMMARY and GITHUB_OUTPUT (each optional: unset, that file
#            is not written)
#
# The exit-code contract this script reads is muhar's own (tools/desk/cmd/muhar/main.go):
#   0  harness healthy; the verdicts and the Totals line are on stdout
#   2  HARNESS BROKEN (red baseline or uncaught control) — no verdict is trustworthy
#   1  usage, spec or IO error
# A healthy run that still has NOT CAUGHT or could-not-mutate mutations exits 0, so
# this script reads the Totals line to decide those.
#
# This script's own exit codes:
#   0  every spec ran healthy and nothing the inputs ask to fail on was found
#   2  at least one spec's harness was broken — never read as clean
#   1  anything else that fails the step: a NOT CAUGHT or could-not-mutate the
#      inputs fail on, a Totals line that could not be read, a bad input, or a muhar
#      exit other than 0 or 2 (that exit code is passed through when it is not 0/1/2)
set -uo pipefail

# The Totals line exactly as Report.Summary prints it. The per-mutant lines are
# indented, so a mutation whose NAME looks like a Totals line never matches.
TOTALS_RE='^Totals: ([0-9]+) caught, ([0-9]+) NOT CAUGHT, ([0-9]+) could-not-mutate\.$'

die() { printf '::error::%s\n' "$(escape_cmd "$*")"; exit 1; }

# escape_cmd makes a value safe inside a workflow command (::error::...).
escape_cmd() {
	local s=$1
	s=${s//'%'/'%25'}
	s=${s//$'\r'/'%0D'}
	s=${s//$'\n'/'%0A'}
	printf '%s' "$s"
}

# escape_html makes a value safe inside the job summary's HTML.
escape_html() { sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g'; }

summary() { [ -n "${GITHUB_STEP_SUMMARY:-}" ] && printf '%s\n' "$@" >>"$GITHUB_STEP_SUMMARY"; return 0; }

output() { [ -n "${GITHUB_OUTPUT:-}" ] && printf '%s=%s\n' "$1" "$2" >>"$GITHUB_OUTPUT"; return 0; }

# parse_totals FILE — prints "C N M" for the one Totals line in FILE; returns 3 when
# there is no such line or more than one (a report this script cannot read is never
# guessed at).
parse_totals() {
	local found
	found=$(sed -nE "s/${TOTALS_RE}/\\1 \\2 \\3/p" "$1")
	if [ -z "$found" ] || [ "$(printf '%s\n' "$found" | wc -l | tr -d ' ')" != 1 ]; then
		return 3
	fi
	printf '%s\n' "$found"
}

cmd_parse() {
	[ $# -eq 1 ] || { echo "usage: muhar-action.sh parse FILE" >&2; exit 1; }
	local t c n m
	t=$(parse_totals "$1") || exit 3
	read -r c n m <<<"$t"
	printf 'caught=%s not-caught=%s could-not-mutate=%s\n' "$c" "$n" "$m"
}

cmd_install() {
	# HOOK for a release-binary install (a pinned, checksum-verified muhar release).
	# Not implemented: until it is, a non-empty muhar-version refuses rather than
	# silently building something other than what was asked for.
	if [ -n "${MUHAR_VERSION:-}" ]; then
		die "muhar-version '${MUHAR_VERSION}' was set, but installing a release binary is not implemented yet. Leave muhar-version empty: the action builds muhar from source at the ref you pinned the action to."
	fi
	local here src bindir
	here=${MUHAR_ACTION_PATH:-$(cd "$(dirname "$0")" && pwd)}
	src="$here/../../tools/desk"
	[ -f "$src/go.mod" ] || die "could-not-check: muhar source not found at $src — the action must run from a checkout of its own repository"
	command -v go >/dev/null 2>&1 || die "could-not-check: no Go toolchain on PATH. Keep setup-go: true, or install Go in an earlier step."
	bindir="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/muhar-action-bin"
	mkdir -p "$bindir" || die "cannot create $bindir"
	# GOFLAGS is pinned for this build only: a caller's own value (say -mod=vendor, set
	# for its suite) must not change how muhar's committed go.mod/go.sum build.
	(cd "$src" && GOFLAGS=-mod=readonly go build -trimpath -o "$bindir/muhar" ./cmd/muhar) || die "go build ./cmd/muhar failed in $src"
	echo "muhar built at $bindir/muhar"
	output muhar-bin "$bindir/muhar"
}

is_bool() { [ "$1" = true ] || [ "$1" = false ]; }

cmd_run() {
	local bin=${MUHAR_BIN:-} workdir=${MUHAR_WORKDIR:-.} jobs=${MUHAR_JOBS:-1} shard=${MUHAR_SHARD:-}
	local fail_nc=${MUHAR_FAIL_ON_NOT_CAUGHT:-true} fail_cnm=${MUHAR_FAIL_ON_COULD_NOT_MUTATE:-true}

	[ -n "$bin" ] && [ -x "$bin" ] || die "could-not-check: muhar binary '$bin' is not executable — did the install step run?"
	[[ $jobs =~ ^[0-9]+$ ]] || die "jobs must be a non-negative integer (0 = one per CPU, capped by muhar), got '$jobs'"
	[ -z "$shard" ] || [[ $shard =~ ^[0-9]+/[0-9]+$ ]] || die "shard must be empty or 'i/n' (0-based shard i of n), got '$shard'"
	is_bool "$fail_nc" || die "fail-on-not-caught must be true or false, got '$fail_nc'"
	is_bool "$fail_cnm" || die "fail-on-could-not-mutate must be true or false, got '$fail_cnm'"
	[ -d "$workdir" ] || die "working-directory '$workdir' does not exist"

	local specs=() line
	while IFS= read -r line || [ -n "$line" ]; do
		line="${line#"${line%%[![:space:]]*}"}"
		line="${line%"${line##*[![:space:]]}"}"
		[ -z "$line" ] && continue
		case "$line" in '#'*) continue ;; esac
		specs+=("$line")
	done <<<"${MUHAR_SPECS:-}"
	[ ${#specs[@]} -gt 0 ] || die "no spec given: set the spec input to one or more mutation spec paths, one per line"

	local tmp
	tmp=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/muhar-action.XXXXXX") || die "cannot create a temp dir"
	trap 'rm -rf "$tmp"' EXIT

	local sum_c=0 sum_n=0 sum_m=0 broken=0 errored=0 passthrough=0 i=0 spec out rc t c n m status
	local rows=()
	summary "## muhar mutation gate" ""
	if [ -n "$shard" ]; then
		summary "> **Partial sweep — shard $shard.** Each Totals line below covers only this shard's share of its spec; the other shards own the rest. The baseline and the positive control still ran in full here." ""
	fi
	for spec in "${specs[@]}"; do
		i=$((i + 1))
		out="$tmp/report.$i"
		local args=(-spec "$spec" -j "$jobs")
		[ -n "$shard" ] && args+=(-shard "$shard")
		echo "::group::muhar $(escape_cmd "$spec")"
		# The spec's test command is the suite under test and inherits muhar's
		# environment. Withhold the workflow file commands from it, so nothing it runs
		# can write this action's outputs or summary, or set env/PATH for later steps.
		(cd "$workdir" && env -u GITHUB_OUTPUT -u GITHUB_ENV -u GITHUB_PATH -u GITHUB_STATE -u GITHUB_STEP_SUMMARY \
			"$bin" "${args[@]}") >"$out"
		rc=$?
		cat "$out"
		echo "::endgroup::"

		c=- n=- m=-
		case $rc in
		0)
			if t=$(parse_totals "$out"); then
				read -r c n m <<<"$t"
				sum_c=$((sum_c + c)) sum_n=$((sum_n + n)) sum_m=$((sum_m + m))
				status=healthy
			else
				status="could-not-check: no Totals line"
				errored=1
				printf '::error::%s\n' "$(escape_cmd "muhar exited 0 for $spec but printed no single Totals line this action can read — could-not-check, not clean")"
			fi
			;;
		2)
			status="harness broken — not clean"
			broken=1
			printf '::error::%s\n' "$(escape_cmd "muhar: harness broken — not clean ($spec). The baseline was red or the positive control was not caught, so this run carries no trustworthy verdict.")"
			;;
		1)
			status="usage or spec error (exit 1)"
			errored=1
			printf '::error::%s\n' "$(escape_cmd "muhar exited 1 for $spec: usage, spec or IO error — no verdict")"
			;;
		*)
			status="muhar exited $rc"
			errored=1
			[ "$passthrough" -eq 0 ] && passthrough=$rc
			printf '::error::%s\n' "$(escape_cmd "muhar exited $rc for $spec — no verdict")"
			;;
		esac

		rows+=("| <code>$(printf '%s' "$spec" | escape_html)</code> | $status | $c | $n | $m |")
		summary "### <code>$(printf '%s' "$spec" | escape_html)</code> — $status" "" "<pre>" "$(escape_html <"$out")" "</pre>" ""
	done

	summary "| Spec | Result | caught | NOT CAUGHT | could-not-mutate |" "|---|---|---|---|---|" "${rows[@]}" ""

	local result=clean code=0
	if [ "$broken" -eq 1 ]; then
		result=harness-broken code=2
	elif [ "$errored" -eq 1 ]; then
		result=error code=1
		[ "$passthrough" -ne 0 ] && code=$passthrough
	else
		# Every spec ran healthy: the counts are trustworthy, so publish them.
		output caught "$sum_c"
		output not-caught "$sum_n"
		output could-not-mutate "$sum_m"
		echo "Totals across ${#specs[@]} spec(s): $sum_c caught, $sum_n NOT CAUGHT, $sum_m could-not-mutate."
		if [ "$sum_n" -gt 0 ]; then
			result=not-caught
			if [ "$fail_nc" = true ]; then
				code=1
				printf '::error::%s\n' "muhar: $sum_n mutation(s) NOT CAUGHT — the guards they disarm have no discriminating test"
			else
				printf '::warning::%s\n' "muhar: $sum_n mutation(s) NOT CAUGHT (fail-on-not-caught is false, so the step still passes)"
			fi
		fi
		if [ "$sum_m" -gt 0 ]; then
			[ "$result" = clean ] && result=could-not-mutate
			if [ "$fail_cnm" = true ]; then
				code=1
				printf '::error::%s\n' "muhar: $sum_m mutation(s) could-not-mutate — those guards were never tested, so this is not a clean gate"
			else
				printf '::warning::%s\n' "muhar: $sum_m mutation(s) could-not-mutate — never tested, not clean (fail-on-could-not-mutate is false, so the step still passes)"
			fi
		fi
	fi
	output result "$result"
	summary "**Result: $result**"
	exit "$code"
}

case "${1:-}" in
install) cmd_install ;;
run) cmd_run ;;
parse) shift; cmd_parse "$@" ;;
*) echo "usage: muhar-action.sh install|run|parse FILE" >&2; exit 1 ;;
esac
