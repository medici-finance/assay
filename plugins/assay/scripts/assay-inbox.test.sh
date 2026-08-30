#!/usr/bin/env bash
# assay-inbox.test.sh — regression suite for assay-inbox.sh.
#
# Every case here pins a finding from a code review of the script. Each one was
# REPRODUCED against the pre-fix script first and FAILS without its fix:
#
#   F1  a failed gh query printed an empty inbox and exited 0 — byte-identical to a
#       genuinely empty inbox (`2>/dev/null || echo "[]"` swallowed a live 401)
#   F2  `gh issue list` had no --limit, so the default 30 silently kept the NEWEST 30
#       and discarded the older items this tool's ordering exists to surface
#   F3  `mapfile` is bash 4+; stock /bin/bash on macOS is 3.2 and died with exit 127
#   F4  `--json url` was fetched and never emitted
#   F5  C0 control bytes in issue titles/labels reached the operator's terminal raw — an
#       ESC (0x1b) sequence in an issue title is attacker-supplied input for any repo
#       where a stranger can file issues, and `@tsv` escapes only tab/LF/CR/backslash.
#       The F5 cases (control stripping, title truncation, and repo-token validation —
#       the delta review's findings A and B) are red against the pre-fix script.
#
# No network and no token: `gh` is stubbed on PATH per case, so the suite is hermetic
# and runs in CI. Run:  bash plugins/assay/scripts/assay-inbox.test.sh
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/assay-inbox.sh"

pass=0
fail=0

ok() {
  printf '  ok   %s\n' "$1"
  pass=$((pass + 1))
}

no() {
  printf '  FAIL %s\n     %s\n' "$1" "$2"
  fail=$((fail + 1))
}

# check <condition-result> <name> <failure-detail>
check() {
  if [[ "$1" -eq 0 ]]; then ok "$2"; else no "$2" "$3"; fi
}

# make_gh <dir> <exit_code> <stdout_payload> <stderr_payload>
# Writes a stub `gh` into <dir> that records its argv to <dir>/argv.log.
make_gh() {
  local dir="$1" code="$2" out="$3" err="$4"
  mkdir -p "$dir"
  {
    printf '#!/usr/bin/env bash\n'
    printf 'printf "%%s\\\\n" "$*" >> %s\n' "$(printf '%q' "$dir/argv.log")"
    if [[ -n "$err" ]]; then
      printf 'printf "%%s\\\\n" %s >&2\n' "$(printf '%q' "$err")"
    fi
    if [[ -n "$out" ]]; then
      printf 'printf "%%s" %s\n' "$(printf '%q' "$out")"
    fi
    printf 'exit %s\n' "$code"
  } > "$dir/gh"
  chmod +x "$dir/gh"
}

# issue_json <n> — an n-element issue array, all carrying needs-decision.
issue_json() {
  jq -nc --argjson n "$1" '[range($n) | {
    number: (100 + .),
    title: ("issue " + (. | tostring)),
    labels: [{name: "needs-decision"}],
    createdAt: "2026-01-01T00:00:00Z",
    url: ("https://github.com/o/r/issues/" + (100 + . | tostring))
  }]'
}

# run_case <workdir> <args...> — sets RC / OUT / ERR.
run_case() {
  local wd="$1"; shift
  OUT=$(PATH="$wd:$PATH" bash "$SCRIPT" "$@" 2>"$wd/stderr")
  RC=$?
  ERR=$(cat "$wd/stderr")
}

# contains <haystack> <needle>  -> 0 if present
contains() {
  case "$1" in (*"$2"*) return 0 ;; esac
  return 1
}

TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT

echo "assay-inbox regression suite ($(bash --version | head -1))"

# ---------------------------------------------------------------- F1 --------
echo "F1 — a failed query must not masquerade as an empty inbox"

w="$TMPROOT/f1-fail"
make_gh "$w" 1 "" 'non-200 OK status code: 401 Unauthorized body: "{ "message": "Bad credentials" }"'
run_case "$w" medici-finance/assay
check "$([[ "$RC" -ne 0 ]] && echo 0 || echo 1)" \
  "gh failure => non-zero exit (got $RC)" \
  "exit was 0 — a dead query reads as an empty inbox"
check "$(contains "$ERR" "Bad credentials" && echo 0 || echo 1)" \
  "gh's own diagnostic reaches stderr" "stderr was: ${ERR:-<empty>}"
check "$(contains "$OUT" "FAILED" && echo 0 || echo 1)" \
  "stdout summary flags the run as incomplete" "stdout was: ${OUT:-<empty>}"

w="$TMPROOT/f1-empty"
make_gh "$w" 0 '[]' ""
run_case "$w" medici-finance/assay
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "genuinely empty inbox => exit 0" "got $RC"
check "$(contains "$OUT" "0 item(s) across 1 repo(s)" && echo 0 || echo 1)" \
  "empty success prints a terminating summary" "stdout was: ${OUT:-<empty>}"

# The whole point of F1: the failed run and the empty run must NOT be identical.
check "$([[ "$(cat "$TMPROOT/f1-fail/stderr")" != "$(cat "$TMPROOT/f1-empty/stderr")" ]] && echo 0 || echo 1)" \
  "failed run is distinguishable from empty run" "both produced identical stderr"

# A partial failure must still print the rows it did get, and still exit 2.
w="$TMPROOT/f1-partial"
mkdir -p "$w"
cat > "$w/gh" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  *"--label urgent"*) echo 'unauthorized' >&2; exit 1 ;;
esac
printf '%s' '[{"number":7,"title":"t","labels":[{"name":"needs-decision"}],"createdAt":"2026-01-01T00:00:00Z","url":"https://x/7"}]'
STUB
chmod +x "$w/gh"
run_case "$w" o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "partial failure => exit 2" "got $RC"
check "$(contains "$OUT" "#7" && echo 0 || echo 1)" \
  "partial failure still prints the rows it did fetch" "stdout: $OUT"

# ---------------------------------------------------------------- F2 --------
echo "F2 — the default 30-item cap must not silently drop the oldest"

w="$TMPROOT/f2-limit"
make_gh "$w" 0 '[]' ""
run_case "$w" o/r
if grep -q -- "--limit" "$w/argv.log"; then
  lim=$(sed -n 's/.*--limit \([0-9]*\).*/\1/p' "$w/argv.log" | head -1)
  ok "an explicit --limit is passed to gh (--limit $lim)"
  check "$([[ "$lim" -gt 30 ]] && echo 0 || echo 1)" \
    "--limit ($lim) exceeds gh's silent default of 30" "got --limit $lim"
else
  no "an explicit --limit is passed to gh" \
    "argv never contained --limit: $(head -1 "$w/argv.log")"
fi

# Hitting the cap must be announced, not swallowed.
w="$TMPROOT/f2-trunc"
make_gh "$w" 0 "$(issue_json 5)" ""
OUT=$(PATH="$w:$PATH" ASSAY_INBOX_LIMIT=5 bash "$SCRIPT" o/r 2>"$w/stderr")
ERR=$(cat "$w/stderr")
check "$(contains "$ERR" "TRUNCATED" && echo 0 || echo 1)" \
  "hitting the cap warns on stderr" "stderr: ${ERR:-<empty>}"
check "$(contains "$OUT" "TRUNCATED" && echo 0 || echo 1)" \
  "truncation is recorded in the summary line" "stdout: $OUT"

# ---------------------------------------------------------------- F3 --------
echo "F3 — must run on bash 3.2 (stock /bin/bash on macOS)"

for builtin in mapfile readarray; do
  if grep -qE "^[^#]*\\b$builtin\\b" "$SCRIPT"; then
    no "no bash-4-only builtin '$builtin'" "found in $SCRIPT"
  else
    ok "no bash-4-only builtin '$builtin'"
  fi
done

# The same bash-4 line also brought associative arrays — pin both declaration forms so a
# `declare -A`/`local -A` regression cannot ride in under the mapfile guard above.
for decl in "declare -A" "local -A"; do
  if grep -qE "^[^#]*$decl" "$SCRIPT"; then
    no "no bash-4-only associative-array declaration '$decl'" "found in $SCRIPT"
  else
    ok "no bash-4-only associative-array declaration '$decl'"
  fi
done

# And case-modification expansion (${var^^}, ${var,,}, ${var^}, ${var,}) — bash 4 only.
# `^[^#]*` skips the header comment, which names ${var^^} on purpose.
if grep -qE '^[^#]*\$\{[A-Za-z_][A-Za-z0-9_]*[,^]{1,2}\}' "$SCRIPT"; then
  no "no bash-4-only case-modification expansion" "found in $SCRIPT"
else
  ok "no bash-4-only case-modification expansion"
fi

# Opportunistic execution check: it only proves bash 3.2 where /bin/bash IS 3.2 (stock
# macOS). On CI runners /bin/bash is bash 5, so the source greps above are the real guard.
if [[ -x /bin/bash ]]; then
  bver=$(/bin/bash -c 'echo $BASH_VERSINFO' 2>/dev/null || echo "?")
  w="$TMPROOT/f3"
  make_gh "$w" 0 '[]' ""
  PATH="$w:$PATH" /bin/bash "$SCRIPT" o/r >"$w/out" 2>"$w/err"
  rc=$?
  check "$([[ "$rc" -eq 0 ]] && echo 0 || echo 1)" \
    "/bin/bash (major $bver) runs the script cleanly" "exit $rc: $(cat "$w/err")"
fi

# The empty-array + `set -u` path is the other bash-3.2 landmine: no repos resolvable.
w="$TMPROOT/f3-empty"
make_gh "$w" 0 '[]' ""
mkdir -p "$w/norepo"
OUT=$(cd "$w/norepo" && PATH="$w:$PATH" bash "$SCRIPT" 2>"$w/stderr")
RC=$?
ERR=$(cat "$w/stderr")
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "no resolvable repos => exit 1" "got $RC"
if contains "$ERR" "unbound variable"; then
  no "no-repo path is bash-3.2 safe under set -u" "set -u tripped: $ERR"
else
  check "$(contains "$ERR" "no repos to query" && echo 0 || echo 1)" \
    "no-repo path reports clearly" "stderr: ${ERR:-<empty>}"
fi

# ---------------------------------------------------------------- F4 --------
echo "F4 — the fetched url must actually be emitted"

w="$TMPROOT/f4"
make_gh "$w" 0 '[{"number":42,"title":"t","labels":[{"name":"urgent"}],"createdAt":"2026-01-01T00:00:00Z","url":"https://github.com/o/r/issues/42"}]' ""
run_case "$w" o/r
check "$(contains "$OUT" "https://github.com/o/r/issues/42" && echo 0 || echo 1)" \
  "issue url appears in the table" "stdout: $OUT"

# ---------------------------------------------------------------- F5 --------
echo "F5 — display fields must not carry terminal control bytes (delta finding A)"

# Case 1: a title and label carrying C0 controls. The raw bytes are built at runtime
# (printf %b), then JSON-encoded with jq exactly the way real `gh` emits them — as the
# printable six-character escapes, which jq decodes back to raw bytes downstream.
w="$TMPROOT/f5-controls"
title=$(printf '%b' 'Hi \033[31mRED\033[0m \033]0;HIJACK\007 \033[2Jtail')
label=$(printf '%b' 'urgent\033[31m')
payload=$(jq -nc --arg t "$title" --arg l "$label" \
  '[{number:9, title:$t, labels:[{name:$l}], createdAt:"2026-01-01T00:00:00Z", url:"https://x/9"}]')
make_gh "$w" 0 "$payload" ""
run_case "$w" o/r
stripped=$(printf '%s' "$OUT" | LC_ALL=C tr -d '\000-\010\013\014\016-\037\177')
check "$([[ "$stripped" == "$OUT" ]] && echo 0 || echo 1)" \
  "stdout contains zero C0/DEL bytes" \
  "control bytes survived: $(printf '%s' "$OUT" | LC_ALL=C tr -d '\000-\010\013\014\016-\037\177' | od -An -tx1 | head -2)"
check "$(contains "$OUT" "RED" && echo 0 || echo 1)" \
  "harmless text survived (RED)" "stdout was: ${OUT:-<empty>}"
check "$(contains "$OUT" "[31m" && echo 0 || echo 1)" \
  "harmless text survived ([31m — ESC stripped, content kept)" "stdout was: ${OUT:-<empty>}"

# Case 2: %-60s pads but never truncates — an over-long title must be cut to fit the column.
w="$TMPROOT/f5-long"
long_title=$(printf 'x%.0s' {1..100})
payload=$(jq -nc --arg t "$long_title" \
  '[{number:10, title:$t, labels:[{name:"urgent"}], createdAt:"2026-01-01T00:00:00Z", url:"https://x/10"}]')
make_gh "$w" 0 "$payload" ""
run_case "$w" o/r
check "$(contains "$OUT" "..." && echo 0 || echo 1)" \
  "over-long title is truncated with ..." "stdout was: ${OUT:-<empty>}"
check "$(contains "$OUT" "$long_title" && echo 1 || echo 0)" \
  "the full 100-char title does not appear" "full title present in: ${OUT:-<empty>}"

# Case 3: repo tokens are validated before any gh call (delta finding B) — a token
# carrying shell metacharacters must be rejected, not interpolated into a command line.
w="$TMPROOT/f5-repoarg"
make_gh "$w" 0 '[]' ""
run_case "$w" 'o/r;touch "$w/pwned"'
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" \
  "invalid repo token => exit 1" "got $RC (stderr: ${ERR:-<empty>})"
check "$(contains "$ERR" "invalid repo" && echo 0 || echo 1)" \
  "stderr names the invalid repo" "stderr was: ${ERR:-<empty>}"
check "$([[ ! -e "$w/pwned" ]] && echo 0 || echo 1)" \
  "the token never reaches a shell (no file created)" "pwned marker file EXISTS"

# --------------------------------------------------- ranking (no regression) --
echo "Ranking/dedupe (guard against regressing what the review found sound)"

w="$TMPROOT/rank"
mkdir -p "$w"
cat > "$w/gh" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  *"--label urgent"*)
    printf '%s' '[{"number":1,"title":"urgent one","labels":[{"name":"urgent"},{"name":"needs-decision"}],"createdAt":"2026-05-01T00:00:00Z","url":"https://x/1"}]' ;;
  *"--label needs-decision"*)
    printf '%s' '[{"number":1,"title":"urgent one","labels":[{"name":"urgent"},{"name":"needs-decision"}],"createdAt":"2026-05-01T00:00:00Z","url":"https://x/1"},{"number":2,"title":"older nd","labels":[{"name":"needs-decision"}],"createdAt":"2026-01-01T00:00:00Z","url":"https://x/2"},{"number":3,"title":"newer nd","labels":[{"name":"needs-decision"}],"createdAt":"2026-03-01T00:00:00Z","url":"https://x/3"}]' ;;
  *) printf '%s' '[]' ;;
esac
STUB
chmod +x "$w/gh"
run_case "$w" o/r
order=$(printf '%s\n' "$OUT" | sed -n 's/.*\(#[0-9][0-9]*\).*/\1/p' | tr '\n' ' ')
check "$([[ "$order" == "#1 #2 #3 " ]] && echo 0 || echo 1)" \
  "urgency-then-oldest order preserved (#1 #2 #3)" "got: $order"
marked=$(printf '%s\n' "$OUT" | grep -c '^\*\*')
check "$([[ "$marked" -eq 1 ]] && echo 0 || echo 1)" \
  "the urgent item is marked once (dedupe held)" "urgent-marked rows: $marked"
check "$(contains "$OUT" "3 item(s) across 1 repo(s)" && echo 0 || echo 1)" \
  "summary counts deduped items" "stdout: $OUT"

# ============================================================================
# The five-part decision format — `--walk` and `--html` (issue #224).
#
# One fixture queue, rendered in all three forms. The assertions below are the FORMAT's
# contract, not the current wording of any one line: the five required sections per item,
# the ordering shared with the table, the recommended-first rule, the stated fallbacks, and
# the self-containedness of the page. Every case here is red against the pre-#224 script,
# which had no --walk/--html and rejected both as invalid repo tokens.
# ============================================================================

# make_gh_walk <dir> — a stub serving BOTH `gh issue list` (the fixture queue) and
# `gh issue view` (each issue's body + comments). Three issues, deliberately shaped:
#   #1  urgent, newest    — Context + Options where the RECOMMENDED option is listed SECOND
#   #2  needs-decision, oldest — a plain body: no Context heading, no Options at all
#   #3  needs-decision, middle — hostile title/body: markup that must be escaped in HTML
make_gh_walk() {
  local dir="$1"
  mkdir -p "$dir"
  cat > "$dir/gh" <<'WALKSTUB'
#!/usr/bin/env bash
b1='## Context

The public leak-sweep gate holds every PR in the queue.
Unblocks: three in-flight PRs plus the release cut.

## Options

A. Loosen the sweep pattern — faster, but weaker detection
B. Keep the gate and fire it on approve — the queue drains, detection unchanged (recommended)
C. Do nothing — the queue stays held indefinitely
'
b3='Some prose. <script>alert(1)</script> & an ampersand.'
case "$*" in
  *"issue view"*)
    case "$*" in
      *" 1 "*) jq -nc --arg b "$b1" '{body:$b, comments:[{author:{login:"assay-desk-app[bot]"}, body:"Desk note: the sweep is house-side and stays there."}]}' ;;
      *" 3 "*) jq -nc --arg b "$b3" '{body:$b, comments:[]}' ;;
      *)       jq -nc '{body:"A plain body with no headings and no options.", comments:[]}' ;;
    esac
    ;;
  *"--label urgent"*)
    printf '%s' '[{"number":1,"title":"leak-sweep gate holds the queue","labels":[{"name":"urgent"}],"createdAt":"2026-05-01T00:00:00Z","url":"https://example.test/o/r/issues/1"}]' ;;
  *"--label needs-decision"*)
    printf '%s' '[{"number":2,"title":"oldest decision","labels":[{"name":"needs-decision"}],"createdAt":"2026-01-01T00:00:00Z","url":"https://example.test/o/r/issues/2"},{"number":3,"title":"<script>alert(1)</script> & more","labels":[{"name":"needs-decision"}],"createdAt":"2026-03-01T00:00:00Z","url":"https://example.test/o/r/issues/3"}]' ;;
  *) printf '%s' '[]' ;;
esac
WALKSTUB
  chmod +x "$dir/gh"
}

# The five section markers every rendered item must carry, in this order.
SECTIONS_RE='question [0-9]+ of [0-9]+'

# ordered_contains <text> <a> <b> — 0 if <a> appears before <b>.
ordered_contains() {
  local ia ib
  ia=$(printf '%s\n' "$1" | grep -n -- "$2" | head -1 | cut -d: -f1)
  ib=$(printf '%s\n' "$1" | grep -n -- "$3" | head -1 | cut -d: -f1)
  [[ -n "$ia" && -n "$ib" && "$ia" -lt "$ib" ]]
}

# ---------------------------------------------------------------- W1 --------
echo "W1 — --walk renders ONE item carrying all five required sections, in order"

w="$TMPROOT/w1"
make_gh_walk "$w"
run_case "$w" --walk o/r
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "--walk exits 0 on a healthy queue" "got $RC (stderr: ${ERR:-<empty>})"
check "$(printf '%s' "$OUT" | grep -Eq "^o/r#1 — $SECTIONS_RE\$" && echo 0 || echo 1)" \
  "header is '<repo>#<N> — question k of n'" "stdout was: ${OUT:-<empty>}"
for section in "Context" "Options" "Reply shape" "Verification"; do
  check "$(printf '%s' "$OUT" | grep -Fxq "$section" && echo 0 || echo 1)" \
    "section '$section' is present" "stdout was: ${OUT:-<empty>}"
done
check "$(ordered_contains "$OUT" "^Context\$" "^Options\$" && echo 0 || echo 1)" \
  "Context precedes Options" "stdout was: ${OUT:-<empty>}"
check "$(ordered_contains "$OUT" "^Options\$" "^Reply shape\$" && echo 0 || echo 1)" \
  "Options precedes Reply shape" "stdout was: ${OUT:-<empty>}"
check "$(ordered_contains "$OUT" "^Reply shape\$" "^Verification\$" && echo 0 || echo 1)" \
  "Reply shape precedes Verification" "stdout was: ${OUT:-<empty>}"
# Exactly ONE item per invocation — the turn boundary belongs to the skill, not the script.
# Counted on the HEADER SHAPE anchored at both ends, not on the phrase: the Verification
# line names the next question ("presents question 2 of 3"), and a loose match counts it.
headers=$(printf '%s\n' "$OUT" | grep -Ec '^[A-Za-z0-9._/-]+#[0-9]+ — question [0-9]+ of [0-9]+$' || true)
check "$([[ "$headers" -eq 1 ]] && echo 0 || echo 1)" \
  "exactly one item is rendered per --walk invocation" "found $headers item headers"
# The context is the 3-6 lines the format asks for, never an empty section.
ctxlines=$(printf '%s\n' "$OUT" | sed -n '/^Context$/,/^$/p' | grep -c '^  - ' || true)
check "$([[ "$ctxlines" -ge 3 && "$ctxlines" -le 6 ]] && echo 0 || echo 1)" \
  "Context carries 3-6 lines (got $ctxlines)" "stdout was: ${OUT:-<empty>}"

# ---------------------------------------------------------------- W2 --------
echo "W2 — the recommended option is FIRST and labelled, whatever order the issue listed it in"

# The fixture lists the recommended option SECOND. Recommended-first is the format's rule,
# so the renderer must reorder — and must relabel: the option that lands at A is the one
# marked [recommended].
firstopt=$(printf '%s\n' "$OUT" | sed -n '/^Options$/,/^$/p' | grep '^  A\. ' | head -1)
check "$(contains "$firstopt" "Keep the gate" && echo 0 || echo 1)" \
  "the recommended option is promoted to A" "option A was: ${firstopt:-<none>}"
check "$(contains "$firstopt" "[recommended]" && echo 0 || echo 1)" \
  "option A is labelled [recommended]" "option A was: ${firstopt:-<none>}"
recount=$(printf '%s\n' "$OUT" | grep -c '\[recommended\]' || true)
check "$([[ "$recount" -eq 1 ]] && echo 0 || echo 1)" \
  "exactly one option is labelled recommended" "labelled options: $recount"
# Never more than four.
optcount=$(printf '%s\n' "$OUT" | sed -n '/^Options$/,/^$/p' | grep -Ec '^  [A-D]\. ' || true)
check "$([[ "$optcount" -ge 1 && "$optcount" -le 4 ]] && echo 0 || echo 1)" \
  "at most four options are offered (got $optcount)" "stdout was: ${OUT:-<empty>}"
# The reply must be answerable in one word.
check "$(contains "$OUT" "reply with one letter" && echo 0 || echo 1)" \
  "Reply shape asks for a single letter when options are stated" "stdout was: ${OUT:-<empty>}"

# ---------------------------------------------------------------- W3 --------
echo "W3 — --walk ordering is the table's ordering, and --item selects within it"

w="$TMPROOT/w3"
make_gh_walk "$w"
run_case "$w" o/r
table_order=$(printf '%s\n' "$OUT" | sed -n 's/.*\(#[0-9][0-9]*\).*/\1/p' | tr '\n' ' ')
check "$([[ "$table_order" == "#1 #2 #3 " ]] && echo 0 || echo 1)" \
  "table order is #1 #2 #3 (urgency then age)" "got: $table_order"

walk_order=""
for k in 1 2 3; do
  run_case "$w" --walk --item "$k" o/r
  walk_order="${walk_order}$(printf '%s\n' "$OUT" | sed -n '1s/.*\(#[0-9][0-9]*\).*/\1/p') "
  check "$(printf '%s' "$OUT" | grep -Eq "question $k of 3" && echo 0 || echo 1)" \
    "--item $k announces itself as 'question $k of 3'" "stdout head: $(printf '%s\n' "$OUT" | head -1)"
done
check "$([[ "$walk_order" == "#1 #2 #3 " ]] && echo 0 || echo 1)" \
  "--walk walks the SAME order as the table" "got: $walk_order"

# The last item's Verification says the queue drains; earlier ones name the next question.
check "$(contains "$OUT" "reports the queue drained" && echo 0 || echo 1)" \
  "the final item's Verification states the queue drains" "stdout was: ${OUT:-<empty>}"

# ---------------------------------------------------------------- W4 --------
echo "W4 — an issue that states no options says so; it never invents them"

run_case "$w" --walk --item 2 o/r
check "$(contains "$OUT" "options not yet stated" && echo 0 || echo 1)" \
  "missing Options falls back to 'options not yet stated — desk to fill'" "stdout was: ${OUT:-<empty>}"
check "$(contains "$OUT" "reply with the ruling in one line" && echo 0 || echo 1)" \
  "Reply shape degrades to a one-line ruling when no options are stated" "stdout was: ${OUT:-<empty>}"
# Even with nothing to quote, the five sections still stand.
for section in "Context" "Options" "Reply shape" "Verification"; do
  check "$(printf '%s' "$OUT" | grep -Fxq "$section" && echo 0 || echo 1)" \
    "section '$section' survives an empty issue body" "stdout was: ${OUT:-<empty>}"
done

# ---------------------------------------------------------------- W5 --------
echo "W5 — bad --item / empty queue: refused or positively empty, never a fake question"

run_case "$w" --walk --item 9 o/r
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "--item past the end exits 1" "got $RC"
check "$(contains "$ERR" "out of range" && echo 0 || echo 1)" \
  "stderr says the item is out of range" "stderr was: ${ERR:-<empty>}"

run_case "$w" --walk --item 0 o/r
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "--item 0 exits 1" "got $RC"
run_case "$w" --walk --item abc o/r
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "a non-numeric --item exits 1" "got $RC"
run_case "$w" --walk --html /dev/null o/r
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "--walk with --html is refused, not silently resolved" "got $RC"
run_case "$w" --bogus o/r
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "an unknown flag exits 1 instead of being queried as a repo" "got $RC"

w="$TMPROOT/w5-empty"
make_gh "$w" 0 '[]' ""
run_case "$w" --walk o/r
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "--walk on an empty queue exits 0" "got $RC"
check "$(contains "$OUT" "0 item(s) across 1 repo(s)" && echo 0 || echo 1)" \
  "--walk on an empty queue prints the terminating summary" "stdout was: ${OUT:-<empty>}"
check "$(printf '%s' "$OUT" | grep -Eq 'question [0-9]+ of' && echo 1 || echo 0)" \
  "--walk on an empty queue asks nothing" "an item header was rendered: $OUT"

# ---------------------------------------------------------------- W6 --------
echo "W6 — an unreadable issue is could-not-check, never a clean item"

# The list query succeeds; the DETAIL fetch fails. The item must still render (the driver
# needs to see that it is waiting) but must say it was not read, and the run must exit 2 —
# an instrument that did not look has not cleared anything.
w="$TMPROOT/w6"
mkdir -p "$w"
cat > "$w/gh" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  *"issue view"*) echo 'HTTP 403: Resource not accessible' >&2; exit 1 ;;
  *"--label urgent"*) printf '%s' '[{"number":5,"title":"unreadable","labels":[{"name":"urgent"}],"createdAt":"2026-01-01T00:00:00Z","url":"https://example.test/5"}]' ;;
  *) printf '%s' '[]' ;;
esac
STUB
chmod +x "$w/gh"
run_case "$w" --walk o/r
check "$(contains "$OUT" "could-not-check" && echo 0 || echo 1)" \
  "an unread issue renders as could-not-check" "stdout was: ${OUT:-<empty>}"
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" \
  "a failed detail fetch exits 2 (the render is INCOMPLETE)" "got $RC"
check "$(contains "$ERR" "403" && echo 0 || echo 1)" \
  "gh's own diagnostic reaches stderr" "stderr was: ${ERR:-<empty>}"

# ---------------------------------------------------------------- H1 --------
echo "H1 — --html renders the same queue, same order, same five sections per card"

w="$TMPROOT/h1"
make_gh_walk "$w"
HTML="$w/inbox.html"
run_case "$w" --html "$HTML" o/r
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "--html exits 0 on a healthy queue" "got $RC (stderr: ${ERR:-<empty>})"
check "$([[ -s "$HTML" ]] && echo 0 || echo 1)" "--html writes a non-empty file" "no file at $HTML"
PAGE=$(cat "$HTML" 2>/dev/null || true)

cards=$(grep -c '<article class="card">' "$HTML" || true)
check "$([[ "$cards" -eq 3 ]] && echo 0 || echo 1)" "one card per queue item (got $cards, want 3)" "cards: $cards"
for section in "Context" "Options" "Reply shape" "Verification"; do
  n=$(grep -c "<h3>$section</h3>" "$HTML" || true)
  check "$([[ "$n" -eq 3 ]] && echo 0 || echo 1)" \
    "every card carries a '$section' section (got $n of 3)" "count: $n"
done
html_order=$(grep -o 'issues/[0-9]*">' "$HTML" | sed 's#issues/##; s#">##' | tr '\n' ' ')
check "$([[ "$html_order" == "1 2 3 " ]] && echo 0 || echo 1)" \
  "the page preserves the queue order (1 2 3)" "got: $html_order"
check "$(contains "$PAGE" "question 1 of 3" && contains "$PAGE" "question 3 of 3" && echo 0 || echo 1)" \
  "cards carry the same 'question k of n' header as --walk" "page head: $(head -c 200 "$HTML")"
check "$(grep -c '<span class="rec">recommended</span>' "$HTML" | grep -qx 3 && echo 0 || echo 1)" \
  "each card labels exactly one recommended option" "count: $(grep -c '<span class="rec">recommended</span>' "$HTML" || true)"

# ---------------------------------------------------------------- H2 --------
echo "H2 — the page is SELF-CONTAINED: no scripts, no external assets, issue links only"

for forbidden in "<script" " src=" "@import" "url(" "<iframe" "<link "; do
  check "$(grep -qF -- "$forbidden" "$HTML" && echo 1 || echo 0)" \
    "the page contains no '$forbidden'" "found '$forbidden' in $HTML"
done

# Every href must be one of the fixture's own issue URLs — nothing else may be fetched.
stray=0
while IFS= read -r href; do
  case "$href" in
    https://example.test/o/r/issues/1|https://example.test/o/r/issues/2|https://example.test/o/r/issues/3) ;;
    *) stray=$((stray + 1)); echo "     stray href: $href" ;;
  esac
done < <(grep -o 'href="[^"]*"' "$HTML" | sed 's/^href="//; s/"$//')
check "$([[ "$stray" -eq 0 ]] && echo 0 || echo 1)" \
  "every href is one of the queue's own issue links" "$stray stray href(s)"

check "$(contains "$PAGE" "prefers-color-scheme" && echo 0 || echo 1)" \
  "light/dark is handled with prefers-color-scheme" "no media query in $HTML"

# ---------------------------------------------------------------- H3 --------
echo "H3 — the page is well-formed and escapes attacker-supplied text"

check "$([[ "$(head -1 "$HTML")" == "<!doctype html>" ]] && echo 0 || echo 1)" \
  "the file opens with a doctype" "first line: $(head -1 "$HTML")"
check "$([[ "$(tail -1 "$HTML")" == "</html>" ]] && echo 0 || echo 1)" \
  "the file closes </html>" "last line: $(tail -1 "$HTML")"
for tag in article ul li p h2 h3; do
  # Braces are load-bearing: "<$tag[ >]" reads as an array subscript to the shell's parser.
  o=$(grep -o "<${tag}[ >]" "$HTML" | wc -l | tr -d ' ')
  c=$(grep -o "</${tag}>" "$HTML" | wc -l | tr -d ' ')
  check "$([[ "$o" -eq "$c" ]] && echo 0 || echo 1)" \
    "<$tag> tags balance ($o open / $c close)" "unbalanced <$tag>"
done

# Issue #3's title and body carry markup. It must arrive escaped — the fixture's only
# <script string is the escaped one, so a raw match means the escaping failed.
check "$(grep -qF '&lt;script&gt;alert(1)&lt;/script&gt;' "$HTML" && echo 0 || echo 1)" \
  "attacker-supplied markup is HTML-escaped" "no escaped script tag found in $HTML"
check "$(grep -qF '&amp;' "$HTML" && echo 0 || echo 1)" \
  "ampersands are escaped" "no &amp; found in $HTML"

# ---------------------------------------------------------------- H4 --------
echo "H4 — an empty queue still yields a valid page, not a broken or absent one"

w="$TMPROOT/h4"
make_gh "$w" 0 '[]' ""
HTML="$w/empty.html"
run_case "$w" --html "$HTML" o/r
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "--html on an empty queue exits 0" "got $RC"
check "$([[ -s "$HTML" ]] && echo 0 || echo 1)" "--html on an empty queue still writes a page" "no file at $HTML"
check "$(grep -qF 'Nothing is waiting' "$HTML" && echo 0 || echo 1)" \
  "the empty page says so positively" "page: $(cat "$HTML" 2>/dev/null | head -c 200)"
check "$(grep -qF '<article' "$HTML" 2>/dev/null && echo 1 || echo 0)" \
  "the empty page renders no cards" "a card was rendered"

# ============================================================================
# The FLOW model and its two renderings — `--flow` and `--flow --html` (issue #224
# follow-up).
#
# Hermetic by the same rule as everything above: `statusgen` and `deskboard` are stubbed on
# PATH, so no board, no historian, no gh token and no network are involved. The stubs' JSON
# IS the fixture — it is the seam this code actually consumes. assay-inbox.sh never parses a
# STATUS.md or a roster file; it arranges what those readers emit. A fixture board on disk
# would be testing statusgen, which has its own suite for that, and would leave the one
# behaviour that IS this script's (what happens when a reader fails) untested, because a
# fixture board cannot make a reader refuse.
#
# Every case here is red against the pre-flow script, which had no --flow and rejected it as
# an unknown option (exit 1) — see the PR's Fail-first section.
# ============================================================================

# make_flow_tools <dir> — stub `statusgen` and `deskboard` (and a `gh` the script's
# precondition check needs) into <dir>. Fixtures are files the stubs read:
#
#   <dir>/bn-<key>.json   statusgen --root <path> --bottleneck --json
#   <dir>/id-<key>.json   statusgen --root <path> --intake-debt --json
#   <dir>/nf-<key>.json   statusgen --root <path> --net-flow --json
#   <dir>/tp.json         deskboard throughput
#
# where <key> is the basename of the --root path. An ABSENT fixture makes the stub exit
# non-zero with a diagnostic — which is exactly how the real binaries behave when the
# installed build predates the flag, and is how the could-not-check cases are set up.
make_flow_tools() {
  local dir="$1"
  mkdir -p "$dir"
  {
    printf '#!/usr/bin/env bash\n'
    printf 'root="."; mode=""\n'
    printf 'while [ $# -gt 0 ]; do case "$1" in\n'
    printf '  --root) root="$2"; shift 2 ;;\n'
    printf '  --bottleneck) mode=bn; shift ;;\n'
    printf '  --intake-debt) mode=id; shift ;;\n'
    printf '  --net-flow) mode=nf; shift ;;\n'
    printf '  *) shift ;;\n'
    printf 'esac; done\n'
    printf 'f=%s/$mode-$(basename "$root").json\n' "$(printf '%q' "$dir")"
    printf 'if [ ! -f "$f" ]; then echo "flag provided but not defined: -$mode" >&2; exit 2; fi\n'
    printf 'cat "$f"\n'
  } > "$dir/statusgen"
  {
    printf '#!/usr/bin/env bash\n'
    printf 'f=%s/tp.json\n' "$(printf '%q' "$dir")"
    printf 'if [ ! -f "$f" ]; then echo %s >&2; exit 5; fi\n' \
      "$(printf '%q' 'refused: unknown subcommand "throughput" (see --help)')"
    printf 'cat "$f"\n'
  } > "$dir/deskboard"
  chmod +x "$dir/statusgen" "$dir/deskboard"
  make_gh "$dir" 0 '[]' ""
}

# board_fixture <dir> <key> <todo> <inprog> <impl> <verified> <done> [constraint]
board_fixture() {
  jq -nc --arg c "${8:-implemented}" \
    --argjson t "$3" --argjson p "$4" --argjson i "$5" --argjson v "$6" --argjson d "$7" '
    {generated:"2026-08-30T00:00:00Z", date:"2026-08-30", note:"diagnostic",
     stages:[{stage:"todo",label:"todo→in-progress",wip:$t,median_dwell:"2d",median_dwell_seconds:172800,unknown_dwell:0,score:1},
             {stage:"in-progress",label:"in-progress→implemented",wip:$p,median_dwell:"1d",median_dwell_seconds:86400,unknown_dwell:0,score:1},
             {stage:"implemented",label:"implemented→verified",wip:$i,median_dwell:"5d",median_dwell_seconds:432000,unknown_dwell:0,score:9},
             {stage:"verified",label:"verified→done",wip:$v,median_dwell:"1d",median_dwell_seconds:86400,unknown_dwell:0,score:1},
             {stage:"done",label:"done (exited)",wip:$d,median_dwell:"",median_dwell_seconds:0,unknown_dwell:0,score:0},
             {stage:"blocked",label:"blocked",wip:0,median_dwell:"",median_dwell_seconds:0,unknown_dwell:0,score:0}],
     constraint:$c, constraint_label:"implemented→verified", shifted_from:"", action:"Verify-bound.",
     total_briefs:($t+$p+$i+$v+$d)}' > "$1/bn-$2.json"
}

# throughput_fixture <dir> <bottleneck> [advice]
throughput_fixture() {
  jq -nc --arg b "$2" --arg adv "${3:-review is the bottleneck (9 waiting / 3 slots = 3.00).}" '
    {asOf:"2026-08-30T00:00:00Z", stale:false, auditReset:false,
     mergeNowCount:0, mergeNowDecay:false, mergeNowThreshold:"20m0s",
     stages:[
       {stage:"dispatch", loop:"worker-desk", depth:5, slots:8, slotsSource:"shipped default",
        ratio:0.625, maxSlots:12, boundBy:"ceiling", depthNote:"eligible, unclaimed Next-up briefs"},
       {stage:"review", loop:"pr-review-desk", depth:9, slots:3, slotsSource:"shipped default",
        ratio:3, maxSlots:8, boundBy:"ceiling", depthNote:"open PRs with no verdict at head"},
       {stage:"verify", loop:"verify-desk", depth:11, slots:4, slotsSource:"shipped default",
        ratio:2.75, maxSlots:6, boundBy:"ceiling", depthNote:"briefs at implemented awaiting a verifier"},
       {stage:"intake", loop:"intake-desk", depth:null, slots:1, slotsSource:"shipped default",
        ratio:null, maxSlots:1, boundBy:"ceiling",
        blind:"depth: not read — `issueboard intake` owns this population",
        depthNote:"the raw-intake queue"}],
     bottleneck:$b, stagesRead:3, stagesTotal:4,
     roots:[{repo:"example-org/app", path:"."}], advice:$adv}' > "$1/tp.json"
}

# flow_cell_fixtures <dir> <key> — the intake + net-flow readers for one cell.
flow_cell_fixtures() {
  jq -nc '{state:"measured", untriaged:4, over_threshold:1, threshold_days:3, oldest_days:9}' \
    > "$1/id-$2.json"
  jq -nc '{generated:"2026-08-30T00:00:00Z", window:{since:"2026-08-23", until:"2026-08-30"},
           state:"ok",
           streams:[{stream:"alpha", arrivals:6, completions:5, net_flow:1, backlog:7, stalled:false},
                    {stream:"beta",  arrivals:2, completions:1, net_flow:1, backlog:3, stalled:false}]}' \
    > "$1/nf-$2.json"
}

# flow_row <output> <stage> — the rendered table row for one stage, whitespace-squeezed.
flow_row() {
  printf '%s\n' "$1" | grep -E "^  $2( \*)? " | head -1 | tr -s ' '
}

# ---------------------------------------------------------------- FL1 -------
echo "FL1 — --flow renders every stage with the count, queue, slots and ratio its reader gave"

w="$TMPROOT/fl1"
make_flow_tools "$w"
mkdir -p "$w/alpha"
board_fixture "$w" alpha 7 3 11 2 40
flow_cell_fixtures "$w" alpha
throughput_fixture "$w" review
run_case "$w" --flow --root "$w/alpha"
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "--flow with every reader healthy exits 0" "got $RC (stderr: ${ERR:-<empty>})"

# The seven stages, in pipeline order, exactly once each.
order=$(printf '%s\n' "$OUT" | grep -Eo '^  (intake|todo|in-progress|review|implemented|verified|done)( \*)? ' \
        | sed 's/ \*//; s/^ *//; s/ *$//' | tr '\n' ' ')
check "$([[ "$order" == "intake todo in-progress review implemented verified done " ]] && echo 0 || echo 1)" \
  "the seven stages render in pipeline order" "got: $order"

check "$([[ "$(flow_row "$OUT" todo)" == " todo 7 5 8 0.63 2d "* ]] && echo 0 || echo 1)" \
  "todo: board WIP 7, loop queue 5, 8 slots, ratio 0.63" "row: $(flow_row "$OUT" todo)"
check "$([[ "$(flow_row "$OUT" implemented)" == " implemented 11 11 4 2.75 5d "* ]] && echo 0 || echo 1)" \
  "implemented: WIP 11 against the verify loop's 11/4" "row: $(flow_row "$OUT" implemented)"
check "$([[ "$(flow_row "$OUT" intake)" == " intake 4 "* ]] && echo 0 || echo 1)" \
  "intake count comes from --intake-debt (.untriaged = 4)" "row: $(flow_row "$OUT" intake)"
check "$([[ "$(flow_row "$OUT" review)" == " review * 9 9 3 3 "* ]] && echo 0 || echo 1)" \
  "review count is the throughput depth (9), not a board status" "row: $(flow_row "$OUT" review)"
# in-progress/verified/done have no loop: absent capacity is stated, never faked.
check "$(contains "$(flow_row "$OUT" in-progress)" "no loop owns this stage" && echo 0 || echo 1)" \
  "a stage with no pool says so instead of showing a slot count" "row: $(flow_row "$OUT" in-progress)"

check "$(contains "$OUT" "arrivals 8 -> ... -> completions 6" && echo 0 || echo 1)" \
  "window flow sums the net-flow streams (6+2 in, 5+1 out)" "stdout: $OUT"
check "$(contains "$OUT" "dwell-weighted constraint (ToC): implemented" && echo 0 || echo 1)" \
  "the ToC constraint is reported beside the ratio bottleneck, not instead of it" "stdout: $OUT"

# ---------------------------------------------------------------- FL2 -------
echo "FL2 — the bottleneck is throughput's stage, translated into the flow vocabulary"

check "$(contains "$OUT" "bottleneck (largest queue/slots ratio): review *" && echo 0 || echo 1)" \
  "throughput's 'review' marks the review stage" "stdout: $OUT"
marks=$(printf '%s\n' "$OUT" | grep -Ec '^  [a-z-]+ \* ' || true)
check "$([[ "$marks" -eq 1 ]] && echo 0 || echo 1)" \
  "exactly one stage is marked as the bottleneck" "marked rows: $marks"

# `verify` is throughput's name for the stage this model calls `implemented`. Translating it
# is the whole reason the mapping exists — an untranslated verdict would mark no stage at all
# and silently drop the signal.
w="$TMPROOT/fl2"
make_flow_tools "$w"
mkdir -p "$w/alpha"
board_fixture "$w" alpha 7 3 11 2 40
flow_cell_fixtures "$w" alpha
throughput_fixture "$w" verify "verify is the bottleneck (11 waiting / 4 slots = 2.75)."
run_case "$w" --flow --root "$w/alpha"
check "$(contains "$(flow_row "$OUT" implemented)" " implemented * " && echo 0 || echo 1)" \
  "throughput's 'verify' marks the flow model's 'implemented'" "row: $(flow_row "$OUT" implemented)"
check "$(contains "$OUT" "bottleneck (largest queue/slots ratio): implemented *" && echo 0 || echo 1)" \
  "the headline names the translated stage" "stdout: $OUT"

# ---------------------------------------------------------------- FL3 -------
echo "FL3 — a reader that did not run leaves could-not-check, never 0"

w="$TMPROOT/fl3"
make_flow_tools "$w"
mkdir -p "$w/alpha"
# No bn-alpha.json and no tp.json: the board reader and the throughput reader both refuse,
# exactly as a pinned binary predating the flag does.
flow_cell_fixtures "$w" alpha
run_case "$w" --flow --root "$w/alpha"
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" \
  "a failed flow reader exits 2 (the render is INCOMPLETE)" "got $RC"
for stage in todo in-progress implemented verified "done"; do
  row=$(flow_row "$OUT" "$stage")
  check "$(contains "$row" "could-not-check" && echo 0 || echo 1)" \
    "$stage renders could-not-check when its reader refused" "row: ${row:-<none>}"
  # The whole point: an unread stage must not be presented as a drained one.
  check "$([[ "$row" == " $stage 0 "* ]] && echo 1 || echo 0)" \
    "$stage is NOT rendered as 0" "row: $row"
done
check "$(contains "$OUT" "flag provided but not defined" && echo 0 || echo 1)" \
  "the reader's own diagnostic is carried onto the render" "stdout: $OUT"
check "$(contains "$ERR" "FLOW READER FAILED" && echo 0 || echo 1)" \
  "the failure is named on stderr too" "stderr: ${ERR:-<empty>}"
check "$(contains "$OUT" "bottleneck (largest queue/slots ratio): none named" && echo 0 || echo 1)" \
  "no bottleneck is named when throughput could not be read" "stdout: $OUT"
check "$(contains "$OUT" "Blind is not idle" && echo 0 || echo 1)" \
  "the advice line says blind is not idle instead of reporting a drained pipeline" "stdout: $OUT"
# The readable stage still renders — one dead reader must not take the others down.
check "$(contains "$(flow_row "$OUT" intake)" " intake 4 " && echo 0 || echo 1)" \
  "a healthy reader still reports beside a failed one" "row: $(flow_row "$OUT" intake)"

# ---------------------------------------------------------------- FL4 -------
echo "FL4 — --flow --html is a self-contained page with an inline SVG diagram"

w="$TMPROOT/fl4"
make_flow_tools "$w"
mkdir -p "$w/alpha"
board_fixture "$w" alpha 7 3 11 2 40
flow_cell_fixtures "$w" alpha
# Hostile text arriving through a reader's own output must be escaped like any other.
throughput_fixture "$w" review '<script>alert(1)</script> & widen review'
HTML="$w/flow.html"
run_case "$w" --flow --html "$HTML" --root "$w/alpha"
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "--flow --html exits 0 on healthy readers" "got $RC (stderr: ${ERR:-<empty>})"
check "$([[ -s "$HTML" ]] && echo 0 || echo 1)" "--flow --html writes a non-empty file" "no file at $HTML"

svgs=$(grep -c '<svg class="flow"' "$HTML" || true)
check "$([[ "$svgs" -eq 1 ]] && echo 0 || echo 1)" "the page carries exactly one inline SVG" "count: $svgs"
boxes=$(grep -c '<rect class="box' "$HTML" || true)
check "$([[ "$boxes" -eq 7 ]] && echo 0 || echo 1)" "one box per stage (got $boxes, want 7)" "boxes: $boxes"
bn=$(grep -c '<rect class="box bneck"' "$HTML" || true)
check "$([[ "$bn" -eq 1 ]] && echo 0 || echo 1)" "exactly one box is highlighted as the bottleneck" "count: $bn"
arrows=$(grep -c '<line class="arr' "$HTML" || true)
check "$([[ "$arrows" -eq 6 ]] && echo 0 || echo 1)" "six arrows join the seven stages" "arrows: $arrows"
# The measured arrows are solid and weighted; the steps no reader measures are dashed.
check "$(grep -q '<line class="arr" [^>]*stroke-width="[2-9]' "$HTML" && echo 0 || echo 1)" \
  "a measured arrow is weighted by its flow" "no weighted arrow in $HTML"
check "$(grep -q '<line class="arr dash"' "$HTML" && echo 0 || echo 1)" \
  "an unmeasured step is drawn dashed" "no dashed arrow in $HTML"

# Cards belong to the decision queue; the flow-only page carries none.
check "$(grep -qF '<article' "$HTML" && echo 1 || echo 0)" \
  "--flow --html renders the flow section ALONE (no decision cards)" "a card was rendered"

# The SVG is never the only carrier: the same numbers follow as a table.
check "$(grep -q '<caption>Stage counts' "$HTML" && echo 0 || echo 1)" \
  "an equivalent data table accompanies the diagram" "no table caption in $HTML"
check "$(grep -q 'role="img"' "$HTML" && echo 0 || echo 1)" \
  "the SVG is labelled for assistive technology" "no role=img in $HTML"
check "$(grep -q '<desc id="flowd">' "$HTML" && echo 0 || echo 1)" \
  "the SVG carries a description, not just a title" "no <desc> in $HTML"

for forbidden in "<script" " src=" "@import" "url(" "<iframe" "<link "; do
  check "$(grep -qF -- "$forbidden" "$HTML" && echo 1 || echo 0)" \
    "the flow page contains no '$forbidden'" "found '$forbidden' in $HTML"
done
hrefs=$(grep -c 'href=' "$HTML" || true)
check "$([[ "$hrefs" -eq 0 ]] && echo 0 || echo 1)" \
  "the flow page fetches nothing — it carries no links at all" "hrefs: $hrefs"
check "$(contains "$(cat "$HTML")" "prefers-color-scheme" && echo 0 || echo 1)" \
  "light/dark is handled with prefers-color-scheme" "no media query in $HTML"

check "$([[ "$(head -1 "$HTML")" == "<!doctype html>" ]] && echo 0 || echo 1)" \
  "the flow page opens with a doctype" "first line: $(head -1 "$HTML")"
check "$([[ "$(tail -1 "$HTML")" == "</html>" ]] && echo 0 || echo 1)" \
  "the flow page closes </html>" "last line: $(tail -1 "$HTML")"
for tag in section svg table thead tbody tr td th ul li p h2 h3 g text; do
  o=$(grep -o "<${tag}[ >]" "$HTML" | wc -l | tr -d ' ')
  c=$(grep -o "</${tag}>" "$HTML" | wc -l | tr -d ' ')
  check "$([[ "$o" -eq "$c" ]] && echo 0 || echo 1)" \
    "<$tag> tags balance ($o open / $c close)" "unbalanced <$tag>"
done

check "$(grep -qF '&lt;script&gt;alert(1)&lt;/script&gt;' "$HTML" && echo 0 || echo 1)" \
  "reader text is HTML-escaped on the page" "no escaped script tag found in $HTML"

# The honesty line is part of the deliverable, not decoration: the page must say it renders
# authored status rather than implying it measured anything.
check "$(grep -qF 'RENDERS AUTHORED STATUS' "$HTML" && echo 0 || echo 1)" \
  "the page states that it renders authored status, not a measurement" "no honesty line in $HTML"
check "$(grep -q 'Where these numbers come from' "$HTML" && echo 0 || echo 1)" \
  "the page names its sources" "no sources section in $HTML"

# ---------------------------------------------------------------- FL5 -------
echo "FL5 — could-not-check reaches the PAGE, not only the terminal"

w="$TMPROOT/fl5"
make_flow_tools "$w"
mkdir -p "$w/alpha"
flow_cell_fixtures "$w" alpha          # no board fixture, no throughput fixture
HTML="$w/blind.html"
run_case "$w" --flow --html "$HTML" --root "$w/alpha"
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a blind flow page still exits 2" "got $RC"
check "$([[ -s "$HTML" ]] && echo 0 || echo 1)" "a blind run still writes a page" "no file at $HTML"
# Six, not five: the five board stages whose --bottleneck reader refused, PLUS `review`,
# whose count comes from the throughput reader that also refused. `intake` still reports,
# because its reader answered.
blindboxes=$(grep -c '<rect class="box blind"' "$HTML" || true)
check "$([[ "$blindboxes" -eq 6 ]] && echo 0 || echo 1)" \
  "each unread stage is drawn as a dashed could-not-check box (got $blindboxes, want 6)" \
  "blind boxes: $blindboxes"
check "$(grep -q '<h3>Could not check</h3>' "$HTML" && echo 0 || echo 1)" \
  "the page lists what it could not check" "no could-not-check section in $HTML"
check "$(grep -qF '>0<' "$HTML" && echo 1 || echo 0)" \
  "no stage is drawn as a bare 0 when nothing was read" "a zero count was rendered"
nobn=$(grep -c '<rect class="box bneck"' "$HTML" || true)
check "$([[ "$nobn" -eq 0 ]] && echo 0 || echo 1)" \
  "no bottleneck is highlighted when throughput was unreadable" "highlighted boxes: $nobn"

# ---------------------------------------------------------------- FL6 -------
echo "FL6 — more than one cell: a fleet total plus a row block per cell"

w="$TMPROOT/fl6"
make_flow_tools "$w"
mkdir -p "$w/alpha" "$w/beta"
board_fixture "$w" alpha 7 3 11 2 40
board_fixture "$w" beta  4 1  6 3 20
flow_cell_fixtures "$w" alpha
flow_cell_fixtures "$w" beta
throughput_fixture "$w" review
run_case "$w" --flow --root "$w/alpha" --root "$w/beta"
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "a two-cell --flow exits 0" "got $RC (stderr: ${ERR:-<empty>})"
check "$([[ "$(flow_row "$OUT" todo)" == " todo 11 "* ]] && echo 0 || echo 1)" \
  "the fleet row sums the cells (7 + 4 = 11)" "row: $(flow_row "$OUT" todo)"
check "$(contains "$OUT" "== cell alpha" && contains "$OUT" "== cell beta" && echo 0 || echo 1)" \
  "each cell gets its own row block" "stdout: $OUT"
blocks=$(printf '%s\n' "$OUT" | grep -c '^== ' || true)
check "$([[ "$blocks" -eq 3 ]] && echo 0 || echo 1)" \
  "one fleet block plus one block per cell (got $blocks, want 3)" "blocks: $blocks"
# A single cell needs no per-cell repetition of the fleet row.
run_case "$TMPROOT/fl1" --flow --root "$TMPROOT/fl1/alpha"
blocks=$(printf '%s\n' "$OUT" | grep -c '^== ' || true)
check "$([[ "$blocks" -eq 1 ]] && echo 0 || echo 1)" \
  "a single cell renders the fleet row only, not the same numbers twice" "blocks: $blocks"

# A sum over cells that were not all read is a LOWER BOUND and must say so.
w="$TMPROOT/fl6-partial"
make_flow_tools "$w"
mkdir -p "$w/alpha" "$w/beta"
board_fixture "$w" alpha 7 3 11 2 40
flow_cell_fixtures "$w" alpha
flow_cell_fixtures "$w" beta            # beta has no board fixture
throughput_fixture "$w" review
run_case "$w" --flow --root "$w/alpha" --root "$w/beta"
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "one unread cell reddens the run" "got $RC"
check "$(contains "$(flow_row "$OUT" todo)" "AT LEAST" && echo 0 || echo 1)" \
  "a partial sum is flagged AT LEAST, never printed as a total" "row: $(flow_row "$OUT" todo)"

# ---------------------------------------------------------------- FL7 -------
echo "FL7 — the decision page carries the Flow section; the modes refuse sensibly"

w="$TMPROOT/fl7"
make_flow_tools "$w"
board_fixture "$w" "$(basename "$TMPROOT/fl7")" 7 3 11 2 40
throughput_fixture "$w" review
HTML="$w/both.html"
(cd "$w" && PATH="$w:$PATH" bash "$SCRIPT" --html "$HTML" o/r >"$w/o" 2>"$w/e")
RC=$?
check "$([[ "$RC" -eq 0 || "$RC" -eq 2 ]] && echo 0 || echo 1)" \
  "--html runs with the flow section attached" "got $RC: $(cat "$w/e")"
check "$(grep -q '<section class="flow-sec">' "$HTML" && echo 0 || echo 1)" \
  "the decision page carries a Flow section" "no flow section in $HTML"
check "$(grep -q '<h1>Decisions waiting on the driver</h1>' "$HTML" && echo 0 || echo 1)" \
  "the decision page keeps its own heading" "heading missing from $HTML"

run_case "$TMPROOT/fl1" --walk --flow o/r
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" \
  "--walk with --flow is refused, not silently resolved" "got $RC"
run_case "$TMPROOT/fl1" --flow --since notadate --root "$TMPROOT/fl1/alpha"
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "a malformed --since exits 1" "got $RC"
check "$(contains "$ERR" "YYYY-MM-DD" && echo 0 || echo 1)" \
  "stderr says what --since should look like" "stderr: ${ERR:-<empty>}"
run_case "$TMPROOT/fl1" --root
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "--root with no path exits 1" "got $RC"

# ------------------------------------------------------------------ done ----
echo
printf '%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]] || exit 1
