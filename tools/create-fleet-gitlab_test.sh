#!/usr/bin/env bash
# Offline, network-free test for tools/create-fleet-gitlab.sh, now a transparent
# adapter onto `deskgitlab create` (tools/desk/cmd/deskgitlab, desktools-v2/30).
#
# Two layers:
#   A. the adapter itself — every argument reaches deskgitlab unchanged after
#      the `create` verb, the exit status comes back unchanged, the credential
#      travels in the environment and never on argv, and the build-from-source
#      fallback works, refuses with 127 when the build fails, and cleans up after
#      itself;
#   B. the provisioner's behaviour — the scenarios this suite used to drive
#      through a fake curl (tier fallback, the guarded protect step, approvals
#      read-back, protected tags, merge checks, labels, avatars, credential
#      custody, transport failures recorded rather than fatal) now run as Go
#      tests against an httptest GitLab: TestFleetScenario* in
#      tools/desk/cmd/deskgitlab/scenario_test.go. This suite runs them.
#
# Needs bash and Go; no GitLab, no credential, no network.
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMPL="${FLEET_IMPL:-$here/create-fleet-gitlab.sh}"
case "$IMPL" in /*) ;; *) IMPL="$here/$IMPL" ;; esac
DESK="$here/desk"
BASH_BIN="$(command -v bash)"

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

if ! command -v go >/dev/null 2>&1; then
  echo "FAIL - go is required (the provisioner is tools/desk/cmd/deskgitlab)"; exit 1
fi

WORK="$(mktemp -d "${TMPDIR:-/tmp}/fleet-adapter-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

# PATH with no deskgitlab on it, so each case chooses exactly which one runs.
CLEAN_PATH=""
IFS=: read -r -a path_dirs <<< "$PATH"
for d in "${path_dirs[@]}"; do
  [ -n "$d" ] && [ ! -x "$d/deskgitlab" ] && CLEAN_PATH="${CLEAN_PATH:+$CLEAN_PATH:}$d"
done

# --- a recording stub deskgitlab ---------------------------------------------
STUB="$WORK/stub"; mkdir -p "$STUB"
cat > "$STUB/deskgitlab" <<'STUBEOF'
#!/usr/bin/env bash
{ for a in "$@"; do printf '%s\n' "$a"; done; } > "$STUB_ARGV"
printf '%s' "${GITLAB_TOKEN-<unset>}" > "$STUB_ENV"
exit "${STUB_RC:-0}"
STUBEOF
chmod +x "$STUB/deskgitlab"
export STUB_ARGV="$WORK/argv" STUB_ENV="$WORK/env"

stub_run() {  # stub_run RC ARGS... — run the adapter against the stub; sets RC
  RC=0
  STUB_RC="$1" PATH="$STUB:$CLEAN_PATH" GITLAB_TOKEN="TOKEN-PLACEHOLDER-owner" "$BASH_BIN" "$IMPL" "${@:2}" >/dev/null 2>&1 || RC=$?
}

# A1 — arguments pass through unchanged, after the `create` verb.
args=(--group "a group" --prefix=exorg --out-dir "-dash leading" --avatars-dir 'C:\x y' --tier ultimate --dry-run --)
stub_run 0 "${args[@]}"
want="$(printf '%s\n' create "${args[@]}")"
if [ "$RC" = "0" ] && [ "$(cat "$STUB_ARGV")" = "$want" ]; then
  ok "A1 every argument reaches deskgitlab create byte-identical, in order"
else
  bad "A1 arguments pass through unchanged (rc=$RC): $(tr '\n' ' ' < "$STUB_ARGV")"
fi

# A2 — the exit status comes back unchanged.
for want_rc in 0 1 2 7; do
  stub_run "$want_rc" --group g --prefix p
  if [ "$RC" = "$want_rc" ]; then ok "A2 exit status $want_rc propagates"; else bad "A2 exit status $want_rc propagates (got $RC)"; fi
done

# A3 — the credential reaches deskgitlab in the environment, never on argv (#786).
stub_run 0 --group g --prefix p
if [ "$(cat "$STUB_ENV")" = "TOKEN-PLACEHOLDER-owner" ] && ! grep -q 'TOKEN-PLACEHOLDER' "$STUB_ARGV"; then
  ok "A3 GITLAB_TOKEN is passed in the environment and is absent from argv"
else
  bad "A3 GITLAB_TOKEN travels in the environment only"
fi

# A4 — no table of its own: the provisioner's data lives in Go and the shared shell file.
if grep -q 'deskgitlab' "$IMPL" && ! grep -qE '^(ROLE_TABLE|LABEL_TABLE)=' "$IMPL" && ! grep -q 'curl' "$IMPL"; then
  ok "A4 the adapter carries no role/label table and no curl"
else
  bad "A4 the adapter carries no role/label table and no curl"
fi

# --- the real binary -------------------------------------------------------------
REAL="$WORK/real"; mkdir -p "$REAL"
if ! go -C "$DESK" build -o "$REAL/deskgitlab" ./cmd/deskgitlab; then
  echo "FAIL - building tools/desk/cmd/deskgitlab"; exit 1
fi
NOCALL="$WORK/nocall"; mkdir -p "$NOCALL"
for t in curl jq glab; do
  printf '#!/bin/sh\necho "RAN %s" >> "%s"\nexit 0\n' "$t" "$WORK/ran.log" > "$NOCALL/$t"; chmod +x "$NOCALL/$t"
done

real_run() {  # real_run ARGS... — sets RC, OUT (stdout+stderr)
  RC=0
  OUT="$(PATH="$REAL:$NOCALL:$CLEAN_PATH" HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 \
    "$BASH_BIN" "$IMPL" "$@" 2>&1)" || RC=$?
}

real_run --help
if [ "$RC" = "0" ] && printf '%s' "$OUT" | grep -q -- '--group' && printf '%s' "$OUT" | grep -q -- '--pat-expiry-days'; then
  ok "B1 --help exits 0 and lists the options"
else
  bad "B1 --help exits 0 and lists the options (rc=$RC)"
fi
real_run --group g --prefix p --bogus
if [ "$RC" = "2" ]; then ok "B2 an unknown flag exits 2"; else bad "B2 an unknown flag exits 2 (rc=$RC)"; fi
real_run --prefix p --dry-run
if [ "$RC" = "2" ] && printf '%s' "$OUT" | grep -q -- '--group and --prefix are required'; then
  ok "B3 a missing --group exits 2 with the script's message"
else
  bad "B3 a missing --group exits 2 (rc=$RC)"
fi
GITLAB_TOKEN="" real_run --group g --prefix p
if [ "$RC" = "2" ] && printf '%s' "$OUT" | grep -q 'GITLAB_TOKEN must be set'; then
  ok "B4 a live run without GITLAB_TOKEN exits 2 before any effect"
else
  bad "B4 a live run without GITLAB_TOKEN exits 2 (rc=$RC)"
fi
: > "$WORK/ran.log"
real_run --group example --prefix exorg --project example/proj --dry-run
accounts="$(printf '%s' "$OUT" | grep -c 'would create service account')"
if [ "$RC" = "0" ] && [ "$accounts" = "7" ] && [ ! -s "$WORK/ran.log" ]; then
  ok "B5 --dry-run enumerates seven accounts, exits 0 and starts no helper program (proxy dead: no network)"
else
  bad "B5 --dry-run enumerates seven accounts offline (rc=$RC accounts=$accounts)"
fi

# --- the build-from-source fallback ------------------------------------------------
FB_TMP="$WORK/fbtmp"; mkdir -p "$FB_TMP"
RC=0
OUT="$(PATH="$CLEAN_PATH" TMPDIR="$FB_TMP" "$BASH_BIN" "$IMPL" --help 2>&1)" || RC=$?
if [ "$RC" = "0" ] && printf '%s' "$OUT" | grep -q -- '--group' && [ -z "$(ls -A "$FB_TMP")" ]; then
  ok "C1 with no deskgitlab on PATH the adapter builds it from tools/desk, runs it and removes the build"
else
  bad "C1 build-from-source fallback (rc=$RC, left behind: $(ls -A "$FB_TMP" | tr '\n' ' '))"
fi
RC=0
OUT="$(PATH="$CLEAN_PATH" TMPDIR="$FB_TMP" "$BASH_BIN" "$IMPL" --group g --prefix p --pat-expiry-days 0 --dry-run 2>&1)" || RC=$?
if [ "$RC" = "2" ] && [ -z "$(ls -A "$FB_TMP")" ]; then
  ok "C2 the fallback propagates a usage exit (2) and still removes the build"
else
  bad "C2 the fallback propagates exit 2 and cleans up (rc=$RC)"
fi
BARE="$WORK/bare"; mkdir -p "$BARE"
ln -s "$(command -v dirname)" "$BARE/dirname"
RC=0
OUT="$(PATH="$BARE" "$BASH_BIN" "$IMPL" --help 2>&1)" || RC=$?
if [ "$RC" = "127" ] && printf '%s' "$OUT" | grep -q 'deskgitlab is not on PATH'; then
  ok "C3 with neither deskgitlab nor go the adapter says so and exits 127"
else
  bad "C3 neither deskgitlab nor go -> exit 127 with a message (rc=$RC)"
fi
GOFAIL="$WORK/gofail"; mkdir -p "$GOFAIL"
cat > "$GOFAIL/go" <<'GOEOF'
#!/usr/bin/env bash
: > "$GOFAIL_MARK"
echo "stub go: build failed" >&2
exit 1
GOEOF
chmod +x "$GOFAIL/go"
RC=0
OUT="$(GOFAIL_MARK="$WORK/gofail.called" PATH="$GOFAIL:$CLEAN_PATH" TMPDIR="$FB_TMP" "$BASH_BIN" "$IMPL" --group g --prefix p --dry-run 2>&1)" || RC=$?
if [ "$RC" = "127" ] && [ -e "$WORK/gofail.called" ] && printf '%s' "$OUT" | grep -q 'building deskgitlab from .* failed' \
   && ! printf '%s' "$OUT" | grep -q 'No such file' && [ -z "$(ls -A "$FB_TMP")" ]; then
  ok "C4 a failed fallback build exits 127 (not 1, a step failure), runs nothing and removes the build dir"
else
  bad "C4 failed fallback build -> exit 127, nothing run, build dir removed (rc=$RC, left behind: $(ls -A "$FB_TMP" | tr '\n' ' '))"
fi

# --- the provisioner's scenarios -----------------------------------------------------
# Includes TestFleetScenarioCredentialCustody (owner PAT never on argv or in a file) and
# TestFleetScenarioTransportFailureIsRecorded (a transport failure is a recorded step failure,
# not an abort) — the #786 hardening this suite has always pinned.
RC=0
OUT="$(cd "$DESK" && KUBECONFIG=/dev/null go test -count=1 -timeout 200s -v \
  -run '^(TestFleetScenario.*|TestRoleTableMatchesShell|TestPATDaysAndNamesMatchShell|TestLabelTableMatchesDeskfleet)$' ./cmd/deskgitlab 2>&1)" || RC=$?
passed="$(printf '%s\n' "$OUT" | grep -c '^--- PASS: ')"
failed="$(printf '%s\n' "$OUT" | grep -c '^--- FAIL: ')"
for name in TestFleetScenarioCredentialCustody TestFleetScenarioTransportFailureIsRecorded TestFleetScenarioRerunIsIdempotent; do
  if printf '%s\n' "$OUT" | grep -q "^--- PASS: $name "; then ok "D $name"; else bad "D $name"; fi
done
if [ "$RC" = "0" ] && [ "$failed" = "0" ] && [ "$passed" -ge 16 ]; then
  ok "D all $passed provisioner scenario and table-parity tests pass"
else
  printf '%s\n' "$OUT" | grep -E '^(--- FAIL|FAIL|panic)|_test.go' | head -40
  bad "D provisioner scenario and table-parity tests (rc=$RC passed=$passed failed=$failed)"
fi

echo
echo "passed: $pass   failed: $fail"
[ "$fail" -eq 0 ]
