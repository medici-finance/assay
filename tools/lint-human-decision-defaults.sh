#!/usr/bin/env bash
# lint-human-decision-defaults.sh — repo-wide guard for the R-3 mandatory-default gate
# (assay#1673).
#
# WHY THIS EXISTS. `tools/decision-issue.sh` is the consumer-side decision-gate script
# `deskdispatch --gate-human` shells out to (tools/desk/cmd/deskdispatch/dispatch.go's
# decisionScriptRel = "tools/decision-issue.sh") — it refuses (exit 5) to file a decision
# issue for a `gate: human` brief whose `## Human decision` section has no PARSEABLE
# default line: R-3 requires the AUTHOR to write either a dated default (`Default if no
# answer: <text> after <YYYY-MM-DD>`) or the explicit blocking literal (`Default if no
# answer: none — blocks until answered`); the tool must never invent one on the author's
# behalf. This repo does not carry its own copy of that script (it is a consumer-supplied
# contract deskdispatch wraps, never re-implements — see tools/desk/cmd/deskdispatch/main.go
# "WRAP, NEVER RE-IMPLEMENT"), so nothing in this tree previously walked the whole brief
# corpus in one pass and reported every non-placeholder offender at once — the refusal only
# ever surfaced one brief at a time, at first dispatch. This script closes the DEFECT CLASS:
# it scans every brief in the tree and reports every non-placeholder `## Human decision`
# section with an unparseable default, so the class can be swept before dispatch-time
# discovery rather than one refusal per session.
#
# The extraction and grammar here are DELIBERATELY kept identical to (and in sync with)
# `tools/decision-issue.sh`'s own `DECISION_BLOCK` awk, `is_placeholder_section` and
# `has_parseable_default` — decision-issue.sh is a single-brief, network-touching workflow
# tool (its `ensure` verb dedupe-checks against the live issue tracker) and is not
# structured to be sourced as a library, so the small (~20-line) grammar is duplicated here
# rather than exec'd once per brief across the whole tree. The same duplication pattern
# (and this script's name) was already used for the same defect class in the sibling house
# tooling repo's own brief corpus — ported here rather than reinvented.
#
# Usage:
#   tools/lint-human-decision-defaults.sh [ROOT]
#     ROOT   directory to scan for docs/streams/**/brief-*.md (default: repo root, i.e.
#            the directory this script's parent lives in).
#
# Exit codes: 0 — clean (or no brief carries an authored, non-placeholder section).
#             1 — at least one brief has an unparseable default; every offender is
#                 printed, one per line, to stdout.
#
# NOT wired into CI as a required check here — this run is the first pass against this
# tree; wiring it in as a required gate is named as follow-up in the PR body rather than
# silently left undone.

set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=${1:-$(cd "$HERE/.." && pwd)}

[ -d "$ROOT/docs/streams" ] || {
	printf 'lint-human-decision-defaults: refused: %s/docs/streams not found\n' "$ROOT" >&2
	exit 2
}

# decision_block <file> — mirrors decision-issue.sh's DECISION_BLOCK extraction: from
# `## Human decision` to the next `## ` heading, blank-trimmed top and bottom.
decision_block() {
	local block
	block=$(awk '
		/^## Human decision/ { inblk = 1; next }
		inblk && /^## /      { exit }
		inblk                { print }' "$1")
	printf '%s\n' "$block" | awk 'NF{f=1} f' | awk '{a[NR]=$0} END{last=NR; while(last>0 && a[last] ~ /^[[:space:]]*$/) last--; for(i=1;i<=last;i++) print a[i]}'
}

# is_placeholder_section <block> — mirrors decision-issue.sh: a comment-only section, or a
# single `Decision-trigger: spec` marker line with no Default line, is not yet an authored
# decision and carries no default obligation.
is_placeholder_section() {
	local residue
	residue=$(printf '%s\n' "$1" | awk '
		{ buf = buf $0 "\n" }
		END {
			s = buf; out = ""
			while ((i = index(s, "<!--")) > 0) {
				out = out substr(s, 1, i - 1)
				rest = substr(s, i + 4)
				j = index(rest, "-->")
				if (j == 0) exit 3
				s = substr(rest, j + 3)
			}
			printf "%s", out s
		}') || return 1
	residue=$(printf '%s\n' "$residue" | grep -v '^[[:space:]]*$' || true)
	[ -z "$residue" ] && return 0
	[ "$(printf '%s\n' "$residue" | wc -l | tr -d ' ')" -eq 1 ] || return 1
	printf '%s\n' "$residue" | grep -q 'Default if no answer:' && return 1
	printf '%s\n' "$residue" | grep -qiE '^[[:space:]]*decision-trigger:[[:space:]]*spec([^[:alnum:]_-]|$)'
}

# gate_human <file> — reads the frontmatter `gate:` scalar (tolerant of a quoted value).
gate_human() {
	local fm gate
	fm=$(awk 'NR==1 && $0!="---"{exit} NR==1{next} /^---[[:space:]]*$/{exit} {print}' "$1")
	gate=$(printf '%s\n' "$fm" | awk -F': *' '/^gate:/{v=$2; gsub(/^"|"$/,"",v); gsub(/[[:space:]]+$/,"",v); print v; exit}')
	[ "$gate" = "human" ]
}

# has_parseable_default <block> — identical grammar to decision-issue.sh's R-3 gate.
has_parseable_default() {
	printf '%s\n' "$1" | grep -qE 'Default if no answer:.*after[[:space:]]+[0-9]{4}-[0-9]{2}-[0-9]{2}' && return 0
	printf '%s\n' "$1" | grep -qF 'Default if no answer: none — blocks until answered' && return 0
	return 1
}

violations=0
while IFS= read -r -d '' f; do
	gate_human "$f" || continue
	block=$(decision_block "$f")
	[ -n "$block" ] || continue
	is_placeholder_section "$block" && continue
	if ! has_parseable_default "$block"; then
		rel=${f#"$ROOT"/}
		printf '%s: has a `## Human decision` section with no parseable default\n' "$rel"
		violations=$((violations + 1))
	fi
done < <(find "$ROOT/docs/streams" -type f -name 'brief-*.md' -print0 | sort -z)

if [ "$violations" -gt 0 ]; then
	printf 'lint-human-decision-defaults: %d brief(s) with an unparseable `## Human decision` default\n' "$violations" >&2
	exit 1
fi

printf 'lint-human-decision-defaults: clean\n'
exit 0
