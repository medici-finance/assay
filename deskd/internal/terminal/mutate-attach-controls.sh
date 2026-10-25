#!/usr/bin/env bash
# mutate-attach-controls.sh — fail-first evidence for deskd's attach lifecycle
# (the service contract, worker-prompt kit §9).
#
# WHY MUTATION AND NOT A PRE-FIX RED RUN. Every test this brief adds pins code
# the brief also adds (the Supervisor, the capability table, the attach
# status). At the pre-fix commit they do not compile, and a build error is not
# evidence that a check can catch what it claims to catch. So each case below
# removes or inverts exactly ONE control, the way a plausible future edit
# would, and asserts the matching test then FAILS.
#
# It never edits the working tree: the module is copied to a tempdir, one
# mutation is applied to the copy, and the copy is thrown away. It is fully
# offline: no container engine, no cluster, no network (KUBECONFIG=/dev/null).
# The container-detach cases run against a fake `docker` on PATH (the test
# installs it), never a real engine.
#
# Usage:  internal/terminal/mutate-attach-controls.sh
# Exit:   0 every control proved load-bearing
#         1 a control is not load-bearing
#         2 usage / environment error
set -uo pipefail
export KUBECONFIG=/dev/null

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "go is not on PATH" >&2; exit 2; }
[ -f "$repo/go.mod" ] || { echo "no go.mod at $repo" >&2; exit 2; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutate-attach.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT

fails=0
cases=0

fresh_copy() {
  local d="$tmp/$1"
  rm -rf "$d"; mkdir -p "$d"
  (cd "$repo" && tar --exclude='./.git' --exclude='./cockpit/node_modules' -cf - .) | (cd "$d" && tar -xf -)
  echo "$d"
}

# mutate <dir> <file> <perl-expr> — apply a perl -0 substitution and insist it
# changed the file (a mutation that silently matches nothing proves nothing).
mutate() {
  local dir=$1 file=$2 expr=$3
  local before after
  before=$(cksum < "$dir/$file")
  perl -0pi -e "$expr" "$dir/$file"
  after=$(cksum < "$dir/$file")
  if [ "$before" = "$after" ]; then
    echo "MUTATION DID NOT APPLY: $file :: $expr" >&2
    exit 2
  fi
}

# expect_red <label> <dir> <pkg> <test-regex>
expect_red() {
  local label=$1 dir=$2 pkg=$3 re=$4
  cases=$((cases + 1))
  local out
  out=$(cd "$dir" && go test -count=1 -timeout 180s -run "$re" "$pkg" 2>&1)
  if [ $? -eq 0 ]; then
    fails=$((fails + 1))
    printf 'NOT LOAD-BEARING: %s\n  %s %s still PASSES with the control removed\n' "$label" "$pkg" "$re"
  else
    printf 'red as expected: %s\n' "$label"
    if printf '%s\n' "$out" | grep -q -E 'undefined|build failed|syntax error|declared and not used'; then
      fails=$((fails + 1))
      printf 'RED FOR THE WRONG REASON (build error): %s\n' "$label"
    fi
    printf '%s\n' "$out" | grep -E '^\s+[a-z0-9_]+\.go:[0-9]+' | head -2 | sed 's/^/    /'
  fi
}

T=internal/terminal

echo "== row 2: one file links the container and local drivers, never the pod driver =="
d=$(fresh_copy links)
mutate "$d" cmd/deskd/drivers.go 's|(\t"github.com/medici-finance/assay/deskd/internal/terminal/local"\n)|$1\t_ "github.com/medici-finance/assay/deskd/internal/terminal/pod"\n|'
expect_red "drivers.go re-adds the pod import" "$d" ./cmd/deskd/ '^TestDeskdLinksContainerAndLocalOnly$'
d=$(fresh_copy links-local)
mutate "$d" cmd/deskd/drivers.go 's|\t_ "github.com/medici-finance/assay/deskd/internal/terminal/container"\n||'
expect_red "drivers.go drops the container import" "$d" ./cmd/deskd/ '^TestDeskdLinksContainerAndLocalOnly$'

echo "== row 3: the driver is the one the entry's kind names =="
d=$(fresh_copy kind)
mutate "$d" $T/supervisor.go 's/factory, ok := s\.opts\.Drivers\(kind\)/factory, ok := s.opts.Drivers(schema.TargetContainer)/'
expect_red "supervisor ignores kind: (always the container driver)" "$d" ./$T/ '^TestSupervisorAttachesByRegistryKind$'

echo "== constraint: a cluster desk is never attached (no grant) =="
d=$(fresh_copy cluster)
# A 'helpful' edit that turns cluster attach on: drop the Start refusal and the
# run guard, and call the pod kind supported.
mutate "$d" $T/supervisor.go 's/\t\tcase h\.spec\.Kind == schema\.TargetPod:\n.*?\n.*?\n\t\t\ts\.transition\(h, AttachFailed, ReasonUnauthorized, 0, nil, nil\)\n//s'
mutate "$d" $T/supervisor.go 's/\tif kind == schema\.TargetPod \{\n.*?\n\t\}\n//s'
mutate "$d" $T/capability.go 's/c\.Support, c\.Reason = SupportCouldNotCheck, CapReasonNoClusterGrant\n\t\treturn c/c.Support = SupportSupported\n\t\treturn c/'
expect_red "the pod driver is built and attached without a grant" "$d" ./$T/ '^TestSupervisorAttachesByRegistryKind$'

echo "== row 4: the backoff ladder is capped =="
d=$(fresh_copy cap)
mutate "$d" $T/supervisor.go 's/\tif wait > BackoffCap \{\n\t\twait = BackoffCap\n\t\}\n//; s/\tif nextDelay > BackoffCap \{\n\t\tnextDelay = BackoffCap\n\t\}\n//'
expect_red "no 60s cap" "$d" ./$T/ '^TestSupervisorReattachBackoffIsBounded$'

echo "== row 4: a re-attach keeps the ring (cursor never goes backwards) =="
d=$(fresh_copy ring)
mutate "$d" $T/registry.go 's/\tn := h\.attach\(sess\)\n\tif !h\.isSupervised\(\)/\th.mu.Lock()\n\th.ring = newRing(DefaultRingBytes, DefaultRingLines)\n\th.mu.Unlock()\n\tn := h.attach(sess)\n\tif !h.isSupervised()/'
expect_red "re-attach resets the ring" "$d" ./$T/ '^TestSupervisorReattachBackoffIsBounded$'

echo "== row 5: a local desk is never re-spawned =="
d=$(fresh_copy local)
mutate "$d" $T/supervisor.go 's/\t\t\tif kind == schema\.TargetLocal \{\n\t\t\t\t\/\/ Spawned once.*?\n\t\t\t\}\n//s'
expect_red "local desk re-spawned after it exits" "$d" ./$T/ '^TestSupervisorLocalKindNeverRespawns$'
d=$(fresh_copy local-unauth)
mutate "$d" $T/supervisor.go 's/\t\tcase kind == schema\.TargetLocal:\n/\t\tcase kind == schema.TargetLocal \&\& !errors.Is(err, ErrUnauthorized):\n/'
expect_red "local desk refused as unauthorized goes on the 5 min timer" "$d" ./$T/ '^TestSupervisorLocalKindNeverRespawns$'

echo "== row 6: unauthorized retries slowly =="
d=$(fresh_copy unauth)
mutate "$d" $T/supervisor.go 's/wait, reason = UnauthorizedRetry, ReasonUnauthorized/wait, reason = BackoffBase, ReasonUnauthorized/'
expect_red "unauthorized on the fast ladder" "$d" ./$T/ '^TestAttachFailureReasonsAreHonest$'

echo "== row 6: the driver's error text never reaches the wire =="
d=$(fresh_copy leak)
mutate "$d" $T/supervisor.go 's/\t\t\twait, delay = s\.backoff\(delay\)\n\t\t\treason = ReasonUnreachable/\t\t\twait, delay = s.backoff(delay)\n\t\t\treason = err.Error()/'
expect_red "wire reason carries the driver error" "$d" ./$T/ '^TestAttachFailureReasonsAreHonest$'

echo "== row 7: a GET never causes an attach =="
d=$(fresh_copy lazy)
mutate "$d" internal/serve/v1.go 's/\th, found := s\.Terminal\.Resolve\(id\)/\tif fac, ok := terminal.DriverFor("container"); ok {\n\t\tif tg, err := fac(); err == nil {\n\t\t\t_ = s.Terminal.Attach(r.Context(), id, tg, terminal.Size{Cols: 80, Rows: 24})\n\t\t}\n\t}\n\th, found := s.Terminal.Resolve(id)/'
expect_red "lazy attach on the terminal read" "$d" ./internal/serve/ '^TestGetNeverTriggersAttach$'
# The same attach placed AFTER the stream's first flush, on the container
# driver only: the recording docker is its only observer (round-8 advisory on
# F-cor-6's fix). The walk must still see it.
d=$(fresh_copy lazy-post-flush)
mutate "$d" internal/serve/v1_terminal.go 's/(\tif !st\.state\(true\) \{\n\t\treturn\n\t\}\n)/$1\tif fac, ok := terminal.DriverFor("container"); ok {\n\t\tif tg, err := fac(); err == nil {\n\t\t\t_ = s.Terminal.Attach(r.Context(), id, tg, terminal.Size{Cols: 80, Rows: 24})\n\t\t}\n\t}\n/'
expect_red "lazy container attach after the stream's first flush" "$d" ./internal/serve/ '^TestGetNeverTriggersAttach$'

echo "== row 8: the supervisor holds no write reference =="
d=$(fresh_copy write)
printf '\nfunc (s *Supervisor) wake(r *Registry, id string) { _ = r.Write(id, []byte("\\n")) }\n' >> "$d/$T/supervisor.go"
expect_red "supervisor sends a newline to wake the prompt" "$d" ./$T/ '^TestSupervisorHoldsNoWriteReference$'

echo "== off switch =="
d=$(fresh_copy off)
mutate "$d" $T/supervisor.go 's/\t\tcase s\.opts\.Mode == AttachModeOff:\n\t\t\ts\.transition\(h, AttachOff, ReasonDisabled, 0, nil, nil\)\n//'
expect_red "--terminal-attach=off ignored" "$d" ./$T/ '^TestSupervisorOffModeDisablesAll$'

echo "== deskd-wide in-flight cap =="
d=$(fresh_copy sem)
mutate "$d" $T/supervisor.go 's/sem: make\(chan struct\{\}, MaxAttachInFlight\)/sem: make(chan struct{}, 64)/'
expect_red "no deskd-wide attempt cap" "$d" ./$T/ '^TestSupervisorGlobalInFlightCap$'

echo "== Stop detaches deskd's own sessions =="
d=$(fresh_copy stop)
mutate "$d" $T/supervisor.go 's/\tvar detaches sync\.WaitGroup\n.*?\tdetaches\.Wait\(\)\n//s'
mutate "$d" $T/supervisor.go 's/\t\t\tcase <-ctx\.Done\(\):\n\t\t\t\ts\.detach\(h\.deskID\)\n\t\t\t\treturn/\t\t\tcase <-ctx.Done():\n\t\t\t\treturn/'
expect_red "Stop leaves the session open" "$d" ./$T/ '^TestSupervisorStopDetaches$'

echo "== one session per desk =="
d=$(fresh_copy second)
# (neutralised, not deleted: deleting it leaves `live` unused, a build error —
# which would be red for the wrong reason)
mutate "$d" $T/registry.go 's/\tif live \{/\tif false && live {/'
expect_red "a second attach is accepted" "$d" ./$T/ '^TestRegistryRefusesSecondAttach$'

echo "== row 101: capability is decided before any attempt =="
d=$(fresh_copy windows)
mutate "$d" $T/capability.go 's/\tif host\.OS == "windows" \{\n\t\tc\.Support, c\.Reason = SupportUnsupported, CapReasonNoPTYBackend\n\t\treturn c\n\t\}\n//'
expect_red "Windows host treated as supported (profiles)" "$d" ./$T/ '^TestTerminalPlatformProfiles$'
expect_red "Windows host treated as supported (flow)" "$d" ./internal/serve/ '^TestTerminalCapabilityFlow$'
expect_red "Windows host treated as supported (session open, store)" "$d" ./$T/ '^TestSessionOpenRefusedForUnsupportedKind$'
expect_red "Windows host treated as supported (session open, HTTP)" "$d" ./internal/serve/ '^TestSessionOpenRefusedOnUnsupportedHost$'

echo "== Task 3: permission refusals are ErrUnauthorized =="
d=$(fresh_copy perm)
mutate "$d" $T/container/driver.go 's/\tif inspectErr != nil && permissionDenied\(inspectErr\) \{\n.*?\n\t\}\n//s'
expect_red "container: socket permission denied reads unreachable" "$d" ./$T/container/ '^TestContainerPermissionDeniedIsUnauthorized$'

echo "== row 13 / F-cor-1: detach ends deskd's own shell inside the container =="
d=$(fresh_copy detach)
mutate "$d" $T/container/session.go 's/\tdetachErr := s\.endShell\(\)\n\treturn errors\.Join\(detachErr, s\.ptmx\.Close\(\)\)/\t_ = errors.Join\n\treturn s.ptmx.Close()/'
expect_red "Close only closes the host pty (the in-container bash -l leaks)" "$d" ./$T/container/ '^TestDetachEndsTheExecShell$'
d=$(fresh_copy detach-ended)
mutate "$d" $T/container/session.go 's/ \|\| s\.eof\.Load\(\) \|\| s\.exited\.Load\(\)//'
expect_red "Close signals a pid whose exec already ended" "$d" ./$T/container/ '^TestDetachAfterTheShellEndedSignalsNothing$'
d=$(fresh_copy detach-pid1)
mutate "$d" $T/container/session.go 's/if err != nil \|\| p <= 1 \{/if err != nil || p < 1 {/'
expect_red "the pid line may name PID 1 (the container's init)" "$d" ./$T/container/ '^TestPIDLineParsing$'
d=$(fresh_copy detach-pid-race)
mutate "$d" $T/container/session.go 's/\tselect \{\n\tcase <-s\.pidDecided:\n\tcase <-wait\.C:\n\t\}\n//'
expect_red "Close before the pid line is read gives up on an unknown pid" "$d" ./$T/container/ '^TestDetachBeforeThePIDLineStillEndsTheShell$'
d=$(fresh_copy bash-env)
mutate "$d" $T/container/session.go 's/"bash", "--posix", "-c"/"bash", "-c"/'
expect_red "attach runs bash outside posix mode (\$BASH_ENV can print the pid line)" "$d" ./$T/container/ '^TestAttachPIDLineIgnoresBashEnv$'
expect_red "detach runs bash outside posix mode" "$d" ./$T/container/ '^TestDetachExecIgnoresBashEnv$'
d=$(fresh_copy detach-ppid)
mutate "$d" $T/container/session.go 's/entered "\$1" \|\| exit 0; kill -HUP/kill -HUP/'
expect_red "detach signals a pid it cannot confirm docker exec entered (forged pid line)" "$d" ./$T/container/ '^TestDetachScriptSignalsOnlyAnExecEnteredProcess$'
d=$(fresh_copy detach-ppid-accept)
mutate "$d" $T/container/session.go 's/\[ "\$p" = 0 \]/[ "\$p" = never ]/'
expect_red "detach never accepts a docker-exec-entered process (inert detach, the shell leaks)" "$d" ./$T/container/ '^TestDetachScriptPPidGate$'
d=$(fresh_copy detach-unset)
mutate "$d" $T/container/session.go 's/`unset -f read kill command; set \+e \+x; entered/`set +e +x; entered/'
expect_red "an exported read function passes the PPid gate for any pid" "$d" ./$T/container/ '^TestDetachScriptPPidGate$'
d=$(fresh_copy detach-unset-kill)
mutate "$d" $T/container/session.go 's/unset -f read kill command;/unset -f read command;/'
expect_red "an exported kill function signals another process" "$d" ./$T/container/ '^TestDetachScriptEnvCannotSignalAnotherProcess$'
d=$(fresh_copy detach-unset-command)
mutate "$d" $T/container/session.go 's/unset -f read kill command;/unset -f read kill;/'
expect_red "an exported command function signals another process" "$d" ./$T/container/ '^TestDetachScriptEnvCannotSignalAnotherProcess$'
d=$(fresh_copy detach-sleep-bare)
mutate "$d" $T/container/session.go 's/command -p sleep 0\.1;/sleep 0.1;/'
expect_red "a bare sleep runs an exported sleep function" "$d" ./$T/container/ '^TestDetachScriptEnvCannotSignalAnotherProcess$'
d=$(fresh_copy detach-sleep-path)
mutate "$d" $T/container/session.go 's/command -p sleep 0\.1;/command sleep 0.1;/'
expect_red "sleep is looked up on the container's PATH" "$d" ./$T/container/ '^TestDetachScriptEnvCannotSignalAnotherProcess$'
# `set +x` is load-bearing only on a bash that imports SHELLOPTS and PS4 into a
# `--posix -c` shell (bash 5.3 as a non-root user does; 3.2 does not). Probe
# the host's bash first; where it does not import them, say so and skip the
# case rather than report a control as not load-bearing.
if env -i PATH="$PATH" SHELLOPTS=xtrace 'PS4=$((p=0))' bash --posix -c 'p=7; :; [ "$p" = 0 ]' 2>/dev/null; then
  d=$(fresh_copy detach-xtrace)
  mutate "$d" $T/container/session.go 's/ set \+e \+x;/ set +e;/'
  expect_red "an exported xtrace with a crafted PS4 passes the PPid gate" "$d" ./$T/container/ '^TestDetachScriptPPidGate$'
else
  echo "skipped (host bash does not import SHELLOPTS/PS4 into bash --posix -c): set +x"
fi
# `set +e` is load-bearing only on a bash that imports SHELLOPTS=errexit into
# a `--posix -c` shell; probe it the same way.
if ! env -i PATH="$PATH" SHELLOPTS=errexit bash --posix -c 'false; :' 2>/dev/null; then
  d=$(fresh_copy detach-errexit)
  mutate "$d" $T/container/session.go 's/ set \+e \+x;/ set +x;/'
  expect_red "an imported errexit turns a SIGKILL on an exited target into a failed detach" "$d" ./$T/container/ '^TestDetachScriptExitsZeroUnderImportedErrexit$'
else
  echo "skipped (host bash does not import SHELLOPTS=errexit into bash --posix -c): set +e"
fi
d=$(fresh_copy detach-report)
mutate "$d" $T/container/session.go 's/(\.Run\(\); err != nil) \{\n\t\treturn fmt\.Errorf\("container %s: the detach exec/$1 \&\& false {\n\t\treturn fmt.Errorf("container %s: the detach exec/'
expect_red "a failed detach exec reads as a clean detach" "$d" ./$T/container/ '^TestDetachFailureIsReported$'
d=$(fresh_copy detach-log)
mutate "$d" $T/supervisor.go 's/\tif err := s\.reg\.Detach\(id\); err != nil && s\.opts\.Logf != nil \{/\tif err := s.reg.Detach(id); false \&\& err != nil \&\& s.opts.Logf != nil {/'
expect_red "the supervisor drops a failed detach" "$d" ./$T/ '^TestSupervisorLogsAFailedDetach$'
d=$(fresh_copy detach-recheck)
mutate "$d" $T/container/session.go 's/done; entered "\$1" \|\| exit 0; kill -KILL/done; kill -KILL/'
expect_red "SIGKILL without re-checking the target is still docker-exec-entered" "$d" ./$T/container/ '^TestDetachScriptRechecksBeforeKill$'
d=$(fresh_copy detach-budget)
mutate "$d" $T/container/session.go 's/const killGrace = time\.Second/const killGrace = 2 * time.Second/; s/"\$i" -lt 10 \]/"\$i" -lt 20 ]/'
expect_red "the HUP->KILL grace goes back to 2 s (SIGKILL can fall outside the detach bound)" "$d" ./$T/container/ '^TestDetachBudgetLeavesHeadroom$'

echo "== combining merge (vi): attach off refuses every session open =="
d=$(fresh_copy open-off)
mutate "$d" $T/sessions.go 's/\tif mode != AttachModeConfigured \{/\tif false \&\& mode != AttachModeConfigured {/'
expect_red "session open ignores the attach mode (store)" "$d" ./$T/ '^TestSessionOpenRefusedWhileAttachOff$'
expect_red "session open ignores the attach mode (HTTP)" "$d" ./internal/serve/ '^TestSessionOpenRefusedWhileAttachOff$'
d=$(fresh_copy open-declare)
mutate "$d" $T/supervisor.go 's/\ts\.reg\.declareAttachMode\(s\.opts\.Mode, s\.opts\.Host\)/\ts.reg.declareAttachMode(AttachModeConfigured, s.opts.Host)/'
expect_red "Start declares configured whatever --terminal-attach says" "$d" ./$T/ '^TestSessionOpenRefusedWhileAttachOff$'
d=$(fresh_copy open-stop)
mutate "$d" $T/supervisor.go 's/\ts\.reg\.declareAttachMode\(AttachModeOff, s\.opts\.Host\)\n//'
expect_red "Stop leaves session opens allowed" "$d" ./$T/ '^TestSessionOpenRefusedWhileAttachOff$'

echo "== combining merge (vii): a kind the host cannot attach is refused =="
d=$(fresh_copy open-cap)
# (neutralised, not deleted: deleting it leaves `host` unused, a build error)
mutate "$d" $T/sessions.go 's/\tif c := CapabilityFor\(base\.spec, host\); c\.Support != SupportSupported \{/\tif c := CapabilityFor(base.spec, host); false \&\& c.Support != SupportSupported {/'
expect_red "session open skips CapabilityFor (store)" "$d" ./$T/ '^TestSessionOpenRefusedForUnsupportedKind$'
expect_red "session open skips CapabilityFor (HTTP)" "$d" ./internal/serve/ '^TestSessionOpenRefusedOnUnsupportedHost$'

echo "== combining merge pr192-F2 (iv): Stop closes custom sessions =="
d=$(fresh_copy stop-custom)
mutate "$d" $T/supervisor.go 's/\tdetaches\.Add\(1\)\n\tgo func\(\) \{\n\t\tdefer detaches\.Done\(\)\n\t\ts\.reg\.closeCustomSessions\(\)\n\t\}\(\)\n//'
expect_red "Stop leaves custom sessions open" "$d" ./$T/ '^TestSupervisorStopClosesCustomSessions$'
d=$(fresh_copy stop-inflight)
mutate "$d" $T/sessions.go 's/\tif r\.attachMode != AttachModeConfigured \{\n\t\tr\.mu\.Unlock\(\)\n\t\treturn contract\.SessionRow\{\}, ErrAttachOff\n\t\}\n//'
expect_red "an open in flight across Stop inserts after the close snapshot" "$d" ./$T/ '^TestSupervisorStopRefusesAnInFlightOpen$'

echo "== review F-cor-7: a custom session's failed close is logged, never on the wire =="
d=$(fresh_copy close-return)
mutate "$d" $T/sessions.go 's/\tif sess != nil \{\n\t\tr\.logFailedDetach\(id, sess\.Close\(\)\)\n\t\}\n\treturn nil\n/\tif sess != nil {\n\t\treturn sess.Close()\n\t}\n\treturn nil\n/'
expect_red "CloseSession returns the driver's close error (500 + driver text on DELETE)" "$d" ./internal/serve/ '^TestSessionCloseKeepsDriverTextOffTheWire$'
expect_red "CloseSession returns the driver's close error (store)" "$d" ./$T/ '^TestCloseSessionLogsAFailedDetach$'
d=$(fresh_copy close-log)
mutate "$d" $T/sessions.go 's/\tif logf != nil \{\n\t\tlogf\("deskd: terminal attach: desk=%s detach failed/\tif false \&\& logf != nil {\n\t\tlogf("deskd: terminal attach: desk=%s detach failed/'
expect_red "a custom session's failed close is dropped on DELETE" "$d" ./$T/ '^TestCloseSessionLogsAFailedDetach$'
expect_red "a custom session's failed close is dropped on Stop" "$d" ./$T/ '^TestSupervisorStopLogsAFailedCustomSessionDetach$'
d=$(fresh_copy close-log-wire)
mutate "$d" $T/supervisor.go 's/\t\treg\.declareDetachLog\(opts\.Logf\)\n//'
expect_red "the Supervisor never hands its Logf to the session store" "$d" ./$T/ '^TestSupervisorStopLogsAFailedCustomSessionDetach$'

echo "== combining merge pr192-F2 (i): pump tears down only its own session =="
d=$(fresh_copy pump-epoch)
mutate "$d" $T/registry.go 's/\tours := h\.epochN == epochN\n/\tours := true\n/'
expect_red "pump tears down a newer session" "$d" ./$T/ '^TestPumpTearsDownOnlyItsOwnSession$'
d=$(fresh_copy pump-seq)
mutate "$d" $T/registry.go 's/\tn := h\.attach\(sess\)\n(\tif !h\.isSupervised)/\tn := h.attach(sess) - 1\n$1/'
expect_red "attachHandle hands pump the wrong epoch sequence" "$d" ./$T/ '^TestPumpTearsDownOnlyItsOwnSession$'

echo "== combining merge S5: the origin floor runs first on the combined tree =="
d=$(fresh_copy floor-off)
mutate "$d" internal/serve/v1_terminal_actions.go 's/(func refuseCrossSite\(w http\.ResponseWriter, r \*http\.Request, needJSON bool\) bool \{\n)/$1\tif needJSON || !needJSON {\n\t\treturn false\n\t}\n/'
expect_red "no origin floor on the three terminal writes" "$d" ./internal/serve/ '^TestTerminalWritesRefuseCrossSiteFirstOnTheCombinedTree$'
d=$(fresh_copy floor-late)
mutate "$d" internal/serve/v1_terminal_actions.go 's/(\tif refuseCrossSite\(w, r, true\) \{\n\t\treturn\n\t\}\n)(\tif _, err := t\.Authority\.Principal\(r\); err != nil \{\n\t\twriteAuthorityErr\(w, err\)\n\t\treturn\n\t\}\n)/$2$1/'
expect_red "the floor runs after identity on open" "$d" ./internal/serve/ '^TestTerminalWritesRefuseCrossSiteFirstOnTheCombinedTree$'

echo "== row 15: --once never attaches =="
d=$(fresh_copy once)
mutate "$d" cmd/deskd/main.go 's/(\t\ttermRegistry\.Len\(\), termRegistry\.WithTarget\(\), attachBanner\(attachMode, \*once\)\)\n)/$1\tterminal.NewSupervisor(termRegistry, nil, terminal.SupervisorOptions{Mode: attachMode, Logf: func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\\n", a...) }}).Start(context.Background())\n/'
expect_red "supervisor started before the --once return" "$d" ./cmd/deskd/ '^TestOnceModeNeverAttaches$'

echo
echo "cases: $cases, not load-bearing: $fails"
[ "$fails" -eq 0 ] || exit 1
exit 0
