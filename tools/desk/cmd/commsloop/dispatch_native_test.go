package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/acp"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// TestMain intercepts the test binary when re-exec'd as a fake ACP agent — the same
// os/exec trick internal/acp and verifyloop's own dispatch_native_test.go use: the
// native dispatch spawns os.Args[0] as its runner, distinguished by an env var, so the
// whole native path is exercised with no real agent and no network.
func TestMain(m *testing.M) {
	if mode := os.Getenv("COMMSLOOP_FAKE_ACP"); mode != "" {
		runFakeExecutorAgent(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runFakeExecutorAgent implements just enough of the ACP agent side to drive
// commsloop's native dispatch through each scenario below. Newline-delimited JSON-RPC
// over stdin/stdout, exactly like the real adapter.
func runFakeExecutorAgent(mode string) {
	r := bufio.NewReaderSize(os.Stdin, 1<<20)
	const sessionID = "fake-executor-session"

	writeMsg := func(v any) {
		b, _ := json.Marshal(v)
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
	}
	readMsg := func() (map[string]any, bool) {
		line, err := r.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		var msg map[string]any
		if len(line) > 0 {
			_ = json.Unmarshal(line, &msg)
		}
		if err != nil {
			return msg, msg != nil
		}
		return msg, true
	}
	chunk := func(text string) {
		writeMsg(map[string]any{
			"jsonrpc": "2.0", "method": "session/update",
			"params": map[string]any{
				"sessionId": sessionID,
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]any{"type": "text", "text": text},
				},
			},
		})
	}

	// modeDefault mirrors verifyloop's fake: the client's PermissionPolicy is dead code
	// until session/set_mode(default) is called, so this fake only calls back for a
	// permission decision once that happened — pinning the MANDATORY set_mode call.
	modeDefault := false
	nextID := 9000

	requestPermission := func(kind, title string) string {
		pid := nextID
		nextID++
		writeMsg(map[string]any{
			"jsonrpc": "2.0", "id": pid, "method": "session/request_permission",
			"params": map[string]any{
				"sessionId": sessionID,
				"toolCall":  map[string]any{"toolCallId": "tc1", "title": title, "kind": kind},
				"options": []any{
					map[string]any{"optionId": "reject", "name": "Deny", "kind": "reject_once"},
					map[string]any{"optionId": "allow", "name": "Allow", "kind": "allow_once"},
				},
			},
		})
		resp, ok := readMsg()
		if !ok {
			return "no-response"
		}
		if result, isMap := resp["result"].(map[string]any); isMap {
			if oc, isMap2 := result["outcome"].(map[string]any); isMap2 {
				if oc["outcome"] == "cancelled" {
					return "cancelled"
				}
				if s, _ := oc["optionId"].(string); s != "" {
					return s
				}
			}
		}
		return "unknown"
	}

	for {
		msg, ok := readMsg()
		if !ok {
			return
		}
		if msg == nil {
			continue
		}
		method, _ := msg["method"].(string)
		id := msg["id"]

		switch method {
		case "initialize":
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{
				"protocolVersion": 1,
				"agentInfo":       map[string]any{"name": "fake-executor-agent", "version": "0.0.0-test"},
				"authMethods":     []any{},
			}})
		case "session/new":
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"sessionId": sessionID}})
		case "session/set_mode":
			if p, ok := msg["params"].(map[string]any); ok {
				if p["modeId"] == "default" {
					modeDefault = true
				}
			}
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{}})
		case "session/prompt":
			var report string
			switch mode {
			case "roundtrip":
				report = "Handled the dispatch.\nVERDICT: PASS\n| land-comment | 0 | ok |\n"
			case "freetext":
				report = "I looked at it but I am not giving a structured verdict at all.\n"
			case "mutate":
				outcome := "allow-internal (auto mode; no set_mode(default))"
				if modeDefault {
					outcome = requestPermission("edit", "Write worker-only-file.go")
				}
				report = "VERDICT: PASS\n| edit-attempt | 0 | perm-outcome=" + outcome + " |\n"
			default:
				report = "VERDICT: BLOCKED\n"
			}
			chunk(report)
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"stopReason": "end_turn"}})
		case "session/cancel":
			// notification, no response
		}
	}
}

// --- helpers -----------------------------------------------------------------

func executorTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("CLAUDE_SESSION_ID", "commsloop-native-test")
	return filepath.Join(home, ".config", "assay")
}

func executorItem(id, toRole string) loopengine.Item {
	return loopengine.Item{
		ID: id,
		Payload: map[string]string{
			"from": "cell-a/the-desk",
			"to":   "cell-a/" + toRole,
			"verb": "handoff",
		},
	}
}

func nativeExecutorLoop(t *testing.T, mode, toRole string, extraEnv ...string) (*Loop, loopengine.Item) {
	t.Helper()
	wt := t.TempDir()
	env := append([]string{"COMMSLOOP_FAKE_ACP=" + mode}, extraEnv...)
	l := &Loop{
		Root:          t.TempDir(),
		Native:        true,
		RunnerCmd:     []string{os.Args[0]},
		NativeEnv:     env,
		NativeTimeout: 20 * time.Second,
		MakeWorktree: func(loopengine.Item) (string, func(), error) {
			return wt, func() {}, nil
		},
	}
	return l, executorItem("commsmsg/"+mode, toRole)
}

func awaitExecutorResult(t *testing.T, h loopengine.Handle) loopengine.Result {
	t.Helper()
	select {
	case r := <-h.Done():
		return r
	case <-time.After(30 * time.Second):
		t.Fatal("native executor dispatch did not complete within deadline")
		return loopengine.Result{}
	}
}

func assertExecutorAuditHas(t *testing.T, substr string) {
	t.Helper()
	path := filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if !strings.Contains(string(data), substr) {
		t.Fatalf("audit log has no line containing %q:\n%s", substr, data)
	}
}

// --- Verify row 1: Native (happy path + core negatives) ----------------------

func TestNativeDispatchRoundTrip(t *testing.T) {
	executorTestHome(t)
	l, it := nativeExecutorLoop(t, "roundtrip", "worker-desk")

	h, err := l.Dispatch(it, loopengine.TierSession)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitExecutorResult(t, h)
	if r.Verdict != loopengine.VerdictPass {
		t.Fatalf("verdict = %q, want PASS", r.Verdict)
	}
	if len(r.Rows) != 1 || r.Rows[0].Command != "land-comment" || r.Rows[0].Exit != 0 {
		t.Fatalf("row mis-parsed: %+v", r.Rows)
	}
	if !strings.HasPrefix(r.RunnerID, "acp:") {
		t.Fatalf("RunnerID = %q, want engine-recorded acp:<cmd>", r.RunnerID)
	}
}

func TestNativeDispatchRefusesWithoutRunner(t *testing.T) {
	executorTestHome(t)
	l := &Loop{Native: true}
	_, err := l.Dispatch(executorItem("x", "worker-desk"), loopengine.TierSession)
	if err == nil || !strings.Contains(err.Error(), "no runner configured") {
		t.Fatalf("native dispatch with no runner must refuse, got: %v", err)
	}
}

func TestNativeUnparseableReportBlocked(t *testing.T) {
	executorTestHome(t)
	l, it := nativeExecutorLoop(t, "freetext", "worker-desk")

	h, err := l.Dispatch(it, loopengine.TierSession)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitExecutorResult(t, h)
	if r.Verdict != loopengine.VerdictBlocked {
		t.Fatalf("unparseable report verdict = %q, want BLOCKED", r.Verdict)
	}
	assertExecutorAuditHas(t, "did not parse")
}

// TestNativeKillSwitchRefusesSpawnBetweenItems: a STOP armed between two dispatches in
// the same drain pass halts the SECOND spawn even though the first already succeeded —
// the kill switch is checked before EVERY spawn, not once per SelectQueue cycle.
func TestNativeKillSwitchRefusesSpawnBetweenItems(t *testing.T) {
	deskDir := executorTestHome(t)
	l, it1 := nativeExecutorLoop(t, "roundtrip", "worker-desk")

	h, err := l.Dispatch(it1, loopengine.TierSession)
	if err != nil {
		t.Fatalf("first native Dispatch: %v", err)
	}
	if r := awaitExecutorResult(t, h); r.Verdict != loopengine.VerdictPass {
		t.Fatalf("first dispatch verdict = %q, want PASS", r.Verdict)
	}

	if err := os.MkdirAll(deskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deskDir, "STOP.worker-desk"), []byte("mid-drain stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	it2 := executorItem("commsmsg/second", "worker-desk")
	if _, err := l.Dispatch(it2, loopengine.TierSession); err == nil {
		t.Fatal("second native Dispatch must refuse once STOP is armed mid-drain")
	}
	assertExecutorAuditHas(t, "kill switch/stop active")
}

// --- Verify row 2: MisroutedFence ---------------------------------------------

// TestMisroutedFenceRefusesOutOfProfileWrite: a worker-shaped mutation attempted under
// the verify-desk role's profile — a deliberately mis-routed dispatch — is REFUSED at
// the policy callback. The fence holds on the PROFILE the session was fired under, not
// on anything the payload claims about itself.
func TestMisroutedFenceRefusesOutOfProfileWrite(t *testing.T) {
	executorTestHome(t)
	l, it := nativeExecutorLoop(t, "mutate", "verify-desk")

	h, err := l.Dispatch(it, loopengine.TierSession)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitExecutorResult(t, h)
	if len(r.Rows) != 1 {
		t.Fatalf("want 1 row carrying the permission outcome, got %+v", r.Rows)
	}
	if !strings.Contains(r.Rows[0].Output, "perm-outcome=reject") {
		t.Fatalf("out-of-profile mutation was NOT refused end-to-end under the mis-routed verify-desk profile: %q", r.Rows[0].Output)
	}
	assertExecutorAuditHas(t, "refuse-mutation-out-of-profile")
}

// The SAME payload under worker-desk's own profile is allowed — proving the fence keys
// on the role the session was fired under, not on the tool call's shape.
func TestMisroutedFenceAllowsInProfileWrite(t *testing.T) {
	executorTestHome(t)
	l, it := nativeExecutorLoop(t, "mutate", "worker-desk")

	h, err := l.Dispatch(it, loopengine.TierSession)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitExecutorResult(t, h)
	if len(r.Rows) != 1 || !strings.Contains(r.Rows[0].Output, "perm-outcome=allow") {
		t.Fatalf("in-profile mutation under worker-desk must be allowed, got: %+v", r.Rows)
	}
}

// --- Verify row 3: UnknownRoleProfile -----------------------------------------

func TestUnknownRoleProfileRefusesDispatch(t *testing.T) {
	executorTestHome(t)
	l, it := nativeExecutorLoop(t, "roundtrip", "some-made-up-role")

	_, err := l.Dispatch(it, loopengine.TierSession)
	if err == nil {
		t.Fatal("dispatch to an unknown role must refuse, not fall back to a permissive default")
	}
	if !strings.Contains(err.Error(), "unknown target role") {
		t.Fatalf("refusal must name the unknown-role reason, got: %v", err)
	}
	assertExecutorAuditHas(t, "unknown target role")
}

// --- Verify row 4: BudgetNoFire ------------------------------------------------

// TestBudgetNoFireOnExhaustedBudget: the per-hour firing budget exhausted for this
// dispatch's scope means zero sessions spawned — the refusal is synchronous, before any
// worktree is created or runner resolved.
func TestBudgetNoFireOnExhaustedBudget(t *testing.T) {
	executorTestHome(t)
	spawnAttempts := 0
	l := &Loop{
		Root:      t.TempDir(),
		Native:    true,
		RunnerCmd: []string{os.Args[0]},
		MakeWorktree: func(loopengine.Item) (string, func(), error) {
			spawnAttempts++
			return t.TempDir(), func() {}, nil
		},
	}
	it := executorItem("commsmsg/budget", "worker-desk")
	scope := dispatchBudgetScope(it)

	// Seed the firing budget's bucket to its cap so the NEXT attempt refuses.
	for i := 0; i < deskkit.UnnumberedCapFor(dispatchBudgetTool); i++ {
		if err := deskkit.Log(deskkit.Entry{Tool: dispatchBudgetTool, Repo: scope, Verb: "dispatch", Result: deskkit.ResultOK}); err != nil {
			t.Fatalf("seed audit entry %d: %v", i, err)
		}
	}

	_, err := l.Dispatch(it, loopengine.TierSession)
	if err == nil {
		t.Fatal("dispatch over the firing budget must refuse, not fire a session")
	}
	if !deskkit.IsRateLimited(err) {
		t.Fatalf("budget-exhausted refusal must classify as RateLimited (so the engine's own retry/backoff holds the item), got: %v", err)
	}
	if spawnAttempts != 0 {
		t.Fatalf("MakeWorktree was called %d times — a budget refusal must fire ZERO sessions", spawnAttempts)
	}
	assertExecutorAuditHas(t, "firing budget exhausted")
}

// --- pure-function unit tests --------------------------------------------------

func TestRoleAllows(t *testing.T) {
	raw := func(kind, cmd string) acp.PermissionRequest {
		body, _ := json.Marshal(map[string]any{
			"toolCall": map[string]any{"title": cmd, "kind": kind, "rawInput": map[string]any{"command": cmd}},
		})
		return acp.PermissionRequest{Kind: kind, Title: cmd, Raw: body}
	}
	cases := []struct {
		name string
		role string
		req  acp.PermissionRequest
		want bool
	}{
		{"worker read allowed", "worker-desk", raw("read", "read file"), true},
		{"worker edit allowed", "worker-desk", raw("edit", "write file"), true},
		{"worker go test allowed", "worker-desk", raw("execute", "go test ./..."), true},
		{"worker arbitrary command refused", "worker-desk", raw("execute", "curl http://evil"), false},
		{"verify read allowed", "verify-desk", raw("read", "read file"), true},
		{"verify edit refused", "verify-desk", raw("edit", "write file"), false},
		{"verify go test allowed", "verify-desk", raw("execute", "go test ./..."), true},
		{"pr-review edit refused", "pr-review-desk", raw("edit", "write file"), false},
		{"pr-review gh pr allowed", "pr-review-desk", raw("execute", "gh pr view 1"), true},
		{"the-desk delete refused", "the-desk", raw("delete", "rm file"), false},
		{"chained command refused", "worker-desk", raw("execute", "go test ./... && rm -rf /"), false},
	}
	for _, c := range cases {
		got, reason := roleAllows(c.role, c.req)
		if got != c.want {
			t.Errorf("%s: got allow=%v (%s), want %v", c.name, got, reason, c.want)
		}
	}
}

func TestResolveRoleProfile(t *testing.T) {
	for role := range KnownRoles {
		if err := resolveRoleProfile(role); err != nil {
			t.Errorf("known role %q must resolve, got: %v", role, err)
		}
	}
	if err := resolveRoleProfile("not-a-role"); err == nil {
		t.Fatal("unknown role must refuse")
	}
}

func TestParseExecutorResult(t *testing.T) {
	if _, ok := parseExecutorResult("just prose, no verdict"); ok {
		t.Fatal("free text parsed as a Result — must be ok=false")
	}
	r, ok := parseExecutorResult("VERDICT: FAIL\n| land-comment | 1 | FAIL: x |\n")
	if !ok || r.Verdict != loopengine.VerdictFail || len(r.Rows) != 1 {
		t.Fatalf("valid report mis-parsed: ok=%v r=%+v", ok, r)
	}
	if _, ok := parseExecutorResult("VERDICT: PASS\nlooked further\nVERDICT: FAIL\n"); ok {
		t.Fatal("differing verdicts across one turn are ambiguous — must be ok=false")
	}
}

func TestTargetRoleFor(t *testing.T) {
	role, err := targetRoleFor(executorItem("x", "worker-desk"))
	if err != nil || role != "worker-desk" {
		t.Fatalf("targetRoleFor = %q, %v; want worker-desk, nil", role, err)
	}
	if _, err := targetRoleFor(loopengine.Item{ID: "y", Payload: map[string]string{"to": ""}}); err == nil {
		t.Fatal("an item with no parseable target role must refuse rather than guess")
	}
}
