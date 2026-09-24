#!/usr/bin/env bash
# Offline, network-free unit test for tools/renew-fleet-gitlab-tokens.sh.
#
# It puts a fake `glab` first on PATH. The fake logs every invocation (argv,
# the pinned GITLAB_HOST, any --input request body) and answers from one
# responder whose behaviour each case steers with environment knobs, so the
# script's real control flow — preflight, both authority models, rotate /
# create / refuse, the atomic custody write — runs end to end without a
# GitLab, a credential, or a network.
#
# No cluster, no GitLab, no toolchain beyond bash + jq.
#
# Default impl is ../renew-fleet-gitlab-tokens.sh. Point RENEW_IMPL at another
# version to see the behaviours it lacks fail — the fail-first evidence. The
# issue's own prototype (instance-admin only, no group-Owner path, no
# preflight of every role before the first rotation) goes RED on the group,
# duplicate-preflight and resume cases:
#   RENEW_IMPL=/path/to/prototype.sh ./tools/renew-fleet-gitlab-tokens_test.sh   # RED
#   ./tools/renew-fleet-gitlab-tokens_test.sh                                    # green
# T12 (single-source role table) goes RED against a provisioner that carries
# its own inline copy of the table:
#   git show <pre-change>:tools/create-fleet-gitlab.sh > tools/create-fleet-gitlab.sh   # RED
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMPL="${RENEW_IMPL:-$here/renew-fleet-gitlab-tokens.sh}"
case "$IMPL" in /*) ;; *) IMPL="$here/$IMPL" ;; esac

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

ROLES="reviewer worker verifier desk issue-loop intake-loop board-writer"
HOST="gitlab.example.test"

# --- the fake glab ------------------------------------------------------------
BIN="$(mktemp -d "${TMPDIR:-/tmp}/renew-fakebin-XXXXXX")"
cat > "$BIN/glab" <<'FAKEGLAB'
#!/usr/bin/env bash
set -uo pipefail
printf 'argv=%s | host=%s | cwd=%s\n' "$*" "${GITLAB_HOST:-}" "$PWD" >> "$FAKE_GLAB_LOG"
sub="${1:-}"; [ $# -gt 0 ] && shift
method="GET"; endpoint=""; input=""; user=""; pos=""; KEY=""
case "$sub" in
  auth) KEY="auth ${1:-}" ;;
  api)
    while [ $# -gt 0 ]; do
      case "$1" in
        --hostname|-H|--header|-f|--raw-field|-F|--field|--output) shift 2 ;;
        -X|--method) method="$2"; shift 2 ;;
        --input) input="$2"; shift 2 ;;
        -*) shift ;;
        *) endpoint="$1"; shift ;;
      esac
    done
    KEY="api ${method} ${endpoint}" ;;
  token)
    tsub="${1:-}"; [ $# -gt 0 ] && shift
    while [ $# -gt 0 ]; do
      case "$1" in
        -U|--user) user="$2"; shift 2 ;;
        -D|--duration|-F|--output|-S|--scope|-E|--expires-at|-A|--access-level|-g|--group|-R|--repo|--description|--jq) shift 2 ;;
        -*) shift ;;
        *) pos="$1"; shift ;;
      esac
    done
    KEY="token ${tsub} ${pos} user=${user}" ;;
esac
if [ -n "$input" ]; then
  printf 'BODY %s | %s\n' "$KEY" "$(tr -d '\n ' < "$input")" >> "$FAKE_GLAB_LOG"
fi
# shellcheck disable=SC1090
. "$FAKE_GLAB_RESPONDER"
respond "$KEY"
FAKEGLAB
chmod +x "$BIN/glab"

# --- the responder -----------------------------------------------------------
# One responder for every case; each case steers it with these knobs:
#   MEMBER_LEVEL=<n>        caller's access level in the group (default 50)
#   ADMIN_OK=0              application/settings answers 403
#   AUTH_FAIL=1             glab auth status fails
#   NONE_ROLE=<role>        that role has no PAT of its name (create fallback)
#   DUP_ROLE=<role>         that role has TWO active PATs of its name
#   BAD_SECRET_ROLE=<role>  that role's rotate/create output is malformed
#   BAD_LIST_ROLE=<role>    that role's PAT listing is malformed
#   FAIL_ONCE_ROLE=<role>   that role's first rotate exits 1 (partial run)
RESPONDER="$(mktemp "${TMPDIR:-/tmp}/renew-responder-XXXXXX")"
cat > "$RESPONDER" <<'RESP'
R_LIST="reviewer worker verifier desk issue-loop intake-loop board-writer"
bump() { local f="$STATE/$1"; local n=0; [ -f "$f" ] && n=$(cat "$f"); n=$((n+1)); echo "$n" > "$f"; printf "%s" "$n"; }
uid_of_role() { local n=100 r; for r in $R_LIST; do n=$((n+1)); [ "$r" = "$1" ] && { echo "$n"; return; }; done; echo 0; }
role_of_uid() { local n=100 r; for r in $R_LIST; do n=$((n+1)); [ "$n" = "$1" ] && { echo "$r"; return; }; done; }
role_of_user() { local u="${1#myorg-}"; echo "${u%-bot}"; }
secret_for() { printf 'glpat-TESTSECRET-%s-%s-0123456789' "$1" "$2"; }

pat_list() {  # pat_list ROLE STRINGY
  local r="$1" id j
  id=$(( $(uid_of_role "$r") + 400 ))
  j="[{\"id\":9${id},\"name\":\"unrelated-${r}\",\"active\":true,\"revoked\":false}"
  if [ "$r" != "${NONE_ROLE:-}" ]; then
    j="${j},{\"id\":${id},\"name\":\"assay-${r}-fleet\",\"active\":true,\"revoked\":false}"
  fi
  if [ "$r" = "${DUP_ROLE:-}" ]; then
    j="${j},{\"id\":$((id+1000)),\"name\":\"assay-${r}-fleet\",\"active\":true,\"revoked\":false}"
  fi
  j="${j}]"
  if [ "$r" = "${BAD_LIST_ROLE:-}" ]; then j='[{"name":"assay-'"$r"'-fleet","active":true}]'; fi
  if [ "$2" = "1" ]; then
    printf '%s' "$j" | jq 'map(with_entries(.value |= tostring))'
  else
    printf '%s\n' "$j"
  fi
}

fail_once() {  # fail_once ROLE -> 0 when this call must fail
  [ "$1" = "${FAIL_ONCE_ROLE:-}" ] && [ "$(bump "failonce-$1")" = "1" ]
}

respond() {
  local k="$1" uid r
  case "$k" in
    "auth status")
      if [ "${AUTH_FAIL:-0}" = "1" ]; then echo "not logged in" >&2; return 1; fi
      return 0 ;;
    "api GET groups/example") echo '{"id":1,"parent_id":null,"full_path":"example"}'; return 0 ;;
    "api GET groups/example-sub") echo '{"id":2,"parent_id":1,"full_path":"example/sub"}'; return 0 ;;
    "api GET user") echo '{"id":7,"username":"owner"}'; return 0 ;;
    "api GET groups/1/members/all/7") echo "{\"id\":7,\"access_level\":${MEMBER_LEVEL:-50}}"; return 0 ;;
    "api GET application/settings")
      if [ "${ADMIN_OK:-1}" != "1" ]; then echo "ERROR: 403 Forbidden" >&2; return 1; fi
      echo '{"signup_enabled":false}'; return 0 ;;
    "api GET groups/1/service_accounts")
      local out="[" n=100
      for r in $R_LIST; do n=$((n+1)); out="${out}{\"id\":${n},\"username\":\"myorg-${r}-bot\"},"; done
      echo "${out}{\"id\":999,\"username\":\"someone-else-bot\"}]"; return 0 ;;
    "api GET users?username="*)
      local u="${k#api GET users?username=}"
      r=$(role_of_user "$u")
      echo "[{\"id\":$(uid_of_role "$r"),\"username\":\"${u}\"}]"; return 0 ;;
    "api GET groups/1/service_accounts/"*"/personal_access_tokens?state=active")
      uid="${k#api GET groups/1/service_accounts/}"; uid="${uid%%/*}"
      pat_list "$(role_of_uid "$uid")" 0; return 0 ;;
    "api POST groups/1/service_accounts/"*"/personal_access_tokens/"*"/rotate")
      uid="${k#api POST groups/1/service_accounts/}"; uid="${uid%%/*}"; r=$(role_of_uid "$uid")
      if fail_once "$r"; then echo "ERROR: 500 boom" >&2; return 1; fi
      if [ "$r" = "${BAD_SECRET_ROLE:-}" ]; then echo "{\"name\":\"assay-${r}-fleet\",\"token\":null}"; return 0; fi
      echo "{\"id\":$(( uid + 700 )),\"name\":\"assay-${r}-fleet\",\"active\":true,\"token\":\"$(secret_for rotated "$r")\"}"; return 0 ;;
    "api POST groups/1/service_accounts/"*"/personal_access_tokens")
      uid="${k#api POST groups/1/service_accounts/}"; uid="${uid%%/*}"; r=$(role_of_uid "$uid")
      if [ "$r" = "${BAD_SECRET_ROLE:-}" ]; then echo "{\"name\":\"assay-${r}-fleet\"}"; return 0; fi
      echo "{\"id\":$(( uid + 800 )),\"name\":\"assay-${r}-fleet\",\"active\":true,\"token\":\"$(secret_for created "$r")\"}"; return 0 ;;
    "token list  user="*)
      r=$(role_of_user "${k#token list  user=}")
      pat_list "$r" 1; return 0 ;;
    "token rotate "*)
      r=$(role_of_user "${k##*user=}")
      if fail_once "$r"; then echo "ERROR: 500 boom" >&2; return 1; fi
      if [ "$r" = "${BAD_SECRET_ROLE:-}" ]; then printf 'glpat-first-line-0123456789\nsecond line\n'; return 0; fi
      printf '%s\n' "$(secret_for rotated "$r")"; return 0 ;;
    "token create "*)
      r=$(role_of_user "${k##*user=}")
      if [ "$r" = "${BAD_SECRET_ROLE:-}" ]; then printf '\n'; return 0; fi
      printf '%s\n' "$(secret_for created "$r")"; return 0 ;;
  esac
  echo "unstubbed: $k" >&2
  return 1
}
RESP

newcase() {
  CASEDIR="$(mktemp -d "${TMPDIR:-/tmp}/renew-case-XXXXXX")"
  OUTDIR="$CASEDIR/out"; mkdir -p "$OUTDIR"
  STATE="$CASEDIR/state"; mkdir -p "$STATE"
  export FAKE_GLAB_LOG="$CASEDIR/glab.log"; : > "$FAKE_GLAB_LOG"
  export FAKE_GLAB_RESPONDER="$RESPONDER"
  export STATE
  unset MEMBER_LEVEL ADMIN_OK AUTH_FAIL NONE_ROLE DUP_ROLE BAD_SECRET_ROLE BAD_LIST_ROLE FAIL_ONCE_ROLE
  for r in $ROLES; do printf 'old-%s' "$r" > "$OUTDIR/gitlab-$r.token"; chmod 600 "$OUTDIR/gitlab-$r.token"; done
}

run_impl() {  # run_impl <args...>; captures $OUT and $RC
  OUT="$(PATH="$BIN:$PATH" bash "$IMPL" "$@" 2>&1)"
  RC=$?
  if [ "${RENEW_TEST_DEBUG:-0}" = "1" ]; then
    echo "--- run: $* (rc=$RC)"; echo "$OUT"; echo "--- log:"; cat "$FAKE_GLAB_LOG"; echo "---"
  fi
}

has()  { case "$OUT" in *"$1"*) return 0 ;; *) return 1 ;; esac; }
logn() { grep -c -- "$1" "$FAKE_GLAB_LOG" 2>/dev/null || true; }
mutations() {  # every call that can mint or rotate a credential
  grep -cE 'argv=api .*-X POST|argv=token (rotate|create)' "$FAKE_GLAB_LOG" 2>/dev/null || true
}
file_is() { [ -f "$1" ] && [ "$(cat "$1")" = "$2" ]; }
mode600() { [ -n "$(find "$1" -prune -perm 600 2>/dev/null)" ]; }
no_secret_leak() {  # no secret in stdout/stderr, glab argv, or a leftover temp file
  ! printf '%s' "$OUT" | grep -q 'TESTSECRET' \
    && ! grep -q '^argv=.*TESTSECRET' "$FAKE_GLAB_LOG" \
    && [ -z "$(find "$OUTDIR" -name '.renew-*' 2>/dev/null)" ]
}
all_old() { for r in $ROLES; do file_is "$OUTDIR/gitlab-$r.token" "old-$r" || return 1; done; }

GROUP_ARGS=(--hostname "$HOST" --group example --prefix myorg)
ADMIN_ARGS=(--hostname "$HOST" --instance-admin --prefix myorg)

# =============================================================================
# T1 — group Owner: every role's live PAT is ROTATED via the group
#      service-account endpoint and its custody file replaced, 0600.
# =============================================================================
newcase
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
rot=$(logn 'argv=api .*-X POST .*personal_access_tokens/[0-9]*/rotate')
if [ "$rot" = "7" ] && [ "$RC" = "0" ]; then ok "T1 group mode rotates all seven role PATs (rc=0)"; else bad "T1 group mode rotates all seven role PATs (rotations=$rot rc=$RC)"; fi
allok=1
for r in $ROLES; do
  file_is "$OUTDIR/gitlab-$r.token" "glpat-TESTSECRET-rotated-$r-0123456789" && mode600 "$OUTDIR/gitlab-$r.token" || allok=0
done
if [ "$allok" = "1" ]; then ok "T1 each gitlab-<role>.token holds its new secret, no newline, mode 0600"; else bad "T1 each gitlab-<role>.token holds its new secret at 0600"; fi
if grep -q 'argv=api .*personal_access_tokens/501/rotate' "$FAKE_GLAB_LOG" && ! grep -q '/rotate.*9501\|9501/rotate' "$FAKE_GLAB_LOG"; then
  ok "T1 the rotation targets the PAT matched by its stable name, not an unrelated one"
else
  bad "T1 the rotation targets the PAT matched by its stable name"
fi
if grep -q 'BODY api POST .*/rotate | {"expires_at":"[0-9-]*"}' "$FAKE_GLAB_LOG"; then ok "T1 the rotate body carries expires_at"; else bad "T1 the rotate body carries expires_at"; fi
if [ "$(grep -c "host=${HOST} " "$FAKE_GLAB_LOG")" = "$(grep -c '^argv=' "$FAKE_GLAB_LOG")" ] && [ "$(logn 'argv=api --hostname '"$HOST")" -ge 1 ]; then
  ok "T1 every glab call is pinned to --hostname (GITLAB_HOST + --hostname on api)"
else
  bad "T1 every glab call is pinned to --hostname"
fi
if grep -q 'token list\|token rotate\|application/settings' "$FAKE_GLAB_LOG"; then bad "T1 group mode must not use the admin-only glab token --user path"; else ok "T1 group mode never touches the admin-only glab token --user path"; fi
if no_secret_leak; then ok "T1 no secret in output, glab argv, or a leftover temp file"; else bad "T1 no secret in output, glab argv, or a leftover temp file"; fi
if has "role=reviewer path=${OUTDIR}/gitlab-reviewer.token outcome=rotated"; then ok "T1 the report line is role + path + outcome"; else bad "T1 the report line is role + path + outcome"; fi

# =============================================================================
# T2 — instance admin: glab token list/rotate --user, rotate by token id.
# =============================================================================
newcase
run_impl "${ADMIN_ARGS[@]}" --out-dir "$OUTDIR" --duration 2w
rot=$(logn 'argv=token rotate [0-9]* --user myorg-.*-bot --duration 14d --output text')
if [ "$rot" = "7" ] && [ "$RC" = "0" ]; then ok "T2 admin mode rotates all seven via glab token rotate <id> --user (rc=0, 2w -> 14d)"; else bad "T2 admin mode rotates via glab token rotate <id> --user (rotations=$rot rc=$RC)"; fi
if logn 'argv=token list --user myorg-worker-bot --active --output json' | grep -q '^1$'; then ok "T2 admin mode lists with glab token list --user --active --output json"; else bad "T2 admin mode lists with glab token list --user --active --output json"; fi
allok=1
for r in $ROLES; do file_is "$OUTDIR/gitlab-$r.token" "glpat-TESTSECRET-rotated-$r-0123456789" && mode600 "$OUTDIR/gitlab-$r.token" || allok=0; done
if [ "$allok" = "1" ]; then ok "T2 the trailing newline glab prints is stripped; files 0600"; else bad "T2 files hold the exact secret at 0600"; fi
if grep -q 'service_accounts' "$FAKE_GLAB_LOG"; then bad "T2 admin mode must not use the group service-account endpoints"; else ok "T2 admin mode never touches the group service-account endpoints"; fi
if no_secret_leak; then ok "T2 no secret in output, glab argv, or a leftover temp file"; else bad "T2 no secret leak"; fi

# =============================================================================
# T3 — create fallback: a role with no active PAT of its name gets one
#      created with the SHARED table's scopes; the rest are rotated.
# =============================================================================
newcase
export NONE_ROLE=worker
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if grep -q 'BODY api POST groups/1/service_accounts/102/personal_access_tokens | {"name":"assay-worker-fleet","scopes":\["api","write_repository"\],"expires_at":"[0-9-]*"}' "$FAKE_GLAB_LOG" \
   && [ "$(logn '^argv=api .*/rotate')" = "6" ] && [ "$RC" = "0" ]; then
  ok "T3 group mode creates the missing PAT with the table's name + scopes, rotates the other six"
else
  bad "T3 group mode create fallback (rc=$RC)"
fi
if file_is "$OUTDIR/gitlab-worker.token" "glpat-TESTSECRET-created-worker-0123456789" && has "role=worker path=${OUTDIR}/gitlab-worker.token outcome=created"; then
  ok "T3 the created secret lands in gitlab-worker.token, reported as created"
else
  bad "T3 the created secret lands in gitlab-worker.token"
fi
newcase
export NONE_ROLE=verifier
run_impl "${ADMIN_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$(logn 'argv=token create assay-verifier-fleet --user myorg-verifier-bot --duration 30d --scope api --scope write_repository --output text')" = "1" ] && [ "$RC" = "0" ] \
   && file_is "$OUTDIR/gitlab-verifier.token" "glpat-TESTSECRET-created-verifier-0123456789"; then
  ok "T3 admin mode creates via glab token create <name> --user --scope ... (one --scope per table scope)"
else
  bad "T3 admin mode create fallback (rc=$RC)"
fi

# =============================================================================
# T4 — duplicate refusal: two active PATs of one name is a PREFLIGHT refusal;
#      nothing is rotated for ANY role, no file changes.
# =============================================================================
for mode in group admin; do
  newcase
  export DUP_ROLE=desk
  if [ "$mode" = "group" ]; then run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"; else run_impl "${ADMIN_ARGS[@]}" --out-dir "$OUTDIR"; fi
  m=$(mutations)
  if [ "$RC" = "1" ] && [ "$m" = "0" ] && has "refusing to guess" && all_old; then
    ok "T4 ($mode) two active PATs of one name -> refused in preflight, zero mutations, files untouched"
  else
    bad "T4 ($mode) duplicate refusal (rc=$RC mutations=$m)"
  fi
done

# =============================================================================
# T5 — authority refusal: each model refuses an identity lacking its authority,
#      before any mutation, and names the other model.
# =============================================================================
newcase
export MEMBER_LEVEL=40
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "authority refused" && has "--instance-admin" && all_old; then
  ok "T5 group mode refuses a non-Owner (Maintainer 40), zero mutations"
else
  bad "T5 group mode refuses a non-Owner (rc=$RC)"
fi
newcase
export ADMIN_OK=0
run_impl "${ADMIN_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && [ "$(logn 'argv=token ')" = "0" ] && has "authority refused" && has "--group" && all_old; then
  ok "T5 admin mode refuses a non-admin before any glab token call, zero mutations"
else
  bad "T5 admin mode refuses a non-admin (rc=$RC)"
fi
newcase
run_impl --hostname "$HOST" --group example-sub --prefix myorg --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "subgroup"; then ok "T5 a subgroup is refused (service accounts are top-level)"; else bad "T5 a subgroup is refused (rc=$RC)"; fi
newcase
export AUTH_FAIL=1
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "not authenticated"; then ok "T5 an unauthenticated glab is refused"; else bad "T5 an unauthenticated glab is refused (rc=$RC)"; fi

# =============================================================================
# T6 — malformed secret: output that is not exactly one token-shaped line
#      fails closed — nothing written for that role, the run stops there.
# =============================================================================
for mode in group admin; do
  newcase
  export BAD_SECRET_ROLE=verifier
  if [ "$mode" = "group" ]; then run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"; else run_impl "${ADMIN_ARGS[@]}" --out-dir "$OUTDIR"; fi
  if [ "$RC" = "1" ] && has "role=verifier path=${OUTDIR}/gitlab-verifier.token outcome=failed (malformed" \
     && file_is "$OUTDIR/gitlab-verifier.token" "old-verifier" \
     && has "role=desk path=${OUTDIR}/gitlab-desk.token outcome=not-attempted" \
     && [ -z "$(find "$OUTDIR" -name '.renew-*')" ] && ! printf '%s' "$OUT" | grep -q 'first-line\|TESTSECRET'; then
    ok "T6 ($mode) malformed output fails closed: file kept, later roles not attempted, no temp left, nothing echoed"
  else
    bad "T6 ($mode) malformed secret fails closed (rc=$RC)"
  fi
done

# =============================================================================
# T7 — malformed listing: a PAT record without an id fails closed in
#      preflight rather than being read as "no match" (which would create a
#      second live credential).
# =============================================================================
newcase
export BAD_LIST_ROLE=reviewer
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "malformed PAT listing" && all_old; then
  ok "T7 a malformed PAT listing is a preflight refusal, zero mutations (no silent create)"
else
  bad "T7 a malformed PAT listing is a preflight refusal (rc=$RC)"
fi

# =============================================================================
# T8 — unwritable output directory: refused in preflight, before any mutation.
# =============================================================================
newcase
chmod 500 "$OUTDIR"
if [ -w "$OUTDIR" ]; then
  echo "skip - T8 cannot make a directory unwritable for this user (running as root?) — could-not-check"
else
  run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
  if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "is not writable" && has "PREFLIGHT FAILED"; then
    ok "T8 an unwritable --out-dir is refused in preflight, zero mutations"
  else
    bad "T8 an unwritable --out-dir is refused in preflight (rc=$RC)"
  fi
fi
chmod 700 "$OUTDIR"

# =============================================================================
# T9 — partial run + resume: a failure mid-fleet stops the run, reports each
#      role's outcome and the --only list; re-running with that list renews
#      exactly the rest and leaves the finished roles alone.
# =============================================================================
newcase
export FAIL_ONCE_ROLE=issue-loop
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && has "role=desk path=${OUTDIR}/gitlab-desk.token outcome=rotated" \
   && has "role=issue-loop path=${OUTDIR}/gitlab-issue-loop.token outcome=failed" \
   && has "role=board-writer path=${OUTDIR}/gitlab-board-writer.token outcome=not-attempted" \
   && has "--only issue-loop,intake-loop,board-writer" \
   && file_is "$OUTDIR/gitlab-issue-loop.token" "old-issue-loop" && file_is "$OUTDIR/gitlab-board-writer.token" "old-board-writer"; then
  ok "T9 a mid-fleet failure stops the run and prints the resume list"
else
  bad "T9 a mid-fleet failure stops the run and prints the resume list (rc=$RC)"
fi
printf 'renewed-marker' > "$OUTDIR/gitlab-reviewer.token"   # prove the resume leaves done roles alone
: > "$FAKE_GLAB_LOG"
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --only issue-loop,intake-loop,board-writer
if [ "$RC" = "0" ] && [ "$(logn '^argv=api .*/rotate')" = "3" ] \
   && file_is "$OUTDIR/gitlab-issue-loop.token" "glpat-TESTSECRET-rotated-issue-loop-0123456789" \
   && file_is "$OUTDIR/gitlab-board-writer.token" "glpat-TESTSECRET-rotated-board-writer-0123456789" \
   && file_is "$OUTDIR/gitlab-reviewer.token" "renewed-marker"; then
  ok "T9 the resume renews exactly the remaining three roles and nothing else"
else
  bad "T9 the resume renews exactly the remaining roles (rc=$RC rotations=$(logn '^argv=api .*/rotate'))"
fi

# =============================================================================
# T10 — --dry-run: every preflight read, the plan, zero mutations, no writes.
# =============================================================================
newcase
export NONE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --dry-run
if [ "$RC" = "0" ] && [ "$(mutations)" = "0" ] && all_old \
   && has "[dry-run] role=reviewer path=${OUTDIR}/gitlab-reviewer.token outcome=would-rotate" \
   && has "[dry-run] role=desk path=${OUTDIR}/gitlab-desk.token outcome=would-create"; then
  ok "T10 dry-run prints the rotate/create plan with zero mutations and no file change"
else
  bad "T10 dry-run plan, zero mutations (rc=$RC mutations=$(mutations))"
fi
newcase
run_impl "${ADMIN_ARGS[@]}" --out-dir "$CASEDIR/not-yet" --dry-run
if [ "$RC" = "0" ] && [ ! -e "$CASEDIR/not-yet" ] && [ "$(mutations)" = "0" ] && has "would create the output directory"; then
  ok "T10 dry-run does not create a missing --out-dir"
else
  bad "T10 dry-run does not create a missing --out-dir (rc=$RC)"
fi

# =============================================================================
# T11 — symlink custody (adopting-assay-gitlab.md §2): the write lands on the
#       link's target so the link survives; a link out of --out-dir is refused.
# =============================================================================
newcase
rm -f "$OUTDIR/gitlab-reviewer.token"
printf 'old-linked' > "$OUTDIR/myorg-reviewer-bot.token"; chmod 600 "$OUTDIR/myorg-reviewer-bot.token"
ln -s myorg-reviewer-bot.token "$OUTDIR/gitlab-reviewer.token"
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "0" ] && [ -L "$OUTDIR/gitlab-reviewer.token" ] \
   && file_is "$OUTDIR/myorg-reviewer-bot.token" "glpat-TESTSECRET-rotated-reviewer-0123456789" && mode600 "$OUTDIR/myorg-reviewer-bot.token"; then
  ok "T11 a linked custody file is written THROUGH the link; the link survives"
else
  bad "T11 a linked custody file is written through the link (rc=$RC)"
fi
newcase
rm -f "$OUTDIR/gitlab-worker.token"
printf 'elsewhere' > "$CASEDIR/outside.token"
ln -s "$CASEDIR/outside.token" "$OUTDIR/gitlab-worker.token"
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "resolves outside the output directory" && file_is "$CASEDIR/outside.token" "elsewhere"; then
  ok "T11 a custody link resolving outside --out-dir is refused in preflight"
else
  bad "T11 a custody link resolving outside --out-dir is refused (rc=$RC)"
fi

# =============================================================================
# T12 — single source: the role table lives ONLY in fleet-gitlab-roles.sh,
#       and both scripts source that file.
# =============================================================================
rows=$(grep -lE '^(reviewer|worker|verifier|desk|issue-loop|intake-loop|board-writer):(developer|reporter):[0-9]+:' "$here"/*.sh 2>/dev/null | sed 's|.*/||' | tr '\n' ' ')
if [ "$rows" = "fleet-gitlab-roles.sh " ]; then
  ok "T12 role-table rows appear only in tools/fleet-gitlab-roles.sh"
else
  bad "T12 role-table rows appear only in tools/fleet-gitlab-roles.sh (found in: ${rows:-nothing})"
fi
srcok=1
for f in create-fleet-gitlab.sh renew-fleet-gitlab-tokens.sh; do
  grep -qE '^\. "\$FLEET_ROLES_FILE"' "$here/$f" && grep -q 'fleet-gitlab-roles.sh"' "$here/$f" || srcok=0
done
if [ "$srcok" = "1" ]; then ok "T12 both fleet scripts source the shared table"; else bad "T12 both fleet scripts source the shared table"; fi

# =============================================================================
# T13 — argument preflight: usage errors exit 2 before glab is ever run.
# =============================================================================
newcase
usage_case() {  # usage_case LABEL ARGS...
  local label="$1"; shift
  : > "$FAKE_GLAB_LOG"
  run_impl "$@"
  if [ "$RC" = "2" ] && [ ! -s "$FAKE_GLAB_LOG" ]; then ok "T13 $label -> exit 2, glab never run"; else bad "T13 $label -> exit 2, glab never run (rc=$RC)"; fi
}
usage_case "no authority model" --hostname "$HOST" --prefix myorg --out-dir "$OUTDIR"
usage_case "both authority models" --hostname "$HOST" --group example --instance-admin --prefix myorg --out-dir "$OUTDIR"
usage_case "hostname with a scheme" --hostname "https://$HOST" --group example --prefix myorg --out-dir "$OUTDIR"
usage_case "no roles" --hostname "$HOST" --group example --out-dir "$OUTDIR"
usage_case "--role FILE is a path" --hostname "$HOST" --group example --role "desk=u:n:api:../x.token" --out-dir "$OUTDIR"
usage_case "malformed --role" --hostname "$HOST" --group example --role "desk=u:n" --out-dir "$OUTDIR"
usage_case "two roles, one file" --hostname "$HOST" --group example --role "a=u1:n1:api:same.token" --role "b=u2:n2:api:same.token" --out-dir "$OUTDIR"
usage_case "--duration over 365d" --hostname "$HOST" --group example --prefix myorg --duration 400d --out-dir "$OUTDIR"
usage_case "--only names an unconfigured role" --hostname "$HOST" --group example --prefix myorg --only nosuch --out-dir "$OUTDIR"

# =============================================================================
# T14 — explicit --role records: non-default names drive the lookup, scopes
#       and destination file; with --prefix they replace the table row.
# =============================================================================
newcase
export NONE_ROLE=reviewer
run_impl --hostname "$HOST" --instance-admin --prefix myorg \
  --role "reviewer=myorg-reviewer-bot:custom-review-pat:read_api:review.token" --out-dir "$OUTDIR" --only reviewer
if [ "$RC" = "0" ] && [ "$(logn 'argv=token create custom-review-pat --user myorg-reviewer-bot --duration 30d --scope read_api --output text')" = "1" ] \
   && file_is "$OUTDIR/review.token" "glpat-TESTSECRET-created-reviewer-0123456789" && file_is "$OUTDIR/gitlab-reviewer.token" "old-reviewer"; then
  ok "T14 an explicit --role record overrides the table row (name, scopes, file)"
else
  bad "T14 an explicit --role record overrides the table row (rc=$RC)"
fi

echo
echo "passed: $pass   failed: $fail"
[ "$fail" -eq 0 ]
