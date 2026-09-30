#!/usr/bin/env bash
# Offline, network-free unit test for tools/lint-human-decision-defaults.sh.
#
# Builds a scratch docs/streams tree, one brief per case, and asserts the script's exit
# code and per-file output against it. No gh, no git remote, no network.
#
# The case that matters most (T3) is a REGRESSION test for assay#1679 F1: a comment-only
# `## Human decision` section under `decision-trigger: start` must be reported as a
# violation, exactly as `tools/decision-issue.sh` refuses it (exit 5) at that trigger — the
# placeholder exemption applies ONLY under `decision-trigger: spec`. Point LINT_IMPL at the
# pre-fix script to see T3 (and only T3) go RED:
#   git show <pre-fix-sha>:tools/lint-human-decision-defaults.sh > /tmp/old-lint.sh
#   LINT_IMPL=/tmp/old-lint.sh ./tools/lint-human-decision-defaults_test.sh   # RED on T3
#   ./tools/lint-human-decision-defaults_test.sh                              # green

set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMPL="${LINT_IMPL:-$here/lint-human-decision-defaults.sh}"
case "$IMPL" in /*) ;; *) IMPL="$here/$IMPL" ;; esac

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); printf '     %s\n' "$2"; }

ROOT="$(mktemp -d "${TMPDIR:-/tmp}/lint-hdd-test-XXXXXX")"
trap 'rm -rf "$ROOT"' EXIT
mkdir -p "$ROOT/docs/streams/t"

write_brief() { # write_brief <name> <decision-trigger-line-or-empty> <section-body>
	local name=$1 trig=$2 body=$3
	{
		printf -- '---\n'
		printf 'brief: assay:test:t:1\n'
		printf 'title: fixture\n'
		printf 'gate: human\n'
		[ -n "$trig" ] && printf '%s\n' "$trig"
		printf 'schema: brief-v2\n'
		printf -- '---\n\n'
		printf '# fixture\n\n## Human decision\n%s\n\n## Ground rules\n- n/a\n' "$body"
	} >"$ROOT/docs/streams/t/brief-$name.md"
}

run() { # run <root> -> sets OUT, RC
	OUT=$("$IMPL" "$1" 2>&1); RC=$?
}

# T1 — spec trigger, comment-only section: exempt (placeholder), clean.
write_brief "01-spec-comment" "decision-trigger: spec" \
	"<!-- the executor authors this at pickup -->"
run "$ROOT"
if [ "$RC" -eq 0 ] && ! printf '%s' "$OUT" | grep -q '01-spec-comment'; then
	ok "T1 spec-trigger comment-only section is exempt"
else
	bad "T1 spec-trigger comment-only section is exempt" "rc=$RC out=$OUT"
fi
rm -f "$ROOT/docs/streams/t/brief-01-spec-comment.md"

# T2 — spec trigger, marker-line-only section: exempt (placeholder), clean.
write_brief "02-spec-marker" "decision-trigger: spec" \
	"Decision-trigger: spec. The executor must prepare this section at pickup."
run "$ROOT"
if [ "$RC" -eq 0 ]; then
	ok "T2 spec-trigger marker-line section is exempt"
else
	bad "T2 spec-trigger marker-line section is exempt" "rc=$RC out=$OUT"
fi
rm -f "$ROOT/docs/streams/t/brief-02-spec-marker.md"

# T3 — REGRESSION (assay#1679 F1): start trigger, comment-only section: NOT exempt.
# decision-issue.sh never consults is_placeholder_section outside decision-trigger: spec,
# so a start-trigger brief with only a comment in its Human decision section is refused
# (non-empty, no parseable default) — the lint must flag it too.
write_brief "03-start-comment" "decision-trigger: start" \
	"<!-- the executor authors this at pickup -->"
run "$ROOT"
if [ "$RC" -eq 1 ] && printf '%s' "$OUT" | grep -q '03-start-comment'; then
	ok "T3 start-trigger comment-only section is a violation (F1 regression)"
else
	bad "T3 start-trigger comment-only section is a violation (F1 regression)" "rc=$RC out=$OUT"
fi
rm -f "$ROOT/docs/streams/t/brief-03-start-comment.md"

# T4 — absent decision-trigger (defaults to start), comment-only section: NOT exempt.
write_brief "04-absent-comment" "" \
	"<!-- the executor authors this at pickup -->"
run "$ROOT"
if [ "$RC" -eq 1 ] && printf '%s' "$OUT" | grep -q '04-absent-comment'; then
	ok "T4 absent-trigger (default start) comment-only section is a violation"
else
	bad "T4 absent-trigger (default start) comment-only section is a violation" "rc=$RC out=$OUT"
fi
rm -f "$ROOT/docs/streams/t/brief-04-absent-comment.md"

# T5 — start trigger, literal blocking default: parseable, clean.
write_brief "05-start-literal" "decision-trigger: start" \
	$'Options:\n1. a\n2. b\n\nDefault if no answer: none — blocks until answered.'
run "$ROOT"
if [ "$RC" -eq 0 ]; then
	ok "T5 literal blocking default is parseable"
else
	bad "T5 literal blocking default is parseable" "rc=$RC out=$OUT"
fi
rm -f "$ROOT/docs/streams/t/brief-05-start-literal.md"

# T6 — start trigger, dated default: parseable, clean.
write_brief "06-start-dated" "decision-trigger: start" \
	$'Options:\n1. a\n2. b\n\nDefault if no answer: option 1 after 2026-12-01.'
run "$ROOT"
if [ "$RC" -eq 0 ]; then
	ok "T6 dated default is parseable"
else
	bad "T6 dated default is parseable" "rc=$RC out=$OUT"
fi
rm -f "$ROOT/docs/streams/t/brief-06-start-dated.md"

# T7 — non-human gate: never inspected, clean regardless of section shape.
{
	printf -- '---\nbrief: assay:test:t:1\ntitle: fixture\ngate: model\nschema: brief-v2\n---\n\n'
	printf '# fixture\n\n## Human decision\n<!-- not a real gate -->\n\n## Ground rules\n- n/a\n'
} >"$ROOT/docs/streams/t/brief-07-non-human.md"
run "$ROOT"
if [ "$RC" -eq 0 ]; then
	ok "T7 non-human gate is never inspected"
else
	bad "T7 non-human gate is never inspected" "rc=$RC out=$OUT"
fi
rm -f "$ROOT/docs/streams/t/brief-07-non-human.md"

# T8 — no docs/streams directory at all: refused, exit 2.
EMPTY="$(mktemp -d "${TMPDIR:-/tmp}/lint-hdd-empty-XXXXXX")"
OUT=$("$IMPL" "$EMPTY" 2>&1); RC=$?
rm -rf "$EMPTY"
if [ "$RC" -eq 2 ]; then
	ok "T8 missing docs/streams refuses with exit 2"
else
	bad "T8 missing docs/streams refuses with exit 2" "rc=$RC out=$OUT"
fi

echo
echo "pass=$pass fail=$fail (impl: $IMPL)"
[ "$fail" -eq 0 ]
