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

# ------------------------------------------------------------------ done ----
echo
printf '%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]] || exit 1
