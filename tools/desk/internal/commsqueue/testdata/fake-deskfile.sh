#!/bin/sh
# fake-deskfile.sh — a stand-in for `deskfile new` that enforces the argv
# contract cmd/deskfile's cmdNew enforces, and nothing else (no forge, no
# token, no dedupe): -R (not --repo), --body-file (no inline body), a
# roster-bound --raised-by role (the roster's role names, never a loop name),
# and the blocker-evidence gate for a "help wanted" filing. Any other flag is
# refused the way flag.Parse refuses it. On success it records what it was
# handed in $FAKE_DESKFILE_LOG.
refuse() { echo "refused: $*" >&2; exit 5; }
[ "$1" = new ] || refuse "unknown subcommand: $1"
shift
repo= title= bodyfile= raised= labels=
while [ $# -gt 0 ]; do
	case "$1" in
	-R) repo=$2 ;;
	--title) title=$2 ;;
	--body-file) bodyfile=$2 ;;
	--label) labels="$labels|$2" ;;
	--raised-by) raised=$2 ;;
	*) refuse "bad flags: flag provided but not defined: $1" ;;
	esac
	[ $# -ge 2 ] || refuse "bad flags: flag needs an argument: $1"
	shift 2
done
[ -n "$repo" ] || refuse "-R <repo> is required"
[ -n "$title" ] || refuse "--title is required"
[ -n "$bodyfile" ] || refuse "--body-file is required (no stdin/inline body)"
[ -r "$bodyfile" ] || refuse "--body-file $bodyfile is not readable"
case "$raised" in
"" | worker | verifier) ;;
*) refuse "--raised-by $raised is not a roster-bound role" ;;
esac
case "$labels" in
*"|help wanted"*)
	awk '/^#+[ \t]*[Ee]vidence/ { h = 1; next } h && /^[ \t]*```/ { f = 1 } END { exit !f }' "$bodyfile" ||
		refuse "a filing labelled \"help wanted\" must carry a ### Evidence section with a fenced block"
	;;
esac
{
	printf 'repo=%s\ntitle=%s\nraised-by=%s\nlabels=%s\n--- body ---\n' "$repo" "$title" "$raised" "$labels"
	cat "$bodyfile"
} >"$FAKE_DESKFILE_LOG"
