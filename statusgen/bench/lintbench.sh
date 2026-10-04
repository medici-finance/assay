#!/usr/bin/env bash
# lintbench.sh — the harness for forge-neutral/18 Verify row 13: wall time of an OFFLINE
# `statusgen --lint` on a tree of 400 briefs or more, best of three, for a pre-change binary
# against a candidate binary built from the same tree's history.
#
#   lintbench.sh gen  <dir> [streams] [briefs-per-stream]
#       Synthesize a fixture: a git repo at <dir>/tree whose origin (<dir>/origin.git) carries
#       main, one commit per stream, every brief a minimal row that lints clean. Defaults:
#       20 streams x 21 briefs = 420 briefs.
#
#   lintbench.sh time <tree> <baseline-bin> <candidate-bin> [runs]
#       Run `<bin> --root . --lint` in <tree>, <runs> times each (default 3), offline: a clean
#       environment whose PATH resolves git and nothing forge-shaped (the run refuses if `gh`,
#       `glab` or `deskread` is resolvable). Prints each run, the best of each, and the
#       candidate's improvement as a percentage of the baseline's best.
#
# It REFUSES (exit 2) a tree with fewer than 400 briefs and a pair whose verdicts differ
# (exit code or stdout) — a faster run that answers differently is not the row's speed-up.
# It never decides the row: it prints the numbers, and "met" only when the improvement is
# at least 60 %. Exit 0 = measured; exit 1 = measured and under 60 %; exit 2 = refused.
set -euo pipefail

die() { echo "lintbench: $*" >&2; exit 2; }
now() { perl -MTime::HiRes=time -e 'printf "%.3f\n", time'; }

gen() {
	local dir=$1 nstreams=${2:-20} nbriefs=${3:-21}
	[[ -e $dir/tree ]] && die "$dir/tree already exists"
	mkdir -p "$dir"
	git init -q --bare -b main "$dir/origin.git"
	git init -q -b main "$dir/tree"
	local t=$dir/tree
	git -C "$t" config user.name lintbench
	git -C "$t" config user.email lintbench@example.invalid
	git -C "$t" config commit.gpgsign false
	mkdir -p "$t/docs/streams"
	printf '# Findings\n' >"$t/docs/streams/FINDINGS.md"
	git -C "$t" add -A
	git -C "$t" commit -q -m "findings"
	local s b name
	for ((s = 1; s <= nstreams; s++)); do
		name=$(printf 'bench%02d' "$s")
		mkdir -p "$t/docs/streams/$name"
		{
			printf -- '---\nstream: %s\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# %s\n\n' "$name" "$name"
			printf '| # | Brief | Wave | Status | Verified | Reviewed |\n|---|-------|------|--------|----------|----------|\n'
			for ((b = 1; b <= nbriefs; b++)); do
				if ((b == 1)); then
					printf '| %02d | [Brief %02d](brief-%02d.md) | 0 | done | grandfathered | grandfathered |\n' "$b" "$b" "$b"
				else
					printf '| %02d | [Brief %02d](brief-%02d.md) | 1 | todo | — | — |\n' "$b" "$b" "$b"
				fi
			done
		} >"$t/docs/streams/$name/README.md"
		for ((b = 1; b <= nbriefs; b++)); do
			printf '# Brief %02d\n' "$b" >"$t/docs/streams/$name/brief-$(printf '%02d' "$b").md"
		done
		git -C "$t" add -A
		git -C "$t" commit -q -m "$name"
	done
	git -C "$t" remote add origin "$dir/origin.git"
	git -C "$t" push -q origin main
	git -C "$t" fetch -q origin
	echo "lintbench: $(count_briefs "$t") briefs in $t"
}

count_briefs() { find "$1/docs/streams" -mindepth 2 -maxdepth 2 -name 'brief-*.md' | wc -l | tr -d ' '; }

time_cmd() {
	local tree=$1 base=$2 cand=$3 runs=${4:-3}
	[[ -d $tree/docs/streams ]] || die "$tree has no docs/streams"
	local n
	n=$(count_briefs "$tree")
	((n >= 400)) || die "$tree has $n briefs; row 13 is measured on 400 or more"
	local scratch
	scratch=$(mktemp -d "${TMPDIR:-/tmp}/lintbench.XXXXXX")
	mkdir -p "$scratch/bin" "$scratch/home"
	ln -s "$(command -v git)" "$scratch/bin/git"
	local path=$scratch/bin:/usr/bin:/bin
	for tool in gh glab deskread; do
		if PATH=$path command -v "$tool" >/dev/null 2>&1; then
			die "'$tool' is resolvable on the offline PATH ($path) — not an offline measurement"
		fi
	done
	echo "lintbench: $n briefs, $runs run(s) each, offline PATH=$path"
	local which bin i t0 t1 rc best_base='' best_cand='' out_base='' out_cand='' rc_base='' rc_cand=''
	for which in base cand; do
		bin=$base
		[[ $which == cand ]] && bin=$cand
		for ((i = 1; i <= runs; i++)); do
			t0=$(now)
			set +e
			(cd "$tree" && env -i HOME="$scratch/home" PATH="$path" TZ=UTC "$bin" --root . --lint \
				>"$scratch/$which.out" 2>"$scratch/$which.err")
			rc=$?
			set -e
			t1=$(now)
			local dt
			dt=$(perl -e "printf '%.3f', $t1 - $t0")
			echo "  $which run $i: ${dt}s rc=$rc"
			if [[ $which == base ]]; then
				rc_base=$rc
				if [[ -z $best_base ]] || perl -e "exit !($dt < $best_base)"; then best_base=$dt; fi
				out_base=$scratch/base.out
			else
				rc_cand=$rc
				if [[ -z $best_cand ]] || perl -e "exit !($dt < $best_cand)"; then best_cand=$dt; fi
				out_cand=$scratch/cand.out
			fi
		done
	done
	[[ $rc_base == "$rc_cand" ]] || die "verdicts differ: baseline exit $rc_base, candidate exit $rc_cand"
	cmp -s "$out_base" "$out_cand" || die "verdicts differ: stdout of the two binaries is not identical"
	local pct
	pct=$(perl -e "printf '%.1f', ($best_base - $best_cand) / $best_base * 100")
	echo "lintbench: best baseline ${best_base}s, best candidate ${best_cand}s, improvement ${pct}% (row 13 asks >= 60%)"
	rm -rf "$scratch"
	if perl -e "exit !($pct >= 60)"; then
		echo "lintbench: met"
	else
		echo "lintbench: NOT met"
		return 1
	fi
}

case ${1:-} in
gen) shift; gen "$@" ;;
time) shift; time_cmd "$@" ;;
*) die "usage: $0 gen <dir> [streams] [briefs-per-stream] | time <tree> <baseline-bin> <candidate-bin> [runs]" ;;
esac
