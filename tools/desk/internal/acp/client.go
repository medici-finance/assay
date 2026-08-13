package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// SessionID is the agent-assigned identifier for an ACP session, returned by
// NewSession and required by Prompt/Cancel.
type SessionID string

// Opts configures Spawn and the negotiated ACP session.
type Opts struct {
	// Dir is the working directory of the spawned agent PROCESS itself --
	// not the session cwd (that's the cwd argument to NewSession).
	Dir string
	// Env is appended to the spawned process's environment; os.Environ() is
	// inherited first, so entries here can override individual keys.
	Env []string
	// Stderr receives the agent process's stderr. Defaults to os.Stderr --
	// spike-grade: passed straight through unattributed. A real consumer
	// (brief 15) should capture and attribute it instead.
	Stderr io.Writer

	// ClientName/ClientVersion identify this client in `initialize`.
	ClientName    string
	ClientVersion string

	// SupportedProtocolVersions lists the protocol versions this client
	// understands; initialize requests the highest entry. Initialize fails
	// closed with *ErrUnsupportedProtocolVersion if the agent negotiates a
	// version outside this set. Defaults to []int{1} -- the only version
	// this spike-grade client has been driven against; see README.md.
	SupportedProtocolVersions []int

	// FSRoot, if non-empty, is the absolute-path prefix under which the
	// default FileAccessPolicy allows fs/read_text_file and
	// fs/write_text_file. Empty means the default policy refuses every fs
	// callback. Ignored if FileAccessPolicy is set.
	FSRoot string
	// PermissionPolicy decides session/request_permission callbacks. Nil
	// means DefaultRefusePermission (fail closed).
	PermissionPolicy PermissionPolicy
	// FileAccessPolicy decides fs/read_text_file and fs/write_text_file
	// callbacks. Nil means the FSRoot-scoped default (see FSRoot).
	FileAccessPolicy FileAccessPolicy
}

// InitializeResult is the caller-facing subset of the agent's `initialize`
// response.
type InitializeResult struct {
	ProtocolVersion int
	AgentName       string
	AgentVersion    string
	// AuthMethods is the raw `authMethods` array -- this spike observed it
	// come back empty ([]) when the agent already had valid ambient
	// credentials; see README.md for the full auth/billing finding.
	AuthMethods json.RawMessage
}

// PromptResult is the caller-facing subset of a `session/prompt` response:
// its arrival IS this package's turn-completion signal (Prompt blocks until
// then; SessionUpdate notifications stream on Client.Updates() concurrently
// while it does).
type PromptResult struct {
	StopReason string
}

// SessionUpdate is one `session/update` notification. Kind is the wire
// "sessionUpdate" discriminator (e.g. "agent_message_chunk", "tool_call",
// "tool_call_update", "usage_update", ...); Raw is the full update object
// for callers that need more than the discriminator.
type SessionUpdate struct {
	SessionID SessionID
	Kind      string
	Raw       json.RawMessage
}

// Client drives one spawned agent process through the ACP core flow.
type Client struct {
	cmd  *exec.Cmd
	conn *conn
	opts Opts

	updates chan SessionUpdate

	mu              sync.Mutex
	initialized     bool
	protocolVersion int

	closeOnce sync.Once
	waitErr   error
}

// Spawn starts cmd (cmd[0] plus any args) as the agent subprocess and wires
// up an ACP connection over its stdin/stdout. It does not send `initialize`
// -- call Initialize next.
func Spawn(cmd []string, opts Opts) (*Client, error) {
	if len(cmd) == 0 {
		return nil, fmt.Errorf("acp: Spawn: empty cmd")
	}
	if len(opts.SupportedProtocolVersions) == 0 {
		opts.SupportedProtocolVersions = []int{1}
	}
	if opts.ClientName == "" {
		opts.ClientName = "assay-toolkit-acp-client"
	}
	if opts.ClientVersion == "" {
		opts.ClientVersion = "spike"
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = opts.Dir
	if len(opts.Env) > 0 {
		c.Env = append(os.Environ(), opts.Env...)
	}
	c.Stderr = opts.Stderr

	stdin, err := c.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("acp: Spawn: stdin pipe: %w", err)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("acp: Spawn: stdout pipe: %w", err)
	}
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("acp: Spawn: start %v: %w", cmd, err)
	}

	cl := &Client{
		cmd:     c,
		opts:    opts,
		updates: make(chan SessionUpdate, 256),
	}
	cl.conn = newConn(stdin, stdout, cl.serveRequest, cl.handleNotify)
	return cl, nil
}

// Initialize sends `initialize`, negotiates the protocol version, and
// records agent capabilities. On an unsupported negotiated version it
// returns *ErrUnsupportedProtocolVersion and leaves the client unusable
// (NewSession/Prompt/Cancel will return ErrNotInitialized) -- fail closed
// per Task 3, rather than guessing at compatibility.
func (cl *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	want := cl.opts.SupportedProtocolVersions[len(cl.opts.SupportedProtocolVersions)-1]
	canFS := cl.opts.FileAccessPolicy != nil || cl.opts.FSRoot != ""
	params := wireInitializeParams{
		ProtocolVersion: want,
		ClientCapabilities: wireClientCapabilities{
			FS:       wireFSCapabilities{ReadTextFile: canFS, WriteTextFile: canFS},
			Terminal: false,
		},
		ClientInfo: &wireImplementation{Name: cl.opts.ClientName, Version: cl.opts.ClientVersion},
	}

	raw, err := cl.conn.call(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("acp: initialize: %w", err)
	}
	var res wireInitializeResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("acp: initialize: decode response: %w", err)
	}

	supported := false
	for _, v := range cl.opts.SupportedProtocolVersions {
		if v == res.ProtocolVersion {
			supported = true
			break
		}
	}
	if !supported {
		return nil, &ErrUnsupportedProtocolVersion{Negotiated: res.ProtocolVersion, Supported: cl.opts.SupportedProtocolVersions}
	}

	cl.mu.Lock()
	cl.initialized = true
	cl.protocolVersion = res.ProtocolVersion
	cl.mu.Unlock()

	return &InitializeResult{
		ProtocolVersion: res.ProtocolVersion,
		AgentName:       res.AgentInfo.Name,
		AgentVersion:    res.AgentInfo.Version,
		AuthMethods:     res.AuthMethods,
	}, nil
}

// ProtocolVersion returns the negotiated protocol version, or 0 if
// Initialize has not completed successfully.
func (cl *Client) ProtocolVersion() int {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.protocolVersion
}

func (cl *Client) requireInitialized() error {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if !cl.initialized {
		return ErrNotInitialized
	}
	return nil
}

// NewSession sends `session/new` with cwd as the session's working
// directory (must be absolute per the ACP schema) and returns the
// agent-assigned SessionID.
func (cl *Client) NewSession(ctx context.Context, cwd string) (SessionID, error) {
	if err := cl.requireInitialized(); err != nil {
		return "", err
	}
	params := wireNewSessionParams{Cwd: cwd, McpServers: []json.RawMessage{}}
	raw, err := cl.conn.call(ctx, "session/new", params)
	if err != nil {
		return "", fmt.Errorf("acp: session/new: %w", err)
	}
	var res wireNewSessionResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", fmt.Errorf("acp: session/new: decode response: %w", err)
	}
	return SessionID(res.SessionID), nil
}

// SetMode sends `session/set_mode`. Notable finding from the live spike
// (see README.md): the agent's default session mode ("auto") resolves
// session/request_permission internally via its own model classifier and
// never calls back to the client at all. A caller that wants this package's
// PermissionPolicy to be the real gate MUST call SetMode(ctx, id, "default")
// (or "dontAsk") right after NewSession, before the first Prompt.
func (cl *Client) SetMode(ctx context.Context, sessionID SessionID, modeID string) error {
	if err := cl.requireInitialized(); err != nil {
		return err
	}
	params := wireSetSessionModeParams{SessionID: string(sessionID), ModeID: modeID}
	_, err := cl.conn.call(ctx, "session/set_mode", params)
	if err != nil {
		return fmt.Errorf("acp: session/set_mode: %w", err)
	}
	return nil
}

// Prompt sends `session/prompt` with text as a single text content block
// and blocks until the turn completes (its return IS the turn-completion
// signal). SessionUpdate notifications for the turn stream concurrently on
// Updates().
func (cl *Client) Prompt(ctx context.Context, sessionID SessionID, text string) (*PromptResult, error) {
	if err := cl.requireInitialized(); err != nil {
		return nil, err
	}
	params := wirePromptParams{
		SessionID: string(sessionID),
		Prompt:    []wireContentBlock{{Type: "text", Text: text}},
	}
	raw, err := cl.conn.call(ctx, "session/prompt", params)
	if err != nil {
		return nil, fmt.Errorf("acp: session/prompt: %w", err)
	}
	var res wirePromptResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("acp: session/prompt: decode response: %w", err)
	}
	return &PromptResult{StopReason: res.StopReason}, nil
}

// Cancel sends the `session/cancel` notification, interrupting an in-flight
// Prompt for sessionID. It does not itself block for the resulting
// PromptResult -- the blocked Prompt call returns it (StopReason
// "cancelled" per the ACP schema) as normal.
func (cl *Client) Cancel(sessionID SessionID) error {
	if err := cl.requireInitialized(); err != nil {
		return err
	}
	if err := cl.conn.notify("session/cancel", wireCancelParams{SessionID: string(sessionID)}); err != nil {
		return fmt.Errorf("acp: session/cancel: %w", err)
	}
	return nil
}

// Updates returns the channel of SessionUpdate notifications. Buffered
// (256); spike-grade -- a caller that stops reading it can eventually cause
// updates to be dropped (never block, to avoid deadlocking the connection's
// single read loop against a synchronous request/response like
// session/request_permission arriving on the same stream).
func (cl *Client) Updates() <-chan SessionUpdate { return cl.updates }

// Close terminates the agent process and the connection. Safe to call more
// than once.
func (cl *Client) Close() error {
	cl.closeOnce.Do(func() {
		if cl.cmd.Process != nil {
			_ = cl.cmd.Process.Kill()
		}
		cl.waitErr = cl.cmd.Wait()
	})
	return cl.waitErr
}

// handleNotify is the conn's notifyHandler: only session/update reaches
// this package's flow.
func (cl *Client) handleNotify(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var n wireSessionNotification
	if err := json.Unmarshal(params, &n); err != nil {
		return
	}
	var kind wireSessionUpdateKind
	_ = json.Unmarshal(n.Update, &kind)
	update := SessionUpdate{SessionID: SessionID(n.SessionID), Kind: kind.SessionUpdate, Raw: n.Update}
	select {
	case cl.updates <- update:
	default:
		// Buffer full: drop rather than block the read loop (see Updates
		// doc). 15 should size/backpressure this properly for the real
		// dispatch path.
	}
}

// serveRequest is the conn's requestHandler: dispatches the client-mediated
// callbacks the brief requires (session/request_permission, fs/*) and
// refuses everything else (terminal/*, mcp/*, ...) with "method not found",
// consistent with this client's advertised clientCapabilities.
func (cl *Client) serveRequest(method string, params json.RawMessage) (any, *wireError) {
	switch method {
	case "session/request_permission":
		return cl.servePermission(params)
	case "fs/read_text_file":
		return cl.serveFSRead(params)
	case "fs/write_text_file":
		return cl.serveFSWrite(params)
	default:
		return nil, &wireError{Code: -32601, Message: fmt.Sprintf("acp: client does not implement %q", method)}
	}
}

func (cl *Client) servePermission(params json.RawMessage) (any, *wireError) {
	var req wireRequestPermissionParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &wireError{Code: -32602, Message: "acp: invalid session/request_permission params"}
	}
	opts := make([]PermissionOption, 0, len(req.Options))
	for _, o := range req.Options {
		opts = append(opts, PermissionOption{OptionID: o.OptionID, Name: o.Name, Kind: o.Kind})
	}
	policy := cl.opts.PermissionPolicy
	if policy == nil {
		policy = DefaultRefusePermission
	}
	decision := policy(context.Background(), PermissionRequest{
		SessionID:  SessionID(req.SessionID),
		ToolCallID: req.ToolCall.ToolCallID,
		Title:      req.ToolCall.Title,
		Kind:       req.ToolCall.Kind,
		Options:    opts,
		Raw:        params,
	})
	optID, cancel := selectOption(opts, decision.Allow)
	if cancel {
		return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
	}
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": optID}}, nil
}

func (cl *Client) fsAllowed(sessionID SessionID, path string, write bool) bool {
	policy := cl.opts.FileAccessPolicy
	if policy == nil {
		policy = rootScopedFileAccess(cl.opts.FSRoot)
	}
	return policy(context.Background(), FileAccessRequest{SessionID: sessionID, Path: path, Write: write})
}

func (cl *Client) serveFSRead(params json.RawMessage) (any, *wireError) {
	var req wireReadTextFileParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &wireError{Code: -32602, Message: "acp: invalid fs/read_text_file params"}
	}
	if !cl.fsAllowed(SessionID(req.SessionID), req.Path, false) {
		return nil, &wireError{Code: -32000, Message: fmt.Sprintf("acp: fs/read_text_file refused: %s outside policy", req.Path)}
	}
	data, err := os.ReadFile(req.Path)
	if err != nil {
		return nil, &wireError{Code: -32000, Message: fmt.Sprintf("acp: read failed: %v", err)}
	}
	return wireReadTextFileResult{Content: string(data)}, nil
}

func (cl *Client) serveFSWrite(params json.RawMessage) (any, *wireError) {
	var req wireWriteTextFileParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &wireError{Code: -32602, Message: "acp: invalid fs/write_text_file params"}
	}
	if !cl.fsAllowed(SessionID(req.SessionID), req.Path, true) {
		return nil, &wireError{Code: -32000, Message: fmt.Sprintf("acp: fs/write_text_file refused: %s outside policy", req.Path)}
	}
	if err := os.WriteFile(req.Path, []byte(req.Content), 0o644); err != nil {
		return nil, &wireError{Code: -32000, Message: fmt.Sprintf("acp: write failed: %v", err)}
	}
	return map[string]any{}, nil
}
