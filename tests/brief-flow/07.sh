#!/usr/bin/env bash
# Offline acceptance runner for the brief-flow event contract
# (spec/brief-flow-event-v1.md, schemas/brief-flow-event-v1.json,
# statusgen/briefevent.go, statusgen/briefstage.go).
#
#   bash tests/brief-flow/07.sh <mode>
#
# Modes: identity | stage | compatibility | flow | mutation | role-identity | replace-set | all
#
# Each behavioural mode runs its targeted Go tests against the synthetic
# fixtures under statusgen/testdata/brief-flow/contract and requires every
# named test to report PASS. `role-identity` additionally dereferences the
# run-record role list it is compared against. `mutation` copies the module,
# corrupts the reducer or a trust-boundary guard one defect at a time and
# exits 0 only after it has observed, for EVERY mutation, a nonzero exit
# carrying each of that mutation's own ASSERT-FAIL[<id>] markers — and then
# the genuine code passing. `replace-set` is the offline self-test of the copy
# set `mutation` builds each scratch tree from (every local `replace` target of
# the statusgen module, validated and copied under the spelling go.mod keeps).
#
# Exit codes: 0 pass, 1 fail, 2 could-not-check. A missing prerequisite, a
# missing fixture, a missing test or a void mutation anchor never passes.
# Offline by construction: no network, no forge, no credentials.
set -u -o pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd -P)"
pkg="$repo/statusgen"
fixtures="$pkg/testdata/brief-flow/contract"
runrec="$repo/docs/streams/graph-execution/brief-06-run-records-and-replay.md"
timeout="${BRIEFFLOW_TEST_TIMEOUT:-180s}"

cnc() { echo "could-not-check: $*" >&2; exit 2; }
fail() { echo "FAIL: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || cnc "go is not on PATH"
[ -f "$pkg/go.mod" ] || cnc "statusgen module not found at $pkg"
for f in briefevent.go briefstage.go briefevent_test.go; do
	[ -f "$pkg/$f" ] || cnc "statusgen/$f not found"
done
[ -f "$repo/schemas/brief-flow-event-v1.json" ] || cnc "schemas/brief-flow-event-v1.json not found"
for f in events.jsonl receipt.jsonl history.jsonl overlap.jsonl expect.json; do
	[ -f "$fixtures/$f" ] || cnc "fixture $f not found under statusgen/testdata/brief-flow/contract"
done

work="$(mktemp -d "${TMPDIR:-/tmp}/brief-flow-accept.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# build <module-dir> <binary> — compile the package's test binary.
build() {
	( cd "$1" && GOWORK=off go test -c -o "$2" . ) >"$work/build.log" 2>&1
}

bin="$work/statusgen.test"
built=""
build_genuine() {
	[ -n "$built" ] && return 0
	# The module under test failing to build is a defect in it, not a missing
	# prerequisite: fail, never could-not-check.
	build "$pkg" "$bin" || { cat "$work/build.log" >&2; fail "statusgen test binary did not build"; }
	built=1
}

# run_tests <module-dir> <binary> <log> <TestName>... — returns the binary's exit code.
run_tests() {
	local dir="$1" b="$2" log="$3"
	shift 3
	local re
	re="^($(IFS='|'; echo "$*"))\$"
	( cd "$dir" && BRIEFFLOW_FIXTURES="$fixtures" "$b" -test.count=1 -test.timeout "$timeout" -test.v -test.run "$re" ) >"$log" 2>&1
}

# require_pass <log> <TestName>... — every named test reported PASS.
require_pass() {
	local log="$1" t
	shift
	for t in "$@"; do
		grep -q -- "^--- PASS: $t " "$log" || { cat "$log" >&2; fail "$t did not report PASS"; }
	done
}

mode_tests() {
	local name="$1"
	shift
	local log="$work/$name.log" rc
	build_genuine
	run_tests "$pkg" "$bin" "$log" "$@"
	rc=$?
	cat "$log"
	[ "$rc" -eq 0 ] || fail "$name: tests exited $rc"
	require_pass "$log" "$@"
	echo "PASS  $name"
}

# dereference — the run-record role list this contract claims to agree with
# is present where the tests read it.
dereference() {
	[ -f "$runrec" ] || cnc "run-record brief not found: docs/streams/graph-execution/brief-06-run-records-and-replay.md"
	# shellcheck disable=SC2016 # literal backticks
	grep -qF '`desk | reviewer | verifier | worker | human`' "$runrec" \
		|| fail "run-record role list not found in graph-execution/06 — the role vocabulary may have drifted"
	grep -q '"role"' "$repo/schemas/workflow-pattern-v1.json" 2>/dev/null \
		|| cnc "schemas/workflow-pattern-v1.json has no role enum to compare"
	echo "resolved: graph-execution/06 role list; workflow-pattern-v1 role enum"
}

# subst <file> <from> <to> — literal replace; a missing anchor voids the
# mutation and fails the runner.
subst() {
	local f="$1" from="$2" to="$3" body
	body="$(cat "$f"; printf x)"
	body="${body%x}"
	case "$body" in *"$from"*) ;; *) fail "mutation anchor not found in ${f##*/}: $from" ;; esac
	printf '%s' "${body//"$from"/"$to"}" >"$f"
}

# id ^ test ^ expected assertion(s, comma-separated) ^ file ^ from ^ to ^ description
# (`^` separates fields: anchors contain `|`.)
mutations=(
	'alias-dedup^TestBriefEventIdentity^identity-uuid-key^briefstage.go^ev.BriefUUID // identity-key^ev.Alias^briefs keyed by alias instead of uuid'
	'partial-complete^TestBriefEventStage^stage-13^briefstage.go^case merged == len(live):^case merged > 0:^one merged PR of three counts as the whole brief merged'
	'seed-birth^TestBriefEventIdentity^identity-births-03^briefstage.go^p.Seeded = true // seed is not a birth^p.Seeded = true; p.Births++^historian seed counted as a birth'
	'alias-latest^TestBriefEventIdentity^identity-overlap^briefevent.go^uuids[sp.uuid] = true // alias-span-cover^uuids = map[string]bool{sp.uuid: true}^overlapping alias spans resolved to the latest claim'
	'unknown-merge^TestBriefEventStage^stage-20^briefstage.go^case "unknown", "":^case "never":^an unknown-executor merge counted as an app merge'
	'scan-off^TestBriefEventRoles^roles-login,roles-no-persist^briefevent.go^if err := scanPerson(m, ""); err != nil {^if err := scanPerson(nil, ""); err != nil {^person-identifier scan disabled'
	'redact-off^TestBriefEventRoles^roles-no-disclose^briefevent.go^if strings.Contains(msg, "missing required key") || strings.Contains(msg, "wrong type") {^if true {^schema refusals echo the refused value'
	'member-name^TestBriefEventRoles^roles-member-name^briefevent.go^if isPersonKey(k) || emailShaped.MatchString(k) { // person-member-name^if isPersonKey(k) {^member names not email-shape tested'
	'path-name^TestBriefEventRoles^roles-scan-path^briefevent.go^if !declared[k] { // scan-path-position^if false {^refusal path echoes producer member names'
	'revalidate-off^TestBriefEventRoles^roles-export-revalidate^briefevent.go^again, perr := ParseBriefEvent(ev.Canonical)^again, perr := ev, error(nil)^export skips re-validation'
	'export-git-off^TestBriefEventRoles^roles-export-git^briefevent.go^if inside {^if false && inside {^export into a git work tree allowed'
	'symlink-off^TestBriefEventRoles^roles-export-symlink^briefevent.go^rp, err := filepath.EvalSymlinks(prefix) // export-symlink-resolve^rp, err := prefix, error(nil)^export destination judged lexically, symlinks unresolved'
	'export-mode^TestBriefEventRoles^roles-export-mode^briefevent.go^os.MkdirAll(dir, 0o700)^os.MkdirAll(dir, 0o755)^export directory readable beyond its owner'
	'dup-named^TestBriefEventCompat^compat-duplicate^briefevent.go^return nil, errBfeDuplicate^return nil, errors.New("duplicate member " + k)^duplicate refusal names the member'
)

# lexclean <path> — filepath.Clean for a relative path, lexically (the way Go
# joins a replace target onto the go.mod directory). Prints the cleaned path
# ("" for the start itself); returns 1 when the path climbs above its start.
lexclean() {
	local rest="$1" seg out=""
	while [ -n "$rest" ]; do
		case "$rest" in
		*/*) seg="${rest%%/*}"; rest="${rest#*/}" ;;
		*) seg="$rest"; rest="" ;;
		esac
		case "$seg" in
		"" | .) ;;
		..)
			case "$out" in
			"") return 1 ;;
			*/*) out="${out%/*}" ;;
			*) out="" ;;
			esac
			;;
		*) out="${out:+$out/}$seg" ;;
		esac
	done
	printf '%s' "$out"
}

# replace_paths <module-dir> — print, one per line, the target of EVERY
# replacement in the module's go.mod that names no version. Go requires a
# versionless replacement to be a directory path (./x, ../x, /abs/x), so this
# is the whole local set whatever its spelling, not a prefix guess. `go mod
# edit -json` only parses the file. A line starting `ERR:` means the replace
# block did not pair up and the caller must refuse.
replace_paths() {
	local json
	json="$(cd "$1" && GOWORK=off go mod edit -json)" || cnc "go mod edit -json failed on ${1##*/}/go.mod"
	printf '%s\n' "$json" | awk '
		/"Replace": \[/ { inrep = 1; next }
		!inrep { next }
		/"Old": \{/ { mode = "old"; olds++; next }
		/"New": \{/ { mode = "new"; news++; ver = 0; path = ""; next }
		mode == "new" && /"Path":/ { path = $0; sub(/^[^:]*: "/, "", path); sub(/",?[ \t]*$/, "", path) }
		mode == "new" && /"Version":/ { ver = 1 }
		mode == "new" && /^[ \t]*\}/ { if (!ver) print path; mode = "" }
		END { if (olds != news) print "ERR:old/new replacement entries do not pair" }
	'
}

# copy_replace_targets <repo> <module-dir-in-repo> <scratch-root> — copy every
# local replace target of <repo>/<module-dir>/go.mod into <scratch-root> at the
# SAME repo-relative path go.mod spells, so the scratch go.mod resolves each
# reference as it does in the repo. A target reached through an in-repo symlink
# lands as a real directory under the alias name go.mod uses. Anything the
# scratch tree cannot reproduce is could-not-check (exit 2), never skipped: an
# absolute target, a path that climbs out of the repo or resolves outside it
# (symlinks included), the repo root itself, a missing directory, a backslash
# spelling, and an empty derivation. The repo root is canonicalised, so a
# checkout reached through a symlinked path component is not misjudged.
copy_replace_targets() {
	local repo_p scratch_p paths rel lex abs dst n=0
	repo_p="$(cd "$1" && pwd -P)" || cnc "repository root $1 not found"
	mkdir -p "$3" || cnc "could not create scratch root $3"
	scratch_p="$(cd -P "$3" && pwd -P)" || cnc "scratch root $3 not found"
	paths="$(replace_paths "$repo_p/$2")" || exit $?
	[ -n "$paths" ] || cnc "${2}/go.mod has no local replace targets to copy — the derivation matched nothing"
	while IFS= read -r rel; do
		n=$((n + 1))
		case "$rel" in
		ERR:*) cnc "${2}/go.mod replace block did not parse: ${rel#ERR:}" ;;
		"") cnc "${2}/go.mod has an empty local replace target" ;;
		*\\*) cnc "local replace target $rel uses a backslash spelling — unsupported" ;;
		/*) cnc "absolute local replace target $rel is unsupported — the scratch tree cannot reproduce a path outside the copied repository" ;;
		esac
		lex="$(lexclean "$2/$rel")" || cnc "local replace target $rel is outside the repository"
		[ -n "$lex" ] || cnc "local replace target $rel is the repository root — refusing to copy it"
		abs="$(cd -P "$repo_p/$lex" 2>/dev/null && pwd -P)" || cnc "local replace target $rel does not exist"
		case "$abs" in
		"$repo_p"/*) ;;
		*) cnc "local replace target $rel resolves outside the repository" ;;
		esac
		dst="$scratch_p/$lex"
		mkdir -p "$dst" || cnc "could not create scratch directory for $rel"
		case "$(cd -P "$dst" && pwd -P)" in
		"$scratch_p"/*) ;;
		*) cnc "scratch directory for local replace target $rel escapes the scratch tree" ;;
		esac
		cp -R "$abs/." "$dst/" || cnc "could not copy replace target $rel"
	done <<<"$paths"
	[ "$n" -gt 0 ] || cnc "${2}/go.mod has no local replace targets to copy"
}

mode_mutation() {
	local entry id test wants want want_list file from to desc copy mbin log rc observed=0
	for entry in "${mutations[@]}"; do
		IFS='^' read -r id test wants file from to desc <<<"$entry"
		copy="$work/mut-$id"
		mkdir -p "$copy/docs/streams/graph-execution"
		cp -R "$pkg" "$copy/statusgen" || cnc "could not copy the module"
		cp -R "$repo/schemas" "$copy/schemas" || cnc "could not copy schemas"
		# Every local replace target, at the path go.mod spells it; a target the
		# copy cannot reproduce refuses here, before any mutant is compiled.
		copy_replace_targets "$repo" statusgen "$copy"
		cp "$runrec" "$copy/docs/streams/graph-execution/" 2>/dev/null || true
		subst "$copy/statusgen/$file" "$from" "$to"
		mbin="$work/mut-$id.test"
		build "$copy/statusgen" "$mbin" || { cat "$work/build.log" >&2; fail "mutation $id did not compile — a build failure is not a detection"; }
		log="$work/mut-$id.log"
		run_tests "$copy/statusgen" "$mbin" "$log" "$test"
		rc=$?
		if [ "$rc" -eq 0 ]; then
			cat "$log" >&2
			fail "mutation $id ($desc): $test exited 0 — the defect was not detected"
		fi
		echo "RED   $id: $test exit $rc — $desc"
		IFS=',' read -r -a want_list <<<"$wants"
		for want in "${want_list[@]}"; do
			if ! grep -qF "ASSERT-FAIL[$want]" "$log"; then
				cat "$log" >&2
				fail "mutation $id: exit $rc but no ASSERT-FAIL[$want]"
			fi
			grep -F "ASSERT-FAIL[$want]" "$log" | head -n 1 | sed 's/^ */        /'
		done
		observed=$((observed + 1))
		rm -rf "$copy" "$mbin"
	done
	[ "$observed" -eq "${#mutations[@]}" ] || fail "observed $observed of ${#mutations[@]} mutations"
	mode_tests genuine TestBriefEventIdentity TestBriefEventStage TestBriefEventCompat TestBriefEventRoles
	echo "PASS  mutation: $observed of ${#mutations[@]} mutations detected; genuine code passes"
}

# mode_replace_set — offline self-test of copy_replace_targets against
# synthetic repos under the scratch root (no statusgen sources involved).
rs_n=0
rs_repo="" rs_out="" rs_err=""
# rs_new <name> <go.mod body> — a fresh synthetic repo whose statusgen/go.mod is
# the body; modules good/, other/ and "my mod/" exist beside it.
rs_new() {
	rs_n=$((rs_n + 1))
	local base="$work/rs/$rs_n"
	rs_repo="$base/repo" rs_out="$base/out" rs_err="$base/err"
	mkdir -p "$rs_repo/statusgen" "$rs_repo/good" "$rs_repo/other" "$rs_repo/my mod" "$base/outside"
	local m
	for m in "$rs_repo/good" "$rs_repo/other" "$rs_repo/my mod" "$base/outside"; do
		printf 'module example.com/%s\n\ngo 1.21\n' "$(basename "$m" | tr -d ' ')" >"$m/go.mod"
		echo "marker $(basename "$m")" >"$m/marker.txt"
	done
	printf 'module example.com/statusgen\n\ngo 1.21\n\n%s\n' "$2" >"$rs_repo/statusgen/go.mod"
	echo "[$1]"
}
# rs_run <repo-path> — run the copier the way mode_mutation does; sets rs_rc.
rs_run() {
	mkdir -p "$rs_out"
	cp -R "$rs_repo/statusgen" "$rs_out/statusgen"
	( copy_replace_targets "$1" statusgen "$rs_out" ) >/dev/null 2>"$rs_err"
	rs_rc=$?
}
rs_ok() {
	[ "$rs_rc" -eq 0 ] || { cat "$rs_err" >&2; fail "replace-set $1: expected exit 0, got $rs_rc"; }
}
rs_refused() {
	local what="$1" want="$2"
	[ "$rs_rc" -eq 2 ] || { cat "$rs_err" >&2; fail "replace-set $what: expected could-not-check (exit 2), got $rs_rc"; }
	grep -qF -- "$want" "$rs_err" || { cat "$rs_err" >&2; fail "replace-set $what: refusal does not say '$want'"; }
}
rs_has() { [ -f "$rs_out/$2" ] || fail "replace-set $1: scratch tree lacks $2"; }
rs_lacks() { [ ! -e "$rs_out/$2" ] || fail "replace-set $1: scratch tree holds unexpected $2"; }

mode_replace_set() {
	local outside_abs

	rs_new direct 'replace example.com/good => ../good
replace example.com/other => ../other'
	rs_run "$rs_repo"
	rs_ok direct
	rs_has direct good/marker.txt; rs_has direct other/marker.txt
	rs_lacks direct "my mod"

	rs_new alias-symlink 'require example.com/alias v0.0.0
replace example.com/alias => ../alias'
	ln -s good "$rs_repo/alias"
	rs_run "$rs_repo"
	rs_ok alias-symlink
	# The scratch go.mod still says ../alias; that path must resolve there.
	rs_has alias-symlink alias/go.mod; rs_has alias-symlink alias/marker.txt
	( cd "$rs_out/statusgen" && GOWORK=off GOTOOLCHAIN=local GOFLAGS=-mod=mod GOPROXY=off go list -m example.com/alias ) >"$work/rs/alias.out" 2>&1 \
		|| { cat "$work/rs/alias.out" >&2; fail "replace-set alias-symlink: the scratch module does not resolve ../alias"; }

	rs_new mixed-absolute-outside 'replace example.com/good => ../good'
	outside_abs="$(cd "$(dirname "$rs_repo")/outside" && pwd -P)"
	printf 'replace example.com/outside => %s\n' "$outside_abs" >>"$rs_repo/statusgen/go.mod"
	rs_run "$rs_repo"
	rs_refused mixed-absolute-outside "absolute local replace target"

	rs_new absolute-missing 'replace example.com/good => ../good
replace example.com/gone => /nonexistent/replace-set-target'
	rs_run "$rs_repo"
	rs_refused absolute-missing "absolute local replace target"

	rs_new absolute-inside 'replace example.com/good => ../good'
	printf 'replace example.com/other => %s\n' "$(cd "$rs_repo/other" && pwd -P)" >>"$rs_repo/statusgen/go.mod"
	rs_run "$rs_repo"
	rs_refused absolute-inside "absolute local replace target"

	rs_new missing-relative 'replace example.com/good => ../good
replace example.com/gone => ../gone'
	rs_run "$rs_repo"
	rs_refused missing-relative "does not exist"

	rs_new relative-outside 'replace example.com/good => ../good
replace example.com/out => ../../outside'
	rs_run "$rs_repo"
	rs_refused relative-outside "outside the repository"

	rs_new roundabout-outside 'replace example.com/out => ../good/../../outside'
	rs_run "$rs_repo"
	rs_refused roundabout-outside "outside the repository"

	rs_new symlink-escape 'replace example.com/esc => ../esc'
	ln -s ../outside "$rs_repo/esc"
	rs_run "$rs_repo"
	rs_refused symlink-escape "resolves outside the repository"

	rs_new repo-root 'replace example.com/good => ../good
replace example.com/root => ..'
	rs_run "$rs_repo"
	rs_refused repo-root "repository root"

	rs_new no-local-replace 'replace example.com/x => example.com/y v1.2.3'
	rs_run "$rs_repo"
	rs_refused no-local-replace "no local replace targets"

	rs_new versioned-ignored 'replace example.com/x => example.com/y v1.2.3
replace example.com/good => ../good'
	rs_run "$rs_repo"
	rs_ok versioned-ignored
	rs_has versioned-ignored good/marker.txt; rs_lacks versioned-ignored other

	rs_new spaces 'replace example.com/mymod => "../my mod"'
	rs_run "$rs_repo"
	rs_ok spaces
	rs_has spaces "my mod/marker.txt"

	rs_new nested-overlap 'replace example.com/good => ../good
replace example.com/in => ../good/inner'
	mkdir -p "$rs_repo/good/inner"
	printf 'module example.com/in\n\ngo 1.21\n' >"$rs_repo/good/inner/go.mod"
	rs_run "$rs_repo"
	rs_ok nested-overlap
	rs_has nested-overlap good/marker.txt; rs_has nested-overlap good/inner/go.mod
	rs_lacks nested-overlap good/inner/inner

	rs_new symlinked-root 'replace example.com/good => ../good'
	ln -s repo "$(dirname "$rs_repo")/repo-link"
	rs_run "$(dirname "$rs_repo")/repo-link"
	rs_ok symlinked-root
	rs_has symlinked-root good/marker.txt

	echo "PASS  replace-set: $rs_n self-test cases"
}

mode="${1:-}"
case "$mode" in
identity) mode_tests identity TestBriefEventIdentity ;;
stage) mode_tests stage TestBriefEventStage ;;
compatibility) mode_tests compatibility TestBriefEventCompat TestBriefEventSchemaParity ;;
flow) mode_tests flow TestBriefEventFlow ;;
role-identity) dereference && mode_tests role-identity TestBriefEventRoles ;;
mutation) mode_mutation ;;
replace-set) mode_replace_set ;;
all)
	mode_tests identity TestBriefEventIdentity
	mode_tests stage TestBriefEventStage
	mode_tests compatibility TestBriefEventCompat TestBriefEventSchemaParity
	mode_tests flow TestBriefEventFlow
	dereference
	mode_tests role-identity TestBriefEventRoles
	mode_replace_set
	mode_mutation
	;;
*) echo "usage: $0 identity|stage|compatibility|flow|mutation|role-identity|replace-set|all" >&2; exit 2 ;;
esac
