#!/usr/bin/env bash
# Offline, network-free unit test for tools/create-fleet-gitlab.sh.
#
# It puts a fake `curl` first on PATH. That fake logs every invocation and
# answers from a per-case responder function, so the script's real control
# flow — tier branching, the guarded protect step, the avatar uploads — is
# exercised end to end without a GitLab, a credential, or a network.
#
# No cluster, no GitLab, no toolchain beyond bash + jq.
#
# Default impl is ../create-fleet-gitlab.sh. Point FLEET_IMPL at the version
# before the tier-safe/avatar fix to see the new behaviours fail — the
# fail-first evidence:
#   git show origin/main:tools/create-fleet-gitlab.sh > /tmp/old-fleet.sh
#   FLEET_IMPL=/tmp/old-fleet.sh ./tools/create-fleet-gitlab_test.sh   # RED
#   ./tools/create-fleet-gitlab_test.sh                                # green
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMPL="${FLEET_IMPL:-$here/create-fleet-gitlab.sh}"
case "$IMPL" in /*) ;; *) IMPL="$here/$IMPL" ;; esac

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

ROLES="reviewer worker verifier desk issue-loop intake-loop board-writer"

# --- the fake curl --------------------------------------------------------
# Written once into a bin dir that every case prepends to PATH. It parses the
# argv shapes the script actually uses (-o/-X/-d/-F/-K, plus the -fsSL icon
# fetch), appends a line to $FAKE_CURL_LOG, and delegates the status+body to
# the responder shell file named by $FAKE_CURL_RESPONDER.
BIN="$(mktemp -d "${TMPDIR:-/tmp}/fleet-fakebin-XXXXXX")"
cat > "$BIN/curl" <<'FAKECURL'
#!/usr/bin/env bash
set -uo pipefail
out=""; method="GET"; url=""; data=""; form=""; cfg=""
args=("$@"); i=0
while [ $i -lt ${#args[@]} ]; do
  a="${args[$i]}"
  case "$a" in
    -o|-X|-d|-F|-K|-H|-w) case "$a" in
        -o) i=$((i+1)); out="${args[$i]}" ;;
        -X) i=$((i+1)); method="${args[$i]}" ;;
        -d) i=$((i+1)); data="${args[$i]}" ;;
        -F) i=$((i+1)); form="${args[$i]}" ;;
        -K) i=$((i+1)); cfg="${args[$i]}" ;;
        *)  i=$((i+1)) ;;
      esac ;;
    -*) ;;
    *) url="$a" ;;
  esac
  i=$((i+1))
done

printf '%s %s | argv=%s\n' "$method" "$url" "$*" >> "$FAKE_CURL_LOG"
if [ -n "$data" ]; then
  printf 'BODY %s %s | %s\n' "$method" "$url" "$(printf '%s' "$data" | tr -d '\n ')" >> "$FAKE_CURL_LOG"
fi
if [ -n "$form" ]; then
  printf 'FORM %s %s | %s | cfg=%s\n' "$method" "$url" "$form" "${cfg:+yes}" >> "$FAKE_CURL_LOG"
fi

# The public role-icon fetch: not the API, no token, plain file download.
case "$url" in
  https://assay.guide/*)
    if [ "${FAKE_ICON_FAIL:-0}" = "1" ]; then exit 22; fi
    printf 'PNGFAKE' > "$out"
    exit 0
    ;;
esac

path="${url#*://*/api/v4}"
# shellcheck disable=SC1090
. "$FAKE_CURL_RESPONDER"
resp="$(respond "$method" "$path" "$data")"
status="$(printf '%s' "$resp" | head -1)"
printf '%s' "$resp" | tail -n +2 > "$out"
printf '%s' "$status"
FAKECURL
chmod +x "$BIN/curl"

# newcase — fresh log, out-dir, responder state.
newcase() {
  CASEDIR="$(mktemp -d "${TMPDIR:-/tmp}/fleet-case-XXXXXX")"
  OUTDIR="$CASEDIR/out";      mkdir -p "$OUTDIR"
  ICONS="$CASEDIR/icons";     mkdir -p "$ICONS"
  STATE="$CASEDIR/state";     mkdir -p "$STATE"
  export FAKE_CURL_LOG="$CASEDIR/curl.log"; : > "$FAKE_CURL_LOG"
  export FAKE_CURL_RESPONDER="$CASEDIR/responder.sh"
  export STATE
  unset FAKE_ICON_FAIL
  for r in $ROLES; do printf 'PNGFAKE' > "$ICONS/$r.png"; done
}

seed_tokens() {
  for r in $ROLES; do printf 'glpat-fake-%s' "$r" > "$OUTDIR/myorg-${r}-bot.token"; done
}

# bump NAME — echoes 1, 2, 3 ... on successive calls (responder call counters).
# These two are CODE TEMPLATES pasted into a responder file, so the $vars in
# them must stay unexpanded here.
# shellcheck disable=SC2016
bump_helper='bump() { local f="$STATE/$1"; local n=0; [ -f "$f" ] && n=$(cat "$f"); n=$((n+1)); echo "$n" > "$f"; printf "%s" "$n"; }'

run_impl() {  # run_impl <args...>; captures $OUT and $RC
  OUT="$(PATH="$BIN:$PATH" GITLAB_TOKEN="${GITLAB_TOKEN:-owner-fake}" bash "$IMPL" "$@" 2>&1)"
  RC=$?
  if [ "${FLEET_TEST_DEBUG:-0}" = "1" ]; then
    echo "--- run: $* (rc=$RC)"; echo "$OUT"; echo "--- log:"; cat "$FAKE_CURL_LOG"; echo "---"
  fi
}

has()  { case "$OUT" in *"$1"*) return 0 ;; *) return 1 ;; esac; }
logn() { grep -c -- "$1" "$FAKE_CURL_LOG" 2>/dev/null || true; }

# --- the shared happy-path responder prologue ------------------------------
# Accounts, memberships and PAT minting all succeed; each case overrides the
# project-settings half.
# shellcheck disable=SC2016
common_accounts='
  case "$m $p" in
    "GET /groups/example") echo "200"; echo "{\"id\":1$PLANFIELD}"; return ;;
    "GET /groups/1/service_accounts") echo "200"; echo "[]"; return ;;
    "POST /groups/1/service_accounts") n=$(bump sa); echo "201"; echo "{\"id\":$((100+n))}"; return ;;
    "POST /groups/1/members") echo "201"; echo "{}"; return ;;
    "GET /projects/proj") echo "200"; echo "{\"id\":7}"; return ;;
    "PUT /projects/7") echo "200"; echo "{}"; return ;;
    "PUT /user/avatar") echo "200"; echo "{\"avatar_url\":\"/uploads/-/system/user/avatar/1/x.png\"}"; return ;;
  esac
  case "$m $p" in
    "GET /groups/1/members/"*) echo "404"; echo "{}"; return ;;
    "POST /groups/1/service_accounts/"*"/personal_access_tokens") echo "201"; echo "{\"token\":\"glpat-fake-minted\"}"; return ;;
  esac
'

# ===========================================================================
# T1 — a 400 from the protect POST re-applies the PREVIOUS rule, and the
#      settings steps after it still run.
# ===========================================================================
newcase
cat > "$FAKE_CURL_RESPONDER" <<RESP
$bump_helper
PLANFIELD=',"plan":"premium"'
respond() {
  local m="\$1" p="\$2" n
  $common_accounts
  case "\$m \$p" in
    "GET /projects/7/protected_branches/main")
      echo "200"
      echo '{"name":"main","push_access_levels":[{"access_level":0}],"merge_access_levels":[{"access_level":30}],"allow_force_push":false}'
      return ;;
    "DELETE /projects/7/protected_branches/main") echo "204"; echo "{}"; return ;;
    "POST /projects/7/protected_branches")
      n=\$(bump protectpost)
      if [ "\$n" = "1" ]; then echo "400"; echo '{"message":"boom"}'; else echo "201"; echo "{}"; fi
      return ;;
    "POST /projects/7/approvals") echo "201"; echo "{}"; return ;;
    "GET /projects/7/approvals")
      echo "200"
      echo '{"merge_requests_author_approval":false,"merge_requests_disable_committers_approval":true,"merge_request_approvers_available":true}'
      return ;;
  esac
  echo "500"; echo '{"unstubbed":true}'
}
RESP
run_impl --group example --prefix myorg --project proj --out-dir "$OUTDIR" --no-avatars
if has "protection re-applied to its previous state"; then
  ok "T1 protect POST 400 -> previous rule re-applied"
else
  bad "T1 protect POST 400 -> previous rule re-applied (no re-apply reported)"
fi
if grep -q 'BODY POST .*/protected_branches |.*"merge_access_level":30' "$FAKE_CURL_LOG"; then
  ok "T1 the re-POST carries the PREVIOUSLY READ rule (merge_access_level 30)"
else
  bad "T1 the re-POST carries the previously read rule"
fi
if [ "$RC" != "0" ]; then ok "T1 exits non-zero after a failed step (rc=$RC)"; else bad "T1 exits non-zero after a failed step (rc=0)"; fi
if has "configured: pipelines must succeed before merge"; then
  ok "T1 a failed protect step does not skip the steps after it"
else
  bad "T1 a failed protect step does not skip the steps after it"
fi

# ===========================================================================
# T2 — --avatars-only uploads and mints NOTHING.
# ===========================================================================
newcase
seed_tokens
cat > "$FAKE_CURL_RESPONDER" <<RESP
$bump_helper
respond() {
  local m="\$1" p="\$2"
  case "\$m \$p" in
    "PUT /user/avatar") echo "200"; echo '{"avatar_url":"/uploads/x.png"}'; return ;;
  esac
  echo "500"; echo '{"unstubbed":true}'
}
RESP
run_impl --avatars-only --prefix myorg --out-dir "$OUTDIR" --avatars-dir "$ICONS"
mints=$(logn "personal_access_tokens")
creates=$(logn "POST https://gitlab.com/api/v4/groups")
uploads=$(logn "^PUT https://gitlab.com/api/v4/user/avatar")
# RC is part of this assertion on purpose: a script that rejects the flag
# outright also makes zero mint calls, and that must not read as a pass.
if [ "$mints" = "0" ] && [ "$creates" = "0" ] && [ "$RC" = "0" ]; then
  ok "T2 --avatars-only performs zero mint/create calls"
else
  bad "T2 --avatars-only performs zero mint/create calls (mint=$mints create=$creates rc=$RC)"
fi
if [ "$uploads" = "7" ]; then ok "T2 --avatars-only uploads all seven avatars"; else bad "T2 --avatars-only uploads all seven avatars (got $uploads)"; fi
if [ "$(printf '%s\n' "$OUT" | grep -c '^avatar: myorg-.*(HTTP 200)$')" = "7" ]; then
  ok "T2 prints one 'avatar: <username> <- <file> (HTTP 200)' line per role"
else
  bad "T2 prints one 'avatar: ...' line per role"
fi
if grep -q 'glpat-fake-reviewer' "$FAKE_CURL_LOG"; then
  bad "T2 the role token never reaches argv"
else
  ok "T2 the role token never reaches argv (passed via curl -K config)"
fi
if [ "$RC" = "0" ]; then ok "T2 --avatars-only exits 0"; else bad "T2 --avatars-only exits 0 (rc=$RC)"; fi

# ===========================================================================
# T3 — unknown tier: the Premium form's 400 falls back to the free-tier form,
#      and `main` ends up protected with merge_access_level 40.
# ===========================================================================
newcase
cat > "$FAKE_CURL_RESPONDER" <<RESP
$bump_helper
PLANFIELD=''
respond() {
  local m="\$1" p="\$2" n
  $common_accounts
  case "\$m \$p" in
    "GET /projects/7/protected_branches/main")
      n=\$(bump pbget)
      if [ "\$n" = "1" ]; then echo "404"; echo '{"message":"404 Not found"}'; else
        echo "200"
        echo '{"name":"main","push_access_levels":[{"access_level":0}],"merge_access_levels":[{"access_level":40}],"allow_force_push":false}'
      fi
      return ;;
    "POST /projects/7/protected_branches")
      n=\$(bump protectpost)
      if [ "\$n" = "1" ]; then
        echo "400"; echo '{"error":"allowed_to_unprotect[access_level] does not have a valid value"}'
      else
        echo "201"; echo "{}"
      fi
      return ;;
    "POST /projects/7/approvals") echo "201"; echo "{}"; return ;;
    "GET /projects/7/approvals")
      echo "200"
      echo '{"merge_requests_author_approval":false,"merge_requests_disable_committers_approval":true,"merge_request_approvers_available":true}'
      return ;;
  esac
  echo "500"; echo '{"unstubbed":true}'
}
RESP
run_impl --group example --prefix myorg --project proj --out-dir "$OUTDIR" --no-avatars
if has "free tier — board-writer push allowlist not available (failed-at-tier, remediation: Premium)"; then
  ok "T3 the free-tier fallback reports the allowlist as failed-at-tier"
else
  bad "T3 the free-tier fallback reports the allowlist as failed-at-tier"
fi
if grep -q 'BODY POST .*/protected_branches |.*"push_access_level":0.*"merge_access_level":40' "$FAKE_CURL_LOG"; then
  ok "T3 the fallback POST uses the three free-tier fields with merge_access_level 40"
else
  bad "T3 the fallback POST uses the three free-tier fields with merge_access_level 40"
fi
if has "read-back: main push_access_level=0 merge_access_level=40 allow_force_push=false"; then
  ok "T3 the three decided fields are read back and printed"
else
  bad "T3 the three decided fields are read back and printed"
fi
if [ "$RC" = "0" ]; then ok "T3 a tier fallback is not a failure (rc=0)"; else bad "T3 a tier fallback is not a failure (rc=$RC)"; fi

# ===========================================================================
# T4 — dry-run enumerates seven accounts AND seven avatar uploads, no calls.
# ===========================================================================
newcase
echo 'respond() { echo "500"; echo "{}"; }' > "$FAKE_CURL_RESPONDER"
run_impl --dry-run --group example --prefix myorg
accs=$(printf '%s\n' "$OUT" | grep -c 'would create service account')
avs=$(printf '%s\n' "$OUT" | grep -c 'would upload avatar for')
if [ "$accs" -ge 7 ] && [ "$avs" = "7" ]; then
  ok "T4 dry-run enumerates $accs service accounts and $avs avatar uploads"
else
  bad "T4 dry-run enumerates >=7 accounts and 7 avatar uploads (accs=$accs avs=$avs)"
fi
if [ ! -s "$FAKE_CURL_LOG" ]; then ok "T4 dry-run makes zero network calls"; else bad "T4 dry-run makes zero network calls"; fi

# ===========================================================================
# T5 — approvals: a 201 whose read-back did not take is reported, not trusted.
# ===========================================================================
newcase
cat > "$FAKE_CURL_RESPONDER" <<RESP
$bump_helper
PLANFIELD=',"plan":"free"'
respond() {
  local m="\$1" p="\$2" n
  $common_accounts
  case "\$m \$p" in
    "GET /projects/7/protected_branches/main")
      n=\$(bump pbget)
      if [ "\$n" = "1" ]; then echo "404"; echo '{"message":"404 Not found"}'; else
        echo "200"
        echo '{"name":"main","push_access_levels":[{"access_level":0}],"merge_access_levels":[{"access_level":40}],"allow_force_push":false}'
      fi
      return ;;
    "POST /projects/7/protected_branches") echo "201"; echo "{}"; return ;;
    "POST /projects/7/approvals") echo "201"; echo "{}"; return ;;
    "GET /projects/7/approvals")
      echo "200"
      echo '{"merge_requests_author_approval":true,"merge_requests_disable_committers_approval":false,"merge_request_approvers_available":false}'
      return ;;
  esac
  echo "500"; echo '{"unstubbed":true}'
}
RESP
run_impl --group example --prefix myorg --project proj --out-dir "$OUTDIR" --no-avatars
if has "approval settings ignored at this tier (failed-at-tier, remediation: Premium)"; then
  ok "T5 an ignored approval write is reported instead of trusted"
else
  bad "T5 an ignored approval write is reported instead of trusted"
fi
if has "do not count approvals as a server-enforced gate on this tier"; then
  ok "T5 the read-back is judged against enforcement, not just the stored field"
else
  bad "T5 the read-back is judged against enforcement, not just the stored field"
fi
if grep -q 'GET https://gitlab.com/api/v4/projects/7/approvals' "$FAKE_CURL_LOG"; then
  ok "T5 approvals are read back after the write"
else
  bad "T5 approvals are read back after the write"
fi

# ===========================================================================
# T6 — default avatars: the public role icons are fetched and self-uploaded;
#      an icon that cannot be fetched is a NOTICE, not a failure.
# ===========================================================================
newcase
cat > "$FAKE_CURL_RESPONDER" <<RESP
$bump_helper
PLANFIELD=',"plan":"free"'
respond() {
  local m="\$1" p="\$2" n
  $common_accounts
  echo "500"; echo '{"unstubbed":true}'
}
RESP
run_impl --group example --prefix myorg --out-dir "$OUTDIR"
icons=$(logn "^GET https://assay.guide/assets/app-icon-")
uploads=$(logn "^PUT https://gitlab.com/api/v4/user/avatar")
if [ "$icons" = "7" ] && [ "$uploads" = "7" ]; then
  ok "T6 seven default icons fetched and seven avatars self-uploaded"
else
  bad "T6 seven default icons fetched and seven avatars self-uploaded (icons=$icons uploads=$uploads)"
fi
if [ "$RC" = "0" ]; then ok "T6 default avatar run exits 0"; else bad "T6 default avatar run exits 0 (rc=$RC)"; fi

newcase
cat > "$FAKE_CURL_RESPONDER" <<RESP
$bump_helper
PLANFIELD=',"plan":"free"'
respond() {
  local m="\$1" p="\$2" n
  $common_accounts
  echo "500"; echo '{"unstubbed":true}'
}
RESP
FAKE_ICON_FAIL=1 run_impl --group example --prefix myorg --out-dir "$OUTDIR"
if has "could not fetch the default icon" && has "skipped, not a failure" && [ "$RC" = "0" ]; then
  ok "T6b an unfetchable icon is a NOTICE, not a failure (rc=$RC)"
else
  bad "T6b an unfetchable icon is a NOTICE, not a failure (rc=$RC)"
fi

echo
echo "passed: $pass   failed: $fail"
[ "$fail" -eq 0 ]
