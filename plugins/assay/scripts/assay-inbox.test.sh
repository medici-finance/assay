#!/usr/bin/env bash
# assay-inbox.test.sh — regression suite for assay-inbox.sh.
#
# Every case here pins a finding from the assay-toolkit#344 review. Each one was
# REPRODUCED against the pre-fix script first and FAILS without its fix (13 of these
# assertions go red against PR head 1737db8):
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
#       the delta review's findings A and B) are red against the CHANGES_REQUESTED head
#       f7fc714 pre-fix.
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
run_case "$w" medici-finance/assay-toolkit
check "$([[ "$RC" -ne 0 ]] && echo 0 || echo 1)" \
  "gh failure => non-zero exit (got $RC)" \
  "exit was 0 — a dead query reads as an empty inbox"
check "$(contains "$ERR" "Bad credentials" && echo 0 || echo 1)" \
  "gh's own diagnostic reaches stderr" "stderr was: ${ERR:-<empty>}"
check "$(contains "$OUT" "FAILED" && echo 0 || echo 1)" \
  "stdout summary flags the run as incomplete" "stdout was: ${OUT:-<empty>}"

w="$TMPROOT/f1-empty"
make_gh "$w" 0 '[]' ""
run_case "$w" medici-finance/assay-toolkit
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

# ------------------------------------------------------------------ done ----
echo
printf '%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]] || exit 1
