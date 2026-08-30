#!/usr/bin/env bash
# assay-inbox.sh — the "inbox": a derived gh query, not a stored roll-up.
#
# Queries open issues across the configured repos for the escalation-contract
# labels, merges the results, sorts urgency-then-age, and renders them in one of three
# forms — all three off ONE ordering and ONE format builder, so they cannot drift:
#
#   (default)          the terminal table (one row per item)
#   --walk [--item k]  ONE item in the five-part decision format (see below)
#   --html <out>       the whole queue as cards in that same format, one self-contained file
#
# And ONE further rendering, of a different question — "how is the system performing":
#
#   --flow             the pipeline FLOW model as a terminal table
#   --flow --html <o>  the same model as a left-to-right inline-SVG stage diagram
#
# The flow model is DERIVED, never probed: every number comes from a JSON emitter that
# already exists (`statusgen --bottleneck/--intake-debt/--net-flow --json`, `deskboard
# throughput --json`). This script runs those readers and arranges their output; it opens no
# new source, parses no STATUS.md, and reaches no cluster. A reader that fails leaves its
# stage `could-not-check` — never 0. See "the flow model" below.
#
# The five-part format is the `ask-decision` skill's contract
# (`../skills/ask-decision/SKILL.md`): Header / Context / Options / Reply shape / Verification.
# This script RENDERS it; the skill drives the turn-taking, records the ruling, and moves the
# label. `--walk` is deliberately NON-interactive — it prints one item and exits, so the agent
# holding the conversation owns the turn boundary and nothing here ever blocks on a tty.
#
# Read-only: shells `gh issue list` (and, in --walk/--html only, `gh issue view` for the body
# and comments that Context/Options are derived from). Never writes to any issue.
# Terminal-agnostic — works with plain git + gh + any terminal.
#
# Repo resolution order:
#   1. repo args on the command line (owner/name ...)
#   2. ./.assay/repos.txt (one owner/name per line; blank lines and #-comments ignored)
#   3. the current repo's `origin` remote, parsed to owner/name
#
# Exit codes (a queue tool that cannot distinguish "nothing waiting" from "the query
# died" is worse than no tool at all — a review found this early on):
#   0  every query succeeded (the printed table is complete)
#   1  precondition failure — gh/jq missing, or no repos resolvable
#   2  at least one gh query FAILED; the table, if any, is PARTIAL
#
# PORTABILITY: bash 3.2 (stock /bin/bash on macOS) is a supported target — the brief's
# premise is "plain git + gh + any terminal". No bash-4-only builtins (mapfile/readarray,
# associative arrays, ${var^^}). assay-inbox.test.sh pins this.
set -euo pipefail

# The four escalation label tokens this inbox folds into one view.
#
# PROVENANCE, precisely (an earlier review left this unchecked; diffed since):
# the escalation contract creates and owns exactly ONE of these — `needs-decision`, whose
# spelling matches it verbatim. The other three are stock GitHub labels folded into the same
# inbox; the contract does not define them, so do not "re-sync" them against it and find them
# missing.
#
# These are the interface seam — keep `needs-decision` in exact sync with the escalation
# contract; do not add/reorder the set without re-reading that contract.
LABELS=("urgent" "needs-decision" "question" "help wanted")

# `gh issue list` defaults to --limit 30 and keeps the THIRTY NEWEST, discarding everything
# older. This tool's whole ordering is oldest-first-within-rank, so the default cap discards
# precisely the items it exists to surface (caught in review). Query well
# above any realistic per-label ceiling, and shout if we ever hit it.
LIMIT="${ASSAY_INBOX_LIMIT:-500}"

usage() {
  cat <<'EOF'
Usage: assay-inbox.sh [--walk [--item K] | --html OUT.html] [owner/repo ...]
       assay-inbox.sh --flow [--html OUT.html] [--root PATH ...] [--since YYYY-MM-DD]

Prints open issues across the given repos (or ./.assay/repos.txt, or the current repo's
origin remote) carrying any of the escalation-contract labels: urgent, needs-decision,
question, help wanted. Sorted urgency-then-age (most urgent, then oldest, first).

Modes (the first three share one ordering and one format builder):
  (none)            the terminal table — one row per item.
  --walk            print ONE item in the five-part decision format: Header / Context /
                    Options / Reply shape / Verification. Prints item 1 and exits.
  --item K          with --walk (and implying it): print item K instead of item 1.
                    1-based; out of range is an error, never a silent empty.
  --html OUT.html   write the whole queue to OUT.html as cards in that same format —
                    one self-contained file: inline CSS, no scripts, no external assets,
                    light/dark via prefers-color-scheme. The only URLs are the issue links.
                    The page also carries the Flow section described below.
  --flow            print the pipeline FLOW model as a terminal table and exit: per stage,
                    the count now, the loop's own queue depth against its pool slots, the
                    ratio, the dwell, and the bottleneck. Fleet total always; one row block
                    per cell when more than one cell resolves.
  --flow --html O   write that model to O as a left-to-right inline-SVG stage diagram —
                    the flow section ALONE, same file rules as above.

Cell (statusgen root) resolution for --flow, in order:
  1. --root PATH        repeatable; each path is one cell.
  2. ./.assay/cells.txt one cell per line as `NAME<space or tab>PATH` (or just PATH).
                        Blank lines and #-comments ignored. This is the inbox's flat mirror
                        of cells.yaml's name/streams_root pairs, exactly as .assay/repos.txt
                        is its flat mirror of the repo roster.
  3. `.`                the current directory, as a single cell.

--walk is NON-interactive by design: it never prompts and never blocks on a tty. The
turn-taking (one decision per turn, record the ruling, move to the next) belongs to the
`ask-decision` skill, which shells this with an incrementing --item.

Read-only — this never writes to any issue, and --flow writes no file but OUT.html.

Environment:
  ASSAY_INBOX_LIMIT   per-repo-per-label fetch cap passed to `gh issue list` (default 500).
  ASSAY_STATUSGEN     statusgen binary for --flow (default: `statusgen` on PATH).
  ASSAY_DESKBOARD     deskboard binary for --flow (default: `deskboard` on PATH).

Exit codes:
  0  all queries succeeded          1  gh/jq missing, no repos resolvable, or bad arguments
  2  one or more queries FAILED — the output is partial; see stderr. In --flow and
     --flow --html this includes any flow reader that could not be read. On the OTHER modes
     the exit code stays a statement about the DECISION queue: a blind flow section on the
     --html page is reported in the summary line and on the page, and does not redden the
     run, because a caller checking this code is asking whether it saw all the decisions.
EOF
}

# ------------------------------------------------------------------ arguments --
# Everything that is not a flag is a repo token. Parsed before anything else runs so a
# typo'd flag is refused up front rather than queried as a repo name.
MODE="table"
WALK_ITEM=1
HTML_OUT=""
WANT_WALK=0
WANT_HTML=0
WANT_FLOW=0
FLOW_SINCE=""
ROOT_ARGS=()
ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --walk) WANT_WALK=1; shift ;;
    --flow) WANT_FLOW=1; shift ;;
    --item)
      [[ $# -ge 2 ]] || { echo "assay-inbox: --item needs a 1-based item number" >&2; exit 1; }
      WALK_ITEM="$2"; WANT_WALK=1; shift 2 ;;
    --root)
      [[ $# -ge 2 ]] || { echo "assay-inbox: --root needs a path" >&2; exit 1; }
      ROOT_ARGS+=("$2"); shift 2 ;;
    --since)
      [[ $# -ge 2 ]] || { echo "assay-inbox: --since needs a YYYY-MM-DD date" >&2; exit 1; }
      FLOW_SINCE="$2"; shift 2 ;;
    --html)
      [[ $# -ge 2 ]] || { echo "assay-inbox: --html needs an output path" >&2; exit 1; }
      WANT_HTML=1; HTML_OUT="$2"; shift 2 ;;
    --) shift; while [[ $# -gt 0 ]]; do ARGS+=("$1"); shift; done ;;
    -*) echo "assay-inbox: unknown option '$1'" >&2; usage >&2; exit 1 ;;
    *) ARGS+=("$1"); shift ;;
  esac
done

# --walk and --html are separate renderings of the same queue; asking for both in one run is
# ambiguous about what stdout should carry, so refuse rather than silently pick one.
if [[ "$WANT_WALK" -eq 1 && "$WANT_HTML" -eq 1 ]]; then
  echo "assay-inbox: --walk and --html are alternative renderings; pass one, not both" >&2
  exit 1
fi
# --flow answers a DIFFERENT question from --walk (how is the system performing, vs which
# decision is next). Combining them is ambiguous about what stdout carries, so it is refused
# on the same grounds — and for the same reason it is NOT refused with --html, where the flow
# section and the decision cards are two sections of one page.
if [[ "$WANT_WALK" -eq 1 && "$WANT_FLOW" -eq 1 ]]; then
  echo "assay-inbox: --walk and --flow answer different questions; pass one, not both" >&2
  exit 1
fi
if [[ "$WANT_WALK" -eq 1 ]]; then MODE="walk"; fi
if [[ "$WANT_HTML" -eq 1 ]]; then MODE="html"; fi
if [[ "$WANT_FLOW" -eq 1 ]]; then
  MODE="flow"
  [[ "$WANT_HTML" -eq 1 ]] && MODE="flowhtml"
fi

if [[ ! "$WALK_ITEM" =~ ^[1-9][0-9]*$ ]]; then
  echo "assay-inbox: --item must be a positive integer (got '$WALK_ITEM')" >&2
  exit 1
fi

# The date reaches a subprocess argv, so it is validated here rather than trusted downstream —
# same rule as the repo tokens below.
if [[ -n "$FLOW_SINCE" && ! "$FLOW_SINCE" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
  echo "assay-inbox: --since must be YYYY-MM-DD (got '$FLOW_SINCE')" >&2
  exit 1
fi

resolve_repos() {
  if [[ $# -gt 0 ]]; then
    printf '%s\n' "$@"
    return
  fi

  if [[ -f "./.assay/repos.txt" ]]; then
    grep -vE '^[[:space:]]*(#|$)' "./.assay/repos.txt"
    return
  fi

  local origin_url owner_name
  origin_url=$(git remote get-url origin 2>/dev/null || true)
  if [[ -z "$origin_url" ]]; then
    # Runs in a process-substitution subshell — `exit` here only ends the subshell, so just
    # emit nothing; the caller detects an empty REPOS array and reports + exits.
    return
  fi
  owner_name=$(printf '%s' "$origin_url" \
    | sed -E 's#^git@github\.com:##; s#^https://github\.com/##; s#\.git$##')
  printf '%s\n' "$owner_name"
}

# resolve_cells — the CELL list for --flow, emitted as `NAME<TAB>PATH` lines.
#
# A cell is one statusgen ROOT: a checkout carrying docs/streams. `.assay/cells.txt` is the
# flat mirror of cells.yaml's name/streams_root pairs, exactly as `.assay/repos.txt` is the
# flat mirror of the repo roster — the inbox reads flat lists and nothing else, so it never
# grows a YAML parser it would then have to keep in step with the schema.
#
# A line may be `NAME PATH`, `NAME<TAB>PATH`, or a bare `PATH` (named by its basename).
resolve_cells() {
  local line name path
  # The `+x` guard is the bash-3.2-safe form: `${#ROOT_ARGS[@]}` on an empty array trips
  # `set -u` there, the same landmine the REPOS check below documents.
  if [[ -n "${ROOT_ARGS[*]+x}" && ${#ROOT_ARGS[@]} -gt 0 ]]; then
    for path in "${ROOT_ARGS[@]}"; do
      printf '%s\t%s\n' "$(cell_name_for "$path")" "$path"
    done
    return
  fi
  if [[ -f "./.assay/cells.txt" ]]; then
    while IFS= read -r line; do
      [[ -z "${line//[[:space:]]/}" ]] && continue
      case "$line" in \#*) continue ;; esac
      name=$(printf '%s' "$line" | awk '{print $1}')
      path=$(printf '%s' "$line" | awk '{ $1=""; sub(/^[[:space:]]+/, ""); print }')
      if [[ -z "$path" ]]; then path="$name"; name=$(basename "$path"); fi
      printf '%s\t%s\n' "$name" "$path"
    done < "./.assay/cells.txt"
    return
  fi
  printf '%s\t%s\n' "$(cell_name_for .)" "."
}

# cell_name_for <path> — a cell's display name: its origin repo where the path is a checkout
# (the name the operator already uses for it), else the directory basename. A worktree
# basename like `tracker-<item>` names the WORKTREE, not the cell, so the remote wins.
cell_name_for() {
  local url
  url=$(git -C "$1" remote get-url origin 2>/dev/null || true)
  if [[ -n "$url" ]]; then
    printf '%s' "$url" | sed -E 's#^git@github\.com:##; s#^https://github\.com(:[0-9]+)?/##; s#\.git$##'
    return
  fi
  basename "$(cd "$1" 2>/dev/null && pwd || printf '%s' "$1")"
}

command -v gh >/dev/null 2>&1 || { echo "assay-inbox: gh CLI not found" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "assay-inbox: jq not found" >&2; exit 1; }

# --flow reads no issues, so it resolves cells instead of repos. Requiring an origin remote
# (or a repos.txt) for a question about pipeline depth would refuse the tool in exactly the
# checkout where it is most useful — a streams root that is not itself a GitHub repo.
CELL_NAMES=()
CELL_PATHS=()
REPOS=()
if [[ "$MODE" == "flow" || "$MODE" == "flowhtml" ]]; then
  while IFS=$'\t' read -r _cname _cpath; do
    [[ -z "$_cpath" ]] && continue
    CELL_NAMES+=("$_cname")
    CELL_PATHS+=("$_cpath")
  done < <(resolve_cells)
  if [[ -z "${CELL_PATHS[*]+x}" || ${#CELL_PATHS[@]} -eq 0 ]]; then
    echo "assay-inbox: no cells to read (no --root, no ./.assay/cells.txt)" >&2
    exit 1
  fi
else
  # bash 3.2 has no `mapfile`; a `while read` fed by process substitution (NOT a pipe) keeps
  # the appends in the current shell, so REPOS survives the loop.
  while IFS= read -r _line; do
    [[ -z "${_line//[[:space:]]/}" ]] && continue
    REPOS+=("$_line")
  done < <(resolve_repos ${ARGS[@]+"${ARGS[@]}"})

  # `${#REPOS[@]}` on an empty array trips `set -u` on bash < 4.4; the +x guard is the
  # portable form.
  if [[ -z "${REPOS[*]+x}" || ${#REPOS[@]} -eq 0 ]]; then
    echo "assay-inbox: no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)" >&2
    exit 1
  fi
fi

# Every repo token is validated before any gh call (a review finding): a metacharacter-laden token must never reach a shell, and garbage from a
# non-GitHub origin remote is clearer rejected here than as a gh failure downstream.
# The charset is GitHub's own owner/name alphabet.
for _repo in ${REPOS[@]+"${REPOS[@]}"}; do
  if [[ ! "$_repo" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]]; then
    echo "assay-inbox: invalid repo '$_repo' (expected owner/name)" >&2
    exit 1
  fi
done

TMP_JSON=$(mktemp)
TMP_ERR=$(mktemp)
TMP_TSV=$(mktemp)
TMP_SORTED=$(mktemp)
TMP_ITEMS=$(mktemp)
TMP_ONE=$(mktemp)
TMP_DETAIL=$(mktemp)
TMP_FMT=$(mktemp)
TMP_WALK=$(mktemp)
TMP_HTML=$(mktemp)
TMP_RAW=$(mktemp)
TMP_FLOW=$(mktemp)
TMP_FLOWFMT=$(mktemp)
trap 'rm -f "$TMP_JSON" "$TMP_ERR" "$TMP_TSV" "$TMP_SORTED" "$TMP_ITEMS" "$TMP_ONE" "$TMP_DETAIL" "$TMP_FMT" "$TMP_WALK" "$TMP_HTML" "$TMP_RAW" "$TMP_FLOW" "$TMP_FLOWFMT"' EXIT
echo "[]" > "$TMP_JSON"
echo "null" > "$TMP_FLOW"

query_failures=0
truncated=0
# Flow readers are counted SEPARATELY from issue queries, because the exit code of the
# decision modes is a statement about the DECISION QUEUE. On `--html` the flow section is
# context beside the cards: a stale `statusgen` that cannot answer `--net-flow` must not make
# a complete, correctly-rendered decision page report itself as an incomplete inbox — the
# caller contract is that a non-zero exit means "the decisions you are looking at are not all
# of them", and reddening it for a blind sidebar would train callers to ignore it. In `--flow`
# and `--flow --html`, where the flow IS the output, the two counters are folded together and
# a blind reader reddens the run exactly as a failed query does.
flow_failures=0

for repo in ${REPOS[@]+"${REPOS[@]}"}; do
  [[ -z "$repo" ]] && continue
  for label in "${LABELS[@]}"; do
    # `if hits=$(...)` puts the command in a condition context, so `set -e` does not fire and
    # we get to inspect the status ourselves. gh's diagnostics are CAPTURED, not discarded —
    # they are the only thing that tells an operator their token expired.
    if hits=$(gh issue list --repo "$repo" --state open --label "$label" \
        --limit "$LIMIT" --json number,title,labels,createdAt,url 2>"$TMP_ERR"); then
      :
    else
      gh_status=$?
      query_failures=$((query_failures + 1))
      printf 'assay-inbox: QUERY FAILED for %s [%s] (gh exit %s): %s\n' \
        "$repo" "$label" "$gh_status" "$(tr '\n' ' ' <"$TMP_ERR")" >&2
      continue
    fi
    [[ -z "$hits" ]] && hits="[]"

    n=$(printf '%s' "$hits" | jq 'length')
    if [[ "$n" -ge "$LIMIT" ]]; then
      truncated=$((truncated + 1))
      printf 'assay-inbox: WARNING %s [%s] returned %s items == --limit %s; results are TRUNCATED (oldest dropped). Raise ASSAY_INBOX_LIMIT.\n' \
        "$repo" "$label" "$n" "$LIMIT" >&2
    fi

    merged=$(jq -s --arg repo "$repo" \
      '.[0] + (.[1] | map(. + {repo: $repo}))' "$TMP_JSON" <(printf '%s' "$hits"))
    printf '%s' "$merged" > "$TMP_JSON"
  done
done

# Dedupe by repo+number (an issue can carry more than one contract label), rank by the most
# urgent label it carries (lower rank = more urgent), sort urgency-then-age (oldest createdAt
# first within the same rank — waiting longest surfaces first), print a table. urgent-labeled
# items (rank 0) are marked so they're visually first even at a glance.
#
# Display hygiene (a review finding): issue titles and labels are
# attacker-supplied for any repo where a stranger can open an issue, and `@tsv` escapes only
# tab/LF/CR/backslash — so a raw ESC (0x1b) in a title would otherwise reach the operator's
# terminal intact. clean() strips every C0/DEL byte (gsub \p{Cc}); rank is still computed on
# the un-cleaned label names, so cleaning is display-only. Titles are also cut to the
# 60-column field — %-60s pads but never truncates — so an over-long title cannot break the
# layout.
#
# ONE ordering, THREE renderings. The sort lands in $TMP_SORTED and every mode reads from
# there — the table, --walk and --html cannot disagree about which item is #1, which is the
# property the walk mode's "question k of n" depends on.
jq --argjson rankorder '["urgent","needs-decision","question","help wanted"]' '
  unique_by(.repo + "#" + (.number | tostring))
  | map(. + {
      rank: ([ .labels[].name as $n | ($rankorder | index($n)) ] | map(select(. != null)) | min // 99)
    })
  | sort_by([.rank, .createdAt])
' "$TMP_JSON" > "$TMP_SORTED"

jq -r '
  def clean: gsub("\\p{Cc}";"");
  .[]
  | [.rank, ([ .labels[].name | clean ] | join(",")), .repo, ("#" + (.number|tostring)),
     (.title | clean | if length > 57 then .[:57] + "..." else . end), .createdAt, .url]
  | @tsv
' "$TMP_SORTED" > "$TMP_TSV"

item_count=$(jq 'length' "$TMP_SORTED")

render_table() {
  # Read from a FILE, not a pipe: a `while read` on the right of `|` runs in a subshell, so
  # any count accumulated inside it would be lost.
  while IFS=$'\t' read -r rank labelnames repo number title createdat url; do
    now_epoch=$(date -u +%s)
    created_epoch=$(date -u -d "$createdat" +%s 2>/dev/null || date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$createdat" +%s 2>/dev/null || echo "")
    if [[ -n "$created_epoch" ]]; then
      age_days=$(( (now_epoch - created_epoch) / 86400 ))
      age="${age_days}d"
    else
      age="$createdat"
    fi
    marker="  "
    [[ "$rank" == "0" ]] && marker="**"
    printf '%s %-14s %-45s %-8s %-60s %-5s %s\n' \
      "$marker" "$labelnames" "$repo" "$number" "$title" "$age" "$url"
  done < "$TMP_TSV"
}

# ------------------------------------------------- the five-part format builder --
# Written to a temp file rather than held in a shell variable: the program contains both
# quote characters and an em dash, and a heredoc quoted <<'JQFMT' passes every byte through
# untouched — no shell expansion, no quoting to get wrong.
#
# It is ONE program, and --walk and --html both render its output. That is the point: the
# format is the `ask-decision` skill's contract, and a second copy of it in a second renderer
# is how two surfaces of one contract drift apart.
write_format_program() {
  cat > "$TMP_FMT" <<'JQFMT'
# in:  [ <sorted queue item>, <issue detail: {body, comments} or {detailUnavailable:true}> ]
# out: one rendered item object — header/context/options/reply/verification.
def clean: gsub("\\p{Cc}"; "");
def lines: (. // "") | gsub("\r"; "") | split("\n");
def strip: sub("^[[:space:]]+"; "") | sub("[[:space:]]+$"; "");
def nonblank: map(select(test("[^[:space:]]")));
def isheading: test("^[[:space:]]*#{1,6}[[:space:]]");
def demd: gsub("\\*\\*"; "") | gsub("`"; "");

.[0] as $it
| .[1] as $d
| (($d.detailUnavailable // false) | if . then true else false end) as $blind
# HTML comments are the marker channel the desk tools write into issue bodies; they are not
# prose for the driver, so they never become Context.
| ((($d.body // "") | lines | map(select(test("^[[:space:]]*<!--") | not)))) as $bl
| def section($re):
    ($bl | to_entries | map(select(.value | test($re; "i"))) | first) as $h
    | if $h == null then []
      else ($bl[($h.key + 1):]) as $rest
        | ($rest | map(isheading) | index(true)) as $stop
        | (if $stop == null then $rest else $rest[:$stop] end)
      end;

  (section("^[[:space:]]*#{1,6}[[:space:]]*(Context|Situation|Ask|Summary|Problem)\\b")) as $csec
| (if ($csec | nonblank | length) > 0 then $csec else ($bl | map(select(isheading | not))) end
   | nonblank | map(strip | demd | clean)
   | map(select(test("^[|>]") | not))
   | .[0:2]) as $ctxraw
# A detail fetch that did not land is reported AS ITSELF. An unread item is not a clear one,
# and rendering it with an empty Context would read exactly like an issue that said nothing.
| (if $blind then
     ["could-not-check: this issue's body and comments could not be read (detail fetch failed) — the item is UNREAD, not empty"]
   elif ($ctxraw | length) > 0 then $ctxraw
   else ["context not stated in the issue body — desk to fill before asking"]
   end) as $ctx
| ((($it.labels // []) | map(.name | clean))) as $labelnames
| (($labelnames | map(select(. == "urgent" or . == "needs-decision" or . == "question" or . == "help wanted"))) as $esc
   | "blocked on the driver: carries "
     + (if ($esc | length) > 0 then ($esc | join(", ")) else "no escalation label" end)) as $blocked
# The unblocks line is what makes the ordering rule legible to the driver, so it is lifted
# out even when it did not fall inside the Context section — but never twice: when the two
# body lines already carry it, adding it again just burns one of the six.
| (($bl | nonblank | map(strip | demd | clean)
    | map(select(test("^(unblocks|blocks|depends|gates)\\b"; "i")))
    | map(select(. as $x | ($ctx | index($x)) == null))
    | .[0:1])) as $unb
| ((($d.comments // []) | map(select(.body != null)))) as $cs
| ((($cs | map(select((.author.login // "") | test("\\[bot\\]$|desk"; "i"))) | last) // ($cs | last))) as $lc
| (if $lc == null then []
   else ["latest desk note (" + (($lc.author.login // "?") | clean) + "): "
         + ((($lc.body | lines | nonblank | map(strip | demd) | .[0:2] | join(" ") | clean))[0:180])]
   end) as $note
| ((($it.createdAt // "") | (try fromdateiso8601 catch null)) as $c
   | if $c == null then null else (((now - $c) / 86400) | floor) end) as $age
| ("gate: " + ($labelnames | join(", "))
   + " · age " + (if $age == null then "unknown" else (($age | tostring) + "d") end)
   + " · " + (($it.url // "") | clean)) as $gate
# 3–6 lines, by construction: at least the Context line, the blocked-on line and the gate
# line; at most two body lines plus unblocks, the desk note and the gate line.
| (($ctx + [$blocked] + $unb + $note + [$gate]) | .[0:6]) as $context

| (section("^[[:space:]]*#{1,6}[[:space:]]*Options?\\b")) as $osec
| ($osec | nonblank | map(strip)
   | map(select(test("^(?:[-*][[:space:]]*)?(?:\\*\\*)?[A-Da-d1-4][.)][[:space:]]")))
   | map(capture("^(?:[-*][[:space:]]*)?(?:\\*\\*)?(?<let>[A-Da-d1-4])[.)][[:space:]]*(?<txt>.+)$"))
   | .[0:4]) as $oraw
# Detected on the RAW text, before the marker is stripped out of the rendered label.
| ((($oraw | to_entries | map(select(.value.txt | test("recommend"; "i"))) | first | .key) // 0)) as $recidx
| ($oraw | to_entries
   | sort_by(if .key == $recidx then 0 else 1 end, .key)
   | map(.value.txt | demd | clean | strip
         | sub("[[:space:]]*[—-]?[[:space:]]*\\(?[Rr]ecommended\\)?[[:space:]]*$"; "")
         | sub("^\\(?[Rr]ecommended\\)?[[:space:],:;—-]*"; "")
         | strip)
   | map(select(. != ""))
   | to_entries
   | map(.key as $i
         | {letter: (["A", "B", "C", "D"] | .[$i]), text: .value, recommended: ($i == 0)})
  ) as $opts
| (if ($opts | length) > 0 then $opts
   else [{letter: "A", text: "options not yet stated — desk to fill", recommended: true}]
   end) as $options
| (($opts | length) > 0) as $stated

| (if $stated
   then "reply with one letter (" + ($options | map(.letter) | join("/")) + ") — nothing else is needed."
   else "reply with the ruling in one line; the desk restates it as lettered options before acting."
   end) as $reply
| (if ($k + 1) < $n then "presents question " + (($k + 2) | tostring) + " of " + ($n | tostring) + "."
   else "reports the queue drained."
   end) as $next
| ("the desk records the ruling on " + $it.repo + "#" + ($it.number | tostring)
   + " as a relayed decision (never in the driver's voice), moves the escalation label per the"
   + " vocabulary, re-reads the issue to confirm the label moved, then " + $next) as $verification

| {
    repo: $it.repo,
    number: $it.number,
    url: (($it.url // "") | clean),
    title: ((($it.title // "") | clean | demd)),
    labels: $labelnames,
    ageDays: $age,
    index: ($k + 1),
    total: $n,
    header: ($it.repo + "#" + ($it.number | tostring)
             + " — question " + (($k + 1) | tostring) + " of " + ($n | tostring)),
    context: $context,
    options: $options,
    optionsStated: $stated,
    reply: $reply,
    verification: $verification,
    unread: $blind
  }
JQFMT
}

# build_item <0-based index> — fetch the issue detail and append the rendered item to
# $TMP_ITEMS. A detail fetch is a READ (`gh issue view`), the only extra call the two new
# modes make; the table mode still costs exactly what it always did.
build_item() {
  local idx="$1" repo number detail gh_status one merged
  repo=$(jq -r --argjson i "$idx" '.[$i].repo' "$TMP_SORTED")
  number=$(jq -r --argjson i "$idx" '.[$i].number' "$TMP_SORTED")
  jq --argjson i "$idx" '.[$i]' "$TMP_SORTED" > "$TMP_ONE"

  if detail=$(gh issue view "$number" --repo "$repo" --json body,comments 2>"$TMP_ERR"); then
    :
  else
    gh_status=$?
    query_failures=$((query_failures + 1))
    printf 'assay-inbox: DETAIL FETCH FAILED for %s#%s (gh exit %s): %s\n' \
      "$repo" "$number" "$gh_status" "$(tr '\n' ' ' <"$TMP_ERR")" >&2
    detail=""
  fi
  [[ -z "$detail" ]] && detail='{"detailUnavailable":true}'
  printf '%s' "$detail" > "$TMP_DETAIL"

  one=$(jq -s --argjson k "$idx" --argjson n "$item_count" -f "$TMP_FMT" "$TMP_ONE" "$TMP_DETAIL")
  merged=$(jq -s '.[0] + [.[1]]' "$TMP_ITEMS" <(printf '%s' "$one"))
  printf '%s' "$merged" > "$TMP_ITEMS"
}

render_walk() {
  jq -r '
    .[0]
    | [ .header,
        "",
        "Context",
        (.context[] | "  - " + .),
        "",
        "Options",
        (.options[] | "  " + .letter + ". " + .text
                      + (if .recommended then "   [recommended]" else "" end)),
        "",
        "Reply shape",
        "  " + .reply,
        "",
        "Verification",
        "  " + .verification
      ] | .[]
  ' "$TMP_ITEMS"
}

# The page is ONE file with no dependencies: inline CSS, no <script>, no src=, no @import,
# no url() — the only URLs it carries are the issue links themselves. That is what lets the
# desk hand the driver a file (or publish it) with nothing else attached, and it is asserted
# by the test suite rather than left as an intention.
write_html_program() {
  cat > "$TMP_HTML" <<'JQHTML'
def esc: tostring | @html;

# ------------------------------------------------------------------ the flow diagram --
# An INLINE SVG, drawn from the same flow model the terminal table reads. It carries no
# script, no external asset and no url() — every colour is a CSS custom property defined in
# the page's own <style>, so the diagram follows prefers-color-scheme with the rest of the
# page and needs no second palette.
#
# The SVG is never the only carrier: an equivalent <table> follows it with the same numbers.
# A diagram a screen reader cannot read is a diagram half the audience does not get, and
# `role="img"` plus a <title>/<desc> pair only names the picture — it does not convey seven
# stages of figures.
def BOXW: 116; def GAP: 36; def PITCH: 152; def BOXH: 66; def GUTTER: 132; def ROWH: 96;
def svgw: GUTTER + (7 * BOXW) + (6 * GAP) + 12;

def num($v): if $v == null then "n/a" else ($v | tostring) end;
def rat($v): if $v == null then "n/a" else ((($v * 100) | round) / 100 | tostring) end;

# Arrow thickness is the last period's flow, clamped so one busy window cannot draw a bar
# across the page. A weight that could not be read is drawn DASHED and thin — visibly not a
# measurement, rather than a thin line that reads as "almost nothing moved".
def stroke($w):
  if $w == null then 1.2
  else ((if $w < 0 then -$w else $w end) as $m
        | 1.4 + (if $m > 24 then 24 else $m end) / 24 * 4.6)
  end;

def stagebox($s; $k; $top):
  (GUTTER + ($k * PITCH)) as $x
  | (if $s.isBottleneck then "box bneck"
     elif $s.count == null and ($s.na // "") != "" then "box na"
     elif $s.count == null then "box blind"
     else "box" end) as $cls
  | [ "<g>",
      "<rect class=\"" + $cls + "\" x=\"\($x)\" y=\"\($top)\" width=\"\(BOXW)\" height=\"\(BOXH)\" rx=\"7\"/>",
      "<text class=\"st\" x=\"\($x + BOXW / 2)\" y=\"\($top + 17)\">" + ($s.stage | esc) + "</text>",
      (if $s.count == null and ($s.na // "") != "" then
         "<text class=\"nat\" x=\"\($x + BOXW / 2)\" y=\"\($top + 41)\">n/a here</text>"
       elif $s.count == null
       then "<text class=\"cnc\" x=\"\($x + BOXW / 2)\" y=\"\($top + 41)\">could-not-check</text>"
       else "<text class=\"ct\" x=\"\($x + BOXW / 2)\" y=\"\($top + 44)\">"
            + ($s.count | esc) + (if $s.countPartial then "+" else "" end) + "</text>" end),
      "<text class=\"sub\" x=\"\($x + BOXW / 2)\" y=\"\($top + 58)\">"
        + (if $s.slots == null then "no pool"
           elif ($s.queueBlind // "") != "" then ("? of " + num($s.slots) + " slots")
           else (num($s.queue) + "/" + num($s.slots) + " · r " + rat($s.ratio)) end | esc)
        + "</text>",
      (if $s.isBottleneck
       then "<text class=\"tag\" x=\"\($x + BOXW / 2)\" y=\"\($top - 4)\">bottleneck</text>"
       else empty end),
      "</g>" ];

# The arrowhead is an explicit <path>, not a <marker> + marker-end. A marker reference is
# spelled `url(#id)`, and the page's self-containedness test forbids `url(` outright — a
# blunt rule, but the right one to keep blunt: it is the assertion that nothing on this page
# can fetch anything, and weakening it to admit same-document fragments would make it a rule
# about syntax rather than about fetching. Two more path elements is a cheap price.
def arrow($a; $k; $top):
  (GUTTER + ($k * PITCH) + BOXW) as $x1
  | (GUTTER + (($k + 1) * PITCH)) as $x2
  | ($top + (BOXH / 2)) as $y
  | [ "<line class=\"" + (if $a.weight == null then "arr dash" else "arr" end) + "\""
      + " x1=\"\($x1)\" y1=\"\($y)\" x2=\"\($x2 - 8)\" y2=\"\($y)\""
      + " stroke-width=\"\(stroke($a.weight))\"/>",
      "<path class=\"ahp\" d=\"M \($x2 - 9) \($y - 4.5) L \($x2) \($y) L \($x2 - 9) \($y + 4.5) z\"/>",
      (if $a.weight == null then empty
       else "<text class=\"wt\" x=\"\(($x1 + $x2) / 2)\" y=\"\($y - 6)\">" + ($a.weight | esc) + "</text>"
       end) ];

def flowrow($row; $i):
  (12 + ($i * ROWH) + 18) as $top
  | [ "<text class=\"cell\" x=\"8\" y=\"\($top + BOXH / 2)\">"
      + ((if $row.fleet then "fleet" else $row.cell end) | esc) + "</text>",
      ($row.stages | to_entries | map(stagebox(.value; .key; $top)) | flatten | .[]),
      ($row.arrows | to_entries | map(arrow(.value; .key; $top)) | flatten | .[]) ];

def flowsvg($f):
  ($f.rows | length) as $n
  | (12 + ($n * ROWH) + 14) as $h
  | [ "<svg class=\"flow\" viewBox=\"0 0 \(svgw) \($h)\" role=\"img\""
      + " aria-labelledby=\"flowt flowd\" preserveAspectRatio=\"xMinYMin meet\">",
      "<title id=\"flowt\">Pipeline flow by stage</title>",
      "<desc id=\"flowd\">Seven stages left to right — intake, todo, in-progress, review, "
      + "implemented, verified, done — each box carrying the count now, the owning loop's "
      + "queue against its pool slots, and the ratio. The same figures follow in the table "
      + "below this diagram.</desc>",
      ($f.rows | to_entries | map(flowrow(.value; .key)) | flatten | .[]),
      "</svg>" ];

def flowtable($f):
  [ ($f.rows[]
     | "<h3>" + ((if .fleet then "fleet" else "cell " + .cell end) | esc)
       + (if .sha == "" then "" else " <span class=\"sha\">board " + (.sha | esc) + "</span>" end)
       + "</h3>",
       "<div class=\"scroll\"><table>",
       "<caption>Stage counts" + (if .fleet then " across every cell read" else " for this cell" end)
       + " — the same figures the diagram draws.</caption>",
       "<thead><tr><th scope=\"col\">Stage</th><th scope=\"col\">What it holds</th>"
       + "<th scope=\"col\">Count</th><th scope=\"col\">Queue</th><th scope=\"col\">Slots</th>"
       + "<th scope=\"col\">Ratio</th><th scope=\"col\">Dwell</th><th scope=\"col\">Source</th></tr></thead>",
       "<tbody>",
       (.stages[]
        | "<tr" + (if .isBottleneck then " class=\"bn\"" else "" end) + ">"
          + "<th scope=\"row\">" + (.stage | esc) + (if .isBottleneck then " <span class=\"rec\">bottleneck</span>" else "" end) + "</th>"
          + "<td>" + (.label | esc) + "</td>"
          + "<td class=\"n\">" + (if .count == null and (.na // "") != "" then "<span class=\"nax\">n/a</span>"
                                  elif .count == null then "<span class=\"cncx\">could-not-check</span>"
                                  else (.count | esc) + (if .countPartial then " <abbr title=\"at least: one or more cells were unread\">+</abbr>" else "" end) end) + "</td>"
          + "<td class=\"n\">" + (if (.queueBlind // "") != ""
                                  then "<span class=\"cncx\" title=\"" + (.queueBlind | esc) + "\">could-not-check</span>"
                                  else (num(.queue) | esc) end) + "</td>"
          + "<td class=\"n\">" + (num(.slots) | esc) + "</td>"
          + "<td class=\"n\">" + (rat(.ratio) | esc) + "</td>"
          + "<td class=\"n\">" + ((if .dwell == "" then "—" else .dwell end) | esc) + "</td>"
          + "<td class=\"src\">" + ((if .count == null and (.na // "") != "" then .na
                                     elif .count == null then .blind
                                     elif .countSource == "" then (.capacityNote // "")
                                     else .countSource end) | esc) + "</td></tr>"),
       "</tbody></table></div>",
       (if (.capacityNote // "") == "" then empty
        else "<p class=\"flowmeta\">" + (.capacityNote | esc) + "</p>" end),
       "<p class=\"flowmeta\">Flow in the window: arrivals <b>" + (num(.arrivals) | esc)
       + "</b> into the pipeline, completions <b>" + (num(.completions) | esc) + "</b> out of it"
       + (if .flowBlind == "" then "" else " — <span class=\"cncx\">" + (.flowBlind | esc) + "</span>" end)
       + ". Interior steps: <span class=\"cncx\">"
       + ((.arrows | map(select(.from == "todo")) | first | .blind) | esc) + "</span></p>",
       "<p class=\"flowmeta\">Dwell-weighted constraint (Theory of Constraints, WIP × dwell): <b>"
       + ((if .constraint == "" then "n/a" else .constraint end) | esc) + "</b> — "
       + (.constraintSource | esc) + "</p>") ];

def flowsection($f):
  [ "<section class=\"flow-sec\">",
    "<h2>Flow — how the system is performing</h2>",
    "<p class=\"meta\">asOf " + ($f.asOf | esc) + " · window " + ($f.since | esc)
      + " · read " + ($f.stagesRead | esc) + " of " + ($f.stagesTotal | esc)
      + " capacity stages · " + ($f.cellCount | esc) + " cell(s)</p>",
    (flowsvg($f) | .[]),
    "<p class=\"legend\">Arrow thickness is the flow measured for the window; a dashed arrow "
      + "is a step no reader measures. A box outlined in the alert colour is a stage whose "
      + "reader failed — could-not-check, never zero. A faintly dotted box is a figure that "
      + "does not exist at that granularity, which is not the same thing.</p>",
    ("<p class=\"advice\"><b>" + ($f.bottleneck | if . == "" then "no bottleneck named" else . end | esc)
      + "</b> — " + ($f.advice | esc) + "</p>"),
    (flowtable($f) | .[]),
    "<h3>Where these numbers come from</h3>",
    "<ul>",
    ($f.sources[] | "<li>" + esc + "</li>"),
    "</ul>",
    (if ($f.blind | length) > 0 then
       "<h3>Could not check</h3>", "<ul class=\"cnclist\">", ($f.blind[] | "<li>" + esc + "</li>"), "</ul>"
     else empty end),
    "<p class=\"honest\">" + ($f.note | esc) + "</p>",
    "</section>" ];

# The model arrives via --slurpfile rather than --argjson: it is a whole document, and
# `--argjson flow "$(cat …)"` would push every byte of it through argv.
($flowdoc[0]) as $flow
|
[
  "<!doctype html>",
  "<html lang=\"en\">",
  "<head>",
  "<meta charset=\"utf-8\">",
  "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
  ("<title>" + (if $flowonly then "Assay inbox — pipeline flow" else "Assay inbox — decisions waiting" end) + "</title>"),
  "<style>",
  ":root{color-scheme:light dark;--bg:#fbfaf7;--fg:#1b1a17;--card:#fff;--line:#e3e0d8;--muted:#6b6860;--accent:#8a5a2b;--recfg:#12603a;--recbg:#e3f3e9;--bnfg:#8c3b12;--bnbg:#fbe9de;--cnc:#8c3b12;--grid:#cfcbc1}",
  "@media (prefers-color-scheme:dark){:root{--bg:#131310;--fg:#eae7e0;--card:#1d1d18;--line:#34332c;--muted:#a09c92;--accent:#d9a066;--recfg:#8ad9ac;--recbg:#1b3327;--bnfg:#f0a97a;--bnbg:#3a2317;--cnc:#f0a97a;--grid:#4a483f}}",
  "*{box-sizing:border-box}",
  "body{margin:0;padding:2rem 1rem;background:var(--bg);color:var(--fg);font:16px/1.55 ui-sans-serif,system-ui,-apple-system,\"Segoe UI\",Roboto,Helvetica,Arial,sans-serif}",
  "main{max-width:54rem;margin:0 auto}",
  "h1{font-size:1.5rem;margin:0 0 .25rem}",
  "p.meta{margin:0 0 2rem;color:var(--muted);font-size:.85rem}",
  "article.card{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:1.1rem 1.25rem;margin:0 0 1.25rem}",
  "article.card h2{font-size:1.05rem;margin:0 0 .1rem;font-weight:650}",
  "article.card h2 a{color:var(--accent);text-decoration:none}",
  "article.card h2 a:hover{text-decoration:underline}",
  "p.title{margin:0 0 .9rem;color:var(--muted);font-size:.9rem}",
  "h3{font-size:.72rem;letter-spacing:.09em;text-transform:uppercase;color:var(--muted);margin:1rem 0 .35rem;font-weight:700}",
  "ul{margin:0;padding-left:1.15rem}",
  "ul li{margin:.15rem 0}",
  "ul.opts{list-style:none;padding-left:0}",
  "ul.opts li{margin:.3rem 0}",
  "span.rec{color:var(--recfg);background:var(--recbg);border-radius:999px;padding:.05rem .5rem;font-size:.72rem;font-weight:700;white-space:nowrap}",
  "p.unread{color:var(--accent);font-weight:600}",
  "p.empty{color:var(--muted)}",
  "section.flow-sec{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:1.1rem 1.25rem;margin:2rem 0 1.25rem}",
  "section.flow-sec h2{font-size:1.15rem;margin:0 0 .2rem}",
  "svg.flow{display:block;width:100%;height:auto;margin:.9rem 0 .3rem;overflow:visible}",
  "svg.flow .box{fill:var(--bg);stroke:var(--line);stroke-width:1.2}",
  "svg.flow .box.bneck{stroke:var(--bnfg);stroke-width:2.4;fill:var(--bnbg)}",
  "svg.flow .box.blind{stroke:var(--cnc);stroke-dasharray:5 4;fill:none}",
  "svg.flow .box.na{stroke:var(--line);stroke-dasharray:2 4;fill:none}",
  "svg.flow text{font:11px ui-sans-serif,system-ui,sans-serif;fill:var(--fg)}",
  "svg.flow .st{font-size:10.5px;fill:var(--muted);text-anchor:middle;letter-spacing:.03em}",
  "svg.flow .ct{font-size:21px;font-weight:700;text-anchor:middle}",
  "svg.flow .cnc{font-size:9.5px;fill:var(--cnc);text-anchor:middle;font-weight:700}",
  "svg.flow .nat{font-size:9.5px;fill:var(--muted);text-anchor:middle}",
  "svg.flow .sub{font-size:9.5px;fill:var(--muted);text-anchor:middle}",
  "svg.flow .tag{font-size:9px;fill:var(--bnfg);text-anchor:middle;font-weight:700;letter-spacing:.06em}",
  "svg.flow .cell{font-size:11px;fill:var(--muted);font-weight:600}",
  "svg.flow .arr{stroke:var(--grid);fill:none}",
  "svg.flow .arr.dash{stroke-dasharray:4 4}",
  "svg.flow .ahp{fill:var(--grid);stroke:none}",
  "svg.flow .wt{font-size:9px;fill:var(--muted);text-anchor:middle}",
  "div.scroll{overflow-x:auto;margin:.5rem 0 .2rem}",
  "table{border-collapse:collapse;width:100%;font-size:.82rem}",
  "caption{caption-side:top;text-align:left;color:var(--muted);font-size:.76rem;padding:.2rem 0 .4rem}",
  "th,td{border-bottom:1px solid var(--line);padding:.3rem .5rem;text-align:left;vertical-align:top}",
  "thead th{color:var(--muted);font-size:.72rem;text-transform:uppercase;letter-spacing:.06em}",
  "td.n,th.n{text-align:right;font-variant-numeric:tabular-nums;white-space:nowrap}",
  "td.src{color:var(--muted);font-size:.74rem}",
  "tr.bn th,tr.bn td{background:var(--bnbg)}",
  "tr.bn span.rec{color:var(--bnfg);background:transparent;padding:0}",
  "span.cncx{color:var(--cnc);font-weight:600}",
  "span.nax{color:var(--muted)}",
  "span.sha{color:var(--muted);font-weight:400;font-size:.78rem}",
  "p.legend,p.flowmeta{color:var(--muted);font-size:.78rem;margin:.35rem 0}",
  "p.advice{margin:.5rem 0 .2rem;font-size:.88rem}",
  "p.honest{color:var(--muted);font-size:.78rem;font-style:italic;margin:.9rem 0 0;border-top:1px solid var(--line);padding-top:.7rem}",
  "ul.cnclist li{color:var(--cnc)}",
  "</style>",
  "</head>",
  "<body>",
  "<main>",
  (if $flowonly then empty else "<h1>Decisions waiting on the driver</h1>" end),
  (if $flowonly then empty else ("<p class=\"meta\">" + ($summary | esc) + "</p>") end),
  (if $flowonly then "<h1>Pipeline flow</h1>" else empty end),
  (if $flowonly or length > 0 then empty
   else "<p class=\"empty\">Nothing is waiting on the driver right now.</p>" end),
  ( if $flowonly then empty else .[] end
    | "<article class=\"card\">",
      ("<h2><a href=\"" + (.url | esc) + "\">" + (.repo | esc) + "#" + (.number | esc)
        + "</a> — question " + (.index | esc) + " of " + (.total | esc) + "</h2>"),
      ("<p class=\"title\">" + (.title | esc) + "</p>"),
      (if .unread then "<p class=\"unread\">could-not-check: this item was not read</p>" else empty end),
      "<h3>Context</h3>",
      "<ul>",
      (.context[] | "<li>" + esc + "</li>"),
      "</ul>",
      "<h3>Options</h3>",
      "<ul class=\"opts\">",
      (.options[]
        | "<li><b>" + (.letter | esc) + ".</b> " + (.text | esc)
          + (if .recommended then " <span class=\"rec\">recommended</span>" else "" end)
          + "</li>"),
      "</ul>",
      "<h3>Reply shape</h3>",
      ("<p>" + (.reply | esc) + "</p>"),
      "<h3>Verification</h3>",
      ("<p>" + (.verification | esc) + "</p>"),
      "</article>"
  ),
  (if $flow == null then empty else (flowsection($flow) | .[]) end),
  "</main>",
  "</body>",
  "</html>"
] | .[]
JQHTML
}

# ============================================================================ the flow ==
# "How is the system performing" — a rendering of AUTHORED STATUS, derived from the JSON
# emitters that already exist. Three properties are load-bearing and every one of them is
# asserted by the test suite rather than left as an intention:
#
#   DERIVED, NOT PROBED.   Nothing here opens a new source. Each number is lifted from one
#                          reader (named on the page beside it), and a number no reader
#                          emits is absent, not estimated.
#   BLIND IS NOT ZERO.     A reader that failed leaves its stage `could-not-check`, carrying
#                          the reader's own diagnostic. It is never rounded to 0 — a stage
#                          shown as 0 reads as DRAINED, which is the opposite of unread.
#   A RENDERING OF STATUS. The board lints consistency; it does not measure truth. Every
#                          count here is as good as the status cell somebody authored, and
#                          the page says so in one line rather than implying a measurement.
#
# THE STAGE MODEL, and where each stage's numbers come from:
#
#   intake       raw-intake front door        count: statusgen --intake-debt (.untriaged)
#   todo         authored, not started        count: statusgen --bottleneck (wip todo)
#   in-progress  dispatched, being worked     count: statusgen --bottleneck (wip in-progress)
#   review       open PRs, no verdict at head count: deskboard throughput (review depth)
#   implemented  merged, awaiting a verifier  count: statusgen --bottleneck (wip implemented)
#   verified     verified, awaiting the flip  count: statusgen --bottleneck (wip verified)
#   done         exited the pipeline          count: statusgen --bottleneck (wip done)
#
# `review` is the FORGE lane that runs alongside `in-progress`: a brief is in-progress while
# its PR is open, and reaches `implemented` when that PR merges. It is placed between them
# for that reason, and it is the one stage with no board status behind it.
#
# COUNT and QUEUE are two different numbers and both are printed. COUNT is the board's WIP
# for the stage. QUEUE is the depth the owning LOOP actually dispatches from — which for
# `todo` is the ELIGIBLE, unclaimed subset, not the whole column. The ratio is QUEUE/SLOTS,
# because that is the ratio `deskboard throughput` defines and the one the widen decision
# turns on; using COUNT there would name a bottleneck nobody is working.
#
# SLOTS are fleet-wide CAPACITY (the loop's resolved pool width), so they appear on the
# fleet row only. A per-cell slots figure would have to be invented — pool width is not
# resolved per cell — and per-cell rows say so rather than repeating the fleet number.

STATUSGEN_BIN="${ASSAY_STATUSGEN:-statusgen}"
DESKBOARD_BIN="${ASSAY_DESKBOARD:-deskboard}"

# read_json <label> <outfile> <verdictfile> <cmd...> — run a reader, capture its stdout to
# <outfile>, and write a JSON `{ok, err}` verdict to <verdictfile>.
#
# The verdict goes to a FILE, not to stdout, precisely so that callers do not have to wrap
# this in `$(...)`: a command substitution runs in a subshell, and `query_failures` bumped
# inside one is discarded when it exits — the exact mechanism that made a dead query look
# like an empty inbox (F1) and the reason render_table already reads from a file rather than
# a pipe. Every flow reader must be able to redden the exit code, so none of them may run in
# a subshell.
#
# A reader is only OK when it exited 0 AND its stdout parses as JSON: a binary too old for
# the flag exits non-zero with a usage dump, and one that half-wrote its output leaves
# unparseable bytes. Both are could-not-check, and both carry the reader's own first stderr
# line so the operator can see WHY. `assay-config:` provenance lines are the desk tools'
# routine banner, not a diagnostic, so the first line past them is the one quoted.
read_json() {
  local label="$1" out="$2" verdict="$3"; shift 3
  local rc detail
  if "$@" > "$out" 2>"$TMP_ERR"; then rc=0; else rc=$?; fi
  if [[ "$rc" -ne 0 ]]; then
    detail=$(grep -v '^assay-config' "$TMP_ERR" 2>/dev/null | head -1 | LC_ALL=C tr -d '\000-\037')
    [[ -z "$detail" ]] && detail="exit ${rc}, no diagnostic"
    flow_failures=$((flow_failures + 1))
    printf 'assay-inbox: FLOW READER FAILED %s (exit %s): %s\n' "$label" "$rc" "$detail" >&2
    jq -nc --arg e "$detail" '{ok:false, err:$e}' > "$verdict"
    # A refused reader leaves a usage dump (or a half-written object) in $out, and the model
    # builder slurps that file unconditionally — unparseable bytes there would abort the whole
    # render rather than degrade one stage. Blanking it to `null` is what keeps ONE dead
    # reader from taking the other six down with it.
    echo "null" > "$out"
    return
  fi
  if ! jq -e . "$out" >/dev/null 2>&1; then
    flow_failures=$((flow_failures + 1))
    printf 'assay-inbox: FLOW READER %s exited 0 but did not emit JSON\n' "$label" >&2
    jq -nc '{ok:false, err:"reader exited 0 but its output is not JSON"}' > "$verdict"
    echo "null" > "$out"
    return
  fi
  jq -nc '{ok:true, err:""}' > "$verdict"
}

# collect_flow — run every reader once per cell (statusgen refuses multi-root for these
# sub-commands, so one invocation per root is the contract, not a workaround) plus ONE
# fleet-wide `deskboard throughput`, and assemble the RAW reader output into $TMP_RAW.
#
# The raw document is deliberately a dumb envelope: each reader's own JSON verbatim, beside
# the verdict on reading it. All interpretation happens in ONE jq program below, so the text
# table and the SVG page cannot disagree about a single number — the same discipline the
# five-part format builder already enforces for the decision queue.
collect_flow() {
  local i=0 n name path sha
  local sg_out id_out nf_out tp_out sg_v id_v nf_v tp_v cells_f
  sg_out=$(mktemp); id_out=$(mktemp); nf_out=$(mktemp); tp_out=$(mktemp)
  sg_v=$(mktemp); id_v=$(mktemp); nf_v=$(mktemp); tp_v=$(mktemp); cells_f=$(mktemp)
  echo "[]" > "$cells_f"

  n=${#CELL_PATHS[@]}
  while [[ "$i" -lt "$n" ]]; do
    name="${CELL_NAMES[$i]}"
    path="${CELL_PATHS[$i]}"
    # The board sha is the provenance stamp the honesty rule asks for: it says WHICH revision
    # of the board these counts were read off. Unreadable (not a git checkout) is stated, not
    # silently blank.
    sha=$(git -C "$path" rev-parse --short HEAD 2>/dev/null || printf 'could-not-check')

    read_json "statusgen --bottleneck (${name})" "$sg_out" "$sg_v" \
      "$STATUSGEN_BIN" --root "$path" --bottleneck --json
    read_json "statusgen --intake-debt (${name})" "$id_out" "$id_v" \
      "$STATUSGEN_BIN" --root "$path" --intake-debt --json
    if [[ -n "$FLOW_SINCE" ]]; then
      read_json "statusgen --net-flow (${name})" "$nf_out" "$nf_v" \
        "$STATUSGEN_BIN" --root "$path" --net-flow --json --since "$FLOW_SINCE"
    else
      read_json "statusgen --net-flow (${name})" "$nf_out" "$nf_v" \
        "$STATUSGEN_BIN" --root "$path" --net-flow --json
    fi

    jq -n \
      --arg cell "$name" --arg root "$path" --arg sha "$sha" \
      --slurpfile cells "$cells_f" \
      --slurpfile bnv "$sg_v" --slurpfile idv "$id_v" --slurpfile nfv "$nf_v" \
      --slurpfile bn "$sg_out" --slurpfile id "$id_out" --slurpfile nf "$nf_out" \
      '$cells[0] + [{
          cell: $cell, root: $root, sha: $sha,
          bottleneck:    (if $bnv[0].ok then ($bn[0] // null) else null end),
          bottleneckErr: $bnv[0].err,
          intake:        (if $idv[0].ok then ($id[0] // null) else null end),
          intakeErr:     $idv[0].err,
          netflow:       (if $nfv[0].ok then ($nf[0] // null) else null end),
          netflowErr:    $nfv[0].err
        }]' > "${cells_f}.next"
    mv "${cells_f}.next" "$cells_f"
    i=$((i + 1))
  done

  # ONE throughput read for the whole fleet: its depths and slots are resolved across the
  # repo/root set as a whole, so calling it per cell would report the same fleet numbers N
  # times under N different cell names — a coverage claim the reader does not make.
  read_json "deskboard throughput" "$tp_out" "$tp_v" "$DESKBOARD_BIN" throughput --json

  jq -n \
    --arg asOf "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg since "$FLOW_SINCE" \
    --slurpfile cells "$cells_f" --slurpfile tpv "$tp_v" --slurpfile tp "$tp_out" \
    '{asOf: $asOf, since: $since, cells: $cells[0],
      throughput: (if $tpv[0].ok then ($tp[0] // null) else null end),
      throughputErr: $tpv[0].err}' > "$TMP_RAW"

  rm -f "$sg_out" "$id_out" "$nf_out" "$tp_out" "$sg_v" "$id_v" "$nf_v" "$tp_v" "$cells_f"
}

# write_flow_program — the ONE interpreter. Raw reader output in, the flow model out.
#
# Everything the two renderers know about the pipeline is decided here: the stage list and
# its order, which reader owns which number, what makes a stage blind, how the fleet row
# aggregates, and which stage is the bottleneck. Two copies of that in two renderers is how
# a terminal table and a page start quoting different depths for the same queue.
write_flow_program() {
  cat > "$TMP_FLOWFMT" <<'JQFLOW'
# in:  the raw reader envelope written by collect_flow
# out: the flow model — { asOf, since, note, bottleneck, advice, rows[], sources[], blind[] }

# The stage list, in pipeline order. `tp` names the `deskboard throughput` stage that owns
# this stage's QUEUE and SLOTS; "" means no loop owns it and both are legitimately absent
# (not blind — nothing failed to read, there is nothing to read).
def stagedefs: [
  {stage:"intake",      label:"raw-intake front door",           tp:"intake"},
  {stage:"todo",        label:"authored, not started",           tp:"dispatch"},
  {stage:"in-progress", label:"dispatched, being worked",        tp:""},
  {stage:"review",      label:"open PRs, no verdict at head",    tp:"review"},
  {stage:"implemented", label:"merged, awaiting a verifier",     tp:"verify"},
  {stage:"verified",    label:"verified, awaiting the flip",     tp:""},
  {stage:"done",        label:"exited the pipeline",             tp:""}
];

def interiorNote:
  "could-not-check: no reader in the tree emits per-stage transition counts for a window. "
  + "`--net-flow` measures arrivals INTO the pipeline and completions OUT of it, not the "
  + "steps between, and a delta inferred from two board reads would be a measurement this "
  + "system does not take.";

def sgstage($bn; $name):
  if $bn == null then null else (($bn.stages // []) | map(select(.stage == $name)) | first) end;

def tpstage($tp; $name):
  if $tp == null or $name == "" then null
  else (($tp.stages // []) | map(select(.stage == $name)) | first) end;

# `deskboard throughput` names its bottleneck in ITS stage vocabulary; this is the one place
# that vocabulary is translated into the flow model's.
def tp2flow($n):
  {"dispatch":"todo", "review":"review", "verify":"implemented", "intake":"intake"}[$n] // "";

def blindtext($err; $fallback): if ($err // "") == "" then $fallback else $err end;

# One cell's COUNT for one stage, with the reason when there is none. A missing number is
# ALWAYS accompanied by the reason it is missing — never a bare null.
#
# Two DIFFERENT reasons, kept apart. `blind` is could-not-check: a reader was asked and did
# not answer. `na` is not-applicable: nothing failed, the figure legitimately does not exist
# at this granularity. Collapsing the second into the first would cry wolf on every per-cell
# row and teach the reader to skip the word that matters.
def cellcount($c; $sd):
  if $sd.stage == "intake" then
    (if $c.intake == null
     then {count:null, na:"", blind:("statusgen --intake-debt: " + blindtext($c.intakeErr; "not read"))}
     elif (($c.intake.state // "") != "measured")
     then {count:null, na:"", blind:("statusgen --intake-debt reported state=" + ($c.intake.state // "absent"))}
     else {count: $c.intake.untriaged, na:"", blind:""} end)
  elif $sd.stage == "review" then
    {count:null, blind:"",
     na:"read fleet-wide by `deskboard throughput`, which resolves no per-cell figure"}
  else
    (sgstage($c.bottleneck; $sd.stage) as $s
     | if $s == null
       then {count:null, na:"",
             blind:("statusgen --bottleneck: " + blindtext($c.bottleneckErr; "stage not emitted"))}
       else {count: $s.wip, na:"", blind:""} end)
  end;

def celldwell($c; $sd):
  sgstage($c.bottleneck; $sd.stage) as $s
  | if $s == null then {dwell:"", unknownDwell:null}
    else {dwell: ($s.median_dwell // ""), unknownDwell: ($s.unknown_dwell // null)} end;

def cellflow($c):
  if $c.netflow == null then
    {arrivals:null, completions:null,
     flowBlind:("statusgen --net-flow: " + blindtext($c.netflowErr; "not read"))}
  elif (($c.netflow.state // "") != "ok") then
    {arrivals:null, completions:null,
     flowBlind:("statusgen --net-flow reported state=" + ($c.netflow.state // "absent"))}
  else
    (($c.netflow.streams // [])
     | {arrivals: (map(.arrivals) | add // 0), completions: (map(.completions) | add // 0),
        flowBlind:""})
  end;

def arrows($arr; $comp; $flowBlind):
  [ {from:"intake", to:"todo", weight:$arr, blind:$flowBlind,
     source:"statusgen --net-flow --json (.streams[].arrivals, summed)"},
    {from:"todo", to:"in-progress", weight:null, blind:interiorNote, source:""},
    {from:"in-progress", to:"review", weight:null, blind:interiorNote, source:""},
    {from:"review", to:"implemented", weight:null, blind:interiorNote, source:""},
    {from:"implemented", to:"verified", weight:null, blind:interiorNote, source:""},
    {from:"verified", to:"done", weight:$comp, blind:$flowBlind,
     source:"statusgen --net-flow --json (.streams[].completions, summed)"} ];

.asOf as $asOf
| .since as $since
| .throughput as $tp
| (.throughputErr // "") as $tperr
| .cells as $cells
| (if $tp == null then tp2flow("") else tp2flow($tp.bottleneck // "") end) as $bneck

# ---- per-cell rows -------------------------------------------------------------------
| ( $cells | map(
    . as $c
    | cellflow($c) as $f
    | { cell: $c.cell, root: $c.root, sha: $c.sha, fleet: false,
        constraint: (if $c.bottleneck == null then "" else ($c.bottleneck.constraint // "") end),
        constraintSource: "statusgen --bottleneck --json (.constraint — WIP x dwell, the ToC locator)",
        arrivals: $f.arrivals, completions: $f.completions, flowBlind: $f.flowBlind,
        arrows: arrows($f.arrivals; $f.completions; $f.flowBlind),
        stages: (stagedefs | map(
          . as $sd
          | cellcount($c; $sd) as $cc
          | celldwell($c; $sd) as $cd
          | { stage: $sd.stage, label: $sd.label,
              count: $cc.count, countPartial: false, blind: $cc.blind, na: $cc.na,
              countSource: (if $sd.stage == "intake"
                            then "statusgen --intake-debt --json (.untriaged) @ " + $c.sha
                            elif $sd.stage == "review" then ""
                            else "statusgen --bottleneck --json (.stages[].wip) @ " + $c.sha end),
              dwell: $cd.dwell, unknownDwell: $cd.unknownDwell,
              # Capacity is resolved fleet-wide, so a per-cell row states that rather than
              # repeating the fleet's slots under a cell name it does not describe.
              queue: null, queueBlind: "", queueSource: "", slots: null, slotsSource: "", ratio: null,
              maxSlots: null, boundBy: "",
              # Said ONCE per cell block (`capacityNote` on the row itself), not on all seven
              # rows: a note repeated down a whole column stops being read.
              capacityNote: "",
              isBottleneck: false })),
        capacityNote: "pool width is resolved fleet-wide, so this block carries no slots or ratio — see the fleet row" }
  )) as $cellrows

# ---- the fleet row -------------------------------------------------------------------
# Counts are summed over the cells that WERE read; a stage where any cell was blind is
# flagged `countPartial`, because a sum over an unknown subset is a LOWER BOUND and must
# not be printed as a total.
| ( stagedefs | map(
    . as $sd
    | ($cellrows | map(.stages[] | select(.stage == $sd.stage))) as $ss
    | ($ss | map(select(.count != null) | .count)) as $known
    | ($ss | map(select(.count == null))) as $unknown
    | tpstage($tp; $sd.tp) as $t
    | (if $sd.stage == "review"
       then (if $t == null then null else $t.depth end)
       elif ($known | length) == 0 then null
       else ($known | add) end) as $count
    | { stage: $sd.stage, label: $sd.label,
        count: $count,
        na: "",
        countPartial: (($unknown | length) > 0 and ($known | length) > 0 and $sd.stage != "review"),
        blind: (if $count != null then ""
                elif $sd.stage == "review"
                then "deskboard throughput: " + blindtext($tperr; (if $t == null then "stage not emitted" else ($t.blind // "depth not read") end))
                else (($unknown | map(.blind) | map(select(. != "")) | first) // "no reader supplied this stage") end),
        countSource: (if $sd.stage == "intake" then "statusgen --intake-debt --json (.untriaged), summed over cells"
                      elif $sd.stage == "review" then "deskboard throughput --json (review depth)"
                      else "statusgen --bottleneck --json (.stages[].wip), summed over cells" end),
        dwell: (if ($cellrows | length) == 1 then ($ss[0].dwell // "") else "" end),
        unknownDwell: (if ($cellrows | length) == 1 then $ss[0].unknownDwell else null end),
        queue: (if $t == null then null else $t.depth end),
        # A blank QUEUE beside a real SLOTS figure is the confusing case: capacity resolved,
        # depth did not. It gets its own reason rather than an unexplained n/a, because a
        # reader who cannot tell "no queue" from "queue unread" will read the first.
        queueBlind: (if $sd.tp == "" then ""
                     elif $t == null then blindtext($tperr; "deskboard throughput not read")
                     elif $t.depth == null then ($t.blind // "depth not read")
                     else "" end),
        queueSource: (if $t == null then "" else ("deskboard throughput --json: " + ($t.depthNote // "")) end),
        slots: (if $t == null then null else $t.slots end),
        slotsSource: (if $t == null then "" else ($t.slotsSource // "") end),
        ratio: (if $t == null then null else $t.ratio end),
        maxSlots: (if $t == null then null else $t.maxSlots end),
        boundBy: (if $t == null then "" else ($t.boundBy // "") end),
        capacityNote: (if $sd.tp == "" then "no loop owns this stage — it has no pool to size" else "" end),
        isBottleneck: ($sd.stage == $bneck and $bneck != "") }
  )) as $fleetstages

| ( $cellrows | map(.arrivals) | map(select(. != null)) ) as $arrs
| ( $cellrows | map(.completions) | map(select(. != null)) ) as $comps
| ( $cellrows | map(select(.flowBlind != "") | .flowBlind) | first // "" ) as $fleetFlowBlind
| (if ($arrs | length) == 0 then null else ($arrs | add) end) as $fleetArr
| (if ($comps | length) == 0 then null else ($comps | add) end) as $fleetComp

| { asOf: $asOf,
    since: (if $since == "" then "the reader's default window" else $since end),
    bottleneck: $bneck,
    bottleneckSource: "deskboard throughput --json (.bottleneck — the largest queue/slots ratio among the stages it read)",
    advice: (if $tp == null
             then "COULD-NOT-CHECK: `deskboard throughput` was not read (" + blindtext($tperr; "no diagnostic")
                  + "), so no stage here carries a queue, a slot count or a ratio, and NO bottleneck is named. Blind is not idle."
             else ($tp.advice // "") end),
    stagesRead: (if $tp == null then 0 else ($tp.stagesRead // 0) end),
    stagesTotal: (if $tp == null then 4 else ($tp.stagesTotal // 4) end),
    note: "This diagram RENDERS AUTHORED STATUS. The board lints consistency between status "
          + "cells, Evidence and PRs; it does not measure whether the work is done. Every count "
          + "here is exactly as good as the status somebody wrote.",
    rows: ([{ cell: "fleet", root: "", sha: "", fleet: true,
              constraint: (if ($cellrows | length) == 1 then ($cellrows[0].constraint // "") else "" end),
              constraintSource: (if ($cellrows | length) == 1
                                 then "statusgen --bottleneck --json (.constraint — WIP x dwell, the ToC locator)"
                                 else "per cell only: the ToC constraint is computed per root and is not summable" end),
              arrivals: $fleetArr, completions: $fleetComp, flowBlind: $fleetFlowBlind,
              arrows: arrows($fleetArr; $fleetComp; $fleetFlowBlind),
              capacityNote: "",
              stages: $fleetstages }]
           + (if ($cellrows | length) > 1 then $cellrows else [] end)),
    cellCount: ($cellrows | length),
    sources: ([ "statusgen --bottleneck --json — per-stage board WIP and median dwell, one call per cell",
                "statusgen --intake-debt --json — the intake front-door depth and its oldest entry",
                "statusgen --net-flow --json — arrivals into and completions out of the pipeline for the window",
                "deskboard throughput --json — per-stage queue depth against resolved pool width, and the bottleneck" ]),
    blind: ([ (if $tp == null then "deskboard throughput: " + blindtext($tperr; "not read") else empty end),
              ($fleetstages[] | select(.queueBlind != "") | ("queue depth for `" + .stage + "` — " + .queueBlind)),
              ($cellrows[] | select(.flowBlind != "") | (.cell + " — " + .flowBlind)),
              ($cellrows[] | .stages[] | select(.blind != "" and .stage != "review") | (.stage + " — " + .blind)) ] | unique) }
JQFLOW
}

# render_flow_text — the terminal form. Same model, same numbers, paste-able into a hand-off.
render_flow_text() {
  jq -r '
    # `" " * 0` is null in jq, and "x" + null is an error — so every pad goes through here.
    def pad($s; $w): ($s | tostring) as $t
      | $t + (if ($w - ($t | length)) > 0 then (" " * ($w - ($t | length))) else "" end);
    def n($v): if $v == null then "n/a" else ($v | tostring) end;
    def r($v): if $v == null then "n/a" else (($v * 100 | round) / 100 | tostring) end;
    def rowline($a; $b; $c; $d; $e; $f; $g):
      "  " + pad($a; 12) + " " + pad($b; 7) + " " + pad($c; 7) + " " + pad($d; 6)
           + " " + pad($e; 6) + " " + pad($f; 9) + " " + $g;
    [ "pipeline flow — asOf " + .asOf + " · window " + .since,
      "",
      (.rows[]
       | . as $row
       | ("== " + (if .fleet then "FLEET" else "cell " + .cell + " (" + .root + ")" end)
          + (if .sha == "" then "" else "  board " + .sha end)),
         rowline("STAGE"; "COUNT"; "QUEUE"; "SLOTS"; "RATIO"; "DWELL"; "NOTE"),
         (.stages[]
          | rowline(
              (.stage + (if .isBottleneck then " *" else "" end));
              n(.count); n(.queue); n(.slots); r(.ratio);
              (if .dwell == "" then "-" else .dwell end);
              (if .count == null and (.na // "") != "" then "n/a — " + .na
               elif .count == null then "could-not-check: " + .blind
               elif .countPartial then "AT LEAST — some cells unread"
               elif (.queueBlind // "") != "" then "queue could-not-check (see below)"
               else (.capacityNote // "") end))),
         (if ($row.capacityNote // "") == "" then empty else "  " + $row.capacityNote end),
         "",
         ("  flow in window: arrivals " + n($row.arrivals) + " -> ... -> completions " + n($row.completions)
          + (if $row.flowBlind == "" then "" else "   [" + $row.flowBlind + "]" end)),
         ("  interior steps: " + ($row.arrows | map(select(.from == "todo")) | first | .blind)),
         ("  dwell-weighted constraint (ToC): "
          + (if $row.constraint == "" then "n/a — " + $row.constraintSource else $row.constraint end)),
         ""),
      "bottleneck (largest queue/slots ratio): "
        + (if .bottleneck == "" then "none named" else .bottleneck + " *" end)
        + "   read " + (.stagesRead | tostring) + " of " + (.stagesTotal | tostring) + " stages",
      "advice: " + .advice,
      "",
      "sources:",
      (.sources[] | "  - " + .),
      (if (.blind | length) > 0 then "could-not-check:" else empty end),
      (.blind[] | "  - " + .),
      "",
      .note
    ] | .[]
  ' "$TMP_FLOW"
}

# A TERMINATING SUMMARY is what makes "0 items" positively distinguishable from "the run
# died before printing anything" — without it the two are byte-identical (finding 1).
finish() {
  local summary
  if [[ "$MODE" == "flow" || "$MODE" == "flowhtml" ]]; then
    summary="assay-inbox: flow across ${#CELL_PATHS[@]} cell(s)"
    query_failures=$((query_failures + flow_failures))
  else
    summary="assay-inbox: ${item_count} item(s) across ${#REPOS[@]} repo(s)"
    # Stated, never silent: the decision page's exit code stays about the decisions, but the
    # operator is still told the flow section on it is partial.
    [[ "$flow_failures" -gt 0 ]] && summary="${summary}; the Flow section is INCOMPLETE (${flow_failures} reader(s) could not be read — see stderr)"
  fi
  [[ "$truncated" -gt 0 ]] && summary="${summary}; ${truncated} query(s) TRUNCATED at --limit ${LIMIT}"
  if [[ "$query_failures" -gt 0 ]]; then
    echo "${summary}; ${query_failures} query(s) FAILED — THIS INBOX IS INCOMPLETE (see stderr)"
    exit 2
  fi
  echo "$summary"
}

# summary_text — the same sentence the page prints in its header. Recomputed rather than
# captured so an incomplete run says so ON THE PAGE, not only on the terminal.
summary_text() {
  local s="assay-inbox: ${item_count} item(s) across ${#REPOS[@]} repo(s)"
  [[ "$truncated" -gt 0 ]] && s="${s}; ${truncated} query(s) TRUNCATED at --limit ${LIMIT}"
  [[ "$query_failures" -gt 0 ]] && s="${s}; ${query_failures} query(s) FAILED — INCOMPLETE"
  [[ "$flow_failures" -gt 0 ]] && s="${s}; the Flow section below is INCOMPLETE (${flow_failures} reader(s) could not be read)"
  printf '%s' "$s"
}

# build_flow — collect, then interpret, leaving the finished model in $TMP_FLOW. Both flow
# renderings and the flow section of the decision page go through here, so there is exactly
# one path from readers to model.
build_flow() {
  write_flow_program
  collect_flow
  if ! jq -f "$TMP_FLOWFMT" "$TMP_RAW" > "${TMP_FLOW}.next"; then
    echo "assay-inbox: failed to build the flow model from the readers' output" >&2
    exit 1
  fi
  mv "${TMP_FLOW}.next" "$TMP_FLOW"
}

case "$MODE" in
  table)
    render_table
    finish
    ;;

  walk)
    if [[ "$item_count" -eq 0 ]]; then
      # Nothing to ask. The summary is the answer, and it is a POSITIVE statement of
      # emptiness rather than silence — same contract as the table. `finish` exits 2 by
      # itself when a query failed; otherwise fall through to a clean exit here.
      finish
      exit 0
    fi
    if [[ "$WALK_ITEM" -gt "$item_count" ]]; then
      echo "assay-inbox: --item ${WALK_ITEM} is out of range (the queue holds ${item_count} item(s))" >&2
      exit 1
    fi
    write_format_program
    echo "[]" > "$TMP_ITEMS"
    build_item "$((WALK_ITEM - 1))"
    render_walk
    echo
    finish
    ;;

  html)
    write_format_program
    write_html_program
    echo "[]" > "$TMP_ITEMS"
    i=0
    while [[ "$i" -lt "$item_count" ]]; do
      build_item "$i"
      i=$((i + 1))
    done
    # The decision page carries the Flow section too: "which decision is next" and "where is
    # the system stuck" are the two halves of one hand-off, and a driver reading the page
    # should not have to open a second one to see whether the queue in front of them is the
    # constraint. `--flow --html` renders the same section alone.
    #
    # The flow model here is built over the CURRENT root only (`.`): the decision page's own
    # repo list is a repo axis, not a cell axis, and inventing a cell list from it would
    # claim a coverage no reader was given. `--flow --root ...` is the multi-cell form.
    CELL_NAMES=("$(cell_name_for .)")
    CELL_PATHS=(".")
    build_flow
    if ! jq -r --arg summary "$(summary_text)" --argjson flowonly false \
         --slurpfile flowdoc "$TMP_FLOW" -f "$TMP_HTML" "$TMP_ITEMS" > "$HTML_OUT"; then
      echo "assay-inbox: failed to write ${HTML_OUT}" >&2
      exit 1
    fi
    echo "assay-inbox: wrote ${item_count} card(s) to ${HTML_OUT}"
    finish
    ;;

  flow)
    build_flow
    render_flow_text
    finish
    ;;

  flowhtml)
    write_html_program
    build_flow
    echo "[]" > "$TMP_ITEMS"
    if ! jq -r --arg summary "" --argjson flowonly true \
         --slurpfile flowdoc "$TMP_FLOW" -f "$TMP_HTML" "$TMP_ITEMS" > "$HTML_OUT"; then
      echo "assay-inbox: failed to write ${HTML_OUT}" >&2
      exit 1
    fi
    echo "assay-inbox: wrote the flow diagram to ${HTML_OUT}"
    finish
    ;;
esac
