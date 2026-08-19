package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/acp"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// TestMain intercepts the test binary when it is re-exec'd as a fake ACP agent (the same
// os/exec trick the acp package uses in its own TestMain): the native dispatch spawns
// os.Args[0] as its runner, distinguished by VERIFYLOOP_FAKE_ACP, so the whole native path is
// exercised with no real agent and no network. A normal `go test` run has the var unset and
// falls through to m.Run().
func TestMain(m *testing.M) {
	if mode := os.Getenv("VERIFYLOOP_FAKE_ACP"); mode != "" {
		runFakeACPAgent(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runFakeACPAgent implements just enough of the ACP agent side to drive verifyloop's native
// dispatch through each scenario. Newline-delimited JSON-RPC over stdin/stdout, exactly like
// the real adapter.
func runFakeACPAgent(mode string) {
	r := bufio.NewReaderSize(os.Stdin, 1<<20)
	const sessionID = "fake-verify-session"

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

	// modeDefault records whether the client set this session to "default" mode BEFORE the
	// prompt. A real agent left in its default "auto" mode resolves session/request_permission
	// internally and NEVER calls the client back — so the client's PermissionPolicy is dead code
	// unless it first calls SetMode(default). This fake mimics that: it only calls back for a
	// permission decision when default mode was set; otherwise it self-approves internally. That
	// pins the MANDATORY set_mode("default") call in dispatch_native.go — drop or reorder it and
	// TestNativePermissionRefusesMutation goes red (the edit is self-approved, never refused).
	modeDefault := false

	nextAgentID := 9000
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
				"agentInfo":       map[string]any{"name": "fake-verify-agent", "version": "0.0.0-test"},
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
			report := fakeAgentTurn(mode, modeDefault, sessionID, &nextAgentID, writeMsg, readMsg, chunk)
			chunk(report)
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"stopReason": "end_turn"}})
		case "session/cancel":
			// notification, no response
		}
	}
}

// fakeAgentTurn runs the mode-specific behaviour during a prompt turn and returns the final
// report text the agent will emit. modeDefault reports whether the client set "default" mode
// before this turn (see runFakeACPAgent): when false, a real agent's "auto" mode would resolve
// permissions internally without calling back, which this fake mimics.
func fakeAgentTurn(mode string, modeDefault bool, sessionID string, nextID *int, writeMsg func(any), readMsg func() (map[string]any, bool), chunk func(string)) string {
	switch mode {
	case "roundtrip":
		return "Ran the Verify table.\nVERDICT: PASS\n" +
			"| go build ./tools/desk/... | 0 | ok |\n" +
			"| go test ./tools/desk/cmd/verifyloop/... | 0 | ok |\n"

	case "freetext":
		// No VERDICT line anywhere — must land BLOCKED (parse-or-BLOCKED).
		return "I looked at the brief but I'm just going to describe things in prose without any structured verdict at all.\n"

	case "perm-mutation":
		if !modeDefault {
			// The client never set "default" mode, so a real agent would be in "auto" and would
			// self-approve this edit internally, never calling the client's PermissionPolicy.
			// Record that outcome so the mutation lands unrefused — which is exactly what the
			// test asserts must NOT happen when set_mode is wired correctly.
			return "VERDICT: PASS\n| edit-attempt | 0 | perm-outcome=allow-internal (auto mode; no set_mode(default)) |\n"
		}
		outcome := requestPermission(sessionID, "edit", "Write dispatch.go", nextID, writeMsg, readMsg)
		return "VERDICT: PASS\n| edit-attempt | 0 | perm-outcome=" + outcome + " |\n"

	case "fs-escape":
		target := os.Getenv("VERIFYLOOP_FAKE_TARGET")
		refused := requestFSRead(sessionID, target, nextID, writeMsg, readMsg)
		out := "fence-leaked"
		if refused {
			out = "fence-refused"
		}
		return "VERDICT: PASS\n| fs-read-outside-worktree | 0 | " + out + " |\n"
	}
	return "VERDICT: BLOCKED\n"
}

// requestPermission issues a session/request_permission to the client and returns the selected
// optionId (or "cancelled").
func requestPermission(sessionID, kind, title string, nextID *int, writeMsg func(any), readMsg func() (map[string]any, bool)) string {
	pid := *nextID
	*nextID++
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

// requestFSRead issues an fs/read_text_file for path and reports whether the client refused it
// (an error response) rather than returning content.
func requestFSRead(sessionID, path string, nextID *int, writeMsg func(any), readMsg func() (map[string]any, bool)) bool {
	rid := *nextID
	*nextID++
	writeMsg(map[string]any{
		"jsonrpc": "2.0", "id": rid, "method": "fs/read_text_file",
		"params": map[string]any{"sessionId": sessionID, "path": path},
	})
	resp, ok := readMsg()
	if !ok {
		return true // connection died = did not leak the content
	}
	_, hasErr := resp["error"]
	return hasErr
}

// --- helpers ---------------------------------------------------------------

// nativeTestHome points HOME at a clean temp dir (so the audit log + Guard read a fresh
// desk-tools dir) and registers DESK_LOOP so per-loop stop flags resolve.
func nativeTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "verify-desk")
	t.Setenv("CLAUDE_SESSION_ID", "verifyloop-native-test")
	return filepath.Join(home, ".config", "assay")
}

// nativeLoop builds a VerifyLoop wired for a fake-agent native dispatch in the given mode, with
// an injected worktree (a plain temp dir, no real git).
func nativeLoop(t *testing.T, mode string, extraEnv ...string) *VerifyLoop {
	t.Helper()
	wt := t.TempDir()
	env := append([]string{"VERIFYLOOP_FAKE_ACP=" + mode}, extraEnv...)
	return &VerifyLoop{
		Root:          t.TempDir(),
		TargetSHA:     "deadbeef",
		Native:        true,
		RunnerCmd:     []string{os.Args[0]},
		NativeEnv:     env,
		NativeTimeout: 20 * time.Second,
		MakeWorktree: func(loopengine.Item) (string, func(), error) {
			return wt, func() {}, nil
		},
	}
}

func awaitResult(t *testing.T, h loopengine.Handle) loopengine.Result {
	t.Helper()
	select {
	case r := <-h.Done():
		return r
	case <-time.After(30 * time.Second):
		t.Fatal("native dispatch did not complete within deadline")
		return loopengine.Result{}
	}
}

func fixtureItem() loopengine.Item {
	return loopengine.Item{
		ID:        "fixture/15",
		BriefPath: "docs/streams/fixture/brief-15-x.md",
		TargetSHA: "deadbeef",
		Payload:   map[string]string{"verify_table": "| 1 | `go test ./...` | exit 0 |"},
	}
}

// --- Task 5: full round-trip ------------------------------------------------

func TestNativeDispatchRoundTrip(t *testing.T) {
	nativeTestHome(t)
	v := nativeLoop(t, "roundtrip")

	h, err := v.Dispatch(fixtureItem(), loopengine.TierLocal)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitResult(t, h)

	if r.Verdict != loopengine.VerdictPass {
		t.Fatalf("verdict = %q, want PASS", r.Verdict)
	}
	if r.Item.ID != "fixture/15" {
		t.Fatalf("result item = %q, want fixture/15", r.Item.ID)
	}
	if len(r.Rows) != 2 {
		t.Fatalf("parsed %d evidence rows, want 2: %+v", len(r.Rows), r.Rows)
	}
	if r.Rows[0].Command != "go build ./tools/desk/..." || r.Rows[0].Exit != 0 || r.Rows[0].Output != "ok" {
		t.Fatalf("row 0 mis-parsed: %+v", r.Rows[0])
	}
	// RunnerID is recorded from what the engine spawned, never self-reported.
	if !strings.HasPrefix(r.RunnerID, "acp:") {
		t.Fatalf("RunnerID = %q, want engine-recorded acp:<cmd>", r.RunnerID)
	}
}

// --- negative tests (brief Task 4) -----------------------------------------

// TestNativeDispatchRefusesWithoutRunner: native mode with no runner configured refuses to
// dispatch rather than silently reach the network.
func TestNativeDispatchRefusesWithoutRunner(t *testing.T) {
	nativeTestHome(t)
	v := &VerifyLoop{Native: true, TargetSHA: "deadbeef"} // RunnerTable nil AND RunnerCmd empty
	_, err := v.Dispatch(fixtureItem(), loopengine.TierLocal)
	if err == nil || !strings.Contains(err.Error(), "no runner configured") {
		t.Fatalf("native dispatch with no runner must refuse, got: %v", err)
	}
}

// TestNativePermissionRefusesMutation: the permission gate refuses a mutating (edit) tool call
// end-to-end — set_mode("default") is wired, the callback reaches PermissionPolicy, and the
// verify-desk profile selects the reject option.
func TestNativePermissionRefusesMutation(t *testing.T) {
	nativeTestHome(t)
	v := nativeLoop(t, "perm-mutation")

	h, err := v.Dispatch(fixtureItem(), loopengine.TierLocal)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitResult(t, h)
	if len(r.Rows) != 1 {
		t.Fatalf("want 1 row carrying the permission outcome, got %+v", r.Rows)
	}
	if !strings.Contains(r.Rows[0].Output, "perm-outcome=reject") {
		t.Fatalf("edit permission was NOT refused end-to-end: %q", r.Rows[0].Output)
	}
	assertAuditHas(t, "refuse-mutation")
}

// TestNativeStopFlagRefusesPermission: a stop flag armed mid-turn refuses the very next
// permission callback (stop-flags checked per callback, not just at iteration boundaries).
func TestNativeStopFlagRefusesPermission(t *testing.T) {
	deskDir := nativeTestHome(t)
	if err := os.MkdirAll(deskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deskDir, "STOP.verify-desk"), []byte("mid-turn stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	v := &VerifyLoop{Root: t.TempDir(), RunnerCmd: []string{"unused"}}
	policy := v.permissionPolicy(fixtureItem(), t.TempDir())
	// A read would otherwise be allowed; the armed stop flag must refuse it anyway.
	dec := policy(context.Background(), acp.PermissionRequest{Kind: "read", Title: "cat go.mod"})
	if dec.Allow {
		t.Fatal("permission callback allowed a tool call while a stop flag was armed")
	}
}

// TestNativeUnparseableReportBlocked: a report with no parseable verdict lands as BLOCKED with
// the raw tail preserved in the audit log — free text never becomes a PASS.
func TestNativeUnparseableReportBlocked(t *testing.T) {
	nativeTestHome(t)
	v := nativeLoop(t, "freetext")

	h, err := v.Dispatch(fixtureItem(), loopengine.TierLocal)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitResult(t, h)
	if r.Verdict != loopengine.VerdictBlocked {
		t.Fatalf("unparseable report verdict = %q, want BLOCKED", r.Verdict)
	}
	assertAuditHas(t, "did not parse")
}

// TestNativeFsAccessRefusedOutsideWorktree: an fs/read_text_file for a path outside the session
// worktree is refused by the fs fence (FSRoot = worktree).
func TestNativeFsAccessRefusedOutsideWorktree(t *testing.T) {
	nativeTestHome(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("top secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := nativeLoop(t, "fs-escape", "VERIFYLOOP_FAKE_TARGET="+outside)

	h, err := v.Dispatch(fixtureItem(), loopengine.TierLocal)
	if err != nil {
		t.Fatalf("native Dispatch: %v", err)
	}
	r := awaitResult(t, h)
	if len(r.Rows) != 1 || !strings.Contains(r.Rows[0].Output, "fence-refused") {
		t.Fatalf("out-of-worktree fs read was NOT refused by the fence: %+v", r.Rows)
	}
}

// --- pure-function unit tests ----------------------------------------------

func TestVerifyDeskAllows(t *testing.T) {
	wt := "/tmp/wt"
	raw := func(kind, cmd string) acp.PermissionRequest {
		body, _ := json.Marshal(map[string]any{
			"toolCall": map[string]any{"title": cmd, "kind": kind, "rawInput": map[string]any{"command": cmd}},
		})
		return acp.PermissionRequest{Kind: kind, Title: cmd, Raw: body}
	}
	cases := []struct {
		name string
		req  acp.PermissionRequest
		want bool
	}{
		{"read allowed", raw("read", "read file"), true},
		{"edit refused", raw("edit", "write file"), false},
		{"delete refused", raw("delete", "rm file"), false},
		{"go test allowed", raw("execute", "go test ./tools/desk/..."), true},
		{"git diff allowed", raw("execute", "git diff --stat main"), true},
		{"arbitrary command refused", raw("execute", "curl http://evil"), false},
		{"chained verify command refused", raw("execute", "go test ./... && rm -rf /"), false},
		{"piped verify command refused", raw("execute", "go test ./... | tee /tmp/x"), false},
		{"empty command refused", raw("execute", ""), false},
	}
	for _, c := range cases {
		got, reason := verifyDeskAllows(c.req, wt)
		if got != c.want {
			t.Errorf("%s: got allow=%v (%s), want %v", c.name, got, reason, c.want)
		}
	}

	// Default-deny: an execute request whose command lives ONLY in the agent-chosen Title (no
	// recognized rawInput.command/cmd key) must be refused, not allowed on the title. An agent
	// could otherwise title a payload `go test ./...` while hiding the real command elsewhere.
	titleOnly, _ := json.Marshal(map[string]any{
		"toolCall": map[string]any{"title": "go test ./...", "kind": "execute"},
	})
	if allow, reason := verifyDeskAllows(acp.PermissionRequest{Kind: "execute", Title: "go test ./...", Raw: titleOnly}, wt); allow {
		t.Errorf("execute allowed on agent-supplied Title alone (%s) — default-deny requires a recognized command key", reason)
	}
}

func TestParseResult(t *testing.T) {
	if _, ok := parseResult("just some prose, no verdict here"); ok {
		t.Fatal("free text parsed as a Result — must be ok=false")
	}
	r, ok := parseResult("VERDICT: FAIL\n| go test ./... | 1 | FAIL: x_test.go |\n")
	if !ok {
		t.Fatal("a valid report failed to parse")
	}
	if r.Verdict != loopengine.VerdictFail {
		t.Fatalf("verdict = %q, want FAIL", r.Verdict)
	}
	if len(r.Rows) != 1 || r.Rows[0].Exit != 1 || r.Rows[0].Command != "go test ./..." {
		t.Fatalf("row mis-parsed: %+v", r.Rows)
	}
	// A verdict with a header + separator table still parses only the data rows.
	r2, _ := parseResult("VERDICT: PASS\n| Command | Exit | Output |\n|---|---|---|\n| go build | 0 | ok |\n")
	if len(r2.Rows) != 1 {
		t.Fatalf("header/separator rows were not skipped: %+v", r2.Rows)
	}

	// Two DIFFERING verdicts across one turn are ambiguous — a provisional PASS an agent later
	// revised to FAIL must NOT resolve to the leftmost PASS. Parse-or-BLOCKED refuses to guess.
	if _, ok := parseResult("VERDICT: PASS\nkept looking...\nfound a failure\nVERDICT: FAIL\n"); ok {
		t.Fatal("ambiguous transcript (PASS early, FAIL late) parsed — must be ok=false so it lands BLOCKED")
	}
	// The SAME verdict repeated across the turn is not ambiguous and still parses (as that verdict).
	r3, ok3 := parseResult("VERDICT: PASS\nre-checked, still good\nVERDICT: PASS\n")
	if !ok3 || r3.Verdict != loopengine.VerdictPass {
		t.Fatalf("repeated identical verdict must parse as PASS: ok=%v verdict=%q", ok3, r3.Verdict)
	}
	// A single closing verdict after prose is taken as the final verdict.
	r4, ok4 := parseResult("Ran the table, one row failed.\nVERDICT: FAIL\n")
	if !ok4 || r4.Verdict != loopengine.VerdictFail {
		t.Fatalf("final verdict after prose must parse as FAIL: ok=%v verdict=%q", ok4, r4.Verdict)
	}
}

// assertAuditHas fails unless the audit log under $HOME contains a line whose detail carries
// substr — proof that decisions are audited.
func assertAuditHas(t *testing.T, substr string) {
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
