package acp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// TestMain intercepts the test binary re-exec'd as a fake ACP agent
// subprocess (the standard os/exec-package trick: Spawn re-runs the test
// binary itself, distinguished by an env var, instead of shelling out to a
// real npx agent). This lets Task 4's tests run with no live agent and no
// network access.
func TestMain(m *testing.M) {
	if mode := os.Getenv("ACP_TEST_FAKE_AGENT"); mode != "" {
		runFakeAgent(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runFakeAgent implements just enough of the agent side of ACP -- over
// os.Stdin/os.Stdout, newline-delimited JSON-RPC, exactly like the real
// adapter -- to drive this package's Client through the three scenarios
// Task 4 requires:
//
//	"happy"             -- initialize (protocolVersion 1) -> session/new ->
//	                       session/prompt streams one update, then completes
//	                       with stopReason "end_turn".
//	"permission-refuse" -- same, but session/prompt first issues a
//	                       session/request_permission callback to the
//	                       client and blocks for the reply; stopReason
//	                       reflects whether the client's decision was
//	                       "reject" or not.
//	"bad-version"       -- initialize responds with protocolVersion 999,
//	                       outside any real client's supported set, to
//	                       exercise the unexpected-version refusal.
func runFakeAgent(mode string) {
	r := bufio.NewReaderSize(os.Stdin, 1<<20)
	const sessionID = "fake-session-1"

	writeMsg := func(v any) {
		b, _ := json.Marshal(v)
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
	}
	readMsg := func() (map[string]any, bool) {
		line, err := r.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		var m map[string]any
		if len(line) > 0 {
			_ = json.Unmarshal(line, &m)
		}
		if err != nil {
			return m, m != nil
		}
		return m, true
	}

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
			pv := 1
			if mode == "bad-version" {
				pv = 999
			}
			writeMsg(map[string]any{
				"jsonrpc": "2.0", "id": id,
				"result": map[string]any{
					"protocolVersion": pv,
					"agentInfo":       map[string]any{"name": "fake-acp-agent", "version": "0.0.0-test"},
					"authMethods":     []any{},
				},
			})

		case "session/new":
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"sessionId": sessionID}})

		case "session/set_mode":
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{}})

		case "session/prompt":
			writeMsg(map[string]any{
				"jsonrpc": "2.0", "method": "session/update",
				"params": map[string]any{
					"sessionId": sessionID,
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]any{"type": "text", "text": "hi"},
					},
				},
			})

			selectedOptionID := ""
			if mode == "permission-refuse" {
				permID := nextAgentID
				nextAgentID++
				writeMsg(map[string]any{
					"jsonrpc": "2.0", "id": permID, "method": "session/request_permission",
					"params": map[string]any{
						"sessionId": sessionID,
						"toolCall":  map[string]any{"toolCallId": "tc1", "title": "Write x.txt", "kind": "edit"},
						"options": []any{
							map[string]any{"optionId": "reject", "name": "Deny", "kind": "reject_once"},
							map[string]any{"optionId": "allow", "name": "Allow", "kind": "allow_once"},
						},
					},
				})
				resp, ok2 := readMsg()
				if ok2 {
					if result, isMap := resp["result"].(map[string]any); isMap {
						if oc, isMap2 := result["outcome"].(map[string]any); isMap2 {
							selectedOptionID, _ = oc["optionId"].(string)
						}
					}
				}
			}

			stop := "end_turn"
			if selectedOptionID == "reject" {
				stop = "refusal"
			}
			writeMsg(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"stopReason": stop}})

		case "session/cancel":
			// Notification: no response.

		case "fs/read_text_file", "fs/write_text_file":
			// Not exercised by any current mode.
		}
	}
}
