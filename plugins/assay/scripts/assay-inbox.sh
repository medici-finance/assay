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

Prints open issues across the given repos (or ./.assay/repos.txt, or the current repo's
origin remote) carrying any of the escalation-contract labels: urgent, needs-decision,
question, help wanted. Sorted urgency-then-age (most urgent, then oldest, first).

Modes (all three share one ordering and one format builder):
  (none)            the terminal table — one row per item.
  --walk            print ONE item in the five-part decision format: Header / Context /
                    Options / Reply shape / Verification. Prints item 1 and exits.
  --item K          with --walk (and implying it): print item K instead of item 1.
                    1-based; out of range is an error, never a silent empty.
  --html OUT.html   write the whole queue to OUT.html as cards in that same format —
                    one self-contained file: inline CSS, no scripts, no external assets,
                    light/dark via prefers-color-scheme. The only URLs are the issue links.

--walk is NON-interactive by design: it never prompts and never blocks on a tty. The
turn-taking (one decision per turn, record the ruling, move to the next) belongs to the
`ask-decision` skill, which shells this with an incrementing --item.

Read-only — this never writes to any issue.

Environment:
  ASSAY_INBOX_LIMIT   per-repo-per-label fetch cap passed to `gh issue list` (default 500).

Exit codes:
  0  all queries succeeded          1  gh/jq missing, no repos resolvable, or bad arguments
  2  one or more queries FAILED — the output is partial; see stderr
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
ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --walk) WANT_WALK=1; shift ;;
    --item)
      [[ $# -ge 2 ]] || { echo "assay-inbox: --item needs a 1-based item number" >&2; exit 1; }
      WALK_ITEM="$2"; WANT_WALK=1; shift 2 ;;
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
if [[ "$WANT_WALK" -eq 1 ]]; then MODE="walk"; fi
if [[ "$WANT_HTML" -eq 1 ]]; then MODE="html"; fi

if [[ ! "$WALK_ITEM" =~ ^[1-9][0-9]*$ ]]; then
  echo "assay-inbox: --item must be a positive integer (got '$WALK_ITEM')" >&2
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

command -v gh >/dev/null 2>&1 || { echo "assay-inbox: gh CLI not found" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "assay-inbox: jq not found" >&2; exit 1; }

# bash 3.2 has no `mapfile`; a `while read` fed by process substitution (NOT a pipe) keeps
# the appends in the current shell, so REPOS survives the loop.
REPOS=()
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

# Every repo token is validated before any gh call (a review finding): a metacharacter-laden token must never reach a shell, and garbage from a
# non-GitHub origin remote is clearer rejected here than as a gh failure downstream.
# The charset is GitHub's own owner/name alphabet.
for _repo in "${REPOS[@]}"; do
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
trap 'rm -f "$TMP_JSON" "$TMP_ERR" "$TMP_TSV" "$TMP_SORTED" "$TMP_ITEMS" "$TMP_ONE" "$TMP_DETAIL" "$TMP_FMT" "$TMP_WALK" "$TMP_HTML"' EXIT
echo "[]" > "$TMP_JSON"

query_failures=0
truncated=0

for repo in "${REPOS[@]}"; do
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
[
  "<!doctype html>",
  "<html lang=\"en\">",
  "<head>",
  "<meta charset=\"utf-8\">",
  "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
  "<title>Assay inbox — decisions waiting</title>",
  "<style>",
  ":root{color-scheme:light dark;--bg:#fbfaf7;--fg:#1b1a17;--card:#fff;--line:#e3e0d8;--muted:#6b6860;--accent:#8a5a2b;--recfg:#12603a;--recbg:#e3f3e9}",
  "@media (prefers-color-scheme:dark){:root{--bg:#131310;--fg:#eae7e0;--card:#1d1d18;--line:#34332c;--muted:#a09c92;--accent:#d9a066;--recfg:#8ad9ac;--recbg:#1b3327}}",
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
  "</style>",
  "</head>",
  "<body>",
  "<main>",
  "<h1>Decisions waiting on the driver</h1>",
  ("<p class=\"meta\">" + ($summary | esc) + "</p>"),
  (if length == 0 then "<p class=\"empty\">Nothing is waiting on the driver right now.</p>" else empty end),
  ( .[]
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
  "</main>",
  "</body>",
  "</html>"
] | .[]
JQHTML
}

# A TERMINATING SUMMARY is what makes "0 items" positively distinguishable from "the run
# died before printing anything" — without it the two are byte-identical (finding 1).
finish() {
  local summary="assay-inbox: ${item_count} item(s) across ${#REPOS[@]} repo(s)"
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
  printf '%s' "$s"
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
    if ! jq -r --arg summary "$(summary_text)" -f "$TMP_HTML" "$TMP_ITEMS" > "$HTML_OUT"; then
      echo "assay-inbox: failed to write ${HTML_OUT}" >&2
      exit 1
    fi
    echo "assay-inbox: wrote ${item_count} card(s) to ${HTML_OUT}"
    finish
    ;;
esac
