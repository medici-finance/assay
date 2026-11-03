#!/usr/bin/env bash
# mutate-workflow-projection.sh — fail-first evidence for Verify row 103 of
# the service contract (the 2026-10-02 task-workflow amendment): observing,
# reconnecting to or refreshing a workflow record makes zero spawn or exec
# calls on the attach side (worker-prompt kit section 9, committed mutation).
#
# WHY MUTATION AND NOT A PRE-FIX RED RUN. The test pins an absence: production
# already behaves this way, so there is no unfixed state to run it against.
# Each case below instead breaks exactly ONE control, the way a plausible future
# edit would, in a throwaway copy of the module, and requires the row-103 test
# to go red for that reason. A mutation that does not apply, or a red that is a
# build error, counts as a failure.
#
# Offline: no container engine, no cluster (KUBECONFIG=/dev/null), and no
# network. GOPROXY=off and GOTOOLCHAIN=local are exported, so no go command here
# can fetch a module or a toolchain. The module and build caches are resolved
# from `go env` BEFORE HOME is replaced with a throwaway directory (Go derives
# its default caches from HOME), so the tests run against the caller's existing
# module cache. That cache must already hold this module's dependencies (one
# `go mod download` while online); when it does not, the unmutated baseline run
# fails to set up and the script exits 2, never 1.
#
# Usage:  internal/terminal/mutate-workflow-projection.sh
# Exit:   0 the unmutated tree passes and every control proved load-bearing
#         1 a control is not load-bearing
#         2 usage / environment error (including a go test that could not set
#           up: a missing module, a toolchain download it was not allowed)
set -uo pipefail
export KUBECONFIG=/dev/null
export GOWORK=off
export GOPROXY=off
export GOTOOLCHAIN=local

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "go is not on PATH" >&2; exit 2; }
command -v perl >/dev/null 2>&1 || { echo "perl is not on PATH" >&2; exit 2; }
[ -f "$repo/go.mod" ] || { echo "no go.mod at $repo" >&2; exit 2; }

# Resolve the caches from the caller's environment before HOME moves.
gomodcache=$(go env GOMODCACHE) && [ -n "$gomodcache" ] || { echo "could not resolve GOMODCACHE from go env" >&2; exit 2; }
gocache=$(go env GOCACHE) && [ -n "$gocache" ] && [ "$gocache" != off ] || { echo "could not resolve GOCACHE from go env" >&2; exit 2; }
export GOMODCACHE="$gomodcache" GOCACHE="$gocache"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutate-wfproj.XXXXXX") || exit 2
# chmod first: anything Go wrote read-only under $tmp must not survive the trap.
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT
export HOME="$tmp/home"; mkdir -p "$HOME"

fails=0
cases=0
T=internal/terminal
TEST='^TestAttachIgnoresWorkflowProjection$'

fresh_copy() {
  local d="$tmp/$1"
  rm -rf "$d"; mkdir -p "$d"
  (cd "$repo" && tar --exclude='./.git' --exclude='./cockpit/node_modules' -cf - .) | (cd "$d" && tar -xf -)
  echo "$d"
}

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

# run_row <dir>: row 103's test in <dir>; sets $out and $rc.
run_row() {
  out=$(cd "$1" && go test -count=1 -timeout 180s -v -run "$TEST" ./$T/ 2>&1)
  rc=$?
}

# setup_failed: the last run never reached the test (no module, no toolchain,
# no network). That is an environment error, never a verdict on a control.
setup_failed() {
  printf '%s\n' "$out" | grep -q -E '\[setup failed\]|module lookup disabled|cannot find module providing|missing go\.sum entry|go: downloading|toolchain not available|GOTOOLCHAIN=local'
}

# expect_red <label> <dir> <subtest-regex-that-must-fail>
expect_red() {
  local label=$1 dir=$2 sub=$3
  cases=$((cases + 1))
  run_row "$dir"
  if [ $rc -eq 0 ]; then
    fails=$((fails + 1))
    printf 'NOT LOAD-BEARING: %s\n  row 103 still PASSES with the control broken\n' "$label"
    return
  fi
  if setup_failed; then
    printf 'ENVIRONMENT ERROR (go test could not set up): %s\n' "$label" >&2
    printf '%s\n' "$out" | head -5 >&2
    exit 2
  fi
  if printf '%s\n' "$out" | grep -q -E 'undefined|build failed|syntax error|declared and not used|\[build failed\]'; then
    fails=$((fails + 1))
    printf 'RED FOR THE WRONG REASON (build error): %s\n' "$label"
    return
  fi
  if ! printf '%s\n' "$out" | grep -q -E -- '--- FAIL: TestAttachIgnoresWorkflowProjection'; then
    printf 'ENVIRONMENT ERROR (go test failed without running row 103): %s\n' "$label" >&2
    printf '%s\n' "$out" | head -5 >&2
    exit 2
  fi
  if ! printf '%s\n' "$out" | grep -q -E -- "--- FAIL: TestAttachIgnoresWorkflowProjection/$sub"; then
    fails=$((fails + 1))
    printf 'WRONG SUBTEST RED: %s (wanted %s)\n' "$label" "$sub"
    return
  fi
  printf 'red as expected: %s\n' "$label"
  printf '%s\n' "$out" | grep -E '^\s+workflow_projection_test\.go:[0-9]+' | head -2 | sed 's/^/    /'
}

echo "== baseline: the unmutated tree passes row 103 =="
d=$(fresh_copy baseline)
run_row "$d"
if [ $rc -ne 0 ]; then
  if setup_failed; then
    echo "ENVIRONMENT ERROR: go test could not set up offline (is this module's dependency set in $GOMODCACHE?)" >&2
  else
    echo "ENVIRONMENT ERROR: row 103 is red on the unmutated tree, so no mutation below would mean anything" >&2
  fi
  printf '%s\n' "$out" | head -10 >&2
  exit 2
fi
printf '%s\n' "$out" | grep -q -- '--- PASS: TestAttachIgnoresWorkflowProjection ' || { echo "ENVIRONMENT ERROR: the baseline run did not report the row-103 PASS line" >&2; exit 2; }
echo "green as expected: unmutated tree"

echo "== a workflow record becomes a registry entry =="
d=$(fresh_copy resolve-registers)
mutate "$d" $T/registry.go 's/(\th := r\.handles\[id\]\n)(\tr\.mu\.Unlock\(\)\n)\tif h == nil \{\n\t\treturn contract\.TerminalHandle\{\}, false\n\t\}\n/$1\tif h == nil {\n\t\th = newHandleAt(id, nil, r.now, r.epoch)\n\t\tr.handles[id] = h\n\t\tr.order = append(r.order, id)\n\t}\n$2/'
expect_red "Resolve lazily registers an unknown id as a desk" "$d" 'observing_workflow_records'

echo "== a workflow id resolves to the desk it embeds =="
d=$(fresh_copy prefix-resolve)
mutate "$d" $T/registry.go 's/(func \(r \*Registry\) handle\(id string\) \*Handle \{\n\tr\.mu\.Lock\(\)\n\tdefer r\.mu\.Unlock\(\)\n)\treturn r\.handles\[id\]\n/$1\tif h := r.handles[id]; h != nil {\n\t\treturn h\n\t}\n\tfor _, d := range r.order {\n\t\tif len(id) > len(d) && id[:len(d)] == d && id[len(d)] == \x27\/\x27 {\n\t\t\treturn r.handles[d]\n\t\t}\n\t}\n\treturn nil\n/'
expect_red "an attempt id under a desk id reaches that desk" "$d" 'observing_workflow_records'

echo "== a viewer disconnect ends the session (so the supervisor re-attaches) =="
d=$(fresh_copy close-ends-session)
mutate "$d" $T/follow.go 's/(\tdelete\(f\.h\.followers, f\)\n)\tf\.h\.mu\.Unlock\(\)\n/$1\tsess := f.h.session\n\tf.h.mu.Unlock()\n\tif sess != nil {\n\t\t_ = sess.Close()\n\t}\n/'
expect_red "Follower.Close closes the desk session" "$d" 'reconnecting_viewers'

echo "== the registry attach path, with the supervisor bypassed =="
d=$(fresh_copy attach-registers)
mutate "$d" $T/registry.go 's/(func \(r \*Registry\) Attach\(ctx context\.Context, id string, t Target, size Size\) error \{\n\th := r\.handle\(id\)\n)\tif h == nil \{\n\t\treturn fmt\.Errorf\("terminal: no configured desk %q to attach", id\)\n\t\}\n/$1\tif h == nil {\n\t\th = newHandleAt(id, &TargetSpec{Kind: schema.TargetLocal}, r.now, r.epoch)\n\t}\n/'
expect_red "Registry.Attach drives a driver for a workflow id" "$d" 'lower_layer_with_the_supervisor_bypassed'

echo "== a read surface spawns through the package driver table =="
d=$(fresh_copy since-spawns)
mutate "$d" $T/registry.go 's/(func \(r \*Registry\) Since\(id string, cursor uint64\) \(\[\]byte, error\) \{\n\th := r\.handle\(id\)\n\tif h == nil \{\n)/$1\t\tif tgt, err := RegisteredDriver(schema.TargetLocal); err == nil {\n\t\t\t_, _ = tgt.Attach(context.Background(), &TargetSpec{Kind: schema.TargetLocal}, DefaultAttachSize)\n\t\t}\n/'
expect_red "Since on a workflow id attaches a local shell through RegisteredDriver" "$d" 'observing_workflow_records'

echo "== a read surface attaches the embedded desk through the driver table =="
d=$(fresh_copy alive-attaches)
mutate "$d" $T/registry.go 's/(func \(r \*Registry\) Alive\(id string\) bool \{\n\th := r\.handle\(id\)\n\tif h == nil \{\n)/$1\t\tfor _, d := range r.ids() {\n\t\t\tif p := r.handle(d); p != nil && p.spec != nil && strings.HasPrefix(id, d+"\/") {\n\t\t\t\tif tgt, err := RegisteredDriver(p.spec.Kind); err == nil {\n\t\t\t\t\t_, _ = tgt.Attach(context.Background(), p.spec, DefaultAttachSize)\n\t\t\t\t}\n\t\t\t}\n\t\t}\n/'
expect_red "Alive on a workflow id attaches the desk it embeds through RegisteredDriver" "$d" 'observing_workflow_records'

echo "== the epoch read registers a workflow id =="
d=$(fresh_copy epoch-registers)
mutate "$d" $T/send.go 's/(func \(r \*Registry\) Epoch\(id string\) string \{\n\th := r\.handle\(id\)\n\tif h == nil \{\n)\t\treturn ""\n/$1\t\tr.mu.Lock()\n\t\tr.handles[id] = newHandleAt(id, nil, r.now, r.epoch)\n\t\tr.order = append(r.order, id)\n\t\tr.mu.Unlock()\n\t\treturn ""\n/'
expect_red "Epoch lazily registers an unknown id as a desk" "$d" 'observing_workflow_records'

echo "== a refresh ends a long-attached session (so the supervisor re-attaches) =="
d=$(fresh_copy refresh-closes)
mutate "$d" $T/registry.go 's/(func \(h \*Handle\) refreshLocked\(now time\.Time\) \{\n)/$1\tif h.session != nil && now.Sub(h.attachSince) > 5*time.Minute {\n\t\t_ = h.session.Close()\n\t}\n/'
expect_red "a refresh on a later clock closes the desk session" "$d" 'refreshing_workflow_records'

echo
echo "cases: $cases, not load-bearing: $fails"
[ "$fails" -eq 0 ] || exit 1
exit 0
