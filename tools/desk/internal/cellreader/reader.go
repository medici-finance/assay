// Package cellreader exposes a tools-stripped Codex App Server through the small
// ACP contract used by comms. It starts no shell and grants no tool callbacks.
package cellreader

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
)

type Options struct{ Binary, SHA256, Model, Effort, Home, Workspace string }

var disabledFeatures = []string{"shell_tool", "stable_environment_tools", "view_image", "shell_snapshot", "hooks", "apps", "plugins", "remote_plugin", "tool_suggest", "multi_agent", "multi_agent_v2", "multi_agent_v2_dynamic_tools", "computer_use", "browser_use", "browser_use_external", "browser_use_full_cdp_access", "code_mode", "code_mode_only", "code_mode_host", "image_generation", "memories", "goals", "skill_search", "skill_mcp_dependency_install", "sleep_tool", "daemon_auto_start", "request_permissions_tool", "default_mode_request_user_input", "send_message_to_user_async", "current_time_reminder", "workspace_dependencies"}

type message struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type nativeRPC struct {
	input   *os.File
	mu      sync.Mutex
	pending map[string]chan message
	events  chan message
	done    chan struct{}
	serial  atomic.Int64
}

func (r *nativeRPC) write(v any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return json.NewEncoder(r.input).Encode(v)
}
func (r *nativeRPC) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	n := r.serial.Add(1)
	id := strconv.FormatInt(n, 10)
	ch := make(chan message, 1)
	r.mu.Lock()
	r.pending[id] = ch
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.pending, id); r.mu.Unlock() }()
	if err := r.write(map[string]any{"id": n, "method": method, "params": params}); err != nil {
		return nil, err
	}
	select {
	case response := <-ch:
		if response.Error != nil {
			return nil, fmt.Errorf("native reader %s failed", method)
		}
		return response.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.done:
		return nil, io.EOF
	}
}
func (r *nativeRPC) read(ctx context.Context, in io.Reader) {
	defer close(r.done)
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var m message
		if json.Unmarshal(line, &m) != nil {
			return
		}
		if m.Method != "" && len(m.ID) > 0 {
			// Never approve a command, file edit, dynamic tool or permission
			// request, including callbacks introduced by a newer CLI.
			_ = r.write(message{JSONRPC: "2.0", ID: m.ID, Error: &rpcError{-32601, "contained reader refuses callbacks"}})
		} else if len(m.ID) > 0 {
			r.mu.Lock()
			ch := r.pending[string(m.ID)]
			r.mu.Unlock()
			if ch != nil {
				select {
				case ch <- m:
				case <-ctx.Done():
					return
				}
			}
		} else if m.Method != "" {
			select {
			case r.events <- m:
			case <-ctx.Done():
				return
			}
		}
	}
}

func config(o Options) map[string]any {
	features := map[string]bool{}
	for _, key := range disabledFeatures {
		features[key] = false
	}
	return map[string]any{"model": o.Model, "model_reasoning_effort": o.Effort, "approval_policy": "never", "sandbox_mode": "read-only", "web_search": "disabled", "features": features, "tools": map[string]bool{"view_image": false}, "mcp_servers": map[string]any{}, "update_plan_tool": false, "experimental_request_user_input": false}
}
func startNative(ctx context.Context, o Options) (*nativeRPC, func(), error) {
	if o.Binary == "" || o.Model == "" || o.Home == "" || o.Workspace == "" {
		return nil, nil, fmt.Errorf("reader configuration is incomplete")
	}
	b, err := os.ReadFile(o.Binary)
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(b)
	if o.SHA256 == "" || hex.EncodeToString(sum[:]) != o.SHA256 {
		return nil, nil, fmt.Errorf("Codex executable pin changed; review and reconfigure the reader after the upgrade")
	}
	args := []string{o.Binary, "app-server", "-c", "web_search=\"disabled\"", "-c", "approval_policy=\"never\"", "-c", "sandbox_mode=\"read-only\""}
	for _, key := range disabledFeatures {
		args = append(args, "-c", "features."+key+"=false")
	}
	env := []string{}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "DESK_COMMS_") || strings.HasPrefix(key, "ASSAY_RUNNER_") || strings.HasPrefix(key, "ASSAY_GO_CACHE_") || containsSecretKey(key) || key == "CODEX_HOME" {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "CODEX_HOME="+o.Home, "KUBECONFIG=/dev/null")
	stdin, writer, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	outputRead, outputWrite := io.Pipe()
	child, cancel := context.WithCancel(ctx)
	rpc := &nativeRPC{input: writer, pending: map[string]chan message{}, events: make(chan message, 512), done: make(chan struct{})}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer stdin.Close()
		defer outputWrite.Close()
		_, _, _ = cellprocess.RunInteractive(child, args, env, o.Workspace, stdin, outputWrite, io.Discard)
	}()
	go rpc.read(child, outputRead)
	cleanup := func() { cancel(); writer.Close(); outputRead.Close(); <-finished; <-rpc.done }
	if _, err = rpc.call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": "cellctl-contained-reader", "version": "1"}, "capabilities": map[string]bool{"experimentalApi": true}}); err != nil {
		cleanup()
		return nil, nil, err
	}
	_ = rpc.write(map[string]any{"method": "initialized"})
	return rpc, cleanup, nil
}
func containsSecretKey(key string) bool {
	switch key {
	case "OPENAI_API_KEY", "CODEX_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN", "NODE_OPTIONS":
		return true
	}
	return false
}

type server struct {
	options   Options
	rpc       *nativeRPC
	mu        sync.Mutex
	output    io.Writer
	session   string
	turn      string
	prompting bool
	workers   sync.WaitGroup
}

func (s *server) send(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = json.NewEncoder(s.output).Encode(v)
}
func (s *server) reply(id json.RawMessage, value any, err error) {
	if err != nil {
		s.send(message{JSONRPC: "2.0", ID: id, Error: &rpcError{-32602, err.Error()}})
		return
	}
	b, _ := json.Marshal(value)
	s.send(message{JSONRPC: "2.0", ID: id, Result: b})
}
func (s *server) prompt(ctx context.Context, id json.RawMessage, params json.RawMessage) {
	defer func() { s.mu.Lock(); s.prompting = false; s.mu.Unlock() }()
	var p struct {
		SessionID string                        `json:"sessionId"`
		Prompt    []struct{ Type, Text string } `json:"prompt"`
	}
	if err := json.Unmarshal(params, &p); err != nil || s.session == "" || p.SessionID != s.session {
		s.reply(id, nil, fmt.Errorf("unknown reader session"))
		return
	}
	input := []map[string]string{}
	for _, part := range p.Prompt {
		if part.Type != "text" {
			s.reply(id, nil, fmt.Errorf("contained reader accepts text only"))
			return
		}
		input = append(input, map[string]string{"type": "text", "text": part.Text})
	}
	b, err := s.rpc.call(ctx, "turn/start", map[string]any{"threadId": s.session, "input": input, "model": s.options.Model, "effort": s.options.Effort, "approvalPolicy": "never", "approvalsReviewer": "user", "sandboxPolicy": map[string]any{"type": "readOnly", "networkAccess": false}, "environments": []any{}, "runtimeWorkspaceRoots": []string{}})
	if err != nil {
		s.reply(id, nil, err)
		return
	}
	var started struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	_ = json.Unmarshal(b, &started)
	if started.Turn.ID == "" {
		s.reply(id, nil, fmt.Errorf("native reader returned no turn"))
		return
	}
	s.mu.Lock()
	s.turn = started.Turn.ID
	s.mu.Unlock()
	for {
		select {
		case <-ctx.Done():
			s.reply(id, nil, ctx.Err())
			return
		case <-s.rpc.done:
			s.reply(id, nil, io.EOF)
			return
		case event := <-s.rpc.events:
			var p map[string]json.RawMessage
			_ = json.Unmarshal(event.Params, &p)
			var threadID string
			_ = json.Unmarshal(p["threadId"], &threadID)
			if threadID != "" && threadID != s.session {
				continue
			}
			var turnID string
			_ = json.Unmarshal(p["turnId"], &turnID)
			if turnID != "" && turnID != started.Turn.ID {
				continue
			}
			switch event.Method {
			case "item/agentMessage/delta":
				var text string
				_ = json.Unmarshal(p["delta"], &text)
				s.send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": s.session, "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text}}}})
			case "turn/completed":
				var t struct {
					ID, Status string
					Error      *struct{ Message string }
				}
				_ = json.Unmarshal(p["turn"], &t)
				if t.ID != started.Turn.ID {
					continue
				}
				if t.Error != nil || t.Status == "failed" {
					s.reply(id, nil, fmt.Errorf("contained reader turn failed"))
					return
				}
				s.reply(id, map[string]string{"stopReason": "end_turn"}, nil)
				return
			case "item/started":
				var item struct{ Type string }
				_ = json.Unmarshal(p["item"], &item)
				if item.Type != "userMessage" && item.Type != "agentMessage" && item.Type != "reasoning" {
					_, _ = s.rpc.call(ctx, "turn/interrupt", map[string]string{"threadId": s.session, "turnId": started.Turn.ID})
					s.reply(id, nil, fmt.Errorf("unexpected tool in contained reader; consultation refused"))
					return
				}
			}
		}
	}
}

func Serve(ctx context.Context, o Options, in io.Reader, out io.Writer) error {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	rpc, cleanup, err := startNative(child, o)
	if err != nil {
		return err
	}
	s := &server{options: o, rpc: rpc, output: out}
	defer func() { cancel(); cleanup(); s.workers.Wait() }()
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var request message
		if json.Unmarshal(line, &request) != nil {
			return fmt.Errorf("invalid ACP input")
		}
		switch request.Method {
		case "initialize":
			s.reply(request.ID, map[string]any{"protocolVersion": 1, "agentInfo": map[string]string{"name": "cellctl-contained-codex", "version": "1"}, "agentCapabilities": map[string]any{}, "authMethods": []any{}}, nil)
		case "session/new":
			if s.session != "" {
				s.reply(request.ID, nil, fmt.Errorf("reader supports one session"))
				continue
			}
			result, err := rpc.call(child, "thread/start", map[string]any{"model": o.Model, "cwd": o.Workspace, "config": config(o), "approvalPolicy": "never", "approvalsReviewer": "user", "sandbox": "read-only", "environments": []any{}, "runtimeWorkspaceRoots": []string{}, "selectedCapabilityRoots": []any{}, "dynamicTools": []any{}, "ephemeral": true})
			if err != nil {
				s.reply(request.ID, nil, err)
				continue
			}
			var r struct {
				Thread struct {
					ID string `json:"id"`
				} `json:"thread"`
			}
			_ = json.Unmarshal(result, &r)
			if r.Thread.ID == "" {
				s.reply(request.ID, nil, fmt.Errorf("native reader returned no session"))
				continue
			}
			s.session = r.Thread.ID
			s.reply(request.ID, map[string]any{"sessionId": s.session, "modes": map[string]any{"currentModeId": "default", "availableModes": []map[string]string{{"id": "default", "name": "Contained read-only"}}}}, nil)
		case "session/set_mode":
			var p struct{ SessionID, ModeID string }
			_ = json.Unmarshal(request.Params, &p)
			if s.session == "" || p.SessionID != s.session || (p.ModeID != "default" && p.ModeID != "read-only") {
				s.reply(request.ID, nil, fmt.Errorf("contained reader permits default/read-only only"))
			} else {
				s.reply(request.ID, map[string]any{}, nil)
			}
		case "session/prompt":
			s.mu.Lock()
			busy := s.prompting
			if !busy {
				s.prompting = true
			}
			s.mu.Unlock()
			if busy {
				s.reply(request.ID, nil, fmt.Errorf("reader already prompting"))
				continue
			}
			s.workers.Add(1)
			go func(id json.RawMessage, params json.RawMessage) { defer s.workers.Done(); s.prompt(child, id, params) }(request.ID, request.Params)
		case "session/cancel":
			s.mu.Lock()
			turn := s.turn
			s.mu.Unlock()
			if turn != "" {
				_, _ = rpc.call(child, "turn/interrupt", map[string]string{"threadId": s.session, "turnId": turn})
			}
		default:
			s.reply(request.ID, nil, fmt.Errorf("method unavailable in contained reader"))
		}
	}
}
