package cellreader

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/cachequiet"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("CELL_READER_FIXTURE") == "1" {
		fixtureNative()
		os.Exit(0)
	}
	cachequiet.Isolate()
	os.Exit(m.Run())
}
func fixtureNative() {
	scan := bufio.NewScanner(os.Stdin)
	send := func(v any) { json.NewEncoder(os.Stdout).Encode(v) }
	var threadID, turnID string
	replyText := `{"kind":"informational"}`
	for scan.Scan() {
		var req message
		if json.Unmarshal(scan.Bytes(), &req) != nil {
			os.Exit(3)
		}
		reply := func(v any) { send(map[string]any{"id": req.ID, "result": v}) }
		switch req.Method {
		case "initialize":
			reply(map[string]any{})
			if os.Getenv("CELL_READER_FLOOD") == "1" {
				for i := 0; i < 1000; i++ {
					send(map[string]any{"method": "warning", "params": map[string]any{}})
				}
			}
		case "initialized":
		case "thread/start":
			var p map[string]any
			json.Unmarshal(req.Params, &p)
			env, ok := p["environments"].([]any)
			roots, rok := p["runtimeWorkspaceRoots"].([]any)
			tools, tok := p["dynamicTools"].([]any)
			cfg, _ := p["config"].(map[string]any)
			features, _ := cfg["features"].(map[string]any)
			if !ok || !rok || !tok || len(env) != 0 || len(roots) != 0 || len(tools) != 0 || p["sandbox"] != "read-only" || p["approvalPolicy"] != "never" || cfg["web_search"] != "disabled" || features["stable_environment_tools"] != false || features["plugins"] != false || os.Getenv("GH_TOKEN") != "" || os.Getenv("DESK_COMMS_KEY") != "" {
				send(map[string]any{"id": req.ID, "error": map[string]any{"code": -1, "message": "containment not enforced"}})
				continue
			}
			threadID = "fixture-thread"
			reply(map[string]any{"thread": map[string]string{"id": threadID}})
		case "turn/start":
			var p map[string]any
			json.Unmarshal(req.Params, &p)
			env, ok := p["environments"].([]any)
			if !ok || len(env) != 0 || p["approvalPolicy"] != "never" {
				os.Exit(4)
			}
			if os.Getenv("CELL_READER_HELLO") == "1" {
				var input []struct{ Text string }
				raw, _ := json.Marshal(p["input"])
				json.Unmarshal(raw, &input)
				replyText = "land-report"
				for _, part := range input {
					if strings.Contains(part.Text, "Screen this outbound cell message") {
						replyText = "clean-send"
					}
				}
			}
			turnID = "fixture-turn"
			reply(map[string]any{"turn": map[string]string{"id": turnID}})
			send(map[string]any{"id": 999, "method": "item/commandExecution/requestApproval", "params": map[string]any{}})
		case "":
			if string(req.ID) != "999" || req.Error == nil {
				os.Exit(5)
			}
			send(map[string]any{"method": "item/agentMessage/delta", "params": map[string]string{"threadId": threadID, "turnId": turnID, "delta": replyText}})
			send(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": threadID, "turn": map[string]string{"id": turnID, "status": "completed"}}})
		case "turn/interrupt":
			reply(map[string]any{})
		}
	}
}
func fixtureOptions(t *testing.T) Options {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return Options{Binary: binary, SHA256: hex.EncodeToString(sum[:]), Model: "fixture-model", Effort: "low", Home: t.TempDir(), Workspace: t.TempDir()}
}
func TestContainedReaderACPAndCallbackRefusal(t *testing.T) {
	t.Setenv("CELL_READER_FIXTURE", "1")
	t.Setenv("GH_TOKEN", "fixture-secret")
	t.Setenv("DESK_COMMS_KEY", "fixture-key")
	in, write := io.Pipe()
	out, output := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, fixtureOptions(t), in, output); output.Close() }()
	defer write.Close()
	defer out.Close()
	reader := bufio.NewReader(out)
	call := func(id int, method string, params any) message {
		t.Helper()
		if err := json.NewEncoder(write).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				t.Fatal(err)
			}
			var m message
			if json.Unmarshal(line, &m) != nil {
				t.Fatal("invalid ACP output")
			}
			if string(m.ID) == fmt.Sprint(id) {
				return m
			}
		}
	}
	if r := call(1, "initialize", map[string]int{"protocolVersion": 1}); r.Error != nil {
		t.Fatal(r.Error)
	}
	if r := call(2, "session/new", map[string]any{}); r.Error != nil {
		t.Fatal(r.Error)
	}
	if r := call(3, "session/set_mode", map[string]string{"sessionId": "fixture-thread", "modeId": "agent-full-access"}); r.Error == nil {
		t.Fatal("mode escalation allowed")
	}
	if r := call(4, "session/set_mode", map[string]string{"sessionId": "fixture-thread", "modeId": "default"}); r.Error != nil {
		t.Fatal(r.Error)
	}
	if r := call(5, "session/prompt", map[string]any{"sessionId": "fixture-thread", "prompt": []map[string]string{{"type": "text", "text": "fixture classification"}}}); r.Error != nil {
		t.Fatal(r.Error)
	}
	write.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestContainedReaderPinAndFloodCleanup(t *testing.T) {
	t.Setenv("CELL_READER_FIXTURE", "1")
	o := fixtureOptions(t)
	o.SHA256 = "wrong"
	if err := Serve(context.Background(), o, nil, io.Discard); err == nil {
		t.Fatal("changed binary accepted")
	}
	t.Setenv("CELL_READER_FLOOD", "1")
	o = fixtureOptions(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, o, bufio.NewReaderSize(&emptyReader{}, 16), io.Discard) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("flood blocked EOF cleanup")
	}
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
