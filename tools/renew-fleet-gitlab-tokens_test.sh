#!/usr/bin/env bash
# Offline, network-free unit test for tools/renew-fleet-gitlab-tokens.sh.
#
# It puts a fake `glab` first on PATH. The fake logs every invocation (argv,
# the pinned GITLAB_HOST, any --input request body) and answers from one
# responder whose behaviour each case steers with environment knobs, so the
# script's real control flow — preflight, the group-Owner authority check,
# rotate / create / skip-in-use / refuse, the atomic custody write — runs end
# to end without a GitLab, a credential, or a network.
#
# No cluster, no GitLab, no toolchain beyond bash + jq.
#
# Default impl is ../renew-fleet-gitlab-tokens.sh. Point RENEW_IMPL at another
# version to see the behaviours it lacks fail — the fail-first evidence. The
# issue's own prototype (instance-admin only, no group-Owner path, no
# preflight of every role before the first rotation) goes RED on the group,
# duplicate-preflight and resume cases, and any version that still carries an
# instance-admin mode goes RED on T13/T17 (#1630 item 3):
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
#   AUTH_FAIL=1             glab auth status fails
#   NONE_ROLE=<role>        that role has no PAT of its name (create fallback)
#   DUP_ROLE=<role>         that role has TWO active PATs of its name
#   BAD_SECRET_ROLE=<role>  that role's rotate/create output is malformed
#   BAD_LIST_ROLE=<role>    that role's PAT listing is malformed
#   EMPTY_LIST_ROLE=<role>  that role's PAT listing answers exit 0 with NO JSON
#                           value; EMPTY_LIST_KIND picks the stdout: zero (0
#                           bytes, default), nl ('\n') or ws ('  \n\n')
#   EMPTY_ARRAY_ROLE=<role> that role's PAT listing is a literal [] (no PATs)
#   PAGED_ROLE=<role>       that role's listing arrives as two page arrays,
#                           the live PAT on the second page
#   INUSE_ROLE=<role>       that role's matched PAT has a last_used_at "now"
#   OLD_USE_ROLE=<role>     that role's matched PAT was last used 3 days ago
#                           (outside the 1d in-use window: must rotate)
#   NO_LU_ROLE=<role>       that role's matched record lacks last_used_at
#   FAIL_ONCE_ROLE=<role>   that role's first rotate exits 1 (partial run)
# Every listing also carries an INACTIVE record with the role's own PAT name,
# so the `active` filter has a red case (A1).
RESPONDER="$(mktemp "${TMPDIR:-/tmp}/renew-responder-XXXXXX")"
cat > "$RESPONDER" <<'RESP'
R_LIST="reviewer worker verifier desk issue-loop intake-loop board-writer"
bump() { local f="$STATE/$1"; local n=0; [ -f "$f" ] && n=$(cat "$f"); n=$((n+1)); echo "$n" > "$f"; printf "%s" "$n"; }
uid_of_role() { local n=100 r; for r in $R_LIST; do n=$((n+1)); [ "$r" = "$1" ] && { echo "$n"; return; }; done; echo 0; }
role_of_uid() { local n=100 r; for r in $R_LIST; do n=$((n+1)); [ "$n" = "$1" ] && { echo "$r"; return; }; done; }
role_of_user() { local u="${1#myorg-}"; echo "${u%-bot}"; }
secret_for() { printf 'glpat-TEST.SECRET-%s-%s-0123456789' "$1" "$2"; }
now_iso() { date -u +%Y-%m-%dT%H:%M:%S.000Z 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%S.000Z'; }
days_ago_iso() { date -u -v-"$1"d +%Y-%m-%dT%H:%M:%S.000Z 2>/dev/null || date -u -d "-$1 days" +%Y-%m-%dT%H:%M:%S.000Z; }

pat_list() {  # pat_list ROLE
  local r="$1" id j lu live
  id=$(( $(uid_of_role "$r") + 400 ))
  lu="null"
  [ "$r" = "${INUSE_ROLE:-}" ] && lu="\"$(now_iso)\""
  [ "$r" = "${OLD_USE_ROLE:-}" ] && lu="\"$(days_ago_iso 3)\""
  j="[{\"id\":9${id},\"name\":\"unrelated-${r}\",\"active\":true,\"revoked\":false,\"last_used_at\":null}"
  # A1: an INACTIVE record carrying the role's own PAT name — the `active`
  # filter is all that keeps it from counting as a second live match.
  j="${j},{\"id\":8${id},\"name\":\"assay-${r}-fleet\",\"active\":false,\"revoked\":false,\"last_used_at\":null}"
  live=""
  if [ "$r" != "${NONE_ROLE:-}" ]; then
    if [ "$r" = "${NO_LU_ROLE:-}" ]; then
      live="{\"id\":${id},\"name\":\"assay-${r}-fleet\",\"active\":true,\"revoked\":false}"
    else
      live="{\"id\":${id},\"name\":\"assay-${r}-fleet\",\"active\":true,\"revoked\":false,\"last_used_at\":${lu}}"
    fi
  fi
  if [ "$r" = "${PAGED_ROLE:-}" ]; then
    printf '%s]\n[%s]\n' "$j" "$live"; return 0
  fi
  [ -n "$live" ] && j="${j},${live}"
  if [ "$r" = "${DUP_ROLE:-}" ]; then
    j="${j},{\"id\":$((id+1000)),\"name\":\"assay-${r}-fleet\",\"active\":true,\"revoked\":false,\"last_used_at\":null}"
  fi
  j="${j}]"
  if [ "$r" = "${BAD_LIST_ROLE:-}" ]; then j='[{"name":"assay-'"$r"'-fleet","active":true}]'; fi
  if [ "$r" = "${EMPTY_ARRAY_ROLE:-}" ]; then j='[]'; fi
  if [ "$r" = "${EMPTY_LIST_ROLE:-}" ]; then  # exit 0, no JSON value — the F-empty-listing repros
    case "${EMPTY_LIST_KIND:-zero}" in
      nl) printf '\n' ;;
      ws) printf '  \n\n' ;;
    esac
    return 0
  fi
  printf '%s\n' "$j"
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
    "api GET groups/1/service_accounts")
      local out="[" n=100
      for r in $R_LIST; do n=$((n+1)); out="${out}{\"id\":${n},\"username\":\"myorg-${r}-bot\"},"; done
      echo "${out}{\"id\":999,\"username\":\"someone-else-bot\"}]"; return 0 ;;
    "api GET groups/1/service_accounts/"*"/personal_access_tokens?state=active")
      uid="${k#api GET groups/1/service_accounts/}"; uid="${uid%%/*}"
      pat_list "$(role_of_uid "$uid")"; return 0 ;;
    "api POST groups/1/service_accounts/"*"/personal_access_tokens/"*"/rotate")
      uid="${k#api POST groups/1/service_accounts/}"; uid="${uid%%/*}"; r=$(role_of_uid "$uid")
      if fail_once "$r"; then echo "ERROR: 500 boom" >&2; return 1; fi
      if [ "$r" = "${BAD_SECRET_ROLE:-}" ]; then echo "{\"name\":\"assay-${r}-fleet\",\"token\":null}"; return 0; fi
      echo "{\"id\":$(( uid + 700 )),\"name\":\"assay-${r}-fleet\",\"active\":true,\"token\":\"$(secret_for rotated "$r")\"}"; return 0 ;;
    "api POST groups/1/service_accounts/"*"/personal_access_tokens")
      uid="${k#api POST groups/1/service_accounts/}"; uid="${uid%%/*}"; r=$(role_of_uid "$uid")
      if [ "$r" = "${BAD_SECRET_ROLE:-}" ]; then echo "{\"name\":\"assay-${r}-fleet\"}"; return 0; fi
      local nm; nm=$(jq -r '.name' "$input")   # a create answers with the name it was sent
      echo "{\"id\":$(( uid + 800 )),\"name\":\"${nm}\",\"active\":true,\"token\":\"$(secret_for created "$r")\"}"; return 0 ;;
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
  unset MEMBER_LEVEL AUTH_FAIL NONE_ROLE DUP_ROLE BAD_SECRET_ROLE BAD_LIST_ROLE FAIL_ONCE_ROLE \
    EMPTY_LIST_ROLE EMPTY_LIST_KIND EMPTY_ARRAY_ROLE PAGED_ROLE INUSE_ROLE OLD_USE_ROLE NO_LU_ROLE
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
  ! printf '%s' "$OUT" | grep -q 'TEST.SECRET' \
    && ! grep -q '^argv=.*TEST.SECRET' "$FAKE_GLAB_LOG" \
    && [ -z "$(find "$OUTDIR" -name '.renew-*' 2>/dev/null)" ]
}
all_old() { for r in $ROLES; do file_is "$OUTDIR/gitlab-$r.token" "old-$r" || return 1; done; }

GROUP_ARGS=(--hostname "$HOST" --group example --prefix myorg)

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
  file_is "$OUTDIR/gitlab-$r.token" "glpat-TEST.SECRET-rotated-$r-0123456789" && mode600 "$OUTDIR/gitlab-$r.token" || allok=0
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
if grep -q 'argv=token \|application/settings\|users?username=' "$FAKE_GLAB_LOG"; then bad "T1 the run must never probe instance admin (#1630 item 3)"; else ok "T1 no instance-admin probe: no application/settings, no users?username=, no glab token --user"; fi
if no_secret_leak; then ok "T1 no secret in output, glab argv, or a leftover temp file"; else bad "T1 no secret in output, glab argv, or a leftover temp file"; fi
if has "role=reviewer path=${OUTDIR}/gitlab-reviewer.token outcome=rotated"; then ok "T1 the report line is role + path + outcome"; else bad "T1 the report line is role + path + outcome"; fi
if has "renewed: 7 of 7 role(s); each renewed PAT expires" && ! has "skipped-in-use:"; then ok "T1 the summary counts seven renewed, none skipped"; else bad "T1 the summary counts seven renewed, none skipped"; fi

# =============================================================================
# T2 — --duration in weeks: 2w is 14 days on the rotate body's expires_at.
# =============================================================================
newcase
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --duration 2w --only worker
want=$(date -u -v+14d +%Y-%m-%d 2>/dev/null || date -u -d '+14 days' +%Y-%m-%d)
if [ "$RC" = "0" ] && grep -q "BODY api POST .*/rotate | {\"expires_at\":\"${want}\"}" "$FAKE_GLAB_LOG"; then
  ok "T2 --duration 2w -> expires_at 14 days out"
else
  bad "T2 --duration 2w -> expires_at 14 days out (rc=$RC)"
fi

# =============================================================================
# T3 — create fallback: a role with no active PAT of its name gets one
#      created with the SHARED table's scopes; the rest are rotated.
# =============================================================================
newcase
export NONE_ROLE=worker
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if grep -q 'BODY api POST groups/1/service_accounts/102/personal_access_tokens | {"name":"assay-worker-fleet","scopes":\["api","write_repository"\],"expires_at":"[0-9-]*"}' "$FAKE_GLAB_LOG" \
   && [ "$(logn '^argv=api .*/rotate')" = "6" ] && [ "$RC" = "0" ]; then
  ok "T3 creates the missing PAT with the table's name + scopes, rotates the other six"
else
  bad "T3 create fallback (rc=$RC)"
fi
if file_is "$OUTDIR/gitlab-worker.token" "glpat-TEST.SECRET-created-worker-0123456789" && has "role=worker path=${OUTDIR}/gitlab-worker.token outcome=created"; then
  ok "T3 the created secret lands in gitlab-worker.token, reported as created"
else
  bad "T3 the created secret lands in gitlab-worker.token"
fi
newcase
export EMPTY_ARRAY_ROLE=verifier
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --only verifier
if [ "$RC" = "0" ] && [ "$(logn 'argv=api .*-X POST .*/personal_access_tokens | host=')" = "1" ] && has "role=verifier path=${OUTDIR}/gitlab-verifier.token outcome=created"; then
  ok "T3 a literal [] listing (parsed, zero records: the expired-PAT case) is a create"
else
  bad "T3 a literal [] listing is a create (rc=$RC)"
fi

# =============================================================================
# T4 — duplicate refusal: two active PATs of one name is a PREFLIGHT refusal;
#      nothing is rotated for ANY role, no file changes.
# =============================================================================
newcase
export DUP_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
m=$(mutations)
if [ "$RC" = "1" ] && [ "$m" = "0" ] && has "refusing to guess" && all_old; then
  ok "T4 two active PATs of one name -> refused in preflight, zero mutations, files untouched"
else
  bad "T4 duplicate refusal (rc=$RC mutations=$m)"
fi

# =============================================================================
# T5 — authority refusal: an identity that is not the group's Owner is
#      refused before any mutation.
# =============================================================================
newcase
export MEMBER_LEVEL=40
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "authority refused" && ! has "instance-admin" && all_old; then
  ok "T5 a non-Owner (Maintainer 40) is refused, zero mutations, no instance-admin route offered"
else
  bad "T5 a non-Owner is refused (rc=$RC)"
fi
newcase
run_impl --hostname "$HOST" --group example-sub --prefix myorg --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "subgroup"; then ok "T5 a subgroup is refused (service accounts are top-level)"; else bad "T5 a subgroup is refused (rc=$RC)"; fi
newcase
export AUTH_FAIL=1
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "not authenticated"; then ok "T5 an unauthenticated glab is refused"; else bad "T5 an unauthenticated glab is refused (rc=$RC)"; fi

# =============================================================================
# T6 — malformed secret: output that is not a well-formed token response
#      fails closed — nothing written for that role, the run stops there.
# =============================================================================
newcase
export BAD_SECRET_ROLE=verifier
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && has "role=verifier path=${OUTDIR}/gitlab-verifier.token outcome=failed (malformed" \
   && file_is "$OUTDIR/gitlab-verifier.token" "old-verifier" \
   && has "role=desk path=${OUTDIR}/gitlab-desk.token outcome=not-attempted" \
   && [ -z "$(find "$OUTDIR" -name '.renew-*')" ] && ! printf '%s' "$OUT" | grep -q 'TEST.SECRET'; then
  ok "T6 malformed output fails closed: file kept, later roles not attempted, no temp left, nothing echoed"
else
  bad "T6 malformed secret fails closed (rc=$RC)"
fi

# =============================================================================
# T7 — malformed listing: a PAT record without an id, or without the
#      last_used_at key, fails closed in preflight rather than being read as
#      "no match" (a second live credential) or "never used" (A6).
# =============================================================================
newcase
export BAD_LIST_ROLE=reviewer
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "malformed PAT listing" && all_old; then
  ok "T7 a malformed PAT listing is a preflight refusal, zero mutations (no silent create)"
else
  bad "T7 a malformed PAT listing is a preflight refusal (rc=$RC)"
fi
newcase
export NO_LU_ROLE=reviewer
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "malformed PAT listing" && all_old; then
  ok "T7 a live record with no last_used_at key fails closed (not read as never-used), zero mutations"
else
  bad "T7 a record missing last_used_at fails closed (rc=$RC)"
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
   && has "PARTIAL RUN — 4 of 7 role(s) renewed; stopped at role=issue-loop" \
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
   && file_is "$OUTDIR/gitlab-issue-loop.token" "glpat-TEST.SECRET-rotated-issue-loop-0123456789" \
   && file_is "$OUTDIR/gitlab-board-writer.token" "glpat-TEST.SECRET-rotated-board-writer-0123456789" \
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
run_impl "${GROUP_ARGS[@]}" --out-dir "$CASEDIR/not-yet" --dry-run
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
   && file_is "$OUTDIR/myorg-reviewer-bot.token" "glpat-TEST.SECRET-rotated-reviewer-0123456789" && mode600 "$OUTDIR/myorg-reviewer-bot.token"; then
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
newcase
rm -f "$OUTDIR/gitlab-reviewer.token" "$OUTDIR/gitlab-worker.token"
printf 'shared-old' > "$OUTDIR/shared.token"; chmod 600 "$OUTDIR/shared.token"
ln -s shared.token "$OUTDIR/gitlab-reviewer.token"
ln -s shared.token "$OUTDIR/gitlab-worker.token"
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --only reviewer,worker
if [ "$RC" = "1" ] && [ "$(mutations)" = "0" ] && has "resolve to the same write target" && file_is "$OUTDIR/shared.token" "shared-old"; then
  ok "T11 two roles' custody links resolving to one write target are refused in preflight, zero mutations"
else
  bad "T11 two roles' custody links resolving to one write target are refused (rc=$RC)"
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
#       --instance-admin no longer exists (#1630 item 3): it is an unknown
#       argument, and --group is the only authority.
# =============================================================================
newcase
usage_case() {  # usage_case LABEL ARGS...
  local label="$1"; shift
  : > "$FAKE_GLAB_LOG"
  run_impl "$@"
  if [ "$RC" = "2" ] && [ ! -s "$FAKE_GLAB_LOG" ]; then ok "T13 $label -> exit 2, glab never run"; else bad "T13 $label -> exit 2, glab never run (rc=$RC)"; fi
}
usage_case "no --group" --hostname "$HOST" --prefix myorg --out-dir "$OUTDIR"
usage_case "--instance-admin (removed)" --hostname "$HOST" --instance-admin --prefix myorg --out-dir "$OUTDIR"
usage_case "--instance-admin beside --group (removed)" --hostname "$HOST" --group example --instance-admin --prefix myorg --out-dir "$OUTDIR"
usage_case "hostname with a scheme" --hostname "https://$HOST" --group example --prefix myorg --out-dir "$OUTDIR"
usage_case "no roles" --hostname "$HOST" --group example --out-dir "$OUTDIR"
usage_case "--role FILE is a path" --hostname "$HOST" --group example --role "desk=u:n:api:../x.token" --out-dir "$OUTDIR"
usage_case "malformed --role" --hostname "$HOST" --group example --role "desk=u:n" --out-dir "$OUTDIR"
usage_case "two roles, one file" --hostname "$HOST" --group example --role "a=u1:n1:api:same.token" --role "b=u2:n2:api:same.token" --out-dir "$OUTDIR"
usage_case "--duration over 365d" --hostname "$HOST" --group example --prefix myorg --duration 400d --out-dir "$OUTDIR"
usage_case "--only names an unconfigured role" --hostname "$HOST" --group example --prefix myorg --only nosuch --out-dir "$OUTDIR"
help_out="$(PATH="$BIN:$PATH" bash "$IMPL" --help 2>&1)"
if printf '%s' "$help_out" | grep -q 'instance-admin\|application/settings'; then bad "T13 --help must not offer an instance-admin mode"; else ok "T13 --help offers no instance-admin mode"; fi

# =============================================================================
# T14 — explicit --role records: non-default names drive the lookup, scopes
#       and destination file; with --prefix they replace the table row.
# =============================================================================
newcase
export NONE_ROLE=reviewer
run_impl "${GROUP_ARGS[@]}" \
  --role "reviewer=myorg-reviewer-bot:custom-review-pat:read_api:review.token" --out-dir "$OUTDIR" --only reviewer
if [ "$RC" = "0" ] && grep -q 'BODY api POST groups/1/service_accounts/101/personal_access_tokens | {"name":"custom-review-pat","scopes":\["read_api"\],"expires_at":"[0-9-]*"}' "$FAKE_GLAB_LOG" \
   && file_is "$OUTDIR/review.token" "glpat-TEST.SECRET-created-reviewer-0123456789" && file_is "$OUTDIR/gitlab-reviewer.token" "old-reviewer"; then
  ok "T14 an explicit --role record overrides the table row (name, scopes, file)"
else
  bad "T14 an explicit --role record overrides the table row (rc=$RC)"
fi

# =============================================================================
# T15 — default --duration is 7d (the shared fleet-gitlab-roles.sh
#       FLEET_PAT_DAYS / spec.md §5 expiry backstop), not a wider value that
#       would silently relax it.
# =============================================================================
newcase
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --only reviewer
want=$(date -u -v+7d +%Y-%m-%d 2>/dev/null || date -u -d '+7 days' +%Y-%m-%d)
if [ "$RC" = "0" ] && grep -q "BODY api POST .*/rotate | {\"expires_at\":\"${want}\"}" "$FAKE_GLAB_LOG"; then
  ok "T15 the default --duration is 7d with no flag passed"
else
  bad "T15 the default --duration is 7d (rc=$RC, wanted expires_at ${want})"
fi

# =============================================================================
# T16 — empty PAT listing (F-empty-listing, reviewer-security rounds 1-4): an
#       exit-0 listing that parses to NO JSON value — zero bytes, a bare
#       newline, or whitespace only — is a preflight refusal, never read as
#       "zero active PATs" (which would create a second live credential).
#       A multi-page listing (one array per page) is merged, not refused.
# =============================================================================
for kind in zero nl ws; do
  newcase
  export EMPTY_LIST_ROLE=desk EMPTY_LIST_KIND="$kind"
  run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
  m=$(mutations)
  if [ "$RC" = "1" ] && [ "$m" = "0" ] && has "empty PAT listing" && all_old && ! has "outcome=created"; then
    ok "T16 ($kind) an exit-0 listing with no JSON value is refused in preflight, zero mutations"
  else
    bad "T16 ($kind) an empty PAT listing is refused (rc=$RC mutations=$m)"
  fi
done
newcase
export PAGED_ROLE=worker
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --only worker
if [ "$RC" = "0" ] && grep -q 'argv=api .*personal_access_tokens/502/rotate' "$FAKE_GLAB_LOG" && [ "$(mutations)" = "1" ]; then
  ok "T16 a two-page listing is merged: the live PAT on page two is rotated (not created beside it)"
else
  bad "T16 a two-page listing is merged (rc=$RC mutations=$(mutations))"
fi

# =============================================================================
# T17 — no instance-admin authority anywhere (#1630 item 3): a full run and a
#       non-Owner refusal both make no admin-only call.
# =============================================================================
newcase
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
a1=$(logn 'application/settings\|users?username=\|argv=token ')
newcase
export MEMBER_LEVEL=30
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
a2=$(logn 'application/settings\|users?username=\|argv=token ')
if [ "$a1" = "0" ] && [ "$a2" = "0" ]; then
  ok "T17 neither a full run nor a refused non-Owner probes instance admin (no settings read, no users?username=, no glab token)"
else
  bad "T17 instance admin is never probed (full=$a1 refused=$a2)"
fi

# =============================================================================
# T18 — in-use protection (#1630 required behavior 5): a role whose active
#       PAT was used inside the in-use window is SKIPPED by default and
#       reported, never rotated; --rotate-in-use overrides it. A PAT used
#       BEFORE the window rotates normally (F5).
# =============================================================================
newcase
export INUSE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
m=$(mutations)
if [ "$m" = "6" ] && has "role=desk path=${OUTDIR}/gitlab-desk.token outcome=skipped-in-use" \
   && file_is "$OUTDIR/gitlab-desk.token" "old-desk"; then
  ok "T18 a role used inside the in-use window is skipped by default (the other six still rotate), file untouched"
else
  bad "T18 in-use skip by default (rc=$RC mutations=$m)"
fi
newcase
export INUSE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --dry-run
if [ "$RC" = "0" ] && has "outcome=would-skip-in-use" && has "--rotate-in-use"; then
  ok "T18 dry-run reports would-skip-in-use with the override hint"
else
  bad "T18 dry-run would-skip-in-use reporting"
fi
newcase
export INUSE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --rotate-in-use --only desk
if [ "$RC" = "0" ] && [ "$(logn 'argv=api .*/rotate')" = "1" ] && has "role=desk path=${OUTDIR}/gitlab-desk.token outcome=rotated"; then
  ok "T18 --rotate-in-use overrides the skip and rotates the in-use role"
else
  bad "T18 --rotate-in-use overrides the skip (rc=$RC)"
fi
newcase
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "0" ] && [ "$(logn 'argv=api .*/rotate')" = "7" ] && ! has "skipped-in-use"; then
  ok "T18 a PAT with a null last_used_at (never used) is rotated normally, no false skip"
else
  bad "T18 a never-used PAT is rotated normally (rc=$RC)"
fi
# F5: an OLD last_used_at (3 days, outside the 1d window) must rotate by
# default. Red against in_use_recent's window check rewritten to `return 0`.
newcase
export OLD_USE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "0" ] && [ "$(logn 'argv=api .*/rotate')" = "7" ] && has "role=desk path=${OUTDIR}/gitlab-desk.token outcome=rotated" \
   && ! has "skipped-in-use" && file_is "$OUTDIR/gitlab-desk.token" "glpat-TEST.SECRET-rotated-desk-0123456789"; then
  ok "T18 a PAT last used 3 days ago (outside the in-use window) rotates by default (F5)"
else
  bad "T18 an old last_used_at rotates by default (rc=$RC rotations=$(logn 'argv=api .*/rotate'))"
fi
newcase
export OLD_USE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --only desk --dry-run
if [ "$RC" = "0" ] && has "[dry-run] role=desk path=${OUTDIR}/gitlab-desk.token outcome=would-rotate"; then
  ok "T18 dry-run plans would-rotate for a PAT used outside the window (F5)"
else
  bad "T18 dry-run plans would-rotate for an old last_used_at (rc=$RC)"
fi

# =============================================================================
# T19 — the summary and exit code never call a skipped role renewed (F4):
#       skips are counted apart, the expiry is stated only for renewed roles,
#       and a run that skipped any role exits 3 (the documented contract).
# =============================================================================
newcase
export INUSE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR" --only desk
if [ "$RC" = "3" ] && [ "$(mutations)" = "0" ] && file_is "$OUTDIR/gitlab-desk.token" "old-desk" \
   && has "renewed: 0 of 1 role(s) — no PAT was rotated or created" \
   && has "skipped-in-use: 1 of 1 role(s) (desk) — NOT renewed" \
   && has "--only desk --rotate-in-use" \
   && ! has "expires" && ! has "renewed: 1 of 1"; then
  ok "T19 an all-skipped run reports renewed 0, skipped 1, no expiry claimed, exit 3"
else
  bad "T19 an all-skipped run must not report success (rc=$RC mutations=$(mutations))"
fi
newcase
export INUSE_ROLE=desk
run_impl "${GROUP_ARGS[@]}" --out-dir "$OUTDIR"
if [ "$RC" = "3" ] && has "renewed: 6 of 7 role(s); each renewed PAT expires" \
   && has "skipped-in-use: 1 of 7 role(s) (desk) — NOT renewed; their PATs keep their previous expiry" \
   && ! has "renewed: 7 of 7"; then
  ok "T19 a mixed run counts 6 renewed + 1 skipped separately, exit 3"
else
  bad "T19 a mixed run counts renewed and skipped separately (rc=$RC)"
fi
help_out="$(PATH="$BIN:$PATH" bash "$IMPL" --help 2>&1)"
if printf '%s' "$help_out" | grep -q '3 = the run finished but one or more roles were SKIPPED'; then
  ok "T19 the usage text documents exit 3 for a skipped role"
else
  bad "T19 the usage text documents exit 3 for a skipped role"
fi

# =============================================================================
# T20 — an --out-dir holding whitespace: every path stays one word (the
#       secret temp-file list is an array), the run works, no temp is left.
# =============================================================================
newcase
SPACED="$CASEDIR/assay config"; mkdir -p "$SPACED"
for r in $ROLES; do printf 'old-%s' "$r" > "$SPACED/gitlab-$r.token"; chmod 600 "$SPACED/gitlab-$r.token"; done
run_impl "${GROUP_ARGS[@]}" --out-dir "$SPACED" --only worker
if [ "$RC" = "0" ] && file_is "$SPACED/gitlab-worker.token" "glpat-TEST.SECRET-rotated-worker-0123456789" \
   && [ -z "$(find "$SPACED" -name '.renew-*')" ]; then
  ok "T20 an --out-dir with a space renews and leaves no temp file"
else
  bad "T20 an --out-dir with a space (rc=$RC)"
fi

echo
echo "passed: $pass   failed: $fail"
[ "$fail" -eq 0 ]
