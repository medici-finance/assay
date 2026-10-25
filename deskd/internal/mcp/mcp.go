// Package mcp is deskd's MCP (Model Context Protocol) surface: the write path
// the loops themselves use (design §7.1). It is the AGENTIC-FIRST surface —
// the HTTP read surface (internal/serve) and the panes it feeds are renderings
// of these tools, not the other way round.
//
// # The two deliberate absences ARE the contract
//
// There is no desk__answer and no desk__adopt, and their absence is enforced
// STRUCTURALLY, not by a comment. Every tool declares the identity any
// platform write it performs is attributed to ([Tool.AuthorsAs]); the ONLY
// value that field may hold is [AppIdentity], the service's own App. No tool
// may author as [HumanIdentity()] (`operator`, the answer/adopt credential). The
// authority test (§11) enumerates the whole registry and asserts exactly that:
// no MCP path can produce a operator-authored mutation. See [AuthorityReport].
//
// # Provenance is enforced by caller identity, not parameter trust
//
// desk__escalate ignores any provenance a loop caller supplies, so a loop
// cannot dress its question as a tripped wire (spec §1.4). Ruling D4 puts the
// policy engine in service contract, so every v1 filing derives `agent-raised`
// from the ABSENCE of the policy-raised label — the enforcement rule ships
// now, the only producer of the other value does not.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// AppIdentity is the ONLY identity an MCP tool may author a platform write as:
// the service's own App (design §6.5-adjacent, "the App is the service's
// hands"). It is a bot, never a human.
const AppIdentity = "assay-desk-app[bot]"

// HumanIdentity() is the answer/adopt credential (§6.4). No MCP tool may EVER
// author as it — that is the whole of the desk__answer / desk__adopt absence,
// stated as a value the authority test can compare against.
func HumanIdentity() string { return humanid.Login() }

// protocolVersion is the MCP protocol revision this server speaks.
const protocolVersion = "2024-11-05"

// Tool is one registered MCP tool.
//
// The two fields that carry the authority contract are [Tool.Mutates] and
// [Tool.AuthorsAs]. A read tool has Mutates=false and AuthorsAs="". A tool
// that writes the platform has Mutates=true and AuthorsAs=[AppIdentity] — and
// [AuthorityReport] fails the build if any tool says otherwise. A field that
// the authority test reads is worth more than a paragraph a reviewer skims.
type Tool struct {
	Name        string
	Description string
	// Mutates is true when calling the tool can write GitHub. Read tools are
	// false and are structurally incapable of a write: their handlers hold no
	// Filer.
	Mutates bool
	// AuthorsAs names the identity any platform write is attributed to. It is
	// "" for a read tool and [AppIdentity] for a mutation. It is NEVER a human
	// identity — the authority test asserts it.
	AuthorsAs string
	// InputSchema is a JSON Schema object describing arguments, surfaced by
	// tools/list. Optional; a nil schema advertises no constraints.
	InputSchema map[string]any
	// Scope is how Dispatch authorizes a call before Handler runs. The zero
	// value is [ScopeCell]: a tool must opt OUT of cell authorization, never in.
	Scope Scope
	// Handler runs the tool. Read handlers ignore call.Filer entirely.
	Handler func(ctx context.Context, s *Server, call Call) (any, error)
}

// Call is one tool invocation: the raw arguments plus the CALLER IDENTITY the
// transport injected. The caller login is NOT a tool parameter — it is who the
// MCP connection authenticated as — which is what makes provenance
// enforcement (spec §1.4) unforgeable by a loop's own arguments.
type Call struct {
	Args json.RawMessage
	// CallerLogin is the App/bot login the MCP connection authenticated as.
	// desk__escalate joins it to a configured loop for attribution and uses it
	// as the rate-limit / flood-collapse key; a loop cannot spoof it, because
	// it never travels in the arguments.
	CallerLogin string
}

// Server is the MCP surface over the read surface index.
type Server struct {
	DB     *index.DB
	Config *schema.File
	Dir    adapter.AutomationDirectory
	// Filer is the App-identity write path. It is nil in a read-only
	// deployment (no write token), and the mutating tools then return an
	// honest could-not-check rather than a silent no-op.
	Filer  Filer
	Leases LeaseStore
	Now    func() time.Time
	// Terminal is the cockpit terminal registry (the service contract), read by
	// the minimal desk__desks tool. Nil means the tool returns could-not-check
	// (never a fabricated empty list).
	Terminal *terminal.Registry

	// caller authenticates every /mcp request before any tool is reached
	// (the service contract P3). It is never nil on a constructed Server:
	// [NewServer] defaults it to loopback-only.
	caller CallerAuth

	tools    map[string]Tool
	order    []string
	rl       *rateLimiter
	floodCfg FloodConfig
}

// Options configure a Server.
type Options struct {
	DB     *index.DB
	Config *schema.File
	Dir    adapter.AutomationDirectory
	Filer  Filer
	Leases LeaseStore
	Now    func() time.Time
	Flood  FloodConfig
	// Terminal is the cockpit terminal registry (the service contract), read by
	// the desk__desks tool. Nil is a legal read-only-of-nothing state.
	Terminal *terminal.Registry
	// Caller authenticates every /mcp request (the service contract P3). Nil means
	// LOOPBACK-ONLY: the machine boundary is the authentication, which is the
	// laptop shape and the in-cluster shape where a per-pod proxy fronts the
	// endpoint. cmd/deskd supplies a bearer-validating one in --auth=bearer.
	Caller CallerAuth
}

// NewServer builds the server and registers the full §7.1 tool table.
func NewServer(o Options) *Server {
	now := o.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	flood := o.Flood
	if flood.Window == 0 {
		flood = DefaultFloodConfig
	}
	leases := o.Leases
	if leases == nil {
		leases = NewMemoryLeaseStore(now)
	}
	caller := o.Caller
	if caller == nil {
		caller = CallerAuthFunc(loopbackCallerAuth)
	}
	s := &Server{
		caller:   caller,
		DB:       o.DB,
		Config:   o.Config,
		Dir:      o.Dir,
		Filer:    o.Filer,
		Leases:   leases,
		Now:      now,
		Terminal: o.Terminal,
		rl:       newRateLimiter(flood),
		floodCfg: flood,
	}
	s.register(registry())
	return s
}

// register indexes the tool set and asserts the authority contract at
// construction. A registry that could author as a human is a programming
// error that must never reach a running server, so it PANICS here rather than
// failing quietly — the same reasoning as the read surface's structural guard.
func (s *Server) register(tools []Tool) {
	rep := AuthorityReport(tools)
	if !rep.Clean() {
		panic("mcp: registry violates the authority contract: " + rep.Summary)
	}
	s.tools = map[string]Tool{}
	s.order = nil
	for _, t := range tools {
		s.tools[t.Name] = t
		s.order = append(s.order, t.Name)
	}
	sort.Strings(s.order)
}

// Tools returns the registered tools in a stable order.
func (s *Server) Tools() []Tool {
	out := make([]Tool, 0, len(s.order))
	for _, name := range s.order {
		out = append(out, s.tools[name])
	}
	return out
}

// ErrUnknownTool is returned when a call names a tool that does not exist —
// the desk__answer / desk__adopt case. It is a first-class error, never a
// fabricated empty success (Verify row 2).
type ErrUnknownTool struct{ Name string }

func (e *ErrUnknownTool) Error() string {
	return fmt.Sprintf("no such tool %q — this surface exposes no answer/adopt path; "+
		"adopting a process change and answering an escalation are the human's (design §7.1, §6.2)", e.Name)
}

// Dispatch runs one tool by name. An unknown tool is an error, not an empty
// result: the two deliberate absences must be visibly absent.
//
// Every call is AUTHORIZED here, before its handler runs ([Server.authorize]):
// a cell-scoped tool serves or writes only the cell its caller is a configured
// loop of, and only that cell's repositories. Authentication alone admits no
// cell.
func (s *Server) Dispatch(ctx context.Context, name string, call Call) (any, error) {
	t, ok := s.tools[name]
	if !ok {
		return nil, &ErrUnknownTool{Name: name}
	}
	if err := s.authorize(t, call); err != nil {
		return nil, err
	}
	return t.Handler(ctx, s, call)
}

// ---- JSON-RPC 2.0 over HTTP ------------------------------------------------

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// callParams is the tools/call params shape.
type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// CallerAuth authenticates the caller of an MCP request. It is the gate
// the service contract property P3 adds: before this, /mcp was mounted ahead of the
// capability loop in cmd/deskd and performed no authentication of its own, so
// with --addr 0.0.0.0 the surface was reachable unauthenticated and its
// mutating tools would write as the desk App with a caller login the requester
// supplied in a header.
//
// An implementation returns the AUTHENTICATED caller login and ok=true, or
// ok=false to refuse. When it returns a non-empty login, that login REPLACES
// anything the X-Desk-Caller header carried: an authenticated caller is not
// asked who it is. When it returns an empty login with ok=true, the caller is
// authenticated by the machine boundary (loopback) and the header is the
// per-pod identity the cluster's own auth set, which is the deployment shape
// design §9.3 describes.
type CallerAuth interface {
	AuthenticateCaller(r *http.Request) (login string, ok bool)
}

// CallerAuthFunc adapts a function to [CallerAuth].
type CallerAuthFunc func(r *http.Request) (string, bool)

// AuthenticateCaller implements [CallerAuth].
func (f CallerAuthFunc) AuthenticateCaller(r *http.Request) (string, bool) { return f(r) }

// loopbackCallerAuth is the DEFAULT when no [CallerAuth] is wired: trust the
// machine boundary and nothing else. A request that did not arrive over
// loopback is refused, so an unguarded off-loopback bind cannot expose the
// agent surface even if the bind gate (internal/identity.CheckBind) were
// somehow bypassed. This is defence in depth, on a different signal (the
// connection's remote address) in a different component (this handler) from the
// startup refusal.
func loopbackCallerAuth(r *http.Request) (string, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || !ip.IsLoopback() {
		return "", false
	}
	return "", true
}

// Handler serves the MCP JSON-RPC surface over HTTP POST, to be mounted at
// /mcp alongside the read surface in cmd/deskd. The in-cluster endpoint is how
// pod loops reach desk__escalate without leaving the cluster (design §9.3).
//
// Every request is authenticated FIRST ([Options.Caller], defaulting to
// loopback-only). Only then is the caller login resolved — from the
// authenticator when it names one, else from the X-Desk-Caller header, which
// the cluster's per-pod auth sets from the pod's App identity. It is NEVER read
// from the tool arguments — that is the whole of the spec §1.4 provenance
// enforcement — and it is never read from an UNAUTHENTICATED request at all.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{-32600, "MCP is JSON-RPC over POST"}})
			return
		}
		login, ok := s.caller.AuthenticateCaller(r)
		if !ok {
			// 401 before the body is read: an unauthenticated caller learns
			// nothing about the tool table, and no tool runs.
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "the MCP surface is authenticated: this request is neither on the loopback interface nor carrying a token the service can validate (the service contract P3)",
			})
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{-32700, "parse error: " + err.Error()}})
			return
		}
		caller := login
		if caller == "" {
			// Machine-boundary authentication: the header is the per-pod
			// identity the cluster set. It is read ONLY here, after the request
			// has been authenticated, and it is a CLAIM, not a grant: Dispatch
			// still requires it to name a configured loop of the cell each call
			// acts on, and the request-origin guard in front of /mcp keeps a
			// browser page from making the claim on a loopback deployment.
			caller = r.Header.Get("X-Desk-Caller")
		}
		resp := s.handleRPC(r.Context(), req, caller)
		writeRPC(w, resp)
	})
}

func (s *Server) handleRPC(ctx context.Context, req rpcRequest, caller string) rpcResponse {
	out := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		out.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"serverInfo":      map[string]any{"name": "deskd", "version": "the service contract"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
	case "tools/list":
		var list []map[string]any
		for _, t := range s.Tools() {
			entry := map[string]any{"name": t.Name, "description": t.Description}
			if t.InputSchema != nil {
				entry["inputSchema"] = t.InputSchema
			}
			list = append(list, entry)
		}
		out.Result = map[string]any{"tools": list}
	case "tools/call":
		var p callParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			out.Error = &rpcError{-32602, "invalid params: " + err.Error()}
			return out
		}
		res, err := s.Dispatch(ctx, p.Name, Call{Args: p.Arguments, CallerLogin: caller})
		if err != nil {
			var unknown *ErrUnknownTool
			if isUnknownTool(err, &unknown) {
				out.Error = &rpcError{-32601, err.Error()}
				return out
			}
			// A tool that ran but failed is reported as an MCP tool error
			// (isError content), not a protocol error — the call was valid.
			out.Result = toolErrorResult(err)
			return out
		}
		out.Result = toolResult(res)
	default:
		out.Error = &rpcError{-32601, "unknown method " + req.Method}
	}
	return out
}

func toolResult(v any) map[string]any {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolErrorResult(err)
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(raw)}},
		"isError": false,
	}
}

func toolErrorResult(err error) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": err.Error()}},
		"isError": true,
	}
}

func writeRPC(w http.ResponseWriter, resp rpcResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

func isUnknownTool(err error, target **ErrUnknownTool) bool {
	u, ok := err.(*ErrUnknownTool)
	if ok {
		*target = u
	}
	return ok
}
